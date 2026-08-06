package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var eraseDeleteTokenCache = cacheDeleteToken
var eraseInvalidateUserCache = invalidateUserCache

const userErasureFreshProofMaxAgeSeconds int64 = 300

// UserErasureAudit retains only pseudonymous, non-secret evidence that a
// destructive self-deletion was authorized by a recent step-up proof.
type UserErasureAudit struct {
	Id                   int    `json:"id"`
	PseudonymousUserRef  string `json:"pseudonymous_user_ref" gorm:"type:varchar(64);uniqueIndex"`
	ActorType            string `json:"actor_type" gorm:"type:varchar(16);not null"`
	AuthenticationMethod string `json:"authentication_method" gorm:"type:varchar(16);not null"`
	VerifiedAt           int64  `json:"verified_at" gorm:"bigint;not null"`
	ErasedAt             int64  `json:"erased_at" gorm:"bigint;not null"`
}

type userErasureEvidence struct {
	method     string
	verifiedAt int64
}

// EraseUserByID revokes credentials and removes or pseudonymizes personal data
// while retaining non-personal billing totals required for reconciliation.
// The log store can be separate from the account database, so cross-store
// atomicity is not implied. Every operation is idempotent and can be retried.
func EraseUserByID(id int) error {
	return eraseUserByID(id, nil)
}

// EraseUserByIDWithFreshProof is the only self-service erasure entry point.
// It fails closed unless the controller supplies a recent TOTP or Passkey
// proof established by SecureVerificationRequired for the same session user.
func EraseUserByIDWithFreshProof(id int, method string, verifiedAt int64) error {
	now := time.Now().Unix()
	if (method != "2fa" && method != "passkey") || verifiedAt <= 0 || verifiedAt > now || now-verifiedAt >= userErasureFreshProofMaxAgeSeconds {
		return errors.New("recent strong authentication proof is required for self deletion")
	}
	return eraseUserByID(id, &userErasureEvidence{method: method, verifiedAt: verifiedAt})
}

func eraseUserByID(id int, evidence *userErasureEvidence) error {
	if id == 0 {
		return errors.New("id 为空！")
	}

	var user User
	if err := DB.Unscoped().First(&user, "id = ?", id).Error; err != nil {
		return err
	}
	var tokenRows []Token
	if err := DB.Unscoped().Where("user_id = ?", id).Find(&tokenRows).Error; err != nil {
		return fmt.Errorf("collect token keys for revocation: %w", err)
	}
	tokenKeys := make([]string, 0, len(tokenRows))
	seenTokenKeys := make(map[string]struct{}, len(tokenRows))
	for _, token := range tokenRows {
		if token.Key == "" {
			continue
		}
		if _, seen := seenTokenKeys[token.Key]; seen {
			continue
		}
		seenTokenKeys[token.Key] = struct{}{}
		tokenKeys = append(tokenKeys, token.Key)
	}

	pseudonym := fmt.Sprintf("deleted-user-%d", id)
	if LOG_DB == nil {
		return errors.New("log database is not initialized")
	}
	if err := scrubErasedUserLogs(id, pseudonym); err != nil {
		return err
	}

	now := time.Now()
	err := DB.Transaction(func(tx *gorm.DB) error {
		deletions := []struct {
			model any
			field string
			value int
		}{
			{&Token{}, "user_id", id},
			{&PasskeyCredential{}, "user_id", id},
			{&TwoFABackupCode{}, "user_id", id},
			{&TwoFA{}, "user_id", id},
			{&UserOAuthBinding{}, "user_id", id},
			{&Checkin{}, "user_id", id},
			{&QuotaData{}, "user_id", id},
			{&Task{}, "user_id", id},
			{&Midjourney{}, "user_id", id},
		}
		for _, deletion := range deletions {
			if err := tx.Unscoped().Where(deletion.field+" = ?", deletion.value).Delete(deletion.model).Error; err != nil {
				return err
			}
		}
		if err := scrubErasedUserFinancialRecords(tx, id); err != nil {
			return err
		}
		if evidence != nil {
			audit := UserErasureAudit{PseudonymousUserRef: pseudonym}
			if err := tx.Where("pseudonymous_user_ref = ?", pseudonym).Assign(UserErasureAudit{
				ActorType:            "self",
				AuthenticationMethod: evidence.method,
				VerifiedAt:           evidence.verifiedAt,
				ErasedAt:             now.Unix(),
			}).FirstOrCreate(&audit).Error; err != nil {
				return fmt.Errorf("record self-erasure authorization evidence: %w", err)
			}
		}

		return tx.Unscoped().Model(&User{}).Where("id = ?", id).Updates(map[string]any{
			"username":        pseudonym,
			"password":        "!account-erased!",
			"display_name":    "Deleted User",
			"status":          2,
			"email":           "",
			"github_id":       "",
			"discord_id":      "",
			"oidc_id":         "",
			"wechat_id":       "",
			"telegram_id":     "",
			"linux_do_id":     "",
			"access_token":    nil,
			"quota":           0,
			"aff_code":        fmt.Sprintf("deleted-%d", id),
			"aff_quota":       0,
			"aff_count":       0,
			"aff_history":     0,
			"inviter_id":      0,
			"setting":         "",
			"remark":          "",
			"stripe_customer": "",
			"last_login_at":   0,
			"deleted_at":      gorm.DeletedAt{Time: now, Valid: true},
		}).Error
	})
	if err != nil {
		return fmt.Errorf("erase user data: %w", err)
	}
	// Repeat after credential revocation and account disablement to catch logs
	// written by requests that were already in flight during the first pass.
	if err := scrubErasedUserLogs(id, pseudonym); err != nil {
		return err
	}
	if err := revokeErasedUserCaches(id, tokenKeys); err != nil {
		// The database erasure is durable, but callers must not report successful
		// credential revocation until every known token cache and the user cache
		// have been synchronously removed.
		return fmt.Errorf("user data erased but credential cache revocation failed: %w", err)
	}
	return nil
}

// scrubErasedUserFinancialRecords inventories every user-linked financial
// model. Settlement totals and transaction references remain linked only to
// the pseudonymized User row for statutory reconciliation; raw provider
// payloads, live entitlements, idempotency handles, and redemption secrets do
// not survive erasure.
func scrubErasedUserFinancialRecords(tx *gorm.DB, id int) error {
	if err := tx.Model(&TopUp{}).
		Where("user_id = ? AND status = ?", id, common.TopUpStatusPending).
		Update("status", common.TopUpStatusFailed).Error; err != nil {
		return fmt.Errorf("close pending topups: %w", err)
	}
	if err := tx.Model(&SubscriptionOrder{}).
		Where("user_id = ?", id).
		Updates(map[string]any{
			"provider_payload": "",
			"status": gorm.Expr(
				"CASE WHEN status = ? THEN ? ELSE status END",
				common.TopUpStatusPending,
				common.TopUpStatusFailed,
			),
		}).Error; err != nil {
		return fmt.Errorf("scrub subscription orders: %w", err)
	}
	if err := tx.Model(&UserSubscription{}).
		Where("user_id = ?", id).
		Updates(map[string]any{
			"status": gorm.Expr(
				"CASE WHEN status = ? THEN ? ELSE status END",
				"active",
				"cancelled",
			),
			"upgrade_group":   "",
			"prev_user_group": "",
		}).Error; err != nil {
		return fmt.Errorf("cancel user subscriptions: %w", err)
	}
	if err := tx.Unscoped().Where("user_id = ?", id).Delete(&SubscriptionPreConsumeRecord{}).Error; err != nil {
		return fmt.Errorf("delete subscription idempotency records: %w", err)
	}

	var redemptions []Redemption
	if err := tx.Unscoped().Where("user_id = ? OR used_user_id = ?", id, id).Find(&redemptions).Error; err != nil {
		return fmt.Errorf("inventory linked redemption codes: %w", err)
	}
	for _, redemption := range redemptions {
		updates := map[string]any{
			"key":  fmt.Sprintf("erased-%025d", redemption.Id),
			"name": "Erased redemption",
		}
		if redemption.UserId == id && redemption.Status == common.RedemptionCodeStatusEnabled {
			updates["status"] = common.RedemptionCodeStatusDisabled
		}
		if err := tx.Unscoped().Model(&Redemption{}).Where("id = ?", redemption.Id).Updates(updates).Error; err != nil {
			return fmt.Errorf("scrub linked redemption code %d: %w", redemption.Id, err)
		}
	}
	return nil
}

func revokeErasedUserCaches(id int, tokenKeys []string) error {
	var revocationErrors []error
	for _, key := range tokenKeys {
		if err := eraseDeleteTokenCache(key); err != nil {
			revocationErrors = append(revocationErrors, fmt.Errorf("delete token cache: %w", err))
		}
	}
	if err := eraseInvalidateUserCache(id); err != nil {
		revocationErrors = append(revocationErrors, fmt.Errorf("delete user cache: %w", err))
	}
	return errors.Join(revocationErrors...)
}

func scrubErasedUserLogs(id int, pseudonym string) error {
	if err := LOG_DB.Model(&Log{}).Where("user_id = ?", id).Updates(map[string]any{
		"username":            pseudonym,
		"token_name":          "",
		"ip":                  "",
		"request_id":          "",
		"upstream_request_id": "",
		"request_content":     "",
		"response_content":    "",
		"content":             "account data erased",
		"other":               "{}",
	}).Error; err != nil {
		return fmt.Errorf("scrub user logs: %w", err)
	}
	return nil
}
