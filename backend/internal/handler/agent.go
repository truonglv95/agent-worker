package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type AgentHandler struct {
	db *sql.DB
}

func NewAgentHandler(db *sql.DB) *AgentHandler {
	return &AgentHandler{db: db}
}

type AgentPayload struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	SystemPrompt    string `json:"system_prompt"`
	TaskDescription string `json:"task_description"`
	Active          bool   `json:"active"`
}

// GET /api/agents
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, name, role, system_prompt, task_description, active FROM agents ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}
	defer rows.Close()

	var agents []AgentPayload
	for rows.Next() {
		var a AgentPayload
		if err := rows.Scan(&a.ID, &a.Name, &a.Role, &a.SystemPrompt, &a.TaskDescription, &a.Active); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan agent")
			return
		}
		agents = append(agents, a)
	}
	if agents == nil {
		agents = []AgentPayload{}
	}
	writeJSON(w, http.StatusOK, agents)
}

// POST /api/agents
func (h *AgentHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var body AgentPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	body.ID = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(body.ID), " ", "-"))
	if body.ID == "" || body.Name == "" || body.Role == "" {
		writeError(w, http.StatusBadRequest, "id, name, and role are required")
		return
	}

	_, err := h.db.Exec(
		`INSERT INTO agents (id, name, role, system_prompt, task_description, active) VALUES ($1, $2, $3, $4, $5, $6)`,
		body.ID, body.Name, body.Role, body.SystemPrompt, body.TaskDescription, body.Active,
	)
	if err != nil {
		writeError(w, http.StatusConflict, "agent id already exists or insert failed")
		return
	}
	writeJSON(w, http.StatusCreated, body)
}

// PUT /api/agents/{id}
func (h *AgentHandler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body AgentPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	res, err := h.db.Exec(
		`UPDATE agents SET name=$1, role=$2, system_prompt=$3, task_description=$4, active=$5 WHERE id=$6`,
		body.Name, body.Role, body.SystemPrompt, body.TaskDescription, body.Active, id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update agent")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	body.ID = id
	writeJSON(w, http.StatusOK, body)
}

// DELETE /api/agents/{id}
func (h *AgentHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := h.db.Exec(`DELETE FROM agents WHERE id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
