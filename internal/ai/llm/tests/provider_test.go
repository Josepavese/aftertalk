package tests

import (
	"encoding/json"
	"testing"

	"github.com/flowup/aftertalk/internal/ai/llm"
)

func TestLLMProviderInterface(t *testing.T) {
	provider := llm.NewOpenAIProvider("sk-valid-key", "gpt-4")

	if provider.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "openai")
	}
	if !provider.IsAvailable() {
		t.Error("IsAvailable() should return true with valid API key")
	}
}

func TestOpenAIConfig(t *testing.T) {
	cfg := llm.OpenAIConfig{APIKey: "sk-test", Model: "gpt-4"}
	if cfg.APIKey != "sk-test" || cfg.Model != "gpt-4" {
		t.Error("OpenAIConfig fields mismatch")
	}
}

func TestAnthropicConfig(t *testing.T) {
	cfg := llm.AnthropicConfig{APIKey: "sk-ant", Model: "claude-3-opus"}
	if cfg.APIKey != "sk-ant" || cfg.Model != "claude-3-opus" {
		t.Error("AnthropicConfig fields mismatch")
	}
}

func TestCitation(t *testing.T) {
	c := llm.Citation{TimestampMs: 1000, Text: "quote", Role: "therapist"}
	if c.TimestampMs != 1000 || c.Text != "quote" || c.Role != "therapist" {
		t.Error("Citation fields mismatch")
	}
}

func TestParseMinutesDynamic_Success(t *testing.T) {
	raw := `{
		"sections": {
			"themes": ["topic1","topic2"],
			"contents_reported": [{"text":"point1","timestamp":100}],
			"next_steps": ["action1"]
		},
		"citations": [
			{"timestamp_ms": 500, "text": "exact quote", "role": "therapist"}
		]
	}`

	r, err := llm.ParseMinutesDynamic(raw)
	if err != nil {
		t.Fatalf("ParseMinutesDynamic error: %v", err)
	}
	if len(r.Sections) != 3 {
		t.Errorf("Expected 3 sections, got %d", len(r.Sections))
	}
	if len(r.Citations) != 1 {
		t.Errorf("Expected 1 citation, got %d", len(r.Citations))
	}
	if r.Citations[0].TimestampMs != 500 {
		t.Errorf("Expected TimestampMs=500, got %d", r.Citations[0].TimestampMs)
	}
}

func TestParseMinutesDynamic_Empty(t *testing.T) {
	raw := `{"sections":{},"citations":[]}`
	r, err := llm.ParseMinutesDynamic(raw)
	if err != nil {
		t.Fatalf("ParseMinutesDynamic error: %v", err)
	}
	if r.Sections == nil {
		t.Error("Sections should not be nil")
	}
	if r.Citations == nil {
		t.Error("Citations should not be nil")
	}
}

func TestParseMinutesDynamic_InvalidJSON(t *testing.T) {
	_, err := llm.ParseMinutesDynamic(`not json`)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseMinutesDynamic_SectionValues(t *testing.T) {
	raw := `{
		"sections": {
			"themes": ["a","b"],
			"progress_issues": {"progress":["p1"],"issues":["i1"]}
		},
		"citations": []
	}`
	r, err := llm.ParseMinutesDynamic(raw)
	if err != nil {
		t.Fatalf("ParseMinutesDynamic error: %v", err)
	}

	var themes []string
	if err := json.Unmarshal(r.Sections["themes"], &themes); err != nil {
		t.Fatalf("Failed to unmarshal themes: %v", err)
	}
	if len(themes) != 2 {
		t.Errorf("Expected 2 themes, got %d", len(themes))
	}
}
