package handler

import (
	"encoding/json"
	"net/http"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type SessionHandler struct {
	service *session.Service
}

func NewSessionHandler(service *session.Service) *SessionHandler {
	return &SessionHandler{service: service}
}

func (h *SessionHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.CreateSession)
	r.Get("/{id}", h.GetSession)
	return r
}

type CreateSessionRequest struct {
	ParticipantCount int                  `json:"participant_count"`
	Participants     []ParticipantRequest `json:"participants"`
	Metadata         string               `json:"metadata,omitempty"`
}

type ParticipantRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func (h *SessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	createReq := &session.CreateSessionRequest{
		ParticipantCount: req.ParticipantCount,
		Participants:     make([]session.ParticipantRequest, len(req.Participants)),
		Metadata:         req.Metadata,
	}

	for i, p := range req.Participants {
		createReq.Participants[i] = session.ParticipantRequest{
			UserID: p.UserID,
			Role:   p.Role,
		}
	}

	res, err := h.service.CreateSession(r.Context(), createReq)
	if err != nil {
		logging.Errorf("Failed to create session: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, res)
}

func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sess, err := h.service.GetSession(r.Context(), sessionID)
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	render.JSON(w, r, sess)
}
