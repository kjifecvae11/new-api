package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func sessionAuthTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
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
	oldRedisEnabled := common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled = oldRedisEnabled
	})
	return db
}

func sessionAuthCookie(t *testing.T, user model.User, includeVersion bool) (*gin.Engine, []*http.Cookie) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("session-auth-test-secret"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		session.Set("role", user.Role)
		session.Set("status", user.Status)
		session.Set("group", user.Group)
		if includeVersion {
			session.Set("auth_version", common.SessionAuthVersion)
		}
		if err := session.Save(); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/login", nil))
	if login.Code != http.StatusNoContent {
		t.Fatalf("login fixture status = %d", login.Code)
	}
	return router, login.Result().Cookies()
}

func authenticatedRequest(router *gin.Engine, cookies []*http.Cookie, path string, userID int) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("New-Api-User", strconv.Itoa(userID))
	for _, sessionCookie := range cookies {
		request.AddCookie(sessionCookie)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestUserAuthRejectsExistingSessionAfterAdministratorErasesUser(t *testing.T) {
	db := sessionAuthTestDatabase(t)
	target := model.User{Username: "session-target", Password: "hashed-password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "session-target-aff"}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	router, cookies := sessionAuthCookie(t, target, true)
	hit := false
	router.GET("/protected", UserAuth(), func(c *gin.Context) {
		hit = true
		c.Status(http.StatusNoContent)
	})
	if recorder := authenticatedRequest(router, cookies, "/protected", target.Id); recorder.Code != http.StatusNoContent || !hit {
		t.Fatalf("valid session was rejected: status=%d hit=%v", recorder.Code, hit)
	}

	// This is the same erasure entry point used by the administrator deletion
	// controllers. The already-issued cookie must cease to authorize immediately.
	if err := model.EraseUserByID(target.Id); err != nil {
		t.Fatal(err)
	}
	hit = false
	recorder := authenticatedRequest(router, cookies, "/protected", target.Id)
	if recorder.Code != http.StatusUnauthorized || hit {
		t.Fatalf("erased user's existing session remained authorized: status=%d hit=%v body=%s", recorder.Code, hit, recorder.Body.String())
	}
}

func TestAdminAuthUsesCurrentDatabaseRoleAndSessionVersion(t *testing.T) {
	db := sessionAuthTestDatabase(t)
	admin := model.User{Username: "session-admin", Password: "hashed-password", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "session-admin-aff"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	router, cookies := sessionAuthCookie(t, admin, true)
	hit := false
	router.GET("/admin", AdminAuth(), func(c *gin.Context) {
		hit = true
		c.Status(http.StatusNoContent)
	})
	if err := db.Model(&model.User{}).Where("id = ?", admin.Id).Update("role", common.RoleCommonUser).Error; err != nil {
		t.Fatal(err)
	}
	recorder := authenticatedRequest(router, cookies, "/admin", admin.Id)
	if recorder.Code == http.StatusNoContent || hit {
		t.Fatalf("stale administrator role remained authorized: status=%d hit=%v", recorder.Code, hit)
	}

	legacyRouter, legacyCookies := sessionAuthCookie(t, admin, false)
	legacyRouter.GET("/protected", UserAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	legacy := authenticatedRequest(legacyRouter, legacyCookies, "/protected", admin.Id)
	if legacy.Code != http.StatusUnauthorized {
		t.Fatalf("unversioned session status = %d, want 401", legacy.Code)
	}
}
