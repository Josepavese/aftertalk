package performance

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/flowup/aftertalk/internal/core"
	"github.com/flowup/aftertalk/internal/config"
	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/core/transcription"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/internal/storage/sqlite"
	"github.com/flowup/aftertalk/pkg/jwt"
)

var stressTestDBPath string

func init() {
	rand.Seed(time.Now().UnixNano())
	stressTestDBPath = "/tmp/stress_aftertalk.db"
}

func setupStressTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sqlite.New(context.Background(), stressTestDBPath)
	if err != nil {
		t.Fatalf("Failed to setup stress test DB: %v", err)
	}

	if err := db.DB.Ping(); err != nil {
		t.Fatalf("Failed to ping DB: %v", err)
	}

	if err := runMigrations(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return db.DB
}

func teardownStressTestDB() {
	os.Remove(stressTestDBPath)
}

func TestLongRunningSessions24Hours(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 24-hour session stress test in short mode")
	}
	if os.Getenv("STRESS_TEST") == "" {
		t.Skip("Skipping 24-hour session stress test: set STRESS_TEST=1 to enable")
	}

	duration := 24 * time.Hour

	t.Logf("Testing long-running sessions for %v", duration)

	db := setupStressTestDB(t)
	defer teardownStressTestDB()

	var wg sync.WaitGroup
	results := make(chan error, 10)
	startTime := time.Now()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()

			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					sessionID := randString(36)
					_, err := db.Exec(
						"UPDATE sessions SET participant_count = ? WHERE id = ?",
						2, sessionID,
					)
					if err != nil {
						results <- fmt.Errorf("thread %d: %w", threadID, err)
						return
					}

				case <-time.After(duration):
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(results)

	elapsed := time.Since(startTime)
	totalErrors := 0

	for err := range results {
		if err != nil {
			t.Logf("Error: %v", err)
			totalErrors++
		}
	}

	if totalErrors > 0 {
		t.Errorf("Total errors: %d", totalErrors)
	}

	throughput := float64(elapsed.Seconds()) / 3600
	t.Logf("Average operations per hour: %.2f", throughput)
	t.Logf("Duration: %v", elapsed)
}

func TestHighFrequencySessionCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-frequency session creation test in short mode")
	}

	sessionsPerHour := 1000
	threads := 5

	t.Logf("Testing high-frequency session creation: %d sessions/hour with %d threads", sessionsPerHour, threads)

	db := setupStressTestDB(t)
	defer teardownStressTestDB()

	repo := session.NewSessionRepository(db)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	service := session.NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil)

	sessionCount := sessionsPerHour * 2
	sessionPerThread := sessionCount / threads

	var wg sync.WaitGroup
	results := make(chan error, sessionCount)
	startTime := time.Now()

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()

			for j := 0; j < sessionPerThread; j++ {
				req := &session.CreateSessionRequest{
					ParticipantCount: 2,
					Participants: []session.ParticipantRequest{
						{UserID: fmt.Sprintf("hf-session-thread%d-user%d", threadID, j), Role: "host"},
						{UserID: fmt.Sprintf("hf-session-thread%d-user%d", threadID, j+1), Role: "guest"},
					},
				}

				_, err := service.CreateSession(context.Background(), req)
				if err != nil {
					results <- fmt.Errorf("thread %d: %w", threadID, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(results)

	elapsed := time.Since(startTime)
	totalErrors := 0

	for err := range results {
		if err != nil {
			t.Logf("Error: %v", err)
			totalErrors++
		}
	}

	if totalErrors > 0 {
		t.Errorf("Total errors: %d/%d sessions", totalErrors, sessionCount)
	}

	throughput := float64(sessionCount) / elapsed.Seconds()
	t.Logf("Session creation throughput: %.2f sessions/second", throughput)
	t.Logf("Sessions per hour: %.2f", throughput*3600)
	t.Logf("Total time: %v", elapsed)
}

func TestLargeTranscriptionDataProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large transcription data processing test in short mode")
	}

	sessionCount := 100
	transcriptionSegmentsPerSession := 1000

	t.Logf("Testing large transcription data processing: %d sessions with %d segments each",
		sessionCount, transcriptionSegmentsPerSession)

	db := setupStressTestDB(t)
	defer teardownStressTestDB()

	sessionRepo := session.NewSessionRepository(db)
	transRepo := transcription.NewTranscriptionRepository(db)

	var wg sync.WaitGroup
	results := make(chan error, sessionCount)
	startTime := time.Now()

	for i := 0; i < sessionCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			sessionID := randString(36)

			// Create session
			session := session.NewSession(sessionID, 2, "")
			if err := sessionRepo.Create(context.Background(), session); err != nil {
				results <- fmt.Errorf("session %d: %w", i, err)
				return
			}

			// Generate many transcription segments
			for j := 0; j < transcriptionSegmentsPerSession; j++ {
				trans := transcription.NewTranscription(
					randString(36),
					sessionID,
					j,
					"host",
					j*100,
					(j+1)*100,
					fmt.Sprintf("This is transcription segment %d-%d. ", i, j),
				)
				trans.SetConfidence(0.95)

				if err := transRepo.Create(context.Background(), trans); err != nil {
					results <- fmt.Errorf("transcription %d: %w", j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(results)

	elapsed := time.Since(startTime)
	totalErrors := 0

	for err := range results {
		if err != nil {
			t.Logf("Error: %v", err)
			totalErrors++
		}
	}

	if totalErrors > 0 {
		t.Errorf("Total errors: %d", totalErrors)
	}

	totalSegments := sessionCount * transcriptionSegmentsPerSession
	throughput := float64(totalSegments) / elapsed.Seconds()
	t.Logf("Total transcription segments: %d", totalSegments)
	t.Logf("Throughput: %.2f segments/second", throughput)
	t.Logf("Duration: %v", elapsed)
}

func TestDatabaseWALStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WAL stress test in short mode")
	}
	if os.Getenv("STRESS_TEST") == "" {
		t.Skip("Skipping WAL stress test: set STRESS_TEST=1 to enable")
	}

	operationsPerSecond := 5000
	duration := 60 * time.Second

	t.Logf("Testing WAL stress: %d operations/second for %v", operationsPerSecond, duration)

	db := setupStressTestDB(t)
	defer teardownStressTestDB()

	var wg sync.WaitGroup
	results := make(chan error, operationsPerSecond)
	startTime := time.Now()

	for i := 0; i < operationsPerSecond; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ticker := time.NewTicker(time.Second / time.Duration(operationsPerSecond))
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					sessionID := randString(36)

					// Mix of read and write operations
					if i%3 == 0 {
						_, err := db.Exec(
							"INSERT INTO sessions (id, status, created_at, ended_at, participant_count, metadata) VALUES (?, ?, ?, ?, ?, ?)",
							sessionID, "active", time.Now(), nil, 2, "wal stress",
						)
						if err != nil {
							results <- err
							return
						}
					} else if i%3 == 1 {
						var status string
						err := db.QueryRow("SELECT status FROM sessions WHERE id = ?", sessionID).Scan(&status)
						if err != nil {
							results <- err
							return
						}
					} else {
						_, err := db.Exec(
							"UPDATE sessions SET status = ? WHERE id = ?",
							"active", sessionID,
						)
						if err != nil {
							results <- err
							return
						}
					}

				case <-time.After(duration):
					return
				}
			}
		}()
	}

	wg.Wait()
	close(results)

	elapsed := time.Since(startTime)
	totalErrors := 0

	for err := range results {
		if err != nil {
			t.Logf("Error: %v", err)
			totalErrors++
		}
	}

	if totalErrors > 0 {
		t.Errorf("Total errors: %d", totalErrors)
	}

	throughput := float64(operationsPerSecond*60) / float64(elapsed.Seconds())
	t.Logf("Throughput: %.2f operations/second", throughput)
	t.Logf("Operations: %d", operationsPerSecond*60)
	t.Logf("Duration: %v", elapsed)
}
