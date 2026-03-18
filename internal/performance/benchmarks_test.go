package performance

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	_ "github.com/Josepavese/aftertalk/internal/core"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/core/minutes"
	"github.com/Josepavese/aftertalk/internal/core/session"
	"github.com/Josepavese/aftertalk/internal/core/transcription"
	"github.com/Josepavese/aftertalk/internal/storage/cache"
	"github.com/Josepavese/aftertalk/internal/storage/sqlite"
	"github.com/Josepavese/aftertalk/pkg/jwt"
	"github.com/Josepavese/aftertalk/pkg/webhook"
)

var testDBPath string

func init() {
	testDBPath = "/tmp/perf_aftertalk.db"
}

func runMigrations(db *sql.DB) error {
	migrationSQL := `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL CHECK (status IN ('active', 'ended', 'processing', 'completed', 'error')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			ended_at TEXT,
			participant_count INTEGER NOT NULL CHECK (participant_count >= 2),
			template_id TEXT NOT NULL DEFAULT '',
			metadata TEXT,
			stt_profile TEXT NOT NULL DEFAULT '',
			llm_profile TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_sessions_status_created ON sessions(status, created_at);
		CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at);

		CREATE TABLE IF NOT EXISTS participants (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			token_jti TEXT NOT NULL UNIQUE,
			token_expires_at TEXT NOT NULL,
			token_used INTEGER NOT NULL DEFAULT 0,
			connected_at TEXT,
			disconnected_at TEXT,
			UNIQUE(session_id, role)
		);

		CREATE INDEX IF NOT EXISTS idx_participants_session ON participants(session_id);
		CREATE INDEX IF NOT EXISTS idx_participants_token_expires ON participants(token_expires_at);

		CREATE TABLE IF NOT EXISTS transcriptions (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
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
		);

		CREATE INDEX IF NOT EXISTS idx_transcriptions_session ON transcriptions(session_id, start_ms);
		CREATE INDEX IF NOT EXISTS idx_transcriptions_status ON transcriptions(status);

		CREATE TABLE IF NOT EXISTS minutes (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL UNIQUE REFERENCES sessions(id) ON DELETE CASCADE,
			template_id TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			content TEXT NOT NULL DEFAULT '{}',
			generated_at TEXT NOT NULL DEFAULT (datetime('now')),
			delivered_at TEXT,
			status TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'delivered', 'error')),
			provider TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_minutes_status ON minutes(status, generated_at);

		CREATE TABLE IF NOT EXISTS minutes_history (
			id TEXT PRIMARY KEY,
			minutes_id TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
			version INTEGER NOT NULL,
			content TEXT NOT NULL,
			edited_at TEXT NOT NULL DEFAULT (datetime('now')),
			edited_by TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_minutes_history ON minutes_history(minutes_id, version);

		CREATE TABLE IF NOT EXISTS webhook_events (
			id TEXT PRIMARY KEY,
			minutes_id TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
			webhook_url TEXT NOT NULL,
			payload_hash TEXT NOT NULL UNIQUE,
			attempt_number INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
			delivered_at TEXT
		);

		CREATE TABLE IF NOT EXISTS processing_queue (
			id TEXT PRIMARY KEY,
			worker_id TEXT NOT NULL,
			task_type TEXT NOT NULL CHECK (task_type IN ('transcription', 'minutes', 'webhook')),
			payload TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_processing_queue_status ON processing_queue(status, created_at);
		CREATE INDEX IF NOT EXISTS idx_processing_queue_worker ON processing_queue(worker_id);
	`

	_, err := db.ExecContext(context.Background(), migrationSQL)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func setupTestDB(t any) *sql.DB {
	if tb, ok := t.(*testing.T); ok {
		tb.Helper()
	}

	db, err := sqlite.New(context.Background(), testDBPath)
	if err != nil {
		panic("Failed to setup test DB: " + err.Error())
	}

	if err := db.DB.PingContext(context.Background()); err != nil {
		panic("Failed to ping DB: " + err.Error())
	}

	if err := runMigrations(db.DB); err != nil {
		panic("Failed to run migrations: " + err.Error())
	}

	return db.DB
}

func teardownTestDB() {
	os.Remove(testDBPath)
}

type benchStubLLM struct{}

func (benchStubLLM) Generate(_ context.Context, _ string) (string, error) {
	return `{"sections":{},"citations":[]}`, nil
}
func (benchStubLLM) Name() string        { return "stub" }
func (benchStubLLM) IsAvailable() bool   { return true }

func BenchmarkSessionCreation1000(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	repo := session.NewSessionRepository(db)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	service := session.NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := &session.CreateSessionRequest{
			ParticipantCount: 2,
			Participants: []session.ParticipantRequest{
				{UserID: fmt.Sprintf("user%d", i), Role: "host"},
				{UserID: fmt.Sprintf("user%d", i+1), Role: "guest"},
			},
		}

		_, err := service.CreateSession(context.Background(), req)
		if err != nil {
			b.Fatalf("Failed to create session: %v", err)
		}
	}
}

func BenchmarkSessionRetrieval(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	repo := session.NewSessionRepository(db)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	service := session.NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	b.ResetTimer()

	var sessionID string
	for i := 0; i < b.N; i++ {
		sessionID = fmt.Sprintf("session-%d", i%100)
		_, err := service.GetSession(context.Background(), sessionID)
		if err != nil {
			b.Fatalf("Failed to get session: %v", err)
		}
	}
}

func BenchmarkTranscriptionProcessing100(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	repo := transcription.NewTranscriptionRepository(db)

	transcriptionText := "This is a test transcription segment. The application is designed to record and transcribe meetings efficiently."

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sessionID := fmt.Sprintf("session-%d", i%10)
		trans := transcription.NewTranscription(
			randString(36),
			sessionID,
			0,
			"host",
			0,
			1000,
			transcriptionText,
		)
		trans.SetConfidence(0.95)

		if err := repo.Create(context.Background(), trans); err != nil {
			b.Fatalf("Failed to create transcription: %v", err)
		}
	}
}

func BenchmarkTranscriptionRetrieval(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	repo := transcription.NewTranscriptionRepository(db)
	service := transcription.NewService(repo, nil)

	b.ResetTimer()

	var sessionID string
	for i := 0; i < b.N; i++ {
		sessionID = fmt.Sprintf("session-%d", i%10)
		_, err := service.GetTranscriptions(context.Background(), sessionID)
		if err != nil {
			b.Fatalf("Failed to get transcriptions: %v", err)
		}
	}
}

func BenchmarkMinutesGeneration(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	repo := minutes.NewMinutesRepository(db)
	service := minutes.NewService(repo)

	sessionID := "test-session"
	transcriptionText := generateLargeTranscriptionText()
	tmpl := config.DefaultTemplates()[0]

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := service.GenerateMinutes(context.Background(), sessionID, transcriptionText, tmpl, webhook.SessionContext{}, "", &benchStubLLM{})
		if err != nil {
			b.Fatalf("Failed to generate minutes: %v", err)
		}
	}
}

func BenchmarkMinutesRetrieval(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	repo := minutes.NewMinutesRepository(db)
	service := minutes.NewService(repo)

	b.ResetTimer()

	var sessionID string
	for i := 0; i < b.N; i++ {
		sessionID = fmt.Sprintf("session-%d", i%10)
		_, err := service.GetMinutes(context.Background(), sessionID)
		if err != nil {
			b.Fatalf("Failed to get minutes: %v", err)
		}
	}
}

func BenchmarkCacheGetSetDelete(b *testing.B) {
	cache := cache.New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := randString(36)

		cache.Set(key, value, 1*time.Minute)
		_, exists := cache.Get(key)
		if !exists {
			b.Fatalf("Failed to get cached value")
		}

		cache.Delete(key)
		_, exists = cache.Get(key)
		if exists {
			b.Fatalf("Failed to delete cached value")
		}
	}
}

func BenchmarkCacheWithTTL(b *testing.B) {
	cache := cache.New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("ttl-key-%d", i)
		value := randString(36)

		cache.Set(key, value, 1*time.Minute)

		_, exists := cache.Get(key)
		if !exists {
			b.Fatalf("Failed to get cached value immediately")
		}

		time.Sleep(100 * time.Millisecond)
		_, exists = cache.Get(key)
		if exists {
			b.Fatalf("Cached value should have expired")
		}
	}
}

func BenchmarkDatabaseInsert(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sessionID := randString(36)
		_, err := db.ExecContext(context.Background(), 
			"INSERT INTO sessions (id, status, created_at, ended_at, participant_count, metadata) VALUES (?, ?, ?, ?, ?, ?)",
			sessionID, "active", time.Now(), nil, 2, "test metadata",
		)
		if err != nil {
			b.Fatalf("Failed to insert session: %v", err)
		}
	}
}

func BenchmarkDatabaseSelect(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	sessionID := randString(36)
	_, err := db.ExecContext(context.Background(), 
		"INSERT INTO sessions (id, status, created_at, ended_at, participant_count, metadata) VALUES (?, ?, ?, ?, ?, ?)",
		sessionID, "active", time.Now(), nil, 2, "test metadata",
	)
	if err != nil {
		b.Fatalf("Failed to insert session for benchmark: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var status, createdAt string
		err := db.QueryRowContext(context.Background(), "SELECT status, created_at FROM sessions WHERE id = ?", sessionID).Scan(&status, &createdAt)
		if err != nil {
			b.Fatalf("Failed to select session: %v", err)
		}
	}
}

func BenchmarkDatabaseUpdate(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	sessionID := randString(36)
	_, err := db.ExecContext(context.Background(), 
		"INSERT INTO sessions (id, status, created_at, ended_at, participant_count, metadata) VALUES (?, ?, ?, ?, ?, ?)",
		sessionID, "active", time.Now(), nil, 2, "test metadata",
	)
	if err != nil {
		b.Fatalf("Failed to insert session for benchmark: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := db.ExecContext(context.Background(), 
			"UPDATE sessions SET status = ? WHERE id = ?",
			"completed", sessionID,
		)
		if err != nil {
			b.Fatalf("Failed to update session: %v", err)
		}
	}
}

func BenchmarkDatabaseDelete(b *testing.B) {
	db := setupTestDB(b)
	defer teardownTestDB()

	sessionID := randString(36)
	_, err := db.ExecContext(context.Background(), 
		"INSERT INTO sessions (id, status, created_at, ended_at, participant_count, metadata) VALUES (?, ?, ?, ?, ?, ?)",
		sessionID, "active", time.Now(), nil, 2, "test metadata",
	)
	if err != nil {
		b.Fatalf("Failed to insert session for benchmark: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := db.ExecContext(context.Background(), "DELETE FROM sessions WHERE id = ?", sessionID)
		if err != nil {
			b.Fatalf("Failed to delete session: %v", err)
		}
	}
}

func BenchmarkJWTGeneration(b *testing.B) {
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	sessionID := "test-session"
	userID := "test-user"
	role := "host"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := jwtManager.Generate(sessionID, userID, role)
		if err != nil {
			b.Fatalf("Failed to generate JWT: %v", err)
		}
	}
}

func BenchmarkUUIDGeneration(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		uuid := randString(36)
		if len(uuid) != 36 {
			b.Fatalf("Invalid UUID length: %d", len(uuid))
		}
	}
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))] //nolint:gosec // math/rand is fine for test data generation
	}
	return string(b)
}

func generateLargeTranscriptionText() string {
	var text string
	for i := 0; i < 100; i++ {
		text += fmt.Sprintf("Transcription segment %d: This is a test segment for performance benchmarking. ", i)
		if i%5 == 0 {
			text += "The quick brown fox jumps over the lazy dog. "
		}
	}
	return text
}
