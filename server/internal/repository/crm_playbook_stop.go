package repository

import (
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"slices"
)

// Stop conditions suspend execution; they never fabricate customer outcomes.
// Achieved/declined outcomes use the canonical human lifecycle command.
func playbookStopReason(tx *gorm.DB, source model.CRMPlaybookExecutionSource) (string, error) {
	work, policy := source.Item.Situation, source.Policy.Definition
	if work.Lifecycle != "open" {
		return "", nil
	}
	if slices.Contains(policy.Policy.StopConditions, "no_longer_eligible") {
		q, err := matchingPlaybookSignals(tx, work.WorkspaceID, policy)
		if err != nil {
			return "", err
		}
		if work.OriginKind == "deal_won" {
			q = q.Where(wonDealHandoffEligibilitySQL)
		}
		var count int64
		if err := q.Where("s.id = ?", work.ID).Select("COUNT(*)").Scan(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return "no_longer_eligible", nil
		}
	}
	if work.ContactID != nil && slices.Contains(policy.Policy.StopConditions, "contact_restricted") {
		var count int64
		if err := tx.Table("crm_contacts").Where("workspace_id = ? AND id = ? AND email_status = 'invalid'", work.WorkspaceID, *work.ContactID).Count(&count).Error; err != nil {
			return "", err
		}
		if count > 0 {
			return "contact_restricted", nil
		}
	}
	return "", nil
}
