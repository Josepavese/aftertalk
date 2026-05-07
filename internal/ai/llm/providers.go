package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
)

var (
	errNoResponseOpenAI    = errors.New("no response from OpenAI")
	errNoResponseAnthropic = errors.New("no response from Anthropic")
	errNoResponseAzure     = errors.New("no response from Azure OpenAI")
	errReasoningOnly       = errors.New("provider returned reasoning/thinking content but no final response")

	affordableTokenRE = regexp.MustCompile(`can only afford (\d+)`)
)

type OpenAIProvider struct {
	apiKey    string
	model     string
	baseURL   string
	timeout   time.Duration
	maxTokens int
	reasoning ReasoningConfig
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.openai.com",
		timeout: 120 * time.Second,
	}
}

func (p *OpenAIProvider) WithBaseURL(url string) *OpenAIProvider {
	p.baseURL = url
	return p
}

func (p *OpenAIProvider) WithMaxTokens(maxTokens int) *OpenAIProvider {
	p.maxTokens = maxTokens
	return p
}

func (p *OpenAIProvider) WithReasoning(reasoning ReasoningConfig) *OpenAIProvider {
	p.reasoning = reasoning
	return p
}

func (p *OpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	logging.InfoEvent("llm.request.started",
		"provider", "openai",
		"model", p.model,
		"phase", RequestMetadataFromContext(ctx).Phase,
	)

	reqBody := map[string]interface{}{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant that generates structured meeting minutes in JSON format."},
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	if p.maxTokens > 0 {
		reqBody["max_tokens"] = p.maxTokens
	}
	if reasoning := buildReasoningRequest(p.model, p.reasoning); len(reasoning) > 0 {
		reqBody["reasoning"] = reasoning
		if p.reasoning.Exclude && !shouldDropReasoningConfig(p.model, p.reasoning) {
			reqBody["include_reasoning"] = false
		}
	}

	body, status, err := p.chatCompletionAdaptive(ctx, reqBody)
	if err != nil {
		return "", err
	}

	if status != http.StatusOK {
		if retryMaxTokens, ok := affordableRetryMaxTokens(body); ok && shouldRetryWithMaxTokens(reqBody, retryMaxTokens) {
			reqBody["max_tokens"] = retryMaxTokens
			logging.Warnf("OpenAI: provider rejected requested token budget; retrying with max_tokens=%d", retryMaxTokens)
			body, status, err = p.chatCompletionAdaptive(ctx, reqBody)
			if err != nil {
				return "", err
			}
		}
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error: %s", string(body)) //nolint:err113 // dynamic error with status body
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				Reasoning        string `json:"reasoning"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		p.markLastUsageFailure(ctx, "parse_failure", "chat_response_parse", err)
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		p.markLastUsageFailure(ctx, "parse_failure", "no_choices", errNoResponseOpenAI)
		return "", errNoResponseOpenAI
	}

	msg := response.Choices[0].Message
	content, err := contentOrReasoningJSON(msg.Content, msg.ReasoningContent, msg.Reasoning)
	if err != nil {
		errorClass := "content_parse_failure"
		status := "parse_failure"
		if errors.Is(err, errReasoningOnly) {
			errorClass = "reasoning_only"
			status = "reasoning_only"
		}
		p.markLastUsageFailure(ctx, status, errorClass, err)
		return "", err
	}
	return content, nil
}

func (p *OpenAIProvider) chatCompletion(ctx context.Context, reqBody map[string]interface{}) ([]byte, int, error) {
	start := time.Now()
	event := p.newUsageEvent(ctx, reqBody)
	if err := CheckUsageBudget(ctx); err != nil {
		event.Status = "budget_exceeded"
		event.ErrorClass = "budget_exceeded"
		event.ErrorMessage = logging.SanitizeError(err)
		event.DurationMs = time.Since(start).Milliseconds()
		p.recordUsageEvent(ctx, event)
		return nil, 0, err
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		event.Status = "client_error"
		event.ErrorClass = "marshal_request"
		event.ErrorMessage = logging.SanitizeError(err)
		event.DurationMs = time.Since(start).Milliseconds()
		p.recordUsageEvent(ctx, event)
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAICompatibleEndpointURL(p.baseURL, "chat/completions"), bytes.NewBuffer(jsonBody))
	if err != nil {
		event.Status = "client_error"
		event.ErrorClass = "create_request"
		event.ErrorMessage = logging.SanitizeError(err)
		event.DurationMs = time.Since(start).Milliseconds()
		p.recordUsageEvent(ctx, event)
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	timeout := p.timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		event.Status = "transport_error"
		event.ErrorClass = classifyTransportError(ctx, err)
		event.ErrorMessage = logging.SanitizeError(err)
		event.DurationMs = time.Since(start).Milliseconds()
		p.recordUsageEvent(ctx, event)
		return nil, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		event.Status = "read_error"
		event.HTTPStatus = resp.StatusCode
		event.ErrorClass = "read_response"
		event.ErrorMessage = logging.SanitizeError(err)
		event.DurationMs = time.Since(start).Milliseconds()
		p.recordUsageEvent(ctx, event)
		return nil, 0, fmt.Errorf("failed to read response: %w", err)
	}

	event.HTTPStatus = resp.StatusCode
	event.DurationMs = time.Since(start).Milliseconds()
	event.populateFromOpenAIResponse(body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		p.backfillOpenRouterGenerationUsage(ctx, &event)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		event.Status = "success"
	} else {
		event.Status = "http_error"
		event.ErrorClass = fmt.Sprintf("http_%d", resp.StatusCode)
		event.ErrorMessage = sanitizeProviderError(body)
		if retryMaxTokens, ok := affordableRetryMaxTokens(body); ok {
			event.AffordableRetryMaxTokens = retryMaxTokens
		}
	}
	p.recordUsageEvent(ctx, event)

	return body, resp.StatusCode, nil
}

func (p *OpenAIProvider) chatCompletionAdaptive(ctx context.Context, reqBody map[string]interface{}) ([]byte, int, error) {
	body, status, err := p.chatCompletion(ctx, reqBody)
	if !p.shouldRetryTimeoutWithLowerBudget(ctx, err) {
		return body, status, err
	}

	for _, maxTokens := range adaptiveMaxTokenFallbacks(reqBody) {
		if !shouldRetryWithMaxTokens(reqBody, maxTokens) {
			continue
		}
		reqBody["max_tokens"] = maxTokens
		logging.Warnf("OpenAI: request timed out; retrying with lower max_tokens=%d", maxTokens)
		body, status, err = p.chatCompletion(ctx, reqBody)
		if !p.shouldRetryTimeoutWithLowerBudget(ctx, err) {
			return body, status, err
		}
	}

	return body, status, err
}

func (p *OpenAIProvider) shouldRetryTimeoutWithLowerBudget(ctx context.Context, err error) bool {
	if err == nil || p.maxTokens > 0 || ctx.Err() != nil {
		return false
	}
	return isTimeoutLike(err)
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) IsAvailable() bool {
	return p.apiKey != ""
}

type AnthropicProvider struct {
	apiKey    string
	model     string
	baseURL   string
	timeout   time.Duration
	maxTokens int
}

func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.anthropic.com",
		timeout: 120 * time.Second,
	}
}

func (p *AnthropicProvider) WithBaseURL(url string) *AnthropicProvider {
	p.baseURL = url
	return p
}

func (p *AnthropicProvider) WithMaxTokens(maxTokens int) *AnthropicProvider {
	p.maxTokens = maxTokens
	return p
}

func (p *AnthropicProvider) Generate(ctx context.Context, prompt string) (string, error) {
	logging.Infof("Anthropic: Generating response with model %s", p.model)

	maxTokens := p.maxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	reqBody := map[string]interface{}{
		"model":      p.model,
		"max_tokens": maxTokens,
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

	timeout := p.timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	client := &http.Client{Timeout: timeout}
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
	timeout    time.Duration
	maxTokens  int
	reasoning  ReasoningConfig
}

func NewAzureOpenAIProvider(apiKey, endpoint, deployment string) *AzureOpenAIProvider {
	return &AzureOpenAIProvider{
		apiKey:     apiKey,
		endpoint:   endpoint,
		deployment: deployment,
		timeout:    120 * time.Second,
	}
}

func (p *AzureOpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	logging.Infof("Azure OpenAI: Generating response with deployment %s", p.deployment)

	reqBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant that generates structured meeting minutes in JSON format."},
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	if p.maxTokens > 0 {
		reqBody["max_tokens"] = p.maxTokens
	}
	if reasoning := buildReasoningRequest(p.deployment, p.reasoning); len(reasoning) > 0 {
		reqBody["reasoning"] = reasoning
		if p.reasoning.Exclude && !shouldDropReasoningConfig(p.deployment, p.reasoning) {
			reqBody["include_reasoning"] = false
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2024-02-15-preview", p.endpoint, p.deployment)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", p.apiKey)

	timeout := p.timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	client := &http.Client{Timeout: timeout}
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
				Content          string `json:"content"`
				Reasoning        string `json:"reasoning"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errNoResponseAzure
	}

	msg := response.Choices[0].Message
	return contentOrReasoningJSON(msg.Content, msg.ReasoningContent, msg.Reasoning)
}

func (p *AzureOpenAIProvider) Name() string {
	return "azure"
}

func (p *AzureOpenAIProvider) IsAvailable() bool {
	return p.apiKey != "" && p.endpoint != "" && p.deployment != ""
}

// NewProvider selects and returns the LLM provider based on cfg.
func NewProvider(cfg *LLMConfig) (LLMProvider, error) {
	switch cfg.Provider {
	case "openai":
		p := NewOpenAIProvider(cfg.OpenAI.APIKey, cfg.OpenAI.Model)
		if cfg.OpenAI.BaseURL != "" {
			p.WithBaseURL(cfg.OpenAI.BaseURL)
		}
		if cfg.OpenAI.RequestTimeout > 0 {
			p.timeout = cfg.OpenAI.RequestTimeout
		}
		if cfg.OpenAI.MaxTokens > 0 {
			p.WithMaxTokens(cfg.OpenAI.MaxTokens)
		}
		p.WithReasoning(cfg.OpenAI.Reasoning)
		return p, nil
	case "anthropic":
		p := NewAnthropicProvider(cfg.Anthropic.APIKey, cfg.Anthropic.Model)
		if cfg.Anthropic.RequestTimeout > 0 {
			p.timeout = cfg.Anthropic.RequestTimeout
		}
		if cfg.Anthropic.MaxTokens > 0 {
			p.WithMaxTokens(cfg.Anthropic.MaxTokens)
		}
		return p, nil
	case "azure":
		p := NewAzureOpenAIProvider(cfg.Azure.APIKey, cfg.Azure.Endpoint, cfg.Azure.Deployment)
		if cfg.Azure.RequestTimeout > 0 {
			p.timeout = cfg.Azure.RequestTimeout
		}
		p.maxTokens = cfg.Azure.MaxTokens
		p.reasoning = cfg.Azure.Reasoning
		return p, nil
	case "ollama":
		return NewOllamaProvider(cfg.Ollama)
	case "stub":
		return NewStubProvider(), nil
	case "":
		return nil, errors.New("llm.provider is required — supported: openai, anthropic, azure, ollama, stub") //nolint:err113
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider) //nolint:err113
	}
}

func buildReasoningRequest(model string, cfg ReasoningConfig) map[string]interface{} {
	if shouldDropReasoningConfig(model, cfg) {
		return map[string]interface{}{}
	}

	out := map[string]interface{}{}
	if cfg.Enabled != nil {
		out["enabled"] = *cfg.Enabled
	}
	if cfg.Effort != "" {
		out["effort"] = cfg.Effort
	}
	if cfg.Exclude {
		out["exclude"] = true
	}
	return out
}

func shouldDropReasoningConfig(model string, cfg ReasoningConfig) bool {
	return cfg.Enabled != nil && !*cfg.Enabled && requiresMandatoryReasoning(model)
}

func requiresMandatoryReasoning(model string) bool {
	model = strings.ToLower(model)
	return strings.Contains(model, "minimax-m2.7")
}

func contentOrReasoningJSON(content string, reasoningFields ...string) (string, error) {
	if strings.TrimSpace(content) != "" {
		return content, nil
	}

	for _, raw := range reasoningFields {
		candidate := sanitizeJSON(raw)
		if candidate != "" && json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	for _, raw := range reasoningFields {
		if strings.TrimSpace(raw) != "" {
			return "", errReasoningOnly
		}
	}

	return "", nil
}

func affordableRetryMaxTokens(body []byte) (int, bool) {
	msg := string(body)
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Message != "" {
		msg = payload.Error.Message
	}

	matches := affordableTokenRE.FindStringSubmatch(msg)
	if len(matches) != 2 {
		return 0, false
	}

	affordable, err := strconv.Atoi(matches[1])
	if err != nil || affordable <= 0 {
		return 0, false
	}

	reserve := 256
	if affordable > 8192 {
		reserve = 1024
	}
	retry := affordable - reserve
	if retry <= 0 {
		retry = affordable
	}
	return retry, true
}

func shouldRetryWithMaxTokens(reqBody map[string]interface{}, retryMaxTokens int) bool {
	if retryMaxTokens <= 0 {
		return false
	}
	current, ok := reqBody["max_tokens"]
	if !ok {
		return true
	}
	switch v := current.(type) {
	case int:
		return v > retryMaxTokens
	case int64:
		return v > int64(retryMaxTokens)
	case float64:
		return v > float64(retryMaxTokens)
	default:
		return true
	}
}

func adaptiveMaxTokenFallbacks(reqBody map[string]interface{}) []int {
	candidates := []int{16384, 8192, 4096, 2048}
	current, ok := requestMaxTokens(reqBody)
	if !ok {
		return candidates
	}

	out := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate < current {
			out = append(out, candidate)
		}
	}
	return out
}

func requestMaxTokens(reqBody map[string]interface{}) (int, bool) {
	current, ok := reqBody["max_tokens"]
	if !ok {
		return 0, false
	}
	switch v := current.(type) {
	case int:
		return v, true
	case int64:
		if v <= 0 {
			return 0, false
		}
		return int(v), true
	case float64:
		if v <= 0 {
			return 0, false
		}
		return int(v), true
	default:
		return 0, false
	}
}

func isTimeoutLike(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

func (p *OpenAIProvider) newUsageEvent(ctx context.Context, reqBody map[string]interface{}) UsageEvent {
	meta := RequestMetadataFromContext(ctx)
	requestedMaxTokens, _ := requestMaxTokens(reqBody)
	model, _ := reqBody["model"].(string)
	if model == "" {
		model = p.model
	}
	provider := meta.Provider
	if provider == "" {
		provider = "openai"
	}
	return UsageEvent{
		RequestID:          meta.RequestID,
		WorkflowID:         meta.WorkflowID,
		SessionID:          meta.SessionID,
		MinutesID:          meta.MinutesID,
		ProviderProfile:    meta.ProviderProfile,
		Phase:              meta.Phase,
		BatchIndex:         meta.BatchIndex,
		BatchTotal:         meta.BatchTotal,
		Attempt:            meta.Attempt,
		Provider:           provider,
		Model:              model,
		RequestedMaxTokens: requestedMaxTokens,
	}
}

func (p *OpenAIProvider) recordUsageEvent(ctx context.Context, event UsageEvent) {
	if event.Provider == "" {
		event.Provider = "openai"
	}
	if event.Model == "" {
		event.Model = p.model
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	_ = RecordUsage(ctx, event) // recorder failures must not mask provider behavior

	fields := []interface{}{
		"status", event.Status,
		"provider", event.Provider,
		"model", event.Model,
		"resolved_provider", event.ResolvedProvider,
		"resolved_model", event.ResolvedModel,
		"generation_id", event.GenerationID,
		"provider_profile", event.ProviderProfile,
		"session_id", event.SessionID,
		"minutes_id", event.MinutesID,
		"phase", event.Phase,
		"attempt", event.Attempt,
		"http_status", event.HTTPStatus,
		"prompt_tokens", event.PromptTokens,
		"completion_tokens", event.CompletionTokens,
		"reasoning_tokens", event.ReasoningTokens,
		"cached_tokens", event.CachedTokens,
		"total_tokens", event.TotalTokens,
		"cost_credits", event.CostCredits,
		"requested_max_tokens", event.RequestedMaxTokens,
		"affordable_retry_max_tokens", event.AffordableRetryMaxTokens,
		"duration_ms", event.DurationMs,
	}
	if event.ErrorMessage != "" {
		fields = append(fields, "error_class", event.ErrorClass, "error", event.ErrorMessage)
	}
	switch event.Status {
	case "success":
		logging.InfoEvent("llm.request.completed", fields...)
	case "budget_exceeded":
		logging.WarnEvent("llm.provider_budget_rejected", fields...)
	default:
		logging.WarnEvent("llm.request.failed", fields...)
	}
}

func (p *OpenAIProvider) markLastUsageFailure(ctx context.Context, status, errorClass string, err error) {
	patch := UsageEventPatch{
		Status:       status,
		ErrorClass:   errorClass,
		ErrorMessage: logging.SanitizeError(err),
	}
	UpdateLastUsage(ctx, patch)
	logging.WarnEvent("llm.response.invalid",
		"status", status,
		"error_class", errorClass,
		"provider", "openai",
		"model", p.model,
		"phase", RequestMetadataFromContext(ctx).Phase,
		"error", err,
	)
}

func (p *OpenAIProvider) backfillOpenRouterGenerationUsage(ctx context.Context, event *UsageEvent) {
	if event == nil || event.GenerationID == "" || !isOpenRouterBaseURL(p.baseURL) || eventHasUsage(*event) {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openAICompatibleEndpointURL(p.baseURL, "generation")+"?id="+url.QueryEscape(event.GenerationID), nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	timeout := p.timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		logging.WarnEvent("llm.usage.backfill_failed",
			"provider", "openrouter",
			"generation_id", event.GenerationID,
			"error", err,
		)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.WarnEvent("llm.usage.backfill_failed",
			"provider", "openrouter",
			"generation_id", event.GenerationID,
			"error", err,
		)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logging.WarnEvent("llm.usage.backfill_failed",
			"provider", "openrouter",
			"generation_id", event.GenerationID,
			"http_status", resp.StatusCode,
			"error", sanitizeProviderError(body),
		)
		return
	}
	event.populateFromOpenAIResponse(body)
}

func (e *UsageEvent) populateFromOpenAIResponse(body []byte) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}
	e.GenerationID = firstString(payload["id"], e.GenerationID)
	e.ResolvedModel = firstString(payload["model"], e.ResolvedModel)
	e.ResolvedProvider = firstString(payload["provider"], payload["provider_name"], e.ResolvedProvider)
	if usage, ok := mapValue(payload["usage"]); ok {
		e.PromptTokens = firstInt(usage["prompt_tokens"], usage["input_tokens"], usage["prompt"], usage["tokens_prompt"])
		e.CompletionTokens = firstInt(usage["completion_tokens"], usage["output_tokens"], usage["completion"], usage["tokens_completion"])
		e.TotalTokens = firstInt(usage["total_tokens"], usage["total"])
		e.ReasoningTokens = firstInt(usage["reasoning_tokens"])
		e.CachedTokens = firstInt(usage["cached_tokens"])
		e.CostCredits = firstFloat(usage["cost"], usage["total_cost"], usage["credits"], usage["usage"])
		if details, ok := mapValue(usage["completion_tokens_details"]); ok && e.ReasoningTokens == 0 {
			e.ReasoningTokens = firstInt(details["reasoning_tokens"])
		}
		if details, ok := mapValue(usage["prompt_tokens_details"]); ok && e.CachedTokens == 0 {
			e.CachedTokens = firstInt(details["cached_tokens"])
		}
	}
	if e.TotalTokens == 0 {
		e.TotalTokens = e.PromptTokens + e.CompletionTokens
	}
	if e.CostCredits == 0 {
		e.CostCredits = firstFloat(payload["cost"], payload["total_cost"], payload["usage_cost"])
	}
	if data, ok := mapValue(payload["data"]); ok {
		e.GenerationID = firstString(data["id"], e.GenerationID)
		e.ResolvedModel = firstString(data["model"], e.ResolvedModel)
		e.ResolvedProvider = firstString(data["provider"], data["provider_name"], e.ResolvedProvider)
		if e.PromptTokens == 0 {
			e.PromptTokens = firstInt(data["prompt_tokens"], data["input_tokens"], data["tokens_prompt"])
		}
		if e.CompletionTokens == 0 {
			e.CompletionTokens = firstInt(data["completion_tokens"], data["output_tokens"], data["tokens_completion"])
		}
		if e.TotalTokens == 0 {
			e.TotalTokens = firstInt(data["total_tokens"], data["total"])
		}
		if e.ReasoningTokens == 0 {
			e.ReasoningTokens = firstInt(data["reasoning_tokens"], data["tokens_reasoning"])
		}
		if e.CachedTokens == 0 {
			e.CachedTokens = firstInt(data["cached_tokens"], data["tokens_cached"])
		}
		if e.CostCredits == 0 {
			e.CostCredits = firstFloat(data["cost"], data["total_cost"], data["usage"], data["credits"])
		}
	}
	if e.TotalTokens == 0 {
		e.TotalTokens = e.PromptTokens + e.CompletionTokens
	}
}

func isOpenRouterBaseURL(baseURL string) bool {
	return strings.Contains(strings.ToLower(baseURL), "openrouter.ai")
}

func openAICompatibleEndpointURL(baseURL, endpoint string) string {
	base := strings.TrimRight(baseURL, "/")
	endpoint = strings.TrimLeft(endpoint, "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/" + endpoint
	}
	return base + "/v1/" + endpoint
}

func eventHasUsage(event UsageEvent) bool {
	return event.PromptTokens > 0 ||
		event.CompletionTokens > 0 ||
		event.ReasoningTokens > 0 ||
		event.CachedTokens > 0 ||
		event.TotalTokens > 0 ||
		event.CostCredits > 0
}

func sanitizeProviderError(body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Message != "" {
		return logging.SanitizeMessage(payload.Error.Message)
	}
	return logging.SanitizeMessage(string(body))
}

func classifyTransportError(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) || isTimeoutLike(err) {
		return "timeout"
	}
	return "transport_error"
}

func mapValue(value interface{}) (map[string]interface{}, bool) {
	m, ok := value.(map[string]interface{})
	return m, ok
}

func firstString(values ...interface{}) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func firstInt(values ...interface{}) int {
	for _, value := range values {
		switch v := value.(type) {
		case int:
			if v != 0 {
				return v
			}
		case int64:
			if v != 0 {
				return int(v)
			}
		case float64:
			if v != 0 {
				return int(v)
			}
		case json.Number:
			if i, err := v.Int64(); err == nil && i != 0 {
				return int(i)
			}
		case string:
			if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && i != 0 {
				return i
			}
		}
	}
	return 0
}

func firstFloat(values ...interface{}) float64 {
	for _, value := range values {
		switch v := value.(type) {
		case float64:
			if v != 0 {
				return v
			}
		case float32:
			if v != 0 {
				return float64(v)
			}
		case int:
			if v != 0 {
				return float64(v)
			}
		case int64:
			if v != 0 {
				return float64(v)
			}
		case json.Number:
			if f, err := v.Float64(); err == nil && f != 0 {
				return f
			}
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f != 0 {
				return f
			}
		}
	}
	return 0
}
