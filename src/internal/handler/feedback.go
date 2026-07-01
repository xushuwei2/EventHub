package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/service"
)

type FeedbackHandler struct {
	cfg      *config.Config
	feedback *service.FeedbackIngestService
	ingest   *service.IngestService
}

func NewFeedbackHandler(cfg *config.Config, feedback *service.FeedbackIngestService, ingest *service.IngestService) *FeedbackHandler {
	return &FeedbackHandler{cfg: cfg, feedback: feedback, ingest: ingest}
}

func (h *FeedbackHandler) Submit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxBodyBytes)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large")
		return
	}

	var req models.IngestFeedbackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_feedback")
		return
	}

	ia, err := h.ingest.ResolveAuth(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		mapAuthError(w, err)
		return
	}

	resp, err := h.feedback.Submit(r.Context(), ia, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDuplicateFeedback):
			writeError(w, http.StatusConflict, "duplicate_feedback")
		case errors.Is(err, service.ErrInvalidEvent):
			writeError(w, http.StatusBadRequest, "invalid_feedback")
		default:
			writeError(w, http.StatusBadRequest, "invalid_feedback")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
