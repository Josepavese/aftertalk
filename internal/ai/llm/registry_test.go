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
