package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ai-agent-backend/internal/models"
	"ai-agent-backend/internal/repository"
	"ai-agent-backend/internal/workflow"
)

// WorkflowHandler groups all HTTP handlers related to workflows.
type WorkflowHandler struct {
	wfRepo       *repository.WorkflowRepo
	debateRepo   *repository.DebateRepo
	taskRepo     *repository.TaskRepo
	orchestrator *workflow.Orchestrator
	debateRounds int
	db           *sql.DB
}

// NewWorkflowHandler constructs a WorkflowHandler with its dependencies.
func NewWorkflowHandler(
	wfRepo *repository.WorkflowRepo,
	debateRepo *repository.DebateRepo,
	taskRepo *repository.TaskRepo,
	orchestrator *workflow.Orchestrator,
	debateRounds int,
	db *sql.DB,
) *WorkflowHandler {
	return &WorkflowHandler{
		wfRepo:       wfRepo,
		debateRepo:   debateRepo,
		taskRepo:     taskRepo,
		orchestrator: orchestrator,
		debateRounds: debateRounds,
		db:           db,
	}
}

// ListWorkflows handles GET /api/workflows.
func (h *WorkflowHandler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	wfs, err := h.wfRepo.ListWorkflows()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workflows")
		return
	}
	// Return empty array instead of null.
	if wfs == nil {
		wfs = []*models.Workflow{}
	}
	writeJSON(w, http.StatusOK, wfs)
}

// CreateWorkflow handles POST /api/workflows.
// Body: { "request": "..." }
func (h *WorkflowHandler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Request string `json:"request"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Request == "" {
		writeError(w, http.StatusBadRequest, "request field is required")
		return
	}

	id, err := h.wfRepo.CreateWorkflow(body.Request, h.debateRounds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workflow")
		return
	}

	wf, err := h.wfRepo.GetWorkflow(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch new workflow")
		return
	}

	// Kick off the debate loop in the background.
	go h.orchestrator.Run(wf.ID, wf.Request)

	writeJSON(w, http.StatusCreated, wf)
}

// GetWorkflow handles GET /api/workflows/{id}.
func (h *WorkflowHandler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	wf, err := h.wfRepo.GetWorkflow(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

// GetDebates handles GET /api/workflows/{id}/debates.
func (h *WorkflowHandler) GetDebates(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	debates, err := h.debateRepo.GetDebates(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch debates")
		return
	}
	if debates == nil {
		debates = []*models.DebateMessage{}
	}
	writeJSON(w, http.StatusOK, debates)
}

// GetTasks handles GET /api/workflows/{id}/tasks.
func (h *WorkflowHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	tasks, err := h.taskRepo.GetTasks(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch tasks")
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

// DeleteWorkflow handles DELETE /api/workflows/{id}.
func (h *WorkflowHandler) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	if err := h.wfRepo.DeleteWorkflow(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete workflow")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SyncWorkflow handles POST /api/workflows/{id}/sync.
func (h *WorkflowHandler) SyncWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	links, err := h.orchestrator.SyncWorkflow(id, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"links": links})
}

// FollowupWorkflow handles POST /api/workflows/{id}/followup.
func (h *WorkflowHandler) FollowupWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req struct {
		Request string `json:"request"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.Request == "" {
		writeError(w, http.StatusBadRequest, "request is required")
		return
	}

	go h.orchestrator.RunFollowup(id, req.Request)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"message": "Followup workflow started",
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid workflow id")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}

// GetSysLogs handles GET /api/workflows/{id}/syslogs.
func (h *WorkflowHandler) GetSysLogs(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow id")
		return
	}

	logs, err := h.wfRepo.GetSystemLogs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get system logs")
		return
	}
	if logs == nil {
		logs = []models.SysLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

// ResumeWorkflow handles POST /api/workflows/{id}/resume.
func (h *WorkflowHandler) ResumeWorkflow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow id")
		return
	}

	repoName, projectKey, repoURL, err := h.wfRepo.GetWorkflowConfig(id)
	if err != nil || repoName == "" || projectKey == "" {
		writeError(w, http.StatusBadRequest, "cannot resume workflow: missing config (was it synced?)")
		return
	}

	// Set status back to running/in-progress
	if err := h.wfRepo.UpdateStatus(id, "running"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update workflow status")
		return
	}

	h.orchestrator.PublishSysLog(id, "Resuming autonomous loop...")
	h.orchestrator.StartAutonomousWorker(id, projectKey, repoName, repoURL)

	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

type AgentRule struct {
	ID      int    `json:"id"`
	Tag     string `json:"tag"`
	AgentID string `json:"agent_id"`
}

// GET /api/workflows/{id}/agent-rules
func (h *WorkflowHandler) GetAgentRules(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow id")
		return
	}
	rows, err := h.db.Query(`SELECT id, tag, agent_id FROM workflow_agent_rules WHERE workflow_id=$1 ORDER BY id`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query rules")
		return
	}
	defer rows.Close()
	var rules []AgentRule
	for rows.Next() {
		var rule AgentRule
		if err := rows.Scan(&rule.ID, &rule.Tag, &rule.AgentID); err != nil {
			continue
		}
		rules = append(rules, rule)
	}
	if rules == nil {
		rules = []AgentRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

// POST /api/workflows/{id}/agent-rules
func (h *WorkflowHandler) UpsertAgentRule(w http.ResponseWriter, r *http.Request) {
	wfID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow id")
		return
	}
	var body struct {
		Tag     string `json:"tag"`
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Tag == "" || body.AgentID == "" {
		writeError(w, http.StatusBadRequest, "tag and agent_id are required")
		return
	}
	_, err = h.db.Exec(
		`INSERT INTO workflow_agent_rules (workflow_id, tag, agent_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (workflow_id, tag) DO UPDATE SET agent_id = EXCLUDED.agent_id`,
		wfID, body.Tag, body.AgentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert rule")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/workflows/{id}/agent-rules/{tag}
func (h *WorkflowHandler) DeleteAgentRule(w http.ResponseWriter, r *http.Request) {
	wfID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow id")
		return
	}
	tag := chi.URLParam(r, "tag")
	h.db.Exec(`DELETE FROM workflow_agent_rules WHERE workflow_id=$1 AND tag=$2`, wfID, tag)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
