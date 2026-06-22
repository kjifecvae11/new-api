package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchCodexWhamUsageSendsCodexHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/backend-api/wham/usage", r.URL.Path)
		require.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "test-account-id", r.Header.Get("chatgpt-account-id"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		require.Equal(t, "codex-cli", r.Header.Get("User-Agent"))
		require.Equal(t, "codex_cli_rs", r.Header.Get("originator"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_type":"pro"}`))
	}))
	defer server.Close()

	statusCode, body, err := FetchCodexWhamUsage(
		context.Background(),
		server.Client(),
		server.URL+"/",
		" test-access-token ",
		" test-account-id ",
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, statusCode)
	require.JSONEq(t, `{"plan_type":"pro"}`, string(body))
}

func TestFetchCodexWhamUsageRejectsMissingInputs(t *testing.T) {
	t.Parallel()

	client := http.DefaultClient
	tests := []struct {
		name        string
		client      *http.Client
		baseURL     string
		accessToken string
		accountID   string
		want        string
	}{
		{
			name:        "nil client",
			client:      nil,
			baseURL:     "https://chatgpt.com",
			accessToken: "token",
			accountID:   "account",
			want:        "nil http client",
		},
		{
			name:        "empty base url",
			client:      client,
			baseURL:     "",
			accessToken: "token",
			accountID:   "account",
			want:        "empty baseURL",
		},
		{
			name:        "empty access token",
			client:      client,
			baseURL:     "https://chatgpt.com",
			accessToken: "",
			accountID:   "account",
			want:        "empty accessToken",
		},
		{
			name:        "empty account id",
			client:      client,
			baseURL:     "https://chatgpt.com",
			accessToken: "token",
			accountID:   "",
			want:        "empty accountID",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := FetchCodexWhamUsage(
				context.Background(),
				tt.client,
				tt.baseURL,
				tt.accessToken,
				tt.accountID,
			)

			require.EqualError(t, err, tt.want)
		})
	}
}
