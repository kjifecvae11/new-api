package middleware

import (
	"net/http"
	"testing"
)

func TestRelayTokenHeaderUsesAuthorizationByDefault(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer chatgpt-or-newapi-token")

	if got := relayTokenHeader(request); got != "Bearer chatgpt-or-newapi-token" {
		t.Fatalf("unexpected token header: %q", got)
	}
}

func TestRelayTokenHeaderPrefersDedicatedNewAPIKey(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer chatgpt-access-token")
	request.Header.Set(newAPIKeyHeader, "newapi-user-token")

	if got := relayTokenHeader(request); got != "newapi-user-token" {
		t.Fatalf("dedicated gateway token was not selected: %q", got)
	}
}

func TestRelayTokenHeaderIgnoresBlankDedicatedHeader(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer fallback-token")
	request.Header.Set(newAPIKeyHeader, "   ")

	if got := relayTokenHeader(request); got != "Bearer fallback-token" {
		t.Fatalf("blank dedicated header should fall back to Authorization: %q", got)
	}
}
