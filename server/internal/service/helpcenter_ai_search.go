package service

// Public help center AI search: semantic (hybrid chunk retrieval grouped into
// article results) and grounded one-shot answers. Answers are validated
// server-side — citations must reference retrieved chunks, links must point at
// cited articles — and cached keyed by query + a content fingerprint, so
// repeat questions cost nothing and publishing invalidates naturally. This is
// deliberately NOT an agent run: anonymous search traffic needs bounded cost
// and sub-second-to-few-second latency; the conversational agent lives in the
// support widget.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	helpcenterSemanticChunkLimit    = 24
	helpcenterSemanticArticleLimit  = 8
	helpcenterSnippetsPerArticle    = 3
	helpcenterSnippetChars          = 280
	helpcenterAnswerChunkLimit      = 10
	helpcenterAnswerContextChars    = 1500
	helpcenterAnswerMinQueryChars   = 8
	helpcenterAnswerMaxQueryChars   = 400
	helpcenterAnswerConfidenceFloor = 0.5
	helpcenterAnswerTimeout         = 25 * time.Second

	// helpcenterAnswerDailyLimit caps LLM answer generations per workspace per
	// UTC day (cache hits are free and do not count).
	helpcenterAnswerDailyLimit = 300

	defaultHelpcenterAnswerProvider = "openrouter"
	defaultHelpcenterAnswerModel    = "deepseek/deepseek-v4-flash-0731"
	// Cheap-tier fallbacks when only one provider key is configured.
	fallbackOpenAIAnswerModel    = "gpt-5-mini"
	fallbackAnthropicAnswerModel = "claude-haiku-4-5"
)

// ResolveHelpcenterAnswerRouting picks the chat provider/model for public
// help-center answers. Explicit configuration (HELPCENTER_ANSWER_PROVIDER /
// HELPCENTER_ANSWER_MODEL) wins; otherwise the first configured provider key
// decides, each with a cheap-tier default model. Empty results mean no chat
// provider is configured — answer generation is disabled and asks degrade to
// the article list.
func ResolveHelpcenterAnswerRouting(explicitProvider, explicitModel string, hasOpenRouter, hasOpenAI, hasAnthropic bool) (string, string) {
	provider := strings.ToLower(strings.TrimSpace(explicitProvider))
	explicitModel = strings.TrimSpace(explicitModel)
	if provider == "" {
		switch {
		case hasOpenRouter:
			provider = model.AgentModelProviderOpenRouter
		case hasOpenAI:
			provider = model.AgentModelProviderOpenAI
		case hasAnthropic:
			provider = model.AgentModelProviderAnthropic
		default:
			return "", ""
		}
	}
	if explicitModel != "" {
		return provider, explicitModel
	}
	switch provider {
	case model.AgentModelProviderOpenAI:
		return provider, fallbackOpenAIAnswerModel
	case model.AgentModelProviderAnthropic:
		return provider, fallbackAnthropicAnswerModel
	default:
		return provider, defaultHelpcenterAnswerModel
	}
}

// HelpcenterAISearchService serves public semantic search and grounded
// answers over published help-center content.
type HelpcenterAISearchService struct {
	chunkRepo         *repository.DocsChunkRepository
	searchRepo        *repository.DocsSearchRepository
	answerRepo        *repository.HelpcenterAnswerRepository
	embeddingProvider llm.EmbeddingProvider
	embeddingModel    string
	llmProvider       llm.Provider
	answerProvider    string
	answerModel       string
	redis             *redis.Client
	autoIndexer       helpcenterAutoIndexer
}

// helpcenterAutoIndexer backfills chunk indexing for a workspace's help
// center spaces; *DocsEmbeddingService satisfies it.
type helpcenterAutoIndexer interface {
	QueueHelpcenterAutoIndex(ctx context.Context, workspaceID string) error
}

// SetAutoIndexer wires the lazy chunk-backfill used when a workspace has
// published articles but no chunks yet.
func (s *HelpcenterAISearchService) SetAutoIndexer(indexer helpcenterAutoIndexer) {
	s.autoIndexer = indexer
}

// NewHelpcenterAISearchService creates a HelpcenterAISearchService.
// answerProvider/answerModel override the flash-tier default when non-empty.
func NewHelpcenterAISearchService(
	chunkRepo *repository.DocsChunkRepository,
	searchRepo *repository.DocsSearchRepository,
	answerRepo *repository.HelpcenterAnswerRepository,
	embeddingProvider llm.EmbeddingProvider,
	embeddingModel string,
	llmProvider llm.Provider,
	answerProvider string,
	answerModel string,
	redisClient *redis.Client,
) *HelpcenterAISearchService {
	// An empty provider/model pair (no chat key configured anywhere) disables
	// answer generation; retrieval and article results keep working.
	answerProvider = strings.TrimSpace(answerProvider)
	answerModel = strings.TrimSpace(answerModel)
	if strings.TrimSpace(embeddingModel) == "" {
		embeddingModel = defaultDocsEmbeddingModel
	}
	return &HelpcenterAISearchService{
		chunkRepo:         chunkRepo,
		searchRepo:        searchRepo,
		answerRepo:        answerRepo,
		embeddingProvider: embeddingProvider,
		embeddingModel:    embeddingModel,
		llmProvider:       llmProvider,
		answerProvider:    answerProvider,
		answerModel:       answerModel,
		redis:             redisClient,
	}
}

// SemanticSearch retrieves public chunks for a query and groups them into
// ranked article results with snippet matches. Falls back to lexical-only
// retrieval when the embedding provider is unavailable.
func (s *HelpcenterAISearchService) SemanticSearch(ctx context.Context, workspaceID, locale, fallbackLocale, query, spaceSlug string, limit int) ([]model.PublicSearchResultResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []model.PublicSearchResultResponse{}, nil
	}
	if limit <= 0 || limit > 20 {
		limit = helpcenterSemanticArticleLimit
	}
	chunks, err := s.retrievePublicChunks(ctx, workspaceID, query, helpcenterSemanticChunkLimit)
	if err != nil {
		return nil, err
	}
	grouped := groupChunksByDocument(chunks)
	if len(grouped) == 0 {
		return []model.PublicSearchResultResponse{}, nil
	}
	refs, err := s.resolveArticleRefs(ctx, workspaceID, locale, fallbackLocale, documentIDsOf(grouped))
	if err != nil {
		return nil, err
	}

	results := make([]model.PublicSearchResultResponse, 0, limit)
	for _, group := range grouped {
		ref, ok := refs[group.documentID]
		if !ok {
			continue
		}
		if spaceSlug != "" && ref.SpaceSlug != spaceSlug {
			continue
		}
		for _, chunk := range group.chunks[:min(len(group.chunks), helpcenterSnippetsPerArticle)] {
			sectionTitle := strings.TrimSpace(chunk.HeadingPath)
			match := model.PublicSearchMatchResponse{
				EntryType: "semantic",
				Snippet:   excerptText(chunk.Content, helpcenterSnippetChars),
			}
			if sectionTitle != "" {
				match.SectionTitle = &sectionTitle
			}
			ref.Matches = append(ref.Matches, match)
		}
		results = append(results, ref)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// AnswerOutcome is the service-level result of an ask.
type AnswerOutcome struct {
	Response      *model.HelpcenterAnswerResponse
	BudgetBlocked bool
}

// Answer produces a grounded answer for a public query, serving from cache
// when the content has not changed since the last identical ask.
func (s *HelpcenterAISearchService) Answer(ctx context.Context, workspaceID, locale, fallbackLocale, rawQuery, spaceSlug string) (*AnswerOutcome, error) {
	query := strings.TrimSpace(rawQuery)
	if len(query) < helpcenterAnswerMinQueryChars {
		return nil, fmt.Errorf("query is too short")
	}
	if len(query) > helpcenterAnswerMaxQueryChars {
		return nil, fmt.Errorf("query is too long")
	}

	fingerprint, err := s.chunkRepo.PublicContentFingerprint(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	cacheKey := helpcenterAnswerCacheKey(workspaceID, locale, spaceSlug, query, fingerprint)
	if cached, err := s.answerRepo.GetByCacheKey(ctx, workspaceID, cacheKey); err == nil && cached != nil {
		return &AnswerOutcome{Response: answerResponseFromRow(cached, true)}, nil
	}

	if s.answerProvider == "" || s.llmProvider == nil {
		// No chat provider configured: degrade to the article list without
		// spending budget or caching a durable "cannot answer" verdict.
		return &AnswerOutcome{Response: &model.HelpcenterAnswerResponse{
			Status:    model.HelpcenterAnswerStatusInsufficientEvidence,
			Citations: []model.HelpcenterAnswerCitation{},
		}}, nil
	}

	if !s.consumeAnswerBudget(ctx, workspaceID) {
		return &AnswerOutcome{BudgetBlocked: true}, nil
	}

	chunks, err := s.retrievePublicChunks(ctx, workspaceID, query, helpcenterAnswerChunkLimit)
	if err != nil {
		return nil, err
	}
	refs, err := s.resolveArticleRefs(ctx, workspaceID, locale, fallbackLocale, documentIDsOfChunks(chunks))
	if err != nil {
		return nil, err
	}
	// Only chunks that resolve to a live published article may ground answers.
	grounded := make([]repository.DocsChunkSearchResult, 0, len(chunks))
	for _, chunk := range chunks {
		if _, ok := refs[chunk.DocumentID]; ok {
			grounded = append(grounded, chunk)
		}
	}

	row := &model.HelpcenterAnswer{
		WorkspaceID: workspaceID,
		CacheKey:    cacheKey,
		Locale:      locale,
		SpaceSlug:   spaceSlug,
		Query:       query,
		Status:      model.HelpcenterAnswerStatusInsufficientEvidence,
		Citations:   json.RawMessage("[]"),
	}
	if len(grounded) == 0 {
		if err := s.answerRepo.Upsert(ctx, row); err != nil {
			return nil, err
		}
		return &AnswerOutcome{Response: answerResponseFromRow(row, false)}, nil
	}

	contract, tokensUsed, err := s.generateAnswer(ctx, query, locale, grounded)
	row.TokensUsed = tokensUsed
	if err != nil {
		slog.WarnContext(ctx, "helpcenter answer generation failed", "error", err, "workspace_id", workspaceID)
		// Degrade to article list rather than surfacing an error page. Not
		// cached as a durable verdict would be wrong — but the row records the
		// attempt for analytics; content-fingerprint keys make retries cheap.
		if upsertErr := s.answerRepo.Upsert(ctx, row); upsertErr != nil {
			return nil, upsertErr
		}
		return &AnswerOutcome{Response: answerResponseFromRow(row, false)}, nil
	}

	citations, valid := validateHelpcenterAnswer(contract, grounded, refs)
	if valid {
		row.Status = model.HelpcenterAnswerStatusAnswered
		row.Answer = strings.TrimSpace(contract.Answer)
		row.Confidence = contract.Confidence
		if encoded, err := json.Marshal(citations); err == nil {
			row.Citations = encoded
		}
	}
	if err := s.answerRepo.Upsert(ctx, row); err != nil {
		return nil, err
	}
	return &AnswerOutcome{Response: answerResponseFromRow(row, false)}, nil
}

// RecordAnswerFeedback stores one thumbs vote.
func (s *HelpcenterAISearchService) RecordAnswerFeedback(ctx context.Context, workspaceID, answerID string, isHelpful bool) error {
	answer, err := s.answerRepo.GetByID(ctx, workspaceID, answerID)
	if err != nil {
		return err
	}
	if answer == nil {
		return fmt.Errorf("answer not found")
	}
	return s.answerRepo.IncrementFeedback(ctx, workspaceID, answerID, isHelpful)
}

// --- retrieval ---

func (s *HelpcenterAISearchService) retrievePublicChunks(ctx context.Context, workspaceID, query string, limit int) ([]repository.DocsChunkSearchResult, error) {
	spaceIDs, err := s.chunkRepo.ListSearchableExternalSpaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(spaceIDs) == 0 {
		// Published articles but no chunks yet (workspace predates
		// auto-indexing, or indexing lagged): queue a lazy backfill so the
		// next search benefits. Debounced per workspace.
		s.queueAutoIndexBackfill(ctx, workspaceID)
		return nil, nil
	}
	queryEmbedding := ""
	if s.embeddingProvider != nil {
		resp, err := s.embeddingProvider.CreateEmbeddings(ctx, llm.EmbeddingRequest{
			Model:  s.embeddingModel,
			Inputs: []string{query},
		})
		if err != nil {
			slog.WarnContext(ctx, "helpcenter query embedding failed; lexical retrieval only", "error", err)
		} else if len(resp.Vectors) > 0 {
			queryEmbedding = formatVector(resp.Vectors[0])
		}
	}
	return s.chunkRepo.HybridSearchWithEmbeddingModel(ctx, workspaceID, spaceIDs, query, queryEmbedding, s.embeddingModel, limit)
}

func (s *HelpcenterAISearchService) resolveArticleRefs(ctx context.Context, workspaceID, locale, fallbackLocale string, documentIDs []string) (map[string]model.PublicSearchResultResponse, error) {
	refs := map[string]model.PublicSearchResultResponse{}
	if len(documentIDs) == 0 {
		return refs, nil
	}
	primary, err := s.searchRepo.PublicArticleRefsByDocumentIDs(ctx, workspaceID, locale, documentIDs)
	if err != nil {
		return nil, err
	}
	for _, ref := range primary {
		refs[ref.ID] = ref
	}
	if fallbackLocale != "" && fallbackLocale != locale {
		var missing []string
		for _, id := range documentIDs {
			if _, ok := refs[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			fallback, err := s.searchRepo.PublicArticleRefsByDocumentIDs(ctx, workspaceID, fallbackLocale, missing)
			if err != nil {
				return nil, err
			}
			for _, ref := range fallback {
				ref.IsFallback = true
				ref.RequestedLocale = locale
				refs[ref.ID] = ref
			}
		}
	}
	return refs, nil
}

type helpcenterChunkGroup struct {
	documentID string
	bestScore  float64
	chunks     []repository.DocsChunkSearchResult
}

func groupChunksByDocument(chunks []repository.DocsChunkSearchResult) []helpcenterChunkGroup {
	byDoc := map[string]*helpcenterChunkGroup{}
	order := []string{}
	for _, chunk := range chunks {
		group, ok := byDoc[chunk.DocumentID]
		if !ok {
			group = &helpcenterChunkGroup{documentID: chunk.DocumentID, bestScore: chunk.CombinedScore}
			byDoc[chunk.DocumentID] = group
			order = append(order, chunk.DocumentID)
		}
		if chunk.CombinedScore > group.bestScore {
			group.bestScore = chunk.CombinedScore
		}
		group.chunks = append(group.chunks, chunk)
	}
	groups := make([]helpcenterChunkGroup, 0, len(order))
	for _, id := range order {
		groups = append(groups, *byDoc[id])
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].bestScore > groups[j].bestScore })
	return groups
}

func documentIDsOf(groups []helpcenterChunkGroup) []string {
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.documentID)
	}
	return ids
}

func documentIDsOfChunks(chunks []repository.DocsChunkSearchResult) []string {
	seen := map[string]bool{}
	var ids []string
	for _, chunk := range chunks {
		if !seen[chunk.DocumentID] {
			seen[chunk.DocumentID] = true
			ids = append(ids, chunk.DocumentID)
		}
	}
	return ids
}

// --- answer generation + validation ---

// helpcenterAnswerContract is the JSON contract the model must return.
type helpcenterAnswerContract struct {
	CanAnswer      bool     `json:"can_answer"`
	Answer         string   `json:"answer"`
	CitedChunkIDs  []string `json:"cited_chunk_ids"`
	Confidence     float64  `json:"confidence"`
	DeclinedReason string   `json:"declined_reason"`
}

func (s *HelpcenterAISearchService) generateAnswer(ctx context.Context, query, locale string, chunks []repository.DocsChunkSearchResult) (*helpcenterAnswerContract, int, error) {
	if s.llmProvider == nil {
		return nil, 0, fmt.Errorf("helpcenter answer LLM is not configured")
	}
	var b strings.Builder
	for _, chunk := range chunks {
		b.WriteString("<chunk id=\"")
		b.WriteString(chunk.ID)
		b.WriteString("\" article=\"")
		b.WriteString(strings.ReplaceAll(chunk.Title, `"`, "'"))
		b.WriteString("\">\n")
		b.WriteString(excerptText(chunk.Content, helpcenterAnswerContextChars))
		b.WriteString("\n</chunk>\n")
	}

	systemPrompt := `You answer help-center questions for visitors using ONLY the provided documentation chunks.

Rules:
- Use only facts stated in the chunks. Exact values (prices, limits, plan names) must appear verbatim in a chunk.
- Do not include any URLs or links in the answer text.
- Answer in the language of the question. Be concise: a short paragraph, or brief steps for procedures.
- cited_chunk_ids must list the chunk ids whose content you used.
- If the chunks do not confidently answer the question, set can_answer=false and leave the answer empty. Never guess.
- confidence is your honest 0-1 estimate that the answer is correct and fully grounded.

Return JSON only.`

	generationCtx, cancel := context.WithTimeout(ctx, helpcenterAnswerTimeout)
	defer cancel()
	resp, err := s.llmProvider.ChatCompletion(generationCtx, llm.ChatRequest{
		Provider:     s.answerProvider,
		Model:        s.answerModel,
		SystemPrompt: systemPrompt,
		Messages: []llm.Message{{
			Role:    "user",
			Content: "Question (locale " + locale + "): " + query + "\n\nDocumentation chunks:\n" + b.String(),
		}},
		JSONMode: true,
		JSONSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"can_answer", "answer", "cited_chunk_ids", "confidence"},
			"properties": map[string]any{
				"can_answer":      map[string]any{"type": "boolean"},
				"answer":          map[string]any{"type": "string"},
				"cited_chunk_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"confidence":      map[string]any{"type": "number"},
				"declined_reason": map[string]any{"type": "string"},
			},
		},
	})
	if err != nil {
		return nil, 0, err
	}
	tokens := resp.TokensUsed.InputTokens + resp.TokensUsed.OutputTokens
	var contract helpcenterAnswerContract
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &contract); err != nil {
		return nil, tokens, fmt.Errorf("parse answer contract: %w", err)
	}
	return &contract, tokens, nil
}

var helpcenterLinkPattern = regexp.MustCompile(`https?://\S+|\[[^\]]*\]\([^)]*\)`)

// validateHelpcenterAnswer enforces the grounding contract server-side: the
// model must claim it can answer with adequate confidence, every cited chunk
// must be one we actually retrieved, and the text may not smuggle links.
func validateHelpcenterAnswer(contract *helpcenterAnswerContract, chunks []repository.DocsChunkSearchResult, refs map[string]model.PublicSearchResultResponse) ([]model.HelpcenterAnswerCitation, bool) {
	if contract == nil || !contract.CanAnswer || strings.TrimSpace(contract.Answer) == "" {
		return nil, false
	}
	if contract.Confidence < helpcenterAnswerConfidenceFloor {
		return nil, false
	}
	if len(contract.CitedChunkIDs) == 0 {
		return nil, false
	}
	if helpcenterLinkPattern.MatchString(contract.Answer) {
		return nil, false
	}
	chunksByID := map[string]repository.DocsChunkSearchResult{}
	for _, chunk := range chunks {
		chunksByID[chunk.ID] = chunk
	}
	seenDocs := map[string]bool{}
	var citations []model.HelpcenterAnswerCitation
	for _, id := range contract.CitedChunkIDs {
		chunk, ok := chunksByID[strings.TrimSpace(id)]
		if !ok {
			return nil, false
		}
		ref, ok := refs[chunk.DocumentID]
		if !ok {
			return nil, false
		}
		if seenDocs[chunk.DocumentID] {
			continue
		}
		seenDocs[chunk.DocumentID] = true
		citations = append(citations, model.HelpcenterAnswerCitation{
			DocumentID:     ref.ID,
			Title:          ref.Title,
			Slug:           ref.Slug,
			PublicID:       ref.PublicID,
			SpaceSlug:      ref.SpaceSlug,
			CollectionSlug: ref.CollectionSlug,
			Snippet:        excerptText(chunk.Content, helpcenterSnippetChars),
		})
	}
	return citations, true
}

// --- cache + budget helpers ---

func helpcenterAnswerCacheKey(workspaceID, locale, spaceSlug, query, fingerprint string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(query)), " ")
	sum := sha256.Sum256([]byte(workspaceID + "|" + locale + "|" + spaceSlug + "|" + normalized + "|" + fingerprint))
	return hex.EncodeToString(sum[:])
}

// queueAutoIndexBackfill triggers the help-center chunk backfill at most once
// per workspace per day (best-effort; skipped entirely without Redis so a
// missing debounce cannot cause repeated embedding work).
func (s *HelpcenterAISearchService) queueAutoIndexBackfill(ctx context.Context, workspaceID string) {
	if s.autoIndexer == nil || s.redis == nil {
		return
	}
	key := "hc:ai:backfill:" + workspaceID
	set, err := s.redis.SetNX(ctx, key, "1", 24*time.Hour).Result()
	if err != nil || !set {
		return
	}
	if err := s.autoIndexer.QueueHelpcenterAutoIndex(ctx, workspaceID); err != nil {
		slog.WarnContext(ctx, "helpcenter auto-index backfill failed", "workspace_id", workspaceID, "error", err)
		s.redis.Del(ctx, key)
	}
}

// consumeAnswerBudget increments the workspace's daily generation counter.
// Fails open without Redis.
func (s *HelpcenterAISearchService) consumeAnswerBudget(ctx context.Context, workspaceID string) bool {
	if s.redis == nil {
		return true
	}
	key := "hc:ai:daily:" + workspaceID + ":" + time.Now().UTC().Format("20060102")
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		slog.WarnContext(ctx, "helpcenter answer budget check failed", "workspace_id", workspaceID, "error", err)
		return true
	}
	if count == 1 {
		s.redis.Expire(ctx, key, 48*time.Hour)
	}
	return count <= helpcenterAnswerDailyLimit
}

func answerResponseFromRow(row *model.HelpcenterAnswer, cached bool) *model.HelpcenterAnswerResponse {
	response := &model.HelpcenterAnswerResponse{
		AnswerID:   row.ID,
		Status:     row.Status,
		Answer:     row.Answer,
		Confidence: row.Confidence,
		Cached:     cached,
		Citations:  []model.HelpcenterAnswerCitation{},
	}
	_ = json.Unmarshal(row.Citations, &response.Citations)
	return response
}
