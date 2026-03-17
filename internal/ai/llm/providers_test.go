package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
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
		if !strings.Contains(r.URL.RawQuery, "api-version=2023-05-15") {
			t.Errorf("Expected api-version query param, got %s", r.URL.RawQuery)
		}
		if r.Header.Get("Api-Key") != "azure-key" {
			t.Error("Expected api-key header")
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
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
