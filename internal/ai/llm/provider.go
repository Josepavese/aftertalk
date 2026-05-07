package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
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
	Think   *bool
}

type OpenAIConfig struct {
	APIKey         string
	Model          string
	BaseURL        string // optional override, e.g. https://openrouter.ai/api
	RequestTimeout time.Duration
	MaxTokens      int
	Reasoning      ReasoningConfig
}

type AnthropicConfig struct {
	APIKey         string
	Model          string
	RequestTimeout time.Duration
	MaxTokens      int
}

type AzureLLMConfig struct {
	APIKey         string
	Endpoint       string
	Deployment     string
	RequestTimeout time.Duration
	MaxTokens      int
	Reasoning      ReasoningConfig
}

// ReasoningConfig captures provider-specific controls for thinking/reasoning
// models. Adapters translate these fields into each provider's request shape.
type ReasoningConfig struct {
	Enabled *bool
	Effort  string
	Exclude bool
}

// Citation is a verbatim quote from the transcript with a role label.
type Citation struct {
	Text        string `json:"text"`
	Role        string `json:"role"`
	TimestampMs int    `json:"timestamp_ms"`
}

// Summary captures the global synopsis of the conversation so far.
type Summary struct {
	Overview string  `json:"overview"`
	Phases   []Phase `json:"phases"`
}

// Phase is one chronological stage of the conversation.
type Phase struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	StartMs int    `json:"start_ms"`
	EndMs   int    `json:"end_ms"`
}

// DynamicMinutesResponse is the flexible LLM output for any template.
// Summary is global and template-agnostic.
// Sections is a map from section key → raw JSON value.
// Citations are always typed and always present.
type DynamicMinutesResponse struct {
	Summary   Summary                    `json:"summary"`
	Sections  map[string]json.RawMessage `json:"sections"`
	Citations []Citation                 `json:"citations"`
}

// ParseMinutesDynamic unmarshals the raw LLM JSON into a DynamicMinutesResponse.
func ParseMinutesDynamic(jsonStr string) (*DynamicMinutesResponse, error) {
	raw := sanitizeJSON(jsonStr)
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("empty JSON payload")
	}
	var r DynamicMinutesResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	if r.Sections == nil {
		r.Sections = map[string]json.RawMessage{}
	}
	if r.Summary.Phases == nil {
		r.Summary.Phases = []Phase{}
	}
	if r.Citations == nil {
		r.Citations = []Citation{}
	}
	return &r, nil
}
