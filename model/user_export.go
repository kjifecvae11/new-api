package model

import (
	"time"
)

const userExportLogLimit = 5000

type UserExportProfile struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	Status       int    `json:"status"`
	Quota        int    `json:"quota"`
	UsedQuota    int    `json:"used_quota"`
	RequestCount int    `json:"request_count"`
	Group        string `json:"group"`
	CreatedAt    int64  `json:"created_at"`
	LastLoginAt  int64  `json:"last_login_at"`
}

type UserExportToken struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	Status             int     `json:"status"`
	CreatedTime        int64   `json:"created_time"`
	AccessedTime       int64   `json:"accessed_time"`
	ExpiredTime        int64   `json:"expired_time"`
	RemainQuota        int     `json:"remain_quota"`
	UsedQuota          int     `json:"used_quota"`
	UnlimitedQuota     bool    `json:"unlimited_quota"`
	ModelLimitsEnabled bool    `json:"model_limits_enabled"`
	ModelLimits        string  `json:"model_limits"`
	AllowIPs           *string `json:"allow_ips"`
	Group              string  `json:"group"`
}

// UserExportLog is an explicit allowlist. It intentionally excludes Log.Other,
// content/error text, IP addresses, request IDs, route/channel identifiers,
// request/response bodies, token identifiers, and administrator/override data.
type UserExportLog struct {
	ID               int    `json:"id"`
	CreatedAt        int64  `json:"created_at"`
	Type             int    `json:"type"`
	ModelName        string `json:"model_name"`
	Quota            int    `json:"quota"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	UseTime          int    `json:"use_time"`
	IsStream         bool   `json:"is_stream"`
}

type UserDataExport struct {
	GeneratedAtUTC string             `json:"generated_at_utc"`
	Profile        UserExportProfile  `json:"profile"`
	Tokens         []UserExportToken  `json:"tokens"`
	Logs           []UserExportLog    `json:"logs"`
	TopUps         []TopUp            `json:"topups"`
	OAuthBindings  []UserOAuthBinding `json:"oauth_bindings"`
	LegalConsents  []UserLegalConsent `json:"legal_consents"`
	LogLimit       int                `json:"log_limit"`
	LogsTruncated  bool               `json:"logs_truncated"`
}

// ExportUserData returns authenticated-user data without passwords, access
// tokens, API token keys, or 2FA secrets. The bounded log export advertises
// truncation so operators can handle larger subject-access requests offline.
func ExportUserData(userID int) (*UserDataExport, error) {
	export := &UserDataExport{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		LogLimit:       userExportLogLimit,
	}
	if err := DB.Model(&User{}).Where("id = ?", userID).Scan(&export.Profile).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&Token{}).Where("user_id = ?", userID).Order("id asc").Scan(&export.Tokens).Error; err != nil {
		return nil, err
	}
	var logCount int64
	if err := LOG_DB.Model(&Log{}).Where("user_id = ?", userID).Count(&logCount).Error; err != nil {
		return nil, err
	}
	export.LogsTruncated = logCount > userExportLogLimit
	if err := LOG_DB.Model(&Log{}).
		Select("id", "created_at", "type", "model_name", "quota", "prompt_tokens", "completion_tokens", "use_time", "is_stream").
		Where("user_id = ?", userID).
		Order("id desc").
		Limit(userExportLogLimit).
		Scan(&export.Logs).Error; err != nil {
		return nil, err
	}
	if err := DB.Where("user_id = ?", userID).Order("id asc").Find(&export.TopUps).Error; err != nil {
		return nil, err
	}
	if err := DB.Where("user_id = ?", userID).Order("id asc").Find(&export.OAuthBindings).Error; err != nil {
		return nil, err
	}
	if err := DB.Where("user_id = ?", userID).Order("id asc").Find(&export.LegalConsents).Error; err != nil {
		return nil, err
	}
	return export, nil
}
