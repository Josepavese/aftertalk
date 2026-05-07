package minutes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/core"
)

var errMinutesNotFound = errors.New("minutes not found")

type MinutesRepository struct {
	*core.BaseRepository
}

type WebhookEvent struct {
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	NextRetryAt   *time.Time `json:"next_retry_at,omitempty"`
	ID            string     `json:"id"`
	MinutesID     string     `json:"minutes_id"`
	WebhookURL    string     `json:"webhook_url"`
	PayloadType   string     `json:"payload_type"`
	Status        string     `json:"status"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	AttemptNumber int        `json:"attempt_number"`
}

type LLMUsageFilter struct {
	From      *time.Time
	To        *time.Time
	SessionID string
	MinutesID string
	Model     string
	Profile   string
}

type LLMUsageGroup struct {
	CostCredits      float64 `json:"cost_credits"`
	Key              string  `json:"key"`
	Calls            int     `json:"calls"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	ReasoningTokens  int     `json:"reasoning_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	TotalTokens      int     `json:"total_tokens"`
}

type LLMUsageReport struct {
	GeneratedAt time.Time       `json:"generated_at"`
	GroupBy     string          `json:"group_by"`
	Total       LLMUsageSummary `json:"total"`
	Groups      []LLMUsageGroup `json:"groups"`
}

func NewMinutesRepository(db *sql.DB) *MinutesRepository {
	return &MinutesRepository{
		BaseRepository: core.NewBaseRepository(db),
	}
}

func (r *MinutesRepository) Create(ctx context.Context, m *Minutes) error {
	content, err := m.MarshalContent()
	if err != nil {
		return fmt.Errorf("failed to marshal minutes content: %w", err)
	}

	var deliveredAt interface{}
	if m.DeliveredAt != nil {
		deliveredAt = m.DeliveredAt.Format(time.RFC3339)
	}

	_, err = r.ExecContext(ctx, `
		INSERT INTO minutes (id, session_id, template_id, version, content, generated_at, delivered_at, status, provider)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.SessionID, m.TemplateID, m.Version, content,
		m.GeneratedAt.Format(time.RFC3339), deliveredAt, string(m.Status), m.Provider,
	)
	if err != nil {
		return fmt.Errorf("failed to create minutes: %w", err)
	}
	return nil
}

func (r *MinutesRepository) Update(ctx context.Context, m *Minutes) error {
	content, err := m.MarshalContent()
	if err != nil {
		return fmt.Errorf("failed to marshal minutes content: %w", err)
	}

	var deliveredAt interface{}
	if m.DeliveredAt != nil {
		deliveredAt = m.DeliveredAt.Format(time.RFC3339)
	}

	_, err = r.ExecContext(ctx, `
		UPDATE minutes SET template_id = ?, version = ?, content = ?, delivered_at = ?, status = ?, provider = ? WHERE id = ?`,
		m.TemplateID, m.Version, content, deliveredAt, string(m.Status), m.Provider, m.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update minutes: %w", err)
	}
	return nil
}

func (r *MinutesRepository) GetByID(ctx context.Context, id string) (*Minutes, error) {
	return r.scanOne(r.QueryRowContext(ctx, `
		SELECT id, session_id, template_id, version, content, generated_at, delivered_at, status, provider
		FROM minutes WHERE id = ?`, id), "id="+id)
}

func (r *MinutesRepository) GetBySession(ctx context.Context, sessionID string) (*Minutes, error) {
	return r.scanOne(r.QueryRowContext(ctx, `
		SELECT id, session_id, template_id, version, content, generated_at, delivered_at, status, provider
		FROM minutes WHERE session_id = ?`, sessionID), "session_id="+sessionID)
}

func (r *MinutesRepository) HasWebhookEvent(ctx context.Context, minutesID string) (bool, error) {
	var count int
	err := r.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM webhook_events
		WHERE minutes_id = ?`,
		minutesID,
	).Scan(&count)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table: webhook_events") {
			return false, nil
		}
		return false, fmt.Errorf("inspect webhook events: %w", err)
	}
	return count > 0, nil
}

func (r *MinutesRepository) ListWebhookEvents(ctx context.Context, sessionID, minutesID string) ([]WebhookEvent, error) {
	query := `
		SELECT e.id, e.minutes_id, e.webhook_url, COALESCE(e.payload_type, 'minutes'),
		       e.attempt_number, e.status, e.delivered_at, e.error_message, e.next_retry_at, e.created_at
		FROM webhook_events e
		JOIN minutes m ON m.id = e.minutes_id`
	args := []interface{}{}
	var filters []string
	if sessionID != "" {
		filters = append(filters, "m.session_id = ?")
		args = append(args, sessionID)
	}
	if minutesID != "" {
		filters = append(filters, "e.minutes_id = ?")
		args = append(args, minutesID)
	}
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}
	query += " ORDER BY e.created_at DESC"

	rows, err := r.QueryContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table: webhook_events") {
			return []WebhookEvent{}, nil
		}
		return nil, fmt.Errorf("list webhook events: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var events []WebhookEvent
	for rows.Next() {
		ev, err := scanWebhookEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook events: %w", err)
	}
	return events, nil
}

func (r *MinutesRepository) ReplayWebhookEvent(ctx context.Context, eventID string) error {
	res, err := r.ExecContext(ctx, `
		UPDATE webhook_events
		SET status='pending', attempt_number=0, next_retry_at=?, delivered_at=NULL, error_message=NULL
		WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), eventID,
	)
	if err != nil {
		return fmt.Errorf("replay webhook event: %w", err)
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("webhook event not found: %s", eventID) //nolint:err113
	}
	return nil
}

func (r *MinutesRepository) InsertLLMUsageEvents(ctx context.Context, events []llm.UsageEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin llm usage insert: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // commit path returns before deferred rollback has effect

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO llm_usage_events (
			id, created_at, request_id, workflow_id, session_id, minutes_id, phase, batch_index, batch_total, attempt,
			provider_profile, provider, model, resolved_provider, resolved_model, generation_id, status, http_status,
			prompt_tokens, completion_tokens, reasoning_tokens, cached_tokens, total_tokens, cost_credits,
			requested_max_tokens, affordable_retry_max_tokens, duration_ms, error_class, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare llm usage insert: %w", err)
	}
	defer stmt.Close() //nolint:errcheck

	for _, ev := range events {
		if ev.ID == "" {
			continue
		}
		ts := ev.Timestamp
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx,
			ev.ID, ts.Format(time.RFC3339Nano), ev.RequestID, ev.WorkflowID, ev.SessionID, ev.MinutesID, ev.Phase,
			ev.BatchIndex, ev.BatchTotal, ev.Attempt, ev.ProviderProfile, ev.Provider, ev.Model, ev.ResolvedProvider,
			ev.ResolvedModel, ev.GenerationID, ev.Status, ev.HTTPStatus, ev.PromptTokens, ev.CompletionTokens,
			ev.ReasoningTokens, ev.CachedTokens, ev.TotalTokens, ev.CostCredits, ev.RequestedMaxTokens,
			ev.AffordableRetryMaxTokens, ev.DurationMs, ev.ErrorClass, ev.ErrorMessage,
		); err != nil {
			return fmt.Errorf("insert llm usage event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit llm usage insert: %w", err)
	}
	return nil
}

func (r *MinutesRepository) SumLLMUsageCostSince(ctx context.Context, since time.Time) (float64, error) {
	var cost sql.NullFloat64
	err := r.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(cost_credits), 0)
		FROM llm_usage_events
		WHERE created_at >= ?`,
		since.UTC().Format(time.RFC3339Nano),
	).Scan(&cost)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table: llm_usage_events") {
			return 0, nil
		}
		return 0, fmt.Errorf("sum llm usage cost: %w", err)
	}
	return cost.Float64, nil
}

func (r *MinutesRepository) LLMUsageSummaryForSession(ctx context.Context, sessionID string) (LLMUsageSummary, error) {
	filter := LLMUsageFilter{SessionID: sessionID}
	total, err := r.llmUsageTotal(ctx, filter)
	if err != nil {
		return LLMUsageSummary{}, err
	}
	return total, nil
}

func (r *MinutesRepository) ReportLLMUsage(ctx context.Context, filter LLMUsageFilter, groupBy string) (LLMUsageReport, error) {
	if groupBy == "" {
		groupBy = "session"
	}
	total, err := r.llmUsageTotal(ctx, filter)
	if err != nil {
		return LLMUsageReport{}, err
	}
	groups, err := r.llmUsageGroups(ctx, filter, groupBy)
	if err != nil {
		return LLMUsageReport{}, err
	}
	return LLMUsageReport{
		GeneratedAt: time.Now().UTC(),
		GroupBy:     groupBy,
		Total:       total,
		Groups:      groups,
	}, nil
}

func (r *MinutesRepository) llmUsageTotal(ctx context.Context, filter LLMUsageFilter) (LLMUsageSummary, error) {
	where, args := llmUsageWhere(filter)
	query := `
		SELECT COALESCE(SUM(CASE WHEN status NOT IN ('budget_exceeded', 'client_error') THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		       COALESCE(SUM(reasoning_tokens),0), COALESCE(SUM(cached_tokens),0), COALESCE(SUM(total_tokens),0),
		       COALESCE(SUM(cost_credits),0)
		FROM llm_usage_events` + where
	var summary LLMUsageSummary
	err := r.QueryRowContext(ctx, query, args...).Scan(
		&summary.Calls,
		&summary.PromptTokens,
		&summary.CompletionTokens,
		&summary.ReasoningTokens,
		&summary.CachedTokens,
		&summary.TotalTokens,
		&summary.CostCredits,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table: llm_usage_events") {
			return LLMUsageSummary{}, nil
		}
		return LLMUsageSummary{}, fmt.Errorf("summarize llm usage: %w", err)
	}
	return summary, nil
}

func (r *MinutesRepository) llmUsageGroups(ctx context.Context, filter LLMUsageFilter, groupBy string) ([]LLMUsageGroup, error) {
	groupExpr, err := llmUsageGroupExpr(groupBy)
	if err != nil {
		return nil, err
	}
	where, args := llmUsageWhere(filter)
	query := `
		SELECT ` + groupExpr + ` AS group_key,
		       COALESCE(SUM(CASE WHEN status NOT IN ('budget_exceeded', 'client_error') THEN 1 ELSE 0 END),0) AS calls,
		       COALESCE(SUM(prompt_tokens),0) AS prompt_tokens,
		       COALESCE(SUM(completion_tokens),0) AS completion_tokens, COALESCE(SUM(reasoning_tokens),0) AS reasoning_tokens,
		       COALESCE(SUM(cached_tokens),0) AS cached_tokens, COALESCE(SUM(total_tokens),0) AS total_tokens,
		       COALESCE(SUM(cost_credits),0) AS cost_credits
		FROM llm_usage_events` + where + `
		GROUP BY group_key
		ORDER BY cost_credits DESC, calls DESC`
	rows, err := r.QueryContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table: llm_usage_events") {
			return []LLMUsageGroup{}, nil
		}
		return nil, fmt.Errorf("group llm usage: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	groups := []LLMUsageGroup{}
	for rows.Next() {
		var group LLMUsageGroup
		if err := rows.Scan(
			&group.Key,
			&group.Calls,
			&group.PromptTokens,
			&group.CompletionTokens,
			&group.ReasoningTokens,
			&group.CachedTokens,
			&group.TotalTokens,
			&group.CostCredits,
		); err != nil {
			return nil, fmt.Errorf("scan llm usage group: %w", err)
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate llm usage groups: %w", err)
	}
	return groups, nil
}

type webhookEventScanner interface {
	Scan(dest ...interface{}) error
}

func scanWebhookEvent(row webhookEventScanner) (WebhookEvent, error) {
	var ev WebhookEvent
	var deliveredAt, errorMessage, nextRetryAt, createdAt sql.NullString
	if err := row.Scan(
		&ev.ID,
		&ev.MinutesID,
		&ev.WebhookURL,
		&ev.PayloadType,
		&ev.AttemptNumber,
		&ev.Status,
		&deliveredAt,
		&errorMessage,
		&nextRetryAt,
		&createdAt,
	); err != nil {
		return ev, fmt.Errorf("scan webhook event: %w", err)
	}
	if deliveredAt.Valid {
		if parsed, err := time.Parse(time.RFC3339, deliveredAt.String); err == nil {
			ev.DeliveredAt = &parsed
		}
	}
	if nextRetryAt.Valid {
		if parsed, err := time.Parse(time.RFC3339, nextRetryAt.String); err == nil {
			ev.NextRetryAt = &parsed
		}
	}
	if createdAt.Valid {
		if parsed, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			ev.CreatedAt = parsed
		}
	}
	if errorMessage.Valid {
		ev.ErrorMessage = errorMessage.String
	}
	return ev, nil
}

func (r *MinutesRepository) scanOne(row *sql.Row, hint string) (*Minutes, error) {
	var m Minutes
	var content string
	var generatedAt string
	var deliveredAt sql.NullString

	err := row.Scan(
		&m.ID, &m.SessionID, &m.TemplateID, &m.Version, &content,
		&generatedAt, &deliveredAt, &m.Status, &m.Provider,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", errMinutesNotFound, hint)
		}
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	if t, err := time.Parse(time.RFC3339, generatedAt); err == nil {
		m.GeneratedAt = t
	}
	if deliveredAt.Valid {
		if t, err := time.Parse(time.RFC3339, deliveredAt.String); err == nil {
			m.DeliveredAt = &t
		}
	}

	if err := m.UnmarshalContent(content); err != nil {
		return nil, fmt.Errorf("failed to parse minutes content: %w", err)
	}

	return &m, nil
}

func (r *MinutesRepository) CreateHistory(ctx context.Context, history *MinutesHistory) error {
	_, err := r.ExecContext(ctx, `
		INSERT INTO minutes_history (id, minutes_id, version, content, edited_at, edited_by)
		VALUES (?, ?, ?, ?, ?, ?)`,
		history.ID, history.MinutesID, history.Version, history.Content,
		history.EditedAt.Format(time.RFC3339), history.EditedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to create minutes history: %w", err)
	}
	return nil
}

func (r *MinutesRepository) GetHistory(ctx context.Context, minutesID string) ([]*MinutesHistory, error) {
	rows, err := r.QueryContext(ctx, `
		SELECT id, minutes_id, version, content, edited_at, edited_by
		FROM minutes_history WHERE minutes_id = ? ORDER BY version DESC`, minutesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes history: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close error is not actionable here

	var history []*MinutesHistory
	for rows.Next() {
		var h MinutesHistory
		var editedAt string
		var editedBy sql.NullString

		if err := rows.Scan(&h.ID, &h.MinutesID, &h.Version, &h.Content, &editedAt, &editedBy); err != nil {
			return nil, fmt.Errorf("failed to scan minutes history: %w", err)
		}
		if t, parseErr := time.Parse(time.RFC3339, editedAt); parseErr == nil {
			h.EditedAt = t
		}
		if editedBy.Valid {
			h.EditedBy = editedBy.String
		}
		history = append(history, &h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate minutes history: %w", err)
	}

	return history, nil
}

// snapshotJSON serializes a Minutes to JSON for history storage.
func snapshotJSON(m *Minutes) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Delete removes minutes and its history by minutes ID.
func (r *MinutesRepository) Delete(ctx context.Context, id string) error {
	for _, q := range []string{
		"DELETE FROM minutes_history WHERE minutes_id = ?",
		"DELETE FROM minutes WHERE id = ?",
	} {
		if _, err := r.ExecContext(ctx, q, id); err != nil {
			return fmt.Errorf("delete minutes: %w", err)
		}
	}
	return nil
}

func llmUsageWhere(filter LLMUsageFilter) (string, []interface{}) {
	clauses := []string{}
	args := []interface{}{}
	if filter.From != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, filter.From.UTC().Format(time.RFC3339Nano))
	}
	if filter.To != nil {
		clauses = append(clauses, "created_at < ?")
		args = append(args, filter.To.UTC().Format(time.RFC3339Nano))
	}
	if filter.SessionID != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, filter.SessionID)
	}
	if filter.MinutesID != "" {
		clauses = append(clauses, "minutes_id = ?")
		args = append(args, filter.MinutesID)
	}
	if filter.Model != "" {
		clauses = append(clauses, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.Profile != "" {
		clauses = append(clauses, "provider_profile = ?")
		args = append(args, filter.Profile)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func llmUsageGroupExpr(groupBy string) (string, error) {
	switch groupBy {
	case "session":
		return "COALESCE(NULLIF(session_id, ''), '<none>')", nil
	case "day":
		return "substr(created_at, 1, 10)", nil
	case "model":
		return "COALESCE(NULLIF(model, ''), '<none>')", nil
	case "profile":
		return "COALESCE(NULLIF(provider_profile, ''), '<none>')", nil
	default:
		return "", fmt.Errorf("unsupported llm usage group: %s", groupBy) //nolint:err113
	}
}
