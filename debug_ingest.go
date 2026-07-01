package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/eventhub/eventhub/internal/auth"
	"github.com/eventhub/eventhub/internal/config"
	"github.com/eventhub/eventhub/internal/db"
	"github.com/eventhub/eventhub/internal/models"
	"github.com/eventhub/eventhub/internal/service"
	"github.com/eventhub/eventhub/internal/store"
)

func main() {
	cfg, _ := config.Load()
	database, _ := db.Open(cfg.DSN())
	defer database.Close()
	projects := store.NewProjectStore(database)
	ingestStore := store.NewIngestStore(database)
	svc := service.NewIngestService(cfg, projects, ingestStore)

	p, err := projects.GetByKey(context.Background(), "demo")
	fmt.Println("project", p, err)

	token, _ := auth.SignReportToken(p.TrustedTokenSecret, &models.TrustedIdentity{
		ProjectKey: "demo", UserID: "u1", Release: "0.1.0",
	})

	ia, err := svc.ResolveAuth(context.Background(), "Bearer "+token)
	fmt.Println("auth", ia, err)

	body := `{"clientSentAt":"2026-07-01T06:00:00.000Z","events":[{"eventId":"test-debug-001","occurredAt":"2026-07-01T05:59:58.000Z","release":"0.1.0","env":"dev","category":"uncaught_js","severity":"error","message":"test uncaught error"}]}`
	var req models.BatchRequest
	json.Unmarshal(body, &req)
	fmt.Printf("parsed: %+v occurredAt=%v\n", req.Events[0], req.Events[0].OccurredAt)

	resp, err := svc.ProcessBatch(context.Background(), ia, req)
	fmt.Println("resp", resp, "err", err)
}
