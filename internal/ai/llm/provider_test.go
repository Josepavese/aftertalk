package llm_test

import (
	"encoding/json"
	"testing"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
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

func TestSummary(t *testing.T) {
	s := llm.Summary{
		Overview: "Overview",
		Phases: []llm.Phase{
			{Title: "Opening", Summary: "Started", StartMs: 0, EndMs: 1000},
		},
	}
	if s.Overview != "Overview" || len(s.Phases) != 1 {
		t.Error("Summary fields mismatch")
	}
}

func TestParseMinutesDynamic_Success(t *testing.T) {
	raw := `{
		"summary": {
			"overview": "short summary",
			"phases": [
				{"title":"Opening","summary":"Started","start_ms":0,"end_ms":1000}
			]
		},
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
	if r.Summary.Overview != "short summary" {
		t.Errorf("Expected summary overview, got %q", r.Summary.Overview)
	}
	if len(r.Summary.Phases) != 1 {
		t.Errorf("Expected 1 phase, got %d", len(r.Summary.Phases))
	}
	if len(r.Citations) != 1 {
		t.Errorf("Expected 1 citation, got %d", len(r.Citations))
	}
	if r.Citations[0].TimestampMs != 500 {
		t.Errorf("Expected TimestampMs=500, got %d", r.Citations[0].TimestampMs)
	}
}

func TestParseMinutesDynamic_Empty(t *testing.T) {
	raw := `{"summary":{"overview":"","phases":[]},"sections":{},"citations":[]}`
	r, err := llm.ParseMinutesDynamic(raw)
	if err != nil {
		t.Fatalf("ParseMinutesDynamic error: %v", err)
	}
	if r.Sections == nil {
		t.Error("Sections should not be nil")
	}
	if r.Summary.Phases == nil {
		t.Error("Summary phases should not be nil")
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

func TestParseMinutesDynamic_SanitizesCodeFenceAndTrailingCommas(t *testing.T) {
	raw := "```json\n{\n  \"summary\": {\"overview\": \"ok\", \"phases\": []},\n  \"sections\": {\n    \"themes\": [\"a\", \"b\"],\n  },\n  \"citations\": [],\n}\n```"
	r, err := llm.ParseMinutesDynamic(raw)
	if err != nil {
		t.Fatalf("ParseMinutesDynamic error: %v", err)
	}
	if r.Summary.Overview != "ok" {
		t.Errorf("Expected overview 'ok', got %q", r.Summary.Overview)
	}
}

func TestParseMinutesDynamic_StripsJunkAroundJSON(t *testing.T) {
	raw := "Some text before {\"summary\":{\"overview\":\"ok\",\"phases\":[]},\"sections\":{},\"citations\":[]} trailing"
	r, err := llm.ParseMinutesDynamic(raw)
	if err != nil {
		t.Fatalf("ParseMinutesDynamic error: %v", err)
	}
	if r.Summary.Overview != "ok" {
		t.Errorf("Expected overview 'ok', got %q", r.Summary.Overview)
	}
}

func TestParseMinutesDynamic_ExtractsFirstBalancedJSONObject(t *testing.T) {
	raw := "prefix {\"summary\":{\"overview\":\"ok } inside string\",\"phases\":[]},\"sections\":{},\"citations\":[]} trailing {not-json}"
	r, err := llm.ParseMinutesDynamic(raw)
	if err != nil {
		t.Fatalf("ParseMinutesDynamic error: %v", err)
	}
	if r.Summary.Overview != "ok } inside string" {
		t.Errorf("Expected balanced object extraction, got %q", r.Summary.Overview)
	}
}

func TestParseMinutesDynamic_StripsBOM(t *testing.T) {
	raw := "\ufeff{\"summary\":{\"overview\":\"ok\",\"phases\":[]},\"sections\":{},\"citations\":[]}"
	r, err := llm.ParseMinutesDynamic(raw)
	if err != nil {
		t.Fatalf("ParseMinutesDynamic error: %v", err)
	}
	if r.Summary.Overview != "ok" {
		t.Errorf("Expected overview 'ok', got %q", r.Summary.Overview)
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
