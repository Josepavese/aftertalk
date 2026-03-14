package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/internal/storage/cache"
	"github.com/Josepavese/aftertalk/pkg/audio"
	"github.com/Josepavese/aftertalk/pkg/jwt"
	"github.com/google/uuid"
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
}

type TranscriptionServiceInterface interface {
	TranscribeAudio(ctx context.Context, audioData *AudioData) error
	GetTranscriptionsAsText(ctx context.Context, sessionID string) (string, error)
}

type MinutesServiceInterface interface {
	GenerateMinutes(ctx context.Context, sessionID string, transcriptionText string, tmpl config.TemplateConfig) (interface{}, error)
	GetMinutes(ctx context.Context, sessionID string) (interface{}, error)
}

// InactivityTimeout is the period of silence after which a session is
// automatically ended and minutes are generated.
const InactivityTimeout = 10 * time.Minute

// OnSessionEndFunc is called synchronously before minutes generation to allow
// callers (e.g. WebRTC manager) to close all peers for the session.
type OnSessionEndFunc func(sessionID string)

type Service struct {
	repo          *SessionRepository
	jwtManager    *jwt.JWTManager
	sessionCache  *cache.SessionCache
	tokenCache    *cache.TokenCache
	audioBuffer   *cache.AudioBufferCache
	transcription TranscriptionServiceInterface
	minutes       MinutesServiceInterface
	transcribeCh  chan *transcribeJob
	chunkSizeMs   int
	wg            sync.WaitGroup
	opusDecoder   *audio.OpusDecoder
	pcmConverter  *audio.PCMConverter
	templates     map[string]config.TemplateConfig // templateID → template

	inactivityMu     sync.Mutex
	inactivityTimers map[string]*time.Timer // sessionID → timer

	onSessionEnd OnSessionEndFunc // optional hook called when a session ends
}

// SetOnSessionEnd registers a callback invoked when any session is ended.
func (s *Service) SetOnSessionEnd(fn OnSessionEndFunc) {
	s.onSessionEnd = fn
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
	OffsetMs int
}

func NewService(
	repo *SessionRepository,
	jwtManager *jwt.JWTManager,
	sessionCache *cache.SessionCache,
	tokenCache *cache.TokenCache,
	audioBuffer *cache.AudioBufferCache,
	transcription TranscriptionServiceInterface,
	minutes MinutesServiceInterface,
	processingCfg config.ProcessingConfig,
	templates []config.TemplateConfig,
) *Service {
	tmplMap := make(map[string]config.TemplateConfig, len(templates))
	for _, t := range templates {
		tmplMap[t.ID] = t
	}
	s := &Service{
		repo:             repo,
		jwtManager:       jwtManager,
		sessionCache:     sessionCache,
		tokenCache:       tokenCache,
		audioBuffer:      audioBuffer,
		transcription:    transcription,
		minutes:          minutes,
		transcribeCh:     make(chan *transcribeJob, processingCfg.TranscriptionQueueSize),
		chunkSizeMs:      processingCfg.ChunkSizeMs,
		opusDecoder:      audio.NewOpusDecoder(48000, 1),
		pcmConverter:     audio.NewPCMConverter(16000, 1),
		inactivityTimers: make(map[string]*time.Timer),
		templates:        tmplMap,
	}

	if transcription != nil {
		s.wg.Add(1)
		go s.processTranscriptionQueue()
	}

	return s
}

func (s *Service) processTranscriptionQueue() {
	defer s.wg.Done()

	for job := range s.transcribeCh {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

		// When Opus frames are available skip PCM conversion (OGG muxing is done
		// inside the STT provider). Only convert when no frames are present.
		var pcmData []byte
		if len(job.OpusFrames) == 0 {
			var err error
			pcmData, err = s.convertToPCM16k(job.AudioData)
			if err != nil {
				logging.Errorf("Failed to convert audio to PCM for session=%s participant=%s: %v",
					job.SessionID, job.ParticipantID, err)
				cancel()
				continue
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
		}

		if err := s.transcription.TranscribeAudio(ctx, audioData); err != nil {
			logging.Errorf("Failed to transcribe audio for session=%s participant=%s: %v",
				job.SessionID, job.ParticipantID, err)
		} else {
			logging.Infof("Transcription completed for session=%s participant=%s",
				job.SessionID, job.ParticipantID)
		}

		s.audioBuffer.ClearBuffer(job.SessionID, job.ParticipantID)
		cancel()
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
		out[i*2] = byte(sample)
		out[i*2+1] = byte(sample >> 8)
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
		t.Reset(InactivityTimeout)
		return
	}
	s.inactivityTimers[sessionID] = time.AfterFunc(InactivityTimeout, func() {
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
	ParticipantCount int                  `json:"participant_count"`
	Participants     []ParticipantRequest `json:"participants"`
	TemplateID       string               `json:"template_id,omitempty"`
	Metadata         string               `json:"metadata,omitempty"`
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
		return nil, fmt.Errorf("at least 2 participants required")
	}

	if len(req.Participants) != req.ParticipantCount {
		return nil, fmt.Errorf("participant count mismatch")
	}

	sessionID := uuid.New().String()
	session := NewSession(sessionID, req.ParticipantCount, req.TemplateID)
	session.Metadata = req.Metadata

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	responses := make([]ParticipantResponse, 0, len(req.Participants))
	for _, p := range req.Participants {
		participantID := uuid.New().String()
		tokenJTI := uuid.New().String()
		tokenExpiresAt := time.Now().Add(2 * time.Hour)

		token, tokenJTI, err := s.jwtManager.Generate(sessionID, p.UserID, p.Role)
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}

		participant := NewParticipant(participantID, sessionID, p.UserID, p.Role, tokenJTI, tokenExpiresAt)

		if err := s.repo.CreateParticipant(ctx, participant); err != nil {
			return nil, fmt.Errorf("failed to create participant: %w", err)
		}

		s.tokenCache.SetToken(tokenJTI, sessionID, 2*time.Hour)

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
	}, 2*time.Hour)

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
		return fmt.Errorf("cannot delete an active session; end it first")
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

	go func() {
		s.processRemainingAudio(sessionID)
		s.generateMinutesForSession(sessionID)
	}()

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
				continue
			}
		}

		var offsetMs int
		if state, ok := s.sessionCache.GetSession(sessionID); ok && !state.StartedAt.IsZero() {
			offsetMs = int(buffer.StartTime.Sub(state.StartedAt).Milliseconds())
			if offsetMs < 0 {
				offsetMs = 0
			}
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
		}

		if err := s.transcription.TranscribeAudio(ctx, audioData); err != nil {
			logging.Errorf("Failed to transcribe remaining audio for session=%s participant=%s: %v",
				sessionID, participantID, err)
		}

		s.audioBuffer.ClearBuffer(sessionID, participantID)
	}
}

func (s *Service) generateMinutesForSession(sessionID string) {
	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	transcriptionText, err := s.transcription.GetTranscriptionsAsText(ctx, sessionID)
	if err != nil {
		logging.Errorf("Failed to get transcriptions for session=%s: %v", sessionID, err)
		return
	}

	if transcriptionText == "" {
		logging.Warnf("No transcriptions found for session=%s", sessionID)
	}

	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		logging.Errorf("Failed to get session for minutes=%s: %v", sessionID, err)
		return
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

	_, err = s.minutes.GenerateMinutes(ctx, sessionID, transcriptionText, tmpl)
	if err != nil {
		logging.Errorf("Failed to generate minutes for session=%s: %v", sessionID, err)
		return
	}

	session.Complete()
	if err := s.repo.Update(ctx, session); err != nil {
		logging.Errorf("Failed to mark session as completed: %v", err)
	}
}

func (s *Service) ValidateParticipant(ctx context.Context, jti string) (*Participant, error) {
	if _, exists := s.tokenCache.GetToken(jti); !exists {
		participant, err := s.repo.GetParticipantByJTI(ctx, jti)
		if err != nil || participant == nil {
			return nil, fmt.Errorf("token not found or expired")
		}
		s.tokenCache.SetToken(jti, participant.SessionID, 2*time.Hour)
	}

	participant, err := s.repo.GetParticipantByJTI(ctx, jti)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}

	if participant.TokenUsed {
		return nil, fmt.Errorf("token already used")
	}

	if !participant.IsTokenValid() {
		return nil, fmt.Errorf("token expired")
	}

	return participant, nil
}

func (s *Service) ConnectParticipant(ctx context.Context, participantID string) error {
	participant, err := s.repo.GetParticipantByJTI(ctx, participantID)
	if err != nil {
		return err
	}

	if participant == nil {
		participants, err := s.repo.GetParticipantsBySession(ctx, participantID)
		if err != nil {
			return err
		}
		if len(participants) == 0 {
			return fmt.Errorf("participant not found")
		}
		participant = participants[0]
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
		s.sessionCache.SetSession(participant.SessionID, state, 2*time.Hour)
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

		job := &transcribeJob{
			SessionID:     sessionID,
			ParticipantID: participantID,
			Role:          participant.Role,
			AudioData:     make([]byte, len(buffer.Data)),
			OpusFrames:    framesCopy,
			DurationMs:    buffer.DurationMs,
			OffsetMs:      offsetMs,
		}
		copy(job.AudioData, buffer.Data)

		select {
		case s.transcribeCh <- job:
			s.audioBuffer.ClearBuffer(sessionID, participantID)
		default:
			logging.Warnf("Transcription queue full, dropping chunk for session=%s participant=%s", sessionID, participantID)
		}
	}

	stream, err := s.repo.GetAudioStreamByParticipant(context.Background(), participantID)
	if err == nil && stream != nil {
		stream.AddChunk()
		s.repo.UpdateAudioStream(context.Background(), stream)
	}

	return nil
}
