package tests

import (
	"testing"

	"github.com/flowup/aftertalk/internal/ai/llm"
)

func TestGenerateMinutesPrompt(t *testing.T) {
	transcriptionText := "Test conversation transcript with various points and questions."
	roles := []string{"user", "moderator"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	tests := []struct {
		name     string
		pattern  string
		contains bool
	}{
		{"session reference", "session1", true},
		{"transcription content", transcriptionText, true},
		{"participant roles", "user and moderator", true},
		{"JSON structure", `"themes"`, true},
		{"JSON structure", `"contents_reported"`, true},
		{"JSON structure", `"professional_interventions"`, true},
		{"JSON structure", `"progress_issues"`, true},
		{"JSON structure", `"next_steps"`, true},
		{"JSON structure", `"citations"`, true},
		{"DO NOT make diagnoses", "Do NOT make diagnoses", true},
		{"factual tone", "factual tone", true},
		{"timestamp in milliseconds", "milliseconds", true},
		{"valid JSON", "Respond ONLY with valid JSON", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.contains {
				if !contains(prompt, tt.pattern) {
					t.Errorf("Expected prompt to contain %s", tt.pattern)
				}
			}
		})
	}
}

func TestGenerateMinutesPrompt_SingleRole(t *testing.T) {
	transcriptionText := "Single participant conversation."
	roles := []string{"user"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	if !contains(prompt, "user") {
		t.Error("Expected prompt to contain role name")
	}
}

func TestGenerateMinutesPrompt_EmptyRoles(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	if !contains(prompt, "Unknown roles") {
		t.Error("Expected prompt to contain 'Unknown roles' when no roles provided")
	}
}

func TestGenerateMinutesPrompt_EmptyTranscription(t *testing.T) {
	transcriptionText := ""
	roles := []string{"user", "moderator"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	if !contains(prompt, "TRANSCRIPT:") {
		t.Error("Expected prompt to contain TRANSCRIPT section")
	}
}

func TestGenerateMinutesPrompt_ThreeRoles(t *testing.T) {
	transcriptionText := "Three participant conversation."
	roles := []string{"user", "moderator", "expert"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	if !contains(prompt, "user, moderator and expert") {
		t.Error("Expected prompt to contain 'user, moderator and expert' for 3 roles")
	}
}

func TestGenerateMinutesPrompt_CustomSessionID(t *testing.T) {
	transcriptionText := "Test content."
	roles := []string{"user"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if !contains(prompt, "session1") {
		t.Error("Expected prompt to contain default session ID 'session1'")
	}
}

func TestGenerateMinutesPrompt_AllJSONFields(t *testing.T) {
	transcriptionText := "Test conversation with various points."
	roles := []string{"user", "moderator"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	tests := []string{
		`"themes"`,
		`"contents_reported"`,
		`"professional_interventions"`,
		`"progress_issues"`,
		`"next_steps"`,
		`"citations"`,
	}

	for _, field := range tests {
		if !contains(prompt, field) {
			t.Errorf("Expected prompt to contain field %s", field)
		}
	}
}

func TestGenerateMinutesPrompt_StructureSections(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{"user"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	tests := []struct {
		name     string
		pattern  string
		contains bool
	}{
		{"PARTICIPANT ROLES section", "PARTICIPANT ROLES:", true},
		{"TRANSCRIPT section", "TRANSCRIPT:", true},
		{"REQUIREMENTS section", "REQUIREMENTS:", true},
		{"IMPORTANT section", "IMPORTANT:", true},
		{"themes count", "3-5 main themes", true},
		{"timestamp requirement", "milliseconds", true},
		{"exact quotes requirement", "exact quotes", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.contains {
				if !contains(prompt, tt.pattern) {
					t.Errorf("Expected prompt to contain %s", tt.pattern)
				}
			}
		})
	}
}

func TestGenerateMinutesPrompt_LengthRequirements(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{"user", "moderator"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	tests := []struct {
		pattern     string
		shouldExist bool
	}{
		{"5 timestamped citations", true},
		{"3-5 main themes", true},
		{"at least 5 timestamped citations", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if tt.shouldExist && !contains(prompt, tt.pattern) {
				t.Errorf("Expected prompt to mention %s", tt.pattern)
			}
		})
	}
}

func TestGenerateMinutesPrompt_ClinicalDisclaimer(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{"user", "moderator"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if !contains(prompt, "Do NOT make diagnoses") {
		t.Error("Expected prompt to contain clinical disclaimer")
	}
	if !contains(prompt, "clinical assessments") {
		t.Error("Expected prompt to mention clinical assessments")
	}
	if !contains(prompt, "Report only what was explicitly stated") {
		t.Error("Expected prompt to mention reporting only stated content")
	}
}

func TestGenerateMinutesPrompt_PromptTemplate(t *testing.T) {
	transcriptionText := "Test content for minutes generation."
	roles := []string{"user", "moderator"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	tests := []string{
		"Analyze the following conversation transcript",
		"PARTICIPANT ROLES:",
		"TRANSCRIPT:",
		"Generate a JSON response",
		"REQUIREMENTS:",
		"IMPORTANT:",
	}

	for _, section := range tests {
		if !contains(prompt, section) {
			t.Errorf("Expected prompt to contain section: %s", section)
		}
	}
}

func TestGenerateMinutesPrompt_VerboseRequirements(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{"user"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	// Check that requirements are clearly stated
	tests := []struct {
		pattern  string
		contains bool
	}{
		{"1. themes", true},
		{"2. contents_reported", true},
		{"3. professional_interventions", true},
		{"4. progress_issues", true},
		{"5. next_steps", true},
		{"6. citations", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if tt.contains && !contains(prompt, tt.pattern) {
				t.Errorf("Expected prompt to mention requirement %s", tt.pattern)
			}
		})
	}
}

func TestGenerateMinutesPrompt_ExactQuotesRequirement(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{"user"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if !contains(prompt, "Quote exact words") {
		t.Error("Expected prompt to require exact quotes")
	}
}

func TestGenerateMinutesPrompt_TimestampFormatRequirement(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{"user"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	if !contains(prompt, "milliseconds") {
		t.Error("Expected prompt to specify timestamp format as milliseconds")
	}
}

func TestGenerateMinutesPrompt_PromptIsNotTooLong(t *testing.T) {
	transcriptionText := "Test conversation about a specific topic."
	roles := []string{"user", "moderator", "expert"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	// Prompt should be reasonable length for API calls
	if len(prompt) > 5000 {
		t.Errorf("Prompt seems too long: %d characters", len(prompt))
	}
}

func TestGenerateMinutesPrompt_NoMarkdownFormatting(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{"user"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	// Ensure prompt doesn't use markdown that might interfere with JSON parsing
	tests := []string{
		"```json", // This is not in the prompt, should not exist
	}

	for _, pattern := range tests {
		if contains(prompt, pattern) {
			t.Errorf("Prompt should not contain %s", pattern)
		}
	}
}

func TestGenerateMinutesPrompt_EmptyArrayRepresentations(t *testing.T) {
	transcriptionText := "Test conversation."
	roles := []string{"user"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	// Check that empty arrays are represented in JSON format
	if !contains(prompt, `[]`) {
		t.Error("Expected prompt to show empty array representation")
	}
}

func TestGenerateMinutesPrompt_NonEmptyArrayRepresentations(t *testing.T) {
	transcriptionText := "Test conversation with multiple points."
	roles := []string{"user", "moderator"}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)

	// Check that array representations show examples
	if !contains(prompt, `["main theme 1"`) {
		t.Error("Expected prompt to show example array elements")
	}
}

// Helper function to check if a string contains a pattern
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
