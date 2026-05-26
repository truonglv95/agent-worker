package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type Agent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	SystemPrompt string `json:"system_prompt"`
}

// Structs for Gemini API communication
type GeminiRequest struct {
	Contents         []GeminiContent  `json:"contents"`
	GenerationConfig *GeminiGenConfig `json:"generationConfig,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiGenConfig struct {
	Temperature float64 `json:"temperature"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// Structs for OpenAI API communication
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func CallLLM(systemPrompt, userPrompt string, conversationHistory []OpenAIMessage) (string, error) {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	openaiKey := os.Getenv("LLM_API_KEY")
	openaiBaseURL := os.Getenv("LLM_BASE_URL")
	openaiModel := os.Getenv("LLM_MODEL")

	// 1. Prefer OpenAI-compatible (Grok/Agy) if configured
	if openaiKey != "" && openaiBaseURL != "" && openaiModel != "" {
		return callOpenAI(openaiKey, openaiBaseURL, openaiModel, systemPrompt, userPrompt, conversationHistory)
	}

	// 2. Fall back to Gemini API
	if geminiKey != "" {
		return callGemini(geminiKey, systemPrompt, userPrompt, conversationHistory)
	}

	return "", fmt.Errorf("no LLM API credentials configured (neither GEMINI_API_KEY nor LLM_API_KEY/LLM_BASE_URL/LLM_MODEL is set)")
}

func callGemini(apiKey, systemPrompt, userPrompt string, history []OpenAIMessage) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", apiKey)

	var contents []GeminiContent

	// Inject system prompt (as user content in Gemini or system role if supported; user content with system instructions works well)
	contents = append(contents, GeminiContent{
		Parts: []GeminiPart{{Text: fmt.Sprintf("System Instructions:\n%s", systemPrompt)}},
	})

	// Add conversation history
	for _, msg := range history {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Content}},
		})
	}

	// Add current prompt
	contents = append(contents, GeminiContent{
		Role:  "user",
		Parts: []GeminiPart{{Text: userPrompt}},
	})

	reqBody := GeminiRequest{
		Contents: contents,
		GenerationConfig: &GeminiGenConfig{
			Temperature: 0.3,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API error: %d %s - %s", resp.StatusCode, resp.Status, string(bodyBytes))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response received from Gemini API")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func callOpenAI(apiKey, baseURL, model, systemPrompt, userPrompt string, history []OpenAIMessage) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", baseURL)

	var messages []OpenAIMessage
	messages = append(messages, OpenAIMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Append history
	messages = append(messages, history...)

	// Append user prompt
	messages = append(messages, OpenAIMessage{
		Role:    "user",
		Content: userPrompt,
	})

	reqBody := OpenAIRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0.3,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai-compatible API error: %d %s - %s", resp.StatusCode, resp.Status, string(bodyBytes))
	}

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices received from OpenAI-compatible API")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

func LoadAgent(id string) (*Agent, error) {
	var a Agent
	query := `SELECT id, name, role, system_prompt FROM agents WHERE id = $1`
	err := DB.QueryRow(query, id).Scan(&a.ID, &a.Name, &a.Role, &a.SystemPrompt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
