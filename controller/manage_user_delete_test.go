package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestManageUserDeleteUsesFullErasurePath(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:manage-user-delete?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Log{}, &model.Token{}, &model.PasskeyCredential{},
		&model.TwoFABackupCode{}, &model.TwoFA{}, &model.UserOAuthBinding{},
		&model.Checkin{}, &model.QuotaData{}, &model.Task{}, &model.Midjourney{},
		&model.TopUp{}, &model.SubscriptionOrder{}, &model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{}, &model.Redemption{}, &model.UserErasureAudit{},
	); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis := common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled = oldRedis
	})

	target := model.User{Username: "managed-delete", Password: "hashed-password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "managed-delete-aff"}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Token{UserId: target.Id, Key: "managed-delete-token"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Log{UserId: target.Id, Username: target.Username, Ip: "203.0.113.8", RequestContent: "private prompt"}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", bytes.NewBufferString(`{"id":1,"action":"delete"}`))
	context.Set("role", common.RoleAdminUser)
	ManageUser(context)
	if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte(`"success":true`)) {
		t.Fatalf("manage delete failed: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var erased model.User
	if err := db.Unscoped().First(&erased, target.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !erased.DeletedAt.Valid || erased.Email != "" || erased.Status != common.UserStatusDisabled {
		t.Fatalf("manage delete bypassed pseudonymization: %+v", erased)
	}
	var tokenCount int64
	if err := db.Unscoped().Model(&model.Token{}).Where("user_id = ?", target.Id).Count(&tokenCount).Error; err != nil || tokenCount != 0 {
		t.Fatalf("manage delete left credentials: count=%d err=%v", tokenCount, err)
	}
	var scrubbed model.Log
	if err := db.First(&scrubbed, "user_id = ?", target.Id).Error; err != nil {
		t.Fatal(err)
	}
	if scrubbed.Ip != "" || scrubbed.RequestContent != "" {
		t.Fatalf("manage delete left personal log data: %+v", scrubbed)
	}
}
