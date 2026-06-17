package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func looksLikeClaudeCodeProxyChannel(ch *model.Channel) bool {
	if ch == nil || ch.Type != constant.ChannelTypeAnthropic {
		return false
	}

	baseURL := strings.TrimSpace(ch.GetBaseURL())
	if baseURL == "" {
		return false
	}

	signals := []string{
		strings.ToLower(ch.Name),
		strings.ToLower(ch.Models),
		strings.ToLower(baseURL),
	}
	for _, signal := range signals {
		if strings.Contains(signal, "claude-code") {
			return true
		}
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsedURL.Hostname())
	port := parsedURL.Port()
	switch host {
	case "host.docker.internal", "localhost", "127.0.0.1", "::1":
		return port == "13140"
	default:
		return false
	}
}

func GetClaudeCodeChannelAccountInfo(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if !looksLikeClaudeCodeProxyChannel(ch) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel is not a Claude Code proxy channel"})
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	proxyToken := strings.TrimSpace(strings.Split(ch.Key, "\n")[0])
	if proxyToken == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "claude code proxy token is required"})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	statusCode, body, err := service.FetchClaudeCodeAccountInfo(ctx, client, ch.GetBaseURL(), proxyToken)
	if err != nil {
		common.SysError("failed to fetch claude code account info: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取 Claude Code 帐号信息失败，请确认本地代理正在运行"})
		return
	}

	var payload any
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}

	ok := statusCode >= 200 && statusCode < 300
	resp := gin.H{
		"success":         ok,
		"message":         "",
		"upstream_status": statusCode,
		"data":            payload,
	}
	if !ok {
		resp["message"] = fmt.Sprintf("upstream status: %d", statusCode)
	}
	c.JSON(http.StatusOK, resp)
}
