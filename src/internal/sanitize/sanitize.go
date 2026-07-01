package sanitize

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	uuidRe      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	tokenRe     = regexp.MustCompile(`(?i)(bearer\s+)[^\s]+`)
	authHeader  = regexp.MustCompile(`(?i)authorization["']?\s*[:=]\s*["']?[^"'\s,}]+`)
	sessionRe   = regexp.MustCompile(`(?i)sessiontoken["']?\s*[:=]\s*["']?[^"'\s,}]+`)
	queryToken  = regexp.MustCompile(`(?i)([?&](?:token|code|access_token|session)=)[^&\s]+`)
	timestampRe = regexp.MustCompile(`\b\d{10,13}\b`)
	longNumRe   = regexp.MustCompile(`\b\d{4,}\b`)
)

func Text(s string) string {
	s = tokenRe.ReplaceAllString(s, "${1}[REDACTED]")
	s = authHeader.ReplaceAllString(s, "authorization=[REDACTED]")
	s = sessionRe.ReplaceAllString(s, "sessionToken=[REDACTED]")
	s = queryToken.ReplaceAllString(s, "${1}[REDACTED]")
	return s
}

func NormalizeMessage(msg string) string {
	msg = Text(msg)
	msg = uuidRe.ReplaceAllString(msg, "{uuid}")
	msg = timestampRe.ReplaceAllString(msg, "{ts}")
	msg = longNumRe.ReplaceAllString(msg, "{id}")
	return strings.TrimSpace(msg)
}

func NormalizePath(path string) string {
	path = Text(path)
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "" {
			continue
		}
		if uuidRe.MatchString(p) {
			parts[i] = "{id}"
			continue
		}
		if matched, _ := regexp.MatchString(`^\d+$`, p); matched {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

func StackTop(stack string) string {
	stack = Text(stack)
	lines := strings.Split(stack, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "at ") || strings.Contains(line, ".js:") || strings.Contains(line, ".ts:") {
			return NormalizeMessage(line)
		}
	}
	if len(lines) > 0 {
		return NormalizeMessage(strings.TrimSpace(lines[0]))
	}
	return ""
}

func ExtraJSON(extra map[string]interface{}) string {
	if len(extra) == 0 {
		return ""
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return ""
	}
	return Text(string(raw))
}

func OrUnknown(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	return v
}

func PtrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
