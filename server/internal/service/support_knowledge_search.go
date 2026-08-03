package service

// Knowledge retrieval: hybrid search, reranking, context building, plan
// normalization utilities, budget/locks, and the runtime search entrypoint.

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// searchSingleQuery runs embedding + hybrid search for a single query string
// against both docs and content chunk repositories. It returns the merged results.
func (s *SupportAIService) searchSingleQuery(
	ctx context.Context,
	workspaceID, agentID, language, query string,
	spaceIDs, contentSourceIDs []string,
) ([]KnowledgeSearchResult, error) {
	// Create embedding for this query.
	queryEmbedding := ""
	embeddingModel := strings.TrimSpace(s.embeddingModel)
	if embeddingModel == "" {
		embeddingModel = defaultDocsEmbeddingModel
	}
	if s.embeddingProvider != nil {
		resp, err := s.embeddingProvider.CreateEmbeddings(ctx, llm.EmbeddingRequest{
			Model:  embeddingModel,
			Inputs: []string{query},
		})
		if err != nil {
			slog.WarnContext(ctx, "support query embedding failed; falling back to lexical retrieval",
				"error", err,
				"query_preview", safeLogPreview(query, 120),
			)
		} else if len(resp.Vectors) > 0 {
			queryEmbedding = formatVector(resp.Vectors[0])
		}
	}

	var results []KnowledgeSearchResult

	// Curated guidance is queried as its own source pool. Its repository applies
	// workspace, agent, status, validity, and language filters before ranking.
	if s.curatedGuidanceRepo != nil {
		guidanceResults, err := s.curatedGuidanceRepo.Search(
			ctx,
			workspaceID,
			agentID,
			language,
			query,
			queryEmbedding,
			embeddingModel,
			8,
		)
		if err != nil {
			return nil, fmt.Errorf("curated guidance search: %w", err)
		}
		for _, result := range guidanceResults {
			results = append(results, KnowledgeSearchResult{
				ID:            result.ID,
				ReferenceID:   knowledgeReferenceID(knowledgeSourceTypeGuidance, result.ID),
				SourceType:    knowledgeSourceTypeGuidance,
				IsInternal:    true,
				DocumentID:    result.ID,
				SourceID:      result.ID,
				Title:         result.Title,
				Content:       result.Answer,
				LexicalScore:  result.LexicalScore,
				VectorScore:   result.VectorScore,
				CombinedScore: result.CombinedScore,
			})
		}
	}

	// Search docs chunks.
	if s.docsChunkRepo != nil && len(spaceIDs) > 0 {
		docResults, err := s.docsChunkRepo.HybridSearchWithEmbeddingModel(ctx, workspaceID, spaceIDs, query, queryEmbedding, embeddingModel, 12)
		if err != nil {
			return nil, fmt.Errorf("docs hybrid search: %w", err)
		}
		for _, result := range docResults {
			results = append(results, KnowledgeSearchResult{
				ID:            result.ID,
				ReferenceID:   knowledgeReferenceID(knowledgeSourceTypeDocs, result.DocumentID),
				SourceType:    knowledgeSourceTypeDocs,
				IsInternal:    result.SpaceType == model.SpaceTypeInternal,
				DocumentID:    result.DocumentID,
				BlockID:       derefString(result.BlockID),
				SourceID:      result.SpaceID,
				ChunkIndex:    result.ChunkIndex,
				SectionKey:    result.SectionKey,
				HeadingPath:   result.HeadingPath,
				Title:         result.Title,
				Content:       result.Content,
				LexicalScore:  result.LexicalScore,
				VectorScore:   result.VectorScore,
				CombinedScore: result.CombinedScore,
			})
		}
	}

	// Search content chunks.
	if s.contentChunkRepo != nil && len(contentSourceIDs) > 0 {
		contentResults, err := s.contentChunkRepo.HybridSearchWithEmbeddingModel(ctx, workspaceID, contentSourceIDs, query, queryEmbedding, embeddingModel, 12)
		if err != nil {
			return nil, fmt.Errorf("content hybrid search: %w", err)
		}
		for _, result := range contentResults {
			results = append(results, KnowledgeSearchResult{
				ID:            result.ID,
				ReferenceID:   knowledgeReferenceID(knowledgeSourceTypeContent, result.PageID),
				SourceType:    knowledgeSourceTypeContent,
				DocumentID:    result.PageID,
				SourceID:      result.ContentSourceID,
				ChunkIndex:    result.ChunkIndex,
				SectionKey:    result.SectionKey,
				HeadingPath:   result.HeadingPath,
				Title:         result.Title,
				URL:           result.URL,
				Content:       result.Content,
				LexicalScore:  result.LexicalScore,
				VectorScore:   result.VectorScore,
				CombinedScore: result.CombinedScore,
			})
		}
	}

	return results, nil
}

func (s *SupportAIService) loadKnowledgeChunks(ctx context.Context, workspaceID, agentID, language string, queries []string) ([]KnowledgeSearchResult, error) {
	if s == nil {
		return nil, nil
	}

	// Resolve released public knowledge once (shared across query variants).
	// External docs and crawled website content are workspace-wide. The agent ID
	// remains relevant only for private curated guidance.
	var spaceIDs []string
	if s.docsChunkRepo != nil {
		ids, err := s.docsChunkRepo.ListSearchableExternalSpaceIDs(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		spaceIDs = ids
	}

	var contentSourceIDs []string
	if s.contentChunkRepo != nil {
		ids, err := s.contentChunkRepo.ListSearchableSourceIDs(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		contentSourceIDs = ids
	}

	// Nothing to search against.
	if len(spaceIDs) == 0 && len(contentSourceIDs) == 0 && s.curatedGuidanceRepo == nil {
		return nil, nil
	}

	queries = dedupeQueries(queries)
	if len(queries) == 0 {
		return nil, nil
	}

	// Run searches concurrently for each query variant.
	var mu sync.Mutex
	allResults := make([]KnowledgeSearchResult, 0, 24*len(queries))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4) // Bound concurrency to avoid overwhelming the DB.
	for _, q := range queries {
		q := q // capture loop variable
		g.Go(func() error {
			results, err := s.searchSingleQuery(gctx, workspaceID, agentID, language, q, spaceIDs, contentSourceIDs)
			if err != nil {
				return err
			}
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Deduplicate equivalent chunks across repeated queries and duplicate crawls
	// of the same URL, keeping the strongest copy.
	deduped := dedupeKnowledgeResults(allResults)

	reranked := rerankKnowledgeResults(queries[0], deduped)
	reranked = s.semanticRerankKnowledgeResults(ctx, queries[0], reranked)
	reranked = applyKnowledgeAuthorityRanking(queries[0], reranked)
	var err error
	reranked, err = s.ensureCanonicalPricingLeadChunks(ctx, workspaceID, "", queries[0], reranked)
	if err != nil {
		return nil, err
	}
	reranked = limitKnowledgeResultsPerDocument(reranked, 2)
	reranked = filterConflictingPricingEvidence(queries[0], reranked)
	// Public neighbors use the same workspace-wide scope as the seed results.
	reranked, err = s.expandKnowledgeNeighbors(ctx, workspaceID, "", reranked, 12)
	if err != nil {
		return nil, err
	}
	reranked = filterConflictingPricingEvidence(queries[0], reranked)

	// Cap final results to avoid oversized context.
	if len(reranked) > 12 {
		reranked = reranked[:12]
	}

	return reranked, nil
}

func buildKnowledgeContext(results []KnowledgeSearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	for idx, result := range results {
		if idx >= 8 {
			break
		}
		if result.IsInternal {
			authority := "standard"
			if result.SourceType == knowledgeSourceTypeGuidance {
				authority = "maximum_applicable"
			}
			sb.WriteString(fmt.Sprintf(
				"---\nEVIDENCE_ID: %s\nVISIBILITY: INTERNAL\nSOURCE_TYPE: %s\nAUTHORITY: %s\nTITLE: Internal guidance\nHEADING_PATH: Internal section\nCHUNK_INDEX: %d\nCONTENT:\n%s\n",
				result.ID,
				result.SourceType,
				authority,
				result.ChunkIndex,
				result.Content,
			))
			continue
		}
		if strings.TrimSpace(result.URL) != "" {
			sb.WriteString(fmt.Sprintf(
				"---\nEVIDENCE_ID: %s\nVISIBILITY: PUBLIC\nDOC_ID: %s\nSOURCE_TYPE: %s\nTITLE: %s\nHEADING_PATH: %s\nURL: %s\nCHUNK_INDEX: %d\nCONTENT:\n%s\n",
				result.ID,
				result.ReferenceID,
				result.SourceType,
				result.Title,
				result.HeadingPath,
				result.URL,
				result.ChunkIndex,
				result.Content,
			))
		} else {
			sb.WriteString(fmt.Sprintf(
				"---\nEVIDENCE_ID: %s\nVISIBILITY: PUBLIC\nDOC_ID: %s\nSOURCE_TYPE: %s\nTITLE: %s\nHEADING_PATH: %s\nCHUNK_INDEX: %d\nCONTENT:\n%s\n",
				result.ID,
				result.ReferenceID,
				result.SourceType,
				result.Title,
				result.HeadingPath,
				result.ChunkIndex,
				result.Content,
			))
		}
	}
	return sb.String()
}

func buildAISources(sourceDocIDs []string, searchResults []KnowledgeSearchResult) []AISource {
	if len(sourceDocIDs) == 0 || len(searchResults) == 0 {
		return nil
	}

	byDocID := map[string]KnowledgeSearchResult{}
	for _, result := range searchResults {
		if result.IsInternal {
			continue
		}
		current, ok := byDocID[result.ReferenceID]
		if !ok || result.CombinedScore > current.CombinedScore {
			byDocID[result.ReferenceID] = result
		}
	}

	seenDocs := map[string]struct{}{}
	sources := make([]AISource, 0, len(sourceDocIDs))
	for _, docID := range sourceDocIDs {
		if _, seen := seenDocs[docID]; seen {
			continue
		}
		result, ok := byDocID[docID]
		if !ok {
			continue
		}
		seenDocs[docID] = struct{}{}
		sources = append(sources, AISource{
			DocID:      docID,
			BlockID:    result.BlockID,
			Title:      result.Title,
			Snippet:    excerptText(result.Content, 180),
			Confidence: supportEvidenceRetrievalQuality(result),
			SourceType: result.SourceType,
			URL:        result.URL,
		})
	}
	return sources
}

func publicSourceDocIDs(sourceDocIDs []string, searchResults []KnowledgeSearchResult) []string {
	publicIDs := make(map[string]struct{}, len(searchResults))
	for _, result := range searchResults {
		if !result.IsInternal {
			publicIDs[result.ReferenceID] = struct{}{}
		}
	}

	filtered := make([]string, 0, len(sourceDocIDs))
	seen := make(map[string]struct{}, len(sourceDocIDs))
	for _, sourceDocID := range sourceDocIDs {
		if _, ok := publicIDs[sourceDocID]; !ok {
			continue
		}
		if _, ok := seen[sourceDocID]; ok {
			continue
		}
		seen[sourceDocID] = struct{}{}
		filtered = append(filtered, sourceDocID)
	}
	return filtered
}

func rerankKnowledgeResults(query string, results []KnowledgeSearchResult) []KnowledgeSearchResult {
	if len(results) == 0 {
		return nil
	}

	queryTerms := normalizedTerms(query)
	reranked := make([]KnowledgeSearchResult, len(results))
	copy(reranked, results)

	for idx := range reranked {
		lexical := clamp01(reranked[idx].LexicalScore / 0.35)
		bodyOverlap := termOverlapScore(queryTerms, reranked[idx].Content)
		titleOverlap := termOverlapScore(queryTerms, reranked[idx].Title)
		reranked[idx].CombinedScore = (reranked[idx].VectorScore * 0.35) +
			(lexical * 0.2) +
			(bodyOverlap * 0.25) +
			(titleOverlap * 0.15) +
			(reranked[idx].CombinedScore * 4)
		if reranked[idx].SourceType == knowledgeSourceTypeGuidance {
			reranked[idx].CombinedScore += 100
		}
	}

	sort.SliceStable(reranked, func(i, j int) bool {
		return reranked[i].CombinedScore > reranked[j].CombinedScore
	})

	return reranked
}

func limitKnowledgeResultsPerDocument(results []KnowledgeSearchResult, limit int) []KnowledgeSearchResult {
	if limit <= 0 || len(results) == 0 {
		return results
	}
	byDocCount := map[string]int{}
	limited := make([]KnowledgeSearchResult, 0, len(results))
	for _, result := range results {
		documentKey := knowledgeResultDocumentKey(result)
		if byDocCount[documentKey] >= limit {
			continue
		}
		byDocCount[documentKey]++
		limited = append(limited, result)
	}
	return limited
}

func dedupeKnowledgeResults(results []KnowledgeSearchResult) []KnowledgeSearchResult {
	seen := make(map[string]int, len(results))
	deduped := make([]KnowledgeSearchResult, 0, len(results))
	for _, result := range results {
		key := knowledgeResultChunkKey(result)
		if idx, ok := seen[key]; ok {
			if result.CombinedScore > deduped[idx].CombinedScore {
				deduped[idx] = result
			}
			continue
		}
		seen[key] = len(deduped)
		deduped = append(deduped, result)
	}
	return deduped
}

func knowledgeResultChunkKey(result KnowledgeSearchResult) string {
	if normalizedURL := normalizeKnowledgeResultURL(result.URL); normalizedURL != "" {
		return fmt.Sprintf("url:%s|chunk:%d|section:%s", normalizedURL, result.ChunkIndex, strings.TrimSpace(result.SectionKey))
	}
	if strings.TrimSpace(result.ReferenceID) != "" {
		return fmt.Sprintf("ref:%s|chunk:%d|section:%s", strings.TrimSpace(result.ReferenceID), result.ChunkIndex, strings.TrimSpace(result.SectionKey))
	}
	return "id:" + result.ID
}

func knowledgeResultDocumentKey(result KnowledgeSearchResult) string {
	if normalizedURL := normalizeKnowledgeResultURL(result.URL); normalizedURL != "" {
		return "url:" + normalizedURL
	}
	if strings.TrimSpace(result.ReferenceID) != "" {
		return "ref:" + strings.TrimSpace(result.ReferenceID)
	}
	return "id:" + result.ID
}

func normalizeKnowledgeResultURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if parsed.Path != "/" {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	return parsed.String()
}

func applyKnowledgeAuthorityRanking(query string, results []KnowledgeSearchResult) []KnowledgeSearchResult {
	if len(results) < 2 || !isPricingKnowledgeQuery(query) {
		return results
	}
	reranked := append([]KnowledgeSearchResult(nil), results...)
	for idx := range reranked {
		switch {
		case isCanonicalPricingURL(reranked[idx].URL):
			reranked[idx].CombinedScore += 5
		case isArticleKnowledgeURL(reranked[idx].URL):
			reranked[idx].CombinedScore -= 1
		}
	}
	sort.SliceStable(reranked, func(i, j int) bool {
		return reranked[i].CombinedScore > reranked[j].CombinedScore
	})
	return reranked
}

// filterConflictingPricingEvidence prevents a stale high-overlap marketing
// page from supplying a different price when the same retrieval already found
// the canonical pricing page. Comparative questions keep both sides because
// the visitor explicitly asked for a comparison.
func filterConflictingPricingEvidence(query string, results []KnowledgeSearchResult) []KnowledgeSearchResult {
	if len(results) < 2 || !isPricingKnowledgeQuery(query) || isComparativeKnowledgeQuery(query) {
		return results
	}
	hasCanonicalPricing := false
	for _, result := range results {
		if isCanonicalPricingURL(result.URL) {
			hasCanonicalPricing = true
			break
		}
	}
	if !hasCanonicalPricing {
		return results
	}
	filtered := make([]KnowledgeSearchResult, 0, len(results))
	for _, result := range results {
		if result.IsInternal || isCanonicalPricingURL(result.URL) || !currencyValuePattern.MatchString(result.Content) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func isPricingKnowledgeQuery(query string) bool {
	for _, term := range normalizedTerms(query) {
		switch term {
		case "price", "prices", "pricing", "cost", "costs", "plan", "plans", "subscription", "subscriptions", "billing":
			return true
		}
	}
	return false
}

func isComparativeKnowledgeQuery(query string) bool {
	for _, term := range normalizedTerms(query) {
		switch term {
		case "compare", "compared", "comparison", "comparisons", "versus", "vs", "alternative", "alternatives":
			return true
		}
	}
	return false
}

func isCanonicalPricingURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || isArticleKnowledgeURL(rawURL) {
		return false
	}
	segments := strings.Split(strings.Trim(strings.ToLower(parsed.Path), "/"), "/")
	if len(segments) == 0 {
		return false
	}
	switch segments[len(segments)-1] {
	case "pricing", "plans", "pricing-plans", "plans-pricing", "subscriptions":
		return true
	default:
		return false
	}
}

func isArticleKnowledgeURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	path := "/" + strings.Trim(strings.ToLower(parsed.Path), "/") + "/"
	for _, segment := range []string{"/blog/", "/blogs/", "/article/", "/articles/", "/news/", "/guides/", "/compare/", "/comparison/", "/comparisons/"} {
		if strings.Contains(path, segment) {
			return true
		}
	}
	return false
}

func defaultSupportQueryPlan(customerMessage string) SupportQueryPlanContract {
	current := strings.TrimSpace(customerMessage)
	definition := supportIntentDefinition(supportIntentUnknown)
	queries := []string{}
	if current != "" {
		queries = []string{current}
	}
	return SupportQueryPlanContract{
		Route:            supportDecisionAnswer,
		Decision:         supportDecisionAnswer,
		Intent:           definition.ID,
		Subject:          "",
		Language:         "",
		Risk:             definition.Risk,
		RequiredEvidence: cloneStringSlice(definition.RequiredEvidence),
		EvidenceMode:     definition.EvidenceMode,
		RegistryVersion:  definition.Version,
		ContextAction:    "new_issue",
		IssueKey:         normalizeSupportIssueKey("", current),
		IssueSummary:     normalizeSupportIssueSummary("", current),
		ProgressSignal:   supportProgressNewIssue,
		StandaloneQuery:  current,
		SearchQueries:    queries,
		Reason:           "planner_unavailable",
	}
}

func normalizeSupportLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.ReplaceAll(language, "_", "-")
	if language == "" {
		return ""
	}
	parts := strings.Split(language, "-")
	if len(language) > 16 || len(parts[0]) < 2 || len(parts[0]) > 3 {
		return ""
	}
	for partIndex, part := range parts {
		if part == "" || len(part) > 8 {
			return ""
		}
		for _, value := range part {
			isAlpha := value >= 'a' && value <= 'z'
			isDigit := value >= '0' && value <= '9'
			if (!isAlpha && partIndex == 0) || (!isAlpha && !isDigit && partIndex > 0) {
				return ""
			}
		}
	}
	return language
}

func normalizeSupportContextAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "continue", "confirm_previous":
		return strings.ToLower(strings.TrimSpace(action))
	default:
		return "new_issue"
	}
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func dedupeQueries(queries []string) []string {
	if len(queries) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(queries))
	for _, query := range queries {
		trimmed := strings.TrimSpace(query)
		if trimmed == "" {
			continue
		}
		key := normalizeQueryKey(trimmed)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	return deduped
}

func normalizeSupportQueryPlan(plan SupportQueryPlanContract, customerMessage string) SupportQueryPlanContract {
	current := strings.TrimSpace(customerMessage)
	normalized := defaultSupportQueryPlan(current)
	route := strings.ToLower(strings.TrimSpace(plan.Route))
	if route == "" {
		route = strings.ToLower(strings.TrimSpace(plan.Decision))
	}
	if route == supportDecisionGreet {
		route = supportRouteConversational
	}
	definition := normalizeSupportIntent(plan.Intent, plan.RequiredEvidence)
	issueKey := normalizeSupportIssueKey(plan.IssueKey, current)
	issueSummary := normalizeSupportIssueSummary(plan.IssueSummary, current)
	progressSignal := normalizeSupportProgressSignal(plan.ProgressSignal)
	contextAction := normalizeSupportContextAction(plan.ContextAction)
	subject := strings.Join(strings.Fields(strings.TrimSpace(plan.Subject)), " ")
	if len(subject) > 120 {
		subject = strings.TrimSpace(subject[:120])
	}
	language := normalizeSupportLanguage(plan.Language)
	applyRegistry := func(result *SupportQueryPlanContract) {
		result.Intent = definition.ID
		result.Subject = subject
		result.Language = language
		result.Risk = definition.Risk
		result.RequiredEvidence = cloneStringSlice(definition.RequiredEvidence)
		result.EvidenceMode = definition.EvidenceMode
		result.RegistryVersion = definition.Version
		result.ContextAction = contextAction
	}

	switch route {
	case supportRouteConversational:
		reply := strings.TrimSpace(plan.Reply)
		if reply == "" {
			reply = strings.TrimSpace(plan.GreetingReply)
		}
		if reply == "" {
			return normalized
		}
		result := SupportQueryPlanContract{
			Route:          supportRouteConversational,
			Decision:       supportDecisionGreet,
			Reply:          reply,
			ProgressSignal: defaultPlannerProgressSignal(progressSignal, supportProgressNewIssue),
			SearchQueries:  []string{},
			GreetingReply:  reply,
			Reason:         normalizedPlannerReason(plan.Reason, "conversational"),
		}
		applyRegistry(&result)
		return result
	case supportDecisionConfirm:
		reply := strings.TrimSpace(plan.Reply)
		if reply == "" {
			return normalized
		}
		result := SupportQueryPlanContract{
			Route:          supportDecisionConfirm,
			Decision:       supportDecisionConfirm,
			Reply:          reply,
			ProgressSignal: defaultPlannerProgressSignal(progressSignal, supportProgressSameNewInfo),
			SearchQueries:  []string{},
			Reason:         normalizedPlannerReason(plan.Reason, "confirmation"),
		}
		result.ContextAction = "confirm_previous"
		applyRegistry(&result)
		result.ContextAction = "confirm_previous"
		return result
	case supportDecisionClarify:
		question := strings.TrimSpace(plan.Reply)
		if question == "" {
			question = strings.TrimSpace(plan.ClarifyingQuestion)
		}
		if question == "" {
			normalized.IssueKey = issueKey
			normalized.IssueSummary = issueSummary
			normalized.ProgressSignal = defaultPlannerProgressSignal(progressSignal, supportProgressSameUnclear)
			return normalized
		}
		result := SupportQueryPlanContract{
			Route:              supportDecisionClarify,
			Decision:           supportDecisionClarify,
			Reply:              question,
			IssueKey:           issueKey,
			IssueSummary:       issueSummary,
			ProgressSignal:     defaultPlannerProgressSignal(progressSignal, supportProgressSameUnclear),
			SearchQueries:      []string{},
			ClarifyingQuestion: question,
			Reason:             normalizedPlannerReason(plan.Reason, "needs_clarification"),
		}
		applyRegistry(&result)
		return result
	case supportDecisionHandoff:
		result := SupportQueryPlanContract{
			Route:          supportDecisionHandoff,
			Decision:       supportDecisionHandoff,
			Reply:          strings.TrimSpace(plan.Reply),
			IssueKey:       issueKey,
			IssueSummary:   issueSummary,
			ProgressSignal: defaultPlannerProgressSignal(progressSignal, supportProgressSameRepeat),
			SearchQueries:  []string{},
			Reason:         normalizedPlannerReason(plan.Reason, "planner_handoff"),
		}
		applyRegistry(&result)
		return result
	default:
		standalone := strings.TrimSpace(plan.StandaloneQuery)
		if standalone == "" {
			standalone = current
		}
		searchQueries := dedupeQueries(append([]string{standalone}, plan.SearchQueries...))
		if len(searchQueries) > 4 {
			searchQueries = searchQueries[:4]
		}
		if len(searchQueries) == 0 && standalone != "" {
			searchQueries = []string{standalone}
		}
		if len(searchQueries) == 0 && current != "" {
			searchQueries = []string{current}
		}
		result := SupportQueryPlanContract{
			Route:           supportDecisionAnswer,
			Decision:        supportDecisionAnswer,
			IssueKey:        issueKey,
			IssueSummary:    issueSummary,
			ProgressSignal:  defaultPlannerProgressSignal(progressSignal, supportProgressNewIssue),
			StandaloneQuery: standalone,
			SearchQueries:   searchQueries,
			Reason:          normalizedPlannerReason(plan.Reason, "resolved_from_context"),
		}
		applyRegistry(&result)
		return result
	}
}
func defaultPlannerProgressSignal(candidate, fallback string) string {
	if candidate != "" {
		return candidate
	}
	return fallback
}

// normalizedPlannerReason lowercases free text into a snake_case token.
func normalizedPlannerReason(raw, fallback string) string {
	candidate := strings.ToLower(strings.TrimSpace(raw))
	if candidate == "" {
		return fallback
	}
	var sb strings.Builder
	lastUnderscore := false
	for _, r := range candidate {
		switch {
		case r >= 'a' && r <= 'z':
			sb.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			sb.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && sb.Len() > 0 {
				sb.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	reason := strings.Trim(sb.String(), "_")
	if reason == "" {
		return fallback
	}
	return reason
}

func normalizeSupportIssueKey(raw, fallback string) string {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		candidate = strings.TrimSpace(fallback)
	}
	if candidate == "" {
		return ""
	}
	tokens := tokenizeWords(candidate)
	if len(tokens) == 0 {
		return normalizedPlannerReason(candidate, "")
	}
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		switch token {
		case "a", "an", "and", "are", "do", "for", "help", "i", "is", "it", "me", "my", "of", "on", "please", "the", "to", "we", "with", "you":
			continue
		default:
			filtered = append(filtered, token)
		}
		if len(filtered) >= 6 {
			break
		}
	}
	if len(filtered) == 0 {
		filtered = tokens
		if len(filtered) > 6 {
			filtered = filtered[:6]
		}
	}
	return strings.Join(filtered, "_")
}

func normalizeSupportIssueSummary(raw, fallback string) string {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		candidate = strings.TrimSpace(fallback)
	}
	if candidate == "" {
		return ""
	}
	if len(candidate) <= 140 {
		return candidate
	}
	return strings.TrimSpace(candidate[:140])
}

func normalizeSupportProgressSignal(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case supportProgressNewIssue:
		return supportProgressNewIssue
	case supportProgressSameNewInfo:
		return supportProgressSameNewInfo
	case supportProgressSameRepeat:
		return supportProgressSameRepeat
	case supportProgressSameUnclear:
		return supportProgressSameUnclear
	default:
		return ""
	}
}

func normalizeQueryKey(query string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(query))), " ")
}

func safeLogPreview(content string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 120
	}
	normalized := strings.Join(strings.Fields(stripPII(strings.TrimSpace(content))), " ")
	if normalized == "" {
		return ""
	}
	return truncateLog(normalized, maxLen)
}

func knowledgeReferenceID(sourceType, id string) string {
	return sourceType + ":" + id
}

func normalizedTerms(input string) []string {
	rawTerms := strings.Fields(strings.ToLower(input))
	if len(rawTerms) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	terms := make([]string, 0, len(rawTerms))
	for _, raw := range rawTerms {
		term := strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z':
				return r
			case r >= '0' && r <= '9':
				return r
			default:
				return -1
			}
		}, raw)
		if len(term) < 2 {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return terms
}

func termOverlapScore(queryTerms []string, text string) float64 {
	if len(queryTerms) == 0 {
		return 0
	}

	lower := strings.ToLower(text)
	matches := 0
	for _, term := range queryTerms {
		if strings.Contains(lower, term) {
			matches++
		}
	}
	return clamp01(float64(matches) / float64(len(queryTerms)))
}

func excerptText(text string, maxLen int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if maxLen <= 0 || len(normalized) <= maxLen {
		return normalized
	}
	return strings.TrimSpace(normalized[:maxLen]) + "..."
}

func resolveSupportLLMConfig(agent *model.Agent) (string, string) {
	provider := model.AgentModelProviderAnthropic
	if agent != nil && agent.Provider != nil && strings.TrimSpace(*agent.Provider) != "" {
		provider = strings.TrimSpace(*agent.Provider)
	}

	if agent != nil && agent.Model != nil && strings.TrimSpace(*agent.Model) != "" {
		return provider, strings.TrimSpace(*agent.Model)
	}

	switch provider {
	case model.AgentModelProviderOpenAI:
		return provider, "gpt-5-mini"
	case model.AgentModelProviderOpenRouter, model.AgentModelProviderOpenRouterResponses:
		return provider, "openai/gpt-5-mini"
	default:
		return model.AgentModelProviderAnthropic, "claude-sonnet-4-6"
	}
}

// checkTokenBudget returns true if the agent has budget remaining.
func (s *SupportAIService) checkTokenBudget(agent *model.Agent) bool {
	if agent.MonthlyTokenBudget == nil {
		return true // no budget configured = unlimited
	}
	return agent.TokensUsedThisMonth < *agent.MonthlyTokenBudget
}

// recordTokenUsage atomically increments the agent's token usage counter.
func (s *SupportAIService) recordTokenUsage(ctx context.Context, agentID string, tokensUsed int) {
	if err := s.db.WithContext(ctx).
		Model(&model.Agent{}).
		Where("id = ?", agentID).
		Update("tokens_used_this_month", gorm.Expr("tokens_used_this_month + ?", tokensUsed)).
		Error; err != nil {
		slog.ErrorContext(ctx, "record token usage failed", "agent_id", agentID, "error", err)
	}
}

// acquireLock acquires a Redis SETNX lock with TTL.
func (s *SupportAIService) acquireLock(ctx context.Context, key string) bool {
	if s.redis == nil {
		return true // no Redis = no lock
	}
	result, err := s.redis.SetArgs(ctx, key, "1", redis.SetArgs{
		Mode: "NX",
		TTL:  60 * time.Second,
	}).Result()
	if err != nil && err != redis.Nil {
		slog.ErrorContext(ctx, "redis lock acquire failed", "key", key, "error", err)
		return true // proceed on Redis failure
	}
	return result == "OK"
}

// supportChatDailyReplyLimit caps AI turns per workspace per UTC day — a
// spend/abuse ceiling behind the per-conversation caps and widget rate
// limits. Exceeding it hands conversations to humans for the rest of the day.
const supportChatDailyReplyLimit = 500

// consumeDailyReplyBudget increments the workspace's daily AI-turn counter
// and reports whether the turn is within budget. Fails open without Redis.
func (s *SupportAIService) consumeDailyReplyBudget(ctx context.Context, workspaceID string) bool {
	if s.redis == nil {
		return true
	}
	key := "support:ai:daily:" + workspaceID + ":" + time.Now().UTC().Format("20060102")
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		slog.WarnContext(ctx, "support daily budget check failed", "workspace_id", workspaceID, "error", err)
		return true
	}
	if count == 1 {
		s.redis.Expire(ctx, key, 48*time.Hour)
	}
	return count <= supportChatDailyReplyLimit
}

// releaseLock releases a Redis lock.
func (s *SupportAIService) releaseLock(ctx context.Context, key string) {
	if s.redis == nil {
		return
	}
	s.redis.Del(ctx, key)
}

// publishTypingIndicator sends an AI thinking event to widget + inbox via WebSocket.
// This is distinct from human typing — the widget renders it as "Thinking..." with a shimmer.
func (s *SupportAIService) publishTypingIndicator(_ context.Context, workspaceID, conversationID string, isThinking bool) {
	action := "ai_thinking_started"
	if !isThinking {
		action = "ai_thinking_stopped"
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     "ai",
	})
}

// piiRegexes for stripping common PII patterns from AI responses.
var piiRegexes = []*regexp.Regexp{
	regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`), // email
	regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),                      // US phone
	regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),                              // SSN
}

// truncateLog truncates a string for safe logging.
func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// stripPII removes common PII patterns from text.
func stripPII(content string) string {
	result := content
	for _, re := range piiRegexes {
		result = re.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

func stripConversationPII(content string, customerEmail, customerPhone *string) string {
	result := content

	for _, value := range []string{derefString(customerEmail), derefString(customerPhone)} {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(trimmed))
		result = re.ReplaceAllString(result, "[REDACTED]")
	}

	// Keep generic redaction for highly sensitive identifiers even in
	// customer-facing text.
	result = piiRegexes[2].ReplaceAllString(result, "[REDACTED]")
	return result
}

// SupportKnowledgeSearchOutcome is what the search_knowledge runtime tool
// receives: the resolved support agent plus the agent-scoped search results.
type SupportKnowledgeSearchOutcome struct {
	AgentID string
	Results []KnowledgeSearchResult
}

// SearchKnowledgeForConversation runs the agent-scoped knowledge search for a
// support conversation on behalf of the search_knowledge runtime tool. It
// resolves the workspace's configured support agent, searches docs, crawled
// content, and curated guidance, and emits a coverage retrieval trace tied to
// the conversation's latest customer message.
func (s *SupportAIService) SearchKnowledgeForConversation(ctx context.Context, workspaceID, conversationID, language string, queries []string) (*SupportKnowledgeSearchOutcome, error) {
	if s == nil {
		return nil, fmt.Errorf("support AI service is not configured")
	}
	settings, err := s.loadSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if settings.AIAgentID == nil || strings.TrimSpace(*settings.AIAgentID) == "" {
		return nil, fmt.Errorf("no support AI agent is configured for this workspace")
	}
	agentID := strings.TrimSpace(*settings.AIAgentID)
	effectiveQueries := cloneStringSlice(queries)
	if s.messageRepo != nil {
		messages, listErr := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
		if listErr != nil {
			slog.WarnContext(ctx, "support knowledge tool: list messages for exact query failed", "error", listErr, "conversation_id", conversationID)
		} else {
			effectiveQueries = prependVisitorKnowledgeQuery(messages, effectiveQueries)
		}
	}
	effectiveQueries = dedupeQueries(effectiveQueries)
	if len(effectiveQueries) > 4 {
		effectiveQueries = effectiveQueries[:4]
	}

	results, err := s.loadKnowledgeChunks(ctx, workspaceID, agentID, language, effectiveQueries)
	if err != nil {
		return nil, err
	}

	s.recordToolRetrievalTrace(ctx, workspaceID, conversationID, effectiveQueries, results)
	return &SupportKnowledgeSearchOutcome{AgentID: agentID, Results: results}, nil
}

func prependVisitorKnowledgeQuery(messages []model.SupportMessage, queries []string) []string {
	for idx := len(messages) - 1; idx >= 0; idx-- {
		if messages[idx].SenderType != "customer" {
			continue
		}
		if visitorQuery := strings.TrimSpace(supportMessagePromptText(messages[idx])); visitorQuery != "" {
			return append([]string{visitorQuery}, queries...)
		}
	}
	return queries
}

// recordToolRetrievalTrace keeps the coverage-analytics feed alive for
// tool-driven retrieval: the trace is attached to the conversation's latest
// customer message, marked with origin runtime_tool.
func (s *SupportAIService) recordToolRetrievalTrace(ctx context.Context, workspaceID, conversationID string, queries []string, results []KnowledgeSearchResult) {
	if s.traceRecorder == nil || s.messageRepo == nil {
		return
	}
	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		slog.WarnContext(ctx, "support knowledge tool: list messages for trace failed",
			"error", err, "workspace_id", workspaceID, "conversation_id", conversationID)
		return
	}
	messageID := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].SenderType == "customer" {
			messageID = messages[i].ID
			break
		}
	}
	if messageID == "" {
		return
	}
	trace, err := BuildSupportAIRetrievalTrace(SupportAIRetrievalTraceInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		MessageID:      messageID,
		SearchQueries:  queries,
		SearchResults:  results,
		Metadata:       map[string]any{"origin": "runtime_tool"},
	})
	if err != nil {
		slog.WarnContext(ctx, "support knowledge tool: build retrieval trace failed",
			"error", err, "workspace_id", workspaceID, "conversation_id", conversationID)
		return
	}
	s.recordSupportAIRetrievalTraceBestEffort(trace)
}
