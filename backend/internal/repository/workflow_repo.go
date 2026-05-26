package repository

import (
	"database/sql"
	"fmt"

	"ai-agent-backend/internal/models"
)

// WorkflowRepo handles persistence for Workflow records.
type WorkflowRepo struct {
	db *sql.DB
}

// NewWorkflowRepo constructs a WorkflowRepo with the given connection pool.
func NewWorkflowRepo(db *sql.DB) *WorkflowRepo {
	return &WorkflowRepo{db: db}
}

// CreateWorkflow inserts a new workflow row and returns its auto-generated ID.
func (r *WorkflowRepo) CreateWorkflow(request string, rounds int) (int, error) {
	var id int
	err := r.db.QueryRow(
		`INSERT INTO workflows (request, status, debate_rounds)
		 VALUES ($1, 'pending', $2)
		 RETURNING id`,
		request, rounds,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("workflow_repo: create: %w", err)
	}
	return id, nil
}

// UpdateStatus changes the status column for the given workflow.
func (r *WorkflowRepo) UpdateStatus(id int, status string) error {
	_, err := r.db.Exec(
		`UPDATE workflows SET status = $1 WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("workflow_repo: update_status: %w", err)
	}
	return nil
}

// UpdateWorkflowSyncDetails saves the Github repo name, project key, and repo URL.
func (r *WorkflowRepo) UpdateWorkflowSyncDetails(id int, repoName, projectKey, repoURL string) error {
	_, err := r.db.Exec(
		`UPDATE workflows SET repo_name = $1, project_key = $2, repo_url = $3 WHERE id = $4`,
		repoName, projectKey, repoURL, id,
	)
	if err != nil {
		return fmt.Errorf("workflow_repo: update_sync_details: %w", err)
	}
	return nil
}

// GetWorkflowConfig retrieves the sync config needed to resume a workflow.
func (r *WorkflowRepo) GetWorkflowConfig(id int) (repoName, projectKey, repoURL string, err error) {
	err = r.db.QueryRow(
		`SELECT COALESCE(repo_name, ''), COALESCE(project_key, ''), COALESCE(repo_url, '') FROM workflows WHERE id = $1`,
		id,
	).Scan(&repoName, &projectKey, &repoURL)
	return
}

// SaveSystemLog inserts a new system log message.
func (r *WorkflowRepo) SaveSystemLog(wfID int, message string) error {
	_, err := r.db.Exec(
		`INSERT INTO system_logs (workflow_id, message) VALUES ($1, $2)`,
		wfID, message,
	)
	if err != nil {
		return fmt.Errorf("workflow_repo: save_sys_log: %w", err)
	}
	return nil
}

// GetSystemLogs returns all system logs for a workflow.
func (r *WorkflowRepo) GetSystemLogs(wfID int) ([]models.SysLog, error) {
	rows, err := r.db.Query(
		`SELECT id, message, created_at FROM system_logs WHERE workflow_id = $1 ORDER BY created_at ASC`,
		wfID,
	)
	if err != nil {
		return nil, fmt.Errorf("workflow_repo: get_sys_logs: %w", err)
	}
	defer rows.Close()

	var logs []models.SysLog
	for rows.Next() {
		var log models.SysLog
		if err := rows.Scan(&log.ID, &log.Message, &log.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

// DeleteWorkflow removes a workflow and all its cascading data from the database.
func (r *WorkflowRepo) DeleteWorkflow(id int) error {
	_, err := r.db.Exec(`DELETE FROM workflows WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("workflow_repo: delete: %w", err)
	}
	return nil
}

// GetWorkflow retrieves a single workflow by primary key.
func (r *WorkflowRepo) GetWorkflow(id int) (*models.Workflow, error) {
	wf := &models.Workflow{}
	err := r.db.QueryRow(
		`SELECT id, request, status, debate_rounds, created_at
		 FROM workflows WHERE id = $1`,
		id,
	).Scan(&wf.ID, &wf.Request, &wf.Status, &wf.DebateRounds, &wf.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workflow_repo: not found: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("workflow_repo: get: %w", err)
	}
	return wf, nil
}

// ListWorkflows returns all workflows ordered by creation time descending.
func (r *WorkflowRepo) ListWorkflows() ([]*models.Workflow, error) {
	rows, err := r.db.Query(
		`SELECT id, request, status, debate_rounds, created_at
		 FROM workflows ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("workflow_repo: list: %w", err)
	}
	defer rows.Close()

	var list []*models.Workflow
	for rows.Next() {
		wf := &models.Workflow{}
		if err := rows.Scan(&wf.ID, &wf.Request, &wf.Status, &wf.DebateRounds, &wf.CreatedAt); err != nil {
			return nil, fmt.Errorf("workflow_repo: list scan: %w", err)
		}
		list = append(list, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workflow_repo: list rows: %w", err)
	}
	return list, nil
}
