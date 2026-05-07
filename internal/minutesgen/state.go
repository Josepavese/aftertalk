package minutesgen

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
)

type transcriptEntry struct {
	Raw string
}

func SplitTranscriptBatches(transcriptionText string, cfg Config) []string {
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
		maxSegments = DefaultConfig().BatchMaxSegments
	}
	maxChars := cfg.BatchMaxChars
	if maxChars <= 0 {
		maxChars = DefaultConfig().BatchMaxChars
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

type DefaultReducer struct{}

func (DefaultReducer) Normalize(state *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse {
	return normalizeDynamicMinutes(state, tmpl, cfg)
}

func (DefaultReducer) Merge(previous, candidate *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse {
	return mergeDynamicMinutes(previous, candidate, tmpl, cfg)
}

func (DefaultReducer) Finalize(previous, candidate *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse {
	return finalizeDynamicMinutes(previous, candidate, tmpl, cfg)
}

type DefaultQualityGuard struct{}

func (DefaultQualityGuard) Evaluate(transcript string, batchCount int, state *llm.DynamicMinutesResponse) []string {
	return qualityWarningsForState(transcript, batchCount, state)
}

func normalizeDynamicMinutes(state *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse {
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

func mergeDynamicMinutes(previous, candidate *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse {
	prev := normalizeDynamicMinutes(cloneDynamicMinutes(previous), tmpl, cfg)
	next := normalizeDynamicMinutes(cloneDynamicMinutes(candidate), tmpl, cfg)
	if !hasDynamicStateContent(prev) {
		return next
	}
	if !hasDynamicStateContent(next) {
		return prev
	}

	overview := mergedOverview(prev, next)
	merged := &llm.DynamicMinutesResponse{
		Summary: llm.Summary{
			Overview: overview,
			Phases:   mergedPhases(prev.Summary.Phases, next.Summary.Phases),
		},
		Sections:  mergeSections(prev.Sections, next.Sections, tmpl),
		Citations: append(append([]llm.Citation{}, prev.Citations...), next.Citations...),
	}
	return normalizeDynamicMinutes(merged, tmpl, cfg)
}

func finalizeDynamicMinutes(previous, candidate *llm.DynamicMinutesResponse, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse {
	prev := normalizeDynamicMinutes(cloneDynamicMinutes(previous), tmpl, cfg)
	next := normalizeDynamicMinutes(cloneDynamicMinutes(candidate), tmpl, cfg)
	if !hasDynamicStateContent(prev) {
		return next
	}
	if !hasDynamicStateContent(next) {
		return prev
	}
	if !candidateCoversPreviousPhases(prev.Summary.Phases, next.Summary.Phases) {
		return mergeDynamicMinutes(prev, next, tmpl, cfg)
	}

	final := &llm.DynamicMinutesResponse{
		Summary:   finalizedSummary(prev.Summary, next.Summary),
		Sections:  finalizedSections(prev.Sections, next.Sections, tmpl),
		Citations: finalizedCitations(prev.Citations, next.Citations),
	}
	return normalizeDynamicMinutes(final, tmpl, cfg)
}

func finalizedSummary(previous, candidate llm.Summary) llm.Summary {
	out := previous
	if strings.TrimSpace(candidate.Overview) != "" {
		out.Overview = candidate.Overview
	}
	if len(candidate.Phases) > 0 {
		out.Phases = append([]llm.Phase{}, candidate.Phases...)
	}
	return out
}

func finalizedSections(previous, candidate map[string]json.RawMessage, tmpl config.TemplateConfig) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(previous)+len(candidate))
	for key, raw := range previous {
		out[key] = cloneRawMessage(raw)
	}
	for key, raw := range candidate {
		if isEmptyJSONValue(raw) {
			continue
		}
		out[key] = cloneRawMessage(raw)
	}
	for _, section := range tmpl.Sections {
		if _, ok := out[section.Key]; !ok {
			out[section.Key] = emptySectionValue(section.Type)
		}
	}
	return out
}

func finalizedCitations(previous, candidate []llm.Citation) []llm.Citation {
	if len(candidate) > 0 {
		return append(append([]llm.Citation{}, previous...), candidate...)
	}
	return append([]llm.Citation{}, previous...)
}

func mergedPhases(previous, candidate []llm.Phase) []llm.Phase {
	if candidateCoversPreviousPhases(previous, candidate) {
		return append([]llm.Phase{}, candidate...)
	}
	return append(append([]llm.Phase{}, previous...), candidate...)
}

func mergedOverview(previous, candidate *llm.DynamicMinutesResponse) string {
	prev := strings.TrimSpace(previous.Summary.Overview)
	next := strings.TrimSpace(candidate.Summary.Overview)
	if prev == "" {
		return next
	}
	if next == "" {
		return prev
	}
	if len(candidate.Summary.Phases) > 0 {
		return prev + " " + next
	}
	return next
}

func candidateCoversPreviousPhases(previous, candidate []llm.Phase) bool {
	if len(previous) == 0 {
		return true
	}
	if len(candidate) == 0 {
		return false
	}
	for _, prev := range previous {
		covered := false
		for _, next := range candidate {
			if phasesOverlap(prev, next) {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

func phasesOverlap(a, b llm.Phase) bool {
	return a.EndMs >= b.StartMs && b.EndMs >= a.StartMs
}

func cloneDynamicMinutes(state *llm.DynamicMinutesResponse) *llm.DynamicMinutesResponse {
	if state == nil {
		return &llm.DynamicMinutesResponse{}
	}
	b, err := json.Marshal(state)
	if err != nil {
		return &llm.DynamicMinutesResponse{}
	}
	var cloned llm.DynamicMinutesResponse
	if err := json.Unmarshal(b, &cloned); err != nil {
		return &llm.DynamicMinutesResponse{}
	}
	return &cloned
}

func hasDynamicStateContent(state *llm.DynamicMinutesResponse) bool {
	if state == nil {
		return false
	}
	if strings.TrimSpace(state.Summary.Overview) != "" || len(state.Summary.Phases) > 0 || len(state.Citations) > 0 {
		return true
	}
	for _, raw := range state.Sections {
		if !isEmptyJSONValue(raw) {
			return true
		}
	}
	return false
}

func isEmptyJSONValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" ||
		trimmed == "null" ||
		trimmed == "[]" ||
		trimmed == "{}" ||
		trimmed == `{"progress":[],"issues":[]}`
}

func mergeSections(previous, candidate map[string]json.RawMessage, tmpl config.TemplateConfig) map[string]json.RawMessage {
	merged := make(map[string]json.RawMessage, len(previous)+len(candidate))
	for key, raw := range previous {
		merged[key] = cloneRawMessage(raw)
	}
	for key, raw := range candidate {
		merged[key] = cloneRawMessage(raw)
	}
	for _, section := range tmpl.Sections {
		prevRaw, prevOK := previous[section.Key]
		nextRaw, nextOK := candidate[section.Key]
		if !prevOK || !nextOK {
			continue
		}
		merged[section.Key] = mergeSectionValue(section.Type, prevRaw, nextRaw)
	}
	return merged
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}

func mergeSectionValue(sectionType string, previous, candidate json.RawMessage) json.RawMessage {
	if sectionType == "progress" {
		return mergeProgressSection(previous, candidate)
	}
	return mergeJSONArray(previous, candidate)
}

func mergeProgressSection(previous, candidate json.RawMessage) json.RawMessage {
	var prevObj map[string]json.RawMessage
	var nextObj map[string]json.RawMessage
	if json.Unmarshal(previous, &prevObj) != nil || json.Unmarshal(candidate, &nextObj) != nil {
		return preferNonEmptyRaw(candidate, previous)
	}
	merged := make(map[string]json.RawMessage, len(prevObj)+len(nextObj))
	for key, raw := range prevObj {
		merged[key] = cloneRawMessage(raw)
	}
	for key, raw := range nextObj {
		merged[key] = cloneRawMessage(raw)
	}
	for _, key := range []string{"progress", "issues"} {
		prevRaw, prevOK := prevObj[key]
		nextRaw, nextOK := nextObj[key]
		if prevOK && nextOK {
			merged[key] = mergeJSONArray(prevRaw, nextRaw)
		}
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return preferNonEmptyRaw(candidate, previous)
	}
	return b
}

func mergeJSONArray(previous, candidate json.RawMessage) json.RawMessage {
	var prevItems []json.RawMessage
	var nextItems []json.RawMessage
	if json.Unmarshal(previous, &prevItems) != nil || json.Unmarshal(candidate, &nextItems) != nil {
		return preferNonEmptyRaw(candidate, previous)
	}
	items := make([]json.RawMessage, 0, len(prevItems)+len(nextItems))
	seen := make(map[string]struct{}, len(prevItems)+len(nextItems))
	for _, item := range append(prevItems, nextItems...) {
		key := canonicalJSON(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, cloneRawMessage(item))
	}
	b, err := json.Marshal(items)
	if err != nil {
		return preferNonEmptyRaw(candidate, previous)
	}
	return b
}

func canonicalJSON(raw json.RawMessage) string {
	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return strings.TrimSpace(string(raw))
	}
	b, err := json.Marshal(decoded)
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	return string(b)
}

func preferNonEmptyRaw(primary, fallback json.RawMessage) json.RawMessage {
	if !isEmptyJSONValue(primary) {
		return cloneRawMessage(primary)
	}
	return cloneRawMessage(fallback)
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
		limit = DefaultConfig().MaxSummaryPhases
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
	indexByWindow := make(map[string]int, len(filtered))
	seen := make(map[string]struct{}, len(filtered))
	for _, phase := range filtered {
		key := fmt.Sprintf("%s|%s|%d|%d", phase.Title, phase.Summary, phase.StartMs, phase.EndMs)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		windowKey := fmt.Sprintf("%d|%d", phase.StartMs, phase.EndMs)
		if idx, ok := indexByWindow[windowKey]; ok {
			deduped[idx] = phase
			continue
		}
		indexByWindow[windowKey] = len(deduped)
		deduped = append(deduped, phase)
	}
	if len(deduped) > limit {
		deduped = selectDistributedPhases(deduped, limit)
	}
	return deduped
}

func normalizeCitations(citations []llm.Citation, limit int) []llm.Citation {
	if limit <= 0 {
		limit = DefaultConfig().MaxCitations
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
		key := fmt.Sprintf("%s|%s|%d", citation.Role, citation.Text, citation.TimestampMs)
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
		filtered = selectDistributedCitations(filtered, limit)
	}
	return filtered
}

func normalizeStateCitationsAgainstTranscript(state *llm.DynamicMinutesResponse, transcriptionText string, tmpl config.TemplateConfig, cfg Config) *llm.DynamicMinutesResponse {
	normalized := normalizeDynamicMinutes(cloneDynamicMinutes(state), tmpl, cfg)
	citationTextByKey := transcriptCitationTextByKey(transcriptionText)
	if len(citationTextByKey) == 0 {
		return normalized
	}
	for i := range normalized.Citations {
		key := transcriptCitationKey(normalized.Citations[i].TimestampMs, normalized.Citations[i].Role)
		if text, ok := citationTextByKey[key]; ok {
			normalized.Citations[i].Text = text
		}
	}
	normalized.Citations = normalizeCitations(normalized.Citations, cfg.MaxCitations)
	return normalized
}

func transcriptCitationTextByKey(transcriptionText string) map[string]string {
	lines := strings.Split(strings.ReplaceAll(transcriptionText, "\r\n", "\n"), "\n")
	out := make(map[string]string, len(lines))
	for _, line := range lines {
		timestampMs, role, text, ok := parseTranscriptCitationLine(line)
		if !ok {
			continue
		}
		out[transcriptCitationKey(timestampMs, role)] = text
	}
	return out
}

func transcriptCitationKey(timestampMs int, role string) string {
	return fmt.Sprintf("%d|%s", timestampMs, strings.TrimSpace(role))
}

func parseTranscriptCitationLine(line string) (int, string, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") {
		return 0, "", "", false
	}
	closeIdx := strings.Index(line, "]:")
	if closeIdx < 0 {
		return 0, "", "", false
	}
	header := line[1:closeIdx]
	text := strings.TrimSpace(line[closeIdx+2:])
	if text == "" {
		return 0, "", "", false
	}
	msIdx := strings.Index(header, "ms")
	if msIdx <= 0 {
		return 0, "", "", false
	}
	timestampMs, err := strconv.Atoi(strings.TrimSpace(header[:msIdx]))
	if err != nil {
		return 0, "", "", false
	}
	role := strings.TrimSpace(header[msIdx+len("ms"):])
	if role == "" {
		return 0, "", "", false
	}
	return timestampMs, role, text, true
}

func selectDistributedPhases(phases []llm.Phase, limit int) []llm.Phase {
	if limit <= 0 || len(phases) <= limit {
		return phases
	}
	selected := make([]llm.Phase, 0, limit)
	seen := make(map[int]struct{}, limit)
	for i := 0; i < limit; i++ {
		idx := i * (len(phases) - 1) / (limit - 1)
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		selected = append(selected, phases[idx])
	}
	for i := range phases {
		if len(selected) >= limit {
			break
		}
		if _, ok := seen[i]; ok {
			continue
		}
		selected = append(selected, phases[i])
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].StartMs == selected[j].StartMs {
			return selected[i].EndMs < selected[j].EndMs
		}
		return selected[i].StartMs < selected[j].StartMs
	})
	return selected
}

func selectDistributedCitations(citations []llm.Citation, limit int) []llm.Citation {
	if limit <= 0 || len(citations) <= limit {
		return citations
	}
	selected := make([]llm.Citation, 0, limit)
	seen := make(map[int]struct{}, limit)
	for i := 0; i < limit; i++ {
		idx := i * (len(citations) - 1) / (limit - 1)
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		selected = append(selected, citations[idx])
	}
	for i := range citations {
		if len(selected) >= limit {
			break
		}
		if _, ok := seen[i]; ok {
			continue
		}
		selected = append(selected, citations[i])
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].TimestampMs == selected[j].TimestampMs {
			return selected[i].Role < selected[j].Role
		}
		return selected[i].TimestampMs < selected[j].TimestampMs
	})
	return selected
}

func qualityWarningsForState(transcriptionText string, batchCount int, state *llm.DynamicMinutesResponse) []string {
	if state == nil {
		return nil
	}
	timeline, ok := transcriptTimeline(transcriptionText)
	if !ok {
		return nil
	}
	duration := timeline.endMs - timeline.startMs
	if batchCount < 3 && duration < 10*60*1000 {
		return nil
	}

	var warnings []string
	if len(state.Summary.Phases) == 0 {
		warnings = append(warnings, "summary.phases_missing_for_long_session")
	} else {
		if !phasesCoverWindow(state.Summary.Phases, timeline.startMs, timeline.firstCutoff()) {
			warnings = append(warnings, "summary.phases_missing_early_coverage")
		}
		if !phasesCoverWindow(state.Summary.Phases, timeline.firstCutoff(), timeline.secondCutoff()) {
			warnings = append(warnings, "summary.phases_missing_middle_coverage")
		}
		if !phasesCoverWindow(state.Summary.Phases, timeline.secondCutoff(), timeline.endMs) {
			warnings = append(warnings, "summary.phases_missing_late_coverage")
		}
		if phasesCoverOnlyLate(state.Summary.Phases, timeline) {
			warnings = append(warnings, "summary.phases_cover_only_late_window")
		}
	}

	if len(state.Citations) >= 3 && citationWindowCount(state.Citations, timeline) < 2 {
		warnings = append(warnings, "citations_not_distributed_across_long_session")
	}
	return warnings
}

type transcriptTimelineRange struct {
	startMs int
	endMs   int
}

func (t transcriptTimelineRange) firstCutoff() int {
	return t.startMs + (t.endMs-t.startMs)/3
}

func (t transcriptTimelineRange) secondCutoff() int {
	return t.startMs + 2*(t.endMs-t.startMs)/3
}

func transcriptTimeline(transcriptionText string) (transcriptTimelineRange, bool) {
	lines := strings.Split(strings.ReplaceAll(transcriptionText, "\r\n", "\n"), "\n")
	minTS := 0
	maxTS := 0
	found := false
	for _, line := range lines {
		ts, ok := parseTranscriptTimestampMs(line)
		if !ok {
			continue
		}
		if !found || ts < minTS {
			minTS = ts
		}
		if !found || ts > maxTS {
			maxTS = ts
		}
		found = true
	}
	if !found || maxTS <= minTS {
		return transcriptTimelineRange{}, false
	}
	return transcriptTimelineRange{startMs: minTS, endMs: maxTS}, true
}

func parseTranscriptTimestampMs(line string) (int, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") {
		return 0, false
	}
	end := strings.Index(line, "ms")
	if end <= 1 {
		return 0, false
	}
	ts, err := strconv.Atoi(line[1:end])
	if err != nil {
		return 0, false
	}
	return ts, true
}

func phasesCoverWindow(phases []llm.Phase, startMs, endMs int) bool {
	for _, phase := range phases {
		if phase.EndMs >= startMs && phase.StartMs <= endMs {
			return true
		}
	}
	return false
}

func phasesCoverOnlyLate(phases []llm.Phase, timeline transcriptTimelineRange) bool {
	if len(phases) == 0 {
		return false
	}
	minStart := phases[0].StartMs
	maxEnd := phases[0].EndMs
	for _, phase := range phases[1:] {
		if phase.StartMs < minStart {
			minStart = phase.StartMs
		}
		if phase.EndMs > maxEnd {
			maxEnd = phase.EndMs
		}
	}
	return minStart >= timeline.secondCutoff() && maxEnd-minStart <= (timeline.endMs-timeline.startMs)/3
}

func citationWindowCount(citations []llm.Citation, timeline transcriptTimelineRange) int {
	windows := map[int]struct{}{}
	for _, citation := range citations {
		switch {
		case citation.TimestampMs <= timeline.firstCutoff():
			windows[0] = struct{}{}
		case citation.TimestampMs <= timeline.secondCutoff():
			windows[1] = struct{}{}
		default:
			windows[2] = struct{}{}
		}
	}
	return len(windows)
}
