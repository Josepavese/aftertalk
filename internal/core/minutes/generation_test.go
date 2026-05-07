package minutes

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
)

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
