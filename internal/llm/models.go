package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var modelHTTPClient = &http.Client{Timeout: 15 * time.Second}

// ModelInfo describes a single model available from a provider.
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
}

// ListModels returns the available models for the given provider type and API key.
func ListModels(providerType, apiKey, baseURL string) ([]ModelInfo, error) {
	switch providerType {
	case "claude", "anthropic":
		return listClaudeModels(apiKey, baseURL)
	case "openai":
		return listOpenAIModels(apiKey, baseURL)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerType)
	}
}

const defaultOpenAIBaseURL = "https://api.openai.com"

func listClaudeModels(apiKey, baseURL string) ([]ModelInfo, error) {
	if baseURL == "" {
		baseURL = defaultClaudeBaseURL
	}
	client := modelHTTPClient
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, ModelInfo{ID: m.ID, DisplayName: m.DisplayName})
	}
	return models, nil
}

func listOpenAIModels(apiKey, baseURL string) ([]ModelInfo, error) {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	client := modelHTTPClient
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, ModelInfo{ID: m.ID, DisplayName: m.ID})
	}
	return models, nil
}
