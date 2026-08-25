package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gorilla/websocket"
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

func codexWebsocketTestEvent(eventType string, errorValue any) *dto.ResponsesStreamResponse {
	return &dto.ResponsesStreamResponse{
		Type:  eventType,
		Error: errorValue,
	}
}

func TestResponsesWebsocketCodexRetriesHighDemandBeforeOutput(t *testing.T) {
	state := &responsesWebsocketState{modelName: "gpt-5.6"}
	requestPayload := []byte(`{"type":"response.create","model":"gpt-5.6","input":"hello"}`)
	state.activate(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeCodex},
	}, requestPayload)

	createdPayload := []byte(`{"type":"response.created","response":{"output":[]}}`)
	action := state.handleCodexEvent(
		codexWebsocketTestEvent("response.created", nil),
		websocket.TextMessage,
		createdPayload,
	)
	require.True(t, action.suppress)
	require.Empty(t, action.retryPayload)

	errorPayload := []byte(`{"type":"error","error":{"type":"server_error","code":"server_error","message":"We're currently experiencing high demand, which may cause temporary errors."}}`)
	action = state.handleCodexEvent(
		codexWebsocketTestEvent("error", map[string]any{
			"type":    "server_error",
			"code":    "server_error",
			"message": "We're currently experiencing high demand, which may cause temporary errors.",
		}),
		websocket.TextMessage,
		errorPayload,
	)
	require.True(t, action.suppress)
	require.Equal(t, 2, action.retryAttempt)
	require.Equal(t, requestPayload, action.retryPayload)
	require.Contains(t, action.retryReason, "high demand")
	require.Empty(t, action.forwardMessage)
}

func TestResponsesWebsocketCodexRetriesUpstreamServerErrorIncomplete(t *testing.T) {
	state := &responsesWebsocketState{modelName: "gpt-5.6-sol"}
	requestPayload := []byte(`{"type":"response.create","model":"gpt-5.6-sol","input":"hello"}`)
	state.activate(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeCodex},
	}, requestPayload)

	action := state.handleCodexEvent(&dto.ResponsesStreamResponse{
		Type: "response.incomplete",
		Response: &dto.OpenAIResponsesResponse{
			Status:            json.RawMessage(`"incomplete"`),
			IncompleteDetails: &dto.IncompleteDetails{Reason: "upstream_server_error"},
		},
	}, websocket.TextMessage, []byte(`{"type":"response.incomplete"}`))

	require.True(t, action.suppress)
	require.Equal(t, 2, action.retryAttempt)
	require.Equal(t, requestPayload, action.retryPayload)
	require.Equal(t, "upstream_server_error", action.retryReason)
}

func TestResponsesWebsocketCodexRetriesTopLevelIncompleteDetails(t *testing.T) {
	state := &responsesWebsocketState{modelName: "gpt-5.6-sol"}
	requestPayload := []byte(`{"type":"response.create","model":"gpt-5.6-sol","input":"hello"}`)
	state.activate(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeCodex},
	}, requestPayload)

	action := state.handleCodexEvent(&dto.ResponsesStreamResponse{
		Type:              "response.incomplete",
		IncompleteDetails: &dto.IncompleteDetails{Reason: "upstream_server_error"},
	}, websocket.TextMessage, []byte(`{"type":"response.incomplete","incomplete_details":{"reason":"upstream_server_error"}}`))

	require.True(t, action.suppress)
	require.Equal(t, "upstream_server_error", action.retryReason)
	require.Equal(t, requestPayload, action.retryPayload)
}

func TestResponsesWebsocketCodexRetriesBeforeOutputDisconnect(t *testing.T) {
	state := &responsesWebsocketState{modelName: "gpt-5.6-sol"}
	requestPayload := []byte(`{"type":"response.create","model":"gpt-5.6-sol","input":"hello"}`)
	state.activate(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeCodex},
	}, requestPayload)

	payload, attempt, ok := state.retryAfterCodexDisconnect()
	require.True(t, ok)
	require.Equal(t, 2, attempt)
	require.Equal(t, requestPayload, payload)
}

func TestResponsesWebsocketCodexFlushesOnlySuccessfulAttemptPrefix(t *testing.T) {
	state := &responsesWebsocketState{modelName: "gpt-5.6"}
	requestPayload := []byte(`{"type":"response.create","model":"gpt-5.6","input":"hello"}`)
	state.activate(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeCodex},
	}, requestPayload)

	firstCreated := []byte(`{"type":"response.created","response":{"id":"first","output":[]}}`)
	state.handleCodexEvent(codexWebsocketTestEvent("response.created", nil), websocket.TextMessage, firstCreated)
	state.handleCodexEvent(codexWebsocketTestEvent("error", map[string]any{
		"message": "high demand",
	}), websocket.TextMessage, []byte(`{"type":"error"}`))

	retryCreated := []byte(`{"type":"response.created","response":{"id":"retry","output":[]}}`)
	action := state.handleCodexEvent(codexWebsocketTestEvent("response.created", nil), websocket.TextMessage, retryCreated)
	require.True(t, action.suppress)

	delta := []byte(`{"type":"response.output_text.delta","delta":"ok"}`)
	action = state.handleCodexEvent(&dto.ResponsesStreamResponse{
		Type:  "response.output_text.delta",
		Delta: "ok",
	}, websocket.TextMessage, delta)
	require.True(t, action.suppress)
	require.Len(t, action.forwardMessage, 2)
	require.Equal(t, retryCreated, action.forwardMessage[0].payload)
	require.Equal(t, delta, action.forwardMessage[1].payload)
	for _, message := range action.forwardMessage {
		require.NotEqual(t, firstCreated, message.payload)
	}

	postOutputError := state.handleCodexEvent(
		codexWebsocketTestEvent("error", map[string]any{"message": "high demand"}),
		websocket.TextMessage,
		[]byte(`{"type":"error"}`),
	)
	require.False(t, postOutputError.suppress)
	require.Empty(t, postOutputError.retryPayload)
}

func TestResponsesWebsocketCodexSuppressesPreviousAttemptTerminalEvent(t *testing.T) {
	state := &responsesWebsocketState{modelName: "gpt-5.6"}
	state.activate(
		&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeCodex}},
		[]byte(`{"type":"response.create","model":"gpt-5.6"}`),
	)
	state.handleCodexEvent(
		codexWebsocketTestEvent("error", map[string]any{"message": "high demand"}),
		websocket.TextMessage,
		[]byte(`{"type":"error"}`),
	)

	action := state.handleCodexEvent(&dto.ResponsesStreamResponse{
		Type: "response.failed",
		Response: &dto.OpenAIResponsesResponse{
			Error: map[string]any{"message": "high demand"},
		},
	}, websocket.TextMessage, []byte(`{"type":"response.failed"}`))
	require.True(t, action.suppress)
	require.Empty(t, action.retryPayload)
}

func TestResponsesWebsocketRetryDoesNotAffectOtherProviders(t *testing.T) {
	state := &responsesWebsocketState{modelName: "gpt-5.6"}
	state.activate(
		&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeOpenAI}},
		[]byte(`{"type":"response.create","model":"gpt-5.6"}`),
	)
	action := state.handleCodexEvent(
		codexWebsocketTestEvent("error", map[string]any{"message": "high demand"}),
		websocket.TextMessage,
		[]byte(`{"type":"error"}`),
	)
	require.False(t, action.suppress)
	require.Empty(t, action.retryPayload)
}
