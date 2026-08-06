package codex

import (
	"encoding/json"
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

func TestCodexResponsesImageToolNormalizesStringInput(t *testing.T) {
	adaptor := &Adaptor{}
	stream := false
	request := dto.OpenAIResponsesRequest{
		Model:  "gpt-5.4-mini",
		Input:  json.RawMessage(`"draw a blue circle"`),
		Stream: &stream,
		Tools:  json.RawMessage(`[{"type":"image_generation"}]`),
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, request)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("unexpected converted request: %#v", converted)
	}

	var inputItems []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(got.Input, &inputItems); err != nil {
		t.Fatalf("normalized input is not an item list: %v", err)
	}
	if len(inputItems) != 1 || inputItems[0].Role != "user" || inputItems[0].Content != "draw a blue circle" {
		t.Fatalf("unexpected normalized input: %#v", inputItems)
	}
	if got.Stream == nil || !*got.Stream {
		t.Fatal("Codex Responses transport must still force streaming")
	}
	if string(got.Tools) != string(request.Tools) {
		t.Fatalf("image generation tool changed: %s", got.Tools)
	}
}

func TestCodexResponsesImageToolPreservesItemListInput(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.OpenAIResponsesRequest{
		Model: "gpt-5.4-mini",
		Input: json.RawMessage(`[{"role":"user","content":"draw a blue circle"}]`),
		Tools: json.RawMessage(`[{"type":"image_generation"}]`),
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, request)
	if err != nil {
		t.Fatal(err)
	}
	got := converted.(dto.OpenAIResponsesRequest)
	if string(got.Input) != string(request.Input) {
		t.Fatalf("item-list input changed: %s", got.Input)
	}
}

func TestCodexResponsesAutomaticallyOffersImageGeneration(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.OpenAIResponsesRequest{
		Model: "gpt-5.4-mini",
		Input: json.RawMessage(`[{"role":"user","content":"generate a station banner"}]`),
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, request)
	if err != nil {
		t.Fatal(err)
	}
	got := converted.(dto.OpenAIResponsesRequest)

	var tools []map[string]any
	if err := json.Unmarshal(got.Tools, &tools); err != nil {
		t.Fatalf("injected tools are invalid: %v", err)
	}
	if len(tools) != 1 || tools[0]["type"] != imageGenerationToolType {
		t.Fatalf("image generation tool was not injected: %#v", tools)
	}
}

func TestCodexResponsesAppendsImageGenerationToExistingTools(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.OpenAIResponsesRequest{
		Model:      "gpt-5.4-mini",
		Input:      json.RawMessage(`[{"role":"user","content":"generate a station banner"}]`),
		Tools:      json.RawMessage(`[{"type":"function","name":"existing_tool","parameters":{"type":"object"}}]`),
		ToolChoice: json.RawMessage(`"auto"`),
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, request)
	if err != nil {
		t.Fatal(err)
	}
	got := converted.(dto.OpenAIResponsesRequest)

	var tools []map[string]any
	if err := json.Unmarshal(got.Tools, &tools); err != nil {
		t.Fatalf("injected tools are invalid: %v", err)
	}
	if len(tools) != 2 || tools[0]["name"] != "existing_tool" || tools[1]["type"] != imageGenerationToolType {
		t.Fatalf("existing tools were not preserved: %#v", tools)
	}
	if string(got.ToolChoice) != string(request.ToolChoice) {
		t.Fatalf("tool choice changed: %s", got.ToolChoice)
	}
}

func TestCodexResponsesDoesNotDuplicateImageGeneration(t *testing.T) {
	raw := json.RawMessage(`[{"type":"image_generation","quality":"low"}]`)
	got, err := ensureImageGenerationTool(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("existing image tool changed: %s", got)
	}
}

func TestCodexResponsesCompactDoesNotInjectImageGeneration(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.OpenAIResponsesRequest{
		Model: "gpt-5.4-mini",
		Input: json.RawMessage(`[{"role":"user","content":"compact this"}]`),
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponsesCompact,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, request)
	if err != nil {
		t.Fatal(err)
	}
	got := converted.(dto.OpenAIResponsesRequest)
	if len(got.Tools) != 0 {
		t.Fatalf("compact request received image tool: %s", got.Tools)
	}
}

func TestCodexResponsesRejectsMalformedToolsBeforeRelay(t *testing.T) {
	_, err := ensureImageGenerationTool(json.RawMessage(`{"type":"function"}`))
	if err == nil || !strings.Contains(err.Error(), "invalid Responses tools") {
		t.Fatalf("expected invalid tools error, got %v", err)
	}
}
