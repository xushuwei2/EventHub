package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/eventhub/eventhub/internal/auth"
	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/store"
)

type FeedbackIngestService struct {
	cfg      *config.Config
	feedback *store.FeedbackStore
	ingest   *IngestService
}

func NewFeedbackIngestService(cfg *config.Config, feedback *store.FeedbackStore, ingest *IngestService) *FeedbackIngestService {
	return &FeedbackIngestService{cfg: cfg, feedback: feedback, ingest: ingest}
}

func (s *FeedbackIngestService) Submit(ctx context.Context, ia *IngestAuth, req models.IngestFeedbackRequest) (models.FeedbackResponse, error) {
	if err := validateFeedback(req, s.cfg.MaxFeedbackContentLen); err != nil {
		return models.FeedbackResponse{}, err
	}

	exists, err := s.feedback.Exists(ctx, req.FeedbackID)
	if err != nil {
		return models.FeedbackResponse{}, ErrInvalidEvent
	}
	if exists {
		return models.FeedbackResponse{}, ErrDuplicateFeedback
	}

	if err := s.feedback.Insert(ctx, ia.Project.ID, req, ia.Identity); err != nil {
		return models.FeedbackResponse{}, ErrInvalidEvent
	}

	return models.FeedbackResponse{
		RequestID:  auth.NewRequestID(),
		FeedbackID: req.FeedbackID,
	}, nil
}

func validateFeedback(req models.IngestFeedbackRequest, maxContent int) error {
	if req.FeedbackID == "" || req.Release == "" || req.Content == "" {
		return ErrInvalidEvent
	}
	if !models.ValidEnvs[req.Env] {
		return ErrInvalidEvent
	}
	if req.Category != "" && !models.ValidFeedbackCategories[req.Category] {
		return ErrInvalidEvent
	}
	contentLen := utf8.RuneCountInString(strings.TrimSpace(req.Content))
	if contentLen == 0 {
		return ErrInvalidEvent
	}
	if maxContent > 0 && contentLen > maxContent {
		return ErrInvalidEvent
	}
	if utf8.RuneCountInString(req.Contact) > 256 {
		return ErrInvalidEvent
	}
	return nil
}
