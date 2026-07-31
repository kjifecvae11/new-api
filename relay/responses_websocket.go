package relay

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// SupportsResponsesWebsocket reports whether the selected channel adaptor has
// an upstream implementation for the Responses API WebSocket transport.
func SupportsResponsesWebsocket(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	switch info.ApiType {
	case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		return true
	default:
		return false
	}
}

// PrepareResponsesWebsocketCreate applies the same model mapping, request
// conversion, field filtering and parameter override used by HTTP Responses.
// Transport-only fields are added after conversion so they cannot leak into
// adaptors that only understand the normal Responses request body.
func PrepareResponsesWebsocketCreate(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request *dto.OpenAIResponsesRequest,
	generate *bool,
) ([]byte, error) {
	if info == nil || request == nil {
		return nil, fmt.Errorf("responses websocket request is nil")
	}

	info.ResponsesWebsocket = true
	info.IsStream = true
	info.InitChannelMeta(c)

	copied, err := common.DeepCopy(request)
	if err != nil {
		return nil, fmt.Errorf("copy responses websocket request: %w", err)
	}
	copied.Stream = nil
	copied.StreamOptions = nil

	if err := helper.ModelMappedHelper(c, info, copied); err != nil {
		return nil, fmt.Errorf("map responses websocket model: %w", err)
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, fmt.Errorf("invalid api type: %d", info.ApiType)
	}
	adaptor.Init(info)

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *copied)
	if err != nil {
		return nil, fmt.Errorf("convert responses websocket request: %w", err)
	}
	relaycommon.AppendRequestConversionFromRequest(info, converted)

	payload, err := common.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("marshal responses websocket request: %w", err)
	}
	payload, err = relaycommon.RemoveDisabledFields(
		payload,
		info.ChannelOtherSettings,
		info.ChannelSetting.PassThroughBodyEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("filter responses websocket request: %w", err)
	}
	if len(info.ParamOverride) > 0 {
		payload, err = relaycommon.ApplyParamOverrideWithRelayInfo(payload, info)
		if err != nil {
			return nil, fmt.Errorf("override responses websocket request: %w", err)
		}
	}

	var event map[string]any
	if err := common.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode converted responses websocket request: %w", err)
	}
	event["type"] = "response.create"
	// Both the public Responses WebSocket protocol and the ChatGPT Codex
	// backend expect the response to be streamed over the upgraded socket.
	// Keep this transport detail out of billing/request parsing, then restore
	// it only on the upstream wire payload.
	event["stream"] = true
	if generate != nil {
		event["generate"] = *generate
	}
	return common.Marshal(event)
}

// DialResponsesWebsocket opens the upstream socket after channel selection and
// request conversion have completed.
func DialResponsesWebsocket(c *gin.Context, info *relaycommon.RelayInfo) (*websocket.Conn, error) {
	if info == nil {
		return nil, fmt.Errorf("responses websocket relay info is nil")
	}
	if !SupportsResponsesWebsocket(info) {
		return nil, fmt.Errorf("channel api type %d does not support responses websocket", info.ApiType)
	}
	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, fmt.Errorf("invalid api type: %d", info.ApiType)
	}
	adaptor.Init(info)
	conn, err := channel.DoWssRequest(adaptor, c, info, nil)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("upstream responses websocket connection is nil")
	}
	return conn, nil
}
