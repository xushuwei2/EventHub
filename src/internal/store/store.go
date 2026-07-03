package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/eventhub/eventhub/internal/fingerprint"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/sanitize"
)

var (
	ErrProjectNotFound  = errors.New("invalid_project")
	ErrProjectKeyExists = errors.New("project_key_exists")
)

type ProjectStore struct {
	db *sql.DB
}

func NewProjectStore(db *sql.DB) *ProjectStore {
	return &ProjectStore{db: db}
}

func (s *ProjectStore) GetByKey(ctx context.Context, key string) (*models.Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_key, project_name, status, trusted_token_secret
		FROM report_project WHERE project_key = ?`, key)

	var p models.Project
	if err := row.Scan(&p.ID, &p.ProjectKey, &p.ProjectName, &p.Status, &p.TrustedTokenSecret); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *ProjectStore) ListAll(ctx context.Context) ([]models.Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_key, project_name, status, trusted_token_secret
		FROM report_project ORDER BY project_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjects(rows)
}

func (s *ProjectStore) GetByID(ctx context.Context, id int64) (*models.Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_key, project_name, status, trusted_token_secret
		FROM report_project WHERE id = ?`, id)

	var p models.Project
	if err := row.Scan(&p.ID, &p.ProjectKey, &p.ProjectName, &p.Status, &p.TrustedTokenSecret); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *ProjectStore) Create(ctx context.Context, p *models.Project) error {
	status := p.Status
	if status == "" {
		status = models.ProjectStatusActive
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO report_project (project_key, project_name, status, trusted_token_secret)
		VALUES (?, ?, ?, ?)`,
		p.ProjectKey, p.ProjectName, status, p.TrustedTokenSecret)
	if err != nil {
		if isDuplicateKey(err) {
			return ErrProjectKeyExists
		}
		return err
	}
	return nil
}

func (s *ProjectStore) Update(ctx context.Context, p *models.Project) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE report_project
		SET project_name = ?, status = ?, trusted_token_secret = ?
		WHERE id = ?`,
		p.ProjectName, p.Status, p.TrustedTokenSecret, p.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrProjectNotFound
	}
	return nil
}

func scanProjects(rows *sql.Rows) ([]models.Project, error) {
	var list []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.ProjectKey, &p.ProjectName, &p.Status, &p.TrustedTokenSecret); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func isDuplicateKey(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Duplicate")
}

func (s *ProjectStore) ListActive(ctx context.Context) ([]models.Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_key, project_name, status, trusted_token_secret
		FROM report_project WHERE status = 'active' ORDER BY project_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjects(rows)
}

type IngestStore struct {
	db *sql.DB
}

func NewIngestStore(db *sql.DB) *IngestStore {
	return &IngestStore{db: db}
}

func (s *IngestStore) EventExists(ctx context.Context, eventID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM error_event WHERE event_id = ? LIMIT 1`, eventID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *IngestStore) Ingest(ctx context.Context, projectID int64, ev models.IngestEvent, fp fingerprint.Result, identity *models.TrustedIdentity) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	occurred := ev.OccurredAt.UTC()
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}

	if identity != nil {
		if identity.UserID != "" {
			ev.UserID = identity.UserID
		}
		if identity.SessionID != "" {
			ev.SessionID = identity.SessionID
		}
		if identity.RoomID != "" {
			room := identity.RoomID
			ev.RoomID = &room
		}
		if identity.Release != "" {
			ev.Release = identity.Release
		}
	}

	language := sanitize.OrUnknown(ev.Language)
	platform := sanitize.OrUnknown(ev.DevicePlatform)

	var issueID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM error_issue
		WHERE project_id = ? AND group_fingerprint = ?`,
		projectID, fp.GroupFingerprint).Scan(&issueID)

	if errors.Is(err, sql.ErrNoRows) {
		res, insErr := tx.ExecContext(ctx, `
			INSERT INTO error_issue (
				project_id, group_fingerprint, category, severity, title,
				normalized_message, normalized_stack_top, status,
				first_seen_at, last_seen_at, total_count,
				last_release, last_language, last_platform, sample_event_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, 1, ?, ?, ?, ?)`,
			projectID, fp.GroupFingerprint, ev.Category, ev.Severity, fp.Title,
			fp.NormalizedMessage, fp.NormalizedStackTop,
			occurred, occurred, ev.Release, language, platform, ev.EventID,
		)
		if insErr != nil {
			return insErr
		}
		issueID, err = res.LastInsertId()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE error_issue SET
				last_seen_at = ?, total_count = total_count + 1,
				last_release = ?, last_language = ?, last_platform = ?,
				sample_event_id = ?, severity = CASE WHEN ? = 'fatal' THEN 'fatal' ELSE severity END
			WHERE id = ?`,
			occurred, ev.Release, language, platform, ev.EventID, ev.Severity, issueID,
		)
		if err != nil {
			return err
		}
	}

	extra := sanitize.ExtraJSON(ev.Extra)
	roomID := sanitize.PtrToString(ev.RoomID)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO error_event (
			event_id, project_id, issue_id, release_fingerprint, occurred_at,
			`+"`release`"+`, env, category, severity, message, stack,
			route, scene, module, language, runtime,
			user_id, room_id, session_id,
			device_platform, device_model, os_version, sdk_version, network_type,
			api_method, api_path, http_status,
			ws_phase, ws_code, ws_reason,
			asset_type, asset_path, asset_url, biz_code, extra_json
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		ev.EventID, projectID, issueID, fp.ReleaseFingerprint, occurred,
		ev.Release, ev.Env, ev.Category, ev.Severity,
		truncate(ev.Message, 512), truncate(ev.Stack, 8192),
		ev.Route, ev.Scene, ev.Module, language, ev.Runtime,
		ev.UserID, roomID, ev.SessionID,
		platform, sanitize.OrUnknown(ev.DeviceModel), sanitize.OrUnknown(ev.OSVersion),
		sanitize.OrUnknown(ev.SDKVersion), sanitize.OrUnknown(ev.NetworkType),
		ev.APIMethod, ev.APIPath, ev.HTTPStatus,
		ev.WSPhase, ev.WSCode, ev.WSReason,
		ev.AssetType, ev.AssetPath, ev.AssetURL, ev.BizCode, nullJSON(extra),
	)
	if err != nil {
		return err
	}

	statDate := occurred.Format("2006-01-02")
	_, err = tx.ExecContext(ctx, `
		INSERT INTO error_issue_release_daily (issue_id, `+"`release`"+`, stat_date, event_count, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, 1, ?, ?)
		ON DUPLICATE KEY UPDATE
			event_count = event_count + 1,
			last_seen_at = VALUES(last_seen_at)`,
		issueID, ev.Release, statDate, occurred, occurred,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func nullJSON(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
