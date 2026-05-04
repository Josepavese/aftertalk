package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/Josepavese/aftertalk/internal/core/session"
	"github.com/Josepavese/aftertalk/internal/logging"
)

type SessionService interface {
	CreateSession(ctx context.Context, req *session.CreateSessionRequest) (*session.CreateSessionResponse, error)
	GetSession(ctx context.Context, sessionID string) (*session.Session, error)
	EndSession(ctx context.Context, sessionID string) error
	ValidateParticipant(ctx context.Context, jti string) (*session.Participant, error)
	ConnectParticipant(ctx context.Context, participantID string) error
	ListSessions(ctx context.Context, status string, limit, offset int) ([]*session.Session, int, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

type SessionHandler struct {
	service SessionService
}

func NewSessionHandler(service SessionService) *SessionHandler {
	return &SessionHandler{service: service}
}

func (h *SessionHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListSessions)
	r.Post("/", h.CreateSession)
	r.Get("/{id}", h.GetSession)
	r.Get("/{id}/status", h.GetSessionStatus)
	r.Post("/{id}/end", h.EndSession)
	r.Delete("/{id}", h.DeleteSession)
	return r
}

type CreateSessionRequest struct {
	TemplateID       string               `json:"template_id,omitempty"`
	Metadata         string               `json:"metadata,omitempty"`
	Participants     []ParticipantRequest `json:"participants"`
	ParticipantCount int                  `json:"participant_count"`
	STTProfile       string               `json:"stt_profile,omitempty"`
	LLMProfile       string               `json:"llm_profile,omitempty"`
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
		writeError(w, http.StatusBadRequest, "at least 2 participants required")
		return
	}

	// Validate participants.
	for i, p := range req.Participants {
		if p.UserID == "" {
			writeError(w, http.StatusBadRequest, "participant user_id is required")
			return
		}
		if p.Role == "" {
			writeError(w, http.StatusBadRequest, "participant role is required")
			return
		}
		if len(p.UserID) > 128 {
			writeError(w, http.StatusBadRequest, "participant user_id too long (max 128)")
			return
		}
		if len(p.Role) > 64 {
			writeError(w, http.StatusBadRequest, "participant role too long (max 64)")
			return
		}
		_ = i
	}
	if req.ParticipantCount > 0 && req.ParticipantCount != len(req.Participants) {
		writeError(w, http.StatusBadRequest, "participant_count does not match number of participants")
		return
	}

	createReq := &session.CreateSessionRequest{
		ParticipantCount: req.ParticipantCount,
		TemplateID:       req.TemplateID,
		Participants:     make([]session.ParticipantRequest, len(req.Participants)),
		Metadata:         req.Metadata,
		STTProfile:       req.STTProfile,
		LLMProfile:       req.LLMProfile,
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

func (h *SessionHandler) GetSessionStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Session ID required")
		return
	}

	sess, err := h.service.GetSession(r.Context(), sessionID)
	if err != nil {
		logging.Errorf("Failed to get session status: %v", err)
		writeError(w, http.StatusNotFound, "session not found: "+err.Error())
		return
	}

	render.JSON(w, r, map[string]string{"id": sess.ID, "status": string(sess.Status)})
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

func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	sessions, total, err := h.service.ListSessions(r.Context(), status, limit, offset)
	if err != nil {
		logging.Errorf("Failed to list sessions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list sessions: "+err.Error())
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"sessions": sessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *SessionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Session ID required")
		return
	}

	if err := h.service.DeleteSession(r.Context(), sessionID); err != nil {
		logging.Errorf("Failed to delete session: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete session: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
