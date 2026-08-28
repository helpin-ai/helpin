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
	contents []string
	beforeFn func(context.Context)
	calls    int
}

func (f *fakeSummaryLLMProvider) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.calls++
	if f.beforeFn != nil {
		f.beforeFn(ctx)
	}
	if len(f.contents) > 0 {
		index := f.calls - 1
		if index >= len(f.contents) {
			index = len(f.contents) - 1
		}
		return &llm.ChatResponse{Content: f.contents[index]}, nil
	}
	return &llm.ChatResponse{Content: f.content}, nil
}

func TestCRMSummaryService_RetriesChineseNarrativeAndKeepsEnglishResult(t *testing.T) {
	provider := &fakeSummaryLLMProvider{contents: []string{
		`{"summary_markdown":"该公司正在评估企业计划，并希望本月作出决定。","highlights":[{"kind":"next_step","text":"安排后续会议。"}]}`,
		`{"summary_markdown":"The company is evaluating the enterprise plan and expects to decide this month.","highlights":[{"kind":"next_step","text":"Schedule the follow-up meeting."}]}`,
	}}
	svc := &CRMSummaryService{llmProvider: provider}
	output, err := svc.generateSummaryLLM(context.Background(), AIUsageMeteringContext{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCRMSummary, IdempotencyKey: "summary-language-test",
	}, summaryPromptEntity{EntityType: model.CRMObjectCompany})
	if err != nil {
		t.Fatalf("generateSummaryLLM: %v", err)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
	if !strings.Contains(output.SummaryMarkdown, "evaluating the enterprise plan") {
		t.Fatalf("summary = %q, want retried English result", output.SummaryMarkdown)
	}
}

func TestCRMSummaryService_RefreshContactSummaryNowGeneratesSparseSummaryAndSatisfiesQueuedRefresh(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()

	summaryRepo := repository.NewCRMSummaryRepository(db)
	provider := &fakeSummaryLLMProvider{
		content: `{"summary_markdown":"There is limited recent activity for Atsuyo, but the contact is currently a new subscriber.","highlights":[{"kind":"next_step","text":"Add a note or reach out to establish the next step."}]}`,
	}
	runner := &fakeSummaryWorkflowRunner{}
	svc := NewCRMSummaryService(
		summaryRepo,
		repository.NewCRMContactRepository(db),
		repository.NewCRMCompanyRepository(db),
		repository.NewCRMDealRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewCRMSignalRepository(db),
		repository.NewCRMEmailRepository(db),
		provider,
		nil,
	)
	svc.runner = runner

	mustExecSummary(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, custom_properties) VALUES (?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"contact-1", "ws-1", "CON-1", "Atsuyo", "atsuyo@example.com", `{}`)

	summary, err := svc.RefreshContactSummaryNow(ctx, "ws-1", "contact-1")
	if err != nil {
		t.Fatalf("RefreshContactSummaryNow: %v", err)
	}
	if summary == nil || summary.Status != model.CRMEntitySummaryStatusReady {
		t.Fatalf("summary = %+v, want ready summary", summary)
	}
	if !strings.Contains(summary.SummaryMarkdown, "limited recent activity") {
		t.Fatalf("summary markdown = %q, want sparse-context summary", summary.SummaryMarkdown)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if len(runner.inputs) != 0 {
		t.Fatalf("manual refresh queued %d workflows, want none", len(runner.inputs))
	}

	result, err := svc.RefreshSummary(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: "ws-1",
		EntityType:  model.CRMObjectContact,
		EntityID:    "contact-1",
	})
	if err != nil {
		t.Fatalf("RefreshSummary after manual generation: %v", err)
	}
	if result.Status != model.CRMEntitySummaryStatusReady || result.NeedsContinue {
		t.Fatalf("queued result = %+v, want satisfied ready result", result)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls after queued refresh = %d, want 1", provider.calls)
	}
}

func TestCRMSummaryService_RefreshDealSummaryNowGeneratesSparseSummary(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()

	provider := &fakeSummaryLLMProvider{
		content: `{"summary_markdown":"The expansion deal is currently qualified, with no recent communication recorded.","highlights":[{"kind":"next_step","text":"Confirm the buyer's timeline and next meeting."}]}`,
	}
	svc := NewCRMSummaryService(
		repository.NewCRMSummaryRepository(db),
		repository.NewCRMContactRepository(db),
		repository.NewCRMCompanyRepository(db),
		repository.NewCRMDealRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewCRMSignalRepository(db),
		repository.NewCRMEmailRepository(db),
		provider,
		nil,
	)

	mustExecSummary(t, db, `INSERT INTO crm_pipeline_stages (id, pipeline_id, name, stage_type, position, probability) VALUES (?, ?, ?, ?, ?, ?)`,
		"stage-open", "pipe-1", "Qualified", "open", 0, 50)
	mustExecSummary(t, db, `INSERT INTO crm_deals (id, workspace_id, display_id, name, pipeline_id, stage_id, currency, custom_properties) VALUES (?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"deal-1", "ws-1", "DEAL-1", "Expansion", "pipe-1", "stage-open", "USD", `{}`)

	summary, err := svc.RefreshDealSummaryNow(ctx, "ws-1", "deal-1")
	if err != nil {
		t.Fatalf("RefreshDealSummaryNow: %v", err)
	}
	if summary == nil || summary.EntityType != model.CRMObjectDeal || summary.Status != model.CRMEntitySummaryStatusReady {
		t.Fatalf("summary = %+v, want ready deal summary", summary)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
}

func TestCRMSummaryService_RefreshCompanySummaryNowGeneratesSparseAccountSummary(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()
	mustExecSummary(t, db, `CREATE TABLE crm_activities (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, activity_type TEXT NOT NULL,
		contact_id TEXT, company_id TEXT, deal_id TEXT, owner_member_id TEXT,
		subject TEXT, body TEXT, occurred_at DATETIME NOT NULL, metadata BLOB
	)`)
	mustExecSummary(t, db, `CREATE TABLE pm_tasks (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, updated_at DATETIME NOT NULL)`)
	mustExecSummary(t, db, `CREATE TABLE support_conversations (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, display_id INTEGER NOT NULL,
		subject TEXT NOT NULL, status TEXT NOT NULL, priority TEXT, channel TEXT,
		crm_contact_id TEXT, crm_company_id TEXT, updated_at DATETIME NOT NULL
	)`)
	mustExecSummary(t, db, `INSERT INTO crm_companies (id, workspace_id, display_id, name, domain, industry, description, custom_properties) VALUES (?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"company-1", "ws-1", "COM-1", "Monita", "monita.example", "SaaS", "Customer analytics platform", `{}`)

	provider := &fakeSummaryLLMProvider{
		content: `{"summary_markdown":"Monita is a SaaS account with limited recent engagement recorded.","highlights":[{"kind":"next_step","text":"Add the next customer touchpoint."}]}`,
	}
	svc := NewCRMSummaryService(
		repository.NewCRMSummaryRepository(db),
		repository.NewCRMContactRepository(db),
		repository.NewCRMCompanyRepository(db),
		repository.NewCRMDealRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewCRMSignalRepository(db),
		repository.NewCRMEmailRepository(db),
		provider,
		nil,
	).SetCompanyEvidenceRepositories(
		repository.NewCRMCompanyTimelineRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewSupportConversationRepository(db),
	)

	summary, err := svc.RefreshCompanySummaryNow(ctx, "ws-1", "company-1")
	if err != nil {
		t.Fatalf("RefreshCompanySummaryNow: %v", err)
	}
	if summary == nil || summary.EntityType != model.CRMObjectCompany || summary.Status != model.CRMEntitySummaryStatusReady {
		t.Fatalf("summary = %+v, want ready company summary", summary)
	}
	if intValue(summary.Metadata["source_contact_count"]) != 0 {
		t.Fatalf("source_contact_count = %#v, want zero", summary.Metadata["source_contact_count"])
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
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
			external_id TEXT,
			name TEXT NOT NULL,
			domain TEXT,
			industry TEXT,
			employee_count INTEGER,
			annual_revenue REAL,
			description TEXT,
			logo_url TEXT,
			linkedin_url TEXT,
			headquarters TEXT,
			owner_member_id TEXT,
			customer_success_owner_member_id TEXT,
			custom_properties BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_pipelines (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			default_commercial_motion TEXT NOT NULL DEFAULT 'new_business',
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
			commercial_motion TEXT,
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
			rfc_message_id TEXT,
			in_reply_to TEXT,
			references_header TEXT,
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
			company_id TEXT,
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
			company_id TEXT,
			signal_type TEXT NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'manual',
			source_id TEXT,
			source_thread_id TEXT,
			summary TEXT NOT NULL,
			evidence_excerpt TEXT,
			metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			confidence REAL NOT NULL DEFAULT 0,
			detected_at DATETIME NOT NULL,
			detector_kind TEXT NOT NULL DEFAULT 'direct',
			signal_domain TEXT NOT NULL DEFAULT 'conversation',
			polarity TEXT NOT NULL DEFAULT 'neutral',
			rule_key TEXT,
			rule_version TEXT,
			window_started_at DATETIME,
			window_ended_at DATETIME,
			evidence_identity_method TEXT NOT NULL DEFAULT 'unknown',
			evidence_identity_trust TEXT NOT NULL DEFAULT 'untrusted',
			evidence_fingerprint TEXT NOT NULL DEFAULT '',
			dismissed_at DATETIME,
			dismissed_by_member_id TEXT,
			dismissal_reason TEXT,
			reviewed_at DATETIME,
			acted_at DATETIME,
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
	mustExecSummary(t, db, `INSERT INTO crm_companies (id, workspace_id, display_id, name, custom_properties, updated_at) VALUES (?, ?, ?, ?, CAST(? AS BLOB), ?)`,
		"company-1", "ws-1", "COM-1", "Monita", `{}`, now)
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
	if result.DealsQueued != 1 || result.ContactsQueued != 1 || result.CompaniesQueued != 1 {
		t.Fatalf("result = %+v, want one deal, contact, and company queued", result)
	}
	if len(runner.inputs) != 3 {
		t.Fatalf("runner inputs = %d, want 3", len(runner.inputs))
	}
}

func TestCRMSummaryService_RequestCompanyRefreshForRelatedDeal(t *testing.T) {
	db := setupCRMSummaryTestDB(t)
	ctx := context.Background()
	runner := &fakeSummaryWorkflowRunner{}
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
	svc.runner = runner

	mustExecSummary(t, db, `INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES (?, ?, ?, ?, ?, ?)`,
		"assoc-company-contact", "ws-1", model.CRMObjectCompany, "company-1", model.CRMObjectContact, "contact-1")
	mustExecSummary(t, db, `INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES (?, ?, ?, ?, ?, ?)`,
		"assoc-contact-deal", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectDeal, "deal-1")

	if err := svc.RequestCompanyRefreshForObject(ctx, "ws-1", model.CRMObjectDeal, "deal-1"); err != nil {
		t.Fatalf("RequestCompanyRefreshForObject: %v", err)
	}
	if len(runner.inputs) != 1 {
		t.Fatalf("runner inputs = %d, want one company refresh", len(runner.inputs))
	}
	if got := runner.inputs[0]; got.EntityType != model.CRMObjectCompany || got.EntityID != "company-1" {
		t.Fatalf("refresh input = %+v, want company-1", got)
	}
}

func TestSampleCompanySummaryActivityAppliesWindowCapsAndDeduplication(t *testing.T) {
	now := time.Now().UTC()
	rows := make([]model.CRMCompanyTimelineItem, 0, 16)
	for i := 0; i < 14; i++ {
		rows = append(rows, model.CRMCompanyTimelineItem{
			ID: fmt.Sprintf("email-%d", i), Kind: model.CRMCompanyTimelineFilterEmail,
			EventType: "email.inbound", SourceType: "crm_email_message", SourceID: fmt.Sprintf("message-%d", i),
			Title: fmt.Sprintf("Email %d", i), OccurredAt: now.Add(-time.Duration(i) * time.Hour),
		})
	}
	rows = append(rows,
		model.CRMCompanyTimelineItem{ID: "duplicate", Kind: model.CRMCompanyTimelineFilterEmail, EventType: "email.inbound", SourceType: "crm_email_message", SourceID: "message-0", Title: "Duplicate", OccurredAt: now},
		model.CRMCompanyTimelineItem{ID: "old-note", Kind: model.CRMCompanyTimelineFilterNote, EventType: "activity.note", SourceType: "crm_activity", SourceID: "old-note", Title: "Old", OccurredAt: now.AddDate(0, 0, -91)},
	)

	activities, counts, times := sampleCompanySummaryActivity(rows, now.AddDate(0, 0, -90))
	if len(activities) != 12 || counts[model.CRMCompanyTimelineFilterEmail] != 12 || len(times) != 12 {
		t.Fatalf("activities=%d counts=%v times=%d, want 12 capped emails", len(activities), counts, len(times))
	}
	for _, activity := range activities {
		if activity.Title == "Duplicate" || activity.Title == "Old" {
			t.Fatalf("unexpected sampled activity %+v", activity)
		}
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

func TestBuildSummaryReadinessCountsDistinctArtifacts(t *testing.T) {
	now := time.Now().UTC()
	rows := []model.CRMTimelineItem{
		{ID: "email-2", Kind: model.CRMTimelineFilterEmail, SourceID: "message-2", Title: "Re: Pricing", OccurredAt: now, Entity: &model.CRMTimelineReference{Type: "email_thread", ID: "thread-1", Name: "Pricing"}},
		{ID: "email-1", Kind: model.CRMTimelineFilterEmail, SourceID: "message-1", Title: "Pricing", OccurredAt: now.Add(-time.Hour), Entity: &model.CRMTimelineReference{Type: "email_thread", ID: "thread-1", Name: "Pricing"}},
		{ID: "deal-1", Kind: model.CRMTimelineFilterDeal, SourceID: "deal-1", Title: "Pro plan", OccurredAt: now.Add(-2 * time.Hour), Entity: &model.CRMTimelineReference{Type: "deal", ID: "deal-1", Name: "Pro plan"}},
		{ID: "note-1", Kind: model.CRMTimelineFilterNote, SourceID: "note-1", Title: "Security review", OccurredAt: now.Add(-3 * time.Hour)},
	}

	readiness, sources := buildSummaryReadiness(rows, now.AddDate(0, 0, -90))
	if !readiness.Ready || readiness.Count != 3 {
		t.Fatalf("readiness = %#v, want three distinct ready artifacts", readiness)
	}
	if len(sources) != 3 || sources[0].ThreadID != "thread-1" {
		t.Fatalf("sources = %#v, want deduplicated email thread first", sources)
	}
	if readiness.Steps[0].Label != "Email logged" || readiness.Steps[2].Label != "Note added" {
		t.Fatalf("steps = %#v, want artifact-specific labels", readiness.Steps)
	}
}

func TestParseSummaryGenerationOutputRejectsEmptyResponseCleanly(t *testing.T) {
	_, err := parseSummaryGenerationOutput("")
	if err == nil || strings.Contains(err.Error(), "unexpected end of JSON") {
		t.Fatalf("error = %v, want a clean empty-response error", err)
	}
}

func TestGenerateSummaryLLMRetriesAnEmptyResponse(t *testing.T) {
	provider := &fakeSummaryLLMProvider{contents: []string{
		"",
		`{"summary_markdown":"The company has active buyer engagement.","highlights":[]}`,
	}}
	svc := &CRMSummaryService{llmProvider: provider}

	output, err := svc.generateSummaryLLM(context.Background(), AIUsageMeteringContext{
		WorkspaceID:    "ws-1",
		FeatureKey:     BillingFeatureCRMSummary,
		IdempotencyKey: "summary-retry-test",
	}, summaryPromptEntity{
		EntityType: model.CRMObjectCompany,
		Company:    &companySummarySnapshot{ID: "company-1", Name: "Acme"},
	})
	if err != nil {
		t.Fatalf("generateSummaryLLM: %v", err)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want fallback retry", provider.calls)
	}
	if output.SummaryMarkdown != "The company has active buyer engagement." {
		t.Fatalf("summary = %q", output.SummaryMarkdown)
	}
}

func mustExecSummary(t *testing.T, db *gorm.DB, stmt string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(stmt, args...).Error; err != nil {
		t.Fatalf("exec %q failed: %v", stmt, err)
	}
}
