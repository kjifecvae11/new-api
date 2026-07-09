package model

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLogChatContentTestDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldLogChatContentEnabled := common.LogChatContentEnabled
	oldLogChatContentMaxBytes := common.LogChatContentMaxBytes
	oldDataExportEnabled := common.DataExportEnabled
	oldMode := gin.Mode()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("failed to migrate logs: %v", err)
	}

	DB = db
	LOG_DB = db
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false

	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.LogChatContentEnabled = oldLogChatContentEnabled
		common.LogChatContentMaxBytes = oldLogChatContentMaxBytes
		common.DataExportEnabled = oldDataExportEnabled
		gin.SetMode(oldMode)
	})
}

func newLogChatContentTestContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	req.RemoteAddr = "203.0.113.10:4567"
	c.Request = req
	c.Set("username", "alice")
	c.Set(common.RequestIdKey, "req-chat-log-test")
	return c
}

func TestLogChatContentOptionBehavior(t *testing.T) {
	oldEnabled := common.LogChatContentEnabled
	oldMaxBytes := common.LogChatContentMaxBytes
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.LogChatContentEnabled = oldEnabled
		common.LogChatContentMaxBytes = oldMaxBytes
		common.OptionMap = oldOptionMap
	})

	if err := updateOptionMap("LogChatContentEnabled", "true"); err != nil {
		t.Fatalf("update LogChatContentEnabled failed: %v", err)
	}
	if !common.LogChatContentEnabled {
		t.Fatal("LogChatContentEnabled = false, want true")
	}

	if err := updateOptionMap("LogChatContentMaxBytes", "1234"); err != nil {
		t.Fatalf("update LogChatContentMaxBytes failed: %v", err)
	}
	if common.LogChatContentMaxBytes != 1234 {
		t.Fatalf("LogChatContentMaxBytes = %d, want 1234", common.LogChatContentMaxBytes)
	}

	if err := updateOptionMap("LogChatContentEnabled", "false"); err != nil {
		t.Fatalf("disable LogChatContentEnabled failed: %v", err)
	}
	if common.LogChatContentEnabled {
		t.Fatal("LogChatContentEnabled = true, want false")
	}
}

func TestRecordConsumeLogChatContentSwitchAndUserRedaction(t *testing.T) {
	setupLogChatContentTestDB(t)
	common.LogChatContentEnabled = false
	common.LogChatContentMaxBytes = common.DefaultLogChatContentMaxBytes

	c := newLogChatContentTestContext()
	params := RecordConsumeLogParams{
		ModelName:       "gpt-test",
		TokenName:       "test-token",
		RequestContent:  `{"messages":[{"role":"user","content":"hello"}]}`,
		ResponseContent: `{"choices":[{"message":{"content":"world"}}]}`,
	}
	if serialized := common.GetJsonString(params); strings.Contains(serialized, "hello") || strings.Contains(serialized, "world") {
		t.Fatalf("consume log params serialization leaked chat content: %s", serialized)
	}

	RecordConsumeLog(c, 1, params)

	var disabledLog Log
	if err := LOG_DB.Order("id desc").First(&disabledLog).Error; err != nil {
		t.Fatalf("failed to read disabled log: %v", err)
	}
	if disabledLog.RequestContent != "" || disabledLog.ResponseContent != "" {
		t.Fatalf("content saved while disabled: request=%q response=%q", disabledLog.RequestContent, disabledLog.ResponseContent)
	}

	common.LogChatContentEnabled = true
	RecordConsumeLog(c, 1, params)

	var enabledLog Log
	if err := LOG_DB.Order("id desc").First(&enabledLog).Error; err != nil {
		t.Fatalf("failed to read enabled log: %v", err)
	}
	if !strings.Contains(enabledLog.RequestContent, "hello") {
		t.Fatalf("request content = %q, want original payload", enabledLog.RequestContent)
	}
	if !strings.Contains(enabledLog.ResponseContent, "world") {
		t.Fatalf("response content = %q, want original payload", enabledLog.ResponseContent)
	}
	if enabledLog.Ip != "203.0.113.10" {
		t.Fatalf("ip = %q, want 203.0.113.10", enabledLog.Ip)
	}

	userLogs, _, err := GetUserLogs(1, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "")
	if err != nil {
		t.Fatalf("GetUserLogs failed: %v", err)
	}
	if len(userLogs) == 0 {
		t.Fatal("GetUserLogs returned no logs")
	}
	if userLogs[0].RequestContent != "" || userLogs[0].ResponseContent != "" {
		t.Fatalf("user log leaked content: request=%q response=%q", userLogs[0].RequestContent, userLogs[0].ResponseContent)
	}
}

func TestRecordConsumeLogChatContentTruncatesFields(t *testing.T) {
	setupLogChatContentTestDB(t)
	common.LogChatContentEnabled = true
	common.LogChatContentMaxBytes = 12

	c := newLogChatContentTestContext()
	RecordConsumeLog(c, 1, RecordConsumeLogParams{
		ModelName:       "gpt-test",
		TokenName:       "test-token",
		RequestContent:  strings.Repeat("a", 40),
		ResponseContent: strings.Repeat("b", 40),
	})

	var log Log
	if err := LOG_DB.Order("id desc").First(&log).Error; err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	if !strings.Contains(log.RequestContent, "TRUNCATED_BY_LOG_CHAT_CONTENT") {
		t.Fatalf("request content missing truncation marker: %q", log.RequestContent)
	}
	if !strings.Contains(log.ResponseContent, "TRUNCATED_BY_LOG_CHAT_CONTENT") {
		t.Fatalf("response content missing truncation marker: %q", log.ResponseContent)
	}
	if !strings.HasPrefix(log.RequestContent, strings.Repeat("a", 12)) {
		t.Fatalf("request content prefix = %q", log.RequestContent)
	}
}
