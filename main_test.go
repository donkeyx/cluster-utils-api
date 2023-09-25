// main_test.go
package main

import (
	"cu-api/routes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
	expectedResponse := "OK"

	// Create a new Gin router
	r := gin.New()
	logger := setupLogger()
	routes.SetupRouter(logger, securityToken, r)

	// Create a test HTTP request to the "/hello" endpoint
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, expectedResponse, rr.Body.String())
}
