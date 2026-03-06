package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/flowup/aftertalk/internal/api/handler"
	custommiddleware "github.com/flowup/aftertalk/internal/api/middleware"
	"github.com/flowup/aftertalk/internal/config"
	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/go-chi/render"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	httpServer *http.Server
	router     *chi.Mux
	botServer  *BotServer
}

func NewServer(cfg *config.Config, sessionService *session.Service, botServer *BotServer) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(custommiddleware.Logging)
	r.Use(custommiddleware.Recovery)
	r.Use(middleware.Timeout(60 * time.Second))

	if cfg.API.Key != "" {
		r.Use(custommiddleware.APIKey(cfg.API.Key))
	}

	sessionHandler := handler.NewSessionHandler(sessionService)

	r.Route("/v1", func(r chi.Router) {
		r.Get("/health", handler.HealthCheck)
		r.Get("/ready", handler.ReadyCheck)
		r.Mount("/sessions", sessionHandler.Routes())
	})

	// Demo test UI
	r.Handle("/demo/*", http.StripPrefix("/demo/", http.FileServer(http.Dir("./cmd/test-ui"))))

	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		botServer.HandleWebSocket(w, r)
	})

	r.Get("/signaling", func(w http.ResponseWriter, r *http.Request) {
		botServer.HandleWebSocket(w, r)
	})

	// Test endpoint for WebRTC testing - creates session and returns token
	r.Post("/test/start", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
			Role string `json:"role"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		
		// Create session via service
		sessReq := &session.CreateSessionRequest{
			ParticipantCount: 2,
			Participants: []session.ParticipantRequest{
				{UserID: req.Name, Role: req.Role},
				{UserID: "test-peer", Role: "speaker"},
			},
		}
		sess, err := sessionService.CreateSession(r.Context(), sessReq)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		
		// Find the participant with the matching name
		var participantToken string
		for _, p := range sess.Participants {
			if p.UserID == req.Name {
				participantToken = p.Token
				break
			}
		}
		if participantToken == "" && len(sess.Participants) > 0 {
			participantToken = sess.Participants[0].Token
		}
		
		render.JSON(w, r, map[string]string{
			"session_id": sess.SessionID,
			"token":      participantToken,
			"name":       req.Name,
		})
	})

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)


	return &Server{
		router:     r,
		botServer:  botServer,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	return s.httpServer.Shutdown(nil)
}
