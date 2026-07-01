package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/db"
	"github.com/eventhub/eventhub/internal/handler"
	"github.com/eventhub/eventhub/internal/logger"
	"github.com/eventhub/eventhub/internal/service"
	"github.com/eventhub/eventhub/internal/store"
)

func main() {
	defer logger.RecoverAndLog()
	logger.Init("log", logger.INFO)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("config: %v", err)
	}

	database, err := db.Open(cfg.DSN())
	if err != nil {
		logger.Fatal("db open: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		logger.Fatal("migrate: %v", err)
	}

	seedDemoProject(database)

	projects := store.NewProjectStore(database)
	issues := store.NewIssueStore(database)
	ingestStore := store.NewIngestStore(database)
	trackStore := store.NewTrackStore(database)
	analyticsStore := store.NewAnalyticsStore(database)
	funnelStore := store.NewFunnelStore(database)
	feedbackStore := store.NewFeedbackStore(database)

	ingestSvc := service.NewIngestService(cfg, projects, ingestStore)
	trackSvc := service.NewTrackIngestService(cfg, trackStore, ingestSvc)
	feedbackSvc := service.NewFeedbackIngestService(cfg, feedbackStore, ingestSvc)
	reportH := handler.NewReportHandler(cfg, ingestSvc)
	trackH := handler.NewTrackHandler(cfg, trackSvc, ingestSvc)
	feedbackH := handler.NewFeedbackHandler(cfg, feedbackSvc, ingestSvc)
	adminH, err := handler.NewAdminHandler(cfg, issues, projects, analyticsStore, funnelStore, feedbackStore)
	if err != nil {
		logger.Fatal("admin handler: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(recoverMiddleware)
	r.Use(middleware.Logger)

	r.Route("/reporting", func(r chi.Router) {
		r.Post("/v1/events/batch", reportH.Batch)
		r.Post("/v1/track/batch", trackH.Batch)
		r.Post("/v1/feedback", feedbackH.Submit)

		r.Route("/admin", func(r chi.Router) {
			r.Handle("/static/*", http.StripPrefix("/reporting/admin/static/", adminH.Static()))
			r.Get("/login", adminH.LoginPage)
			r.Post("/login", adminH.Login)
			r.Get("/logout", adminH.Logout)

			r.Group(func(r chi.Router) {
				r.Use(adminH.RequireAuth)
				r.Get("/", func(w http.ResponseWriter, req *http.Request) {
					http.Redirect(w, req, "/reporting/admin/issues", http.StatusSeeOther)
				})
				r.Get("/issues", adminH.IssueList)
				r.Get("/issues/{id}", adminH.IssueDetail)
				r.Post("/issues/{id}/status", adminH.UpdateStatus)
				r.Get("/feedback", adminH.FeedbackList)
				r.Get("/feedback/{id}", adminH.FeedbackDetail)
				r.Post("/feedback/{id}/status", adminH.UpdateFeedbackStatus)
				r.Get("/password", adminH.ChangePasswordPage)
				r.Post("/password", adminH.ChangePassword)
				r.Get("/projects", adminH.ProjectList)
				r.Post("/projects", adminH.CreateProject)
				r.Get("/projects/{id}", adminH.ProjectEditPage)
				r.Post("/projects/{id}", adminH.UpdateProject)
				r.Get("/test", adminH.TestPage)
				r.Get("/test/errors", adminH.TestErrorsPage)
				r.Get("/test/track", adminH.TestTrackPage)
				r.Get("/test/feedback", adminH.TestFeedbackPage)
				r.Post("/test/sign-token", adminH.SignTestToken)
				r.Get("/analytics", adminH.AnalyticsOverview)
				r.Get("/analytics/retention", adminH.AnalyticsRetention)
				r.Get("/funnels", adminH.FunnelList)
				r.Get("/funnels/new", adminH.FunnelCreatePage)
				r.Post("/funnels", adminH.CreateFunnel)
				r.Get("/funnels/{id}", adminH.FunnelDetail)
				r.Get("/funnels/{id}/edit", adminH.FunnelEditPage)
				r.Post("/funnels/{id}", adminH.UpdateFunnel)
				r.Post("/funnels/{id}/steps", adminH.AddFunnelStep)
				r.Post("/funnels/{id}/steps/{stepId}/delete", adminH.DeleteFunnelStep)
			})
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("EventHub listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logger.Close()
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.WriteCrashLog("panic in HTTP handler", fmt.Sprint(rec))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func seedDemoProject(dbconn *sql.DB) {
	var count int
	_ = dbconn.QueryRow(`SELECT COUNT(*) FROM report_project`).Scan(&count)
	if count > 0 {
		return
	}
	secret := os.Getenv("DEMO_PROJECT_SECRET")
	if secret == "" {
		secret = "change-me-demo-secret"
	}
	_, err := dbconn.Exec(`
		INSERT INTO report_project (project_key, project_name, status, trusted_token_secret)
		VALUES ('demo', 'demo', 'active', ?)`, secret)
	if err != nil {
		logger.Warn("seed demo project: %v", err)
	}
}
