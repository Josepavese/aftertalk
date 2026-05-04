package webhook

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Josepavese/aftertalk/internal/logging"
)

const (
	maxAttempts    = 5
	workerInterval = 30 * time.Second
)

// Retrier persists webhook deliveries to the webhook_events table and retries
// failed deliveries with exponential backoff. It replaces the fire-and-forget
// goroutine in minutes.Service.deliverWebhook.
type Retrier struct {
	db     *sql.DB
	client *Client
}

func NewRetrier(db *sql.DB, client *Client) *Retrier {
	return &Retrier{db: db, client: client}
}

// payloadTypeMinutes is the push-mode payload discriminator stored in webhook_events.
const payloadTypeMinutes = "minutes"

// payloadTypeNotification is the notify_pull-mode payload discriminator.
const payloadTypeNotification = "notification"

// Enqueue persists a pending push-mode webhook event (full minutes payload).
// The background worker will call Client.Send() to deliver it with retries.
func (r *Retrier) Enqueue(ctx context.Context, minutesID, webhookURL string, payload *MinutesPayload) error {
	return r.enqueue(ctx, minutesID, webhookURL, payloadTypeMinutes, payload)
}

// EnqueueNotification persists a pending notify_pull-mode webhook event.
// The background worker will call Client.SendNotification() to deliver it.
func (r *Retrier) EnqueueNotification(ctx context.Context, minutesID, webhookURL string, payload *NotificationPayload) error {
	return r.enqueue(ctx, minutesID, webhookURL, payloadTypeNotification, payload)
}

func (r *Retrier) enqueue(ctx context.Context, minutesID, webhookURL, payloadType string, payload interface{}) error {
	if webhookURL == "" {
		return nil
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook retrier: marshal payload: %w", err)
	}

	id := uuid.New().String()
	payloadHash := computePayloadHash(minutesID, webhookURL, payloadType, payloadBytes)
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO webhook_events
			(id, minutes_id, webhook_url, payload_hash, payload, payload_type, attempt_number, status, next_retry_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, 'pending', ?, ?)
		ON CONFLICT(payload_hash) DO NOTHING`,
		id, minutesID, webhookURL, payloadHash, string(payloadBytes), payloadType,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil && isLegacyWebhookEventsSchemaError(err) {
		// Backward compatibility for test/legacy schemas without payload_hash.
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO webhook_events
				(id, minutes_id, webhook_url, payload, payload_type, attempt_number, status, next_retry_at, created_at)
			VALUES (?, ?, ?, ?, ?, 0, 'pending', ?, ?)`,
			id, minutesID, webhookURL, string(payloadBytes), payloadType,
			now.Format(time.RFC3339), now.Format(time.RFC3339),
		)
	}
	if err != nil {
		return err
	}
	if res != nil {
		if rows, rowsErr := res.RowsAffected(); rowsErr == nil && rows == 0 {
			logging.Infof("webhook retrier: duplicate enqueue skipped for minutes=%s hash=%s", minutesID, payloadHash)
		}
	}
	return nil
}

func computePayloadHash(minutesID, webhookURL, payloadType string, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(minutesID))
	h.Write([]byte{0})
	h.Write([]byte(webhookURL))
	h.Write([]byte{0})
	h.Write([]byte(payloadType))
	h.Write([]byte{0})
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func isLegacyWebhookEventsSchemaError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no column named payload_hash") ||
		strings.Contains(msg, "no such column: payload_hash") ||
		strings.Contains(msg, "on conflict clause does not match any primary key or unique constraint")
}

// Run starts the background delivery loop. It blocks until ctx is canceled.
func (r *Retrier) Run(ctx context.Context) {
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()
	logging.Infof("webhook retrier: started (interval=%s, maxAttempts=%d)", workerInterval, maxAttempts)
	for {
		select {
		case <-ctx.Done():
			logging.Infof("webhook retrier: stopped")
			return
		case <-ticker.C:
			r.processPending(ctx)
		}
	}
}

func (r *Retrier) processPending(ctx context.Context) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, minutes_id, webhook_url, payload, payload_type, attempt_number
		FROM webhook_events
		WHERE status = 'pending' AND next_retry_at <= ?
		ORDER BY created_at ASC
		LIMIT 20`,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		logging.Errorf("webhook retrier: query pending: %v", err)
		return
	}
	defer rows.Close() //nolint:errcheck // rows.Close error is not actionable here

	type event struct {
		id, minutesID, webhookURL, payload, payloadType string
		attempt                                         int
	}
	var events []event
	for rows.Next() {
		var e event
		var pt sql.NullString
		if err := rows.Scan(&e.id, &e.minutesID, &e.webhookURL, &e.payload, &pt, &e.attempt); err != nil {
			logging.Errorf("webhook retrier: scan row: %v", err)
			continue
		}
		if pt.Valid {
			e.payloadType = pt.String
		} else {
			e.payloadType = payloadTypeMinutes // legacy rows without payload_type
		}
		events = append(events, e)
	}
	rows.Close() //nolint:errcheck // close after full read; error irrelevant
	if err := rows.Err(); err != nil {
		logging.Errorf("webhook retrier: rows error: %v", err)
		return
	}

	for _, e := range events {
		r.deliver(ctx, e.id, e.minutesID, e.webhookURL, e.payload, e.payloadType, e.attempt)
	}
}

func (r *Retrier) deliver(ctx context.Context, id, minutesID, webhookURL, payloadJSON, payloadType string, attempt int) {
	attempt++

	deliverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var sendErr error
	switch payloadType {
	case payloadTypeNotification:
		var payload NotificationPayload
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			logging.Errorf("webhook retrier: unmarshal notification payload for event %s: %v", id, err)
			r.markFailed(ctx, id, attempt, "unmarshal error: "+err.Error())
			return
		}
		sendErr = r.client.SendNotification(deliverCtx, &payload)
	default: // payloadTypeMinutes and legacy rows
		var payload MinutesPayload
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			logging.Errorf("webhook retrier: unmarshal payload for event %s: %v", id, err)
			r.markFailed(ctx, id, attempt, "unmarshal error: "+err.Error())
			return
		}
		sendErr = r.client.Send(deliverCtx, &payload)
	}
	now := time.Now().UTC()

	if sendErr == nil {
		_, err := r.db.ExecContext(ctx, `
			UPDATE webhook_events
			SET status='delivered', attempt_number=?, delivered_at=?, error_message=NULL
			WHERE id=?`,
			attempt, now.Format(time.RFC3339), id,
		)
		if err != nil {
			logging.Errorf("webhook retrier: mark delivered event %s: %v", id, err)
		} else {
			logging.Infof("webhook retrier: delivered event %s (attempt %d) minutes=%s", id, attempt, minutesID)
		}
		return
	}

	logging.Warnf("webhook retrier: event %s attempt %d failed: %v", id, attempt, sendErr)

	if attempt >= maxAttempts {
		r.markFailed(ctx, id, attempt, sendErr.Error())
		return
	}

	// Exponential backoff: 30s, 2m, 8m, 30m.
	backoff := time.Duration(1<<uint(attempt)) * 30 * time.Second //nolint:gosec // attempt is bounded (max retries)
	nextRetry := now.Add(backoff)
	_, err := r.db.ExecContext(ctx, `
		UPDATE webhook_events
		SET attempt_number=?, next_retry_at=?, error_message=?
		WHERE id=?`,
		attempt, nextRetry.Format(time.RFC3339), sendErr.Error(), id,
	)
	if err != nil {
		logging.Errorf("webhook retrier: update retry for event %s: %v", id, err)
	}
}

func (r *Retrier) markFailed(ctx context.Context, id string, attempt int, errMsg string) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE webhook_events SET status='failed', attempt_number=?, error_message=? WHERE id=?`,
		attempt, errMsg, id,
	)
	if err != nil {
		logging.Errorf("webhook retrier: mark failed event %s: %v", id, err)
	} else {
		logging.Errorf("webhook retrier: event %s permanently failed after %d attempts: %s", id, attempt, errMsg)
	}
}
