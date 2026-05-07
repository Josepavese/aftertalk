package minutes

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE minutes (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL UNIQUE,
			template_id TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			content TEXT NOT NULL DEFAULT '{}',
			generated_at TEXT NOT NULL DEFAULT (datetime('now')),
			delivered_at TEXT,
			status TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'delivered', 'error')),
			provider TEXT NOT NULL DEFAULT ''
		);

		CREATE TABLE minutes_history (
			id TEXT PRIMARY KEY,
			minutes_id TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
			version INTEGER NOT NULL,
			content TEXT NOT NULL,
			edited_at TEXT NOT NULL DEFAULT (datetime('now')),
			edited_by TEXT
		);
	`)
	require.NoError(t, err)

	return db
}

func TestMinutesRepositoryCreate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	m := NewMinutes("test-minutes-1", "session-123", "tmpl-1")
	m.Summary = Summary{
		Overview: "Repository summary",
		Phases: []Phase{
			{Title: "Opening", Summary: "Started session", StartMs: 0, EndMs: 1000},
		},
	}
	m.Sections = map[string]json.RawMessage{
		"themes": json.RawMessage(`["Theme 1","Theme 2"]`),
	}
	m.Citations = []Citation{{TimestampMs: 1000, Text: "Quote 1", Role: "client"}}
	m.MarkReady()

	err := repo.Create(ctx, m)
	require.NoError(t, err)

	db.Close()
}

func TestMinutesRepositoryCreateWithDeliveredAt(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	m := NewMinutes("test-minutes-delivered", "session-456", "tmpl-1")
	m.MarkDelivered()

	err := repo.Create(ctx, m)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-minutes-delivered")
	require.NoError(t, err)
	assert.Equal(t, MinutesStatusDelivered, retrieved.Status)
	assert.NotNil(t, retrieved.DeliveredAt)

	db.Close()
}

func TestMinutesRepositoryGetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	m := NewMinutes("test-minutes-1", "session-123", "tmpl-1")
	m.Summary = Summary{
		Overview: "Repository summary",
		Phases: []Phase{
			{Title: "Opening", Summary: "Started session", StartMs: 0, EndMs: 1000},
		},
	}
	m.Sections = map[string]json.RawMessage{
		"themes": json.RawMessage(`["Theme 1","Theme 2"]`),
	}
	m.Citations = []Citation{{TimestampMs: 1000, Text: "Quote 1", Role: "client"}}
	m.MarkReady()

	err := repo.Create(ctx, m)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-minutes-1")
	require.NoError(t, err)

	assert.Equal(t, m.ID, retrieved.ID)
	assert.Equal(t, m.SessionID, retrieved.SessionID)
	assert.Equal(t, m.TemplateID, retrieved.TemplateID)
	assert.Equal(t, m.Version, retrieved.Version)
	assert.Equal(t, m.Status, retrieved.Status)
	assert.Equal(t, "Repository summary", retrieved.Summary.Overview)
	assert.Len(t, retrieved.Summary.Phases, 1)
	assert.Len(t, retrieved.Citations, 1)
	assert.Equal(t, 1000, retrieved.Citations[0].TimestampMs)
	assert.Equal(t, "Quote 1", retrieved.Citations[0].Text)

	db.Close()
}

func TestMinutesRepositoryGetByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	_, err := repo.GetByID(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minutes not found")

	db.Close()
}

func TestMinutesRepositoryGetBySession(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	m := NewMinutes("test-minutes-1", "session-456", "tmpl-1")
	m.MarkReady()

	err := repo.Create(ctx, m)
	require.NoError(t, err)

	retrieved, err := repo.GetBySession(ctx, "session-456")
	require.NoError(t, err)

	assert.Equal(t, "test-minutes-1", retrieved.ID)
	assert.Equal(t, "session-456", retrieved.SessionID)

	db.Close()
}

func TestMinutesRepositoryGetBySessionNotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	_, err := repo.GetBySession(ctx, "non-existent-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minutes not found")

	db.Close()
}

func TestMinutesRepositoryUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	m := NewMinutes("test-minutes-1", "session-789", "tmpl-1")
	m.MarkReady()

	err := repo.Create(ctx, m)
	require.NoError(t, err)

	m.Sections = map[string]json.RawMessage{
		"themes": json.RawMessage(`["Updated Theme 1","Updated Theme 2"]`),
	}
	m.IncrementVersion()

	err = repo.Update(ctx, m)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-minutes-1")
	require.NoError(t, err)

	assert.Equal(t, 2, retrieved.Version)

	db.Close()
}

func TestMinutesRepositoryUpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	m := NewMinutes("test-minutes-1", "session-999", "tmpl-1")
	m.MarkReady()

	err := repo.Create(ctx, m)
	require.NoError(t, err)

	m.MarkDelivered()
	now := time.Now().UTC()
	m.DeliveredAt = &now

	err = repo.Update(ctx, m)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-minutes-1")
	require.NoError(t, err)
	assert.Equal(t, MinutesStatusDelivered, retrieved.Status)
	assert.NotNil(t, retrieved.DeliveredAt)

	db.Close()
}

func TestMinutesRepositoryCreateHistory(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	history := NewMinutesHistory("history-1", "minutes-1", 1, "Initial content")
	history.SetEditedBy("user-123")

	err := repo.CreateHistory(ctx, history)
	require.NoError(t, err)

	retrieved, err := repo.GetHistory(ctx, "minutes-1")
	require.NoError(t, err)

	assert.Len(t, retrieved, 1)
	assert.Equal(t, "history-1", retrieved[0].ID)
	assert.Equal(t, "minutes-1", retrieved[0].MinutesID)
	assert.Equal(t, 1, retrieved[0].Version)
	assert.Equal(t, "Initial content", retrieved[0].Content)
	assert.Equal(t, "user-123", retrieved[0].EditedBy)

	db.Close()
}

func TestMinutesRepositoryGetHistoryEmpty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	histories, err := repo.GetHistory(ctx, "minutes-no-histories")
	require.NoError(t, err)
	assert.Empty(t, histories)

	db.Close()
}

func TestMinutesRepositoryTimeParsing(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	expectedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	m := NewMinutes("time-test", "session-456", "tmpl-1")
	m.GeneratedAt = expectedTime
	m.MarkDelivered()

	err := repo.Create(ctx, m)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "time-test")
	require.NoError(t, err)

	assert.Equal(t, expectedTime.UTC(), retrieved.GeneratedAt.UTC())
	assert.NotNil(t, retrieved.DeliveredAt)

	db.Close()
}

func TestMinutesRepositoryNullDeliveredAt(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	m := NewMinutes("null-delivered", "session-777", "tmpl-1")
	m.Status = MinutesStatusReady
	m.DeliveredAt = nil

	err := repo.Create(ctx, m)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "null-delivered")
	require.NoError(t, err)

	assert.Equal(t, MinutesStatusReady, retrieved.Status)
	assert.Nil(t, retrieved.DeliveredAt)

	db.Close()
}

func TestMinutesRepositoryVersionIncrement(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	m := NewMinutes("version-test", "session-888", "tmpl-1")

	err := repo.Create(ctx, m)
	require.NoError(t, err)
	assert.Equal(t, 1, m.Version)

	m.IncrementVersion()
	err = repo.Update(ctx, m)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "version-test")
	require.NoError(t, err)
	assert.Equal(t, 2, retrieved.Version)

	db.Close()
}

func TestMinutesRepositoryUpdatePersistsProviderAndTemplate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	ctx := context.Background()

	m := NewMinutes("provider-update", "session-provider-update", "therapy")
	m.Provider = "ollama"
	require.NoError(t, repo.Create(ctx, m))

	m.TemplateID = "premium"
	m.Provider = "openai"
	m.MarkReady()
	require.NoError(t, repo.Update(ctx, m))

	retrieved, err := repo.GetByID(ctx, "provider-update")
	require.NoError(t, err)
	assert.Equal(t, "premium", retrieved.TemplateID)
	assert.Equal(t, "openai", retrieved.Provider)

	db.Close()
}

func TestMinutesRepositoryHasWebhookEvent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	ctx := context.Background()

	hasEvent, err := repo.HasWebhookEvent(ctx, "minutes-1")
	require.NoError(t, err)
	assert.False(t, hasEvent)

	_, err = db.ExecContext(ctx, `
		CREATE TABLE webhook_events (
			id TEXT PRIMARY KEY,
			minutes_id TEXT NOT NULL
		);
		INSERT INTO webhook_events (id, minutes_id) VALUES ('event-1', 'minutes-1');
	`)
	require.NoError(t, err)

	hasEvent, err = repo.HasWebhookEvent(ctx, "minutes-1")
	require.NoError(t, err)
	assert.True(t, hasEvent)

	db.Close()
}

func TestMinutesRepositoryListAndReplayWebhookEvents(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)
	ctx := context.Background()

	m := NewMinutes("minutes-1", "session-1", "tmpl-1")
	require.NoError(t, repo.Create(ctx, m))
	_, err := db.ExecContext(ctx, `
		CREATE TABLE webhook_events (
			id TEXT PRIMARY KEY,
			minutes_id TEXT NOT NULL,
			webhook_url TEXT NOT NULL,
			payload_type TEXT NOT NULL DEFAULT 'minutes',
			attempt_number INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			delivered_at TEXT,
			error_message TEXT,
			next_retry_at TEXT,
			created_at TEXT NOT NULL
		);
		INSERT INTO webhook_events
			(id, minutes_id, webhook_url, payload_type, attempt_number, status, delivered_at, error_message, next_retry_at, created_at)
		VALUES
			('event-1', 'minutes-1', 'https://example.test/hook', 'error', 5, 'failed', '2026-05-06T10:01:00Z', 'boom', '2099-01-01T00:00:00Z', '2026-05-06T10:00:00Z');
	`)
	require.NoError(t, err)

	events, err := repo.ListWebhookEvents(ctx, "session-1", "")
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "event-1", events[0].ID)
	assert.Equal(t, "error", events[0].PayloadType)
	assert.Equal(t, "failed", events[0].Status)
	assert.Equal(t, 5, events[0].AttemptNumber)
	assert.Equal(t, "boom", events[0].ErrorMessage)

	require.NoError(t, repo.ReplayWebhookEvent(ctx, "event-1"))
	var status string
	var errorMessage sql.NullString
	var deliveredAt sql.NullString
	var attemptNumber int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT status, attempt_number, delivered_at, error_message FROM webhook_events WHERE id='event-1'`).Scan(&status, &attemptNumber, &deliveredAt, &errorMessage))
	assert.Equal(t, "pending", status)
	assert.Equal(t, 0, attemptNumber)
	assert.False(t, deliveredAt.Valid)
	assert.False(t, errorMessage.Valid)

	db.Close()
}
