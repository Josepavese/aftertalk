package bot

import (
	"net/http"
	"sync"
	"time"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/pkg/jwt"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	sessionService *session.Service
	jwtManager     *jwt.JWTManager
	tokenCache     *cache.TokenCache
	connections    map[string]*Connection
	mu             sync.RWMutex
}

type Connection struct {
	SessionID     string
	ParticipantID string
	Role          string
	Conn          *websocket.Conn
	Send          chan []byte
	Done          chan struct{}
}

func NewServer(sessionService *session.Service, jwtManager *jwt.JWTManager, tokenCache *cache.TokenCache) *Server {
	return &Server{
		sessionService: sessionService,
		jwtManager:     jwtManager,
		tokenCache:     tokenCache,
		connections:    make(map[string]*Connection),
	}
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}

	claims, err := s.jwtManager.Validate(token)
	if err != nil {
		logging.Warnf("Invalid token: %v", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	participant, err := s.sessionService.ValidateParticipant(r.Context(), claims.ID)
	if err != nil {
		logging.Warnf("Participant validation failed: %v", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	connection := &Connection{
		SessionID:     claims.SessionID,
		ParticipantID: participant.ID,
		Role:          claims.Role,
		Conn:          conn,
		Send:          make(chan []byte, 256),
		Done:          make(chan struct{}),
	}

	s.mu.Lock()
	s.connections[participant.ID] = connection
	s.mu.Unlock()

	logging.Infof("WebSocket connected: session=%s participant=%s role=%s",
		claims.SessionID, participant.ID, claims.Role)

	if err := s.sessionService.ConnectParticipant(r.Context(), participant.ID); err != nil {
		logging.Errorf("Failed to connect participant: %v", err)
	}

	go s.readPump(connection)
	go s.writePump(connection)
}

func (s *Server) readPump(conn *Connection) {
	defer func() {
		close(conn.Done)
		conn.Conn.Close()
		s.mu.Lock()
		delete(s.connections, conn.ParticipantID)
		s.mu.Unlock()
		logging.Infof("WebSocket disconnected: participant=%s", conn.ParticipantID)
	}()

	conn.Conn.SetReadLimit(512 * 1024)
	conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.Conn.SetPongHandler(func(string) error {
		conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logging.Errorf("WebSocket read error: %v", err)
			}
			break
		}

		logging.Debugf("Received message from %s: %d bytes", conn.ParticipantID, len(message))
	}
}

func (s *Server) writePump(conn *Connection) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Conn.Close()
	}()

	for {
		select {
		case <-conn.Done:
			return
		case message, ok := <-conn.Send:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) Broadcast(sessionID string, message []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, conn := range s.connections {
		if conn.SessionID == sessionID {
			select {
			case conn.Send <- message:
			default:
				logging.Warnf("Connection buffer full for participant %s", conn.ParticipantID)
			}
		}
	}

	return nil
}

func (s *Server) GetConnection(participantID string) (*Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, exists := s.connections[participantID]
	return conn, exists
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, conn := range s.connections {
		close(conn.Done)
		conn.Conn.Close()
	}

	s.connections = make(map[string]*Connection)
	return nil
}
