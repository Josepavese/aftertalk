package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
)

func TestOllamaProvider_Generate_DefaultDisablesThinkForThinkingModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("expected /api/generate, got %s", r.URL.Path)
		}
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if reqBody["think"] != false {
			t.Fatalf("expected think=false for qwen3.5 default, got %#v", reqBody["think"])
		}
		if reqBody["format"] != "json" {
			t.Fatalf("expected format=json, got %#v", reqBody["format"])
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"response":"{\"summary\":{\"overview\":\"ok\",\"phases\":[]},\"sections\":{},\"citations\":[]}","done":true}` + "\n"))
	}))
	defer server.Close()

	provider, err := llm.NewOllamaProvider(llm.OllamaConfig{BaseURL: server.URL, Model: "qwen3.5:7b"})
	if err != nil {
		t.Fatalf("NewOllamaProvider failed: %v", err)
	}
	got, err := provider.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty response")
	}
}

func TestOllamaProvider_Generate_ReasoningOnlyError(t *testing.T) {
	think := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"thinking":"draft thoughts","done":true}` + "\n"))
	}))
	defer server.Close()

	provider, err := llm.NewOllamaProvider(llm.OllamaConfig{BaseURL: server.URL, Model: "qwen3.5:7b", Think: &think})
	if err != nil {
		t.Fatalf("NewOllamaProvider failed: %v", err)
	}
	_, err = provider.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected reasoning-only error")
	}
}
