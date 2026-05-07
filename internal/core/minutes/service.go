package minutes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/internal/minutesgen"
	"github.com/Josepavese/aftertalk/pkg/webhook"
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

type GenerationConfig = minutesgen.Config

func DefaultGenerationConfig() GenerationConfig {
	return minutesgen.DefaultConfig()
}

type Service struct {
	repo           *MinutesRepository
	retryConfig    *RetryConfig
	webhookClient  *webhook.Client
	webhookConfig  *WebhookConfig
	webhookRetrier *webhook.Retrier
	orchestrator   minutesgen.Orchestrator
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
		orchestrator: minutesgen.NewDefaultOrchestrator(),
		generation:   DefaultGenerationConfig(),
	}
}

func NewServiceWithDeps(repo *MinutesRepository, retryConfig *RetryConfig, webhookConfig *WebhookConfig) *Service {
	s := &Service{
		repo:         repo,
		retryConfig:  retryConfig,
		orchestrator: minutesgen.NewDefaultOrchestrator(),
		generation:   DefaultGenerationConfig(),
	}
	if webhookConfig != nil && webhookConfig.URL != "" {
		timeout := webhookConfig.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		if webhookConfig.Secret != "" {
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

// WithGenerationOrchestrator swaps the generation engine without changing
// persistence, session, or webhook behavior. Intended for tests and future
// adapters to external orchestration backends.
func (s *Service) WithGenerationOrchestrator(orchestrator minutesgen.Orchestrator) *Service {
	if orchestrator != nil {
		s.orchestrator = orchestrator
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
// sessCtx carries the opaque metadata and participant list set at session-creation
// time; it is propagated unchanged to every webhook delivery so recipients can
// associate the minutes with their own data model (e.g. appointment_id, user IDs)
// without maintaining a separate session-id → context mapping table.
// GenerateMinutes generates structured minutes using the given LLM provider.
// Provider resolution (profile → concrete provider) is the caller's responsibility
// and must happen in the Middleware/Adapter layer, not here.
func (s *Service) GenerateMinutes(ctx context.Context, sessionID, transcriptionText string, tmpl config.TemplateConfig, sessCtx webhook.SessionContext, detectedLanguage string, provider llm.LLMProvider) (*Minutes, error) {
	logging.Infof("Generating minutes for session %s (template=%s)", sessionID, tmpl.ID)

	m, err := s.prepareMinutesRecord(ctx, sessionID, tmpl.ID, provider.Name())
	if err != nil {
		return nil, err
	}
	if m.Status == MinutesStatusReady || m.Status == MinutesStatusDelivered {
		logging.Infof("Minutes already generated for session %s (minutes=%s, status=%s)", sessionID, m.ID, m.Status)
		s.ensureWebhookDelivery(ctx, sessionID, m, sessCtx)
		return m, nil
	}

	if strings.TrimSpace(transcriptionText) == "" {
		logging.Warnf("Session %s has no transcription text; storing empty minutes", sessionID)
		m.MarkReady()
		if err := s.repo.Update(ctx, m); err != nil {
			logging.Errorf("Failed to update minutes: %v", err)
		}
		return m, nil
	}

	generationCtx := ctx
	if timeout := generationTimeoutForProvider(provider); timeout > 0 {
		var cancel context.CancelFunc
		generationCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result, err := s.generationOrchestrator().Generate(generationCtx, minutesgen.Input{
		Provider:         provider,
		Template:         tmpl,
		Config:           s.generation,
		Retry:            convertRetryConfig(s.retryConfigForProvider(provider)),
		Transcript:       transcriptionText,
		DetectedLanguage: detectedLanguage,
	})
	if err != nil {
		s.markMinutesError(sessionID, m, sessCtx, err)
		return nil, fmt.Errorf("failed to generate minutes: %w", err)
	}
	parsed := result.State

	m.Summary = convertSummary(parsed.Summary)
	m.Sections = parsed.Sections
	m.Citations = convertCitations(parsed.Citations)
	m.QualityWarnings = result.QualityWarnings
	if len(m.QualityWarnings) > 0 {
		logging.Warnf("Minutes quality warnings for session %s: %s", sessionID, strings.Join(m.QualityWarnings, ", "))
	}
	m.MarkReady()

	if err := s.repo.Update(ctx, m); err != nil {
		return nil, fmt.Errorf("failed to update minutes: %w", err)
	}

	logging.Infof("Minutes generated successfully for session %s", sessionID)
	s.deliverWebhook(sessionID, m, sessCtx)

	return m, nil
}

func (s *Service) generationOrchestrator() minutesgen.Orchestrator {
	if s.orchestrator != nil {
		return s.orchestrator
	}
	return minutesgen.NewDefaultOrchestrator()
}

func (s *Service) MarkSessionError(ctx context.Context, sessionID, templateID, providerName string, sessCtx webhook.SessionContext, cause error) error {
	m, err := s.prepareMinutesRecord(ctx, sessionID, templateID, providerName)
	if err != nil {
		return err
	}
	s.markMinutesError(sessionID, m, sessCtx, cause)
	return nil
}

func (s *Service) markMinutesError(sessionID string, m *Minutes, sessCtx webhook.SessionContext, cause error) {
	m.MarkError()
	updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if updateErr := s.repo.Update(updateCtx, m); updateErr != nil {
		logging.Errorf("Failed to update minutes error state for session=%s minutes=%s: %v", sessionID, m.ID, updateErr)
	}
	s.deliverErrorWebhook(sessionID, m, sessCtx, cause)
}

func (s *Service) prepareMinutesRecord(ctx context.Context, sessionID, templateID, providerName string) (*Minutes, error) {
	existing, err := s.repo.GetBySession(ctx, sessionID)
	if err == nil {
		existing.TemplateID = templateID
		existing.Provider = providerName
		if existing.Status == MinutesStatusError {
			existing.Status = MinutesStatusPending
			existing.DeliveredAt = nil
			if updateErr := s.repo.Update(ctx, existing); updateErr != nil {
				return nil, fmt.Errorf("failed to reset existing minutes: %w", updateErr)
			}
		}
		return existing, nil
	}
	if !errors.Is(err, errMinutesNotFound) {
		return nil, fmt.Errorf("failed to inspect existing minutes: %w", err)
	}

	m := NewMinutes(uuid.New().String(), sessionID, templateID)
	m.Provider = providerName
	if err := s.repo.Create(ctx, m); err != nil {
		// Concurrent recovery/manual triggers can race on the UNIQUE(session_id)
		// constraint. Re-read by session and continue from the single canonical row.
		existing, getErr := s.repo.GetBySession(ctx, sessionID)
		if getErr == nil {
			existing.TemplateID = templateID
			existing.Provider = providerName
			return existing, nil
		}
		return nil, fmt.Errorf("failed to create minutes: %w", err)
	}
	return m, nil
}

func (s *Service) ensureWebhookDelivery(ctx context.Context, sessionID string, m *Minutes, sessCtx webhook.SessionContext) {
	if s.webhookClient == nil {
		return
	}
	if s.webhookRetrier != nil {
		hasEvent, err := s.repo.HasWebhookEvent(ctx, m.ID)
		if err != nil {
			logging.Errorf("ensureWebhookDelivery: inspect webhook event for minutes=%s: %v", m.ID, err)
			return
		}
		if hasEvent {
			return
		}
	}
	s.deliverWebhook(sessionID, m, sessCtx)
}

func (s *Service) retryConfigForProvider(provider llm.LLMProvider) RetryConfig {
	cfg := s.effectiveDefaultRetryConfig()
	runtimeProvider, ok := provider.(llm.RuntimeConfigProvider)
	if !ok {
		return cfg
	}
	runtime := runtimeProvider.RuntimeConfig()
	if runtime.Retry.MaxAttempts > 0 {
		cfg.MaxRetries = runtime.Retry.MaxAttempts - 1
	}
	if runtime.Retry.InitialBackoff > 0 {
		cfg.InitialBackoff = runtime.Retry.InitialBackoff
	}
	if runtime.Retry.MaxBackoff > 0 {
		cfg.MaxBackoff = runtime.Retry.MaxBackoff
	}
	return normalizeRetryConfig(cfg)
}

func (s *Service) effectiveDefaultRetryConfig() RetryConfig {
	if s.retryConfig == nil {
		return normalizeRetryConfig(RetryConfig{
			MaxRetries:     3,
			InitialBackoff: time.Second,
			MaxBackoff:     10 * time.Second,
		})
	}
	return normalizeRetryConfig(*s.retryConfig)
}

func normalizeRetryConfig(cfg RetryConfig) RetryConfig {
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = cfg.InitialBackoff
	}
	if cfg.MaxBackoff < cfg.InitialBackoff {
		cfg.MaxBackoff = cfg.InitialBackoff
	}
	return cfg
}

func convertRetryConfig(cfg RetryConfig) minutesgen.RetryConfig {
	return minutesgen.RetryConfig{
		MaxRetries:     cfg.MaxRetries,
		InitialBackoff: cfg.InitialBackoff,
		MaxBackoff:     cfg.MaxBackoff,
	}
}

func generationTimeoutForProvider(provider llm.LLMProvider) time.Duration {
	runtimeProvider, ok := provider.(llm.RuntimeConfigProvider)
	if !ok {
		return 0
	}
	return runtimeProvider.RuntimeConfig().GenerationTimeout
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

func (s *Service) deliverErrorWebhook(sessionID string, m *Minutes, sessCtx webhook.SessionContext, cause error) {
	if s.webhookClient == nil {
		return
	}
	payload := &webhook.ErrorPayload{
		SessionID:       sessionID,
		MinutesID:       m.ID,
		Status:          string(MinutesStatusError),
		ErrorCode:       classifyGenerationError(cause),
		ErrorMessage:    safeErrorMessage(cause),
		Timestamp:       time.Now(),
		SessionMetadata: sessCtx.Metadata,
		Participants:    sessCtx.Participants,
	}
	if s.webhookRetrier != nil {
		ctx := context.Background()
		if err := s.webhookRetrier.EnqueueError(ctx, m.ID, s.webhookClient.URL(), payload); err != nil {
			logging.Errorf("webhook retrier: enqueue error failed for session %s: %v", sessionID, err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.webhookClient.SendError(ctx, payload); err != nil {
		logging.Errorf("Failed to deliver error webhook for session %s: %v", sessionID, err)
	}
}

func classifyGenerationError(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.Canceled), strings.Contains(msg, "stuck processing exceeded"), strings.Contains(msg, "generation deadline"):
		return "internal_timeout"
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "provider_timeout"
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"), strings.Contains(msg, "unauthorized"), strings.Contains(msg, "forbidden"), strings.Contains(msg, "api_key"):
		return "provider_auth"
	case strings.Contains(msg, "402"), strings.Contains(msg, "quota"), strings.Contains(msg, "credit"), strings.Contains(msg, "budget"), strings.Contains(msg, "max_tokens"):
		return "provider_quota_or_budget"
	case strings.Contains(msg, "parse"), strings.Contains(msg, "json"), strings.Contains(msg, "reasoning/thinking"):
		return "parse_error"
	case strings.Contains(msg, fmt.Sprintf("http %d", http.StatusInternalServerError)):
		return "provider_timeout"
	default:
		return "unknown"
	}
}

func safeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return msg
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
	if err := ctx.Err(); err != nil {
		logging.Warnf("PurgeMinutes: cleanup skipped for minutes %s: context already done: %v", minutesID, err)
		return
	}
	// Retrieve session_id so we can also clean up transcriptions.
	m, err := s.repo.GetByID(ctx, minutesID)
	if err != nil {
		logPurgeError("get minutes", minutesID, err)
		return
	}

	// Delete transcriptions for the session (they contain the full STT text).
	if _, err := s.repo.ExecContext(ctx,
		"DELETE FROM transcriptions WHERE session_id = ?", m.SessionID); err != nil {
		logPurgeError("delete transcriptions for session", m.SessionID, err)
	}

	// Delete the minutes record (cascades to minutes_history and retrieval_tokens).
	if err := s.repo.Delete(ctx, minutesID); err != nil {
		logPurgeError("delete minutes", minutesID, err)
		return
	}

	logging.Infof("PurgeMinutes: session %s purged after pull", m.SessionID)
}

func logPurgeError(action, id string, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		logging.Warnf("PurgeMinutes: cleanup %s %s stopped by context: %v", action, id, err)
		return
	}
	logging.Errorf("PurgeMinutes: cleanup %s %s failed: %v", action, id, err)
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

func (s *Service) ListWebhookEvents(ctx context.Context, sessionID, minutesID string) ([]WebhookEvent, error) {
	return s.repo.ListWebhookEvents(ctx, sessionID, minutesID)
}

func (s *Service) ReplayWebhookEvent(ctx context.Context, eventID string) error {
	return s.repo.ReplayWebhookEvent(ctx, eventID)
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
