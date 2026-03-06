package webrtc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Logger interface {
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type defaultLogger struct{}

func (d defaultLogger) Infof(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (d defaultLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func (d defaultLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

var log Logger = defaultLogger{}

func SetLogger(l Logger) {
	log = l
}

type SignalingMessage struct {
	Type         string `json:"type"`
	SessionID    string `json:"session_id,omitempty"`
	ParticipantID string `json:"participant_id,omitempty"`
	Role         string `json:"role,omitempty"`
	SDP          string `json:"sdp,omitempty"`
	Candidate    string `json:"candidate,omitempty"`
	Mid          string `json:"mid,omitempty"`
}

type SignalingServer struct {
	manager      *Manager
	connections  map[string]*websocket.Conn
	mu           sync.RWMutex
	validateToken func(token string) (*Claims, error)
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
		log.Warnf("Invalid token: %v", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	key := claims.SessionID + ":" + claims.UserID
	s.mu.Lock()
	s.connections[key] = conn
	s.mu.Unlock()

	log.Infof("Signaling connected: session=%s user=%s role=%s",
		claims.SessionID, claims.UserID, claims.Role)

	go s.handleMessages(conn, claims.SessionID, claims.UserID, claims.Role)
}

func (s *SignalingServer) handleMessages(conn *websocket.Conn, sessionID, participantID, role string) {
	defer func() {
		conn.Close()
		key := sessionID + ":" + participantID
		s.mu.Lock()
		delete(s.connections, key)
		s.mu.Unlock()

		s.manager.RemovePeer(sessionID, participantID)
		log.Infof("Signaling disconnected: session=%s participant=%s", sessionID, participantID)
	}()

	for {
		var msg SignalingMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Errorf("Error reading signaling message: %v", err)
			return
		}

		s.handleMessage(conn, sessionID, participantID, role, &msg)
	}
}

func (s *SignalingServer) handleMessage(conn *websocket.Conn, sessionID, participantID, role string, msg *SignalingMessage) {
	switch msg.Type {
	case "join":
		s.handleJoin(conn, sessionID, participantID, role)

	case "offer":
		s.handleOffer(conn, sessionID, participantID, role, msg.SDP)

	case "answer":
		s.handleAnswer(conn, sessionID, participantID, msg.SDP)

	case "candidate":
		s.handleCandidate(conn, sessionID, participantID, msg.Candidate, msg.Mid)

	default:
		log.Warnf("Unknown signaling message type: %s", msg.Type)
	}
}

func (s *SignalingServer) handleJoin(conn *websocket.Conn, sessionID, participantID, role string) {
	log.Infof("Peer joining: session=%s participant=%s role=%s", sessionID, participantID, role)

	peer, err := s.manager.CreatePeer(nil, sessionID, participantID, role)
	if err != nil {
		log.Errorf("Failed to create peer: %v", err)
		return
	}

	offer, err := peer.PC.CreateOffer(nil)
	if err != nil {
		log.Errorf("Failed to create offer: %v", err)
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(peer.PC)
	if err := peer.PC.SetLocalDescription(offer); err != nil {
		log.Errorf("Failed to set local description: %v", err)
		return
	}

	<-gatherComplete

	response := SignalingMessage{
		Type:          "offer",
		SessionID:     sessionID,
		ParticipantID: participantID,
		SDP:           peer.PC.LocalDescription().SDP,
	}

	if err := conn.WriteJSON(response); err != nil {
		log.Errorf("Failed to send offer: %v", err)
	}
}

func (s *SignalingServer) handleOffer(conn *websocket.Conn, sessionID, participantID, role, sdp string) {
	log.Infof("Received offer from: session=%s participant=%s", sessionID, participantID)

	peer, err := s.manager.CreatePeer(nil, sessionID, participantID, role)
	if err != nil {
		log.Errorf("Failed to create peer: %v", err)
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}

	if err := peer.PC.SetRemoteDescription(offer); err != nil {
		log.Errorf("Failed to set remote description: %v", err)
		return
	}

	answer, err := peer.PC.CreateAnswer(nil)
	if err != nil {
		log.Errorf("Failed to create answer: %v", err)
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(peer.PC)
	if err := peer.PC.SetLocalDescription(answer); err != nil {
		log.Errorf("Failed to set local description: %v", err)
		return
	}

	<-gatherComplete

	response := SignalingMessage{
		Type:          "answer",
		SessionID:     sessionID,
		ParticipantID: participantID,
		SDP:           peer.PC.LocalDescription().SDP,
	}

	if err := conn.WriteJSON(response); err != nil {
		log.Errorf("Failed to send answer: %v", err)
	}
}

func (s *SignalingServer) handleAnswer(conn *websocket.Conn, sessionID, participantID, sdp string) {
	log.Infof("Received answer from: session=%s participant=%s", sessionID, participantID)

	peer, exists := s.manager.GetPeer(sessionID, participantID)
	if !exists {
		log.Warnf("Peer not found: %s", participantID)
		return
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdp,
	}

	if err := peer.PC.SetRemoteDescription(answer); err != nil {
		log.Errorf("Failed to set remote description: %v", err)
	}
}

func (s *SignalingServer) handleCandidate(conn *websocket.Conn, sessionID, participantID, candidate, mid string) {
	peer, exists := s.manager.GetPeer(sessionID, participantID)
	if !exists {
		log.Warnf("Peer not found for ICE candidate: %s", participantID)
		return
	}

	iceCandidate := webrtc.ICECandidateInit{
		Candidate: candidate,
		SDPMid:    &mid,
	}

	if err := peer.PC.AddICECandidate(iceCandidate); err != nil {
		log.Errorf("Failed to add ICE candidate: %v", err)
	}
}

func (s *SignalingServer) BroadcastToSession(sessionID string, msg SignalingMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, _ := json.Marshal(msg)
	for key, conn := range s.connections {
		if len(key) > len(sessionID) && key[:len(sessionID)] == sessionID {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

type Claims struct {
	SessionID string
	UserID    string
	Role      string
}
