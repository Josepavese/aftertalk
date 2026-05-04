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
	httpServer  *http.Server
	router      *chi.Mux
	botServer   *BotServer
	tlsCertFile string
	tlsKeyFile  string
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

	// /v1/config and /v1/rtc-config are public — no API key required.
	// ICE servers are not sensitive (STUN/TURN addresses are public). Making
	// rtc-config public allows frontend SDKs to connect WebRTC without an
	// API key, using only the JWT session token for signaling auth.
	if rtcHandler != nil {
		r.Get("/v1/rtc-config", rtcHandler.ServeHTTP)
	}

	r.Get("/v1/config", func(w http.ResponseWriter, req *http.Request) {
		sttProfiles := make([]string, 0, len(cfg.STT.Profiles))
		for name := range cfg.STT.Profiles {
			sttProfiles = append(sttProfiles, name)
		}
		llmProfiles := make([]string, 0, len(cfg.LLM.Profiles))
		for name := range cfg.LLM.Profiles {
			llmProfiles = append(llmProfiles, name)
		}
		response.OK(w, map[string]interface{}{
			"templates":           cfg.Templates,
			"default_template_id": defaultTemplateID,
			"stt_profiles":        sttProfiles,
			"llm_profiles":        llmProfiles,
			"default_stt_profile": cfg.STT.DefaultProfile,
			"default_llm_profile": cfg.LLM.DefaultProfile,
		})
	})

	// --- Protected routes (API key required) ---
	r.Group(func(r chi.Router) {
		r.Use(apiKeyMiddleware)

			r.Route("/v1", func(r chi.Router) {
				r.Get("/health", handler.HealthCheck)
				r.Get("/version", handler.VersionCheck)
				r.Get("/ready", handler.ReadyCheck)

			// POST /v1/rooms/join — join or create a session by room code.
			// Promotes the former /test/start logic to a stable, versioned endpoint.
			r.Post("/rooms/join", func(w http.ResponseWriter, req *http.Request) {
				var body struct {
					Code       string `json:"code"`
					Name       string `json:"name"`
					Role       string `json:"role"`
					TemplateID string `json:"template_id"`
					STTProfile string `json:"stt_profile"`
					LLMProfile string `json:"llm_profile"`
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
					for _, ro := range tmpl.Roles {
						if ro.Key != body.Role {
							otherRole = ro.Key
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
						STTProfile:       body.STTProfile,
						LLMProfile:       body.LLMProfile,
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

	// Static file server for test-ui.
	// Using r.NotFound ensures the file server is only invoked when no API route matched,
	// preventing the wildcard from shadowing /v1/*, /demo/*, /signaling etc.
	uiPath := findTestUIPath()
	if uiPath != "" {
		fs := http.FileServer(http.Dir(uiPath))
		r.Get("/", fs.ServeHTTP)
		r.NotFound(fs.ServeHTTP)
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)

	return &Server{
		router:      r,
		botServer:   botServer,
		tlsCertFile: cfg.TLS.CertFile,
		tlsKeyFile:  cfg.TLS.KeyFile,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// findTestUIPath searches for the cmd/test-ui/dist directory (Vite build output)
// relative to common locations. Falls back to cmd/test-ui for dev convenience.
func findTestUIPath() string {
	bases := []string{
		"./cmd/test-ui",
		"../cmd/test-ui",
	}

	// Also try relative to the executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		bases = append(bases,
			filepath.Join(exeDir, "cmd/test-ui"),
			filepath.Join(exeDir, "../cmd/test-ui"),
		)
	}

	// Prefer the Vite build output (dist/) over the raw source directory.
	var candidates []string
	for _, b := range bases {
		candidates = append(candidates, filepath.Join(b, "dist"), b)
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

// ListenAndServe starts the server.
// If tls.cert_file and tls.key_file are both configured, it serves HTTPS/WSS.
// If the files are configured but missing or unreadable, it returns an error
// immediately — it never silently falls back to plain HTTP.
// Leave both fields empty to run as plain HTTP (e.g. behind a reverse proxy).
func (s *Server) ListenAndServe() error {
	if s.tlsCertFile != "" && s.tlsKeyFile != "" {
		if _, err := os.Stat(s.tlsCertFile); err != nil {
			return fmt.Errorf("TLS cert file not found (%s): %w", s.tlsCertFile, err)
		}
		if _, err := os.Stat(s.tlsKeyFile); err != nil {
			return fmt.Errorf("TLS key file not found (%s): %w", s.tlsKeyFile, err)
		}
		return s.httpServer.ListenAndServeTLS(s.tlsCertFile, s.tlsKeyFile)
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
