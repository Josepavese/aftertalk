package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flowup/aftertalk/internal/logging"
	"github.com/google/uuid"
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

// Enqueue persists a pending webhook event. The caller should call this instead
// of Client.Send directly; the background worker delivers it with retries.
func (r *Retrier) Enqueue(ctx context.Context, minutesID, webhookURL string, payload *MinutesPayload) error {
	if webhookURL == "" {
		return nil
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook retrier: marshal payload: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO webhook_events
			(id, minutes_id, webhook_url, payload, attempt_number, status, next_retry_at, created_at)
		VALUES (?, ?, ?, ?, 0, 'pending', ?, ?)`,
		id, minutesID, webhookURL, string(payloadBytes), now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	return err
}

// Run starts the background delivery loop. It blocks until ctx is cancelled.
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
		SELECT id, minutes_id, webhook_url, payload, attempt_number
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
	defer rows.Close()

	type event struct {
		id, minutesID, webhookURL, payload string
		attempt                            int
	}
	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.id, &e.minutesID, &e.webhookURL, &e.payload, &e.attempt); err != nil {
			logging.Errorf("webhook retrier: scan row: %v", err)
			continue
		}
		events = append(events, e)
	}
	rows.Close()

	for _, e := range events {
		r.deliver(ctx, e.id, e.minutesID, e.webhookURL, e.payload, e.attempt)
	}
}

func (r *Retrier) deliver(ctx context.Context, id, minutesID, webhookURL, payloadJSON string, attempt int) {
	attempt++
	var payload MinutesPayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		logging.Errorf("webhook retrier: unmarshal payload for event %s: %v", id, err)
		r.markFailed(ctx, id, attempt, "unmarshal error: "+err.Error())
		return
	}

	deliverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sendErr := r.client.Send(deliverCtx, &payload)
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
	backoff := time.Duration(1<<uint(attempt)) * 30 * time.Second
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
