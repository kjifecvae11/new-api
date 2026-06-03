package codex

import (
	"testing"

	appconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

func TestGetRequestURLNormalizesWebSocketBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "wss",
			baseURL: "wss://chatgpt.com",
			want:    "https://chatgpt.com/backend-api/codex/responses",
		},
		{
			name:    "ws",
			baseURL: "ws://chatgpt.com",
			want:    "http://chatgpt.com/backend-api/codex/responses",
		},
		{
			name:    "trim trailing slash",
			baseURL: "https://chatgpt.com/",
			want:    "https://chatgpt.com/backend-api/codex/responses",
		},
		{
			name:    "default",
			baseURL: "",
			want:    "https://chatgpt.com/backend-api/codex/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeResponses,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: tt.baseURL,
					ChannelType:    appconstant.ChannelTypeCodex,
				},
			}

			got, err := (&Adaptor{}).GetRequestURL(info)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("GetRequestURL() = %q, want %q", got, tt.want)
			}
			if info.ChannelBaseUrl == "" || info.ChannelBaseUrl[:2] == "ws" {
				t.Fatalf("ChannelBaseUrl was not normalized: %q", info.ChannelBaseUrl)
			}
		})
	}
}

func TestGetRequestURLNormalizesCompactBaseURL(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponsesCompact,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "wss://chatgpt.com/",
			ChannelType:    appconstant.ChannelTypeCodex,
		},
	}

	got, err := (&Adaptor{}).GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://chatgpt.com/backend-api/codex/responses/compact"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}
