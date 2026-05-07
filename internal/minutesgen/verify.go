package minutesgen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
)

type DefaultVerifier struct {
	Reducer Reducer
	Logger  Logger
}

func NewDefaultVerifier(reducer Reducer) DefaultVerifier {
	if reducer == nil {
		reducer = DefaultReducer{}
	}
	return DefaultVerifier{Reducer: reducer, Logger: loggingAdapter{}}
}

func (v DefaultVerifier) Verify(ctx context.Context, req VerificationRequest) (*VerificationResult, error) {
	if req.Provider == nil {
		return nil, fmt.Errorf("minutes verification provider is required") //nolint:err113
	}
	result := &VerificationResult{}
	retryConfig := normalizeRetryConfig(req.Retry)
	reducer := v.Reducer
	if reducer == nil {
		reducer = DefaultReducer{}
	}
	logger := v.Logger
	if logger == nil {
		logger = loggingAdapter{}
	}

	var err error
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := retryConfig.InitialBackoff * time.Duration(1<<uint(attempt-1))
			if backoff > retryConfig.MaxBackoff {
				backoff = retryConfig.MaxBackoff
			}
			logger.Warnf("LLM verification failed, retrying in %v (attempt %d/%d)", backoff, attempt, retryConfig.MaxRetries)
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(backoff):
			}
		}

		callCtx := llm.WithRequestMetadata(ctx, llm.RequestMetadata{
			Phase:      req.Phase,
			BatchIndex: req.BatchIndex,
			BatchTotal: req.BatchTotal,
			Attempt:    attempt + 1,
		})
		response, genErr := req.Provider.Generate(callCtx, req.Prompt)
		result.Calls++
		if genErr != nil {
			err = genErr
			logger.Errorf("LLM verification request failed (attempt %d): %v", attempt+1, genErr)
			if errors.Is(genErr, llm.ErrLLMBudgetExceeded) {
				return result, genErr
			}
			continue
		}

		parsed, issues, parseErr := parseVerificationResponse(response)
		if parseErr == nil {
			result.State = reducer.Normalize(parsed, req.Template, req.Config)
			result.Issues = issues
			return result, nil
		}

		var repairPrompt string
		if req.RepairPrompt != nil {
			repairPrompt = req.RepairPrompt(response)
		}
		if repairPrompt != "" {
			repairCtx := llm.WithRequestMetadata(ctx, llm.RequestMetadata{
				Phase:      repairPhase(req.Phase),
				BatchIndex: req.BatchIndex,
				BatchTotal: req.BatchTotal,
				Attempt:    attempt + 1,
			})
			repairResponse, repairErr := req.Provider.Generate(repairCtx, repairPrompt)
			result.Calls++
			if repairErr == nil {
				repaired, repairIssues, repairedErr := parseVerificationResponse(repairResponse)
				if repairedErr == nil {
					result.State = reducer.Normalize(repaired, req.Template, req.Config)
					result.Issues = repairIssues
					return result, nil
				}
				parseErr = repairedErr
			} else {
				logger.Errorf("LLM verification repair failed (attempt %d): %v", attempt+1, repairErr)
				if errors.Is(repairErr, llm.ErrLLMBudgetExceeded) {
					return result, repairErr
				}
			}
		}

		err = fmt.Errorf("failed to parse verification response: %w", parseErr)
	}
	if err != nil {
		return result, fmt.Errorf("failed to verify minutes after %d retries: %w", retryConfig.MaxRetries+1, err)
	}
	return result, fmt.Errorf("failed to verify minutes after %d retries", retryConfig.MaxRetries+1)
}

func parseVerificationResponse(raw string) (*llm.DynamicMinutesResponse, []string, error) {
	var envelope struct {
		OK      *bool                       `json:"ok"`
		Issues  *[]string                   `json:"issues"`
		Minutes *llm.DynamicMinutesResponse `json:"minutes"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err == nil &&
		envelope.OK != nil &&
		envelope.Issues != nil &&
		envelope.Minutes != nil {
		if !*envelope.OK {
			return nil, nil, fmt.Errorf("verification response returned ok=false")
		}
		return envelope.Minutes, normalizeVerificationIssues(*envelope.Issues), nil
	}
	return nil, nil, fmt.Errorf("verification response must contain top-level ok, issues, and minutes")
}

func guardVerifiedMinutes(original, verified *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse {
	baseline := normalizeDynamicMinutes(cloneDynamicMinutes(original), tmpl, cfg)
	candidate := normalizeDynamicMinutes(cloneDynamicMinutes(verified), tmpl, cfg)
	candidate.Summary.Phases = guardVerifiedPhases(baseline.Summary.Phases, candidate.Summary.Phases)
	candidate.Sections = guardVerifiedSections(baseline.Sections, candidate.Sections, tmpl)
	candidate.Citations = append([]llm.Citation{}, baseline.Citations...)
	return normalizeDynamicMinutes(candidate, tmpl, cfg)
}

func guardVerifiedPhases(original, verified []llm.Phase) []llm.Phase {
	if len(original) != len(verified) {
		return append([]llm.Phase{}, original...)
	}
	out := make([]llm.Phase, len(original))
	for i := range original {
		out[i] = verified[i]
		out[i].StartMs = original[i].StartMs
		out[i].EndMs = original[i].EndMs
	}
	return out
}

func guardVerifiedSections(original, verified map[string]json.RawMessage, tmpl config.TemplateConfig) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(original))
	for _, section := range tmpl.Sections {
		origRaw, origOK := original[section.Key]
		if !origOK {
			origRaw = emptySectionValue(section.Type)
		}
		verifiedRaw, verifiedOK := verified[section.Key]
		if !verifiedOK {
			out[section.Key] = cloneRawMessage(origRaw)
			continue
		}
		out[section.Key] = guardVerifiedJSONRaw(origRaw, verifiedRaw)
	}
	for key, origRaw := range original {
		if _, ok := out[key]; ok {
			continue
		}
		verifiedRaw, verifiedOK := verified[key]
		if !verifiedOK {
			out[key] = cloneRawMessage(origRaw)
			continue
		}
		out[key] = guardVerifiedJSONRaw(origRaw, verifiedRaw)
	}
	return out
}

func guardVerifiedJSONRaw(original, verified json.RawMessage) json.RawMessage {
	var originalValue interface{}
	var verifiedValue interface{}
	if json.Unmarshal(original, &originalValue) != nil || json.Unmarshal(verified, &verifiedValue) != nil {
		return cloneRawMessage(original)
	}
	guarded := guardVerifiedJSONValue(originalValue, verifiedValue)
	b, err := json.Marshal(guarded)
	if err != nil {
		return cloneRawMessage(original)
	}
	return b
}

func guardVerifiedJSONValue(original, verified interface{}) interface{} {
	switch originalValue := original.(type) {
	case string:
		if verifiedValue, ok := verified.(string); ok {
			return verifiedValue
		}
		return originalValue
	case []interface{}:
		verifiedValue, ok := verified.([]interface{})
		if !ok || len(originalValue) != len(verifiedValue) {
			return originalValue
		}
		out := make([]interface{}, len(originalValue))
		for i := range originalValue {
			out[i] = guardVerifiedJSONValue(originalValue[i], verifiedValue[i])
		}
		return out
	case map[string]interface{}:
		verifiedValue, ok := verified.(map[string]interface{})
		if !ok {
			return originalValue
		}
		out := make(map[string]interface{}, len(originalValue))
		for key, originalChild := range originalValue {
			verifiedChild, ok := verifiedValue[key]
			if !ok {
				out[key] = originalChild
				continue
			}
			out[key] = guardVerifiedJSONValue(originalChild, verifiedChild)
		}
		return out
	default:
		return originalValue
	}
}

func normalizeVerificationIssues(issues []string) []string {
	if len(issues) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		issue = strings.TrimSpace(issue)
		if issue != "" {
			out = append(out, issue)
		}
	}
	return out
}
