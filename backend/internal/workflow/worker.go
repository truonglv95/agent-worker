package workflow

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"ai-agent-backend/internal/agent"
	"ai-agent-backend/internal/integration"
)

// StartAutonomousWorker starts the background loop to process Jira subtasks.
func (o *Orchestrator) StartAutonomousWorker(workflowID int, projectKey, repoName, repoURL string) {
	go func() {
		o.PublishSysLog(workflowID, "Starting autonomous loop for workflow %d (Project: %s)", workflowID, projectKey)

		// 1. Setup workspace
		workspacePath := fmt.Sprintf("%s/%s", o.cfg.RootWorkspace, repoName)
		
		git := integration.NewGitWorkspace(repoURL, o.cfg.GithubToken, workspacePath)
		if err := git.Clone(); err != nil {
			o.PublishSysLog(workflowID, "Failed to clone repo: %v", err)
			_ = o.wfRepo.UpdateStatus(workflowID, "failed")
			o.publishStatus(workflowID, "failed")
			return
		}

		// 1.5 Base Project Initialization via real CLI scaffolding
		if isRepoEmpty(workspacePath) {
			o.PublishSysLog(workflowID, "Repo %s is empty — running project scaffolding...", repoName)

			// Ask the dev agent to analyze the DAB and produce a ScaffoldPlan
			scaffoldPrompt := `You are analyzing a project's Design Architecture Blueprint (DAB) from your previous conversation.
Based on the DAB, determine the correct tech stack and return a JSON ScaffoldPlan.

Return ONLY valid JSON in this EXACT format (no explanation, no markdown):
{
  "backend": {
    "enabled": true,
    "framework": "go",
    "language": "go",
    "module_name": "github.com/org/repo/backend",
    "extra_packages": ["github.com/lib/pq"]
  },
  "frontend": {
    "enabled": true,
    "framework": "vue",
    "language": "typescript",
    "module_name": "",
    "extra_packages": []
  }
}

Framework options:
- backend: "go", "express", "nestjs", "fastapi"
- frontend: "vue" (Vite+Vue3+TS), "react" (Vite+React+TS), "next" (Next.js), "nuxt" (Nuxt3)

If there is no backend (static site), set backend.enabled = false.
If there is no frontend (API only), set frontend.enabled = false.
module_name is only required for Go (e.g. "github.com/org/` + repoName + `/backend").
extra_packages: list any additional libraries mentioned in the DAB (e.g. "gorm.io/gorm" for Go, "pg" for Node).`

			devAgent, err := agent.LoadAgent("dev-backend", o.db, o.sseBus, o.cfg)
			if err != nil {
				o.PublishSysLog(workflowID, "Warn: could not load dev agent for scaffold plan: %v", err)
			} else {
				resp, err := devAgent.Call(workflowID, scaffoldPrompt, 99, workspacePath)
				if err != nil {
					o.PublishSysLog(workflowID, "Warn: scaffold plan LLM failed: %v — using default Go+Vue", err)
				}

				scaffolder := integration.NewProjectScaffolder(workspacePath, repoName)
				var plan *integration.ScaffoldPlan

				if err == nil {
					plan, err = integration.ParseScaffoldPlan(resp)
					if err != nil {
						o.PublishSysLog(workflowID, "Warn: could not parse scaffold plan (%v) — using default Go+Vue", err)
					}
				}

				// Default fallback plan
				if plan == nil {
					plan = &integration.ScaffoldPlan{
						Backend: integration.ScaffoldTarget{
							Enabled:   true,
							Framework: "go",
							Language:  "go",
							ModuleName: "github.com/project/" + repoName + "/backend",
						},
						Frontend: integration.ScaffoldTarget{
							Enabled:   true,
							Framework: "vue",
							Language:  "typescript",
						},
					}
				}

				o.PublishSysLog(workflowID, "Scaffold plan: backend=%s frontend=%s",
					plan.Backend.Framework, plan.Frontend.Framework)

				if err := scaffolder.Scaffold(plan); err != nil {
					o.PublishSysLog(workflowID, "Scaffold failed: %v", err)
				} else {
					_ = git.CommitAndPush("chore: scaffold project structure (backend+frontend)", "main")
					o.PublishSysLog(workflowID, "Project scaffolded and committed successfully.")
				}
			}
		}

		jiraClient := integration.NewJiraClient(o.cfg.JiraBaseURL, o.cfg.JiraUser, o.cfg.JiraToken)
		
		// 2. Loop over subtasks
		for {
			issues, err := jiraClient.GetPendingSubtasks(projectKey)
			if err != nil {
				o.PublishSysLog(workflowID, "Failed to get subtasks: %v", err)
				_ = o.wfRepo.UpdateStatus(workflowID, "failed")
				o.publishStatus(workflowID, "failed")
				break
			}
			
			if len(issues) == 0 {
				o.PublishSysLog(workflowID, "No pending subtasks found. Autonomous loop finished.")
				_ = o.wfRepo.UpdateStatus(workflowID, "completed")
				o.publishStatus(workflowID, "completed")
				break
			}

			// Process maximum subtasks at a time based on user config
			processCount := len(issues)
			if processCount > o.cfg.MaxSubtasksPerRun {
				processCount = o.cfg.MaxSubtasksPerRun
			}
			issues = issues[:processCount]

			for _, issue := range issues {
				o.PublishSysLog(workflowID, "Processing subtask %s: %s", issue.Key, issue.Fields.Summary)
				
				// Transition to "In Progress"
				statusLower := strings.ToLower(issue.Fields.Status.Name)
				if !strings.Contains(statusLower, "in progress") {
					if err := jiraClient.TransitionIssue(issue.Key, "In Progress"); err != nil {
						o.PublishSysLog(workflowID, "Fatal: Could not transition %s to In Progress: %v", issue.Key, err)
						_ = o.wfRepo.UpdateStatus(workflowID, "failed")
						o.publishStatus(workflowID, "failed")
						return
					}
				}

				// Setup branch
				branchName := fmt.Sprintf("feature/%s", issue.Key)
				_ = git.CheckoutBranch(branchName)

				// Create LLM prompt
				prompt := fmt.Sprintf(`Implement the following subtask. Return ONLY a JSON object containing a "files" array with "path" and "content" for the files to create/update.
				
## Subtask: %s
%s

Example Format:
{"files": [{"path": "main.go", "content": "..."}]}`, issue.Fields.Summary, issue.Fields.Description)
			agentId := resolveAgentForIssue(o.db, workflowID, issue.Fields.Summary)

				devAgent, err := agent.LoadAgent(agentId, o.db, o.sseBus, o.cfg)
				if err != nil {
					o.PublishSysLog(workflowID, "Failed to load %s agent: %v", agentId, err)
					continue
				}

				qaAgent, _ := agent.LoadAgent("qa", o.db, o.sseBus, o.cfg)
				
				retryCount := 0
				maxRetries := 3
				
				for retryCount < maxRetries {
					o.PublishSysLog(workflowID, "Issue %s - Attempt %d", issue.Key, retryCount+1)
					
					// 1. Dev generates code
					resp, err := devAgent.Call(workflowID, prompt, 99, workspacePath)
					if err != nil {
						o.PublishSysLog(workflowID, "LLM failed for %s: %v", issue.Key, err)
						break
					}
	
					// Parse JSON
					resp = strings.TrimSpace(resp)
					startIdx := strings.Index(resp, "{")
					endIdx := strings.LastIndex(resp, "}")
					if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
						resp = resp[startIdx : endIdx+1]
					}
	
					var payload struct {
						Files []map[string]string `json:"files"`
					}
					
					if err := json.Unmarshal([]byte(resp), &payload); err != nil {
						o.PublishSysLog(workflowID, "JSON parse failed for %s: %v", issue.Key, err)
						break
					}
					
					// Write files
					filesMap := make(map[string]string)
					for _, f := range payload.Files {
						filesMap[f["path"]] = f["content"]
					}
					if err := git.WriteFiles(filesMap); err != nil {
						o.PublishSysLog(workflowID, "WriteFiles failed: %v", err)
						break
					}
					
					// 2. Start Dev Runner for QA
					runner := integration.NewDevRunner(workspacePath)
					if err := runner.Start(); err != nil {
						o.PublishSysLog(workflowID, "DevRunner failed to start: %v", err)
						// Fallback to push anyway if runner fails (maybe not a web app)
						_ = git.CommitAndPush(fmt.Sprintf("[%s] %s", issue.Key, issue.Fields.Summary), branchName)
						if err := jiraClient.TransitionIssue(issue.Key, "Done"); err != nil {
							o.PublishSysLog(workflowID, "Warn: Could not transition %s to Done: %v", issue.Key, err)
						}
						break
					}
					
					// 3. QA Verification (Code Review)
					var qaPayload struct {
						Status   string `json:"status"`
						Feedback string `json:"feedback"`
					}
					
					if qaAgent.Active {
						qaPrompt := fmt.Sprintf(`The feature '%s' has been implemented. Here are the code changes made by the developer:
%s

Please review the code changes to ensure they meet the requirements and follow best practices. Check for any obvious bugs or missing logic.
Return ONLY a valid JSON object in this format: {"status": "pass" or "fail", "feedback": "your detailed feedback"}.`, issue.Fields.Summary, resp)
						
						qaResp, qaErr := qaAgent.Call(workflowID, qaPrompt, 99, workspacePath)
						
						// Stop Server
						runner.Stop()
						
						if qaErr != nil {
							o.PublishSysLog(workflowID, "QA LLM failed: %v", qaErr)
							break
						}
						
						// Check Feedback
						qaResp = strings.TrimSpace(qaResp)
						qStart := strings.Index(qaResp, "{")
						qEnd := strings.LastIndex(qaResp, "}")
						if qStart != -1 && qEnd != -1 && qEnd >= qStart {
							qaResp = qaResp[qStart : qEnd+1]
						}
						_ = json.Unmarshal([]byte(qaResp), &qaPayload)
					} else {
						// Stop Server
						runner.Stop()
						o.PublishSysLog(workflowID, "QA Agent is on bench. Skipping code review & test suite checks.")
						qaPayload.Status = "pass"
					}
					
					if qaPayload.Status == "fail" && retryCount < maxRetries-1 {
						o.PublishSysLog(workflowID, "QA rejected %s: %s", issue.Key, qaPayload.Feedback)
						prompt = fmt.Sprintf("QA found the following issues:\n%s\n\nPlease fix them and return the updated JSON files array.", qaPayload.Feedback)
						retryCount++
						continue
					}
					
					// QA Passed or Max Retries reached
					o.PublishSysLog(workflowID, "QA passed or max retries reached for %s", issue.Key)
					msg := fmt.Sprintf("[%s] %s", issue.Key, issue.Fields.Summary)
					if err := git.CommitAndPush(msg, branchName); err == nil {
						if tErr := jiraClient.TransitionIssue(issue.Key, "Review"); tErr != nil {
							if dErr := jiraClient.TransitionIssue(issue.Key, "Done"); dErr != nil {
								o.PublishSysLog(workflowID, "Warn: Could not transition %s to Done/Review: %v", issue.Key, dErr)
							}
						}
					}
					break
				}
			}

			// Wait a bit before checking again to avoid spamming
			time.Sleep(10 * time.Second)
			
			// According to user, we stop at max 5 subtasks per run for now
			o.PublishSysLog(workflowID, "Reached %d max subtasks limit per run. Pausing.", o.cfg.MaxSubtasksPerRun)
			_ = o.wfRepo.UpdateStatus(workflowID, "paused")
			o.publishStatus(workflowID, "paused")
			break
		}
	}()
}

// isRepoEmpty checks if the directory is essentially empty (only contains .git)
func isRepoEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return true // If we can't read it, assume it needs init
	}
	for _, entry := range entries {
		if entry.Name() != ".git" {
			return false
		}
	}
	return true
}

// resolveAgentForIssue looks up the workflow_agent_rules table to find which
// agent should handle a given Jira issue summary. Falls back to "dev-backend" if no
// matching rule is found.
func resolveAgentForIssue(db *sql.DB, workflowID int, summary string) string {
	rows, err := db.Query(
		`SELECT tag, agent_id FROM workflow_agent_rules WHERE workflow_id = $1`,
		workflowID,
	)
	if err != nil {
		return "dev-backend"
	}
	defer rows.Close()

	for rows.Next() {
		var tag, agentID string
		if err := rows.Scan(&tag, &agentID); err != nil {
			continue
		}
		if tag != "" && strings.Contains(summary, tag) {
			return agentID
		}
	}
	return "dev-backend"
}
