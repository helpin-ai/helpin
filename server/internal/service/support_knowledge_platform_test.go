package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestNormalizeSupportIntentUsesSourceSufficiencyForEveryCompany(t *testing.T) {
	definition := normalizeSupportIntent(supportIntentPricingGeneral, []string{"company_specific_field"})
	if definition.ID != supportIntentPricingGeneral || definition.EvidenceMode != supportEvidenceModeSufficiency || len(definition.RequiredEvidence) != 0 {
		t.Fatalf("pricing should be evaluated from retrieved sources without hard-coded fields: %+v", definition)
	}
}

func TestChunkStructuredDocumentPrependsHeadingContextForSearch(t *testing.T) {
	chunks := chunkStructuredDocument("Plans", "# Pricing\n## Growth\nGrowth starts at $49 per month.\n## Enterprise\nContact sales for a custom annual plan.")
	if len(chunks) != 2 {
		t.Fatalf("expected 2 section chunks, got %d", len(chunks))
	}
	if chunks[0].HeadingPath != "Pricing > Growth" {
		t.Fatalf("unexpected heading path: %q", chunks[0].HeadingPath)
	}
	if !strings.Contains(chunks[0].SearchContent, "Plans\nPricing > Growth\n") {
		t.Fatalf("search content lacks title and heading context: %q", chunks[0].SearchContent)
	}
	if chunks[0].SectionKey == chunks[1].SectionKey {
		t.Fatal("different sections must have different stable section keys")
	}
	previous, next := neighborChunkIndexes(0, len(chunks))
	if previous != nil || next == nil || *next != 1 {
		t.Fatalf("unexpected neighbor indexes: previous=%v next=%v", previous, next)
	}
}

func TestStructuredSectionKeysSurviveUnrelatedSectionInsertion(t *testing.T) {
	before := chunkStructuredDocument("Plans", "# Pricing\n## Growth\nGrowth starts at $49.\n## Enterprise\nContact sales.")
	after := chunkStructuredDocument("Plans", "# Pricing\n## Free\nFree plan details.\n## Growth\nGrowth starts at $49.\n## Enterprise\nContact sales.")
	if len(before) != 2 || len(after) != 3 {
		t.Fatalf("unexpected chunks before=%d after=%d", len(before), len(after))
	}
	if before[0].SectionKey != after[1].SectionKey || before[1].SectionKey != after[2].SectionKey {
		t.Fatal("unrelated section insertion changed stable section keys")
	}
}

func TestCrawlRecordTextPreservesMarkdownAndHTMLSections(t *testing.T) {
	markdown, format := crawlRecordText(crawler.CrawlRecord{Markdown: "# Pricing\n\n## Growth\nGrowth starts at $49 per month."})
	if format != model.ContentSourceFormatMarkdown || !strings.Contains(markdown, "## Growth\nGrowth starts") {
		t.Fatalf("Markdown structure was flattened: %q", markdown)
	}
	markdownChunks := chunkStructuredDocument("Plans", markdown)
	if len(markdownChunks) != 1 || markdownChunks[0].HeadingPath != "Pricing > Growth" {
		t.Fatalf("unexpected Markdown chunks: %+v", markdownChunks)
	}

	htmlText, format := crawlRecordText(crawler.CrawlRecord{HTML: "<h1>Pricing</h1><h2>Growth</h2><p>Growth starts at $49 per month.</p>"})
	if format != model.ContentSourceFormatHTML || !strings.Contains(htmlText, "## Growth") {
		t.Fatalf("HTML headings were not preserved: %q", htmlText)
	}
	htmlChunks := chunkStructuredDocument("Plans", htmlText)
	if len(htmlChunks) != 1 || htmlChunks[0].HeadingPath != "Pricing > Growth" {
		t.Fatalf("unexpected HTML chunks: %+v", htmlChunks)
	}
}

func TestPricingAnswerIsValidatedAgainstRetrievedSources(t *testing.T) {
	plan := normalizeSupportQueryPlan(SupportQueryPlanContract{
		Route:           supportDecisionAnswer,
		Intent:          supportIntentPricingGeneral,
		StandaloneQuery: "What is your pricing?",
	}, "What is your pricing?")
	results := []KnowledgeSearchResult{{
		ID:          "price-1",
		ReferenceID: "content:pricing",
		SourceType:  knowledgeSourceTypeContent,
		Title:       "Pricing",
		URL:         "https://example.test/pricing",
		Content:     "Usermaven Growth plan starts at $49 per month. Enterprise uses custom pricing; contact sales.",
	}}
	if plan.Decision != supportDecisionAnswer || plan.EvidenceMode != supportEvidenceModeSufficiency || len(plan.RequiredEvidence) != 0 {
		t.Fatalf("pricing should proceed to source-grounded generation: %+v", plan)
	}
	response := &AIResponseContract{
		Content:      "The Growth plan starts at $49 per month. Enterprise pricing is custom.",
		CanAnswer:    true,
		Confidence:   0.95,
		SourceDocIDs: []string{"content:pricing"},
		Claims: []AIResponseClaim{
			{Text: "The Growth plan starts at $49 per month.", EvidenceIDs: []string{"price-1"}},
			{Text: "Enterprise pricing is custom.", EvidenceIDs: []string{"price-1"}},
		},
		EvidenceCoverage: map[string][]string{},
	}
	validation := validateSupportAnswer(plan, supportEvidenceCoverage{Found: map[string][]string{}}, results, response)
	if validation.Outcome != supportValidationPass {
		t.Fatalf("expected supported answer to pass: %+v", validation)
	}

	response.Content = "The Growth plan starts at $79 per month."
	response.Claims = []AIResponseClaim{{Text: response.Content, EvidenceIDs: []string{"price-1"}}}
	validation = validateSupportAnswer(plan, supportEvidenceCoverage{Found: map[string][]string{}}, results, response)
	if validation.Outcome != supportValidationNumeric {
		t.Fatalf("unsupported price must fail numeric validation: %+v", validation)
	}
}

func TestCommercialPlanWithoutSubjectStillRetrievesSources(t *testing.T) {
	plan := normalizeSupportQueryPlan(SupportQueryPlanContract{
		Route:           supportDecisionAnswer,
		Intent:          supportIntentPricingGeneral,
		StandaloneQuery: "What is your pricing?",
	}, "What is your pricing?")
	if plan.Decision != supportDecisionAnswer || len(plan.SearchQueries) == 0 {
		t.Fatalf("subjectless commercial question should be answered from retrieved sources: %+v", plan)
	}
}

func TestValidationRejectsUncitedNumber(t *testing.T) {
	plan := normalizeSupportQueryPlan(SupportQueryPlanContract{
		Route:           supportDecisionAnswer,
		Intent:          supportIntentPricingGeneral,
		Subject:         "Usermaven",
		StandaloneQuery: "What is your pricing?",
	}, "What is your pricing?")
	results := []KnowledgeSearchResult{
		{ID: "cited", Title: "Usermaven Pricing", URL: "https://usermaven.com/pricing", Content: "Growth starts at $49 per month. Enterprise has custom pricing."},
		{ID: "uncited", Title: "Legacy", Content: "An old plan cost $99 per month."},
	}
	response := &AIResponseContract{
		Content:    "Growth starts at $99 per month. Enterprise has custom pricing.",
		CanAnswer:  true,
		Confidence: 0.95,
		Claims: []AIResponseClaim{
			{Text: "Growth starts at $99 per month.", EvidenceIDs: []string{"cited"}},
		},
		EvidenceCoverage: map[string][]string{},
	}
	validation := validateSupportAnswer(plan, supportEvidenceCoverage{Found: map[string][]string{}}, results, response)
	if validation.Outcome != supportValidationNumeric {
		t.Fatalf("uncited retrieved number must not validate: %+v", validation)
	}
}

func TestValidationRejectsUnknownEvidenceID(t *testing.T) {
	plan := defaultSupportQueryPlan("Does every plan include unlimited projects?")
	results := []KnowledgeSearchResult{{
		ID: "guidance-1", SourceType: knowledgeSourceTypeGuidance,
		Content: "Growth starts at $49 per month.",
	}}
	response := &AIResponseContract{
		Content:   "Every plan includes unlimited projects.",
		CanAnswer: true,
		Claims: []AIResponseClaim{{
			Text: "Every plan includes unlimited projects.", EvidenceIDs: []string{"missing-guidance"},
		}},
	}
	validation := validateSupportAnswer(plan, supportEvidenceCoverage{Found: map[string][]string{}}, results, response)
	if validation.Outcome != supportValidationUngrounded {
		t.Fatalf("unknown evidence ID must fail grounding: %+v", validation)
	}
}

func TestValidationAcceptsSourceMappedParaphrase(t *testing.T) {
	plan := defaultSupportQueryPlan("What is your pricing?")
	results := []KnowledgeSearchResult{{ID: "doc-1", Content: "Monthly YearlySave up to 34%"}}
	response := &AIResponseContract{
		Content:   "Yearly billing is advertised as saving up to 34%.",
		CanAnswer: true,
		Claims: []AIResponseClaim{{
			Text: "Yearly billing is advertised as saving up to 34%.", EvidenceIDs: []string{"doc-1"},
		}},
	}
	validation := validateSupportAnswer(plan, supportEvidenceCoverage{Found: map[string][]string{}}, results, response)
	if validation.Outcome != supportValidationPass {
		t.Fatalf("strict source mapping plus exact numeric evidence should accept paraphrases: %+v", validation)
	}
}

func TestSelectSupportEvidenceContextUsesRetrievalRank(t *testing.T) {
	plan := SupportQueryPlanContract{}
	results := []KnowledgeSearchResult{
		{ID: "top", CombinedScore: 1},
		{ID: "price", CombinedScore: 0.2},
		{ID: "enterprise", CombinedScore: 0.1},
	}
	coverage := supportEvidenceCoverage{Found: map[string][]string{}}
	selected := selectSupportEvidenceContext(plan, coverage, results, 2)
	if len(selected) != 2 || selected[0].ID != "top" || selected[1].ID != "price" {
		t.Fatalf("retrieval order should be preserved: %+v", selected)
	}
}

type fixedSupportReranker struct {
	scores []SupportRerankScore
}

func (r fixedSupportReranker) Rerank(context.Context, string, []SupportRerankCandidate) ([]SupportRerankScore, error) {
	return r.scores, nil
}

func (fixedSupportReranker) Name() string { return "test-cross-encoder" }

func TestSemanticRerankerPreservesApplicableGuidanceAuthority(t *testing.T) {
	service := &SupportAIService{knowledgeReranker: fixedSupportReranker{scores: []SupportRerankScore{
		{Index: 1, Score: 0.95},
		{Index: 2, Score: 0.1},
	}}}
	results := []KnowledgeSearchResult{
		{ID: "doc-a", SourceType: knowledgeSourceTypeContent},
		{ID: "doc-b", SourceType: knowledgeSourceTypeContent},
		{ID: "guidance", SourceType: knowledgeSourceTypeGuidance},
	}
	reranked := service.semanticRerankKnowledgeResults(context.Background(), "pricing", results)
	if len(reranked) != 3 || reranked[0].ID != "guidance" || reranked[1].ID != "doc-b" {
		t.Fatalf("unexpected semantic rerank order: %+v", reranked)
	}
}

func TestBuildSupportConversationStateRequiresImmediateAnswerForConfirmation(t *testing.T) {
	metadata, _ := json.Marshal(AIMessageMetadata{
		AIReplyKind:    supportReplyKindAnswer,
		AIIssueKey:     "pricing",
		AIIssueSummary: "Customer asked about pricing.",
	})
	history := []model.SupportMessage{
		{SenderType: "customer", Content: "What is your pricing?"},
		{SenderType: "ai", Content: "Growth starts at $49.", Metadata: string(metadata)},
	}
	state := buildSupportConversationState(history)
	if !state.ConfirmationEligible || state.ActiveIssueKey != "pricing" || state.LastAIAnswer == "" {
		t.Fatalf("unexpected conversation state: %+v", state)
	}
	history = append(history, model.SupportMessage{SenderType: "customer", Content: "What about annual billing?"})
	if buildSupportConversationState(history).ConfirmationEligible {
		t.Fatal("an intervening customer request must make confirmation ineligible")
	}
}

func TestCuratedGuidanceSearchEnforcesScopeStatusAndValidity(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE curated_guidance (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, agent_id TEXT NOT NULL,
		title TEXT NOT NULL, question_patterns TEXT, answer TEXT NOT NULL,
		intent TEXT NOT NULL DEFAULT 'unknown', topics TEXT, language TEXT NOT NULL DEFAULT '',
		audience_policy_id TEXT, brand_id TEXT, status TEXT NOT NULL DEFAULT 'active',
		valid_from DATETIME, valid_until DATETIME, embedding TEXT,
		embedding_provider TEXT, embedding_model TEXT, embedding_version TEXT,
		embedding_dimensions INTEGER, created_by_id TEXT NOT NULL,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create curated guidance table: %v", err)
	}
	repo := repository.NewCuratedGuidanceRepository(db)
	now := time.Now()
	items := []model.CuratedGuidance{
		{ID: "eligible", WorkspaceID: "ws-1", AgentID: "agent-1", Title: "Pricing", QuestionPatterns: model.DocsStringArray{"What is pricing?"}, Answer: "Growth starts at $49.", Intent: supportIntentPricingGeneral, Status: model.CuratedGuidanceStatusActive, CreatedByID: "user-1"},
		{ID: "english-only", WorkspaceID: "ws-1", AgentID: "agent-1", Title: "Pricing", Answer: "English-only pricing.", Language: "en", Status: model.CuratedGuidanceStatusActive, CreatedByID: "user-1"},
		{ID: "scoped-audience", WorkspaceID: "ws-1", AgentID: "agent-1", Title: "Pricing", Answer: "Private audience pricing.", AudiencePolicyID: strPtr("audience-1"), Status: model.CuratedGuidanceStatusActive, CreatedByID: "user-1"},
		{ID: "scoped-brand", WorkspaceID: "ws-1", AgentID: "agent-1", Title: "Pricing", Answer: "Private brand pricing.", BrandID: strPtr("brand-1"), Status: model.CuratedGuidanceStatusActive, CreatedByID: "user-1"},
		{ID: "other-workspace", WorkspaceID: "ws-2", AgentID: "agent-1", Title: "Pricing", Answer: "Wrong workspace price is $1.", Status: model.CuratedGuidanceStatusActive, CreatedByID: "user-1"},
		{ID: "other-agent", WorkspaceID: "ws-1", AgentID: "agent-2", Title: "Pricing", Answer: "Wrong agent price is $2.", Status: model.CuratedGuidanceStatusActive, CreatedByID: "user-1"},
		{ID: "disabled", WorkspaceID: "ws-1", AgentID: "agent-1", Title: "Pricing", Answer: "Disabled price is $3.", Status: model.CuratedGuidanceStatusDisabled, CreatedByID: "user-1"},
		{ID: "expired", WorkspaceID: "ws-1", AgentID: "agent-1", Title: "Pricing", Answer: "Expired price is $4.", Status: model.CuratedGuidanceStatusActive, ValidUntil: &now, CreatedByID: "user-1"},
	}
	for idx := range items {
		if err := repo.Create(context.Background(), &items[idx]); err != nil {
			t.Fatalf("create guidance %s: %v", items[idx].ID, err)
		}
	}
	results, err := repo.Search(context.Background(), "ws-1", "agent-1", "", "pricing", "", "", 10)
	if err != nil {
		t.Fatalf("search guidance: %v", err)
	}
	if len(results) != 1 || results[0].ID != "eligible" {
		t.Fatalf("scope leak in guidance results: %+v", results)
	}
	results, err = repo.Search(context.Background(), "ws-1", "agent-1", "en", "pricing", "", "", 10)
	if err != nil {
		t.Fatalf("search English guidance: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected unscoped and English guidance only, got %+v", results)
	}
}

func TestNormalizeSupportLanguage(t *testing.T) {
	if got := normalizeSupportLanguage("EN_us"); got != "en-us" {
		t.Fatalf("normalizeSupportLanguage()=%q, want en-us", got)
	}
	if got := normalizeSupportLanguage("en<script>"); got != "" {
		t.Fatalf("invalid language must fail closed, got %q", got)
	}
	if got := normalizeSupportLanguage("english"); got != "" {
		t.Fatalf("language names must not become global scope, got %q", got)
	}
}

type capturePlatformTraceRecorder struct {
	traces chan *model.SupportAIRetrievalTrace
}

func (r *capturePlatformTraceRecorder) RecordSupportAIRetrievalTrace(_ context.Context, trace *model.SupportAIRetrievalTrace) error {
	r.traces <- trace
	return nil
}

func TestAnswerTraceIncludesShadowComparisonMetadata(t *testing.T) {
	recorder := &capturePlatformTraceRecorder{traces: make(chan *model.SupportAIRetrievalTrace, 1)}
	service := (&SupportAIService{}).SetSupportAIRetrievalTraceRecorder(recorder)
	plan := normalizeSupportQueryPlan(SupportQueryPlanContract{
		Route:           supportDecisionAnswer,
		Intent:          supportIntentUnknown,
		Language:        "en",
		StandaloneQuery: "How do I configure analytics?",
	}, "How do I configure analytics?")
	results := []KnowledgeSearchResult{{ID: "chunk-1", ReferenceID: "docs:doc-1", SourceType: knowledgeSourceTypeDocs, Content: "Configure analytics from Settings."}}
	response := &AIResponseContract{SourceDocIDs: []string{"docs:doc-1"}}
	service.recordSupportAIAnswerTrace(
		context.Background(), "ws-1", "conv-1", "note-1", "customer-1", "agent-1",
		plan, results, response, supportEvidenceCoverage{Found: map[string][]string{}, Missing: []string{}},
		supportAnswerValidation{Outcome: supportValidationPass}, 0.92, supportReplyKindAnswer,
		supportStateProgressing, false, map[string]int64{"planner": 10, "total": 40},
	)
	select {
	case trace := <-recorder.traces:
		if trace.MessageID != "note-1" || trace.CanAnswer == nil || *trace.CanAnswer != "true" {
			t.Fatalf("unexpected answer trace: %+v", trace)
		}
		var metadata map[string]any
		if err := json.Unmarshal(trace.Metadata, &metadata); err != nil {
			t.Fatalf("unmarshal trace metadata: %v", err)
		}
		if metadata["intent"] != supportIntentUnknown || metadata["language"] != "en" {
			t.Fatalf("missing comparison metadata: %+v", metadata)
		}
	case <-time.After(time.Second):
		t.Fatal("answer trace was not recorded")
	}
}

func TestDocsRetrievalEnforcesWorkspaceAgentAndEnabledLinkInSQL(t *testing.T) {
	db := newTestDB(t)
	for _, ddl := range []string{
		`CREATE TABLE docs_spaces (id TEXT PRIMARY KEY, workspace_id TEXT, type TEXT)`,
		`CREATE TABLE docs_documents (id TEXT PRIMARY KEY, workspace_id TEXT, space_id TEXT, status TEXT, deleted_at DATETIME)`,
		`CREATE TABLE docs_helpcenter_articles (document_id TEXT, public_published_at DATETIME)`,
		`CREATE TABLE docs_chunks (id TEXT PRIMARY KEY, workspace_id TEXT, space_id TEXT, document_id TEXT, block_id TEXT, chunk_index INTEGER, section_key TEXT, heading_path TEXT, title TEXT, content TEXT, search_content TEXT, updated_at DATETIME)`,
		`CREATE TABLE agent_knowledge_sources (id TEXT PRIMARY KEY, agent_id TEXT, space_id TEXT, workspace_id TEXT, sync_status TEXT)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create retrieval table: %v", err)
		}
	}
	for _, statement := range []string{
		`INSERT INTO docs_spaces (id, workspace_id, type) VALUES ('space-ok', 'ws-1', 'internal'), ('space-agent-leak', 'ws-1', 'internal'), ('space-disabled', 'ws-1', 'internal'), ('space-workspace-leak', 'ws-2', 'internal')`,
		`INSERT INTO docs_documents (id, workspace_id, space_id, status) VALUES ('doc-ok', 'ws-1', 'space-ok', 'published'), ('doc-agent-leak', 'ws-1', 'space-agent-leak', 'published'), ('doc-disabled', 'ws-1', 'space-disabled', 'published'), ('doc-workspace-leak', 'ws-2', 'space-workspace-leak', 'published')`,
		`INSERT INTO docs_chunks (id, workspace_id, space_id, document_id, chunk_index, title, content, search_content, updated_at) VALUES ('chunk-ok', 'ws-1', 'space-ok', 'doc-ok', 0, 'Pricing', 'Price is $49', 'Pricing Price is $49', CURRENT_TIMESTAMP), ('chunk-agent-leak', 'ws-1', 'space-agent-leak', 'doc-agent-leak', 0, 'Pricing', 'Secret agent price', 'Pricing Secret agent price', CURRENT_TIMESTAMP), ('chunk-disabled', 'ws-1', 'space-disabled', 'doc-disabled', 0, 'Pricing', 'Disabled price', 'Pricing Disabled price', CURRENT_TIMESTAMP), ('chunk-workspace-leak', 'ws-2', 'space-workspace-leak', 'doc-workspace-leak', 0, 'Pricing', 'Other workspace price', 'Pricing Other workspace price', CURRENT_TIMESTAMP)`,
		`INSERT INTO agent_knowledge_sources (id, agent_id, space_id, workspace_id, sync_status) VALUES ('link-ok', 'agent-1', 'space-ok', 'ws-1', 'ready'), ('link-other-agent', 'agent-2', 'space-agent-leak', 'ws-1', 'ready'), ('link-disabled', 'agent-1', 'space-disabled', 'ws-1', 'disabled'), ('link-other-workspace', 'agent-1', 'space-workspace-leak', 'ws-2', 'ready')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed retrieval table: %v", err)
		}
	}
	results, err := repository.NewDocsChunkRepository(db).HybridSearchForAgent(
		context.Background(), "ws-1", "agent-1",
		[]string{"space-ok", "space-agent-leak", "space-disabled", "space-workspace-leak"},
		"pricing", "", "", 20,
	)
	if err != nil {
		t.Fatalf("search docs: %v", err)
	}
	if len(results) != 1 || results[0].ID != "chunk-ok" {
		t.Fatalf("ineligible docs entered candidate set: %+v", results)
	}
}

func TestContentRetrievalEnforcesAgentSourceStatusAndPageValidityInSQL(t *testing.T) {
	db := newTestDB(t)
	for _, ddl := range []string{
		`CREATE TABLE support_content_sources (id TEXT PRIMARY KEY, workspace_id TEXT, sync_status TEXT)`,
		`CREATE TABLE support_content_pages (id TEXT PRIMARY KEY, workspace_id TEXT, content_source_id TEXT, http_status INTEGER)`,
		`CREATE TABLE support_content_chunks (id TEXT PRIMARY KEY, workspace_id TEXT, content_source_id TEXT, page_id TEXT, chunk_index INTEGER, section_key TEXT, heading_path TEXT, title TEXT, url TEXT, content TEXT, search_content TEXT, updated_at DATETIME)`,
		`CREATE TABLE agent_content_sources (id TEXT PRIMARY KEY, agent_id TEXT, content_source_id TEXT, workspace_id TEXT)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create content retrieval table: %v", err)
		}
	}
	for _, statement := range []string{
		`INSERT INTO support_content_sources (id, workspace_id, sync_status) VALUES ('source-ok', 'ws-1', 'ready'), ('source-agent-leak', 'ws-1', 'ready'), ('source-disabled', 'ws-1', 'disabled'), ('source-bad-page', 'ws-1', 'ready')`,
		`INSERT INTO support_content_pages (id, workspace_id, content_source_id, http_status) VALUES ('page-ok', 'ws-1', 'source-ok', 200), ('page-agent-leak', 'ws-1', 'source-agent-leak', 200), ('page-disabled', 'ws-1', 'source-disabled', 200), ('page-bad', 'ws-1', 'source-bad-page', 404)`,
		`INSERT INTO support_content_chunks (id, workspace_id, content_source_id, page_id, chunk_index, title, url, content, search_content, updated_at) VALUES ('content-ok', 'ws-1', 'source-ok', 'page-ok', 0, 'Pricing', 'https://ok.test', 'Price is $49', 'Pricing Price is $49', CURRENT_TIMESTAMP), ('content-agent-leak', 'ws-1', 'source-agent-leak', 'page-agent-leak', 0, 'Pricing', 'https://leak.test', 'Secret price', 'Pricing Secret price', CURRENT_TIMESTAMP), ('content-disabled', 'ws-1', 'source-disabled', 'page-disabled', 0, 'Pricing', 'https://disabled.test', 'Disabled price', 'Pricing Disabled price', CURRENT_TIMESTAMP), ('content-bad-page', 'ws-1', 'source-bad-page', 'page-bad', 0, 'Pricing', 'https://bad.test', 'Bad page price', 'Pricing Bad page price', CURRENT_TIMESTAMP)`,
		`INSERT INTO agent_content_sources (id, agent_id, content_source_id, workspace_id) VALUES ('link-ok', 'agent-1', 'source-ok', 'ws-1'), ('link-other', 'agent-2', 'source-agent-leak', 'ws-1'), ('link-disabled', 'agent-1', 'source-disabled', 'ws-1'), ('link-bad-page', 'agent-1', 'source-bad-page', 'ws-1')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed content retrieval table: %v", err)
		}
	}
	results, err := repository.NewSupportContentChunkRepository(db).HybridSearchForAgent(
		context.Background(), "ws-1", "agent-1",
		[]string{"source-ok", "source-agent-leak", "source-disabled", "source-bad-page"},
		"pricing", "", "", 20,
	)
	if err != nil {
		t.Fatalf("search content: %v", err)
	}
	if len(results) != 1 || results[0].ID != "content-ok" {
		t.Fatalf("ineligible content entered candidate set: %+v", results)
	}
}
