package integration

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ScaffoldPlan is what the LLM returns when analyzing the DAB.
type ScaffoldPlan struct {
	Backend  ScaffoldTarget `json:"backend"`
	Frontend ScaffoldTarget `json:"frontend"`
}

type ScaffoldTarget struct {
	Enabled   bool     `json:"enabled"`
	Framework string   `json:"framework"` // e.g. "go", "express", "fastapi", "none"
	Language  string   `json:"language"`  // e.g. "go", "typescript", "python"
	ModuleName string  `json:"module_name"` // for go mod init
	ExtraPackages []string `json:"extra_packages"` // additional npm/go packages to install
}

// ProjectScaffolder handles real framework-based project initialization.
type ProjectScaffolder struct {
	WorkspacePath string
	RepoName      string
}

func NewProjectScaffolder(workspacePath, repoName string) *ProjectScaffolder {
	return &ProjectScaffolder{
		WorkspacePath: workspacePath,
		RepoName:      repoName,
	}
}

// ParseScaffoldPlan extracts a ScaffoldPlan from raw LLM JSON output.
func ParseScaffoldPlan(raw string) (*ScaffoldPlan, error) {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("no JSON found in response")
	}
	raw = raw[start : end+1]

	var plan ScaffoldPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, fmt.Errorf("parse scaffold plan: %w", err)
	}
	return &plan, nil
}

// Scaffold runs the real CLI commands to set up the project structure.
func (s *ProjectScaffolder) Scaffold(plan *ScaffoldPlan) error {
	// Always create root .gitignore
	if err := s.writeRootGitignore(); err != nil {
		log.Printf("[scaffolder] warn: could not write .gitignore: %v", err)
	}

	// Always write root README
	if err := s.writeReadme(); err != nil {
		log.Printf("[scaffolder] warn: could not write README: %v", err)
	}

	// Scaffold backend
	if plan.Backend.Enabled {
		backendDir := filepath.Join(s.WorkspacePath, "backend")
		if err := os.MkdirAll(backendDir, 0755); err != nil {
			return fmt.Errorf("create backend dir: %w", err)
		}
		if err := s.scaffoldBackend(backendDir, plan.Backend); err != nil {
			return fmt.Errorf("scaffold backend: %w", err)
		}
		log.Printf("[scaffolder] backend scaffolded at %s", backendDir)
	}

	// Scaffold frontend
	if plan.Frontend.Enabled {
		feDir := filepath.Join(s.WorkspacePath, "frontend")
		if err := os.MkdirAll(feDir, 0755); err != nil {
			return fmt.Errorf("create frontend dir: %w", err)
		}
		if err := s.scaffoldFrontend(feDir, plan.Frontend); err != nil {
			return fmt.Errorf("scaffold frontend: %w", err)
		}
		log.Printf("[scaffolder] frontend scaffolded at %s", feDir)
	}

	return nil
}

func (s *ProjectScaffolder) scaffoldBackend(dir string, t ScaffoldTarget) error {
	switch strings.ToLower(t.Framework) {
	case "go", "golang":
		return s.scaffoldGo(dir, t)
	case "express", "node", "nodejs":
		return s.scaffoldExpress(dir, t)
	case "fastapi", "python":
		return s.scaffoldFastAPI(dir, t)
	case "nestjs":
		return s.scaffoldNestJS(dir, t)
	default:
		// Fallback: go
		log.Printf("[scaffolder] unknown backend framework %q, defaulting to Go", t.Framework)
		return s.scaffoldGo(dir, t)
	}
}

func (s *ProjectScaffolder) scaffoldFrontend(dir string, t ScaffoldTarget) error {
	switch strings.ToLower(t.Framework) {
	case "vue", "vue3":
		return s.scaffoldVite(dir, "vue-ts")
	case "react":
		return s.scaffoldVite(dir, "react-ts")
	case "next", "nextjs":
		return s.scaffoldNextJS(dir)
	case "nuxt", "nuxtjs":
		return s.scaffoldNuxt(dir)
	default:
		log.Printf("[scaffolder] unknown frontend framework %q, defaulting to Vue+Vite", t.Framework)
		return s.scaffoldVite(dir, "vue-ts")
	}
}

// ── Go ────────────────────────────────────────────────────────────────────────

func (s *ProjectScaffolder) scaffoldGo(dir string, t ScaffoldTarget) error {
	moduleName := t.ModuleName
	if moduleName == "" {
		moduleName = "github.com/project/" + s.RepoName + "/backend"
	}

	log.Printf("[scaffolder] go mod init %s", moduleName)
	if err := s.run(dir, "go", "mod", "init", moduleName); err != nil {
		return err
	}

	// Create standard Go layout
	for _, subdir := range []string{"cmd/server", "internal/handler", "internal/repository", "internal/models", "internal/config"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0755); err != nil {
			return err
		}
	}

	// Write main.go
	mainGo := `package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(` + "`" + `{"status":"ok"}` + "`" + `))
	})

	log.Println("[server] listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "cmd/server/main.go"), []byte(mainGo), 0644); err != nil {
		return err
	}

	// Install base dependencies
	baseDeps := []string{"github.com/go-chi/chi/v5", "github.com/rs/cors"}
	for _, dep := range baseDeps {
		log.Printf("[scaffolder] go get %s", dep)
		if err := s.run(dir, "go", "get", dep); err != nil {
			log.Printf("[scaffolder] warn: go get %s failed: %v", dep, err)
		}
	}

	// Install extra packages from plan
	for _, pkg := range t.ExtraPackages {
		log.Printf("[scaffolder] go get %s", pkg)
		if err := s.run(dir, "go", "get", pkg); err != nil {
			log.Printf("[scaffolder] warn: go get %s failed: %v", pkg, err)
		}
	}

	if err := s.run(dir, "go", "mod", "tidy"); err != nil {
		log.Printf("[scaffolder] warn: go mod tidy failed: %v", err)
	}

	// Write backend .gitignore
	return os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("server\n*.exe\n*.test\n*.out\n"), 0644)
}

// ── Node/Express ──────────────────────────────────────────────────────────────

func (s *ProjectScaffolder) scaffoldExpress(dir string, t ScaffoldTarget) error {
	log.Printf("[scaffolder] scaffolding Express/TypeScript backend")

	packageJSON := fmt.Sprintf(`{
  "name": "%s-backend",
  "version": "1.0.0",
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "build": "tsc",
    "start": "node dist/index.js"
  },
  "dependencies": {},
  "devDependencies": {}
}`, s.RepoName)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644); err != nil {
		return err
	}

	// tsconfig
	tsconfig := `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "lib": ["ES2022"],
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0644); err != nil {
		return err
	}

	// Create src structure
	for _, sub := range []string{"src/routes", "src/middleware", "src/models", "src/config"} {
		os.MkdirAll(filepath.Join(dir, sub), 0755)
	}

	// src/index.ts
	indexTs := `import express from 'express'
import cors from 'cors'

const app = express()
const PORT = process.env.PORT || 3001

app.use(cors())
app.use(express.json())

app.get('/health', (_req, res) => {
  res.json({ status: 'ok' })
})

app.listen(PORT, () => {
  console.log("[server] listening on port " + PORT)
})

export default app
`
	if err := os.WriteFile(filepath.Join(dir, "src/index.ts"), []byte(indexTs), 0644); err != nil {
		return err
	}

	// Install deps
	baseDeps := []string{"express", "cors"}
	baseDevDeps := []string{"typescript", "tsx", "@types/node", "@types/express", "@types/cors"}

	log.Printf("[scaffolder] npm install %v", baseDeps)
	args := append([]string{"install"}, baseDeps...)
	if err := s.run(dir, "npm", args...); err != nil {
		return err
	}

	args = append([]string{"install", "--save-dev"}, baseDevDeps...)
	if err := s.run(dir, "npm", args...); err != nil {
		return err
	}

	// Install extra packages
	if len(t.ExtraPackages) > 0 {
		args = append([]string{"install"}, t.ExtraPackages...)
		if err := s.run(dir, "npm", args...); err != nil {
			log.Printf("[scaffolder] warn: npm install extra packages failed: %v", err)
		}
	}

	return os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\ndist/\n.env\n*.log\n"), 0644)
}

// ── NestJS ────────────────────────────────────────────────────────────────────

func (s *ProjectScaffolder) scaffoldNestJS(dir string, t ScaffoldTarget) error {
	log.Printf("[scaffolder] scaffolding NestJS backend via CLI")
	// npx @nestjs/cli new . --package-manager npm --skip-git
	if err := s.run(filepath.Dir(dir), "npx", "-y", "@nestjs/cli", "new", filepath.Base(dir),
		"--package-manager", "npm", "--skip-git", "--strict"); err != nil {
		return fmt.Errorf("nestjs new: %w", err)
	}
	if len(t.ExtraPackages) > 0 {
		args := append([]string{"install"}, t.ExtraPackages...)
		if err := s.run(dir, "npm", args...); err != nil {
			log.Printf("[scaffolder] warn: npm install extra: %v", err)
		}
	}
	return nil
}

// ── FastAPI ───────────────────────────────────────────────────────────────────

func (s *ProjectScaffolder) scaffoldFastAPI(dir string, t ScaffoldTarget) error {
	log.Printf("[scaffolder] scaffolding FastAPI backend")
	for _, sub := range []string{"app/routers", "app/models", "app/schemas", "app/services"} {
		os.MkdirAll(filepath.Join(dir, sub), 0755)
	}

	reqs := "fastapi>=0.104.0\nuvicorn[standard]>=0.24.0\npydantic>=2.0.0\npython-dotenv>=1.0.0\n"
	for _, pkg := range t.ExtraPackages {
		reqs += pkg + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(reqs), 0644); err != nil {
		return err
	}

	mainPy := `from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

app = FastAPI(title="` + s.RepoName + `")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.get("/health")
def health():
    return {"status": "ok"}
`
	if err := os.WriteFile(filepath.Join(dir, "app/main.py"), []byte(mainPy), 0644); err != nil {
		return err
	}
	os.WriteFile(filepath.Join(dir, "app/__init__.py"), []byte(""), 0644)

	if err := s.run(dir, "python3", "-m", "venv", "venv"); err != nil {
		log.Printf("[scaffolder] warn: venv failed: %v", err)
	} else {
		s.run(dir, "venv/bin/pip", "install", "-r", "requirements.txt")
	}

	return os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("venv/\n__pycache__/\n*.pyc\n.env\n"), 0644)
}

// ── Vite (Vue/React) ──────────────────────────────────────────────────────────

func (s *ProjectScaffolder) scaffoldVite(dir string, template string) error {
	log.Printf("[scaffolder] creating Vite app with template=%s in %s", template, dir)

	// npx create-vite@latest . --template vue-ts (runs in parent, targets basename)
	parent := filepath.Dir(dir)
	base := filepath.Base(dir)

	if err := s.run(parent, "npx", "-y", "create-vite@latest", base,
		"--template", template); err != nil {
		return fmt.Errorf("create-vite: %w", err)
	}

	// Install deps
	if err := s.run(dir, "npm", "install"); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}

	// Install common extras: axios, pinia (for vue), zustand (for react)
	extras := []string{"axios"}
	if strings.Contains(template, "vue") {
		extras = append(extras, "pinia", "vue-router")
	}
	args := append([]string{"install"}, extras...)
	if err := s.run(dir, "npm", args...); err != nil {
		log.Printf("[scaffolder] warn: npm install extras failed: %v", err)
	}

	// Enforce .gitignore has node_modules
	s.appendGitignore(dir, "node_modules/\ndist/\n.env\n*.local\n")
	return nil
}

// ── Next.js ───────────────────────────────────────────────────────────────────

func (s *ProjectScaffolder) scaffoldNextJS(dir string) error {
	log.Printf("[scaffolder] creating Next.js app via CLI")
	parent := filepath.Dir(dir)
	base := filepath.Base(dir)

	if err := s.run(parent, "npx", "-y", "create-next-app@latest", base,
		"--typescript", "--tailwind", "--eslint", "--app", "--no-git",
		"--import-alias", "@/*"); err != nil {
		return fmt.Errorf("create-next-app: %w", err)
	}
	return nil
}

// ── Nuxt ──────────────────────────────────────────────────────────────────────

func (s *ProjectScaffolder) scaffoldNuxt(dir string) error {
	log.Printf("[scaffolder] creating Nuxt app via CLI")
	parent := filepath.Dir(dir)
	base := filepath.Base(dir)

	if err := s.run(parent, "npx", "-y", "nuxi@latest", "init", base); err != nil {
		return fmt.Errorf("nuxi init: %w", err)
	}
	if err := s.run(dir, "npm", "install"); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}
	return nil
}

// ── Root files ────────────────────────────────────────────────────────────────

func (s *ProjectScaffolder) writeRootGitignore() error {
	content := `# OS
.DS_Store
Thumbs.db

# Editor
.vscode/
.idea/
*.swp

# Node
node_modules/
dist/
.next/
.nuxt/
.output/
*.local

# Go
*.exe
*.test
*.out
backend/server

# Python
venv/
__pycache__/
*.pyc
*.pyo

# Env
.env
.env.local
.env.*.local

# Logs
*.log
npm-debug.log*
`
	return os.WriteFile(filepath.Join(s.WorkspacePath, ".gitignore"), []byte(content), 0644)
}

func (s *ProjectScaffolder) writeReadme() error {
	content := fmt.Sprintf(`# %s

## Project Structure
`+"```"+`
.
├── backend/     # Server-side code
├── frontend/    # Client-side code
└── docker-compose.yml
`+"```"+`

## Getting Started

### Backend
` + "```bash" + `
cd backend
# See backend/README.md
` + "```" + `

### Frontend
` + "```bash" + `
cd frontend
npm install
npm run dev
` + "```" + `
`, s.RepoName)
	return os.WriteFile(filepath.Join(s.WorkspacePath, "README.md"), []byte(content), 0644)
}

func (s *ProjectScaffolder) appendGitignore(dir, content string) {
	f, err := os.OpenFile(filepath.Join(dir, ".gitignore"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(content)
}

// run executes a command in a specific directory, streaming output to logs.
func (s *ProjectScaffolder) run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("[scaffolder] $ %s %s (in %s)", name, strings.Join(args, " "), dir)
	return cmd.Run()
}
