package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMMeetingRepository persists provider-neutral meeting records and artifacts.
type CRMMeetingRepository struct {
	db *gorm.DB
}

// NewCRMMeetingRepository creates a CRMMeetingRepository.
func NewCRMMeetingRepository(db *gorm.DB) *CRMMeetingRepository {
	return &CRMMeetingRepository{db: db}
}

// Create inserts a meeting.
func (r *CRMMeetingRepository) Create(ctx context.Context, meeting *model.CRMMeeting) error {
	if err := r.db.WithContext(ctx).Create(meeting).Error; err != nil {
		return fmt.Errorf("create CRM meeting: %w", err)
	}
	return nil
}

// GetByID returns a workspace-scoped meeting or nil when it does not exist.
func (r *CRMMeetingRepository) GetByID(ctx context.Context, workspaceID, meetingID string) (*model.CRMMeeting, error) {
	var meeting model.CRMMeeting
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, meetingID).
		First(&meeting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get CRM meeting: %w", err)
	}
	return &meeting, nil
}

// List returns workspace meetings with safe, provider-neutral filters.
func (r *CRMMeetingRepository) List(
	ctx context.Context,
	workspaceID string,
	filters model.CRMMeetingListFilters,
	pagination model.PMPagination,
) ([]model.CRMMeeting, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMMeeting{}).Where("crm_meetings.workspace_id = ?", workspaceID)
	if filters.Status != nil {
		query = query.Where("crm_meetings.status = ?", strings.TrimSpace(*filters.Status))
	}
	if filters.OwnerMemberID != nil {
		query = query.Where("crm_meetings.owner_member_id = ?", strings.TrimSpace(*filters.OwnerMemberID))
	}
	if filters.Search != nil && strings.TrimSpace(*filters.Search) != "" {
		query = query.Where("LOWER(crm_meetings.title) LIKE ?", "%"+strings.ToLower(strings.TrimSpace(*filters.Search))+"%")
	}
	if filters.StartAfter != nil {
		query = query.Where("crm_meetings.scheduled_start_at >= ?", *filters.StartAfter)
	}
	if filters.StartBefore != nil {
		query = query.Where("crm_meetings.scheduled_start_at <= ?", *filters.StartBefore)
	}
	query = applyMeetingAssociationFilter(query, model.CRMObjectContact, filters.ContactID)
	query = applyMeetingAssociationFilter(query, model.CRMObjectCompany, filters.CompanyID)
	query = applyMeetingAssociationFilter(query, model.CRMObjectDeal, filters.DealID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count CRM meetings: %w", err)
	}
	page, perPage := normalizedPagination(pagination)
	var meetings []model.CRMMeeting
	if err := query.
		Order("COALESCE(scheduled_start_at, created_at) DESC, created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&meetings).Error; err != nil {
		return nil, 0, fmt.Errorf("list CRM meetings: %w", err)
	}
	return meetings, total, nil
}

// Update persists editable or lifecycle meeting fields.
func (r *CRMMeetingRepository) Update(ctx context.Context, meeting *model.CRMMeeting) error {
	if err := r.db.WithContext(ctx).Save(meeting).Error; err != nil {
		return fmt.Errorf("update CRM meeting: %w", err)
	}
	return nil
}

// Delete deletes a meeting and its owned artifacts transactionally.
func (r *CRMMeetingRepository) Delete(ctx context.Context, workspaceID, meetingID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		models := []interface{}{
			&model.CRMMeetingProviderEvent{},
			&model.CRMMeetingActionItem{},
			&model.CRMMeetingIntelligence{},
			&model.CRMMeetingTranscript{},
			&model.CRMMeetingCapture{},
		}
		for _, record := range models {
			if err := tx.Where("workspace_id = ? AND meeting_id = ?", workspaceID, meetingID).Delete(record).Error; err != nil {
				return err
			}
		}
		if err := tx.Where(
			"workspace_id = ? AND ((from_object_type = ? AND from_object_id = ?) OR (to_object_type = ? AND to_object_id = ?))",
			workspaceID, model.CRMObjectMeeting, meetingID, model.CRMObjectMeeting, meetingID,
		).Delete(&model.CRMAssociation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, meetingID).Delete(&model.CRMMeeting{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// CreateCapture inserts one immutable provider capture attempt.
func (r *CRMMeetingRepository) CreateCapture(ctx context.Context, capture *model.CRMMeetingCapture) error {
	if err := r.db.WithContext(ctx).Create(capture).Error; err != nil {
		return fmt.Errorf("create meeting capture: %w", err)
	}
	return nil
}

// WithCaptureLaunchLock serializes one idempotency key across API replicas.
// PostgreSQL advisory locks avoid launching duplicate provider bots before the
// unique capture row exists. Tests and non-PostgreSQL tools run inline.
func (r *CRMMeetingRepository) WithCaptureLaunchLock(
	ctx context.Context,
	workspaceID, idempotencyKey string,
	fn func(*CRMMeetingRepository) error,
) error {
	if r.db.Dialector.Name() != "postgres" {
		return fn(r)
	}
	lockKey := meetingCaptureLaunchLockKey(workspaceID, idempotencyKey)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey).Error; err != nil {
			return fmt.Errorf("lock meeting capture launch: %w", err)
		}
		return fn(NewCRMMeetingRepository(tx))
	})
}

func meetingCaptureLaunchLockKey(workspaceID, idempotencyKey string) string {
	workspaceID = strings.TrimSpace(workspaceID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	return fmt.Sprintf("%d:%s:%s", len(workspaceID), workspaceID, idempotencyKey)
}

// GetCaptureByIdempotencyKey returns an earlier launch response, if any.
func (r *CRMMeetingRepository) GetCaptureByIdempotencyKey(ctx context.Context, workspaceID, key string) (*model.CRMMeetingCapture, error) {
	var capture model.CRMMeetingCapture
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND request_idempotency_key = ?", workspaceID, key).
		First(&capture).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting capture by idempotency key: %w", err)
	}
	return &capture, nil
}

// GetLatestCapture returns the newest attempt for a meeting.
func (r *CRMMeetingRepository) GetLatestCapture(ctx context.Context, workspaceID, meetingID string) (*model.CRMMeetingCapture, error) {
	var capture model.CRMMeetingCapture
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND meeting_id = ?", workspaceID, meetingID).
		Order("created_at DESC").
		First(&capture).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest meeting capture: %w", err)
	}
	return &capture, nil
}

// GetCaptureByProviderID resolves a provider webhook to a capture.
func (r *CRMMeetingRepository) GetCaptureByProviderID(ctx context.Context, provider, providerCaptureID string) (*model.CRMMeetingCapture, error) {
	var capture model.CRMMeetingCapture
	err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_capture_id = ?", provider, providerCaptureID).
		First(&capture).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting capture by provider id: %w", err)
	}
	return &capture, nil
}

// UpdateCapture persists normalized provider state.
func (r *CRMMeetingRepository) UpdateCapture(ctx context.Context, capture *model.CRMMeetingCapture) error {
	if err := r.db.WithContext(ctx).Save(capture).Error; err != nil {
		return fmt.Errorf("update meeting capture: %w", err)
	}
	return nil
}

// CreateProviderEvent inserts an event exactly once and reports whether it was new.
func (r *CRMMeetingRepository) CreateProviderEvent(ctx context.Context, event *model.CRMMeetingProviderEvent) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(event)
	if result.Error != nil {
		return false, fmt.Errorf("create meeting provider event: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// GetProviderEvent returns a previously received event for replay recovery.
func (r *CRMMeetingRepository) GetProviderEvent(ctx context.Context, provider, eventID string) (*model.CRMMeetingProviderEvent, error) {
	var event model.CRMMeetingProviderEvent
	err := r.db.WithContext(ctx).
		Where("provider = ? AND event_id = ?", provider, eventID).
		First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting provider event: %w", err)
	}
	return &event, nil
}

// MarkProviderEventProcessed records successful event application.
func (r *CRMMeetingRepository) MarkProviderEventProcessed(ctx context.Context, eventID string) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMMeetingProviderEvent{}).
		Where("id = ?", eventID).
		Update("processed_at", gorm.Expr("CURRENT_TIMESTAMP")).Error; err != nil {
		return fmt.Errorf("mark meeting provider event processed: %w", err)
	}
	return nil
}

// GetTranscript returns the canonical transcript for a meeting.
func (r *CRMMeetingRepository) GetTranscript(ctx context.Context, workspaceID, meetingID string) (*model.CRMMeetingTranscript, error) {
	var transcript model.CRMMeetingTranscript
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND meeting_id = ?", workspaceID, meetingID).
		First(&transcript).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting transcript: %w", err)
	}
	return &transcript, nil
}

// UpsertTranscript creates or replaces the canonical transcript.
func (r *CRMMeetingRepository) UpsertTranscript(ctx context.Context, transcript *model.CRMMeetingTranscript) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "meeting_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"capture_id", "source_provider", "language", "plain_text", "segments", "checksum", "updated_at",
		}),
	}).Create(transcript).Error; err != nil {
		return fmt.Errorf("upsert meeting transcript: %w", err)
	}
	return nil
}

// GetIntelligence returns the generated meeting intelligence artifact.
func (r *CRMMeetingRepository) GetIntelligence(ctx context.Context, workspaceID, meetingID string) (*model.CRMMeetingIntelligence, error) {
	var intelligence model.CRMMeetingIntelligence
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND meeting_id = ?", workspaceID, meetingID).
		First(&intelligence).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting intelligence: %w", err)
	}
	return &intelligence, nil
}

// UpsertIntelligence persists deterministic generation output.
func (r *CRMMeetingRepository) UpsertIntelligence(ctx context.Context, intelligence *model.CRMMeetingIntelligence) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "meeting_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"generation_version", "transcript_checksum", "summary_markdown", "key_points",
			"decisions", "objections", "risks", "next_steps", "follow_up_draft", "updated_at",
		}),
	}).Create(intelligence).Error; err != nil {
		return fmt.Errorf("upsert meeting intelligence: %w", err)
	}
	return nil
}

// ReplaceActionItems atomically replaces generated action items that have not been reviewed.
func (r *CRMMeetingRepository) ReplaceActionItems(ctx context.Context, workspaceID, meetingID string, items []model.CRMMeetingActionItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND meeting_id = ? AND status = ?", workspaceID, meetingID, model.CRMMeetingActionPending).
			Delete(&model.CRMMeetingActionItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
		return nil
	})
}

// ListActionItems lists generated actions in transcript order.
func (r *CRMMeetingRepository) ListActionItems(ctx context.Context, workspaceID, meetingID string) ([]model.CRMMeetingActionItem, error) {
	var items []model.CRMMeetingActionItem
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND meeting_id = ?", workspaceID, meetingID).
		Order("position ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list meeting action items: %w", err)
	}
	return items, nil
}

// GetActionItem returns one workspace- and meeting-scoped action item.
func (r *CRMMeetingRepository) GetActionItem(ctx context.Context, workspaceID, meetingID, itemID string) (*model.CRMMeetingActionItem, error) {
	var item model.CRMMeetingActionItem
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND meeting_id = ? AND id = ?", workspaceID, meetingID, itemID).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting action item: %w", err)
	}
	return &item, nil
}

// UpdateActionItem persists review state and linked task.
func (r *CRMMeetingRepository) UpdateActionItem(ctx context.Context, item *model.CRMMeetingActionItem) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("update meeting action item: %w", err)
	}
	return nil
}

// GetSettings returns workspace settings with safe defaults when absent.
func (r *CRMMeetingRepository) GetSettings(ctx context.Context, workspaceID string) (*model.CRMMeetingSettings, error) {
	var settings model.CRMMeetingSettings
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultMeetingSettings(workspaceID), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting settings: %w", err)
	}
	return &settings, nil
}

// UpsertSettings saves workspace policy.
func (r *CRMMeetingRepository) UpsertSettings(ctx context.Context, settings *model.CRMMeetingSettings) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}},
		UpdateAll: true,
	}).Create(settings).Error; err != nil {
		return fmt.Errorf("upsert meeting settings: %w", err)
	}
	return nil
}

func applyMeetingAssociationFilter(query *gorm.DB, objectType string, objectID *string) *gorm.DB {
	if objectID == nil || strings.TrimSpace(*objectID) == "" {
		return query
	}
	return query.Where(`EXISTS (
		SELECT 1 FROM crm_associations a
		WHERE a.workspace_id = crm_meetings.workspace_id
		AND ((a.from_object_type = ? AND a.from_object_id = crm_meetings.id AND a.to_object_type = ? AND a.to_object_id = ?)
		OR (a.to_object_type = ? AND a.to_object_id = crm_meetings.id AND a.from_object_type = ? AND a.from_object_id = ?))
	)`, model.CRMObjectMeeting, objectType, strings.TrimSpace(*objectID), model.CRMObjectMeeting, objectType, strings.TrimSpace(*objectID))
}

func normalizedPagination(pagination model.PMPagination) (int, int) {
	page := pagination.Page
	if page < 1 {
		page = 1
	}
	perPage := pagination.PerPage
	if perPage < 1 {
		perPage = 25
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

func defaultMeetingSettings(workspaceID string) *model.CRMMeetingSettings {
	return &model.CRMMeetingSettings{
		WorkspaceID:             workspaceID,
		Enabled:                 false,
		DefaultProvider:         model.CRMMeetingProviderRecall,
		BotName:                 "Helpin Notetaker",
		AutoJoinMode:            "manual",
		RecordAudioByDefault:    false,
		DefaultVisibility:       model.CRMMeetingVisibilityWorkspace,
		TranscriptRetentionDays: 365,
		AudioRetentionDays:      30,
	}
}
