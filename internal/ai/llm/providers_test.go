package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
)

func init() { //nolint:gochecknoinits // test package init for logger setup
	logging.Init("info", "console") //nolint:errcheck
}

func TestOpenAIProvider_Name(t *testing.T) {
	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4")
	name := provider.Name()

	if name != "openai" {
		t.Errorf("Name mismatch: got %s, want %s", name, "openai")
	}
}

func TestOpenAIProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		model    string
		expected bool
	}{
		{"with valid API key", "sk-test-key", "gpt-4", true},
		{"with empty API key", "", "gpt-4", false},
		{"with empty model", "sk-test-key", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := llm.NewOpenAIProvider(tt.apiKey, tt.model)
			available := provider.IsAvailable()

			if available != tt.expected {
				t.Errorf("Availability mismatch: got %v, want %v", available, tt.expected)
			}
		})
	}
}

func TestOpenAIProvider_Generate_Success(t *testing.T) {
	expectedResponse := `{
		"themes": ["discussion1", "discussion2"],
		"contents_reported": [{"text": "point1", "timestamp": 100}],
		"professional_interventions": [{"text": "intervention1", "timestamp": 200}],
		"progress_issues": {"progress": ["progress1"], "issues": ["issue1"]},
		"next_steps": ["step1", "step2"],
		"citations": [{"timestamp_ms": 100, "text": "quote1", "role": "user"}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type: application/json")
		}
		if r.Header.Get("Authorization") != "Bearer sk-test-key" {
			t.Error("Expected Authorization header with test key")
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		model, ok := reqBody["model"].(string)
		if !ok {
			t.Error("model field is not a string")
			return
		}
		if model != "gpt-4" {
			t.Errorf("Expected model gpt-4, got %s", model)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": expectedResponse,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Errorf("Generate failed: %v", err)
	}
	if result != expectedResponse {
		t.Errorf("Response mismatch:\ngot:      %s\nwant:     %s", result, expectedResponse)
	}
}

func TestOpenAIProvider_Generate_RecordsUsageTelemetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "gen-openrouter-123",
			"model":    "minimax/minimax-m2.7",
			"provider": "MiniMax",
			"usage": map[string]interface{}{
				"prompt_tokens":     101,
				"completion_tokens": 23,
				"total_tokens":      124,
				"cost":              0.000058,
				"completion_tokens_details": map[string]interface{}{
					"reasoning_tokens": 7,
				},
				"prompt_tokens_details": map[string]interface{}{
					"cached_tokens": 11,
				},
			},
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
			},
		})
	}))
	defer server.Close()

	recorder := llm.NewCollectingUsageRecorder(llm.UsageBudget{})
	ctx := llm.WithUsageRecorder(context.Background(), recorder)
	ctx = llm.WithRequestMetadata(ctx, llm.RequestMetadata{
		RequestID:       "req-1",
		SessionID:       "session-1",
		MinutesID:       "minutes-1",
		ProviderProfile: "cloud",
		Phase:           "minutes.batch",
		BatchIndex:      2,
		BatchTotal:      3,
		Attempt:         1,
	})
	provider := llm.NewOpenAIProvider("sk-test-key", "minimax/minimax-m2.7").
		WithBaseURL(server.URL).
		WithMaxTokens(2048)

	if _, err := provider.Generate(ctx, "test prompt"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 usage event, got %d", len(events))
	}
	ev := events[0]
	if ev.Status != "success" || ev.GenerationID != "gen-openrouter-123" || ev.ResolvedProvider != "MiniMax" {
		t.Fatalf("unexpected event identity: %#v", ev)
	}
	if ev.SessionID != "session-1" || ev.MinutesID != "minutes-1" || ev.ProviderProfile != "cloud" || ev.Phase != "minutes.batch" {
		t.Fatalf("metadata not propagated: %#v", ev)
	}
	if ev.PromptTokens != 101 || ev.CompletionTokens != 23 || ev.ReasoningTokens != 7 || ev.CachedTokens != 11 || ev.TotalTokens != 124 {
		t.Fatalf("unexpected token usage: %#v", ev)
	}
	if ev.RequestedMaxTokens != 2048 || ev.CostCredits != 0.000058 {
		t.Fatalf("unexpected budget fields: %#v", ev)
	}
}

func TestOpenAIProvider_Generate_BackfillsOpenRouterGenerationUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openrouter.ai/api/v1/chat/completions":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "gen-backfill-1",
				"choices": []map[string]interface{}{
					{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
				},
			})
		case "/openrouter.ai/api/v1/generation":
			if r.URL.Query().Get("id") != "gen-backfill-1" {
				t.Fatalf("unexpected generation id: %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":                "gen-backfill-1",
					"model":             "minimax/minimax-m2.7",
					"provider_name":     "MiniMax",
					"tokens_prompt":     77,
					"tokens_completion": 13,
					"total_cost":        0.0000387,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	recorder := llm.NewCollectingUsageRecorder(llm.UsageBudget{})
	ctx := llm.WithUsageRecorder(context.Background(), recorder)
	provider := llm.NewOpenAIProvider("sk-test-key", "minimax/minimax-m2.7").
		WithBaseURL(server.URL + "/openrouter.ai/api/v1")

	if _, err := provider.Generate(ctx, "test prompt"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	events := recorder.Events()
	require.Len(t, events, 1)
	ev := events[0]
	assertUsage := map[string]int{
		"prompt":     ev.PromptTokens,
		"completion": ev.CompletionTokens,
		"total":      ev.TotalTokens,
	}
	if assertUsage["prompt"] != 77 || assertUsage["completion"] != 13 || assertUsage["total"] != 90 {
		t.Fatalf("usage was not backfilled: %#v", ev)
	}
	if ev.ResolvedProvider != "MiniMax" || ev.CostCredits != 0.0000387 {
		t.Fatalf("generation metadata was not backfilled: %#v", ev)
	}
}

func TestOpenAIProvider_Generate_UpdatesUsageStatusOnMalformedProviderResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	recorder := llm.NewCollectingUsageRecorder(llm.UsageBudget{})
	ctx := llm.WithUsageRecorder(context.Background(), recorder)
	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)

	_, err := provider.Generate(ctx, "test prompt")
	require.Error(t, err)
	events := recorder.Events()
	require.Len(t, events, 1)
	if events[0].Status != "parse_failure" || events[0].ErrorClass != "chat_response_parse" {
		t.Fatalf("expected usage parse failure, got %#v", events[0])
	}
}

func TestOpenAIProvider_Generate_StopsBeforeRequestWhenBudgetExceeded(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	recorder := llm.NewCollectingUsageRecorder(llm.UsageBudget{MaxSessionCostCredits: 0.01})
	require.NoError(t, recorder.RecordLLMUsage(context.Background(), llm.UsageEvent{
		Status:      "success",
		CostCredits: 0.01,
	}))
	ctx := llm.WithUsageRecorder(context.Background(), recorder)
	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)

	_, err := provider.Generate(ctx, "test prompt")
	if !errors.Is(err, llm.ErrLLMBudgetExceeded) {
		t.Fatalf("expected budget error, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("expected no HTTP request after budget guard, got %d", requests)
	}
	events := recorder.Events()
	if events[len(events)-1].Status != "budget_exceeded" {
		t.Fatalf("expected budget event, got %#v", events[len(events)-1])
	}
}

func TestOpenAIProvider_Generate_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected error for empty response")
	}
	if result != "" {
		t.Error("Expected empty result on error")
	}
}

func TestOpenAIProvider_Generate_MultipleChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"themes":["theme1"]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestOpenAIProvider_Generate_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected error for malformed JSON response")
	}
	if result != "" {
		t.Error("Expected empty result on error")
	}
}

func TestOpenAIProvider_Generate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "response",
					},
				},
			},
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := provider.Generate(ctx, "test prompt")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context cancellation, got: %v", err)
	}
}

func TestOpenAIProvider_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected error for HTTP 401")
	}
	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}
	if !strings.Contains(err.Error(), "OpenAI API error") {
		t.Error("Expected error to mention OpenAI API error")
	}
}

func TestOpenAIProvider_Generate_NetworkError(t *testing.T) {
	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL("http://localhost:1")

	_, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected network error")
	}
	if !strings.Contains(err.Error(), "failed to send request") {
		t.Errorf("Expected network error message, got: %v", err)
	}
}

func TestOpenAIProvider_Generate_JSONObjectResponse(t *testing.T) {
	expectedJSON := `{"themes":["theme1"],"next_steps":["step1"]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": expectedJSON,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Errorf("Generate failed: %v", err)
	}
	if result != expectedJSON {
		t.Errorf("Response mismatch:\ngot:      %s\nwant:     %s", result, expectedJSON)
	}

	// Parse to verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}
}

func TestStubProvider_Generate(t *testing.T) {
	provider := llm.NewStubProvider()
	result, err := provider.Generate(context.Background(), llm.GenerateMinutesPrompt("[0ms therapist]: Buongiorno", config.TemplateConfig{
		ID:   "therapy",
		Name: "Seduta",
		Sections: []config.SectionConfig{
			{Key: "themes", Description: "Themes", Type: "string_list"},
		},
	}, "it"))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !strings.Contains(result, `"summary"`) {
		t.Fatalf("expected stub response to contain summary, got %s", result)
	}
}

func TestOpenAIProvider_Generate_ResponseFormatRequirement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Check that response_format is set to json_object
		responseFormat, ok := reqBody["response_format"].(map[string]interface{})
		if !ok {
			t.Error("Expected response_format to be a map")
		}

		if responseFormat["type"] != "json_object" {
			t.Error("Expected response_format to be json_object")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"themes":["test"]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)
	provider.Generate(context.Background(), "test prompt")
}

func TestOpenAIProvider_Generate_MaxTokensAndReasoning(t *testing.T) {
	enabled := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if reqBody["max_tokens"] != float64(2048) {
			t.Fatalf("expected max_tokens=2048, got %#v", reqBody["max_tokens"])
		}
		if reqBody["include_reasoning"] != false {
			t.Fatalf("expected include_reasoning=false, got %#v", reqBody["include_reasoning"])
		}
		reasoning, ok := reqBody["reasoning"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected reasoning object, got %#v", reqBody["reasoning"])
		}
		if reasoning["enabled"] != true || reasoning["effort"] != "low" || reasoning["exclude"] != true {
			t.Fatalf("unexpected reasoning payload: %#v", reasoning)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").
		WithBaseURL(server.URL).
		WithMaxTokens(2048).
		WithReasoning(llm.ReasoningConfig{Enabled: &enabled, Effort: "low", Exclude: true})
	if _, err := provider.Generate(context.Background(), "test prompt"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
}

func TestOpenAIProvider_Generate_DropsDisableReasoningForMandatoryModel(t *testing.T) {
	enabled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if _, ok := reqBody["reasoning"]; ok {
			t.Fatalf("expected no reasoning object for mandatory reasoning model disable, got %#v", reqBody["reasoning"])
		}
		if _, ok := reqBody["include_reasoning"]; ok {
			t.Fatalf("expected no include_reasoning flag for mandatory reasoning model disable, got %#v", reqBody["include_reasoning"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "minimax/minimax-m2.7").
		WithBaseURL(server.URL).
		WithReasoning(llm.ReasoningConfig{Enabled: &enabled, Effort: "low", Exclude: true})
	if _, err := provider.Generate(context.Background(), "test prompt"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
}

func TestOpenAIProvider_Generate_RetriesAffordableMaxTokens(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if requests == 1 {
			if _, ok := reqBody["max_tokens"]; ok {
				t.Fatalf("expected first request without max_tokens, got %#v", reqBody["max_tokens"])
			}
			w.WriteHeader(http.StatusPaymentRequired)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "This request requires more credits, or fewer max_tokens. You requested up to 65536 tokens, but can only afford 31064.",
					"code":    402,
				},
			})
			return
		}

		if reqBody["max_tokens"] != float64(30040) {
			t.Fatalf("expected retry max_tokens=30040, got %#v", reqBody["max_tokens"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "minimax/minimax-m2.7").WithBaseURL(server.URL)
	if _, err := provider.Generate(context.Background(), "test prompt"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}
}

func TestOpenAIProvider_Generate_ReducesMaxTokensAfterAffordableRetryTimeout(t *testing.T) {
	var mu sync.Mutex
	requestedMaxTokens := []interface{}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		mu.Lock()
		requestedMaxTokens = append(requestedMaxTokens, reqBody["max_tokens"])
		mu.Unlock()

		switch reqBody["max_tokens"] {
		case nil:
			w.WriteHeader(http.StatusPaymentRequired)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "This request requires more credits, or fewer max_tokens. You requested up to 65536 tokens, but can only afford 31064.",
					"code":    402,
				},
			})
		case float64(30040):
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]interface{}{"content": `{"summary":{"overview":"slow","phases":[]},"sections":{},"citations":[]}`}},
				},
			})
		case float64(16384):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
				},
			})
		default:
			t.Fatalf("unexpected max_tokens retry: %#v", reqBody["max_tokens"])
		}
	}))
	defer server.Close()

	provider, err := llm.NewProvider(&llm.LLMConfig{
		Provider: "openai",
		OpenAI: llm.OpenAIConfig{
			APIKey:         "sk-test-key",
			Model:          "minimax/minimax-m2.7",
			BaseURL:        server.URL,
			RequestTimeout: 20 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	result, err := provider.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !strings.Contains(result, `"overview":"ok"`) {
		t.Fatalf("expected fallback result, got %s", result)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requestedMaxTokens) != 3 {
		t.Fatalf("expected 3 requests, got %d: %#v", len(requestedMaxTokens), requestedMaxTokens)
	}
}

func TestOpenAIProvider_Generate_ReasoningOnlyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"reasoning_content": "internal chain of thought"}},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "gpt-4").WithBaseURL(server.URL)
	_, err := provider.Generate(context.Background(), "test prompt")
	if err == nil || !strings.Contains(err.Error(), "reasoning/thinking") {
		t.Fatalf("expected reasoning-only error, got %v", err)
	}
}

func TestOpenAIProvider_Generate_ReasoningOnlyJSONFallback(t *testing.T) {
	expected := `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"reasoning_content": "final JSON:\n```json\n" + expected + "\n```"}},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewOpenAIProvider("sk-test-key", "minimax/minimax-m2.7").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if result != expected {
		t.Fatalf("expected reasoning JSON fallback %s, got %s", expected, result)
	}
}

func TestAnthropicProvider_Name(t *testing.T) {
	provider := llm.NewAnthropicProvider("sk-ant-test-key", "claude-3-opus-20240229")
	name := provider.Name()

	if name != "anthropic" {
		t.Errorf("Name mismatch: got %s, want %s", name, "anthropic")
	}
}

func TestAnthropicProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		model    string
		expected bool
	}{
		{"with valid API key", "sk-ant-test-key", "claude-3-opus", true},
		{"with empty API key", "", "claude-3-opus", false},
		{"with empty model", "sk-ant-test-key", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := llm.NewAnthropicProvider(tt.apiKey, tt.model)
			available := provider.IsAvailable()

			if available != tt.expected {
				t.Errorf("Availability mismatch: got %v, want %v", available, tt.expected)
			}
		})
	}
}

func TestAnthropicProvider_Generate_Success(t *testing.T) {
	expectedResponse := `{
		"themes": ["discussion1"],
		"contents_reported": [{"text": "point1", "timestamp": 100}],
		"progress_issues": {"progress": ["progress1"], "issues": ["issue1"]},
		"next_steps": ["step1"]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "sk-ant-test-key" {
			t.Error("Expected x-api-key header")
		}
		if r.Header.Get("Anthropic-Version") != "2023-06-01" {
			t.Error("Expected anthropic-version header")
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		model, ok := reqBody["model"].(string)
		if !ok {
			t.Error("model field is not a string")
			return
		}
		if model != "claude-3-opus-20240229" {
			t.Errorf("Expected model claude-3-opus-20240229, got %s", model)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": expectedResponse,
				},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewAnthropicProvider("sk-ant-test-key", "claude-3-opus-20240229").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Errorf("Generate failed: %v", err)
	}
	if result != expectedResponse {
		t.Errorf("Response mismatch:\ngot:      %s\nwant:     %s", result, expectedResponse)
	}
}

func TestAnthropicProvider_Generate_CustomMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if reqBody["max_tokens"] != float64(8192) {
			t.Fatalf("expected max_tokens=8192, got %#v", reqBody["max_tokens"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewAnthropicProvider("sk-ant-test-key", "claude-3-opus-20240229").
		WithBaseURL(server.URL).
		WithMaxTokens(8192)
	if _, err := provider.Generate(context.Background(), "test prompt"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
}

func TestAnthropicProvider_Generate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "response",
				},
			},
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	provider := llm.NewAnthropicProvider("sk-ant-test-key", "claude-3-opus-20240229").WithBaseURL(server.URL)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := provider.Generate(ctx, "test prompt")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context cancellation, got: %v", err)
	}
}

func TestAnthropicProvider_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	provider := llm.NewAnthropicProvider("sk-ant-test-key", "claude-3-opus-20240229").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected error for HTTP 401")
	}
	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}
	if !strings.Contains(err.Error(), "anthropic API error") {
		t.Error("Expected error to mention anthropic API error")
	}
}

func TestAnthropicProvider_Generate_NetworkError(t *testing.T) {
	provider := llm.NewAnthropicProvider("sk-ant-test-key", "claude-3-opus-20240229").WithBaseURL("http://localhost:1")

	_, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected network error")
	}
	if !strings.Contains(err.Error(), "failed to send request") {
		t.Errorf("Expected network error message, got: %v", err)
	}
}

func TestAnthropicProvider_Generate_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	provider := llm.NewAnthropicProvider("sk-ant-test-key", "claude-3-opus-20240229").WithBaseURL(server.URL)
	result, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected error for empty content")
	}
	if result != "" {
		t.Error("Expected empty result on error")
	}
}

func TestAzureOpenAIProvider_Name(t *testing.T) {
	provider := llm.NewAzureOpenAIProvider("azure-key", "https://openai.openai.azure.com/", "gpt-4-deployment")
	name := provider.Name()

	if name != "azure" {
		t.Errorf("Name mismatch: got %s, want %s", name, "azure")
	}
}

func TestAzureOpenAIProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		endpoint   string
		deployment string
		expected   bool
	}{
		{"with all credentials", "azure-key", "https://openai.openai.azure.com/", "gpt-4", true},
		{"with empty API key", "", "https://openai.openai.azure.com/", "gpt-4", false},
		{"with empty endpoint", "azure-key", "", "gpt-4", false},
		{"with empty deployment", "azure-key", "https://openai.openai.azure.com/", "", false},
		{"all empty", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := llm.NewAzureOpenAIProvider(tt.apiKey, tt.endpoint, tt.deployment)
			available := provider.IsAvailable()

			if available != tt.expected {
				t.Errorf("Availability mismatch: got %v, want %v", available, tt.expected)
			}
		})
	}
}

func TestAzureOpenAIProvider_Generate_Success(t *testing.T) {
	expectedResponse := `{
		"themes": ["discussion1"],
		"contents_reported": [{"text": "point1", "timestamp": 100}],
		"progress_issues": {"progress": ["progress1"], "issues": ["issue1"]},
		"next_steps": ["step1"]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "openai/deployments/gpt-4-deployment/chat/completions") {
			t.Errorf("Expected deployment path, got %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "api-version=2024-02-15-preview") {
			t.Errorf("Expected api-version query param, got %s", r.URL.RawQuery)
		}
		if r.Header.Get("Api-Key") != "azure-key" {
			t.Error("Expected api-key header")
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		responseFormat, ok := reqBody["response_format"].(map[string]interface{})
		if !ok {
			t.Error("Expected response_format to be a map")
		} else if responseFormat["type"] != "json_object" {
			t.Error("Expected response_format to be json_object")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": expectedResponse,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := llm.NewAzureOpenAIProvider("azure-key", server.URL, "gpt-4-deployment")
	result, err := provider.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Errorf("Generate failed: %v", err)
	}
	if result != expectedResponse {
		t.Errorf("Response mismatch:\ngot:      %s\nwant:     %s", result, expectedResponse)
	}
}

func TestAzureOpenAIProvider_Generate_MaxTokensAndReasoning(t *testing.T) {
	enabled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if reqBody["max_tokens"] != float64(1024) {
			t.Fatalf("expected max_tokens=1024, got %#v", reqBody["max_tokens"])
		}
		reasoning, ok := reqBody["reasoning"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected reasoning object, got %#v", reqBody["reasoning"])
		}
		if reasoning["enabled"] != false || reasoning["effort"] != "medium" {
			t.Fatalf("unexpected reasoning payload: %#v", reasoning)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
			},
		})
	}))
	defer server.Close()

	cfg := &llm.LLMConfig{
		Provider: "azure",
		Azure: llm.AzureLLMConfig{
			APIKey:     "azure-key",
			Endpoint:   server.URL,
			Deployment: "gpt-4-deployment",
			MaxTokens:  1024,
			Reasoning:  llm.ReasoningConfig{Enabled: &enabled, Effort: "medium"},
		},
	}
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}
	if _, err := provider.Generate(context.Background(), "test prompt"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
}

func TestAzureOpenAIProvider_Generate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "response",
					},
				},
			},
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	provider := llm.NewAzureOpenAIProvider("azure-key", server.URL, "gpt-4-deployment")

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := provider.Generate(ctx, "test prompt")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context cancellation, got: %v", err)
	}
}

func TestAzureOpenAIProvider_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	provider := llm.NewAzureOpenAIProvider("azure-key", server.URL, "gpt-4-deployment")
	result, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected error for HTTP 401")
	}
	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}
	if !strings.Contains(err.Error(), "azure OpenAI API error") {
		t.Error("Expected error to mention azure OpenAI API error")
	}
}

func TestAzureOpenAIProvider_Generate_NetworkError(t *testing.T) {
	provider := llm.NewAzureOpenAIProvider("azure-key", "http://localhost:1", "gpt-4-deployment")

	_, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected network error")
	}
	if !strings.Contains(err.Error(), "failed to send request") {
		t.Errorf("Expected network error message, got: %v", err)
	}
}

func TestAzureOpenAIProvider_Generate_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	provider := llm.NewAzureOpenAIProvider("azure-key", server.URL, "gpt-4-deployment")
	result, err := provider.Generate(context.Background(), "test prompt")

	if err == nil {
		t.Error("Expected error for empty response")
	}
	if result != "" {
		t.Error("Expected empty result on error")
	}
}

func TestLLMConfig_NewProvider(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		hasProvider bool
		err         bool
	}{
		{"openai provider", "openai", true, false},
		{"anthropic provider", "anthropic", true, false},
		{"azure provider", "azure", true, false},
		{"unsupported provider", "unsupported", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &llm.LLMConfig{
				Provider: tt.provider,
				OpenAI: llm.OpenAIConfig{
					APIKey: "sk-test",
					Model:  "gpt-4",
				},
			}

			provider, err := llm.NewProvider(cfg)

			if tt.err {
				if err == nil {
					t.Error("Expected error for unsupported provider")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected provider %s, got error: %v", tt.provider, err)
				return
			}

			if tt.hasProvider {
				if provider == nil {
					t.Error("Expected provider to be created")
				}
			} else {
				if provider != nil {
					t.Error("Expected nil provider for unsupported type")
				}
			}
		})
	}
}

func TestLLMProviderInterface_Implementation(t *testing.T) {
	tests := []struct {
		provider llm.LLMProvider
		name     string
	}{
		{name: "OpenAI", provider: llm.NewOpenAIProvider("test", "gpt-4")},
		{name: "Anthropic", provider: llm.NewAnthropicProvider("test", "claude-3-opus")},
		{name: "Azure", provider: llm.NewAzureOpenAIProvider("key", "endpoint", "deployment")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.provider.Name() == "" {
				t.Error("Name() should return non-empty string")
			}
			if !tt.provider.IsAvailable() {
				t.Error("IsAvailable() should return true with valid credentials")
			}
		})
	}
}

func TestOpenAIProvider_EmptyCredentials(t *testing.T) {
	provider := llm.NewOpenAIProvider("", "")

	if provider.IsAvailable() {
		t.Error("Provider should not be available with empty credentials")
	}
}

func TestAnthropicProvider_EmptyCredentials(t *testing.T) {
	provider := llm.NewAnthropicProvider("", "")

	if provider.IsAvailable() {
		t.Error("Provider should not be available with empty credentials")
	}
}

func TestAzureOpenAIProvider_EmptyCredentials(t *testing.T) {
	provider := llm.NewAzureOpenAIProvider("", "", "")

	if provider.IsAvailable() {
		t.Error("Provider should not be available with empty credentials")
	}
}
