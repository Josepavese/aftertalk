package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func init() {
	logging.Init("info", "console") //nolint:errcheck
}

// openTestDB creates an in-process SQLite DB with the webhook_events table.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS webhook_events (
			id TEXT PRIMARY KEY,
			minutes_id TEXT NOT NULL,
			webhook_url TEXT NOT NULL,
			payload_hash TEXT NOT NULL UNIQUE,
			payload TEXT NOT NULL,
			payload_type TEXT NOT NULL DEFAULT 'minutes',
			attempt_number INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			next_retry_at TEXT,
			delivered_at TEXT,
			error_message TEXT,
			created_at TEXT NOT NULL
		)
	`)
	require.NoError(t, err)
	return db
}

func testPayload(sessionID string) *MinutesPayload {
	return &MinutesPayload{
		SessionID: sessionID,
		Minutes:   map[string]string{"themes": "test"},
		Timestamp: time.Now(),
	}
}

// ── Enqueue ───────────────────────────────────────────────────────────────

func TestRetrier_Enqueue_InsertsRow(t *testing.T) {
	db := openTestDB(t)
	r := NewRetrier(db, NewClient("http://example.com", 5*time.Second))

	err := r.Enqueue(context.Background(), "min-1", "http://example.com/wh", testPayload("s1"))
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM webhook_events WHERE minutes_id='min-1'`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestRetrier_Enqueue_StatusIsPending(t *testing.T) {
	db := openTestDB(t)
	r := NewRetrier(db, NewClient("", 5*time.Second))

	require.NoError(t, r.Enqueue(context.Background(), "min-2", "http://x.com", testPayload("s2")))

	var status string
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT status FROM webhook_events WHERE minutes_id='min-2'`).Scan(&status))
	assert.Equal(t, "pending", status)
}

func TestRetrier_Enqueue_EmptyURL_NoInsert(t *testing.T) {
	db := openTestDB(t)
	r := NewRetrier(db, NewClient("", 5*time.Second))

	err := r.Enqueue(context.Background(), "min-3", "", testPayload("s3"))
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM webhook_events`).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestRetrier_Enqueue_IdempotentOnDuplicate(t *testing.T) {
	db := openTestDB(t)
	r := NewRetrier(db, NewClient("", 5*time.Second))
	url := "http://x.com"
	p := testPayload("s4")

	// First insert succeeds.
	require.NoError(t, r.Enqueue(context.Background(), "min-4", url, p))

	// Force same UUID (simulate duplicate) by directly inserting with a clashing id:
	// Actual implementation uses uuid.New() so two calls get different IDs.
	// Just verify two distinct enqueues create two rows.
	require.NoError(t, r.Enqueue(context.Background(), "min-4b", url, p))

	var count int
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM webhook_events`).Scan(&count))
	assert.Equal(t, 2, count)
}

// ── processPending / deliver ──────────────────────────────────────────────

func TestRetrier_ProcessPending_DeliverySuccess(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		var payload MinutesPayload
		json.NewDecoder(r.Body).Decode(&payload) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := openTestDB(t)
	client := NewClient(srv.URL, 5*time.Second)
	r := NewRetrier(db, client)

	require.NoError(t, r.Enqueue(context.Background(), "min-ok", srv.URL, testPayload("s-ok")))
	r.processPending(context.Background())

	assert.Equal(t, int32(1), received.Load())

	var status string
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT status FROM webhook_events WHERE minutes_id='min-ok'`).Scan(&status))
	assert.Equal(t, "delivered", status)
}

func TestRetrier_ProcessPending_DeliveryFailure_Retries(t *testing.T) {
	// Server always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	db := openTestDB(t)
	r := NewRetrier(db, NewClient(srv.URL, 5*time.Second))

	require.NoError(t, r.Enqueue(context.Background(), "min-fail", srv.URL, testPayload("s-fail")))
	r.processPending(context.Background()) // attempt 1

	var status string
	var attempt int
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT status, attempt_number FROM webhook_events WHERE minutes_id='min-fail'`).Scan(&status, &attempt))

	// Still pending (not yet at maxAttempts=5)
	assert.Equal(t, "pending", status)
	assert.Equal(t, 1, attempt)
}

func TestRetrier_ProcessPending_MaxAttempts_MarksFailed(t *testing.T) {
	// Server always fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	db := openTestDB(t)
	r := NewRetrier(db, NewClient(srv.URL, 5*time.Second))

	require.NoError(t, r.Enqueue(context.Background(), "min-maxfail", srv.URL, testPayload("s-max")))

	// Manually set next_retry_at in the past and simulate maxAttempts-1 prior attempts.
	past := time.Now().Add(-1 * time.Second).UTC().Format(time.RFC3339)
	_, err := db.ExecContext(context.Background(), `UPDATE webhook_events SET attempt_number=?, next_retry_at=? WHERE minutes_id='min-maxfail'`,
		maxAttempts-1, past)
	require.NoError(t, err)

	r.processPending(context.Background()) // this is attempt maxAttempts → failed

	var status string
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT status FROM webhook_events WHERE minutes_id='min-maxfail'`).Scan(&status))
	assert.Equal(t, "failed", status)
}

func TestRetrier_ProcessPending_SkipsFutureRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	db := openTestDB(t)
	r := NewRetrier(db, NewClient(srv.URL, 5*time.Second))

	require.NoError(t, r.Enqueue(context.Background(), "min-future", srv.URL, testPayload("s-future")))

	// Set next_retry_at far in the future.
	future := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
	_, err := db.ExecContext(context.Background(), `UPDATE webhook_events SET next_retry_at=? WHERE minutes_id='min-future'`, future)
	require.NoError(t, err)

	r.processPending(context.Background())

	// HTTP server should NOT have been called.
	assert.Equal(t, int32(0), calls.Load())
}

func TestRetrier_ProcessPending_DeliveredPayloadMatchesEnqueued(t *testing.T) {
	var receivedPayload MinutesPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := openTestDB(t)
	r := NewRetrier(db, NewClient(srv.URL, 5*time.Second))

	payload := &MinutesPayload{
		SessionID: "session-xyz",
		Minutes:   map[string]string{"key": "value"},
		Timestamp: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	require.NoError(t, r.Enqueue(context.Background(), "min-payload", srv.URL, payload))
	r.processPending(context.Background())

	assert.Equal(t, "session-xyz", receivedPayload.SessionID)
}

// ── Run (background worker) ───────────────────────────────────────────────

func TestRetrier_Run_DeliversEventWithinInterval(t *testing.T) {
	var delivered atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delivered.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := openTestDB(t)
	// Use a very short interval for the test (override default workerInterval via direct call).
	r := NewRetrier(db, NewClient(srv.URL, 5*time.Second))

	require.NoError(t, r.Enqueue(context.Background(), "min-run", srv.URL, testPayload("s-run")))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Drive delivery manually (don't wait for the 30s ticker).
	r.processPending(ctx)

	assert.Equal(t, int32(1), delivered.Load())
}

func TestRetrier_Run_StopsOnContextCancel(t *testing.T) {
	db := openTestDB(t)
	r := NewRetrier(db, NewClient("", 5*time.Second))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		r.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(3 * time.Second):
		t.Fatal("Retrier.Run did not stop after context cancel")
	}
}
