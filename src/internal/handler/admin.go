package handler

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	version "github.com/eventhub/eventhub"
	"github.com/eventhub/eventhub/internal/auth"
	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/service"
	"github.com/eventhub/eventhub/internal/store"
)

//go:embed web/*
var webFS embed.FS

const adminCookie = "eventhub_admin_session"

var projectKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type AdminHandler struct {
	cfg       *config.Config
	projects  *store.ProjectStore
	analytics *store.AnalyticsStore
	funnels   *store.FunnelStore
	feedback  *store.FeedbackStore
	loginRL   *service.RateLimiter
	tmpl      *template.Template
}

func NewAdminHandler(cfg *config.Config, projects *store.ProjectStore, analytics *store.AnalyticsStore, funnels *store.FunnelStore, feedback *store.FeedbackStore) (*AdminHandler, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	tmpl, err := template.ParseFS(sub, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	return &AdminHandler{
		cfg:       cfg,
		projects:  projects,
		analytics: analytics,
		funnels:   funnels,
		feedback:  feedback,
		loginRL:   service.NewRateLimiter(5, 15*time.Minute),
		tmpl:      tmpl,
	}, nil
}

func (h *AdminHandler) Static() http.Handler {
	sub, _ := fs.Sub(webFS, "web/static")
	return http.FileServer(http.FS(sub))
}

func (h *AdminHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if h.currentUser(r) != nil {
		http.Redirect(w, r, "/reporting/admin/projects", http.StatusSeeOther)
		return
	}
	h.render(w, "login.html", map[string]interface{}{"Error": ""})
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, "login.html", map[string]interface{}{"Error": "请求无效"})
		return
	}
	ip := clientIP(r)
	if !h.loginRL.Allow("login:" + ip) {
		h.render(w, "login.html", map[string]interface{}{"Error": "登录尝试过多，请稍后再试"})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	if username != h.cfg.AdminUsername || !auth.VerifyAdminPassword(h.cfg.AdminPasswordHash, password) {
		h.render(w, "login.html", map[string]interface{}{"Error": "用户名或密码错误"})
		return
	}

	token, err := auth.SignAdminSession(h.cfg.AdminSessionSecret, username, 24*time.Hour)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookie,
		Value:    token,
		Path:     "/reporting/admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	http.Redirect(w, r, "/reporting/admin/projects", http.StatusSeeOther)
}

func (h *AdminHandler) ChangePasswordPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "change_password.html", map[string]interface{}{
		"Error":   "",
		"Success": "",
	})
}

func (h *AdminHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, "change_password.html", map[string]interface{}{"Error": "请求无效"})
		return
	}

	ip := clientIP(r)
	if !h.loginRL.Allow("password:" + ip) {
		h.render(w, "change_password.html", map[string]interface{}{"Error": "尝试次数过多，请稍后再试"})
		return
	}

	current := r.FormValue("current_password")
	newPass := r.FormValue("new_password")
	confirm := r.FormValue("confirm_password")

	if !auth.VerifyAdminPassword(h.cfg.AdminPasswordHash, current) {
		h.render(w, "change_password.html", map[string]interface{}{"Error": "当前密码错误"})
		return
	}
	if len(newPass) < 8 {
		h.render(w, "change_password.html", map[string]interface{}{"Error": "新密码至少 8 位"})
		return
	}
	if newPass != confirm {
		h.render(w, "change_password.html", map[string]interface{}{"Error": "两次输入的新密码不一致"})
		return
	}
	if auth.VerifyAdminPassword(h.cfg.AdminPasswordHash, newPass) {
		h.render(w, "change_password.html", map[string]interface{}{"Error": "新密码不能与当前密码相同"})
		return
	}

	hash, err := auth.HashAdminPassword(newPass)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	envPath := config.EnvFilePath()
	if envPath == "" {
		h.render(w, "change_password.html", map[string]interface{}{"Error": "未找到配置文件，无法保存新密码"})
		return
	}
	if err := config.UpdateEnvFileKey(envPath, "ADMIN_PASSWORD_HASH", hash); err != nil {
		h.render(w, "change_password.html", map[string]interface{}{"Error": "保存密码失败，请检查配置文件权限"})
		return
	}
	h.cfg.AdminPasswordHash = hash

	h.render(w, "change_password.html", map[string]interface{}{"Success": "密码已修改"})
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookie,
		Value:    "",
		Path:     "/reporting/admin",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/reporting/admin/login", http.StatusSeeOther)
}

func (h *AdminHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.currentUser(r) == nil {
			http.Redirect(w, r, "/reporting/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *AdminHandler) currentUser(r *http.Request) *auth.AdminSession {
	c, err := r.Cookie(adminCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	sess, err := auth.ParseAdminSession(h.cfg.AdminSessionSecret, c.Value)
	if err != nil {
		return nil
	}
	return sess
}

func (h *AdminHandler) ProjectList(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projects.ListAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "projects.html", map[string]interface{}{
		"Projects": projects,
		"Error":    "",
		"Success":  r.URL.Query().Get("success"),
	})
}

func (h *AdminHandler) ProjectEditPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	project, err := h.projects.GetByID(r.Context(), id)
	if errors.Is(err, store.ErrProjectNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "project_edit.html", map[string]interface{}{
		"Project": project,
		"Error":   "",
	})
}

func (h *AdminHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderProjectListError(w, r, "请求无效")
		return
	}
	p, errMsg := h.parseProjectForm(r, 0)
	if errMsg != "" {
		h.renderProjectListError(w, r, errMsg)
		return
	}
	if err := h.projects.Create(r.Context(), p); err != nil {
		if errors.Is(err, store.ErrProjectKeyExists) {
			h.renderProjectListError(w, r, "项目 Key 已存在")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reporting/admin/projects?success="+url.QueryEscape("项目已创建"), http.StatusSeeOther)
}

func (h *AdminHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderProjectEditError(w, r, id, "请求无效")
		return
	}
	p, errMsg := h.parseProjectForm(r, id)
	if errMsg != "" {
		h.renderProjectEditError(w, r, id, errMsg)
		return
	}
	if err := h.projects.Update(r.Context(), p); err != nil {
		if errors.Is(err, store.ErrProjectNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reporting/admin/projects?success="+url.QueryEscape("项目已保存"), http.StatusSeeOther)
}

func (h *AdminHandler) parseProjectForm(r *http.Request, id int64) (*models.Project, string) {
	key := r.FormValue("project_key")
	secret := r.FormValue("trusted_token_secret")
	status := r.FormValue("status")

	if id == 0 {
		if key == "" {
			return nil, "项目 Key 不能为空"
		}
		if !projectKeyPattern.MatchString(key) {
			return nil, "项目 Key 仅支持小写字母、数字、下划线和连字符，且须以小写字母开头"
		}
	} else {
		existing, err := h.projects.GetByID(r.Context(), id)
		if err != nil {
			return nil, "项目不存在"
		}
		key = existing.ProjectKey
	}

	if secret == "" {
		if id == 0 {
			secret = generateTokenSecret()
		} else {
			return nil, "Token 密钥不能为空"
		}
	}
	if len(secret) < 8 {
		return nil, "Token 密钥至少 8 位"
	}
	if len(secret) > 256 {
		return nil, "Token 密钥过长"
	}
	if status != models.ProjectStatusActive && status != models.ProjectStatusDisabled {
		status = models.ProjectStatusActive
	}

	return &models.Project{
		ID:                 id,
		ProjectKey:         key,
		ProjectName:        key,
		Status:             status,
		TrustedTokenSecret: secret,
	}, ""
}

func (h *AdminHandler) renderProjectListError(w http.ResponseWriter, r *http.Request, msg string) {
	projects, _ := h.projects.ListAll(r.Context())
	h.render(w, "projects.html", map[string]interface{}{
		"Projects": projects,
		"Error":    msg,
		"Success":  "",
	})
}

func (h *AdminHandler) renderProjectEditError(w http.ResponseWriter, r *http.Request, id int64, msg string) {
	project, _ := h.projects.GetByID(r.Context(), id)
	h.render(w, "project_edit.html", map[string]interface{}{
		"Project": project,
		"Error":   msg,
	})
}

func generateTokenSecret() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *AdminHandler) TestPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/reporting/admin/test/errors", http.StatusSeeOther)
}

func (h *AdminHandler) TestErrorsPage(w http.ResponseWriter, r *http.Request) {
	h.renderTestPage(w, r, "test_errors.html")
}

func (h *AdminHandler) TestTrackPage(w http.ResponseWriter, r *http.Request) {
	h.renderTestPage(w, r, "test_track.html")
}

func (h *AdminHandler) TestFeedbackPage(w http.ResponseWriter, r *http.Request) {
	h.renderTestPage(w, r, "test_feedback.html")
}

func (h *AdminHandler) renderTestPage(w http.ResponseWriter, r *http.Request, tmpl string) {
	projects, err := h.projects.ListAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, tmpl, map[string]interface{}{
		"Projects": projects,
	})
}

type signTestTokenRequest struct {
	ProjectID int64  `json:"projectId"`
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId"`
	RoomID    string `json:"roomId"`
	Release   string `json:"release"`
}

func (h *AdminHandler) SignTestToken(w http.ResponseWriter, r *http.Request) {
	var req signTestTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid_request"})
		return
	}
	if req.ProjectID <= 0 {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid_project"})
		return
	}
	project, err := h.projects.GetByID(r.Context(), req.ProjectID)
	if errors.Is(err, store.ErrProjectNotFound) {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid_project"})
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project.Status != models.ProjectStatusActive {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid_project"})
		return
	}

	token, err := auth.SignReportToken(project.TrustedTokenSecret, &models.TrustedIdentity{
		ProjectKey: project.ProjectKey,
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		RoomID:     req.RoomID,
		Release:    req.Release,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"reportToken": token})
}

var templateNavActive = map[string]string{
	"feedback_list.html":       "feedback",
	"feedback_detail.html":     "feedback",
	"analytics_overview.html":  "analytics",
	"analytics_retention.html": "retention",
	"funnels.html":             "funnels",
	"funnel_detail.html":       "funnels",
	"funnel_edit.html":         "funnels",
	"projects.html":            "projects",
	"project_edit.html":        "projects",
	"test_errors.html":         "test-errors",
	"test_track.html":          "test-track",
	"test_feedback.html":       "test-feedback",
	"change_password.html":     "password",
}

func (h *AdminHandler) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Version"] = version.Version
	if active, ok := templateNavActive[name]; ok {
		data["NavActive"] = active
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
