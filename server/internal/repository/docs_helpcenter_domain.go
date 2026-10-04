package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrHelpcenterDomainTaken means another workspace serves its help center on
// the domain.
var ErrHelpcenterDomainTaken = errors.New("this domain is already used by another help center")

// liveHelpcenterDomainStatuses are the states in which a custom domain serves.
var liveHelpcenterDomainStatuses = []string{model.HelpcenterDomainVerified, model.HelpcenterDomainFailing}

// HelpcenterDomainCheck is the result of checking a custom domain's DNS.
type HelpcenterDomainCheck struct {
	Status       string
	LastError    *string
	CheckedAt    time.Time
	VerifiedAt   *time.Time
	FailingSince *time.Time
}

// ReleaseHelpcenterDomainClaims clears the domain from other workspaces'
// help centers that never verified it, so an unproven claim cannot block the
// owner. It returns ErrHelpcenterDomainTaken when another workspace serves it.
func (r *DocsHelpcenterRepository) ReleaseHelpcenterDomainClaims(ctx context.Context, workspaceID, domain string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var live int64
		if err := tx.Model(&model.DocsHelpcenterConfig{}).
			Where("custom_domain = ? AND workspace_id <> ? AND custom_domain_status IN ?", domain, workspaceID, liveHelpcenterDomainStatuses).
			Count(&live).Error; err != nil {
			return err
		}
		if live > 0 {
			return ErrHelpcenterDomainTaken
		}
		return tx.Model(&model.DocsHelpcenterConfig{}).
			Where("custom_domain = ? AND workspace_id <> ?", domain, workspaceID).
			Updates(map[string]any{
				"custom_domain": nil, "custom_domain_status": nil, "custom_domain_verified_at": nil,
				"custom_domain_checked_at": nil, "custom_domain_last_error": nil, "custom_domain_failing_since": nil,
				"custom_domain_alerted_status": nil, "public_url_mode": model.HelpcenterPublicURLModeHostedSubdomain,
			}).Error
	})
}

// RecordHelpcenterDomainCheck stores a check for the workspace's domain. It
// applies only while the domain is unchanged, so a stale check cannot
// overwrite a newly entered domain.
func (r *DocsHelpcenterRepository) RecordHelpcenterDomainCheck(ctx context.Context, workspaceID, domain string, check HelpcenterDomainCheck) error {
	return r.db.WithContext(ctx).Model(&model.DocsHelpcenterConfig{}).
		Where("workspace_id = ? AND custom_domain = ?", workspaceID, domain).
		Updates(map[string]any{
			"custom_domain_status": check.Status, "custom_domain_last_error": check.LastError,
			"custom_domain_checked_at": check.CheckedAt, "custom_domain_verified_at": check.VerifiedAt,
			"custom_domain_failing_since": check.FailingSince,
		}).Error
}

// MarkHelpcenterDomainAlerted records the domain state admins were told about.
func (r *DocsHelpcenterRepository) MarkHelpcenterDomainAlerted(ctx context.Context, workspaceID, domain, status string) error {
	return r.db.WithContext(ctx).Model(&model.DocsHelpcenterConfig{}).
		Where("workspace_id = ? AND custom_domain = ?", workspaceID, domain).
		Update("custom_domain_alerted_status", status).Error
}

// ClaimHelpcenterDomainChecks returns up to limit custom domains due for a
// check and stamps them as checked, so concurrent replicas skip them. Pending
// domains are due after pendingEvery, live ones after liveEvery.
func (r *DocsHelpcenterRepository) ClaimHelpcenterDomainChecks(ctx context.Context, now time.Time, pendingEvery, liveEvery time.Duration, limit int) ([]model.DocsHelpcenterConfig, error) {
	var claimed []model.DocsHelpcenterConfig
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		due := tx.Model(&model.DocsHelpcenterConfig{}).Select("id").
			Where("custom_domain IS NOT NULL AND custom_domain <> ''").
			Where(`(custom_domain_status = ? AND (custom_domain_checked_at IS NULL OR custom_domain_checked_at < ?))
				OR (custom_domain_status IN ? AND (custom_domain_checked_at IS NULL OR custom_domain_checked_at < ?))`,
				model.HelpcenterDomainPending, now.Add(-pendingEvery), liveHelpcenterDomainStatuses, now.Add(-liveEvery)).
			Order("custom_domain_checked_at ASC NULLS FIRST").Limit(limit)
		if tx.Dialector.Name() == "postgres" {
			due = due.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var ids []string
		if err := due.Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Model(&model.DocsHelpcenterConfig{}).Where("id IN ?", ids).Update("custom_domain_checked_at", now).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Find(&claimed).Error
	})
	return claimed, err
}
