package middleware

import (
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return cors.New(corsConfig())
}

// corsConfig keeps browser credentials on same-origin or explicitly trusted
// origins. Non-browser API clients do not send Origin and are unaffected.
func corsConfig() cors.Config {
	config := cors.DefaultConfig()
	allowedOrigins, allowAll := configuredCORSOrigins(os.Getenv("CORS_ALLOW_ORIGINS"))
	config.AllowCredentials = !allowAll
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{
		"Accept", "Authorization", "Cache-Control", "Content-Type",
		"New-Api-User", "Origin", "X-Requested-With",
	}
	config.ExposeHeaders = []string{"Content-Disposition", "X-New-Api-Version", "X-Request-Id"}
	config.MaxAge = 12 * time.Hour
	if allowAll {
		config.AllowAllOrigins = true
		return config
	}
	config.AllowOriginWithContextFunc = func(c *gin.Context, origin string) bool {
		if sameRequestOrigin(c, origin) {
			return true
		}
		_, ok := allowedOrigins[normalizeOrigin(origin)]
		return ok
	}
	return config
}

func configuredCORSOrigins(raw string) (map[string]struct{}, bool) {
	origins := make(map[string]struct{})
	for _, candidate := range strings.Split(raw, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == "*" {
			return origins, true
		}
		if normalized := normalizeOrigin(candidate); normalized != "" {
			origins[normalized] = struct{}{}
		}
	}
	return origins, false
}

func normalizeOrigin(origin string) string {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func sameRequestOrigin(c *gin.Context, origin string) bool {
	parsedOrigin := normalizeOrigin(origin)
	if parsedOrigin == "" {
		return false
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0]); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	return parsedOrigin == strings.ToLower(scheme+"://"+c.Request.Host)
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
