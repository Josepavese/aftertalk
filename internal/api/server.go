package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Josepavese/aftertalk/internal/api/handler"
	custommiddleware "github.com/Josepavese/aftertalk/internal/api/middleware"
	"github.com/Josepavese/aftertalk/internal/api/response"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/core/session"
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
	rooms map[string]*roomEntry
	mu    sync.Mutex
}

var errRoleTaken = errors.New("role already taken in this room")

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
	return NewServerWithDeps(cfg, sessionService, botServer, nil, nil, nil)
}

func NewServerWithDeps(cfg *config.Config, sessionService *session.Service, botServer *BotServer, minutesHandler *handler.MinutesHandler, transcriptionHandler *handler.TranscriptionHandler, rtcHandler *handler.RTCConfigHandler) *Server {
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

	// CORS — configurable via cfg.API.CORS; defaults to wildcard for dev.
	corsOrigins := cfg.API.CORS.AllowedOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	corsHeaders := strings.Join(cfg.API.CORS.AllowedHeaders, ", ")
	if corsHeaders == "" {
		corsHeaders = "Authorization, Content-Type, X-API-Key, X-Request-ID"
	}
	corsMethods := strings.Join(cfg.API.CORS.AllowedMethods, ", ")
	if corsMethods == "" {
		corsMethods = "GET, POST, PUT, DELETE, OPTIONS"
	}
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range corsOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			if allowed {
				if len(corsOrigins) == 1 && corsOrigins[0] == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Headers", corsHeaders)
				w.Header().Set("Access-Control-Allow-Methods", corsMethods)
				if cfg.API.CORS.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Rate limiting — applied globally when enabled.
	if cfg.API.RateLimit.Enabled && cfg.API.RateLimit.RequestsPerMinute > 0 {
		r.Use(custommiddleware.RateLimit(cfg.API.RateLimit.RequestsPerMinute))
	}

	sessionHandler := handler.NewSessionHandler(sessionService)

	// --- Public routes (no API key) ---

	// apiKeyMiddleware is defined early so it can be used on both public and protected routes.
	apiKeyMiddleware := func(next http.Handler) http.Handler {
		if cfg.API.Key == "" {
			return next
		}
		return custommiddleware.APIKey(cfg.API.Key)(next)
	}

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

	// Build a lookup map for templates (templateID → TemplateConfig).
	templateMap := make(map[string]config.TemplateConfig, len(cfg.Templates))
	for _, t := range cfg.Templates {
		templateMap[t.ID] = t
	}
	// Default template ID (first in list).
	defaultTemplateID := ""
	if len(cfg.Templates) > 0 {
		defaultTemplateID = cfg.Templates[0].ID
	}

	// /demo/config — public metadata for the test UI (no auth required).
	// The API key is included only when cfg.Demo.Enabled=true (local demo mode).
	// Never set Demo.Enabled=true in production.
	r.Get("/demo/config", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"templates":           cfg.Templates,
			"default_template_id": defaultTemplateID,
		}
		if cfg.Demo.Enabled {
			resp["api_key"] = cfg.API.Key
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	})

	// /test/start — requires API key when one is configured.
	// Joins or creates a session by room code.
	r.With(apiKeyMiddleware).Post("/test/start", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Code       string `json:"code"`
			Name       string `json:"name"`
			Role       string `json:"role"`
			TemplateID string `json:"template_id"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Name == "" || body.Role == "" || body.Code == "" {
			http.Error(w, "invalid request body: code, name and role are required", http.StatusBadRequest)
			return
		}

		// Resolve the template; fall back to default if not specified.
		if body.TemplateID == "" {
			body.TemplateID = defaultTemplateID
		}
		tmpl, hasTmpl := templateMap[body.TemplateID]

		// Derive the "other" role from the template (second role that isn't ours).
		otherRole := ""
		if hasTmpl && len(tmpl.Roles) >= 2 {
			for _, r := range tmpl.Roles {
				if r.Key != body.Role {
					otherRole = r.Key
					break
				}
			}
		}
		if otherRole == "" {
			// Fallback for unknown templates.
			if body.Role == "therapist" || body.Role == "consultant" || body.Role == "host" {
				otherRole = "patient"
			} else {
				otherRole = "therapist"
			}
		}

		sessionID, token, err := rooms.getOrCreate(body.Code, body.Role, body.Name, func() (map[string]string, error) { //nolint:contextcheck // callback closure captures req.Context() below
			createReq := &session.CreateSessionRequest{
				ParticipantCount: 2,
				TemplateID:       body.TemplateID,
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
		if errors.Is(err, errRoleTaken) {
			http.Error(w, fmt.Sprintf("Role '%s' is already taken in room '%s'. Choose another role.", body.Role, body.Code), http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create session: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"session_id": sessionID,
			"token":      token,
		}); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	})

	// --- Protected routes (API key required) ---
	r.Group(func(r chi.Router) {
		r.Use(apiKeyMiddleware)

		r.Route("/v1", func(r chi.Router) {
			r.Get("/health", handler.HealthCheck)
			r.Get("/ready", handler.ReadyCheck)

			// /v1/config — public metadata (templates, version) without API key exposure.
			r.Get("/config", func(w http.ResponseWriter, req *http.Request) {
				response.OK(w, map[string]interface{}{
					"templates":           cfg.Templates,
					"default_template_id": defaultTemplateID,
				})
			})

			// OpenAPI spec — served from specs/contracts/api.yaml.
			r.Get("/openapi.yaml", func(w http.ResponseWriter, req *http.Request) {
				candidates := []string{
					"./specs/contracts/api.yaml",
					"../specs/contracts/api.yaml",
				}
				if exe, err := os.Executable(); err == nil {
					dir := filepath.Dir(exe)
					candidates = append(candidates,
						filepath.Join(dir, "specs/contracts/api.yaml"),
						filepath.Join(dir, "../specs/contracts/api.yaml"),
					)
				}
				for _, p := range candidates {
					if _, err := os.Stat(p); err == nil {
						w.Header().Set("Content-Type", "application/yaml")
						http.ServeFile(w, req, p)
						return
					}
				}
				response.NotFound(w, "openapi spec not found")
			})

			r.Mount("/sessions", sessionHandler.Routes())

			if minutesHandler != nil {
				r.Mount("/minutes", minutesHandler.Routes())
			}

			if transcriptionHandler != nil {
				r.Mount("/transcriptions", transcriptionHandler.Routes())
			}

			if rtcHandler != nil {
				r.Get("/rtc-config", rtcHandler.ServeHTTP)
			}
		})
	})

	// GET /v1/minutes/pull/{token} — notify_pull secure retrieval endpoint.
	// Registered outside the API key middleware group: the token in the URL path
	// IS the credential (single-use, time-limited). All invalid/expired/used tokens
	// return 404 (intentionally indistinguishable from not-found).
	if minutesHandler != nil {
		r.Get("/v1/minutes/pull/{token}", minutesHandler.PullMinutes)
	}

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

// Handler returns the underlying http.Handler for use in tests.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
