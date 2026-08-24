package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (r *CRMSignalRepository) GetSignal(ctx context.Context, workspaceID, signalID string) (*model.CRMBuyerSignal, error) {
	var signal model.CRMBuyerSignal
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, signalID).First(&signal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get buyer signal: %w", err)
	}
	signals := []model.CRMBuyerSignal{signal}
	if err := r.hydrateSignalContext(ctx, workspaceID, signals); err != nil {
		return nil, err
	}
	return &signals[0], nil
}

func (r *CRMSignalRepository) RecordSignalFeedback(ctx context.Context, signal *model.CRMBuyerSignal, memberID, action string, reason *string, occurredAt time.Time) error {
	elapsedMillis := occurredAt.Sub(signal.DetectedAt).Milliseconds()
	if elapsedMillis < 0 {
		elapsedMillis = 0
	}
	feedback := model.CRMSignalFeedback{
		WorkspaceID: signal.WorkspaceID, SignalID: signal.ID, MemberID: memberID,
		Action: action, DismissalReason: reason, RuleKey: signal.RuleKey, RuleVersion: signal.RuleVersion,
		SignalDomain: signal.SignalDomain, IdentityMethod: signal.EvidenceIdentityMethod,
		DetectedAt: signal.DetectedAt, OccurredAt: occurredAt,
		DetectionToEventMillis: elapsedMillis,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&feedback).Error; err != nil {
			return fmt.Errorf("record signal feedback: %w", err)
		}
		updates := map[string]interface{}{"reviewed_at": occurredAt}
		switch action {
		case model.CRMSignalFeedbackDismissed:
			updates["dismissed_at"] = occurredAt
			updates["dismissed_by_member_id"] = memberID
			updates["dismissal_reason"] = reason
		case model.CRMSignalFeedbackActed:
			updates["acted_at"] = occurredAt
		}
		result := tx.Model(&model.CRMBuyerSignal{}).
			Where("workspace_id = ? AND id = ?", signal.WorkspaceID, signal.ID).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("update signal feedback state: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("buyer signal not found")
		}
		return nil
	})
}

func (r *CRMSignalRepository) ListSignalPrecision(ctx context.Context, workspaceID string) ([]model.CRMSignalPrecisionRow, error) {
	var rows []model.CRMSignalPrecisionRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT workspace_id,
		       COALESCE(rule_key, 'llm_extracted') AS rule_key,
		       COALESCE(rule_version, 0) AS rule_version,
		       signal_domain,
		       identity_method,
		       COUNT(*) AS reviewed_count,
		       SUM(CASE WHEN action = 'acted' OR dismissal_reason IN ('handled', 'bad_timing', 'irrelevant') THEN 1 ELSE 0 END) AS valid_count,
		       SUM(CASE WHEN dismissal_reason IN ('incorrect_evidence', 'wrong_entity', 'duplicate') THEN 1 ELSE 0 END) AS incorrect_count,
		       SUM(CASE WHEN action = 'acted' THEN 1 ELSE 0 END) AS acted_count,
		       CASE WHEN COUNT(*) = 0 THEN 0 ELSE
		         1.0 * SUM(CASE WHEN action = 'acted' OR dismissal_reason IN ('handled', 'bad_timing', 'irrelevant') THEN 1 ELSE 0 END) / COUNT(*)
		       END AS precision,
		       CAST(AVG(CASE WHEN action IN ('reviewed', 'dismissed') THEN detection_to_event_millis END) AS BIGINT) AS average_review_millis,
		       CAST(AVG(CASE WHEN action = 'acted' THEN detection_to_event_millis END) AS BIGINT) AS average_action_millis
		FROM crm_signal_feedback
		WHERE workspace_id = ?
		GROUP BY workspace_id, COALESCE(rule_key, 'llm_extracted'), COALESCE(rule_version, 0), signal_domain, identity_method
		ORDER BY reviewed_count DESC, rule_key, rule_version`, workspaceID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("report signal precision: %w", err)
	}
	return rows, nil
}

func (r *CRMSignalRepository) GetActiveRoutingPolicy(ctx context.Context, workspaceID string) (*model.CRMSignalRoutingPolicy, error) {
	var policy model.CRMSignalRoutingPolicy
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND enabled = ?", workspaceID, true).Order("version DESC").First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get signal routing policy: %w", err)
	}
	return &policy, nil
}

func (r *CRMSignalRepository) CreateRoutingPolicy(ctx context.Context, policy *model.CRMSignalRoutingPolicy) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var latest int
		if err := tx.Model(&model.CRMSignalRoutingPolicy{}).Where("workspace_id = ?", policy.WorkspaceID).
			Select("COALESCE(MAX(version), 0)").Scan(&latest).Error; err != nil {
			return fmt.Errorf("load routing policy version: %w", err)
		}
		policy.Version = latest + 1
		if err := tx.Model(&model.CRMSignalRoutingPolicy{}).Where("workspace_id = ? AND enabled = ?", policy.WorkspaceID, true).Update("enabled", false).Error; err != nil {
			return fmt.Errorf("deactivate routing policy: %w", err)
		}
		if err := tx.Create(policy).Error; err != nil {
			return fmt.Errorf("create routing policy: %w", err)
		}
		return nil
	})
}

func (r *CRMSignalRepository) ActivateRoutingPolicyVersion(ctx context.Context, workspaceID string, version int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.CRMSignalRoutingPolicy{}).Where("workspace_id = ? AND version = ?", workspaceID, version).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("routing policy version not found")
		}
		if err := tx.Model(&model.CRMSignalRoutingPolicy{}).Where("workspace_id = ?", workspaceID).Update("enabled", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.CRMSignalRoutingPolicy{}).Where("workspace_id = ? AND version = ?", workspaceID, version).Update("enabled", true).Error
	})
}

func (r *CRMSignalRepository) ActivateRuleVersion(ctx context.Context, workspaceID, ruleKey string, version int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		scope := tx.Model(&model.CRMSignalRuleConfig{}).Where("rule_key = ? AND (workspace_id = ? OR (? = '' AND workspace_id IS NULL))", ruleKey, workspaceID, workspaceID)
		var count int64
		if err := scope.Where("version = ?", version).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("rule version not found")
		}
		if err := scope.Update("enabled", false).Error; err != nil {
			return err
		}
		result := tx.Model(&model.CRMSignalRuleConfig{}).
			Where("rule_key = ? AND version = ? AND (workspace_id = ? OR (? = '' AND workspace_id IS NULL))", ruleKey, version, workspaceID, workspaceID).
			Update("enabled", true)
		return result.Error
	})
}

// FindOpenTaskForSignal checks both canonical CRM associations and the stable signal fingerprint.
func (r *CRMSignalRepository) FindOpenTaskForSignal(ctx context.Context, signal model.CRMBuyerSignal) (*string, error) {
	if !r.db.Migrator().HasTable(&model.PMTask{}) {
		return nil, nil
	}
	query := r.db.WithContext(ctx).Model(&model.PMTask{}).
		Select("pm_tasks.id").Where("pm_tasks.workspace_id = ? AND pm_tasks.completed = ? AND pm_tasks.archived = ?", signal.WorkspaceID, false, false)
	conditions := []string{}
	args := []interface{}{}
	if strings.TrimSpace(signal.EvidenceFingerprint) != "" {
		conditions = append(conditions, "pm_tasks.external_id = ?")
		args = append(args, "crm-signal:"+signal.EvidenceFingerprint)
	}
	entities := []struct {
		kind string
		id   *string
	}{
		{model.CRMObjectDeal, signal.DealID}, {model.CRMObjectCompany, signal.CompanyID}, {model.CRMObjectContact, signal.ContactID},
	}
	if r.db.Migrator().HasTable(&model.CRMAssociation{}) {
		for _, entity := range entities {
			if entity.id == nil || strings.TrimSpace(*entity.id) == "" {
				continue
			}
			conditions = append(conditions, `EXISTS (
				SELECT 1 FROM crm_associations a
				WHERE a.workspace_id = pm_tasks.workspace_id
				AND ((a.from_object_type = 'task' AND a.from_object_id = pm_tasks.id AND a.to_object_type = ? AND a.to_object_id = ?)
				  OR (a.to_object_type = 'task' AND a.to_object_id = pm_tasks.id AND a.from_object_type = ? AND a.from_object_id = ?))
			)`)
			args = append(args, entity.kind, *entity.id, entity.kind, *entity.id)
		}
	}
	if len(conditions) == 0 {
		return nil, nil
	}
	var id string
	err := query.Where("("+strings.Join(conditions, " OR ")+")", args...).Order("pm_tasks.created_at DESC").Limit(1).Scan(&id).Error
	if err != nil {
		return nil, fmt.Errorf("check existing signal task: %w", err)
	}
	if id == "" {
		return nil, nil
	}
	return &id, nil
}

func (r *CRMSignalRepository) CreateSignalDelivery(ctx context.Context, delivery *model.CRMSignalDelivery) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMSignalDelivery{}).
		Where("signal_id = ? AND policy_id = ? AND channel = ? AND COALESCE(recipient_member_id, '') = COALESCE(?, '') AND COALESCE(destination_team_id, '') = COALESCE(?, '')",
			delivery.SignalID, delivery.PolicyID, delivery.Channel, delivery.RecipientMemberID, delivery.DestinationTeamID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check signal delivery: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(delivery)
	if result.Error != nil {
		return false, fmt.Errorf("create signal delivery: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (r *CRMSignalRepository) HasSignalDelivery(ctx context.Context, signalID, policyID string) (bool, error) {
	if !r.db.Migrator().HasTable(&model.CRMSignalDelivery{}) {
		return false, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMSignalDelivery{}).
		Where("signal_id = ? AND policy_id = ?", signalID, policyID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check prior signal delivery: %w", err)
	}
	return count > 0, nil
}

func (r *CRMSignalRepository) RoutingRecipientUserIDs(ctx context.Context, workspaceID string, ownerMemberID, teamID *string) ([]string, error) {
	query := r.db.WithContext(ctx).Model(&model.WorkspaceMember{}).
		Distinct("workspace_members.user_id").Where("workspace_members.workspace_id = ? AND workspace_members.status = ? AND workspace_members.user_id IS NOT NULL", workspaceID, model.WorkspaceMemberStatusActive)
	conditions := []string{}
	args := []interface{}{}
	if ownerMemberID != nil && strings.TrimSpace(*ownerMemberID) != "" {
		conditions = append(conditions, "workspace_members.id = ?")
		args = append(args, *ownerMemberID)
	}
	if teamID != nil && strings.TrimSpace(*teamID) != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM team_workspace_memberships twm WHERE twm.workspace_member_id = workspace_members.id AND twm.team_id = ?)")
		args = append(args, *teamID)
	}
	if len(conditions) == 0 {
		return []string{}, nil
	}
	query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
	var ids []string
	if err := query.Pluck("workspace_members.user_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("resolve signal routing recipients: %w", err)
	}
	return ids, nil
}

func (r *CRMSignalRepository) SignalFiltersForMeeting(ctx context.Context, workspaceID, meetingID string) (model.CRMBuyerSignalListFilters, error) {
	if !r.db.Migrator().HasTable(&model.CRMAssociation{}) {
		return model.CRMBuyerSignalListFilters{}, fmt.Errorf("meeting associations are unavailable")
	}
	var associations []model.CRMAssociation
	if err := r.db.WithContext(ctx).Where(
		"workspace_id = ? AND ((from_object_type = ? AND from_object_id = ?) OR (to_object_type = ? AND to_object_id = ?))",
		workspaceID, model.CRMObjectMeeting, meetingID, model.CRMObjectMeeting, meetingID,
	).Find(&associations).Error; err != nil {
		return model.CRMBuyerSignalListFilters{}, fmt.Errorf("load meeting signal associations: %w", err)
	}
	filters := model.CRMBuyerSignalListFilters{}
	for _, association := range associations {
		kind, id := association.ToObjectType, association.ToObjectID
		if kind == model.CRMObjectMeeting {
			kind, id = association.FromObjectType, association.FromObjectID
		}
		switch kind {
		case model.CRMObjectDeal:
			filters.DealID = &id
		case model.CRMObjectCompany:
			filters.CompanyID = &id
		case model.CRMObjectContact:
			filters.ContactID = &id
		}
	}
	if filters.DealID == nil && filters.CompanyID == nil && filters.ContactID == nil {
		return filters, fmt.Errorf("meeting has no CRM entity associations")
	}
	// Prefer the most specific commercial context instead of intersecting
	// several associations that may not be attached to every signal row.
	if filters.DealID != nil {
		filters.CompanyID, filters.ContactID = nil, nil
	} else if filters.CompanyID != nil {
		filters.ContactID = nil
	}
	return filters, nil
}
