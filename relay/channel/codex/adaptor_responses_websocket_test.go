package codex

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLUsesWebsocketSchemeForResponses(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:          relayconstant.RelayModeResponses,
		ResponsesWebsocket: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
	}

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "wss://chatgpt.com/backend-api/codex/responses", url)
}

func TestSetupRequestHeaderUsesCodexBetaForResponsesWebsocket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	c.Request.Header.Set("OpenAI-Beta", "responses_websockets=2026-02-06")

	info := &relaycommon.RelayInfo{
		RelayMode:          relayconstant.RelayModeResponses,
		IsStream:           true,
		ResponsesWebsocket: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: `{"access_token":"access","account_id":"account"}`,
		},
	}
	headers := make(http.Header)

	err := (&Adaptor{}).SetupRequestHeader(c, &headers, info)

	require.NoError(t, err)
	require.Equal(t, "responses=experimental", headers.Get("OpenAI-Beta"))
}
