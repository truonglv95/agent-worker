package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

type RunRequest struct {
	Prompt         string `json:"prompt"`
	SysPrompt      string `json:"sys_prompt"`
	AgentName      string `json:"agent_name"`
	AgentRole      string `json:"agent_role"`
	ConversationID string `json:"conversation_id"`
	Stream         bool   `json:"stream"`
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	log.Printf("[runner] handling request for workflow: %s, stream: %v", req.ConversationID, req.Stream)

	args := []string{
		"--print", req.Prompt,
		"--dangerously-skip-permissions",
		"--log-file", "/dev/stderr",
	}
	if req.ConversationID != "" {
		args = append(args, "--conversation", "Workflow-"+req.ConversationID)
	}

	cmd := exec.Command("agy", args...)

	if req.Stream {
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			http.Error(w, "Failed to get stdout pipe", http.StatusInternalServerError)
			return
		}

		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		if err := cmd.Start(); err != nil {
			http.Error(w, "Failed to start command", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if isGlogLine(line) {
				continue
			}
			
			// We wrap the text in JSON to make it easy to parse on the frontend,
			// or simply send the raw text in an SSE data block.
			// Let's send raw text for simplicity.
			chunk, _ := json.Marshal(map[string]string{"text": line + "\n"})
			w.Write([]byte("data: " + string(chunk) + "\n\n"))
			flusher.Flush()
		}

		if err := cmd.Wait(); err != nil {
			log.Printf("[runner] stream CLI error: %v, stderr: %s", err, stderrBuf.String())
			errChunk, _ := json.Marshal(map[string]string{"error": err.Error(), "stderr": stderrBuf.String()})
			w.Write([]byte("data: " + string(errChunk) + "\n\n"))
			flusher.Flush()
		}

		// Send a done event
		w.Write([]byte("event: done\ndata: {}\n\n"))
		flusher.Flush()
		return
	}

	// Non-streaming mode
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		log.Printf("[runner] CLI error: %v, stderr: %s", err, stderrBuf.String())
		http.Error(w, "CLI execution failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cleanOutput := filterGlogLines(stdoutBuf.String())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"output": cleanOutput,
	})
}

func isGlogLine(line string) bool {
	if strings.HasPrefix(line, "Warning: conversation ") {
		return true
	}
	if len(line) > 0 && (line[0] == 'I' || line[0] == 'E' || line[0] == 'W' || line[0] == 'F') && len(line) > 20 && line[2] == '2' {
		return true
	}
	return false
}

func filterGlogLines(output string) string {
	var clean []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if isGlogLine(line) {
			continue
		}
		clean = append(clean, line)
	}
	return strings.TrimSpace(strings.Join(clean, "\n"))
}
