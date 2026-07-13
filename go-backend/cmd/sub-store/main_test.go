package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"sub-store/internal/config"
)

func TestBackendPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/subs", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	handler := backendPath(&config.Config{BackendPrefix: "true", FrontendBackendPath: "/secret"}, r)

	for path, want := range map[string]int{
		"/secret/api/subs": http.StatusNoContent,
		"/api/subs":        http.StatusNotFound,
		"/healthz":         http.StatusOK,
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Fatalf("%s: got %d, want %d", path, w.Code, want)
		}
	}
}
