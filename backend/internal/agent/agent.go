package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"ai-agent-backend/internal/config"
	"ai-agent-backend/internal/models"
	"ai-agent-backend/internal/repository"
	"ai-agent-backend/internal/sse"
)

// Agent wraps a persisted agent definition and the live dependencies it needs
// to call the CLI, persist memory, and publish SSE events.
type Agent struct {
	ID              string
	Name            string
	Role            string
	SystemPrompt    string
	TaskDescription string
	Active          bool

	memRepo *repository.MemoryRepo
	debRepo *repository.DebateRepo
	sseBus  *sse.SSEBus
	cfg     *config.Config
}

// Call invokes the agent via the configured CLI or API, processes its logic, and returns the response.
func (a *Agent) Call(workflowID int, userPrompt string, round int, workspacePath ...string) (string, error) {
	// 1. Fetch conversation history for this workflow.us memory.
	memories, err := a.memRepo.GetAgentMemory(workflowID, a.ID)
	if err != nil {
		log.Printf("[agent:%s] warn: could not load memory: %v", a.ID, err)
	}
	contextBlock := BuildContextString(memories)

	// 2. Assemble the full prompt.
	var sb strings.Builder
	sb.WriteString(a.SystemPrompt)
	if contextBlock != "" {
		sb.WriteString("\n\n## Your Previous Context:\n")
		sb.WriteString(contextBlock)
	}
	sb.WriteString("\n\n## Current Task:\n")
	sb.WriteString(userPrompt)

	antiToolInstruction := "\n\nIMPORTANT: Do NOT use any tools. Do NOT create files, plans, or artifacts. Answer strictly in plain text immediately."
	sb.WriteString(antiToolInstruction)

	fullPrompt := sb.String()

	// 3. Save the user-side memory entry.
	if err := a.memRepo.SaveMemory(workflowID, a.ID, "user", userPrompt, round); err != nil {
		log.Printf("[agent:%s] warn: save user memory: %v", a.ID, err)
	}

	// 3.5 Publish "agent_thinking" SSE event before running the CLI.
	a.sseBus.Publish(workflowID, models.SSEEvent{
		Type: "agent_thinking",
		Data: map[string]interface{}{
			"agent_id":   a.ID,
			"agent_name": a.Name,
			"role":       a.Role,
		},
	})

	// 4. Execute the CLI.
	wsPath := ""
	if len(workspacePath) > 0 {
		wsPath = workspacePath[0]
	}
	response, err := a.execCLI(fullPrompt, strconv.Itoa(workflowID), wsPath)
	if err != nil {
		return "", err
	}
	response = strings.TrimSpace(response)

	// 5. Persist assistant memory.
	if err := a.memRepo.SaveMemory(workflowID, a.ID, "assistant", response, round); err != nil {
		log.Printf("[agent:%s] warn: save assistant memory: %v", a.ID, err)
	}

	// 6. Persist debate entry.
	id, err := a.debRepo.SaveDebate(workflowID, a.ID, a.Name, a.Role, response, round)
	if err != nil {
		log.Printf("[agent:%s] warn: save debate: %v", a.ID, err)
	}

	// 7. Publish SSE event.
	dm := models.DebateMessage{
		ID:         id,
		WorkflowID: workflowID,
		AgentID:    a.ID,
		AgentName:  a.Name,
		Role:       a.Role,
		Message:    response,
		Round:      round,
	}
	a.sseBus.Publish(workflowID, models.SSEEvent{Type: "debate_message", Data: dm})

	return response, nil
}

// execCLI shells out to the configured agy binary or calls the runner API and returns stdout.
func (a *Agent) execCLI(prompt string, workflowID string, workspacePath string) (string, error) {
	if a.cfg.AgyRunnerURL != "" {
		return a.execRunnerAPI(prompt, workflowID)
	}

	path, err := exec.LookPath(a.cfg.AgyCLIPath)
	if err != nil {
		return "", fmt.Errorf(
			"agent: agy CLI not found at %q — install it or set AGY_CLI_PATH: %w",
			a.cfg.AgyCLIPath, err,
		)
	}

	cmdArgs := []string{"--print", prompt, "--dangerously-skip-permissions", "--log-file=/dev/stderr"}
	if workflowID != "" {
		cmdArgs = append(cmdArgs, "--conversation-id", workflowID)
	}
	if workspacePath != "" {
		cmdArgs = append(cmdArgs, "--add-dir", workspacePath)
	}

	if a.Role == "qa" {
		// Pass the Chrome DevTools MCP for QA so it can verify UI locally
		cmdArgs = append(cmdArgs, "--mcp-server", `{"name": "chrome-devtools", "command": "npx", "args": ["-y", "@antigravity/chrome-devtools-mcp"]}`)
	}

	cmd := exec.Command(path, cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	log.Printf("[agent:%s] running agy CLI local (prompt length=%d)", a.ID, len(prompt))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(
			"agent: agy CLI error: %w\nstderr: %s",
			err, stderr.String(),
		)
	}

	rawOutput := stdout.String()
	return filterGlogOutput(rawOutput), nil
}

var (
	runnerAccessToken  string
	runnerRefreshToken string
	runnerTokenMutex   sync.Mutex
)

func (a *Agent) getRunnerToken() (string, error) {
	runnerTokenMutex.Lock()
	defer runnerTokenMutex.Unlock()

	if runnerAccessToken != "" {
		return runnerAccessToken, nil
	}

	adminUser := os.Getenv("RUNNER_ADMIN_USER")
	adminPass := os.Getenv("RUNNER_ADMIN_PASS")
	if adminUser == "" || adminPass == "" {
		return "", fmt.Errorf("RUNNER_ADMIN_USER or RUNNER_ADMIN_PASS not configured in backend")
	}

	payload := map[string]string{"username": adminUser, "password": adminPass}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", a.cfg.AgyRunnerURL+"/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("runner login failed with status %d", resp.StatusCode)
	}

	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	runnerAccessToken = data["access_token"]
	runnerRefreshToken = data["refresh_token"]

	return runnerAccessToken, nil
}

func (a *Agent) execRunnerAPI(prompt, workflowID string) (string, error) {
	token, err := a.getRunnerToken()
	if err != nil {
		return "", fmt.Errorf("agent: failed to get runner token: %w", err)
	}

	payload := map[string]string{
		"prompt":          prompt,
		"agent_name":      a.Name,
		"agent_role":      a.Role,
		"sys_prompt":      a.SystemPrompt,
		"conversation_id": workflowID,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", a.cfg.AgyRunnerURL+"/api/v1/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	log.Printf("[agent:%s] calling runner API: %s", a.ID, a.cfg.AgyRunnerURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("agent: runner api error: %w", err)
	}
	defer resp.Body.Close()

	// If 401, clear cache and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		runnerTokenMutex.Lock()
		runnerAccessToken = ""
		runnerTokenMutex.Unlock()
		return a.execRunnerAPI(prompt, workflowID)
	}

	var resData map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return "", fmt.Errorf("agent: runner api decode error: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("agent: runner api failed: %s", resData["error"])
	}

	return filterGlogOutput(resData["output"]), nil
}

func filterGlogOutput(raw string) string {
	lines := strings.Split(raw, "\n")
	var cleanLines []string
	for _, line := range lines {
		if len(line) >= 5 && (line[0] == 'I' || line[0] == 'W' || line[0] == 'E' || line[0] == 'F') {
			if line[1] >= '0' && line[1] <= '1' && line[2] >= '0' && line[2] <= '9' && line[3] >= '0' && line[3] <= '3' && line[4] >= '0' && line[4] <= '9' {
				continue
			}
		}
		cleanLines = append(cleanLines, line)
	}
	return strings.Join(cleanLines, "\n")
}
