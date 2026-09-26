package service

// Knowledge retrieval: hybrid search, reranking, context building, plan
// normalization utilities, budget/locks, and the runtime search entrypoint.

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
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
	if embeddingsAvailable(ctx, s.embeddingProvider, workspaceID) {
		embedCtx := withAIActionMetering(ctx, workspaceID, aipolicy.ActionSupportKnowledgeEmbed, "support_knowledge_embed", query, map[string]interface{}{
			"surface": "support_knowledge", "agent_id": agentID,
		})
		resp, err := s.embeddingProvider.CreateEmbeddings(embedCtx, llm.EmbeddingRequest{
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
	reranked = s.semanticRerankKnowledgeResults(ctx, workspaceID, queries[0], reranked)
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
	} else {
		s.publishProgress(workspaceID, conversationID, supportAIProgressLooking)
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
	Queries []string
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
	var messages []model.SupportMessage
	if s.messageRepo != nil {
		messages, err = s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
		if err != nil {
			slog.WarnContext(ctx, "support knowledge tool: list messages for exact query failed", "error", err, "conversation_id", conversationID)
		}
	}
	effectiveQueries := supportKnowledgeQueries(messages, queries)
	results, err := s.searchSupportKnowledge(ctx, workspaceID, agentID, language, messages, queries)
	if err != nil {
		return nil, err
	}

	s.recordToolRetrievalTrace(ctx, workspaceID, conversationID, effectiveQueries, results)
	return &SupportKnowledgeSearchOutcome{AgentID: agentID, Results: results, Queries: effectiveQueries}, nil
}

// Both production and previews prepend the exact visitor message and use the
// same scoped retrieval/reranking pipeline. Only live turns emit coverage events.
func supportKnowledgeQueries(messages []model.SupportMessage, queries []string) []string {
	queries = dedupeQueries(prependVisitorKnowledgeQuery(messages, cloneStringSlice(queries)))
	if len(queries) > 4 {
		queries = queries[:4]
	}
	return queries
}
func (s *SupportAIService) searchSupportKnowledge(ctx context.Context, workspaceID, agentID, language string, messages []model.SupportMessage, queries []string) ([]KnowledgeSearchResult, error) {
	return s.loadKnowledgeChunks(ctx, workspaceID, agentID, language, supportKnowledgeQueries(messages, queries))
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
