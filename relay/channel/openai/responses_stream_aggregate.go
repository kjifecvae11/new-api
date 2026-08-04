package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const maxAggregatedResponsesStreamEventSize = 64 << 20

func responsesStreamErrorStatus(openAIError *types.OpenAIError) int {
	if openAIError == nil {
		return http.StatusBadGateway
	}
	code := strings.ToLower(fmt.Sprint(openAIError.Code))
	message := strings.ToLower(openAIError.Message)
	switch {
	case strings.Contains(code, "overload"),
		strings.Contains(code, "service_unavailable"),
		strings.Contains(message, "overload"),
		strings.Contains(message, "service unavailable"),
		strings.Contains(message, "high demand"),
		strings.Contains(message, "at capacity"):
		return http.StatusServiceUnavailable
	case strings.Contains(code, "rate_limit"),
		strings.Contains(message, "rate limit"):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadGateway
	}
}

func ensureResponsesOutputText(completed *dto.OpenAIResponsesResponse, completedItems []dto.ResponsesOutput, text string) bool {
	if completed == nil {
		return false
	}
	changed := false
	if len(completed.Output) == 0 && len(completedItems) > 0 {
		completed.Output = completedItems
		changed = true
	}
	if text == "" {
		return changed
	}

	hasOutputText := false
	for outputIndex := range completed.Output {
		if completed.Output[outputIndex].Type != "message" {
			continue
		}
		for contentIndex := range completed.Output[outputIndex].Content {
			content := &completed.Output[outputIndex].Content[contentIndex]
			if content.Type == "output_text" && content.Text != "" {
				hasOutputText = true
				break
			}
		}
		if !hasOutputText {
			completed.Output[outputIndex].Status = "completed"
			completed.Output[outputIndex].Role = "assistant"
			completed.Output[outputIndex].Content = []dto.ResponsesOutputContent{{
				Type:        "output_text",
				Text:        text,
				Annotations: []interface{}{},
			}}
			hasOutputText = true
			changed = true
		}
		break
	}
	if hasOutputText {
		return changed
	}

	messageID := strings.TrimPrefix(completed.ID, "resp_")
	if messageID == "" {
		messageID = "aggregated"
	}
	completed.Output = append(completed.Output, dto.ResponsesOutput{
		ID:     "msg_" + messageID,
		Type:   "message",
		Status: "completed",
		Role:   "assistant",
		Content: []dto.ResponsesOutputContent{{
			Type:        "output_text",
			Text:        text,
			Annotations: []interface{}{},
		}},
	})
	return true
}

func aggregateResponsesStream(resp *http.Response) (*http.Response, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("invalid responses stream"),
			types.ErrorCodeBadResponse,
			http.StatusInternalServerError,
		)
	}

	originalBody := resp.Body
	defer originalBody.Close()

	scanner := bufio.NewScanner(originalBody)
	scanner.Buffer(make([]byte, 64<<10), maxAggregatedResponsesStreamEventSize)

	var completed *dto.OpenAIResponsesResponse
	var completedRaw json.RawMessage
	var streamError *types.OpenAIError
	var outputText strings.Builder
	var completedItems []dto.ResponsesOutput
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var event dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			return nil, types.NewOpenAIError(
				fmt.Errorf("invalid responses stream event: %w", err),
				types.ErrorCodeBadResponseBody,
				http.StatusBadGateway,
			)
		}

		switch event.Type {
		case "response.output_text.delta":
			outputText.WriteString(event.Delta)
		case "response.output_text.done":
			if outputText.Len() == 0 {
				outputText.WriteString(event.Text)
			}
		case dto.ResponsesOutputTypeItemDone:
			if event.Item != nil {
				completedItems = append(completedItems, *event.Item)
			}
		case "error":
			streamError = dto.GetOpenAIError(event.Error)
		case "response.failed":
			if event.Response != nil {
				streamError = event.Response.GetOpenAIError()
			}
		case "response.completed":
			completed = event.Response
			var rawEvent struct {
				Response json.RawMessage `json:"response"`
			}
			if err := common.Unmarshal([]byte(data), &rawEvent); err == nil {
				completedRaw = append(completedRaw[:0], rawEvent.Response...)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("read responses stream: %w", err),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusBadGateway,
		)
	}
	if streamError != nil {
		return nil, types.WithOpenAIError(*streamError, responsesStreamErrorStatus(streamError))
	}
	if completed == nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("responses stream ended without response.completed"),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}
	changed := ensureResponsesOutputText(completed, completedItems, outputText.String())

	body := completedRaw
	if changed || len(body) == 0 {
		var err error
		body, err = common.Marshal(completed)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	resp.TransferEncoding = nil
	return resp, nil
}

func OaiResponsesStreamToResponseHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	aggregated, newAPIError := aggregateResponsesStream(resp)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return OaiResponsesHandler(c, info, aggregated)
}

func OaiResponsesStreamToChatHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	aggregated, newAPIError := aggregateResponsesStream(resp)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return OaiResponsesToChatHandler(c, info, aggregated)
}
