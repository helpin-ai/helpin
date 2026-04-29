package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestEnrichContactProtectsExistingEmailAndFillsEmptyFields(t *testing.T) {
	db := setupGuardedCRMEnrichmentTestDB(t)
	svc := setupGuardedCRMEnrichmentService(db)

	existingEmail := "owner@example.com"
	contact := model.CRMContact{
		ID:               "contact-1",
		WorkspaceID:      "ws-1",
		DisplayID:        "CON-1",
		FirstName:        "Jane",
		Email:            &existingEmail,
		LifecycleStage:   model.CRMLifecycleLead,
		LeadStatus:       model.CRMLeadStatusOpen,
		CustomProperties: model.JSONB{"manual_key": "keep"},
	}
	if err := db.Create(&contact).Error; err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	result, err := svc.EnrichContact(context.Background(), "ws-1", model.EnrichCRMContactRequest{
		ContactID: "contact-1",
		Fields: []model.CRMEnrichmentFieldInput{
			crmEnrichmentTestField("email", "new@example.com"),
			crmEnrichmentTestField("phone", "+1 415 555 0101"),
			crmEnrichmentTestField("job_title", "VP Sales"),
			crmEnrichmentTestField("linkedin_url", "https://www.linkedin.com/in/jane-example"),
		},
		EvidenceSummary: "Public profile research.",
	})
	if err != nil {
		t.Fatalf("EnrichContact returned error: %v", err)
	}
	if result.Status != "partial" {
		t.Fatalf("status = %q, want partial", result.Status)
	}
	if len(result.Applied) != 3 || len(result.Skipped) != 1 {
		t.Fatalf("applied/skipped = %d/%d, want 3/1: %+v", len(result.Applied), len(result.Skipped), result)
	}
	if result.Skipped[0].Field != "email" || result.Skipped[0].Reason != "existing_value_protected" {
		t.Fatalf("email skip = %+v, want protected existing value", result.Skipped[0])
	}

	var updated model.CRMContact
	if err := db.First(&updated, "id = ?", "contact-1").Error; err != nil {
		t.Fatalf("reload contact: %v", err)
	}
	if updated.Email == nil || *updated.Email != existingEmail {
		t.Fatalf("email was changed to %v, want %q", updated.Email, existingEmail)
	}
	if updated.Phone == nil || *updated.Phone != "+1 415 555 0101" {
		t.Fatalf("phone = %v, want filled phone", updated.Phone)
	}
	if updated.JobTitle == nil || *updated.JobTitle != "VP Sales" {
		t.Fatalf("job_title = %v, want VP Sales", updated.JobTitle)
	}
	if updated.CustomProperties["manual_key"] != "keep" {
		t.Fatalf("manual custom property was not preserved: %#v", updated.CustomProperties)
	}
	agentEnrichment, ok := updated.CustomProperties["agent_enrichment"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing agent_enrichment custom property: %#v", updated.CustomProperties)
	}
	if _, ok := agentEnrichment["linkedin_url"]; !ok {
		t.Fatalf("missing linkedin_url enrichment: %#v", agentEnrichment)
	}

	var count int64
	if err := db.Model(&model.CRMEnrichmentResult{}).Count(&count).Error; err != nil {
		t.Fatalf("count enrichments: %v", err)
	}
	if count != 1 || result.EnrichmentResultID == "" {
		t.Fatalf("audit count/id = %d/%q, want one audit row with id", count, result.EnrichmentResultID)
	}
}

func TestEnrichContactRejectsNameField(t *testing.T) {
	db := setupGuardedCRMEnrichmentTestDB(t)
	svc := setupGuardedCRMEnrichmentService(db)

	if err := db.Create(&model.CRMContact{
		ID:               "contact-1",
		WorkspaceID:      "ws-1",
		DisplayID:        "CON-1",
		FirstName:        "Jane",
		LifecycleStage:   model.CRMLifecycleLead,
		LeadStatus:       model.CRMLeadStatusOpen,
		CustomProperties: model.JSONB{},
	}).Error; err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	_, err := svc.EnrichContact(context.Background(), "ws-1", model.EnrichCRMContactRequest{
		ContactID: "contact-1",
		Fields: []model.CRMEnrichmentFieldInput{
			crmEnrichmentTestField("first_name", "Janet"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("err = %v, want disallowed field error", err)
	}
}

func TestEnrichCompanyProtectsNameAndExistingDomain(t *testing.T) {
	db := setupGuardedCRMEnrichmentTestDB(t)
	svc := setupGuardedCRMEnrichmentService(db)

	domain := "example.com"
	company := model.CRMCompany{
		ID:               "company-1",
		WorkspaceID:      "ws-1",
		DisplayID:        "COM-1",
		Name:             "Example Inc",
		Domain:           &domain,
		CustomProperties: model.JSONB{},
	}
	if err := db.Create(&company).Error; err != nil {
		t.Fatalf("seed company: %v", err)
	}

	result, err := svc.EnrichCompany(context.Background(), "ws-1", model.EnrichCRMCompanyRequest{
		CompanyID: "company-1",
		Fields: []model.CRMEnrichmentFieldInput{
			crmEnrichmentTestField("domain", "new.example.com"),
			crmEnrichmentTestField("industry", "Product Analytics"),
		},
	})
	if err != nil {
		t.Fatalf("EnrichCompany returned error: %v", err)
	}
	if result.Status != "partial" || len(result.Applied) != 1 || len(result.Skipped) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Skipped[0].Field != "domain" || result.Skipped[0].Reason != "existing_value_protected" {
		t.Fatalf("domain skip = %+v, want protected existing value", result.Skipped[0])
	}

	var updated model.CRMCompany
	if err := db.First(&updated, "id = ?", "company-1").Error; err != nil {
		t.Fatalf("reload company: %v", err)
	}
	if updated.Name != "Example Inc" {
		t.Fatalf("name = %q, want unchanged", updated.Name)
	}
	if updated.Domain == nil || *updated.Domain != domain {
		t.Fatalf("domain = %v, want unchanged %q", updated.Domain, domain)
	}
	if updated.Industry == nil || *updated.Industry != "Product Analytics" {
		t.Fatalf("industry = %v, want filled", updated.Industry)
	}
}

func TestEnrichContactDryRunDoesNotWrite(t *testing.T) {
	db := setupGuardedCRMEnrichmentTestDB(t)
	svc := setupGuardedCRMEnrichmentService(db)

	if err := db.Create(&model.CRMContact{
		ID:               "contact-1",
		WorkspaceID:      "ws-1",
		DisplayID:        "CON-1",
		FirstName:        "Jane",
		LifecycleStage:   model.CRMLifecycleLead,
		LeadStatus:       model.CRMLeadStatusOpen,
		CustomProperties: model.JSONB{},
	}).Error; err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	result, err := svc.EnrichContact(context.Background(), "ws-1", model.EnrichCRMContactRequest{
		ContactID: "contact-1",
		DryRun:    true,
		Fields: []model.CRMEnrichmentFieldInput{
			crmEnrichmentTestField("phone", "+1 415 555 0101"),
		},
	})
	if err != nil {
		t.Fatalf("EnrichContact returned error: %v", err)
	}
	if result.Status != "dry_run" || len(result.Applied) != 1 {
		t.Fatalf("result = %+v, want dry_run with one would-apply field", result)
	}

	var updated model.CRMContact
	if err := db.First(&updated, "id = ?", "contact-1").Error; err != nil {
		t.Fatalf("reload contact: %v", err)
	}
	if updated.Phone != nil {
		t.Fatalf("phone = %v, want no write during dry run", updated.Phone)
	}
}

func TestEnrichContactRejectsWrongWorkspaceTarget(t *testing.T) {
	db := setupGuardedCRMEnrichmentTestDB(t)
	svc := setupGuardedCRMEnrichmentService(db)

	if err := db.Create(&model.CRMContact{
		ID:               "contact-1",
		WorkspaceID:      "ws-other",
		DisplayID:        "CON-1",
		FirstName:        "Jane",
		LifecycleStage:   model.CRMLifecycleLead,
		LeadStatus:       model.CRMLeadStatusOpen,
		CustomProperties: model.JSONB{},
	}).Error; err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	_, err := svc.EnrichContact(context.Background(), "ws-1", model.EnrichCRMContactRequest{
		ContactID: "contact-1",
		Fields: []model.CRMEnrichmentFieldInput{
			crmEnrichmentTestField("phone", "+1 415 555 0101"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "contact not found") {
		t.Fatalf("err = %v, want workspace-scoped not found", err)
	}
}

func setupGuardedCRMEnrichmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE crm_enrichment_results (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		object_type TEXT NOT NULL,
		object_id TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'manual',
		data TEXT NOT NULL DEFAULT '{}',
		confidence REAL NOT NULL DEFAULT 0,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create crm_enrichment_results: %v", err)
	}
	return db
}

func setupGuardedCRMEnrichmentService(db *gorm.DB) *CRMEnrichmentService {
	return NewCRMEnrichmentService(
		repository.NewCRMEnrichmentRepository(db),
		repository.NewCRMContactRepository(db),
		repository.NewCRMCompanyRepository(db),
	)
}

func crmEnrichmentTestField(field string, value interface{}) model.CRMEnrichmentFieldInput {
	return model.CRMEnrichmentFieldInput{
		Field:      field,
		Value:      value,
		SourceURL:  "https://example.com/source",
		Evidence:   "Public source supports this value.",
		Confidence: 0.86,
	}
}
