package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type SessionService interface {
	CreateSession(ctx context.Context, req *session.CreateSessionRequest) (*session.CreateSessionResponse, error)
	GetSession(ctx context.Context, sessionID string) (*session.Session, error)
	EndSession(ctx context.Context, sessionID string) error
	ValidateParticipant(ctx context.Context, jti string) (*session.Participant, error)
	ConnectParticipant(ctx context.Context, participantID string) error
}

type SessionHandler struct {
	service SessionService
}

func NewSessionHandler(service SessionService) *SessionHandler {
	return &SessionHandler{service: service}
}

func (h *SessionHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.CreateSession)
	r.Get("/{id}", h.GetSession)
	r.Post("/{id}/end", h.EndSession)
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
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Participants) < 2 {
		writeError(w, http.StatusInternalServerError, "at least 2 participants required")
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
		writeError(w, http.StatusBadRequest, "Session ID required")
		return
	}

	sess, err := h.service.GetSession(r.Context(), sessionID)
	if err != nil {
		logging.Errorf("Failed to get session: %v", err)
		writeError(w, http.StatusNotFound, "failed to get session: "+err.Error())
		return
	}

	render.JSON(w, r, sess)
}

func (h *SessionHandler) EndSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Session ID required")
		return
	}

	if err := h.service.EndSession(r.Context(), sessionID); err != nil {
		logging.Errorf("Failed to end session: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to end session: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
