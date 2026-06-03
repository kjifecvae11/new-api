package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetStatusIncludesRegistrationCapabilityFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalPasswordLoginEnabled := common.PasswordLoginEnabled
	originalOptionMap := common.OptionMap
	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.PasswordLoginEnabled = originalPasswordLoginEnabled
		common.OptionMap = originalOptionMap
	})

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = false
	common.PasswordLoginEnabled = true
	common.OptionMap = map[string]string{
		"HeaderNavModules":    "",
		"SidebarModulesAdmin": "",
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	GetStatus(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			PasswordLoginEnabled    bool `json:"password_login_enabled"`
			RegisterEnabled         bool `json:"register_enabled"`
			PasswordRegisterEnabled bool `json:"password_register_enabled"`
			OAuthRegisterEnabled    bool `json:"oauth_register_enabled"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.True(t, payload.Data.PasswordLoginEnabled)
	require.True(t, payload.Data.RegisterEnabled)
	require.False(t, payload.Data.PasswordRegisterEnabled)
	require.True(t, payload.Data.OAuthRegisterEnabled)
}

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
