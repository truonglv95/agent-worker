package repository

import (
	"database/sql"
	"fmt"

	"ai-agent-backend/internal/models"
)

// TaskRepo handles persistence for workflow tasks.
type TaskRepo struct {
	db *sql.DB
}

// NewTaskRepo constructs a TaskRepo with the given connection pool.
func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

// SaveTask inserts a new task row for a workflow.
func (r *TaskRepo) SaveTask(wfID int, title, desc, taskType, assignedTo, status, content string) error {
	_, err := r.db.Exec(
		`INSERT INTO tasks (workflow_id, title, description, type, assigned_to, status, content)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		wfID, title, desc, taskType, assignedTo, status, content,
	)
	if err != nil {
		return fmt.Errorf("task_repo: save: %w", err)
	}
	return nil
}

// GetTasks returns all tasks belonging to a workflow.
func (r *TaskRepo) GetTasks(wfID int) ([]*models.Task, error) {
	rows, err := r.db.Query(
		`SELECT id, workflow_id, title, description, type, assigned_to, status, content, created_at
		 FROM tasks WHERE workflow_id = $1 ORDER BY created_at ASC`,
		wfID,
	)
	if err != nil {
		return nil, fmt.Errorf("task_repo: get: %w", err)
	}
	defer rows.Close()

	var list []*models.Task
	for rows.Next() {
		t := &models.Task{}
		if err := rows.Scan(
			&t.ID, &t.WorkflowID, &t.Title, &t.Description,
			&t.Type, &t.AssignedTo, &t.Status, &t.Content, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("task_repo: scan: %w", err)
		}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task_repo: rows: %w", err)
	}
	return list, nil
}

// UpdateTaskStatus updates the status and content of an existing task.
func (r *TaskRepo) UpdateTaskStatus(id int, status, content string) error {
	_, err := r.db.Exec(
		`UPDATE tasks SET status = $1, content = $2 WHERE id = $3`,
		status, content, id,
	)
	if err != nil {
		return fmt.Errorf("task_repo: update_status: %w", err)
	}
	return nil
}
