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

// Probe modes — delaySeconds is orthogonal (applied before the status decision, except latched startup).
const (
	ProbeModeOK    = "ok"
	ProbeModeFail  = "fail"
	ProbeModeDelay = "delay" // same outcome as ok; name makes "timeout testing" configs obvious
	ProbeModeFlap  = "flap"  // alternate ok/fail on a timer or every N requests
)

const (
	maxProbeDelaySec = 30.0
	defaultFlapSec   = 5.0
)

// ProbeConfig is the knobs for one probe type (live / ready / startup).
type ProbeConfig struct {
	Mode         string  `json:"mode"`         // ok | fail | delay | flap
	DelaySeconds float64 `json:"delaySeconds"` // sleep before answering (0–30)
	// Startup only: wall-clock seconds from process start before a success is allowed.
	BootDelaySeconds float64 `json:"bootDelaySeconds,omitempty"`
	// Flap (mode=flap):
	//   flapSeconds — wall-clock half-period: ok for N sec, fail for N sec, repeat (default 5).
	//   flapEvery   — if > 0, every Nth request fails instead of using the clock (handy for tests).
	FlapSeconds float64 `json:"flapSeconds,omitempty"`
	FlapEvery   int     `json:"flapEvery,omitempty"`
}

// ProbeSnapshot is what /a/control/probes returns (includes runtime bits).
type ProbeSnapshot struct {
	Live             ProbeConfig `json:"live"`
	Ready            ProbeConfig `json:"ready"`
	Startup          ProbeConfig `json:"startup"`
	StartupLatched   bool        `json:"startupLatched"`
	UptimeSeconds    float64     `json:"uptimeSeconds"`
	StartupWouldPass bool        `json:"startupWouldPass"`
	// Which half of a time-based flap we are in right now (ok|fail|n/a).
	LiveFlapPhase    string `json:"liveFlapPhase,omitempty"`
	ReadyFlapPhase   string `json:"readyFlapPhase,omitempty"`
	StartupFlapPhase string `json:"startupFlapPhase,omitempty"`
}

// ProbeUpdate is the body for PUT /a/control/probes (all fields optional).
type ProbeUpdate struct {
	Live              *ProbeConfig `json:"live,omitempty"`
	Ready             *ProbeConfig `json:"ready,omitempty"`
	Startup           *ProbeConfig `json:"startup,omitempty"`
	ResetStartupLatch *bool        `json:"resetStartupLatch,omitempty"`
}

type probeState struct {
	mu             sync.Mutex
	live           ProbeConfig
	ready          ProbeConfig
	startup        ProbeConfig
	startupLatched bool
	startedAt      time.Time
	// request counters for flapEvery
	liveHits    int
	readyHits   int
	startupHits int
}

var probes = &probeState{
	live:      ProbeConfig{Mode: ProbeModeOK},
	ready:     ProbeConfig{Mode: ProbeModeOK},
	startup:   ProbeConfig{Mode: ProbeModeOK},
	startedAt: time.Now(),
}

// InitProbesFromEnv seeds probe config from environment (call once at process start).
//
//	LIVE_MODE / HEALTHY_MODE / HEALTHY   + LIVE_DELAY + LIVE_FLAP_SECONDS / LIVE_FLAP_EVERY
//	READY_MODE / READY                  + READY_DELAY + READY_FLAP_*
//	STARTUP_MODE / STARTUP              + STARTUP_DELAY + STARTUP_BOOT_DELAY + STARTUP_FLAP_*
func InitProbesFromEnv() {
	probes.mu.Lock()
	defer probes.mu.Unlock()

	probes.startedAt = time.Now()
	probes.startupLatched = false
	probes.liveHits = 0
	probes.readyHits = 0
	probes.startupHits = 0

	probes.live = ProbeConfig{
		Mode:         envMode("LIVE_MODE", "HEALTHY_MODE", "HEALTHY"),
		DelaySeconds: envDelay("LIVE_DELAY", "HEALTHY_DELAY"),
		FlapSeconds:  envDelay("LIVE_FLAP_SECONDS"),
		FlapEvery:    envInt("LIVE_FLAP_EVERY"),
	}
	probes.ready = ProbeConfig{
		Mode:         envMode("READY_MODE", "READY"),
		DelaySeconds: envDelay("READY_DELAY"),
		FlapSeconds:  envDelay("READY_FLAP_SECONDS"),
		FlapEvery:    envInt("READY_FLAP_EVERY"),
	}
	probes.startup = ProbeConfig{
		Mode:             envMode("STARTUP_MODE", "STARTUP"),
		DelaySeconds:     envDelay("STARTUP_DELAY"),
		BootDelaySeconds: envDelay("STARTUP_BOOT_DELAY"),
		FlapSeconds:      envDelay("STARTUP_FLAP_SECONDS"),
		FlapEvery:        envInt("STARTUP_FLAP_EVERY"),
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

func envInt(keys ...string) int {
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			n, err := strconv.Atoi(v)
			if err == nil && n > 0 {
				return n
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
	case ProbeModeFlap, "flapping", "oscillate":
		return ProbeModeFlap
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
	if c.FlapSeconds < 0 {
		c.FlapSeconds = 0
	}
	if c.FlapSeconds > maxProbeDelaySec {
		c.FlapSeconds = maxProbeDelaySec
	}
	if c.FlapEvery < 0 {
		c.FlapEvery = 0
	}
	return c
}

func (c ProbeConfig) isHardFail() bool {
	return c.Mode == ProbeModeFail
}

func (c ProbeConfig) isFlap() bool {
	return c.Mode == ProbeModeFlap
}

// flapFail decides if this request is on the fail half of a flap.
// hitCount should already include this request when using flapEvery.
func (c ProbeConfig) flapFail(startedAt time.Time, hitCount int) bool {
	c = c.normalized()
	if c.FlapEvery > 0 {
		// every Nth request fails: hits  N, 2N, 3N, ...
		return hitCount > 0 && hitCount%c.FlapEvery == 0
	}
	period := c.FlapSeconds
	if period <= 0 {
		period = defaultFlapSec
	}
	// even windows = ok, odd windows = fail
	phase := int(time.Since(startedAt).Seconds()/period) % 2
	return phase == 1
}

func (c ProbeConfig) flapPhase(startedAt time.Time, hitCount int) string {
	if !c.isFlap() {
		return ""
	}
	if c.flapFail(startedAt, hitCount) {
		return "fail"
	}
	return "ok"
}

// applyProbe runs delay + status for live/ready (not startup).
// name is "live" or "ready" for hit counters.
func applyProbe(c *gin.Context, name, queryKey, okBody, failBody string) {
	// one-shot query overrides first (no counter bump needed for pure override... still bump so flapEvery stays predictable)
	probes.mu.Lock()
	var cfg ProbeConfig
	var hits int
	switch name {
	case "live":
		probes.liveHits++
		hits = probes.liveHits
		cfg = probes.live.normalized()
	default:
		probes.readyHits++
		hits = probes.readyHits
		cfg = probes.ready.normalized()
	}
	startedAt := probes.startedAt
	probes.mu.Unlock()

	// query overrides (curl only; kube never sends these)
	if q := c.Query("ok"); q != "" {
		sleepDelay(cfg.DelaySeconds)
		if !isTruthy(q) {
			c.String(http.StatusServiceUnavailable, failBody)
			return
		}
		c.String(http.StatusOK, okBody)
		return
	}
	if q := c.Query(queryKey); q != "" {
		sleepDelay(cfg.DelaySeconds)
		if !isTruthy(q) {
			c.String(http.StatusServiceUnavailable, failBody)
			return
		}
		c.String(http.StatusOK, okBody)
		return
	}

	sleepDelay(cfg.DelaySeconds)

	fail := cfg.isHardFail()
	if cfg.isFlap() {
		fail = cfg.flapFail(startedAt, hits)
	}

	if fail {
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
// @Description Kube liveness. 200 = fine; 503 = restart. Modes: ok|fail|delay|flap. LIVE_* env or /a/control/probes. Aliases: /healthz /health
// @ID livez
// @Produce plain
// @Param ok query string false "one-shot force fail with 0/false (curl only)"
// @Success 200 {string} string "ok"
// @Failure 503 {string} string "unhealthy"
// @Router /livez [get]
func LiveHandler(c *gin.Context) {
	applyProbe(c, "live", "healthy", "ok", "unhealthy")
}

func HealthHandler(c *gin.Context)  { LiveHandler(c) }
func HealthzHandler(c *gin.Context) { LiveHandler(c) }

// @Summary Readiness (readyz)
// @Description Kube readiness. 200 = take traffic; 503 = leave Service endpoints. Modes: ok|fail|delay|flap. Alias: /ready
// @ID readyz
// @Produce plain
// @Param ok query string false "one-shot force fail (curl only)"
// @Success 200 {string} string "ready"
// @Failure 503 {string} string "not ready"
// @Router /readyz [get]
func ReadyHandler(c *gin.Context) {
	applyProbe(c, "ready", "ready", "ready", "not ready")
}

func ReadyzHandler(c *gin.Context) { ReadyHandler(c) }

// @Summary Startup (startupz)
// @Description Cold start latch: 503 until bootDelay, then sticky 200. mode=fail never latches. mode=flap oscillates until a success latches (or forever if you only hit fail phases — use flap carefully here).
// @ID startupz
// @Produce plain
// @Success 200 {string} string "started"
// @Failure 503 {string} string "starting"
// @Router /startupz [get]
func StartupHandler(c *gin.Context) {
	probes.mu.Lock()
	cfg := probes.startup.normalized()
	latched := probes.startupLatched
	delay := cfg.DelaySeconds
	if latched && !cfg.isHardFail() {
		delay = 0
	}
	probes.startupHits++
	hits := probes.startupHits
	startedAt := probes.startedAt
	probes.mu.Unlock()

	sleepDelay(delay)

	probes.mu.Lock()
	defer probes.mu.Unlock()
	cfg = probes.startup.normalized()

	if cfg.isHardFail() {
		probes.startupLatched = false
		c.String(http.StatusServiceUnavailable, "startup failed")
		return
	}

	if probes.startupLatched {
		c.String(http.StatusOK, "started")
		return
	}

	elapsed := time.Since(probes.startedAt).Seconds()
	if cfg.BootDelaySeconds > 0 && elapsed < cfg.BootDelaySeconds {
		c.String(http.StatusServiceUnavailable, "starting")
		return
	}

	// flap during pre-latch: fail phases keep returning starting; ok phase latches
	if cfg.isFlap() && cfg.flapFail(startedAt, hits) {
		c.String(http.StatusServiceUnavailable, "starting")
		return
	}

	probes.startupLatched = true
	c.String(http.StatusOK, "started")
}

// --- control API ---

// @Summary Get probe control state
// @Description Current live/ready/startup config + startup latch + flap phase + uptime
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
// @Description Partial update of live/ready/startup. resetStartupLatch re-runs cold start without restarting the process.
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
		probes.liveHits = 0
	}
	if upd.Ready != nil {
		probes.ready = upd.Ready.normalized()
		probes.readyHits = 0
	}
	if upd.Startup != nil {
		probes.startup = upd.Startup.normalized()
		probes.startupHits = 0
	}
	if upd.ResetStartupLatch != nil && *upd.ResetStartupLatch {
		probes.startupLatched = false
		probes.startedAt = time.Now()
		probes.startupHits = 0
	}
	if probes.startup.isHardFail() {
		probes.startupLatched = false
	}
	probes.mu.Unlock()

	c.JSON(http.StatusOK, snapshotProbes())
}

func snapshotProbes() ProbeSnapshot {
	probes.mu.Lock()
	defer probes.mu.Unlock()

	uptime := time.Since(probes.startedAt).Seconds()
	cfg := probes.startup.normalized()
	wouldPass := probes.startupLatched ||
		(!cfg.isHardFail() && (cfg.BootDelaySeconds <= 0 || uptime >= cfg.BootDelaySeconds))

	// phase uses current hit counts (next request will increment)
	return ProbeSnapshot{
		Live:             probes.live.normalized(),
		Ready:            probes.ready.normalized(),
		Startup:          cfg,
		StartupLatched:   probes.startupLatched,
		UptimeSeconds:    uptime,
		StartupWouldPass: wouldPass && !cfg.isHardFail(),
		LiveFlapPhase:    probes.live.normalized().flapPhase(probes.startedAt, probes.liveHits+1),
		ReadyFlapPhase:   probes.ready.normalized().flapPhase(probes.startedAt, probes.readyHits+1),
		StartupFlapPhase: probes.startup.normalized().flapPhase(probes.startedAt, probes.startupHits+1),
	}
}
