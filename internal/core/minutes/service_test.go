package minutes

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/internal/minutesgen"
	"github.com/Josepavese/aftertalk/pkg/webhook"
)

func init() { //nolint:gochecknoinits // test logger setup
	_ = logging.Init("info", "console")
}

type scriptedLLMProvider struct {
	prompts   []string
	responses []string
}

type fakeOrchestrator struct {
	input  minutesgen.Input
	called bool
}

func (o *fakeOrchestrator) Generate(_ context.Context, input minutesgen.Input) (*minutesgen.Result, error) {
	o.called = true
	o.input = input
	return &minutesgen.Result{
		State: &llm.DynamicMinutesResponse{
			Summary:  llm.Summary{Overview: "orchestrated", Phases: []llm.Phase{}},
			Sections: map[string]json.RawMessage{"themes": json.RawMessage(`["orchestration"]`)},
		},
		QualityWarnings: []string{"test_quality_warning"},
		Metrics:         minutesgen.Metrics{BatchCount: 1, LLMCalls: 1},
	}, nil
}

type runtimeTunedLLMProvider struct {
	failures          int
	calls             int
	runtime           llm.RuntimeConfig
	blockUntilContext bool
}

func (p *runtimeTunedLLMProvider) Generate(ctx context.Context, _ string) (string, error) {
	p.calls++
	if p.blockUntilContext {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if p.calls <= p.failures {
		return "", errors.New("temporary provider error")
	}
	return `{"summary":{"overview":"ok","phases":[]},"sections":{},"citations":[]}`, nil
}

func (p *runtimeTunedLLMProvider) Name() string {
	return "runtime-tuned"
}

func (p *runtimeTunedLLMProvider) IsAvailable() bool {
	return true
}

func (p *runtimeTunedLLMProvider) RuntimeConfig() llm.RuntimeConfig {
	return p.runtime
}

func (p *scriptedLLMProvider) Generate(_ context.Context, prompt string) (string, error) {
	p.prompts = append(p.prompts, prompt)
	if len(p.responses) == 0 {
		return `{"summary":{"overview":"","phases":[]},"sections":{},"citations":[]}`, nil
	}
	response := p.responses[0]
	p.responses = p.responses[1:]
	return response, nil
}

func (p *scriptedLLMProvider) Name() string {
	return "scripted"
}

func (p *scriptedLLMProvider) IsAvailable() bool {
	return true
}

func TestGenerateMinutes_IncrementalFlow(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	service := NewService(repo).WithGenerationConfig(GenerationConfig{
		Incremental:      true,
		BatchMaxSegments: 2,
		BatchMaxChars:    120,
		MaxSummaryPhases: 4,
		MaxCitations:     4,
	})

	provider := &scriptedLLMProvider{
		responses: []string{
			`{
				"summary":{"overview":"Prima parte","phases":[{"title":"Apertura","summary":"Saluti iniziali","start_ms":0,"end_ms":1000}]},
				"sections":{"themes":["accoglienza"],"contents_reported":[],"professional_interventions":[],"progress_issues":{"progress":[],"issues":[]},"next_steps":[]},
				"citations":[{"timestamp_ms":0,"text":"Buongiorno","role":"therapist"}]
			}`,
			`{
				"summary":{"overview":"Bozza completa","phases":[{"title":"Apertura","summary":"Saluti iniziali","start_ms":0,"end_ms":1000},{"title":"Aggiornamento","summary":"Discussione sullo stato recente","start_ms":2000,"end_ms":3000}]},
				"sections":{"themes":["accoglienza","stato emotivo"],"contents_reported":[{"text":"Il paziente riferisce di stare meglio","timestamp":3000}],"professional_interventions":[{"text":"Il terapeuta chiede un aggiornamento","timestamp":2000}],"progress_issues":{"progress":["Miglioramento percepito"],"issues":[]},"next_steps":["Monitorare i progressi"]},
				"citations":[{"timestamp_ms":0,"text":"Buongiorno","role":"therapist"},{"timestamp_ms":3000,"text":"Meglio di ieri","role":"patient"}]
			}`,
			`{
				"summary":{"overview":"La conversazione apre con i saluti e prosegue con un aggiornamento sul miglioramento percepito.","phases":[{"title":"Apertura","summary":"Saluti iniziali","start_ms":0,"end_ms":1000},{"title":"Aggiornamento","summary":"Il paziente riferisce un lieve miglioramento","start_ms":2000,"end_ms":3000}]},
				"sections":{"themes":["accoglienza","stato emotivo"],"contents_reported":[{"text":"Il paziente riferisce di stare meglio del giorno precedente","timestamp":3000}],"professional_interventions":[{"text":"Il terapeuta chiede come sta andando","timestamp":2000}],"progress_issues":{"progress":["Miglioramento percepito"],"issues":[]},"next_steps":["Monitorare i progressi"]},
				"citations":[{"timestamp_ms":3000,"text":"Meglio di ieri","role":"patient"}]
			}`,
		},
	}

	transcript := strings.Join([]string{
		"[0ms therapist]: Buongiorno",
		"[1000ms patient]: Buongiorno",
		"[2000ms therapist]: Come sta andando?",
		"[3000ms patient]: Meglio di ieri",
	}, "\n")
	tmpl := config.DefaultTemplates()[0]

	mins, err := service.GenerateMinutes(context.Background(), "session-123", transcript, tmpl, webhook.SessionContext{}, "it", provider)
	require.NoError(t, err)

	require.Len(t, provider.prompts, 3)
	assert.Contains(t, provider.prompts[1], `"overview": "Prima parte"`)
	assert.Contains(t, provider.prompts[2], "CURRENT MINUTES STATE")
	assert.Equal(t, "La conversazione apre con i saluti e prosegue con un aggiornamento sul miglioramento percepito.", mins.Summary.Overview)
	assert.Len(t, mins.Summary.Phases, 2)
	assert.Equal(t, MinutesStatusReady, mins.Status)
	assert.Equal(t, "scripted", mins.Provider)
	assert.Len(t, mins.Citations, 2)
	assert.Contains(t, string(mins.Sections["themes"]), "stato emotivo")
}

func TestGenerateMinutes_IncrementalMergePreservesEarlierContentWhenFinalizationCollapses(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	service := NewService(repo).WithGenerationConfig(GenerationConfig{
		Incremental:      true,
		BatchMaxSegments: 2,
		BatchMaxChars:    400,
		MaxSummaryPhases: 8,
		MaxCitations:     6,
	})

	provider := &scriptedLLMProvider{
		responses: []string{
			`{
				"summary":{"overview":"Apertura con ansia e contesto personale.","phases":[{"title":"Apertura","summary":"Emergono ansia e contesto personale iniziale.","start_ms":0,"end_ms":120000}]},
				"sections":{"themes":["ansia"],"contents_reported":[{"text":"Il paziente introduce ansia e situazione personale.","timestamp":0}],"professional_interventions":[],"progress_issues":{"progress":[],"issues":["ansia"]},"next_steps":[]},
				"citations":[{"timestamp_ms":0,"text":"Mi sento in ansia","role":"patient"}]
			}`,
			`{
				"summary":{"overview":"Parte centrale su lavoro e incertezza.","phases":[{"title":"Approfondimento","summary":"Si approfondiscono lavoro e incertezza professionale.","start_ms":300000,"end_ms":420000}]},
				"sections":{"themes":["lavoro"],"contents_reported":[{"text":"Il paziente descrive incertezza professionale.","timestamp":300000}],"professional_interventions":[{"text":"Il terapeuta riformula la complessità.","timestamp":360000}],"progress_issues":{"progress":[],"issues":["incertezza professionale"]},"next_steps":[]},
				"citations":[{"timestamp_ms":300000,"text":"Non so come muovermi col lavoro","role":"patient"}]
			}`,
			`{
				"summary":{"overview":"Chiusura con sintesi e prossimi passi espliciti.","phases":[{"title":"Chiusura","summary":"La conversazione si chiude con riepilogo e saluti.","start_ms":600000,"end_ms":720000}]},
				"sections":{"themes":["chiusura"],"contents_reported":[],"professional_interventions":[{"text":"Il terapeuta riepiloga il percorso.","timestamp":600000}],"progress_issues":{"progress":["maggiore chiarezza"],"issues":[]},"next_steps":["Riflettere sui punti emersi"]},
				"citations":[{"timestamp_ms":700000,"text":"Ci aggiorniamo al prossimo incontro","role":"therapist"}]
			}`,
			`{
				"summary":{"overview":"Solo saluti finali.","phases":[{"title":"Saluti","summary":"La sessione termina con saluti.","start_ms":700000,"end_ms":720000}]},
				"sections":{"themes":["saluti"],"contents_reported":[],"professional_interventions":[],"progress_issues":{"progress":[],"issues":[]},"next_steps":[]},
				"citations":[{"timestamp_ms":700000,"text":"Ci aggiorniamo al prossimo incontro","role":"therapist"}]
			}`,
		},
	}

	transcript := strings.Join([]string{
		"[0ms patient]: Mi sento in ansia",
		"[120000ms therapist]: Partiamo da questo",
		"[300000ms patient]: Non so come muovermi col lavoro",
		"[420000ms therapist]: Proviamo a ordinare la complessità",
		"[600000ms therapist]: Riepiloghiamo quanto emerso",
		"[720000ms patient]: Va bene, grazie",
	}, "\n")

	mins, err := service.GenerateMinutes(context.Background(), "session-merge-coverage", transcript, config.DefaultTemplates()[0], webhook.SessionContext{}, "it", provider)
	require.NoError(t, err)

	require.Len(t, provider.prompts, 4)
	assert.Contains(t, mins.Summary.Overview, "ansia")
	assert.Contains(t, mins.Summary.Overview, "lavoro")
	assert.Contains(t, mins.Summary.Overview, "Chiusura")
	require.GreaterOrEqual(t, len(mins.Summary.Phases), 3)
	phaseTitles := make([]string, 0, len(mins.Summary.Phases))
	for _, phase := range mins.Summary.Phases {
		phaseTitles = append(phaseTitles, phase.Title)
	}
	assert.Contains(t, phaseTitles, "Apertura")
	assert.Contains(t, phaseTitles, "Approfondimento")
	assert.Contains(t, phaseTitles, "Chiusura")
	assert.Contains(t, string(mins.Sections["themes"]), "ansia")
	assert.Contains(t, string(mins.Sections["themes"]), "lavoro")
	assert.Contains(t, string(mins.Sections["themes"]), "chiusura")
	assert.Empty(t, mins.QualityWarnings)
}

func TestGenerateMinutes_EmptyTranscript(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	service := NewService(repo)

	mins, err := service.GenerateMinutes(context.Background(), "session-empty", "   ", config.DefaultTemplates()[0], webhook.SessionContext{}, "it", &scriptedLLMProvider{})
	require.NoError(t, err)
	assert.Equal(t, MinutesStatusReady, mins.Status)
	assert.Empty(t, mins.Summary.Overview)
	assert.Empty(t, mins.Citations)
}

func TestGenerateMinutes_DelegatesToGenerationOrchestrator(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	orchestrator := &fakeOrchestrator{}
	service := NewService(repo).WithGenerationOrchestrator(orchestrator)
	provider := &scriptedLLMProvider{}

	mins, err := service.GenerateMinutes(context.Background(), "session-orchestrator", "[0ms therapist]: Ciao", config.DefaultTemplates()[0], webhook.SessionContext{}, "it", provider)
	require.NoError(t, err)

	assert.True(t, orchestrator.called)
	assert.Equal(t, "it", orchestrator.input.DetectedLanguage)
	assert.Equal(t, provider, orchestrator.input.Provider)
	assert.Equal(t, "orchestrated", mins.Summary.Overview)
	assert.Equal(t, []string{"test_quality_warning"}, mins.QualityWarnings)
	assert.Contains(t, string(mins.Sections["themes"]), "orchestration")
}

func TestGenerateMinutes_ReturnsExistingReadyMinutes(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	service := NewService(repo)

	existing := NewMinutes("existing-minutes", "session-existing", "therapy")
	existing.Summary.Overview = "Already generated"
	existing.MarkReady()
	require.NoError(t, repo.Create(context.Background(), existing))

	provider := &scriptedLLMProvider{
		responses: []string{`{"summary":{"overview":"should not be used","phases":[]},"sections":{},"citations":[]}`},
	}

	mins, err := service.GenerateMinutes(context.Background(), "session-existing", "[0ms therapist]: Ciao", config.DefaultTemplates()[0], webhook.SessionContext{}, "it", provider)
	require.NoError(t, err)
	assert.Equal(t, "existing-minutes", mins.ID)
	assert.Equal(t, "Already generated", mins.Summary.Overview)
	assert.Empty(t, provider.prompts)
}

func TestGenerateMinutes_ReusesExistingErrorMinutes(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	service := NewService(repo)

	existing := NewMinutes("retry-minutes", "session-retry", "therapy")
	existing.MarkError()
	require.NoError(t, repo.Create(context.Background(), existing))

	provider := &scriptedLLMProvider{
		responses: []string{`{
			"summary":{"overview":"Retry completed","phases":[]},
			"sections":{"themes":["retry"]},
			"citations":[]
		}`},
	}

	mins, err := service.GenerateMinutes(context.Background(), "session-retry", "[0ms therapist]: Ripartiamo", config.DefaultTemplates()[0], webhook.SessionContext{}, "it", provider)
	require.NoError(t, err)
	assert.Equal(t, "retry-minutes", mins.ID)
	assert.Equal(t, MinutesStatusReady, mins.Status)
	assert.Equal(t, "Retry completed", mins.Summary.Overview)

	bySession, err := repo.GetBySession(context.Background(), "session-retry")
	require.NoError(t, err)
	assert.Equal(t, "retry-minutes", bySession.ID)
}

func TestGenerateMinutes_UsesProviderScopedRetryPolicy(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	service := NewServiceWithDeps(repo, &RetryConfig{
		MaxRetries:     0,
		InitialBackoff: time.Hour,
		MaxBackoff:     time.Hour,
	}, nil)
	provider := &runtimeTunedLLMProvider{
		failures: 1,
		runtime: llm.RuntimeConfig{
			Retry: llm.RetryConfig{
				MaxAttempts:    2,
				InitialBackoff: time.Millisecond,
				MaxBackoff:     time.Millisecond,
			},
		},
	}

	mins, err := service.GenerateMinutes(context.Background(), "session-profile-retry", "[0ms therapist]: Ciao", config.DefaultTemplates()[0], webhook.SessionContext{}, "it", provider)
	require.NoError(t, err)
	assert.Equal(t, MinutesStatusReady, mins.Status)
	assert.Equal(t, 2, provider.calls)
}

func TestGenerateMinutes_UsesProviderScopedGenerationTimeout(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	service := NewService(repo)
	provider := &runtimeTunedLLMProvider{
		blockUntilContext: true,
		runtime:           llm.RuntimeConfig{GenerationTimeout: time.Nanosecond},
	}

	_, err := service.GenerateMinutes(context.Background(), "session-profile-timeout", "[0ms therapist]: Ciao", config.DefaultTemplates()[0], webhook.SessionContext{}, "it", provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestClassifyGenerationError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{err: context.Canceled, want: "internal_timeout"},
		{err: errors.New("stuck processing exceeded timeout 5m0s"), want: "internal_timeout"},
		{err: context.DeadlineExceeded, want: "provider_timeout"},
		{err: errors.New("OpenAI API error: 401 unauthorized"), want: "provider_auth"},
		{err: errors.New("OpenAI API error: 402 insufficient credits"), want: "provider_quota_or_budget"},
		{err: errors.New("failed to parse LLM response: invalid JSON"), want: "parse_error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, classifyGenerationError(tt.err))
		})
	}
}
