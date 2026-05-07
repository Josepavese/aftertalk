package minutes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
