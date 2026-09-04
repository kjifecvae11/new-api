package codex

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
)

const (
	codexStreamRetryAttempts = 4
	codexStreamProbeMaxBytes = 4 << 20
	codexStreamProbeTimeout  = 30 * time.Second
)

var errCodexStreamProbeTimeout = errors.New("codex stream probe timeout")

type replayReadCloser struct {
	io.Reader
	closer io.Closer
}

func (r *replayReadCloser) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

func rebuildCodexStreamBody(resp *http.Response, prefix []byte, remainder io.Reader, original io.Closer) {
	resp.Body = &replayReadCloser{
		Reader: io.MultiReader(bytes.NewReader(prefix), remainder),
		closer: original,
	}
	resp.ContentLength = -1
	resp.Header.Del("Content-Length")
}

func codexStreamErrorReason(value any) string {
	if value == nil {
		return ""
	}
	if openAIError := dto.GetOpenAIError(value); openAIError != nil {
		parts := make([]string, 0, 3)
		for _, part := range []string{
			strings.TrimSpace(openAIError.Type),
			strings.TrimSpace(fmt.Sprint(openAIError.Code)),
			strings.TrimSpace(openAIError.Message),
		} {
			if part != "" && part != "<nil>" && !containsFold(parts, part) {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, ": ")
	}
	return ""
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func isRetryableCodexStreamFailure(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	return strings.Contains(code, "overload") ||
		strings.Contains(code, "service_unavailable") ||
		strings.Contains(code, "upstream_server_error") ||
		strings.Contains(code, "server_error") ||
		strings.Contains(code, "temporarily unavailable") ||
		strings.Contains(code, "high demand") ||
		strings.Contains(code, "at capacity") ||
		strings.Contains(code, "try again later")
}

func readCodexStreamLine(ctx context.Context, reader *bufio.Reader, timeout time.Duration) ([]byte, error) {
	type result struct {
		line []byte
		err  error
	}
	resultChan := make(chan result, 1)
	go func() {
		line, err := reader.ReadBytes('\n')
		resultChan <- result{line: line, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-resultChan:
		return result.line, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, errCodexStreamProbeTimeout
	}
}

func probeCodexStreamResponseWithTimeout(ctx context.Context, resp *http.Response, timeout time.Duration) (retry bool, reason string, err error) {
	if resp == nil || resp.Body == nil {
		return false, "", fmt.Errorf("invalid codex stream response")
	}

	originalBody := resp.Body
	reader := bufio.NewReader(originalBody)
	var prefix bytes.Buffer
	completed := false

	for prefix.Len() <= codexStreamProbeMaxBytes {
		line, readErr := readCodexStreamLine(ctx, reader, timeout)
		if errors.Is(readErr, errCodexStreamProbeTimeout) {
			_ = originalBody.Close()
			rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
			return true, "stream_probe_timeout", nil
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			_ = originalBody.Close()
			return false, "", readErr
		}
		if len(line) > 0 {
			_, _ = prefix.Write(line)
		}

		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if data != "" && data != "[DONE]" {
				var event dto.ResponsesStreamResponse
				if unmarshalErr := common.UnmarshalJsonStr(data, &event); unmarshalErr == nil {
					switch event.Type {
					case "response.output_text.delta":
						if event.Delta != "" {
							rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
							return false, "", nil
						}
					case "response.output_text.done":
						if event.Text != "" || event.Delta != "" {
							rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
							return false, "", nil
						}
					case "response.output_item.added", "response.output_item.done":
						if event.Item != nil && event.Item.Type != "" && event.Item.Type != "message" && event.Item.Type != "reasoning" {
							rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
							return false, "", nil
						}
					case "error":
						code := codexStreamErrorReason(event.Error)
						rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
						return isRetryableCodexStreamFailure(code), code, nil
					case "response.failed":
						code := ""
						if event.Response != nil {
							code = codexStreamErrorReason(event.Response.Error)
						}
						if code == "" {
							code = codexStreamErrorReason(event.Error)
						}
						rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
						return isRetryableCodexStreamFailure(code), code, nil
					case "response.incomplete":
						code := ""
						if event.Response != nil && event.Response.IncompleteDetails != nil {
							code = event.Response.IncompleteDetails.Reason
						}
						if code == "" && event.IncompleteDetails != nil {
							code = event.IncompleteDetails.Reason
						}
						rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
						return isRetryableCodexStreamFailure(code), code, nil
					case "response.completed":
						completed = true
						if event.Response != nil && len(event.Response.Output) > 0 {
							rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
							return false, "", nil
						}
					}
				}
			}
		}

		if readErr != nil {
			rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
			if readErr != io.EOF {
				return false, "", readErr
			}
			if completed {
				return true, "empty_completed_response", nil
			}
			return true, "stream_ended_without_output", nil
		}
	}

	rebuildCodexStreamBody(resp, prefix.Bytes(), reader, originalBody)
	return false, "", nil
}

func probeCodexStreamResponse(resp *http.Response) (retry bool, reason string, err error) {
	return probeCodexStreamResponseWithTimeout(context.Background(), resp, codexStreamProbeTimeout)
}

func codexUnavailableResponse(resp *http.Response, reason string) *http.Response {
	if resp == nil {
		resp = &http.Response{Header: make(http.Header)}
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	if resp.Header == nil {
		resp.Header = make(http.Header)
	}
	message := strings.TrimSpace(reason)
	if message == "" {
		message = "Codex upstream is temporarily unavailable"
	}
	body, err := common.Marshal(map[string]any{
		"error": map[string]any{
			"type":    "service_unavailable_error",
			"code":    "codex_upstream_overloaded",
			"message": message,
		},
	})
	if err != nil {
		body = []byte(`{"error":{"type":"service_unavailable_error","code":"codex_upstream_overloaded","message":"Codex upstream is temporarily unavailable"}}`)
	}
	resp.StatusCode = http.StatusServiceUnavailable
	resp.Status = fmt.Sprintf("%d %s", http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable))
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.TransferEncoding = nil
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	if resp.Header.Get("Retry-After") == "" {
		resp.Header.Set("Retry-After", "1")
	}
	return resp
}

func waitCodexStreamRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(250*(1<<(attempt-1))) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetryableCodexHTTPStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooEarly ||
		statusCode == http.StatusTooManyRequests ||
		(statusCode >= http.StatusInternalServerError && statusCode <= 599)
}

func doCodexRequestWithStreamRetry(
	adaptor channel.Adaptor,
	c *gin.Context,
	info *relaycommon.RelayInfo,
	requestBody io.Reader,
	client *http.Client,
) (*http.Response, error) {
	if info == nil || info.RelayMode != relayconstant.RelayModeResponses {
		return channel.DoApiRequestWithClient(adaptor, c, info, requestBody, client)
	}

	body, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("read codex request body: %w", err)
	}

	for attempt := 1; attempt <= codexStreamRetryAttempts; attempt++ {
		resp, requestErr := channel.DoApiRequestWithClient(adaptor, c, info, bytes.NewReader(body), client)
		if requestErr != nil {
			return nil, requestErr
		}
		if resp.StatusCode != http.StatusOK ||
			!strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
			if isRetryableCodexHTTPStatus(resp.StatusCode) && attempt < codexStreamRetryAttempts {
				_ = resp.Body.Close()
				logger.LogWarn(c, fmt.Sprintf("codex HTTP retry %d/%d after status %d", attempt, codexStreamRetryAttempts, resp.StatusCode))
				if err := waitCodexStreamRetry(c.Request.Context(), attempt); err != nil {
					return nil, err
				}
				continue
			}
			return resp, nil
		}

		retry, reason, probeErr := probeCodexStreamResponseWithTimeout(c.Request.Context(), resp, codexStreamProbeTimeout)
		if probeErr != nil {
			return nil, probeErr
		}
		if !retry {
			return resp, nil
		}
		if attempt == codexStreamRetryAttempts {
			return codexUnavailableResponse(resp, reason), nil
		}

		_ = resp.Body.Close()
		logger.LogWarn(c, fmt.Sprintf("codex stream retry %d/%d after %s", attempt, codexStreamRetryAttempts, reason))
		if err := waitCodexStreamRetry(c.Request.Context(), attempt); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("codex stream retry exhausted")
}
