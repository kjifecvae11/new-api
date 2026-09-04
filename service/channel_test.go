package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"
)

func TestShouldDisableRelayChannelSkipsTransientCodex5xx(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests, http.StatusBadGateway} {
		err := types.NewErrorWithStatusCode(
			errors.New("transient upstream"),
			types.ErrorCodeBadResponseStatusCode,
			status,
		)
		if ShouldDisableRelayChannel(constant.ChannelTypeCodex, err) {
			t.Fatalf("transient Codex status %d must not auto-disable the channel", status)
		}
	}
}
