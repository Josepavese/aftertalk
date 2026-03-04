package session

import (
	"context"
	"fmt"
	"time"

	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/pkg/jwt"
	"github.com/google/uuid"
)

type Service struct {
	repo         *SessionRepository
	jwtManager   *jwt.JWTManager
	sessionCache *cache.SessionCache
	tokenCache   *cache.TokenCache
}

func NewService(repo *SessionRepository, jwtManager *jwt.JWTManager, sessionCache *cache.SessionCache, tokenCache *cache.TokenCache) *Service {
	return &Service{
		repo:         repo,
		jwtManager:   jwtManager,
		sessionCache: sessionCache,
		tokenCache:   tokenCache,
	}
}

type CreateSessionRequest struct {
	ParticipantCount int                  `json:"participant_count"`
	Participants     []ParticipantRequest `json:"participants"`
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
	session := NewSession(sessionID, req.ParticipantCount)
	session.Metadata = req.Metadata

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	responses := make([]ParticipantResponse, 0, len(req.Participants))
	for _, p := range req.Participants {
		participantID := uuid.New().String()
		tokenJTI := uuid.New().String()
		tokenExpiresAt := time.Now().Add(2 * time.Hour)

		token, _, err := s.jwtManager.Generate(sessionID, p.UserID, p.Role)
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

func (s *Service) EndSession(ctx context.Context, sessionID string) error {
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	session.End()
	session.StartProcessing()

	if err := s.repo.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

func (s *Service) ValidateParticipant(ctx context.Context, jti string) (*Participant, error) {
	if _, exists := s.tokenCache.GetToken(jti); !exists {
		return nil, fmt.Errorf("token not found or expired")
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
	participants, err := s.repo.GetParticipantsBySession(ctx, participantID)
	if err != nil {
		return err
	}

	if len(participants) == 0 {
		return fmt.Errorf("participant not found")
	}

	participant := participants[0]
	participant.Connect()

	if err := s.repo.UpdateParticipant(ctx, participant); err != nil {
		return fmt.Errorf("failed to update participant: %w", err)
	}

	if state, exists := s.sessionCache.GetSession(participant.SessionID); exists {
		state.ActiveParticipants++
		s.sessionCache.SetSession(participant.SessionID, state, 2*time.Hour)
	}

	return nil
}
