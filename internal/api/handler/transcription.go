package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/flowup/aftertalk/internal/core/transcription"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type TranscriptionService interface {
	GetTranscriptions(ctx context.Context, sessionID string) ([]*transcription.Transcription, error)
	GetTranscriptionByID(ctx context.Context, id string) (*transcription.Transcription, error)
}

type TranscriptionHandler struct {
	service TranscriptionService
}

func NewTranscriptionHandler(service TranscriptionService) *TranscriptionHandler {
	return &TranscriptionHandler{service: service}
}

func (h *TranscriptionHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetTranscriptions)
	r.Get("/{id}", h.GetTranscriptionByID)
	return r
}

func (h *TranscriptionHandler) GetTranscriptions(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Session ID required")
		return
	}

	all, err := h.service.GetTranscriptions(r.Context(), sessionID)
	if err != nil {
		logging.Errorf("Failed to get transcriptions: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	total := len(all)
	limit := total
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	if offset >= total {
		all = nil
	} else {
		all = all[offset:]
		if limit < len(all) {
			all = all[:limit]
		}
	}

	render.JSON(w, r, map[string]interface{}{
		"transcriptions": all,
		"total":          total,
		"limit":          limit,
		"offset":         offset,
	})
}

func (h *TranscriptionHandler) GetTranscriptionByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Transcription ID required")
		return
	}

	t, err := h.service.GetTranscriptionByID(r.Context(), id)
	if err != nil {
		logging.Errorf("Failed to get transcription: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	render.JSON(w, r, t)
}
