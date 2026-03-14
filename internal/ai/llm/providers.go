package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
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

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
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
		return "", fmt.Errorf("OpenAI API error: %s", string(body))
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
		return "", fmt.Errorf("no response from OpenAI")
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

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
		return "", fmt.Errorf("Anthropic API error: %s", string(body))
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
		return "", fmt.Errorf("no response from Anthropic")
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

	jsonBody, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2023-05-15", p.endpoint, p.deployment)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.apiKey)

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
		return "", fmt.Errorf("Azure OpenAI API error: %s", string(body))
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
		return "", fmt.Errorf("no response from Azure OpenAI")
	}

	return response.Choices[0].Message.Content, nil
}

func (p *AzureOpenAIProvider) Name() string {
	return "azure"
}

func (p *AzureOpenAIProvider) IsAvailable() bool {
	return p.apiKey != "" && p.endpoint != "" && p.deployment != ""
}

// StubLLMProvider is the no-op provider used when no real LLM is configured.
// It returns a minimal valid JSON minutes structure without calling any external API.
// Replace this provider with a real implementation when an LLM backend is available.
type StubLLMProvider struct{}

func NewStubLLMProvider() *StubLLMProvider {
	return &StubLLMProvider{}
}

func (p *StubLLMProvider) Generate(_ context.Context, prompt string) (string, error) {
	logging.Warnf("LLM stub: building minutes from transcript\n--- PROMPT ---\n%s\n--- END PROMPT ---", prompt)

	// Extract transcript lines between "TRANSCRIPT:\n" and "\n\nGenerate"
	transcript := ""
	if start := strings.Index(prompt, "TRANSCRIPT:\n"); start != -1 {
		rest := prompt[start+len("TRANSCRIPT:\n"):]
		if end := strings.Index(rest, "\n\nGenerate"); end != -1 {
			transcript = strings.TrimSpace(rest[:end])
		} else {
			transcript = strings.TrimSpace(rest)
		}
	}

	// Build contents_reported and citations from each transcript line
	type entry struct {
		Text      string `json:"text"`
		Timestamp int    `json:"timestamp"`
	}
	type citation struct {
		TimestampMs int    `json:"timestamp_ms"`
		Text        string `json:"text"`
		Role        string `json:"role"`
	}

	var contents []entry
	var citations []citation

	for _, line := range strings.Split(transcript, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: [MM:SS role]: text
		role := ""
		text := line
		if idx := strings.Index(line, "]: "); idx != -1 {
			header := line[1:idx] // strip leading [
			parts := strings.SplitN(header, " ", 2)
			if len(parts) == 2 {
				role = parts[1]
				text = line[idx+3:]
			}
		}
		contents = append(contents, entry{Text: text, Timestamp: 0})
		citations = append(citations, citation{TimestampMs: 0, Text: text, Role: role})
	}

	if len(contents) == 0 {
		contents = []entry{}
	}
	if len(citations) == 0 {
		citations = []citation{}
	}

	// Build sections map by parsing the JSON schema embedded in the prompt.
	// The schema block looks like:  "sections": {\n    "key": [...],\n  }
	// Fall back to a generic "transcript" key if parsing yields nothing.
	sections := make(map[string]interface{})
	if start := strings.Index(prompt, "\"sections\": {\n"); start != -1 {
		rest := prompt[start+len("\"sections\": {\n"):]
		if end := strings.Index(rest, "\n  }"); end != -1 {
			for _, line := range strings.Split(rest[:end], "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "\"") {
					if colonIdx := strings.Index(line, "\": "); colonIdx != -1 {
						key := line[1:colonIdx]
						sections[key] = []interface{}{}
					}
				}
			}
		}
	}
	if len(sections) == 0 {
		sections["transcript"] = contents
	} else {
		// Populate the first section with the transcript contents.
		for k := range sections {
			sections[k] = contents
			break
		}
	}

	result := map[string]interface{}{
		"sections":  sections,
		"citations": citations,
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (p *StubLLMProvider) Name() string      { return "stub" }
func (p *StubLLMProvider) IsAvailable() bool { return true }

// NewProvider selects and returns the LLM provider based on cfg.
// Falls back to StubLLMProvider when provider name is empty or unrecognised.
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
	case "", "stub":
		return NewStubLLMProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
