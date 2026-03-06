package minutes

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE minutes (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL UNIQUE REFERENCES sessions(id) ON DELETE CASCADE,
			version INTEGER NOT NULL DEFAULT 1,
			themes TEXT NOT NULL,
			contents_reported TEXT NOT NULL,
			professional_interventions TEXT NOT NULL,
			progress_issues TEXT NOT NULL,
			next_steps TEXT NOT NULL,
			citations TEXT NOT NULL,
			generated_at TEXT NOT NULL DEFAULT (datetime('now')),
			delivered_at TEXT,
			status TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'delivered', 'error')),
			provider TEXT NOT NULL
		);

		CREATE TABLE minutes_history (
			id TEXT PRIMARY KEY,
			minutes_id TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
			version INTEGER NOT NULL,
			content TEXT NOT NULL,
			edited_at TEXT NOT NULL DEFAULT (datetime('now')),
			edited_by TEXT
		);

		CREATE INDEX idx_minutes_status ON minutes(status, generated_at);
		CREATE INDEX idx_minutes_history ON minutes_history(minutes_id, version);
	`)
	require.NoError(t, err)

	return db
}

func TestMinutesRepositoryCreate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("test-minutes-1", "session-123")
	minutes.Themes = []string{"Theme 1", "Theme 2"}
	minutes.ContentsReported = []ContentItem{{Text: "Content 1"}}
	minutes.ProfessionalInterventions = []ContentItem{{Text: "Intervention 1"}}
	minutes.ProgressIssues = Progress{Progress: []string{"Progress 1"}, Issues: []string{"Issue 1"}}
	minutes.NextSteps = []string{"Step 1"}
	minutes.Citations = []Citation{{TimestampMs: 1000, Text: "Quote 1", Role: "client"}}
	minutes.MarkReady()

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	db.Close()
}

func TestMinutesRepositoryCreateMultiple(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		minutes := NewMinutes(
			"test-minutes-"+string(rune('0'+i)),
			"session-"+string(rune('0'+i)),
		)
		minutes.Themes = []string{string(rune('T' + i))}
		minutes.MarkReady()

		err := repo.Create(ctx, minutes)
		require.NoError(t, err)
	}

	db.Close()
}

func TestMinutesRepositoryCreateWithDeliveredAt(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("test-minutes-delivered", "session-456")
	minutes.MarkDelivered()

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-minutes-delivered")
	require.NoError(t, err)
	assert.Equal(t, MinutesStatusDelivered, retrieved.Status)
	assert.NotNil(t, retrieved.DeliveredAt)
	assert.True(t, !minutes.DeliveredAt.IsZero() && minutes.DeliveredAt.UTC().Unix() > 0)

	db.Close()
}

func TestMinutesRepositoryGetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("test-minutes-1", "session-123")
	minutes.Themes = []string{"Theme 1", "Theme 2"}
	minutes.ContentsReported = []ContentItem{{Text: "Content 1"}}
	minutes.ProfessionalInterventions = []ContentItem{{Text: "Intervention 1"}}
	minutes.ProgressIssues = Progress{Progress: []string{"Progress 1"}, Issues: []string{"Issue 1"}}
	minutes.NextSteps = []string{"Step 1"}
	minutes.Citations = []Citation{{TimestampMs: 1000, Text: "Quote 1", Role: "client"}}
	minutes.MarkReady()

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-minutes-1")
	require.NoError(t, err)

	assert.Equal(t, minutes.ID, retrieved.ID)
	assert.Equal(t, minutes.SessionID, retrieved.SessionID)
	assert.Equal(t, minutes.Version, retrieved.Version)
	assert.Equal(t, minutes.Status, retrieved.Status)
	assert.Equal(t, minutes.Provider, retrieved.Provider)
	assert.Equal(t, len(minutes.Themes), len(retrieved.Themes))
	assert.Equal(t, minutes.Themes[0], retrieved.Themes[0])
	assert.Equal(t, len(minutes.ContentsReported), len(retrieved.ContentsReported))
	assert.Equal(t, minutes.ContentsReported[0].Text, retrieved.ContentsReported[0].Text)
	assert.Equal(t, len(minutes.ProfessionalInterventions), len(retrieved.ProfessionalInterventions))
	assert.Equal(t, minutes.ProfessionalInterventions[0].Text, retrieved.ProfessionalInterventions[0].Text)
	assert.Equal(t, len(minutes.ProgressIssues.Progress), len(retrieved.ProgressIssues.Progress))
	assert.Equal(t, minutes.ProgressIssues.Progress[0], retrieved.ProgressIssues.Progress[0])
	assert.Equal(t, len(minutes.NextSteps), len(retrieved.NextSteps))
	assert.Equal(t, minutes.NextSteps[0], retrieved.NextSteps[0])
	assert.Equal(t, len(minutes.Citations), len(retrieved.Citations))
	assert.Equal(t, minutes.Citations[0].TimestampMs, retrieved.Citations[0].TimestampMs)
	assert.Equal(t, minutes.Citations[0].Text, retrieved.Citations[0].Text)
	assert.Equal(t, minutes.Citations[0].Role, retrieved.Citations[0].Role)
	assert.True(t, !minutes.GeneratedAt.IsZero() && minutes.GeneratedAt.UTC().Unix() > 0)
	assert.True(t, !minutes.GeneratedAt.IsZero() && retrieved.GeneratedAt.UTC().Unix() > 0)

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
	minutes := NewMinutes("test-minutes-1", "session-456")
	minutes.Themes = []string{"Session Theme"}
	minutes.MarkReady()

	err := repo.Create(ctx, minutes)
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
	assert.Contains(t, err.Error(), "minutes not found for session")

	db.Close()
}

func TestMinutesRepositoryUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("test-minutes-1", "session-789")
	minutes.Themes = []string{"Initial Theme"}
	minutes.MarkReady()

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	updatedMinutes := NewMinutes("test-minutes-1", "session-789")
	updatedMinutes.Themes = []string{"Updated Theme 1", "Updated Theme 2"}
	updatedMinutes.ContentsReported = []ContentItem{{Text: "Updated Content"}}
	updatedMinutes.NextSteps = []string{"Updated Step 1"}

	err = repo.Update(ctx, updatedMinutes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "test-minutes-1")
	require.NoError(t, err)

	assert.Equal(t, 1, retrieved.Version)
	assert.Equal(t, len(updatedMinutes.Themes), len(retrieved.Themes))
	assert.Equal(t, updatedMinutes.Themes[0], retrieved.Themes[0])
	assert.Equal(t, updatedMinutes.Themes[1], retrieved.Themes[1])
	assert.Len(t, retrieved.ContentsReported, 1)
	assert.Equal(t, updatedMinutes.ContentsReported[0].Text, retrieved.ContentsReported[0].Text)
	assert.Len(t, updatedMinutes.NextSteps, 1)
	assert.Equal(t, updatedMinutes.NextSteps[0], retrieved.NextSteps[0])

	db.Close()
}

func TestMinutesRepositoryUpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("test-minutes-1", "session-999")
	minutes.Themes = []string{"Theme"}
	minutes.MarkReady()

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	minutes.MarkDelivered()
	now := time.Now().UTC()
	minutes.DeliveredAt = &now

	err = repo.Update(ctx, minutes)
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
	assert.NotZero(t, retrieved[0].EditedAt)

	db.Close()
}

func TestMinutesRepositoryCreateMultipleHistories(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		history := NewMinutesHistory(
			"history-"+string(rune('0'+i)),
			"minutes-1",
			i,
			"Version "+string(rune('0'+i)),
		)
		err := repo.CreateHistory(ctx, history)
		require.NoError(t, err)
	}

	retrieved, err := repo.GetHistory(ctx, "minutes-1")
	require.NoError(t, err)

	assert.Len(t, retrieved, 5)

	assert.Equal(t, 5, retrieved[0].Version)
	assert.Equal(t, "Version 5", retrieved[0].Content)

	assert.Equal(t, 1, retrieved[4].Version)
	assert.Equal(t, "Version 1", retrieved[4].Content)

	db.Close()
}

func TestMinutesRepositoryGetHistoryByMinutesID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()

	minutesID := "minutes-test-123"
	for i := 1; i <= 3; i++ {
		history := NewMinutesHistory(
			"history-"+string(rune('0'+i)),
			minutesID,
			i,
			"Content version "+string(rune('0'+i)),
		)
		if i == 2 {
			history.SetEditedBy("user-admin")
		}
		err := repo.CreateHistory(ctx, history)
		require.NoError(t, err)
	}

	retrieved, err := repo.GetHistory(ctx, minutesID)
	require.NoError(t, err)

	assert.Len(t, retrieved, 3)

	assert.Equal(t, "Content version 3", retrieved[0].Content)
	assert.Equal(t, "user-admin", retrieved[1].EditedBy)
	assert.Equal(t, "Content version 1", retrieved[2].Content)

	db.Close()
}

func TestMinutesRepositoryGetHistoryEmpty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	histories, err := repo.GetHistory(ctx, "minutes-no-histories")
	require.NoError(t, err)
	assert.Len(t, histories, 0)

	db.Close()
}

func TestMinutesRepositoryJSONParsing(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("json-test", "session-123")
	minutes.Themes = []string{"Complex theme with nested JSON array"}
	minutes.ContentsReported = []ContentItem{
		{Text: "Item with complex structure", Timestamp: 1000},
		{Text: "Another item", Timestamp: 2000},
	}
	minutes.ProfessionalInterventions = []ContentItem{
		{Text: "Professional intervention with multiple parts"},
	}
	minutes.ProgressIssues = Progress{
		Progress: []string{"Progress 1", "Progress 2", "Progress 3"},
		Issues:   []string{"Issue 1", "Issue 2"},
	}
	minutes.NextSteps = []string{"Step 1", "Step 2", "Step 3"}
	minutes.Citations = []Citation{
		{TimestampMs: 1000, Text: "Quote 1", Role: "client"},
		{TimestampMs: 2000, Text: "Quote 2", Role: "professional"},
		{TimestampMs: 3000, Text: "Quote 3", Role: "client"},
	}
	minutes.MarkReady()

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "json-test")
	require.NoError(t, err)

	assert.Len(t, retrieved.Themes, 1)
	assert.Len(t, retrieved.ContentsReported, 2)
	assert.Len(t, retrieved.ProfessionalInterventions, 1)
	assert.Len(t, retrieved.ProgressIssues.Progress, 3)
	assert.Len(t, retrieved.ProgressIssues.Issues, 2)
	assert.Len(t, retrieved.NextSteps, 3)
	assert.Len(t, retrieved.Citations, 3)

	db.Close()
}

func TestMinutesRepositoryTimeParsing(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	expectedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	minutes := NewMinutes("time-test", "session-456")
	minutes.GeneratedAt = expectedTime
	minutes.MarkDelivered()

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "time-test")
	require.NoError(t, err)

	assert.Equal(t, expectedTime.UTC(), retrieved.GeneratedAt.UTC())
	assert.NotNil(t, retrieved.DeliveredAt)

	db.Close()
}

func TestMinutesRepositoryConcurrentCreate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()

	go func() {
		for i := 0; i < 10; i++ {
			minutes := NewMinutes(
				"concurrent-"+string(rune('0'+i%10)),
				"session-concurrent",
			)
			minutes.MarkReady()
			repo.Create(ctx, minutes)
		}
	}()

	time.Sleep(10 * time.Millisecond)
	retrieved, err := repo.GetBySession(ctx, "session-concurrent")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)

	db.Close()
}

func TestMinutesRepositoryNullDeliveredAt(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("null-delivered", "session-777")
	minutes.Themes = []string{"Theme"}
	minutes.Status = MinutesStatusReady
	minutes.DeliveredAt = nil

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "null-delivered")
	require.NoError(t, err)

	assert.Equal(t, MinutesStatusReady, retrieved.Status)
	assert.Nil(t, retrieved.DeliveredAt)

	db.Close()
}

func TestMinutesRepositoryNullEditedBy(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	history := NewMinutesHistory("history-null", "minutes-null", 1, "Content")
	history.SetEditedBy("")

	err := repo.CreateHistory(ctx, history)
	require.NoError(t, err)

	retrieved, err := repo.GetHistory(ctx, "minutes-null")
	require.NoError(t, err)

	assert.Len(t, retrieved, 1)
	assert.Empty(t, retrieved[0].EditedBy)

	db.Close()
}

func TestMinutesRepositoryVersionIncrement(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("version-test", "session-888")
	minutes.Themes = []string{"Theme 1"}

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)
	assert.Equal(t, 1, minutes.Version)

	minutes.IncrementVersion()
	minutes.Themes = []string{"Theme 2"}

	err = repo.Update(ctx, minutes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "version-test")
	require.NoError(t, err)
	assert.Equal(t, 2, retrieved.Version)

	minutes.IncrementVersion()
	minutes.Themes = []string{"Theme 3"}

	err = repo.Update(ctx, minutes)
	require.NoError(t, err)

	retrieved, err = repo.GetByID(ctx, "version-test")
	require.NoError(t, err)
	assert.Equal(t, 3, retrieved.Version)

	db.Close()
}

func TestMinutesRepositoryStatusCheckConstraints(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMinutesRepository(db)

	ctx := context.Background()
	minutes := NewMinutes("status-test", "session-999")
	minutes.Themes = []string{"Theme"}
	minutes.Status = MinutesStatusPending

	err := repo.Create(ctx, minutes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, "status-test")
	require.NoError(t, err)
	assert.Equal(t, MinutesStatusPending, retrieved.Status)

	minutes.MarkReady()
	err = repo.Update(ctx, minutes)
	require.NoError(t, err)

	retrieved, err = repo.GetByID(ctx, "status-test")
	require.NoError(t, err)
	assert.Equal(t, MinutesStatusReady, retrieved.Status)

	db.Close()
}
