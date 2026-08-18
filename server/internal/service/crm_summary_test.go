package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeSummaryLLMProvider struct {
	content  string
	beforeFn func(context.Context)
}

func (f *fakeSummaryLLMProvider) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if f.beforeFn != nil {
		f.beforeFn(ctx)
	}
	return &llm.ChatResponse{Content: f.content}, nil
}

type fakeSummaryWorkflowRunner struct {
	inputs []model.CRMEntitySummaryRefreshInput
	daily  int
}

func (f *fakeSummaryWorkflowRunner) StartSummaryRefresh(ctx context.Context, input model.CRMEntitySummaryRefreshInput) error {
	f.inputs = append(f.inputs, input)
	return nil
}

func (f *fakeSummaryWorkflowRunner) StartDailyReconciliation(ctx context.Context) error {
	f.daily++
	return nil
}

func setupCRMSummaryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:crm_summary_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE crm_contacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT,
			email TEXT,
			phone TEXT,
			job_title TEXT,
			description TEXT,
			labels BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			primary_location TEXT,
			country_code TEXT,
			country_name TEXT,
			linkedin_url TEXT,
			facebook_url TEXT,
			instagram_url TEXT,
			angellist_url TEXT,
			x_url TEXT,
			lifecycle_stage TEXT NOT NULL DEFAULT 'subscriber',
			lead_status TEXT NOT NULL DEFAULT 'new',
			owner_member_id TEXT,
			avatar_url TEXT,
			source TEXT,
			custom_properties BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			email_status TEXT NOT NULL DEFAULT 'valid',
			email_status_reason TEXT,
			email_status_updated_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_companies (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			name TEXT NOT NULL,
			domain TEXT,
			industry TEXT,
			size INTEGER,
			revenue REAL,
			owner_member_id TEXT,
			custom_properties BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_pipelines (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_pipeline_stages (
			id TEXT PRIMARY KEY,
			pipeline_id TEXT NOT NULL,
			name TEXT NOT NULL,
			stage_type TEXT NOT NULL DEFAULT 'open',
			position INTEGER NOT NULL DEFAULT 0,
			probability INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_deals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			name TEXT NOT NULL,
			pipeline_id TEXT NOT NULL,
			stage_id TEXT NOT NULL,
			amount REAL,
			currency TEXT NOT NULL DEFAULT 'USD',
			close_date DATETIME,
			owner_member_id TEXT,
			probability INTEGER,
			custom_properties BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_associations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			from_object_type TEXT NOT NULL,
			from_object_id TEXT NOT NULL,
			to_object_type TEXT NOT NULL,
			to_object_id TEXT NOT NULL,
			association_label TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_email_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			thread_id TEXT,
			message_external_id TEXT,
			from_address TEXT NOT NULL,
			from_name TEXT,
			to_addresses BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			cc_addresses BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			subject TEXT,
			body_text TEXT,
			body_html TEXT,
			direction TEXT NOT NULL DEFAULT 'inbound',
			sent_at DATETIME NOT NULL,
			contact_id TEXT,
			deal_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_email_message_contacts (
			message_id TEXT NOT NULL,
			contact_id TEXT NOT NULL,
			participant_role TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, contact_id, participant_role)
		)`,
		`CREATE TABLE crm_buyer_signals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			contact_id TEXT,
			deal_id TEXT,
			signal_type TEXT NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'manual',
			source_id TEXT,
			source_thread_id TEXT,
			summary TEXT NOT NULL,
			evidence_excerpt TEXT,
			metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			confidence REAL NOT NULL DEFAULT 0,
			detected_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_entity_summaries (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			summary_markdown TEXT NOT NULL DEFAULT '',
			highlights BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			status TEXT NOT NULL DEFAULT 'pending_refresh',
			computed_at DATETIME,
			source_window_start DATETIME,
			source_window_end DATETIME,
			last_triggered_at DATETIME,
			last_error TEXT,
			metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX idx_crm_entity_summaries_ws_entity ON crm_entity_summaries(workspace_id, entity_type, entity_id)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply schema %q: %v", stmt, err)
		}
	}

	return db
}

func TestCRMSummaryService_RefreshContactSummaryPersistsReady(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()

	summaryRepo := repository.NewCRMSummaryRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	companyRepo := repository.NewCRMCompanyRepository(db)
	dealRepo := repository.NewCRMDealRepository(db)
	associationRepo := repository.NewCRMAssociationRepository(db)
	signalRepo := repository.NewCRMSignalRepository(db)
	emailRepo := repository.NewCRMEmailRepository(db)

	now := time.Now().UTC()
	mustExecSummary(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, custom_properties) VALUES (?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"contact-1", "ws-1", "CON-1", "Atsuyo", "atsuyo@example.com", `{}`)
	mustExecSummary(t, db, `INSERT INTO crm_companies (id, workspace_id, display_id, name, domain, custom_properties) VALUES (?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"company-1", "ws-1", "COM-1", "Monita", "monita.com", `{}`)
	mustExecSummary(t, db, `INSERT INTO crm_pipeline_stages (id, pipeline_id, name, stage_type, position, probability) VALUES (?, ?, ?, ?, ?, ?)`,
		"stage-open", "pipe-1", "Qualified", "open", 0, 50)
	mustExecSummary(t, db, `INSERT INTO crm_deals (id, workspace_id, display_id, name, pipeline_id, stage_id, amount, currency, probability, custom_properties, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB), ?)`,
		"deal-1", "ws-1", "DEAL-1", "Monita Expansion", "pipe-1", "stage-open", 25000, "USD", 50, `{}`, now)
	mustExecSummary(t, db, `INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES (?, ?, ?, ?, ?, ?)`,
		"assoc-1", "ws-1", "contact", "contact-1", "company", "company-1")
	mustExecSummary(t, db, `INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES (?, ?, ?, ?, ?, ?)`,
		"assoc-2", "ws-1", "contact", "contact-1", "deal", "deal-1")
	mustExecSummary(t, db, `INSERT INTO crm_email_messages (id, workspace_id, email_account_id, from_address, to_addresses, cc_addresses, subject, body_text, direction, sent_at, contact_id) VALUES (?, ?, ?, ?, CAST(? AS BLOB), CAST(? AS BLOB), ?, ?, ?, ?, ?)`,
		"msg-1", "ws-1", "acct-1", "buyer@example.com", `["owner@example.com"]`, `[]`, "Pricing follow-up", "We have budget approved and want pricing this week.", "inbound", now.Add(-2*time.Hour), "contact-1")
	mustExecSummary(t, db, `INSERT INTO crm_email_message_contacts (message_id, contact_id, participant_role, workspace_id) VALUES (?, ?, ?, ?)`,
		"msg-1", "contact-1", "from", "ws-1")
	mustExecSummary(t, db, `INSERT INTO crm_buyer_signals (id, workspace_id, contact_id, deal_id, signal_type, source_type, source_id, summary, metadata, confidence, detected_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB), ?, ?)`,
		"signal-1", "ws-1", "contact-1", "deal-1", "buying_intent", "email", "msg-1", "Prospect requested pricing and confirmed budget.", `{}`, 0.91, now.Add(-time.Hour))

	requestedAt := now.Add(-30 * time.Second)
	if err := summaryRepo.UpsertRefreshRequest(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: "ws-1",
		EntityType:  model.CRMObjectContact,
		EntityID:    "contact-1",
	}, requestedAt); err != nil {
		t.Fatalf("UpsertRefreshRequest: %v", err)
	}

	svc := NewCRMSummaryService(summaryRepo, contactRepo, companyRepo, dealRepo, associationRepo, signalRepo, emailRepo, &fakeSummaryLLMProvider{
		content: `{"summary_markdown":"Momentum is positive.\n\nBudget has been confirmed and pricing is the active topic.","highlights":[{"kind":"momentum","text":"Budget is confirmed and pricing is under discussion."},{"kind":"next_step","text":"Send pricing details and book the next follow-up."}]}`,
	}, nil)

	result, err := svc.RefreshSummary(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: "ws-1",
		EntityType:  model.CRMObjectContact,
		EntityID:    "contact-1",
	})
	if err != nil {
		t.Fatalf("RefreshSummary: %v", err)
	}
	if result.Status != model.CRMEntitySummaryStatusReady || result.NeedsContinue {
		t.Fatalf("result = %+v, want ready without continue", result)
	}

	summary, err := summaryRepo.GetByEntity(ctx, "ws-1", model.CRMObjectContact, "contact-1")
	if err != nil {
		t.Fatalf("GetByEntity: %v", err)
	}
	if summary == nil {
		t.Fatal("expected persisted summary")
	}
	if summary.Status != model.CRMEntitySummaryStatusReady {
		t.Fatalf("summary status = %q, want ready", summary.Status)
	}
	if len(summary.Highlights) != 2 {
		t.Fatalf("highlights = %d, want 2", len(summary.Highlights))
	}
	if summary.Metadata["generation_version"] != crmSummaryGenerationVersion {
		t.Fatalf("generation_version = %v, want %s", summary.Metadata["generation_version"], crmSummaryGenerationVersion)
	}
}

func TestCRMSummaryService_LoadContactAssociationsPrefersPrimaryCompanyFirst(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()

	svc := NewCRMSummaryService(
		repository.NewCRMSummaryRepository(db),
		repository.NewCRMContactRepository(db),
		repository.NewCRMCompanyRepository(db),
		repository.NewCRMDealRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewCRMSignalRepository(db),
		repository.NewCRMEmailRepository(db),
		nil,
		nil,
	)

	mustExecSummary(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, custom_properties) VALUES (?, ?, ?, ?, CAST(? AS BLOB))`,
		"contact-1", "ws-1", "CON-1", "Atsuyo", `{}`)
	mustExecSummary(t, db, `INSERT INTO crm_companies (id, workspace_id, display_id, name, custom_properties) VALUES (?, ?, ?, ?, CAST(? AS BLOB))`,
		"company-1", "ws-1", "COM-1", "Primary Co", `{}`)
	mustExecSummary(t, db, `INSERT INTO crm_companies (id, workspace_id, display_id, name, custom_properties) VALUES (?, ?, ?, ?, CAST(? AS BLOB))`,
		"company-2", "ws-1", "COM-2", "Secondary Co", `{}`)
	mustExecSummary(t, db, `INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id, association_label) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"assoc-secondary", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectCompany, "company-2", nil)
	mustExecSummary(t, db, `INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id, association_label) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"assoc-primary", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectCompany, "company-1", primaryCompanyAssociationLabel)

	companies, companyNames, openDeals, err := svc.loadContactAssociations(ctx, "ws-1", "contact-1")
	if err != nil {
		t.Fatalf("loadContactAssociations returned error: %v", err)
	}

	if len(openDeals) != 0 {
		t.Fatalf("expected no deals, got %d", len(openDeals))
	}
	if len(companies) != 2 {
		t.Fatalf("expected two company snapshots, got %d", len(companies))
	}
	if got, want := companyNames, []string{"Primary Co", "Secondary Co"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("companyNames = %v, want %v", got, want)
	}
	if companies[0].Name != "Primary Co" {
		t.Fatalf("expected primary company first, got %+v", companies)
	}
}

func TestCRMSummaryService_RefreshSummaryMarksPendingWhenNewerTriggerExists(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()

	summaryRepo := repository.NewCRMSummaryRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	companyRepo := repository.NewCRMCompanyRepository(db)
	dealRepo := repository.NewCRMDealRepository(db)
	associationRepo := repository.NewCRMAssociationRepository(db)
	signalRepo := repository.NewCRMSignalRepository(db)
	emailRepo := repository.NewCRMEmailRepository(db)

	now := time.Now().UTC()
	mustExecSummary(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, custom_properties) VALUES (?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"contact-1", "ws-1", "CON-1", "Atsuyo", "atsuyo@example.com", `{}`)

	initialTriggeredAt := now.Add(-2 * time.Minute)
	input := model.CRMEntitySummaryRefreshInput{
		WorkspaceID: "ws-1",
		EntityType:  model.CRMObjectContact,
		EntityID:    "contact-1",
	}
	if err := summaryRepo.UpsertRefreshRequest(ctx, input, initialTriggeredAt); err != nil {
		t.Fatalf("UpsertRefreshRequest: %v", err)
	}

	svc := NewCRMSummaryService(summaryRepo, contactRepo, companyRepo, dealRepo, associationRepo, signalRepo, emailRepo, &fakeSummaryLLMProvider{
		content: `{"summary_markdown":"Momentum is positive.","highlights":[{"kind":"momentum","text":"The account is active."}]}`,
		beforeFn: func(context.Context) {
			later := now.Add(-10 * time.Second)
			if err := summaryRepo.UpsertRefreshRequest(ctx, input, later); err != nil {
				t.Fatalf("second UpsertRefreshRequest: %v", err)
			}
		},
	}, nil)

	result, err := svc.RefreshSummary(ctx, input)
	if err != nil {
		t.Fatalf("RefreshSummary: %v", err)
	}
	if !result.NeedsContinue || result.Status != model.CRMEntitySummaryStatusPendingRefresh {
		t.Fatalf("result = %+v, want pending_refresh with continue", result)
	}

	summary, err := summaryRepo.GetByEntity(ctx, "ws-1", model.CRMObjectContact, "contact-1")
	if err != nil {
		t.Fatalf("GetByEntity: %v", err)
	}
	if summary.Status != model.CRMEntitySummaryStatusPendingRefresh {
		t.Fatalf("summary status = %q, want pending_refresh", summary.Status)
	}
	if strings.TrimSpace(summary.SummaryMarkdown) == "" {
		t.Fatal("expected latest generated summary to be preserved")
	}
}

func TestCRMSummaryService_RunDailyReconciliationQueuesOpenDealsAndTouchedContacts(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()

	summaryRepo := repository.NewCRMSummaryRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	companyRepo := repository.NewCRMCompanyRepository(db)
	dealRepo := repository.NewCRMDealRepository(db)
	associationRepo := repository.NewCRMAssociationRepository(db)
	signalRepo := repository.NewCRMSignalRepository(db)
	emailRepo := repository.NewCRMEmailRepository(db)
	runner := &fakeSummaryWorkflowRunner{}

	now := time.Now().UTC()
	mustExecSummary(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, custom_properties) VALUES (?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"contact-1", "ws-1", "CON-1", "Atsuyo", "atsuyo@example.com", `{}`)
	mustExecSummary(t, db, `INSERT INTO crm_pipeline_stages (id, pipeline_id, name, stage_type, position, probability) VALUES (?, ?, ?, ?, ?, ?)`,
		"stage-open", "pipe-1", "Qualified", "open", 0, 50)
	mustExecSummary(t, db, `INSERT INTO crm_deals (id, workspace_id, display_id, name, pipeline_id, stage_id, custom_properties, updated_at) VALUES (?, ?, ?, ?, ?, ?, CAST(? AS BLOB), ?)`,
		"deal-1", "ws-1", "DEAL-1", "Expansion", "pipe-1", "stage-open", `{}`, now)
	mustExecSummary(t, db, `INSERT INTO crm_email_messages (id, workspace_id, email_account_id, from_address, to_addresses, cc_addresses, subject, body_text, direction, sent_at) VALUES (?, ?, ?, ?, CAST(? AS BLOB), CAST(? AS BLOB), ?, ?, ?, ?)`,
		"msg-1", "ws-1", "acct-1", "buyer@example.com", `["owner@example.com"]`, `[]`, "Follow-up", "Checking timeline.", "inbound", now.Add(-2*time.Hour))
	mustExecSummary(t, db, `INSERT INTO crm_email_message_contacts (message_id, contact_id, participant_role, workspace_id) VALUES (?, ?, ?, ?)`,
		"msg-1", "contact-1", "from", "ws-1")

	svc := NewCRMSummaryService(summaryRepo, contactRepo, companyRepo, dealRepo, associationRepo, signalRepo, emailRepo, nil, nil)
	svc.runner = runner

	result, err := svc.RunDailyReconciliation(ctx)
	if err != nil {
		t.Fatalf("RunDailyReconciliation: %v", err)
	}
	if result.DealsQueued != 1 || result.ContactsQueued != 1 {
		t.Fatalf("result = %+v, want one deal and one contact queued", result)
	}
	if len(runner.inputs) != 2 {
		t.Fatalf("runner inputs = %d, want 2", len(runner.inputs))
	}
}

func TestCRMSummaryService_RefreshSummaryFailurePreservesLastGoodSummary(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()

	summaryRepo := repository.NewCRMSummaryRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	companyRepo := repository.NewCRMCompanyRepository(db)
	dealRepo := repository.NewCRMDealRepository(db)
	associationRepo := repository.NewCRMAssociationRepository(db)
	signalRepo := repository.NewCRMSignalRepository(db)
	emailRepo := repository.NewCRMEmailRepository(db)

	now := time.Now().UTC()
	mustExecSummary(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, custom_properties) VALUES (?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"contact-1", "ws-1", "CON-1", "Atsuyo", "atsuyo@example.com", `{}`)

	input := model.CRMEntitySummaryRefreshInput{
		WorkspaceID: "ws-1",
		EntityType:  model.CRMObjectContact,
		EntityID:    "contact-1",
	}
	triggeredAt := now.Add(-30 * time.Second)
	if err := summaryRepo.UpsertRefreshRequest(ctx, input, triggeredAt); err != nil {
		t.Fatalf("UpsertRefreshRequest: %v", err)
	}
	computedAt := now.Add(-time.Minute)
	start := now.AddDate(0, 0, -7)
	end := now.Add(-time.Hour)
	status, needsContinue, err := summaryRepo.UpdateGenerated(
		ctx,
		input,
		triggeredAt,
		computedAt,
		"Existing summary that should survive a failed refresh.",
		model.CRMSummaryHighlights{{Kind: model.CRMSummaryHighlightMomentum, Text: "Momentum exists."}},
		&start,
		&end,
		model.JSONB{"generation_version": crmSummaryGenerationVersion},
	)
	if err != nil {
		t.Fatalf("UpdateGenerated: %v", err)
	}
	if status != model.CRMEntitySummaryStatusReady || needsContinue {
		t.Fatalf("generated status = %q continue=%v, want ready false", status, needsContinue)
	}

	laterTriggeredAt := now
	if err := summaryRepo.UpsertRefreshRequest(ctx, input, laterTriggeredAt); err != nil {
		t.Fatalf("second UpsertRefreshRequest: %v", err)
	}

	svc := NewCRMSummaryService(summaryRepo, contactRepo, companyRepo, dealRepo, associationRepo, signalRepo, emailRepo, &fakeSummaryLLMProvider{
		content: `{"highlights":[{"kind":"momentum","text":"Missing summary body"}]}`,
	}, nil)

	if _, err := svc.RefreshSummary(ctx, input); err == nil {
		t.Fatal("expected RefreshSummary error")
	}

	summary, err := summaryRepo.GetByEntity(ctx, "ws-1", model.CRMObjectContact, "contact-1")
	if err != nil {
		t.Fatalf("GetByEntity: %v", err)
	}
	if summary == nil {
		t.Fatal("expected persisted summary")
	}
	if summary.Status != model.CRMEntitySummaryStatusStale {
		t.Fatalf("summary status = %q, want stale", summary.Status)
	}
	if strings.TrimSpace(summary.SummaryMarkdown) != "Existing summary that should survive a failed refresh." {
		t.Fatalf("summary markdown = %q, want preserved prior summary", summary.SummaryMarkdown)
	}
	if summary.LastError == nil || strings.TrimSpace(*summary.LastError) == "" {
		t.Fatal("expected last_error to be recorded")
	}
}

func mustExecSummary(t *testing.T, db *gorm.DB, stmt string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(stmt, args...).Error; err != nil {
		t.Fatalf("exec %q failed: %v", stmt, err)
	}
}
