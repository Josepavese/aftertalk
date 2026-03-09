package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/flowup/aftertalk/internal/api/handler"
	custommiddleware "github.com/flowup/aftertalk/internal/api/middleware"
	"github.com/flowup/aftertalk/internal/config"
	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	httpServer *http.Server
	router     *chi.Mux
	botServer  *BotServer
}

// roomTTL is how long a room entry is kept in the cache after creation.
// After this period the room code can be reused for a new session.
const roomTTL = 3 * time.Hour

// roomEntry holds the tokens and metadata for one active room.
type roomEntry struct {
	tokens    map[string]string // role/key → token
	issued    map[string]bool   // role → already issued
	names     map[string]string // role → participant name (for reconnection)
	createdAt time.Time
}

// roomCache maps a room code to the pre-generated tokens for each role.
// All operations are atomic to prevent race conditions when two browsers
// join the same room simultaneously.
type roomCache struct {
	mu    sync.Mutex
	rooms map[string]*roomEntry
}

var errRoleTaken = fmt.Errorf("role already taken in this room")

// getOrCreate atomically returns the token for the given role in the room,
// creating the session (via create()) if the room does not yet exist or has expired.
// If the role is already issued but the name matches (reconnection), the existing
// token is re-issued. Returns errRoleTaken if the role is taken by a different name.
func (rc *roomCache) getOrCreate(code, role, name string, create func() (map[string]string, error)) (string, string, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Expire stale entries so the same room code can be reused.
	if entry, ok := rc.rooms[code]; ok && time.Since(entry.createdAt) < roomTTL {
		if entry.issued[role] {
			// Allow reconnection if the name matches.
			if entry.names[role] == name {
				return entry.tokens["_session_id"], entry.tokens[role], nil
			}
			return "", "", errRoleTaken
		}
		entry.issued[role] = true
		entry.names[role] = name
		return entry.tokens["_session_id"], entry.tokens[role], nil
	}

	// Room is new or expired — create a fresh session.
	tokens, err := create()
	if err != nil {
		return "", "", err
	}
	rc.rooms[code] = &roomEntry{
		tokens:    tokens,
		issued:    map[string]bool{role: true},
		names:     map[string]string{role: name},
		createdAt: time.Now(),
	}
	return tokens["_session_id"], tokens[role], nil
}

func NewServer(cfg *config.Config, sessionService *session.Service, botServer *BotServer) *Server {
	return NewServerWithDeps(cfg, sessionService, botServer, nil, nil)
}

func NewServerWithDeps(cfg *config.Config, sessionService *session.Service, botServer *BotServer, minutesHandler *handler.MinutesHandler, transcriptionHandler *handler.TranscriptionHandler) *Server {
	rooms := &roomCache{
		rooms: make(map[string]*roomEntry),
	}
	r := chi.NewRouter()

	// Global middleware (no auth)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(custommiddleware.Logging)
	r.Use(custommiddleware.Recovery)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS for all routes
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.Header().Set("Access-Control-Allow-Methods", "*")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	sessionHandler := handler.NewSessionHandler(sessionService)

	// --- Public routes (no API key) ---

	// Static file server for test-ui
	uiPath := findTestUIPath()
	if uiPath != "" {
		fs := http.FileServer(http.Dir(uiPath))
		r.Get("/", fs.ServeHTTP)
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			// strip the wildcard prefix so FileServer sees the plain path
			http.StripPrefix("", fs).ServeHTTP(w, req)
		})
	}

	// /test/start — joins or creates a session by room code.
	// First caller creates the session; second caller joins with the pre-generated token.
	r.Post("/test/start", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Code string `json:"code"`
			Name string `json:"name"`
			Role string `json:"role"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Name == "" || body.Role == "" || body.Code == "" {
			http.Error(w, "invalid request body: code, name and role are required", http.StatusBadRequest)
			return
		}

		otherRole := "speaker"
		if body.Role == "speaker" {
			otherRole = "host"
		}

		sessionID, token, err := rooms.getOrCreate(body.Code, body.Role, body.Name, func() (map[string]string, error) {
			createReq := &session.CreateSessionRequest{
				ParticipantCount: 2,
				Participants: []session.ParticipantRequest{
					{UserID: body.Name, Role: body.Role},
					{UserID: "guest-" + otherRole, Role: otherRole},
				},
			}
			resp, err := sessionService.CreateSession(req.Context(), createReq)
			if err != nil {
				return nil, err
			}
			tokens := map[string]string{"_session_id": resp.SessionID}
			for _, p := range resp.Participants {
				tokens[p.Role] = p.Token
			}
			return tokens, nil
		})
		if err == errRoleTaken {
			otherRoleLabel := "Speaker"
			if body.Role == "speaker" {
				otherRoleLabel = "Host"
			}
			http.Error(w, fmt.Sprintf("Il ruolo '%s' è già occupato nella stanza '%s'. Scegli '%s'.", body.Role, body.Code, otherRoleLabel), http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create session: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"session_id": sessionID,
			"token":      token,
		})
	})

	// --- Protected routes (API key required) ---
	apiKeyMiddleware := func(next http.Handler) http.Handler {
		if cfg.API.Key == "" {
			return next
		}
		return custommiddleware.APIKey(cfg.API.Key)(next)
	}

	r.Group(func(r chi.Router) {
		r.Use(apiKeyMiddleware)

		r.Route("/v1", func(r chi.Router) {
			r.Get("/health", handler.HealthCheck)
			r.Get("/ready", handler.ReadyCheck)
			r.Mount("/sessions", sessionHandler.Routes())

			if minutesHandler != nil {
				r.Mount("/minutes", minutesHandler.Routes())
			}

			if transcriptionHandler != nil {
				r.Mount("/transcriptions", transcriptionHandler.Routes())
			}
		})
	})

	// WebSocket endpoints are public — they authenticate via JWT token in query param
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		botServer.HandleWebSocket(w, r)
	})

	r.Get("/signaling", func(w http.ResponseWriter, r *http.Request) {
		botServer.HandleWebSocket(w, r)
	})

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)

	return &Server{
		router:    r,
		botServer: botServer,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// findTestUIPath searches for the cmd/test-ui directory relative to common locations.
func findTestUIPath() string {
	candidates := []string{
		"./cmd/test-ui",
		"../cmd/test-ui",
		"/home/jose/hpdev/Libraries/aftertalk/cmd/test-ui",
	}

	// Also try relative to the executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "cmd/test-ui"),
			filepath.Join(exeDir, "../cmd/test-ui"),
		)
	}

	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			abs, err := filepath.Abs(p)
			if err == nil {
				return abs
			}
			return p
		}
	}
	return ""
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	return s.httpServer.Shutdown(nil)
}
