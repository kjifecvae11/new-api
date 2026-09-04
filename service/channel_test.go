package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"
)

func TestShouldDisableRelayChannelSkipsTransientCodex5xx(t *testing.T) {
	err := types.NewErrorWithStatusCode(
		errors.New("transient upstream"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)

	if ShouldDisableRelayChannel(constant.ChannelTypeCodex, err) {
		t.Fatalf("transient Codex 5xx must not auto-disable the channel")
	}
}
