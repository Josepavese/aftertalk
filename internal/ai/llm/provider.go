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

type LLMConfig struct {
	Provider  string
	OpenAI    OpenAIConfig
	Anthropic AnthropicConfig
	Azure     AzureLLMConfig
	Ollama    OllamaConfig
}

type OllamaConfig struct {
	BaseURL string
	Model   string
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

// Citation is a verbatim quote from the transcript with a role label.
type Citation struct {
	TimestampMs int    `json:"timestamp_ms"`
	Text        string `json:"text"`
	Role        string `json:"role"`
}

// DynamicMinutesResponse is the flexible LLM output for any template.
// Sections is a map from section key → raw JSON value.
// Citations are always typed and always present.
type DynamicMinutesResponse struct {
	Sections  map[string]json.RawMessage `json:"sections"`
	Citations []Citation                 `json:"citations"`
}

// ParseMinutesDynamic unmarshals the raw LLM JSON into a DynamicMinutesResponse.
func ParseMinutesDynamic(jsonStr string) (*DynamicMinutesResponse, error) {
	var r DynamicMinutesResponse
	if err := json.Unmarshal([]byte(jsonStr), &r); err != nil {
		return nil, err
	}
	if r.Sections == nil {
		r.Sections = map[string]json.RawMessage{}
	}
	if r.Citations == nil {
		r.Citations = []Citation{}
	}
	return &r, nil
}
