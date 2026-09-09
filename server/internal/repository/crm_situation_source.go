package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ImportSource links a canonical source exactly once, without touching its
// decision or delivery state. Only an explicit situation ID can consolidate work.
func (r *CRMSituationRepository) ImportSource(ctx context.Context, input model.CRMSituationSourceInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ws := input.Situation.WorkspaceID
		// Serialize enrollment within a workspace across replicas. This also
		// prevents orphan situations when concurrent callers suggest different links.
		var workspace struct{ ID string }
		if err := tx.Table("workspaces").Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", ws).Take(&workspace).Error; err != nil {
			return fmt.Errorf("lock source workspace: %w", err)
		}
		if input.Kind == model.CRMSituationReferenceSuggestion {
			var current model.CRMSuggestion
			if err := tx.Where("workspace_id = ? AND id = ?", ws, input.SourceID).Take(&current).Error; err != nil {
				return err
			}
			if model.IsInternalMeetingFollowUp(current) {
				if current.Context["meeting_follow_up_reviewed_in"] == "my_work" {
					return nil
				}
				var routable int64
				if err := tx.Model(&model.CRMSuggestion{}).Where("workspace_id = ? AND id = ?", ws, input.SourceID).Where(PMAISuggestionRoutingEligible(tx, "crm_suggestions")).Count(&routable).Error; err != nil {
					return err
				}
				if routable > 0 {
					return nil
				}
			}
		}
		var link model.CRMSituationSourceLink
		err := tx.Where("workspace_id = ? AND kind = ? AND source_id = ?", ws, input.Kind, input.SourceID).Take(&link).Error
		if err == nil {
			if input.Kind == model.CRMSituationReferenceSuggestion {
				// Keep later explicitly attached evidence, without moving the
				// source to a different situation or deleting previous context.
				for _, ref := range input.References {
					if ref.Kind != model.CRMSituationReferenceSignal {
						return ErrCRMSituationInvalidReference
					}
					if err := situationSourceExists(tx, "crm_signals", ws, ref.SourceID); err != nil {
						return err
					}
					ref.WorkspaceID, ref.SituationID = ws, link.SituationID
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
						return err
					}
				}
			}
			// Source retries cannot rewrite assignment, outcomes, or explicit next steps.
			if input.Kind == model.CRMSituationReferenceSignal {
				return tx.Model(&model.CRMSituation{}).Where("workspace_id = ? AND id = ? AND lifecycle = ?", ws, link.SituationID, "open").
					UpdateColumn("priority", input.Situation.Priority).Error
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("read source mapping: %w", err)
		}
		refs := append([]model.CRMSituationReference{{Kind: input.Kind, SourceID: input.SourceID}}, input.References...)
		if err := validateSituationReferences(tx, input.Situation, refs); err != nil {
			return err
		}
		destination := input.ExistingSituationID
		if destination != nil {
			var count int64
			if err := tx.Model(&model.CRMSituation{}).Where("workspace_id = ? AND id = ? AND lifecycle = 'open'", ws, *destination).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return ErrCRMSituationInvalidReference
			}
		} else {
			input.Situation.ID = uuid.NewString()
			input.Situation.OriginKind = input.Kind
			input.Situation.CreationKey = "source:" + input.Kind + ":" + input.SourceID
			input.Situation.CreationFingerprint = input.Situation.CreationKey
			input.Situation.CreatedByMemberID = nil
			if err := tx.Create(&input.Situation).Error; err != nil {
				return fmt.Errorf("create source situation: %w", err)
			}
			if err := recordSituationCreation(tx, input.Situation); err != nil {
				return err
			}
			destination = &input.Situation.ID
		}
		link = model.CRMSituationSourceLink{WorkspaceID: ws, Kind: input.Kind, SourceID: input.SourceID, SituationID: *destination}
		if err := tx.Create(&link).Error; err != nil {
			return fmt.Errorf("map source situation: %w", err)
		}
		for _, ref := range refs {
			ref.WorkspaceID, ref.SituationID = ws, *destination
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
				return fmt.Errorf("link source evidence: %w", err)
			}
		}
		return enqueueAutomaticPlaybookEntry(tx, ws, *destination, input.Kind, input.SourceID)
	})
}

// SourceSignals keyset-pages all current evidence; policy qualification is
// applied by the existing signal service, independently of notification delivery.
func (r *CRMSituationRepository) SourceSignals(ctx context.Context, ws, after string, limit int) ([]model.CRMSignal, error) {
	var rows []model.CRMSignal
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND dismissed_at IS NULL AND superseded_at IS NULL AND acted_at IS NULL", ws)
	if after != "" {
		query = query.Where("id > ?", after)
	}
	if err := query.Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := NewCRMSignalRepository(r.db).hydrateSignalContext(ctx, ws, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// SourceSuggestions includes unresolved work, never reopens historical successes
// or dismissals, and does not require a CRM target, signal, or enabled playbook.
func (r *CRMSituationRepository) SourceSuggestions(ctx context.Context, ws, after string, limit int) ([]model.CRMSuggestion, error) {
	var rows []model.CRMSuggestion
	query := r.db.WithContext(ctx).Where("workspace_id = ?", ws).
		Where("status = 'pending' OR (status = 'accepted' AND (execution_status IS NULL OR execution_status IN ('','pending','in_progress','failed','manual_required')))")
	if after != "" {
		query = query.Where("id > ?", after)
	}
	if err := query.Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// SourceMember resolves a suggestion's user identity to an active workspace
// membership. Missing/inactive assignees remain an explicit routing gap.
func (r *CRMSituationRepository) SourceMember(ctx context.Context, ws, userID string) (*string, error) {
	var row struct{ ID string }
	err := r.db.WithContext(ctx).Table("workspace_members").Select("id").Where("workspace_id = ? AND user_id = ? AND status = 'active'", ws, userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row.ID, nil
}
