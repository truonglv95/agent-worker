package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"ai-agent-backend/internal/config"
	"ai-agent-backend/internal/database"
	"ai-agent-backend/internal/handler"
	"ai-agent-backend/internal/repository"
	"ai-agent-backend/internal/sse"
	"ai-agent-backend/internal/workflow"
)

func main() {
	// ── Configuration ──────────────────────────────────────────────────────
	cfg := config.Load()

	if cfg.DatabaseURL == "" {
		log.Fatal("[main] DATABASE_URL environment variable is required")
	}

	// ── Database ────────────────────────────────────────────────────────────
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("[main] database connection failed: %v", err)
	}

	if err := database.RunMigrations(database.DB); err != nil {
		log.Fatalf("[main] migrations failed: %v", err)
	}

	// ── Infrastructure ──────────────────────────────────────────────────────
	sseBus := sse.NewSSEBus()
	orchestrator := workflow.NewOrchestrator(database.DB, sseBus, cfg)

	wfRepo := repository.NewWorkflowRepo(database.DB)
	debateRepo := repository.NewDebateRepo(database.DB)
	taskRepo := repository.NewTaskRepo(database.DB)

	// ── Handlers ────────────────────────────────────────────────────────────
	wfHandler := handler.NewWorkflowHandler(wfRepo, debateRepo, taskRepo, orchestrator, cfg.DebateRounds, database.DB)
	sseHandler := handler.NewSSEHandler(sseBus)
	agentHandler := handler.NewAgentHandler(database.DB)

	// ── Router ──────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// CORS — allow all origins for development; tighten in production.
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	})
	r.Use(corsMiddleware.Handler)

	// API routes.
	r.Route("/api", func(r chi.Router) {
		r.Get("/workflows", wfHandler.ListWorkflows)
		r.Post("/workflows", wfHandler.CreateWorkflow)
		r.Get("/workflows/{id}", wfHandler.GetWorkflow)
		r.Delete("/workflows/{id}", wfHandler.DeleteWorkflow)
		r.Post("/workflows/{id}/sync", wfHandler.SyncWorkflow)
		r.Post("/workflows/{id}/followup", wfHandler.FollowupWorkflow)
		r.Get("/workflows/{id}/debates", wfHandler.GetDebates)
		r.Get("/workflows/{id}/tasks", wfHandler.GetTasks)
		r.Get("/workflows/{id}/syslogs", wfHandler.GetSysLogs)
		r.Post("/workflows/{id}/resume", wfHandler.ResumeWorkflow)
		r.Get("/workflows/{id}/agent-rules", wfHandler.GetAgentRules)
		r.Post("/workflows/{id}/agent-rules", wfHandler.UpsertAgentRule)
		r.Delete("/workflows/{id}/agent-rules/{tag}", wfHandler.DeleteAgentRule)

		// Agent CRUD
		r.Get("/agents", agentHandler.ListAgents)
		r.Post("/agents", agentHandler.CreateAgent)
		r.Put("/agents/{id}", agentHandler.UpdateAgent)
		r.Delete("/agents/{id}", agentHandler.DeleteAgent)
	})

	// SSE endpoint.
	r.Get("/sse/workflows/{id}", sseHandler.Stream)

	// Health check.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	addr := ":" + cfg.Port
	log.Printf("[main] listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("[main] server error: %v", err)
	}
}
