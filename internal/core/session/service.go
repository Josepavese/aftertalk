package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/internal/storage/cache"
	"github.com/Josepavese/aftertalk/pkg/audio"
	"github.com/Josepavese/aftertalk/pkg/jwt"
	"github.com/Josepavese/aftertalk/pkg/webhook"
)

var (
	errAtLeast2Participants     = errors.New("at least 2 participants required")
	errParticipantCountMismatch = errors.New("participant count mismatch")
	errCannotDeleteActive       = errors.New("cannot delete an active session; end it first")
	errTokenNotFoundExpired     = errors.New("token not found or expired")
	errTokenAlreadyUsed         = errors.New("token already used")
	errTokenExpired             = errors.New("token expired")
	errParticipantNotFound      = errors.New("participant not found")
	errTranscriptionQueueFull   = errors.New("transcription queue full")
)

type AudioData struct {
	SessionID     string
	ParticipantID string
	Role          string
	Data          []byte   // raw PCM bytes (int16 LE) or concatenated Opus payloads
	Frames        [][]byte // individual Opus RTP payloads; preferred by whisper-local
	SampleRate    int
	Duration      int
	// OffsetMs: milliseconds from session start to the beginning of this audio chunk.
	// STT providers return segment timestamps relative to the audio file; this offset
	// must be added to convert them to session-absolute timestamps.
	OffsetMs int
	// STTProfile selects the STT provider profile for this audio chunk.
	// Empty string means "use the registry default".
	STTProfile string
}

type TranscriptionServiceInterface interface {
	TranscribeAudio(ctx context.Context, audioData *AudioData) error
	GetTranscriptionsAsText(ctx context.Context, sessionID string) (string, error)
	GetDetectedLanguageForSession(ctx context.Context, sessionID string) string
}

// LastActivityProvider is implemented by the transcription repository and used
// at startup to restore inactivity timers for sessions that survived a restart.
type LastActivityProvider interface {
	GetLastActivityTime(ctx context.Context, sessionID string) (time.Time, error)
}

type MinutesServiceInterface interface {
	// GenerateMinutes calls the LLM to produce structured minutes. sessCtx carries
	// the opaque session metadata and participant list set at session-creation time;
	// it is propagated unchanged to webhook deliveries so recipients can correlate
	// the minutes with their own data model without a second API call.
	// llmProfile selects the LLM provider profile; empty = registry default.
	GenerateMinutes(ctx context.Context, sessionID, transcriptionText string, tmpl config.TemplateConfig, sessCtx webhook.SessionContext, detectedLanguage, llmProfile string) (interface{}, error)
	GetMinutes(ctx context.Context, sessionID string) (interface{}, error)
	MarkSessionError(ctx context.Context, sessionID, templateID, llmProfile string, sessCtx webhook.SessionContext, cause error) error
}

// defaultInactivityTimeout is the fallback when SessionConfig.InactivityTimeout is 0.
const defaultInactivityTimeout = 10 * time.Minute

// OnSessionEndFunc is called synchronously before minutes generation to allow
// callers (e.g. WebRTC manager) to close all peers for the session.
type OnSessionEndFunc func(sessionID string)

type Service struct {
	transcription        TranscriptionServiceInterface
	lastActivityProvider LastActivityProvider
	minutes              MinutesServiceInterface
	templates            map[string]config.TemplateConfig
	repo                 *SessionRepository
	audioBuffer          *cache.AudioBufferCache
	sessionCache         *cache.SessionCache
	jwtManager           *jwt.JWTManager
	transcribeCh         chan *transcribeJob
	onSessionEnd         OnSessionEndFunc
	inactivityTimers     map[string]*time.Timer
	opusDecoder          *audio.OpusDecoder
	pcmConverter         *audio.PCMConverter
	tokenCache           *cache.TokenCache
	wg                   sync.WaitGroup
	// transcribeTracker tracks in-flight transcription jobs by session so that
	// generateMinutesForSession can wait for all DB writes to complete
	// before reading transcriptions without blocking unrelated sessions.
	transcribeTracker *transcriptionTracker
	jwtExpiration     time.Duration
	maxDuration       time.Duration
	inactivityTimeout time.Duration
	minutesTimeout    time.Duration
	chunkSizeMs       int
	inactivityMu      sync.Mutex
	minutesLockMu     sync.Mutex
	minutesLocks      map[string]*minutesLock
	minutesSemaphore  chan struct{}
}

type minutesLock struct {
	mu   sync.Mutex
	refs int
}

type transcriptionTracker struct {
	mu       sync.Mutex
	cond     *sync.Cond
	inFlight map[string]int
}

func newTranscriptionTracker() *transcriptionTracker {
	t := &transcriptionTracker{
		inFlight: make(map[string]int),
	}
	t.cond = sync.NewCond(&t.mu)
	return t
}

func (t *transcriptionTracker) Add(sessionID string) {
	t.mu.Lock()
	t.inFlight[sessionID]++
	t.mu.Unlock()
}

func (t *transcriptionTracker) Done(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.inFlight[sessionID] <= 1 {
		delete(t.inFlight, sessionID)
		t.cond.Broadcast()
		return
	}
	t.inFlight[sessionID]--
}

func (t *transcriptionTracker) Wait(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for t.inFlight[sessionID] > 0 {
		t.cond.Wait()
	}
}

// SetOnSessionEnd registers a callback invoked when any session is ended.
func (s *Service) SetOnSessionEnd(fn OnSessionEndFunc) {
	s.onSessionEnd = fn
}

// SetLastActivityProvider wires the repository used by RestoreInactivityTimers
// to look up the last transcription timestamp for each active session at boot.
func (s *Service) SetLastActivityProvider(p LastActivityProvider) {
	s.lastActivityProvider = p
}

// RestoreInactivityTimers re-arms inactivity timers for sessions that are
// still active in the DB after a process restart. Without this, the timers
// (which are in-memory only) are lost on restart and the session would only
// be closed by the session reaper at MaxDuration — potentially hours later.
//
// For each active session, the last-activity time is the MAX(created_at) of
// its transcriptions. If no transcriptions exist, the session's own created_at
// is used as the baseline. The remaining timer duration is:
//
//	remaining = inactivityTimeout - elapsed_since_last_activity
//
// If elapsed >= inactivityTimeout (session was already overdue at restart),
// EndSession is called immediately in a goroutine.
func (s *Service) RestoreInactivityTimers(ctx context.Context) {
	if s.inactivityTimeout == 0 {
		return
	}
	sessions, _, err := s.repo.List(ctx, string(StatusActive), 0, 0)
	if err != nil {
		logging.Errorf("RestoreInactivityTimers: list active sessions: %v", err)
		return
	}
	if len(sessions) == 0 {
		return
	}
	logging.Infof("RestoreInactivityTimers: restoring timers for %d active session(s)", len(sessions))
	for _, sess := range sessions {
		id := sess.ID
		baseline := sess.CreatedAt

		if s.lastActivityProvider != nil {
			if t, err := s.lastActivityProvider.GetLastActivityTime(ctx, id); err != nil {
				logging.Warnf("RestoreInactivityTimers: session=%s last activity lookup failed: %v", id, err)
			} else if !t.IsZero() {
				baseline = t
			}
		}

		elapsed := time.Since(baseline)
		remaining := s.inactivityTimeout - elapsed
		if remaining <= 0 {
			logging.Infof("RestoreInactivityTimers: session=%s overdue by %s — ending immediately",
				id, (-remaining).Round(time.Second))
			go func() {
				endCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s.EndSession(endCtx, id); err != nil {
					logging.Errorf("RestoreInactivityTimers: EndSession %s: %v", id, err)
				}
			}()
			continue
		}
		logging.Infof("RestoreInactivityTimers: session=%s restoring timer (remaining=%s)",
			id, remaining.Round(time.Second))
		s.inactivityMu.Lock()
		s.inactivityTimers[id] = time.AfterFunc(remaining, func() {
			logging.Infof("Inactivity timeout reached, auto-ending session=%s", id)
			endCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.EndSession(endCtx, id); err != nil {
				logging.Errorf("Auto-end session=%s failed: %v", id, err)
			}
		})
		s.inactivityMu.Unlock()
	}
}

type transcribeJob struct {
	SessionID     string
	ParticipantID string
	Role          string
	AudioData     []byte
	OpusFrames    [][]byte
	DurationMs    int
	// OffsetMs: milliseconds from session start to the beginning of this audio chunk.
	// Added to STT segment timestamps before storing, so all transcriptions share
	// the same absolute timeline regardless of when each participant connected.
	OffsetMs   int
	STTProfile string // provider profile to use; empty = registry default
}

func NewService(
	repo *SessionRepository,
	jwtManager *jwt.JWTManager,
	sessionCache *cache.SessionCache,
	tokenCache *cache.TokenCache,
	audioBuffer *cache.AudioBufferCache,
	transcription TranscriptionServiceInterface,
	minutes MinutesServiceInterface,
	jwtExpiration time.Duration,
	processingCfg config.ProcessingConfig,
	templates []config.TemplateConfig,
	sessionCfg config.SessionConfig,
) *Service {
	tmplMap := make(map[string]config.TemplateConfig, len(templates))
	for _, t := range templates {
		tmplMap[t.ID] = t
	}
	if jwtExpiration <= 0 {
		jwtExpiration = 2 * time.Hour
	}
	inactivityTimeout := sessionCfg.InactivityTimeout
	if inactivityTimeout <= 0 {
		inactivityTimeout = defaultInactivityTimeout
	}
	minutesTimeout := processingCfg.MinutesGenerationTimeout
	if minutesTimeout <= 0 {
		minutesTimeout = 5 * time.Minute
	}
	maxMinutes := processingCfg.MaxConcurrentMinutesGenerations
	if maxMinutes <= 0 {
		maxMinutes = 1
	}
	s := &Service{
		repo:              repo,
		jwtManager:        jwtManager,
		sessionCache:      sessionCache,
		tokenCache:        tokenCache,
		audioBuffer:       audioBuffer,
		transcription:     transcription,
		minutes:           minutes,
		jwtExpiration:     jwtExpiration,
		maxDuration:       sessionCfg.MaxDuration,
		inactivityTimeout: inactivityTimeout,
		minutesTimeout:    minutesTimeout,
		transcribeCh:      make(chan *transcribeJob, processingCfg.TranscriptionQueueSize),
		chunkSizeMs:       processingCfg.ChunkSizeMs,
		opusDecoder:       audio.NewOpusDecoder(48000, 1),
		pcmConverter:      audio.NewPCMConverter(16000, 1),
		inactivityTimers:  make(map[string]*time.Timer),
		transcribeTracker: newTranscriptionTracker(),
		minutesLocks:      make(map[string]*minutesLock),
		minutesSemaphore:  make(chan struct{}, maxMinutes),
		templates:         tmplMap,
	}

	if transcription != nil {
		s.wg.Add(1)
		go s.processTranscriptionQueue()
	}

	return s
}

// RecoverProcessingSessions re-triggers minutes generation for sessions that
// were left in status='processing' by a previous process that was killed or
// crashed mid-flight. Call this once at startup, after the service is fully wired.
//
// The recovery is safe to run on every boot: processRemainingAudio reads from
// the in-memory audio buffer (empty after restart, so it is a no-op), and
// generateMinutesForSession is idempotent — if a minutes record already exists
// for the session it will not create a duplicate.
func (s *Service) RecoverProcessingSessions(ctx context.Context) {
	sessions, _, err := s.repo.List(ctx, string(StatusProcessing), 0, 0)
	if err != nil {
		logging.Errorf("RecoverProcessingSessions: failed to list processing sessions: %v", err)
		return
	}
	if len(sessions) == 0 {
		return
	}
	logging.Warnf("RecoverProcessingSessions: found %d session(s) stuck in 'processing' — re-triggering minutes generation", len(sessions))
	for _, sess := range sessions {
		id := sess.ID
		if s.isProcessingExpired(sess) {
			logging.Errorf("RecoverProcessingSessions: session %s exceeded minutes timeout; marking error", id)
			s.failSessionWithCause(sess, fmt.Errorf("stuck processing exceeded timeout %s", s.minutesTimeout))
			continue
		}
		go func() {
			s.processRemainingAudio(id)
			s.generateMinutesForSession(id)
		}()
	}
}

func (s *Service) isProcessingExpired(sess *Session) bool {
	if s.minutesTimeout <= 0 {
		return false
	}
	start := sess.CreatedAt
	if sess.EndedAt != nil {
		start = *sess.EndedAt
	}
	return time.Since(start) > s.minutesTimeout
}

// reaperSweepInterval is how often the reaper checks for expired sessions.
const reaperSweepInterval = 5 * time.Minute

// StartSessionReaper starts a background goroutine that periodically closes
// sessions that have exceeded cfg.Session.MaxDuration. It is a no-op when
// MaxDuration is 0 (disabled). The goroutine exits when ctx is canceled.
//
// Call this once from main.go after wiring the service.
func (s *Service) StartSessionReaper(ctx context.Context) {
	if s.maxDuration == 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(reaperSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.reapExpiredSessions(ctx)
			}
		}
	}()
}

func (s *Service) reapExpiredSessions(ctx context.Context) {
	sessions, err := s.repo.ListActive(ctx)
	if err != nil {
		logging.Errorf("session reaper: list active sessions: %v", err)
		return
	}
	for _, sess := range sessions {
		if time.Since(sess.CreatedAt) > s.maxDuration {
			logging.Infof("session reaper: auto-closing session %s (age %s > max_duration %s)",
				sess.ID, time.Since(sess.CreatedAt).Round(time.Second), s.maxDuration)
			if err := s.EndSession(ctx, sess.ID); err != nil {
				logging.Errorf("session reaper: EndSession %s: %v", sess.ID, err)
			}
		}
	}
}

func (s *Service) processTranscriptionQueue() {
	defer s.wg.Done()

	for job := range s.transcribeCh {
		func() {
			defer s.transcribeTracker.Done(job.SessionID) // signal completion after DB write

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// When Opus frames are available skip PCM conversion (OGG muxing is done
			// inside the STT provider). Only convert when no frames are present.
			var pcmData []byte
			if len(job.OpusFrames) == 0 {
				var err error
				pcmData, err = s.convertToPCM16k(job.AudioData)
				if err != nil {
					logging.Errorf("Failed to convert audio to PCM for session=%s participant=%s: %v",
						job.SessionID, job.ParticipantID, err)
					return
				}
			}

			audioData := &AudioData{
				SessionID:     job.SessionID,
				ParticipantID: job.ParticipantID,
				Role:          job.Role,
				Data:          pcmData,
				Frames:        job.OpusFrames,
				SampleRate:    16000,
				Duration:      job.DurationMs,
				OffsetMs:      job.OffsetMs,
				STTProfile:    job.STTProfile,
			}

			if err := s.transcription.TranscribeAudio(ctx, audioData); err != nil {
				logging.Errorf("Failed to transcribe audio for session=%s participant=%s: %v",
					job.SessionID, job.ParticipantID, err)
			} else {
				logging.Infof("Transcription completed for session=%s participant=%s",
					job.SessionID, job.ParticipantID)
			}

			s.audioBuffer.ClearBuffer(job.SessionID, job.ParticipantID)
		}()
	}
}

func (s *Service) convertToPCM16k(opusData []byte) ([]byte, error) {
	pcmData, err := s.opusDecoder.Decode(opusData)
	if err != nil {
		return nil, fmt.Errorf("opus decode failed: %w", err)
	}

	resampled := s.pcmConverter.ConvertToInt16(s.pcmConverter.ConvertToFloat32(pcmData))

	out := make([]byte, len(resampled)*2)
	for i, sample := range resampled {
		out[i*2] = byte(sample)        //nolint:gosec // intentional little-endian PCM byte extraction
		out[i*2+1] = byte(sample >> 8) //nolint:gosec // intentional little-endian PCM byte extraction
	}

	return out, nil
}

func (s *Service) Close() {
	close(s.transcribeCh)
	s.wg.Wait()
}

// resetInactivityTimer restarts the inactivity countdown for a session.
// Called each time an audio chunk arrives. When the timer fires, the session
// is automatically ended and minutes are generated.
func (s *Service) resetInactivityTimer(sessionID string) {
	s.inactivityMu.Lock()
	defer s.inactivityMu.Unlock()

	if t, ok := s.inactivityTimers[sessionID]; ok {
		t.Reset(s.inactivityTimeout)
		return
	}
	s.inactivityTimers[sessionID] = time.AfterFunc(s.inactivityTimeout, func() {
		logging.Infof("Inactivity timeout reached, auto-ending session=%s", sessionID)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.EndSession(ctx, sessionID); err != nil {
			logging.Errorf("Auto-end session=%s failed: %v", sessionID, err)
		}
	})
}

// cancelInactivityTimer stops and removes the inactivity timer for a session.
func (s *Service) cancelInactivityTimer(sessionID string) {
	s.inactivityMu.Lock()
	defer s.inactivityMu.Unlock()

	if t, ok := s.inactivityTimers[sessionID]; ok {
		t.Stop()
		delete(s.inactivityTimers, sessionID)
	}
}

type CreateSessionRequest struct {
	TemplateID       string               `json:"template_id,omitempty"`
	Metadata         string               `json:"metadata,omitempty"`
	Participants     []ParticipantRequest `json:"participants"`
	ParticipantCount int                  `json:"participant_count"`
	// STTProfile and LLMProfile select provider profiles defined in the server config.
	// Leave empty to use the server's configured default_profile.
	STTProfile string `json:"stt_profile,omitempty"`
	LLMProfile string `json:"llm_profile,omitempty"`
}

type ParticipantRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type CreateSessionResponse struct {
	SessionID    string                `json:"session_id"`
	Participants []ParticipantResponse `json:"participants"`
}

type ParticipantResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func (s *Service) CreateSession(ctx context.Context, req *CreateSessionRequest) (*CreateSessionResponse, error) {
	if req.ParticipantCount < 2 {
		return nil, errAtLeast2Participants
	}

	if len(req.Participants) != req.ParticipantCount {
		return nil, errParticipantCountMismatch
	}

	sessionID := uuid.New().String()
	session := NewSession(sessionID, req.ParticipantCount, req.TemplateID, req.STTProfile, req.LLMProfile)
	session.Metadata = req.Metadata

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	responses := make([]ParticipantResponse, 0, len(req.Participants))
	for _, p := range req.Participants {
		participantID := uuid.New().String()
		tokenExpiresAt := time.Now().Add(s.jwtExpiration)

		token, tokenJTI, err := s.jwtManager.Generate(sessionID, p.UserID, p.Role)
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}

		participant := NewParticipant(participantID, sessionID, p.UserID, p.Role, tokenJTI, tokenExpiresAt)

		if err := s.repo.CreateParticipant(ctx, participant); err != nil {
			return nil, fmt.Errorf("failed to create participant: %w", err)
		}

		s.tokenCache.SetToken(tokenJTI, sessionID, s.jwtExpiration)

		responses = append(responses, ParticipantResponse{
			ID:        participantID,
			UserID:    p.UserID,
			Role:      p.Role,
			Token:     token,
			ExpiresAt: tokenExpiresAt.Format(time.RFC3339),
		})
	}

	s.sessionCache.SetSession(sessionID, &cache.SessionState{
		SessionID:          sessionID,
		Status:             string(StatusActive),
		StartedAt:          session.CreatedAt,
		ParticipantCount:   session.ParticipantCount,
		ActiveParticipants: 0,
		STTProfile:         session.STTProfile,
		LLMProfile:         session.LLMProfile,
	}, s.jwtExpiration)

	return &CreateSessionResponse{
		SessionID:    sessionID,
		Participants: responses,
	}, nil
}

func (s *Service) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// ListSessions returns a paginated list of sessions.
// status="" returns all statuses. limit=0 returns all.
func (s *Service) ListSessions(ctx context.Context, status string, limit, offset int) ([]*Session, int, error) {
	return s.repo.List(ctx, status, limit, offset)
}

// DeleteSession removes a session and its participants/streams from the DB.
// Returns an error if the session is still active.
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	sess, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	if sess.Status == StatusActive {
		return errCannotDeleteActive
	}
	s.cancelInactivityTimer(sessionID)
	return s.repo.Delete(ctx, sessionID)
}

func (s *Service) EndSession(ctx context.Context, sessionID string) error {
	// Cancel the inactivity timer (handles both manual and auto-triggered ends).
	s.cancelInactivityTimer(sessionID)

	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Idempotent: don't re-process an already-ended session.
	if session.Status != StatusActive {
		return nil
	}

	session.End()
	session.StartProcessing()

	if err := s.repo.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Close all WebRTC peers for this session so every connected participant
	// is disconnected immediately and their audio buffers are fully flushed.
	if s.onSessionEnd != nil {
		s.onSessionEnd(sessionID)
	}

	go func() { //nolint:contextcheck // post-session processing must outlive the request context
		s.processRemainingAudio(sessionID)
		s.generateMinutesForSession(sessionID)
	}()

	return nil
}

// RegenerateSession retries minutes generation for an ended/completed/error
// session using the already persisted transcriptions. It does not duplicate a
// ready minutes row: minutes.Service is idempotent and only resets error rows.
func (s *Service) RegenerateSession(ctx context.Context, sessionID string) error {
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if session.Status == StatusActive {
		return fmt.Errorf("cannot regenerate active session %s; end it first", sessionID) //nolint:err113
	}
	session.StartProcessing()
	if err := s.repo.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to mark session processing: %w", err)
	}
	go s.generateMinutesForSession(sessionID) //nolint:contextcheck // regeneration continues after request returns
	return nil
}

func (s *Service) processRemainingAudio(sessionID string) {
	if s.audioBuffer == nil {
		return
	}
	buffers := s.audioBuffer.GetAllParticipantBuffers(sessionID)

	for participantID, buffer := range buffers {
		if len(buffer.Frames) == 0 && len(buffer.Data) == 0 {
			continue
		}

		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// When individual Opus frames are available pass them directly to the STT
			// provider (which handles OGG muxing). Only fall back to PCM conversion
			// for providers that require raw PCM and have no frames.
			var pcmData []byte
			if len(buffer.Frames) == 0 {
				var err error
				pcmData, err = s.convertToPCM16k(buffer.Data)
				if err != nil {
					logging.Errorf("Failed to convert remaining audio for session=%s participant=%s: %v",
						sessionID, participantID, err)
					return
				}
			}

			var offsetMs int
			if state, ok := s.sessionCache.GetSession(sessionID); ok && !state.StartedAt.IsZero() {
				offsetMs = int(buffer.StartTime.Sub(state.StartedAt).Milliseconds())
				if offsetMs < 0 {
					offsetMs = 0
				}
			}

			var sttProfileRemain string
			if state, ok := s.sessionCache.GetSession(sessionID); ok {
				sttProfileRemain = state.STTProfile
			}

			audioData := &AudioData{
				SessionID:     sessionID,
				ParticipantID: participantID,
				Role:          buffer.Role,
				Data:          pcmData,
				Frames:        buffer.Frames,
				SampleRate:    16000,
				Duration:      buffer.DurationMs,
				OffsetMs:      offsetMs,
				STTProfile:    sttProfileRemain,
			}

			if err := s.transcription.TranscribeAudio(ctx, audioData); err != nil {
				logging.Errorf("Failed to transcribe remaining audio for session=%s participant=%s: %v",
					sessionID, participantID, err)
			}

			s.audioBuffer.ClearBuffer(sessionID, participantID)
		}()
	}
}

func (s *Service) generateMinutesForSession(sessionID string) {
	unlock := s.acquireMinutesLock(sessionID)
	defer unlock()
	if s.minutesSemaphore != nil {
		s.minutesSemaphore <- struct{}{}
		defer func() { <-s.minutesSemaphore }()
	}

	// Wait for this session's in-flight transcription jobs to finish writing to
	// DB before reading them. This avoids cross-session blocking while preserving
	// strict sequencing for the session being finalized.
	s.transcribeTracker.Wait(sessionID)

	ctx, cancel := context.WithTimeout(context.Background(), s.minutesTimeout)
	defer cancel()

	var transcriptionText string
	var detectedLanguage string
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		logging.Errorf("Failed to get session for minutes=%s: %v", sessionID, err)
		return
	}

	if s.transcription != nil {
		transcriptionText, err = s.transcription.GetTranscriptionsAsText(ctx, sessionID)
		if err != nil {
			logging.Errorf("Failed to get transcriptions for session=%s: %v", sessionID, err)
			s.failSessionWithCause(session, err)
			return
		}
		if transcriptionText == "" {
			logging.Warnf("No transcriptions found for session=%s", sessionID)
		}
		detectedLanguage = s.transcription.GetDetectedLanguageForSession(ctx, sessionID)
		if detectedLanguage != "" {
			logging.Infof("Detected language for session=%s: %s", sessionID, detectedLanguage)
		}
	}

	tmpl, ok := s.templates[session.TemplateID]
	if !ok {
		// Fallback: use first available template or an empty one.
		for _, t := range s.templates {
			tmpl = t
			ok = true
			break
		}
		if !ok {
			tmpl = config.TemplateConfig{ID: "default", Name: "Default"}
		}
		logging.Warnf("Template %q not found for session=%s, using %q", session.TemplateID, sessionID, tmpl.ID)
	}

	// Build the session context that will be propagated to webhook payloads.
	// Participants are loaded here so webhook recipients can correlate the delivery
	// with their own data model (e.g. appointment_id, doctor_id) without a second call.
	sessCtx := webhook.SessionContext{Metadata: session.Metadata}
	if participants, pErr := s.repo.GetParticipantsBySession(ctx, sessionID); pErr != nil {
		logging.Warnf("generateMinutesForSession: could not load participants for session=%s: %v", sessionID, pErr)
	} else {
		summaries := make([]webhook.ParticipantSummary, len(participants))
		for i, p := range participants {
			summaries[i] = webhook.ParticipantSummary{UserID: p.UserID, Role: p.Role}
		}
		sessCtx.Participants = summaries
	}

	if s.minutes != nil {
		_, err = s.minutes.GenerateMinutes(ctx, sessionID, transcriptionText, tmpl, sessCtx, detectedLanguage, session.LLMProfile)
		if err != nil {
			logging.Errorf("Failed to generate minutes for session=%s: %v", sessionID, err)
			s.failSessionWithContext(session, tmpl.ID, sessCtx, err)
			return
		}
	}

	session.Complete()
	if err := s.repo.Update(ctx, session); err != nil {
		logging.Errorf("Failed to mark session as completed: %v", err)
	}
}

func (s *Service) failSessionWithCause(sess *Session, cause error) {
	s.failSessionWithContext(sess, sess.TemplateID, webhook.SessionContext{Metadata: sess.Metadata}, cause)
}

func (s *Service) failSessionWithContext(sess *Session, templateID string, sessCtx webhook.SessionContext, cause error) {
	if sess == nil {
		return
	}
	failCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess.Fail()
	if updateErr := s.repo.Update(failCtx, sess); updateErr != nil {
		logging.Errorf("generateMinutesForSession: failed to mark session %s as error: %v", sess.ID, updateErr)
	}
	if s.minutes != nil {
		if err := s.minutes.MarkSessionError(failCtx, sess.ID, templateID, sess.LLMProfile, sessCtx, cause); err != nil {
			logging.Errorf("generateMinutesForSession: failed to mark minutes for session %s as error: %v", sess.ID, err)
		}
	}
}

func (s *Service) acquireMinutesLock(sessionID string) func() {
	s.minutesLockMu.Lock()
	lock := s.minutesLocks[sessionID]
	if lock == nil {
		lock = &minutesLock{}
		s.minutesLocks[sessionID] = lock
	}
	lock.refs++
	s.minutesLockMu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		s.minutesLockMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(s.minutesLocks, sessionID)
		}
		s.minutesLockMu.Unlock()
	}
}

func (s *Service) ValidateParticipant(ctx context.Context, jti string) (*Participant, error) {
	if _, exists := s.tokenCache.GetToken(jti); !exists {
		participant, err := s.repo.GetParticipantByJTI(ctx, jti)
		if err != nil || participant == nil {
			return nil, errTokenNotFoundExpired
		}
		s.tokenCache.SetToken(jti, participant.SessionID, s.jwtExpiration)
	}

	participant, err := s.repo.GetParticipantByJTI(ctx, jti)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}

	if participant.TokenUsed {
		return nil, errTokenAlreadyUsed
	}

	if !participant.IsTokenValid() {
		return nil, errTokenExpired
	}

	return participant, nil
}

func (s *Service) ConnectParticipant(ctx context.Context, participantID string) error {
	participant, err := s.repo.GetParticipantByID(ctx, participantID)
	if err != nil {
		return err
	}

	sess, err := s.repo.GetByID(ctx, participant.SessionID)
	if err != nil {
		return fmt.Errorf("failed to get participant session: %w", err)
	}
	if sess.Status != StatusActive {
		return fmt.Errorf("session is not active: %s", sess.Status) //nolint:err113
	}

	participant.Connect()

	if err := s.repo.UpdateParticipant(ctx, participant); err != nil {
		return fmt.Errorf("failed to update participant: %w", err)
	}

	chunkSizeSecs := float64(s.chunkSizeMs) / 1000.0
	stream := NewAudioStream(uuid.New().String(), participant.ID, chunkSizeSecs)
	if err := s.repo.CreateAudioStream(ctx, stream); err != nil {
		logging.Errorf("Failed to create audio stream: %v", err)
	}

	if state, exists := s.sessionCache.GetSession(participant.SessionID); exists {
		state.ActiveParticipants++
		s.sessionCache.SetSession(participant.SessionID, state, s.jwtExpiration)
	}

	return nil
}

func (s *Service) ProcessAudioChunk(sessionID, participantID string, payload []byte) error {
	participant, err := s.repo.GetParticipantByJTI(context.Background(), participantID)
	if err != nil {
		return fmt.Errorf("participant not found: %w", err)
	}

	// WebRTC Opus uses 20ms frames — the RTP payload is compressed Opus,
	// not raw PCM, so its byte size cannot be used to estimate duration.
	const chunkDurationMs = 20 // ms per RTP packet (standard WebRTC Opus frame size)

	logging.Debugf("Processing audio chunk for session=%s participant=%s bytes=%d",
		sessionID, participantID, len(payload))

	buffer, _ := s.audioBuffer.AppendToBuffer(sessionID, participantID, participant.Role, payload, chunkDurationMs)

	// Reset the inactivity countdown whenever audio arrives.
	s.resetInactivityTimer(sessionID)

	if audio.ShouldFlushOnSilence(buffer.Frames, buffer.DurationMs) {
		logging.Infof("Triggering transcription (VAD) for session=%s participant=%s bufferDuration=%dms silentFrame=%v",
			sessionID, participantID, buffer.DurationMs, audio.IsOpusSilentFrame(payload))

		framesCopy := make([][]byte, len(buffer.Frames))
		for i, f := range buffer.Frames {
			framesCopy[i] = make([]byte, len(f))
			copy(framesCopy[i], f)
		}

		// Compute session-absolute offset for this chunk.
		// buffer.StartTime is the wall-clock time the buffer started accumulating.
		// session.StartedAt (from cache) is the wall-clock time the session was created.
		// offsetMs = time elapsed from session start to beginning of this audio chunk.
		var offsetMs int
		if state, ok := s.sessionCache.GetSession(sessionID); ok && !state.StartedAt.IsZero() {
			offsetMs = int(buffer.StartTime.Sub(state.StartedAt).Milliseconds())
			if offsetMs < 0 {
				offsetMs = 0
			}
		}

		var sttProfile string
		if state, ok := s.sessionCache.GetSession(sessionID); ok {
			sttProfile = state.STTProfile
		}

		job := &transcribeJob{
			SessionID:     sessionID,
			ParticipantID: participantID,
			Role:          participant.Role,
			AudioData:     make([]byte, len(buffer.Data)),
			OpusFrames:    framesCopy,
			DurationMs:    buffer.DurationMs,
			OffsetMs:      offsetMs,
			STTProfile:    sttProfile,
		}
		copy(job.AudioData, buffer.Data)

		s.transcribeTracker.Add(sessionID)
		select {
		case s.transcribeCh <- job:
			s.audioBuffer.ClearBuffer(sessionID, participantID)
		default:
			s.transcribeTracker.Done(sessionID)
			logging.Warnf("Transcription queue full, keeping buffered chunk for retry session=%s participant=%s", sessionID, participantID)
			return errTranscriptionQueueFull
		}
	}

	stream, err := s.repo.GetAudioStreamByParticipant(context.Background(), participantID)
	if err == nil && stream != nil {
		stream.AddChunk()
		if updateErr := s.repo.UpdateAudioStream(context.Background(), stream); updateErr != nil {
			logging.Errorf("Failed to update audio stream: %v", updateErr)
		}
	}

	return nil
}
