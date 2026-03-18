package transcription

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestCreateTranscription(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	transcription := NewTranscription(
		"test-id-1",
		"session-123",
		0,
		"host",
		0,
		1000,
		"Hello world",
	)
	transcription.SetConfidence(0.95)
	transcription.SetProvider("google")

	err := repo.Create(ctx, transcription)
	require.NoError(t, err)

	assert.Equal(t, "test-id-1", transcription.ID)
	assert.Equal(t, "session-123", transcription.SessionID)
	assert.InEpsilon(t, 0.95, transcription.Confidence, 1e-9)
	assert.Equal(t, "google", transcription.Provider)
	assert.Equal(t, StatusPending, transcription.Status)
}

func TestGetByID(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	trans := createTestTranscription(t, db, "test-id-1", "session-123", 0, "host", 0, 1000, "Hello world", 0.95, "google", StatusReady)

	retrieved, err := repo.GetByID(ctx, "test-id-1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, trans.ID, retrieved.ID)
	assert.Equal(t, trans.SessionID, retrieved.SessionID)
	assert.Equal(t, trans.SegmentIndex, retrieved.SegmentIndex)
	assert.Equal(t, trans.Role, retrieved.Role)
	assert.Equal(t, trans.StartMs, retrieved.StartMs)
	assert.Equal(t, trans.EndMs, retrieved.EndMs)
	assert.Equal(t, trans.Text, retrieved.Text)
	assert.InEpsilon(t, trans.Confidence, retrieved.Confidence, 1e-9)
	assert.Equal(t, trans.Provider, retrieved.Provider)
	assert.Equal(t, trans.Status, retrieved.Status)
	assert.WithinDuration(t, trans.CreatedAt, retrieved.CreatedAt, time.Second)
}

func TestGetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	_, err := repo.GetByID(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetBySession(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	sessionID := "session-123"
	createTestTranscription(t, db, "id-1", sessionID, 0, "host", 0, 1000, "Hello world", 0.9, "google", StatusReady)
	createTestTranscription(t, db, "id-2", sessionID, 1, "guest", 1000, 2000, "Hi there", 0.85, "google", StatusReady)
	createTestTranscription(t, db, "id-3", "session-456", 0, "host", 0, 1000, "Different session", 0.95, "google", StatusReady)

	transcriptions, err := repo.GetBySession(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, transcriptions, 2)

	assert.Equal(t, "id-1", transcriptions[0].ID)
	assert.Equal(t, "id-2", transcriptions[1].ID)

	for _, trans := range transcriptions {
		assert.Equal(t, sessionID, trans.SessionID)
	}
}

func TestGetBySessionOrdered(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	sessionID := "session-ordered"
	createTestTranscription(t, db, "id-1", sessionID, 2, "host", 2000, 3000, "Third segment", 0.8, "google", StatusReady)
	createTestTranscription(t, db, "id-2", sessionID, 0, "host", 0, 1000, "First segment", 0.9, "google", StatusReady)
	createTestTranscription(t, db, "id-3", sessionID, 1, "host", 1000, 2000, "Second segment", 0.85, "google", StatusReady)

	transcriptions, err := repo.GetBySessionOrdered(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, transcriptions, 3)

	assert.Equal(t, "id-2", transcriptions[0].ID)
	assert.Equal(t, "id-3", transcriptions[1].ID)
	assert.Equal(t, "id-1", transcriptions[2].ID)
}

func TestUpdateStatus(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	err := repo.Create(ctx, NewTranscription("test-id", "session-123", 0, "host", 0, 1000, "Hello"))
	require.NoError(t, err)

	err = repo.UpdateStatus(ctx, "test-id", StatusProcessing)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, "test-id")
	require.NoError(t, err)
	assert.Equal(t, StatusProcessing, updated.Status)
}

func TestGetBySession_Empty(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	transcriptions, err := repo.GetBySession(ctx, "non-existent-session")
	require.NoError(t, err)
	require.Empty(t, transcriptions)
}

func TestTranscriptionWithNullConfidence(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	trans := NewTranscription("test-id", "session-123", 0, "host", 0, 1000, "Hello world")
	trans.SetProvider("google")

	err := repo.Create(ctx, trans)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-id")
	require.NoError(t, err)

	assert.Zero(t, retrieved.Confidence)
}

func TestTranscriptionWithConfidenceZero(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	trans := NewTranscription("test-id", "session-123", 0, "host", 0, 1000, "Hello world")
	trans.SetConfidence(0.0)
	trans.SetProvider("google")

	err := repo.Create(ctx, trans)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-id")
	require.NoError(t, err)

	assert.Zero(t, retrieved.Confidence)
}

func TestTranscriptionWithMaxConfidence(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	trans := NewTranscription("test-id", "session-123", 0, "host", 0, 1000, "Hello world")
	trans.SetConfidence(1.0)
	trans.SetProvider("google")

	err := repo.Create(ctx, trans)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-id")
	require.NoError(t, err)

	assert.InEpsilon(t, 1.0, retrieved.Confidence, 1e-9)
}

func TestCreateTranscriptionWithMultipleSegments(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	for i := 0; i < 5; i++ {
		trans := NewTranscription(
			uuid.New().String(),
			"test-session",
			i,
			"host",
			i*1000,
			(i+1)*1000,
			"Segment text",
		)
		trans.SetProvider("google")
		trans.MarkReady()

		err := repo.Create(ctx, trans)
		require.NoError(t, err)
	}

	transcriptions, err := repo.GetBySession(ctx, "test-session")
	require.NoError(t, err)
	require.Len(t, transcriptions, 5)

	for i, trans := range transcriptions {
		assert.Equal(t, i, trans.SegmentIndex)
		assert.Equal(t, i*1000, trans.StartMs)
		assert.Equal(t, (i+1)*1000, trans.EndMs)
	}
}

func TestGetBySession_MixedStatuses(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	sessionID := "session-mixed"
	createTestTranscription(t, db, "id-1", sessionID, 0, "host", 0, 1000, "Ready", 0.9, "google", StatusReady)
	createTestTranscription(t, db, "id-2", sessionID, 1, "guest", 1000, 2000, "Processing", 0.8, "google", StatusProcessing)
	createTestTranscription(t, db, "id-3", sessionID, 2, "host", 2000, 3000, "Pending", 0.85, "google", StatusPending)

	transcriptions, err := repo.GetBySession(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, transcriptions, 3)

	assert.Equal(t, StatusReady, transcriptions[0].Status)
	assert.Equal(t, StatusProcessing, transcriptions[1].Status)
	assert.Equal(t, StatusPending, transcriptions[2].Status)
}

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	err = db.PingContext(t.Context())
	require.NoError(t, err)

	err = createTranscriptionTable(db)
	require.NoError(t, err)

	return db
}

func createTranscriptionTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS transcriptions (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			segment_index INTEGER NOT NULL,
			role TEXT NOT NULL,
			start_ms INTEGER NOT NULL CHECK (start_ms >= 0),
			end_ms INTEGER NOT NULL CHECK (end_ms > start_ms),
			text TEXT NOT NULL,
			confidence REAL CHECK (confidence BETWEEN 0.0 AND 1.0),
			provider TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'ready', 'error')),
			language TEXT NOT NULL DEFAULT '',
			UNIQUE(session_id, segment_index)
		)
	`
	_, err := db.ExecContext(context.Background(), query)
	return err
}

func createTestTranscription(t *testing.T, db *sql.DB, id, sessionID string, segmentIndex int, role string, startMs, endMs int, text string, confidence float64, provider string, status TranscriptionStatus) *Transcription {
	ctx := context.Background()
	repo := NewTranscriptionRepository(db)

	trans := NewTranscription(id, sessionID, segmentIndex, role, startMs, endMs, text)
	trans.SetConfidence(confidence)
	trans.SetProvider(provider)
	trans.Status = status

	err := repo.Create(ctx, trans)
	require.NoError(t, err)

	return trans
}
