package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrLLMBudgetExceeded = errors.New("llm cloud budget exceeded")

type RequestMetadata struct {
	RequestID       string `json:"request_id,omitempty"`
	WorkflowID      string `json:"workflow_id,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	MinutesID       string `json:"minutes_id,omitempty"`
	ProviderProfile string `json:"provider_profile,omitempty"`
	Phase           string `json:"phase,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	BatchIndex      int    `json:"batch_index,omitempty"`
	BatchTotal      int    `json:"batch_total,omitempty"`
	Attempt         int    `json:"attempt,omitempty"`
}

type UsageEvent struct {
	Timestamp                time.Time `json:"ts"`
	CostCredits              float64   `json:"cost_credits,omitempty"`
	ID                       string    `json:"id"`
	RequestID                string    `json:"request_id,omitempty"`
	WorkflowID               string    `json:"workflow_id,omitempty"`
	SessionID                string    `json:"session_id,omitempty"`
	MinutesID                string    `json:"minutes_id,omitempty"`
	Phase                    string    `json:"phase,omitempty"`
	ProviderProfile          string    `json:"provider_profile,omitempty"`
	Provider                 string    `json:"provider,omitempty"`
	Model                    string    `json:"model,omitempty"`
	ResolvedProvider         string    `json:"resolved_provider,omitempty"`
	ResolvedModel            string    `json:"resolved_model,omitempty"`
	GenerationID             string    `json:"generation_id,omitempty"`
	Status                   string    `json:"status"`
	ErrorClass               string    `json:"error_class,omitempty"`
	ErrorMessage             string    `json:"error_message,omitempty"`
	BatchIndex               int       `json:"batch_index,omitempty"`
	BatchTotal               int       `json:"batch_total,omitempty"`
	Attempt                  int       `json:"attempt,omitempty"`
	HTTPStatus               int       `json:"http_status,omitempty"`
	PromptTokens             int       `json:"prompt_tokens,omitempty"`
	CompletionTokens         int       `json:"completion_tokens,omitempty"`
	ReasoningTokens          int       `json:"reasoning_tokens,omitempty"`
	CachedTokens             int       `json:"cached_tokens,omitempty"`
	TotalTokens              int       `json:"total_tokens,omitempty"`
	RequestedMaxTokens       int       `json:"requested_max_tokens,omitempty"`
	AffordableRetryMaxTokens int       `json:"affordable_retry_max_tokens,omitempty"`
	DurationMs               int64     `json:"duration_ms,omitempty"`
}

type UsageSummary struct {
	CostCredits      float64 `json:"cost_credits,omitempty"`
	Calls            int     `json:"calls"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	ReasoningTokens  int     `json:"reasoning_tokens,omitempty"`
	CachedTokens     int     `json:"cached_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
}

type UsageBudget struct {
	MaxSessionCostCredits float64
	MaxDailyCostCredits   float64
	BaseDailyCostCredits  float64
	AllowLocalFallback    bool
}

type UsageRecorder interface {
	RecordLLMUsage(ctx context.Context, event UsageEvent) error
}

type UsageEventPatch struct {
	Status       string
	ErrorClass   string
	ErrorMessage string
}

type UsageUpdater interface {
	UpdateLastLLMUsage(ctx context.Context, patch UsageEventPatch) bool
}

type BudgetChecker interface {
	CheckLLMBudget(ctx context.Context) error
}

type CollectingUsageRecorder struct {
	mu     sync.Mutex
	events []UsageEvent
	budget UsageBudget
}

type metadataContextKey struct{}
type usageRecorderContextKey struct{}

func NewCollectingUsageRecorder(budget UsageBudget) *CollectingUsageRecorder {
	return &CollectingUsageRecorder{budget: budget}
}

func WithRequestMetadata(ctx context.Context, meta RequestMetadata) context.Context {
	current := RequestMetadataFromContext(ctx)
	merged := mergeRequestMetadata(current, meta)
	return context.WithValue(ctx, metadataContextKey{}, merged)
}

func RequestMetadataFromContext(ctx context.Context) RequestMetadata {
	meta, _ := ctx.Value(metadataContextKey{}).(RequestMetadata)
	return meta
}

func WithUsageRecorder(ctx context.Context, recorder UsageRecorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, usageRecorderContextKey{}, recorder)
}

func UsageRecorderFromContext(ctx context.Context) UsageRecorder {
	recorder, _ := ctx.Value(usageRecorderContextKey{}).(UsageRecorder)
	return recorder
}

func RecordUsage(ctx context.Context, event UsageEvent) error {
	recorder := UsageRecorderFromContext(ctx)
	if recorder == nil {
		return nil
	}
	return recorder.RecordLLMUsage(ctx, event)
}

func UpdateLastUsage(ctx context.Context, patch UsageEventPatch) bool {
	recorder := UsageRecorderFromContext(ctx)
	updater, ok := recorder.(UsageUpdater)
	if !ok {
		return false
	}
	return updater.UpdateLastLLMUsage(ctx, patch)
}

func CheckUsageBudget(ctx context.Context) error {
	recorder := UsageRecorderFromContext(ctx)
	checker, ok := recorder.(BudgetChecker)
	if !ok {
		return nil
	}
	return checker.CheckLLMBudget(ctx)
}

func (r *CollectingUsageRecorder) RecordLLMUsage(_ context.Context, event UsageEvent) error {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *CollectingUsageRecorder) UpdateLastLLMUsage(_ context.Context, patch UsageEventPatch) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].Status == "" {
			continue
		}
		if patch.Status != "" {
			r.events[i].Status = patch.Status
		}
		if patch.ErrorClass != "" {
			r.events[i].ErrorClass = patch.ErrorClass
		}
		if patch.ErrorMessage != "" {
			r.events[i].ErrorMessage = patch.ErrorMessage
		}
		return true
	}
	return false
}

func (r *CollectingUsageRecorder) CheckLLMBudget(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	summary := summarizeUsageEvents(r.events)
	if r.budget.MaxSessionCostCredits > 0 && summary.CostCredits >= r.budget.MaxSessionCostCredits {
		return fmt.Errorf("%w: session cost %.8f >= limit %.8f", ErrLLMBudgetExceeded, summary.CostCredits, r.budget.MaxSessionCostCredits)
	}
	dailyTotal := r.budget.BaseDailyCostCredits + summary.CostCredits
	if r.budget.MaxDailyCostCredits > 0 && dailyTotal >= r.budget.MaxDailyCostCredits {
		return fmt.Errorf("%w: daily cost %.8f >= limit %.8f", ErrLLMBudgetExceeded, dailyTotal, r.budget.MaxDailyCostCredits)
	}
	return nil
}

func (r *CollectingUsageRecorder) Events() []UsageEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]UsageEvent, len(r.events))
	copy(out, r.events)
	return out
}

func (r *CollectingUsageRecorder) Summary() UsageSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	return summarizeUsageEvents(r.events)
}

func (b UsageBudget) IsZero() bool {
	return b.MaxSessionCostCredits == 0 &&
		b.MaxDailyCostCredits == 0 &&
		b.BaseDailyCostCredits == 0 &&
		!b.AllowLocalFallback
}

func mergeRequestMetadata(base, override RequestMetadata) RequestMetadata {
	if override.RequestID != "" {
		base.RequestID = override.RequestID
	}
	if override.WorkflowID != "" {
		base.WorkflowID = override.WorkflowID
	}
	if override.SessionID != "" {
		base.SessionID = override.SessionID
	}
	if override.MinutesID != "" {
		base.MinutesID = override.MinutesID
	}
	if override.ProviderProfile != "" {
		base.ProviderProfile = override.ProviderProfile
	}
	if override.Phase != "" {
		base.Phase = override.Phase
	}
	if override.Provider != "" {
		base.Provider = override.Provider
	}
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.BatchIndex != 0 {
		base.BatchIndex = override.BatchIndex
	}
	if override.BatchTotal != 0 {
		base.BatchTotal = override.BatchTotal
	}
	if override.Attempt != 0 {
		base.Attempt = override.Attempt
	}
	return base
}

func summarizeUsageEvents(events []UsageEvent) UsageSummary {
	var summary UsageSummary
	for _, event := range events {
		if event.Status == "" {
			continue
		}
		if usageEventCountsAsProviderCall(event.Status) {
			summary.Calls++
		}
		summary.PromptTokens += event.PromptTokens
		summary.CompletionTokens += event.CompletionTokens
		summary.ReasoningTokens += event.ReasoningTokens
		summary.CachedTokens += event.CachedTokens
		summary.TotalTokens += event.TotalTokens
		summary.CostCredits += event.CostCredits
	}
	return summary
}

func usageEventCountsAsProviderCall(status string) bool {
	return status != "budget_exceeded" && status != "client_error"
}
