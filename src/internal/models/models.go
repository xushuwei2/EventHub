package models

import "time"

type Project struct {
	ID                     int64
	ProjectKey             string
	ProjectName            string
	Status                 string
	TrustedTokenSecret string
}

type Issue struct {
	ID                  int64
	ProjectID           int64
	ProjectKey          string
	ProjectName         string
	GroupFingerprint    string
	Category            string
	Severity            string
	Title               string
	NormalizedMessage   string
	NormalizedStackTop  string
	Status              string
	FirstSeenAt         time.Time
	LastSeenAt          time.Time
	TotalCount          int64
	Last24hCount        int64
	LastRelease         string
	LastLanguage        string
	LastPlatform        string
	SampleEventID       string
}

type Event struct {
	ID                 int64
	EventID            string
	ProjectID          int64
	IssueID            int64
	ReleaseFingerprint string
	OccurredAt         time.Time
	Release            string
	Env                string
	Category           string
	Severity           string
	Message            string
	Stack              string
	Route              string
	Scene              string
	Module             string
	Language           string
	Runtime            string
	UserID             string
	RoomID             string
	SessionID          string
	DevicePlatform     string
	DeviceModel        string
	OSVersion          string
	SDKVersion         string
	NetworkType        string
	APIMethod          string
	APIPath            string
	HTTPStatus         int
	WSPhase            string
	WSCode             int
	WSReason           string
	AssetType          string
	AssetPath          string
	AssetURL           string
	BizCode            string
	ExtraJSON          string
}

type IngestEvent struct {
	EventID        string                 `json:"eventId"`
	OccurredAt     time.Time              `json:"occurredAt"`
	Release        string                 `json:"release"`
	Env            string                 `json:"env"`
	Category       string                 `json:"category"`
	Severity       string                 `json:"severity"`
	Message        string                 `json:"message"`
	Route          string                 `json:"route"`
	Scene          string                 `json:"scene"`
	Module         string                 `json:"module"`
	Stack          string                 `json:"stack"`
	File           string                 `json:"file"`
	Line           int                    `json:"line"`
	Column         int                    `json:"column"`
	Language       string                 `json:"language"`
	Runtime        string                 `json:"runtime"`
	DevicePlatform string                 `json:"devicePlatform"`
	DeviceModel    string                 `json:"deviceModel"`
	OSVersion      string                 `json:"osVersion"`
	SDKVersion     string                 `json:"sdkVersion"`
	NetworkType    string                 `json:"networkType"`
	UserID         string                 `json:"userId"`
	RoomID         *string                `json:"roomId"`
	SessionID      string                 `json:"sessionId"`
	APIMethod      string                 `json:"apiMethod"`
	APIPath        string                 `json:"apiPath"`
	HTTPStatus     int                    `json:"httpStatus"`
	WSPhase        string                 `json:"wsPhase"`
	WSCode         int                    `json:"wsCode"`
	WSReason       string                 `json:"wsReason"`
	AssetType      string                 `json:"assetType"`
	AssetPath      string                 `json:"assetPath"`
	AssetURL       string                 `json:"assetUrl"`
	BizCode        string                 `json:"bizCode"`
	Extra          map[string]interface{} `json:"extra"`
}

type BatchRequest struct {
	ClientSentAt time.Time     `json:"clientSentAt"`
	Events       []IngestEvent `json:"events"`
}

type BatchResponse struct {
	RequestID string `json:"requestId"`
	Accepted  int    `json:"accepted"`
	Rejected  int    `json:"rejected"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type TrustedIdentity struct {
	ProjectKey string
	UserID     string
	SessionID  string
	RoomID     string
	Release    string
}

const (
	IssueStatusOpen     = "open"
	IssueStatusResolved = "resolved"
	IssueStatusIgnored  = "ignored"

	ProjectStatusActive   = "active"
	ProjectStatusDisabled = "disabled"
)

var ValidCategories = map[string]bool{
	"uncaught_js":        true,
	"unhandled_promise":  true,
	"api_failure":        true,
	"ws_failure":         true,
	"asset_failure":      true,
	"biz_error":          true,
}

var ValidSeverities = map[string]bool{
	"fatal": true,
	"error": true,
	"warn":  true,
}

var ValidEnvs = map[string]bool{
	"prod":    true,
	"staging": true,
	"dev":     true,
}
