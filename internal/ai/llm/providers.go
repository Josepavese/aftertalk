package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
)

var (
	errNoResponseOpenAI    = errors.New("no response from OpenAI")
	errNoResponseAnthropic = errors.New("no response from Anthropic")
	errNoResponseAzure     = errors.New("no response from Azure OpenAI")
)

type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.openai.com",
	}
}

func (p *OpenAIProvider) WithBaseURL(url string) *OpenAIProvider {
	p.baseURL = url
	return p
}

func (p *OpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	logging.Infof("OpenAI: Generating response with model %s", p.model)

	reqBody := map[string]interface{}{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant that generates structured meeting minutes in JSON format."},
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error: %s", string(body)) //nolint:err113 // dynamic error with status body
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errNoResponseOpenAI
	}

	return response.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) IsAvailable() bool {
	return p.apiKey != ""
}

type AnthropicProvider struct {
	apiKey  string
	model   string
	baseURL string
}

func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.anthropic.com",
	}
}

func (p *AnthropicProvider) WithBaseURL(url string) *AnthropicProvider {
	p.baseURL = url
	return p
}

func (p *AnthropicProvider) Generate(ctx context.Context, prompt string) (string, error) {
	logging.Infof("Anthropic: Generating response with model %s", p.model)

	reqBody := map[string]interface{}{
		"model":      p.model,
		"max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", p.apiKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API error: %s", string(body)) //nolint:err113 // dynamic error with status body
	}

	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", errNoResponseAnthropic
	}

	return response.Content[0].Text, nil
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) IsAvailable() bool {
	return p.apiKey != ""
}

type AzureOpenAIProvider struct {
	apiKey     string
	endpoint   string
	deployment string
}

func NewAzureOpenAIProvider(apiKey, endpoint, deployment string) *AzureOpenAIProvider {
	return &AzureOpenAIProvider{
		apiKey:     apiKey,
		endpoint:   endpoint,
		deployment: deployment,
	}
}

func (p *AzureOpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	logging.Infof("Azure OpenAI: Generating response with deployment %s", p.deployment)

	reqBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant that generates structured meeting minutes in JSON format."},
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2023-05-15", p.endpoint, p.deployment)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", p.apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("azure OpenAI API error: %s", string(body)) //nolint:err113 // dynamic error with status body
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errNoResponseAzure
	}

	return response.Choices[0].Message.Content, nil
}

func (p *AzureOpenAIProvider) Name() string {
	return "azure"
}

func (p *AzureOpenAIProvider) IsAvailable() bool {
	return p.apiKey != "" && p.endpoint != "" && p.deployment != ""
}

// NewProvider selects and returns the LLM provider based on cfg.
// Returns an error if provider is not set or unrecognised — no stub fallback.
func NewProvider(cfg *LLMConfig) (LLMProvider, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAIProvider(cfg.OpenAI.APIKey, cfg.OpenAI.Model), nil
	case "anthropic":
		return NewAnthropicProvider(cfg.Anthropic.APIKey, cfg.Anthropic.Model), nil
	case "azure":
		return NewAzureOpenAIProvider(cfg.Azure.APIKey, cfg.Azure.Endpoint, cfg.Azure.Deployment), nil
	case "ollama":
		return NewOllamaProvider(cfg.Ollama)
	case "":
		return nil, errors.New("llm.provider is required — supported: openai, anthropic, azure, ollama") //nolint:err113
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider) //nolint:err113
	}
}
