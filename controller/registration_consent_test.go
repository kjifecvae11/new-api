package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func consentTestSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func TestRegisterRequiresAndRecordsServerSideLegalConsent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:registration-consent?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserLegalConsent{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis := common.RedisEnabled
	oldRegister, oldPasswordRegister := common.RegisterEnabled, common.PasswordRegisterEnabled
	oldEmailVerification := common.EmailVerificationEnabled
	oldNewUserQuota, oldInviterQuota, oldInviteeQuota := common.QuotaForNewUser, common.QuotaForInviter, common.QuotaForInvitee
	oldGenerateToken := constant.GenerateDefaultToken
	legal := system_setting.GetLegalSettings()
	oldAgreement, oldPrivacy := legal.UserAgreement, legal.PrivacyPolicy
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser, common.QuotaForInviter, common.QuotaForInvitee = 0, 0, 0
	constant.GenerateDefaultToken = false
	legal.UserAgreement = "agreement version under test"
	legal.PrivacyPolicy = "privacy version under test"
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled = oldRedis
		common.RegisterEnabled, common.PasswordRegisterEnabled = oldRegister, oldPasswordRegister
		common.EmailVerificationEnabled = oldEmailVerification
		common.QuotaForNewUser, common.QuotaForInviter, common.QuotaForInvitee = oldNewUserQuota, oldInviterQuota, oldInviteeQuota
		constant.GenerateDefaultToken = oldGenerateToken
		legal.UserAgreement, legal.PrivacyPolicy = oldAgreement, oldPrivacy
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", Register)

	rejected := httptest.NewRecorder()
	router.ServeHTTP(rejected, httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(`{"username":"no-consent","password":"password123"}`)))
	if !strings.Contains(rejected.Body.String(), `"success":false`) {
		t.Fatalf("registration without consent was not rejected: status=%d body=%s", rejected.Code, rejected.Body.String())
	}
	var rejectedUsers int64
	if err := db.Model(&model.User{}).Count(&rejectedUsers).Error; err != nil || rejectedUsers != 0 {
		t.Fatalf("rejected registration persisted a user: count=%d err=%v", rejectedUsers, err)
	}

	before := time.Now().Unix()
	accepted := httptest.NewRecorder()
	acceptedRequest := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(`{"username":"with-consent","password":"password123","legal_consent_accepted":true,"terms_version_sha256":"client-spoof"}`))
	acceptedRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(accepted, acceptedRequest)
	if !strings.Contains(accepted.Body.String(), `"success":true`) {
		t.Fatalf("registration with consent failed: status=%d body=%s", accepted.Code, accepted.Body.String())
	}

	var user model.User
	if err := db.First(&user, "username = ?", "with-consent").Error; err != nil {
		t.Fatal(err)
	}
	var consent model.UserLegalConsent
	if err := db.First(&consent, "user_id = ?", user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if consent.Source != "password_registration" ||
		consent.UserAgreementSHA256 != consentTestSHA256(legal.UserAgreement) ||
		consent.PrivacyPolicySHA256 != consentTestSHA256(legal.PrivacyPolicy) ||
		consent.TermsVersionSHA256 != consentTestSHA256(legal.UserAgreement+"\x00"+legal.PrivacyPolicy) ||
		consent.TermsVersionSHA256 == "client-spoof" ||
		consent.AcceptedAt < before || consent.AcceptedAt > time.Now().Unix() {
		t.Fatalf("server-side consent evidence is invalid: %+v", consent)
	}
}
