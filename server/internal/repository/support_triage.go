package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SupportConversationTriageRepository struct {
	db *gorm.DB
}

func NewSupportConversationTriageRepository(db *gorm.DB) *SupportConversationTriageRepository {
	return &SupportConversationTriageRepository{db: db}
}

func (r *SupportConversationTriageRepository) GetByConversation(ctx context.Context, workspaceID, conversationID string) (*model.SupportConversationTriage, error) {
	var triage model.SupportConversationTriage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).
		First(&triage).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support conversation triage: %w", err)
	}
	return &triage, nil
}

func (r *SupportConversationTriageRepository) ListByConversationIDs(ctx context.Context, workspaceID string, conversationIDs []string) (map[string]model.SupportConversationTriage, error) {
	result := map[string]model.SupportConversationTriage{}
	if len(conversationIDs) == 0 {
		return result, nil
	}

	var rows []model.SupportConversationTriage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id IN ?", workspaceID, conversationIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list support conversation triage: %w", err)
	}

	for _, row := range rows {
		result[row.ConversationID] = row
	}
	return result, nil
}

func (r *SupportConversationTriageRepository) Upsert(ctx context.Context, triage *model.SupportConversationTriage) error {
	if triage == nil {
		return nil
	}
	var existing model.SupportConversationTriage
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", triage.WorkspaceID, triage.ConversationID).
		First(&existing).Error
	switch {
	case err == nil:
		triage.ID = existing.ID
		if triage.CreatedAt.IsZero() {
			triage.CreatedAt = existing.CreatedAt
		}
		if err := r.db.WithContext(ctx).Save(triage).Error; err != nil {
			return fmt.Errorf("update support conversation triage: %w", err)
		}
		return nil
	case err == gorm.ErrRecordNotFound:
		if err := r.db.WithContext(ctx).Create(triage).Error; err != nil {
			return fmt.Errorf("create support conversation triage: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("load support conversation triage: %w", err)
	}
}

func (r *SupportConversationTriageRepository) DB() *gorm.DB {
	return r.db
}

func (r *SupportConversationTriageRepository) WithTx(tx *gorm.DB) *SupportConversationTriageRepository {
	return &SupportConversationTriageRepository{db: tx}
}

type SupportConversationTriageEventRepository struct {
	db *gorm.DB
}

func NewSupportConversationTriageEventRepository(db *gorm.DB) *SupportConversationTriageEventRepository {
	return &SupportConversationTriageEventRepository{db: db}
}

func (r *SupportConversationTriageEventRepository) Create(ctx context.Context, event *model.SupportConversationTriageEvent) error {
	if event == nil {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create support conversation triage event: %w", err)
	}
	return nil
}

func (r *SupportConversationTriageEventRepository) CountAIEvaluationsSince(ctx context.Context, workspaceID string, since time.Time) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.SupportConversationTriageEvent{}).
		Where("workspace_id = ? AND event_type = ? AND source = ? AND cached = ? AND created_at >= ?",
			workspaceID,
			"evaluated",
			model.SupportConversationTriageSourceAI,
			false,
			since,
		).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count support conversation triage AI evaluations: %w", err)
	}
	return count, nil
}

func (r *SupportConversationTriageEventRepository) FindLatestEvaluatedByInputHash(ctx context.Context, workspaceID, inputHash string, since time.Time) (*model.SupportConversationTriageEvent, error) {
	var event model.SupportConversationTriageEvent
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND event_type = ? AND input_hash = ? AND created_at >= ?",
			workspaceID,
			"evaluated",
			inputHash,
			since,
		).
		Order("created_at DESC").
		First(&event).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find support conversation triage event by input hash: %w", err)
	}
	return &event, nil
}

func (r *SupportConversationTriageEventRepository) WithTx(tx *gorm.DB) *SupportConversationTriageEventRepository {
	return &SupportConversationTriageEventRepository{db: tx}
}

type SupportTriageRuleRepository struct {
	db *gorm.DB
}

func NewSupportTriageRuleRepository(db *gorm.DB) *SupportTriageRuleRepository {
	return &SupportTriageRuleRepository{db: db}
}

func (r *SupportTriageRuleRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.SupportTriageRule, error) {
	var rules []model.SupportTriageRule
	if err := r.baseQuery(ctx).
		Where("str.workspace_id = ?", workspaceID).
		Order("str.priority ASC, str.created_at ASC").
		Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("list support triage rules: %w", err)
	}
	return rules, nil
}

func (r *SupportTriageRuleRepository) ListActiveByWorkspace(ctx context.Context, workspaceID string) ([]model.SupportTriageRule, error) {
	var rules []model.SupportTriageRule
	if err := r.baseQuery(ctx).
		Where("str.workspace_id = ? AND str.active = ?", workspaceID, true).
		Order("str.priority ASC, str.created_at ASC").
		Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("list active support triage rules: %w", err)
	}
	return rules, nil
}

func (r *SupportTriageRuleRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.SupportTriageRule, error) {
	var rule model.SupportTriageRule
	if err := r.baseQuery(ctx).
		Where("str.workspace_id = ? AND str.id = ?", workspaceID, id).
		First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support triage rule: %w", err)
	}
	return &rule, nil
}

func (r *SupportTriageRuleRepository) Create(ctx context.Context, rule *model.SupportTriageRule) error {
	if err := r.db.WithContext(ctx).Create(rule).Error; err != nil {
		return fmt.Errorf("create support triage rule: %w", err)
	}
	return nil
}

func (r *SupportTriageRuleRepository) Update(ctx context.Context, rule *model.SupportTriageRule) error {
	if err := r.db.WithContext(ctx).Save(rule).Error; err != nil {
		return fmt.Errorf("update support triage rule: %w", err)
	}
	return nil
}

func (r *SupportTriageRuleRepository) Delete(ctx context.Context, workspaceID, id string) error {
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Delete(&model.SupportTriageRule{}).Error; err != nil {
		return fmt.Errorf("delete support triage rule: %w", err)
	}
	return nil
}

func (r *SupportTriageRuleRepository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("support_triage_rules str").
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = str.target_mailbox_id").
		Select("str.*, sm.name AS target_mailbox_name, sm.handle AS target_mailbox_handle")
}
