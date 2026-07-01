package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/eventhub/eventhub/internal/auth"
	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/fingerprint"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/sanitize"
	"github.com/eventhub/eventhub/internal/store"
)

var ErrInvalidEvent = errors.New("invalid_event")

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateEntry
	limit   int
	window  time.Duration
}

type rateEntry struct {
	count   int
	resetAt time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*rateEntry),
		limit:   limit,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	e, ok := r.entries[key]
	if !ok || now.After(e.resetAt) {
		r.entries[key] = &rateEntry{count: 1, resetAt: now.Add(r.window)}
		return true
	}
	if e.count >= r.limit {
		return false
	}
	e.count++
	return true
}

func (r *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for k, e := range r.entries {
			if now.After(e.resetAt) {
				delete(r.entries, k)
			}
		}
		r.mu.Unlock()
	}
}

type IngestService struct {
	cfg      *config.Config
	projects *store.ProjectStore
	ingest   *store.IngestStore
}

func NewIngestService(cfg *config.Config, projects *store.ProjectStore, ingest *store.IngestStore) *IngestService {
	return &IngestService{cfg: cfg, projects: projects, ingest: ingest}
}

type IngestAuth struct {
	Project  *models.Project
	Identity *models.TrustedIdentity
}

func (s *IngestService) ResolveAuth(ctx context.Context, authHeader string) (*IngestAuth, error) {
	authHeader = strings.TrimSpace(authHeader)
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return nil, auth.ErrInvalidToken
	}

	token := strings.TrimSpace(authHeader[7:])
	projectKey, err := projectKeyFromJWT(token)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}
	project, err := s.projects.GetByKey(ctx, projectKey)
	if err != nil {
		return nil, store.ErrProjectNotFound
	}
	if project.Status != models.ProjectStatusActive {
		return nil, store.ErrProjectNotFound
	}
	identity, err := auth.ParseReportToken(project.TrustedTokenSecret, token)
	if err != nil {
		return nil, err
	}
	return &IngestAuth{Project: project, Identity: identity}, nil
}

func (s *IngestService) ProcessBatch(ctx context.Context, ia *IngestAuth, req models.BatchRequest) (models.BatchResponse, error) {
	if len(req.Events) == 0 {
		return models.BatchResponse{}, ErrInvalidEvent
	}
	if len(req.Events) > s.cfg.MaxEventsPerBatch {
		return models.BatchResponse{}, errors.New("too_many_events")
	}

	resp := models.BatchResponse{RequestID: auth.NewRequestID()}
	for _, ev := range req.Events {
		if err := validateEvent(ev, s.cfg); err != nil {
			resp.Rejected++
			continue
		}
		ev.Message = sanitize.Text(ev.Message)
		ev.Stack = sanitize.Text(ev.Stack)

		exists, err := s.ingest.EventExists(ctx, ev.EventID)
		if err != nil {
			resp.Rejected++
			continue
		}
		if exists {
			resp.Rejected++
			continue
		}

		fp := fingerprint.Compute(ev)
		if err := s.ingest.Ingest(ctx, ia.Project.ID, ev, fp, ia.Identity); err != nil {
			resp.Rejected++
			continue
		}
		resp.Accepted++
	}
	return resp, nil
}

func validateEvent(ev models.IngestEvent, cfg *config.Config) error {
	if ev.EventID == "" || ev.Release == "" || ev.Message == "" {
		return ErrInvalidEvent
	}
	if !models.ValidCategories[ev.Category] || !models.ValidSeverities[ev.Severity] || !models.ValidEnvs[ev.Env] {
		return ErrInvalidEvent
	}
	if len(ev.Message) > cfg.MaxMessageLen || len(ev.Stack) > cfg.MaxStackLen {
		return ErrInvalidEvent
	}

	switch ev.Category {
	case "api_failure":
		if ev.APIMethod == "" || ev.APIPath == "" {
			return ErrInvalidEvent
		}
	case "ws_failure":
		if ev.WSPhase == "" {
			return ErrInvalidEvent
		}
	case "asset_failure":
		if ev.AssetType == "" {
			return ErrInvalidEvent
		}
	case "biz_error":
		if ev.BizCode == "" {
			return ErrInvalidEvent
		}
	}
	return nil
}

func projectKeyFromJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", auth.ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", auth.ErrInvalidToken
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", auth.ErrInvalidToken
	}
	pk, _ := claims["project_key"].(string)
	if pk == "" {
		return "", auth.ErrInvalidToken
	}
	return pk, nil
}
