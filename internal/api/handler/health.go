package handler

import (
	"net/http"

	"github.com/flowup/aftertalk/internal/version"
	"github.com/go-chi/render"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]string{
		"status":  "ok",
		"version": version.Current,
	})
}

func ReadyCheck(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]string{
		"status": "ready",
	})
}
