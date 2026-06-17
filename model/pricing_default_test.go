package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultVendorMappingBackfillsMissingVendorID(t *testing.T) {
	vendorMap := map[int]*Vendor{
		1: {Id: 1, Name: "OpenAI", Status: 1},
		2: {Id: 2, Name: "Anthropic", Status: 1},
	}
	metaMap := map[string]*Model{
		"gpt-5.5": {
			ModelName: "gpt-5.5",
			VendorID:  0,
			Status:    1,
			NameRule:  NameRuleExact,
		},
		"gpt-5.4-mini": {
			ModelName: "gpt-5.4-mini",
			VendorID:  0,
			Status:    1,
			NameRule:  NameRuleExact,
		},
		"claude-code-default": {
			ModelName: "claude-code-default",
			VendorID:  2,
			Status:    1,
			NameRule:  NameRuleExact,
		},
	}
	abilities := []AbilityWithChannel{
		{Ability: Ability{Model: "gpt-5.5"}},
		{Ability: Ability{Model: "gpt-5.4-mini"}},
		{Ability: Ability{Model: "codex-auto-review"}},
		{Ability: Ability{Model: "claude-code-default"}},
	}

	initDefaultVendorMapping(metaMap, vendorMap, abilities)

	require.Equal(t, 1, metaMap["gpt-5.5"].VendorID)
	require.Equal(t, 1, metaMap["gpt-5.4-mini"].VendorID)
	require.Equal(t, 1, metaMap["codex-auto-review"].VendorID)
	require.Equal(t, 2, metaMap["claude-code-default"].VendorID)
}

func TestInferDefaultVendorNameUsesOrderedRules(t *testing.T) {
	require.Equal(t, "OpenAI", inferDefaultVendorName("gpt-5.3-codex-spark"))
	require.Equal(t, "OpenAI", inferDefaultVendorName("codex-auto-review"))
	require.Equal(t, "讯飞", inferDefaultVendorName("spark-max"))
	require.Empty(t, inferDefaultVendorName("unknown-local-model"))
}
