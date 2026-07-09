package common

import "fmt"

const DefaultLogChatContentMaxBytes = 64 * 1024

func EffectiveLogChatContentMaxBytes() int {
	if LogChatContentMaxBytes > 0 {
		return LogChatContentMaxBytes
	}
	return DefaultLogChatContentMaxBytes
}

func TruncateLogChatContent(content string) string {
	return TruncateLogChatContentBytes([]byte(content), int64(len([]byte(content))))
}

func TruncateLogChatContentBytes(data []byte, originalSize int64) string {
	limit := EffectiveLogChatContentMaxBytes()
	if originalSize <= int64(limit) && len(data) <= limit {
		return string(data)
	}
	if len(data) > limit {
		data = data[:limit]
	}
	return string(data) + fmt.Sprintf("\n[TRUNCATED_BY_LOG_CHAT_CONTENT: original_bytes=%d limit_bytes=%d]", originalSize, limit)
}
