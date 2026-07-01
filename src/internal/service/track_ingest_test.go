package service

import (
	"testing"
	"time"

	"github.com/eventhub/eventhub/internal/models"
)

func TestValidateTrackEvent(t *testing.T) {
	valid := models.IngestTrackEvent{
		EventID:   "e1",
		Release:   "1.0.0",
		Env:       "dev",
		EventName: "page_view",
	}
	if err := validateTrackEvent(valid); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	missingName := valid
	missingName.EventName = ""
	if err := validateTrackEvent(missingName); err == nil {
		t.Fatal("expected error for missing eventName")
	}

	funnelOnly := valid
	funnelOnly.FunnelKey = "register_flow"
	if err := validateTrackEvent(funnelOnly); err == nil {
		t.Fatal("expected error when funnelKey without stepKey")
	}

	withFunnel := valid
	withFunnel.FunnelKey = "register_flow"
	withFunnel.StepKey = "home_view"
	if err := validateTrackEvent(withFunnel); err != nil {
		t.Fatalf("expected valid funnel pair: %v", err)
	}

	badEnv := valid
	badEnv.Env = "production"
	if err := validateTrackEvent(badEnv); err == nil {
		t.Fatal("expected error for invalid env")
	}
}

func TestValidateTrackEventOccurredAtOptional(t *testing.T) {
	ev := models.IngestTrackEvent{
		EventID:    "e2",
		OccurredAt: time.Time{},
		Release:    "1.0.0",
		Env:        "prod",
		EventName:  "app_launch",
	}
	if err := validateTrackEvent(ev); err != nil {
		t.Fatalf("zero occurredAt should pass validation: %v", err)
	}
}
