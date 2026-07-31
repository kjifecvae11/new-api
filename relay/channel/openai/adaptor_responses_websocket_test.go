package openai

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLUsesWebsocketSchemeForResponses(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:          relayconstant.RelayModeResponses,
		RequestURLPath:     "/v1/responses",
		ResponsesWebsocket: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.openai.com",
		},
	}

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "wss://api.openai.com/v1/responses", url)
}
