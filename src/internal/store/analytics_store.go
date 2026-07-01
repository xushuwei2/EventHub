package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/eventhub/eventhub/internal/models"
)

type AnalyticsStore struct {
	db *sql.DB
}

func NewAnalyticsStore(db *sql.DB) *AnalyticsStore {
	return &AnalyticsStore{db: db}
}

func (s *AnalyticsStore) DailyTrend(ctx context.Context, projectID int64, days int) ([]models.DailyStatRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(stat_date, '%Y-%m-%d'), dau, new_users, event_count
		FROM project_daily_stats
		WHERE project_id = ? AND stat_date >= UTC_DATE() - INTERVAL ? DAY
		ORDER BY stat_date ASC`, projectID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DailyStatRow
	for rows.Next() {
		var r models.DailyStatRow
		if err := rows.Scan(&r.StatDate, &r.DAU, &r.NewUsers, &r.EventCount); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

func (s *AnalyticsStore) MAU(ctx context.Context, projectID int64) (int64, error) {
	var mau int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM user_daily_active
		WHERE project_id = ? AND stat_date >= UTC_DATE() - INTERVAL 30 DAY`,
		projectID).Scan(&mau)
	return mau, err
}

func (s *AnalyticsStore) RetentionCohort(ctx context.Context, projectID int64, days int) ([]models.RetentionRow, error) {
	endDate := time.Now().UTC().AddDate(0, 0, -31)
	startDate := endDate.AddDate(0, 0, -days+1)

	rows, err := s.db.QueryContext(ctx, `
		SELECT first_active_date, COUNT(*) AS cohort_size
		FROM user_first_active
		WHERE project_id = ? AND first_active_date >= ? AND first_active_date <= ?
		GROUP BY first_active_date
		ORDER BY first_active_date DESC`,
		projectID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.RetentionRow
	for rows.Next() {
		var cohortDate time.Time
		var row models.RetentionRow
		if err := rows.Scan(&cohortDate, &row.CohortSize); err != nil {
			return nil, err
		}
		row.CohortDate = cohortDate.Format("2006-01-02")
		row.D1 = s.retentionRate(ctx, projectID, cohortDate, 1, row.CohortSize)
		row.D7 = s.retentionRate(ctx, projectID, cohortDate, 7, row.CohortSize)
		row.D14 = s.retentionRate(ctx, projectID, cohortDate, 14, row.CohortSize)
		row.D30 = s.retentionRate(ctx, projectID, cohortDate, 30, row.CohortSize)
		list = append(list, row)
	}
	return list, rows.Err()
}

func (s *AnalyticsStore) retentionRate(ctx context.Context, projectID int64, cohortDate time.Time, dayN int, cohortSize int64) float64 {
	if cohortSize == 0 {
		return 0
	}
	targetDate := cohortDate.AddDate(0, 0, dayN)
	if targetDate.After(time.Now().UTC()) {
		return -1
	}
	var retained int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT ufa.user_id)
		FROM user_first_active ufa
		JOIN user_daily_active uda
			ON ufa.project_id = uda.project_id AND ufa.user_id = uda.user_id
		WHERE ufa.project_id = ? AND ufa.first_active_date = ? AND uda.stat_date = ?`,
		projectID, cohortDate.Format("2006-01-02"), targetDate.Format("2006-01-02"),
	).Scan(&retained)
	if err != nil {
		return 0
	}
	return float64(retained) / float64(cohortSize) * 100
}
