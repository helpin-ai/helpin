package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportImportJobNotFound means the job does not exist in the workspace.
var ErrSupportImportJobNotFound = errors.New("import not found")

// ErrSupportAlreadyImported means the source conversation was imported before.
var ErrSupportAlreadyImported = errors.New("conversation already imported")

// SupportImportRepository stores conversation import jobs and what they
// imported.
type SupportImportRepository struct {
	db *gorm.DB
}

func NewSupportImportRepository(db *gorm.DB) *SupportImportRepository {
	return &SupportImportRepository{db: db}
}

// CreateJob inserts a job.
func (r *SupportImportRepository) CreateJob(ctx context.Context, job *model.SupportImportJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("create support import job: %w", err)
	}
	return nil
}

// ListJobs returns the workspace's most recent jobs.
func (r *SupportImportRepository) ListJobs(ctx context.Context, workspaceID string) ([]model.SupportImportJob, error) {
	var jobs []model.SupportImportJob
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at DESC").Limit(20).Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("list support import jobs: %w", err)
	}
	return jobs, nil
}

// GetJob returns one job.
func (r *SupportImportRepository) GetJob(ctx context.Context, workspaceID, jobID string) (*model.SupportImportJob, error) {
	var job model.SupportImportJob
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, jobID).Take(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSupportImportJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get support import job: %w", err)
	}
	return &job, nil
}

// HasActiveJob reports whether the workspace already runs an import from source.
func (r *SupportImportRepository) HasActiveJob(ctx context.Context, workspaceID, source string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SupportImportJob{}).
		Where("workspace_id = ? AND source = ? AND status IN ?", workspaceID, source, []string{model.SupportImportStatusPending, model.SupportImportStatusRunning}).
		Count(&count).Error
	return count > 0, err
}

// SaveLeased writes fields of a job while the worker still holds its lease.
// It reports false when the lease was lost to another worker.
func (r *SupportImportRepository) SaveLeased(ctx context.Context, jobID, leaseToken string, fields map[string]any) (bool, error) {
	fields["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.SupportImportJob{}).Where("id = ? AND lease_token = ?", jobID, leaseToken).Updates(fields)
	if result.Error != nil {
		return false, fmt.Errorf("update support import job: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// ExtendLease keeps a running job leased to the worker. It reports false when
// the job was cancelled or another worker took it over.
func (r *SupportImportRepository) ExtendLease(ctx context.Context, jobID, leaseToken string, until time.Time) (bool, error) {
	return r.updateRunning(ctx, jobID, leaseToken, map[string]any{"locked_until": until})
}

// FinishJob ends a running job and deletes its stored credentials. A job
// cancelled meanwhile stays cancelled.
func (r *SupportImportRepository) FinishJob(ctx context.Context, jobID, leaseToken, status, message string, at time.Time) (bool, error) {
	return r.updateRunning(ctx, jobID, leaseToken, map[string]any{
		"status": status, "error": message, "completed_at": at, "locked_until": nil, "payload_encrypted": "",
	})
}

func (r *SupportImportRepository) updateRunning(ctx context.Context, jobID, leaseToken string, fields map[string]any) (bool, error) {
	fields["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.SupportImportJob{}).
		Where("id = ? AND lease_token = ? AND status = ?", jobID, leaseToken, model.SupportImportStatusRunning).Updates(fields)
	if result.Error != nil {
		return false, fmt.Errorf("update support import job: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// CancelJob stops a pending or running job and deletes its stored
// credentials. A page in progress still saves its counts.
func (r *SupportImportRepository) CancelJob(ctx context.Context, workspaceID, jobID string, at time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.SupportImportJob{}).
		Where("workspace_id = ? AND id = ? AND status IN ?", workspaceID, jobID, []string{model.SupportImportStatusPending, model.SupportImportStatusRunning}).
		Updates(map[string]any{"status": model.SupportImportStatusCancelled, "completed_at": at, "locked_until": nil, "payload_encrypted": "", "updated_at": at})
	if result.Error != nil {
		return fmt.Errorf("cancel support import job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSupportImportJobNotFound
	}
	return nil
}

// ClaimJob leases the waiting job that was worked on least recently, so
// imports in different workspaces take turns page by page.
func (r *SupportImportRepository) ClaimJob(ctx context.Context, now, leaseUntil time.Time, leaseToken string) (*model.SupportImportJob, error) {
	var claimed *model.SupportImportJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("status IN ? AND (locked_until IS NULL OR locked_until < ?)",
			[]string{model.SupportImportStatusPending, model.SupportImportStatusRunning}, now).Order("updated_at, created_at")
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var job model.SupportImportJob
		err := query.Take(&job).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&model.SupportImportJob{}).Where("id = ?", job.ID).Updates(map[string]any{
			"status": model.SupportImportStatusRunning, "locked_until": leaseUntil, "lease_token": leaseToken, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		job.Status, job.LockedUntil, job.LeaseToken = model.SupportImportStatusRunning, &leaseUntil, leaseToken
		claimed = &job
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim support import job: %w", err)
	}
	return claimed, nil
}

// ImportedIDs returns which of the source IDs were imported already, and by
// which job.
func (r *SupportImportRepository) ImportedIDs(ctx context.Context, workspaceID, source string, externalIDs []string) (map[string]string, error) {
	result := map[string]string{}
	if len(externalIDs) == 0 {
		return result, nil
	}
	var rows []model.SupportImportRecord
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND source = ? AND external_id IN ?", workspaceID, source, externalIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list imported conversations: %w", err)
	}
	for _, row := range rows {
		result[row.ExternalID] = derefRepositoryString(row.JobID)
	}
	return result, nil
}

func derefRepositoryString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// ImportConversation stores one imported conversation with its messages and
// tags in one transaction. It returns ErrSupportAlreadyImported when the
// source conversation was imported before.
func (r *SupportImportRepository) ImportConversation(ctx context.Context, record model.SupportImportRecord, conversation *model.SupportConversation, messages []model.SupportMessage, tagIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
		if inserted.Error != nil {
			return fmt.Errorf("record imported conversation: %w", inserted.Error)
		}
		if inserted.RowsAffected == 0 {
			return ErrSupportAlreadyImported
		}
		if tx.Dialector.Name() == "postgres" {
			// Serialize display numbers with the inbox's own creation path.
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", conversation.WorkspaceID).Error; err != nil {
				return fmt.Errorf("lock display number: %w", err)
			}
		}
		var maxDisplayID int
		if err := tx.Model(&model.SupportConversation{}).Where("workspace_id = ?", conversation.WorkspaceID).
			Select("COALESCE(MAX(display_id), 0)").Scan(&maxDisplayID).Error; err != nil {
			return fmt.Errorf("next display number: %w", err)
		}
		conversation.DisplayID = maxDisplayID + 1
		if err := tx.Create(conversation).Error; err != nil {
			return fmt.Errorf("create imported conversation: %w", err)
		}
		if len(messages) > 0 {
			if err := tx.CreateInBatches(messages, 200).Error; err != nil {
				return fmt.Errorf("create imported messages: %w", err)
			}
			// Message triggers add each message to the projection the
			// conversation was created with; restore the computed state.
			if err := tx.Model(&model.SupportConversation{}).Where("id = ?", conversation.ID).UpdateColumns(map[string]any{
				"unanswered_customer_message_count": conversation.UnansweredCustomerMessageCount,
				"customer_awaiting_response":        conversation.CustomerAwaitingResponse,
				"needs_human_reply":                 conversation.NeedsHumanReply,
			}).Error; err != nil {
				return fmt.Errorf("set imported conversation state: %w", err)
			}
		}
		for _, tagID := range tagIDs {
			link := model.SupportConversationTag{ConversationID: conversation.ID, TagID: tagID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
				return fmt.Errorf("tag imported conversation: %w", err)
			}
		}
		return nil
	})
}
