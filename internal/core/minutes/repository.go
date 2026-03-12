package minutes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flowup/aftertalk/internal/core"
)

type MinutesRepository struct {
	*core.BaseRepository
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
		UPDATE minutes SET version = ?, content = ?, delivered_at = ?, status = ? WHERE id = ?`,
		m.Version, content, deliveredAt, string(m.Status), m.ID,
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
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("minutes not found: %s", hint)
		}
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	m.GeneratedAt, _ = time.Parse(time.RFC3339, generatedAt)
	if deliveredAt.Valid {
		t, _ := time.Parse(time.RFC3339, deliveredAt.String)
		m.DeliveredAt = &t
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
	defer rows.Close()

	var history []*MinutesHistory
	for rows.Next() {
		var h MinutesHistory
		var editedAt string
		var editedBy sql.NullString

		if err := rows.Scan(&h.ID, &h.MinutesID, &h.Version, &h.Content, &editedAt, &editedBy); err != nil {
			return nil, fmt.Errorf("failed to scan minutes history: %w", err)
		}
		h.EditedAt, _ = time.Parse(time.RFC3339, editedAt)
		if editedBy.Valid {
			h.EditedBy = editedBy.String
		}
		history = append(history, &h)
	}
	return history, nil
}

// snapshotJSON serializes a Minutes to JSON for history storage.
func snapshotJSON(m *Minutes) string {
	b, _ := json.Marshal(m)
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
