package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ConfluenceClient handles communication with Confluence REST API.
type ConfluenceClient struct {
	BaseURL    string
	User       string
	Token      string
	SpaceKey   string
	HTTPClient *http.Client
}

// NewConfluenceClient creates a new Confluence client.
func NewConfluenceClient(baseURL, user, token string) *ConfluenceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &ConfluenceClient{
		BaseURL:    baseURL,
		User:       user,
		Token:      token,
		HTTPClient: &http.Client{},
	}
}

// CreateSpace creates a new Space in Confluence for a project.
func (c *ConfluenceClient) CreateSpace(name, key string) error {
	if c.BaseURL == "" || c.User == "" || c.Token == "" {
		return fmt.Errorf("confluence is not fully configured")
	}

	url := fmt.Sprintf("%s/wiki/rest/api/space", c.BaseURL)
	payload := map[string]interface{}{
		"key":  key,
		"name": name,
		"description": map[string]interface{}{
			"plain": map[string]string{
				"value":          "Space for " + name,
				"representation": "plain",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.SetBasicAuth(c.User, c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create confluence space: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		// Ignore if it already exists
		if strings.Contains(string(b), "A space with that key already exists") {
			return nil
		}
		return fmt.Errorf("confluence create space failed (status %d): %s", resp.StatusCode, string(b))
	}

	return nil
}

// SyncPage creates a page in Confluence inside the given spaceKey.
func (c *ConfluenceClient) SyncPage(spaceKey, title, content string) (string, error) {
	if c.BaseURL == "" || c.User == "" || c.Token == "" || spaceKey == "" {
		return "", fmt.Errorf("confluence is not fully configured")
	}

	pageURL := fmt.Sprintf("%s/wiki/rest/api/content", c.BaseURL)

	// Basic HTML wrapping to preserve formatting
	htmlContent := fmt.Sprintf("<pre>%s</pre>", strings.ReplaceAll(content, "\n", "<br/>"))

	payload := map[string]interface{}{
		"type":  "page",
		"title": title,
		"space": map[string]string{
			"key": spaceKey,
		},
		"body": map[string]interface{}{
			"storage": map[string]string{
				"value":          htmlContent,
				"representation": "storage",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", pageURL, bytes.NewReader(body))
	req.SetBasicAuth(c.User, c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create confluence page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("confluence page creation failed (status %d): %s", resp.StatusCode, string(b))
	}

	var respData struct {
		Links map[string]string `json:"_links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("failed to decode confluence response: %w", err)
	}

	webURL := respData.Links["base"] + respData.Links["webui"]
	return webURL, nil
}
