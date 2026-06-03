package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStatusServerAddressUsesForwardedOriginForDefaultLocalAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalServerAddress := system_setting.ServerAddress
	t.Cleanup(func() {
		system_setting.ServerAddress = originalServerAddress
	})

	system_setting.ServerAddress = "http://localhost:3000"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	ctx.Request.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request.Header.Set("X-Forwarded-Host", "prod.example.com")

	require.Equal(t, "https://prod.example.com", statusServerAddress(ctx))
}

func TestStatusServerAddressPreservesConfiguredProductionAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalServerAddress := system_setting.ServerAddress
	t.Cleanup(func() {
		system_setting.ServerAddress = originalServerAddress
	})

	system_setting.ServerAddress = "https://configured.example.com/"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	ctx.Request.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request.Header.Set("X-Forwarded-Host", "prod.example.com")

	require.Equal(t, "https://configured.example.com", statusServerAddress(ctx))
}

func TestStatusServerAddressNormalizesWebSocketScheme(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalServerAddress := system_setting.ServerAddress
	t.Cleanup(func() {
		system_setting.ServerAddress = originalServerAddress
	})

	system_setting.ServerAddress = "wss://configured.example.com/"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	require.Equal(t, "https://configured.example.com", statusServerAddress(ctx))
}
