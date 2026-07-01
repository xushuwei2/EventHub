package fingerprint_test

import (
	"testing"

	"github.com/eventhub/eventhub/internal/fingerprint"
	"github.com/eventhub/eventhub/internal/models"
)

func TestAPIFailureSameFingerprintForDifferentIDs(t *testing.T) {
	a := models.IngestEvent{
		Category: "api_failure", Message: "HTTP 500",
		APIMethod: "POST", APIPath: "/api/orders/123/submit", HTTPStatus: 500,
		Release: "1.0.0",
	}
	b := a
	b.APIPath = "/api/orders/456/submit"

	fa := fingerprint.Compute(a)
	fb := fingerprint.Compute(b)
	if fa.GroupFingerprint != fb.GroupFingerprint {
		t.Fatalf("expected same group fingerprint, got %s vs %s", fa.GroupFingerprint, fb.GroupFingerprint)
	}
}

func TestBizErrorFingerprint(t *testing.T) {
	ev := models.IngestEvent{Category: "biz_error", BizCode: "room_result_retry_exhausted", Release: "2.0.0"}
	fp := fingerprint.Compute(ev)
	if fp.GroupFingerprint == "" {
		t.Fatal("empty fingerprint")
	}
}
