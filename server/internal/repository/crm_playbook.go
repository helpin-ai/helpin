package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	// ErrCRMPlaybookStale rejects edits based on an old configuration or publication.
	ErrCRMPlaybookStale = errors.New("playbook revision changed")
	// ErrCRMPlaybookConflict prevents request-key reuse or duplicate process attachment.
	ErrCRMPlaybookConflict = errors.New("playbook request conflicts")
	// ErrCRMPlaybookUnavailable rejects unavailable policy, enrollment or eligibility.
	ErrCRMPlaybookUnavailable = errors.New("playbook is unavailable for this operation")
)

// CRMPlaybookRepository persists business configuration, never Flow or Agent execution.
type CRMPlaybookRepository struct{ db *gorm.DB }

// NewCRMPlaybookRepository binds tenant-scoped CRM storage.
func NewCRMPlaybookRepository(db *gorm.DB) *CRMPlaybookRepository {
	return &CRMPlaybookRepository{db: db}
}

// Create records a draft and its initial receipt atomically. Retries return that original snapshot.
func (r *CRMPlaybookRepository) Create(ctx context.Context, input model.CRMPlaybook) (*model.CRMPlaybook, bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, input.WorkspaceID); err != nil {
			return err
		}
		if err := situationSourceExists(tx, "workspace_members", input.WorkspaceID, input.CreatedByMemberID); err != nil {
			return err
		}
		var existing model.CRMPlaybook
		err := tx.Where("workspace_id = ? AND creation_key = ?", input.WorkspaceID, input.CreationKey).Take(&existing).Error
		if err == nil {
			if existing.CreationFingerprint != input.CreationFingerprint {
				return ErrCRMPlaybookConflict
			}
			var receipt model.CRMPlaybookChange
			if err := tx.Where("workspace_id = ? AND playbook_id = ? AND revision = 1", input.WorkspaceID, existing.ID).Take(&receipt).Error; err != nil {
				return err
			}
			input = receipt.After
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := validatePlaybookReferences(tx, input.WorkspaceID, input.Draft); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Returning{}).Create(&input).Error; err != nil {
			return err
		}
		if err := tx.Create(newPlaybookChange(input, "system:created", input.CreationFingerprint, "created", "")).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &input, created, nil
}

// Command atomically changes a draft, publishes a snapshot, or controls new participation.
func (r *CRMPlaybookRepository) Command(ctx context.Context, ws, id, actor string, req model.CRMPlaybookCommandRequest, fingerprint string,
	validate func(model.CRMPlaybookDefinition) (model.CRMPlaybookDefinition, error),
) (*model.CRMPlaybookCommandResult, error) {
	var result *model.CRMPlaybookCommandResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		if err := situationSourceExists(tx, "workspace_members", ws, actor); err != nil {
			return err
		}
		var current model.CRMPlaybook
		err := tx.Where("workspace_id = ? AND id = ?", ws, id).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var receipt model.CRMPlaybookChange
		err = tx.Where("workspace_id = ? AND playbook_id = ? AND command_key = ?", ws, id, req.CommandKey).Take(&receipt).Error
		if err == nil {
			if receipt.CommandFingerprint != fingerprint {
				return ErrCRMPlaybookConflict
			}
			result = &model.CRMPlaybookCommandResult{Change: receipt, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if current.Revision != req.ExpectedRevision {
			return ErrCRMPlaybookStale
		}
		current.Revision++
		current.UpdatedByMemberID, current.UpdatedAt = actor, time.Now().UTC().Truncate(time.Microsecond)
		switch req.Operation {
		case "update_draft":
			if req.Definition == nil {
				return ErrCRMPlaybookUnavailable
			}
			current.Draft = *req.Definition
			if err := validatePlaybookReferences(tx, ws, current.Draft); err != nil {
				return err
			}
		case "publish":
			definition, err := validate(current.Draft)
			if err != nil {
				return err
			}
			if err := validatePlaybookReferences(tx, ws, definition); err != nil {
				return err
			}
			version, err := publishPlaybook(tx, current, definition)
			if err != nil {
				return err
			}
			current.PublishedVersionID = &version.ID
		case "set_enrollment":
			if req.AcceptingCustomers == nil || (current.PublishedVersionID == nil && *req.AcceptingCustomers) {
				return ErrCRMPlaybookUnavailable
			}
			if *req.AcceptingCustomers {
				var version model.CRMPlaybookVersion
				if err := tx.Where("workspace_id = ? AND playbook_id = ? AND id = ?", ws, id, *current.PublishedVersionID).Take(&version).Error; err != nil {
					return err
				}
				if _, err := validate(version.Definition); err != nil {
					return err
				}
				if err := validatePlaybookReferences(tx, ws, version.Definition); err != nil {
					return err
				}
			}
			current.AcceptingCustomers = *req.AcceptingCustomers
		default:
			return ErrCRMPlaybookUnavailable
		}
		encoded, err := json.Marshal(current.Draft)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.CRMPlaybook{}).Where("workspace_id = ? AND id = ?", ws, id).Updates(map[string]any{
			"draft": string(encoded), "revision": current.Revision, "published_version_id": current.PublishedVersionID,
			"accepting_customers": current.AcceptingCustomers, "updated_by_member_id": actor, "updated_at": current.UpdatedAt,
		}).Error; err != nil {
			return err
		}
		receipt = *newPlaybookChange(current, req.CommandKey, fingerprint, req.Operation, req.Reason)
		if err := tx.Clauses(clause.Returning{}).Create(&receipt).Error; err != nil {
			return err
		}
		result = &model.CRMPlaybookCommandResult{Change: receipt}
		return nil
	})
	return result, err
}

func newPlaybookChange(pb model.CRMPlaybook, key, fingerprint, operation, reason string) *model.CRMPlaybookChange {
	pb.CreationKey, pb.CreationFingerprint = "", ""
	return &model.CRMPlaybookChange{ID: uuid.NewString(), WorkspaceID: pb.WorkspaceID, PlaybookID: pb.ID, Revision: pb.Revision,
		CommandKey: key, CommandFingerprint: fingerprint, Operation: operation, ActorMemberID: pb.UpdatedByMemberID,
		Reason: reason, After: pb, CreatedAt: pb.UpdatedAt}
}

func publishPlaybook(tx *gorm.DB, pb model.CRMPlaybook, definition model.CRMPlaybookDefinition) (*model.CRMPlaybookVersion, error) {
	var last int64
	if err := tx.Model(&model.CRMPlaybookVersion{}).Select("COALESCE(MAX(version), 0)").Where("workspace_id = ? AND playbook_id = ?", pb.WorkspaceID, pb.ID).Scan(&last).Error; err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, err
	}
	version := model.CRMPlaybookVersion{ID: uuid.NewString(), WorkspaceID: pb.WorkspaceID, PlaybookID: pb.ID,
		Version: last + 1, Definition: definition, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(encoded)), PublishedByMemberID: pb.UpdatedByMemberID, PublishedAt: pb.UpdatedAt}
	if err := tx.Create(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}
