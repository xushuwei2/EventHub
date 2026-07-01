package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/eventhub/eventhub/internal/auth"
	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/service"
	"github.com/eventhub/eventhub/internal/store"
)

type ReportHandler struct {
	cfg     *config.Config
	ingest  *service.IngestService
}

func NewReportHandler(cfg *config.Config, ingest *service.IngestService) *ReportHandler {
	return &ReportHandler{cfg: cfg, ingest: ingest}
}

func (h *ReportHandler) Batch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxBodyBytes)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large")
		return
	}

	var req models.BatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event")
		return
	}

	ia, err := h.ingest.ResolveAuth(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		mapAuthError(w, err)
		return
	}

	resp, err := h.ingest.ProcessBatch(r.Context(), ia, req)
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

func mapAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrProjectNotFound):
		writeError(w, http.StatusUnauthorized, "invalid_project")
	case errors.Is(err, auth.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "invalid_token")
	default:
		writeError(w, http.StatusUnauthorized, "invalid_project")
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, models.ErrorResponse{Error: code})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}
