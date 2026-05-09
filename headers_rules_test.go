package traefik_headers_rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestRulesExpression(t *testing.T) {
	cfg := CreateConfig()
	cfg.RequestRules = []Rule{
		{
			Expression: `Header("User-Agent", "curl/(.+)") && Method("GET")`,
			SetHeader:  "X-Is-Curl",
			SetValue:   "true",
		},
		{
			Expression: `Path("^/api/v1/.*") && Method("POST")`,
			SetHeader:  "X-Api-Call",
			SetValue:   "active",
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			if r.Header.Get("X-Is-Curl") != "true" {
				t.Errorf("Expected X-Is-Curl header to be set (GET with curl User-Agent)")
			}
		}
		if r.URL.Path == "/api/v1/users" && r.Method == http.MethodPost {
			if r.Header.Get("X-Api-Call") != "active" {
				t.Errorf("Expected X-Api-Call header to be active (POST on /api/v1/)")
			}
		}

		w.WriteHeader(http.StatusOK)
	})

	handler, err := New(context.Background(), next, cfg, "test")
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}

	// Test 1: GET with curl
	req1 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	req1.Header.Set("User-Agent", "curl/7.68.0")
	rw1 := httptest.NewRecorder()
	handler.ServeHTTP(rw1, req1)

	// Test 2: POST to api
	req2 := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/users", nil)
	rw2 := httptest.NewRecorder()
	handler.ServeHTTP(rw2, req2)

	// Test 3: GET to api (Should NOT trigger rule 2 since method is GET)
	req3 := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/users", nil)
	rw3 := httptest.NewRecorder()
	handler.ServeHTTP(rw3, req3)

	if req3.Header.Get("X-Api-Call") == "active" {
		t.Errorf("X-Api-Call should NOT be set on GET request")
	}
}

func TestResponseRulesExpression(t *testing.T) {
	cfg := CreateConfig()
	cfg.ResponseRules = []Rule{
		{
			Expression: `Status("404") && Path("^/images/.*$")`,
			SetHeader:  "Cache-Control",
			SetValue:   "no-cache",
		},
		{
			Expression: `Status("200") && Header("Content-Type", "^image/(.+)$")`,
			SetHeader:  "Cache-Control",
			SetValue:   "public, max-age=31536000",
		},
	}

	// Test 1: Hit 404 on /images/ path
	next404 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler404, err := New(context.Background(), next404, cfg, "test")
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	req404 := httptest.NewRequest(http.MethodGet, "http://localhost/images/nonexistent.png", nil)
	rw404 := httptest.NewRecorder()
	handler404.ServeHTTP(rw404, req404)

	if rw404.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control: no-cache, got %s", rw404.Header().Get("Cache-Control"))
	}

	// Test 2: Hit 200 with Content-Type match
	next200 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
	})
	handler200, err := New(context.Background(), next200, cfg, "test")
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	req200 := httptest.NewRequest(http.MethodGet, "http://localhost/docs", nil)
	rw200 := httptest.NewRecorder()
	handler200.ServeHTTP(rw200, req200)

	if rw200.Header().Get("Cache-Control") != "public, max-age=31536000" {
		t.Errorf("Expected Cache-Control: public..., got %s", rw200.Header().Get("Cache-Control"))
	}
}
