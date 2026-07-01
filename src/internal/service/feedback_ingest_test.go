package service

import (
	"testing"
	"time"

	"github.com/eventhub/eventhub/internal/models"
)

func TestValidateFeedback(t *testing.T) {
	base := models.IngestFeedbackRequest{
		FeedbackID: "fb-001",
		Release:    "1.0.0",
		Env:        "prod",
		Content:    "支付按钮无反应",
	}

	if err := validateFeedback(base, 2000); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	empty := base
	empty.Content = "   "
	if err := validateFeedback(empty, 2000); err != ErrInvalidEvent {
		t.Fatalf("expected invalid for empty content, got %v", err)
	}

	badCat := base
	badCat.Category = "complaint"
	if err := validateFeedback(badCat, 2000); err != ErrInvalidEvent {
		t.Fatalf("expected invalid category, got %v", err)
	}

	long := base
	long.Content = string(make([]rune, 2001))
	if err := validateFeedback(long, 2000); err != ErrInvalidEvent {
		t.Fatalf("expected content too long, got %v", err)
	}

	withTime := base
	withTime.SubmittedAt = time.Now()
	if err := validateFeedback(withTime, 2000); err != nil {
		t.Fatalf("expected valid with submittedAt: %v", err)
	}
}
