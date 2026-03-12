package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flowup/aftertalk/internal/core/minutes"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type MinutesService interface {
	GetMinutes(ctx context.Context, sessionID string) (*minutes.Minutes, error)
	GetMinutesByID(ctx context.Context, id string) (*minutes.Minutes, error)
	UpdateMinutes(ctx context.Context, id string, updatedMinutes *minutes.Minutes, editedBy string) (*minutes.Minutes, error)
	GetMinutesHistory(ctx context.Context, minutesID string) ([]*minutes.MinutesHistory, error)
	DeleteMinutes(ctx context.Context, id string) error
}

type MinutesHandler struct {
	service MinutesService
}

func NewMinutesHandler(service MinutesService) *MinutesHandler {
	return &MinutesHandler{service: service}
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
	w.WriteHeader(code)
	fmt.Fprint(w, msg)
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

	editedBy := r.Header.Get("X-User-ID")
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

	render.JSON(w, r, history)
}
