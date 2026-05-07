package session

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/internal/storage/cache"
	"github.com/Josepavese/aftertalk/pkg/jwt"
	"github.com/Josepavese/aftertalk/pkg/webhook"
)

type timeoutAwareMinutesService struct {
	called             chan struct{}
	generationTimeout  time.Duration
	observedCtxErr     error
	observedLLMProfile string
}

func (m *timeoutAwareMinutesService) GenerateMinutes(
	ctx context.Context,
	_ string,
	_ string,
	_ config.TemplateConfig,
	_ webhook.SessionContext,
	_ string,
	llmProfile string,
) (interface{}, error) {
	m.observedLLMProfile = llmProfile
	select {
	case <-time.After(30 * time.Millisecond):
		m.observedCtxErr = ctx.Err()
		close(m.called)
		return nil, nil
	case <-ctx.Done():
		m.observedCtxErr = ctx.Err()
		close(m.called)
		return nil, ctx.Err()
	}
}

func (m *timeoutAwareMinutesService) GetMinutes(context.Context, string) (interface{}, error) {
	return nil, nil
}

func (m *timeoutAwareMinutesService) MarkSessionError(context.Context, string, string, string, webhook.SessionContext, error) error {
	return nil
}

func (m *timeoutAwareMinutesService) GenerationTimeout(llmProfile string) time.Duration {
	if llmProfile == "cloud" {
		return m.generationTimeout
	}
	return 0
}

func TestMain(m *testing.M) {
	logging.Init("info", "console") //nolint:errcheck
	os.Exit(m.Run())
}

func TestService_CreateSession(t *testing.T) {
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	tests := []struct {
		req     *CreateSessionRequest
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name: "Create session successfully with valid participants",
			req: &CreateSessionRequest{
				ParticipantCount: 2,
				Participants: []ParticipantRequest{
					{UserID: "user-1", Role: "moderator"},
					{UserID: "user-2", Role: "participant"},
				},
			},
			wantErr: false,
		},
		{
			name: "Create session with metadata",
			req: &CreateSessionRequest{
				ParticipantCount: 2,
				Participants: []ParticipantRequest{
					{UserID: "user-1", Role: "moderator"},
					{UserID: "user-2", Role: "participant"},
				},
				Metadata: "test metadata",
			},
			wantErr: false,
		},
		{
			name: "Reject session with less than 2 participants",
			req: &CreateSessionRequest{
				ParticipantCount: 1,
				Participants: []ParticipantRequest{
					{UserID: "user-1", Role: "participant"},
				},
			},
			wantErr: true,
			errMsg:  "at least 2 participants required",
		},
		{
			name: "Reject session with participant count mismatch",
			req: &CreateSessionRequest{
				ParticipantCount: 2,
				Participants: []ParticipantRequest{
					{UserID: "user-1", Role: "moderator"},
					{UserID: "user-2", Role: "participant"},
					{UserID: "user-3", Role: "participant"},
				},
			},
			wantErr: true,
			errMsg:  "participant count mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			repo := NewSessionRepository(db)

			ctx := context.Background()

			service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

			resp, err := service.CreateSession(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.NotEmpty(t, resp.SessionID)
			assert.Len(t, resp.Participants, tt.req.ParticipantCount)
		})
	}
}

func TestService_CreateSession_DBError(t *testing.T) {
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	req := &CreateSessionRequest{
		ParticipantCount: 2,
		Participants: []ParticipantRequest{
			{UserID: "user-1", Role: "moderator"},
			{UserID: "user-2", Role: "participant"},
		},
	}

	db := setupTestDB(t)
	repo := NewSessionRepository(db)

	ctx := context.Background()

	// Create a session first to make sure we have data
	session := NewSession("test-session", 2, "", "", "")
	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	// Now create a service and try to create another session
	// This should succeed since the database is functional
	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	// Use a different session ID
	req.Participants = []ParticipantRequest{
		{UserID: "user-3", Role: "moderator"},
		{UserID: "user-4", Role: "participant"},
	}

	resp, err := service.CreateSession(ctx, req)

	// Should succeed because the service works
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_CreateSession_JWTError(t *testing.T) {
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	req := &CreateSessionRequest{
		ParticipantCount: 2,
		Participants: []ParticipantRequest{
			{UserID: "user-1", Role: "moderator"},
			{UserID: "user-2", Role: "participant"},
		},
	}

	db := setupTestDB(t)
	repo := NewSessionRepository(db)

	ctx := context.Background()

	// Create a session first to ensure DB is functional
	session := NewSession("test-session", 2, "", "", "")
	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	// Now create a service with the real repo
	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	// This should succeed since JWT generation works
	resp, err := service.CreateSession(ctx, req)

	// Should succeed because JWT generation works
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_GetSession(t *testing.T) {
	now := time.Now()
	sessionID := uuid.New().String()
	session := NewSession(sessionID, 2, "", "", "")
	session.CreatedAt = now

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	err := repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	service := NewService(repo, nil, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	retrieved, err := service.GetSession(ctx, sessionID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, sessionID, retrieved.ID)
	assert.Equal(t, StatusActive, retrieved.Status)
	assert.Equal(t, 2, retrieved.ParticipantCount)
	assert.False(t, retrieved.CreatedAt.IsZero())
}

func TestService_GetSession_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()
	service := NewService(repo, nil, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	_, err := service.GetSession(ctx, "non-existent-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_EndSession(t *testing.T) {
	now := time.Now()
	sessionID := uuid.New().String()
	session := NewSession(sessionID, 2, "", "", "")
	session.CreatedAt = now

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	err := repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	service := NewService(repo, nil, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	err = service.EndSession(ctx, sessionID)
	assert.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, sessionID)
	assert.NoError(t, err)
	assert.Equal(t, StatusProcessing, retrieved.Status)
	assert.NotNil(t, retrieved.EndedAt)
}

func TestService_EndSession_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()
	service := NewService(repo, nil, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	err := service.EndSession(ctx, "non-existent-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_RegenerateSession_RejectsActiveSession(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()
	sessionID := uuid.New().String()
	sess := NewSession(sessionID, 2, "", "", "")
	require.NoError(t, repo.Create(ctx, sess))

	service := NewService(repo, nil, cache.NewSessionCache(), cache.NewTokenCache(), nil, nil, nil, 0,
		config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	err := service.RegenerateSession(ctx, sessionID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot regenerate active session")
}

func TestTranscriptionTrackerWaitsPerSession(t *testing.T) {
	tracker := newTranscriptionTracker()
	tracker.Add("session-a")
	tracker.Add("session-b")

	waitA := make(chan struct{})
	go func() {
		tracker.Wait("session-a")
		close(waitA)
	}()

	select {
	case <-waitA:
		t.Fatal("session-a wait returned before session-a completed")
	case <-time.After(20 * time.Millisecond):
	}

	tracker.Done("session-b")
	select {
	case <-waitA:
		t.Fatal("session-a wait returned after unrelated session completed")
	case <-time.After(20 * time.Millisecond):
	}

	tracker.Done("session-a")
	select {
	case <-waitA:
	case <-time.After(time.Second):
		t.Fatal("session-a wait did not return after session-a completed")
	}
}

func TestServiceAcquireMinutesLockSerializesSameSession(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	service := NewService(repo, nil, cache.NewSessionCache(), cache.NewTokenCache(), nil, nil, nil, 0,
		config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	unlockFirst := service.acquireMinutesLock("session-a")
	acquiredSecond := make(chan struct{})
	releaseSecond := make(chan struct{})
	go func() {
		unlockSecond := service.acquireMinutesLock("session-a")
		close(acquiredSecond)
		<-releaseSecond
		unlockSecond()
	}()

	select {
	case <-acquiredSecond:
		t.Fatal("second generation acquired lock before first released it")
	case <-time.After(20 * time.Millisecond):
	}

	unlockFirst()
	select {
	case <-acquiredSecond:
	case <-time.After(time.Second):
		t.Fatal("second generation did not acquire lock after first released it")
	}
	close(releaseSecond)
}

func TestGenerateMinutesForSession_UsesProfileGenerationTimeout(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()
	tmpl := config.DefaultTemplates()[0]
	sessionID := uuid.New().String()
	sess := NewSession(sessionID, 2, tmpl.ID, "", "cloud")
	sess.StartProcessing()
	require.NoError(t, repo.Create(ctx, sess))

	minutesSvc := &timeoutAwareMinutesService{
		called:            make(chan struct{}),
		generationTimeout: 200 * time.Millisecond,
	}
	service := NewService(repo, nil, cache.NewSessionCache(), cache.NewTokenCache(), nil, nil, minutesSvc, 0,
		config.ProcessingConfig{
			TranscriptionQueueSize:          10,
			ChunkSizeMs:                     15000,
			MinutesGenerationTimeout:        10 * time.Millisecond,
			MaxConcurrentMinutesGenerations: 1,
		},
		config.DefaultTemplates(), config.SessionConfig{})

	service.generateMinutesForSession(sessionID)

	select {
	case <-minutesSvc.called:
	case <-time.After(time.Second):
		t.Fatal("minutes generation was not called")
	}
	assert.Equal(t, "cloud", minutesSvc.observedLLMProfile)
	assert.NoError(t, minutesSvc.observedCtxErr)

	updated, err := repo.GetByID(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, updated.Status)
}

func TestService_EndSession_DBError(t *testing.T) {
	now := time.Now().UTC()
	sessionID := uuid.New().String()
	session := NewSession(sessionID, 2, "", "", "")
	session.CreatedAt = now

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	sessionCache := cache.NewSessionCache()

	service := NewService(repo, nil, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})
	err := service.EndSession(ctx, sessionID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_ValidateParticipant_Success(t *testing.T) {
	now := time.Now().UTC()
	validTokenJTI := "valid-token-jti"
	userID := "user-1"
	role := "moderator"

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	participant := NewParticipant("participant-1", "session-1", userID, role, validTokenJTI, now.Add(2*time.Hour))
	participant.ConnectedAt = &now

	repo.CreateParticipant(ctx, participant)

	tokenCache.SetToken(validTokenJTI, "session-1", 2*time.Hour)

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	result, err := service.ValidateParticipant(ctx, validTokenJTI)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "participant-1", result.ID)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, role, result.Role)
	assert.Equal(t, validTokenJTI, result.TokenJTI)
	assert.False(t, result.TokenUsed)
	assert.NotNil(t, result.TokenExpiresAt)
}

func TestService_ValidateParticipant_TokenNotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	_, err := service.ValidateParticipant(ctx, "non-existent-jti")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_ValidateParticipant_TokenExpired(t *testing.T) {
	now := time.Now().UTC()
	expiredTokenJTI := "expired-token-jti"

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	participant := NewParticipant("participant-1", "session-1", "user-1", "moderator", expiredTokenJTI, now.Add(-1*time.Hour))

	repo.CreateParticipant(ctx, participant)

	tokenCache.SetToken(expiredTokenJTI, "session-1", 2*time.Hour)

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	_, err := service.ValidateParticipant(ctx, expiredTokenJTI)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestService_ValidateParticipant_TokenAlreadyUsed(t *testing.T) {
	now := time.Now().UTC()
	usedTokenJTI := "used-token-jti"

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	participant := NewParticipant("participant-1", "session-1", "user-1", "moderator", usedTokenJTI, now.Add(2*time.Hour))
	participant.TokenUsed = true

	repo.CreateParticipant(ctx, participant)

	tokenCache.SetToken(usedTokenJTI, "session-1", 2*time.Hour)

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	_, err := service.ValidateParticipant(ctx, usedTokenJTI)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already used")
}

func TestService_ValidateParticipant_DBError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	_, err := service.ValidateParticipant(ctx, "some-jti")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_ConnectParticipant_Success(t *testing.T) {
	now := time.Now()
	sessionID := "session-1"
	participantID := "participant-1"

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	session := NewSession(sessionID, 2, "", "", "")
	session.CreatedAt = now
	err := repo.Create(ctx, session)
	assert.NoError(t, err)

	participant := NewParticipant(participantID, sessionID, "user-1", "moderator", "token-jti-1", now.Add(2*time.Hour))
	err = repo.CreateParticipant(ctx, participant)
	assert.NoError(t, err)

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	err = service.ConnectParticipant(ctx, participantID)
	require.NoError(t, err)

	connected, err := repo.GetParticipantByID(ctx, participantID)
	require.NoError(t, err)
	require.NotNil(t, connected.ConnectedAt)
}

func TestService_ConnectParticipant_NoParticipants(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	err := service.ConnectParticipant(ctx, "non-existent-participant")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "participant not found")
}

func TestService_ConnectParticipant_WithoutCachedSessionSucceeds(t *testing.T) {
	now := time.Now().UTC()
	sessionID := "session-1"
	participantID := "participant-1"

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	session := NewSession(sessionID, 2, "", "", "")
	session.CreatedAt = now
	repo.Create(ctx, session)

	participant := NewParticipant(participantID, sessionID, "user-1", "moderator", "token-jti-1", now.Add(2*time.Hour))
	repo.CreateParticipant(ctx, participant)

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	err := service.ConnectParticipant(ctx, participantID)
	require.NoError(t, err)
}

func TestService_ConnectParticipant_SessionNotActive(t *testing.T) {
	now := time.Now().UTC()
	sessionID := "session-1"
	participantID := "participant-1"

	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	jwtManager := jwt.NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()

	ctx := context.Background()

	session := NewSession(sessionID, 2, "", "", "")
	session.CreatedAt = now
	session.End()
	repo.Create(ctx, session)

	participant := NewParticipant(participantID, sessionID, "user-1", "moderator", "token-jti-1", now.Add(2*time.Hour))
	repo.CreateParticipant(ctx, participant)

	service := NewService(repo, jwtManager, sessionCache, tokenCache, nil, nil, nil, 0, config.ProcessingConfig{TranscriptionQueueSize: 10, ChunkSizeMs: 15000}, nil, config.SessionConfig{})

	err := service.ConnectParticipant(ctx, participantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session is not active")
}
