package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all environment-driven configuration values for the server.
type Config struct {
	DatabaseURL          string
	AgyCLIPath           string
	AgyRunnerURL         string
	DebateRounds         int
	Port                 string
	RequireHumanApproval bool
	GitlabBaseURL        string
	GitlabToken          string
	GithubToken          string
	JiraBaseURL          string
	JiraUser             string
	JiraToken            string
	FigmaToken           string
	RootWorkspace        string
	MaxSubtasksPerRun    int
}

// Load reads environment variables and returns a populated Config.
// It provides sensible defaults so the server can run without full env setup.
func Load() *Config {
	// Attempt to load .env file (ignores error if file doesn't exist)
	_ = godotenv.Load()

	debateRounds := 3
	if v := os.Getenv("DEBATE_ROUNDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			debateRounds = n
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	rootWorkspace := os.Getenv("ROOT_WORKSPACE")
	if rootWorkspace == "" {
		rootWorkspace = "/tmp/ai-workspace"
	}

	agyCLIPath := "agy"
	if v := os.Getenv("AGY_CLI_PATH"); v != "" {
		agyCLIPath = v
	}

	agyRunnerURL := os.Getenv("AGY_RUNNER_URL")

	requireHumanApproval := true
	if v := os.Getenv("REQUIRE_HUMAN_APPROVAL"); v == "false" {
		requireHumanApproval = false
	}

	maxSubtasksPerRun := 1
	if v := os.Getenv("MAX_SUBTASKS_PER_RUN"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			maxSubtasksPerRun = parsed
		}
	}

	return &Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		AgyCLIPath:           agyCLIPath,
		AgyRunnerURL:         agyRunnerURL,
		DebateRounds:         debateRounds,
		Port:                 port,
		RequireHumanApproval: requireHumanApproval,
		GitlabBaseURL:        os.Getenv("GITLAB_BASE_URL"),
		GitlabToken:          os.Getenv("GITLAB_TOKEN"),
		GithubToken:          os.Getenv("GITHUB_TOKEN"),
		JiraBaseURL:          os.Getenv("JIRA_BASE_URL"),
		JiraUser:             os.Getenv("JIRA_USER"),
		JiraToken:            os.Getenv("JIRA_TOKEN"),
		FigmaToken:           os.Getenv("FIGMA_TOKEN"),
		RootWorkspace:        rootWorkspace,
		MaxSubtasksPerRun:    maxSubtasksPerRun,
	}
}
