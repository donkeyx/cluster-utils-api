// main_test.go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHelloEndpoint(t *testing.T) {
	// Create a new Gin router
	r := gin.New()

	// Define your routes as in main.go
	r.GET("/hello", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello, World!")
	})

	// Create a test HTTP request to the "/hello" endpoint
	req, err := http.NewRequest("GET", "/hello", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder to record the response
	rr := httptest.NewRecorder()

	// Serve the request to the router
	r.ServeHTTP(rr, req)

	// Verify the HTTP status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify the response body
	expectedResponse := "Hello, World!"
	assert.Equal(t, expectedResponse, rr.Body.String())
}
