package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"sub-store/internal/app"
	"sub-store/internal/cache"
	"sub-store/internal/config"
	"sub-store/internal/store"
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

func TestRefreshCacheDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataBasePath: dir}
	st := store.NewStore(dir)
	cache.InitScriptResourceCache(st)
	a := app.New(cfg, st)
	r := setupRouter(a)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/utils/refresh", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}
