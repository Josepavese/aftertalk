package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/version"
)

func TestHealthCheck(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	HealthCheck(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, version.Current, response["version"])
	assert.Equal(t, "dev", response["commit"])
	assert.Equal(t, "dev", response["tag"])
	assert.Equal(t, "local", response["build_source"])
}

func TestVersionCheck(t *testing.T) {
	req := httptest.NewRequest("GET", "/version", nil)
	rec := httptest.NewRecorder()

	VersionCheck(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, version.Current, response["version"])
	assert.Equal(t, "dev", response["commit"])
	assert.Equal(t, "dev", response["tag"])
	assert.Equal(t, "", response["build_time"])
	assert.Equal(t, "local", response["build_source"])
}

func TestReadyCheck(t *testing.T) {
	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()

	ReadyCheck(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ready", response["status"])
}

func TestNewReadyCheck_DetailsReady(t *testing.T) {
	sttRegistry := stt.NewSTTRegistryFromProvider(stt.NewStubProvider())
	llmRegistry := llm.NewLLMRegistryFromProvider(llm.NewStubProvider())
	req := httptest.NewRequest("GET", "/ready?details=1", nil)
	rec := httptest.NewRecorder()

	NewReadyCheck(sttRegistry, llmRegistry)(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "ready", response["status"])
	profiles := response["profiles"].(map[string]interface{})
	assert.Len(t, profiles["stt"], 1)
	assert.Len(t, profiles["llm"], 1)
}

func TestNewReadyCheck_DetailsDegraded(t *testing.T) {
	sttRegistry := stt.NewSTTRegistryFromProvider(stt.NewStubProvider())
	llmRegistry := llm.NewLLMRegistryFromProvider(llm.NewOpenAIProvider("", "gpt-4"))
	req := httptest.NewRequest("GET", "/ready?details=1", nil)
	rec := httptest.NewRecorder()

	NewReadyCheck(sttRegistry, llmRegistry)(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "degraded", response["status"])
}
