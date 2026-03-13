package minutes

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RetrievalToken is a single-use, time-limited credential that authorises one
// GET /v1/minutes/pull/{id} request.
//
// Security properties:
//   - single-use: ConsumeToken() marks it atomically; any replay returns an error
//   - time-limited: tokens expire at ExpiresAt; expired tokens are rejected
//   - opaque: the token ID is a UUIDv4 — no information about the session is
//     embedded, so intercepting a notification webhook reveals nothing on its own
type RetrievalToken struct {
	ID        string
	MinutesID string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// CreateRetrievalToken inserts a new retrieval token into the DB.
func (r *MinutesRepository) CreateRetrievalToken(ctx context.Context, t *RetrievalToken) error {
	_, err := r.ExecContext(ctx, `
		INSERT INTO retrieval_tokens (id, minutes_id, expires_at, created_at)
		VALUES (?, ?, ?, ?)`,
		t.ID,
		t.MinutesID,
		t.ExpiresAt.UTC().Format(time.RFC3339),
		t.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create retrieval token: %w", err)
	}
	return nil
}

// ConsumeToken atomically marks the token as used and returns it.
//
// Returns an error if:
//   - the token does not exist
//   - the token has already been used (used_at IS NOT NULL)
//   - the token has expired (expires_at <= now)
//
// All three cases return the same generic error to prevent oracle attacks
// (an attacker cannot distinguish "wrong token" from "already used").
func (r *MinutesRepository) ConsumeToken(ctx context.Context, tokenID string) (*RetrievalToken, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Atomic mark-as-used: only succeeds if token exists, is unused, and not expired.
	result, err := r.ExecContext(ctx, `
		UPDATE retrieval_tokens
		SET used_at = ?
		WHERE id = ? AND used_at IS NULL AND expires_at > ?`,
		now, tokenID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("consume retrieval token: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("consume retrieval token rows affected: %w", err)
	}
	if affected == 0 {
		// Intentionally vague — covers not-found, already-used, and expired.
		return nil, fmt.Errorf("retrieval token not found or already consumed")
	}

	// Fetch the row to return MinutesID to the caller.
	var t RetrievalToken
	var expiresAt, createdAt string
	var usedAt sql.NullString

	err = r.QueryRowContext(ctx, `
		SELECT id, minutes_id, expires_at, used_at, created_at
		FROM retrieval_tokens WHERE id = ?`, tokenID).
		Scan(&t.ID, &t.MinutesID, &expiresAt, &usedAt, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("retrieve token after consume: %w", err)
	}

	t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if usedAt.Valid {
		u, _ := time.Parse(time.RFC3339, usedAt.String)
		t.UsedAt = &u
	}

	return &t, nil
}

// DeleteExpiredTokens removes all tokens that have expired more than graceWindow
// ago. This is a maintenance operation; call it periodically (e.g. daily).
func (r *MinutesRepository) DeleteExpiredTokens(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339)
	_, err := r.ExecContext(ctx, `
		DELETE FROM retrieval_tokens WHERE expires_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("delete expired tokens: %w", err)
	}
	return nil
}
