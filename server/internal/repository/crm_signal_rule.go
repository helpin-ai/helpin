package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListActiveSignalRuleConfigs returns the newest enabled global version of each rule.
func (r *CRMSignalRepository) ListActiveSignalRuleConfigs(ctx context.Context, cadence string) ([]model.CRMSignalRuleConfig, error) {
	var rows []model.CRMSignalRuleConfig
	if err := r.db.WithContext(ctx).
		Where("workspace_id IS NULL AND cadence = ? AND enabled = ?", cadence, true).
		Order("rule_key ASC, version DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list active signal rules: %w", err)
	}
	latest := make([]model.CRMSignalRuleConfig, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.RuleKey]; ok {
			continue
		}
		seen[row.RuleKey] = struct{}{}
		latest = append(latest, row)
	}
	return latest, nil
}

// TryAcquireSignalEvaluatorLease serializes one global cadence across API replicas.
func (r *CRMSignalRepository) TryAcquireSignalEvaluatorLease(
	ctx context.Context,
	cadence, owner string,
	now time.Time,
	leaseFor time.Duration,
	initialWatermark time.Time,
) (time.Time, bool, error) {
	if cadence == "" || owner == "" || leaseFor <= 0 {
		return time.Time{}, false, fmt.Errorf("cadence, owner, and positive lease are required")
	}
	row := model.CRMSignalEvaluatorWatermark{Cadence: cadence, Watermark: initialWatermark.UTC()}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return time.Time{}, false, fmt.Errorf("initialize signal evaluator watermark: %w", err)
	}
	leaseUntil := now.UTC().Add(leaseFor)
	result := r.db.WithContext(ctx).Model(&model.CRMSignalEvaluatorWatermark{}).
		Where("cadence = ? AND (lease_until IS NULL OR lease_until < ? OR lease_owner = ?)", cadence, now.UTC(), owner).
		Updates(map[string]interface{}{
			"lease_owner": owner, "lease_until": leaseUntil, "last_started_at": now.UTC(), "updated_at": now.UTC(),
		})
	if result.Error != nil {
		return time.Time{}, false, fmt.Errorf("acquire signal evaluator lease: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return time.Time{}, false, nil
	}
	if err := r.db.WithContext(ctx).Where("cadence = ?", cadence).First(&row).Error; err != nil {
		return time.Time{}, false, fmt.Errorf("read signal evaluator watermark: %w", err)
	}
	return row.Watermark.UTC(), true, nil
}

// ReleaseSignalEvaluatorLease advances the watermark only for the current owner.
func (r *CRMSignalRepository) ReleaseSignalEvaluatorLease(ctx context.Context, cadence, owner string, watermark time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.CRMSignalEvaluatorWatermark{}).
		Where("cadence = ? AND lease_owner = ?", cadence, owner).
		Updates(map[string]interface{}{
			"watermark": watermark.UTC(), "lease_owner": nil, "lease_until": nil, "updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("release signal evaluator lease: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("signal evaluator lease was lost")
	}
	return nil
}

// AbandonSignalEvaluatorLease releases a failed sweep without advancing its watermark.
func (r *CRMSignalRepository) AbandonSignalEvaluatorLease(ctx context.Context, cadence, owner string) error {
	return r.db.WithContext(ctx).Model(&model.CRMSignalEvaluatorWatermark{}).
		Where("cadence = ? AND lease_owner = ?", cadence, owner).
		Updates(map[string]interface{}{"lease_owner": nil, "lease_until": nil, "updated_at": time.Now().UTC()}).Error
}

// CreateSignalEvaluationRun starts one rule-level shadow measurement.
func (r *CRMSignalRepository) CreateSignalEvaluationRun(ctx context.Context, run *model.CRMSignalEvaluationRun) error {
	if run != nil && run.ID == "" {
		run.ID = uuid.NewString()
	}
	if err := r.db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("create signal evaluation run: %w", err)
	}
	return nil
}

// FinishSignalEvaluationRun records rule coverage and errors.
func (r *CRMSignalRepository) FinishSignalEvaluationRun(
	ctx context.Context,
	runID, status string,
	candidates, inserted int,
	errMessage *string,
	completedAt time.Time,
) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMSignalEvaluationRun{}).Where("id = ?", runID).Updates(map[string]interface{}{
		"status": status, "candidate_count": candidates, "inserted_count": inserted,
		"error_message": errMessage, "completed_at": completedAt.UTC(),
	}).Error; err != nil {
		return fmt.Errorf("finish signal evaluation run: %w", err)
	}
	return nil
}

// CreateRuleSignalIfAbsent applies the durable evaluator idempotency key.
func (r *CRMSignalRepository) CreateRuleSignalIfAbsent(ctx context.Context, signal *model.CRMBuyerSignal) (bool, error) {
	if signal == nil || signal.RuleKey == nil || signal.RuleVersion == nil || signal.WindowStartedAt == nil || signal.WindowEndedAt == nil {
		return false, fmt.Errorf("complete rule identity and evidence window are required")
	}
	ensureSignalDimensions(signal)
	ensureSignalEvidenceFingerprint(signal)
	if signal.ID == "" {
		signal.ID = uuid.NewString()
	}
	query := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).
		Where("workspace_id = ? AND rule_key = ? AND rule_version = ?", signal.WorkspaceID, *signal.RuleKey, *signal.RuleVersion).
		Where("window_started_at = ? AND window_ended_at = ? AND evidence_fingerprint = ?", signal.WindowStartedAt.UTC(), signal.WindowEndedAt.UTC(), signal.EvidenceFingerprint)
	query = nullableSignalID(query, "contact_id", signal.ContactID)
	query = nullableSignalID(query, "deal_id", signal.DealID)
	query = nullableSignalID(query, "company_id", signal.CompanyID)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("lookup rule signal: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	if err := r.db.WithContext(ctx).Create(signal).Error; err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, fmt.Errorf("create rule signal: %w", err)
	}
	return true, nil
}

func nullableSignalID(query *gorm.DB, column string, value *string) *gorm.DB {
	if value == nil || strings.TrimSpace(*value) == "" {
		return query.Where(column + " IS NULL")
	}
	return query.Where(column+" = ?", *value)
}

// ResolveBehavioralIdentity maps an event identity to internal CRM entities.
func (r *CRMSignalRepository) ResolveBehavioralIdentity(
	ctx context.Context,
	workspaceID, anonymousID, externalUserID, companyExternalID string,
) (*string, *string, string, string, error) {
	var link model.CRMIdentityLink
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND anonymous_id = ?", workspaceID, anonymousID).
		Order("CASE WHEN identity_trust = 'verified' THEN 0 ELSE 1 END, created_at DESC").First(&link).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, "", "", fmt.Errorf("resolve behavioral identity link: %w", err)
	}
	var contactID, companyID *string
	method, trust := model.IdentityMethodAnonymous, model.IdentityTrustUntrusted
	if err == nil {
		contactID, companyID, method, trust = link.ContactID, link.CompanyID, link.IdentityMethod, link.IdentityTrust
	}
	if contactID == nil && strings.TrimSpace(externalUserID) != "" {
		var contact model.CRMContact
		if findErr := r.db.WithContext(ctx).Select("id").Where("workspace_id = ? AND id = ?", workspaceID, externalUserID).First(&contact).Error; findErr == nil {
			contactID = &contact.ID
		}
	}
	if companyID == nil && strings.TrimSpace(companyExternalID) != "" {
		var company model.CRMCompany
		if findErr := r.db.WithContext(ctx).Select("id").Where("workspace_id = ? AND (id = ? OR external_id = ?)", workspaceID, companyExternalID, companyExternalID).First(&company).Error; findErr == nil {
			companyID = &company.ID
		}
	}
	return contactID, companyID, method, trust, nil
}

// ResolveOpenDealForIdentity attaches behavioral evidence to one active deal when unambiguous.
func (r *CRMSignalRepository) ResolveOpenDealForIdentity(ctx context.Context, workspaceID string, contactID, companyID *string) (*string, error) {
	if contactID == nil && companyID == nil {
		return nil, nil
	}
	query := r.db.WithContext(ctx).Table("crm_deals d").
		Select("d.id").Joins("JOIN crm_pipeline_stages ps ON ps.id = d.stage_id AND ps.stage_type = ?", model.CRMStageTypeOpen).
		Where("d.workspace_id = ?", workspaceID)
	var clauses []string
	var args []interface{}
	if contactID != nil {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM crm_associations a WHERE a.workspace_id=d.workspace_id AND
			((a.from_object_type='deal' AND a.from_object_id=d.id AND a.to_object_type='contact' AND a.to_object_id=?) OR
			 (a.to_object_type='deal' AND a.to_object_id=d.id AND a.from_object_type='contact' AND a.from_object_id=?)))`)
		args = append(args, *contactID, *contactID)
	}
	if companyID != nil {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM crm_associations a WHERE a.workspace_id=d.workspace_id AND
			((a.from_object_type='deal' AND a.from_object_id=d.id AND a.to_object_type='company' AND a.to_object_id=?) OR
			 (a.to_object_type='deal' AND a.to_object_id=d.id AND a.from_object_type='company' AND a.from_object_id=?)))`)
		args = append(args, *companyID, *companyID)
	}
	query = query.Where("("+strings.Join(clauses, " OR ")+")", args...)
	var ids []string
	if err := query.Order("d.updated_at DESC").Limit(2).Pluck("d.id", &ids).Error; err != nil {
		return nil, fmt.Errorf("resolve active deal for event identity: %w", err)
	}
	if len(ids) != 1 {
		return nil, nil
	}
	return &ids[0], nil
}
