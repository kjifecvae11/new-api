package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDefaultVendorTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&Vendor{}, &Model{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = previousDB
		LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		initCol()
	})

	return db
}

func TestDefaultVendorMappingBackfillsMissingVendorID(t *testing.T) {
	vendorMap := map[int]*Vendor{
		1: {Id: 1, Name: "OpenAI", Status: 1},
		2: {Id: 2, Name: "Anthropic", Status: 1},
	}
	metaMap := map[string]*Model{
		"gpt-5.5": {
			ModelName: "gpt-5.5",
			VendorID:  0,
			Status:    1,
			NameRule:  NameRuleExact,
		},
		"gpt-5.4-mini": {
			ModelName: "gpt-5.4-mini",
			VendorID:  0,
			Status:    1,
			NameRule:  NameRuleExact,
		},
		"claude-code-default": {
			ModelName: "claude-code-default",
			VendorID:  2,
			Status:    1,
			NameRule:  NameRuleExact,
		},
	}
	abilities := []AbilityWithChannel{
		{Ability: Ability{Model: "gpt-5.5"}},
		{Ability: Ability{Model: "gpt-5.4-mini"}},
		{Ability: Ability{Model: "codex-auto-review"}},
		{Ability: Ability{Model: "claude-code-default"}},
		{Ability: Ability{Model: "private-deployment"}, ChannelType: constant.ChannelTypeCodex},
	}

	initDefaultVendorMapping(metaMap, vendorMap, abilities)

	require.Equal(t, 1, metaMap["gpt-5.5"].VendorID)
	require.Equal(t, 1, metaMap["gpt-5.4-mini"].VendorID)
	require.Equal(t, 1, metaMap["codex-auto-review"].VendorID)
	require.Equal(t, 2, metaMap["claude-code-default"].VendorID)
	require.Equal(t, 1, metaMap["private-deployment"].VendorID)
}

func TestInferDefaultVendorNameUsesOrderedRules(t *testing.T) {
	require.Equal(t, "OpenAI", inferDefaultVendorName("gpt-5.3-codex-spark"))
	require.Equal(t, "OpenAI", inferDefaultVendorName("codex-auto-review"))
	require.Equal(t, "OpenAI", inferDefaultVendorName("private-deployment", constant.ChannelTypeCodex))
	require.Equal(t, "Anthropic", inferDefaultVendorName("private-deployment", constant.ChannelTypeAnthropic))
	require.Equal(t, "讯飞", inferDefaultVendorName("spark-max"))
	require.Empty(t, inferDefaultVendorName("unknown-local-model"))
}

func TestModelInsertAppliesDefaultVendor(t *testing.T) {
	db := setupDefaultVendorTestDB(t)

	meta := &Model{
		ModelName: "codex-auto-review",
		Status:    1,
		NameRule:  NameRuleExact,
	}
	require.NoError(t, meta.Insert())
	require.NotZero(t, meta.VendorID)

	var vendor Vendor
	require.NoError(t, db.First(&vendor, meta.VendorID).Error)
	require.Equal(t, "OpenAI", vendor.Name)
	require.Equal(t, "OpenAI", vendor.Icon)
}
