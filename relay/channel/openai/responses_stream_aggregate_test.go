package openai

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func aggregateTestResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

func TestAggregateResponsesStreamBuildsStandardResponseJSON(t *testing.T) {
	body := "event: response.output_text.done\n" +
		"data: {\"type\":\"response.output_text.done\",\"text\":\"OK\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"gpt-5.6-sol\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"

	resp, newAPIError := aggregateResponsesStream(aggregateTestResponse(body))
	if newAPIError != nil {
		t.Fatalf("aggregateResponsesStream() error = %v", newAPIError)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	encoded, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read aggregate body: %v", err)
	}
	var result dto.OpenAIResponsesResponse
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatalf("unmarshal aggregate body: %v", err)
	}
	if result.Object != "response" || len(result.Output) != 1 {
		t.Fatalf("aggregate response shape = object %q, output %d", result.Object, len(result.Output))
	}
	if got := result.Output[0].Content[0].Text; got != "OK" {
		t.Fatalf("output text = %q, want OK", got)
	}
}

func TestAggregateResponsesStreamReturnsOverloadAs503(t *testing.T) {
	body := "event: error\n" +
		"data: {\"type\":\"error\",\"error\":{\"type\":\"service_unavailable_error\",\"code\":\"server_is_overloaded\",\"message\":\"Our servers are currently overloaded.\"}}\n\n" +
		"event: response.failed\n" +
		"data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"server_is_overloaded\",\"message\":\"Our servers are currently overloaded.\"},\"output\":[]}}\n\n"

	_, newAPIError := aggregateResponsesStream(aggregateTestResponse(body))
	if newAPIError == nil {
		t.Fatal("aggregateResponsesStream() error = nil, want overload error")
	}
	if newAPIError.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", newAPIError.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestAggregateResponsesStreamPreservesImageAndFutureMediaFields(t *testing.T) {
	body := "event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_image\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"gpt-5.6-sol\",\"output\":[{\"id\":\"ig_test\",\"type\":\"image_generation_call\",\"status\":\"completed\",\"result\":\"aW1hZ2U=\",\"future_media\":{\"kind\":\"image\"}}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"

	resp, newAPIError := aggregateResponsesStream(aggregateTestResponse(body))
	if newAPIError != nil {
		t.Fatalf("aggregateResponsesStream() error = %v", newAPIError)
	}
	encoded, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type":"image_generation_call"`, `"result":"aW1hZ2U="`, `"future_media":{"kind":"image"}`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("aggregate body lost %s: %s", want, encoded)
		}
	}
}

func TestAggregateResponsesStreamKeepsImageResultWhenCompletedItemMustBeRebuilt(t *testing.T) {
	body := "event: response.output_item.done\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_test\",\"type\":\"image_generation_call\",\"status\":\"completed\",\"result\":\"aW1hZ2U=\"}}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_image\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"gpt-5.6-sol\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"

	resp, newAPIError := aggregateResponsesStream(aggregateTestResponse(body))
	if newAPIError != nil {
		t.Fatalf("aggregateResponsesStream() error = %v", newAPIError)
	}
	encoded, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"result":"aW1hZ2U="`) {
		t.Fatalf("aggregate body lost rebuilt image result: %s", encoded)
	}
}

func TestOaiResponsesStreamHandlerRepairsCompletedOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := "event: response.output_text.done\n" +
		"data: {\"type\":\"response.output_text.done\",\"text\":\"OK\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"gpt-5.6-sol\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"
	resp := aggregateTestResponse(body)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.6-sol",
		},
	}

	_, newAPIError := OaiResponsesStreamHandler(context, info, resp)
	if newAPIError != nil {
		t.Fatalf("OaiResponsesStreamHandler() error = %v", newAPIError)
	}

	var completed dto.ResponsesStreamResponse
	for _, block := range strings.Split(recorder.Body.String(), "\n\n") {
		for _, line := range strings.Split(block, "\n") {
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			var event dto.ResponsesStreamResponse
			if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event); err != nil {
				continue
			}
			if event.Type == "response.completed" {
				completed = event
			}
		}
	}

	if completed.Response == nil {
		t.Fatal("completed response is nil")
	}
	if len(completed.Response.Output) != 1 {
		t.Fatalf("completed output count = %d, want 1", len(completed.Response.Output))
	}
	if got := completed.Response.Output[0].Content[0].Text; got != "OK" {
		t.Fatalf("completed output text = %q, want OK", got)
	}
}
