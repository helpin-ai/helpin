package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// ActionFacts reads the exact linked records and a contact-level communication watermark.
// It never reads mailbox tokens or email content from another member's mailbox.
func (r *CRMPlaybookExecutionRepository) ActionFacts(ctx context.Context, ws, situationID string) (*model.CRMPlaybookActionFacts, error) {
	var situation model.CRMSituation
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", ws, situationID).Take(&situation).Error; err != nil {
		return nil, err
	}
	return playbookActionFacts(r.db.WithContext(ctx), situation)
}

func playbookActionFacts(tx *gorm.DB, situation model.CRMSituation) (*model.CRMPlaybookActionFacts, error) {
	facts := &model.CRMPlaybookActionFacts{}
	if tx.Migrator().HasTable(&model.PMTask{}) && tx.Migrator().HasTable(&model.CRMPlaybookActionIntent{}) {
		if err := tx.Table("pm_tasks task").Select("task.id, task.team_id, task.workflow_state_id, task.completed_at, task.deadline").
			Joins("JOIN crm_playbook_action_intents intent ON intent.workspace_id = task.workspace_id AND intent.result_id = task.id AND intent.result_type = 'task'").
			Where("intent.workspace_id = ? AND intent.situation_id = ?", situation.WorkspaceID, situation.ID).Order("task.id").Scan(&facts.LinkedTasks).Error; err != nil {
			return nil, err
		}
	}
	if situation.CompanyID != nil {
		facts.Company = &model.CRMCompany{}
		if err := tx.Where("workspace_id = ? AND id = ?", situation.WorkspaceID, *situation.CompanyID).Take(facts.Company).Error; err != nil {
			return nil, err
		}
	}
	if situation.DealID != nil {
		facts.Deal = &model.CRMDeal{}
		if err := tx.Where("workspace_id = ? AND id = ?", situation.WorkspaceID, *situation.DealID).Take(facts.Deal).Error; err != nil {
			return nil, err
		}
	}
	if situation.ContactID != nil {
		facts.Contact = &model.CRMContact{}
		if err := tx.Where("workspace_id = ? AND id = ?", situation.WorkspaceID, *situation.ContactID).Take(facts.Contact).Error; err != nil {
			return nil, err
		}
		var latest model.CRMEmailMessage
		err := tx.Select("sent_at").Where("workspace_id = ?", situation.WorkspaceID).
			Where(`contact_id = ? OR EXISTS (SELECT 1 FROM crm_email_message_contacts c WHERE c.message_id = crm_email_messages.id AND c.contact_id = ?)`, *situation.ContactID, *situation.ContactID).
			Order("sent_at DESC").Take(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			facts.LastContactMessageAt = &latest.SentAt
		}
		var outbound model.CRMEmailMessage
		err = tx.Select("sent_at").Where("workspace_id = ? AND direction = 'outbound'", situation.WorkspaceID).
			Where(`contact_id = ? OR EXISTS (SELECT 1 FROM crm_email_message_contacts c WHERE c.message_id = crm_email_messages.id AND c.contact_id = ?)`, *situation.ContactID, *situation.ContactID).
			Order("sent_at DESC").Take(&outbound).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			facts.LastOutboundMessageAt = &outbound.SentAt
		}
	}
	return facts, nil
}

// ActionIntent reads only a tenant-scoped canonical action's authority receipt.
func (r *CRMPlaybookExecutionRepository) ActionIntent(ctx context.Context, ws, id string) (*model.CRMPlaybookActionIntent, error) {
	var result model.CRMPlaybookActionIntent
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND suggestion_id = ?", ws, id).Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &result, err
}

// ProposeAction serializes repeat interpretations into one canonical proposal per intent.
// The compiler is pure: it verifies policy, current facts and returns exact authority.
func (r *CRMPlaybookExecutionRepository) ProposeAction(ctx context.Context, ws, runID string, action model.CRMPlaybookAction, now time.Time,
	compile func(model.AutomationRunBinding, model.CRMPlaybookExecutionSource, model.CRMPlaybookActionFacts) (model.CRMPlaybookActionIntent, error),
) (*model.CRMSuggestion, error) {
	var id string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		binding, source, err := lockedPlaybookRunSource(tx, ws, runID)
		if err != nil {
			return err
		}
		var run model.AgentRun
		if err := tx.Where("workspace_id = ? AND id = ? AND status IN ?", ws, runID, []string{"queued", "running"}).Take(&run).Error; err != nil {
			return ErrCRMPlaybookExecutionBlocked
		}
		facts, err := playbookActionFacts(tx, source.Item.Situation)
		if err != nil {
			return err
		}
		intent, err := compile(*binding, *source, *facts)
		if err != nil {
			return err
		}
		var previous model.CRMPlaybookActionIntent
		err = tx.Where("workspace_id = ? AND intent_key = ?", ws, intent.IntentKey).Take(&previous).Error
		if err == nil {
			id = previous.SuggestionID
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		intent.SuggestionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("crm.playbook.action:"+ws+":"+intent.IntentKey)).String()
		intent.WorkspaceID, intent.SituationID, intent.RunID = ws, binding.SituationID, runID
		intent.ConnectionID, intent.Generation, intent.SituationRevision = binding.ConnectionID, binding.Generation, binding.SituationRevision
		intent.CreatedAt = now
		var member model.WorkspaceMember
		if err := tx.Where("workspace_id = ? AND id = ? AND status = 'active'", ws, intent.ApproverMemberID).Take(&member).Error; err != nil {
			return ErrCRMPlaybookExecutionBlocked
		}
		if member.UserID == nil {
			return ErrCRMPlaybookExecutionBlocked
		}
		objectType, objectID := "company", source.Item.Situation.CompanyID
		if source.Item.Situation.ContactID != nil {
			objectType, objectID = "contact", source.Item.Situation.ContactID
		}
		if source.Item.Situation.DealID != nil {
			objectType, objectID = "deal", source.Item.Situation.DealID
		}
		proposal := model.CRMSuggestion{ID: intent.SuggestionID, WorkspaceID: ws, UserID: member.UserID, SuggestionType: "playbook_action", ObjectType: &objectType, ObjectID: objectID,
			Title: action.Title, Description: &action.Reason, Context: model.JSONB{"playbook_action": action, "playbook_action_expires_at": intent.ExpiresAt, "situation_id": binding.SituationID},
			SignalIDs: model.StringArray{}, Status: model.CRMSuggestionStatusPending, ExecutionStatus: model.CRMSuggestionExecutionPending, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&proposal).Error; err != nil {
			return err
		}
		if err := tx.Create(&intent).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.CRMSituationReference{WorkspaceID: ws, SituationID: binding.SituationID, Kind: model.CRMSituationReferenceSuggestion, SourceID: proposal.ID, CreatedAt: now}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.CRMSituationSourceLink{WorkspaceID: ws, SituationID: binding.SituationID, Kind: model.CRMSituationReferenceSuggestion, SourceID: proposal.ID}).Error; err != nil {
			return err
		}
		id = proposal.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return NewCRMSuggestionRepository(r.db).GetByID(ctx, ws, id)
}

func lockedPlaybookRunSource(tx *gorm.DB, ws, runID string) (*model.AutomationRunBinding, *model.CRMPlaybookExecutionSource, error) {
	binding, err := NewCRMPlaybookExecutionRepository(tx).RunBinding(tx.Statement.Context, ws, runID)
	if err != nil {
		return nil, nil, err
	}
	if binding == nil || binding.Input.CRMPlaybook == nil {
		return nil, nil, ErrCRMPlaybookExecutionBlocked
	}
	source, err := playbookExecutionSource(tx, ws, binding.SituationID)
	if err != nil {
		return nil, nil, err
	}
	if !playbookExecutionAllowed(source) || source.Binding.Generation != binding.Generation || source.Binding.ConnectionID != binding.ConnectionID || source.Item.Situation.Revision != binding.SituationRevision {
		return nil, nil, ErrCRMPlaybookExecutionBlocked
	}
	return binding, source, nil
}

// ClaimAction records exact human authorization before contacting any provider.
// Replaying a consumed approval cannot invoke the executor a second time.
func (r *CRMPlaybookExecutionRepository) ClaimAction(ctx context.Context, ws, id, revision, memberID, approvedFingerprint string, action model.CRMPlaybookAction, now time.Time,
	validate func(model.CRMPlaybookActionIntent, model.CRMPlaybookExecutionSource, model.CRMPlaybookActionFacts) error,
) (*model.CRMPlaybookActionIntent, bool, error) {
	var result *model.CRMPlaybookActionIntent
	claimed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		intent, err := NewCRMPlaybookExecutionRepository(tx).ActionIntent(ctx, ws, id)
		if err != nil {
			return err
		}
		if intent == nil {
			return ErrCRMPlaybookExecutionBlocked
		}
		if intent.ApprovedAt != nil {
			if intent.ApprovedByMemberID == nil || *intent.ApprovedByMemberID != memberID || intent.ApprovedFingerprint != approvedFingerprint {
				return ErrCRMPlaybookConflict
			}
			result = intent
			return nil
		}
		_, source, err := lockedPlaybookRunSource(tx, ws, intent.RunID)
		if err != nil {
			return err
		}
		if !intent.ExpiresAt.After(now) || intent.ApproverMemberID != memberID {
			return ErrCRMPlaybookExecutionBlocked
		}
		facts, err := playbookActionFacts(tx, source.Item.Situation)
		if err != nil {
			return err
		}
		if err := validate(*intent, *source, *facts); err != nil {
			return err
		}
		if intent.RecipientKey != "" {
			// Serialize contact attempts across Playbooks, including unresolved sends.
			var competing int64
			if err := tx.Table("crm_playbook_action_intents i").Joins("JOIN crm_suggestions a ON a.workspace_id = i.workspace_id AND a.id = i.suggestion_id").
				Where("i.workspace_id = ? AND i.recipient_key = ? AND i.suggestion_id <> ? AND a.status = 'accepted'", ws, intent.RecipientKey, id).
				Where("a.execution_status = 'in_progress' OR (a.execution_status = 'succeeded' AND a.executed_at >= ?)", now.Add(-24*time.Hour)).Count(&competing).Error; err != nil {
				return err
			}
			if competing > 0 || facts.LastOutboundMessageAt != nil && facts.LastOutboundMessageAt.After(now.Add(-24*time.Hour)) {
				return ErrCRMPlaybookExecutionBlocked
			}
		}
		suggestion, err := NewCRMSuggestionRepository(tx).GetByID(ctx, ws, id)
		if err != nil {
			return err
		}
		if suggestion == nil || suggestion.Status != "pending" || suggestion.ExecutionStatus != "pending" || revision == "" || model.CRMSuggestionRevision(*suggestion) != revision {
			return ErrCRMPlaybookConflict
		}
		contextData := model.JSONB{}
		for k, v := range suggestion.Context {
			contextData[k] = v
		}
		contextData["playbook_action"] = action
		encoded, err := json.Marshal(contextData)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.CRMSuggestion{}).Where("workspace_id = ? AND id = ?", ws, id).
			Updates(map[string]any{"status": "accepted", "execution_status": "in_progress", "context": string(encoded), "updated_at": now}).Error; err != nil {
			return err
		}
		intent.ApprovedByMemberID, intent.ApprovedAt, intent.ApprovedFingerprint = &memberID, &now, approvedFingerprint
		if err := tx.Model(intent).Select("approved_by_member_id", "approved_at", "approved_fingerprint").Updates(intent).Error; err != nil {
			return err
		}
		result, claimed = intent, true
		return nil
	})
	return result, claimed, err
}

// FinishAction stores the authoritative executor result on the existing suggestion.
// An unknown send/create remains in progress for reconciliation, never retryable by approval.
func (r *CRMPlaybookExecutionRepository) FinishAction(ctx context.Context, intent model.CRMPlaybookActionIntent, status, resultType string, resultID *string, safeError *string, now time.Time) error {
	if status != "succeeded" && status != "failed" && status != "in_progress" {
		return ErrCRMPlaybookConflict
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, intent.WorkspaceID); err != nil {
			return err
		}
		updates := map[string]any{"execution_status": status, "execution_error": safeError, "updated_at": now}
		var matching int64
		if intent.ApprovedFingerprint == "" {
			return ErrCRMPlaybookConflict
		}
		if err := tx.Model(&model.CRMPlaybookActionIntent{}).Where("workspace_id = ? AND suggestion_id = ? AND approved_fingerprint = ? AND approved_at IS NOT NULL", intent.WorkspaceID, intent.SuggestionID, intent.ApprovedFingerprint).Count(&matching).Error; err != nil {
			return err
		}
		if matching != 1 {
			return ErrCRMPlaybookConflict
		}
		if status != "in_progress" {
			updates["executed_at"] = now
		}
		result := tx.Model(&model.CRMSuggestion{}).Where("workspace_id = ? AND id = ? AND status = 'accepted' AND execution_status = 'in_progress'", intent.WorkspaceID, intent.SuggestionID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var current model.CRMSuggestion
			if err := tx.Where("workspace_id = ? AND id = ?", intent.WorkspaceID, intent.SuggestionID).Take(&current).Error; err != nil {
				return err
			}
			// A slower inspection must never downgrade a confirmed result.
			if current.Status == "accepted" && (current.ExecutionStatus == "succeeded" || current.ExecutionStatus == "failed") {
				return nil
			}
			return ErrCRMPlaybookConflict
		}
		return tx.Model(&model.CRMPlaybookActionIntent{}).Where("workspace_id = ? AND suggestion_id = ? AND approved_fingerprint = ?", intent.WorkspaceID, intent.SuggestionID, intent.ApprovedFingerprint).
			Updates(map[string]any{"result_type": resultType, "result_id": resultID}).Error
	})
}
