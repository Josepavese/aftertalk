package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Josepavese/aftertalk/internal/config"
)

// GenerateMinutesPrompt builds a one-shot prompt for providers/tests that still
// expect the legacy API. Internally it uses the incremental update prompt with
// an empty existing state.
func GenerateMinutesPrompt(transcriptionText string, tmpl config.TemplateConfig, detectedLanguage string) string {
	return GenerateIncrementalMinutesPrompt(nil, transcriptionText, tmpl, detectedLanguage, true)
}

// GenerateIncrementalMinutesPrompt asks the model to update the structured
// minutes state using only the new transcript chunk plus the compact state.
func GenerateIncrementalMinutesPrompt(existing *DynamicMinutesResponse, transcriptChunk string, tmpl config.TemplateConfig, detectedLanguage string, finalPass bool) string {
	stateJSON := mustMarshalState(seedState(existing))
	schemaJSON := buildSchemaExample(tmpl)
	sectionRules := buildSectionRules(tmpl)
	langRule := buildLanguageRule(detectedLanguage)
	finalRule := "INTERMEDIATE PASS: optimize for compactness and factual accumulation."
	if finalPass {
		finalRule = "FINAL PASS: resolve duplicates, tighten wording, and ensure the result is globally coherent."
	}

	return fmt.Sprintf(`You are a professional session secretary updating structured minutes incrementally.

%s

You receive:
1. the EXISTING MINUTES STATE summarizing the conversation so far
2. a NEW TRANSCRIPT CHUNK containing only the next slice of the conversation

Your task is to return the FULL UPDATED MINUTES STATE as JSON.

SESSION TEMPLATE: %s
PARTICIPANT ROLES: %s

EXISTING MINUTES STATE:
%s

NEW TRANSCRIPT CHUNK:
%s

Return a JSON object with this exact structure:
%s

RULES:
- Base everything strictly on the transcript chunks plus the existing state; do not fabricate.
- Preserve valid information already present in the existing state unless the new chunk clearly refines it.
- Correct obvious STT errors using context but do not invent new meaning.
- Keep summary.overview to one concise paragraph.
- Keep summary.phases chronological, merged when adjacent phases describe the same stage.
- Each phase must have: title, summary, start_ms, end_ms.
- summary.phases must describe the conversation progression, not generic topics.
- Use citations only for the most relevant verbatim quotes; prefer quality over quantity.
- timestamp and timestamp_ms must be exact integers copied from the [Xms role] transcript prefix.
- Deduplicate repeated facts, citations, and phases.
- If the new chunk adds nothing useful, return the existing state normalized.
%s%s- Do NOT include diagnoses or clinical assessments.
- Output MUST be a single valid JSON object (no code fences, no trailing commas).
- %s
- Respond ONLY with valid JSON — no extra text, no markdown fences.`,
		langRule,
		tmpl.Name,
		formatRoles(tmpl.Roles),
		stateJSON,
		transcriptChunk,
		schemaJSON,
		buildLanguageRuleInline(detectedLanguage),
		sectionRules,
		finalRule,
	)
}

// GenerateMinutesFinalizePrompt asks the model to polish a compact state
// without seeing the transcript again. This is useful after multi-batch reduce.
func GenerateMinutesFinalizePrompt(existing *DynamicMinutesResponse, tmpl config.TemplateConfig, detectedLanguage string) string {
	stateJSON := mustMarshalState(seedState(existing))
	schemaJSON := buildSchemaExample(tmpl)
	sectionRules := buildSectionRules(tmpl)
	langRule := buildLanguageRule(detectedLanguage)

	return fmt.Sprintf(`You are finalizing structured minutes for a completed session.

%s

You receive the current structured minutes state. Return the same information in a cleaned, fully normalized JSON object.

SESSION TEMPLATE: %s
PARTICIPANT ROLES: %s

CURRENT MINUTES STATE:
%s

Return a JSON object with this exact structure:
%s

RULES:
- Do not add new facts that are not already supported by the current state.
- Deduplicate overlapping items, citations, and phases.
- Ensure summary.overview is concise and complete.
- Ensure summary.phases are chronological and non-overlapping.
%s%s- Do NOT include diagnoses or clinical assessments.
- FINAL PASS: produce the best compact final version of the minutes.
- Output MUST be a single valid JSON object (no code fences, no trailing commas).
- Respond ONLY with valid JSON — no extra text, no markdown fences.`,
		langRule,
		tmpl.Name,
		formatRoles(tmpl.Roles),
		stateJSON,
		schemaJSON,
		buildLanguageRuleInline(detectedLanguage),
		sectionRules,
	)
}

// GenerateMinutesRepairPrompt asks the model to fix invalid JSON while preserving content.
func GenerateMinutesRepairPrompt(rawResponse string, tmpl config.TemplateConfig, detectedLanguage string) string {
	schemaJSON := buildSchemaExample(tmpl)
	sectionRules := buildSectionRules(tmpl)
	langRule := buildLanguageRule(detectedLanguage)

	return fmt.Sprintf(`You are a strict JSON repair assistant.

%s

The previous response is invalid JSON or malformed. Your task is to output a corrected JSON object that matches the required schema, preserving the original content as much as possible.

SESSION TEMPLATE: %s
PARTICIPANT ROLES: %s

INVALID RESPONSE (verbatim):
%s

Return a JSON object with this exact structure:
%s

RULES:
- Preserve meaning and content; do not invent new facts.
- If a field is missing, use empty values of the correct type.
- Keep summary.overview concise, and summary.phases chronological.
%s- Do NOT include diagnoses or clinical assessments.
- Output MUST be a single valid JSON object (no code fences, no trailing commas).
- Respond ONLY with JSON.`,
		langRule,
		tmpl.Name,
		formatRoles(tmpl.Roles),
		rawResponse,
		schemaJSON,
		sectionRules,
	)
}

func seedState(existing *DynamicMinutesResponse) *DynamicMinutesResponse {
	if existing == nil {
		return &DynamicMinutesResponse{
			Summary:   Summary{Phases: []Phase{}},
			Sections:  map[string]json.RawMessage{},
			Citations: []Citation{},
		}
	}
	if existing.Sections == nil {
		existing.Sections = map[string]json.RawMessage{}
	}
	if existing.Summary.Phases == nil {
		existing.Summary.Phases = []Phase{}
	}
	if existing.Citations == nil {
		existing.Citations = []Citation{}
	}
	return existing
}

func mustMarshalState(state *DynamicMinutesResponse) string {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return `{"summary":{"overview":"","phases":[]},"sections":{},"citations":[]}`
	}
	return string(b)
}

func buildLanguageRule(detectedLanguage string) string {
	if detectedLanguage != "" {
		return fmt.Sprintf("OUTPUT LANGUAGE — MANDATORY: The transcript language is %q. Write ALL text fields in that SAME language. NEVER switch to another language.", detectedLanguage)
	}
	return "LANGUAGE RULE — HIGHEST PRIORITY: Detect the language of the transcript and write ALL output text in that SAME language."
}

func buildLanguageRuleInline(detectedLanguage string) string {
	if detectedLanguage != "" {
		return fmt.Sprintf("- ALL text fields MUST be written in %q.\n", detectedLanguage)
	}
	return "- ALL text fields MUST be written in the same language as the transcript.\n"
}

// buildSchemaExample generates the JSON schema shown to the LLM.
func buildSchemaExample(tmpl config.TemplateConfig) string {
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString("  \"summary\": {\n")
	sb.WriteString("    \"overview\": \"concise summary of the conversation so far\",\n")
	sb.WriteString("    \"phases\": [\n")
	sb.WriteString("      {\n")
	sb.WriteString("        \"title\": \"Opening\",\n")
	sb.WriteString("        \"summary\": \"what happened in this phase\",\n")
	sb.WriteString("        \"start_ms\": 0,\n")
	sb.WriteString("        \"end_ms\": 60000\n")
	sb.WriteString("      }\n")
	sb.WriteString("    ]\n")
	sb.WriteString("  },\n")
	sb.WriteString("  \"sections\": {\n")

	for i, sec := range tmpl.Sections {
		comma := ","
		if i == len(tmpl.Sections)-1 {
			comma = ""
		}

		switch sec.Type {
		case "string_list":
			fmt.Fprintf(&sb, "    %q: [\"item 1\", \"item 2\"]%s\n", sec.Key, comma)
		case "content_items":
			fmt.Fprintf(&sb, "    %q: [{\"text\": \"synthesized point\", \"timestamp\": 0}]%s\n", sec.Key, comma)
		case "progress":
			fmt.Fprintf(&sb, "    %q: {\"progress\": [\"progress item\"], \"issues\": [\"issue\"]}%s\n", sec.Key, comma)
		default:
			fmt.Fprintf(&sb, "    %q: []%s\n", sec.Key, comma)
		}
	}

	sb.WriteString("  },\n")
	sb.WriteString("  \"citations\": [\n")
	sb.WriteString("    {\"timestamp_ms\": 0, \"text\": \"verbatim quote from transcript\", \"role\": \"role_key\"}\n")
	sb.WriteString("  ]\n")
	sb.WriteString("}")
	return sb.String()
}

func buildSectionRules(tmpl config.TemplateConfig) string {
	var sb strings.Builder
	for _, sec := range tmpl.Sections {
		fmt.Fprintf(&sb, "- sections.%s: %s\n", sec.Key, sec.Description)
	}
	fmt.Fprintf(&sb, "- citations.role must be one of: %s\n", formatRoleKeys(tmpl.Roles))
	return sb.String()
}

func formatRoles(roles []config.RoleConfig) string {
	if len(roles) == 0 {
		return "unknown"
	}
	parts := make([]string, len(roles))
	for i, r := range roles {
		parts[i] = fmt.Sprintf("%s (%s)", r.Label, r.Key)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func formatRoleKeys(roles []config.RoleConfig) string {
	keys := make([]string, len(roles))
	for i, r := range roles {
		keys[i] = r.Key
	}
	return strings.Join(keys, ", ")
}
