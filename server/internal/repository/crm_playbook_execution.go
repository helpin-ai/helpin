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

// ErrCRMPlaybookExecutionBlocked means current customer work cannot authorize execution.
var ErrCRMPlaybookExecutionBlocked = errors.New("Playbook automation needs attention")

// CRMPlaybookExecutionRepository persists live gates and normal-run correlation.
type CRMPlaybookExecutionRepository struct{ db *gorm.DB }

// NewCRMPlaybookExecutionRepository shares CRM's workspace lock and Automation outbox.
func NewCRMPlaybookExecutionRepository(db *gorm.DB) *CRMPlaybookExecutionRepository {
	return &CRMPlaybookExecutionRepository{db: db}
}

// Settings returns the live gate without loading private instructions.
func (r *CRMPlaybookExecutionRepository) Settings(ctx context.Context, ws, id string) (*model.CRMPlaybookAutomationSettings, error) {
	var result model.CRMPlaybookAutomationSettings
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND playbook_id = ?", ws, id).Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &result, err
}

// Binding returns only the execution link; CRM still owns the customer lifecycle.
func (r *CRMPlaybookExecutionRepository) Binding(ctx context.Context, ws, id string) (*model.CRMPlaybookAutomationBinding, error) {
	var result model.CRMPlaybookAutomationBinding
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND situation_id = ?", ws, id).Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &result, err
}

// Configure changes the live gate with an actor-bound optimistic receipt.
// Disabling pauses bound work; enabling never implicitly resumes old Signals.
func (r *CRMPlaybookExecutionRepository) Configure(ctx context.Context, ws, id, actor string,
	req model.CRMPlaybookAutomationCommand, fingerprint string, now time.Time,
) (*model.CRMPlaybookAutomationReceipt, error) {
	var result *model.CRMPlaybookAutomationReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		if err := requirePlaybookExecutionMember(tx, ws, actor); err != nil {
			return err
		}
		previous, err := executionReceipt(tx, ws, id, req.CommandKey, fingerprint)
		if err != nil || previous != nil {
			result = previous
			return err
		}
		connection, err := NewCRMPlaybookRepository(tx).Connection(ctx, ws, id, req.ConnectionID)
		if err != nil {
			return err
		}
		if connection == nil {
			return ErrCRMPlaybookUnavailable
		}
		current, err := NewCRMPlaybookExecutionRepository(tx).Settings(ctx, ws, id)
		if err != nil {
			return err
		}
		revision := int64(0)
		if current != nil {
			revision = current.Revision
		}
		if revision != req.ExpectedRevision {
			return ErrCRMPlaybookStale
		}
		settings := model.CRMPlaybookAutomationSettings{WorkspaceID: ws, PlaybookID: id, ConnectionID: connection.ID,
			Revision: revision + 1, Enabled: req.Enabled, EntryMode: req.EntryMode, MaxRunsPerDay: req.MaxRunsPerDay,
			MaxNoProgressRuns: req.MaxNoProgressRuns, AuthorizedByMemberID: actor, UpdatedAt: now}
		if req.Enabled && req.EntryMode == "automatic" {
			settings.AutomaticSince = &now
			if current != nil && current.Enabled && current.EntryMode == "automatic" && current.ConnectionID == connection.ID && current.AutomaticSince != nil {
				settings.AutomaticSince = current.AutomaticSince
			}
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "playbook_id"}}, UpdateAll: true}).Create(&settings).Error; err != nil {
			return err
		}
		if !req.Enabled {
			var bindings []model.CRMPlaybookAutomationBinding
			if err := tx.Where("workspace_id = ? AND playbook_id = ? AND enabled = ?", ws, id, true).Find(&bindings).Error; err != nil {
				return err
			}
			for _, binding := range bindings {
				if err := pausePlaybookBinding(tx, binding, "automation_paused", now); err != nil {
					return err
				}
			}
		}
		result = &model.CRMPlaybookAutomationReceipt{WorkspaceID: ws, SubjectID: id, CommandKey: req.CommandKey, Fingerprint: fingerprint, Settings: &settings, CreatedAt: now}
		return tx.Create(result).Error
	})
	return result, err
}

// Adopt explicitly selects the approved setup for one existing canonical Signal.
func (r *CRMPlaybookExecutionRepository) Adopt(ctx context.Context, ws, id, actor string,
	req model.CRMPlaybookAutomationAdoption, fingerprint string, now time.Time,
) (*model.CRMPlaybookAutomationReceipt, error) {
	var result *model.CRMPlaybookAutomationReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		if err := requirePlaybookExecutionMember(tx, ws, actor); err != nil {
			return err
		}
		previous, err := executionReceipt(tx, ws, id, req.CommandKey, fingerprint)
		if err != nil || previous != nil {
			result = previous
			return err
		}
		var situation model.CRMSituation
		if err := tx.Where("workspace_id = ? AND id = ?", ws, id).Take(&situation).Error; err != nil {
			return err
		}
		if situation.Revision != req.ExpectedRevision || situation.PlaybookID == nil || situation.PlaybookVersionID == nil {
			return ErrCRMPlaybookStale
		}
		store := NewCRMPlaybookExecutionRepository(tx)
		current, err := store.Binding(ctx, ws, id)
		if err != nil {
			return err
		}
		generation := int64(0)
		if current != nil {
			generation = current.Generation
		}
		if generation != req.ExpectedGeneration {
			return ErrCRMPlaybookStale
		}
		connection, err := NewCRMPlaybookRepository(tx).Connection(ctx, ws, *situation.PlaybookID, req.ConnectionID)
		if err != nil {
			return err
		}
		if connection == nil || connection.PlaybookVersionID != *situation.PlaybookVersionID {
			return ErrCRMPlaybookUnavailable
		}
		settings, err := store.Settings(ctx, ws, *situation.PlaybookID)
		if err != nil {
			return err
		}
		if req.Enabled && (settings == nil || !settings.Enabled || situation.Lifecycle != model.CRMSituationOpen) {
			return ErrCRMPlaybookExecutionBlocked
		}
		binding := model.CRMPlaybookAutomationBinding{WorkspaceID: ws, SituationID: id, PlaybookID: *situation.PlaybookID,
			ConnectionID: connection.ID, Enabled: req.Enabled, Generation: generation + 1, AuthorizedByMemberID: actor, UpdatedAt: now, ProgressObservedAt: &now}
		if current != nil {
			binding.LastRunID = current.LastRunID
			binding.LastCheckedAt = current.LastCheckedAt
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "situation_id"}}, UpdateAll: true}).Create(&binding).Error; err != nil {
			return err
		}
		if err := supersedePlaybookActions(tx, ws, id, now); err != nil {
			return err
		}
		if err := NewAutomationScheduledEventRepository(tx).CancelTarget(ctx, ws, model.CRMPlaybookWorkDue, "crm_situation", id, now); err != nil {
			return err
		}
		if req.Enabled {
			if err := enqueuePlaybookWork(tx, binding, "start", now); err != nil {
				return err
			}
		}
		result = &model.CRMPlaybookAutomationReceipt{WorkspaceID: ws, SubjectID: id, CommandKey: req.CommandKey, Fingerprint: fingerprint, Binding: &binding, CreatedAt: now}
		return tx.Create(result).Error
	})
	return result, err
}

// Source reads the exact pinned setup together with current CRM facts.
func (r *CRMPlaybookExecutionRepository) Source(ctx context.Context, ws, id string) (*model.CRMPlaybookExecutionSource, error) {
	var source *model.CRMPlaybookExecutionSource
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		source, err = playbookExecutionSource(tx, ws, id)
		return err
	})
	return source, err
}

func playbookExecutionSource(tx *gorm.DB, ws, id string) (*model.CRMPlaybookExecutionSource, error) {
	ctx := tx.Statement.Context
	store := NewCRMPlaybookExecutionRepository(tx)
	binding, err := store.Binding(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, nil
	}
	settings, err := store.Settings(ctx, ws, binding.PlaybookID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, ErrCRMPlaybookExecutionBlocked
	}
	connection, err := NewCRMPlaybookRepository(tx).Connection(ctx, ws, binding.PlaybookID, binding.ConnectionID)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, ErrCRMPlaybookUnavailable
	}
	var policy model.CRMPlaybookVersion
	if err := tx.Where("workspace_id = ? AND playbook_id = ? AND id = ?", ws, binding.PlaybookID, connection.PlaybookVersionID).Take(&policy).Error; err != nil {
		return nil, err
	}
	item, err := NewCRMSituationRepository(tx).GetByID(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	source := &model.CRMPlaybookExecutionSource{Settings: *settings, Binding: *binding, Connection: *connection, Policy: policy, Item: *item}
	source.StopReason, err = playbookStopReason(tx, *source)
	return source, err
}

// Receipt checks exact actor-bound retries before current configuration freshness.
func (r *CRMPlaybookExecutionRepository) Receipt(ctx context.Context, ws, id, key, fingerprint string) (*model.CRMPlaybookAutomationReceipt, error) {
	return executionReceipt(r.db.WithContext(ctx), ws, id, key, fingerprint)
}

func executionReceipt(tx *gorm.DB, ws, id, key, fingerprint string) (*model.CRMPlaybookAutomationReceipt, error) {
	var receipt model.CRMPlaybookAutomationReceipt
	err := tx.Where("workspace_id = ? AND subject_id = ? AND command_key = ?", ws, id, key).Take(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if receipt.Fingerprint != fingerprint {
		return nil, ErrCRMPlaybookConflict
	}
	return &receipt, nil
}

func requirePlaybookExecutionMember(tx *gorm.DB, ws, id string) error {
	var count int64
	if err := tx.Model(&model.WorkspaceMember{}).Where("workspace_id = ? AND id = ? AND status = ?", ws, id, model.WorkspaceMemberStatusActive).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return ErrCRMPlaybookUnavailable
	}
	return nil
}

func pausePlaybookBinding(tx *gorm.DB, binding model.CRMPlaybookAutomationBinding, reason string, now time.Time) error {
	if err := supersedePlaybookActions(tx, binding.WorkspaceID, binding.SituationID, now); err != nil {
		return err
	}
	if err := tx.Model(&model.CRMPlaybookAutomationBinding{}).Where("workspace_id = ? AND situation_id = ?", binding.WorkspaceID, binding.SituationID).
		Updates(map[string]any{"enabled": false, "generation": binding.Generation + 1, "blocker": reason, "updated_at": now}).Error; err != nil {
		return err
	}
	return NewAutomationScheduledEventRepository(tx).CancelTarget(tx.Statement.Context, binding.WorkspaceID, model.CRMPlaybookWorkDue, "crm_situation", binding.SituationID, now)
}

func enqueuePlaybookWork(tx *gorm.DB, binding model.CRMPlaybookAutomationBinding, reason string, due time.Time) error {
	return NewAutomationScheduledEventRepository(tx).Enqueue(tx.Statement.Context, model.AutomationScheduledEvent{
		WorkspaceID: binding.WorkspaceID, Kind: model.CRMPlaybookWorkDue, TargetType: "crm_situation", TargetID: binding.SituationID,
		EventKey: fmt.Sprintf("crm.playbook:%s:%d:%s", binding.SituationID, binding.Generation, reason), ExpectedRevision: binding.Generation, DueAt: due})
}
