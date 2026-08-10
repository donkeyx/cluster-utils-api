package main

import (
	"bytes"
	"github.com/donkeyx/cluster-utils-api/handlers"
	"github.com/donkeyx/cluster-utils-api/routes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(token string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handlers.SetBuildInfo("test-ver", "abc123")
	handlers.InitProbesFromEnv()
	// reset to known defaults for tests (env may not be set)
	resetProbesForTest()
	r := gin.New()
	logger := setupLogger()
	routes.SetupRouter(logger, token, r)
	return r
}

func resetProbesForTest() {
	ok := true
	body, _ := json.Marshal(handlers.ProbeUpdate{
		Live:              &handlers.ProbeConfig{Mode: "ok", DelaySeconds: 0},
		Ready:             &handlers.ProbeConfig{Mode: "ok", DelaySeconds: 0},
		Startup:           &handlers.ProbeConfig{Mode: "ok", DelaySeconds: 0, BootDelaySeconds: 0},
		ResetStartupLatch: &ok,
	})
	// use handler internals via HTTP would need router; call Put via package by spinning mini router
	// simpler: hit control after router exists — for setup, use PUT through a throwaway engine
	r := gin.New()
	r.PUT("/p", handlers.PutProbesHandler)
	req := httptest.NewRequest(http.MethodPut, "/p", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
}

func TestHealthEndpoint(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

func TestLivezAlias(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

func TestHealthUnhealthyQuery(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/healthz?ok=0", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "unhealthy", rr.Body.String())
}

func TestReadyNotReadyQuery(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/ready?ready=false", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "not ready", rr.Body.String())
}

func TestReadyFailViaControl(t *testing.T) {
	token := "tok"
	r := setupTestRouter(token)

	body := `{"ready":{"mode":"fail","delaySeconds":0}}`
	req := httptest.NewRequest(http.MethodPut, "/a/control/probes", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusServiceUnavailable, rr2.Code)
	assert.Equal(t, "not ready", rr2.Body.String())
}

func TestReadyFlapEvery(t *testing.T) {
	token := "tok"
	r := setupTestRouter(token)

	// every 2nd request fails
	body := `{"ready":{"mode":"flap","delaySeconds":0,"flapEvery":2}}`
	req := httptest.NewRequest(http.MethodPut, "/a/control/probes", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// hit 1 ok, hit 2 fail, hit 3 ok, hit 4 fail
	codes := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		reqN := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rrN := httptest.NewRecorder()
		r.ServeHTTP(rrN, reqN)
		codes = append(codes, rrN.Code)
	}
	assert.Equal(t, []int{200, 503, 200, 503}, codes)
}

func TestStartupLatchAndBootDelay(t *testing.T) {
	token := "tok"
	r := setupTestRouter(token)

	// set boot delay into the future
	body := `{"startup":{"mode":"ok","delaySeconds":0,"bootDelaySeconds":2},"resetStartupLatch":true}`
	req := httptest.NewRequest(http.MethodPut, "/a/control/probes", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// still starting
	req2 := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusServiceUnavailable, rr2.Code)
	assert.Equal(t, "starting", rr2.Body.String())

	time.Sleep(2100 * time.Millisecond)

	// first success latches
	req3 := httptest.NewRequest(http.MethodGet, "/startup", nil)
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusOK, rr3.Code)
	assert.Equal(t, "started", rr3.Body.String())

	// sticky after latch even if we can't easily re-apply boot without reset
	req4 := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	rr4 := httptest.NewRecorder()
	r.ServeHTTP(rr4, req4)
	assert.Equal(t, http.StatusOK, rr4.Code)
}

func TestStartupFailNeverLatches(t *testing.T) {
	token := "tok"
	r := setupTestRouter(token)

	body := `{"startup":{"mode":"fail","delaySeconds":0,"bootDelaySeconds":0},"resetStartupLatch":true}`
	req := httptest.NewRequest(http.MethodPut, "/a/control/probes", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusServiceUnavailable, rr2.Code)
	assert.Equal(t, "startup failed", rr2.Body.String())
}

func TestGetProbesAuth(t *testing.T) {
	token := "secret"
	r := setupTestRouter(token)

	req := httptest.NewRequest(http.MethodGet, "/a/control/probes", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/a/control/probes", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
	assert.Contains(t, rr2.Body.String(), "startupLatched")
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

	req := httptest.NewRequest(http.MethodGet, "/a/env", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

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
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "http_request_duration_seconds")
	assert.Contains(t, body, "go_goroutines")
}

func TestProxyRequiresAuth(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/a/proxy?url=http://example.com/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestProxyHopToSelf(t *testing.T) {
	token := "tok"
	gin.SetMode(gin.TestMode)
	handlers.SetBuildInfo("test-ver", "abc123")
	handlers.InitProbesFromEnv()
	resetProbesForTest()
	r := gin.New()
	logger := setupLogger()
	routes.SetupRouter(logger, token, r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	proxyURL := srv.URL + "/a/proxy?url=" + srv.URL + "/debug"
	req, err := http.NewRequest(http.MethodGet, proxyURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Trace-Demo", "east-west-1")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var wrap map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	assert.Contains(t, wrap, "request")
	assert.Contains(t, wrap, "response")
	assert.Contains(t, wrap, "meta")

	// wrap includes upstream headers map
	respObj := wrap["response"].(map[string]interface{})
	assert.Contains(t, respObj, "headers")
	assert.Contains(t, respObj, "status")
	assert.Contains(t, respObj, "body")

	body := respObj["body"].(string)
	assert.Contains(t, body, "X-Trace-Demo")
	assert.Contains(t, body, "east-west-1")

	// our bearer for this api should NOT be auto-forwarded east-west
	assert.NotContains(t, body, "Bearer "+token)
}

func TestProxyRequiresAbsoluteURL(t *testing.T) {
	r := setupTestRouter("tok")
	req := httptest.NewRequest(http.MethodGet, "/a/proxy?url=/debug", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
