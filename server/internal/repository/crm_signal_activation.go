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
		WITH ranked AS (
			SELECT feedback.*,
			       ROW_NUMBER() OVER (PARTITION BY signal_id ORDER BY occurred_at DESC, created_at DESC, id DESC) AS feedback_rank
			FROM crm_signal_feedback feedback
			WHERE workspace_id = ?
		), latest AS (
			SELECT * FROM ranked WHERE feedback_rank = 1
		), first_reviews AS (
			SELECT signal_id, MIN(detection_to_event_millis) AS review_millis
			FROM crm_signal_feedback
			WHERE workspace_id = ?
			GROUP BY signal_id
		)
		SELECT latest.workspace_id,
		       COALESCE(latest.rule_key, 'llm_extracted') AS rule_key,
		       COALESCE(latest.rule_version, 0) AS rule_version,
		       latest.signal_domain,
		       latest.identity_method,
		       COUNT(*) AS reviewed_count,
		       SUM(CASE WHEN latest.action = 'acted' OR latest.dismissal_reason IN ('handled', 'bad_timing', 'irrelevant') THEN 1 ELSE 0 END) AS valid_count,
		       SUM(CASE WHEN latest.dismissal_reason IN ('incorrect_evidence', 'wrong_entity', 'duplicate') THEN 1 ELSE 0 END) AS incorrect_count,
		       SUM(CASE WHEN latest.action = 'acted' THEN 1 ELSE 0 END) AS acted_count,
		       CASE WHEN SUM(CASE WHEN latest.action IN ('acted', 'dismissed') THEN 1 ELSE 0 END) = 0 THEN 0 ELSE
		         1.0 * SUM(CASE WHEN latest.action = 'acted' OR latest.dismissal_reason IN ('handled', 'bad_timing', 'irrelevant') THEN 1 ELSE 0 END)
		         / SUM(CASE WHEN latest.action IN ('acted', 'dismissed') THEN 1 ELSE 0 END)
		       END AS precision,
		       CAST(AVG(first_reviews.review_millis) AS BIGINT) AS average_review_millis,
		       CAST(AVG(CASE WHEN latest.action = 'acted' THEN latest.detection_to_event_millis END) AS BIGINT) AS average_action_millis
		FROM latest
		JOIN first_reviews ON first_reviews.signal_id = latest.signal_id
		GROUP BY latest.workspace_id, COALESCE(latest.rule_key, 'llm_extracted'), COALESCE(latest.rule_version, 0), latest.signal_domain, latest.identity_method
		ORDER BY reviewed_count DESC, rule_key, rule_version`, workspaceID, workspaceID).Scan(&rows).Error
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
		var source model.CRMSignalRuleConfig
		err := tx.Where("rule_key = ? AND version = ? AND (workspace_id = ? OR workspace_id IS NULL)", ruleKey, version, workspaceID).
			Order("CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END").First(&source).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("rule version not found")
		}
		if err != nil {
			return err
		}
		if !source.ActivationEligible {
			return fmt.Errorf("rule version is context-only and cannot be activated")
		}
		if err := tx.Model(&model.CRMSignalRuleConfig{}).
			Where("workspace_id = ? AND rule_key = ?", workspaceID, ruleKey).
			Update("enabled", false).Error; err != nil {
			return err
		}
		if source.WorkspaceID != nil {
			return tx.Model(&model.CRMSignalRuleConfig{}).Where("id = ?", source.ID).
				Updates(map[string]interface{}{"enabled": true, "shadow_mode": false}).Error
		}
		workspaceConfig := source
		workspaceConfig.ID = uuid.NewString()
		workspaceConfig.WorkspaceID = &workspaceID
		workspaceConfig.Enabled = true
		workspaceConfig.ShadowMode = false
		workspaceConfig.CreatedAt = time.Time{}
		workspaceConfig.UpdatedAt = time.Time{}
		if err := tx.Create(&workspaceConfig).Error; err != nil {
			return err
		}
		// GORM's default:true tag replaces a false zero value during Create, so
		// explicitly persist the promotion out of shadow mode.
		return tx.Model(&model.CRMSignalRuleConfig{}).Where("id = ?", workspaceConfig.ID).
			Update("shadow_mode", false).Error
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
	if delivery.ID == "" {
		delivery.ID = uuid.NewString()
	}
	if delivery.Status == "" {
		delivery.Status = model.CRMSignalDeliveryPending
	}
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

// ListRetryableSignalDeliveries returns queued, failed, and abandoned sends.
func (r *CRMSignalRepository) ListRetryableSignalDeliveries(
	ctx context.Context,
	workspaceID string,
	staleBefore time.Time,
	limit int,
) ([]model.CRMSignalDelivery, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	var deliveries []model.CRMSignalDelivery
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND channel <> ?", workspaceID, model.CRMSignalDeliveryFeed).
		Where("status IN ? OR (status = ? AND (last_attempted_at IS NULL OR last_attempted_at <= ?))",
			[]string{model.CRMSignalDeliveryPending, model.CRMSignalDeliveryFailed},
			model.CRMSignalDeliverySending, staleBefore.UTC()).
		Order("created_at ASC, id ASC").Limit(limit).Find(&deliveries).Error
	if err != nil {
		return nil, fmt.Errorf("list retryable signal deliveries: %w", err)
	}
	return deliveries, nil
}

// ClaimSignalDelivery atomically leases one queued delivery for emission.
func (r *CRMSignalRepository) ClaimSignalDelivery(
	ctx context.Context,
	deliveryID string,
	staleBefore, attemptedAt time.Time,
) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.CRMSignalDelivery{}).
		Where("id = ?", deliveryID).
		Where("status IN ? OR (status = ? AND (last_attempted_at IS NULL OR last_attempted_at <= ?))",
			[]string{model.CRMSignalDeliveryPending, model.CRMSignalDeliveryFailed},
			model.CRMSignalDeliverySending, staleBefore.UTC()).
		Updates(map[string]interface{}{
			"status":            model.CRMSignalDeliverySending,
			"attempts":          gorm.Expr("attempts + 1"),
			"last_attempted_at": attemptedAt.UTC(),
			"last_error":        nil,
		})
	if result.Error != nil {
		return false, fmt.Errorf("claim signal delivery: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// MarkSignalDeliverySent completes a claimed delivery after successful emission.
func (r *CRMSignalRepository) MarkSignalDeliverySent(
	ctx context.Context,
	deliveryID string,
	deliveredAt time.Time,
) error {
	result := r.db.WithContext(ctx).Model(&model.CRMSignalDelivery{}).
		Where("id = ? AND status = ?", deliveryID, model.CRMSignalDeliverySending).
		Updates(map[string]interface{}{
			"status": model.CRMSignalDeliverySent, "delivered_at": deliveredAt.UTC(), "last_error": nil,
		})
	if result.Error != nil {
		return fmt.Errorf("complete signal delivery: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("signal delivery claim was lost")
	}
	return nil
}

// MarkSignalDeliveryFailed releases a claimed delivery for a later retry.
func (r *CRMSignalRepository) MarkSignalDeliveryFailed(
	ctx context.Context,
	deliveryID, message string,
) error {
	result := r.db.WithContext(ctx).Model(&model.CRMSignalDelivery{}).
		Where("id = ? AND status = ?", deliveryID, model.CRMSignalDeliverySending).
		Updates(map[string]interface{}{
			"status": model.CRMSignalDeliveryFailed, "delivered_at": nil, "last_error": message,
		})
	if result.Error != nil {
		return fmt.Errorf("fail signal delivery: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("signal delivery claim was lost")
	}
	return nil
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
