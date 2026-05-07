package minutesgen

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
)

func TestSplitTranscriptBatches(t *testing.T) {
	transcript := strings.Join([]string{
		"[0ms therapist]: Buongiorno",
		"[1000ms patient]: Buongiorno",
		"[2000ms therapist]: Come sta andando?",
		"[3000ms patient]: Meglio di ieri",
	}, "\n")

	batches := SplitTranscriptBatches(transcript, Config{
		Incremental:      true,
		BatchMaxSegments: 2,
		BatchMaxChars:    120,
	})

	assert.Len(t, batches, 2)
	assert.Contains(t, batches[0], "[0ms therapist]")
	assert.Contains(t, batches[1], "[3000ms patient]")
}

func TestNormalizePhases_DistributesWhenOverLimit(t *testing.T) {
	phases := make([]llm.Phase, 10)
	for i := range phases {
		phases[i] = llm.Phase{
			Title:   string(rune('A' + i)),
			Summary: "phase",
			StartMs: i * 1000,
			EndMs:   i*1000 + 500,
		}
	}

	got := normalizePhases(phases, 4)

	assert.Len(t, got, 4)
	assert.Equal(t, 0, got[0].StartMs)
	assert.Equal(t, 9000, got[len(got)-1].StartMs)
}

func TestNormalizePhases_DedupesSameWindowWithLatestState(t *testing.T) {
	phases := []llm.Phase{
		{Title: "Apertura", Summary: "Stesso contenuto", StartMs: 0, EndMs: 4800},
		{Title: "Apertura e indagine", Summary: "Stesso contenuto", StartMs: 0, EndMs: 4800},
		{Title: "Strategia", Summary: "Nuovo contenuto", StartMs: 6300, EndMs: 12000},
	}

	got := normalizePhases(phases, 8)

	assert.Len(t, got, 2)
	assert.Equal(t, "Apertura e indagine", got[0].Title)
	assert.Equal(t, "Strategia", got[1].Title)
}

func TestNormalizePhases_SameWindowLatestWinsWithoutTextHeuristics(t *testing.T) {
	phases := []llm.Phase{
		{Title: "Apertura", Summary: "Sintesi piu dettagliata della fase iniziale", StartMs: 0, EndMs: 4800},
		{Title: "Apertura", Summary: "Breve", StartMs: 0, EndMs: 4800},
	}

	got := normalizePhases(phases, 8)

	assert.Len(t, got, 1)
	assert.Equal(t, "Breve", got[0].Summary)
}

func TestNormalizeStateCitationsAgainstTranscript_RewritesVerbatimByTimestampAndRole(t *testing.T) {
	state := &llm.DynamicMinutesResponse{
		Summary: llm.Summary{Phases: []llm.Phase{}},
		Sections: map[string]json.RawMessage{
			"themes": json.RawMessage(`[]`),
		},
		Citations: []llm.Citation{
			{TimestampMs: 0, Role: "therapist", Text: "Buongiorno, ripartiamo da come è andata la settimana."},
			{TimestampMs: 0, Role: "therapist", Text: "Buongiorno, ripartiamo da come e andata la settimana."},
			{TimestampMs: 180000, Role: "patient", Text: "Lunedì ho evitato una telefonata importante."},
		},
	}
	transcript := strings.Join([]string{
		"[0ms therapist]: Buongiorno, ripartiamo da come e andata la settimana.",
		"[180000ms patient]: Lunedi ho evitato una telefonata importante.",
	}, "\n")

	got := normalizeStateCitationsAgainstTranscript(state, transcript, testTemplate(), Config{MaxSummaryPhases: 8, MaxCitations: 12})

	assert.Equal(t, []llm.Citation{
		{TimestampMs: 0, Role: "therapist", Text: "Buongiorno, ripartiamo da come e andata la settimana."},
		{TimestampMs: 180000, Role: "patient", Text: "Lunedi ho evitato una telefonata importante."},
	}, got.Citations)
}

func TestMergedPhases_ReplacesWhenCandidateCoversPreviousTimeline(t *testing.T) {
	previous := []llm.Phase{
		{Title: "Apertura", Summary: "A", StartMs: 0, EndMs: 5000},
		{Title: "Centro", Summary: "B", StartMs: 5000, EndMs: 10000},
	}
	candidate := []llm.Phase{
		{Title: "Prima fase consolidata", Summary: "A e B", StartMs: 0, EndMs: 10000},
		{Title: "Chiusura", Summary: "C", StartMs: 10000, EndMs: 15000},
	}

	got := mergedPhases(previous, candidate)

	assert.Equal(t, candidate, got)
}

func TestMergedPhases_AppendsWhenCandidateDoesNotCoverPreviousTimeline(t *testing.T) {
	previous := []llm.Phase{{Title: "Apertura", Summary: "A", StartMs: 0, EndMs: 5000}}
	candidate := []llm.Phase{{Title: "Chiusura", Summary: "C", StartMs: 10000, EndMs: 15000}}

	got := mergedPhases(previous, candidate)

	assert.Len(t, got, 2)
	assert.Equal(t, "Apertura", got[0].Title)
	assert.Equal(t, "Chiusura", got[1].Title)
}

func TestFinalizeDynamicMinutes_PrefersFinalSummaryWithoutTextHeuristics(t *testing.T) {
	tmpl := testTemplate()
	previous := &llm.DynamicMinutesResponse{
		Summary: llm.Summary{
			Overview: "overview incrementale",
			Phases:   []llm.Phase{{Title: "Incrementale", Summary: "A", StartMs: 0, EndMs: 5000}},
		},
		Sections: map[string]json.RawMessage{
			"themes": json.RawMessage(`["old"]`),
		},
		Citations: []llm.Citation{{Text: "old quote", Role: "speaker", TimestampMs: 0}},
	}
	candidate := &llm.DynamicMinutesResponse{
		Summary: llm.Summary{
			Overview: "overview finale",
			Phases:   []llm.Phase{{Title: "Finale", Summary: "B", StartMs: 0, EndMs: 10000}},
		},
		Sections: map[string]json.RawMessage{
			"themes": json.RawMessage(`["new"]`),
		},
		Citations: []llm.Citation{{Text: "new quote", Role: "speaker", TimestampMs: 1000}},
	}

	got := finalizeDynamicMinutes(previous, candidate, tmpl, Config{MaxSummaryPhases: 8, MaxCitations: 12})

	assert.Equal(t, "overview finale", got.Summary.Overview)
	assert.Equal(t, []llm.Phase{{Title: "Finale", Summary: "B", StartMs: 0, EndMs: 10000}}, got.Summary.Phases)
	assert.JSONEq(t, `["new"]`, string(got.Sections["themes"]))
	assert.Equal(t, []llm.Citation{
		{Text: "old quote", Role: "speaker", TimestampMs: 0},
		{Text: "new quote", Role: "speaker", TimestampMs: 1000},
	}, got.Citations)
}

func TestFinalizeDynamicMinutes_FallsBackOnlyForEmptyFinalFields(t *testing.T) {
	tmpl := testTemplate()
	previous := &llm.DynamicMinutesResponse{
		Summary: llm.Summary{
			Overview: "overview incrementale",
			Phases:   []llm.Phase{{Title: "Incrementale", Summary: "A", StartMs: 0, EndMs: 5000}},
		},
		Sections: map[string]json.RawMessage{
			"themes": json.RawMessage(`["old"]`),
		},
		Citations: []llm.Citation{{Text: "old quote", Role: "speaker", TimestampMs: 0}},
	}
	candidate := &llm.DynamicMinutesResponse{
		Sections: map[string]json.RawMessage{
			"themes": json.RawMessage(`[]`),
		},
	}

	got := finalizeDynamicMinutes(previous, candidate, tmpl, Config{MaxSummaryPhases: 8, MaxCitations: 12})

	assert.Equal(t, "overview incrementale", got.Summary.Overview)
	assert.Equal(t, previous.Summary.Phases, got.Summary.Phases)
	assert.JSONEq(t, `["old"]`, string(got.Sections["themes"]))
	assert.Equal(t, previous.Citations, got.Citations)
}

func TestFinalizeDynamicMinutes_MergesWhenFinalDoesNotCoverPreviousTimeline(t *testing.T) {
	tmpl := testTemplate()
	previous := &llm.DynamicMinutesResponse{
		Summary: llm.Summary{
			Overview: "overview iniziale",
			Phases:   []llm.Phase{{Title: "Apertura", Summary: "A", StartMs: 0, EndMs: 5000}},
		},
		Sections: map[string]json.RawMessage{
			"themes": json.RawMessage(`["old"]`),
		},
		Citations: []llm.Citation{{Text: "old quote", Role: "speaker", TimestampMs: 0}},
	}
	candidate := &llm.DynamicMinutesResponse{
		Summary: llm.Summary{
			Overview: "overview finale parziale",
			Phases:   []llm.Phase{{Title: "Chiusura", Summary: "B", StartMs: 10000, EndMs: 12000}},
		},
		Sections: map[string]json.RawMessage{
			"themes": json.RawMessage(`["new"]`),
		},
		Citations: []llm.Citation{{Text: "new quote", Role: "speaker", TimestampMs: 10000}},
	}

	got := finalizeDynamicMinutes(previous, candidate, tmpl, Config{MaxSummaryPhases: 8, MaxCitations: 12})

	assert.Equal(t, "overview iniziale overview finale parziale", got.Summary.Overview)
	assert.Len(t, got.Summary.Phases, 2)
	assert.JSONEq(t, `["old","new"]`, string(got.Sections["themes"]))
	assert.Len(t, got.Citations, 2)
}

func TestQualityWarningsForState_FlagsLateOnlyLongSessionCoverage(t *testing.T) {
	transcript := strings.Join([]string{
		"[0ms patient]: early context",
		"[300000ms therapist]: middle exchange",
		"[600000ms patient]: late discussion",
		"[900000ms therapist]: final close",
	}, "\n")
	state := &llm.DynamicMinutesResponse{
		Summary: llm.Summary{
			Overview: "Final close only",
			Phases: []llm.Phase{
				{Title: "Close", Summary: "Only final close", StartMs: 850000, EndMs: 900000},
			},
		},
		Sections: map[string]json.RawMessage{},
		Citations: []llm.Citation{
			{TimestampMs: 860000, Text: "closing", Role: "therapist"},
			{TimestampMs: 870000, Text: "goodbye", Role: "patient"},
			{TimestampMs: 880000, Text: "see you", Role: "therapist"},
		},
	}

	warnings := qualityWarningsForState(transcript, 4, state)

	assert.Contains(t, warnings, "summary.phases_missing_early_coverage")
	assert.Contains(t, warnings, "summary.phases_missing_middle_coverage")
	assert.Contains(t, warnings, "summary.phases_cover_only_late_window")
	assert.Contains(t, warnings, "citations_not_distributed_across_long_session")
}

func testTemplate() config.TemplateConfig {
	return config.TemplateConfig{
		Sections: []config.SectionConfig{
			{Key: "themes", Type: "string_list"},
		},
	}
}
