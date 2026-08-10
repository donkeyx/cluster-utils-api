package handlers

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Probe modes — delay is orthogonal (always applied before the status decision).
const (
	ProbeModeOK    = "ok"
	ProbeModeFail  = "fail"
	ProbeModeDelay = "delay" // same outcome as ok; name makes "timeout testing" configs obvious
)

const maxProbeDelaySec = 30.0

// ProbeConfig is the knobs for one probe type (live / ready / startup).
type ProbeConfig struct {
	Mode         string  `json:"mode"`                   // ok | fail | delay
	DelaySeconds float64 `json:"delaySeconds"`           // sleep before answering (0–30)
	// Startup only: wall-clock seconds from process start before a success is allowed.
	// Simulates real cold start — kube keeps hitting startup until this elapses (or control forces ok).
	BootDelaySeconds float64 `json:"bootDelaySeconds,omitempty"`
}

// ProbeSnapshot is what /a/control/probes returns (includes runtime bits).
type ProbeSnapshot struct {
	Live           ProbeConfig `json:"live"`
	Ready          ProbeConfig `json:"ready"`
	Startup        ProbeConfig `json:"startup"`
	StartupLatched bool        `json:"startupLatched"`
	UptimeSeconds  float64     `json:"uptimeSeconds"`
	// True when startup would succeed *right now* (after boot delay / latch rules).
	StartupWouldPass bool `json:"startupWouldPass"`
}

// ProbeUpdate is the body for PUT /a/control/probes (all fields optional).
type ProbeUpdate struct {
	Live    *ProbeConfig `json:"live,omitempty"`
	Ready   *ProbeConfig `json:"ready,omitempty"`
	Startup *ProbeConfig `json:"startup,omitempty"`
	// Clear the startup latch so the next checks behave like a fresh process again.
	ResetStartupLatch *bool `json:"resetStartupLatch,omitempty"`
}

type probeState struct {
	mu               sync.RWMutex
	live             ProbeConfig
	ready            ProbeConfig
	startup          ProbeConfig
	startupLatched   bool
	startedAt        time.Time
}

var probes = &probeState{
	live:      ProbeConfig{Mode: ProbeModeOK},
	ready:     ProbeConfig{Mode: ProbeModeOK},
	startup:   ProbeConfig{Mode: ProbeModeOK},
	startedAt: time.Now(),
}

// InitProbesFromEnv seeds probe config from environment (call once at process start).
//
//	LIVE_MODE / HEALTHY_MODE / HEALTHY   + LIVE_DELAY / HEALTHY_DELAY
//	READY_MODE / READY                  + READY_DELAY
//	STARTUP_MODE / STARTUP              + STARTUP_DELAY + STARTUP_BOOT_DELAY
func InitProbesFromEnv() {
	probes.mu.Lock()
	defer probes.mu.Unlock()

	probes.startedAt = time.Now()
	probes.startupLatched = false

	probes.live = ProbeConfig{
		Mode:         envMode("LIVE_MODE", "HEALTHY_MODE", "HEALTHY"),
		DelaySeconds: envDelay("LIVE_DELAY", "HEALTHY_DELAY"),
	}
	probes.ready = ProbeConfig{
		Mode:         envMode("READY_MODE", "READY"),
		DelaySeconds: envDelay("READY_DELAY"),
	}
	probes.startup = ProbeConfig{
		Mode:             envMode("STARTUP_MODE", "STARTUP"),
		DelaySeconds:     envDelay("STARTUP_DELAY"),
		BootDelaySeconds: envDelay("STARTUP_BOOT_DELAY"),
	}
}

func envMode(keys ...string) string {
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			return normalizeMode(v)
		}
	}
	return ProbeModeOK
}

func envDelay(keys ...string) float64 {
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			f, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return clampDelay(f)
			}
		}
	}
	return 0
}

func normalizeMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ProbeModeFail, "0", "false", "no", "off", "unhealthy", "notready", "not-ready", "not ready":
		return ProbeModeFail
	case ProbeModeDelay, "slow":
		return ProbeModeDelay
	default:
		return ProbeModeOK
	}
}

func clampDelay(d float64) float64 {
	if d < 0 {
		return 0
	}
	if d > maxProbeDelaySec {
		return maxProbeDelaySec
	}
	return d
}

func (c ProbeConfig) normalized() ProbeConfig {
	c.Mode = normalizeMode(c.Mode)
	c.DelaySeconds = clampDelay(c.DelaySeconds)
	c.BootDelaySeconds = clampDelay(c.BootDelaySeconds)
	return c
}

func (c ProbeConfig) shouldFail() bool {
	return c.Mode == ProbeModeFail
}

// applyProbe runs delay + status for live/ready (not startup).
// Query overrides: ?ok=0 / ?ok=false force fail for this request only (handy for curl; kube never sends these).
func applyProbe(c *gin.Context, cfg ProbeConfig, queryKey, okBody, failBody string) {
	cfg = cfg.normalized()

	// one-shot query overrides (debug / manual only)
	if q := c.Query("ok"); q != "" {
		if !isTruthy(q) {
			sleepDelay(cfg.DelaySeconds)
			c.String(http.StatusServiceUnavailable, failBody)
			return
		}
	} else if q := c.Query(queryKey); q != "" {
		if !isTruthy(q) {
			sleepDelay(cfg.DelaySeconds)
			c.String(http.StatusServiceUnavailable, failBody)
			return
		}
	}

	sleepDelay(cfg.DelaySeconds)

	if cfg.shouldFail() {
		c.String(http.StatusServiceUnavailable, failBody)
		return
	}
	c.String(http.StatusOK, okBody)
}

func sleepDelay(seconds float64) {
	if seconds > 0 {
		time.Sleep(time.Duration(seconds * float64(time.Second)))
	}
}

// --- HTTP handlers: live / ready / startup ---

// @Summary Liveness (livez)
// @Description Kube liveness style. 200 = process fine; 503 = kube will restart. Config via LIVE_MODE/LIVE_DELAY or /a/control/probes. Aliases: /healthz /health
// @ID livez
// @Produce plain
// @Param ok query string false "one-shot force fail with 0/false (curl only; kube does not send this)"
// @Success 200 {string} string "ok"
// @Failure 503 {string} string "unhealthy"
// @Router /livez [get]
func LiveHandler(c *gin.Context) {
	probes.mu.RLock()
	cfg := probes.live
	probes.mu.RUnlock()
	applyProbe(c, cfg, "healthy", "ok", "unhealthy")
}

// HealthHandler kept as name for older tests/docs — same as live.
func HealthHandler(c *gin.Context)  { LiveHandler(c) }
func HealthzHandler(c *gin.Context) { LiveHandler(c) }

// @Summary Readiness (readyz)
// @Description Kube readiness style. 200 = take traffic; 503 = drop from Service endpoints (no restart). READY_MODE/READY_DELAY or control API. Alias: /ready
// @ID readyz
// @Produce plain
// @Param ok query string false "one-shot force fail (curl only)"
// @Success 200 {string} string "ready"
// @Failure 503 {string} string "not ready"
// @Router /readyz [get]
func ReadyHandler(c *gin.Context) {
	probes.mu.RLock()
	cfg := probes.ready
	probes.mu.RUnlock()
	applyProbe(c, cfg, "ready", "ready", "not ready")
}

func ReadyzHandler(c *gin.Context) { ReadyHandler(c) }

// @Summary Startup (startupz)
// @Description Real startup semantics: fails until boot delay elapses, then latches success until process restart or resetStartupLatch. After latch, always 200 (fast) so kube stops startup probes. Mode=fail never latches.
// @ID startupz
// @Produce plain
// @Success 200 {string} string "started"
// @Failure 503 {string} string "starting"
// @Router /startupz [get]
func StartupHandler(c *gin.Context) {
	// snapshot under read lock, sleep without holding the lock
	probes.mu.RLock()
	cfg := probes.startup.normalized()
	latched := probes.startupLatched
	delay := cfg.DelaySeconds
	if latched && !cfg.shouldFail() {
		delay = 0 // finished init: answer immediately
	}
	probes.mu.RUnlock()

	sleepDelay(delay)

	probes.mu.Lock()
	defer probes.mu.Unlock()
	cfg = probes.startup.normalized()

	// forced fail: never latch
	if cfg.shouldFail() {
		probes.startupLatched = false
		c.String(http.StatusServiceUnavailable, "startup failed")
		return
	}

	// already started — sticky success (real startup endpoint behaviour)
	if probes.startupLatched {
		c.String(http.StatusOK, "started")
		return
	}

	// still in cold-start window
	elapsed := time.Since(probes.startedAt).Seconds()
	if cfg.BootDelaySeconds > 0 && elapsed < cfg.BootDelaySeconds {
		c.String(http.StatusServiceUnavailable, "starting")
		return
	}

	// first success → latch until reset / process death
	probes.startupLatched = true
	c.String(http.StatusOK, "started")
}

// --- control API ---

// @Summary Get probe control state
// @Description Current live/ready/startup config + startup latch + uptime
// @ID getProbes
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ProbeSnapshot
// @Failure 401 {object} map[string]string
// @Router /a/control/probes [get]
func GetProbesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, snapshotProbes())
}

// @Summary Update probe control state
// @Description Partial update of live/ready/startup. Use resetStartupLatch to re-run cold start without restarting the process.
// @ID putProbes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ProbeUpdate true "probe update"
// @Success 200 {object} ProbeSnapshot
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /a/control/probes [put]
func PutProbesHandler(c *gin.Context) {
	var upd ProbeUpdate
	if err := c.ShouldBindJSON(&upd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json: " + err.Error()})
		return
	}

	probes.mu.Lock()
	if upd.Live != nil {
		probes.live = upd.Live.normalized()
	}
	if upd.Ready != nil {
		probes.ready = upd.Ready.normalized()
	}
	if upd.Startup != nil {
		// preserve boot delay if client omitted it (0 is valid though — use pointer fields ideally;
		// for simplicity: always take normalized startup update as full replacement of those fields)
		probes.startup = upd.Startup.normalized()
	}
	if upd.ResetStartupLatch != nil && *upd.ResetStartupLatch {
		probes.startupLatched = false
		probes.startedAt = time.Now() // restart the boot clock for another cold-start demo
	}
	// if startup forced to fail, drop latch
	if probes.startup.shouldFail() {
		probes.startupLatched = false
	}
	probes.mu.Unlock()

	c.JSON(http.StatusOK, snapshotProbes())
}

func snapshotProbes() ProbeSnapshot {
	probes.mu.RLock()
	defer probes.mu.RUnlock()

	uptime := time.Since(probes.startedAt).Seconds()
	cfg := probes.startup.normalized()
	wouldPass := probes.startupLatched ||
		(!cfg.shouldFail() && (cfg.BootDelaySeconds <= 0 || uptime >= cfg.BootDelaySeconds))

	return ProbeSnapshot{
		Live:             probes.live.normalized(),
		Ready:            probes.ready.normalized(),
		Startup:          cfg,
		StartupLatched:   probes.startupLatched,
		UptimeSeconds:    uptime,
		StartupWouldPass: wouldPass && !cfg.shouldFail(),
	}
}
