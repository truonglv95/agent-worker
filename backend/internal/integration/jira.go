package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// JiraClient handles communication with Jira REST API v2.
type JiraClient struct {
	BaseURL    string
	User       string
	Token      string
	ProjectKey string
	HTTPClient *http.Client
}

// NewJiraClient creates a new Jira client.
func NewJiraClient(baseURL, user, token string) *JiraClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &JiraClient{
		BaseURL:    baseURL,
		User:       user,
		Token:      token,
		HTTPClient: &http.Client{},
	}
}

// GetCurrentUserAccountId fetches the account ID of the authenticated user.
func (c *JiraClient) GetCurrentUserAccountId() (string, error) {
	if c.BaseURL == "" || c.User == "" || c.Token == "" {
		return "", fmt.Errorf("jira is not fully configured")
	}

	url := fmt.Sprintf("%s/rest/api/2/myself", c.BaseURL)
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jira myself api failed (status %d): %s", resp.StatusCode, string(b))
	}

	var data struct {
		AccountId string `json:"accountId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode user info: %w", err)
	}

	return data.AccountId, nil
}

// CreateProject creates a new Team-managed Software Project in Jira.
func (c *JiraClient) CreateProject(name, key string) error {
	accountId, err := c.GetCurrentUserAccountId()
	if err != nil {
		return fmt.Errorf("cannot create project without lead account id: %w", err)
	}

	url := fmt.Sprintf("%s/rest/api/2/project", c.BaseURL)
	payload := map[string]interface{}{
		"key":                key,
		"name":               name,
		"projectTypeKey":     "software",
		"projectTemplateKey": "com.pyxis.greenhopper.jira:gh-simplified-scrum-classic",
		"description":        "Auto-generated project for workflow " + name,
		"leadAccountId":      accountId,
		"assigneeType":       "PROJECT_LEAD",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.SetBasicAuth(c.User, c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		// If project already exists, we can ignore or return error.
		// Jira returns 400 with errors in json if key exists.
		if strings.Contains(string(b), "A project with that project key already exists") {
			return nil // Project already exists, we can proceed
		}
		return fmt.Errorf("jira create project failed (status %d): %s", resp.StatusCode, string(b))
	}

	return nil
}

// SyncIssues creates Jira issues (Story + Sub-tasks) in the specified Project, and organizes a Sprint.
func (c *JiraClient) SyncIssues(workflowName string, projectKey string, tasks []TaskPayload) (string, error) {
	if c.BaseURL == "" || c.User == "" || c.Token == "" || projectKey == "" {
		return "", fmt.Errorf("jira is not fully configured (missing url, user, token, or project key)")
	}

	accountId, _ := c.GetCurrentUserAccountId() // Get for assigning
	issueURL := fmt.Sprintf("%s/rest/api/2/issue", c.BaseURL)
	var firstIssueKey string
	var createdStoryKeys []string

	for _, t := range tasks {
		fields := map[string]interface{}{
			"project":     map[string]string{"key": projectKey},
			"summary":     fmt.Sprintf("[%s] %s", workflowName, t.Title),
			"description": t.Description,
			"issuetype":   map[string]string{"name": "Story"},
		}
		if accountId != "" {
			fields["assignee"] = map[string]string{"accountId": accountId}
		}

		payload := map[string]interface{}{"fields": fields}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", issueURL, bytes.NewReader(body))
		req.SetBasicAuth(c.User, c.Token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			continue // Skip on error to try others
		}

		if resp.StatusCode < 300 {
			var respData struct{ Key string `json:"key"` }
			if err := json.NewDecoder(resp.Body).Decode(&respData); err == nil {
				if firstIssueKey == "" {
					firstIssueKey = respData.Key
				}
				createdStoryKeys = append(createdStoryKeys, respData.Key)

				// Create subtasks
				for _, st := range t.Subtasks {
					stFields := map[string]interface{}{
						"project":     map[string]string{"key": projectKey},
						"summary":     fmt.Sprintf("[Subtask] %s", st.Title),
						"description": st.Description,
						"issuetype":   map[string]string{"name": "Sub-task"},
						"parent":      map[string]string{"key": respData.Key},
					}
					if accountId != "" {
						stFields["assignee"] = map[string]string{"accountId": accountId}
					}
					stPayload := map[string]interface{}{"fields": stFields}
					stBody, _ := json.Marshal(stPayload)
					stReq, _ := http.NewRequest("POST", issueURL, bytes.NewReader(stBody))
					stReq.SetBasicAuth(c.User, c.Token)
					stReq.Header.Set("Content-Type", "application/json")
					stResp, stErr := c.HTTPClient.Do(stReq)
					if stErr == nil {
						stResp.Body.Close()
					}
				}
			}
		} else {
			b, _ := io.ReadAll(resp.Body)
			fmt.Printf("Jira error: %d %s\n", resp.StatusCode, string(b))
		}
		resp.Body.Close()
	}

	if firstIssueKey == "" {
		return "", fmt.Errorf("failed to create any jira issues")
	}

	// Sprint Management
	boardID, err := c.GetBoardID(projectKey)
	if err == nil {
		sprintID, err := c.CreateSprint(boardID, fmt.Sprintf("%s Sprint 1", workflowName))
		if err == nil {
			// Pull up to 3 stories into sprint
			sprintIssues := createdStoryKeys
			if len(sprintIssues) > 3 {
				sprintIssues = sprintIssues[:3]
			}
			_ = c.MoveIssuesToSprint(sprintID, sprintIssues)
			_ = c.StartSprint(sprintID) // Start it
		}
	}

	// Return a link to the project board
	return fmt.Sprintf("%s/jira/software/projects/%s/boards/%d", c.BaseURL, projectKey, boardID), nil
}

// GetBoardID fetches the board ID for a given project key.
func (c *JiraClient) GetBoardID(projectKey string) (int, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board?projectKeyOrId=%s", c.BaseURL, projectKey)
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var data struct {
		Values []struct {
			ID int `json:"id"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}
	if len(data.Values) == 0 {
		return 0, fmt.Errorf("no board found for project %s", projectKey)
	}
	return data.Values[0].ID, nil
}

// CreateSprint creates a new future sprint on the board.
func (c *JiraClient) CreateSprint(boardID int, name string) (int, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint", c.BaseURL)
	payload := map[string]interface{}{
		"name":          name,
		"originBoardId": boardID,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.SetBasicAuth(c.User, c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to create sprint: %s", string(b))
	}

	var data struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}
	return data.ID, nil
}

// MoveIssuesToSprint moves issues to a sprint.
func (c *JiraClient) MoveIssuesToSprint(sprintID int, issueKeys []string) error {
	if len(issueKeys) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", c.BaseURL, sprintID)
	payload := map[string]interface{}{
		"issues": issueKeys,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.SetBasicAuth(c.User, c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to move issues to sprint: %s", string(b))
	}
	return nil
}

// StartSprint starts a sprint.
func (c *JiraClient) StartSprint(sprintID int) error {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d", c.BaseURL, sprintID)
	
	// Start date now, end date in 14 days
	now := time.Now()
	endDate := now.Add(14 * 24 * time.Hour)
	
	payload := map[string]interface{}{
		"state":     "active",
		"startDate": now.Format("2006-01-02T15:04:05.000-07:00"),
		"endDate":   endDate.Format("2006-01-02T15:04:05.000-07:00"),
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.SetBasicAuth(c.User, c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to start sprint: %s", string(b))
	}
	return nil
}

// Issue represents a basic Jira issue from search API
type Issue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

// GetPendingSubtasks fetches undone subtasks for a project.
func (c *JiraClient) GetPendingSubtasks(projectKey string) ([]Issue, error) {
	// Search for subtasks that are not done, ordered by creation time ascending
	jql := fmt.Sprintf("project = \"%s\" AND issuetype = \"Sub-task\" AND statusCategory != Done ORDER BY created ASC", projectKey)
	url := fmt.Sprintf("%s/rest/api/2/search/jql?jql=%s&maxResults=5&fields=summary,description,status", c.BaseURL, url.QueryEscape(jql))

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s", string(b))
	}

	var data struct {
		Issues []Issue `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Issues, nil
}

// TransitionIssue moves an issue to a new status (e.g., "In Review" or "Code Review" or "Done")
func (c *JiraClient) TransitionIssue(issueKey, statusName string) error {
	// 1. Get transitions
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.BaseURL, issueKey)
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var transData struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&transData); err != nil {
		return err
	}

	var targetID string
	for _, t := range transData.Transitions {
		// Check both the transition name and the destination status name
		if strings.Contains(strings.ToLower(t.Name), strings.ToLower(statusName)) || 
		   strings.Contains(strings.ToLower(t.To.Name), strings.ToLower(statusName)) {
			targetID = t.ID
			break
		}
	}
	if targetID == "" {
		return fmt.Errorf("transition %s not found for issue %s", statusName, issueKey)
	}

	// 2. Perform transition
	payload := map[string]interface{}{
		"transition": map[string]string{
			"id": targetID,
		},
	}
	body, _ := json.Marshal(payload)
	tReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	tReq.SetBasicAuth(c.User, c.Token)
	tReq.Header.Set("Content-Type", "application/json")

	tResp, err := c.HTTPClient.Do(tReq)
	if err != nil {
		return err
	}
	defer tResp.Body.Close()

	if tResp.StatusCode >= 300 {
		b, _ := io.ReadAll(tResp.Body)
		return fmt.Errorf("failed to transition: %s", string(b))
	}
	return nil
}
