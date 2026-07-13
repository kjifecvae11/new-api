package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEraseUserByIDRevokesCredentialsAndScrubsPersonalData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-erasure?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogDB := DB, LOG_DB
	DB, LOG_DB = db, db
	oldRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedisEnabled })
	oldDeleteTokenCache := eraseDeleteTokenCache
	oldInvalidateUserCache := eraseInvalidateUserCache
	var revokedTokenKeys []string
	var invalidatedUserIDs []int
	eraseDeleteTokenCache = func(key string) error {
		revokedTokenKeys = append(revokedTokenKeys, key)
		return nil
	}
	eraseInvalidateUserCache = func(id int) error {
		invalidatedUserIDs = append(invalidatedUserIDs, id)
		return nil
	}
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		eraseDeleteTokenCache = oldDeleteTokenCache
		eraseInvalidateUserCache = oldInvalidateUserCache
	})

	if err := db.AutoMigrate(
		&User{}, &UserLegalConsent{}, &UserErasureAudit{}, &Log{}, &Token{}, &PasskeyCredential{}, &TwoFABackupCode{},
		&TwoFA{}, &UserOAuthBinding{}, &Checkin{}, &QuotaData{}, &Task{},
		&Midjourney{}, &TopUp{}, &SubscriptionOrder{}, &UserSubscription{},
		&SubscriptionPreConsumeRecord{}, &Redemption{},
	); err != nil {
		t.Fatal(err)
	}
	user := User{
		Username: "privacy-user", Password: "hashed-password", DisplayName: "Private Name",
		Email: "private@example.com", Status: 1, Quota: 1000, AffCode: "privacy-aff",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	key := "secret-management-token"
	allowIP := "203.0.113.1"
	fixtures := []any{
		&Token{UserId: user.Id, Key: "sk-secret", Name: "private token", AllowIps: &allowIP},
		&PasskeyCredential{UserID: user.Id, CredentialID: "credential", PublicKey: "public-key"},
		&TwoFA{UserId: user.Id, Secret: "totp-secret"},
		&UserOAuthBinding{UserId: user.Id, ProviderId: 1, ProviderUserId: "provider-user"},
		&Log{UserId: user.Id, Username: user.Username, TokenName: "private token", Ip: "203.0.113.1", RequestContent: "prompt", ResponseContent: "response", Content: "private detail", ModelName: "safe-model", PromptTokens: 12, CompletionTokens: 4, Other: `{"admin_info":{"secret":"admin-secret"},"user_visible":"retained","route":"internal"}`},
		&TopUp{UserId: user.Id, TradeNo: "trade-retained", Status: common.TopUpStatusSuccess},
		&TopUp{UserId: user.Id, TradeNo: "trade-pending", Status: common.TopUpStatusPending},
		&SubscriptionOrder{UserId: user.Id, TradeNo: "subscription-pending", Status: common.TopUpStatusPending, ProviderPayload: `{"payer_email":"private@example.com","credential":"provider-private"}`},
		&SubscriptionOrder{UserId: user.Id, TradeNo: "subscription-success", Status: common.TopUpStatusSuccess, ProviderPayload: `{"customer":"private-customer"}`},
		&UserSubscription{UserId: user.Id, Status: "active", UpgradeGroup: "private-tier", PrevUserGroup: "private-old-tier"},
		&SubscriptionPreConsumeRecord{RequestId: "private-idempotency-handle", UserId: user.Id, Status: "consumed"},
		&Redemption{UserId: user.Id, Key: "redeem-created-secret", Name: "private campaign", Status: common.RedemptionCodeStatusEnabled},
		&Redemption{UsedUserId: user.Id, Key: "redeem-used-secret", Name: "private used code", Status: common.RedemptionCodeStatusUsed},
	}
	user.AccessToken = &key
	if err := db.Model(&user).Update("access_token", key).Error; err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		if err := db.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	exported, err := ExportUserData(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	exportJSON, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"hashed-password", key, "sk-secret", "totp-secret"} {
		if strings.Contains(string(exportJSON), secret) {
			t.Fatalf("export contains secret value %q", secret)
		}
	}
	exportedLogsJSON, err := json.Marshal(exported.Logs)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"admin-secret", "retained", "internal", "other", "request_content", "response_content", "channel", "token_id", "request_id", "ip", "content"} {
		if strings.Contains(string(exportedLogsJSON), forbidden) {
			t.Fatalf("export contains forbidden log field or value %q: %s", forbidden, exportedLogsJSON)
		}
	}
	if exported.Profile.ID != user.Id || len(exported.Logs) != 1 || exported.Logs[0].ModelName != "safe-model" || exported.Logs[0].PromptTokens != 12 {
		t.Fatalf("export is missing allowlisted usage data: %+v", exported)
	}

	verifiedAt := time.Now().Unix()
	if err := EraseUserByIDWithFreshProof(user.Id, "2fa", verifiedAt); err != nil {
		t.Fatal(err)
	}
	if len(revokedTokenKeys) != 1 || revokedTokenKeys[0] != "sk-secret" || len(invalidatedUserIDs) != 1 || invalidatedUserIDs[0] != user.Id {
		t.Fatalf("credential caches were not synchronously revoked: token_keys=%v user_ids=%v", revokedTokenKeys, invalidatedUserIDs)
	}

	var erased User
	if err := db.Unscoped().First(&erased, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !erased.DeletedAt.Valid || erased.Username != "deleted-user-1" || erased.Email != "" || erased.AccessToken != nil || erased.Quota != 0 {
		t.Fatalf("user was not safely pseudonymized: %+v", erased)
	}
	var audit UserErasureAudit
	if err := db.First(&audit, "pseudonymous_user_ref = ?", "deleted-user-1").Error; err != nil {
		t.Fatal(err)
	}
	if audit.ActorType != "self" || audit.AuthenticationMethod != "2fa" || audit.VerifiedAt != verifiedAt || audit.ErasedAt < verifiedAt {
		t.Fatalf("self-erasure authorization evidence is incomplete: %+v", audit)
	}
	for name, model := range map[string]any{
		"token": &Token{}, "passkey": &PasskeyCredential{}, "twofa": &TwoFA{}, "oauth": &UserOAuthBinding{},
	} {
		var count int64
		if err := db.Unscoped().Model(model).Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0", name, count)
		}
	}
	var logEntry Log
	if err := db.First(&logEntry, "user_id = ?", user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if logEntry.Username != "deleted-user-1" || logEntry.Ip != "" || logEntry.RequestContent != "" || logEntry.ResponseContent != "" {
		t.Fatalf("log was not scrubbed: %+v", logEntry)
	}
	lateLog := Log{UserId: user.Id, Username: "stale-context-user", Ip: "203.0.113.2", RequestContent: "late prompt", Other: `{"admin_info":{"secret":"late"}}`}
	if err := db.Create(&lateLog).Error; err != nil {
		t.Fatal(err)
	}
	if err := EraseUserByID(user.Id); err != nil {
		t.Fatalf("idempotent erasure retry failed: %v", err)
	}
	if err := db.First(&lateLog, lateLog.Id).Error; err != nil {
		t.Fatal(err)
	}
	if lateLog.Username != "deleted-user-1" || lateLog.Ip != "" || lateLog.RequestContent != "" || lateLog.Other != "{}" {
		t.Fatalf("retry did not scrub a late in-flight log: %+v", lateLog)
	}
	var retainedTopups []TopUp
	if err := db.Where("user_id = ?", user.Id).Order("id").Find(&retainedTopups).Error; err != nil {
		t.Fatal(err)
	}
	if len(retainedTopups) != 2 || retainedTopups[0].Status != common.TopUpStatusSuccess || retainedTopups[1].Status != common.TopUpStatusFailed {
		t.Fatalf("topup settlement records were not safely retained: %+v", retainedTopups)
	}
	var orders []SubscriptionOrder
	if err := db.Where("user_id = ?", user.Id).Order("id").Find(&orders).Error; err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 || orders[0].Status != common.TopUpStatusFailed || orders[0].ProviderPayload != "" || orders[1].ProviderPayload != "" {
		t.Fatalf("subscription orders retained PII or an actionable pending state: %+v", orders)
	}
	var subscription UserSubscription
	if err := db.First(&subscription, "user_id = ?", user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if subscription.Status != "cancelled" || subscription.UpgradeGroup != "" || subscription.PrevUserGroup != "" {
		t.Fatalf("live subscription entitlement survived erasure: %+v", subscription)
	}
	var preConsumeCount int64
	if err := db.Model(&SubscriptionPreConsumeRecord{}).Where("user_id = ?", user.Id).Count(&preConsumeCount).Error; err != nil || preConsumeCount != 0 {
		t.Fatalf("subscription idempotency handle survived: count=%d err=%v", preConsumeCount, err)
	}
	var redemptions []Redemption
	if err := db.Unscoped().Where("user_id = ? OR used_user_id = ?", user.Id, user.Id).Order("id").Find(&redemptions).Error; err != nil {
		t.Fatal(err)
	}
	if len(redemptions) != 2 || redemptions[0].Status != common.RedemptionCodeStatusDisabled || redemptions[1].Status != common.RedemptionCodeStatusUsed {
		t.Fatalf("redemption status retention is unsafe: %+v", redemptions)
	}
	financialJSON, err := json.Marshal(struct {
		Orders      []SubscriptionOrder
		Redemptions []Redemption
	}{orders, redemptions})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private@example.com", "provider-private", "private-customer", "redeem-created-secret", "redeem-used-secret", "private campaign", "private used code"} {
		if strings.Contains(string(financialJSON), forbidden) {
			t.Fatalf("retained financial records contain PII or usable credential %q: %s", forbidden, financialJSON)
		}
	}
}

func TestEraseUserByIDWithFreshProofFailsClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-erasure-fresh-proof?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogDB := DB, LOG_DB
	DB, LOG_DB = db, db
	t.Cleanup(func() { DB, LOG_DB = oldDB, oldLogDB })
	if err := db.AutoMigrate(
		&User{}, &UserErasureAudit{}, &Log{}, &Token{}, &PasskeyCredential{}, &TwoFABackupCode{},
		&TwoFA{}, &UserOAuthBinding{}, &Checkin{}, &QuotaData{}, &Task{}, &Midjourney{},
		&TopUp{}, &SubscriptionOrder{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}, &Redemption{},
	); err != nil {
		t.Fatal(err)
	}
	user := User{Username: "fresh-proof", Password: "hashed-password", Status: common.UserStatusEnabled, AffCode: "fresh-proof-aff"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method     string
		verifiedAt int64
	}{
		{"", time.Now().Unix()},
		{"password", time.Now().Unix()},
		{"2fa", time.Now().Add(-301 * time.Second).Unix()},
		{"passkey", time.Now().Add(time.Minute).Unix()},
	} {
		if err := EraseUserByIDWithFreshProof(user.Id, tc.method, tc.verifiedAt); err == nil {
			t.Fatalf("unsafe proof was accepted: %+v", tc)
		}
	}
	var current User
	if err := db.First(&current, user.Id).Error; err != nil || current.Status != common.UserStatusEnabled {
		t.Fatalf("failed proof mutated account: user=%+v err=%v", current, err)
	}
}

func TestEraseUserByIDReportsCacheRevocationFailureAfterCommit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-erasure-cache-failure?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogDB := DB, LOG_DB
	DB, LOG_DB = db, db
	oldDeleteTokenCache := eraseDeleteTokenCache
	oldInvalidateUserCache := eraseInvalidateUserCache
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		eraseDeleteTokenCache = oldDeleteTokenCache
		eraseInvalidateUserCache = oldInvalidateUserCache
	})
	if err := db.AutoMigrate(
		&User{}, &UserErasureAudit{}, &Log{}, &Token{}, &PasskeyCredential{}, &TwoFABackupCode{},
		&TwoFA{}, &UserOAuthBinding{}, &Checkin{}, &QuotaData{}, &Task{},
		&Midjourney{}, &TopUp{}, &SubscriptionOrder{}, &UserSubscription{},
		&SubscriptionPreConsumeRecord{}, &Redemption{},
	); err != nil {
		t.Fatal(err)
	}
	user := User{Username: "cache-failure", Password: "hashed-password", Status: common.UserStatusEnabled, AffCode: "cache-failure-aff"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Token{UserId: user.Id, Key: "cache-key"}).Error; err != nil {
		t.Fatal(err)
	}
	var tokenAttempted, userAttempted bool
	eraseDeleteTokenCache = func(key string) error {
		tokenAttempted = key == "cache-key"
		return errors.New("redis token delete failed")
	}
	eraseInvalidateUserCache = func(id int) error {
		userAttempted = id == user.Id
		return errors.New("redis user delete failed")
	}

	err = EraseUserByID(user.Id)
	if err == nil || !strings.Contains(err.Error(), "credential cache revocation failed") {
		t.Fatalf("expected explicit cache revocation failure, got %v", err)
	}
	if !tokenAttempted || !userAttempted {
		t.Fatalf("all cache revocations must be attempted: token=%v user=%v", tokenAttempted, userAttempted)
	}
	var erased User
	if err := db.Unscoped().First(&erased, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !erased.DeletedAt.Valid || erased.Status != common.UserStatusDisabled {
		t.Fatalf("durable erasure did not commit before cache failure: %+v", erased)
	}
}
