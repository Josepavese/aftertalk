package transcription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Josepavese/aftertalk/internal/core"
)

var errTranscriptionNotFound = errors.New("transcription not found")

type TranscriptionRepository struct {
	*core.BaseRepository
}

func NewTranscriptionRepository(db *sql.DB) *TranscriptionRepository {
	return &TranscriptionRepository{
		BaseRepository: core.NewBaseRepository(db),
	}
}

func (r *TranscriptionRepository) Create(ctx context.Context, transcription *Transcription) error {
	query := `
		INSERT INTO transcriptions (id, session_id, segment_index, role, start_ms, end_ms, text, confidence, provider, language, created_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.ExecContext(ctx, query,
		transcription.ID,
		transcription.SessionID,
		transcription.SegmentIndex,
		transcription.Role,
		transcription.StartMs,
		transcription.EndMs,
		transcription.Text,
		transcription.Confidence,
		transcription.Provider,
		transcription.Language,
		transcription.CreatedAt.Format(time.RFC3339),
		string(transcription.Status),
	)
	if err != nil {
		return fmt.Errorf("failed to create transcription: %w", err)
	}

	return nil
}

func (r *TranscriptionRepository) GetByID(ctx context.Context, id string) (*Transcription, error) {
	query := `
		SELECT id, session_id, segment_index, role, start_ms, end_ms, text, confidence, provider, created_at, status
		FROM transcriptions
		WHERE id = ?
	`

	var t Transcription
	var createdAt string
	var confidence sql.NullFloat64

	err := r.QueryRowContext(ctx, query, id).Scan(
		&t.ID,
		&t.SessionID,
		&t.SegmentIndex,
		&t.Role,
		&t.StartMs,
		&t.EndMs,
		&t.Text,
		&confidence,
		&t.Provider,
		&createdAt,
		&t.Status,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", errTranscriptionNotFound, id)
		}
		return nil, fmt.Errorf("failed to get transcription: %w", err)
	}

	if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
		t.CreatedAt = parsed
	}
	if confidence.Valid {
		t.Confidence = confidence.Float64
	}

	return &t, nil
}

func (r *TranscriptionRepository) GetBySession(ctx context.Context, sessionID string) ([]*Transcription, error) {
	query := `
		SELECT id, session_id, segment_index, role, start_ms, end_ms, text, confidence, provider, created_at, status
		FROM transcriptions
		WHERE session_id = ?
		ORDER BY start_ms ASC
	`

	rows, err := r.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transcriptions: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close error is not actionable here

	var transcriptions []*Transcription
	for rows.Next() {
		var t Transcription
		var createdAt string
		var confidence sql.NullFloat64

		err := rows.Scan(
			&t.ID,
			&t.SessionID,
			&t.SegmentIndex,
			&t.Role,
			&t.StartMs,
			&t.EndMs,
			&t.Text,
			&confidence,
			&t.Provider,
			&createdAt,
			&t.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			t.CreatedAt = parsed
		}
		if confidence.Valid {
			t.Confidence = confidence.Float64
		}

		transcriptions = append(transcriptions, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate transcriptions: %w", err)
	}

	return transcriptions, nil
}

func (r *TranscriptionRepository) GetBySessionOrdered(ctx context.Context, sessionID string) ([]*Transcription, error) {
	return r.GetBySession(ctx, sessionID)
}

// GetLastActivityTime returns the created_at of the most recent transcription
// for a session. Returns zero time and no error when no transcriptions exist.
// Used at startup to restore inactivity timers for active sessions.
func (r *TranscriptionRepository) GetLastActivityTime(ctx context.Context, sessionID string) (time.Time, error) {
	var raw sql.NullString
	err := r.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM transcriptions WHERE session_id = ?`,
		sessionID,
	).Scan(&raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("get last activity time: %w", err)
	}
	if !raw.Valid {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse last activity time: %w", err)
	}
	return t, nil
}

// GetDetectedLanguage returns the most recent non-empty STT-detected language
// code for a session (e.g. "it", "en"). Returns "" if none is stored yet.
func (r *TranscriptionRepository) GetDetectedLanguage(ctx context.Context, sessionID string) (string, error) {
	var lang sql.NullString
	err := r.QueryRowContext(ctx,
		`SELECT language FROM transcriptions WHERE session_id = ? AND language != '' ORDER BY created_at DESC LIMIT 1`,
		sessionID,
	).Scan(&lang)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get detected language: %w", err)
	}
	if !lang.Valid {
		return "", nil
	}
	return lang.String, nil
}

// CountBySession returns the number of already-saved segments for a session,
// used to compute the segment_index offset when a session reconnects.
func (r *TranscriptionRepository) CountBySession(ctx context.Context, sessionID string) (int, error) {
	var count int
	err := r.QueryRowContext(ctx, `SELECT COUNT(*) FROM transcriptions WHERE session_id = ?`, sessionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transcriptions: %w", err)
	}
	return count, nil
}

func (r *TranscriptionRepository) UpdateStatus(ctx context.Context, id string, status TranscriptionStatus) error {
	query := `UPDATE transcriptions SET status = ? WHERE id = ?`

	_, err := r.ExecContext(ctx, query, string(status), id)
	if err != nil {
		return fmt.Errorf("failed to update transcription status: %w", err)
	}

	return nil
}
