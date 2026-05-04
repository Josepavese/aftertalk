package api

import (
	"net/http"

	"github.com/Josepavese/aftertalk/internal/bot/webrtc"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/core/session"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/internal/storage/cache"
	"github.com/Josepavese/aftertalk/pkg/jwt"
)

type BotServer struct {
	SessionService  *session.Service
	JWTManager      *jwt.JWTManager
	TokenCache      *cache.TokenCache
	WebRTCManager   *webrtc.Manager
	SignalingServer *webrtc.SignalingServer
}

func NewBotServer(sessionService *session.Service, jwtManager *jwt.JWTManager, tokenCache *cache.TokenCache, iceServers []config.ICEServerConfig, iceUDPPortMin, iceUDPPortMax uint16) *BotServer {
	webrtcManager := webrtc.NewManager(func(sessionID, participantID, role string, payload []byte) {
		if err := sessionService.ProcessAudioChunk(sessionID, participantID, payload); err != nil {
			logging.Errorf("ProcessAudioChunk error session=%s participant=%s: %v", sessionID, participantID, err)
		}
	}, iceServers, iceUDPPortMin, iceUDPPortMax)

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

	bot := &BotServer{
		SessionService:  sessionService,
		JWTManager:      jwtManager,
		TokenCache:      tokenCache,
		WebRTCManager:   webrtcManager,
		SignalingServer: signalingServer,
	}

	// When a session is ended, close all WebRTC peers for that session so every
	// connected participant is disconnected immediately.
	sessionService.SetOnSessionEnd(func(sessionID string) {
		webrtcManager.CloseSessionPeers(sessionID)
	})

	return bot
}

func (s *BotServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.SignalingServer.HandleWebSocket(w, r)
}
