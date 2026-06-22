package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type modelSyncResponse struct {
	Success bool `json:"success"`
	Data    struct {
		CreatedModels  int      `json:"created_models"`
		CreatedVendors int      `json:"created_vendors"`
		SkippedModels  []string `json:"skipped_models"`
		CreatedList    []string `json:"created_list"`
	} `json:"data"`
}

func setupModelSyncControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousRedisEnabled := common.RedisEnabled

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.RedisEnabled = previousRedisEnabled
	})

	return db
}

func TestSyncUpstreamModelsCreatesDefaultVendorMetadataWhenUpstreamMissing(t *testing.T) {
	db := setupModelSyncControllerTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/newapi/models.json":
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
		case "/api/newapi/vendors.json":
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	t.Setenv("SYNC_UPSTREAM_BASE", upstream.URL)

	require.NoError(t, db.Create(&model.Channel{
		Id:     1,
		Name:   "codex-channel",
		Type:   constant.ChannelTypeCodex,
		Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "private-deployment",
		ChannelId: 1,
		Enabled:   true,
	}).Error)

	requestBody, err := common.Marshal(map[string]any{})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", bytes.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	SyncUpstreamModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload modelSyncResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 1, payload.Data.CreatedModels)
	require.Equal(t, 1, payload.Data.CreatedVendors)
	require.Empty(t, payload.Data.SkippedModels)
	require.Equal(t, []string{"private-deployment"}, payload.Data.CreatedList)

	var vendor model.Vendor
	require.NoError(t, db.Where("name = ?", "OpenAI").First(&vendor).Error)
	require.Equal(t, "OpenAI", vendor.Icon)

	var meta model.Model
	require.NoError(t, db.Where("model_name = ?", "private-deployment").First(&meta).Error)
	require.Equal(t, vendor.Id, meta.VendorID)
	require.Equal(t, 1, meta.SyncOfficial)
	require.Equal(t, model.NameRuleExact, meta.NameRule)
}

func TestResolveSyncVendorNamePrefersIntrinsicModelVendor(t *testing.T) {
	ownerChannelTypes := map[string]int{
		"private-deployment": constant.ChannelTypeCodex,
	}

	require.Equal(t, "OpenAI", resolveSyncVendorName("gpt-5.5", "Vivgrid", nil))
	require.Equal(t, "OpenAI", resolveSyncVendorName("gpt-5.3-codex-spark", "OpenCode Zen", nil))
	require.Equal(t, "Anthropic", resolveSyncVendorName("claude-sonnet-4-5", "302.AI", nil))
	require.Equal(t, "Vivgrid", resolveSyncVendorName("private-deployment", "Vivgrid", ownerChannelTypes))
	require.Equal(t, "OpenAI", resolveSyncVendorName("private-deployment", "", ownerChannelTypes))
}
