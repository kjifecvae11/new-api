package model

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

type defaultVendorRule struct {
	Vendor   string
	Patterns []string
}

// 默认供应商映射规则。顺序即优先级；先根据模型名识别真实模型供应商，
// 再在无法识别时回退到渠道类型归属。
var defaultVendorRules = []defaultVendorRule{
	{Vendor: "OpenAI", Patterns: []string{"gpt", "codex", "dall-e", "dalle", "whisper", "tts", "sora", "omni", "o1", "o3", "o4"}},
	{Vendor: "Anthropic", Patterns: []string{"claude"}},
	{Vendor: "Google", Patterns: []string{"gemini", "gemma", "imagen", "veo", "palm"}},
	{Vendor: "Moonshot", Patterns: []string{"moonshot", "kimi"}},
	{Vendor: "智谱", Patterns: []string{"chatglm", "glm-", "glm_", "cogview", "cogvideo"}},
	{Vendor: "阿里巴巴", Patterns: []string{"qwen", "qwq", "qvq", "wan"}},
	{Vendor: "DeepSeek", Patterns: []string{"deepseek"}},
	{Vendor: "Suno", Patterns: []string{"suno"}},
	{Vendor: "MiniMax", Patterns: []string{"abab", "minimax", "hailuo"}},
	{Vendor: "百度", Patterns: []string{"ernie", "wenxin"}},
	{Vendor: "讯飞", Patterns: []string{"spark"}},
	{Vendor: "腾讯", Patterns: []string{"hunyuan"}},
	{Vendor: "Cohere", Patterns: []string{"command", "cohere", "aya"}},
	{Vendor: "Cloudflare", Patterns: []string{"@cf/"}},
	{Vendor: "360", Patterns: []string{"360"}},
	{Vendor: "零一万物", Patterns: []string{"yi-"}},
	{Vendor: "Jina", Patterns: []string{"jina"}},
	{Vendor: "Mistral", Patterns: []string{"mistral", "mixtral", "codestral", "magistral", "pixtral"}},
	{Vendor: "xAI", Patterns: []string{"grok"}},
	{Vendor: "Meta", Patterns: []string{"llama", "codellama"}},
	{Vendor: "字节跳动", Patterns: []string{"doubao", "seed"}},
	{Vendor: "快手", Patterns: []string{"kling"}},
	{Vendor: "即梦", Patterns: []string{"jimeng"}},
	{Vendor: "Vidu", Patterns: []string{"vidu"}},
	{Vendor: "Midjourney", Patterns: []string{"midjourney", "niji"}},
	{Vendor: "Stability AI", Patterns: []string{"stable-diffusion", "sdxl", "sd-"}},
}

var defaultVendorByChannelType = map[int]string{
	constant.ChannelTypeOpenAI:         "OpenAI",
	constant.ChannelTypeOpenAIMax:      "OpenAI",
	constant.ChannelTypeCodex:          "OpenAI",
	constant.ChannelTypeSora:           "OpenAI",
	constant.ChannelTypeAzure:          "Microsoft",
	constant.ChannelTypeAnthropic:      "Anthropic",
	constant.ChannelTypePaLM:           "Google",
	constant.ChannelTypeGemini:         "Google",
	constant.ChannelTypeVertexAi:       "Google",
	constant.ChannelTypeBaidu:          "百度",
	constant.ChannelTypeBaiduV2:        "百度",
	constant.ChannelTypeZhipu:          "智谱",
	constant.ChannelTypeZhipu_v4:       "智谱",
	constant.ChannelTypeAli:            "阿里巴巴",
	constant.ChannelTypeXunfei:         "讯飞",
	constant.ChannelType360:            "360",
	constant.ChannelTypeTencent:        "腾讯",
	constant.ChannelTypeMoonshot:       "Moonshot",
	constant.ChannelTypeCohere:         "Cohere",
	constant.ChannelTypeMiniMax:        "MiniMax",
	constant.ChannelTypeSunoAPI:        "Suno",
	constant.ChannelTypeJina:           "Jina",
	constant.ChannelTypeMistral:        "Mistral",
	constant.ChannelTypeDeepSeek:       "DeepSeek",
	constant.ChannelTypeVolcEngine:     "字节跳动",
	constant.ChannelTypeXai:            "xAI",
	constant.ChannelTypeKling:          "快手",
	constant.ChannelTypeJimeng:         "即梦",
	constant.ChannelTypeVidu:           "Vidu",
	constant.ChannelTypeMidjourney:     "Midjourney",
	constant.ChannelTypeMidjourneyPlus: "Midjourney",
}

// 供应商默认图标映射
var defaultVendorIcons = map[string]string{
	"OpenAI":       "OpenAI",
	"Anthropic":    "Claude.Color",
	"Google":       "Gemini.Color",
	"Moonshot":     "Moonshot",
	"智谱":           "Zhipu.Color",
	"阿里巴巴":         "Qwen.Color",
	"DeepSeek":     "DeepSeek.Color",
	"Suno":         "",
	"MiniMax":      "Minimax.Color",
	"百度":           "Wenxin.Color",
	"讯飞":           "Spark.Color",
	"腾讯":           "Hunyuan.Color",
	"Cohere":       "Cohere.Color",
	"Cloudflare":   "Cloudflare.Color",
	"360":          "Ai360.Color",
	"零一万物":         "Yi.Color",
	"Jina":         "Jina",
	"Mistral":      "Mistral.Color",
	"xAI":          "XAI",
	"Meta":         "Ollama",
	"字节跳动":         "Doubao.Color",
	"快手":           "Kling.Color",
	"即梦":           "Jimeng.Color",
	"Vidu":         "Vidu",
	"微软":           "AzureAI",
	"Microsoft":    "AzureAI",
	"Azure":        "AzureAI",
	"Midjourney":   "Midjourney",
	"Stability AI": "Stability",
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	channelTypesByModel := collectAbilityChannelTypes(enableAbilities)
	for _, ability := range enableAbilities {
		modelName := ability.Model
		channelTypes := channelTypesByModel[modelName]
		if meta, exists := metaMap[modelName]; exists {
			if meta.VendorID == 0 {
				meta.VendorID = inferDefaultVendorID(modelName, vendorMap, channelTypes...)
			}
			continue
		}

		// 创建模型元数据
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  inferDefaultVendorID(modelName, vendorMap, channelTypes...),
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

func collectAbilityChannelTypes(enableAbilities []AbilityWithChannel) map[string][]int {
	seen := make(map[string]map[int]struct{})
	result := make(map[string][]int)
	for _, ability := range enableAbilities {
		modelName := strings.TrimSpace(ability.Model)
		if modelName == "" || ability.ChannelType == 0 {
			continue
		}
		if seen[modelName] == nil {
			seen[modelName] = make(map[int]struct{})
		}
		if _, ok := seen[modelName][ability.ChannelType]; ok {
			continue
		}
		seen[modelName][ability.ChannelType] = struct{}{}
		result[modelName] = append(result[modelName], ability.ChannelType)
	}
	return result
}

func inferDefaultVendorID(modelName string, vendorMap map[int]*Vendor, channelTypes ...int) int {
	vendorName := inferDefaultVendorName(modelName, channelTypes...)
	if vendorName == "" {
		return 0
	}
	return getOrCreateVendor(vendorName, vendorMap)
}

// InferDefaultVendorName returns the inferred vendor name for a model.
// It first uses model-name rules, then falls back to direct provider channel types.
func InferDefaultVendorName(modelName string, channelTypes ...int) string {
	return inferDefaultVendorName(modelName, channelTypes...)
}

func inferDefaultVendorName(modelName string, channelTypes ...int) string {
	modelLower := strings.ToLower(strings.TrimSpace(modelName))
	for _, rule := range defaultVendorRules {
		for _, pattern := range rule.Patterns {
			if strings.Contains(modelLower, pattern) {
				return rule.Vendor
			}
		}
	}
	for _, channelType := range channelTypes {
		if vendorName := defaultVendorByChannelType[channelType]; vendorName != "" {
			return vendorName
		}
	}
	return ""
}

// EnsureDefaultVendorID returns a database vendor ID for an inferable model vendor.
func EnsureDefaultVendorID(modelName string, channelTypes ...int) int {
	if DB == nil {
		return 0
	}
	var vendors []Vendor
	if err := DB.Find(&vendors).Error; err != nil {
		return 0
	}
	vendorMap := make(map[int]*Vendor, len(vendors))
	for i := range vendors {
		vendorMap[vendors[i].Id] = &vendors[i]
	}
	return inferDefaultVendorID(modelName, vendorMap, channelTypes...)
}

// ApplyDefaultVendor fills a model's missing VendorID when it can be inferred.
func ApplyDefaultVendor(mi *Model, channelTypes ...int) {
	if mi == nil || mi.VendorID != 0 {
		return
	}
	mi.VendorID = EnsureDefaultVendorID(mi.ModelName, channelTypes...)
}

// 查找或创建供应商
func getOrCreateVendor(vendorName string, vendorMap map[int]*Vendor) int {
	// 查找现有供应商
	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id
		}
	}

	// 创建新供应商
	newVendor := &Vendor{
		Name:   vendorName,
		Status: 1,
		Icon:   getDefaultVendorIcon(vendorName),
	}

	if err := newVendor.Insert(); err != nil {
		if DB != nil {
			var existing Vendor
			if findErr := DB.Where("name = ?", vendorName).First(&existing).Error; findErr == nil {
				vendorMap[existing.Id] = &existing
				return existing.Id
			}
		}
		return 0
	}

	vendorMap[newVendor.Id] = newVendor
	return newVendor.Id
}

// DefaultVendorIcon returns the built-in icon name for a vendor.
func DefaultVendorIcon(vendorName string) string {
	return getDefaultVendorIcon(vendorName)
}

// 获取供应商默认图标
func getDefaultVendorIcon(vendorName string) string {
	if icon, exists := defaultVendorIcons[vendorName]; exists {
		return icon
	}
	return ""
}
