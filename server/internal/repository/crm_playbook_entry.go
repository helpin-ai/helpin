package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// A newly imported source is not necessarily new evidence. Both detection and
// creation must follow the explicit activation boundary, including after restart.
func enqueueAutomaticPlaybookEntry(tx *gorm.DB, ws, situationID, kind, sourceID string) error {
	if kind != "signal" || !tx.Migrator().HasTable(&model.CRMPlaybookAutomationSettings{}) {
		return nil
	}
	var signal model.CRMSignal
	if err := tx.Where("workspace_id = ? AND id = ?", ws, sourceID).Take(&signal).Error; err != nil {
		return err
	}
	var count int64
	if err := tx.Model(&model.CRMPlaybookAutomationSettings{}).Where("workspace_id = ? AND enabled = true AND entry_mode = 'automatic' AND automatic_since <= ? AND automatic_since <= ?", ws, signal.DetectedAt, signal.CreatedAt).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	var situation model.CRMSituation
	if err := tx.Where("workspace_id = ? AND id = ?", ws, situationID).Take(&situation).Error; err != nil {
		return err
	}
	return NewAutomationScheduledEventRepository(tx).Enqueue(tx.Statement.Context, model.AutomationScheduledEvent{WorkspaceID: ws, EventKey: "crm.playbook.entry:" + sourceID, Kind: model.CRMPlaybookEntryDue, TargetType: "crm_situation", TargetID: situationID, ExpectedRevision: situation.Revision, DueAt: situation.CreatedAt})
}

// AutomaticPlaybookCandidate is a fresh qualifying policy, not a grant to execute.
type AutomaticPlaybookCandidate struct {
	Settings      model.CRMPlaybookAutomationSettings
	Playbook      model.CRMPlaybook
	Connection    model.CRMPlaybookConnection
	Situation     model.CRMSituation
	OwnerMemberID *string
}

func (r *CRMPlaybookExecutionRepository) AutomaticCandidates(ctx context.Context, ws, id string) ([]AutomaticPlaybookCandidate, error) {
	return automaticPlaybookCandidates(r.db.WithContext(ctx), ws, id)
}

func automaticPlaybookCandidates(tx *gorm.DB, ws, id string) ([]AutomaticPlaybookCandidate, error) {
	var situation model.CRMSituation
	if err := tx.Where("workspace_id = ? AND id = ?", ws, id).Take(&situation).Error; err != nil {
		return nil, err
	}
	var settings []model.CRMPlaybookAutomationSettings
	if err := tx.Where("workspace_id = ? AND enabled = true AND entry_mode = 'automatic' AND automatic_since IS NOT NULL", ws).Order("playbook_id").Find(&settings).Error; err != nil {
		return nil, err
	}
	var candidates []AutomaticPlaybookCandidate
	for _, setting := range settings {
		var book model.CRMPlaybook
		if err := tx.Where("workspace_id = ? AND id = ?", ws, setting.PlaybookID).Take(&book).Error; err != nil {
			return nil, err
		}
		if !book.AcceptingCustomers || book.PublishedVersionID == nil {
			continue
		}
		connection, err := NewCRMPlaybookRepository(tx).Connection(tx.Statement.Context, ws, book.ID, setting.ConnectionID)
		if err != nil {
			return nil, err
		}
		if connection == nil || connection.PlaybookVersionID != *book.PublishedVersionID {
			continue
		}
		var version model.CRMPlaybookVersion
		if err := tx.Where("workspace_id = ? AND id = ?", ws, connection.PlaybookVersionID).Take(&version).Error; err != nil {
			return nil, err
		}
		query, err := eligiblePlaybookSignals(tx, ws, version.Definition)
		if err != nil {
			return nil, err
		}
		query = query.Where("s.id = ? AND s.created_at >= ?", id, setting.AutomaticSince)
		if situation.OriginKind == "deal_won" {
			if version.Definition.Journey != "sales_handoff" {
				continue
			}
			query = query.Where(wonDealHandoffEligibilitySQL)
		} else {
			query = query.Where("EXISTS (SELECT 1 FROM crm_situation_source_links source JOIN crm_signals evidence ON evidence.workspace_id = source.workspace_id AND evidence.id = source.source_id WHERE source.workspace_id = s.workspace_id AND source.situation_id = s.id AND source.kind = 'signal' AND evidence.created_at >= ? AND evidence.detected_at >= ? AND evidence.dismissed_at IS NULL AND evidence.superseded_at IS NULL)", setting.AutomaticSince, setting.AutomaticSince)
		}
		var count int64
		if err := query.Select("COUNT(*)").Scan(&count).Error; err != nil {
			return nil, err
		}
		if count != 1 {
			continue
		}
		owner, err := automaticPlaybookOwner(tx, situation, version.Definition)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, AutomaticPlaybookCandidate{Settings: setting, Playbook: book, Connection: *connection, Situation: situation, OwnerMemberID: owner})
	}
	return candidates, nil
}

// EnrollAutomatically rechecks matching and activation under the same workspace
// lock as explicit enrollment and event acknowledgement. Overlap never picks a winner.
func (r *CRMPlaybookExecutionRepository) EnrollAutomatically(ctx context.Context, claim model.AutomationScheduledEvent, expected *AutomaticPlaybookCandidate, owner string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, claim.WorkspaceID); err != nil {
			return err
		}
		events := NewAutomationScheduledEventRepository(tx)
		if _, err := events.LockClaim(ctx, claim, now); err != nil {
			return err
		}
		if claim.Kind != model.CRMPlaybookEntryDue || claim.TargetType != "crm_situation" {
			return ErrCRMPlaybookExecutionBlocked
		}
		candidates, err := automaticPlaybookCandidates(tx, claim.WorkspaceID, claim.TargetID)
		if err != nil {
			return err
		}
		if len(candidates) != 1 || expected == nil {
			return events.Complete(ctx, claim, "needs_review", now)
		}
		candidate := candidates[0]
		if candidate.Settings.Revision != expected.Settings.Revision || candidate.Playbook.ID != expected.Playbook.ID || candidate.Situation.Revision != claim.ExpectedRevision || candidate.OwnerMemberID == nil || *candidate.OwnerMemberID != owner {
			return events.Complete(ctx, claim, "changed", now)
		}
		if err := requirePlaybookExecutionMember(tx, claim.WorkspaceID, owner); err != nil {
			return err
		}
		applied, err := NewCRMPlaybookRepository(tx).applyWithRouting(ctx, claim.WorkspaceID, candidate.Playbook.ID, candidate.Settings.AuthorizedByMemberID, model.ApplyCRMPlaybookRequest{CommandKey: "auto:" + claim.ID, SituationID: claim.TargetID, VersionID: candidate.Connection.PlaybookVersionID, ExpectedPlaybookRevision: candidate.Playbook.Revision, ExpectedSituationRevision: claim.ExpectedRevision, Confirmed: true}, "automatic:"+claim.ID, &owner)
		if err != nil {
			return err
		}
		if applied == nil {
			return ErrCRMPlaybookExecutionBlocked
		}
		// The actor is the live configuration authorizer; the reason distinguishes
		// automatic policy execution from an individual teammate clicking Apply.
		if err := tx.Model(&model.CRMSituationChange{}).Where("workspace_id = ? AND id = ?", claim.WorkspaceID, applied.Change.ID).Update("reason", "Automatically enrolled under the configured playbook policy").Error; err != nil {
			return err
		}
		_, err = NewCRMPlaybookExecutionRepository(tx).Adopt(ctx, claim.WorkspaceID, claim.TargetID, owner, model.CRMPlaybookAutomationAdoption{CommandKey: "auto:" + claim.ID, ExpectedRevision: applied.Change.Revision, ExpectedGeneration: 0, ConnectionID: candidate.Connection.ID, Enabled: true, Confirmed: true}, fmt.Sprintf("automatic:%s:%s", claim.ID, owner), now)
		if err != nil {
			return err
		}
		return events.Complete(ctx, claim, "enrolled", now)
	})
}

// Automatic entry resolves the configured role once, under the enrollment lock.
// Missing roles remain routing gaps; no workspace admin is silently substituted.
func automaticPlaybookOwner(tx *gorm.DB, work model.CRMSituation, definition model.CRMPlaybookDefinition) (*string, error) {
	role := definition.Responsibilities.OwnerRole
	// A native win has an explicit selling owner. Success takes responsibility
	// only through the receiving owner's reviewed handoff action.
	if work.OriginKind == "deal_won" {
		role = "deal_owner"
	}
	if role == "signal_owner" {
		return work.OwnerMemberID, nil
	}
	var owner struct{ ID *string }
	var query *gorm.DB
	if role == "deal_owner" {
		if work.DealID == nil {
			return nil, nil
		}
		query = tx.Table("crm_deals").Select("owner_member_id AS id").Where("workspace_id = ? AND id = ?", work.WorkspaceID, *work.DealID)
	} else {
		if work.CompanyID == nil {
			return nil, nil
		}
		column := "owner_member_id"
		if role == "customer_success_owner" {
			column = "customer_success_owner_member_id"
		}
		query = tx.Table("crm_companies").Select(column+" AS id").Where("workspace_id = ? AND id = ?", work.WorkspaceID, *work.CompanyID)
	}
	if err := query.Scan(&owner).Error; err != nil {
		return nil, err
	}
	return owner.ID, nil
}
