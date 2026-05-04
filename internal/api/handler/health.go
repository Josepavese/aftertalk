package handler

import (
	"net/http"

	"github.com/go-chi/render"

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
