package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	// ErrCRMSituationConflict means a creation key was reused for different work.
	ErrCRMSituationConflict = errors.New("situation creation key conflicts")
	// ErrCRMSituationInvalidReference hides missing and cross-workspace source identities.
	ErrCRMSituationInvalidReference = errors.New("situation reference is unavailable in workspace")
)

// CRMSituationRepository persists customer work without executing referenced actions.
type CRMSituationRepository struct {
	db           *gorm.DB
	inboxSignals CRMInboxSignalComposer
}

// NewCRMSituationRepository creates the customer-work store.
func NewCRMSituationRepository(db *gorm.DB) *CRMSituationRepository {
	return &CRMSituationRepository{db: db}
}

// Create atomically inserts work and references, or returns an identical prior request.
// It never merges by company, motion, evidence, or another situation's title.
func (r *CRMSituationRepository) Create(
	ctx context.Context, situation model.CRMSituation, references []model.CRMSituationReference,
) (*model.CRMSituation, bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, situation.WorkspaceID); err != nil {
			return err
		}
		var existing model.CRMSituation
		err := tx.Where("workspace_id = ? AND creation_key = ?", situation.WorkspaceID, situation.CreationKey).
			First(&existing).Error
		if err == nil {
			if existing.CreationFingerprint != situation.CreationFingerprint {
				return ErrCRMSituationConflict
			}
			situation = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find situation creation: %w", err)
		}
		if err := validateSituationReferences(tx, situation, references); err != nil {
			return err
		}
		// Return canonical stored values, including PostgreSQL timestamp
		// precision, so the first response and an identical replay agree.
		result := tx.Clauses(clause.Returning{}, clause.OnConflict{
			Columns: []clause.Column{{Name: "workspace_id"}, {Name: "creation_key"}}, DoNothing: true,
		}).Create(&situation)
		if result.Error != nil {
			return fmt.Errorf("create customer situation: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if err := tx.Where("workspace_id = ? AND creation_key = ?", situation.WorkspaceID, situation.CreationKey).
				First(&existing).Error; err != nil {
				return fmt.Errorf("read concurrently created situation: %w", err)
			}
			if existing.CreationFingerprint != situation.CreationFingerprint {
				return ErrCRMSituationConflict
			}
			situation = existing
			return nil
		}
		for _, reference := range references {
			reference.WorkspaceID, reference.SituationID = situation.WorkspaceID, situation.ID
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reference).Error; err != nil {
				return fmt.Errorf("link situation source: %w", err)
			}
		}
		if err := recordSituationCreation(tx, situation); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &situation, created, nil
}

// ResolveOwner snapshots the existing motion-aware signal routing policy at creation.
// Explicit owner/unassigned choices bypass this method; reads never reroute stored work.
func (r *CRMSituationRepository) ResolveOwner(
	ctx context.Context, workspaceID string, req model.CreateCRMSituationRequest,
) (*string, error) {
	signals := []model.CRMSignal{{
		WorkspaceID: workspaceID, CompanyID: req.CompanyID, ContactID: req.ContactID,
		DealID: req.DealID, CommercialMotion: req.CommercialMotion,
	}}
	if err := NewCRMSignalRepository(r.db).hydrateSignalContext(ctx, workspaceID, signals); err != nil {
		return nil, err
	}
	return signals[0].OwnerMemberID, nil
}

// GetByID returns workspace-scoped work and links whose source still belongs there.
// It includes a minimal CRM evidence projection; source endpoints enforce source access.
func (r *CRMSituationRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.CRMSituationItem, error) {
	var item model.CRMSituationItem
	err := situationReadQuery(r.db.WithContext(ctx), workspaceID).
		Where("s.id = ?", id).Take(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read customer situation: %w", err)
	}
	err = r.db.WithContext(ctx).Table("crm_situation_references AS ref").
		Where("ref.workspace_id = ? AND ref.situation_id = ?", workspaceID, id).
		Where(`(ref.kind = 'signal' AND EXISTS (SELECT 1 FROM crm_signals source
			WHERE source.workspace_id = ref.workspace_id AND source.id = ref.source_id))
			OR (ref.kind = 'suggestion' AND EXISTS (SELECT 1 FROM crm_suggestions source
			WHERE source.workspace_id = ref.workspace_id AND source.id = ref.source_id))`).
		Order("ref.created_at ASC, ref.kind ASC, ref.source_id ASC").Find(&item.References).Error
	if err != nil {
		return nil, fmt.Errorf("read situation references: %w", err)
	}
	item.Actions, err = r.situationActions(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	item.Evidence, err = r.situationEvidence(ctx, workspaceID, item)
	if err != nil {
		return nil, err
	}
	for i := range item.Actions {
		if item.Actions[i].UserID != nil {
			continue
		}
		if item.NextActionOwnerAvailable {
			item.Actions[i].AssigneeMemberID = item.Situation.NextActionOwnerMemberID
			item.Actions[i].AssigneeAvailable = true
		} else if item.OwnerAvailable {
			item.Actions[i].AssigneeMemberID = item.Situation.OwnerMemberID
			item.Actions[i].AssigneeAvailable = true
		}
	}
	return &item, nil
}

func (r *CRMSituationRepository) situationEvidence(ctx context.Context, ws string, item model.CRMSituationItem) ([]model.CRMSituationEvidence, error) {
	ids := make(map[string]bool)
	for _, ref := range item.References {
		if ref.Kind == model.CRMSituationReferenceSignal {
			ids[ref.SourceID] = true
		}
	}
	for _, action := range item.Actions {
		for _, id := range action.SignalIDs {
			ids[id] = true
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(ids))
	for id := range ids {
		keys = append(keys, id)
	}
	var evidence []model.CRMSituationEvidence
	err := r.db.WithContext(ctx).Table("crm_signals").
		Select("id, summary, evidence_excerpt, source_type, source_id, source_thread_id, contact_id, company_id, evidence_identity_trust, detected_at, reviewed_at, dismissed_at, superseded_at").
		Where("workspace_id = ? AND id IN ?", ws, keys).
		Order("detected_at DESC, id ASC").Scan(&evidence).Error
	if err != nil {
		return nil, fmt.Errorf("read situation evidence: %w", err)
	}
	return evidence, nil
}

func validateSituationReferences(tx *gorm.DB, situation model.CRMSituation, refs []model.CRMSituationReference) error {
	for _, target := range []struct {
		table string
		id    *string
	}{
		{table: "crm_companies", id: situation.CompanyID},
		{table: "crm_contacts", id: situation.ContactID},
		{table: "crm_deals", id: situation.DealID},
		{table: "workspace_members", id: situation.OwnerMemberID},
		{table: "workspace_members", id: situation.NextActionOwnerMemberID},
		{table: "workspace_members", id: situation.CreatedByMemberID},
	} {
		if target.id == nil {
			continue
		}
		if err := situationSourceExists(tx, target.table, situation.WorkspaceID, *target.id); err != nil {
			return err
		}
	}
	for _, ref := range refs {
		var table string
		switch ref.Kind {
		case model.CRMSituationReferenceSignal:
			table = "crm_signals"
		case model.CRMSituationReferenceSuggestion:
			table = "crm_suggestions"
		default:
			return ErrCRMSituationInvalidReference
		}
		if err := situationSourceExists(tx, table, situation.WorkspaceID, ref.SourceID); err != nil {
			return err
		}
	}
	return nil
}

func situationSourceExists(tx *gorm.DB, table, workspaceID, id string) error {
	var count int64
	query := tx.Table(table).Where("workspace_id = ? AND id = ?", workspaceID, id)
	if table == "workspace_members" {
		query = query.Where("status = ?", model.WorkspaceMemberStatusActive)
	}
	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("validate situation source: %w", err)
	}
	if count != 1 {
		return ErrCRMSituationInvalidReference
	}
	return nil
}
