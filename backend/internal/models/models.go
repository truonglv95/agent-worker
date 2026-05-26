package models

import "time"

// Workflow represents a top-level multi-agent workflow run.
type Workflow struct {
	ID           int       `json:"id"`
	Request      string    `json:"request"`
	Status       string    `json:"status"`
	DebateRounds int       `json:"debate_rounds"`
	CreatedAt    time.Time `json:"created_at"`
}

// Agent represents a persistent AI agent definition stored in the database.
type Agent struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	SystemPrompt    string `json:"system_prompt"`
	TaskDescription string `json:"task_description"`
	Active          bool   `json:"active"`
}

// AgentMemory is one memory entry for a given agent within a workflow round.
type AgentMemory struct {
	ID         int       `json:"id"`
	WorkflowID int       `json:"workflow_id"`
	AgentID    string    `json:"agent_id"`
	Role       string    `json:"role"` // "user" | "assistant"
	Content    string    `json:"content"`
	Round      int       `json:"round"`
	CreatedAt  time.Time `json:"created_at"`
}

// DebateMessage is one turn in the structured debate between agents.
type DebateMessage struct {
	ID         int       `json:"id"`
	WorkflowID int       `json:"workflow_id"`
	AgentID    string    `json:"agent_id"`
	AgentName  string    `json:"agent_name"`
	Role       string    `json:"role"`
	Message    string    `json:"message"`
	Round      int       `json:"round"`
	CreatedAt  time.Time `json:"created_at"`
}

// Task is a work item created or updated during the workflow.
type Task struct {
	ID          int       `json:"id"`
	WorkflowID  int       `json:"workflow_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	AssignedTo  string    `json:"assigned_to"`
	Status      string    `json:"status"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}

// SSEEvent is the envelope pushed over the Server-Sent Events stream.
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// SysLog represents a system log entry for a workflow.
type SysLog struct {
	ID        int       `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
