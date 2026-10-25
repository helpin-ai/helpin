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

// EnqueueOpenRecords queues the open conversations that carry SLA state,
// after the workspace's policies changed in a way that may re-select or
// stop their policy.
func (r *SupportSLARepository) EnqueueOpenRecords(ctx context.Context, workspaceID string, at time.Time) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO support_sla_work (conversation_id, workspace_id, enqueued_at)
		SELECT sla.conversation_id, sla.workspace_id, ?
		FROM support_conversation_slas AS sla
		JOIN support_conversations AS c ON c.id = sla.conversation_id
		WHERE sla.workspace_id = ? AND sla.detached_at IS NULL AND c.status IN ?
		ON CONFLICT (conversation_id) DO UPDATE SET enqueued_at = excluded.enqueued_at`,
		at, workspaceID,
		[]string{model.SupportConversationStatusOpen, model.SupportConversationStatusWaitingOnCustomer}).Error
	if err != nil {
		return fmt.Errorf("enqueue open SLA records: %w", err)
	}
	return nil
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

// ClaimWork leases up to limit queued conversations that are due by now,
// hiding them from other workers until leaseUntil. The caller completes each
// one after evaluating it; a lease that runs out returns the row to the
// queue, so a crash or shutdown loses nothing.
func (r *SupportSLARepository) ClaimWork(ctx context.Context, now, leaseUntil time.Time, limit int) ([]model.SupportSLAWork, error) {
	if limit <= 0 {
		limit = 100
	}
	// Postgres keeps microseconds; the lease must compare equal on completion.
	leaseUntil = leaseUntil.UTC().Truncate(time.Microsecond)
	var claimed []model.SupportSLAWork
	if r.postgres() {
		err := r.db.WithContext(ctx).Raw(`
			UPDATE support_sla_work SET enqueued_at = ?
			WHERE conversation_id IN (
				SELECT conversation_id FROM support_sla_work
				WHERE enqueued_at <= ?
				ORDER BY enqueued_at
				LIMIT ?
				FOR UPDATE SKIP LOCKED
			)
			RETURNING conversation_id, workspace_id, enqueued_at`, leaseUntil, now, limit).Scan(&claimed).Error
		if err != nil {
			return nil, fmt.Errorf("claim SLA work: %w", err)
		}
		return claimed, nil
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("enqueued_at <= ?", now).Order("enqueued_at").Limit(limit).Find(&claimed).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(claimed))
		for i := range claimed {
			ids = append(ids, claimed[i].ConversationID)
			claimed[i].EnqueuedAt = leaseUntil
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Model(&model.SupportSLAWork{}).Where("conversation_id IN ?", ids).Update("enqueued_at", leaseUntil).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim SLA work: %w", err)
	}
	return claimed, nil
}

// CompleteWork removes a claimed row unless it was queued again while the
// evaluation ran.
func (r *SupportSLARepository) CompleteWork(ctx context.Context, lease model.SupportSLAWork) error {
	if err := r.db.WithContext(ctx).Where("conversation_id = ? AND enqueued_at = ?", lease.ConversationID, lease.EnqueuedAt).
		Delete(&model.SupportSLAWork{}).Error; err != nil {
		return fmt.Errorf("complete SLA work: %w", err)
	}
	return nil
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
