package minutesgen

import (
	"context"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
)

// Provider is the minimal LLM surface required by the generation layer.
// Concrete adapters can wrap local providers, remote providers, or future
// orchestration frameworks without leaking persistence/webhook concerns here.
type Provider interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Name() string
}

// Orchestrator owns minutes generation flow only: chunking, prompts, LLM calls,
// state reduction, finalization, repair, and quality checks.
type Orchestrator interface {
	Generate(ctx context.Context, input Input) (*Result, error)
}

type Config struct {
	Incremental      bool
	BatchMaxSegments int
	BatchMaxChars    int
	MaxSummaryPhases int
	MaxCitations     int
}

func DefaultConfig() Config {
	return Config{
		Incremental:      true,
		BatchMaxSegments: 24,
		BatchMaxChars:    6000,
		MaxSummaryPhases: 8,
		MaxCitations:     12,
	}
}

type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

type Input struct {
	Provider         Provider
	Template         config.TemplateConfig
	Config           Config
	Retry            RetryConfig
	Transcript       string
	DetectedLanguage string
}

type Result struct {
	State           *llm.DynamicMinutesResponse
	QualityWarnings []string
	Metrics         Metrics
}

type Metrics struct {
	BatchCount int
	LLMCalls   int
}

type PromptBuilder interface {
	Incremental(existing *llm.DynamicMinutesResponse, transcriptChunk string, tmpl config.TemplateConfig, detectedLanguage string, finalPass bool) string
	Finalize(existing *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, detectedLanguage string) string
	Repair(rawResponse string, tmpl config.TemplateConfig, detectedLanguage string) string
}

type Runner interface {
	GenerateState(ctx context.Context, req RunRequest) (*RunResult, error)
}

type RunRequest struct {
	Provider         Provider
	Template         config.TemplateConfig
	Retry            RetryConfig
	Prompt           string
	RepairPrompt     func(rawResponse string) string
	DetectedLanguage string
	Config           Config
}

type RunResult struct {
	State *llm.DynamicMinutesResponse
	Calls int
}

type Reducer interface {
	Normalize(state *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse
	Merge(previous, candidate *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse
}

type QualityGuard interface {
	Evaluate(transcript string, batchCount int, state *llm.DynamicMinutesResponse) []string
}

type Logger interface {
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}
