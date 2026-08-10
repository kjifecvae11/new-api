package xai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestConvertImageRequestPreservesXAIOptions(t *testing.T) {
	requestBody := []byte(`{
		"model":"grok-imagine-image-quality",
		"prompt":"a station at dusk",
		"n":0,
		"response_format":"b64_json",
		"aspect_ratio":"16:9",
		"resolution":"2k"
	}`)
	var request dto.ImageRequest
	if err := common.Unmarshal(requestBody, &request); err != nil {
		t.Fatal(err)
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(nil, nil, request)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := common.Marshal(converted)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := common.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}

	if got["n"] != float64(0) {
		t.Fatalf("n = %#v, want explicit zero", got["n"])
	}
	if got["response_format"] != "b64_json" {
		t.Fatalf("response_format = %#v", got["response_format"])
	}
	if got["aspect_ratio"] != "16:9" {
		t.Fatalf("aspect_ratio = %#v", got["aspect_ratio"])
	}
	if got["resolution"] != "2k" {
		t.Fatalf("resolution = %#v", got["resolution"])
	}
}

func TestGrokQualityImageResolutionPricingRatio(t *testing.T) {
	requestBody := []byte(`{
		"model":"grok-imagine-image-quality",
		"prompt":"a station at dusk",
		"resolution":"2k"
	}`)
	var request dto.ImageRequest
	if err := common.Unmarshal(requestBody, &request); err != nil {
		t.Fatal(err)
	}
	if got := request.GetTokenCountMeta().ImagePriceRatio; got != 1.4 {
		t.Fatalf("ImagePriceRatio = %v, want 1.4", got)
	}
}
