package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/store"
)

func (h *AdminHandler) AnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	projectKey := r.URL.Query().Get("project")
	projects, _ := h.projects.ListActive(r.Context())

	var projectID int64
	for _, p := range projects {
		if projectKey == "" {
			projectKey = p.ProjectKey
			projectID = p.ID
			break
		}
		if p.ProjectKey == projectKey {
			projectID = p.ID
			break
		}
	}

	data := map[string]interface{}{
		"Projects":   projects,
		"ProjectKey": projectKey,
		"Trend":      []models.DailyStatRow{},
		"MAU":        int64(0),
	}

	if projectID > 0 {
		trend, _ := h.analytics.DailyTrend(r.Context(), projectID, 30)
		mau, _ := h.analytics.MAU(r.Context(), projectID)
		data["Trend"] = trend
		data["MAU"] = mau
	}

	h.render(w, "analytics_overview.html", data)
}

func (h *AdminHandler) AnalyticsRetention(w http.ResponseWriter, r *http.Request) {
	projectKey := r.URL.Query().Get("project")
	projects, _ := h.projects.ListActive(r.Context())

	var projectID int64
	for _, p := range projects {
		if projectKey == "" {
			projectKey = p.ProjectKey
			projectID = p.ID
			break
		}
		if p.ProjectKey == projectKey {
			projectID = p.ID
			break
		}
	}

	rows := []models.RetentionRow{}
	if projectID > 0 {
		rows, _ = h.analytics.RetentionCohort(r.Context(), projectID, 14)
	}

	h.render(w, "analytics_retention.html", map[string]interface{}{
		"Projects":   projects,
		"ProjectKey": projectKey,
		"Rows":       rows,
	})
}

func (h *AdminHandler) FunnelList(w http.ResponseWriter, r *http.Request) {
	projectKey := r.URL.Query().Get("project")
	projects, _ := h.projects.ListActive(r.Context())

	var funnels []models.FunnelDefinition
	if projectKey != "" {
		for _, p := range projects {
			if p.ProjectKey == projectKey {
				funnels, _ = h.funnels.ListByProject(r.Context(), p.ID)
				break
			}
		}
	} else {
		funnels, _ = h.funnels.ListAll(r.Context())
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -7)
	for i := range funnels {
		full, err := h.funnels.GetByID(r.Context(), funnels[i].ID)
		if err != nil {
			continue
		}
		rate, step1 := h.funnels.OverallConversion(r.Context(), full, start, end)
		funnels[i].OverallRate = rate
		funnels[i].Step1Users = step1
		funnels[i].Steps = full.Steps
	}

	h.render(w, "funnels.html", map[string]interface{}{
		"Projects":   projects,
		"ProjectKey": projectKey,
		"Funnels":    funnels,
		"Error":      "",
		"Success":    r.URL.Query().Get("success"),
	})
}

func (h *AdminHandler) FunnelDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	funnel, err := h.funnels.GetByID(r.Context(), id)
	if errors.Is(err, store.ErrFunnelNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 90 {
			days = n
		}
	}
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)

	stats, _ := h.funnels.ComputeFunnelStats(r.Context(), funnel, start, end)

	nextStepOrder := len(funnel.Steps) + 1

	h.render(w, "funnel_detail.html", map[string]interface{}{
		"Funnel":        funnel,
		"Stats":         stats,
		"Days":          days,
		"NextStepOrder": nextStepOrder,
		"Error":         "",
	})
}

func (h *AdminHandler) FunnelCreatePage(w http.ResponseWriter, r *http.Request) {
	projects, _ := h.projects.ListActive(r.Context())
	h.render(w, "funnel_edit.html", map[string]interface{}{
		"Funnel":   nil,
		"Projects": projects,
		"Error":    "",
	})
}

func (h *AdminHandler) CreateFunnel(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderFunnelListError(w, r, "请求无效")
		return
	}
	f, errMsg := h.parseFunnelForm(r, 0)
	if errMsg != "" {
		h.renderFunnelListError(w, r, errMsg)
		return
	}
	if err := h.funnels.Create(r.Context(), f); err != nil {
		if err.Error() == "funnel_key_exists" {
			h.renderFunnelListError(w, r, "漏斗 Key 已存在")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reporting/admin/funnels/"+strconv.FormatInt(f.ID, 10), http.StatusSeeOther)
}

func (h *AdminHandler) FunnelEditPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	funnel, err := h.funnels.GetByID(r.Context(), id)
	if errors.Is(err, store.ErrFunnelNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projects, _ := h.projects.ListActive(r.Context())
	h.render(w, "funnel_edit.html", map[string]interface{}{
		"Funnel":   funnel,
		"Projects": projects,
		"Error":    "",
	})
}

func (h *AdminHandler) UpdateFunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderFunnelEditError(w, r, id, "请求无效")
		return
	}
	f, errMsg := h.parseFunnelForm(r, id)
	if errMsg != "" {
		h.renderFunnelEditError(w, r, id, errMsg)
		return
	}
	if err := h.funnels.Update(r.Context(), f); err != nil {
		if errors.Is(err, store.ErrFunnelNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reporting/admin/funnels/"+strconv.FormatInt(id, 10)+"?success="+url.QueryEscape("漏斗已保存"), http.StatusSeeOther)
}

func (h *AdminHandler) AddFunnelStep(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/reporting/admin/funnels/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}
	stepKey := r.FormValue("step_key")
	stepName := r.FormValue("step_name")
	stepOrder, _ := strconv.Atoi(r.FormValue("step_order"))
	if stepKey == "" || stepName == "" || stepOrder <= 0 {
		http.Redirect(w, r, "/reporting/admin/funnels/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}
	_ = h.funnels.AddStep(r.Context(), id, models.FunnelStep{
		StepKey:   stepKey,
		StepName:  stepName,
		StepOrder: stepOrder,
	})
	http.Redirect(w, r, "/reporting/admin/funnels/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (h *AdminHandler) DeleteFunnelStep(w http.ResponseWriter, r *http.Request) {
	funnelID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	stepID, err := strconv.ParseInt(chi.URLParam(r, "stepId"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.funnels.DeleteStep(r.Context(), stepID)
	http.Redirect(w, r, "/reporting/admin/funnels/"+strconv.FormatInt(funnelID, 10), http.StatusSeeOther)
}

func (h *AdminHandler) parseFunnelForm(r *http.Request, id int64) (*models.FunnelDefinition, string) {
	projectID, _ := strconv.ParseInt(r.FormValue("project_id"), 10, 64)
	funnelKey := r.FormValue("funnel_key")
	funnelName := r.FormValue("funnel_name")
	windowHours, _ := strconv.Atoi(r.FormValue("window_hours"))
	status := r.FormValue("status")

	if id == 0 {
		if projectID <= 0 {
			return nil, "请选择项目"
		}
		if funnelKey == "" {
			return nil, "漏斗 Key 不能为空"
		}
		if !projectKeyPattern.MatchString(funnelKey) {
			return nil, "漏斗 Key 格式与项目 Key 相同：小写字母开头，仅含 a-z、0-9、_、-"
		}
	} else {
		existing, err := h.funnels.GetByID(r.Context(), id)
		if err != nil {
			return nil, "漏斗不存在"
		}
		projectID = existing.ProjectID
		funnelKey = existing.FunnelKey
	}

	if funnelName == "" {
		return nil, "漏斗名称不能为空"
	}
	if windowHours <= 0 {
		windowHours = 24
	}
	if windowHours > 720 {
		return nil, "转化窗口不能超过 720 小时"
	}
	if status != models.FunnelStatusActive && status != models.FunnelStatusDisabled {
		status = models.FunnelStatusActive
	}

	return &models.FunnelDefinition{
		ID:          id,
		ProjectID:   projectID,
		FunnelKey:   funnelKey,
		FunnelName:  funnelName,
		WindowHours: windowHours,
		Status:      status,
	}, ""
}

func (h *AdminHandler) renderFunnelListError(w http.ResponseWriter, r *http.Request, msg string) {
	projects, _ := h.projects.ListActive(r.Context())
	funnels, _ := h.funnels.ListAll(r.Context())
	h.render(w, "funnels.html", map[string]interface{}{
		"Projects": projects,
		"Funnels":  funnels,
		"Error":    msg,
		"Success":  "",
	})
}

func (h *AdminHandler) renderFunnelEditError(w http.ResponseWriter, r *http.Request, id int64, msg string) {
	funnel, _ := h.funnels.GetByID(r.Context(), id)
	projects, _ := h.projects.ListActive(r.Context())
	h.render(w, "funnel_edit.html", map[string]interface{}{
		"Funnel":   funnel,
		"Projects": projects,
		"Error":    msg,
	})
}
