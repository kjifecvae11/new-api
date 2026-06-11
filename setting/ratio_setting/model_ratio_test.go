package ratio_setting

import "testing"

func resetRatioSettingsForTest() {
	modelPriceMap.Clear()
	modelRatioMap.Clear()
	completionRatioMap.Clear()
	cacheRatioMap.Clear()
	createCacheRatioMap.Clear()
	imageRatioMap.Clear()
	audioRatioMap.Clear()
	audioCompletionRatioMap.Clear()
	InitRatioSettings()
}

func TestSuggestedPricingProfiles(t *testing.T) {
	resetRatioSettingsForTest()

	testCases := []struct {
		name           string
		wantRatio      float64
		wantCompletion float64
	}{
		{name: "gpt-5", wantRatio: 0.525, wantCompletion: 8},
		{name: "gpt-5-mini", wantRatio: 0.105, wantCompletion: 8},
		{name: "claude-sonnet-4-5-20250929", wantRatio: 2.1, wantCompletion: 5},
		{name: "claude-sonnet-4.5", wantRatio: 2.1, wantCompletion: 5},
		{name: "gemini-3.1-flash-lite-preview", wantRatio: 0.225, wantCompletion: 6},
		{name: "gemini-3.5-flash", wantRatio: 1.575, wantCompletion: 6},
		{name: "deepseek-v4-flash", wantRatio: 0.6, wantCompletion: 2},
		{name: "qwen3.7-plus", wantRatio: 2.1, wantCompletion: 4},
	}

	for _, tc := range testCases {
		ratio, ok, _ := GetModelRatio(tc.name)
		if !ok {
			t.Fatalf("expected ratio for %s", tc.name)
		}
		if ratio != tc.wantRatio {
			t.Fatalf("unexpected ratio for %s: got %v want %v", tc.name, ratio, tc.wantRatio)
		}

		completionRatio := GetCompletionRatio(tc.name)
		if completionRatio != tc.wantCompletion {
			t.Fatalf("unexpected completion ratio for %s: got %v want %v", tc.name, completionRatio, tc.wantCompletion)
		}
	}
}

func TestSuggestedFixedPriceProfiles(t *testing.T) {
	resetRatioSettingsForTest()

	testCases := []struct {
		name      string
		wantPrice float64
	}{
		{name: "qwen-image", wantPrice: 0.35},
		{name: "qwen-image-2.0", wantPrice: 0.35},
		{name: "wan2.7", wantPrice: 0.89},
		{name: "wan2.7-image-pro", wantPrice: 0.89},
		{name: "suno_music", wantPrice: 0.05},
		{name: "suno_music_open", wantPrice: 0.05},
	}

	for _, tc := range testCases {
		price, ok := GetModelPrice(tc.name, false)
		if !ok {
			t.Fatalf("expected fixed price for %s", tc.name)
		}
		if price != tc.wantPrice {
			t.Fatalf("unexpected fixed price for %s: got %v want %v", tc.name, price, tc.wantPrice)
		}
	}
}
