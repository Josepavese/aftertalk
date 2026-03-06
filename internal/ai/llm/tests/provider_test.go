package tests

import (
	"testing"

	"github.com/flowup/aftertalk/internal/ai/llm"
)

func TestLLMProviderInterface(t *testing.T) {
	// Test interface methods by checking provider types
	provider := &llm.OpenAIProvider{}

	// Check that methods exist by calling them
	name := provider.Name()
	if name != "openai" {
		t.Errorf("Name() should return 'openai', got: %s", name)
	}

	// IsAvailable should return true with valid API key
	available := provider.IsAvailable()
	if !available {
		t.Error("IsAvailable() should return true with valid API key")
	}
}

func TestMinutesPrompt(t *testing.T) {
	prompt := llm.MinutesPrompt{
		SessionID:         "session1",
		TranscriptionText: "transcript content",
		ParticipantRoles:  []string{"user", "moderator"},
	}

	if prompt.SessionID != "session1" {
		t.Errorf("SessionID mismatch: got %s, want %s", prompt.SessionID, "session1")
	}
	if prompt.TranscriptionText != "transcript content" {
		t.Errorf("TranscriptionText mismatch: got %s, want %s", prompt.TranscriptionText, "transcript content")
	}
	if len(prompt.ParticipantRoles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(prompt.ParticipantRoles))
	}
}

func TestOpenAIConfig(t *testing.T) {
	cfg := llm.OpenAIConfig{
		APIKey: "sk-test-key",
		Model:  "gpt-4",
	}

	if cfg.APIKey != "sk-test-key" {
		t.Errorf("APIKey mismatch")
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("Model mismatch: got %s, want %s", cfg.Model, "gpt-4")
	}
}

func TestAnthropicConfig(t *testing.T) {
	cfg := llm.AnthropicConfig{
		APIKey: "sk-ant-test-key",
		Model:  "claude-3-opus-20240229",
	}

	if cfg.APIKey != "sk-ant-test-key" {
		t.Errorf("APIKey mismatch")
	}
	if cfg.Model != "claude-3-opus-20240229" {
		t.Errorf("Model mismatch: got %s, want %s", cfg.Model, "claude-3-opus-20240229")
	}
}

func TestAzureLLMConfig(t *testing.T) {
	cfg := llm.AzureLLMConfig{
		APIKey:     "azure-key",
		Endpoint:   "https://openai.openai.azure.com/",
		Deployment: "gpt-4-deployment",
	}

	if cfg.APIKey != "azure-key" {
		t.Errorf("APIKey mismatch")
	}
	if cfg.Endpoint != "https://openai.openai.azure.com/" {
		t.Errorf("Endpoint mismatch: got %s, want %s", cfg.Endpoint, "https://openai.openai.azure.com/")
	}
	if cfg.Deployment != "gpt-4-deployment" {
		t.Errorf("Deployment mismatch: got %s, want %s", cfg.Deployment, "gpt-4-deployment")
	}
}

func TestLLMConfig(t *testing.T) {
	cfg := &llm.LLMConfig{
		Provider: "openai",
		OpenAI: llm.OpenAIConfig{
			APIKey: "sk-test",
			Model:  "gpt-4",
		},
		Anthropic: llm.AnthropicConfig{
			APIKey: "sk-ant-test",
			Model:  "claude-3-opus",
		},
		Azure: llm.AzureLLMConfig{
			APIKey:     "azure-key",
			Endpoint:   "https://openai.openai.azure.com/",
			Deployment: "gpt-4",
		},
	}

	if cfg.Provider != "openai" {
		t.Errorf("Provider mismatch")
	}
	if cfg.OpenAI.APIKey != "sk-test" {
		t.Errorf("OpenAI APIKey mismatch")
	}
	if cfg.OpenAI.Model != "gpt-4" {
		t.Errorf("OpenAI Model mismatch")
	}
	if cfg.Anthropic.APIKey != "sk-ant-test" {
		t.Errorf("Anthropic APIKey mismatch")
	}
	if cfg.Azure.APIKey != "azure-key" {
		t.Errorf("Azure APIKey mismatch")
	}
}

func TestMinutesResponse(t *testing.T) {
	response := &llm.MinutesResponse{
		Themes:                    []string{"theme1", "theme2"},
		ContentsReported:          []llm.ContentItem{{Text: "content1", Timestamp: 100}},
		ProfessionalInterventions: []llm.ContentItem{{Text: "intervention1", Timestamp: 200}},
		ProgressIssues:            llm.Progress{Progress: []string{"progress1"}, Issues: []string{"issue1"}},
		NextSteps:                 []string{"step1", "step2"},
		Citations: []llm.Citation{
			{TimestampMs: 100, Text: "quote1", Role: "user"},
			{TimestampMs: 200, Text: "quote2", Role: "moderator"},
		},
	}

	if len(response.Themes) != 2 {
		t.Errorf("Expected 2 themes, got %d", len(response.Themes))
	}
	if len(response.ContentsReported) != 1 {
		t.Errorf("Expected 1 content, got %d", len(response.ContentsReported))
	}
	if len(response.Citations) != 2 {
		t.Errorf("Expected 2 citations, got %d", len(response.Citations))
	}
}

func TestContentItem(t *testing.T) {
	item := llm.ContentItem{
		Text:      "important point",
		Timestamp: 500,
	}

	if item.Text != "important point" {
		t.Errorf("Text mismatch: got %s, want %s", item.Text, "important point")
	}
	if item.Timestamp != 500 {
		t.Errorf("Timestamp mismatch: got %d, want %d", item.Timestamp, 500)
	}
}

func TestProgress(t *testing.T) {
	progress := llm.Progress{
		Progress: []string{"progress1", "progress2"},
		Issues:   []string{"issue1", "issue2"},
	}

	if len(progress.Progress) != 2 {
		t.Errorf("Expected 2 progress items, got %d", len(progress.Progress))
	}
	if len(progress.Issues) != 2 {
		t.Errorf("Expected 2 issues, got %d", len(progress.Issues))
	}
}

func TestCitation(t *testing.T) {
	citation := llm.Citation{
		TimestampMs: 1000,
		Text:        "exact quote from conversation",
		Role:        "moderator",
	}

	if citation.TimestampMs != 1000 {
		t.Errorf("TimestampMs mismatch: got %d, want %d", citation.TimestampMs, 1000)
	}
	if citation.Text != "exact quote from conversation" {
		t.Errorf("Text mismatch: got %s, want %s", citation.Text, "exact quote from conversation")
	}
	if citation.Role != "moderator" {
		t.Errorf("Role mismatch: got %s, want %s", citation.Role, "moderator")
	}
}

func TestParseMinutesResponse_Success(t *testing.T) {
	jsonStr := `{
		"themes": ["theme1", "theme2"],
		"contents_reported": [
			{"text": "point1", "timestamp": 100}
		],
		"professional_interventions": [
			{"text": "intervention1", "timestamp": 200}
		],
		"progress_issues": {
			"progress": ["progress1"],
			"issues": ["issue1"]
		},
		"next_steps": ["step1", "step2"],
		"citations": [
			{"timestamp_ms": 100, "text": "quote1", "role": "user"}
		]
	}`

	response, err := llm.ParseMinutesResponse(jsonStr)

	if err != nil {
		t.Fatalf("Expected success parsing valid JSON, got error: %v", err)
	}

	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if len(response.Themes) != 2 {
		t.Errorf("Expected 2 themes, got %d", len(response.Themes))
	}
	if len(response.ContentsReported) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(response.ContentsReported))
	}
	if len(response.Citations) != 1 {
		t.Errorf("Expected 1 citation, got %d", len(response.Citations))
	}
	if len(response.ProgressIssues.Progress) != 2 {
		t.Errorf("Expected 2 progress items, got %d", len(response.ProgressIssues.Progress))
	}
	if len(response.ProgressIssues.Issues) != 2 {
		t.Errorf("Expected 2 issues, got %d", len(response.ProgressIssues.Issues))
	}
}

func TestParseMinutesResponse_EmptyStrings(t *testing.T) {
	jsonStr := `{
		"themes": [],
		"contents_reported": [],
		"professional_interventions": [],
		"progress_issues": {
			"progress": [],
			"issues": []
		},
		"next_steps": [],
		"citations": []
	}`

	response, err := llm.ParseMinutesResponse(jsonStr)

	if err != nil {
		t.Fatalf("Expected success parsing valid JSON, got error: %v", err)
	}

	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if len(response.Themes) != 0 {
		t.Errorf("Expected 0 themes, got %d", len(response.Themes))
	}
	if len(response.ContentsReported) != 0 {
		t.Errorf("Expected 0 content items, got %d", len(response.ContentsReported))
	}
}

func TestParseMinutesResponse_InvalidJSON(t *testing.T) {
	jsonStr := `{"invalid": json}`

	_, err := llm.ParseMinutesResponse(jsonStr)

	if err == nil {
		t.Error("Expected error parsing invalid JSON")
	}
}

func TestParseMinutesResponse_MalformedJSON(t *testing.T) {
	jsonStr := `{"themes": ["theme", invalid_syntax], "next_steps": ["step"]}`

	_, err := llm.ParseMinutesResponse(jsonStr)

	if err == nil {
		t.Error("Expected error parsing malformed JSON")
	}
}

func TestParseMinutesResponse_MinimalValid(t *testing.T) {
	jsonStr := `{
		"themes": [],
		"contents_reported": [],
		"professional_interventions": [],
		"progress_issues": {
			"progress": [],
			"issues": []
		},
		"next_steps": [],
		"citations": []
	}`

	response, err := llm.ParseMinutesResponse(jsonStr)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if response.Themes != nil || response.ContentsReported != nil ||
		response.ProfessionalInterventions != nil || response.Citations != nil {
		t.Error("Expected nil slices in minimal response")
	}
	if response.ProgressIssues.Progress != nil || response.ProgressIssues.Issues != nil {
		t.Error("Expected nil slices in progress issues")
	}
}

func TestParseMinutesResponse_WithAllFields(t *testing.T) {
	jsonStr := `{
		"themes": ["discussion1", "discussion2"],
		"contents_reported": [
			{"text": "item1", "timestamp": 100},
			{"text": "item2", "timestamp": 200}
		],
		"professional_interventions": [
			{"text": "intervention1", "timestamp": 300}
		],
		"progress_issues": {
			"progress": ["progress1", "progress2"],
			"issues": ["issue1", "issue2"]
		},
		"next_steps": ["step1", "step2", "step3"],
		"citations": [
			{"timestamp_ms": 100, "text": "quote1", "role": "user"},
			{"timestamp_ms": 150, "text": "quote2", "role": "moderator"},
			{"timestamp_ms": 200, "text": "quote3", "role": "user"},
			{"timestamp_ms": 250, "text": "quote4", "role": "moderator"},
			{"timestamp_ms": 300, "text": "quote5", "role": "user"}
		]
	}`

	response, err := llm.ParseMinutesResponse(jsonStr)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	tests := []struct {
		field    string
		expected int
	}{
		{"themes", 2},
		{"contents_reported", 2},
		{"professional_interventions", 1},
		{"progress_issues.progress", 2},
		{"progress_issues.issues", 2},
		{"next_steps", 3},
		{"citations", 5},
	}

	for _, tt := range tests {
		field := tt.field
		expected := tt.expected

		switch field {
		case "themes":
			if len(response.Themes) != expected {
				t.Errorf("Expected %d themes, got %d", expected, len(response.Themes))
			}
		case "contents_reported":
			if len(response.ContentsReported) != expected {
				t.Errorf("Expected %d content items, got %d", expected, len(response.ContentsReported))
			}
		case "professional_interventions":
			if len(response.ProfessionalInterventions) != expected {
				t.Errorf("Expected %d interventions, got %d", expected, len(response.ProfessionalInterventions))
			}
		case "progress_issues.progress":
			if len(response.ProgressIssues.Progress) != expected {
				t.Errorf("Expected %d progress items, got %d", expected, len(response.ProgressIssues.Progress))
			}
		case "progress_issues.issues":
			if len(response.ProgressIssues.Issues) != expected {
				t.Errorf("Expected %d issues, got %d", expected, len(response.ProgressIssues.Issues))
			}
		case "next_steps":
			if len(response.NextSteps) != expected {
				t.Errorf("Expected %d next steps, got %d", expected, len(response.NextSteps))
			}
		case "citations":
			if len(response.Citations) != expected {
				t.Errorf("Expected %d citations, got %d", expected, len(response.Citations))
			}
		}
	}
}
