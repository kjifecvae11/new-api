package model

import (
	"crypto/sha256"
	"encoding/hex"

	"gorm.io/gorm"
)

// UserLegalConsent records the exact server-side legal documents accepted at
// registration. It contains no client-supplied document version or IP address;
// hashes are calculated from the documents active in the server process.
type UserLegalConsent struct {
	ID                  uint   `json:"id"`
	UserID              int    `json:"user_id" gorm:"index;uniqueIndex:idx_user_legal_consent_source"`
	Source              string `json:"source" gorm:"type:varchar(32);uniqueIndex:idx_user_legal_consent_source"`
	TermsVersionSHA256  string `json:"terms_version_sha256" gorm:"type:char(64);not null"`
	UserAgreementSHA256 string `json:"user_agreement_sha256" gorm:"type:char(64);not null"`
	PrivacyPolicySHA256 string `json:"privacy_policy_sha256" gorm:"type:char(64);not null"`
	AcceptedAt          int64  `json:"accepted_at" gorm:"not null;index"`
}

func legalDocumentSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func RecordUserLegalConsentWithTx(tx *gorm.DB, userID int, source, agreement, privacy string, acceptedAt int64) error {
	record := UserLegalConsent{
		UserID:              userID,
		Source:              source,
		TermsVersionSHA256:  legalDocumentSHA256(agreement + "\x00" + privacy),
		UserAgreementSHA256: legalDocumentSHA256(agreement),
		PrivacyPolicySHA256: legalDocumentSHA256(privacy),
		AcceptedAt:          acceptedAt,
	}
	return tx.Create(&record).Error
}
