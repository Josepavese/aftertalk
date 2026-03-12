package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestDB_Migrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(context.Background(), dbPath)
	require.NoError(t, err)
	defer db.Close()

	t.Run("AllTablesCreated", func(t *testing.T) {
		tables := []struct {
			name  string
			query string
		}{
			{
				name:  "sessions",
				query: `CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, status TEXT, created_at TEXT, ended_at TEXT, participant_count INTEGER, metadata TEXT)`,
			},
			{
				name:  "participants",
				query: `CREATE TABLE IF NOT EXISTS participants (id TEXT PRIMARY KEY, session_id TEXT, user_id TEXT, role TEXT, token_jti TEXT, token_expires_at TEXT, token_used INTEGER, connected_at TEXT, disconnected_at TEXT)`,
			},
			{
				name:  "audio_streams",
				query: `CREATE TABLE IF NOT EXISTS audio_streams (id TEXT PRIMARY KEY, participant_id TEXT, codec TEXT, sample_rate INTEGER, channels INTEGER, chunk_size_seconds REAL, started_at TEXT, ended_at TEXT, chunks_received INTEGER, status TEXT)`,
			},
			{
				name:  "transcriptions",
				query: `CREATE TABLE IF NOT EXISTS transcriptions (id TEXT PRIMARY KEY, session_id TEXT, segment_index INTEGER, role TEXT, start_ms INTEGER, end_ms INTEGER, text TEXT, confidence REAL, provider TEXT, created_at TEXT, status TEXT)`,
			},
			{
				name:  "minutes",
				query: `CREATE TABLE IF NOT EXISTS minutes (id TEXT PRIMARY KEY, session_id TEXT, version INTEGER, themes TEXT, contents_reported TEXT, professional_interventions TEXT, progress_issues TEXT, next_steps TEXT, citations TEXT, generated_at TEXT, delivered_at TEXT, status TEXT, provider TEXT)`,
			},
		}

		for _, table := range tables {
			_, err := db.ExecContext(context.Background(), table.query)
			assert.NoError(t, err)

			var tableName string
			err = db.QueryRowContext(context.Background(), `
				SELECT name FROM sqlite_master WHERE type='table' AND name=?
			`, table.name).Scan(&tableName)

			assert.NoError(t, err)
			assert.Equal(t, table.name, tableName)
		}
	})

	t.Run("ColumnsExist", func(t *testing.T) {
		rows, err := db.QueryContext(context.Background(), "PRAGMA table_info(sessions)")
		require.NoError(t, err)
		defer rows.Close()

		columnNames := make(map[string]bool)
		for rows.Next() {
			var cid int
			var name string
			var type_ string
			var notn int
			var dflt interface{}
			var pk int

			err := rows.Scan(&cid, &name, &type_, &notn, &dflt, &pk)
			assert.NoError(t, err)
			columnNames[name] = true
		}
		assert.NoError(t, rows.Err())

		expectedColumns := map[string]bool{
			"id": true,
			"status": true,
			"created_at": true,
			"ended_at": true,
			"participant_count": true,
			"metadata": true,
		}

		for col := range expectedColumns {
			assert.True(t, columnNames[col], "column %s should exist", col)
		}
	})
}

func TestSessionCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(context.Background(), dbPath)
	require.NoError(t, err)
	defer db.Close()

	sessionRepo := session.NewSessionRepository(db.DB)

	_, err = db.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, status TEXT, created_at TEXT, ended_at TEXT, participant_count INTEGER, template_id TEXT NOT NULL DEFAULT '', metadata TEXT)`)
	require.NoError(t, err)

	t.Run("CreateSession", func(t *testing.T) {
		session := session.NewSession(generateID(t), 3, "")
		session.Status = "active"

		err := sessionRepo.Create(context.Background(), session)
		assert.NoError(t, err)
		assert.NotEmpty(t, session.ID)
		assert.NotZero(t, session.CreatedAt)
		assert.Equal(t, "active", string(session.Status))
		assert.Equal(t, 3, session.ParticipantCount)
	})

	t.Run("GetSession", func(t *testing.T) {
		created := createTestSession(t, sessionRepo)

		retrieved, err := sessionRepo.GetByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, created.Status, retrieved.Status)
	})

	t.Run("UpdateSession", func(t *testing.T) {
		created := createTestSession(t, sessionRepo)

		updated := &session.Session{
			ID:               created.ID,
			Status:           "ended",
			ParticipantCount: 4,
			Metadata:         `{"title": "Updated Title"}`,
		}
		updated.CreatedAt = created.CreatedAt

		err := sessionRepo.Update(context.Background(), updated)
		assert.NoError(t, err)

		retrieved, err := sessionRepo.GetByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, "ended", string(retrieved.Status))
	})
}

func TestConcurrentOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(context.Background(), dbPath)
	require.NoError(t, err)
	defer db.Close()

	sessionRepo := session.NewSessionRepository(db.DB)

	_, err = db.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, status TEXT, created_at TEXT, ended_at TEXT, participant_count INTEGER, template_id TEXT NOT NULL DEFAULT '', metadata TEXT)`)
	require.NoError(t, err)

	t.Run("ConcurrentSessionCRUD", func(t *testing.T) {
		numSessions := 50
		var wg sync.WaitGroup

		for i := 0; i < numSessions; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				session := session.NewSession(generateID(t), 1, "")
				session.Status = "active"
				err := sessionRepo.Create(context.Background(), session)
				assert.NoError(t, err)
			}(i)
		}
		wg.Wait()

		var count int
		err = db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM sessions").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, numSessions, count)
	})
}

func generateID(t *testing.T) string {
	return fmt.Sprintf("test-id-%d", time.Now().UnixNano())
}

func createTestSession(t *testing.T, repo *session.SessionRepository) *session.Session {
	session := session.NewSession(generateID(t), 2, "")
	session.Metadata = `{"title": "Test"}`
	session.Status = "active"
	err := repo.Create(context.Background(), session)
	assert.NoError(t, err)
	return session
}
