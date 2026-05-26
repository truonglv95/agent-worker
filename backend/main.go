package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	_ = godotenv.Load()
	log.Println("Starting AI Agent Workflow Backend...")

	// Initialize Database
	InitDB()
	defer DB.Close()

	// Setup Router
	r := chi.NewRouter()

	// Endpoints
	r.Get("/api/workflows", GetWorkflows)
	r.Post("/api/workflows", CreateWorkflow)
	r.Get("/api/workflows/{id}", GetWorkflowDetails)
	r.Get("/api/workflows/{id}/debates", GetWorkflowDebates)
	r.Get("/api/workflows/{id}/tasks", GetWorkflowTasks)

	// Configure CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding"},
		AllowCredentials: true,
	})

	handler := c.Handler(r)

	port := ":8080"
	log.Printf("Server is running on port %s", port)
	if err := http.ListenAndServe(port, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// REST Handlers
func GetWorkflows(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id, request, status, TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS') FROM workflows ORDER BY id DESC`
	rows, err := DB.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	workflows := []Workflow{}
	for rows.Next() {
		var wf Workflow
		if err := rows.Scan(&wf.ID, &wf.Request, &wf.Status, &wf.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		workflows = append(workflows, wf)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflows)
}

func CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Request string `json:"request"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid input payload", http.StatusBadRequest)
		return
	}

	if input.Request == "" {
		http.Error(w, "Request description cannot be empty", http.StatusBadRequest)
		return
	}

	var workflowID int
	query := `INSERT INTO workflows (request, status) VALUES ($1, $2) RETURNING id`
	err := DB.QueryRow(query, input.Request, "PENDING").Scan(&workflowID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Trigger background orchestrator execution
	StartWorkflow(workflowID, input.Request)

	resp := map[string]interface{}{
		"id":      workflowID,
		"request": input.Request,
		"status":  "PENDING",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func GetWorkflowDetails(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	var wf Workflow
	query := `SELECT id, request, status, TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS') FROM workflows WHERE id = $1`
	err = DB.QueryRow(query, id).Scan(&wf.ID, &wf.Request, &wf.Status, &wf.CreatedAt)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

func GetWorkflowDebates(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	query := `SELECT id, workflow_id, agent_id, agent_name, role, message, TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS') 
		FROM debates WHERE workflow_id = $1 ORDER BY id ASC`
	rows, err := DB.Query(query, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	debates := []DebateMessage{}
	for rows.Next() {
		var dm DebateMessage
		if err := rows.Scan(&dm.ID, &dm.WorkflowID, &dm.AgentID, &dm.AgentName, &dm.Role, &dm.Message, &dm.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		debates = append(debates, dm)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debates)
}

func GetWorkflowTasks(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	query := `SELECT id, workflow_id, title, description, assigned_to, status, COALESCE(output_content, ''), TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS') 
		FROM tasks WHERE workflow_id = $1 ORDER BY id ASC`
	rows, err := DB.Query(query, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := []TaskItem{}
	for rows.Next() {
		var t TaskItem
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Title, &t.Description, &t.AssignedTo, &t.Status, &t.OutputContent, &t.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
