package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowup/aftertalk/internal/api"
	"github.com/flowup/aftertalk/internal/config"
	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/internal/storage/sqlite"
	"github.com/flowup/aftertalk/pkg/jwt"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	if err := logging.Init(cfg.Logging.Level, cfg.Logging.Format); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer logging.Sync()

	logger := logging.With("service", "aftertalk")
	logger.Infof("Starting Aftertalk Core...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := sqlite.New(ctx, cfg.Database.Path)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		logger.Info("Closing database connection...")
		if err := db.Close(); err != nil {
			logger.Errorf("Failed to close database: %v", err)
		}
	}()

	logger.Infof("Database initialized: %s", cfg.Database.Path)

	if err := runMigrations(ctx, db); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}
	logger.Info("Migrations completed")

	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	jwtManager := jwt.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.Expiration)

	sessionRepo := session.NewSessionRepository(db.DB)
	sessionService := session.NewService(sessionRepo, jwtManager, sessionCache, tokenCache)

	botServer := api.NewBotServer(sessionService, jwtManager, tokenCache)

	apiServer := api.NewServer(cfg, sessionService, botServer)

	go func() {
		logger.Infof("HTTP server listening on %s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutdown signal received, initiating graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := apiServer.Shutdown(); err != nil {
		logger.Errorf("HTTP server shutdown error: %v", err)
	}

	logger.Info("Graceful shutdown completed")

	_ = shutdownCtx
}

func runMigrations(ctx context.Context, db *sqlite.DB) error {
	return db.RunInTx(ctx, func(tx *sql.Tx) error {
		migrationSQL := `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL CHECK (status IN ('active', 'ended', 'processing', 'completed', 'error')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			ended_at TEXT,
			participant_count INTEGER NOT NULL CHECK (participant_count >= 2),
			metadata TEXT
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
		
		CREATE TABLE IF NOT EXISTS audio_streams (
			id TEXT PRIMARY KEY,
			participant_id TEXT NOT NULL UNIQUE REFERENCES participants(id) ON DELETE CASCADE,
			codec TEXT NOT NULL DEFAULT 'opus',
			sample_rate INTEGER NOT NULL DEFAULT 48000,
			channels INTEGER NOT NULL DEFAULT 1,
			chunk_size_seconds REAL NOT NULL CHECK (chunk_size_seconds BETWEEN 10.0 AND 30.0),
			started_at TEXT NOT NULL,
			ended_at TEXT,
			chunks_received INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL CHECK (status IN ('receiving', 'ended', 'error'))
		);
		
		CREATE INDEX IF NOT EXISTS idx_audio_streams_status ON audio_streams(status, started_at);
		
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
			UNIQUE(session_id, segment_index)
		);
		
		CREATE INDEX IF NOT EXISTS idx_transcriptions_session ON transcriptions(session_id, start_ms);
		CREATE INDEX IF NOT EXISTS idx_transcriptions_status ON transcriptions(status);
		
		CREATE TABLE IF NOT EXISTS minutes (
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
			delivered_at TEXT,
			error_message TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		
		CREATE INDEX IF NOT EXISTS idx_webhook_status ON webhook_events(status, created_at);
		
		CREATE TABLE IF NOT EXISTS processing_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_type TEXT NOT NULL CHECK (job_type IN ('transcription', 'minutes')),
			session_id TEXT NOT NULL,
			payload TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			started_at TEXT,
			completed_at TEXT,
			error_message TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0
		);
		
		CREATE INDEX IF NOT EXISTS idx_processing_queue_status ON processing_queue(status, created_at);
		`
		_, err := tx.ExecContext(ctx, migrationSQL)
		return err
	})
}
