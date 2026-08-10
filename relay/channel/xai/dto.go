package xai

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/dto"
)

// ChatCompletionResponse represents the response from XAI chat completion API
type ChatCompletionResponse struct {
	Id                string                         `json:"id"`
	Object            string                         `json:"object"`
	Created           int64                          `json:"created"`
	Model             string                         `json:"model"`
	Choices           []dto.OpenAITextResponseChoice `json:"choices"`
	Usage             *dto.Usage                     `json:"usage"`
	SystemFingerprint string                         `json:"system_fingerprint"`
}

type ImageRequest struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt" binding:"required"`
	N              *uint           `json:"n,omitempty"`
	ResponseFormat string          `json:"response_format,omitempty"`
	AspectRatio    json.RawMessage `json:"aspect_ratio,omitempty"`
	Resolution     json.RawMessage `json:"resolution,omitempty"`
}
