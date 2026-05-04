package webrtc

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"

	"github.com/Josepavese/aftertalk/internal/logging"
)

// iceCandidatePayload matches the RTCIceCandidate object sent by browsers.
type iceCandidatePayload struct {
	SDPMid           *string `json:"sdpMid"`
	SDPMLineIndex    *uint16 `json:"sdpMLineIndex"`
	UsernameFragment *string `json:"usernameFragment"`
	Candidate        string  `json:"candidate"`
}

var upgrader = websocket.Upgrader{ //nolint:gochecknoglobals // package-level upgrader is idiomatic for WebSocket servers
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type SignalingMessage struct {
	Type          string          `json:"type"`
	SessionID     string          `json:"session_id,omitempty"`
	ParticipantID string          `json:"participant_id,omitempty"`
	Role          string          `json:"role,omitempty"`
	SDP           string          `json:"sdp,omitempty"`
	Mid           string          `json:"mid,omitempty"`
	Candidate     json.RawMessage `json:"candidate,omitempty"`
}

// connWriter wraps a WebSocket connection with a write mutex.
// gorilla/websocket allows one concurrent writer; all sends must go through here.
type connWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (cw *connWriter) writeJSON(v interface{}) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	return cw.conn.WriteJSON(v)
}

type SignalingServer struct {
	manager       *Manager
	connections   map[string]*websocket.Conn
	validateToken func(token string) (*Claims, error)
	mu            sync.RWMutex
}

func NewSignalingServer(manager *Manager, validateToken func(token string) (*Claims, error)) *SignalingServer {
	return &SignalingServer{
		manager:       manager,
		connections:   make(map[string]*websocket.Conn),
		validateToken: validateToken,
	}
}

func (s *SignalingServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}

	claims, err := s.validateToken(token)
	if err != nil {
		logging.Warnf("Invalid token: %v", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	// Use JTI as participantID so audio chunks can be looked up via GetParticipantByJTI.
	// Fall back to UserID if JTI is empty (e.g. tests).
	participantID := claims.JTI
	if participantID == "" {
		participantID = claims.UserID
	}

	key := claims.SessionID + ":" + participantID
	s.mu.Lock()
	s.connections[key] = conn
	s.mu.Unlock()

	logging.Infof("Signaling connected: session=%s user=%s role=%s jti=%s",
		claims.SessionID, claims.UserID, claims.Role, participantID)

	// Create a context tied to the WebSocket connection lifetime.
	// We cannot use r.Context() because it is canceled as soon as HandleWebSocket returns.
	ctx, cancel := context.WithCancel(context.Background())
	cw := &connWriter{conn: conn}
	go s.handleMessages(ctx, cancel, cw, claims.SessionID, participantID, claims.Role)
}

func (s *SignalingServer) handleMessages(ctx context.Context, cancel context.CancelFunc, cw *connWriter, sessionID, participantID, role string) {
	defer func() {
		cancel()            // signal peer cleanup goroutine
		_ = cw.conn.Close() //nolint:errcheck // best-effort cleanup on disconnect
		key := sessionID + ":" + participantID
		s.mu.Lock()
		delete(s.connections, key)
		s.mu.Unlock()

		s.manager.RemovePeer(sessionID, participantID)
		logging.Infof("Signaling disconnected: session=%s participant=%s", sessionID, participantID)
	}()

	for {
		_, raw, err := cw.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
				logging.Errorf("Signaling read error: %v", err)
			}
			return
		}

		var msg SignalingMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			logging.Warnf("Ignoring unparseable signaling message: %v", err)
			continue
		}

		s.handleMessage(ctx, cw, sessionID, participantID, role, &msg)
	}
}

func (s *SignalingServer) handleMessage(ctx context.Context, cw *connWriter, sessionID, participantID, role string, msg *SignalingMessage) {
	switch msg.Type {
	case "join":
		s.handleJoin(ctx, cw, sessionID, participantID, role)

	case "offer":
		s.handleOffer(ctx, cw, sessionID, participantID, role, msg.SDP)

	case "answer":
		s.handleAnswer(sessionID, participantID, msg.SDP)

	case "candidate", "ice-candidate":
		s.handleCandidate(sessionID, participantID, msg.Candidate, msg.Mid)

	default:
		logging.Warnf("Unknown signaling message type: %s", msg.Type)
	}
}

func (s *SignalingServer) handleJoin(ctx context.Context, cw *connWriter, sessionID, participantID, role string) {
	logging.Infof("Peer joining: session=%s participant=%s role=%s", sessionID, participantID, role)

	peer, err := s.manager.CreatePeer(ctx, sessionID, participantID, role)
	if err != nil {
		logging.Errorf("Failed to create peer: %v", err)
		return
	}

	// Trickle ICE: register candidate callback before SetLocalDescription
	peer.PC.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		candidateJSON, icErr := json.Marshal(c.ToJSON())
		if icErr != nil {
			return
		}
		msg := SignalingMessage{Type: "ice-candidate", Candidate: json.RawMessage(candidateJSON)}
		if icErr = cw.writeJSON(msg); icErr != nil {
			logging.Warnf("Failed to send ICE candidate: %v", icErr)
		}
	})

	offer, err := peer.PC.CreateOffer(nil)
	if err != nil {
		logging.Errorf("Failed to create offer: %v", err)
		return
	}

	if err := peer.PC.SetLocalDescription(offer); err != nil {
		logging.Errorf("Failed to set local description: %v", err)
		return
	}

	response := SignalingMessage{
		Type:          "offer",
		SessionID:     sessionID,
		ParticipantID: participantID,
		SDP:           peer.PC.LocalDescription().SDP,
	}

	if err := cw.writeJSON(response); err != nil {
		logging.Errorf("Failed to send offer: %v", err)
	}
}

func (s *SignalingServer) handleOffer(ctx context.Context, cw *connWriter, sessionID, participantID, role, sdp string) {
	logging.Infof("Received offer from: session=%s participant=%s", sessionID, participantID)

	peer, err := s.manager.CreatePeer(ctx, sessionID, participantID, role)
	if err != nil {
		logging.Errorf("Failed to create peer: %v", err)
		return
	}

	// Trickle ICE: register candidate callback before SetLocalDescription
	peer.PC.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		candidateJSON, icErr := json.Marshal(c.ToJSON())
		if icErr != nil {
			return
		}
		msg := SignalingMessage{Type: "ice-candidate", Candidate: json.RawMessage(candidateJSON)}
		if icErr = cw.writeJSON(msg); icErr != nil {
			logging.Warnf("Failed to send ICE candidate: %v", icErr)
		}
	})

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}

	if err = peer.PC.SetRemoteDescription(offer); err != nil {
		logging.Errorf("Failed to set remote description: %v", err)
		return
	}

	answer, err := peer.PC.CreateAnswer(nil)
	if err != nil {
		logging.Errorf("Failed to create answer: %v", err)
		return
	}

	if err := peer.PC.SetLocalDescription(answer); err != nil {
		logging.Errorf("Failed to set local description: %v", err)
		return
	}

	// Send answer immediately — ICE candidates follow separately via OnICECandidate
	response := SignalingMessage{
		Type:          "answer",
		SessionID:     sessionID,
		ParticipantID: participantID,
		SDP:           peer.PC.LocalDescription().SDP,
	}

	if err := cw.writeJSON(response); err != nil {
		logging.Errorf("Failed to send answer: %v", err)
	}
}

func (s *SignalingServer) handleAnswer(sessionID, participantID, sdp string) {
	logging.Infof("Received answer from: session=%s participant=%s", sessionID, participantID)

	peer, exists := s.manager.GetPeer(sessionID, participantID)
	if !exists {
		logging.Warnf("Peer not found: %s", participantID)
		return
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdp,
	}

	if err := peer.PC.SetRemoteDescription(answer); err != nil {
		logging.Errorf("Failed to set remote description: %v", err)
	}
}

func (s *SignalingServer) handleCandidate(sessionID, participantID string, candidateRaw json.RawMessage, mid string) {
	peer, exists := s.manager.GetPeer(sessionID, participantID)
	if !exists {
		logging.Warnf("Peer not found for ICE candidate: %s", participantID)
		return
	}

	// Browser sends candidate as an RTCIceCandidate object; try that first,
	// then fall back to a plain string.
	var iceCandidate webrtc.ICECandidateInit
	var payload iceCandidatePayload
	if err := json.Unmarshal(candidateRaw, &payload); err == nil && payload.Candidate != "" {
		iceCandidate = webrtc.ICECandidateInit{
			Candidate: payload.Candidate,
			SDPMid:    payload.SDPMid,
		}
	} else {
		var candidateStr string
		if err := json.Unmarshal(candidateRaw, &candidateStr); err != nil {
			logging.Warnf("Cannot parse ICE candidate: %v", err)
			return
		}
		iceCandidate = webrtc.ICECandidateInit{Candidate: candidateStr, SDPMid: &mid}
	}

	if err := peer.PC.AddICECandidate(iceCandidate); err != nil {
		logging.Errorf("Failed to add ICE candidate: %v", err)
	}
}

func (s *SignalingServer) BroadcastToSession(sessionID string, msg SignalingMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		logging.Errorf("Failed to marshal signaling message: %v", err)
		return
	}
	for key, conn := range s.connections {
		if len(key) > len(sessionID) && key[:len(sessionID)] == sessionID {
			if writeErr := conn.WriteMessage(websocket.TextMessage, data); writeErr != nil {
				logging.Warnf("Failed to broadcast to connection %s: %v", key, writeErr)
			}
		}
	}
}

type Claims struct {
	SessionID string
	UserID    string
	Role      string
	JTI       string // JWT ID — used as participantID for audio processing
}
