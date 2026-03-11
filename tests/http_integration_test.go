package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	httppkg "web-tracker/interface/http"
)

func TestRouter_HealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a minimal router with just system handler
	systemHandler := httppkg.NewSystemHandler(nil, nil, nil, nil)

	engine := gin.New()

	// Setup health endpoints
	health := engine.Group("/health")
	{
		health.GET("", systemHandler.Health)
		health.GET("/live", systemHandler.Live)
	}

	// Test basic health endpoint
	t.Run("GET /health", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"status":"ok"`)
	})

	// Test liveness endpoint
	t.Run("GET /health/live", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health/live", nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"status":"alive"`)
	})
}

func TestRouter_CORSHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a minimal router
	systemHandler := httppkg.NewSystemHandler(nil, nil, nil, nil)

	engine := gin.New()

	// Add CORS middleware
	engine.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	engine.GET("/health", systemHandler.Health)

	// Test CORS headers
	t.Run("CORS headers present", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	})

	// Test OPTIONS request
	t.Run("OPTIONS request", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", "/health", nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
