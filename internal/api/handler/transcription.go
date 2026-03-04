package handler

import (
	"net/http"

	"github.com/flowup/aftertalk/internal/core/transcription"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type TranscriptionHandler struct {
	service *transcription.Service
}

func NewTranscriptionHandler(service *transcription.Service) *TranscriptionHandler {
	return &TranscriptionHandler{service: service}
}

func (h *TranscriptionHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetTranscriptions)
	r.Get("/{id}", h.GetTranscriptionByID)
	return r
}

func (h *TranscriptionHandler) GetTranscriptions(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	transcriptions, err := h.service.GetTranscriptions(r.Context(), sessionID)
	if err != nil {
		logging.Errorf("Failed to get transcriptions: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, transcriptions)
}

func (h *TranscriptionHandler) GetTranscriptionByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Transcription ID required", http.StatusBadRequest)
		return
	}

	transcription, err := h.service.GetTranscriptionByID(r.Context(), id)
	if err != nil {
		logging.Errorf("Failed to get transcription: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	render.JSON(w, r, transcription)
}
