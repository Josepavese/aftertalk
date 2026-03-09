package api

import (
	"net/http"
	"github.com/flowup/aftertalk/internal/bot/webrtc"
	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/pkg/jwt"
)

type BotServer struct {
	SessionService  *session.Service
	JWTManager      *jwt.JWTManager
	TokenCache      *cache.TokenCache
	WebRTCManager   *webrtc.Manager
	SignalingServer *webrtc.SignalingServer
}

func NewBotServer(sessionService *session.Service, jwtManager *jwt.JWTManager, tokenCache *cache.TokenCache) *BotServer {
	webrtcManager := webrtc.NewManager(func(sessionID, participantID, role string, payload []byte) {
		if err := sessionService.ProcessAudioChunk(sessionID, participantID, payload); err != nil {
			logging.Errorf("ProcessAudioChunk error session=%s participant=%s: %v", sessionID, participantID, err)
		}
	})

	signalingServer := webrtc.NewSignalingServer(webrtcManager, func(tokenString string) (*webrtc.Claims, error) {
		claims, err := jwtManager.Validate(tokenString)
		if err != nil {
			return nil, err
		}
		return &webrtc.Claims{
			SessionID: claims.SessionID,
			UserID:    claims.UserID,
			Role:      claims.Role,
			JTI:       claims.ID, // RegisteredClaims.ID is the JWT JTI
		}, nil
	})

	return &BotServer{
		SessionService:  sessionService,
		JWTManager:     jwtManager,
		TokenCache:     tokenCache,
		WebRTCManager:  webrtcManager,
		SignalingServer: signalingServer,
	}
}

func (s *BotServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.SignalingServer.HandleWebSocket(w, r)
}
