package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
