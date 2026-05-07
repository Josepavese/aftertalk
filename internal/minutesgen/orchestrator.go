package minutesgen

import (
	"context"
	"fmt"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
)

type DefaultOrchestrator struct {
	Prompts PromptBuilder
	Runner  Runner
	Reducer Reducer
	Quality QualityGuard
	Logger  Logger
}

func NewDefaultOrchestrator() *DefaultOrchestrator {
	reducer := DefaultReducer{}
	return &DefaultOrchestrator{
		Prompts: LLMPromptBuilder{},
		Runner:  NewDefaultRunner(reducer),
		Reducer: reducer,
		Quality: DefaultQualityGuard{},
		Logger:  loggingAdapter{},
	}
}

func (o *DefaultOrchestrator) Generate(ctx context.Context, input Input) (*Result, error) {
	if input.Provider == nil {
		return nil, fmt.Errorf("minutes generation provider is required") //nolint:err113
	}
	cfg := normalizeConfig(input.Config)
	retry := normalizeRetryConfig(input.Retry)
	prompts := o.promptBuilder()
	runner := o.runner()
	reducer := o.reducer()
	quality := o.qualityGuard()
	logger := o.logger()

	batches := SplitTranscriptBatches(input.Transcript, cfg)
	if len(batches) == 0 {
		batches = []string{input.Transcript}
	}

	result := &Result{Metrics: Metrics{BatchCount: len(batches)}}
	state := &llm.DynamicMinutesResponse{}
	for i, batch := range batches {
		logger.Infof("Generating minutes batch %d/%d", i+1, len(batches))
		prompt := prompts.Incremental(state, batch, input.Template, input.DetectedLanguage, len(batches) == 1)
		runResult, err := runner.GenerateState(ctx, RunRequest{
			Provider:         input.Provider,
			Template:         input.Template,
			Retry:            retry,
			Prompt:           prompt,
			RepairPrompt:     func(raw string) string { return prompts.Repair(raw, input.Template, input.DetectedLanguage) },
			DetectedLanguage: input.DetectedLanguage,
			Config:           cfg,
		})
		if err != nil {
			return nil, fmt.Errorf("batch %d/%d: %w", i+1, len(batches), err)
		}
		result.Metrics.LLMCalls += runResult.Calls
		state = reducer.Merge(state, runResult.State, input.Template, cfg)
	}

	if len(batches) > 1 {
		logger.Infof("Finalizing minutes after %d incremental batches", len(batches))
		prompt := prompts.Finalize(state, input.Template, input.DetectedLanguage)
		runResult, err := runner.GenerateState(ctx, RunRequest{
			Provider:         input.Provider,
			Template:         input.Template,
			Retry:            retry,
			Prompt:           prompt,
			RepairPrompt:     func(raw string) string { return prompts.Repair(raw, input.Template, input.DetectedLanguage) },
			DetectedLanguage: input.DetectedLanguage,
			Config:           cfg,
		})
		if err != nil {
			return nil, fmt.Errorf("finalize minutes: %w", err)
		}
		result.Metrics.LLMCalls += runResult.Calls
		state = reducer.Merge(state, runResult.State, input.Template, cfg)
	}

	result.State = state
	result.QualityWarnings = quality.Evaluate(input.Transcript, len(batches), state)
	return result, nil
}

func (o *DefaultOrchestrator) promptBuilder() PromptBuilder {
	if o != nil && o.Prompts != nil {
		return o.Prompts
	}
	return LLMPromptBuilder{}
}

func (o *DefaultOrchestrator) runner() Runner {
	if o != nil && o.Runner != nil {
		return o.Runner
	}
	return NewDefaultRunner(DefaultReducer{})
}

func (o *DefaultOrchestrator) reducer() Reducer {
	if o != nil && o.Reducer != nil {
		return o.Reducer
	}
	return DefaultReducer{}
}

func (o *DefaultOrchestrator) qualityGuard() QualityGuard {
	if o != nil && o.Quality != nil {
		return o.Quality
	}
	return DefaultQualityGuard{}
}

func (o *DefaultOrchestrator) logger() Logger {
	if o != nil && o.Logger != nil {
		return o.Logger
	}
	return loggingAdapter{}
}

type LLMPromptBuilder struct{}

func (LLMPromptBuilder) Incremental(existing *llm.DynamicMinutesResponse, transcriptChunk string, tmpl config.TemplateConfig, detectedLanguage string, finalPass bool) string {
	return llm.GenerateIncrementalMinutesPrompt(existing, transcriptChunk, tmpl, detectedLanguage, finalPass)
}

func (LLMPromptBuilder) Finalize(existing *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, detectedLanguage string) string {
	return llm.GenerateMinutesFinalizePrompt(existing, tmpl, detectedLanguage)
}

func (LLMPromptBuilder) Repair(rawResponse string, tmpl config.TemplateConfig, detectedLanguage string) string {
	return llm.GenerateMinutesRepairPrompt(rawResponse, tmpl, detectedLanguage)
}

type DefaultRunner struct {
	Reducer Reducer
	Logger  Logger
}

func NewDefaultRunner(reducer Reducer) DefaultRunner {
	if reducer == nil {
		reducer = DefaultReducer{}
	}
	return DefaultRunner{Reducer: reducer, Logger: loggingAdapter{}}
}

func (r DefaultRunner) GenerateState(ctx context.Context, req RunRequest) (*RunResult, error) {
	var err error
	result := &RunResult{}
	retryConfig := normalizeRetryConfig(req.Retry)
	reducer := r.Reducer
	if reducer == nil {
		reducer = DefaultReducer{}
	}
	logger := r.Logger
	if logger == nil {
		logger = loggingAdapter{}
	}
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := retryConfig.InitialBackoff * time.Duration(1<<uint(attempt-1))
			if backoff > retryConfig.MaxBackoff {
				backoff = retryConfig.MaxBackoff
			}
			logger.Warnf("LLM request failed, retrying in %v (attempt %d/%d)", backoff, attempt, retryConfig.MaxRetries)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, genErr := req.Provider.Generate(ctx, req.Prompt)
		result.Calls++
		if genErr != nil {
			err = genErr
			logger.Errorf("LLM request failed (attempt %d): %v", attempt+1, genErr)
			continue
		}

		parsed, parseErr := llm.ParseMinutesDynamic(response)
		if parseErr == nil {
			result.State = reducer.Normalize(parsed, req.Template, req.Config)
			return result, nil
		}

		var repairPrompt string
		if req.RepairPrompt != nil {
			repairPrompt = req.RepairPrompt(response)
		} else {
			repairPrompt = llm.GenerateMinutesRepairPrompt(response, req.Template, req.DetectedLanguage)
		}
		repairResponse, repairErr := req.Provider.Generate(ctx, repairPrompt)
		result.Calls++
		if repairErr == nil {
			repaired, repairedErr := llm.ParseMinutesDynamic(repairResponse)
			if repairedErr == nil {
				result.State = reducer.Normalize(repaired, req.Template, req.Config)
				return result, nil
			}
			parseErr = repairedErr
		} else {
			logger.Errorf("LLM repair failed (attempt %d): %v", attempt+1, repairErr)
		}

		err = fmt.Errorf("failed to parse LLM response: %w", parseErr)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate minutes after %d retries: %w", retryConfig.MaxRetries+1, err)
	}
	return nil, fmt.Errorf("failed to generate minutes after %d retries", retryConfig.MaxRetries+1)
}

type loggingAdapter struct{}

func (loggingAdapter) Infof(format string, args ...interface{}) {
	logging.Infof(format, args...)
}

func (loggingAdapter) Warnf(format string, args ...interface{}) {
	logging.Warnf(format, args...)
}

func (loggingAdapter) Errorf(format string, args ...interface{}) {
	logging.Errorf(format, args...)
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.BatchMaxSegments <= 0 {
		cfg.BatchMaxSegments = defaults.BatchMaxSegments
	}
	if cfg.BatchMaxChars <= 0 {
		cfg.BatchMaxChars = defaults.BatchMaxChars
	}
	if cfg.MaxSummaryPhases <= 0 {
		cfg.MaxSummaryPhases = defaults.MaxSummaryPhases
	}
	if cfg.MaxCitations <= 0 {
		cfg.MaxCitations = defaults.MaxCitations
	}
	return cfg
}

func normalizeRetryConfig(cfg RetryConfig) RetryConfig {
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = cfg.InitialBackoff
	}
	if cfg.MaxBackoff < cfg.InitialBackoff {
		cfg.MaxBackoff = cfg.InitialBackoff
	}
	return cfg
}
