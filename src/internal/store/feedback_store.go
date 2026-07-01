package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/sanitize"
)

type FeedbackFilter struct {
	ProjectKey string
	Status     string
	Category   string
	UserID     string
	Limit      int
}

type FeedbackStore struct {
	db *sql.DB
}

func NewFeedbackStore(db *sql.DB) *FeedbackStore {
	return &FeedbackStore{db: db}
}

func (s *FeedbackStore) Exists(ctx context.Context, feedbackID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM user_feedback WHERE feedback_id = ? LIMIT 1`, feedbackID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *FeedbackStore) Insert(ctx context.Context, projectID int64, req models.IngestFeedbackRequest, identity *models.TrustedIdentity) error {
	submitted := req.SubmittedAt.UTC()
	if submitted.IsZero() {
		submitted = time.Now().UTC()
	}

	if identity != nil {
		if identity.UserID != "" {
			req.UserID = identity.UserID
		}
		if identity.SessionID != "" {
			req.SessionID = identity.SessionID
		}
		if identity.RoomID != "" {
			room := identity.RoomID
			req.RoomID = &room
		}
		if identity.Release != "" {
			req.Release = identity.Release
		}
	}

	category := req.Category
	if category == "" {
		category = "other"
	}

	extra := sanitize.ExtraJSON(req.Extra)
	roomID := sanitize.PtrToString(req.RoomID)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_feedback (
			feedback_id, project_id, submitted_at, `+"`release`"+`, env,
			category, content, contact, route, scene, language, runtime,
			user_id, room_id, session_id,
			device_platform, device_model, os_version, sdk_version, network_type,
			extra_json, status
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'open')`,
		req.FeedbackID, projectID, submitted, req.Release, req.Env,
		category, req.Content, strings.TrimSpace(req.Contact), req.Route, req.Scene,
		sanitize.OrUnknown(req.Language), req.Runtime,
		req.UserID, roomID, req.SessionID,
		sanitize.OrUnknown(req.DevicePlatform), sanitize.OrUnknown(req.DeviceModel),
		sanitize.OrUnknown(req.OSVersion), sanitize.OrUnknown(req.SDKVersion),
		sanitize.OrUnknown(req.NetworkType),
		nullJSON(extra),
	)
	return err
}

func (s *FeedbackStore) List(ctx context.Context, filter FeedbackFilter) ([]models.UserFeedback, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	var args []interface{}
	where := "WHERE 1=1"
	if filter.ProjectKey != "" {
		where += " AND p.project_key = ?"
		args = append(args, filter.ProjectKey)
	}
	if filter.Status != "" {
		where += " AND f.status = ?"
		args = append(args, filter.Status)
	}
	if filter.Category != "" {
		where += " AND f.category = ?"
		args = append(args, filter.Category)
	}
	if filter.UserID != "" {
		where += " AND f.user_id = ?"
		args = append(args, filter.UserID)
	}

	query := fmt.Sprintf(`
		SELECT f.id, f.feedback_id, f.project_id, p.project_key, p.project_name,
			f.submitted_at, f.release, f.env, f.category, f.content, f.contact,
			f.route, f.scene, f.language, f.runtime,
			f.user_id, f.room_id, f.session_id,
			f.device_platform, f.device_model, f.os_version, f.sdk_version, f.network_type,
			COALESCE(f.extra_json, ''), f.status, COALESCE(f.admin_note, ''),
			f.created_at, f.updated_at
		FROM user_feedback f
		JOIN report_project p ON p.id = f.project_id
		%s
		ORDER BY f.submitted_at DESC
		LIMIT ?`, where)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.UserFeedback
	for rows.Next() {
		var fb models.UserFeedback
		if err := rows.Scan(
			&fb.ID, &fb.FeedbackID, &fb.ProjectID, &fb.ProjectKey, &fb.ProjectName,
			&fb.SubmittedAt, &fb.Release, &fb.Env, &fb.Category, &fb.Content, &fb.Contact,
			&fb.Route, &fb.Scene, &fb.Language, &fb.Runtime,
			&fb.UserID, &fb.RoomID, &fb.SessionID,
			&fb.DevicePlatform, &fb.DeviceModel, &fb.OSVersion, &fb.SDKVersion, &fb.NetworkType,
			&fb.ExtraJSON, &fb.Status, &fb.AdminNote,
			&fb.CreatedAt, &fb.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, fb)
	}
	return list, rows.Err()
}

func (s *FeedbackStore) GetByID(ctx context.Context, id int64) (*models.UserFeedback, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT f.id, f.feedback_id, f.project_id, p.project_key, p.project_name,
			f.submitted_at, f.release, f.env, f.category, f.content, f.contact,
			f.route, f.scene, f.language, f.runtime,
			f.user_id, f.room_id, f.session_id,
			f.device_platform, f.device_model, f.os_version, f.sdk_version, f.network_type,
			COALESCE(f.extra_json, ''), f.status, COALESCE(f.admin_note, ''),
			f.created_at, f.updated_at
		FROM user_feedback f
		JOIN report_project p ON p.id = f.project_id
		WHERE f.id = ?`, id)

	var fb models.UserFeedback
	err := row.Scan(
		&fb.ID, &fb.FeedbackID, &fb.ProjectID, &fb.ProjectKey, &fb.ProjectName,
		&fb.SubmittedAt, &fb.Release, &fb.Env, &fb.Category, &fb.Content, &fb.Contact,
		&fb.Route, &fb.Scene, &fb.Language, &fb.Runtime,
		&fb.UserID, &fb.RoomID, &fb.SessionID,
		&fb.DevicePlatform, &fb.DeviceModel, &fb.OSVersion, &fb.SDKVersion, &fb.NetworkType,
		&fb.ExtraJSON, &fb.Status, &fb.AdminNote,
		&fb.CreatedAt, &fb.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &fb, nil
}

func (s *FeedbackStore) UpdateStatus(ctx context.Context, id int64, status, adminNote string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE user_feedback SET status = ?, admin_note = ? WHERE id = ?`,
		status, adminNote, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
