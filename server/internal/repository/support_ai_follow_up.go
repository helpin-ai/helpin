package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportFollowUpRepository persists bounded, restart-safe inactivity work.
type SupportFollowUpRepository struct{ db *gorm.DB }

// NewSupportFollowUpRepository creates the support follow-up store.
func NewSupportFollowUpRepository(db *gorm.DB) *SupportFollowUpRepository {
	return &SupportFollowUpRepository{db: db}
}

// EnabledInstallations returns installations explicitly opting into follow-ups.
func (r *SupportFollowUpRepository) EnabledInstallations(ctx context.Context) ([]model.SupportWidgetInstallation, error) {
	var rows []model.SupportWidgetInstallation
	err := r.db.WithContext(ctx).Where("active = true AND coalesce(settings->>'ai_follow_up_enabled', 'true') = 'true'").Find(&rows).Error
	return rows, err
}

// Seed reserves at most 25 assessments per workspace per UTC day. Existing
// backlog is limited to 30 days; a follow-up itself can never seed another.
func (r *SupportFollowUpRepository) Seed(ctx context.Context, workspaceID string, settings model.SupportInboxSettings, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var installation model.SupportWidgetInstallation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ?", workspaceID).First(&installation).Error; err != nil {
			return err
		}
		settings = model.DefaultSupportInboxSettings()
		if err := json.Unmarshal([]byte(installation.Settings), &settings); err != nil {
			return err
		}
		if !installation.Active || !settings.AIFollowUpEnabled || !settings.AIEnabled || settings.AIResponseMode != "ai_first" {
			return nil
		}
		var count int64
		if err := tx.Model(&model.SupportAIFollowUp{}).Where("workspace_id = ? AND created_at >= ?", workspaceID, now.UTC().Truncate(24*time.Hour)).Count(&count).Error; err != nil {
			return err
		}
		if count >= 25 {
			return nil
		}
		var rows []model.SupportConversation
		err := supportFollowUpCandidates(tx, workspaceID, settings, now).
			Order("last_public_message_at, id").Limit(25 - int(count)).Find(&rows).Error
		if err != nil {
			return err
		}
		for _, conv := range rows {
			episode := model.SupportAIFollowUp{ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conv.ID, SourceMessageID: *conv.LastPublicMessageID, RunID: uuid.NewString(), Status: "scheduled", DueAt: now, SequenceVersion: 2, AssessmentAttempts: 1, SecondDelayHours: settings.AIFollowUpSecondDelayHours, CloseHours: settings.AIFollowUpCloseHours, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&episode).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Claim leases a bounded batch. The episode state and source fences still
// determine whether a claimed action can commit.
func (r *SupportFollowUpRepository) Claim(ctx context.Context, now time.Time) ([]model.SupportAIFollowUp, error) {
	var rows []model.SupportAIFollowUp
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status IN ? AND due_at <= ? AND (lease_until IS NULL OR lease_until <= ?)", []string{"scheduled", "assessing", "waiting"}, now, now).Order("due_at,id").Limit(50).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := tx.Model(&model.SupportAIFollowUp{}).Where("id = ?", row.ID).Update("lease_until", now.Add(2*time.Minute)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return rows, err
}

// WithEpisode serializes policy, ownership and source-message checks with the
// state mutation. Lock order matches cancellation triggers.
func (r *SupportFollowUpRepository) WithEpisode(ctx context.Context, workspaceID, id string, fn func(*gorm.DB, *model.SupportWidgetInstallation, *model.SupportConversation, *model.SupportAIFollowUp) error) error {
	var ref model.SupportAIFollowUp
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&ref).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inst model.SupportWidgetInstallation
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("workspace_id = ?", workspaceID).First(&inst).Error; err != nil {
			return err
		}
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, ref.ConversationID).First(&conv).Error; err != nil {
			return err
		}
		var episode model.SupportAIFollowUp
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&episode).Error; err != nil {
			return err
		}
		return fn(tx, &inst, &conv, &episode)
	})
}

// Latest returns the most recent follow-up outcome for the conversation panel.
func (r *SupportFollowUpRepository) Latest(ctx context.Context, workspaceID, conversationID string) (*model.SupportAIFollowUp, error) {
	var row model.SupportAIFollowUp
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).Order("created_at DESC,id DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

// CancelConversation cancels current work without changing conversation ownership.
func (r *SupportFollowUpRepository) CancelConversation(ctx context.Context, workspaceID, conversationID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, conversationID).First(&conv).Error; err != nil {
			return err
		}
		return tx.Model(&model.SupportAIFollowUp{}).Where("workspace_id = ? AND conversation_id = ? AND status IN ?", workspaceID, conversationID, []string{"scheduled", "assessing", "waiting"}).Updates(map[string]any{"status": "cancelled", "reason": "cancelled_by_teammate", "updated_at": time.Now().UTC()}).Error
	})
}

func supportFollowUpCandidates(tx *gorm.DB, workspaceID string, settings model.SupportInboxSettings, now time.Time) *gorm.DB {
	channels := []string{}
	if model.SupportAIChannelEnabled(settings, "chat") {
		channels = append(channels, "widget")
	}
	if model.SupportAIChannelEnabled(settings, "email") {
		channels = append(channels, "email")
	}
	return tx.Model(&model.SupportConversation{}).Where(`(CASE WHEN source = 'email' OR channel = 'email' OR EXISTS
 (SELECT 1 FROM support_messages m WHERE m.id = support_conversations.last_public_message_id AND m.workspace_id = support_conversations.workspace_id AND m.via_channel = 'email')
 THEN 'email' ELSE channel END) IN ?`, channels).Where(`workspace_id = ? AND flow_state = 'ai_handling' AND ai_state = 'pending'
   AND status IN ('open','waiting_on_customer')
   AND NOT coalesce(human_takeover,false) AND assigned_user_id IS NULL AND opened_by_user_id IS NULL
   AND customer_requested_human_at IS NULL AND linked_task_id IS NULL
   AND last_public_message_id IS NOT NULL AND NOT email_unsubscribed
   AND assigned_agent_id = ? AND last_public_sender_type = 'ai' AND last_public_message_at <= ? AND last_public_message_at >= ?
   AND NOT customer_awaiting_response
   AND NOT EXISTS (SELECT 1 FROM support_ai_follow_ups f WHERE f.workspace_id = support_conversations.workspace_id AND f.conversation_id = support_conversations.id AND (f.source_message_id = support_conversations.last_public_message_id OR f.sent_message_id = support_conversations.last_public_message_id OR f.second_message_id = support_conversations.last_public_message_id OR f.status IN ('scheduled','assessing','waiting')))`, workspaceID, settings.AIAgentID, now.Add(-time.Duration(settings.AIFollowUpDelayHours)*time.Hour), now.Add(-30*24*time.Hour))
}

// Preview returns counts and a bounded sample without creating work or agent runs.
func (r *SupportFollowUpRepository) Preview(ctx context.Context, workspaceID string, settings model.SupportInboxSettings, now time.Time) (*model.SupportFollowUpPreview, error) {
	result := &model.SupportFollowUpPreview{DailyLimit: 25, LookbackDays: 30}
	query := supportFollowUpCandidates(r.db.WithContext(ctx), workspaceID, settings, now)
	if err := query.Count(&result.Candidates).Error; err != nil {
		return nil, err
	}
	if err := query.Select("id,display_id,subject,last_public_message_at").Order("last_public_message_at,id").Limit(10).Find(&result.Sample).Error; err != nil {
		return nil, err
	}
	return result, nil
}

// SupportConversationBusy reports active work that must finish before sending
// a scheduled follow-up or closing. Idle visitor chat runs do not block it.
func SupportConversationBusy(ctx context.Context, tx *gorm.DB, workspaceID, conversationID, assessmentRunID string) (bool, error) {
	var count int64
	query := tx.WithContext(ctx).Model(&model.AgentRun{}).Where("workspace_id = ? AND target_type = 'support_conversation' AND target_id = ? AND (status IN ('queued','running') OR (status = 'paused' AND coalesce(pause_reason,'') <> ?))", workspaceID, conversationID, model.AgentRunPauseReasonUserMessage)
	if assessmentRunID != "" {
		query = query.Where("id <> ?", assessmentRunID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}
