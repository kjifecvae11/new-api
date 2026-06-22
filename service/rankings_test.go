package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelMetaInfersDefaultVendorWhenPricingMetaMissing(t *testing.T) {
	item := modelMeta("codex-auto-review", map[string]rankingModelMeta{})

	require.Equal(t, "OpenAI", item.vendor)
	require.Equal(t, "OpenAI", item.vendorIcon)
}

func TestModelMetaKeepsExplicitPricingVendor(t *testing.T) {
	item := modelMeta("codex-auto-review", map[string]rankingModelMeta{
		"codex-auto-review": {
			vendor:     "Custom Vendor",
			vendorIcon: "Custom.Icon",
		},
	})

	require.Equal(t, "Custom Vendor", item.vendor)
	require.Equal(t, "Custom.Icon", item.vendorIcon)
}
