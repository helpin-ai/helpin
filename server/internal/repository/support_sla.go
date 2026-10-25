package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportSLAPolicyNotFound means the policy does not exist in the workspace.
var ErrSupportSLAPolicyNotFound = errors.New("SLA policy not found")

// SupportSLARepository stores SLA policies, evaluated conversation state and
// the evaluation queue.
type SupportSLARepository struct {
	db *gorm.DB
}

func NewSupportSLARepository(db *gorm.DB) *SupportSLARepository {
	return &SupportSLARepository{db: db}
}

func (r *SupportSLARepository) postgres() bool { return r.db.Dialector.Name() == "postgres" }

// ListPolicies returns the workspace's policies in evaluation order.
func (r *SupportSLARepository) ListPolicies(ctx context.Context, workspaceID string) ([]model.SupportSLAPolicy, error) {
	var policies []model.SupportSLAPolicy
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("position, created_at, id").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("list SLA policies: %w", err)
	}
	return policies, nil
}

// GetPolicy returns one policy.
func (r *SupportSLARepository) GetPolicy(ctx context.Context, workspaceID, policyID string) (*model.SupportSLAPolicy, error) {
	var policy model.SupportSLAPolicy
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, policyID).Take(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSupportSLAPolicyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get SLA policy: %w", err)
	}
	return &policy, nil
}

// CreatePolicy appends a policy after the workspace's existing ones.
func (r *SupportSLARepository) CreatePolicy(ctx context.Context, policy *model.SupportSLAPolicy) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var last struct{ Position *int }
		if err := tx.Model(&model.SupportSLAPolicy{}).Select("MAX(position) AS position").
			Where("workspace_id = ?", policy.WorkspaceID).Scan(&last).Error; err != nil {
			return fmt.Errorf("find last SLA policy position: %w", err)
		}
		policy.Position = 0
		if last.Position != nil {
			policy.Position = *last.Position + 1
		}
		if policy.ID == "" {
			policy.ID = uuid.NewString()
		}
		if err := tx.Create(policy).Error; err != nil {
			return fmt.Errorf("create SLA policy: %w", err)
		}
		return nil
	})
}

// CountPolicies returns how many policies the workspace has.
func (r *SupportSLARepository) CountPolicies(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SupportSLAPolicy{}).Where("workspace_id = ?", workspaceID).Count(&count).Error
	return count, err
}

// SavePolicy writes every editable field of an existing policy.
func (r *SupportSLARepository) SavePolicy(ctx context.Context, policy *model.SupportSLAPolicy) error {
	result := r.db.WithContext(ctx).Model(&model.SupportSLAPolicy{}).
		Where("workspace_id = ? AND id = ?", policy.WorkspaceID, policy.ID).
		Updates(map[string]any{
			"name": policy.Name, "enabled": policy.Enabled, "priorities": policy.Priorities,
			"mailbox_ids": policy.MailboxIDs, "channels": policy.Channels,
			"first_response_minutes": policy.FirstResponseMinutes, "next_response_minutes": policy.NextResponseMinutes,
			"resolution_minutes": policy.ResolutionMinutes, "business_hours": policy.BusinessHours,
			"pause_while_waiting": policy.PauseWhileWaiting, "active_since": policy.ActiveSince,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update SLA policy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSupportSLAPolicyNotFound
	}
	return nil
}

// DeletePolicy removes a policy. Conversations it governed are re-evaluated
// by the caller.
func (r *SupportSLARepository) DeletePolicy(ctx context.Context, workspaceID, policyID string) error {
	result := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, policyID).Delete(&model.SupportSLAPolicy{})
	if result.Error != nil {
		return fmt.Errorf("delete SLA policy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSupportSLAPolicyNotFound
	}
	return nil
}

// ReorderPolicies sets positions to the order given. The list must contain
// every policy of the workspace exactly once.
func (r *SupportSLARepository) ReorderPolicies(ctx context.Context, workspaceID string, policyIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []string
		if err := tx.Model(&model.SupportSLAPolicy{}).Where("workspace_id = ?", workspaceID).Pluck("id", &existing).Error; err != nil {
			return fmt.Errorf("list SLA policies to reorder: %w", err)
		}
		known := make(map[string]bool, len(existing))
		for _, id := range existing {
			known[id] = true
		}
		seen := make(map[string]bool, len(policyIDs))
		for _, id := range policyIDs {
			if !known[id] || seen[id] {
				return ErrSupportSLAPolicyNotFound
			}
			seen[id] = true
		}
		if len(seen) != len(known) {
			return ErrSupportSLAPolicyNotFound
		}
		for position, id := range policyIDs {
			if err := tx.Model(&model.SupportSLAPolicy{}).Where("workspace_id = ? AND id = ?", workspaceID, id).
				Updates(map[string]any{"position": position, "updated_at": time.Now().UTC()}).Error; err != nil {
				return fmt.Errorf("reorder SLA policies: %w", err)
			}
		}
		return nil
	})
}

// EnqueueWorkspace queues every conversation that may need a new SLA
// evaluation after the workspace's policies changed: open ones, and any that
// already carry SLA state.
func (r *SupportSLARepository) EnqueueWorkspace(ctx context.Context, workspaceID string) error {
	now := time.Now().UTC()
	db := r.db.WithContext(ctx)
	candidates := db.Model(&model.SupportConversation{}).Select("id").
		Where("workspace_id = ? AND merged_into_id IS NULL AND status IN ?", workspaceID,
			[]string{model.SupportConversationStatusOpen, model.SupportConversationStatusWaitingOnCustomer})
	var ids []string
	if err := candidates.Pluck("id", &ids).Error; err != nil {
		return fmt.Errorf("list conversations for SLA evaluation: %w", err)
	}
	var withState []string
	if err := db.Model(&model.SupportConversationSLARecord{}).Where("workspace_id = ?", workspaceID).
		Pluck("conversation_id", &withState).Error; err != nil {
		return fmt.Errorf("list SLA records for evaluation: %w", err)
	}
	ids = append(ids, withState...)
	return r.Enqueue(ctx, workspaceID, ids, now)
}

// Enqueue queues conversations for evaluation.
func (r *SupportSLARepository) Enqueue(ctx context.Context, workspaceID string, conversationIDs []string, at time.Time) error {
	if len(conversationIDs) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(conversationIDs))
	rows := make([]model.SupportSLAWork, 0, len(conversationIDs))
	for _, id := range conversationIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		rows = append(rows, model.SupportSLAWork{ConversationID: id, WorkspaceID: workspaceID, EnqueuedAt: at})
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "conversation_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enqueued_at"}),
	}).CreateInBatches(rows, 500).Error
	if err != nil {
		return fmt.Errorf("enqueue SLA evaluation: %w", err)
	}
	return nil
}

// ClaimWork removes up to limit queued conversations and returns them. On
// PostgreSQL concurrent workers claim disjoint rows.
func (r *SupportSLARepository) ClaimWork(ctx context.Context, limit int) ([]model.SupportSLAWork, error) {
	if limit <= 0 {
		limit = 100
	}
	var claimed []model.SupportSLAWork
	if r.postgres() {
		err := r.db.WithContext(ctx).Raw(`
			DELETE FROM support_sla_work
			WHERE conversation_id IN (
				SELECT conversation_id FROM support_sla_work
				ORDER BY enqueued_at
				LIMIT ?
				FOR UPDATE SKIP LOCKED
			)
			RETURNING conversation_id, workspace_id, enqueued_at`, limit).Scan(&claimed).Error
		if err != nil {
			return nil, fmt.Errorf("claim SLA work: %w", err)
		}
		return claimed, nil
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Order("enqueued_at").Limit(limit).Find(&claimed).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(claimed))
		for _, row := range claimed {
			ids = append(ids, row.ConversationID)
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Where("conversation_id IN ?", ids).Delete(&model.SupportSLAWork{}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim SLA work: %w", err)
	}
	return claimed, nil
}

// ListDueWakes returns conversations with a target due at or before now.
func (r *SupportSLARepository) ListDueWakes(ctx context.Context, now time.Time, limit int) ([]model.SupportSLAWork, error) {
	if limit <= 0 {
		limit = 200
	}
	var rows []model.SupportSLAWork
	err := r.db.WithContext(ctx).Model(&model.SupportConversationSLARecord{}).
		Select("conversation_id, workspace_id, wake_at AS enqueued_at").
		Where("wake_at IS NOT NULL AND wake_at <= ?", now).Order("wake_at").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list due SLA targets: %w", err)
	}
	return rows, nil
}

// SupportSLAInputs is what one evaluation reads.
type SupportSLAInputs struct {
	Conversation *model.SupportConversation
	Replies      []SupportSLAReplyRow
	Previous     *model.SupportConversationSLARecord
}

// SupportSLAReplyRow is one public reply.
type SupportSLAReplyRow struct {
	SenderType string
	CreatedAt  time.Time
}

// WithConversationLock runs fn in a transaction. On PostgreSQL a per-conversation advisory
// lock serializes evaluations, so a breach is recorded once.
func (r *SupportSLARepository) WithConversationLock(ctx context.Context, conversationID string, fn func(repo *SupportSLARepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if r.postgres() {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended('support_sla:' || ?, 0))", conversationID).Error; err != nil {
				return fmt.Errorf("lock SLA evaluation: %w", err)
			}
		}
		return fn(&SupportSLARepository{db: tx})
	})
}

// LoadInputs reads the conversation, its public replies and its SLA record.
func (r *SupportSLARepository) LoadInputs(ctx context.Context, workspaceID, conversationID string) (*SupportSLAInputs, error) {
	db := r.db.WithContext(ctx)
	var conversation model.SupportConversation
	err := db.Where("workspace_id = ? AND id = ?", workspaceID, conversationID).Take(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &SupportSLAInputs{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load conversation for SLA: %w", err)
	}
	inputs := &SupportSLAInputs{Conversation: &conversation}
	if err := db.Model(&model.SupportMessage{}).Select("sender_type, created_at").
		Where("workspace_id = ? AND conversation_id = ? AND message_type = 'reply' AND is_internal = ? AND system_event_type IS NULL",
			workspaceID, conversationID, false).
		Order("created_at, id").Scan(&inputs.Replies).Error; err != nil {
		return nil, fmt.Errorf("load replies for SLA: %w", err)
	}
	var previous model.SupportConversationSLARecord
	err = db.Where("conversation_id = ?", conversationID).Take(&previous).Error
	switch {
	case err == nil:
		inputs.Previous = &previous
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, fmt.Errorf("load SLA record: %w", err)
	}
	return inputs, nil
}

// SaveRecord inserts or replaces a conversation's SLA record.
func (r *SupportSLARepository) SaveRecord(ctx context.Context, record *model.SupportConversationSLARecord) error {
	record.UpdatedAt = time.Now().UTC()
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "conversation_id"}},
		UpdateAll: true,
	}).Create(record).Error
	if err != nil {
		return fmt.Errorf("save SLA record: %w", err)
	}
	return nil
}

// DeleteRecord removes a conversation's SLA record.
func (r *SupportSLARepository) DeleteRecord(ctx context.Context, conversationID string) error {
	if err := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID).Delete(&model.SupportConversationSLARecord{}).Error; err != nil {
		return fmt.Errorf("delete SLA record: %w", err)
	}
	return nil
}

// RecordsByConversation returns SLA records for the given conversations.
func (r *SupportSLARepository) RecordsByConversation(ctx context.Context, workspaceID string, conversationIDs []string) (map[string]model.SupportConversationSLARecord, error) {
	result := make(map[string]model.SupportConversationSLARecord, len(conversationIDs))
	if len(conversationIDs) == 0 {
		return result, nil
	}
	var rows []model.SupportConversationSLARecord
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id IN ?", workspaceID, conversationIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list SLA records: %w", err)
	}
	for _, row := range rows {
		result[row.ConversationID] = row
	}
	return result, nil
}
