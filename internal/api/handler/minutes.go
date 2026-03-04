package handler

import (
	"encoding/json"
	"net/http"

	"github.com/flowup/aftertalk/internal/core/minutes"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type MinutesHandler struct {
	service *minutes.Service
}

func NewMinutesHandler(service *minutes.Service) *MinutesHandler {
	return &MinutesHandler{service: service}
}

func (h *MinutesHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetMinutes)
	r.Get("/{id}", h.GetMinutesByID)
	r.Put("/{id}", h.UpdateMinutes)
	r.Get("/{id}/versions", h.GetMinutesHistory)
	return r
}

func (h *MinutesHandler) GetMinutes(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	minutes, err := h.service.GetMinutes(r.Context(), sessionID)
	if err != nil {
		logging.Errorf("Failed to get minutes: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	render.JSON(w, r, minutes)
}

func (h *MinutesHandler) GetMinutesByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Minutes ID required", http.StatusBadRequest)
		return
	}

	minutes, err := h.service.GetMinutesByID(r.Context(), id)
	if err != nil {
		logging.Errorf("Failed to get minutes: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	render.JSON(w, r, minutes)
}

func (h *MinutesHandler) UpdateMinutes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Minutes ID required", http.StatusBadRequest)
		return
	}

	var req minutes.Minutes
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	editedBy := r.Header.Get("X-User-ID")
	if editedBy == "" {
		editedBy = "unknown"
	}

	updated, err := h.service.UpdateMinutes(r.Context(), id, &req, editedBy)
	if err != nil {
		logging.Errorf("Failed to update minutes: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, updated)
}

func (h *MinutesHandler) GetMinutesHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Minutes ID required", http.StatusBadRequest)
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
