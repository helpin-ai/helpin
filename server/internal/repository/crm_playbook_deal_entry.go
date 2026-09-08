package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// enterWonDeal records the actual transition and its wake-up in the deal's
// transaction. A restart cannot lose the handoff; enabling never scans old wins.
func enterWonDeal(tx *gorm.DB, deal model.CRMDeal, previousStage string, now time.Time) error {
	if previousStage == deal.StageID || !tx.Migrator().HasTable(&model.CRMPlaybookAutomationSettings{}) {
		return nil
	}
	var won int64
	if err := tx.Table("crm_pipeline_stages").Where("id = ? AND pipeline_id = ? AND stage_type = 'won'", deal.StageID, deal.PipelineID).Count(&won).Error; err != nil || won == 0 {
		return err
	}
	var settings []model.CRMPlaybookAutomationSettings
	if err := tx.Where("workspace_id = ? AND enabled = true AND entry_mode = 'automatic' AND automatic_since <= ?", deal.WorkspaceID, now).Find(&settings).Error; err != nil {
		return err
	}
	qualifies := false
	for _, setting := range settings {
		var version model.CRMPlaybookVersion
		err := tx.Table("crm_playbook_versions v").Select("v.*").Joins("JOIN crm_playbooks p ON p.workspace_id = v.workspace_id AND p.published_version_id = v.id AND p.accepting_customers = true").Joins("JOIN crm_playbook_connections c ON c.workspace_id = v.workspace_id AND c.playbook_version_id = v.id").Where("v.workspace_id = ? AND c.id = ? AND p.id = ?", deal.WorkspaceID, setting.ConnectionID, setting.PlaybookID).Take(&version).Error
		if err == gorm.ErrRecordNotFound {
			continue
		}
		if err != nil {
			return err
		}
		qualifies = qualifies || version.Definition.Journey == "sales_handoff"
	}
	if !qualifies {
		return nil
	}
	var active int64
	if err := tx.Model(&model.CRMSituation{}).Where("workspace_id = ? AND deal_id = ? AND commercial_motion = 'onboarding' AND lifecycle IN ('open', 'paused')", deal.WorkspaceID, deal.ID).Count(&active).Error; err != nil || active > 0 {
		return err
	}
	work := model.CRMSituation{ID: uuid.NewString(), WorkspaceID: deal.WorkspaceID, OriginKind: "deal_won", CreationKey: "deal_won:" + deal.ID + ":" + now.Format(time.RFC3339Nano), Title: "Prepare handoff · " + deal.Name, Objective: "Have the receiving success owner accept a usable handoff", CommercialMotion: "onboarding", DealID: &deal.ID, OwnerMemberID: deal.OwnerMemberID, NextActionOwnerMemberID: deal.OwnerMemberID, Lifecycle: "open", Attention: "needs_context", NextStep: "Confirm the sold scope and receiving success owner", Revision: 1, CreatedAt: now, UpdatedAt: now}
	work.CreationFingerprint = work.CreationKey
	customer, err := NewCRMDealRepository(tx).GetCustomer(tx.Statement.Context, deal.WorkspaceID, deal.ID)
	if err != nil {
		return err
	}
	if customer != nil {
		if customer.CustomerType == model.CRMObjectCompany {
			work.CompanyID = &customer.CustomerID
		}
		if customer.PrimaryContactID != "" {
			work.ContactID = &customer.PrimaryContactID
		}
	}
	if err := tx.Create(&work).Error; err != nil {
		return err
	}
	if err := recordSituationCreation(tx, work); err != nil {
		return err
	}
	return NewAutomationScheduledEventRepository(tx).Enqueue(tx.Statement.Context, model.AutomationScheduledEvent{WorkspaceID: deal.WorkspaceID, EventKey: work.CreationKey, Kind: model.CRMPlaybookEntryDue, TargetType: "crm_situation", TargetID: work.ID, ExpectedRevision: work.Revision, DueAt: now})
}
