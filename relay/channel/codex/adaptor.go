package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

const imageGenerationToolType = "image_generation"

// ensureImageGenerationTool makes the hosted image tool available to every
// regular Codex Responses request. Existing API-key clients do not need to
// change their request body: the model can decide whether an image is needed,
// while explicit tool_choice values and all client-provided tools are kept.
func ensureImageGenerationTool(rawTools json.RawMessage) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(rawTools))
	if trimmed == "" || trimmed == "null" {
		return json.RawMessage(`[{"type":"image_generation"}]`), nil
	}

	var tools []map[string]any
	if err := common.Unmarshal(rawTools, &tools); err != nil {
		return nil, fmt.Errorf("codex channel: invalid Responses tools: %w", err)
	}
	for _, tool := range tools {
		if toolType, _ := tool["type"].(string); toolType == imageGenerationToolType {
			return rawTools, nil
		}
	}

	tools = append(tools, map[string]any{"type": imageGenerationToolType})
	encoded, err := common.Marshal(tools)
	if err != nil {
		return nil, fmt.Errorf("codex channel: append image generation tool: %w", err)
	}
	return encoded, nil
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("codex channel: endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("codex channel: /v1/messages endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("codex channel: endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info == nil || info.RelayMode != relayconstant.RelayModeImagesGenerations {
		return nil, errors.New("codex channel: only /v1/images/generations is supported for images")
	}
	return request, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("codex channel: /v1/chat/completions endpoint not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("codex channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("codex channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	isCompact := info != nil && info.RelayMode == relayconstant.RelayModeResponsesCompact

	// The public Responses API accepts a plain string for `input`, while the
	// ChatGPT Codex backend requires an input-item list. Normalize the public
	// shorthand at the server boundary so API-key clients can use official SDK
	// examples, including the image_generation tool, without handling the
	// private Codex transport shape themselves.
	if len(request.Input) > 0 {
		var inputText string
		if err := common.Unmarshal(request.Input, &inputText); err == nil {
			normalizedInput, err := common.Marshal([]map[string]any{
				{
					"role":    "user",
					"content": inputText,
				},
			})
			if err != nil {
				return nil, err
			}
			request.Input = normalizedInput
		}
	}

	if info != nil && info.ChannelSetting.SystemPrompt != "" {
		systemPrompt := info.ChannelSetting.SystemPrompt

		if len(request.Instructions) == 0 {
			if b, err := common.Marshal(systemPrompt); err == nil {
				request.Instructions = b
			} else {
				return nil, err
			}
		} else if info.ChannelSetting.SystemPromptOverride {
			var existing string
			if err := common.Unmarshal(request.Instructions, &existing); err == nil {
				existing = strings.TrimSpace(existing)
				if existing == "" {
					if b, err := common.Marshal(systemPrompt); err == nil {
						request.Instructions = b
					} else {
						return nil, err
					}
				} else {
					if b, err := common.Marshal(systemPrompt + "\n" + existing); err == nil {
						request.Instructions = b
					} else {
						return nil, err
					}
				}
			} else {
				if b, err := common.Marshal(systemPrompt); err == nil {
					request.Instructions = b
				} else {
					return nil, err
				}
			}
		}
	}
	// Codex backend requires the `instructions` field to be present.
	// Keep it consistent with Codex CLI behavior by defaulting to an empty string.
	if len(request.Instructions) == 0 {
		request.Instructions = json.RawMessage(`""`)
	}

	if isCompact {
		return request, nil
	}

	// The upstream hosted tool performs generation with the server-side Codex
	// OAuth credential and returns image_generation_call.result as Base64. This
	// keeps the user's existing NewAPI key and client request unchanged.
	tools, err := ensureImageGenerationTool(request.Tools)
	if err != nil {
		return nil, err
	}
	request.Tools = tools

	// codex: store must be false
	request.Store = json.RawMessage("false")
	// The ChatGPT Codex backend only accepts streaming Responses requests.
	// Non-streaming clients are served by aggregating the upstream SSE stream.
	stream := true
	request.Stream = &stream
	// rm max_output_tokens
	request.MaxOutputTokens = nil
	request.Temperature = nil
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	client, err := service.NewNoReuseHttpClient(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, err
	}
	return doCodexRequestWithStreamRetry(a, c, info, requestBody, client)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == relayconstant.RelayModeImagesGenerations {
		return openai.OpenaiHandlerWithUsage(c, info, resp)
	}
	if info.RelayMode != relayconstant.RelayModeResponses && info.RelayMode != relayconstant.RelayModeResponsesCompact {
		return nil, types.NewError(errors.New("codex channel: endpoint not supported"), types.ErrorCodeInvalidRequest)
	}

	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		return openai.OaiResponsesCompactionHandler(c, info, resp)
	}

	if info.IsStream {
		return openai.OaiResponsesStreamHandler(c, info, resp)
	}
	return openai.OaiResponsesStreamToResponseHandler(c, info, resp)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode == relayconstant.RelayModeImagesGenerations {
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, "/backend-api/codex/images/generations", info.ChannelType), nil
	}
	if info.RelayMode != relayconstant.RelayModeResponses && info.RelayMode != relayconstant.RelayModeResponsesCompact {
		return "", errors.New("codex channel: only /v1/responses, /v1/responses/compact, and /v1/images/generations are supported")
	}
	path := "/backend-api/codex/responses"
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		path = "/backend-api/codex/responses/compact"
	}
	baseURL := info.ChannelBaseUrl
	if info.ResponsesWebsocket {
		switch {
		case strings.HasPrefix(baseURL, "https://"):
			baseURL = "wss://" + strings.TrimPrefix(baseURL, "https://")
		case strings.HasPrefix(baseURL, "http://"):
			baseURL = "ws://" + strings.TrimPrefix(baseURL, "http://")
		}
	}
	return relaycommon.GetFullRequestURL(baseURL, path, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	key := strings.TrimSpace(info.ApiKey)
	if !strings.HasPrefix(key, "{") {
		return errors.New("codex channel: key must be a JSON object")
	}

	oauthKey, err := ParseOAuthKey(key)
	if err != nil {
		return err
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)

	if accessToken == "" {
		return errors.New("codex channel: access_token is required")
	}
	if accountID == "" {
		return errors.New("codex channel: account_id is required")
	}

	req.Set("Authorization", "Bearer "+accessToken)
	req.Set("chatgpt-account-id", accountID)

	// The ChatGPT Codex backend uses its own beta namespace. In particular,
	// forwarding the public API's responses_websockets beta value makes the
	// backend accept the upgrade and then close with 1011 on response.create.
	req.Set("OpenAI-Beta", "responses=experimental")
	if req.Get("originator") == "" {
		req.Set("originator", "codex_cli_rs")
	}

	// ChatGPT Codex endpoints are strict about Content-Type.
	// Clients may omit it or include parameters like `application/json; charset=utf-8`,
	// which can be rejected by the upstream. Force the exact media type.
	req.Set("Content-Type", "application/json")
	if info.RelayMode == relayconstant.RelayModeResponses || info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}

	return nil
}
