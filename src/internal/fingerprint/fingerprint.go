package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/sanitize"
)

type Result struct {
	GroupFingerprint   string
	ReleaseFingerprint string
	NormalizedMessage  string
	NormalizedStackTop string
	Title              string
}

func Compute(ev models.IngestEvent) Result {
	normMsg := sanitize.NormalizeMessage(ev.Message)
	stackTop := sanitize.StackTop(ev.Stack)

	var parts []string
	switch ev.Category {
	case "uncaught_js", "unhandled_promise":
		parts = []string{ev.Category, normMsg, stackTop, ev.Module}
	case "api_failure":
		parts = []string{
			ev.Category,
			strings.ToUpper(ev.APIMethod),
			sanitize.NormalizePath(ev.APIPath),
			fmt.Sprintf("%d", ev.HTTPStatus),
			normMsg,
		}
	case "ws_failure":
		parts = []string{
			ev.Category,
			ev.WSPhase,
			fmt.Sprintf("%d", ev.WSCode),
			sanitize.NormalizeMessage(ev.WSReason),
		}
	case "asset_failure":
		key := ev.AssetPath
		if key == "" {
			key = ev.AssetURL
		}
		parts = []string{ev.Category, ev.AssetType, sanitize.NormalizePath(key)}
	case "biz_error":
		parts = []string{ev.Category, ev.BizCode}
	default:
		parts = []string{ev.Category, normMsg}
	}

	group := hash(strings.Join(parts, "|"))
	release := hash(group + "|" + ev.Release)

	title := normMsg
	if len(title) > 200 {
		title = title[:200]
	}
	if title == "" {
		title = ev.Category
	}

	return Result{
		GroupFingerprint:   group,
		ReleaseFingerprint: release,
		NormalizedMessage:  normMsg,
		NormalizedStackTop: stackTop,
		Title:              title,
	}
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
