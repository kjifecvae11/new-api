package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	defaultResponsesWebsocketMaxConnections         = 1000
	defaultResponsesWebsocketMaxConnectionsPerToken = 5
	responsesWebsocketMaxDuration                   = 60 * time.Minute
	responsesWebsocketReadLimit                     = 32 << 20
)

var responsesWebsocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

type responsesWebsocketConnectionRegistry struct {
	mu      sync.Mutex
	total   int
	byToken map[int]int
}

var responsesWebsocketConnections = responsesWebsocketConnectionRegistry{
	byToken: make(map[int]int),
}

func envPositiveInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func acquireResponsesWebsocketConnection(tokenID int) bool {
	maxTotal := envPositiveInt("RESPONSES_WEBSOCKET_MAX_CONNECTIONS", defaultResponsesWebsocketMaxConnections)
	maxPerToken := envPositiveInt("RESPONSES_WEBSOCKET_MAX_CONNECTIONS_PER_TOKEN", defaultResponsesWebsocketMaxConnectionsPerToken)

	responsesWebsocketConnections.mu.Lock()
	defer responsesWebsocketConnections.mu.Unlock()
	if responsesWebsocketConnections.total >= maxTotal || responsesWebsocketConnections.byToken[tokenID] >= maxPerToken {
		return false
	}
	responsesWebsocketConnections.total++
	responsesWebsocketConnections.byToken[tokenID]++
	return true
}

func releaseResponsesWebsocketConnection(tokenID int) {
	responsesWebsocketConnections.mu.Lock()
	defer responsesWebsocketConnections.mu.Unlock()
	if responsesWebsocketConnections.total > 0 {
		responsesWebsocketConnections.total--
	}
	if responsesWebsocketConnections.byToken[tokenID] <= 1 {
		delete(responsesWebsocketConnections.byToken, tokenID)
	} else {
		responsesWebsocketConnections.byToken[tokenID]--
	}
}

type responsesWebsocketClientEvent struct {
	Type       string          `json:"type"`
	Model      string          `json:"model"`
	Generate   *bool           `json:"generate,omitempty"`
	Stream     json.RawMessage `json:"stream,omitempty"`
	Background json.RawMessage `json:"background,omitempty"`
}

func parseResponsesWebsocketCreate(payload []byte) (*responsesWebsocketClientEvent, *dto.OpenAIResponsesRequest, error) {
	var event responsesWebsocketClientEvent
	if err := common.Unmarshal(payload, &event); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if event.Type != "response.create" {
		return nil, nil, fmt.Errorf("first event must be response.create")
	}
	if strings.TrimSpace(event.Model) == "" {
		return nil, nil, fmt.Errorf("model is required")
	}
	if len(event.Stream) > 0 && string(event.Stream) != "null" {
		var stream bool
		if err := common.Unmarshal(event.Stream, &stream); err != nil || !stream {
			return nil, nil, fmt.Errorf("stream must be true in WebSocket mode when provided")
		}
	}
	if len(event.Background) > 0 && string(event.Background) != "null" && string(event.Background) != "false" {
		return nil, nil, fmt.Errorf("background is not supported in WebSocket mode")
	}

	var request dto.OpenAIResponsesRequest
	if err := common.Unmarshal(payload, &request); err != nil {
		return nil, nil, fmt.Errorf("invalid response.create payload: %w", err)
	}
	request.Stream = nil
	request.StreamOptions = nil
	return &event, &request, nil
}

func validateResponsesWebsocketTokenModel(c *gin.Context, modelName string) error {
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		return nil
	}
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
	if !ok {
		return fmt.Errorf("token has no access to model %s", modelName)
	}
	allowed, ok := value.(map[string]bool)
	if !ok {
		return fmt.Errorf("token model limit is invalid")
	}
	if !allowed[ratio_setting.FormatMatchingModelName(modelName)] {
		return fmt.Errorf("token has no access to model %s", modelName)
	}
	return nil
}

func selectResponsesWebsocketChannel(c *gin.Context, modelName string) (*model.Channel, error) {
	if err := validateResponsesWebsocketTokenModel(c, modelName); err != nil {
		return nil, err
	}

	var selected *model.Channel
	if channelValue, ok := common.GetContextKey(c, constant.ContextKeyTokenSpecificChannelId); ok {
		channelID, err := strconv.Atoi(fmt.Sprint(channelValue))
		if err != nil {
			return nil, fmt.Errorf("invalid token-specific channel id")
		}
		selected, err = model.GetChannelById(channelID, true)
		if err != nil {
			return nil, fmt.Errorf("load token-specific channel: %w", err)
		}
		if selected.Status != common.ChannelStatusEnabled {
			return nil, fmt.Errorf("token-specific channel is disabled")
		}
	} else {
		usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		var err error
		selected, _, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
			Ctx:        c,
			TokenGroup: usingGroup,
			ModelName:  modelName,
			Retry:      common.GetPointer(0),
		})
		if err != nil {
			return nil, err
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no available channel for model %s", modelName)
	}
	if selected.Type != constant.ChannelTypeOpenAI && selected.Type != constant.ChannelTypeCodex {
		return nil, fmt.Errorf("selected channel %d does not support Responses WebSocket", selected.Id)
	}
	if apiErr := middleware.SetupContextForSelectedChannel(c, selected, modelName); apiErr != nil {
		return nil, apiErr
	}
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	return selected, nil
}

type responsesWebsocketActiveRequest struct {
	info *relaycommon.RelayInfo
	text strings.Builder
}

type responsesWebsocketState struct {
	mu        sync.Mutex
	modelName string
	sequence  atomic.Uint64
	active    *responsesWebsocketActiveRequest
	preparing bool
}

func (s *responsesWebsocketState) reserve() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.preparing || s.active != nil {
		return false
	}
	s.preparing = true
	return true
}

func (s *responsesWebsocketState) activate(info *relaycommon.RelayInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preparing = false
	s.active = &responsesWebsocketActiveRequest{info: info}
}

func (s *responsesWebsocketState) cancelReservation() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preparing = false
}

func (s *responsesWebsocketState) appendText(delta string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		s.active.text.WriteString(delta)
	}
}

func (s *responsesWebsocketState) recordItem(item *dto.ResponsesOutput) {
	if item == nil || item.Type != dto.BuildInCallWebSearchCall {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil || s.active.info == nil || s.active.info.ResponsesUsageInfo == nil {
		return
	}
	tools := s.active.info.ResponsesUsageInfo.BuiltInTools
	if tool, ok := tools[dto.BuildInToolWebSearchPreview]; ok && tool != nil {
		tool.CallCount++
	}
}

func (s *responsesWebsocketState) takeActive() *responsesWebsocketActiveRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := s.active
	s.active = nil
	return active
}

func refundResponsesWebsocketActive(c *gin.Context, state *responsesWebsocketState) {
	active := state.takeActive()
	if active != nil && active.info != nil && active.info.Billing != nil {
		active.info.Billing.Refund(c)
	}
}

func usageFromResponsesWebsocket(response *dto.OpenAIResponsesResponse) *dto.Usage {
	if response == nil || response.Usage == nil {
		return nil
	}
	usage := *response.Usage
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	if usage.TotalTokens == 0 {
		usage.PromptTokens = response.Usage.InputTokens
		usage.CompletionTokens = response.Usage.OutputTokens
		usage.TotalTokens = response.Usage.TotalTokens
	}
	return &usage
}

func settleResponsesWebsocketRequest(
	c *gin.Context,
	active *responsesWebsocketActiveRequest,
	response *dto.OpenAIResponsesResponse,
) {
	if active == nil || active.info == nil {
		return
	}
	active.info.SetResponseContent(active.text.String())
	usage := usageFromResponsesWebsocket(response)
	if usage == nil || usage.TotalTokens == 0 {
		if active.info.Billing != nil {
			active.info.Billing.Refund(c)
		}
		return
	}
	service.PostTextConsumeQuota(c, active.info, usage, []string{"Responses WebSocket"})
}

func prepareResponsesWebsocketCreate(
	c *gin.Context,
	state *responsesWebsocketState,
	payload []byte,
) (*relaycommon.RelayInfo, []byte, error) {
	event, request, err := parseResponsesWebsocketCreate(payload)
	if err != nil {
		return nil, nil, err
	}
	if request.Model != state.modelName {
		return nil, nil, fmt.Errorf("model cannot change on an active Responses WebSocket connection")
	}

	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	info := relaycommon.GenRelayInfoResponses(c, request)
	info.RequestId = fmt.Sprintf("%s-ws-%d", info.RequestId, state.sequence.Add(1))
	info.IsStream = true
	info.ResponsesWebsocket = true

	meta := request.GetTokenCountMeta()
	if setting.ShouldCheckPromptSensitive() && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			return nil, nil, fmt.Errorf("sensitive words detected: %s", strings.Join(words, ", "))
		}
	}
	tokens, err := service.EstimateRequestToken(c, meta, info)
	if err != nil {
		return nil, nil, fmt.Errorf("estimate request tokens: %w", err)
	}
	info.SetEstimatePromptTokens(tokens)
	priceData, err := helper.ModelPriceHelper(c, info, tokens, meta)
	if err != nil {
		return nil, nil, fmt.Errorf("calculate request price: %w", err)
	}
	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(c, priceData.QuotaToPreConsume, info); apiErr != nil {
			return nil, nil, apiErr
		}
	}

	converted, err := relay.PrepareResponsesWebsocketCreate(c, info, request, event.Generate)
	if err != nil {
		if info.Billing != nil {
			info.Billing.Refund(c)
		}
		return nil, nil, err
	}
	return info, converted, nil
}

func responsesWebsocketErrorPayload(code, message string, status int) []byte {
	payload, _ := common.Marshal(gin.H{
		"type":   "error",
		"status": status,
		"error": gin.H{
			"type":    "invalid_request_error",
			"code":    code,
			"message": message,
		},
	})
	return payload
}

func writeResponsesWebsocketError(conn *websocket.Conn, writeMu *sync.Mutex, code string, err error, status int) {
	if conn == nil || err == nil {
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	_ = conn.WriteMessage(websocket.TextMessage, responsesWebsocketErrorPayload(code, err.Error(), status))
}

func isExpectedWebsocketClose(err error) bool {
	return err == nil ||
		websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
		strings.Contains(strings.ToLower(err.Error()), "closed network connection")
}

// RelayResponsesWebsocket implements the persistent Responses API transport.
// Authentication and the handshake-level rate limit run before this handler.
func RelayResponsesWebsocket(c *gin.Context) {
	tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
	if !acquireResponsesWebsocketConnection(tokenID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
			"type":    "rate_limit_error",
			"code":    "websocket_connection_limit_reached",
			"message": "Responses WebSocket connection limit reached",
		}})
		return
	}
	defer releaseResponsesWebsocketConnection(tokenID)

	client, err := responsesWebsocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.LogError(c, "upgrade Responses WebSocket: "+err.Error())
		return
	}
	defer client.Close()
	client.SetReadLimit(responsesWebsocketReadLimit)
	_ = client.SetReadDeadline(time.Now().Add(responsesWebsocketMaxDuration))

	firstType, firstPayload, err := client.ReadMessage()
	if err != nil {
		return
	}
	if firstType != websocket.TextMessage && firstType != websocket.BinaryMessage {
		return
	}
	_, firstRequest, err := parseResponsesWebsocketCreate(firstPayload)
	var clientWriteMu sync.Mutex
	if err != nil {
		writeResponsesWebsocketError(client, &clientWriteMu, "invalid_request", err, http.StatusBadRequest)
		return
	}
	if _, err := selectResponsesWebsocketChannel(c, firstRequest.Model); err != nil {
		writeResponsesWebsocketError(client, &clientWriteMu, "model_not_found", err, http.StatusServiceUnavailable)
		return
	}

	state := &responsesWebsocketState{modelName: firstRequest.Model}
	if !state.reserve() {
		return
	}
	info, firstUpstreamPayload, err := prepareResponsesWebsocketCreate(c, state, firstPayload)
	if err != nil {
		state.cancelReservation()
		writeResponsesWebsocketError(client, &clientWriteMu, "invalid_request", err, http.StatusBadRequest)
		return
	}
	upstream, err := relay.DialResponsesWebsocket(c, info)
	if err != nil {
		if info.Billing != nil {
			info.Billing.Refund(c)
		}
		state.cancelReservation()
		writeResponsesWebsocketError(client, &clientWriteMu, "upstream_connection_error", err, http.StatusBadGateway)
		return
	}
	defer upstream.Close()
	upstream.SetReadLimit(responsesWebsocketReadLimit)
	_ = upstream.SetReadDeadline(time.Now().Add(responsesWebsocketMaxDuration))

	state.activate(info)
	if err := upstream.WriteMessage(websocket.TextMessage, firstUpstreamPayload); err != nil {
		refundResponsesWebsocketActive(c, state)
		writeResponsesWebsocketError(client, &clientWriteMu, "upstream_write_error", err, http.StatusBadGateway)
		return
	}

	clientDone := make(chan error, 1)
	go func() {
		for {
			messageType, payload, readErr := client.ReadMessage()
			if readErr != nil {
				_ = upstream.Close()
				clientDone <- readErr
				return
			}
			var envelope struct {
				Type string `json:"type"`
			}
			if err := common.Unmarshal(payload, &envelope); err != nil {
				writeResponsesWebsocketError(client, &clientWriteMu, "invalid_request", err, http.StatusBadRequest)
				continue
			}
			if envelope.Type == "response.create" {
				if !state.reserve() {
					writeResponsesWebsocketError(client, &clientWriteMu, "response_in_flight", fmt.Errorf("only one response may be in flight per connection"), http.StatusConflict)
					continue
				}
				nextInfo, converted, err := prepareResponsesWebsocketCreate(c, state, payload)
				if err != nil {
					state.cancelReservation()
					writeResponsesWebsocketError(client, &clientWriteMu, "invalid_request", err, http.StatusBadRequest)
					continue
				}
				state.activate(nextInfo)
				payload = converted
				messageType = websocket.TextMessage
			}
			if err := upstream.WriteMessage(messageType, payload); err != nil {
				refundResponsesWebsocketActive(c, state)
				clientDone <- err
				return
			}
		}
	}()

	for {
		select {
		case <-clientDone:
			refundResponsesWebsocketActive(c, state)
			return
		default:
		}

		messageType, payload, err := upstream.ReadMessage()
		if err != nil {
			refundResponsesWebsocketActive(c, state)
			if !isExpectedWebsocketClose(err) {
				writeResponsesWebsocketError(client, &clientWriteMu, "upstream_disconnected", err, http.StatusBadGateway)
			}
			return
		}

		var event dto.ResponsesStreamResponse
		if err := common.Unmarshal(payload, &event); err == nil {
			switch event.Type {
			case "response.output_text.delta":
				state.appendText(event.Delta)
			case dto.ResponsesOutputTypeItemDone:
				state.recordItem(event.Item)
			case "response.completed", "response.incomplete":
				settleResponsesWebsocketRequest(c, state.takeActive(), event.Response)
			case "response.failed", "error":
				active := state.takeActive()
				if usage := usageFromResponsesWebsocket(event.Response); usage != nil && usage.TotalTokens > 0 {
					settleResponsesWebsocketRequest(c, active, event.Response)
				} else if active != nil && active.info != nil && active.info.Billing != nil {
					active.info.Billing.Refund(c)
				}
			}
		}

		clientWriteMu.Lock()
		writeErr := client.WriteMessage(messageType, payload)
		clientWriteMu.Unlock()
		if writeErr != nil {
			refundResponsesWebsocketActive(c, state)
			return
		}
	}
}
