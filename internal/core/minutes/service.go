package minutes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flowup/aftertalk/internal/ai/llm"
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

func (s *Service) GenerateMinutes(ctx context.Context, sessionID string, transcriptionText string, roles []string) (*Minutes, error) {
	logging.Infof("Generating minutes for session %s", sessionID)

	minutes := NewMinutes(uuid.New().String(), sessionID)
	minutes.Provider = s.llmProvider.Name()

	if err := s.repo.Create(ctx, minutes); err != nil {
		return nil, fmt.Errorf("failed to create minutes: %w", err)
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

		prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)
		response, err = s.llmProvider.Generate(ctx, prompt)
		if err == nil {
			break
		}

		logging.Errorf("LLM request failed (attempt %d): %v", attempt+1, err)
	}

	if err != nil {
		minutes.MarkError()
		s.repo.Update(ctx, minutes)
		return nil, fmt.Errorf("failed to generate minutes after %d retries: %w", s.retryConfig.MaxRetries+1, err)
	}

	minutesResponse, err := llm.ParseMinutesResponse(response)
	if err != nil {
		minutes.MarkError()
		s.repo.Update(ctx, minutes)
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	minutes.Themes = minutesResponse.Themes
	minutes.ContentsReported = convertContentItems(minutesResponse.ContentsReported)
	minutes.ProfessionalInterventions = convertContentItems(minutesResponse.ProfessionalInterventions)
	minutes.ProgressIssues = Progress{
		Progress: minutesResponse.ProgressIssues.Progress,
		Issues:   minutesResponse.ProgressIssues.Issues,
	}
	minutes.NextSteps = minutesResponse.NextSteps
	minutes.Citations = convertCitations(minutesResponse.Citations)
	minutes.MarkReady()

	if err := s.repo.Update(ctx, minutes); err != nil {
		return nil, fmt.Errorf("failed to update minutes: %w", err)
	}

	logging.Infof("Minutes generated successfully for session %s", sessionID)

	go s.deliverWebhook(sessionID, minutes)

	return minutes, nil
}

func (s *Service) deliverWebhook(sessionID string, minutes *Minutes) {
	if s.webhookClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload := &webhook.MinutesPayload{
		SessionID: sessionID,
		Minutes:   minutes,
		Timestamp: time.Now(),
	}

	err := s.webhookClient.Send(ctx, payload)
	if err != nil {
		logging.Errorf("Failed to deliver webhook for session %s: %v", sessionID, err)
		return
	}

	logging.Infof("Webhook delivered successfully for session %s", sessionID)

	minutes.MarkDelivered()
	if err := s.repo.Update(context.Background(), minutes); err != nil {
		logging.Errorf("Failed to update minutes delivery status: %v", err)
	}
}

func convertContentItems(items []llm.ContentItem) []ContentItem {
	result := make([]ContentItem, len(items))
	for i, item := range items {
		result[i] = ContentItem{
			Text:      item.Text,
			Timestamp: item.Timestamp,
		}
	}
	return result
}

func convertCitations(citations []llm.Citation) []Citation {
	result := make([]Citation, len(citations))
	for i, c := range citations {
		result[i] = Citation{
			TimestampMs: c.TimestampMs,
			Text:        c.Text,
			Role:        c.Role,
		}
	}
	return result
}

func (s *Service) GetMinutes(ctx context.Context, sessionID string) (*Minutes, error) {
	minutes, err := s.repo.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	return minutes, nil
}

func (s *Service) GetMinutesByID(ctx context.Context, id string) (*Minutes, error) {
	minutes, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	return minutes, nil
}

func (s *Service) UpdateMinutes(ctx context.Context, id string, updatedMinutes *Minutes, editedBy string) (*Minutes, error) {
	minutes, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	contentJSON, _ := json.Marshal(minutes)
	history := NewMinutesHistory(uuid.New().String(), minutes.ID, minutes.Version, string(contentJSON))
	history.SetEditedBy(editedBy)

	if err := s.repo.CreateHistory(ctx, history); err != nil {
		return nil, fmt.Errorf("failed to create history: %w", err)
	}

	minutes.Themes = updatedMinutes.Themes
	minutes.ContentsReported = updatedMinutes.ContentsReported
	minutes.ProfessionalInterventions = updatedMinutes.ProfessionalInterventions
	minutes.ProgressIssues = updatedMinutes.ProgressIssues
	minutes.NextSteps = updatedMinutes.NextSteps
	minutes.Citations = updatedMinutes.Citations
	minutes.IncrementVersion()

	if err := s.repo.Update(ctx, minutes); err != nil {
		return nil, fmt.Errorf("failed to update minutes: %w", err)
	}

	return minutes, nil
}

func (s *Service) GetMinutesHistory(ctx context.Context, minutesID string) ([]*MinutesHistory, error) {
	history, err := s.repo.GetHistory(ctx, minutesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	return history, nil
}
