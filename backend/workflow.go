package main

import (
	"fmt"
	"log"
	"regexp"
)

type Workflow struct {
	ID        int    `json:"id"`
	Request   string `json:"request"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type DebateMessage struct {
	ID         int    `json:"id"`
	WorkflowID int    `json:"workflow_id"`
	AgentID    string `json:"agent_id"`
	AgentName  string `json:"agent_name"`
	Role       string `json:"role"`
	Message    string `json:"message"`
	CreatedAt  string `json:"created_at"`
}

type TaskItem struct {
	ID            int    `json:"id"`
	WorkflowID    int    `json:"workflow_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	AssignedTo    string `json:"assigned_to"`
	Status        string `json:"status"`
	OutputContent string `json:"output_content,omitempty"`
	CreatedAt     string `json:"created_at"`
}

// StartWorkflow runs the orchestrator chain in the background
func StartWorkflow(workflowID int, userRequest string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Workflow %d panicked: %v", workflowID, r)
				updateWorkflowStatus(workflowID, "FAILED")
			}
		}()

		log.Printf("Starting workflow %d for request: %s", workflowID, userRequest)
		updateWorkflowStatus(workflowID, "PROCESSING")

		// 1. Load agents
		coord, err := LoadAgent("coordinator")
		if err != nil {
			log.Printf("Error loading coordinator agent: %v", err)
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}

		po, err := LoadAgent("po")
		if err != nil {
			log.Printf("Error loading PO agent: %v", err)
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}

		techlead, err := LoadAgent("techlead")
		if err != nil {
			log.Printf("Error loading Tech Lead agent: %v", err)
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}

		qa, err := LoadAgent("qa")
		if err != nil {
			log.Printf("Error loading QA agent: %v", err)
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}

		dev, err := LoadAgent("dev")
		if err != nil {
			log.Printf("Error loading Dev agent: %v", err)
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}

		// Keep track of all messages for agent context
		var history []OpenAIMessage

		// --- STEP 1: Coordinator introduction ---
		coordUserPrompt := fmt.Sprintf("A new feature request has been submitted: '%s'. Introduce the project, outline the agent workflow, and instruct the Product Owner to start analyzing the requirements.", userRequest)
		coordIntro, err := CallLLM(coord.SystemPrompt, coordUserPrompt, history)
		if err != nil {
			saveDebate(workflowID, "coordinator", coord.Name, coord.Role, fmt.Sprintf("Error calling LLM: %v", err))
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}
		saveDebate(workflowID, "coordinator", coord.Name, coord.Role, coordIntro)
		history = append(history, OpenAIMessage{Role: "user", Content: coordUserPrompt}, OpenAIMessage{Role: "assistant", Content: coordIntro})

		// --- STEP 2: Product Owner (PO) PRD & Ticket Generation ---
		poUserPrompt := fmt.Sprintf("The Coordinator has initialized the project for: '%s'. Create a Product Requirement Document (PRD) / User Story specification. Then, define at least 2 distinct dev tasks or tickets with clear descriptions and Acceptance Criteria in standard format.", userRequest)
		poOutput, err := CallLLM(po.SystemPrompt, poUserPrompt, history)
		if err != nil {
			saveDebate(workflowID, "po", po.Name, po.Role, fmt.Sprintf("Error calling LLM: %v", err))
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}
		saveDebate(workflowID, "po", po.Name, po.Role, poOutput)
		history = append(history, OpenAIMessage{Role: "user", Content: poUserPrompt}, OpenAIMessage{Role: "assistant", Content: poOutput})

		// Parse PO output and create tickets
		tickets := parseTicketsFromOutput(poOutput)
		if len(tickets) == 0 {
			// Fallback: create default tickets if parsing failed
			tickets = []TaskItem{
				{Title: "Implement Core Feature Logic", Description: "Build the primary functional logic for " + userRequest, AssignedTo: "dev", Status: "TODO"},
				{Title: "Integrate Database and API Endpoints", Description: "Set up the schemas and connect API router for " + userRequest, AssignedTo: "dev", Status: "TODO"},
			}
		}
		for _, ticket := range tickets {
			saveTask(workflowID, ticket.Title, ticket.Description, "dev", "TODO", "")
		}

		// --- STEP 3: Tech Lead Technical Design ---
		techLeadPrompt := "Read the Product Owner's requirements and tickets above. Design the system architecture, database schema, and outline technical packages/modules needed for the project. Explain how you will structure the TypeScript components."
		techLeadOutput, err := CallLLM(techlead.SystemPrompt, techLeadPrompt, history)
		if err != nil {
			saveDebate(workflowID, "techlead", techlead.Name, techlead.Role, fmt.Sprintf("Error calling LLM: %v", err))
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}
		saveDebate(workflowID, "techlead", techlead.Name, techlead.Role, techLeadOutput)
		history = append(history, OpenAIMessage{Role: "user", Content: techLeadPrompt}, OpenAIMessage{Role: "assistant", Content: techLeadOutput})

		// Save technical design task
		saveTask(workflowID, "Technical Design Specification", "System Architecture & Database Schema Design", "techlead", "COMPLETED", techLeadOutput)

		// --- STEP 4: QA Test Plan & Scenarios ---
		qaPrompt := "Read the requirements and the Tech Lead's system design. Draft a detailed Test Plan including smoke test scenarios, positive/negative test cases, and edge cases to ensure quality."
		qaOutput, err := CallLLM(qa.SystemPrompt, qaPrompt, history)
		if err != nil {
			saveDebate(workflowID, "qa", qa.Name, qa.Role, fmt.Sprintf("Error calling LLM: %v", err))
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}
		saveDebate(workflowID, "qa", qa.Name, qa.Role, qaOutput)
		history = append(history, OpenAIMessage{Role: "user", Content: qaPrompt}, OpenAIMessage{Role: "assistant", Content: qaOutput})

		// Save test plan task
		saveTask(workflowID, "QA Test Case Specification", "Test Cases, Boundary Checks, and Unit Test Recommendations", "qa", "COMPLETED", qaOutput)

		// --- STEP 5: Developer Coding & Unit Testing ---
		devPrompt := "You are ready to write code. Create complete, valid TypeScript/ESM modules implementing the requested feature based on the technical design and QA test scenarios. Also write mocked unit tests and show their output."
		devOutput, err := CallLLM(dev.SystemPrompt, devPrompt, history)
		if err != nil {
			saveDebate(workflowID, "dev", dev.Name, dev.Role, fmt.Sprintf("Error calling LLM: %v", err))
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}
		saveDebate(workflowID, "dev", dev.Name, dev.Role, devOutput)
		history = append(history, OpenAIMessage{Role: "user", Content: devPrompt}, OpenAIMessage{Role: "assistant", Content: devOutput})

		// Update developer tasks to completed and save code output
		updateDevTasksWithCode(workflowID, devOutput)

		// --- STEP 6: Coordinator Project Sign-Off ---
		coordFinalPrompt := "Review the PRD, technical design, test plans, and generated code output. Write a final sign-off report outlining what was delivered, confirmation of QA tests passing, and next steps."
		coordFinalOutput, err := CallLLM(coord.SystemPrompt, coordFinalPrompt, history)
		if err != nil {
			saveDebate(workflowID, "coordinator", coord.Name, coord.Role, fmt.Sprintf("Error calling LLM: %v", err))
			updateWorkflowStatus(workflowID, "FAILED")
			return
		}
		saveDebate(workflowID, "coordinator", coord.Name, coord.Role, coordFinalOutput)

		// Complete workflow
		updateWorkflowStatus(workflowID, "COMPLETED")
		log.Printf("Workflow %d completed successfully!", workflowID)
	}()
}

// Helpers
func updateWorkflowStatus(id int, status string) {
	query := `UPDATE workflows SET status = $1 WHERE id = $2`
	if _, err := DB.Exec(query, status, id); err != nil {
		log.Printf("Error updating workflow status: %v", err)
	}
}

func saveDebate(workflowID int, agentID, agentName, role, message string) {
	query := `INSERT INTO debates (workflow_id, agent_id, agent_name, role, message) VALUES ($1, $2, $3, $4, $5)`
	if _, err := DB.Exec(query, workflowID, agentID, agentName, role, message); err != nil {
		log.Printf("Error saving debate: %v", err)
	}
}

func saveTask(workflowID int, title, description, assignedTo, status, outputContent string) {
	query := `INSERT INTO tasks (workflow_id, title, description, assigned_to, status, output_content) VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := DB.Exec(query, workflowID, title, description, assignedTo, status, outputContent); err != nil {
		log.Printf("Error saving task: %v", err)
	}
}

func updateDevTasksWithCode(workflowID int, codeOutput string) {
	// Mark all 'TODO' dev tasks in this workflow as completed and save the code output
	query := `UPDATE tasks SET status = 'COMPLETED', output_content = $1 WHERE workflow_id = $2 AND assigned_to = 'dev'`
	if _, err := DB.Exec(query, codeOutput, workflowID); err != nil {
		log.Printf("Error updating dev tasks: %v", err)
	}
}

// Parser helper to extract tickets/tasks from PO LLM response
func parseTicketsFromOutput(output string) []TaskItem {
	var tickets []TaskItem

	// Regular expression to look for numbered list or "Ticket: Title" / "Task: Title"
	re := regexp.MustCompile(`(?i)(?:ticket|task|story)\s*\d*[:#-]\s*([^\n]+)`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) > 1 {
			title := match[1]
			// Limit title length
			if len(title) > 255 {
				title = title[:252] + "..."
			}
			tickets = append(tickets, TaskItem{
				Title:       title,
				Description: "Acceptance criteria and requirements specified in PO logs.",
				AssignedTo:  "dev",
				Status:      "TODO",
			})
		}
	}

	// Limit to maximum 3 parsed tickets
	if len(tickets) > 3 {
		tickets = tickets[:3]
	}

	return tickets
}
