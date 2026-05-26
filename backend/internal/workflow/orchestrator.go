package workflow

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"ai-agent-backend/internal/agent"
	"ai-agent-backend/internal/config"
	"ai-agent-backend/internal/integration"
	"ai-agent-backend/internal/models"
	"ai-agent-backend/internal/repository"
	"ai-agent-backend/internal/sse"
)

// Orchestrator drives the multi-agent debate loop for a workflow.
type Orchestrator struct {
	db       *sql.DB
	sseBus   *sse.SSEBus
	cfg      *config.Config
	wfRepo   *repository.WorkflowRepo
	taskRepo *repository.TaskRepo
}

// NewOrchestrator constructs an Orchestrator with all required dependencies.
func NewOrchestrator(
	db *sql.DB,
	sseBus *sse.SSEBus,
	cfg *config.Config,
) *Orchestrator {
	return &Orchestrator{
		db:       db,
		sseBus:   sseBus,
		cfg:      cfg,
		wfRepo:   repository.NewWorkflowRepo(db),
		taskRepo: repository.NewTaskRepo(db),
	}
}

// Run executes the full 3-round debate loop in a goroutine.
// Workflow status is updated progressively and set to COMPLETED or FAILED.
func (o *Orchestrator) Run(workflowID int, request string) {
	log.Printf("[orchestrator] starting workflow %d", workflowID)

	if err := o.wfRepo.UpdateStatus(workflowID, "running"); err != nil {
		log.Printf("[orchestrator] warn: set running: %v", err)
	}
	o.publishStatus(workflowID, "running")
	// Pre-process request for Figma URLs
	if strings.Contains(request, "figma.com") && o.cfg.FigmaToken != "" {
		re := regexp.MustCompile(`(https://www.figma.com/[^\s]+)`)
		if match := re.FindString(request); match != "" {
			log.Printf("[orchestrator] found figma URL, extracting design tokens...")
			figmaClient := integration.NewFigmaClient(o.cfg.FigmaToken)
			designTokens, err := figmaClient.ExtractDesignTokens(match)
			if err != nil {
				log.Printf("[orchestrator] figma extraction failed (continuing without it): %v", err)
			} else {
				request = request + "\n\n" + designTokens
				log.Printf("[orchestrator] successfully appended figma design tokens to request.")
			}
		}
	}

	if err := o.runDebate(workflowID, request); err != nil {
		log.Printf("[orchestrator] workflow %d failed: %v", workflowID, err)
		_ = o.wfRepo.UpdateStatus(workflowID, "failed")
		o.publishStatus(workflowID, "failed")
		o.publishError(workflowID, err.Error())
		return
	}

	_ = o.wfRepo.UpdateStatus(workflowID, "completed")
	o.publishStatus(workflowID, "completed")
	log.Printf("[orchestrator] workflow %d completed", workflowID)

	if !o.cfg.RequireHumanApproval {
		log.Printf("[orchestrator] auto-syncing workflow %d to platforms...", workflowID)
		links, err := o.SyncWorkflow(workflowID, false)
		if err != nil {
			log.Printf("[orchestrator] auto-sync failed: %v", err)
			o.publishError(workflowID, "Auto-sync failed: "+err.Error())
		} else {
			log.Printf("[orchestrator] auto-sync succeeded: %d platforms", len(links))
		}
	}
}

// RunFollowup executes a fast-track debate loop for additional user requests.
func (o *Orchestrator) RunFollowup(workflowID int, request string) {
	log.Printf("[orchestrator] starting followup for workflow %d", workflowID)

	if err := o.wfRepo.UpdateStatus(workflowID, "processing"); err != nil {
		log.Printf("[orchestrator] warn: set processing: %v", err)
	}
	o.publishStatus(workflowID, "processing")

	// Save the user's message to the debate timeline
	debRepo := repository.NewDebateRepo(o.db)
	id, err := debRepo.SaveDebate(workflowID, "user", "User", "client", request, 99) // using a high round number for followup start
	if err == nil {
		o.sseBus.Publish(workflowID, models.SSEEvent{Type: "debate_message", Data: models.DebateMessage{
			ID: id, WorkflowID: workflowID, AgentID: "user", AgentName: "User", Role: "client", Message: request, Round: 99,
		}})
	}

	if err := o.runFollowupDebate(workflowID, request); err != nil {
		log.Printf("[orchestrator] workflow %d followup failed: %v", workflowID, err)
		_ = o.wfRepo.UpdateStatus(workflowID, "failed")
		o.publishStatus(workflowID, "failed")
		o.publishError(workflowID, err.Error())
		return
	}

	_ = o.wfRepo.UpdateStatus(workflowID, "completed")
	o.publishStatus(workflowID, "completed")
	log.Printf("[orchestrator] workflow %d followup completed", workflowID)

	if !o.cfg.RequireHumanApproval {
		log.Printf("[orchestrator] auto-syncing workflow %d followup...", workflowID)
		links, err := o.SyncWorkflow(workflowID, true)
		if err != nil {
			log.Printf("[orchestrator] auto-sync followup failed: %v", err)
			o.publishError(workflowID, "Auto-sync failed: "+err.Error())
		} else {
			log.Printf("[orchestrator] auto-sync followup succeeded: %d platforms", len(links))
		}
	}
}

// runDebate contains the full 3-round debate logic.
func (o *Orchestrator) runDebate(workflowID int, request string) error {
	coordinator, err := agent.LoadAgent("coordinator", o.db, o.sseBus, o.cfg)
	if err != nil {
		return fmt.Errorf("load coordinator: %w", err)
	}
	po, err := agent.LoadAgent("po", o.db, o.sseBus, o.cfg)
	if err != nil {
		return fmt.Errorf("load po: %w", err)
	}
	techLead, err := agent.LoadAgent("techlead", o.db, o.sseBus, o.cfg)
	if err != nil {
		return fmt.Errorf("load techlead: %w", err)
	}
	qa, err := agent.LoadAgent("qa", o.db, o.sseBus, o.cfg)
	if err != nil {
		return fmt.Errorf("load qa: %w", err)
	}
	dev, err := agent.LoadAgent("dev-backend", o.db, o.sseBus, o.cfg)
	if err != nil {
		return fmt.Errorf("load dev-backend: %w", err)
	}
	cto, err := agent.LoadAgent("cto", o.db, o.sseBus, o.cfg)
	if err != nil {
		return fmt.Errorf("load cto: %w", err)
	}
	_ = cto
	cio, err := agent.LoadAgent("cio", o.db, o.sseBus, o.cfg)
	if err != nil {
		return fmt.Errorf("load cio: %w", err)
	}

	// ─── Round 1 — Discovery ─────────────────────────────────────────────────
	log.Printf("[orchestrator] round 1: discovery")
	_ = o.wfRepo.UpdateStatus(workflowID, "round_1_discovery")
	o.publishStatus(workflowID, "round_1_discovery")

	// Step 1: Coordinator introduces the project.
	coordIntro, err := coordinator.Call(workflowID,
		fmt.Sprintf("A new software project request has arrived. Please introduce it to the team and frame the problem clearly.\n\n## User Request:\n%s", request),
		1,
	)
	if err != nil {
		return fmt.Errorf("round1 coordinator intro: %w", err)
	}

	// Step 1.2: CIO Strategic Scoping & Bench Agent Activation
	o.PublishSysLog(workflowID, "[CIO] Auditing project scale and compliance to determine active resource allocations...")
	cioPrompt := fmt.Sprintf(`Analyze the following software project request and the Coordinator's introduction. Assess the project scale (Small, Medium, Large) and determine if we need to activate specialized agents from the bench:
1. **QA Engineer ('qa')**: Act if there are backend integrations, API endpoints, test cases, or high reliability needs.
2. **Frontend Developer ('dev-fe')**: Act if there is a User Interface, web views, Vue/React component specs, or client-side UI pages requested.

Return ONLY a valid JSON object in this EXACT format (no markdown, no explanations outside the JSON):
{
  "project_scale": "Small" | "Medium" | "Large",
  "activate_qa": true | false,
  "activate_frontend": true | false,
  "justification": "Detailed explanation of why these agents are activated or kept on bench."
}

## Request:
%s

## Coordinator Intro:
%s`, request, coordIntro)

	cioResp, err := cio.Call(workflowID, cioPrompt, 1)
	if err == nil {
		cioResp = strings.TrimSpace(cioResp)
		cStart := strings.Index(cioResp, "{")
		cEnd := strings.LastIndex(cioResp, "}")
		if cStart != -1 && cEnd != -1 && cEnd >= cStart {
			cioResp = cioResp[cStart : cEnd+1]
		}
		var payload struct {
			ProjectScale     string `json:"project_scale"`
			ActivateQA       bool   `json:"activate_qa"`
			ActivateFrontend bool   `json:"activate_frontend"`
			Justification    string `json:"justification"`
		}
		var parseErr error
		if strings.Contains(cioResp, "{") {
			var unmarshalErr error
			unmarshalErr = json.Unmarshal([]byte(cioResp), &payload)
			if unmarshalErr == nil {
				o.PublishSysLog(workflowID, "[CIO Assessment] Project Scale: %s. Justification: %s", payload.ProjectScale, payload.Justification)
				if payload.ActivateQA {
					_, _ = o.db.Exec("UPDATE agents SET active = true WHERE id = 'qa'")
					o.PublishSysLog(workflowID, "[CIO/CTO] Scope requires testing compliance. Activating QA Engineer agent!")
					qa.Active = true
				} else {
					_, _ = o.db.Exec("UPDATE agents SET active = false WHERE id = 'qa'")
					o.PublishSysLog(workflowID, "[CIO/CTO] Small/internal project. QA Engineer remains on bench.")
					qa.Active = false
				}
				if payload.ActivateFrontend {
					_, _ = o.db.Exec("UPDATE agents SET active = true WHERE id = 'dev-fe'")
					o.PublishSysLog(workflowID, "[CIO/CTO] Project requires User Interface. Activating Frontend Developer agent!")
				} else {
					_, _ = o.db.Exec("UPDATE agents SET active = false WHERE id = 'dev-fe'")
					o.PublishSysLog(workflowID, "[CIO/CTO] Backend/API-only project. Frontend Developer remains on bench.")
				}
			} else {
				parseErr = unmarshalErr
			}
		}
		if parseErr != nil {
			log.Printf("[orchestrator] Warn: failed to parse CIO JSON: %v", parseErr)
		}
	}

	// Step 2: PO writes the PRD.
	poPRD, err := po.Call(workflowID,
		fmt.Sprintf("Please write a detailed Product Requirements Document (PRD) for the following request.\nInclude: user stories (As a / I want / So that), acceptance criteria (Given/When/Then), edge cases, non-functional requirements.\n\n## Coordinator Introduction:\n%s\n\n## Original Request:\n%s",
			coordIntro, request),
		1,
	)
	if err != nil {
		return fmt.Errorf("round1 PO PRD: %w", err)
	}

	// Step 3: TechLead reviews PO output and raises concerns.
	techConcerns, err := techLead.Call(workflowID,
		fmt.Sprintf("Review the following PRD and raise any technical concerns, infeasible requirements, or ambiguities that must be resolved before architecture can begin.\n\n## PRD:\n%s", poPRD),
		1,
	)
	if err != nil {
		return fmt.Errorf("round1 TechLead review: %w", err)
	}

	// Step 4: PO responds to TechLead concerns and updates PRD.
	poFinalPRD, err := po.Call(workflowID,
		fmt.Sprintf("The Tech Lead has raised the following concerns about your PRD. Address each concern specifically and provide an updated, final PRD.\n\n## Tech Lead Concerns:\n%s\n\n## Your Original PRD:\n%s",
			techConcerns, poPRD),
		1,
	)
	if err != nil {
		return fmt.Errorf("round1 PO revision: %w", err)
	}

	// Step 5: Save PO ticket tasks to DB.
	if err := o.taskRepo.SaveTask(workflowID,
		"Product Requirements Document",
		"Final PRD including user stories, acceptance criteria, edge cases, and NFRs",
		"prd", "po", "completed", poFinalPRD,
	); err != nil {
		log.Printf("[orchestrator] warn: save PO task: %v", err)
	}

	// ─── Round 2 — Design & Review ───────────────────────────────────────────
	log.Printf("[orchestrator] round 2: design & review")
	_ = o.wfRepo.UpdateStatus(workflowID, "round_2_design")
	o.publishStatus(workflowID, "round_2_design")

	// Step 1: TechLead writes full architecture.
	techArch, err := techLead.Call(workflowID,
		fmt.Sprintf("Based on the final PRD below, design a complete system architecture. Include: component diagram, database schema (tables + columns + relationships), API contract (endpoints, request/response shapes), technology stack decisions, and deployment topology.\n\n## Final PRD:\n%s",
			poFinalPRD),
		2,
	)
	if err != nil {
		return fmt.Errorf("round2 TechLead architecture: %w", err)
	}

	// Step 2: QA reviews TechLead design for testability.
	var qaConcerns string
	if qa.Active {
		qaConcerns, err = qa.Call(workflowID,
			fmt.Sprintf("Review the following system architecture from a QA/testability perspective. Identify: hard-to-test components, missing observability hooks, suggested mock strategies, and integration test checkpoints.\n\n## Architecture:\n%s",
				techArch),
			2,
		)
		if err != nil {
			return fmt.Errorf("round2 QA review: %w", err)
		}
	} else {
		o.PublishSysLog(workflowID, "[QA Agent] (On Bench) Skipping QA architectural review.")
		qaConcerns = "QA is not active for this project, skipping testing review."
	}

	// Step 3: Dev reviews TechLead design for feasibility.
	devConcerns, err := dev.Call(workflowID,
		fmt.Sprintf("Review the following system architecture from an implementation feasibility perspective. Identify: over-engineered areas, missing implementation details, complexity concerns, and propose alternative approaches where appropriate with brief code examples.\n\n## Architecture:\n%s",
			techArch),
		2,
	)
	if err != nil {
		return fmt.Errorf("round2 Dev review: %w", err)
	}

	// Step 4: TechLead responds to QA and Dev concerns.
	techFinalArch, err := techLead.Call(workflowID,
		fmt.Sprintf("Address the concerns raised by QA and Dev and produce a final, revised architecture document.\n\n## QA Concerns:\n%s\n\n## Dev Concerns:\n%s\n\n## Original Architecture:\n%s",
			qaConcerns, devConcerns, techArch),
		2,
	)
	if err != nil {
		return fmt.Errorf("round2 TechLead final arch: %w", err)
	}

	// Step 5: Save TechLead architecture task.
	if err := o.taskRepo.SaveTask(workflowID,
		"System Architecture Document",
		"Final architecture including schema, API contracts, and component design",
		"architecture", "techlead", "completed", techFinalArch,
	); err != nil {
		log.Printf("[orchestrator] warn: save TechLead task: %v", err)
	}

	// ─── Round 3 — Implementation ────────────────────────────────────────────
	log.Printf("[orchestrator] round 3: implementation")
	_ = o.wfRepo.UpdateStatus(workflowID, "round_3_implementation")
	o.publishStatus(workflowID, "round_3_implementation")

	// Step 1: Dev writes full TypeScript implementation.
	devCode, err := dev.Call(workflowID,
		fmt.Sprintf("Write the complete, production-ready TypeScript implementation for this feature. Follow the final architecture strictly. Include: all files, imports, types, error handling, and comments. Do not use placeholders.\n\n## Final Architecture:\n%s\n\n## Final PRD:\n%s",
			techFinalArch, poFinalPRD),
		3,
	)
	if err != nil {
		return fmt.Errorf("round3 Dev implementation: %w", err)
	}

	// Step 2: QA writes full test suite.
	var qaTests string
	if qa.Active {
		qaTests, err = qa.Call(workflowID,
			fmt.Sprintf("Write a complete test suite (unit + integration tests) for the implementation below. Include: smoke tests, positive path, negative path, edge cases, and mock strategies. Use Jest/Vitest.\n\n## Implementation:\n%s\n\n## Final Architecture:\n%s",
				devCode, techFinalArch),
			3,
		)
		if err != nil {
			return fmt.Errorf("round3 QA tests: %w", err)
		}
		if err := o.taskRepo.SaveTask(workflowID,
			"Test Suite",
			"Unit and integration tests",
			"tests", "qa", "completed", qaTests,
		); err != nil {
			log.Printf("[orchestrator] warn: save qa task: %v", err)
		}
	} else {
		o.PublishSysLog(workflowID, "[QA Agent] (On Bench) Skipping test suite generation task.")
		qaTests = "QA was not activated for this small project."
	}

	// Step 3: Coordinator writes final sign-off.
	coordSignOff, err := coordinator.Call(workflowID,
		fmt.Sprintf("All agents have completed their work. Write a final sign-off summary covering: what was built, key decisions made, risks identified, and next steps for deployment.\n\n## PRD:\n%s\n\n## Architecture:\n%s\n\n## Implementation Summary:\n(see implementation task)\n\n## Test Plan:\n(see QA task)",
			poFinalPRD, techFinalArch),
		3,
	)
	if err != nil {
		return fmt.Errorf("round3 Coordinator sign-off: %w", err)
	}

	if err := o.taskRepo.SaveTask(workflowID,
		"Sign-off Summary",
		"Final sign-off and deployment steps",
		"signoff", "coordinator", "completed", coordSignOff,
	); err != nil {
		log.Printf("[orchestrator] warn: save coord task: %v", err)
	}

	return nil
}

// runFollowupDebate is a lighter 3-round debate designed to update existing tasks.
func (o *Orchestrator) runFollowupDebate(workflowID int, request string) error {
	coordinator, err := agent.LoadAgent("coordinator", o.db, o.sseBus, o.cfg)
	if err != nil {
		return err
	}
	po, err := agent.LoadAgent("po", o.db, o.sseBus, o.cfg)
	if err != nil {
		return err
	}
	techLead, err := agent.LoadAgent("techlead", o.db, o.sseBus, o.cfg)
	if err != nil {
		return err
	}
	qa, err := agent.LoadAgent("qa", o.db, o.sseBus, o.cfg)
	if err != nil {
		return err
	}
	dev, err := agent.LoadAgent("dev-backend", o.db, o.sseBus, o.cfg)
	if err != nil {
		return err
	}

	round := 100 // Use round 100+ for followups to distinguish in timeline

	// Round 100: Coord & PO update PRD
	_ = o.wfRepo.UpdateStatus(workflowID, "followup_discovery")
	o.publishStatus(workflowID, "followup_discovery")

	coordIntro, err := coordinator.Call(workflowID,
		fmt.Sprintf("The user has requested a change or addition to the project:\n\n%s\n\nPlease introduce this change to the team and explain how it impacts our current direction.", request), round)
	if err != nil {
		return err
	}

	poUpdatedPRD, err := po.Call(workflowID,
		fmt.Sprintf("Based on the Coordinator's introduction of the user's new request:\n%s\n\nPlease update the Product Requirements Document (PRD). Highlight the new user stories and changes.", coordIntro), round)
	if err != nil {
		return err
	}

	_ = o.taskRepo.SaveTask(workflowID, "Updated PRD", "Revised PRD with new changes", "prd", "po", "completed", poUpdatedPRD)

	// Round 101: TechLead updates Architecture
	round++
	_ = o.wfRepo.UpdateStatus(workflowID, "followup_design")
	o.publishStatus(workflowID, "followup_design")

	techUpdatedArch, err := techLead.Call(workflowID,
		fmt.Sprintf("Based on the updated PRD:\n%s\n\nPlease update the system architecture, schema, or API contracts to support the new requirements.", poUpdatedPRD), round)
	if err != nil {
		return err
	}

	_ = o.taskRepo.SaveTask(workflowID, "Updated Architecture", "Revised architecture with new changes", "architecture", "techlead", "completed", techUpdatedArch)

	// Round 102: Dev & QA update code and tests
	round++
	_ = o.wfRepo.UpdateStatus(workflowID, "followup_implementation")
	o.publishStatus(workflowID, "followup_implementation")

	devUpdatedCode, err := dev.Call(workflowID,
		fmt.Sprintf("Based on the updated Architecture:\n%s\n\nPlease write or update the implementation code necessary for these changes.", techUpdatedArch), round)
	if err != nil {
		return err
	}

	var qaUpdatedTests string
	if qa.Active {
		qaUpdatedTests, err = qa.Call(workflowID,
			fmt.Sprintf("Based on the updated Implementation:\n%s\n\nPlease write or update the test suite for these changes.", devUpdatedCode), round)
		if err != nil {
			return err
		}
		_ = o.taskRepo.SaveTask(workflowID, "Updated Tests", "Revised test suite", "tests", "qa", "completed", qaUpdatedTests)
	} else {
		qaUpdatedTests = "QA remains on bench for this followup."
	}

	_ = o.taskRepo.SaveTask(workflowID, "Updated Implementation", "Revised implementation code", "implementation", "dev-backend", "completed", devUpdatedCode)

	return nil
}

// publishStatus emits a status SSE event so the frontend can update its UI.
func (o *Orchestrator) publishStatus(workflowID int, status string) {
	o.sseBus.Publish(workflowID, models.SSEEvent{
		Type: "status",
		Data: map[string]interface{}{
			"workflow_id": workflowID,
			"status":      status,
		},
	})
}

// PublishSysLog emits a system log event and also prints it to the terminal.
func (o *Orchestrator) PublishSysLog(workflowID int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[workflow %d] %s", workflowID, msg)
	_ = o.wfRepo.SaveSystemLog(workflowID, msg)
	o.sseBus.Publish(workflowID, models.SSEEvent{
		Type: "sys_log",
		Data: map[string]interface{}{
			"workflow_id": workflowID,
			"message":     msg,
		},
	})
}

// publishError emits an error SSE event.
func (o *Orchestrator) publishError(workflowID int, msg string) {
	o.sseBus.Publish(workflowID, models.SSEEvent{
		Type: "error",
		Data: map[string]interface{}{
			"workflow_id": workflowID,
			"message":     msg,
		},
	})
}

// SyncWorkflow extracts tasks from debate history and syncs them to external platforms.
func (o *Orchestrator) SyncWorkflow(workflowID int, isFollowup bool) ([]map[string]string, error) {
	_, err := o.wfRepo.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("sync: get workflow: %w", err)
	}

	debRepo := repository.NewDebateRepo(o.db)
	debates, err := debRepo.GetDebates(workflowID)
	if err != nil {
		return nil, fmt.Errorf("sync: get debates: %w", err)
	}

	var ctxStr string
	for _, d := range debates {
		ctxStr += fmt.Sprintf("[%s]: %s\n\n", d.AgentName, d.Message)
	}

	var prompt string
	if isFollowup {
		prompt = fmt.Sprintf(`Based on the following debate history, extract ONLY the newly added development subtasks related to the user's latest request. Do NOT include tasks that were already discussed or extracted in previous phases. Return a strict JSON object format.
DO NOT use markdown blocks, DO NOT add explanation. Return ONLY raw JSON object.
Format:
{
  "project_name": "A short descriptive name (max 30 chars)",
  "project_key": "SHORTKEY",
  "tasks": [{"title": "User Story title", "description": "Detailed description...", "subtasks": [{"title": "Subtask title", "description": "..."}]}]
}

## Debate History:
%s`, ctxStr)
	} else {
		prompt = fmt.Sprintf(`Based on the following debate history, extract the final list of development subtasks into a strict JSON object format.
DO NOT use markdown blocks, DO NOT add explanation. Return ONLY raw JSON object.
Format:
{
  "project_name": "A short descriptive name (max 30 chars)",
  "project_key": "SHORTKEY",
  "tasks": [{"title": "User Story title", "description": "Detailed description...", "subtasks": [{"title": "Subtask title", "description": "..."}]}]
}

## Debate History:
%s`, ctxStr)
	}

	coord, err := agent.LoadAgent("coordinator", o.db, o.sseBus, o.cfg)
	if err != nil {
		return nil, err
	}

	jsonResp, err := coord.Call(workflowID, prompt, 999)
	if err != nil {
		return nil, fmt.Errorf("sync: extract tasks: %w", err)
	}

	jsonResp = strings.TrimPrefix(jsonResp, "```json\n")
	jsonResp = strings.TrimPrefix(jsonResp, "```json")
	jsonResp = strings.TrimPrefix(jsonResp, "```")
	jsonResp = strings.TrimSuffix(jsonResp, "\n```")
	jsonResp = strings.TrimSpace(jsonResp)

	// Extract just the JSON object to ignore CLI warnings or trailing text
	startIdx := strings.Index(jsonResp, "{")
	endIdx := strings.LastIndex(jsonResp, "}")
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		jsonResp = jsonResp[startIdx : endIdx+1]
	}

	var payload integration.SyncPayload
	if err := json.Unmarshal([]byte(jsonResp), &payload); err != nil {
		return nil, fmt.Errorf("sync: failed to parse JSON: %w\nRaw: %s", err, jsonResp)
	}

	projName := fmt.Sprintf("%s", payload.ProjectName)
	if projName == "" {
		projName = "Workflow"
	}
	// Append workflow ID for uniqueness
	// Do not append workflow ID so we can reuse existing projects.
	projKey := strings.ToUpper(payload.ProjectKey)
	projKey = regexp.MustCompile(`[^A-Z]`).ReplaceAllString(projKey, "")
	if len(projKey) > 6 {
		projKey = projKey[:6]
	}
	if projKey == "" {
		projKey = "WF"
	}
	// (No longer appending workflow ID to projKey)
	repoName := strings.ToLower(payload.ProjectName)
	repoName = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(repoName, "-")
	repoName = strings.Trim(repoName, "-")
	if repoName == "" {
		repoName = "workflow"
	}
	// (No longer appending workflow ID to repoName)

	tasks := payload.Tasks
	var links []map[string]string

	// 1. GitLab Sync
	if o.cfg.GitlabToken != "" {
		glClient := integration.NewGitlabClient(o.cfg.GitlabBaseURL, o.cfg.GitlabToken)
		url, err := glClient.SyncProject(projName, tasks)
		if err == nil {
			links = append(links, map[string]string{"name": "GitLab", "url": url})
		} else {
			log.Printf("[sync] gitlab error: %v", err)
		}
	}

	// 1.5 GitHub Sync
	if o.cfg.GithubToken != "" {
		ghClient := integration.NewGithubClient(o.cfg.GithubToken)
		url, err := ghClient.SyncRepository(repoName)
		if err == nil {
			links = append(links, map[string]string{"name": "GitHub", "url": url})
		} else {
			log.Printf("[sync] github error: %v", err)
		}
	}

	// 2. Jira Sync
	if o.cfg.JiraToken != "" {
		jiraClient := integration.NewJiraClient(o.cfg.JiraBaseURL, o.cfg.JiraUser, o.cfg.JiraToken)

		// Create Project dynamically
		if err := jiraClient.CreateProject(projName, projKey); err != nil {
			log.Printf("[sync] jira project creation error: %v", err)
		} else {
			url, err := jiraClient.SyncIssues(projName, projKey, tasks)
			if err == nil {
				links = append(links, map[string]string{"name": "Jira", "url": url})
			} else {
				log.Printf("[sync] jira error: %v", err)
			}
		}
	}

	// 3. Confluence Sync
	if o.cfg.JiraToken != "" {
		confClient := integration.NewConfluenceClient(o.cfg.JiraBaseURL, o.cfg.JiraUser, o.cfg.JiraToken)

		// Create Space dynamically
		if err := confClient.CreateSpace(projName, projKey); err != nil {
			log.Printf("[sync] confluence space creation error: %v", err)
		} else {
			url, err := confClient.SyncPage(projKey, fmt.Sprintf("PRD - %s", projName), ctxStr)
			if err == nil {
				links = append(links, map[string]string{"name": "Confluence (PRD)", "url": url})
			} else {
				log.Printf("[sync] confluence prd error: %v", err)
			}

			// Extract TechLead architecture output for DAB page
			var techLeadContent string
			for _, d := range debates {
				if strings.Contains(strings.ToLower(d.AgentName), "tech") || d.AgentID == "techLead" {
					techLeadContent = d.Message
				}
			}

			if techLeadContent != "" {
				dabUrl, err := confClient.SyncPage(projKey, fmt.Sprintf("DAB - %s", projName), techLeadContent)
				if err == nil {
					links = append(links, map[string]string{"name": "Confluence (DAB)", "url": dabUrl})
				} else {
					log.Printf("[sync] confluence dab error: %v", err)
				}
			}
		}
	}

	_ = o.wfRepo.UpdateStatus(workflowID, "synced")
	o.publishStatus(workflowID, "synced")

	// Publish a system message to the frontend so the user can click the links
	if len(links) > 0 {
		var msgLines []string
		msgLines = append(msgLines, "🎉 **Đồng bộ thành công!** Các hệ thống đã được thiết lập:")
		for _, l := range links {
			msgLines = append(msgLines, fmt.Sprintf("- **%s**: [%s](%s)", l["name"], l["url"], l["url"]))
		}
		sysMsg := strings.Join(msgLines, "\n")

		debRepo := repository.NewDebateRepo(o.db)
		id, err := debRepo.SaveDebate(workflowID, "system", "System", "system", sysMsg, 999)
		if err == nil {
			o.sseBus.Publish(workflowID, models.SSEEvent{Type: "debate_message", Data: models.DebateMessage{
				ID: id, WorkflowID: workflowID, AgentID: "system", AgentName: "System", Role: "system", Message: sysMsg, Round: 999,
			}})
		}
	}

	// Extract repoURL
	var repoURL string
	for _, l := range links {
		if l["name"] == "GitHub" {
			repoURL = l["url"]
			break
		}
	}
	if repoURL == "" {
		// Fallback just in case
		repoURL = fmt.Sprintf("https://github.com/truonglv95/%s", repoName)
	}

	// Persist the sync config so it can be resumed later
	_ = o.wfRepo.UpdateWorkflowSyncDetails(workflowID, repoName, projKey, repoURL)

	// Trigger Autonomous Worker (max 5 subtasks per run, pushing to GitHub and Jira)
	o.StartAutonomousWorker(workflowID, projKey, repoName, repoURL)

	return links, nil
}
