package handlers

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// hop-by-hop headers we never copy outbound
var skipForwardHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"content-length":      true,
	"host":                true,
}

// credentials / session material — not forwarded unless forwardSensitiveHeaders=true
var sensitiveForwardHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
}

// ProxyRequest is the JSON body for POST /a/proxy.
// North-south hits this pod; we call another URL east-west (auth required — open proxy is SSRF).
type ProxyRequest struct {
	// Absolute URL to call, e.g. http://other-api:8080/debug
	URL string `json:"url" binding:"required"`
	// HTTP method (default GET)
	Method string `json:"method,omitempty"`
	// Extra headers to set/override on the outbound request
	Headers map[string]string `json:"headers,omitempty"`
	// Optional body (string; use for JSON text, form, etc.)
	Body string `json:"body,omitempty"`
	// Timeout for the outbound call (default 10; capped by MAX_DELAY_SECONDS)
	TimeoutSeconds float64 `json:"timeoutSeconds,omitempty"`
	// When true (default), copy inbound request headers onto the outbound call
	// (minus hop-by-hop). Tracing headers like X-Request-Id ride along.
	ForwardIncomingHeaders *bool `json:"forwardIncomingHeaders,omitempty"`
	// When true, also forward Authorization / Cookie from the inbound request.
	// Default false so your bearer token for *this* api is not sent east-west by accident.
	ForwardSensitiveHeaders bool `json:"forwardSensitiveHeaders,omitempty"`
	// When true, return the upstream body as the raw HTTP response (status + headers from upstream).
	// Default false → JSON wrap with request/response/meta (includes upstream headers + body).
	Raw bool `json:"raw,omitempty"`
}

// @Summary Proxy / hop to another service (auth)
// @Description Auth required. North→south hits this pod; this pod calls url east-west. Default JSON wrap includes upstream status, headers, and body. Open proxy would be SSRF — keep behind bearer.
// @ID proxy
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body ProxyRequest true "proxy request"
// @Param url query string false "absolute url (GET form)"
// @Param method query string false "HTTP method for GET form (default GET)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 502 {object} map[string]interface{}
// @Router /a/proxy [post]
// @Router /a/proxy [get]
// @Param Authorization header string true "Bearer token" default(Bearer )
func ProxyHandler(c *gin.Context) {
	var req ProxyRequest

	// GET convenience: /a/proxy?url=http://svc:8080/debug&method=GET
	if c.Request.Method == http.MethodGet {
		req.URL = c.Query("url")
		req.Method = c.DefaultQuery("method", http.MethodGet)
		if t := c.Query("timeoutSeconds"); t != "" {
			if f, err := strconv.ParseFloat(t, 64); err == nil {
				req.TimeoutSeconds = f
			}
		}
		if c.Query("forwardIncomingHeaders") == "0" || c.Query("forwardIncomingHeaders") == "false" {
			f := false
			req.ForwardIncomingHeaders = &f
		}
		if c.Query("forwardSensitiveHeaders") == "1" || c.Query("forwardSensitiveHeaders") == "true" {
			req.ForwardSensitiveHeaders = true
		}
		if c.Query("raw") == "1" || c.Query("raw") == "true" {
			req.Raw = true
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json: " + err.Error()})
			return
		}
	}

	if strings.TrimSpace(req.URL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}
	u, err := url.Parse(req.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url must be absolute, e.g. http://other-svc:8080/debug"})
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url scheme must be http or https"})
		return
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}
	timeout = clampProxyTimeout(timeout)

	forward := true
	if req.ForwardIncomingHeaders != nil {
		forward = *req.ForwardIncomingHeaders
	}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	outReq, err := http.NewRequestWithContext(c.Request.Context(), method, req.URL, bodyReader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1) optional: copy north-south inbound headers → east-west outbound
	if forward {
		for k, vals := range c.Request.Header {
			lk := strings.ToLower(k)
			if skipForwardHeaders[lk] {
				continue
			}
			if sensitiveForwardHeaders[lk] && !req.ForwardSensitiveHeaders {
				continue
			}
			for _, v := range vals {
				outReq.Header.Add(k, v)
			}
		}
	}

	// 2) explicit headers win (including Authorization if you set it here on purpose)
	for k, v := range req.Headers {
		outReq.Header.Set(k, v)
	}

	if outReq.Header.Get("X-Forwarded-By") == "" {
		outReq.Header.Set("X-Forwarded-By", "cluster-utils-api")
	}
	hostname, _ := os.Hostname()
	outReq.Header.Add("X-Cu-Proxy-Hop", hostname)

	client := &http.Client{Timeout: time.Duration(timeout * float64(time.Second))}
	start := time.Now()
	resp, err := client.Do(outReq)
	elapsed := time.Since(start)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":          err.Error(),
			"url":            req.URL,
			"method":         method,
			"durationMs":     elapsed.Milliseconds(),
			"timeoutSeconds": timeout,
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB cap in debug wrap
	respHeaders := map[string][]string{}
	for k, v := range resp.Header {
		respHeaders[k] = v
	}

	if req.Raw {
		for k, vals := range resp.Header {
			for _, v := range vals {
				c.Writer.Header().Add(k, v)
			}
		}
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
		return
	}

	sentHeaders := map[string][]string{}
	for k, v := range outReq.Header {
		sentHeaders[k] = v
	}

	// Default wrap: full upstream response (status + headers + body) for mesh debugging
	c.JSON(http.StatusOK, gin.H{
		"request": gin.H{
			"url":     req.URL,
			"method":  method,
			"headers": sentHeaders,
			"body":    req.Body,
		},
		"response": gin.H{
			"status":  resp.StatusCode,
			"headers": respHeaders,
			"body":    string(respBody),
		},
		"meta": gin.H{
			"durationMs":                elapsed.Milliseconds(),
			"timeoutSeconds":            timeout,
			"forwardIncomingHeaders":    forward,
			"forwardSensitiveHeaders":   req.ForwardSensitiveHeaders,
			"proxyHostname":             hostname,
			"inboundClientIP":           getClientIP(c.Request),
			"responseBodyTruncatedAtMB": 2,
		},
	})
}

func clampProxyTimeout(seconds float64) float64 {
	max := maxDelaySeconds()
	if seconds > max {
		return max
	}
	if seconds < 0.1 {
		return 0.1
	}
	return seconds
}
