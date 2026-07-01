package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/service"
)

type TrackHandler struct {
	cfg       *config.Config
	track     *service.TrackIngestService
	ingest    *service.IngestService
}

func NewTrackHandler(cfg *config.Config, track *service.TrackIngestService, ingest *service.IngestService) *TrackHandler {
	return &TrackHandler{cfg: cfg, track: track, ingest: ingest}
}

func (h *TrackHandler) Batch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxBodyBytes)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large")
		return
	}

	var req models.TrackBatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event")
		return
	}

	ia, err := h.ingest.ResolveAuth(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		mapAuthError(w, err)
		return
	}

	resp, err := h.track.ProcessBatch(r.Context(), ia, req)
	if err != nil {
		switch err.Error() {
		case "too_many_events":
			writeError(w, http.StatusBadRequest, "too_many_events")
		default:
			writeError(w, http.StatusBadRequest, "invalid_event")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
