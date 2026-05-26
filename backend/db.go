package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Default development fallback
		dbURL = "postgres://postgres:postgres@localhost:5432/ai_agent?sslmode=disable"
	}

	var err error
	// Retry connection if PostgreSQL is still starting up (e.g. in Docker Compose)
	for i := 0; i < 10; i++ {
		DB, err = sql.Open("postgres", dbURL)
		if err == nil {
			err = DB.Ping()
			if err == nil {
				break
			}
		}
		log.Printf("Awaiting PostgreSQL connection (attempt %d/10)... error: %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to PostgreSQL: %v", err)
	}

	log.Println("Successfully connected to PostgreSQL")

	RunMigrations()
}

func RunMigrations() {
	// Create tables if they do not exist
	queries := []string{
		`CREATE TABLE IF NOT EXISTS workflows (
			id SERIAL PRIMARY KEY,
			request TEXT NOT NULL,
			status VARCHAR(50) NOT NULL,
			repo_name VARCHAR(255) DEFAULT '',
			jira_project_key VARCHAR(50) DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS agents (
			id VARCHAR(50) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			role VARCHAR(100) NOT NULL,
			system_prompt TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS debates (
			id SERIAL PRIMARY KEY,
			workflow_id INT REFERENCES workflows(id) ON DELETE CASCADE,
			agent_id VARCHAR(50) NOT NULL,
			agent_name VARCHAR(100) NOT NULL,
			role VARCHAR(100) NOT NULL,
			message TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id SERIAL PRIMARY KEY,
			workflow_id INT REFERENCES workflows(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			description TEXT NOT NULL,
			assigned_to VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL,
			output_content TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			log.Fatalf("Migration failed on query: %s \nError: %v", query, err)
		}
	}

	log.Println("Database migrations applied successfully")
	SeedAgents()
}

func SeedAgents() {
	agents := []struct {
		ID           string
		Name         string
		Role         string
		SystemPrompt string
	}{
		{
			ID:   "coordinator",
			Name: "Coordinator Agent",
			Role: "Coordinator & Orchestrator",
			SystemPrompt: `You are the Coordinator Agent. Your job is to orchestrate the software development lifecycle when a user submits a feature request.
You decide the flow and manage the transition between agents: Product Owner (PO), Tech Lead, QA, and Developer.
Act as the facilitator, direct agent tasks, evaluate if their outputs align, and present a final integrated report to the user.
Keep discussions focused and ensure quality.`,
		},
		{
			ID:   "po",
			Name: "Product Owner Agent",
			Role: "Product Owner & Requirements Analyst",
			SystemPrompt: `You are the Product Owner (PO) Agent. Your responsibility is to analyze user requests, refine requirements, and draft user stories.
You create clear tickets/requirements detailing:
1. Feature Description
2. User Acceptance Criteria (Gherkin style if applicable)
3. Priority and Edge Cases to consider.
Write detailed, structured Product Requirement Documents (PRD) or User Stories.`,
		},
		{
			ID:   "techlead",
			Name: "Tech Lead Agent",
			Role: "Technical Lead & Software Architect",
			SystemPrompt: `You are the Tech Lead Agent (Software Architect). Your job is to analyze the Product Owner's requirements and define a professional Design Architecture Blueprint (DAB).
Your outputs MUST include:
1. System Architecture & Component Design.
2. A Markdown Table listing all required APIs (Columns: Endpoint, Method, Request Payload, Response Payload, Description).
3. A detailed UML Sequence Diagram (using Mermaid syntax) illustrating the full user flow from start to finish.
4. Data models, database schema updates, and technical implementation strategy.
Act as a senior, professional Software Architect. Ensure the UML diagram clearly covers frontend, backend, and database interactions.`,
		},
		{
			ID:   "qa",
			Name: "QA Agent",
			Role: "Quality Assurance Engineer",
			SystemPrompt: `You are the QA Agent. Your task is to write detailed Test Plans and Test Cases based on the PO's acceptance criteria and Tech Lead's design.
Define:
1. Smoke tests and positive scenarios
2. Negative scenarios and boundary cases
3. Unit test recommendations and integration test checkpoints.
Ensure everything matches the PO acceptance criteria.`,
		},
		{
			ID:   "dev-fe",
			Name: "Frontend Developer Agent",
			Role: "Frontend Software Engineer",
			SystemPrompt: `You are the Frontend Developer Agent. Your job is to write the actual frontend code (Vue/React, CSS, Responsive) based on the Tech Lead's design and QA's test plan.
You must output ONLY valid JSON containing a list of files to create or modify.`,
		},
		{
			ID:   "dev-backend",
			Name: "Backend Developer Agent",
			Role: "Backend Software Engineer",
			SystemPrompt: `You are the Backend Developer Agent. Your job is to write the backend code (Go/NodeJS, REST API, Database Logic) based on the Tech Lead's design and QA's test plan.
You must output ONLY valid JSON containing a list of files to create or modify.`,
		},
		{
			ID:   "dev",
			Name: "Developer Agent",
			Role: "Full Stack Software Developer",
			SystemPrompt: `You are the Developer Agent. Your job is to write the actual TypeScript code and mockup unit tests based on the Tech Lead's design and QA's test plan.
Provide complete, functional code modules with:
1. Strong-typed interfaces (TypeScript)
2. Complete implementations
3. Mocked unit test runs showcasing passing/failing assertions.
Do not use placeholders; output complete code files.`,
		},
	}

	for _, a := range agents {
		query := `INSERT INTO agents (id, name, role, system_prompt)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE 
			SET name = EXCLUDED.name, role = EXCLUDED.role, system_prompt = EXCLUDED.system_prompt;`
		if _, err := DB.Exec(query, a.ID, a.Name, a.Role, a.SystemPrompt); err != nil {
			log.Fatalf("Failed to seed agent %s: %v", a.ID, err)
		}
	}

	log.Println("Default agents seeded successfully")
}
