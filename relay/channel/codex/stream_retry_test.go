package codex

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func codexTestStreamResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

func TestProbeCodexStreamResponseRetriesWhenFirstEventTimesOut(t *testing.T) {
	reader, writer := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: reader}
	done := make(chan struct{})
	go func() {
		defer close(done)
		retry, reason, err := probeCodexStreamResponseWithTimeout(context.Background(), resp, 10*time.Millisecond)
		if err != nil || !retry || reason != "stream_probe_timeout" {
			t.Errorf("probeCodexStreamResponseWithTimeout() = (%v, %q, %v), want timeout retry", retry, reason, err)
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe did not return after first-event timeout")
	}
	_ = writer.Close()
}

func TestIsRetryableCodexHTTPStatus(t *testing.T) {
	for _, status := range []int{408, 425, 429, 500, 502, 503, 599} {
		if !isRetryableCodexHTTPStatus(status) {
			t.Fatalf("status %d should be retried", status)
		}
	}
	for _, status := range []int{400, 401, 400, 403, 600} {
		if isRetryableCodexHTTPStatus(status) {
			t.Fatalf("status %d should not be retried by the Codex HTTP policy", status)
		}
	}
}

func TestProbeCodexStreamResponseRetriesOverloadAndPreservesBody(t *testing.T) {
	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"status\":\"in_progress\",\"output\":[]}}\n\n" +
		"event: error\n" +
		"data: {\"type\":\"error\",\"error\":{\"type\":\"service_unavailable_error\",\"code\":\"server_is_overloaded\",\"message\":\"overloaded\"}}\n\n" +
		"event: response.failed\n" +
		"data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"server_is_overloaded\",\"message\":\"overloaded\"},\"output\":[]}}\n\n"
	resp := codexTestStreamResponse(body)

	retry, reason, err := probeCodexStreamResponse(resp)
	if err != nil {
		t.Fatalf("probeCodexStreamResponse() error = %v", err)
	}
	if !retry {
		t.Fatal("probeCodexStreamResponse() retry = false, want true")
	}
	if !strings.Contains(reason, "server_is_overloaded") {
		t.Fatalf("probeCodexStreamResponse() reason = %q, want server_is_overloaded details", reason)
	}

	replayed, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read replayed body: %v", err)
	}
	if string(replayed) != body {
		t.Fatalf("replayed body mismatch:\ngot:  %q\nwant: %q", replayed, body)
	}
}

func TestProbeCodexStreamResponseRetriesHighDemandMessage(t *testing.T) {
	body := "event: error\n" +
		"data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"We're currently experiencing high demand, which may cause temporary errors.\"}}\n\n"
	resp := codexTestStreamResponse(body)

	retry, reason, err := probeCodexStreamResponse(resp)
	if err != nil {
		t.Fatalf("probeCodexStreamResponse() error = %v", err)
	}
	if !retry || !strings.Contains(reason, "high demand") {
		t.Fatalf("probeCodexStreamResponse() = (%v, %q), want retryable high demand", retry, reason)
	}
}

func TestProbeCodexStreamResponseRetriesUpstreamServerErrorIncomplete(t *testing.T) {
	body := "event: response.incomplete\n" +
		"data: {\"type\":\"response.incomplete\",\"response\":{\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"upstream_server_error\"},\"output\":[]}}\n\n"
	resp := codexTestStreamResponse(body)

	retry, reason, err := probeCodexStreamResponse(resp)
	if err != nil {
		t.Fatalf("probeCodexStreamResponse() error = %v", err)
	}
	if !retry || reason != "upstream_server_error" {
		t.Fatalf("probeCodexStreamResponse() = (%v, %q), want (true, upstream_server_error)", retry, reason)
	}

	replayed, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read replayed body: %v", err)
	}
	if string(replayed) != body {
		t.Fatalf("replayed body mismatch: got %q want %q", replayed, body)
	}
}

func TestProbeCodexStreamResponseRetriesTopLevelIncompleteDetails(t *testing.T) {
	body := "event: response.incomplete\n" +
		"data: {\"type\":\"response.incomplete\",\"incomplete_details\":{\"reason\":\"upstream_server_error\"}}\n\n"
	resp := codexTestStreamResponse(body)

	retry, reason, err := probeCodexStreamResponse(resp)
	if err != nil {
		t.Fatalf("probeCodexStreamResponse() error = %v", err)
	}
	if !retry || reason != "upstream_server_error" {
		t.Fatalf("probeCodexStreamResponse() = (%v, %q), want (true, upstream_server_error)", retry, reason)
	}
}

func TestCodexUnavailableResponseConvertsStreamFailureToRetryableHTTPStatus(t *testing.T) {
	resp := codexUnavailableResponse(codexTestStreamResponse("data: ignored\n\n"), "high demand")

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1", got)
	}
	encoded, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "codex_upstream_overloaded") {
		t.Fatalf("body = %s", encoded)
	}
}

func TestProbeCodexStreamResponseAcceptsTextDeltaAndPreservesRemainder(t *testing.T) {
	body := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"status\":\"in_progress\",\"output\":[]}}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"OK\"}]}]}}\n\n"
	resp := codexTestStreamResponse(body)

	retry, reason, err := probeCodexStreamResponse(resp)
	if err != nil {
		t.Fatalf("probeCodexStreamResponse() error = %v", err)
	}
	if retry {
		t.Fatalf("probeCodexStreamResponse() retry = true, reason = %q", reason)
	}

	replayed, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read replayed body: %v", err)
	}
	if string(replayed) != body {
		t.Fatalf("replayed body mismatch:\ngot:  %q\nwant: %q", replayed, body)
	}
}

func TestProbeCodexStreamResponseRetriesEmptyCompletion(t *testing.T) {
	body := "event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n"
	resp := codexTestStreamResponse(body)

	retry, reason, err := probeCodexStreamResponse(resp)
	if err != nil {
		t.Fatalf("probeCodexStreamResponse() error = %v", err)
	}
	if !retry || reason != "empty_completed_response" {
		t.Fatalf("probeCodexStreamResponse() = (%v, %q), want (true, empty_completed_response)", retry, reason)
	}
}

func TestProbeCodexStreamResponseAcceptsOutputTextDone(t *testing.T) {
	body := "event: response.output_text.done\n" +
		"data: {\"type\":\"response.output_text.done\",\"text\":\"OK\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n"
	resp := codexTestStreamResponse(body)

	retry, reason, err := probeCodexStreamResponse(resp)
	if err != nil {
		t.Fatalf("probeCodexStreamResponse() error = %v", err)
	}
	if retry {
		t.Fatalf("probeCodexStreamResponse() retry = true, reason = %q", reason)
	}
}
