package api

import (
	"fmt"
	"net/http"
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
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("WebSocket endpoint - Bot integration pending"))
	})

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)

	return &Server{
		router: r,
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
