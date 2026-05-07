package handler

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/version"
)

type healthResponse struct {
	Status string `json:"status"`
	version.BuildInfo
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, healthResponse{
		Status:    "ok",
		BuildInfo: version.Info(),
	})
}

func VersionCheck(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, version.Info())
}

func ReadyCheck(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]string{
		"status": "ready",
	})
}

type readinessResponse struct {
	Profiles readinessProfiles `json:"profiles,omitempty"`
	Status   string            `json:"status"`
}

type readinessProfiles struct {
	STT []stt.ProfileStatus `json:"stt"`
	LLM []llm.ProfileStatus `json:"llm"`
}

func NewReadyCheck(sttRegistry *stt.STTRegistry, llmRegistry *llm.LLMRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("details") == "" {
			ReadyCheck(w, r)
			return
		}

		resp := readinessResponse{Status: "ready"}
		if sttRegistry != nil {
			resp.Profiles.STT = sttRegistry.Readiness()
		}
		if llmRegistry != nil {
			resp.Profiles.LLM = llmRegistry.Readiness()
		}
		if hasUnhealthySTT(resp.Profiles.STT) || hasUnhealthyLLM(resp.Profiles.LLM) {
			resp.Status = "degraded"
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		render.JSON(w, r, resp)
	}
}

func hasUnhealthySTT(statuses []stt.ProfileStatus) bool {
	for _, s := range statuses {
		if !s.Available {
			return true
		}
	}
	return false
}

func hasUnhealthyLLM(statuses []llm.ProfileStatus) bool {
	for _, s := range statuses {
		if !s.Available {
			return true
		}
	}
	return false
}
