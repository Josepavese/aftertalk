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
	URL          string
	Mode         string
	Secret       string
	PullBaseURL  string
	Timeout      time.Duration
	TokenTTL     time.Duration
	DeleteOnPull bool
}

type GenerationConfig struct {
	Incremental      bool
	BatchMaxSegments int
	BatchMaxChars    int
	MaxSummaryPhases int
	MaxCitations     int
}

func DefaultGenerationConfig() GenerationConfig {
	return GenerationConfig{
		Incremental:      true,
		BatchMaxSegments: 24,
		BatchMaxChars:    6000,
		MaxSummaryPhases: 8,
		MaxCitations:     12,
	}
}

type Service struct {
	repo           *MinutesRepository
	retryConfig    *RetryConfig
	webhookClient  *webhook.Client
	webhookConfig  *WebhookConfig
	webhookRetrier *webhook.Retrier
	generation     GenerationConfig
}

func NewService(repo *MinutesRepository) *Service {
	return &Service{
		repo: repo,
		retryConfig: &RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
		generation: DefaultGenerationConfig(),
	}
}

func NewServiceWithDeps(repo *MinutesRepository, retryConfig *RetryConfig, webhookConfig *WebhookConfig) *Service {
	s := &Service{
		repo:        repo,
		retryConfig: retryConfig,
		generation:  DefaultGenerationConfig(),
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

// WithGenerationConfig overrides the minutes generation strategy.
func (s *Service) WithGenerationConfig(cfg GenerationConfig) *Service {
	defaults := DefaultGenerationConfig()
	if cfg.BatchMaxSegments <= 0 {
		cfg.BatchMaxSegments = defaults.BatchMaxSegments
	}
	if cfg.BatchMaxChars <= 0 {
		cfg.BatchMaxChars = defaults.BatchMaxChars
	}
	if cfg.MaxSummaryPhases <= 0 {
		cfg.MaxSummaryPhases = defaults.MaxSummaryPhases
	}
	if cfg.MaxCitations <= 0 {
		cfg.MaxCitations = defaults.MaxCitations
	}
	s.generation = cfg
	return s
}

// WithWebhookRetrier wires a persistent retry worker. When set, webhook
// deliveries are enqueued in the DB instead of fire-and-forget goroutines.
func (s *Service) WithWebhookRetrier(r *webhook.Retrier) {
	s.webhookRetrier = r
}

// GenerateMinutes calls the LLM to produce structured minutes for a session.
// The prompt and expected JSON schema are derived from the provided template.
// sessCtx carries the opaque metadata and participant list set at session-creation
// time; it is propagated unchanged to every webhook delivery so recipients can
// associate the minutes with their own data model (e.g. appointment_id, user IDs)
// without maintaining a separate session-id → context mapping table.
// GenerateMinutes generates structured minutes using the given LLM provider.
// Provider resolution (profile → concrete provider) is the caller's responsibility
// and must happen in the Middleware/Adapter layer, not here.
func (s *Service) GenerateMinutes(ctx context.Context, sessionID string, transcriptionText string, tmpl config.TemplateConfig, sessCtx webhook.SessionContext, detectedLanguage string, provider llm.LLMProvider) (*Minutes, error) {
	logging.Infof("Generating minutes for session %s (template=%s)", sessionID, tmpl.ID)

	m := NewMinutes(uuid.New().String(), sessionID, tmpl.ID)
	m.Provider = provider.Name()

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("failed to create minutes: %w", err)
	}

	if strings.TrimSpace(transcriptionText) == "" {
		logging.Warnf("Session %s has no transcription text; storing empty minutes", sessionID)
		m.MarkReady()
		if err := s.repo.Update(ctx, m); err != nil {
			logging.Errorf("Failed to update minutes: %v", err)
		}
		return m, nil
	}

	parsed, err := s.generateStructuredMinutes(ctx, transcriptionText, tmpl, detectedLanguage, provider)
	if err != nil {
		m.MarkError()
		if updateErr := s.repo.Update(ctx, m); updateErr != nil {
			logging.Errorf("Failed to update minutes: %v", updateErr)
		}
		return nil, fmt.Errorf("failed to generate minutes: %w", err)
	}

	m.Summary = convertSummary(parsed.Summary)
	m.Sections = parsed.Sections
	m.Citations = convertCitations(parsed.Citations)
	m.MarkReady()

	if err := s.repo.Update(ctx, m); err != nil {
		return nil, fmt.Errorf("failed to update minutes: %w", err)
	}

	logging.Infof("Minutes generated successfully for session %s", sessionID)
	go s.deliverWebhook(sessionID, m, sessCtx) //nolint:contextcheck,gosec // webhook delivery is intentionally fire-and-forget, must outlive the request context

	return m, nil
}

func (s *Service) generateStructuredMinutes(ctx context.Context, transcriptionText string, tmpl config.TemplateConfig, detectedLanguage string, provider llm.LLMProvider) (*llm.DynamicMinutesResponse, error) {
	batches := splitTranscriptBatches(transcriptionText, s.generation)
	if len(batches) == 0 {
		batches = []string{transcriptionText}
	}

	state := &llm.DynamicMinutesResponse{}
	for i, batch := range batches {
		logging.Infof("Generating minutes batch %d/%d", i+1, len(batches))
		prompt := llm.GenerateIncrementalMinutesPrompt(state, batch, tmpl, detectedLanguage, len(batches) == 1)
		updated, err := s.generateLLMState(ctx, provider, prompt, tmpl, detectedLanguage)
		if err != nil {
			return nil, fmt.Errorf("batch %d/%d: %w", i+1, len(batches), err)
		}
		state = normalizeDynamicMinutes(updated, tmpl, s.generation)
	}

	if len(batches) > 1 {
		logging.Infof("Finalizing minutes after %d incremental batches", len(batches))
		prompt := llm.GenerateMinutesFinalizePrompt(state, tmpl, detectedLanguage)
		finalized, err := s.generateLLMState(ctx, provider, prompt, tmpl, detectedLanguage)
		if err != nil {
			return nil, fmt.Errorf("finalize minutes: %w", err)
		}
		state = normalizeDynamicMinutes(finalized, tmpl, s.generation)
	}

	return state, nil
}

func (s *Service) generateLLMState(ctx context.Context, provider llm.LLMProvider, prompt string, tmpl config.TemplateConfig, detectedLanguage string) (*llm.DynamicMinutesResponse, error) {
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

		response, genErr := provider.Generate(ctx, prompt)
		if genErr != nil {
			err = genErr
			logging.Errorf("LLM request failed (attempt %d): %v", attempt+1, genErr)
			continue
		}

		parsed, parseErr := llm.ParseMinutesDynamic(response)
		if parseErr == nil {
			return normalizeDynamicMinutes(parsed, tmpl, s.generation), nil
		}

		repairPrompt := llm.GenerateMinutesRepairPrompt(response, tmpl, detectedLanguage)
		repairResponse, repairErr := provider.Generate(ctx, repairPrompt)
		if repairErr == nil {
			repaired, repairedErr := llm.ParseMinutesDynamic(repairResponse)
			if repairedErr == nil {
				return normalizeDynamicMinutes(repaired, tmpl, s.generation), nil
			}
			parseErr = repairedErr
		} else {
			logging.Errorf("LLM repair failed (attempt %d): %v", attempt+1, repairErr)
		}

		err = fmt.Errorf("failed to parse LLM response: %w", parseErr)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate minutes after %d retries: %w", s.retryConfig.MaxRetries+1, err)
	}
	return nil, fmt.Errorf("failed to generate minutes after %d retries", s.retryConfig.MaxRetries+1)
}

// deliverWebhook dispatches the minutes notification based on the configured mode.
//
//   - "push" (legacy): full minutes JSON POSTed to the webhook URL.
//   - "notify_pull": a signed notification carrying only a retrieval URL is POSTed;
//     the recipient must call GET /v1/minutes/pull/{token} to fetch the data.
func (s *Service) deliverWebhook(sessionID string, m *Minutes, sessCtx webhook.SessionContext) {
	if s.webhookClient == nil {
		return
	}

	if s.webhookConfig != nil && s.webhookConfig.Mode == webhookModeNotifyPull {
		s.deliverNotification(sessionID, m, sessCtx)
		return
	}

	// Push mode: send full minutes payload together with session context.
	payload := &webhook.MinutesPayload{
		SessionID:       sessionID,
		Minutes:         m,
		Timestamp:       time.Now(),
		SessionMetadata: sessCtx.Metadata,
		Participants:    sessCtx.Participants,
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
	if updateErr := s.repo.Update(context.Background(), m); updateErr != nil {
		logging.Errorf("Failed to update minutes after delivery: %v", updateErr)
	}
}

// deliverNotification implements the notify_pull delivery strategy:
//  1. Generate a single-use retrieval token stored in the DB.
//  2. Build the retrieval URL: {pull_base_url}/v1/minutes/pull/{tokenID}.
//  3. POST a NotificationPayload (no sensitive data) to the webhook URL.
//     The payload is HMAC-SHA256 signed if a secret is configured.
//  4. The actual minutes data is transmitted only when the recipient calls
//     the retrieval URL — at which point it is deleted from the DB.
func (s *Service) deliverNotification(sessionID string, m *Minutes, sessCtx webhook.SessionContext) {
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
		SessionID:       sessionID,
		RetrieveURL:     retrieveURL,
		ExpiresAt:       tok.ExpiresAt,
		Timestamp:       time.Now(),
		SessionMetadata: sessCtx.Metadata,
		Participants:    sessCtx.Participants,
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

	if updatedMinutes.Sections != nil {
		m.Sections = updatedMinutes.Sections
	}
	if updatedMinutes.Citations != nil {
		m.Citations = updatedMinutes.Citations
	}
	if !updatedMinutes.Summary.IsZero() {
		m.Summary = updatedMinutes.Summary
	}
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

func convertSummary(summary llm.Summary) Summary {
	phases := make([]Phase, len(summary.Phases))
	for i, phase := range summary.Phases {
		phases[i] = Phase{
			Title:   phase.Title,
			Summary: phase.Summary,
			StartMs: phase.StartMs,
			EndMs:   phase.EndMs,
		}
	}
	return Summary{
		Overview: summary.Overview,
		Phases:   phases,
	}
}
