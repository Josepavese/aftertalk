package minutes

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
)

type transcriptEntry struct {
	Raw string
}

func splitTranscriptBatches(transcriptionText string, cfg GenerationConfig) []string {
	lines := strings.Split(strings.ReplaceAll(transcriptionText, "\r\n", "\n"), "\n")
	entries := make([]transcriptEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, transcriptEntry{Raw: line})
	}
	if len(entries) == 0 {
		return nil
	}
	if !cfg.Incremental {
		return []string{joinTranscriptEntries(entries)}
	}

	maxSegments := cfg.BatchMaxSegments
	if maxSegments <= 0 {
		maxSegments = DefaultGenerationConfig().BatchMaxSegments
	}
	maxChars := cfg.BatchMaxChars
	if maxChars <= 0 {
		maxChars = DefaultGenerationConfig().BatchMaxChars
	}

	var batches []string
	var current []transcriptEntry
	currentChars := 0
	for _, entry := range entries {
		entryChars := len(entry.Raw) + 1
		if len(current) > 0 && (len(current) >= maxSegments || currentChars+entryChars > maxChars) {
			batches = append(batches, joinTranscriptEntries(current))
			current = nil
			currentChars = 0
		}
		current = append(current, entry)
		currentChars += entryChars
	}
	if len(current) > 0 {
		batches = append(batches, joinTranscriptEntries(current))
	}
	if len(batches) == 0 {
		return []string{transcriptionText}
	}
	return batches
}

func joinTranscriptEntries(entries []transcriptEntry) string {
	lines := make([]string, len(entries))
	for i, entry := range entries {
		lines[i] = entry.Raw
	}
	return strings.Join(lines, "\n")
}

func normalizeDynamicMinutes(state *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg GenerationConfig) *llm.DynamicMinutesResponse {
	if state == nil {
		state = &llm.DynamicMinutesResponse{}
	}
	if state.Sections == nil {
		state.Sections = map[string]json.RawMessage{}
	}
	if state.Summary.Phases == nil {
		state.Summary.Phases = []llm.Phase{}
	}
	if state.Citations == nil {
		state.Citations = []llm.Citation{}
	}

	for _, section := range tmpl.Sections {
		if _, ok := state.Sections[section.Key]; ok {
			continue
		}
		state.Sections[section.Key] = emptySectionValue(section.Type)
	}

	state.Summary.Overview = strings.TrimSpace(state.Summary.Overview)
	state.Summary.Phases = normalizePhases(state.Summary.Phases, cfg.MaxSummaryPhases)
	state.Citations = normalizeCitations(state.Citations, cfg.MaxCitations)

	return state
}

func emptySectionValue(sectionType string) json.RawMessage {
	switch sectionType {
	case "progress":
		return json.RawMessage(`{"progress":[],"issues":[]}`)
	default:
		return json.RawMessage(`[]`)
	}
}

func normalizePhases(phases []llm.Phase, limit int) []llm.Phase {
	if limit <= 0 {
		limit = DefaultGenerationConfig().MaxSummaryPhases
	}
	filtered := make([]llm.Phase, 0, len(phases))
	for _, phase := range phases {
		phase.Title = strings.TrimSpace(phase.Title)
		phase.Summary = strings.TrimSpace(phase.Summary)
		if phase.Title == "" && phase.Summary == "" {
			continue
		}
		if phase.StartMs < 0 {
			phase.StartMs = 0
		}
		if phase.EndMs < phase.StartMs {
			phase.EndMs = phase.StartMs
		}
		filtered = append(filtered, phase)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].StartMs == filtered[j].StartMs {
			return filtered[i].EndMs < filtered[j].EndMs
		}
		return filtered[i].StartMs < filtered[j].StartMs
	})

	deduped := make([]llm.Phase, 0, len(filtered))
	seen := make(map[string]struct{}, len(filtered))
	for _, phase := range filtered {
		key := fmt.Sprintf("%s|%s|%d|%d", strings.ToLower(phase.Title), strings.ToLower(phase.Summary), phase.StartMs, phase.EndMs)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, phase)
	}
	if len(deduped) > limit {
		deduped = deduped[:limit]
	}
	return deduped
}

func normalizeCitations(citations []llm.Citation, limit int) []llm.Citation {
	if limit <= 0 {
		limit = DefaultGenerationConfig().MaxCitations
	}
	filtered := make([]llm.Citation, 0, len(citations))
	seen := make(map[string]struct{}, len(citations))
	for _, citation := range citations {
		citation.Text = strings.TrimSpace(citation.Text)
		citation.Role = strings.TrimSpace(citation.Role)
		if citation.Text == "" {
			continue
		}
		if citation.TimestampMs < 0 {
			citation.TimestampMs = 0
		}
		key := fmt.Sprintf("%s|%s|%d", strings.ToLower(citation.Role), strings.ToLower(citation.Text), citation.TimestampMs)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, citation)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].TimestampMs == filtered[j].TimestampMs {
			return filtered[i].Role < filtered[j].Role
		}
		return filtered[i].TimestampMs < filtered[j].TimestampMs
	})

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}
