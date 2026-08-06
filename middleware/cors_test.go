package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func corsProbe(t *testing.T, method, allowedOrigins, requestOrigin string) *httptest.ResponseRecorder {
	t.Helper()
	t.Setenv("CORS_ALLOW_ORIGINS", allowedOrigins)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS())
	router.GET("/probe", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(method, "http://ainiubi.org/probe", nil)
	request.Host = "ainiubi.org"
	request.Header.Set("Origin", requestOrigin)
	if method == http.MethodOptions {
		request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	}
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestCORSDeniesUntrustedOriginByDefault(t *testing.T) {
	recorder := corsProbe(t, http.MethodOptions, "", "https://evil.example")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestCORSAllowsSameAndConfiguredOriginsWithCredentials(t *testing.T) {
	for name, values := range map[string][2]string{
		"same origin":       {"", "https://ainiubi.org"},
		"configured origin": {"https://console.example", "https://console.example"},
	} {
		t.Run(name, func(t *testing.T) {
			configured, origin := values[0], values[1]
			method := http.MethodOptions
			if name == "same origin" {
				method = http.MethodGet
			}
			recorder := corsProbe(t, method, configured, origin)
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
			}
			if name == "same origin" {
				if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
					t.Fatalf("same-origin Access-Control-Allow-Origin = %q, want empty", got)
				}
				return
			}
			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, origin)
			}
			if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
				t.Fatalf("Access-Control-Allow-Credentials = %q, want true", got)
			}
		})
	}
}

func TestCORSWildcardNeverAllowsCredentials(t *testing.T) {
	recorder := corsProbe(t, http.MethodOptions, "*", "https://client.example")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty", got)
	}
}

func TestConfiguredCORSOriginsRejectsPathsAndCredentials(t *testing.T) {
	origins, allowAll := configuredCORSOrigins("https://good.example,https://bad.example/path,https://user@bad.example")
	if allowAll {
		t.Fatal("allowAll = true, want false")
	}
	if _, ok := origins["https://good.example"]; !ok {
		t.Fatal("valid origin missing")
	}
	if len(origins) != 1 {
		t.Fatalf("origin count = %d, want 1", len(origins))
	}
}
