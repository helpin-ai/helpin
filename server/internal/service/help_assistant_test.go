package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func createHelpAssistantTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS help_assistant_sessions (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL, surface TEXT NOT NULL, audience TEXT NOT NULL,
		portal_identity_id TEXT, visitor_id TEXT NOT NULL, locale TEXT NOT NULL DEFAULT '',
		turn_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME)`)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS help_assistant_messages (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		session_id TEXT NOT NULL, workspace_id TEXT NOT NULL, role TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, progress_label TEXT, citations TEXT,
		page_article_id TEXT, run_id TEXT, helpful BOOLEAN, created_at DATETIME, updated_at DATETIME, completed_at DATETIME)`)
}

func copySupportRunEvidenceSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	var schema string
	if err := setupSupportKnowledgeTestDB(t).Raw("SELECT sql FROM sqlite_master WHERE name='support_run_evidence'").Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, schema)
}

// helpAssistantLaunchFixture is a workspace with the help assistant turned
// on, its system agent, and a fake Agent Runtime.
func helpAssistantLaunchFixture(t *testing.T) (*gorm.DB, *HelpAssistantService, *fakeAgentRuntimeSignalClient) {
	t.Helper()
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeInteractive, now)
	mustExec(t, db, `UPDATE agents SET workspace_id='ws-1', name='Help assistant', preset_key=?, preset_version_key='help_assistant_default', allowed_tools=?, allowed_targets=?, approval_mode='never' WHERE id='agent-1'`,
		model.AgentPresetHelpAssistant, []byte(`["search_help","read_help_article","send_help_answer","send_help_progress","offer_contact_support"]`), []byte(`["help_assistant_turn"]`))
	mustExec(t, db, `CREATE TABLE support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT, widget_key TEXT, settings TEXT, active BOOLEAN)`)
	mustExec(t, db, `INSERT INTO support_widget_installations VALUES ('installation','ws-1','wk','{"help_assistant_enabled":true}',true)`)
	mustExec(t, db, `CREATE TABLE docs_helpcenter_configs (id TEXT PRIMARY KEY, workspace_id TEXT, subdomain TEXT, default_locale TEXT, is_published BOOLEAN)`)
	mustExec(t, db, `INSERT INTO docs_helpcenter_configs VALUES ('cfg','ws-1','acme','en',true)`)
	createHelpAssistantTables(t, db)
	copySupportRunEvidenceSchema(t, db)
	runtime := &fakeAgentRuntimeSignalClient{}
	agents := newDelegatedSupportRunService(t, db, runtime)
	inbox := &SupportInboxService{installationRepo: repository.NewSupportInboxInstallationRepository(db)}
	assistant := NewHelpAssistantService(repository.NewHelpAssistantRepository(db), agents, inbox,
		repository.NewDocsHelpcenterRepository(db), nil, nil, nil, repository.NewSupportRunEvidenceRepository(db))
	return db, assistant, runtime
}

var helpVisitor = HelpAssistantAsker{WorkspaceID: "ws-1", Surface: model.HelpAssistantSurfaceHelpcenter, VisitorID: "visitor-123456"}

func TestHelpAssistantAskStartsALightweightTurn(t *testing.T) {
	db, assistant, runtime := helpAssistantLaunchFixture(t)
	ctx := context.Background()
	turn, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{Message: "How do I export a report?"})
	if err != nil {
		t.Fatal(err)
	}
	if len(runtime.startRunCalls) != 1 {
		t.Fatalf("runtime starts = %d, want 1", len(runtime.startRunCalls))
	}
	req := runtime.startRunCalls[0]
	if req.ExecutionMode != agentRuntimeExecutionModeLightweight || req.TurnPolicy.Mode != agentRuntimeTurnCompleteOnFinish {
		t.Fatalf("execution = %q / %q, want lightweight complete_on_finish", req.ExecutionMode, req.TurnPolicy.Mode)
	}
	if req.Target.Type != model.HelpAssistantTurnTarget || req.Target.ID != turn.AnswerID || len(req.MCPServers) != 0 {
		t.Fatalf("target = %+v", req.Target)
	}
	for _, tool := range req.AllowedTools {
		if !slices.Contains(agentcontract.HelpAssistantTools, tool) {
			t.Fatalf("unsafe tool %s", tool)
		}
	}
	if !strings.Contains(req.Instructions, "New question:\nHow do I export a report?") || !strings.Contains(req.Instructions, "public help center") {
		t.Fatalf("instructions = %q", req.Instructions)
	}

	view, err := assistant.Session(ctx, helpVisitor, turn.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Messages) != 2 || view.Messages[0].Role != "user" || view.Messages[1].Status != model.HelpAssistantStatusPending || view.Messages[1].ProgressLabel == "" {
		t.Fatalf("session = %+v", view.Messages)
	}
	var runID string
	if err := db.Raw("SELECT run_id FROM help_assistant_messages WHERE id = ?", turn.AnswerID).Scan(&runID).Error; err != nil || runID == "" {
		t.Fatalf("run id = %q, %v", runID, err)
	}

	// A second question waits for the first answer.
	if _, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{SessionID: turn.SessionID, Message: "And to CSV?"}); !errors.Is(err, ErrHelpAssistantPending) {
		t.Fatalf("concurrent question error = %v", err)
	}
	// The follow-up carries the conversation.
	mustExec(t, db, `UPDATE help_assistant_messages SET status='answered', content='Open Reports and choose Export.' WHERE id = ?`, turn.AnswerID)
	if _, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{SessionID: turn.SessionID, Message: "And to CSV?"}); err != nil {
		t.Fatal(err)
	}
	if got := runtime.startRunCalls[1].Instructions; !strings.Contains(got, "Person: How do I export a report?\nYou: Open Reports and choose Export.") {
		t.Fatalf("follow-up instructions = %q", got)
	}
}

func TestHelpAssistantSessionsBelongToTheirVisitorAndAudience(t *testing.T) {
	_, assistant, _ := helpAssistantLaunchFixture(t)
	ctx := context.Background()
	turn, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{Message: "How do I export a report?"})
	if err != nil {
		t.Fatal(err)
	}
	for name, asker := range map[string]HelpAssistantAsker{
		"other visitor":     {WorkspaceID: "ws-1", Surface: model.HelpAssistantSurfaceHelpcenter, VisitorID: "visitor-other1"},
		"other surface":     {WorkspaceID: "ws-1", Surface: model.HelpAssistantSurfacePortal, VisitorID: helpVisitor.VisitorID},
		"signed in later":   {WorkspaceID: "ws-1", Surface: model.HelpAssistantSurfaceHelpcenter, VisitorID: helpVisitor.VisitorID, PortalIdentityID: "identity-1"},
		"other workspace":   {WorkspaceID: "ws-2", Surface: model.HelpAssistantSurfaceHelpcenter, VisitorID: helpVisitor.VisitorID},
		"malformed visitor": {WorkspaceID: "ws-1", Surface: model.HelpAssistantSurfaceHelpcenter, VisitorID: "x"},
	} {
		if _, err := assistant.Session(ctx, asker, turn.SessionID); !errors.Is(err, ErrHelpAssistantNotFound) {
			t.Errorf("%s: session error = %v, want not found", name, err)
		}
	}
	if _, err := assistant.Session(ctx, helpVisitor, "not-a-uuid"); !errors.Is(err, ErrHelpAssistantNotFound) {
		t.Errorf("malformed session id error = %v", err)
	}
}

func TestHelpAssistantLimits(t *testing.T) {
	db, assistant, runtime := helpAssistantLaunchFixture(t)
	ctx := context.Background()
	for name, text := range map[string]string{"empty": " ", "too long": strings.Repeat("a", helpAssistantMaxQuestionRunes+1)} {
		if _, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{Message: text}); !errors.Is(err, ErrHelpAssistantInvalid) {
			t.Errorf("%s question error = %v", name, err)
		}
	}
	turn, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{Message: "How do I export a report?"})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE help_assistant_messages SET status='answered' WHERE id = ?`, turn.AnswerID)

	mustExec(t, db, `UPDATE help_assistant_sessions SET turn_count = ? WHERE id = ?`, helpAssistantSessionTurns, turn.SessionID)
	if _, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{SessionID: turn.SessionID, Message: "One more?"}); !errors.Is(err, ErrHelpAssistantSessionFull) {
		t.Errorf("full session error = %v", err)
	}

	for i := 0; i < helpAssistantConcurrentTurns; i++ {
		mustExec(t, db, `INSERT INTO help_assistant_messages (session_id, workspace_id, role, status, created_at) VALUES ('s', 'ws-1', 'assistant', 'pending', ?)`, time.Now().UTC())
	}
	if _, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{Message: "Is anyone there?"}); !errors.Is(err, ErrHelpAssistantBusy) {
		t.Errorf("busy error = %v", err)
	}
	mustExec(t, db, `DELETE FROM help_assistant_messages WHERE session_id = 's'`)

	mustExec(t, db, `UPDATE support_widget_installations SET settings = '{"help_assistant_enabled":false}'`)
	if _, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{Message: "Is it off?"}); !errors.Is(err, ErrHelpAssistantUnavailable) {
		t.Errorf("turned off error = %v", err)
	}
	if assistant.Available(ctx, "ws-1") {
		t.Error("assistant reports available while turned off")
	}
	if len(runtime.startRunCalls) != 1 {
		t.Errorf("runtime starts = %d, want only the first question", len(runtime.startRunCalls))
	}
}

func TestHelpAssistantTimesOutAnUnansweredTurn(t *testing.T) {
	db, assistant, runtime := helpAssistantLaunchFixture(t)
	ctx := context.Background()
	turn, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{Message: "How do I export a report?"})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE agent_runs SET status='running', external_runtime='agent-runtime', external_runtime_id='rt-1'`)
	mustExec(t, db, `UPDATE help_assistant_messages SET created_at = ? WHERE id = ?`, time.Now().UTC().Add(-2*helpAssistantTurnTimeout), turn.AnswerID)
	view, err := assistant.Session(ctx, helpVisitor, turn.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range view.Messages {
		if got.ID == turn.AnswerID && (got.Status != model.HelpAssistantStatusFailed || got.ProgressLabel != "") {
			t.Fatalf("timed out answer = %+v", got)
		}
	}
	if len(runtime.cancelCalls) != 1 {
		t.Fatalf("runtime cancels = %v, want the turn's run", runtime.cancelCalls)
	}
}

// helpToolsFixture has a public and a customers-only article, each indexed,
// and a session with a pending turn.
func helpToolsFixture(t *testing.T, audience string) (*gorm.DB, *HelpAssistantService, model.InternalCommandContext, *DocsHelpcenterService) {
	t.Helper()
	ctx := context.Background()
	db, hc := setupAudienceTestDB(t)
	if err := hc.PublishExternally(ctx, audienceDocumentID, "", audienceContent); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{ID: "space-public", WorkspaceID: audienceWorkspaceID, Name: "Guides", Slug: "guides", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable, HelpcenterAudience: model.HelpcenterAudiencePublic, CreatedBy: "user-1", CreatedAt: now, UpdatedAt: now})
	guidesSlug := "guides"
	seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{ID: "st-guides", SpaceID: "space-public", WorkspaceID: audienceWorkspaceID, Locale: "en", Name: "Guides", Slug: &guidesSlug, Status: model.DocsHelpcenterTranslationStatusPublished, PublishedAt: &now})
	seedDocsHelpcenterTranslationServiceDocument(t, db, model.DocsDocument{ID: "doc-export", WorkspaceID: audienceWorkspaceID, SpaceID: "space-public", Title: "Export reports", Status: model.DocStatusPublished, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: "user-1", PublishedAt: &now, CreatedAt: now, UpdatedAt: now})
	exportContent := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Open Reports and choose Export."}]}]}`)
	seedDocsHelpcenterTranslationServiceContent(t, db, model.DocsContent{ID: "content-export", DocumentID: "doc-export", Content: exportContent, ContentText: "Open Reports and choose Export.", CreatedAt: now, UpdatedAt: now})
	if err := hc.PublishExternally(ctx, "doc-export", "", exportContent); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO docs_chunks (id, workspace_id, space_id, document_id, chunk_index, title, content) VALUES
		('chunk-private', ?, ?, ?, 0, 'Renewal pricing', 'Renewal pricing for enterprise plans is set per contract.'),
		('chunk-public', ?, 'space-public', 'doc-export', 0, 'Export reports', 'Open Reports and choose Export.')`,
		audienceWorkspaceID, audienceSpaceID, audienceDocumentID, audienceWorkspaceID)
	createHelpAssistantTables(t, db)
	copySupportRunEvidenceSchema(t, db)
	identity := ""
	if audience == model.HelpcenterAudienceCustomers {
		identity = "identity-1"
	}
	mustExec(t, db, `INSERT INTO help_assistant_sessions (id, workspace_id, surface, audience, portal_identity_id, visitor_id, locale) VALUES ('11111111-1111-1111-1111-111111111111', ?, 'portal', ?, NULLIF(?, ''), 'visitor-123456', 'en')`, audienceWorkspaceID, audience, identity)
	mustExec(t, db, `INSERT INTO help_assistant_messages (id, session_id, workspace_id, role, status, created_at) VALUES ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', ?, 'assistant', 'pending', ?)`, audienceWorkspaceID, now)
	knowledge := &HelpcenterAISearchService{chunkRepo: repository.NewDocsChunkRepository(db), searchRepo: repository.NewDocsSearchRepository(db)}
	assistant := NewHelpAssistantService(repository.NewHelpAssistantRepository(db), nil, nil, repository.NewDocsHelpcenterRepository(db, false),
		repository.NewDocsHelpcenterPublicationRepository(db), repository.NewDocsChunkRepository(db), knowledge, repository.NewSupportRunEvidenceRepository(db))
	meta := model.InternalCommandContext{WorkspaceID: audienceWorkspaceID, RunID: "33333333-3333-3333-3333-333333333333", TargetType: model.HelpAssistantTurnTarget, TargetID: "22222222-2222-2222-2222-222222222222"}
	return db, assistant, meta, hc
}

func helpResultArticles(results []HelpSearchResult) []string {
	var ids []string
	for _, result := range results {
		ids = append(ids, result.ArticleID)
	}
	slices.Sort(ids)
	return ids
}

func TestHelpAssistantToolsKeepPrivateArticlesFromPublicSessions(t *testing.T) {
	ctx := context.Background()
	_, assistant, meta, _ := helpToolsFixture(t, model.HelpcenterAudiencePublic)
	results, err := assistant.SearchHelp(ctx, meta, "renewal pricing")
	if err != nil {
		t.Fatal(err)
	}
	if got := helpResultArticles(results); !slices.Equal(got, []string{"doc-export"}) {
		t.Fatalf("public session found %v, want only the public article", got)
	}
	if _, err := assistant.ReadArticle(ctx, meta, audienceDocumentID); err == nil {
		t.Fatal("a public session read an article for signed-in customers")
	}
	if err := assistant.SendAnswer(ctx, meta, "Pricing is per contract.", []string{"chunk-private"}); err == nil {
		t.Fatal("a public session cited evidence it was never given")
	}
}

func TestHelpAssistantToolsAnswerSignedInCustomers(t *testing.T) {
	ctx := context.Background()
	db, assistant, meta, _ := helpToolsFixture(t, model.HelpcenterAudienceCustomers)
	results, err := assistant.SearchHelp(ctx, meta, "renewal pricing")
	if err != nil {
		t.Fatal(err)
	}
	if got := helpResultArticles(results); !slices.Equal(got, []string{"doc-export", audienceDocumentID}) {
		t.Fatalf("customer session found %v, want both articles", got)
	}
	article, err := assistant.ReadArticle(ctx, meta, audienceDocumentID)
	if err != nil || !strings.Contains(article["text"].(string), "set per contract") {
		t.Fatalf("read article = %v, %v", article, err)
	}

	for name, answer := range map[string]string{"link": "See https://example.com", "markdown link": "See [pricing](/pricing)"} {
		if err := assistant.SendAnswer(ctx, meta, answer, []string{"chunk-private"}); err == nil {
			t.Errorf("%s: answer with a link was accepted", name)
		}
	}
	if err := assistant.SendAnswer(ctx, meta, "Pricing is per contract.", nil); err == nil {
		t.Error("an answer without citations was accepted")
	}
	if err := assistant.SendAnswer(ctx, meta, "Pricing is **per contract**.", []string{"chunk-private", "article:" + audienceDocumentID, "chunk-public"}); err != nil {
		t.Fatal(err)
	}
	var saved model.HelpAssistantMessage
	if err := db.First(&saved, "id = ?", meta.TargetID).Error; err != nil {
		t.Fatal(err)
	}
	var citations []model.HelpAssistantCitation
	_ = json.Unmarshal([]byte(derefString(saved.Citations)), &citations)
	if saved.Status != model.HelpAssistantStatusAnswered || len(citations) != 2 ||
		citations[0].ArticleID != audienceDocumentID || citations[0].PortalArticleID == "" || citations[0].URL != "" ||
		citations[1].ArticleID != "doc-export" {
		t.Fatalf("saved answer = %+v citations %+v", saved, citations)
	}
	// The turn is over: tools stop working.
	if _, err := assistant.SearchHelp(ctx, meta, "more"); err == nil {
		t.Fatal("search ran after the turn finished")
	}
	if err := assistant.OfferContactSupport(ctx, meta, "Contact us."); err == nil {
		t.Fatal("a finished turn was answered twice")
	}
}

func TestHelpAssistantContactSupportAndProgress(t *testing.T) {
	ctx := context.Background()
	db, assistant, meta, _ := helpToolsFixture(t, model.HelpcenterAudiencePublic)
	if err := assistant.SetProgress(ctx, meta, "Reading the export guide…"); err != nil {
		t.Fatal(err)
	}
	if err := assistant.SetProgress(ctx, meta, "see https://x.test"); err == nil {
		t.Fatal("a progress label with a link was accepted")
	}
	if err := assistant.OfferContactSupport(ctx, meta, "Visit https://evil.test"); err != nil {
		t.Fatal(err)
	}
	var saved model.HelpAssistantMessage
	if err := db.First(&saved, "id = ?", meta.TargetID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.HelpAssistantStatusHandedOff || saved.Content != helpAssistantDefaultHandoff || saved.ProgressLabel != nil {
		t.Fatalf("handoff = %+v", saved)
	}

	wrongTarget := meta
	wrongTarget.TargetType = "support_conversation"
	if _, err := assistant.SearchHelp(ctx, wrongTarget, "export"); err == nil {
		t.Fatal("help tools ran outside a help assistant turn")
	}
}

func TestHelpAssistantFinalizeClosesUnansweredTurn(t *testing.T) {
	ctx := context.Background()
	db, assistant, meta, _ := helpToolsFixture(t, model.HelpcenterAudiencePublic)
	if err := assistant.FinalizeRun(ctx, &model.AgentRun{ID: meta.RunID, WorkspaceID: meta.WorkspaceID, TargetType: model.HelpAssistantTurnTarget, TargetID: meta.TargetID, Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.Raw("SELECT status FROM help_assistant_messages WHERE id = ?", meta.TargetID).Scan(&status).Error; err != nil || status != model.HelpAssistantStatusFailed {
		t.Fatalf("status = %q, %v", status, err)
	}
}

func TestHelpAssistantCommandGate(t *testing.T) {
	db, assistant, _ := helpAssistantLaunchFixture(t)
	ctx := context.Background()
	turn, err := assistant.Ask(ctx, helpVisitor, HelpAssistantQuestion{Message: "How do I export a report?"})
	if err != nil {
		t.Fatal(err)
	}
	var runID string
	if err := db.Raw("SELECT run_id FROM help_assistant_messages WHERE id = ?", turn.AnswerID).Scan(&runID).Error; err != nil {
		t.Fatal(err)
	}
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commands.SetAgentRunDependencies(assistant.agents.runRepo, nil)
	commands.SetHelpAssistant(assistant)
	own := model.InternalCommandContext{WorkspaceID: "ws-1", RunID: runID, AgentID: "agent-1", TargetType: model.HelpAssistantTurnTarget, TargetID: turn.AnswerID}

	if _, err := commands.Execute(ctx, own, "support.search_knowledge", json.RawMessage(`{"queries":["export"]}`)); err == nil || !strings.Contains(err.Error(), "scope mismatch") {
		t.Errorf("a help run used a support tool: %v", err)
	}
	otherTurn := own
	otherTurn.TargetID = "44444444-4444-4444-4444-444444444444"
	if _, err := commands.Execute(ctx, otherTurn, "help.progress", json.RawMessage(`{"label":"Working"}`)); err == nil || !strings.Contains(err.Error(), "scope mismatch") {
		t.Errorf("a help run acted on another turn: %v", err)
	}
	if _, err := commands.Execute(ctx, model.InternalCommandContext{WorkspaceID: "ws-1", TargetType: "support_conversation", TargetID: "conv-1"}, "help.search", json.RawMessage(`{"query":"export"}`)); err == nil {
		t.Error("a help tool ran outside a help assistant run")
	}
	if _, err := commands.Execute(ctx, own, "help.progress", json.RawMessage(`{"label":"Reading the export guide…"}`)); err != nil {
		t.Fatalf("own progress: %v", err)
	}

	// The runtime calls tools with its own run id; evidence is Helpin's.
	mustExec(t, db, `UPDATE agent_runs SET external_runtime = ?, external_runtime_id = 'run_runtime_1' WHERE id = ?`, agentRuntimeName, runID)
	mustExec(t, db, `INSERT INTO support_run_evidence (id, workspace_id, run_id, evidence_id, source_type, document_id, title, url, content) VALUES ('e1', 'ws-1', ?, 'chunk-1', 'help_article', 'doc-1', 'Export reports', 'https://help.test/export', 'Open Reports.')`, runID)
	runtimeMeta := own
	runtimeMeta.RunID = "run_runtime_1"
	if _, err := commands.Execute(ctx, runtimeMeta, "help.send_answer", json.RawMessage(`{"answer":"Open Reports.","citations":["chunk-1"]}`)); err != nil {
		t.Fatalf("answer with the runtime's run id: %v", err)
	}
}

func TestHelpAssistantHandoffFeedsCoverageAndReports(t *testing.T) {
	ctx := context.Background()
	db, assistant, meta, _ := helpToolsFixture(t, model.HelpcenterAudiencePublic)
	recorder := &captureSupportEventRecorder{}
	assistant.inbox = &SupportInboxService{supportEventRecorder: recorder}
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO help_assistant_messages (session_id, workspace_id, role, content, status, created_at) VALUES ('11111111-1111-1111-1111-111111111111', ?, 'user', 'Can I get a refund for last month?', 'answered', ?)`, audienceWorkspaceID, now.Add(-time.Second))
	if err := assistant.OfferContactSupport(ctx, meta, "The team can help with refunds."); err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 || recorder.events[0].SourceSignal != "no_results" || recorder.events[0].IssueSummary != "Can I get a refund for last month?" {
		t.Fatalf("coverage events = %+v", recorder.events)
	}

	mustExec(t, db, `INSERT INTO help_assistant_messages (session_id, workspace_id, role, content, status, helpful, created_at) VALUES
		('11111111-1111-1111-1111-111111111111', ?, 'user', 'How do I export?', 'answered', NULL, ?),
		('11111111-1111-1111-1111-111111111111', ?, 'assistant', 'Open Reports.', 'answered', true, ?)`,
		audienceWorkspaceID, now, audienceWorkspaceID, now)
	report, err := repository.NewSupportConversationRepository(db).ReportHelpAssistant(ctx, audienceWorkspaceID, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if report == nil || report.Conversations != 1 || report.Questions != 2 || report.Answered != 1 || report.HandedOff != 1 || report.Helpful != 1 ||
		len(report.Unanswered) != 1 || report.Unanswered[0].Question != "Can I get a refund for last month?" {
		t.Fatalf("report = %+v", report)
	}
	if empty, err := repository.NewSupportConversationRepository(db).ReportHelpAssistant(ctx, "other-workspace", now.Add(-time.Hour), now.Add(time.Hour)); err != nil || empty != nil {
		t.Fatalf("report without questions = %+v, %v", empty, err)
	}
}

func TestHelpAssistantSearchFallsBackToKeywords(t *testing.T) {
	ctx := context.Background()
	db, assistant, meta, _ := helpToolsFixture(t, model.HelpcenterAudienceCustomers)
	// Articles not chunked yet are still found by their text, public ones only.
	mustExec(t, db, `DELETE FROM docs_chunks`)
	results, err := assistant.SearchHelp(ctx, meta, "Export reports")
	if err != nil {
		t.Fatal(err)
	}
	if got := helpResultArticles(results); !slices.Equal(got, []string{"doc-export"}) || results[0].EvidenceID != "article:doc-export" {
		t.Fatalf("keyword results = %+v", results)
	}
	if err := assistant.SendAnswer(ctx, meta, "Open Reports and choose Export.", []string{"article:doc-export"}); err != nil {
		t.Fatalf("cite a keyword result: %v", err)
	}
}

func TestPortalIntakeCarriesTheAssistantSession(t *testing.T) {
	intakeID := "55555555-5555-5555-5555-555555555555"
	intake, err := normalizePortalIntake(&model.PortalIntake{IntakeID: intakeID, AssistantSessionID: " 11111111-1111-1111-1111-111111111111 "})
	if err != nil || intake.AssistantSessionID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("intake = %+v, %v", intake, err)
	}
	if _, err := normalizePortalIntake(&model.PortalIntake{IntakeID: intakeID, AssistantSessionID: "not-a-session"}); !errors.Is(err, ErrPortalIntakeInvalid) {
		t.Fatalf("invalid session error = %v", err)
	}
}

func TestHelpAssistantTranscriptForTeammates(t *testing.T) {
	ctx := context.Background()
	db, assistant, meta, _ := helpToolsFixture(t, model.HelpcenterAudiencePublic)
	mustExec(t, db, `INSERT INTO help_assistant_messages (session_id, workspace_id, role, content, status, created_at) VALUES ('11111111-1111-1111-1111-111111111111', ?, 'user', 'Can I get a refund?', 'answered', ?)`, audienceWorkspaceID, time.Now().UTC().Add(-time.Second))
	if err := assistant.OfferContactSupport(ctx, meta, "The team can help with refunds."); err != nil {
		t.Fatal(err)
	}
	turns, err := assistant.Transcript(ctx, audienceWorkspaceID, "11111111-1111-1111-1111-111111111111")
	if err != nil || len(turns) != 2 || turns[0].Content != "Can I get a refund?" || turns[1].Status != model.HelpAssistantStatusHandedOff {
		t.Fatalf("transcript = %+v, %v", turns, err)
	}
	for name, id := range map[string]string{"other workspace": "11111111-1111-1111-1111-111111111111", "malformed": "x"} {
		workspace := audienceWorkspaceID
		if name == "other workspace" {
			workspace = "other"
		}
		if turns, err := assistant.Transcript(ctx, workspace, id); err != nil || len(turns) != 0 {
			t.Errorf("%s: transcript = %+v, %v", name, turns, err)
		}
	}
}

func TestReaderHelpLocale(t *testing.T) {
	cfg := &model.DocsHelpcenterConfig{DefaultLocale: "en", EnabledLocales: model.DocsStringArray{"en", "de", "pt-BR"}}
	for requested, want := range map[string]string{"de": "de", "de-AT": "de", "pt-br": "pt-BR", "fr": "en", "": "en"} {
		if got := readerHelpLocale(cfg, requested); got != want {
			t.Errorf("readerHelpLocale(%q) = %q, want %q", requested, got, want)
		}
	}
}

func TestPortalHelpReadsTranslationsWhereTheyExist(t *testing.T) {
	ctx := context.Background()
	db, svc := setupAudienceTestDB(t)
	if err := svc.PublishExternally(ctx, audienceDocumentID, "", audienceContent); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.DocsHelpcenterConfig{}).Where("workspace_id = ?", audienceWorkspaceID).Update("enabled_locales", model.DocsStringArray{"en", "de"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&model.DocsHelpcenterArticlePublication{ID: "pub-de", DocumentID: audienceDocumentID, WorkspaceID: audienceWorkspaceID, SpaceID: audienceSpaceID, Locale: "de", Title: "Verlängerungspreise", Slug: "preise",
		Content: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Pro Vertrag verhandelt."}]}]}`), PublishedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	portal := &CustomerPortalService{inbox: &SupportInboxService{docsHelpcenterRepo: repository.NewDocsHelpcenterRepository(db, false)}}
	ws := &PortalWorkspace{ID: audienceWorkspaceID}

	german, err := portal.HelpArticles(ctx, ws, "de-AT")
	if err != nil || len(german) != 1 || german[0].Title != "Verlängerungspreise" {
		t.Fatalf("German list = %+v, %v", german, err)
	}
	detail, err := portal.HelpArticle(ctx, ws, german[0].ID, "de")
	if err != nil || !strings.Contains(detail.ContentHTML, "Pro Vertrag verhandelt.") {
		t.Fatalf("German article = %+v, %v", detail, err)
	}
	// No French translation: the default language is shown.
	french, err := portal.HelpArticles(ctx, ws, "fr")
	if err != nil || len(french) != 1 || french[0].Title != "Renewal pricing" {
		t.Fatalf("French list = %+v, %v", french, err)
	}
}
