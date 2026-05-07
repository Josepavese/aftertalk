package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/api"
	"github.com/Josepavese/aftertalk/internal/api/handler"
	"github.com/Josepavese/aftertalk/internal/bot/webrtc"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/core/minutes"
	"github.com/Josepavese/aftertalk/internal/core/session"
	"github.com/Josepavese/aftertalk/internal/core/transcription"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/internal/storage/cache"
	"github.com/Josepavese/aftertalk/internal/storage/sqlite"
	"github.com/Josepavese/aftertalk/internal/version"
	"github.com/Josepavese/aftertalk/pkg/jwt"
	"github.com/Josepavese/aftertalk/pkg/webhook"
)

func main() {
	// Handle metadata-only flags before loading config so deploy scripts can
	// inspect the binary without DB/provider dependencies.
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" {
			fmt.Println(version.Line("aftertalk")) //nolint:forbidigo // intentional stdout output for CLI --version
			return
		}
		if arg == "--dump-defaults" || arg == "-dump-defaults" {
			out, err := config.DumpYAML()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(out) //nolint:forbidigo // intentional stdout output for CLI --dump-defaults
			return
		}
	}

	configPath := ""
	for i, arg := range os.Args[1:] {
		if (arg == "--config" || arg == "-config") && i+1 < len(os.Args[1:]) {
			configPath = os.Args[i+2]
			break
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	buildInfo := version.Info()
	if err = logging.InitWithOptions(logging.Options{
		Level:   cfg.Logging.Level,
		Format:  cfg.Logging.Format,
		Service: "aftertalk",
		Env:     firstNonEmpty(os.Getenv("AFTERTALK_ENV"), "development"),
		Version: buildInfo.Version,
		Release: buildInfo.Tag,
		Output: logging.OutputOptions{
			Stdout: cfg.Logging.Output.Stdout,
			File: logging.FileOutputOptions{
				Enabled:   cfg.Logging.Output.File.Enabled,
				Path:      cfg.Logging.Output.File.Path,
				Mandatory: cfg.Logging.Output.File.Mandatory,
			},
		},
		Rotation: logging.RotationOptions{
			MaxSizeMB:  cfg.Logging.Rotation.MaxSizeMB,
			MaxAgeDays: cfg.Logging.Rotation.MaxAgeDays,
			MaxBackups: cfg.Logging.Rotation.MaxBackups,
			Compress:   cfg.Logging.Rotation.Compress,
		},
		Retention: logging.RetentionOptions{
			DeleteAfterDays:       cfg.Logging.Retention.DeleteAfterDays,
			EmergencyCutoffSizeMB: cfg.Logging.Retention.EmergencyCutoffSizeMB,
		},
		Redaction: logging.RedactionOptions{
			Enabled: cfg.Logging.Redaction.Enabled,
			Fields:  cfg.Logging.Redaction.Fields,
		},
	}); err != nil {
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
		if closeErr := db.Close(); closeErr != nil {
			logger.Errorf("Failed to close database: %v", closeErr)
		}
	}()

	logger.Infof("Database initialized: %s", cfg.Database.Path)

	if err = runMigrations(ctx, db); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}
	migrateWebhookEvents(ctx, db)         // idempotent: adds payload + next_retry_at columns
	migrateRetrievalTokens(ctx, db)       // idempotent: adds retrieval_tokens table
	migrateWebhookPayloadType(ctx, db)    // idempotent: adds payload_type column to webhook_events
	migrateTranscriptionLanguage(ctx, db) // idempotent: adds language column to transcriptions
	migrateSessionProfiles(ctx, db)       // idempotent: adds stt_profile + llm_profile columns to sessions
	migrateLLMUsageEvents(ctx, db)        // idempotent: records cloud LLM usage/cost telemetry
	logger.Info("Migrations completed")

	if handled, utilityErr := handleUtilityCommand(ctx, db.DB, os.Args[1:]); handled {
		if utilityErr != nil {
			logger.Errorf("Utility command failed: %v", utilityErr)
			fmt.Fprintf(os.Stderr, "error: %v\n", utilityErr)
			os.Exit(1)
		}
		return
	}

	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()
	audioBufferCache := cache.NewAudioBufferCache()

	jwtManager := jwt.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.Expiration)

	sessionRepo := session.NewSessionRepository(db.DB)
	transcriptionRepo := transcription.NewTranscriptionRepository(db.DB)
	minutesRepo := minutes.NewMinutesRepository(db.DB)

	sttRegistry, err := stt.NewSTTRegistry(&cfg.STT)
	if err != nil {
		logger.Fatalf("Failed to initialize STT registry: %v", err)
	}

	// Health-check whisper-local at startup so the operator knows immediately
	// if the Python server is not reachable, rather than discovering it on the
	// first transcription attempt.
	if cfg.STT.Provider == "whisper-local" {
		go checkWhisperHealth(cfg.STT.WhisperLocal.URL)
	}

	retryConfig := stt.DefaultRetryConfig()

	transcriptionService := transcription.NewService(transcriptionRepo, retryConfig)

	llmRegistry, err := llm.NewLLMRegistry(&cfg.LLM)
	if err != nil {
		logger.Fatalf("Failed to initialize LLM registry: %v", err)
	}

	// Resolve delete_on_pull: default true for notify_pull mode.
	deleteOnPull := true
	if cfg.Webhook.DeleteOnPull != nil {
		deleteOnPull = *cfg.Webhook.DeleteOnPull
	}

	minutesService := minutes.NewServiceWithDeps(
		minutesRepo,
		&minutes.RetryConfig{
			MaxRetries:     cfg.Processing.LLMMaxRetries,
			InitialBackoff: cfg.Processing.LLMInitialBackoff,
			MaxBackoff:     cfg.Processing.LLMMaxBackoff,
		},
		&minutes.WebhookConfig{
			URL:          cfg.Webhook.URL,
			Timeout:      cfg.Webhook.Timeout,
			Mode:         cfg.Webhook.Mode,
			Secret:       cfg.Webhook.Secret,
			TokenTTL:     cfg.Webhook.TokenTTL,
			DeleteOnPull: deleteOnPull,
			PullBaseURL:  cfg.Webhook.PullBaseURL,
		},
	)
	minutesService.WithGenerationConfig(minutes.GenerationConfig{
		Incremental:              cfg.Processing.MinutesIncremental,
		DisableFinalVerification: !cfg.Processing.MinutesVerifyFinal,
		BatchMaxSegments:         cfg.Processing.MinutesBatchMaxSegments,
		BatchMaxChars:            cfg.Processing.MinutesBatchMaxChars,
		MaxSummaryPhases:         cfg.Processing.MinutesMaxSummaryPhases,
		MaxCitations:             cfg.Processing.MinutesMaxCitations,
	})

	// Wire persistent webhook retry if a URL is configured.
	// IMPORTANT: the retrier must always be wired when a webhook URL is set.
	// Without it, webhook delivery is fire-and-forget and will be lost on restart.
	if cfg.Webhook.URL != "" {
		timeout := cfg.Webhook.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		var webhookClient *webhook.Client
		if cfg.Webhook.Mode == "notify_pull" && cfg.Webhook.Secret != "" {
			webhookClient = webhook.NewClientWithSecret(cfg.Webhook.URL, cfg.Webhook.Secret, timeout)
		} else {
			webhookClient = webhook.NewClient(cfg.Webhook.URL, timeout)
		}
		retrier := webhook.NewRetrier(db.DB, webhookClient)
		minutesService.WithWebhookRetrier(retrier)
		go retrier.Run(ctx)
		logger.Infof("Webhook retry worker started (url=%s, mode=%s)", cfg.Webhook.URL, cfg.Webhook.Mode)
	}

	transcriptionAdapter := &api.TranscriptionAdapter{Svc: transcriptionService, STTRegistry: sttRegistry}
	minutesAdapter := &api.MinutesAdapter{Svc: minutesService, LLMRegistry: llmRegistry}

	sessionService := session.NewService(
		sessionRepo,
		jwtManager,
		sessionCache,
		tokenCache,
		audioBufferCache,
		transcriptionAdapter,
		minutesAdapter,
		cfg.JWT.Expiration,
		cfg.Processing,
		cfg.Templates,
		cfg.Session,
	)
	sessionService.SetLastActivityProvider(transcriptionRepo)
	sessionService.StartSessionReaper(ctx)
	sessionService.RecoverProcessingSessions(ctx)
	sessionService.RestoreInactivityTimers(ctx)

	// Start embedded TURN server if enabled.
	// When running, automatically appends the TURN server entry to ICEServers
	// so that all clients (botServer, /v1/rtc-config) use it.
	iceServers := cfg.WebRTC.ICEServers
	// Start embedded TURN server if the "embedded" ICE provider is configured.
	var turnServer *webrtc.TURNServer
	if cfg.WebRTC.TURN.Enabled || cfg.WebRTC.ICEProviderName == "embedded" {
		var ts *webrtc.TURNServer
		ts, err = webrtc.StartTURNServer(ctx, cfg.WebRTC.TURN)
		if err != nil {
			logger.Fatalf("Failed to start TURN server: %v", err)
		}
		turnServer = ts
		logger.Infof("TURN server running at %s", ts.Addr())
	}

	// Build the ICE provider (PAL factory — single routing point).
	iceProvider, err := webrtc.NewICEProvider(&cfg.WebRTC, turnServer)
	if err != nil {
		logger.Fatalf("Failed to init ICE provider: %v", err)
	}

	botServer := api.NewBotServer(sessionService, jwtManager, tokenCache, iceServers, cfg.WebRTC.ICEUDPPortMin, cfg.WebRTC.ICEUDPPortMax)

	minutesHandler := handler.NewMinutesHandlerWithConfig(minutesService, deleteOnPull)
	transcriptionHandler := handler.NewTranscriptionHandler(transcriptionService)
	rtcHandler := handler.NewRTCConfigHandler(cfg, iceProvider)
	readyHandler := handler.NewReadyCheck(sttRegistry, llmRegistry)
	apiServer := api.NewServerWithDeps(cfg, sessionService, botServer, minutesHandler, transcriptionHandler, rtcHandler, readyHandler)

	go func() {
		logger.Infof("HTTP server listening on %s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutdown signal received, initiating graceful shutdown...")

	sessionService.Close()

	if err := apiServer.Shutdown(); err != nil {
		logger.Errorf("HTTP server shutdown error: %v", err)
	}

	logger.Info("Graceful shutdown completed")
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
			template_id TEXT NOT NULL DEFAULT '',
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
			template_id TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			content TEXT NOT NULL DEFAULT '{"sections":{},"citations":[]}',
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

		CREATE TABLE IF NOT EXISTS llm_usage_events (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			request_id TEXT NOT NULL DEFAULT '',
			workflow_id TEXT NOT NULL DEFAULT '',
			session_id TEXT NOT NULL DEFAULT '',
			minutes_id TEXT NOT NULL DEFAULT '',
			phase TEXT NOT NULL DEFAULT '',
			batch_index INTEGER NOT NULL DEFAULT 0,
			batch_total INTEGER NOT NULL DEFAULT 0,
			attempt INTEGER NOT NULL DEFAULT 0,
			provider_profile TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			resolved_provider TEXT NOT NULL DEFAULT '',
			resolved_model TEXT NOT NULL DEFAULT '',
			generation_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			http_status INTEGER NOT NULL DEFAULT 0,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			reasoning_tokens INTEGER NOT NULL DEFAULT 0,
			cached_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			cost_credits REAL NOT NULL DEFAULT 0,
			requested_max_tokens INTEGER NOT NULL DEFAULT 0,
			affordable_retry_max_tokens INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			error_class TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_llm_usage_session_created ON llm_usage_events(session_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_llm_usage_minutes_created ON llm_usage_events(minutes_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_llm_usage_model_created ON llm_usage_events(model, created_at);
		
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

// checkWhisperHealth pings the whisper-local server at startup and logs a clear
// warning if it is unreachable, so operators know immediately rather than
// discovering it on the first transcription attempt.
func checkWhisperHealth(baseURL string) {
	if baseURL == "" {
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := strings.TrimRight(baseURL, "/") + "/health"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, healthURL, nil)
	if err != nil {
		logging.Warnf("whisper-local health check: build request: %v", err)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		logging.Warnf("whisper-local server unreachable at %s: %v", baseURL, err)
		logging.Warnf("Transcriptions will fail until whisper_server.py is running (run: aftertalk start)")
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logging.Infof("whisper-local server healthy at %s", baseURL)
	} else {
		logging.Warnf("whisper-local server returned HTTP %d at %s", resp.StatusCode, baseURL)
	}
}

func handleUtilityCommand(ctx context.Context, db *sql.DB, args []string) (bool, error) {
	args = stripGlobalConfigArgs(args)
	if len(args) < 2 || args[0] != "logs" || args[1] != "usage" {
		return false, nil
	}
	fs := flag.NewFlagSet("aftertalk logs usage", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	sessionID := fs.String("session", "", "filter by session_id")
	minutesID := fs.String("minutes", "", "filter by minutes_id")
	model := fs.String("model", "", "filter by requested model")
	profile := fs.String("profile", "", "filter by provider profile")
	fromRaw := fs.String("from", "", "inclusive start date/RFC3339")
	toRaw := fs.String("to", "", "exclusive end date/RFC3339")
	groupBy := fs.String("group-by", "session", "session|day|model|profile")
	if err := fs.Parse(args[2:]); err != nil {
		return true, err
	}
	from, err := parseUtilityTime(*fromRaw)
	if err != nil {
		return true, fmt.Errorf("parse --from: %w", err)
	}
	to, err := parseUtilityTime(*toRaw)
	if err != nil {
		return true, fmt.Errorf("parse --to: %w", err)
	}
	report, err := minutes.NewMinutesRepository(db).ReportLLMUsage(ctx, minutes.LLMUsageFilter{
		From:      from,
		To:        to,
		SessionID: *sessionID,
		MinutesID: *minutesID,
		Model:     *model,
		Profile:   *profile,
	}, *groupBy)
	if err != nil {
		return true, err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return true, enc.Encode(report)
}

func stripGlobalConfigArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" || args[i] == "-config" {
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func parseUtilityTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		utc := t.UTC()
		return &utc, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		utc := t.UTC()
		return &utc, nil
	}
	return nil, fmt.Errorf("expected RFC3339 or YYYY-MM-DD")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// migrateWebhookEvents upgrades the webhook_events table to add the `payload`
// and `next_retry_at` columns required by the Retrier worker.
// Uses ADD COLUMN which is idempotent-safe via a separate error-ignored exec.
func migrateWebhookEvents(ctx context.Context, db *sqlite.DB) {
	upgrades := []string{
		`ALTER TABLE webhook_events ADD COLUMN payload TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE webhook_events ADD COLUMN next_retry_at TEXT`,
	}
	for _, stmt := range upgrades {
		// Ignore "duplicate column" errors (SQLite returns them as generic errors).
		if err := db.RunInTx(ctx, func(tx *sql.Tx) error {
			_, innerErr := tx.ExecContext(ctx, stmt)
			return innerErr
		}); err != nil {
			logging.Warnf("Schema upgrade skipped (column may already exist): %v", err)
		}
	}
}

// migrateRetrievalTokens creates the retrieval_tokens table used by the
// notify_pull secure delivery mode. Idempotent (CREATE TABLE IF NOT EXISTS).
//
// Schema notes:
//   - used_at NULL means the token is unconsumed; set atomically on first pull.
//   - expires_at enforced in ConsumeToken(); expired rows are cleaned up by
//     MinutesRepository.DeleteExpiredTokens() (call periodically, e.g. daily).
//   - ON DELETE CASCADE means tokens are auto-removed when the minutes row is
//     deleted (e.g. by PurgeMinutes after a successful pull).
func migrateRetrievalTokens(ctx context.Context, db *sqlite.DB) {
	if err := db.RunInTx(ctx, func(tx *sql.Tx) error {
		_, innerErr := tx.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS retrieval_tokens (
				id          TEXT PRIMARY KEY,
				minutes_id  TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
				expires_at  TEXT NOT NULL,
				used_at     TEXT,
				created_at  TEXT NOT NULL DEFAULT (datetime('now'))
			);
			CREATE INDEX IF NOT EXISTS idx_retrieval_tokens_minutes ON retrieval_tokens(minutes_id);
			CREATE INDEX IF NOT EXISTS idx_retrieval_tokens_expires ON retrieval_tokens(expires_at);
		`)
		return innerErr
	}); err != nil {
		logging.Warnf("migrateRetrievalTokens: %v", err)
	}
}

// migrateWebhookPayloadType adds the payload_type column to webhook_events,
// needed to distinguish push-mode (MinutesPayload) from notify_pull-mode
// (NotificationPayload) rows in the Retrier worker. Idempotent.
func migrateWebhookPayloadType(ctx context.Context, db *sqlite.DB) {
	// ADD COLUMN returns an error if the column already exists; ignore it.
	if err := db.RunInTx(ctx, func(tx *sql.Tx) error {
		_, innerErr := tx.ExecContext(ctx,
			`ALTER TABLE webhook_events ADD COLUMN payload_type TEXT NOT NULL DEFAULT 'minutes'`)
		return innerErr
	}); err != nil {
		logging.Warnf("migrateWebhookPayloadType: column may already exist: %v", err)
	}
}

// migrateTranscriptionLanguage adds the language column to transcriptions,
// storing the ISO 639-1 code detected by the STT provider. Idempotent.
func migrateTranscriptionLanguage(ctx context.Context, db *sqlite.DB) {
	if err := db.RunInTx(ctx, func(tx *sql.Tx) error {
		_, innerErr := tx.ExecContext(ctx,
			`ALTER TABLE transcriptions ADD COLUMN language TEXT NOT NULL DEFAULT ''`)
		return innerErr
	}); err != nil {
		logging.Warnf("migrateTranscriptionLanguage: column may already exist: %v", err)
	}
}

// migrateSessionProfiles adds stt_profile and llm_profile columns to sessions,
// enabling per-session provider profile selection. Idempotent.
func migrateSessionProfiles(ctx context.Context, db *sqlite.DB) {
	for _, col := range []string{"stt_profile", "llm_profile"} {
		col := col
		if err := db.RunInTx(ctx, func(tx *sql.Tx) error {
			_, innerErr := tx.ExecContext(ctx,
				`ALTER TABLE sessions ADD COLUMN `+col+` TEXT NOT NULL DEFAULT ''`)
			return innerErr
		}); err != nil {
			logging.Warnf("migrateSessionProfiles: %s may already exist: %v", col, err)
		}
	}
}

func migrateLLMUsageEvents(ctx context.Context, db *sqlite.DB) {
	if err := db.RunInTx(ctx, func(tx *sql.Tx) error {
		_, innerErr := tx.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS llm_usage_events (
				id TEXT PRIMARY KEY,
				created_at TEXT NOT NULL DEFAULT (datetime('now')),
				request_id TEXT NOT NULL DEFAULT '',
				workflow_id TEXT NOT NULL DEFAULT '',
				session_id TEXT NOT NULL DEFAULT '',
				minutes_id TEXT NOT NULL DEFAULT '',
				phase TEXT NOT NULL DEFAULT '',
				batch_index INTEGER NOT NULL DEFAULT 0,
				batch_total INTEGER NOT NULL DEFAULT 0,
				attempt INTEGER NOT NULL DEFAULT 0,
				provider_profile TEXT NOT NULL DEFAULT '',
				provider TEXT NOT NULL DEFAULT '',
				model TEXT NOT NULL DEFAULT '',
				resolved_provider TEXT NOT NULL DEFAULT '',
				resolved_model TEXT NOT NULL DEFAULT '',
				generation_id TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT '',
				http_status INTEGER NOT NULL DEFAULT 0,
				prompt_tokens INTEGER NOT NULL DEFAULT 0,
				completion_tokens INTEGER NOT NULL DEFAULT 0,
				reasoning_tokens INTEGER NOT NULL DEFAULT 0,
				cached_tokens INTEGER NOT NULL DEFAULT 0,
				total_tokens INTEGER NOT NULL DEFAULT 0,
				cost_credits REAL NOT NULL DEFAULT 0,
				requested_max_tokens INTEGER NOT NULL DEFAULT 0,
				affordable_retry_max_tokens INTEGER NOT NULL DEFAULT 0,
				duration_ms INTEGER NOT NULL DEFAULT 0,
				error_class TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT ''
			);
			CREATE INDEX IF NOT EXISTS idx_llm_usage_session_created ON llm_usage_events(session_id, created_at);
			CREATE INDEX IF NOT EXISTS idx_llm_usage_minutes_created ON llm_usage_events(minutes_id, created_at);
			CREATE INDEX IF NOT EXISTS idx_llm_usage_model_created ON llm_usage_events(model, created_at);
		`)
		return innerErr
	}); err != nil {
		logging.Warnf("migrateLLMUsageEvents: %v", err)
	}
}
