package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// HelpAssistantRepository stores help assistant sessions and messages.
type HelpAssistantRepository struct {
	db *gorm.DB
}

// NewHelpAssistantRepository creates a HelpAssistantRepository.
func NewHelpAssistantRepository(db *gorm.DB) *HelpAssistantRepository {
	return &HelpAssistantRepository{db: db}
}

// CreateSession stores a new session.
func (r *HelpAssistantRepository) CreateSession(ctx context.Context, session *model.HelpAssistantSession) error {
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("create help assistant session: %w", err)
	}
	return nil
}

// GetSession returns a workspace's session, or nil when there is none.
func (r *HelpAssistantRepository) GetSession(ctx context.Context, workspaceID, sessionID string) (*model.HelpAssistantSession, error) {
	var session model.HelpAssistantSession
	err := r.db.WithContext(ctx).Where("id = ? AND workspace_id = ?", sessionID, workspaceID).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get help assistant session: %w", err)
	}
	return &session, nil
}

// StartTurn stores a question and its pending answer and counts the turn.
// It fails with ErrHelpAssistantTurnPending when the session already waits
// for an answer, so one session never runs two turns at once.
func (r *HelpAssistantRepository) StartTurn(ctx context.Context, session *model.HelpAssistantSession, question, answer *model.HelpAssistantMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the session row so concurrent requests for one session serialize.
		update := tx.Model(&model.HelpAssistantSession{}).Where("id = ?", session.ID).
			Updates(map[string]any{"turn_count": gorm.Expr("turn_count + 1"), "updated_at": time.Now()})
		if update.Error != nil {
			return fmt.Errorf("count help assistant turn: %w", update.Error)
		}
		var pending int64
		if err := tx.Model(&model.HelpAssistantMessage{}).
			Where("session_id = ? AND status = ?", session.ID, model.HelpAssistantStatusPending).
			Count(&pending).Error; err != nil {
			return fmt.Errorf("check pending help assistant turn: %w", err)
		}
		if pending > 0 {
			return ErrHelpAssistantTurnPending
		}
		if err := tx.Create(question).Error; err != nil {
			return fmt.Errorf("store help assistant question: %w", err)
		}
		if err := tx.Create(answer).Error; err != nil {
			return fmt.Errorf("store help assistant answer: %w", err)
		}
		session.TurnCount++
		return nil
	})
}

// ErrHelpAssistantTurnPending reports a session that still waits for an answer.
var ErrHelpAssistantTurnPending = errors.New("help assistant turn already pending")

// ListMessages returns a session's messages, oldest first.
func (r *HelpAssistantRepository) ListMessages(ctx context.Context, sessionID string) ([]model.HelpAssistantMessage, error) {
	var messages []model.HelpAssistantMessage
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Order("created_at ASC, role DESC").Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list help assistant messages: %w", err)
	}
	return messages, nil
}

// GetMessage returns a workspace's message, or nil when there is none.
func (r *HelpAssistantRepository) GetMessage(ctx context.Context, workspaceID, messageID string) (*model.HelpAssistantMessage, error) {
	var message model.HelpAssistantMessage
	err := r.db.WithContext(ctx).Where("id = ? AND workspace_id = ?", messageID, workspaceID).First(&message).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get help assistant message: %w", err)
	}
	return &message, nil
}

// CountPending counts a workspace's answers still being written that started
// after since.
func (r *HelpAssistantRepository) CountPending(ctx context.Context, workspaceID string, since time.Time) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.HelpAssistantMessage{}).
		Where("workspace_id = ? AND status = ? AND created_at > ?", workspaceID, model.HelpAssistantStatusPending, since).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count pending help assistant turns: %w", err)
	}
	return count, nil
}

// CountQuestionsSince counts a workspace's questions asked after since.
func (r *HelpAssistantRepository) CountQuestionsSince(ctx context.Context, workspaceID string, since time.Time) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.HelpAssistantMessage{}).
		Where("workspace_id = ? AND role = ? AND created_at > ?", workspaceID, model.HelpAssistantRoleUser, since).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count help assistant questions: %w", err)
	}
	return count, nil
}

// SetRunID records the run answering a pending message.
func (r *HelpAssistantRepository) SetRunID(ctx context.Context, messageID, runID string) error {
	if err := r.db.WithContext(ctx).Model(&model.HelpAssistantMessage{}).Where("id = ?", messageID).
		Update("run_id", runID).Error; err != nil {
		return fmt.Errorf("set help assistant run: %w", err)
	}
	return nil
}

// SetProgress updates the progress label of a pending answer.
func (r *HelpAssistantRepository) SetProgress(ctx context.Context, messageID, label string) error {
	if err := r.db.WithContext(ctx).Model(&model.HelpAssistantMessage{}).
		Where("id = ? AND status = ?", messageID, model.HelpAssistantStatusPending).
		Update("progress_label", label).Error; err != nil {
		return fmt.Errorf("set help assistant progress: %w", err)
	}
	return nil
}

// Complete finishes a pending answer. It reports false when the answer was
// no longer pending, for example after it timed out.
func (r *HelpAssistantRepository) Complete(ctx context.Context, messageID, status, content string, citations []model.HelpAssistantCitation) (bool, error) {
	updates := map[string]any{"status": status, "content": content, "progress_label": nil, "completed_at": time.Now()}
	if len(citations) > 0 {
		encoded, err := json.Marshal(citations)
		if err != nil {
			return false, err
		}
		// Text, not bytes: Postgres reads a byte parameter as bytea, not jsonb.
		updates["citations"] = string(encoded)
	}
	result := r.db.WithContext(ctx).Model(&model.HelpAssistantMessage{}).
		Where("id = ? AND status = ?", messageID, model.HelpAssistantStatusPending).Updates(updates)
	if result.Error != nil {
		return false, fmt.Errorf("complete help assistant answer: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// SetFeedback records whether an answer helped. Only finished answers in the
// session take feedback.
func (r *HelpAssistantRepository) SetFeedback(ctx context.Context, sessionID, messageID string, helpful bool) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.HelpAssistantMessage{}).
		Where("id = ? AND session_id = ? AND role = ? AND status IN ?", messageID, sessionID, model.HelpAssistantRoleAssistant,
			[]string{model.HelpAssistantStatusAnswered, model.HelpAssistantStatusHandedOff}).
		Update("helpful", helpful)
	if result.Error != nil {
		return false, fmt.Errorf("set help assistant feedback: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}
