package agent

import (
	"database/sql"
	"fmt"
	"os"

	"ai-agent-backend/internal/config"
	"ai-agent-backend/internal/repository"
	"ai-agent-backend/internal/sse"
)

// LoadAllAgents fetches all agent definitions from the database and wraps each
// one in an Agent ready for use (with DB, SSEBus, and Config injected).
func LoadAllAgents(
	db *sql.DB,
	sseBus *sse.SSEBus,
	cfg *config.Config,
) ([]*Agent, error) {
	rows, err := db.Query(
		`SELECT id, name, role, system_prompt, task_description, active FROM agents ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("definitions: query agents: %w", err)
	}
	defer rows.Close()

	memRepo := repository.NewMemoryRepo(db)
	debRepo := repository.NewDebateRepo(db)

	var agents []*Agent
	for rows.Next() {
		a := &Agent{
			memRepo: memRepo,
			debRepo: debRepo,
			sseBus:  sseBus,
			cfg:     cfg,
		}
		if err := rows.Scan(&a.ID, &a.Name, &a.Role, &a.SystemPrompt, &a.TaskDescription, &a.Active); err != nil {
			return nil, fmt.Errorf("definitions: scan: %w", err)
		}
		injectSkillContent(a)
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("definitions: rows: %w", err)
	}
	return agents, nil
}

// LoadAgent fetches a single agent by its ID.
func LoadAgent(
	id string,
	db *sql.DB,
	sseBus *sse.SSEBus,
	cfg *config.Config,
) (*Agent, error) {
	a := &Agent{
		memRepo: repository.NewMemoryRepo(db),
		debRepo: repository.NewDebateRepo(db),
		sseBus:  sseBus,
		cfg:     cfg,
	}
	err := db.QueryRow(
		`SELECT id, name, role, system_prompt, task_description, active FROM agents WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.Name, &a.Role, &a.SystemPrompt, &a.TaskDescription, &a.Active)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("definitions: agent %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("definitions: load agent: %w", err)
	}
	injectSkillContent(a)
	return a, nil
}

func injectSkillContent(a *Agent) {
	skillPath := fmt.Sprintf("skills/%s.md", a.ID)
	content, err := os.ReadFile(skillPath)
	if err == nil && len(content) > 0 {
		a.SystemPrompt += "\n\n## Agent Skills & Guidelines:\n" + string(content)
	}
}
