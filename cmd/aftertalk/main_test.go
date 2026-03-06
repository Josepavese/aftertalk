package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/flowup/aftertalk/internal/api"
	"github.com/flowup/aftertalk/internal/config"
	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/internal/storage/sqlite"
	"github.com/flowup/aftertalk/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Setup before tests
	os.Setenv("AFTERTALK_DB_PATH", "/tmp/test_aftertalk.db")

	exitCode := m.Run()

	// Cleanup after tests
	os.Remove("/tmp/test_aftertalk.db")

	os.Exit(exitCode)
}

func TestRunMigrations(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify migrations ran by checking tables exist
	var tableName string
	err = db.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' LIMIT 1").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "sessions", tableName)
}

func TestRunMigrations_SessionsTable(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify sessions table exists
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify sessions table has correct columns
	var columns []string
	rows, err := db.DB.Query("PRAGMA table_info(sessions)")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dtype string
		var notnull int
		var dflt_value interface{}
		err := rows.Scan(&cid, &name, &dtype, &notnull, &dflt_value)
		require.NoError(t, err)
		columns = append(columns, name)
	}

	assert.Contains(t, columns, "id")
	assert.Contains(t, columns, "status")
	assert.Contains(t, columns, "created_at")
	assert.Contains(t, columns, "ended_at")
	assert.Contains(t, columns, "participant_count")
	assert.Contains(t, columns, "metadata")
}

func TestRunMigrations_ParticipantsTable(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify participants table exists
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='participants'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunMigrations_AudioStreamsTable(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify audio_streams table exists
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='audio_streams'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunMigrations_TranscriptionsTable(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify transcriptions table exists
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='transcriptions'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunMigrations_MinutesTable(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify minutes table exists
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='minutes'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunMigrations_WebhookEventsTable(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify webhook_events table exists
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='webhook_events'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunMigrations_ProcessingQueueTable(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify processing_queue table exists
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='processing_queue'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunMigrations_Indexes(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify indexes exist for sessions
	var indexCount int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND tbl_name='sessions'").Scan(&indexCount)
	require.NoError(t, err)
	assert.Equal(t, 2, indexCount)
}

func TestRunMigrations_IndexesForParticipants(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify indexes exist for participants
	var indexCount int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND tbl_name='participants'").Scan(&indexCount)
	require.NoError(t, err)
	assert.Equal(t, 2, indexCount)
}

func TestRunMigrations_WithTransaction(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	// Run migrations in a transaction
	err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
		return runMigrations(context.Background(), db)
	})

	require.NoError(t, err)

	// Verify data was committed
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 6, count)
}

func TestRunMigrations_CreatesAllTables(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	expectedTables := []string{
		"sessions",
		"participants",
		"audio_streams",
		"transcriptions",
		"minutes",
		"webhook_events",
		"processing_queue",
	}

	for _, tableName := range expectedTables {
		var count int
		err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Table %s should exist", tableName)
	}
}

func TestRunMigrations_Transactions(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	// Verify migration runs successfully
	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify data is persisted
	var tableName string
	err = db.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' LIMIT 1").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "sessions", tableName)
}

func TestDatabaseInitialization(t *testing.T) {
	dbPath := "/tmp/test_aftertalk.db"
	defer os.Remove(dbPath)

	db, err := sqlite.New(context.Background(), dbPath)
	require.NoError(t, err)
	defer db.Close()

	assert.NotNil(t, db)
	assert.NotNil(t, db.DB)
}

func TestDatabaseInitialization_Failure(t *testing.T) {
	// Test with invalid path
	db, err := sqlite.New(context.Background(), "/invalid/path/to/database.db")
	assert.Error(t, err)
	assert.Nil(t, db)
}

func TestServiceCreation(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)

	assert.NotNil(t, sessionService)
}

func TestTokenCacheCreation(t *testing.T) {
	tokenCache := cache.NewTokenCache()

	assert.NotNil(t, tokenCache)
}

func TestSessionCacheCreation(t *testing.T) {
	sessionCache := cache.NewSessionCache()

	assert.NotNil(t, sessionCache)
}

func TestJWTManagerCreation(t *testing.T) {
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)

	assert.NotNil(t, jwtManager)
}

func TestAPIServerCreation(t *testing.T) {
	cfg := &config.Config{}
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	botServer := api.NewBotServer(sessionService, nil, nil)

	apiServer := api.NewServer(cfg, sessionService, botServer)

	assert.NotNil(t, apiServer)
}

func TestBotServerCreation(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()

	botServer := api.NewBotServer(sessionService, jwtManager, tokenCache)

	assert.NotNil(t, botServer)
}

func TestLoggerInitialization(t *testing.T) {
	err := logging.Init("info", "console")
	assert.NoError(t, err)
	assert.NotNil(t, logging.Logger)

	defer logging.Sync()
}

func TestLoggerInitialization_InvalidFormat(t *testing.T) {
	err := logging.Init("info", "invalid")
	assert.Error(t, err)
	assert.Nil(t, logging.Logger)
}

func TestLoggerInitialization_InvalidLevel(t *testing.T) {
	err := logging.Init("invalid", "console")
	assert.NoError(t, err)
	assert.NotNil(t, logging.Logger)

	defer logging.Sync()
}

func TestDatabaseConnection(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	assert.NotNil(t, db)
	assert.NotNil(t, db.DB)
}

func TestDatabaseConnection_Close(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	// Verify database is closed
	err = db.DB.Ping()
	assert.Error(t, err)
}

func TestDatabaseConnection_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sqlite.New(ctx, "/tmp/test_aftertalk.db")
	assert.Error(t, err)
}

func TestMainContext(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.New(ctx, "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	assert.NotNil(t, db)
}

func TestMigrationSQL(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	// Test that migration SQL contains expected statements
	sql := runMigrations(context.Background(), db)

	assert.NotEmpty(t, sql)

	// Verify key statements are present
	assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS sessions")
	assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS participants")
	assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS audio_streams")
	assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS transcriptions")
	assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS minutes")
	assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS webhook_events")
	assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS processing_queue")
}

func TestMigrationConstraints(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify CHECK constraints
	var constraint string
	rows, err := db.DB.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name='sessions'")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		rows.Scan(&constraint)
		if constraint != "" {
			assert.Contains(t, constraint, "CHECK (status IN")
			assert.Contains(t, constraint, "CHECK (participant_count")
		}
	}
}

func TestMigrationIndexesOnStatus(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify indexes on status columns
	var indexName string
	rows, err := db.DB.Query("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='sessions'")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		rows.Scan(&indexName)
		assert.True(t, indexName == "idx_sessions_status_created" || indexName == "idx_sessions_status")
	}
}

func TestMigrationIndexesOnDateColumns(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify indexes on date columns
	var indexName string
	rows, err := db.DB.Query("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='sessions'")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		rows.Scan(&indexName)
		assert.Contains(t, indexName, "created")
	}
}

func TestMigrationUniqueConstraints(t *testing.T) {
	db, err := sqlite.New(context.Background(), "/tmp/test_aftertalk.db")
	require.NoError(t, err)
	defer db.Close()

	err = runMigrations(context.Background(), db)
	require.NoError(t, err)

	// Verify unique constraints exist
	var constraint string
	rows, err := db.DB.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name='transcriptions'")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		rows.Scan(&constraint)
		if constraint != "" {
			assert.Contains(t, constraint, "UNIQUE")
		}
	}
}
