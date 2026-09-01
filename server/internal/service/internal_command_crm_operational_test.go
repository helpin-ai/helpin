package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestCRMOperationalCommandsApplyBoundedUpdates(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	ctx := context.Background()

	output, err := env.commands.Execute(ctx, env.meta("crm_contact", "contact-1"), "crm.update_contact", json.RawMessage(`{
		"contact_id":"contact-1","lifecycle_stage":"customer","lead_status":"open","labels":["vip","renewal"],"clear_owner":true
	}`))
	if err != nil {
		t.Fatalf("update contact: %v", err)
	}
	var contactResult map[string]any
	if err := json.Unmarshal(output, &contactResult); err != nil {
		t.Fatal(err)
	}
	if contactResult["lifecycle_stage"] != model.CRMLifecycleCustomer || contactResult["owner_member_id"] != nil {
		t.Fatalf("unexpected contact result: %#v", contactResult)
	}
	contact, err := env.contact.GetByID(ctx, "contact-1")
	if err != nil {
		t.Fatal(err)
	}
	if contact.FirstName != "Ada" || contact.Email == nil || *contact.Email != "ada@example.com" || contact.OwnerMemberID != nil || len(contact.Labels) != 2 {
		t.Fatalf("guarded identity changed or bounded fields missing: %#v", contact)
	}

	if _, err := env.commands.Execute(ctx, env.meta("crm_company", "company-1"), "crm.update_company", json.RawMessage(`{"company_id":"company-1","clear_owner":true}`)); err != nil {
		t.Fatalf("update company: %v", err)
	}
	company, err := env.company.GetByID(ctx, "company-1")
	if err != nil {
		t.Fatal(err)
	}
	if company.Name != "Analytical Engines" || company.OwnerMemberID != nil {
		t.Fatalf("company identity changed or owner not cleared: %#v", company)
	}
}

func TestCRMOperationalDealAndActivityCommandsValidateWorkspaceObjects(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	ctx := context.Background()

	closeDate := "2027-06-30"
	if _, err := env.commands.Execute(ctx, env.meta("crm_deal", "deal-1"), "crm.update_deal", mustJSON(map[string]any{
		"deal_id": "deal-1", "stage_id": "stage-won", "amount": 42000, "currency": "EUR", "close_date": closeDate, "probability": 100,
	})); err != nil {
		t.Fatalf("update deal: %v", err)
	}
	deal, err := env.deal.GetByID(ctx, "deal-1")
	if err != nil {
		t.Fatal(err)
	}
	if deal.StageID != "stage-won" || deal.Amount == nil || *deal.Amount != 42000 || deal.CloseDate == nil || deal.Currency != "EUR" {
		t.Fatalf("deal update not persisted: %#v", deal)
	}

	if _, err := env.commands.Execute(ctx, env.meta("crm_contact", "contact-1"), "crm.add_activity", json.RawMessage(`{
		"activity_type":"meeting","contact_id":"contact-1","deal_id":"deal-1","subject":"Invalid double target"
	}`)); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected one-target validation, got %v", err)
	}
	output, err := env.commands.Execute(ctx, env.meta("crm_contact", "contact-1"), "crm.add_activity", json.RawMessage(`{
		"activity_type":"meeting","contact_id":"contact-1","subject":"Discovery","occurred_at":"2027-01-02T15:04:05Z"
	}`))
	if err != nil {
		t.Fatalf("add activity: %v", err)
	}
	var activity map[string]any
	if err := json.Unmarshal(output, &activity); err != nil {
		t.Fatal(err)
	}
	if activity["activity_type"] != model.CRMActivityMeeting || activity["contact_id"] != "contact-1" {
		t.Fatalf("unexpected activity: %#v", activity)
	}
}

func TestCRMCreateDealUsesExplicitPipelineAndStage(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	ctx := context.Background()

	output, err := env.commands.Execute(ctx, env.meta("workspace", "ws-crm-1"), "crm.create_deal", json.RawMessage(`{
		"name":"Analytical Engine expansion","contact_id":"contact-1","pipeline_id":"pipeline-1","stage_id":"stage-open","amount":12500,"currency":"eur","close_date":"2027-09-30","probability":35
	}`))
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if result["name"] != "Analytical Engine expansion" || result["pipeline_id"] != "pipeline-1" || result["stage_id"] != "stage-open" || result["contact_id"] != "contact-1" || result["currency"] != "EUR" {
		t.Fatalf("unexpected created deal: %#v", result)
	}
	dealID, _ := result["deal_id"].(string)
	var associationCount int64
	if err := db.Model(&model.CRMAssociation{}).Where("workspace_id = ? AND from_object_type = ? AND from_object_id = ? AND to_object_type = ? AND to_object_id = ?", "ws-crm-1", model.CRMObjectDeal, dealID, model.CRMObjectContact, "contact-1").Count(&associationCount).Error; err != nil {
		t.Fatal(err)
	}
	if associationCount != 1 {
		t.Fatalf("deal/contact association count = %d, want 1", associationCount)
	}
	var association model.CRMAssociation
	if err := db.Where("from_object_type = ? AND from_object_id = ?", model.CRMObjectDeal, dealID).First(&association).Error; err != nil {
		t.Fatal(err)
	}
	if association.AssociationLabel == nil || *association.AssociationLabel != model.CRMAssociationLabelDealCustomer {
		t.Fatalf("deal contact was not labeled as the customer: %#v", association)
	}
}

func TestCRMCreateDealSupportsCompanyWithoutContact(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	output, err := env.commands.Execute(context.Background(), env.meta("crm_company", "company-1"), "crm.create_deal", json.RawMessage(`{
		"name":"Company opportunity","company_id":"company-1","pipeline_id":"pipeline-1","stage_id":"stage-open"
	}`))
	if err != nil {
		t.Fatalf("create company deal: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if result["customer_type"] != model.CRMObjectCompany || result["customer_id"] != "company-1" {
		t.Fatalf("unexpected customer output: %#v", result)
	}
	dealID, _ := result["deal_id"].(string)
	var associations []model.CRMAssociation
	if err := db.Where("from_object_type = ? AND from_object_id = ?", model.CRMObjectDeal, dealID).Find(&associations).Error; err != nil {
		t.Fatal(err)
	}
	if len(associations) != 1 || associations[0].ToObjectType != model.CRMObjectCompany || associations[0].AssociationLabel == nil || *associations[0].AssociationLabel != model.CRMAssociationLabelDealCustomer {
		t.Fatalf("unexpected company deal associations: %#v", associations)
	}
}

func TestCRMCreateDealResolvesContactPrimaryCompany(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	primary := "primary"
	if err := db.Create(&model.CRMAssociation{ID: "contact-company-primary", WorkspaceID: "ws-crm-1", FromObjectType: model.CRMObjectContact, FromObjectID: "contact-1", ToObjectType: model.CRMObjectCompany, ToObjectID: "company-1", AssociationLabel: &primary}).Error; err != nil {
		t.Fatal(err)
	}
	env := newCRMOperationalCommandTestEnv(t, db)
	output, err := env.commands.Execute(context.Background(), env.meta("crm_contact", "contact-1"), "crm.create_deal", json.RawMessage(`{
		"name":"Account opportunity","contact_id":"contact-1","pipeline_id":"pipeline-1","stage_id":"stage-open"
	}`))
	if err != nil {
		t.Fatalf("create contact deal: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if result["customer_type"] != model.CRMObjectCompany || result["customer_id"] != "company-1" || result["primary_contact_id"] != "contact-1" {
		t.Fatalf("contact primary company was not resolved: %#v", result)
	}
}

func TestCRMSetDealCustomerPreservesPrimaryPersonWhenCompanyChanges(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	customerLabel := model.CRMAssociationLabelDealCustomer
	if err := db.Create(&model.CRMAssociation{ID: "deal-contact-customer", WorkspaceID: "ws-crm-1", FromObjectType: model.CRMObjectDeal, FromObjectID: "deal-1", ToObjectType: model.CRMObjectContact, ToObjectID: "contact-1", AssociationLabel: &customerLabel}).Error; err != nil {
		t.Fatal(err)
	}
	customer, err := env.deal.SetCustomer(context.Background(), "deal-1", model.SetCRMDealCustomerRequest{WorkspaceID: "ws-crm-1", CompanyID: "company-1"}, "user-crm-owner")
	if err != nil {
		t.Fatalf("set deal customer: %v", err)
	}
	if customer.CustomerType != model.CRMObjectCompany || customer.CustomerID != "company-1" || customer.PrimaryContactID != "contact-1" {
		t.Fatalf("unexpected resolved customer: %#v", customer)
	}
	var associations []model.CRMAssociation
	if err := db.Where("from_object_type = ? AND from_object_id = ? AND association_label IS NOT NULL", model.CRMObjectDeal, "deal-1").Order("association_label").Find(&associations).Error; err != nil {
		t.Fatal(err)
	}
	if len(associations) != 2 {
		t.Fatalf("expected customer and primary contact labels, got %#v", associations)
	}
}

func TestCRMCreateDealRequiresChoicesInsteadOfGuessing(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	ctx := context.Background()

	_, err := env.commands.Execute(ctx, env.meta("workspace", "ws-crm-1"), "crm.create_deal", json.RawMessage(`{"name":"Expansion","contact_id":"contact-1","stage_id":"stage-open"}`))
	if err == nil || !strings.Contains(err.Error(), "multiple pipelines") {
		t.Fatalf("expected pipeline choice error, got %v", err)
	}
	_, err = env.commands.Execute(ctx, env.meta("workspace", "ws-crm-1"), "crm.create_deal", json.RawMessage(`{"name":"Expansion","contact_id":"contact-1","pipeline_id":"pipeline-1"}`))
	if err == nil || !strings.Contains(err.Error(), "ask the user which stage") {
		t.Fatalf("expected stage choice error, got %v", err)
	}
}

func TestCRMCreateDealRejectsContactFromAnotherWorkspace(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	email := "other@example.com"
	if err := db.Create(&model.CRMContact{ID: "contact-other", WorkspaceID: "ws-crm-2", DisplayID: "CON-2", FirstName: "Other", Email: &email, LifecycleStage: model.CRMLifecycleLead, LeadStatus: model.CRMLeadStatusNew}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := env.commands.Execute(context.Background(), env.meta("workspace", "ws-crm-1"), "crm.create_deal", json.RawMessage(`{"name":"Wrong workspace","contact_id":"contact-other"}`))
	if err == nil || !strings.Contains(err.Error(), "contact not found") {
		t.Fatalf("expected workspace-scoped contact error, got %v", err)
	}
}

func TestCRMOperationalDiscoveryIsWorkspaceScopedAndBounded(t *testing.T) {
	db := setupCRMOperationalCommandTestDB(t)
	env := newCRMOperationalCommandTestEnv(t, db)
	ctx := context.Background()

	if _, err := env.commands.Execute(ctx, env.meta("workspace", "ws-crm-1"), "crm.get_company", json.RawMessage(`{"company_id":"company-other"}`)); err == nil || !strings.Contains(err.Error(), "company not found") {
		t.Fatalf("expected scoped company not found, got %v", err)
	}
	output, err := env.commands.Execute(ctx, env.meta("workspace", "ws-crm-1"), "crm.list_pipelines", json.RawMessage(`{"limit":1}`))
	if err != nil {
		t.Fatalf("list pipelines: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if items, ok := result["pipelines"].([]any); !ok || len(items) != 1 {
		t.Fatalf("expected bounded pipeline output, got %#v", result)
	}
}

type crmOperationalCommandTestEnv struct {
	commands *InternalCommandService
	contact  *CRMContactService
	company  *CRMCompanyService
	deal     *CRMDealService
}

func newCRMOperationalCommandTestEnv(t *testing.T, db *gorm.DB) *crmOperationalCommandTestEnv {
	t.Helper()
	contactRepo := repository.NewCRMContactRepository(db)
	companyRepo := repository.NewCRMCompanyRepository(db)
	dealRepo := repository.NewCRMDealRepository(db)
	assocRepo := repository.NewCRMAssociationRepository(db)
	activityRepo := repository.NewCRMActivityRepository(db)
	contact := NewCRMContactService(contactRepo)
	company := NewCRMCompanyService(companyRepo)
	deal := NewCRMDealService(dealRepo, assocRepo)
	activity := NewCRMActivityService(activityRepo)
	commands := NewInternalCommandService(nil, nil, deal, activity, nil, nil, nil, nil)
	commands.SetCRMReadServices(contact, nil)
	commands.SetCRMOperationalServices(company, NewCRMAssociationService(assocRepo))
	commands.SetPMOperationalServices(repository.NewWorkspaceRepository(db), nil, nil, nil, nil, nil)
	return &crmOperationalCommandTestEnv{commands: commands, contact: contact, company: company, deal: deal}
}

func (e *crmOperationalCommandTestEnv) meta(targetType, targetID string) model.InternalCommandContext {
	return model.InternalCommandContext{WorkspaceID: "ws-crm-1", ActorID: "user-crm-owner", ActorRole: model.RoleOwner, TargetType: targetType, TargetID: targetID}
}

func setupCRMOperationalCommandTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT NOT NULL, is_default BOOLEAN NOT NULL DEFAULT 0, default_commercial_motion TEXT NOT NULL DEFAULT 'new_business', position INTEGER NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY, pipeline_id TEXT NOT NULL, name TEXT NOT NULL, stage_type TEXT NOT NULL DEFAULT 'open', position INTEGER NOT NULL DEFAULT 0, probability INTEGER NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT NOT NULL, display_id TEXT NOT NULL, name TEXT NOT NULL, pipeline_id TEXT NOT NULL, stage_id TEXT NOT NULL, amount REAL, currency TEXT NOT NULL DEFAULT 'USD', close_date DATETIME, owner_member_id TEXT, commercial_motion TEXT, probability INTEGER, custom_properties TEXT NOT NULL DEFAULT '{}', created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_activities (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT NOT NULL, activity_type TEXT NOT NULL DEFAULT 'note', contact_id TEXT, company_id TEXT, deal_id TEXT, owner_member_id TEXT, subject TEXT, body TEXT, occurred_at DATETIME NOT NULL, metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME, updated_at DATETIME)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create CRM operational table: %v", err)
		}
	}
	seedUser(t, db, "user-crm-owner", "owner@crm.test", "Owner", "hash")
	seedWorkspace(t, db, "ws-crm-1", "CRM One", "crm-one", "user-crm-owner")
	seedWorkspace(t, db, "ws-crm-2", "CRM Two", "crm-two", "user-crm-owner")
	seedWorkspaceMember(t, db, "member-crm-owner", "ws-crm-1", "user-crm-owner", "owner@crm.test", "Owner", model.RoleOwner)
	now := time.Now().UTC()
	email := "ada@example.com"
	ownerMemberID := "member-crm-owner"
	if err := db.Create(&model.CRMContact{ID: "contact-1", WorkspaceID: "ws-crm-1", DisplayID: "CON-1", FirstName: "Ada", Email: &email, LifecycleStage: model.CRMLifecycleLead, LeadStatus: model.CRMLeadStatusNew, OwnerMemberID: &ownerMemberID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CRMCompany{ID: "company-1", WorkspaceID: "ws-crm-1", DisplayID: "COM-1", Name: "Analytical Engines", OwnerMemberID: &ownerMemberID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CRMCompany{ID: "company-other", WorkspaceID: "ws-crm-2", DisplayID: "COM-2", Name: "Other"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, pipeline := range []model.CRMPipeline{{ID: "pipeline-1", WorkspaceID: "ws-crm-1", Name: "Sales", IsDefault: true, Position: 0, CreatedAt: now}, {ID: "pipeline-2", WorkspaceID: "ws-crm-1", Name: "Renewals", Position: 1, CreatedAt: now}} {
		if err := db.Create(&pipeline).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, stage := range []model.CRMPipelineStage{{ID: "stage-open", PipelineID: "pipeline-1", Name: "Open", StageType: model.CRMStageTypeOpen, Position: 0, Probability: 10}, {ID: "stage-won", PipelineID: "pipeline-1", Name: "Won", StageType: model.CRMStageTypeWon, Position: 1, Probability: 100}} {
		if err := db.Create(&stage).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.CRMDeal{ID: "deal-1", WorkspaceID: "ws-crm-1", DisplayID: "DEAL-1", Name: "Engine", PipelineID: "pipeline-1", StageID: "stage-open", Currency: "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
