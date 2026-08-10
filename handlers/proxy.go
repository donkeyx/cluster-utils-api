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

// hop-by-hop + sensitive-ish headers we don't blindly copy outbound
var skipForwardHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
	// these belong to *this* hop, not the east-west one
	"content-length": true,
	"host":           true,
}

// ProxyRequest is the JSON body for POST /proxy.
// Use this to fire east-west traffic from a north-south entry (ingress → this pod → other svc).
type ProxyRequest struct {
	// Absolute URL to call, e.g. http://other-api:8080/debug
	URL string `json:"url" binding:"required"`
	// HTTP method (default GET)
	Method string `json:"method,omitempty"`
	// Extra headers to set/override on the outbound request
	Headers map[string]string `json:"headers,omitempty"`
	// Optional body (string; use for JSON text, form, etc.)
	Body string `json:"body,omitempty"`
	// Timeout for the outbound call (default 10, max same as MAX_DELAY_SECONDS / 300)
	TimeoutSeconds float64 `json:"timeoutSeconds,omitempty"`
	// When true (default), copy inbound request headers onto the outbound call
	// (minus hop-by-hop). Good for tracing / auth / x-request-id passthrough.
	ForwardIncomingHeaders *bool `json:"forwardIncomingHeaders,omitempty"`
	// When true, return the upstream body as raw response (status + headers from upstream).
	// Default false → JSON wrap with timing + what we sent/received (better for debugging).
	Raw bool `json:"raw,omitempty"`
}

// @Summary Proxy / hop to another service
// @Description North→south hits this pod; this pod calls another URL east-west. Forwards headers by default so you can test mesh/ingress propagation. Also supports GET /proxy?url=
// @ID proxy
// @Accept json
// @Produce json
// @Param body body ProxyRequest true "proxy request"
// @Param url query string false "absolute url (GET form)"
// @Param method query string false "HTTP method for GET form (default GET)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 502 {object} map[string]interface{}
// @Router /proxy [post]
// @Router /proxy [get]
func ProxyHandler(c *gin.Context) {
	var req ProxyRequest

	// GET convenience: /proxy?url=http://svc:8080/debug&method=GET
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
			if skipForwardHeaders[strings.ToLower(k)] {
				continue
			}
			for _, v := range vals {
				outReq.Header.Add(k, v)
			}
		}
	}

	// 2) explicit headers win
	for k, v := range req.Headers {
		outReq.Header.Set(k, v)
	}

	// identify hop for debugging
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

	// what we actually sent (after merges)
	sentHeaders := map[string][]string{}
	for k, v := range outReq.Header {
		sentHeaders[k] = v
	}

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
			"durationMs":               elapsed.Milliseconds(),
			"timeoutSeconds":           timeout,
			"forwardIncomingHeaders":   forward,
			"proxyHostname":            hostname,
			"inboundClientIP":          getClientIP(c.Request),
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
