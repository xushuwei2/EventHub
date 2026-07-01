package fingerprint_test

import (
	"context"
	"testing"
	"time"

	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/db"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/store"
)

func TestTrackIngestUpdatesDAU(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skip(err)
	}
	database, err := db.Open(cfg.DSN())
	if err != nil {
		t.Skip(err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}

	projects := store.NewProjectStore(database)
	track := store.NewTrackStore(database)
	analytics := store.NewAnalyticsStore(database)

	p, err := projects.GetByKey(context.Background(), "demo")
	if err != nil {
		t.Skip("demo project:", err)
	}

	userID := "track_test_user_" + time.Now().Format("150405")
	eventID := "track_evt_" + userID
	now := time.Now().UTC()

	ev := models.IngestTrackEvent{
		EventID:    eventID,
		OccurredAt: now,
		Release:    "test",
		Env:        "dev",
		EventName:  "page_view",
		UserID:     userID,
	}
	identity := &models.TrustedIdentity{UserID: userID, Release: "test"}

	if err := track.Ingest(context.Background(), p.ID, ev, identity); err != nil {
		t.Fatal(err)
	}

	trend, err := analytics.DailyTrend(context.Background(), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	statDate := now.Format("2006-01-02")
	found := false
	for _, row := range trend {
		if row.StatDate == statDate && row.DAU >= 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DAU >= 1 on %s, trend: %+v", statDate, trend)
	}

	// duplicate event_id should be detectable
	exists, err := track.EventExists(context.Background(), eventID)
	if err != nil || !exists {
		t.Fatal("event should exist after ingest")
	}
}

func TestFunnelConversion(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skip(err)
	}
	database, err := db.Open(cfg.DSN())
	if err != nil {
		t.Skip(err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}

	projects := store.NewProjectStore(database)
	track := store.NewTrackStore(database)
	funnels := store.NewFunnelStore(database)

	p, err := projects.GetByKey(context.Background(), "demo")
	if err != nil {
		t.Skip(err)
	}

	suffix := time.Now().Format("150405")
	funnelKey := "test_funnel_" + suffix
	f := &models.FunnelDefinition{
		ProjectID:   p.ID,
		FunnelKey:   funnelKey,
		FunnelName:  "Test Funnel",
		WindowHours: 24,
		Status:      models.FunnelStatusActive,
	}
	if err := funnels.Create(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM funnel_step WHERE funnel_id = ?`, f.ID)
		_, _ = database.Exec(`DELETE FROM funnel_definition WHERE id = ?`, f.ID)
	})

	_ = funnels.AddStep(context.Background(), f.ID, models.FunnelStep{StepKey: "step_a", StepName: "A", StepOrder: 1})
	_ = funnels.AddStep(context.Background(), f.ID, models.FunnelStep{StepKey: "step_b", StepName: "B", StepOrder: 2})

	userID := "funnel_user_" + suffix
	identity := &models.TrustedIdentity{UserID: userID}
	now := time.Now().UTC()

	ingest := func(eid, step string, at time.Time) {
		ev := models.IngestTrackEvent{
			EventID:    eid,
			OccurredAt: at,
			Release:    "test",
			Env:        "dev",
			EventName:  step,
			FunnelKey:  funnelKey,
			StepKey:    step,
			UserID:     userID,
		}
		if err := track.Ingest(context.Background(), p.ID, ev, identity); err != nil {
			t.Fatal(err)
		}
	}

	ingest("fe_"+suffix+"_a", "step_a", now)
	ingest("fe_"+suffix+"_b", "step_b", now.Add(time.Minute))

	full, err := funnels.GetByID(context.Background(), f.ID)
	if err != nil {
		t.Fatal(err)
	}
	end := now.Add(24 * time.Hour)
	stats, err := funnels.ComputeFunnelStats(context.Background(), full, now.Add(-time.Hour), end)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(stats))
	}
	if stats[0].UserCount != 1 || stats[1].UserCount != 1 {
		t.Fatalf("expected 1 user per step, got %+v", stats)
	}
	if stats[1].ConvFromPrev < 99.9 {
		t.Fatalf("expected ~100%% step conversion, got %f", stats[1].ConvFromPrev)
	}
}
