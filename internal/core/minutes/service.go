package minutes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/pkg/webhook"
	"github.com/google/uuid"
)

// webhookModeNotifyPull is the notify_pull delivery mode identifier.
const webhookModeNotifyPull = "notify_pull"

type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// WebhookConfig mirrors the relevant subset of config.WebhookConfig for the
// minutes service. See config.WebhookConfig for full documentation.
type WebhookConfig struct {
	URL     string
	Timeout time.Duration
	// Mode: "push" (default) or "notify_pull". See config.WebhookConfig.
	Mode string
	// Secret is the HMAC-SHA256 signing key for notify_pull notifications.
	Secret string
	// TokenTTL is how long a pull token remains valid (default: 1h).
	TokenTTL time.Duration
	// DeleteOnPull removes minutes + transcriptions after a successful pull.
	// Defaults to true when Mode = "notify_pull".
	DeleteOnPull bool
	// PullBaseURL is the public base URL used to build the retrieval URL.
	PullBaseURL string
}

type Service struct {
	repo           *MinutesRepository
	llmProvider    llm.LLMProvider
	retryConfig    *RetryConfig
	webhookClient  *webhook.Client
	webhookConfig  *WebhookConfig
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
		if webhookConfig.Mode == webhookModeNotifyPull && webhookConfig.Secret != "" {
			s.webhookClient = webhook.NewClientWithSecret(webhookConfig.URL, webhookConfig.Secret, timeout)
		} else {
			s.webhookClient = webhook.NewClient(webhookConfig.URL, timeout)
		}
		s.webhookConfig = webhookConfig
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

// deliverWebhook dispatches the minutes notification based on the configured mode.
//
//   - "push" (legacy): full minutes JSON POSTed to the webhook URL.
//   - "notify_pull": a signed notification carrying only a retrieval URL is POSTed;
//     the recipient must call GET /v1/minutes/pull/{token} to fetch the data.
func (s *Service) deliverWebhook(sessionID string, m *Minutes) {
	if s.webhookClient == nil {
		return
	}

	if s.webhookConfig != nil && s.webhookConfig.Mode == webhookModeNotifyPull {
		s.deliverNotification(sessionID, m)
		return
	}

	// Legacy push mode: send full minutes payload.
	payload := &webhook.MinutesPayload{
		SessionID: sessionID,
		Minutes:   m,
		Timestamp: time.Now(),
	}

	if s.webhookRetrier != nil {
		ctx := context.Background()
		if err := s.webhookRetrier.Enqueue(ctx, m.ID, s.webhookClient.URL(), payload); err != nil {
			logging.Errorf("webhook retrier: enqueue failed for session %s: %v", sessionID, err)
		}
		return
	}

	// Fallback: direct delivery, no retry.
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

// deliverNotification implements the notify_pull delivery strategy:
//  1. Generate a single-use retrieval token stored in the DB.
//  2. Build the retrieval URL: {pull_base_url}/v1/minutes/pull/{tokenID}.
//  3. POST a NotificationPayload (no sensitive data) to the webhook URL.
//     The payload is HMAC-SHA256 signed if a secret is configured.
//  4. The actual minutes data is transmitted only when the recipient calls
//     the retrieval URL — at which point it is deleted from the DB.
func (s *Service) deliverNotification(sessionID string, m *Minutes) {
	ttl := s.webhookConfig.TokenTTL
	if ttl == 0 {
		ttl = time.Hour
	}
	tok := &RetrievalToken{
		ID:        uuid.New().String(),
		MinutesID: m.ID,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}

	ctx := context.Background()
	if err := s.repo.CreateRetrievalToken(ctx, tok); err != nil {
		logging.Errorf("deliverNotification: create token for session %s: %v", sessionID, err)
		return
	}

	retrieveURL := s.webhookConfig.PullBaseURL + "/v1/minutes/pull/" + tok.ID
	payload := &webhook.NotificationPayload{
		SessionID:   sessionID,
		RetrieveURL: retrieveURL,
		ExpiresAt:   tok.ExpiresAt,
		Timestamp:   time.Now(),
	}

	if s.webhookRetrier != nil {
		if err := s.webhookRetrier.EnqueueNotification(ctx, m.ID, s.webhookClient.URL(), payload); err != nil {
			logging.Errorf("webhook retrier: enqueue notification failed for session %s: %v", sessionID, err)
		}
		return
	}

	// Fallback: direct delivery, no retry.
	dCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.webhookClient.SendNotification(dCtx, payload); err != nil {
		logging.Errorf("Failed to deliver notification for session %s: %v", sessionID, err)
	} else {
		logging.Infof("Notification webhook delivered for session %s (token=%s)", sessionID, tok.ID)
	}
}

// ConsumeRetrievalToken validates and atomically consumes a pull token.
// Returns the token (with MinutesID) on success. Any failure returns a generic
// error — the caller should respond with 404 to prevent oracle attacks.
func (s *Service) ConsumeRetrievalToken(ctx context.Context, tokenID string) (*RetrievalToken, error) {
	return s.repo.ConsumeToken(ctx, tokenID)
}

// PurgeMinutes deletes a minutes record, its history, its transcriptions, and
// its retrieval tokens from the DB. Used after a successful pull when
// delete_on_pull is enabled — Aftertalk becomes a pure processing pipeline,
// not a long-term archive of sensitive session data.
func (s *Service) PurgeMinutes(ctx context.Context, minutesID string) {
	// Retrieve session_id so we can also clean up transcriptions.
	m, err := s.repo.GetByID(ctx, minutesID)
	if err != nil {
		logging.Errorf("PurgeMinutes: get minutes %s: %v", minutesID, err)
		return
	}

	// Delete transcriptions for the session (they contain the full STT text).
	if _, err := s.repo.ExecContext(ctx,
		"DELETE FROM transcriptions WHERE session_id = ?", m.SessionID); err != nil {
		logging.Errorf("PurgeMinutes: delete transcriptions for session %s: %v", m.SessionID, err)
	}

	// Delete the minutes record (cascades to minutes_history and retrieval_tokens).
	if err := s.repo.Delete(ctx, minutesID); err != nil {
		logging.Errorf("PurgeMinutes: delete minutes %s: %v", minutesID, err)
		return
	}

	logging.Infof("PurgeMinutes: session %s purged after pull", m.SessionID)
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
