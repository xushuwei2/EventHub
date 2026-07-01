package service

import (
	"context"
	"errors"

	"github.com/eventhub/eventhub/internal/auth"
	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/store"
)

type TrackIngestService struct {
	cfg    *config.Config
	track  *store.TrackStore
	ingest *IngestService
}

func NewTrackIngestService(cfg *config.Config, track *store.TrackStore, ingest *IngestService) *TrackIngestService {
	return &TrackIngestService{cfg: cfg, track: track, ingest: ingest}
}

func (s *TrackIngestService) ProcessBatch(ctx context.Context, ia *IngestAuth, req models.TrackBatchRequest) (models.BatchResponse, error) {
	if len(req.Events) == 0 {
		return models.BatchResponse{}, ErrInvalidEvent
	}
	if len(req.Events) > s.cfg.MaxEventsPerBatch {
		return models.BatchResponse{}, errors.New("too_many_events")
	}

	resp := models.BatchResponse{RequestID: auth.NewRequestID()}
	for _, ev := range req.Events {
		if err := validateTrackEvent(ev); err != nil {
			resp.Rejected++
			continue
		}

		exists, err := s.track.EventExists(ctx, ev.EventID)
		if err != nil {
			resp.Rejected++
			continue
		}
		if exists {
			resp.Rejected++
			continue
		}

		if err := s.track.Ingest(ctx, ia.Project.ID, ev, ia.Identity); err != nil {
			resp.Rejected++
			continue
		}
		resp.Accepted++
	}
	return resp, nil
}

func validateTrackEvent(ev models.IngestTrackEvent) error {
	if ev.EventID == "" || ev.Release == "" || ev.EventName == "" {
		return ErrInvalidEvent
	}
	if !models.ValidEnvs[ev.Env] {
		return ErrInvalidEvent
	}
	hasFunnel := ev.FunnelKey != ""
	hasStep := ev.StepKey != ""
	if hasFunnel != hasStep {
		return ErrInvalidEvent
	}
	return nil
}
