package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func NormalizeClaudeCodeProxyBaseURL(baseURL string) string {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return ""
	}
	parsedURL, err := url.Parse(raw)
	if err != nil {
		return strings.TrimRight(raw, "/")
	}

	pathname := strings.TrimRight(parsedURL.Path, "/")
	for _, suffix := range []string{"/v1/chat/completions", "/v1/messages"} {
		if strings.HasSuffix(strings.ToLower(pathname), suffix) {
			pathname = strings.TrimRight(pathname[:len(pathname)-len(suffix)], "/")
			break
		}
	}
	if pathname == "" {
		pathname = "/"
	}
	parsedURL.Path = pathname
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return strings.TrimRight(parsedURL.String(), "/")
}

func FetchClaudeCodeAccountInfo(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	proxyToken string,
) (statusCode int, body []byte, err error) {
	if client == nil {
		return 0, nil, fmt.Errorf("nil http client")
	}
	bu := NormalizeClaudeCodeProxyBaseURL(baseURL)
	if bu == "" {
		return 0, nil, fmt.Errorf("empty baseURL")
	}
	token := strings.TrimSpace(proxyToken)
	if token == "" {
		return 0, nil, fmt.Errorf("empty proxy token")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bu+"/account-info", nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}
