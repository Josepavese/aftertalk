package llm_test

import (
	"strings"
	"testing"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
)

var therapyTemplate = config.TemplateConfig{
	ID:   "therapy",
	Name: "Seduta Terapeutica",
	Roles: []config.RoleConfig{
		{Key: "therapist", Label: "Terapeuta"},
		{Key: "patient", Label: "Paziente"},
	},
	Sections: []config.SectionConfig{
		{Key: "themes", Label: "Temi", Description: "Main topics", Type: "string_list"},
		{Key: "contents_reported", Label: "Contenuti", Description: "What the patient said", Type: "content_items"},
		{Key: "professional_interventions", Label: "Interventi", Description: "What the therapist did", Type: "content_items"},
		{Key: "progress_issues", Label: "Progressi", Description: "Progress and issues", Type: "progress"},
		{Key: "next_steps", Label: "Prossimi passi", Description: "Next actions", Type: "string_list"},
	},
}

func TestGenerateMinutesPrompt_NonEmpty(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("Test transcript", therapyTemplate)
	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}
}

func TestGenerateMinutesPrompt_ContainsTranscript(t *testing.T) {
	text := "Test conversation transcript."
	prompt := llm.GenerateMinutesPrompt(text, therapyTemplate)
	if !strings.Contains(prompt, text) {
		t.Error("Expected prompt to contain the transcript text")
	}
}

func TestGenerateMinutesPrompt_ContainsRoles(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("transcript", therapyTemplate)
	if !strings.Contains(prompt, "Terapeuta") {
		t.Error("Expected prompt to contain role label Terapeuta")
	}
	if !strings.Contains(prompt, "Paziente") {
		t.Error("Expected prompt to contain role label Paziente")
	}
}

func TestGenerateMinutesPrompt_ContainsTemplateName(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("transcript", therapyTemplate)
	if !strings.Contains(prompt, "Seduta Terapeutica") {
		t.Error("Expected prompt to contain template name")
	}
}

func TestGenerateMinutesPrompt_ContainsSectionKeys(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("transcript", therapyTemplate)
	for _, sec := range therapyTemplate.Sections {
		if !strings.Contains(prompt, sec.Key) {
			t.Errorf("Expected prompt to contain section key %q", sec.Key)
		}
	}
}

func TestGenerateMinutesPrompt_ContainsCitations(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("transcript", therapyTemplate)
	if !strings.Contains(prompt, "citations") {
		t.Error("Expected prompt to contain citations field")
	}
}

func TestGenerateMinutesPrompt_ValidJSONInstruction(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("transcript", therapyTemplate)
	if !strings.Contains(prompt, "valid JSON") {
		t.Error("Expected prompt to instruct valid JSON output")
	}
}

func TestGenerateMinutesPrompt_LanguageRule(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("transcript", therapyTemplate)
	if !strings.Contains(prompt, "LANGUAGE") {
		t.Error("Expected prompt to contain language rule")
	}
}

func TestGenerateMinutesPrompt_NoClinicalAssessments(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("transcript", therapyTemplate)
	if !strings.Contains(prompt, "clinical assessments") {
		t.Error("Expected prompt to mention clinical assessments rule")
	}
}

func TestGenerateMinutesPrompt_SectionDescriptions(t *testing.T) {
	prompt := llm.GenerateMinutesPrompt("transcript", therapyTemplate)
	for _, sec := range therapyTemplate.Sections {
		if !strings.Contains(prompt, sec.Description) {
			t.Errorf("Expected prompt to contain section description %q", sec.Description)
		}
	}
}

func TestGenerateMinutesPrompt_EmptyTemplate(t *testing.T) {
	tmpl := config.TemplateConfig{ID: "empty", Name: "Empty"}
	prompt := llm.GenerateMinutesPrompt("transcript", tmpl)
	if prompt == "" {
		t.Error("Expected non-empty prompt even for empty template")
	}
}
