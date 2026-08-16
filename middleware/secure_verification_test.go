package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func secureVerificationTestCookies(t *testing.T, router *gin.Engine, mode string) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fixture/"+mode, nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("fixture status = %d", recorder.Code)
	}
	return recorder.Result().Cookies()
}

func TestSecureVerificationRequiredBindsFreshProofToCurrentUserAndSessionVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("secure-verification-test-secret"))))
	router.GET("/fixture/:mode", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set(SecureVerificationSessionKey, time.Now().Unix())
		session.Set(secureVerificationMethodSessionKey, "2fa")
		session.Set(secureVerificationUserIDSessionKey, 41)
		session.Set(secureVerificationAuthSessionKey, common.SessionAuthVersion)
		switch c.Param("mode") {
		case "stale":
			session.Set(SecureVerificationSessionKey, time.Now().Add(-301*time.Second).Unix())
		case "other-user":
			session.Set(secureVerificationUserIDSessionKey, 99)
		case "old-session-version":
			session.Set(secureVerificationAuthSessionKey, common.SessionAuthVersion-1)
		case "unsupported-method":
			session.Set(secureVerificationMethodSessionKey, "password")
		case "legacy-unbound":
			session.Delete(secureVerificationUserIDSessionKey)
			session.Delete(secureVerificationAuthSessionKey)
		}
		if err := session.Save(); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.DELETE("/self",
		func(c *gin.Context) {
			c.Set("id", 41)
			c.Next()
		},
		SecureVerificationRequired(),
		func(c *gin.Context) {
			if c.GetInt64("secure_verified_at") == 0 || c.GetString("secure_verified_method") != "2fa" {
				c.Status(http.StatusInternalServerError)
				return
			}
			c.Status(http.StatusNoContent)
		},
	)

	for _, mode := range []string{"stale", "other-user", "old-session-version", "unsupported-method", "legacy-unbound"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/self", nil)
		for _, sessionCookie := range secureVerificationTestCookies(t, router, mode) {
			request.AddCookie(sessionCookie)
		}
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("mode %s status = %d, want 403; body=%s", mode, recorder.Code, recorder.Body.String())
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/self", nil)
	for _, sessionCookie := range secureVerificationTestCookies(t, router, "fresh") {
		request.AddCookie(sessionCookie)
	}
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("fresh bound proof status = %d, want 204; body=%s", recorder.Code, recorder.Body.String())
	}
}
