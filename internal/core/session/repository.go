package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/flowup/aftertalk/internal/core"
)

type SessionRepository struct {
	*core.BaseRepository
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{
		BaseRepository: core.NewBaseRepository(db),
	}
}

func (r *SessionRepository) Create(ctx context.Context, session *Session) error {
	query := `
		INSERT INTO sessions (id, status, created_at, ended_at, participant_count, template_id, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	var endedAt interface{}
	if session.EndedAt != nil {
		endedAt = session.EndedAt.Format(time.RFC3339)
	}

	_, err := r.ExecContext(ctx, query,
		session.ID,
		string(session.Status),
		session.CreatedAt.Format(time.RFC3339),
		endedAt,
		session.ParticipantCount,
		session.TemplateID,
		session.Metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id string) (*Session, error) {
	query := `
		SELECT id, status, created_at, ended_at, participant_count, template_id, metadata
		FROM sessions
		WHERE id = ?
	`

	var session Session
	var status, createdAt, endedAt sql.NullString
	var templateID, metadata sql.NullString

	err := r.QueryRowContext(ctx, query, id).Scan(
		&session.ID,
		&status,
		&createdAt,
		&endedAt,
		&session.ParticipantCount,
		&templateID,
		&metadata,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	session.Status = SessionStatus(status.String)
	if createdAt.Valid {
		session.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if endedAt.Valid {
		t, _ := time.Parse(time.RFC3339, endedAt.String)
		session.EndedAt = &t
	}
	if templateID.Valid {
		session.TemplateID = templateID.String
	}
	if metadata.Valid {
		session.Metadata = metadata.String
	}

	return &session, nil
}

func (r *SessionRepository) Update(ctx context.Context, session *Session) error {
	query := `
		UPDATE sessions
		SET status = ?, ended_at = ?, participant_count = ?, metadata = ?
		WHERE id = ?
	`

	var endedAt interface{}
	if session.EndedAt != nil {
		endedAt = session.EndedAt.Format(time.RFC3339)
	}

	_, err := r.ExecContext(ctx, query,
		string(session.Status),
		endedAt,
		session.ParticipantCount,
		session.Metadata,
		session.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

func (r *SessionRepository) CreateParticipant(ctx context.Context, participant *Participant) error {
	query := `
		INSERT INTO participants (id, session_id, user_id, role, token_jti, token_expires_at, token_used, connected_at, disconnected_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var connectedAt, disconnectedAt interface{}
	if participant.ConnectedAt != nil {
		connectedAt = participant.ConnectedAt.Format(time.RFC3339)
	}
	if participant.DisconnectedAt != nil {
		disconnectedAt = participant.DisconnectedAt.Format(time.RFC3339)
	}

	tokenUsed := 0
	if participant.TokenUsed {
		tokenUsed = 1
	}

	_, err := r.ExecContext(ctx, query,
		participant.ID,
		participant.SessionID,
		participant.UserID,
		participant.Role,
		participant.TokenJTI,
		participant.TokenExpiresAt.Format(time.RFC3339),
		tokenUsed,
		connectedAt,
		disconnectedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create participant: %w", err)
	}

	return nil
}

func (r *SessionRepository) GetParticipantByJTI(ctx context.Context, jti string) (*Participant, error) {
	query := `
		SELECT id, session_id, user_id, role, token_jti, token_expires_at, token_used, connected_at, disconnected_at
		FROM participants
		WHERE token_jti = ?
	`

	var participant Participant
	var tokenExpiresAt, connectedAt, disconnectedAt sql.NullString
	var tokenUsed int

	err := r.QueryRowContext(ctx, query, jti).Scan(
		&participant.ID,
		&participant.SessionID,
		&participant.UserID,
		&participant.Role,
		&participant.TokenJTI,
		&tokenExpiresAt,
		&tokenUsed,
		&connectedAt,
		&disconnectedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("participant not found with jti: %s", jti)
		}
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}

	if tokenExpiresAt.Valid {
		participant.TokenExpiresAt, _ = time.Parse(time.RFC3339, tokenExpiresAt.String)
	}
	participant.TokenUsed = tokenUsed == 1
	if connectedAt.Valid {
		t, _ := time.Parse(time.RFC3339, connectedAt.String)
		participant.ConnectedAt = &t
	}
	if disconnectedAt.Valid {
		t, _ := time.Parse(time.RFC3339, disconnectedAt.String)
		participant.DisconnectedAt = &t
	}

	return &participant, nil
}

func (r *SessionRepository) GetParticipantsBySession(ctx context.Context, sessionID string) ([]*Participant, error) {
	query := `
		SELECT id, session_id, user_id, role, token_jti, token_expires_at, token_used, connected_at, disconnected_at
		FROM participants
		WHERE session_id = ?
	`

	rows, err := r.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}
	defer rows.Close()

	var participants []*Participant
	for rows.Next() {
		var participant Participant
		var tokenExpiresAt, connectedAt, disconnectedAt sql.NullString
		var tokenUsed int

		err := rows.Scan(
			&participant.ID,
			&participant.SessionID,
			&participant.UserID,
			&participant.Role,
			&participant.TokenJTI,
			&tokenExpiresAt,
			&tokenUsed,
			&connectedAt,
			&disconnectedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}

		if tokenExpiresAt.Valid {
			participant.TokenExpiresAt, _ = time.Parse(time.RFC3339, tokenExpiresAt.String)
		}
		participant.TokenUsed = tokenUsed == 1
		if connectedAt.Valid {
			t, _ := time.Parse(time.RFC3339, connectedAt.String)
			participant.ConnectedAt = &t
		}
		if disconnectedAt.Valid {
			t, _ := time.Parse(time.RFC3339, disconnectedAt.String)
			participant.DisconnectedAt = &t
		}

		participants = append(participants, &participant)
	}

	return participants, nil
}

func (r *SessionRepository) UpdateParticipant(ctx context.Context, participant *Participant) error {
	query := `
		UPDATE participants
		SET token_used = ?, connected_at = ?, disconnected_at = ?
		WHERE id = ?
	`

	var connectedAt, disconnectedAt interface{}
	if participant.ConnectedAt != nil {
		connectedAt = participant.ConnectedAt.Format(time.RFC3339)
	}
	if participant.DisconnectedAt != nil {
		disconnectedAt = participant.DisconnectedAt.Format(time.RFC3339)
	}

	tokenUsed := 0
	if participant.TokenUsed {
		tokenUsed = 1
	}

	_, err := r.ExecContext(ctx, query,
		tokenUsed,
		connectedAt,
		disconnectedAt,
		participant.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update participant: %w", err)
	}

	return nil
}

func (r *SessionRepository) CreateAudioStream(ctx context.Context, stream *AudioStream) error {
	query := `
		INSERT INTO audio_streams (id, participant_id, codec, sample_rate, channels, chunk_size_seconds, started_at, ended_at, chunks_received, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var endedAt interface{}
	if stream.EndedAt != nil {
		endedAt = stream.EndedAt.Format(time.RFC3339)
	}

	_, err := r.ExecContext(ctx, query,
		stream.ID,
		stream.ParticipantID,
		stream.Codec,
		stream.SampleRate,
		stream.Channels,
		stream.ChunkSizeSeconds,
		stream.StartedAt.Format(time.RFC3339),
		endedAt,
		stream.ChunksReceived,
		string(stream.Status),
	)

	if err != nil {
		return fmt.Errorf("failed to create audio stream: %w", err)
	}

	return nil
}

func (r *SessionRepository) GetAudioStreamByParticipant(ctx context.Context, participantID string) (*AudioStream, error) {
	query := `
		SELECT id, participant_id, codec, sample_rate, channels, chunk_size_seconds, started_at, ended_at, chunks_received, status
		FROM audio_streams
		WHERE participant_id = ?
	`

	var stream AudioStream
	var startedAt, endedAt sql.NullString

	err := r.QueryRowContext(ctx, query, participantID).Scan(
		&stream.ID,
		&stream.ParticipantID,
		&stream.Codec,
		&stream.SampleRate,
		&stream.Channels,
		&stream.ChunkSizeSeconds,
		&startedAt,
		&endedAt,
		&stream.ChunksReceived,
		&stream.Status,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("audio stream not found for participant: %s", participantID)
		}
		return nil, fmt.Errorf("failed to get audio stream: %w", err)
	}

	if startedAt.Valid {
		stream.StartedAt, _ = time.Parse(time.RFC3339, startedAt.String)
	}
	if endedAt.Valid {
		t, _ := time.Parse(time.RFC3339, endedAt.String)
		stream.EndedAt = &t
	}

	return &stream, nil
}

func (r *SessionRepository) UpdateAudioStream(ctx context.Context, stream *AudioStream) error {
	query := `
		UPDATE audio_streams
		SET chunks_received = ?, status = ?, ended_at = ?
		WHERE id = ?
	`

	var endedAt interface{}
	if stream.EndedAt != nil {
		endedAt = stream.EndedAt.Format(time.RFC3339)
	}

	_, err := r.ExecContext(ctx, query,
		stream.ChunksReceived,
		string(stream.Status),
		endedAt,
		stream.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update audio stream: %w", err)
	}

	return nil
}

// List returns sessions filtered by optional status, ordered by created_at desc.
// limit=0 means no limit (returns all). offset is 0-based.
func (r *SessionRepository) List(ctx context.Context, status string, limit, offset int) ([]*Session, int, error) {
	args := []interface{}{}
	where := ""
	if status != "" {
		where = " WHERE status = ?"
		args = append(args, status)
	}

	// Total count
	var total int
	if err := r.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}

	query := "SELECT id, status, created_at, ended_at, participant_count, template_id, metadata FROM sessions" +
		where + " ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	rows, err := r.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var s Session
		var statusStr, createdAt, endedAt sql.NullString
		var templateID, metadata sql.NullString
		if err := rows.Scan(&s.ID, &statusStr, &createdAt, &endedAt, &s.ParticipantCount, &templateID, &metadata); err != nil {
			return nil, 0, fmt.Errorf("scan session: %w", err)
		}
		s.Status = SessionStatus(statusStr.String)
		if createdAt.Valid {
			s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		}
		if endedAt.Valid {
			t, _ := time.Parse(time.RFC3339, endedAt.String)
			s.EndedAt = &t
		}
		if templateID.Valid {
			s.TemplateID = templateID.String
		}
		if metadata.Valid {
			s.Metadata = metadata.String
		}
		sessions = append(sessions, &s)
	}
	return sessions, total, rows.Err()
}

// Delete removes a session and its related data (participants, audio_streams).
// Transcriptions and minutes are kept for audit purposes unless explicitly deleted.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	queries := []string{
		"DELETE FROM participants WHERE session_id = ?",
		"DELETE FROM audio_streams WHERE session_id = ?",
		"DELETE FROM sessions WHERE id = ?",
	}
	for _, q := range queries {
		if _, err := r.ExecContext(ctx, q, id); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
	}
	return nil
}
