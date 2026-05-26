# AI Agent Workflow Platform

A multi-agent workflow orchestration system that integrates various LLMs (Google Gemini, OpenAI, etc.), Jira, Confluence, and GitHub. This system enables AI agents to autonomously manage, debate, review, and execute software development tasks.

## Project Structure

- `backend/`: Go-based orchestrator, API server, and database migrations. Manages the workflow state and agent communications.
- `frontend/`: Vue 3 + Vite UI for visualizing the virtual office, active agents, debates, and task execution progress.
- `runner/`: Go-based secure API wrapper executing the `agy` CLI to run AI agent steps in an isolated environment.

## Prerequisites

- **Go** (1.21+)
- **Node.js** (18+)
- **PostgreSQL** (via `docker-compose`)
- **agy CLI**: Must be installed globally or available in your system `PATH`.

## Getting Started

### 1. Database Setup
Start the PostgreSQL database using Docker:
```bash
docker-compose up -d
```

### 2. Backend Orchestrator Setup
Configure environment variables and start the Go orchestrator server.

```bash
cd backend
cp .env.example .env
# Important: Edit .env and fill in your LLM API keys (e.g. GEMINI_API_KEY), 
# Jira/GitHub integration tokens, and configure ROOT_WORKSPACE.
go run ./cmd/server
```
*Note: The backend defaults to running on port 8080.*

### 3. Agent Runner Setup
The runner provides a secure API for the backend to execute AI agent commands. It needs to run concurrently with the backend.

```bash
cd runner
# Set the admin credentials that the backend will use to authenticate
export RUNNER_ADMIN_USER=admin
export RUNNER_ADMIN_PASS=123456
go run .
```
*Note: The runner defaults to running on port 8081.*

### 4. Frontend Web UI Setup
Start the Vue development server to interact with the platform.

```bash
cd frontend
npm install
npm run dev
```

The Web UI will be accessible at `http://localhost:5173`.

## Architecture & Workflows

1. **Dynamic Scoping (CIO/CTO)**: The Orchestrator queries the CIO/CTO agents to determine the project size and automatically toggles the necessary agents (like QA, Frontend Dev) to active or bench status.
2. **Product & Architecture Review**: The PM and Tech Lead agents review requirements and debate implementation strategies.
3. **Execution**: Developer agents (`dev-backend`, `dev-fe`) generate code and automatically scaffold projects using framework CLI commands directly inside your configured `ROOT_WORKSPACE`.
4. **Quality Assurance**: QA agents automatically spin up development servers to review implementations and run verification checks.
