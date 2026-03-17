package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// PlanningSessionRepository manages planning session persistence.
type PlanningSessionRepository struct {
	db *gorm.DB
}

// NewPlanningSessionRepository creates a new repository.
func NewPlanningSessionRepository(db *gorm.DB) *PlanningSessionRepository {
	return &PlanningSessionRepository{db: db}
}

// Create persists a new planning session.
func (r *PlanningSessionRepository) Create(ctx context.Context, session *model.PlanningSession) (*model.PlanningSession, error) {
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return nil, fmt.Errorf("create planning session: %w", err)
	}
	return session, nil
}

// GetByID loads a planning session by ID.
func (r *PlanningSessionRepository) GetByID(ctx context.Context, id string) (*model.PlanningSession, error) {
	var session model.PlanningSession
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get planning session: %w", err)
	}
	return &session, nil
}

// GetActiveByEpicID loads the active planning session for an epic.
func (r *PlanningSessionRepository) GetActiveByEpicID(ctx context.Context, epicID string) (*model.PlanningSession, error) {
	var session model.PlanningSession
	err := r.db.WithContext(ctx).
		Where("epic_id = ? AND status IN ?", epicID, []string{
			model.PlanningSessionStatusActive,
			model.PlanningSessionStatusPaused,
		}).
		First(&session).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get active planning session: %w", err)
	}
	return &session, nil
}

// Update saves changes to a planning session.
func (r *PlanningSessionRepository) Update(ctx context.Context, session *model.PlanningSession) error {
	if err := r.db.WithContext(ctx).Save(session).Error; err != nil {
		return fmt.Errorf("update planning session: %w", err)
	}
	return nil
}

// UpdateLastActive touches the last_active_at timestamp.
func (r *PlanningSessionRepository) UpdateLastActive(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).
		Model(&model.PlanningSession{}).
		Where("id = ?", id).
		Update("last_active_at", time.Now()).Error
	if err != nil {
		return fmt.Errorf("update last active: %w", err)
	}
	return nil
}

// CreateMessage persists a new message in a planning session.
func (r *PlanningSessionRepository) CreateMessage(ctx context.Context, msg *model.PlanningSessionMessage) (*model.PlanningSessionMessage, error) {
	if err := r.db.WithContext(ctx).Create(msg).Error; err != nil {
		return nil, fmt.Errorf("create planning session message: %w", err)
	}
	return msg, nil
}

// ListMessages returns all messages for a session ordered by creation time.
func (r *PlanningSessionRepository) ListMessages(ctx context.Context, sessionID string) ([]model.PlanningSessionMessage, error) {
	var messages []model.PlanningSessionMessage
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("list planning session messages: %w", err)
	}
	return messages, nil
}

// ListStale returns sessions that have been inactive longer than the given threshold.
func (r *PlanningSessionRepository) ListStale(ctx context.Context, workspaceID string, staleSince time.Time) ([]model.PlanningSession, error) {
	var sessions []model.PlanningSession
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND status = ? AND last_active_at < ?",
			workspaceID, model.PlanningSessionStatusActive, staleSince).
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("list stale planning sessions: %w", err)
	}
	return sessions, nil
}
