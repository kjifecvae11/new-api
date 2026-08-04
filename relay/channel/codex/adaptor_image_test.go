package codex

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

func TestCodexImageGenerationRequestIsPassedThrough(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations}
	request := dto.ImageRequest{Model: "gpt-image-2", Prompt: "blue square", Size: "1024x1024", Quality: "low"}

	converted, err := adaptor.ConvertImageRequest(nil, info, request)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := converted.(dto.ImageRequest)
	if !ok || got.Model != request.Model || got.Prompt != request.Prompt {
		t.Fatalf("unexpected converted request: %#v", converted)
	}
}

func TestCodexImageEditRemainsUnsupported(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}

	_, err := adaptor.ConvertImageRequest(nil, info, dto.ImageRequest{Model: "gpt-image-2", Prompt: "edit"})
	if err == nil || !strings.Contains(err.Error(), "/v1/images/generations") {
		t.Fatalf("expected generation-only error, got %v", err)
	}
}

func TestCodexImageGenerationURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://chatgpt.com",
			ChannelType:    constant.ChannelTypeCodex,
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://chatgpt.com/backend-api/codex/images/generations" {
		t.Fatalf("unexpected image generation URL: %s", got)
	}
}

func TestCodexModelListIncludesImageGenerationWithoutCompactAlias(t *testing.T) {
	containsImageModel := false
	for _, model := range ModelList {
		if model == "gpt-image-2" {
			containsImageModel = true
		}
		if strings.HasPrefix(model, "gpt-image-2-") {
			t.Fatalf("image model must not receive a compact alias: %s", model)
		}
	}
	if !containsImageModel {
		t.Fatal("gpt-image-2 missing from Codex model list")
	}
}
