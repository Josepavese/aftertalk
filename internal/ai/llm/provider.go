package llm

import (
	"context"
	"encoding/json"
)

type LLMProvider interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Name() string
	IsAvailable() bool
}

type MinutesPrompt struct {
	SessionID         string
	TranscriptionText string
	ParticipantRoles  []string
}

type LLMConfig struct {
	Provider  string
	OpenAI    OpenAIConfig
	Anthropic AnthropicConfig
	Azure     AzureLLMConfig
	Ollama    OllamaConfig
}

type OllamaConfig struct {
	// BaseURL is the Ollama server URL (default: http://localhost:11434)
	BaseURL string
	// Model is the model name, e.g. "llama3.2:3b", "mistral", "qwen2.5"
	Model string
}

type OpenAIConfig struct {
	APIKey string
	Model  string
}

type AnthropicConfig struct {
	APIKey string
	Model  string
}

type AzureLLMConfig struct {
	APIKey     string
	Endpoint   string
	Deployment string
}

type MinutesResponse struct {
	Themes                    []string      `json:"themes"`
	ContentsReported          []ContentItem `json:"contents_reported"`
	ProfessionalInterventions []ContentItem `json:"professional_interventions"`
	ProgressIssues            Progress      `json:"progress_issues"`
	NextSteps                 []string      `json:"next_steps"`
	Citations                 []Citation    `json:"citations"`
}

type ContentItem struct {
	Text      string `json:"text"`
	Timestamp int    `json:"timestamp,omitempty"`
}

type Progress struct {
	Progress []string `json:"progress"`
	Issues   []string `json:"issues"`
}

type Citation struct {
	TimestampMs int    `json:"timestamp_ms"`
	Text        string `json:"text"`
	Role        string `json:"role"`
}

func ParseMinutesResponse(jsonStr string) (*MinutesResponse, error) {
	var response MinutesResponse
	err := json.Unmarshal([]byte(jsonStr), &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}
