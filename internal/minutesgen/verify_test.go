package minutesgen

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
)

type scriptedProvider struct {
	prompts   []string
	responses []string
}

func (p *scriptedProvider) Generate(_ context.Context, prompt string) (string, error) {
	p.prompts = append(p.prompts, prompt)
	if len(p.responses) == 0 {
		return `{"summary":{"overview":"","phases":[]},"sections":{},"citations":[]}`, nil
	}
	response := p.responses[0]
	p.responses = p.responses[1:]
	return response, nil
}

func (p *scriptedProvider) Name() string { return "scripted" }

func TestParseVerificationResponse_RequiresEnvelope(t *testing.T) {
	_, _, err := parseVerificationResponse(`{"summary":{"overview":"direct","phases":[]},"sections":{},"citations":[]}`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "verification response must contain")
}

func TestParseVerificationResponse_RejectsUnsafeEnvelope(t *testing.T) {
	_, _, err := parseVerificationResponse(`{
		"ok": false,
		"issues": ["cannot verify"],
		"minutes": {"summary":{"overview":"unsafe","phases":[]},"sections":{},"citations":[]}
	}`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ok=false")
}

func TestDefaultVerifier_ParsesEnvelope(t *testing.T) {
	provider := &scriptedProvider{responses: []string{`{
		"ok": true,
		"issues": ["fixed mixed-language token"],
		"minutes": {
			"summary": {"overview": "corretto", "phases": []},
			"sections": {"themes": ["tema"]},
			"citations": []
		}
	}`}}

	result, err := NewDefaultVerifier(DefaultReducer{}).Verify(context.Background(), VerificationRequest{
		Provider: provider,
		Template: testTemplate(),
		Retry:    RetryConfig{MaxRetries: 0},
		Prompt:   "verify",
		Config:   Config{MaxSummaryPhases: 8, MaxCitations: 12},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.Calls)
	assert.Equal(t, "corretto", result.State.Summary.Overview)
	assert.Equal(t, []string{"fixed mixed-language token"}, result.Issues)
}

func TestDefaultOrchestrator_VerifiesFinalMinutes(t *testing.T) {
	provider := &scriptedProvider{responses: []string{
		`{
			"summary": {"overview": "Seduta con token sporco 试用ate.", "phases": []},
			"sections": {"themes": ["tema"]},
			"citations": []
		}`,
		`{
			"ok": true,
			"issues": ["corrected malformed word"],
			"minutes": {
				"summary": {"overview": "Seduta con token corretto.", "phases": []},
				"sections": {"themes": ["tema"]},
				"citations": []
			}
		}`,
	}}

	result, err := NewDefaultOrchestrator().Generate(context.Background(), Input{
		Provider:         provider,
		Template:         testTemplate(),
		Config:           Config{Incremental: true, MaxSummaryPhases: 8, MaxCitations: 12},
		Retry:            RetryConfig{MaxRetries: 0},
		Transcript:       "[0ms speaker]: Ciao",
		DetectedLanguage: "it",
	})

	require.NoError(t, err)
	assert.Equal(t, "Seduta con token corretto.", result.State.Summary.Overview)
	assert.Equal(t, []string{"corrected malformed word"}, result.VerificationIssues)
	assert.Equal(t, 2, result.Metrics.LLMCalls)
	require.Len(t, provider.prompts, 2)
	assert.Contains(t, provider.prompts[1], "final quality verifier/editor")
}

func TestDefaultOrchestrator_KeepsGeneratedMinutesWhenVerificationFails(t *testing.T) {
	provider := &scriptedProvider{responses: []string{
		`{
			"summary": {"overview": "generated", "phases": []},
			"sections": {"themes": ["tema"]},
			"citations": []
		}`,
		`{"summary":{"overview":"not an envelope","phases":[]},"sections":{},"citations":[]}`,
	}}

	result, err := NewDefaultOrchestrator().Generate(context.Background(), Input{
		Provider:         provider,
		Template:         testTemplate(),
		Config:           Config{Incremental: true, MaxSummaryPhases: 8, MaxCitations: 12},
		Retry:            RetryConfig{MaxRetries: 0},
		Transcript:       "[0ms speaker]: Ciao",
		DetectedLanguage: "it",
	})

	require.NoError(t, err)
	assert.Equal(t, "generated", result.State.Summary.Overview)
	assert.Contains(t, result.QualityWarnings, "verification_failed")
}

func TestDefaultOrchestrator_VerificationPreservesVerbatimAndStructuralFields(t *testing.T) {
	tmpl := testTemplate()
	tmpl.Sections = append(tmpl.Sections,
		config.SectionConfig{Key: "contents_reported", Type: "content_items"},
	)
	provider := &scriptedProvider{responses: []string{
		`{
			"summary": {
				"overview": "Seduta con typo.",
				"phases": [{"title": "Apert", "summary": "Testo sporco", "start_ms": 10, "end_ms": 20}]
			},
			"sections": {
				"themes": ["tema"],
				"contents_reported": [{"text": "Punto sporco", "timestamp": 1234}]
			},
			"citations": [{"timestamp_ms": 1234, "text": "testo citazione originale", "role": "speaker"}]
		}`,
		`{
			"ok": true,
			"issues": ["corrected text fields"],
			"minutes": {
				"summary": {
					"overview": "Seduta con testo corretto.",
					"phases": [{"title": "Apertura", "summary": "Testo corretto", "start_ms": 99, "end_ms": 999}]
				},
				"sections": {
					"themes": ["tema corretto"],
					"contents_reported": [{"text": "Punto corretto", "timestamp": 9999}]
				},
				"citations": [{"timestamp_ms": 9999, "text": "changed citation text", "role": "other"}]
			}
		}`,
	}}

	result, err := NewDefaultOrchestrator().Generate(context.Background(), Input{
		Provider:         provider,
		Template:         tmpl,
		Config:           Config{Incremental: true, MaxSummaryPhases: 8, MaxCitations: 12},
		Retry:            RetryConfig{MaxRetries: 0},
		Transcript:       "[0ms speaker]: Ciao",
		DetectedLanguage: "it",
	})

	require.NoError(t, err)
	assert.Equal(t, "Seduta con testo corretto.", result.State.Summary.Overview)
	require.Len(t, result.State.Summary.Phases, 1)
	assert.Equal(t, "Apertura", result.State.Summary.Phases[0].Title)
	assert.Equal(t, "Testo corretto", result.State.Summary.Phases[0].Summary)
	assert.Equal(t, 10, result.State.Summary.Phases[0].StartMs)
	assert.Equal(t, 20, result.State.Summary.Phases[0].EndMs)
	assert.JSONEq(t, `["tema corretto"]`, string(result.State.Sections["themes"]))
	assert.JSONEq(t, `[{"text":"Punto corretto","timestamp":1234}]`, string(result.State.Sections["contents_reported"]))
	assert.Equal(t, []llm.Citation{{TimestampMs: 1234, Text: "testo citazione originale", Role: "speaker"}}, result.State.Citations)
	assert.Equal(t, []string{"verification_structural_guard_applied"}, result.VerificationIssues)
}

var _ Provider = (*scriptedProvider)(nil)
