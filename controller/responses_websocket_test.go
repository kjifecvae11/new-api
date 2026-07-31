package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestParseResponsesWebsocketCreate(t *testing.T) {
	event, request, err := parseResponsesWebsocketCreate([]byte(`{
		"type":"response.create",
		"model":"gpt-5.6",
		"store":false,
		"stream":true,
		"generate":false,
		"client_metadata":{"session_id":"session-1"},
		"input":[{"role":"user","content":"hello"}]
	}`))
	require.NoError(t, err)
	require.Equal(t, "response.create", event.Type)
	require.Equal(t, "gpt-5.6", request.Model)
	require.NotNil(t, event.Generate)
	require.False(t, *event.Generate)
	require.Nil(t, request.Stream)
	require.JSONEq(t, `{"session_id":"session-1"}`, string(request.ClientMetadata))
}

func TestParseResponsesWebsocketCreateRejectsTransportFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		message string
	}{
		{
			name:    "first event",
			payload: `{"type":"response.cancel","model":"gpt-5.6"}`,
			message: "first event must be response.create",
		},
		{
			name:    "missing model",
			payload: `{"type":"response.create"}`,
			message: "model is required",
		},
		{
			name:    "stream false",
			payload: `{"type":"response.create","model":"gpt-5.6","stream":false}`,
			message: "stream must be true",
		},
		{
			name:    "background",
			payload: `{"type":"response.create","model":"gpt-5.6","background":true}`,
			message: "background is not supported",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := parseResponsesWebsocketCreate([]byte(test.payload))
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestUsageFromResponsesWebsocketNormalizesResponsesFields(t *testing.T) {
	usage := usageFromResponsesWebsocket(&dto.OpenAIResponsesResponse{
		Usage: &dto.Usage{
			InputTokens:  12,
			OutputTokens: 7,
			TotalTokens:  19,
		},
	})
	require.NotNil(t, usage)
	require.Equal(t, 12, usage.PromptTokens)
	require.Equal(t, 7, usage.CompletionTokens)
	require.Equal(t, 19, usage.TotalTokens)
}

func TestResponsesWebsocketConnectionLimits(t *testing.T) {
	t.Setenv("RESPONSES_WEBSOCKET_MAX_CONNECTIONS", "2")
	t.Setenv("RESPONSES_WEBSOCKET_MAX_CONNECTIONS_PER_TOKEN", "1")

	responsesWebsocketConnections.mu.Lock()
	responsesWebsocketConnections.total = 0
	responsesWebsocketConnections.byToken = make(map[int]int)
	responsesWebsocketConnections.mu.Unlock()

	require.True(t, acquireResponsesWebsocketConnection(10))
	require.False(t, acquireResponsesWebsocketConnection(10))
	require.True(t, acquireResponsesWebsocketConnection(11))
	require.False(t, acquireResponsesWebsocketConnection(12))

	releaseResponsesWebsocketConnection(10)
	require.True(t, acquireResponsesWebsocketConnection(12))
	releaseResponsesWebsocketConnection(11)
	releaseResponsesWebsocketConnection(12)
}
