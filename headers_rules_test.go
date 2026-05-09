package traefik_headers_rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestRulesHeader(t *testing.T) {
	cfg := CreateConfig()
	cfg.RequestRules = []Rule{
		{
			CheckHeader: "User-Agent",
			CheckRegex:  "curl/(.+)",
			SetHeader:   "X-Is-Curl",
			SetValue:    "true",
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Is-Curl") != "true" {
			t.Errorf("Expected X-Is-Curl header to be set")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler, _ := New(context.Background(), next, cfg, "test")

	req := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
}

func TestRequestRulesPathMethod(t *testing.T) {
	cfg := CreateConfig()
	cfg.RequestRules = []Rule{
		{
			CheckMethod: "POST",
			CheckPath:   "^/api/v1/.*$",
			SetHeader:   "X-Api-Call",
			SetValue:    "active",
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Call") != "active" {
			t.Errorf("Expected X-Api-Call header to be active")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler, _ := New(context.Background(), next, cfg, "test")

	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/users", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
}

func TestResponseRulesStatusAndHeader(t *testing.T) {
	cfg := CreateConfig()
	cfg.ResponseRules = []Rule{
		{
			CheckStatus: 404,
			CheckPath:   "^/images/.*$",
			SetHeader:   "Cache-Control",
			SetValue:    "no-cache",
		},
		{
			CheckHeader: "Content-Type",
			CheckRegex:  "^image/(.+)$",
			SetHeader:   "Cache-Control",
			SetValue:    "public, max-age=31536000",
		},
	}

	// Test 1: Hit 404 on /images/ path
	next404 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler404, _ := New(context.Background(), next404, cfg, "test")
	req404 := httptest.NewRequest(http.MethodGet, "http://localhost/images/nonexistent.png", nil)
	rw404 := httptest.NewRecorder()
	handler404.ServeHTTP(rw404, req404)

	if rw404.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control: no-cache, got %s", rw404.Header().Get("Cache-Control"))
	}

	// Test 2: Hit 200 with Content-Type on any path
	next200 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
	})
	handler200, _ := New(context.Background(), next200, cfg, "test")
	req200 := httptest.NewRequest(http.MethodGet, "http://localhost/docs", nil)
	rw200 := httptest.NewRecorder()
	handler200.ServeHTTP(rw200, req200)

	if rw200.Header().Get("Cache-Control") != "public, max-age=31536000" {
		t.Errorf("Expected Cache-Control: public..., got %s", rw200.Header().Get("Cache-Control"))
	}
}
