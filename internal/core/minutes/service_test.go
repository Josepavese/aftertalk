package minutes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/pkg/webhook"
)

func init() { //nolint:gochecknoinits // test logger setup
	_ = logging.Init("info", "console")
}

type scriptedLLMProvider struct {
	prompts   []string
	responses []string
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

func TestSplitTranscriptBatches(t *testing.T) {
	transcript := strings.Join([]string{
		"[0ms therapist]: Buongiorno",
		"[1000ms patient]: Buongiorno",
		"[2000ms therapist]: Come sta andando?",
		"[3000ms patient]: Meglio di ieri",
	}, "\n")

	batches := splitTranscriptBatches(transcript, GenerationConfig{
		Incremental:      true,
		BatchMaxSegments: 2,
		BatchMaxChars:    120,
	})

	require.Len(t, batches, 2)
	assert.Contains(t, batches[0], "[0ms therapist]")
	assert.Contains(t, batches[1], "[3000ms patient]")
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
	assert.Len(t, mins.Citations, 1)
	assert.Contains(t, string(mins.Sections["themes"]), "stato emotivo")
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
