package main

import (
	"cu-api/handlers"
	"cu-api/routes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(token string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handlers.SetBuildInfo("test-ver", "abc123")
	r := gin.New()
	logger := setupLogger()
	routes.SetupRouter(logger, token, r)
	return r
}

func TestHealthEndpoint(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestHealthUnhealthyQuery(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/healthz?ok=0", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "Unhealthy", rr.Body.String())
}

func TestReadyNotReadyQuery(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/ready?ready=false", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "Not Ready", rr.Body.String())
}

func TestVersion(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "test-ver")
	assert.Contains(t, rr.Body.String(), "abc123")
}

func TestStatusCode(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/status/418", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, 418, rr.Code)
	assert.Contains(t, rr.Body.String(), "418")
}

func TestStatusBadCode(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/status/999", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDelay(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/delay/0", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "delayed=")
}

func TestEcho(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodPost, "/echo?x=1", strings.NewReader(`{"hi":"there"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "POST")
	assert.Contains(t, body, "hi")
	assert.Contains(t, body, "x")
}

func TestEnvAuth(t *testing.T) {
	token := "secret-token"
	r := setupTestRouter(token)

	// no auth
	req := httptest.NewRequest(http.MethodGet, "/a/env", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// with auth
	req2 := httptest.NewRequest(http.MethodGet, "/a/env", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestPing(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "PONG", rr.Body.String())
}

func TestMetrics(t *testing.T) {
	r := setupTestRouter("tok")
	// hit something first so counter has labels
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "http_requests_total")
}
