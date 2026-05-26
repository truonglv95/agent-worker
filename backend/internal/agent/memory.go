package agent

import (
	"fmt"
	"strings"

	"ai-agent-backend/internal/models"
)

// BuildContextString formats a slice of agent memory entries into a
// human-readable block that is prepended to every agent prompt.
// Entries are printed in chronological order grouped by round.
func BuildContextString(memories []*models.AgentMemory) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, m := range memories {
		role := strings.Title(strings.ToLower(m.Role)) //nolint:staticcheck // acceptable for display
		sb.WriteString(fmt.Sprintf("[Round %d - %s]: %s\n\n", m.Round, role, m.Content))
	}
	return sb.String()
}
