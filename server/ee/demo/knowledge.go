//go:build ee

package demo

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	coreservice "github.com/helpin-ai/helpin/server/internal/service"
)

const (
	// demoIndexPollDelay spaces the checks on indexing; a help center takes
	// minutes to index, so polling faster only adds load.
	demoIndexPollDelay = 30 * time.Second
	// demoVerifyPollDelay spaces the checks on running check questions.
	demoVerifyPollDelay = 10 * time.Second
	// demoIndexTolerance is the share of imported articles, in percent, that
	// must be indexed: a few articles with no text never are.
	demoIndexTolerance = 98
	// demoMaxVerifyQuestions caps the metered preview runs one build makes.
	demoMaxVerifyQuestions = 5
	demoArticleQuestions   = 3
)

// Issues a build can finish with. A verify handoff is recorded as
// "verify_handoffs:<reason>".
const (
	IssueAIRepliesOff        = "ai_replies_off"
	IssueIndexIncomplete     = "index_incomplete"
	IssueWebsiteCrawlFailed  = "website_crawl_failed"
	IssueVerifyUnavailable   = "verify_unavailable"
	IssueVerifyFailed        = "verify_failed"
	issueVerifyHandoffPrefix = "verify_handoffs:"
)

// Kinds of check question.
const (
	verifyKindArticle = "article"
	verifyKindPricing = "pricing"
	verifyKindCustom  = "custom"
)

// WebsiteReport is the company website a build crawled for the AI.
type WebsiteReport struct {
	URL          string `json:"url"`
	SourceID     string `json:"source_id,omitempty"`
	SyncStatus   string `json:"sync_status,omitempty"`
	IndexedPages int    `json:"indexed_pages"`
	Error        string `json:"error,omitempty"`
}

// KnowledgeReport is how far the help center was indexed for the AI.
type KnowledgeReport struct {
	SyncStatus       string     `json:"sync_status,omitempty"`
	IndexedDocuments int        `json:"indexed_documents"`
	Error            string     `json:"error,omitempty"`
	IndexStartedAt   *time.Time `json:"index_started_at,omitempty"`
}

// VerifyResult is one check question a build asked the support agent. The
// answer text is left out: only whether it answered, and from what.
type VerifyResult struct {
	Question           string   `json:"question"`
	Kind               string   `json:"kind"`
	RunID              string   `json:"run_id,omitempty"`
	Decision           string   `json:"decision"`
	Reason             string   `json:"reason,omitempty"`
	GroundedConfidence float64  `json:"grounded_confidence,omitempty"`
	SourceTypes        []string `json:"source_types,omitempty"`
}

// validateDemoWebsiteURL accepts "acme.com" or an https address without
// credentials, the same rule as the help center's official docs address.
func validateDemoWebsiteURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return "", demoInputError("website_url must be an https address without a user name or password, such as https://acme.com.")
	}
	return parsed.String(), nil
}

// demoActorContext acts as the demo owner inside the demo workspace, for the
// services that check who is asking.
func demoActorContext(ctx context.Context, workspaceID, ownerID string) context.Context {
	return authorization.WithActor(ctx, &authorization.Actor{
		UserID: ownerID, WorkspaceID: workspaceID, Role: model.RoleOwner, Status: "active",
	})
}

func addDemoIssue(report *Report, issue string) {
	if !slices.Contains(report.Issues, issue) {
		report.Issues = append(report.Issues, issue)
	}
}

// stepWebsite saves the company website on the demo workspace and crawls it
// as AI knowledge. A website that cannot be crawled never fails the build.
func (s *Service) stepWebsite(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	workspaceID := derefDemoString(demo.WorkspaceID)
	report := readDemoReport(demo)
	if report.Website == nil {
		report.Website = &WebsiteReport{}
	}
	website := report.Website
	if website.URL == "" && demo.Domain != "" {
		website.URL = "https://" + demo.Domain
	}
	if website.URL == "" {
		addDemoIssue(&report, IssueWebsiteCrawlFailed)
		report.Notes = appendNote(report.Notes, "No company website is known, so pricing and product questions the help center does not cover go unanswered.")
		writeDemoReport(demo, report)
		return demoStepResult{next: demoStepWidget}, nil
	}
	// The widget step replaces the allowlist the website suggests; without a
	// demo address it could not, and the company's own site would stay on it.
	if demo.DemoURL != nil {
		owner, err := s.owner(ctx)
		if err != nil {
			return demoStepResult{}, err
		}
		if _, err := s.deps.Workspaces.Update(ctx, workspaceID, model.UpdateWorkspaceRequest{WebsiteURL: &website.URL}, owner.ID); err != nil {
			return demoStepResult{}, fmt.Errorf("save demo workspace website: %w", err)
		}
	}
	if website.SourceID == "" {
		source, err := s.websiteSource(ctx, workspaceID, demo.CompanyName, website.URL)
		if err != nil {
			if ctx.Err() != nil {
				return demoStepResult{}, ctx.Err()
			}
			s.logger.WarnContext(ctx, "demo: website crawl not started", "demo_id", demo.ID, "error", err)
			website.Error = "The crawl could not be started."
			addDemoIssue(&report, IssueWebsiteCrawlFailed)
			report.Notes = appendNote(report.Notes, "The company website could not be crawled. Add it under the AI's content sources.")
			writeDemoReport(demo, report)
			return demoStepResult{next: demoStepWidget}, nil
		}
		website.SourceID, website.SyncStatus = source.ID, source.SyncStatus
	}
	writeDemoReport(demo, report)
	return demoStepResult{next: demoStepWidget}, nil
}

// websiteSource returns the workspace's content source for the website,
// creating it unless one for the same site exists, so a resumed or
// refreshed build never crawls the site twice.
func (s *Service) websiteSource(ctx context.Context, workspaceID, companyName, websiteURL string) (*model.SupportContentSource, error) {
	sources, err := s.deps.ContentSources.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list content sources: %w", err)
	}
	for i := range sources {
		// Same site, with or without "www.": sources added by hand count too.
		if sources[i].SourceType == model.ContentSourceTypeWebsite && normalizeDemoDomain(sources[i].StartURL) == normalizeDemoDomain(websiteURL) {
			return &sources[i], nil
		}
	}
	render := true
	return s.deps.ContentSources.Create(ctx, workspaceID, model.CreateSupportContentSourceRequest{
		Name: companyName + " website", StartURL: websiteURL, CrawlLimit: s.cfg.WebsiteCrawlLimit, CrawlDepth: 2,
		CrawlSource: "all", Formats: []string{"markdown"}, Render: &render,
		CrawlPurposes: []string{"search", "ai-input"}, MaxAgeSeconds: 86400,
	})
}

// supportAgentID returns the agent that answers the demo's chat widget, or
// "" when AI replies are off.
func (s *Service) supportAgentID(ctx context.Context, workspaceID string) (string, error) {
	_, settings, err := s.deps.Support.GetInstallation(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("read chat widget: %w", err)
	}
	if settings == nil {
		return "", nil
	}
	return strings.TrimSpace(derefDemoString(settings.AIAgentID)), nil
}

// stepKnowledge makes the imported help center an explicit knowledge source
// of the support agent and indexes it. Without an explicit source, help
// centers imported in parallel were found unindexed for hours.
func (s *Service) stepKnowledge(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	workspaceID, spaceID := derefDemoString(demo.WorkspaceID), derefDemoString(demo.SpaceID)
	agentID, err := s.supportAgentID(ctx, workspaceID)
	if err != nil {
		return demoStepResult{}, err
	}
	report := readDemoReport(demo)
	if !report.AIRepliesEnabled {
		addDemoIssue(&report, IssueAIRepliesOff)
	}
	if agentID == "" {
		// No agent answers the widget, so there is nothing to index or ask.
		addDemoIssue(&report, IssueAIRepliesOff)
		writeDemoReport(demo, report)
		return demoStepResult{next: demoStepCRM}, nil
	}
	sources, err := s.deps.Knowledge.List(ctx, workspaceID, agentID)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("list agent knowledge: %w", err)
	}
	selected := slices.ContainsFunc(sources, func(source coreservice.KnowledgeSourceWithSpace) bool { return source.SpaceID == spaceID })
	if selected {
		err = s.deps.Knowledge.Reindex(ctx, workspaceID, agentID, spaceID)
	} else {
		// Selecting a space queues its indexing.
		err = s.deps.Knowledge.Set(ctx, workspaceID, agentID, []string{spaceID})
	}
	if err != nil {
		return demoStepResult{}, fmt.Errorf("index demo help center: %w", err)
	}
	started := s.now()
	report.Knowledge = &KnowledgeReport{IndexStartedAt: &started}
	writeDemoReport(demo, report)
	return demoStepResult{next: demoStepIndexWait}, nil
}

// stepIndexWait waits, without holding the build's lease, until the help
// center and website are indexed or the wait times out.
func (s *Service) stepIndexWait(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	workspaceID, spaceID := derefDemoString(demo.WorkspaceID), derefDemoString(demo.SpaceID)
	agentID, err := s.supportAgentID(ctx, workspaceID)
	if err != nil {
		return demoStepResult{}, err
	}
	report := readDemoReport(demo)
	if report.Knowledge == nil || report.Knowledge.IndexStartedAt == nil {
		// Built before this step existed: index first.
		return demoStepResult{next: demoStepKnowledge}, nil
	}
	knowledge := report.Knowledge
	docsDone := false
	if agentID != "" {
		sources, err := s.deps.Knowledge.List(ctx, workspaceID, agentID)
		if err != nil {
			return demoStepResult{}, fmt.Errorf("list agent knowledge: %w", err)
		}
		for _, source := range sources {
			if source.SpaceID != spaceID {
				continue
			}
			knowledge.SyncStatus, knowledge.IndexedDocuments = source.SyncStatus, source.IndexedDocuments
			knowledge.Error = derefDemoString(source.LastSyncError)
			// Only a sync that ended after this step's request counts.
			finished := source.LastSyncCompletedAt != nil && !source.LastSyncCompletedAt.Before(*knowledge.IndexStartedAt)
			docsDone = finished && (source.SyncStatus == model.KnowledgeSourceSyncReady || source.SyncStatus == model.KnowledgeSourceSyncFailed)
		}
	}
	websiteDone := true
	if website := report.Website; website != nil && website.SourceID != "" {
		source, err := s.contentSource(ctx, workspaceID, website.SourceID)
		if err != nil {
			return demoStepResult{}, err
		}
		if source == nil {
			website.SyncStatus, website.Error = model.KnowledgeSourceSyncFailed, "The content source was deleted."
		} else {
			website.SyncStatus, website.IndexedPages = source.SyncStatus, source.IndexedPages
			website.Error = derefDemoString(source.LastSyncError)
		}
		websiteDone = slices.Contains([]string{model.KnowledgeSourceSyncReady, model.KnowledgeSourceSyncFailed, model.KnowledgeSourceSyncDisabled}, website.SyncStatus)
	}
	timedOut := s.now().Sub(*knowledge.IndexStartedAt) > s.cfg.IndexTimeout
	if !(docsDone && websiteDone) && !timedOut {
		writeDemoReport(demo, report)
		return demoStepResult{wait: true, delay: demoIndexPollDelay}, nil
	}
	if knowledge.SyncStatus != model.KnowledgeSourceSyncReady || knowledge.IndexedDocuments*100 < report.ArticlesImported*demoIndexTolerance {
		addDemoIssue(&report, IssueIndexIncomplete)
	}
	if website := report.Website; website != nil && website.SourceID != "" && website.SyncStatus != model.KnowledgeSourceSyncReady {
		addDemoIssue(&report, IssueWebsiteCrawlFailed)
	}
	writeDemoReport(demo, report)
	return demoStepResult{next: demoStepVerify}, nil
}

func (s *Service) contentSource(ctx context.Context, workspaceID, id string) (*model.SupportContentSource, error) {
	sources, err := s.deps.ContentSources.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list content sources: %w", err)
	}
	for i := range sources {
		if sources[i].ID == id {
			return &sources[i], nil
		}
	}
	return nil, nil
}

// stepVerify asks the support agent a few questions a prospect would ask and
// records whether it answered from the help center. The runs are started
// once and checked on later passes, never waited on while leased.
func (s *Service) stepVerify(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	workspaceID := derefDemoString(demo.WorkspaceID)
	agentID, err := s.supportAgentID(ctx, workspaceID)
	if err != nil {
		return demoStepResult{}, err
	}
	report := readDemoReport(demo)
	if agentID == "" {
		addDemoIssue(&report, IssueAIRepliesOff)
		writeDemoReport(demo, report)
		return demoStepResult{next: demoStepCRM}, nil
	}
	owner, err := s.owner(ctx)
	if err != nil {
		return demoStepResult{}, err
	}
	actorCtx := demoActorContext(ctx, workspaceID, owner.ID)
	if len(report.Verify) == 0 {
		if report.Verify, err = s.verifyQuestions(actorCtx, demo, report); err != nil {
			return demoStepResult{}, err
		}
	}
	pending := false
	for i := range report.Verify {
		result := &report.Verify[i]
		if result.Decision != "" && result.Decision != "pending" {
			continue
		}
		response, err := s.askVerifyQuestion(actorCtx, workspaceID, agentID, result)
		if err != nil {
			if ctx.Err() != nil {
				return demoStepResult{}, ctx.Err()
			}
			// Preview needs a runnable agent with the support tools; without
			// one the demo still works, but nothing was checked.
			s.logger.WarnContext(ctx, "demo: check question failed", "demo_id", demo.ID, "error", err)
			result.Decision, result.Reason = "failed", "preview_unavailable"
			addDemoIssue(&report, IssueVerifyUnavailable)
			continue
		}
		result.RunID, result.Decision, result.Reason = response.RunID, response.FinalDecision, response.FinalReason
		if response.Answer != nil {
			result.GroundedConfidence = response.Answer.GroundedConfidence
		}
		result.SourceTypes = nil
		for _, found := range response.Retrieval.Results {
			if !slices.Contains(result.SourceTypes, found.SourceType) {
				result.SourceTypes = append(result.SourceTypes, found.SourceType)
			}
		}
		pending = pending || result.Decision == "pending"
	}
	if pending {
		writeDemoReport(demo, report)
		return demoStepResult{wait: true, delay: demoVerifyPollDelay}, nil
	}
	report.VerifyPassed = verifyPassed(report.Verify)
	for _, result := range report.Verify {
		if result.Decision != "answer" && result.Reason != "" && result.Reason != "preview_unavailable" {
			addDemoIssue(&report, issueVerifyHandoffPrefix+result.Reason)
		}
	}
	if !report.VerifyPassed {
		addDemoIssue(&report, IssueVerifyFailed)
	}
	writeDemoReport(demo, report)
	return demoStepResult{next: demoStepCRM}, nil
}

// askVerifyQuestion starts a check question's preview run, or reads the run
// it already started.
func (s *Service) askVerifyQuestion(ctx context.Context, workspaceID, agentID string, result *VerifyResult) (*model.SupportAIPreviewResponse, error) {
	if result.RunID != "" {
		return s.deps.Preview.Get(ctx, workspaceID, agentID, result.RunID)
	}
	includeAnswer := true
	return s.deps.Preview.Start(ctx, workspaceID, agentID, model.SupportAIPreviewRequest{Message: result.Question, IncludeAnswer: &includeAnswer})
}

// verifyPassed applies the pass rule: at least two article questions (or
// every one, when fewer were asked) answered from the help center.
func verifyPassed(results []VerifyResult) bool {
	asked, answered := 0, 0
	for _, result := range results {
		if result.Kind != verifyKindArticle {
			continue
		}
		asked++
		if result.Decision == "answer" && slices.Contains(result.SourceTypes, "docs") {
			answered++
		}
	}
	return asked > 0 && answered >= min(2, asked)
}

// verifyQuestions picks the check questions: one per leading collection,
// from its first article's title (never its text), a pricing question when
// the website was indexed, and the caller's own, five at most.
func (s *Service) verifyQuestions(ctx context.Context, demo *eerepository.DemoWorkspace, report Report) ([]VerifyResult, error) {
	workspaceID, spaceID := derefDemoString(demo.WorkspaceID), derefDemoString(demo.SpaceID)
	collections, err := s.deps.Collections.List(ctx, spaceID)
	if err != nil {
		return nil, fmt.Errorf("list demo collections: %w", err)
	}
	collections = slices.DeleteFunc(collections, func(collection model.DocsCollection) bool { return collection.ParentCollectionID != nil })
	slices.SortStableFunc(collections, func(a, b model.DocsCollection) int { return a.Position - b.Position })
	var results []VerifyResult
	published := model.DocStatusPublished
	for _, collection := range collections {
		if len(results) == demoArticleQuestions {
			break
		}
		documents, err := s.deps.Documents.List(ctx, workspaceID, &spaceID, &collection.ID, &published, nil, "", model.RoleOwner, false)
		if err != nil {
			return nil, fmt.Errorf("list demo articles: %w", err)
		}
		slices.SortStableFunc(documents, func(a, b model.DocsDocument) int { return a.Position - b.Position })
		for _, document := range documents {
			if title := strings.TrimSpace(document.Title); title != "" {
				results = append(results, VerifyResult{Question: articleQuestion(title), Kind: verifyKindArticle})
				break
			}
		}
	}
	if report.Website != nil && report.Website.IndexedPages > 0 {
		results = append(results, VerifyResult{Question: fmt.Sprintf("How much does %s cost?", demo.CompanyName), Kind: verifyKindPricing})
	}
	for _, question := range report.VerifyQuestions {
		results = append(results, VerifyResult{Question: question, Kind: verifyKindCustom})
	}
	return results[:min(len(results), demoMaxVerifyQuestions)], nil
}

// articleQuestion turns an article title into what a customer would type.
func articleQuestion(title string) string {
	if strings.HasSuffix(title, "?") {
		return title
	}
	return "Can you help me with " + strings.TrimRight(title, ".!") + "?"
}

func appendNote(notes []string, note string) []string {
	if slices.Contains(notes, note) {
		return notes
	}
	return append(notes, note)
}
