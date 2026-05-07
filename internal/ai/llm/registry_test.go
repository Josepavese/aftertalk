package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
)

func TestLLMRegistry_ProfileOverridesProviderConfig(t *testing.T) {
	reasoningEnabled := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if reqBody["model"] != "openrouter/minimax/minimax-m2.7" {
			t.Fatalf("unexpected model: %#v", reqBody["model"])
		}
		if reqBody["max_tokens"] != float64(1536) {
			t.Fatalf("unexpected max_tokens: %#v", reqBody["max_tokens"])
		}
		reasoning, ok := reqBody["reasoning"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected reasoning object, got %#v", reqBody["reasoning"])
		}
		if reasoning["enabled"] != true || reasoning["effort"] != "low" {
			t.Fatalf("unexpected reasoning: %#v", reasoning)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
			},
		})
	}))
	defer server.Close()

	registry, err := llm.NewLLMRegistry(&config.LLMConfig{
		Provider:       "ollama",
		DefaultProfile: "local",
		Ollama:         config.OllamaLLMConfig{BaseURL: "http://localhost:11434", Model: "gemma3:4b"},
		OpenAI:         config.OpenAIConfig{APIKey: "sk-global", Model: "gpt-4o", BaseURL: "https://example.invalid", MaxTokens: 4096},
		Profiles: map[string]config.LLMProfileConfig{
			"local": {
				Provider: "ollama",
				Model:    "gemma3:4b",
			},
			"cloud": {
				Provider:       "openai",
				APIKey:         "sk-profile",
				BaseURL:        server.URL,
				Model:          "openrouter/minimax/minimax-m2.7",
				MaxTokens:      1536,
				RequestTimeout: time.Second,
				Reasoning: config.ReasoningConfig{
					Enabled: &reasoningEnabled,
					Effort:  "low",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewLLMRegistry failed: %v", err)
	}

	if _, err := registry.Get("cloud").Generate(context.Background(), "test prompt"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
}

func TestLLMRegistry_ProfileRuntimeTuning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(25 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`}},
			},
		})
	}))
	defer server.Close()

	registry, err := llm.NewLLMRegistry(&config.LLMConfig{
		Provider:       "stub",
		DefaultProfile: "local",
		OpenAI: config.OpenAIConfig{
			APIKey:         "sk-global",
			Model:          "gpt-4o",
			BaseURL:        server.URL,
			RequestTimeout: time.Millisecond,
		},
		Profiles: map[string]config.LLMProfileConfig{
			"local": {Provider: "stub"},
			"cloud": {
				Provider:          "openai",
				Model:             "openrouter/minimax/minimax-m2.7",
				RequestTimeout:    200 * time.Millisecond,
				GenerationTimeout: 20 * time.Minute,
				Retry: config.RetryPolicyConfig{
					MaxAttempts:    4,
					InitialBackoff: 2 * time.Second,
					MaxBackoff:     30 * time.Second,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewLLMRegistry failed: %v", err)
	}

	cloud := registry.Get("cloud")
	profileProvider, ok := cloud.(llm.ProfileNameProvider)
	if !ok || profileProvider.ProfileName() != "cloud" {
		t.Fatalf("expected cloud provider profile name, got %#v", cloud)
	}
	runtimeProvider, ok := cloud.(llm.RuntimeConfigProvider)
	if !ok {
		t.Fatal("expected cloud provider to expose runtime config")
	}
	runtime := runtimeProvider.RuntimeConfig()
	if runtime.GenerationTimeout != 20*time.Minute {
		t.Fatalf("unexpected generation timeout: %v", runtime.GenerationTimeout)
	}
	if runtime.Retry.MaxAttempts != 4 || runtime.Retry.InitialBackoff != 2*time.Second || runtime.Retry.MaxBackoff != 30*time.Second {
		t.Fatalf("unexpected retry runtime config: %#v", runtime.Retry)
	}
	if !runtime.Budget.IsZero() {
		t.Fatalf("unexpected budget runtime config: %#v", runtime.Budget)
	}
	if _, err := cloud.Generate(context.Background(), "test prompt"); err != nil {
		t.Fatalf("profile request_timeout was not applied: %v", err)
	}

	if _, ok := registry.Get("local").(llm.RuntimeConfigProvider); ok {
		t.Fatal("local profile should not expose runtime config when none is configured")
	}
}

func TestLLMRegistry_ProfileBudgetRuntime(t *testing.T) {
	registry, err := llm.NewLLMRegistry(&config.LLMConfig{
		Provider:       "stub",
		DefaultProfile: "cloud",
		Budget: config.LLMBudgetConfig{
			MaxSessionCostCredits: 0.5,
			MaxDailyCostCredits:   2.0,
		},
		Profiles: map[string]config.LLMProfileConfig{
			"cloud": {
				Provider: "stub",
				Budget: config.LLMBudgetConfig{
					MaxSessionCostCredits: 0.25,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewLLMRegistry failed: %v", err)
	}
	runtimeProvider, ok := registry.Get("cloud").(llm.RuntimeConfigProvider)
	if !ok {
		t.Fatal("expected budgeted profile to expose runtime config")
	}
	budget := runtimeProvider.RuntimeConfig().Budget
	if budget.MaxSessionCostCredits != 0.25 || budget.MaxDailyCostCredits != 0 {
		t.Fatalf("unexpected profile budget: %#v", budget)
	}
}
