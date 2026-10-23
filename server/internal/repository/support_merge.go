package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrSupportMerged prevents a stale mutation from changing another thread.
	ErrSupportMerged = errors.New("conversation was merged; open the active conversation")
	// ErrSupportMergeChanged requires a new review after a meaningful change.
	ErrSupportMergeChanged = errors.New("this conversation changed; review the update")
	// ErrSupportMergeNotFound deliberately does not distinguish missing and inaccessible records.
	ErrSupportMergeNotFound = errors.New("conversation not found")
	// ErrSupportMergeSending protects outstanding teammate replies from rerouting.
	ErrSupportMergeSending = errors.New("a reply is sending; try again when it finishes")
)

// SupportMergeRepository stores reversible relationships and shared history.
type SupportMergeRepository struct{ db *gorm.DB }

// NewSupportMergeRepository creates a merge repository.
func NewSupportMergeRepository(db *gorm.DB) *SupportMergeRepository {
	return &SupportMergeRepository{db: db}
}

// LockSupportRouting serializes short routing transactions within a workspace.
// Call before taking conversation/message locks, including AI and delayed sends.
func LockSupportRouting(tx *gorm.DB, workspaceID string) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "support-routing:"+workspaceID).Error
}

// Transaction runs a routing change with a consistent lock order.
func (r *SupportMergeRepository) Transaction(ctx context.Context, workspaceID string, fn func(*SupportMergeRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := LockSupportRouting(tx, workspaceID); err != nil {
			return err
		}
		return fn(NewSupportMergeRepository(tx))
	})
}

// DB returns the transaction handle for scoped collaborating repositories.
func (r *SupportMergeRepository) DB() *gorm.DB { return r.db }

// Active returns the workspace's active direct edges, without customer data.
func (r *SupportMergeRepository) Active(ctx context.Context, workspaceID string) ([]model.SupportConversationMerge, error) {
	var rows []model.SupportConversationMerge
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND ended_at IS NULL", workspaceID).Order("created_at,id").Find(&rows).Error
	return rows, err
}

// Root resolves the currently active handler for a conversation.
func (r *SupportMergeRepository) Root(ctx context.Context, workspaceID, id string) (string, error) {
	edges, err := r.Active(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return model.SupportMergeRoot(id, mergeParents(edges))
}

func mergeParents(edges []model.SupportConversationMerge) map[string]string {
	parents := make(map[string]string, len(edges))
	for _, e := range edges {
		parents[e.SourceID] = e.TargetID
	}
	return parents
}

// Group returns all records handled by root. Callers still authorize each record.
func (r *SupportMergeRepository) Group(ctx context.Context, workspaceID, root string) ([]model.SupportConversation, error) {
	edges, err := r.Active(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	parents := mergeParents(edges)
	ids := []string{root}
	for id := range parents {
		resolved, err := model.SupportMergeRoot(id, parents)
		if err != nil {
			return nil, err
		}
		if resolved == root {
			ids = append(ids, id)
		}
	}
	return r.Conversations(ctx, workspaceID, ids)
}

// Conversations loads original records without applying a staff access grant.
func (r *SupportMergeRepository) Conversations(ctx context.Context, workspaceID string, ids []string) ([]model.SupportConversation, error) {
	var rows []model.SupportConversation
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id IN ?", workspaceID, ids).Order("id").Find(&rows).Error
	return rows, err
}

// FormerMergeAudience returns customer and CC addresses of conversations that
// were merged into group and later split out, including records still merged
// under them. Their emails must not reach the remaining customer's thread.
func (r *SupportMergeRepository) FormerMergeAudience(ctx context.Context, workspaceID string, group []model.SupportConversation) ([]string, error) {
	ids := make([]string, 0, len(group))
	for _, c := range group {
		ids = append(ids, c.ID)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var ended []model.SupportConversationMerge
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND ended_at IS NOT NULL AND (source_id IN ? OR target_id IN ?)", workspaceID, ids, ids).Find(&ended).Error; err != nil {
		return nil, err
	}
	var former []string
	for _, e := range ended {
		for _, id := range []string{e.SourceID, e.TargetID} {
			if !slices.Contains(ids, id) && !slices.Contains(former, id) {
				former = append(former, id)
			}
		}
	}
	if len(former) == 0 {
		return nil, nil
	}
	active, err := r.Active(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	parents := mergeParents(active)
	splitRoots := slices.Clone(former)
	for id := range parents {
		// Bounded walk: a malformed cycle must not hang inbound processing.
		for p, steps := parents[id], 0; p != "" && steps <= len(parents); p, steps = parents[p], steps+1 {
			if slices.Contains(splitRoots, p) {
				if !slices.Contains(ids, id) && !slices.Contains(former, id) {
					former = append(former, id)
				}
				break
			}
		}
	}
	records, err := r.Conversations(ctx, workspaceID, former)
	if err != nil {
		return nil, err
	}
	var emails []string
	for _, c := range records {
		addresses := append([]string{}, c.EmailCC...)
		if c.CustomerEmail != nil {
			addresses = append(addresses, *c.CustomerEmail)
		}
		for _, raw := range addresses {
			if email := strings.ToLower(strings.TrimSpace(raw)); email != "" && !slices.Contains(emails, email) {
				emails = append(emails, email)
			}
		}
	}
	return emails, nil
}

// Get returns a single merge interval scoped to its workspace.
func (r *SupportMergeRepository) Get(ctx context.Context, workspaceID, id string) (*model.SupportConversationMerge, error) {
	var row model.SupportConversationMerge
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSupportMergeNotFound
	}
	return &row, err
}

// List returns incoming and outgoing historical edges for an original record.
func (r *SupportMergeRepository) List(ctx context.Context, workspaceID, id string) ([]model.SupportConversationMerge, error) {
	rows := []model.SupportConversationMerge{}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND (target_id = ? OR source_id = ?)", workspaceID, id, id).Order("created_at,id").Find(&rows).Error
	return rows, err
}

// Merge atomically adds edges; originals and their messages are never moved.
// The service rechecks authorization and the preview in the outer transaction.
func (r *SupportMergeRepository) Merge(ctx context.Context, workspaceID, targetID string, sourceIDs []string, actorID, operationID string, crossCustomer bool) ([]model.SupportConversationMerge, error) {
	var result []model.SupportConversationMerge
	err := r.Transaction(ctx, workspaceID, func(tx *SupportMergeRepository) error {
		if len(sourceIDs) == 0 || len(sourceIDs) > 49 || operationID == "" || actorID == "" {
			return fmt.Errorf("select between 2 and 50 conversations")
		}
		ids := append([]string{targetID}, sourceIDs...)
		slices.Sort(ids)
		if len(slices.Compact(slices.Clone(ids))) != len(ids) {
			return fmt.Errorf("select each conversation once")
		}
		if err := tx.db.Where("workspace_id = ? AND operation_id = ?", workspaceID, operationID).Order("source_id").Find(&result).Error; err != nil {
			return err
		}
		if len(result) > 0 {
			got := make([]string, 0, len(result))
			for _, edge := range result {
				if edge.TargetID != targetID || edge.ActorID != actorID {
					return ErrSupportMergeChanged
				}
				got = append(got, edge.SourceID)
			}
			want := slices.Clone(sourceIDs)
			sort.Strings(want)
			if !slices.Equal(got, want) {
				return ErrSupportMergeChanged
			}
			return nil
		}
		records, err := tx.Conversations(ctx, workspaceID, ids)
		if err != nil {
			return err
		}
		if len(records) != len(ids) {
			return ErrSupportMergeNotFound
		}
		var target model.SupportConversation
		for _, c := range records {
			if c.AnonymizedAt != nil {
				return fmt.Errorf("deleted customers cannot be merged")
			}
			if c.MergedIntoID != nil {
				return ErrSupportMerged
			}
			if c.ID == targetID {
				target = c
			}
		}
		for _, c := range records {
			if c.ID != targetID && !crossCustomer && !model.SameSupportCustomer(&target, &c) {
				return fmt.Errorf("different customer; ask an admin to enable this")
			}
		}
		groupIDs := slices.Clone(ids)
		for _, id := range ids {
			group, err := tx.Group(ctx, workspaceID, id)
			if err != nil {
				return err
			}
			for _, c := range group {
				groupIDs = append(groupIDs, c.ID)
			}
		}
		slices.Sort(groupIDs)
		groupIDs = slices.Compact(groupIDs)
		if len(groupIDs) > 50 {
			return fmt.Errorf("select up to 50 conversations including merged history")
		}
		if err := tx.EnsureNoPendingSends(ctx, workspaceID, groupIDs); err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, sourceID := range sourceIDs {
			edge := model.SupportConversationMerge{ID: uuid.NewString(), WorkspaceID: workspaceID, OperationID: operationID, SourceID: sourceID, TargetID: targetID, ActorID: actorID, CrossCustomer: crossCustomer, CreatedAt: now}
			if err := tx.db.Create(&edge).Error; err != nil {
				return err
			}
			result = append(result, edge)
			if err := tx.db.Model(&model.SupportConversation{}).Where("workspace_id = ? AND id = ?", workspaceID, sourceID).Updates(map[string]any{"merged_into_id": targetID}).Error; err != nil {
				return err
			}
		}
		if err := tx.preserveUnread(ctx, workspaceID, targetID, groupIDs, now); err != nil {
			return err
		}
		if err := tx.fence(ctx, workspaceID, groupIDs, actorID, now); err != nil {
			return err
		}
		allRecords, err := tx.Conversations(ctx, workspaceID, groupIDs)
		if err != nil {
			return err
		}
		fields := map[string]any{}
		for _, c := range allRecords {
			if c.CustomerAwaitingResponse {
				fields["customer_awaiting_response"] = true
				fields["needs_human_reply"] = true
			}
			if c.CustomerRequestedHumanAt != nil && target.CustomerRequestedHumanAt == nil {
				fields["customer_requested_human_at"] = c.CustomerRequestedHumanAt
				fields["human_takeover"] = true
			}
			if c.HumanTakeover != nil && *c.HumanTakeover {
				fields["human_takeover"] = true
			}
			if target.Status == model.SupportConversationStatusResolved && c.Status != model.SupportConversationStatusResolved && c.Status != model.SupportConversationStatusSpam {
				fields["status"] = "open"
				fields["resolved_at"] = nil
				fields["closed_at"] = nil
				fields["ai_resolved_at"] = nil
				fields["ai_resolution_type"] = nil
			}
		}
		if len(fields) > 0 {
			if err := tx.db.Model(&model.SupportConversation{}).Where("workspace_id = ? AND id = ?", workspaceID, targetID).Updates(fields).Error; err != nil {
				return err
			}
		}
		return tx.recordOperation(ctx, workspaceID, targetID, actorID, operationID, model.SupportEventConversationMerged, sourceIDs, now)
	})
	return result, err
}

// EnsureNoPendingSends keeps merges from redirecting queued teammate messages.
func (r *SupportMergeRepository) EnsureNoPendingSends(ctx context.Context, workspaceID string, ids []string) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.SupportPendingSend{}).Where("workspace_id = ? AND conversation_id IN ? AND status IN ?", workspaceID, ids, []string{"pending", "preparing", "processing", "queued", "translating", "sending"}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrSupportMergeSending
	}
	// Explicit and fallback replies may already be on their way to a provider.
	if err := r.db.WithContext(ctx).Model(&model.SupportMessage{}).
		Joins("JOIN support_conversations sc ON sc.id = support_messages.conversation_id AND sc.workspace_id = support_messages.workspace_id").
		Where("support_messages.workspace_id = ? AND conversation_id IN ? AND sender_type <> 'customer' AND is_internal = false AND message_type = 'reply' AND email_notified_at IS NULL AND cancellable_until IS NOT NULL", workspaceID, ids).
		Where(supportDeliveryModeSQL(r.db, "support_messages")+" <> ?", model.SupportDeliveryChatOnly).
		Where(supportMetadataStringSQL(r.db, "support_messages", "email_delivery_status")+" <> ?", "blocked").
		Where("(" + supportDeliveryModeSQL(r.db, "support_messages") + " IN ('email_only','chat_and_email') OR " + supportMetadataStringSQL(r.db, "support_messages", "merge_email_recipients") + " NOT IN ('', '[]', 'null') OR (sc.email_unsubscribed = false AND sc.customer_email IS NOT NULL AND (sc.contact_last_seen_at IS NULL OR support_messages.created_at > sc.contact_last_seen_at)))").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrSupportMergeSending
	}
	return nil
}

func (r *SupportMergeRepository) fence(ctx context.Context, workspaceID string, ids []string, actorID string, now time.Time) error {
	records, err := r.Conversations(ctx, workspaceID, ids)
	if err != nil {
		return err
	}
	for _, c := range records {
		job := model.SupportMergeJob{ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: c.ID, ActorID: actorID, RunID: c.AIActiveRunID, AvailableAt: now, CreatedAt: now}
		if err := r.db.WithContext(ctx).Create(&job).Error; err != nil {
			return err
		}
	}

	fields := map[string]any{"human_takeover": true, "ai_state": "escalated", "assigned_agent_id": nil, "flow_state": gorm.Expr("CASE WHEN assigned_user_id IS NULL THEN 'waiting_for_human' ELSE 'assigned_to_human' END"), "merge_revision": gorm.Expr("merge_revision + 1"), "ai_control_version": gorm.Expr("ai_control_version + 1"), "support_state_version": gorm.Expr("support_state_version + 1"), "ai_active_run_id": nil, "ai_paused_at": now, "ai_paused_by_user_id": actorID, "updated_at": now}
	if err := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("workspace_id = ? AND id IN ?", workspaceID, ids).Updates(fields).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.SupportAIFollowUp{}).Where("workspace_id = ? AND conversation_id IN ? AND status IN ?", workspaceID, ids, []string{"scheduled", "assessing", "waiting"}).Updates(map[string]any{"status": "cancelled", "reason": "conversation_merged", "cancelled_by_user_id": actorID, "lease_until": nil, "updated_at": now}).Error
}

// Unmerge ends one direct edge; descendants follow their restored parent.
// Existing explicit message grants remain bounded to their committed messages.
func (r *SupportMergeRepository) Unmerge(ctx context.Context, workspaceID, mergeID, actorID, operationID string) error {
	return r.Transaction(ctx, workspaceID, func(tx *SupportMergeRepository) error {
		edge, err := tx.Get(ctx, workspaceID, mergeID)
		if err != nil {
			return err
		}
		if edge.EndedAt != nil {
			if edge.UnmergeOperationID != nil && *edge.UnmergeOperationID == operationID && edge.EndedBy != nil && *edge.EndedBy == actorID {
				return nil
			}
			return ErrSupportMergeChanged
		}
		records, err := tx.Conversations(ctx, workspaceID, []string{edge.SourceID, edge.TargetID})
		if err != nil {
			return err
		}
		if len(records) != 2 {
			return ErrSupportMergeNotFound
		}
		for _, c := range records {
			if c.AnonymizedAt != nil {
				return fmt.Errorf("this conversation was deleted and cannot be restored")
			}
		}
		root, err := tx.Root(ctx, workspaceID, edge.TargetID)
		if err != nil {
			return err
		}
		group, err := tx.Group(ctx, workspaceID, root)
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(group))
		for _, c := range group {
			ids = append(ids, c.ID)
		}
		if err := tx.EnsureNoPendingSends(ctx, workspaceID, ids); err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.db.Model(edge).Updates(map[string]any{"ended_at": now, "ended_by": actorID, "unmerge_operation_id": operationID}).Error; err != nil {
			return err
		}
		if err := tx.db.Model(&model.SupportConversation{}).Where("workspace_id = ? AND id = ?", workspaceID, edge.SourceID).Update("merged_into_id", nil).Error; err != nil {
			return err
		}
		if err := tx.fence(ctx, workspaceID, ids, actorID, now); err != nil {
			return err
		}
		return tx.recordOperation(ctx, workspaceID, root, actorID, operationID, model.SupportEventConversationUnmerged, []string{edge.SourceID}, now)
	})
}

// LockConversation follows the common routing-before-row locking order.
func (r *SupportMergeRepository) LockConversation(ctx context.Context, workspaceID, id string) (*model.SupportConversation, error) {
	var c model.SupportConversation
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSupportMergeNotFound
	}
	return &c, err
}

// Operation looks up an idempotent merge operation within its workspace.
func (r *SupportMergeRepository) Operation(ctx context.Context, workspaceID, operationID string) ([]model.SupportConversationMerge, error) {
	rows := []model.SupportConversationMerge{}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND operation_id = ?", workspaceID, operationID).Order("source_id").Find(&rows).Error
	return rows, err
}

// RestoreLocation applies the reviewed inbox and a still-eligible assignee.
func (r *SupportMergeRepository) RestoreLocation(ctx context.Context, workspaceID string, c model.SupportConversation) error {
	fields := map[string]any{"mailbox_id": c.MailboxID, "assigned_user_id": c.AssignedUserID}
	if c.AssignedUserID == nil {
		fields["flow_state"] = model.SupportConversationFlowStateWaitingForHuman
	} else {
		fields["flow_state"] = model.SupportConversationFlowStateAssignedToHuman
	}
	return r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("workspace_id = ? AND id = ? AND merged_into_id IS NULL", workspaceID, c.ID).Updates(fields).Error
}

// recordOperation joins the same transaction as routing; retries produce no duplicate events.
func (r *SupportMergeRepository) recordOperation(ctx context.Context, ws, target, actor, operation, kind string, sources []string, now time.Time) error {
	metadata, err := json.Marshal(map[string]any{"actor_id": actor, "operation_id": operation, "source_ids": sources})
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Table("support_events").Create(map[string]any{"id": uuid.NewString(), "workspace_id": ws, "conversation_id": target, "event_type": kind, "actor_type": model.SupportEventActorAgent, "channel": model.SupportEventChannelInternal, "metadata": string(metadata), "occurred_at": now, "created_at": now}).Error
}

// preserveUnread keeps existing personal reminders visible in the destination.
// Original cursors stay unchanged so separation can restore each user's state.
func (r *SupportMergeRepository) preserveUnread(ctx context.Context, ws, target string, ids []string, now time.Time) error {
	var users []string
	if err := r.db.WithContext(ctx).Model(&model.SupportConversationUserState{}).Distinct("user_id").Where("workspace_id = ? AND conversation_id IN ? AND conversation_id <> ? AND (unread_customer_message_count > 0 OR manually_unread = true)", ws, ids, target).Pluck("user_id", &users).Error; err != nil {
		return err
	}
	for _, user := range users {
		state := model.SupportConversationUserState{WorkspaceID: ws, ConversationID: target, UserID: user, ManuallyUnread: true, Version: 1, CreatedAt: now, UpdatedAt: now}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "conversation_id"}, {Name: "user_id"}}, DoUpdates: clause.Assignments(map[string]any{"manually_unread": true, "version": gorm.Expr("support_conversation_user_states.version + 1"), "updated_at": now})}).Create(&state).Error; err != nil {
			return err
		}
	}
	return nil
}
