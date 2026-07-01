package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/eventhub/eventhub/internal/models"
)

var ErrFunnelNotFound = errors.New("funnel_not_found")

type FunnelStore struct {
	db *sql.DB
}

func NewFunnelStore(db *sql.DB) *FunnelStore {
	return &FunnelStore{db: db}
}

func (s *FunnelStore) ListByProject(ctx context.Context, projectID int64) ([]models.FunnelDefinition, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.id, f.project_id, p.project_key, p.project_name,
			f.funnel_key, f.funnel_name, f.window_hours, f.status
		FROM funnel_definition f
		JOIN report_project p ON p.id = f.project_id
		WHERE f.project_id = ?
		ORDER BY f.funnel_name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.FunnelDefinition
	for rows.Next() {
		var f models.FunnelDefinition
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.ProjectKey, &f.ProjectName,
			&f.FunnelKey, &f.FunnelName, &f.WindowHours, &f.Status); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (s *FunnelStore) ListAll(ctx context.Context) ([]models.FunnelDefinition, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.id, f.project_id, p.project_key, p.project_name,
			f.funnel_key, f.funnel_name, f.window_hours, f.status
		FROM funnel_definition f
		JOIN report_project p ON p.id = f.project_id
		ORDER BY p.project_key, f.funnel_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.FunnelDefinition
	for rows.Next() {
		var f models.FunnelDefinition
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.ProjectKey, &f.ProjectName,
			&f.FunnelKey, &f.FunnelName, &f.WindowHours, &f.Status); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (s *FunnelStore) GetByID(ctx context.Context, id int64) (*models.FunnelDefinition, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT f.id, f.project_id, p.project_key, p.project_name,
			f.funnel_key, f.funnel_name, f.window_hours, f.status
		FROM funnel_definition f
		JOIN report_project p ON p.id = f.project_id
		WHERE f.id = ?`, id)

	var f models.FunnelDefinition
	if err := row.Scan(&f.ID, &f.ProjectID, &f.ProjectKey, &f.ProjectName,
		&f.FunnelKey, &f.FunnelName, &f.WindowHours, &f.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFunnelNotFound
		}
		return nil, err
	}

	steps, err := s.listSteps(ctx, id)
	if err != nil {
		return nil, err
	}
	f.Steps = steps
	return &f, nil
}

func (s *FunnelStore) listSteps(ctx context.Context, funnelID int64) ([]models.FunnelStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, funnel_id, step_key, step_name, step_order
		FROM funnel_step WHERE funnel_id = ?
		ORDER BY step_order ASC`, funnelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.FunnelStep
	for rows.Next() {
		var st models.FunnelStep
		if err := rows.Scan(&st.ID, &st.FunnelID, &st.StepKey, &st.StepName, &st.StepOrder); err != nil {
			return nil, err
		}
		list = append(list, st)
	}
	return list, rows.Err()
}

func (s *FunnelStore) Create(ctx context.Context, f *models.FunnelDefinition) error {
	status := f.Status
	if status == "" {
		status = models.FunnelStatusActive
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO funnel_definition (project_id, funnel_key, funnel_name, window_hours, status)
		VALUES (?, ?, ?, ?, ?)`,
		f.ProjectID, f.FunnelKey, f.FunnelName, f.WindowHours, status)
	if err != nil {
		if isDuplicateKey(err) {
			return errors.New("funnel_key_exists")
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	f.ID = id
	return nil
}

func (s *FunnelStore) Update(ctx context.Context, f *models.FunnelDefinition) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE funnel_definition
		SET funnel_name = ?, window_hours = ?, status = ?
		WHERE id = ?`,
		f.FunnelName, f.WindowHours, f.Status, f.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrFunnelNotFound
	}
	return nil
}

func (s *FunnelStore) AddStep(ctx context.Context, funnelID int64, step models.FunnelStep) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO funnel_step (funnel_id, step_key, step_name, step_order)
		VALUES (?, ?, ?, ?)`,
		funnelID, step.StepKey, step.StepName, step.StepOrder)
	if err != nil {
		if isDuplicateKey(err) {
			return errors.New("step_key_or_order_exists")
		}
		return err
	}
	return nil
}

func (s *FunnelStore) DeleteStep(ctx context.Context, stepID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM funnel_step WHERE id = ?`, stepID)
	return err
}

type step1User struct {
	UserID    string
	Step1Time time.Time
}

func (s *FunnelStore) ComputeFunnelStats(ctx context.Context, funnel *models.FunnelDefinition, startDate, endDate time.Time) ([]models.FunnelStepStat, error) {
	if len(funnel.Steps) == 0 {
		return nil, nil
	}

	step1 := funnel.Steps[0]
	users, err := s.loadStep1Users(ctx, funnel, step1, startDate, endDate)
	if err != nil {
		return nil, err
	}

	stats := make([]models.FunnelStepStat, len(funnel.Steps))
	var prevCount int64

	for i, step := range funnel.Steps {
		var count int64
		if i == 0 {
			count = int64(len(users))
		} else {
			count = s.countStepNUsers(ctx, funnel, step, users)
		}

		stat := models.FunnelStepStat{
			StepOrder: step.StepOrder,
			StepKey:   step.StepKey,
			StepName:  step.StepName,
			UserCount: count,
		}
		if i > 0 && prevCount > 0 {
			stat.ConvFromPrev = float64(count) / float64(prevCount) * 100
		} else if i > 0 {
			stat.ConvFromPrev = 0
		}
		if len(users) > 0 {
			stat.ConvFromFirst = float64(count) / float64(len(users)) * 100
		}
		stats[i] = stat
		prevCount = count
	}
	return stats, nil
}

func (s *FunnelStore) loadStep1Users(ctx context.Context, funnel *models.FunnelDefinition, step1 models.FunnelStep, startDate, endDate time.Time) ([]step1User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, MIN(occurred_at) AS step1_at
		FROM track_event
		WHERE project_id = ? AND funnel_key = ? AND step_key = ?
			AND user_id != ''
			AND occurred_at >= ? AND occurred_at < ?
		GROUP BY user_id`,
		funnel.ProjectID, funnel.FunnelKey, step1.StepKey,
		startDate.UTC(), endDate.UTC().Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []step1User
	for rows.Next() {
		var u step1User
		if err := rows.Scan(&u.UserID, &u.Step1Time); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *FunnelStore) countStepNUsers(ctx context.Context, funnel *models.FunnelDefinition, step models.FunnelStep, step1Users []step1User) int64 {
	if len(step1Users) == 0 {
		return 0
	}
	window := time.Duration(funnel.WindowHours) * time.Hour
	var count int64
	for _, u := range step1Users {
		windowEnd := u.Step1Time.Add(window)
		var n int
		err := s.db.QueryRowContext(ctx, `
			SELECT 1 FROM track_event
			WHERE project_id = ? AND user_id = ? AND funnel_key = ? AND step_key = ?
				AND occurred_at >= ? AND occurred_at <= ?
			LIMIT 1`,
			funnel.ProjectID, u.UserID, funnel.FunnelKey, step.StepKey,
			u.Step1Time, windowEnd,
		).Scan(&n)
		if err == nil {
			count++
		}
	}
	return count
}

func (s *FunnelStore) OverallConversion(ctx context.Context, funnel *models.FunnelDefinition, startDate, endDate time.Time) (float64, int64) {
	stats, err := s.ComputeFunnelStats(ctx, funnel, startDate, endDate)
	if err != nil || len(stats) == 0 {
		return 0, 0
	}
	first := stats[0].UserCount
	last := stats[len(stats)-1].UserCount
	if first == 0 {
		return 0, first
	}
	return float64(last) / float64(first) * 100, first
}
