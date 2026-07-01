package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/store"
)

func (h *AdminHandler) FeedbackList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.FeedbackFilter{
		ProjectKey: q.Get("project"),
		Status:     q.Get("status"),
		Category:   q.Get("category"),
		UserID:     q.Get("userId"),
		Limit:      100,
	}

	list, err := h.feedback.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projects, _ := h.projects.ListActive(r.Context())

	h.render(w, "feedback_list.html", map[string]interface{}{
		"Feedbacks": list,
		"Projects":  projects,
		"Filter":    filter,
	})
}

func (h *AdminHandler) FeedbackDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	fb, err := h.feedback.GetByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, "feedback_detail.html", map[string]interface{}{
		"Feedback": fb,
	})
}

func (h *AdminHandler) UpdateFeedbackStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	status := r.FormValue("status")
	if !models.ValidFeedbackStatuses[status] {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	adminNote := r.FormValue("admin_note")
	if err := h.feedback.UpdateStatus(r.Context(), id, status, adminNote); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/reporting/admin/feedback/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
