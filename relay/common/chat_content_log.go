package common

import (
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

func isChatContentLogPath(path string) bool {
	switch {
	case strings.HasPrefix(path, "/v1/chat/completions"):
		return true
	case strings.HasPrefix(path, "/v1/responses"):
		return true
	case strings.HasPrefix(path, "/v1/messages"):
		return true
	case strings.HasPrefix(path, "/pg/chat/completions"):
		return true
	default:
		return false
	}
}

func (info *RelayInfo) CaptureRequestContent(c *gin.Context) {
	if info == nil || c == nil || c.Request == nil || !common.LogChatContentEnabled {
		return
	}
	if !isChatContentLogPath(c.Request.URL.Path) {
		return
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return
	}
	limit := common.EffectiveLogChatContentMaxBytes()
	if _, err = storage.Seek(0, io.SeekStart); err != nil {
		return
	}
	data, readErr := io.ReadAll(io.LimitReader(storage, int64(limit)+1))
	_, _ = storage.Seek(0, io.SeekStart)
	if readErr != nil {
		return
	}
	info.RequestContent = common.TruncateLogChatContentBytes(data, storage.Size())
}

func (info *RelayInfo) SetResponseContent(content string) {
	if info == nil || !common.LogChatContentEnabled {
		return
	}
	info.ResponseContent = common.TruncateLogChatContent(content)
}
