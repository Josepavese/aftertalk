package minutes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flowup/aftertalk/internal/ai/llm"
	"github.com/flowup/aftertalk/internal/config"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/flowup/aftertalk/pkg/webhook"
	"github.com/google/uuid"
)

type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

type WebhookConfig struct {
	URL     string
	Timeout time.Duration
}

type Service struct {
	repo           *MinutesRepository
	llmProvider    llm.LLMProvider
	retryConfig    *RetryConfig
	webhookClient  *webhook.Client
	webhookRetrier *webhook.Retrier
}

func NewService(repo *MinutesRepository, provider llm.LLMProvider) *Service {
	return &Service{
		repo:        repo,
		llmProvider: provider,
		retryConfig: &RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	}
}

func NewServiceWithDeps(repo *MinutesRepository, provider llm.LLMProvider, retryConfig *RetryConfig, webhookConfig *WebhookConfig) *Service {
	s := &Service{
		repo:        repo,
		llmProvider: provider,
		retryConfig: retryConfig,
	}
	if webhookConfig != nil && webhookConfig.URL != "" {
		timeout := webhookConfig.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		s.webhookClient = webhook.NewClient(webhookConfig.URL, timeout)
	}
	return s
}

// WithWebhookRetrier wires a persistent retry worker. When set, webhook
// deliveries are enqueued in the DB instead of fire-and-forget goroutines.
func (s *Service) WithWebhookRetrier(r *webhook.Retrier) {
	s.webhookRetrier = r
}

// GenerateMinutes calls the LLM to produce structured minutes for a session.
// The prompt and expected JSON schema are derived from the provided template.
func (s *Service) GenerateMinutes(ctx context.Context, sessionID string, transcriptionText string, tmpl config.TemplateConfig) (*Minutes, error) {
	logging.Infof("Generating minutes for session %s (template=%s)", sessionID, tmpl.ID)

	m := NewMinutes(uuid.New().String(), sessionID, tmpl.ID)
	m.Provider = s.llmProvider.Name()

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("failed to create minutes: %w", err)
	}

	if strings.TrimSpace(transcriptionText) == "" {
		logging.Warnf("Session %s has no transcription text; storing empty minutes", sessionID)
		m.MarkReady()
		s.repo.Update(ctx, m)
		return m, nil
	}

	var response string
	var err error

	for attempt := 0; attempt <= s.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := s.retryConfig.InitialBackoff * time.Duration(1<<uint(attempt-1))
			if backoff > s.retryConfig.MaxBackoff {
				backoff = s.retryConfig.MaxBackoff
			}
			logging.Warnf("LLM request failed, retrying in %v (attempt %d/%d)", backoff, attempt, s.retryConfig.MaxRetries)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		prompt := llm.GenerateMinutesPrompt(transcriptionText, tmpl)
		response, err = s.llmProvider.Generate(ctx, prompt)
		if err == nil {
			break
		}
		logging.Errorf("LLM request failed (attempt %d): %v", attempt+1, err)
	}

	if err != nil {
		m.MarkError()
		s.repo.Update(ctx, m)
		return nil, fmt.Errorf("failed to generate minutes after %d retries: %w", s.retryConfig.MaxRetries+1, err)
	}

	parsed, err := llm.ParseMinutesDynamic(response)
	if err != nil {
		m.MarkError()
		s.repo.Update(ctx, m)
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	m.Sections = parsed.Sections
	m.Citations = convertCitations(parsed.Citations)
	m.MarkReady()

	if err := s.repo.Update(ctx, m); err != nil {
		return nil, fmt.Errorf("failed to update minutes: %w", err)
	}

	logging.Infof("Minutes generated successfully for session %s", sessionID)
	go s.deliverWebhook(sessionID, m)

	return m, nil
}

func (s *Service) deliverWebhook(sessionID string, m *Minutes) {
	if s.webhookClient == nil {
		return
	}
	payload := &webhook.MinutesPayload{
		SessionID: sessionID,
		Minutes:   m,
		Timestamp: time.Now(),
	}

	// Prefer persistent retry queue over fire-and-forget.
	if s.webhookRetrier != nil {
		ctx := context.Background()
		if err := s.webhookRetrier.Enqueue(ctx, m.ID, s.webhookClient.URL(), payload); err != nil {
			logging.Errorf("webhook retrier: enqueue failed for session %s: %v", sessionID, err)
		}
		return
	}

	// Fallback: direct delivery (fire-and-forget, no retry).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.webhookClient.Send(ctx, payload); err != nil {
		logging.Errorf("Failed to deliver webhook for session %s: %v", sessionID, err)
		return
	}
	logging.Infof("Webhook delivered for session %s", sessionID)
	m.MarkDelivered()
	s.repo.Update(context.Background(), m)
}

func (s *Service) GetMinutes(ctx context.Context, sessionID string) (*Minutes, error) {
	m, err := s.repo.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}
	return m, nil
}

func (s *Service) GetMinutesByID(ctx context.Context, id string) (*Minutes, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}
	return m, nil
}

func (s *Service) UpdateMinutes(ctx context.Context, id string, updatedMinutes *Minutes, editedBy string) (*Minutes, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	// Snapshot current version for history.
	snapshot := snapshotJSON(m)
	history := NewMinutesHistory(uuid.New().String(), m.ID, m.Version, snapshot)
	history.SetEditedBy(editedBy)
	if err := s.repo.CreateHistory(ctx, history); err != nil {
		return nil, fmt.Errorf("failed to create history: %w", err)
	}

	m.Sections = updatedMinutes.Sections
	m.Citations = updatedMinutes.Citations
	m.IncrementVersion()

	if err := s.repo.Update(ctx, m); err != nil {
		return nil, fmt.Errorf("failed to update minutes: %w", err)
	}
	return m, nil
}

func (s *Service) GetMinutesHistory(ctx context.Context, minutesID string) ([]*MinutesHistory, error) {
	history, err := s.repo.GetHistory(ctx, minutesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	return history, nil
}

func (s *Service) DeleteMinutes(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete minutes: %w", err)
	}
	return nil
}

func convertCitations(citations []llm.Citation) []Citation {
	result := make([]Citation, len(citations))
	for i, c := range citations {
		result[i] = Citation{TimestampMs: c.TimestampMs, Text: c.Text, Role: c.Role}
	}
	return result
}
