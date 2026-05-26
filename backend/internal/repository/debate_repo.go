package repository

import (
	"database/sql"
	"fmt"

	"ai-agent-backend/internal/models"
)

// DebateRepo handles persistence for debate messages.
type DebateRepo struct {
	db *sql.DB
}

// NewDebateRepo constructs a DebateRepo with the given connection pool.
func NewDebateRepo(db *sql.DB) *DebateRepo {
	return &DebateRepo{db: db}
}

// SaveDebate inserts a new debate turn record and returns its ID.
func (r *DebateRepo) SaveDebate(wfID int, agentID, agentName, role, message string, round int) (int, error) {
	var id int
	err := r.db.QueryRow(
		`INSERT INTO debates (workflow_id, agent_id, agent_name, role, message, round)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		wfID, agentID, agentName, role, message, round,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("debate_repo: save: %w", err)
	}
	return id, nil
}

// GetDebates returns all debate messages for a workflow, ordered chronologically.
func (r *DebateRepo) GetDebates(wfID int) ([]*models.DebateMessage, error) {
	rows, err := r.db.Query(
		`SELECT id, workflow_id, agent_id, agent_name, role, message, round, created_at
		 FROM debates WHERE workflow_id = $1 ORDER BY created_at ASC`,
		wfID,
	)
	if err != nil {
		return nil, fmt.Errorf("debate_repo: get: %w", err)
	}
	defer rows.Close()

	var list []*models.DebateMessage
	for rows.Next() {
		dm := &models.DebateMessage{}
		if err := rows.Scan(
			&dm.ID, &dm.WorkflowID, &dm.AgentID, &dm.AgentName,
			&dm.Role, &dm.Message, &dm.Round, &dm.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("debate_repo: scan: %w", err)
		}
		list = append(list, dm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("debate_repo: rows: %w", err)
	}
	return list, nil
}
