package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/Josepavese/aftertalk/internal/core/minutes"
	"github.com/Josepavese/aftertalk/internal/logging"
)

// MinutesService is the interface required by MinutesHandler.
// It is implemented by minutes.Service.
type MinutesService interface {
	GetMinutes(ctx context.Context, sessionID string) (*minutes.Minutes, error)
	GetMinutesByID(ctx context.Context, id string) (*minutes.Minutes, error)
	UpdateMinutes(ctx context.Context, id string, updatedMinutes *minutes.Minutes, editedBy string) (*minutes.Minutes, error)
	GetMinutesHistory(ctx context.Context, minutesID string) ([]*minutes.MinutesHistory, error)
	DeleteMinutes(ctx context.Context, id string) error
	// ConsumeRetrievalToken validates and atomically marks a pull token as used.
	// Returns a generic error (not distinguishing invalid/expired/used) to prevent oracle attacks.
	ConsumeRetrievalToken(ctx context.Context, tokenID string) (*minutes.RetrievalToken, error)
	// PurgeMinutes deletes minutes, transcriptions, and retrieval tokens for the
	// given minutes ID. Called after a successful pull when delete_on_pull is true.
	PurgeMinutes(ctx context.Context, minutesID string)
}

// MinutesHandler handles all HTTP routes for the minutes resource.
type MinutesHandler struct {
	service      MinutesService
	deleteOnPull bool // true → purge DB after a successful pull (notify_pull mode)
}

func NewMinutesHandler(service MinutesService) *MinutesHandler {
	return &MinutesHandler{service: service}
}

// NewMinutesHandlerWithConfig creates a handler with the delete-on-pull flag
// derived from the webhook configuration.
func NewMinutesHandlerWithConfig(service MinutesService, deleteOnPull bool) *MinutesHandler {
	return &MinutesHandler{service: service, deleteOnPull: deleteOnPull}
}

func (h *MinutesHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetMinutes)
	r.Get("/{id}", h.GetMinutesByID)
	r.Put("/{id}", h.UpdateMinutes)
	r.Delete("/{id}", h.DeleteMinutes)
	r.Get("/{id}/versions", h.GetMinutesHistory)
	return r
}

func writeError(w http.ResponseWriter, code int, msg string) {
	http.Error(w, msg, code)
}

func (h *MinutesHandler) GetMinutes(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Session ID required")
		return
	}

	m, err := h.service.GetMinutes(r.Context(), sessionID)
	if err != nil {
		logging.Errorf("Failed to get minutes: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	render.JSON(w, r, m)
}

func (h *MinutesHandler) GetMinutesByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Minutes ID required")
		return
	}

	m, err := h.service.GetMinutesByID(r.Context(), id)
	if err != nil {
		logging.Errorf("Failed to get minutes: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	render.JSON(w, r, m)
}

func (h *MinutesHandler) UpdateMinutes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Minutes ID required")
		return
	}

	var req minutes.Minutes
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	editedBy := r.Header.Get("X-User-Id")
	if editedBy == "" {
		editedBy = "unknown"
	}

	existing, err := h.service.GetMinutesByID(r.Context(), id)
	if err != nil {
		logging.Errorf("Failed to get minutes: %v", err)
		http.Error(w, "failed to get minutes: "+err.Error(), http.StatusNotFound)
		return
	}
	_ = existing

	updated, err := h.service.UpdateMinutes(r.Context(), id, &req, editedBy)
	if err != nil {
		logging.Errorf("Failed to update minutes: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, updated)
}

func (h *MinutesHandler) DeleteMinutes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Minutes ID required")
		return
	}

	if err := h.service.DeleteMinutes(r.Context(), id); err != nil {
		logging.Errorf("Failed to delete minutes: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete minutes: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *MinutesHandler) GetMinutesHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Minutes ID required")
		return
	}

	history, err := h.service.GetMinutesHistory(r.Context(), id)
	if err != nil {
		logging.Errorf("Failed to get minutes history: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if history == nil {
		history = []*minutes.MinutesHistory{}
	}
	render.JSON(w, r, history)
}

// PullMinutes handles GET /v1/minutes/pull/{token}.
//
// This endpoint is part of the "notify_pull" secure delivery flow:
//  1. After minutes generation, a notification webhook is sent to the operator's
//     system containing a signed retrieval URL (no medical data).
//  2. The operator's server calls this endpoint using the token from that URL.
//  3. The minutes are returned once; the token is then permanently consumed.
//  4. If delete_on_pull is enabled (default for notify_pull mode), the minutes
//     and transcriptions are purged from the Aftertalk DB after delivery.
//
// Authentication: the token in the URL path IS the credential — no API key needed.
// All invalid/expired/used tokens return 404 (intentionally indistinguishable).
func (h *MinutesHandler) PullMinutes(w http.ResponseWriter, r *http.Request) {
	tokenID := chi.URLParam(r, "token")
	if tokenID == "" {
		http.NotFound(w, r)
		return
	}

	tok, err := h.service.ConsumeRetrievalToken(r.Context(), tokenID)
	if err != nil {
		// Intentionally generic 404 — do not leak whether the token existed.
		logging.Warnf("PullMinutes: invalid/consumed/expired token %s: %v", tokenID, err)
		http.NotFound(w, r)
		return
	}

	m, err := h.service.GetMinutesByID(r.Context(), tok.MinutesID)
	if err != nil {
		logging.Errorf("PullMinutes: fetch minutes %s: %v", tok.MinutesID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, m)

	// Purge after responding so the client receives the data even if purge fails.
	if h.deleteOnPull {
		go h.service.PurgeMinutes(r.Context(), tok.MinutesID)
	}
}
