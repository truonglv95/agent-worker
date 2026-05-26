package repository

import (
	"database/sql"
	"fmt"

	"ai-agent-backend/internal/models"
)

// MemoryRepo handles persistence for per-agent conversation memory.
type MemoryRepo struct {
	db *sql.DB
}

// NewMemoryRepo constructs a MemoryRepo with the given connection pool.
func NewMemoryRepo(db *sql.DB) *MemoryRepo {
	return &MemoryRepo{db: db}
}

// SaveMemory inserts one memory entry for an agent within a workflow.
func (r *MemoryRepo) SaveMemory(wfID int, agentID, role, content string, round int) error {
	_, err := r.db.Exec(
		`INSERT INTO agent_memories (workflow_id, agent_id, role, content, round)
		 VALUES ($1, $2, $3, $4, $5)`,
		wfID, agentID, role, content, round,
	)
	if err != nil {
		return fmt.Errorf("memory_repo: save: %w", err)
	}
	return nil
}

// GetAgentMemory returns all memory entries for a specific agent in a workflow.
func (r *MemoryRepo) GetAgentMemory(wfID int, agentID string) ([]*models.AgentMemory, error) {
	rows, err := r.db.Query(
		`SELECT id, workflow_id, agent_id, role, content, round, created_at
		 FROM agent_memories
		 WHERE workflow_id = $1 AND agent_id = $2
		 ORDER BY round ASC, created_at ASC`,
		wfID, agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("memory_repo: get: %w", err)
	}
	defer rows.Close()

	var list []*models.AgentMemory
	for rows.Next() {
		m := &models.AgentMemory{}
		if err := rows.Scan(
			&m.ID, &m.WorkflowID, &m.AgentID, &m.Role, &m.Content, &m.Round, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("memory_repo: scan: %w", err)
		}
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory_repo: rows: %w", err)
	}
	return list, nil
}

// ClearAgentMemory deletes all memory entries for a specific agent in a workflow.
func (r *MemoryRepo) ClearAgentMemory(wfID int, agentID string) error {
	_, err := r.db.Exec(
		`DELETE FROM agent_memories WHERE workflow_id = $1 AND agent_id = $2`,
		wfID, agentID,
	)
	if err != nil {
		return fmt.Errorf("memory_repo: clear: %w", err)
	}
	return nil
}
