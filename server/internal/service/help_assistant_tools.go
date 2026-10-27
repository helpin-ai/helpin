package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	helpAssistantSearchArticles   = 5
	helpAssistantPassagesPerDoc   = 2
	helpAssistantPassageRunes     = 900
	helpAssistantArticleRunes     = 8000
	helpAssistantMaxAnswerRunes   = 4000
	helpAssistantMaxProgressRunes = 80
	helpAssistantEvidenceSource   = "help_article"
	helpAssistantArticleEvidence  = "article:"
	helpAssistantDefaultHandoff   = "I couldn't find this in the help articles. Contact support and the team will help you."
)

// helpAssistantLink matches links and URLs, which answers must not contain:
// citations become the links.
var helpAssistantLink = regexp.MustCompile(`(?i)https?://|www\.|\]\(`)

// HelpSearchResult is one passage search_help returns. The model cites it by
// EvidenceID.
type HelpSearchResult struct {
	EvidenceID string `json:"evidence_id"`
	ArticleID  string `json:"article_id"`
	Title      string `json:"title"`
	Section    string `json:"section,omitempty"`
	Passage    string `json:"passage"`
}

// helpArticleRef is an article the session's audience may read.
type helpArticleRef struct {
	DocumentID      string
	Title           string
	URL             string
	PortalArticleID string
}

// turnForRun returns the session and the pending answer of a help assistant
// run. Tools act only for a turn that is still being answered.
func (s *HelpAssistantService) turnForRun(ctx context.Context, meta model.InternalCommandContext) (*model.HelpAssistantSession, *model.HelpAssistantMessage, error) {
	if meta.TargetType != model.HelpAssistantTurnTarget || meta.TargetID == "" || meta.RunID == "" {
		return nil, nil, fmt.Errorf("help assistant tools run only in a help assistant turn")
	}
	answer, err := s.repo.GetMessage(ctx, meta.WorkspaceID, meta.TargetID)
	if err != nil {
		return nil, nil, err
	}
	if answer == nil || answer.Role != model.HelpAssistantRoleAssistant {
		return nil, nil, fmt.Errorf("help assistant turn not found")
	}
	if answer.Status != model.HelpAssistantStatusPending {
		return nil, nil, fmt.Errorf("this turn is already finished; end your turn")
	}
	session, err := s.repo.GetSession(ctx, meta.WorkspaceID, answer.SessionID)
	if err != nil {
		return nil, nil, err
	}
	if session == nil {
		return nil, nil, fmt.Errorf("help assistant session not found")
	}
	return session, answer, nil
}

// SearchHelp searches the articles the session's audience may read and keeps
// the passages as citable evidence for this run.
func (s *HelpAssistantService) SearchHelp(ctx context.Context, meta model.InternalCommandContext, query string) ([]HelpSearchResult, error) {
	session, _, err := s.turnForRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	query = truncateRunes(query, 300)
	ctx = WithKnowledgeSurface(ctx, "help_assistant")

	groups, err := s.publicGroups(ctx, session.WorkspaceID, query)
	if err != nil {
		return nil, err
	}
	if session.Audience == model.HelpcenterAudienceCustomers {
		spaceIDs, err := s.hcRepo.ListCustomerHelpSpaceIDs(ctx, session.WorkspaceID)
		if err != nil {
			return nil, err
		}
		private, err := s.knowledge.searchCustomerChunks(ctx, session.WorkspaceID, spaceIDs, query)
		if err != nil {
			return nil, err
		}
		groups = append(groups, private...)
		sort.SliceStable(groups, func(i, j int) bool { return groups[i].bestScore > groups[j].bestScore })
	}
	if len(groups) == 0 {
		// Never worse than keyword search: articles without search chunks yet
		// are still found by their text.
		groups, err = s.keywordGroups(ctx, session, query)
		if err != nil {
			return nil, err
		}
	}
	refs, err := s.articleRefs(ctx, session, documentIDsOf(groups))
	if err != nil {
		return nil, err
	}

	results := []HelpSearchResult{}
	evidence := []model.SupportRunEvidence{}
	articles := 0
	for _, group := range groups {
		ref, ok := refs[group.documentID]
		if !ok {
			continue
		}
		for _, chunk := range group.chunks[:min(len(group.chunks), helpAssistantPassagesPerDoc)] {
			passage := truncateRunes(chunk.Content, helpAssistantPassageRunes)
			results = append(results, HelpSearchResult{EvidenceID: chunk.ID, ArticleID: ref.DocumentID, Title: ref.Title, Section: strings.TrimSpace(chunk.HeadingPath), Passage: passage})
			evidence = append(evidence, helpAssistantEvidence(meta, ref, chunk.ID, passage, chunk))
		}
		if articles++; articles == helpAssistantSearchArticles {
			break
		}
	}
	if err := s.evidence.UpsertBatch(ctx, evidence); err != nil {
		return nil, err
	}
	return results, nil
}

// publicGroups retrieves relevant public passages grouped by article.
func (s *HelpAssistantService) publicGroups(ctx context.Context, workspaceID, query string) ([]helpcenterChunkGroup, error) {
	chunks, err := s.knowledge.retrievePublicChunks(ctx, workspaceID, query, helpcenterSemanticChunkLimit)
	if err != nil {
		return nil, err
	}
	return groupChunksByDocument(relevantChunks(chunks)), nil
}

// keywordGroups finds public articles by full-text search, as passages
// citable like chunks. The evidence id of a keyword passage is the article's.
func (s *HelpAssistantService) keywordGroups(ctx context.Context, session *model.HelpAssistantSession, query string) ([]helpcenterChunkGroup, error) {
	if s.knowledge == nil || s.knowledge.searchRepo == nil {
		return nil, nil
	}
	results, err := s.knowledge.searchRepo.PublicSearch(ctx, session.WorkspaceID, session.Locale, query, "", helpAssistantSearchArticles)
	if err != nil {
		return nil, err
	}
	groups := make([]helpcenterChunkGroup, 0, len(results))
	for _, result := range results {
		passage := strings.TrimSpace(derefString(result.Excerpt))
		for _, match := range result.Matches {
			if text := strings.TrimSpace(match.Snippet); text != "" {
				passage = portalSnippetMarkup.Replace(text)
				break
			}
		}
		if passage == "" {
			passage = result.Title
		}
		groups = append(groups, helpcenterChunkGroup{documentID: result.ID, chunks: []repository.DocsChunkSearchResult{{
			ID: helpAssistantArticleEvidence + result.ID, DocumentID: result.ID, Title: result.Title, Content: passage, LexicalScore: 1,
		}}})
	}
	return groups, nil
}

// articleRefs resolves documents to the articles the session's audience may
// read: live public articles, plus articles for signed-in customers when the
// session has that audience. Anything else is left out.
func (s *HelpAssistantService) articleRefs(ctx context.Context, session *model.HelpAssistantSession, documentIDs []string) (map[string]helpArticleRef, error) {
	refs := map[string]helpArticleRef{}
	if len(documentIDs) == 0 {
		return refs, nil
	}
	base := s.baseLocale(ctx, session)
	// Articles read in the session's language where translated, else the default.
	public, err := s.knowledge.resolveArticleRefs(ctx, session.WorkspaceID, session.Locale, base, documentIDs)
	if err != nil {
		return nil, err
	}
	urls, err := supportPublicArticleURLs(ctx, s.hcRepo, session.WorkspaceID, documentIDs)
	if err != nil {
		return nil, err
	}
	for id, article := range public {
		refs[id] = helpArticleRef{DocumentID: id, Title: article.Title, URL: urls[id]}
	}
	if session.Audience != model.HelpcenterAudienceCustomers {
		return refs, nil
	}
	private, err := s.hcRepo.ListCustomerHelpArticles(ctx, session.WorkspaceID, base, session.Locale)
	if err != nil {
		return nil, err
	}
	for _, article := range private {
		if slices.Contains(documentIDs, article.DocumentID) {
			refs[article.DocumentID] = helpArticleRef{DocumentID: article.DocumentID, Title: article.Title, PortalArticleID: article.PublicID}
		}
	}
	return refs, nil
}

// baseLocale is the help center's default locale, or the session's when the
// help center cannot be read.
func (s *HelpAssistantService) baseLocale(ctx context.Context, session *model.HelpAssistantSession) string {
	cfg, err := s.hcRepo.GetConfig(ctx, session.WorkspaceID)
	if err != nil || cfg == nil {
		return session.Locale
	}
	return defaultHelpcenterLocale(cfg)
}

func (s *HelpAssistantService) articleTitle(ctx context.Context, session *model.HelpAssistantSession, documentID string) string {
	refs, err := s.articleRefs(ctx, session, []string{documentID})
	if err != nil {
		return ""
	}
	return refs[documentID].Title
}

// ReadArticle returns the text of one article the session's audience may
// read, and keeps it as citable evidence.
func (s *HelpAssistantService) ReadArticle(ctx context.Context, meta model.InternalCommandContext, documentID string) (map[string]any, error) {
	session, _, err := s.turnForRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	documentID = strings.TrimSpace(documentID)
	refs, err := s.articleRefs(ctx, session, []string{documentID})
	if err != nil {
		return nil, err
	}
	ref, ok := refs[documentID]
	if !ok {
		return nil, fmt.Errorf("article not found; use an article_id from search_help")
	}
	publication, err := s.publications.GetArticlePublication(ctx, documentID, session.Locale)
	if err != nil {
		return nil, err
	}
	if base := s.baseLocale(ctx, session); publication == nil && base != session.Locale {
		if publication, err = s.publications.GetArticlePublication(ctx, documentID, base); err != nil {
			return nil, err
		}
	}
	if publication == nil {
		return nil, fmt.Errorf("article not found; use an article_id from search_help")
	}
	text := truncateRunes(publication.ContentText, helpAssistantArticleRunes)
	evidenceID := helpAssistantArticleEvidence + documentID
	row := helpAssistantEvidence(meta, ref, evidenceID, text, repository.DocsChunkSearchResult{})
	if err := s.evidence.UpsertBatch(ctx, []model.SupportRunEvidence{row}); err != nil {
		return nil, err
	}
	return map[string]any{"evidence_id": evidenceID, "article_id": documentID, "title": ref.Title, "text": text}, nil
}

// SendAnswer finishes the turn with an answer. Every citation must be
// evidence this run retrieved, and the text may not contain links.
func (s *HelpAssistantService) SendAnswer(ctx context.Context, meta model.InternalCommandContext, answer string, evidenceIDs []string) error {
	_, turn, err := s.turnForRun(ctx, meta)
	if err != nil {
		return err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" || utf8.RuneCountInString(answer) > helpAssistantMaxAnswerRunes {
		return fmt.Errorf("answer must be 1-%d characters", helpAssistantMaxAnswerRunes)
	}
	if helpAssistantLink.MatchString(answer) {
		return fmt.Errorf("remove links and URLs from the answer; cited articles are linked automatically")
	}
	if len(evidenceIDs) == 0 {
		return fmt.Errorf("cite at least one evidence_id from search_help or read_help_article; if the articles do not answer, use offer_contact_support")
	}
	rows, err := s.evidence.ListByRun(ctx, meta.WorkspaceID, meta.RunID)
	if err != nil {
		return err
	}
	byID := make(map[string]model.SupportRunEvidence, len(rows))
	for _, row := range rows {
		if row.SourceType == helpAssistantEvidenceSource {
			byID[row.EvidenceID] = row
		}
	}
	var citations []model.HelpAssistantCitation
	seen := map[string]bool{}
	for _, id := range evidenceIDs {
		row, ok := byID[strings.TrimSpace(id)]
		if !ok {
			return fmt.Errorf("evidence_id %q was not returned to this turn; cite only ids from search_help or read_help_article", id)
		}
		if seen[row.DocumentID] {
			continue
		}
		seen[row.DocumentID] = true
		citations = append(citations, model.HelpAssistantCitation{ArticleID: row.DocumentID, Title: row.Title, URL: row.URL, PortalArticleID: row.ReferenceID})
	}
	return s.finish(ctx, turn.ID, model.HelpAssistantStatusAnswered, answer, citations)
}

// OfferContactSupport finishes the turn by offering to contact support.
func (s *HelpAssistantService) OfferContactSupport(ctx context.Context, meta model.InternalCommandContext, message string) error {
	_, turn, err := s.turnForRun(ctx, meta)
	if err != nil {
		return err
	}
	message = strings.TrimSpace(message)
	if message == "" || helpAssistantLink.MatchString(message) {
		message = helpAssistantDefaultHandoff
	}
	if err := s.finish(ctx, turn.ID, model.HelpAssistantStatusHandedOff, truncateRunes(message, 800), nil); err != nil {
		return err
	}
	s.recordUnansweredQuestion(ctx, turn)
	return nil
}

// recordUnansweredQuestion reports a question the help articles did not
// answer to Coverage gaps, like a help search without results.
func (s *HelpAssistantService) recordUnansweredQuestion(ctx context.Context, turn *model.HelpAssistantMessage) {
	if s.inbox == nil {
		return
	}
	messages, err := s.repo.ListMessages(ctx, turn.SessionID)
	if err != nil {
		slog.WarnContext(ctx, "help assistant question not read for coverage", "answer_id", turn.ID, "error", err)
		return
	}
	question := ""
	for _, message := range messages {
		if message.Role == model.HelpAssistantRoleUser && !message.CreatedAt.After(turn.CreatedAt) {
			question = message.Content
		}
	}
	if !IsMeaningfulCoverageSearchQuery(question) {
		return
	}
	s.inbox.recordSupportEvent(SupportEventInput{
		WorkspaceID:  turn.WorkspaceID,
		EventType:    model.SupportEventWidgetSearchPerformed,
		ActorType:    model.SupportEventActorCustomer,
		Channel:      "help_assistant",
		SourceSignal: "no_results",
		IssueSummary: truncateRunes(question, 300),
		Metadata:     map[string]any{"query": truncateRunes(question, 300), "result_count": 0, "surface": "help_assistant"},
	})
}

// SetProgress shows what the assistant is doing while the turn runs.
func (s *HelpAssistantService) SetProgress(ctx context.Context, meta model.InternalCommandContext, label string) error {
	_, turn, err := s.turnForRun(ctx, meta)
	if err != nil {
		return err
	}
	label = strings.TrimSpace(label)
	if label == "" || helpAssistantLink.MatchString(label) {
		return fmt.Errorf("label is required and may not contain links")
	}
	return s.repo.SetProgress(ctx, turn.ID, truncateRunes(label, helpAssistantMaxProgressRunes))
}

func (s *HelpAssistantService) finish(ctx context.Context, answerID, status, content string, citations []model.HelpAssistantCitation) error {
	closed, err := s.repo.Complete(ctx, answerID, status, content, citations)
	if err != nil {
		return err
	}
	if !closed {
		return fmt.Errorf("this turn is already finished; end your turn")
	}
	return nil
}

func helpAssistantEvidence(meta model.InternalCommandContext, ref helpArticleRef, evidenceID, content string, chunk repository.DocsChunkSearchResult) model.SupportRunEvidence {
	return model.SupportRunEvidence{
		WorkspaceID: meta.WorkspaceID, RunID: meta.RunID, EvidenceID: evidenceID,
		ReferenceID: ref.PortalArticleID, SourceType: helpAssistantEvidenceSource, SourceID: chunk.ID,
		DocumentID: ref.DocumentID, Title: ref.Title, URL: ref.URL, Content: content,
		LexicalScore: chunk.LexicalScore, VectorScore: chunk.VectorScore, CombinedScore: chunk.CombinedScore,
	}
}
