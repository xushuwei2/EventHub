package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/sanitize"
)

type TrackStore struct {
	db *sql.DB
}

func NewTrackStore(db *sql.DB) *TrackStore {
	return &TrackStore{db: db}
}

func (s *TrackStore) EventExists(ctx context.Context, eventID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM track_event WHERE event_id = ? LIMIT 1`, eventID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *TrackStore) Ingest(ctx context.Context, projectID int64, ev models.IngestTrackEvent, identity *models.TrustedIdentity) error {
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
	extra := sanitize.ExtraJSON(ev.Extra)
	roomID := sanitize.PtrToString(ev.RoomID)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO track_event (
			event_id, project_id, occurred_at, event_name,
			`+"`release`"+`, env, route, scene, module, language, runtime,
			user_id, room_id, session_id,
			device_platform, device_model, os_version, sdk_version, network_type,
			funnel_key, step_key, extra_json
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		ev.EventID, projectID, occurred, ev.EventName,
		ev.Release, ev.Env, ev.Route, ev.Scene, ev.Module, language, ev.Runtime,
		ev.UserID, roomID, ev.SessionID,
		platform, sanitize.OrUnknown(ev.DeviceModel), sanitize.OrUnknown(ev.OSVersion),
		sanitize.OrUnknown(ev.SDKVersion), sanitize.OrUnknown(ev.NetworkType),
		ev.FunnelKey, ev.StepKey, nullJSON(extra),
	)
	if err != nil {
		return err
	}

	statDate := occurred.Format("2006-01-02")

	_, err = tx.ExecContext(ctx, `
		INSERT INTO project_daily_stats (project_id, stat_date, dau, new_users, event_count)
		VALUES (?, ?, 0, 0, 1)
		ON DUPLICATE KEY UPDATE event_count = event_count + 1`,
		projectID, statDate,
	)
	if err != nil {
		return err
	}

	if ev.UserID != "" {
		var isNewUser int
		res, insErr := tx.ExecContext(ctx, `
			INSERT IGNORE INTO user_first_active (project_id, user_id, first_active_date)
			VALUES (?, ?, ?)`,
			projectID, ev.UserID, statDate,
		)
		if insErr != nil {
			return insErr
		}
		if n, _ := res.RowsAffected(); n > 0 {
			isNewUser = 1
			_, err = tx.ExecContext(ctx, `
				UPDATE project_daily_stats SET new_users = new_users + 1
				WHERE project_id = ? AND stat_date = ?`,
				projectID, statDate,
			)
			if err != nil {
				return err
			}
		}

		var existingDAU int
		err = tx.QueryRowContext(ctx, `
			SELECT 1 FROM user_daily_active
			WHERE project_id = ? AND user_id = ? AND stat_date = ? LIMIT 1`,
			projectID, ev.UserID, statDate,
		).Scan(&existingDAU)
		isFirstToday := errors.Is(err, sql.ErrNoRows)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO user_daily_active (project_id, user_id, stat_date, event_count, first_event_at, last_event_at)
			VALUES (?, ?, ?, 1, ?, ?)
			ON DUPLICATE KEY UPDATE
				event_count = event_count + 1,
				last_event_at = VALUES(last_event_at)`,
			projectID, ev.UserID, statDate, occurred, occurred,
		)
		if err != nil {
			return err
		}

		if isFirstToday {
			_, err = tx.ExecContext(ctx, `
				UPDATE project_daily_stats SET dau = dau + 1
				WHERE project_id = ? AND stat_date = ?`,
				projectID, statDate,
			)
			if err != nil {
				return err
			}
		}
		_ = isNewUser
	}

	return tx.Commit()
}
