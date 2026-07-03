package models

import "time"

type Project struct {
	ID                     int64
	ProjectKey             string
	ProjectName            string
	Status                 string
	TrustedTokenSecret string
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

type TrackEvent struct {
	ID             int64
	EventID        string
	ProjectID      int64
	OccurredAt     time.Time
	EventName      string
	Release        string
	Env            string
	Route          string
	Scene          string
	Module         string
	Language       string
	Runtime        string
	UserID         string
	RoomID         string
	SessionID      string
	DevicePlatform string
	DeviceModel    string
	OSVersion      string
	SDKVersion     string
	NetworkType    string
	FunnelKey      string
	StepKey        string
	ExtraJSON      string
}

type IngestTrackEvent struct {
	EventID        string                 `json:"eventId"`
	OccurredAt     time.Time              `json:"occurredAt"`
	Release        string                 `json:"release"`
	Env            string                 `json:"env"`
	EventName      string                 `json:"eventName"`
	Route          string                 `json:"route"`
	Scene          string                 `json:"scene"`
	Module         string                 `json:"module"`
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
	FunnelKey      string                 `json:"funnelKey"`
	StepKey        string                 `json:"stepKey"`
	Extra          map[string]interface{} `json:"extra"`
}

type TrackBatchRequest struct {
	ClientSentAt time.Time          `json:"clientSentAt"`
	Events       []IngestTrackEvent `json:"events"`
}

type FunnelDefinition struct {
	ID           int64
	ProjectID    int64
	ProjectKey   string
	ProjectName  string
	FunnelKey    string
	FunnelName   string
	WindowHours  int
	Status       string
	Steps        []FunnelStep
	OverallRate  float64
	Step1Users   int64
}

type FunnelStep struct {
	ID        int64
	FunnelID  int64
	StepKey   string
	StepName  string
	StepOrder int
}

type FunnelStepStat struct {
	StepOrder    int
	StepKey      string
	StepName     string
	UserCount    int64
	ConvFromPrev float64
	ConvFromFirst float64
}

type DailyStatRow struct {
	StatDate   string
	DAU        int64
	NewUsers   int64
	EventCount int64
}

type RetentionRow struct {
	CohortDate string
	CohortSize int64
	D1         float64
	D7         float64
	D14        float64
	D30        float64
}

const (
	FunnelStatusActive   = "active"
	FunnelStatusDisabled = "disabled"
)

type UserFeedback struct {
	ID             int64
	FeedbackID     string
	ProjectID      int64
	ProjectKey     string
	ProjectName    string
	SubmittedAt    time.Time
	Release        string
	Env            string
	Category       string
	Content        string
	Contact        string
	Route          string
	Scene          string
	Language       string
	Runtime        string
	UserID         string
	RoomID         string
	SessionID      string
	DevicePlatform string
	DeviceModel    string
	OSVersion      string
	SDKVersion     string
	NetworkType    string
	ExtraJSON      string
	Status         string
	AdminNote      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type IngestFeedbackRequest struct {
	FeedbackID     string                 `json:"feedbackId"`
	SubmittedAt    time.Time              `json:"submittedAt"`
	Release        string                 `json:"release"`
	Env            string                 `json:"env"`
	Category       string                 `json:"category"`
	Content        string                 `json:"content"`
	Contact        string                 `json:"contact"`
	Route          string                 `json:"route"`
	Scene          string                 `json:"scene"`
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
	Extra          map[string]interface{} `json:"extra"`
}

type FeedbackResponse struct {
	RequestID  string `json:"requestId"`
	FeedbackID string `json:"feedbackId"`
}

const (
	FeedbackStatusOpen       = "open"
	FeedbackStatusProcessing = "processing"
	FeedbackStatusResolved   = "resolved"
	FeedbackStatusClosed     = "closed"
)

var ValidFeedbackCategories = map[string]bool{
	"bug":        true,
	"suggestion": true,
	"question":   true,
	"other":      true,
}

var ValidFeedbackStatuses = map[string]bool{
	FeedbackStatusOpen:       true,
	FeedbackStatusProcessing: true,
	FeedbackStatusResolved:   true,
	FeedbackStatusClosed:     true,
}
