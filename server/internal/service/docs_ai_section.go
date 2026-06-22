package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type DocsAISectionService struct {
	candidateRepo    *repository.DocsAISectionCandidateRepository
	blockRepo        *repository.DocsBlockRepository
	blockSvc         *DocsBlockService
	docRepo          *repository.DocsDocumentRepository
	searchSvc        *DocsSearchService
	conversationRepo *repository.SupportConversationRepository
	agentSvc         *AgentService
	llmProvider      llm.Provider
	ruleEngine       *AutomationRuleEngine
	activity         *PMActivityService
	fetchClients     []*http.Client
	nextFetchClient  atomic.Uint64
}

func NewDocsAISectionService(candidateRepo *repository.DocsAISectionCandidateRepository, blockRepo *repository.DocsBlockRepository, blockSvc *DocsBlockService, docRepo *repository.DocsDocumentRepository, searchSvc *DocsSearchService, conversationRepo *repository.SupportConversationRepository, agentSvc *AgentService, llmProvider llm.Provider, crawlerProxyURLs ...string) *DocsAISectionService {
	proxyURLs := ""
	if len(crawlerProxyURLs) > 0 {
		proxyURLs = crawlerProxyURLs[0]
	}
	return &DocsAISectionService{
		candidateRepo:    candidateRepo,
		blockRepo:        blockRepo,
		blockSvc:         blockSvc,
		docRepo:          docRepo,
		searchSvc:        searchSvc,
		conversationRepo: conversationRepo,
		agentSvc:         agentSvc,
		llmProvider:      llmProvider,
		fetchClients:     newAISectionFetchClients(proxyURLs),
	}
}

func (s *DocsAISectionService) SetRuleEngine(engine *AutomationRuleEngine) {
	s.ruleEngine = engine
}

func (s *DocsAISectionService) SetActivityService(activity *PMActivityService) {
	s.activity = activity
}

func (s *DocsAISectionService) LatestCandidate(ctx context.Context, workspaceID, documentID, blockID string) (*model.DocsAISectionCandidate, error) {
	if _, _, err := s.loadAISectionBlockForRead(ctx, workspaceID, documentID, blockID); err != nil {
		return nil, err
	}
	return s.candidateRepo.LatestOpen(ctx, documentID, blockID)
}

func (s *DocsAISectionService) Regenerate(ctx context.Context, workspaceID, documentID, blockID, actorID string, req model.RegenerateAISectionRequest) (*model.AISectionCandidateResponse, error) {
	doc, block, err := s.loadAISectionBlock(ctx, workspaceID, documentID, blockID)
	if err != nil {
		return nil, err
	}
	instructions, err := validateAISectionInstructions(req.Instructions)
	if err != nil {
		return nil, err
	}
	agent, err := s.loadAISectionAgent(ctx, workspaceID, strings.TrimSpace(req.AgentID))
	if err != nil {
		return nil, err
	}
	runContext := buildAISectionRunContext(doc, block, instructions)
	run, err := s.agentSvc.StartTargetRun(ctx, workspaceID, "document", documentID, model.StartAgentRunRequest{
		AgentID:           strings.TrimSpace(req.AgentID),
		AdditionalContext: strPtr(runContext),
		AllowedTools:      aiSectionRunToolsForAgent(agent),
		Output: &model.AgentRunOutputContext{
			Type:           "docs_ai_section_candidate",
			IdempotencyKey: blockID,
		},
	}, actorID)
	if err != nil {
		return nil, err
	}
	if existing, err := s.candidateRepo.LatestOpen(ctx, documentID, blockID); err != nil {
		return nil, err
	} else if existing != nil {
		if err := s.candidateRepo.UpdateStatus(ctx, existing.ID, model.DocsAISectionCandidateStatusRejected, actorID); err != nil {
			return nil, err
		}
	}

	s.logAISectionActivity(ctx, doc, block, actorID, "ai_section_regenerated", map[string]interface{}{
		"block_id":     blockID,
		"agent_run_id": run.ID,
	})
	s.emitAISectionEvent(ctx, model.TriggerAISectionRegenerated, doc, blockID, run.ID)
	return &model.AISectionCandidateResponse{Candidate: nil, AgentRun: run}, nil
}

func (s *DocsAISectionService) loadAISectionAgent(ctx context.Context, workspaceID, agentID string) (*model.Agent, error) {
	if s.agentSvc == nil || s.agentSvc.agentRepo == nil {
		return nil, fmt.Errorf("agent service is not configured")
	}
	if strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	agent, err := s.agentSvc.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	return agent, nil
}

func validateAISectionInstructions(raw *string) (string, error) {
	instructions := strings.TrimSpace(derefString(raw))
	if instructions == "" {
		return "", fmt.Errorf("regeneration instructions are required")
	}
	return instructions, nil
}

func aiSectionRunToolsForAgent(agent *model.Agent) []string {
	if agent == nil {
		return nil
	}
	agentTools := parseJSONStringSlice(agent.AllowedTools)
	if len(agentTools) == 0 {
		return nil
	}
	defaultTools := []string{
		"list_documents",
		"list_collections",
		"read_document",
		"get_document_blocks",
		"search_documents",
		"web_search_exa",
		"web_search_brave",
		"fetch_url",
		"crawl_url",
		"publish_ai_section_candidate",
		"list_deals",
		"list_contacts",
		"list_buyer_signals",
	}
	allowedSet := make(map[string]bool, len(agentTools))
	for _, tool := range agentTools {
		allowedSet[strings.TrimSpace(tool)] = true
	}
	out := make([]string, 0, len(defaultTools))
	seen := map[string]bool{}
	for _, tool := range defaultTools {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		if tool != "publish_ai_section_candidate" && !allowedSet[tool] {
			continue
		}
		seen[tool] = true
		out = append(out, tool)
	}
	return out
}

func (s *DocsAISectionService) Approve(ctx context.Context, workspaceID, documentID, blockID, actorID string) (*model.AISectionCandidateResponse, error) {
	doc, block, err := s.loadAISectionBlock(ctx, workspaceID, documentID, blockID)
	if err != nil {
		return nil, err
	}
	candidate, err := s.candidateRepo.LatestOpen(ctx, documentID, blockID)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, fmt.Errorf("candidate not found")
	}
	if !aiSectionContentEqualForApproval(candidate.CurrentContent, block.Content) {
		return nil, ErrDocsStaleBlockRevision
	}
	approvedContent, err := aiSectionCandidateContentWithStatusForCurrent(candidate.CandidateContent, block.Content, model.DocsAISectionCandidateStatusApproved)
	if err != nil {
		return nil, err
	}
	content, err := s.blockSvc.Patch(ctx, documentID, blockID, block.Revision, approvedContent, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.candidateRepo.UpdateStatus(ctx, candidate.ID, model.DocsAISectionCandidateStatusApproved, actorID); err != nil {
		return nil, err
	}
	candidate.Status = model.DocsAISectionCandidateStatusApproved
	candidate.ApprovedBy = &actorID
	candidate.CandidateContent = approvedContent
	s.logAISectionActivity(ctx, doc, block, actorID, "ai_section_approved", map[string]interface{}{
		"block_id":     blockID,
		"candidate_id": candidate.ID,
		"agent_run_id": derefString(candidate.AgentRunID),
	})
	s.emitAISectionEvent(ctx, model.TriggerAISectionApproved, doc, blockID, derefString(candidate.AgentRunID))
	return &model.AISectionCandidateResponse{Candidate: candidate, Content: content}, nil
}

func (s *DocsAISectionService) Reject(ctx context.Context, workspaceID, documentID, blockID, actorID string) (*model.AISectionCandidateResponse, error) {
	doc, block, err := s.loadAISectionBlock(ctx, workspaceID, documentID, blockID)
	if err != nil {
		return nil, err
	}
	candidate, err := s.candidateRepo.LatestOpen(ctx, documentID, blockID)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, fmt.Errorf("candidate not found")
	}
	if err := s.candidateRepo.UpdateStatus(ctx, candidate.ID, model.DocsAISectionCandidateStatusRejected, actorID); err != nil {
		return nil, err
	}
	candidate.Status = model.DocsAISectionCandidateStatusRejected
	s.logAISectionActivity(ctx, doc, block, actorID, "ai_section_rejected", map[string]interface{}{
		"block_id":     blockID,
		"candidate_id": candidate.ID,
		"agent_run_id": derefString(candidate.AgentRunID),
	})
	return &model.AISectionCandidateResponse{Candidate: candidate}, nil
}

func (s *DocsAISectionService) loadAISectionBlock(ctx context.Context, workspaceID, documentID, blockID string) (*model.DocsDocument, *model.DocsBlock, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, nil, fmt.Errorf("document not found")
	}
	if err := checkLocked(doc); err != nil {
		return nil, nil, err
	}
	block, err := s.blockRepo.GetByID(ctx, blockID)
	if err != nil {
		return nil, nil, err
	}
	if block == nil || block.DocumentID != documentID || block.WorkspaceID != workspaceID || block.DeletedAt != nil {
		return nil, nil, fmt.Errorf("block not found")
	}
	if block.Type != "aiSection" {
		return nil, nil, fmt.Errorf("block is not an AI section")
	}
	return doc, block, nil
}

func (s *DocsAISectionService) loadAISectionBlockForRead(ctx context.Context, workspaceID, documentID, blockID string) (*model.DocsDocument, *model.DocsBlock, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, nil, fmt.Errorf("document not found")
	}
	block, err := s.blockRepo.GetByID(ctx, blockID)
	if err != nil {
		return nil, nil, err
	}
	if block == nil || block.DocumentID != documentID || block.WorkspaceID != workspaceID || block.DeletedAt != nil {
		return nil, nil, fmt.Errorf("block not found")
	}
	if block.Type != "aiSection" {
		return nil, nil, fmt.Errorf("block is not an AI section")
	}
	return doc, block, nil
}

func jsonRawEqual(a, b json.RawMessage) bool {
	var ca bytes.Buffer
	if err := json.Compact(&ca, a); err != nil {
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	var cb bytes.Buffer
	if err := json.Compact(&cb, b); err != nil {
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

func aiSectionContentEqualForApproval(candidateSnapshot, current json.RawMessage) bool {
	if jsonRawEqual(candidateSnapshot, current) {
		return true
	}
	return jsonRawEqual(normalizeAISectionForApproval(candidateSnapshot), normalizeAISectionForApproval(current))
}

func normalizeAISectionForApproval(raw json.RawMessage) json.RawMessage {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return bytes.TrimSpace(raw)
	}
	attrs, _ := node["attrs"].(map[string]any)
	for _, key := range []string{
		"title",
		"status",
		"ownerAgentId",
		"ownerAgentName",
		"lastGeneratedAt",
		"model",
		"promptHash",
		"sourceCount",
	} {
		delete(attrs, key)
	}
	normalized, err := json.Marshal(node)
	if err != nil {
		return bytes.TrimSpace(raw)
	}
	return normalized
}

func aiSectionCandidateContentWithStatus(raw json.RawMessage, candidateStatus string) (json.RawMessage, error) {
	return aiSectionCandidateContentWithStatusForCurrent(raw, nil, candidateStatus)
}

func aiSectionCandidateContentWithStatusForCurrent(raw, current json.RawMessage, candidateStatus string) (json.RawMessage, error) {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("parse AI section candidate: %w", err)
	}
	attrs, _ := node["attrs"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
		node["attrs"] = attrs
	}
	if len(current) > 0 {
		var currentNode map[string]any
		if err := json.Unmarshal(current, &currentNode); err == nil {
			if currentAttrs, _ := currentNode["attrs"].(map[string]any); currentAttrs != nil {
				for _, key := range []string{"blockId", "title"} {
					if value, ok := currentAttrs[key]; ok {
						attrs[key] = value
					}
				}
			}
		}
	}
	switch candidateStatus {
	case model.DocsAISectionCandidateStatusApproved:
		attrs["status"] = "approved"
	case model.DocsAISectionCandidateStatusRejected:
		attrs["status"] = "draft"
	default:
		attrs["status"] = "needs_review"
	}
	encoded, err := json.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("marshal AI section candidate: %w", err)
	}
	return encoded, nil
}

func (s *DocsAISectionService) generateCandidate(ctx context.Context, block *model.DocsBlock, prompt string, run *model.AgentRun, agent *model.Agent, sources []map[string]any) (json.RawMessage, string, error) {
	runID := ""
	if run != nil {
		runID = run.ID
	}
	resp, err := s.llmProvider.ChatCompletion(WithAIUsageMetering(ctx, AIUsageMeteringContext{
		WorkspaceID:    block.WorkspaceID,
		FeatureKey:     BillingFeatureDocsAISectionGeneration,
		IdempotencyKey: aiUsageIdempotencyKey(block.WorkspaceID, BillingFeatureDocsAISectionGeneration, block.DocumentID, block.ID, runID),
		Metadata: map[string]interface{}{
			"document_id":  block.DocumentID,
			"block_id":     block.ID,
			"agent_run_id": runID,
		},
	}), llm.ChatRequest{
		Provider: normalizeModelProvider(derefString(agent.Provider)),
		Model:    strings.TrimSpace(derefString(agent.Model)),
		SystemPrompt: `You are the selected Helpin documentation agent. Regenerate one AI-owned documentation section.
Return only markdown for the replacement section body. Do not include markdown fences, commentary, document title, or provenance notes.`,
		Messages: []llm.Message{{
			Role:    "user",
			Content: prompt,
		}},
		Temperature: 0.2,
		MaxTokens:   1400,
	})
	if err != nil {
		return nil, "", fmt.Errorf("generate AI section candidate: %w", err)
	}

	body := strings.TrimSpace(resp.Content)
	if body == "" {
		return nil, "", fmt.Errorf("generated AI section candidate was empty")
	}
	node, err := aiSectionNodeWithGeneratedBody(block.Content, body, run, len(sources))
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(node)
	if err != nil {
		return nil, "", fmt.Errorf("marshal AI section candidate: %w", err)
	}
	return raw, tiptap.StripHTML(body), nil
}

func buildAISectionCandidatePrompt(doc *model.DocsDocument, block *model.DocsBlock, instructions string, sources []map[string]any) string {
	return fmt.Sprintf("Document title: %s\nSection title: %s\nCurrent section text:\n%s\n\nRelevant sources:\n%s\n\nInstructions:\n%s\n\nRegeneration constraints:\n- Replace only this AI section body.\n- Follow the user instructions over the document title.\n- Treat the document title as surrounding context, not as the requested topic.\n- If the instructions include a URL, base the replacement on fetched URL facts and cite only facts supported by the source context.\n- Preserve the section topic from the instructions and current section; do not expand into an unrelated document-wide guide.",
		doc.Title,
		aiSectionTitle(block.Content),
		block.ContentText,
		formatAISectionSourceContext(sources),
		strings.TrimSpace(instructions),
	)
}

func hashAISectionPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(prompt)))
	return hex.EncodeToString(sum[:])
}

func aiSectionAgentModel(agent *model.Agent) string {
	if agent == nil {
		return ""
	}
	modelName := strings.TrimSpace(derefString(agent.Model))
	provider := strings.TrimSpace(derefString(agent.Provider))
	if provider == "" {
		return modelName
	}
	if modelName == "" {
		return provider
	}
	return provider + "/" + modelName
}

func aiSectionNodeWithGeneratedBody(current json.RawMessage, markdown string, run *model.AgentRun, sourceCount int) (map[string]any, error) {
	var node map[string]any
	if err := json.Unmarshal(current, &node); err != nil {
		return nil, fmt.Errorf("parse AI section block: %w", err)
	}
	attrs, _ := node["attrs"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
		node["attrs"] = attrs
	}
	attrs["status"] = "needs_review"
	attrs["lastGeneratedAt"] = time.Now().UTC().Format(time.RFC3339)
	attrs["sourceCount"] = sourceCount
	if run != nil {
		attrs["ownerAgentId"] = run.AgentID
	}

	var generated struct {
		Type    string           `json:"type"`
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(tiptap.MarkdownToJSON(markdown), &generated); err != nil {
		return nil, fmt.Errorf("parse generated markdown: %w", err)
	}
	if len(generated.Content) == 0 {
		generated.Content = []map[string]any{{"type": "paragraph"}}
	}
	node["content"] = generated.Content
	return node, nil
}

func (s *DocsAISectionService) collectSources(ctx context.Context, workspaceID, currentDocumentID string, block *model.DocsBlock) []map[string]any {
	query := strings.TrimSpace(block.ContentText)
	if len(query) > 180 {
		query = query[:180]
	}
	if query == "" {
		return nil
	}

	sources := make([]map[string]any, 0, 5)
	if s.searchSvc != nil {
		if docs, err := s.searchSvc.Search(ctx, workspaceID, query, nil, nil, 4); err == nil {
			for _, hit := range docs {
				if hit.ID == currentDocumentID {
					continue
				}
				sources = append(sources, map[string]any{
					"sourceType": "docs_chunk",
					"sourceId":   hit.ID,
					"documentId": hit.ID,
					"title":      hit.Title,
					"excerpt":    truncate(derefString(hit.Excerpt), 240),
					"confidence": hit.Rank,
					"access":     "granted",
				})
				if len(sources) >= 3 {
					break
				}
			}
		}
	}
	if s.conversationRepo != nil {
		conversations, _, err := s.conversationRepo.List(ctx, repository.ConversationRepositoryListParams{
			ConversationListParams: repository.ConversationListParams{
				WorkspaceID: workspaceID,
				Pagination:  model.PMPagination{Page: 1, PerPage: 2},
				Search:      query,
			},
			Role: model.RoleOwner,
		})
		if err == nil {
			for _, conv := range conversations {
				sources = append(sources, map[string]any{
					"sourceType":     "support_conversation",
					"sourceId":       conv.ID,
					"conversationId": conv.ID,
					"title":          "Restricted support conversation",
					"excerpt":        "",
					"access":         "redacted",
					"redacted":       true,
				})
			}
		}
	}
	return sources
}

var aiSectionURLPattern = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"')\]]+`)

func (s *DocsAISectionService) collectAISectionInstructionURLSources(ctx context.Context, instructions string) []map[string]any {
	return collectAISectionInstructionURLSourcesWithClient(ctx, instructions, s.nextAISectionFetchClient())
}

func collectAISectionInstructionURLSources(ctx context.Context, instructions string) []map[string]any {
	return collectAISectionInstructionURLSourcesWithClient(ctx, instructions, nil)
}

func collectAISectionInstructionURLSourcesWithClient(ctx context.Context, instructions string, client *http.Client) []map[string]any {
	urls := extractAISectionInstructionURLs(instructions)
	if len(urls) == 0 {
		return nil
	}
	sources := make([]map[string]any, 0, len(urls))
	for _, rawURL := range urls {
		title, excerpt, finalURL, err := fetchAISectionURLSourceWithClient(ctx, rawURL, instructions, client)
		source := map[string]any{
			"sourceType": "web_page",
			"sourceId":   rawURL,
			"url":        rawURL,
			"title":      firstNonEmptyAISection(title, rawURL),
			"excerpt":    excerpt,
			"access":     "granted",
		}
		if finalURL != "" && finalURL != rawURL {
			source["finalUrl"] = finalURL
		}
		if err != nil {
			source["access"] = "unavailable"
			source["excerpt"] = "Unable to fetch this URL automatically: " + truncate(err.Error(), 180)
		}
		sources = append(sources, source)
	}
	return sources
}

func (s *DocsAISectionService) nextAISectionFetchClient() *http.Client {
	if s == nil || len(s.fetchClients) == 0 {
		return nil
	}
	idx := int(s.nextFetchClient.Add(1)-1) % len(s.fetchClients)
	return s.fetchClients[idx]
}

func extractAISectionInstructionURLs(instructions string) []string {
	matches := aiSectionURLPattern.FindAllString(strings.TrimSpace(instructions), 4)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		cleaned := strings.TrimRight(match, ".,;:")
		if cleaned == "" || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		out = append(out, cleaned)
		if len(out) >= 2 {
			break
		}
	}
	return out
}

func fetchAISectionURLSource(ctx context.Context, rawURL string) (title, excerpt, finalURL string, err error) {
	return fetchAISectionURLSourceWithClient(ctx, rawURL, "", nil)
}

func fetchAISectionURLSourceWithClient(ctx context.Context, rawURL, instructions string, client *http.Client) (title, excerpt, finalURL string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", "", fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", "", "", fmt.Errorf("unsupported URL scheme")
	}
	if err := validateAISectionFetchHost(parsed.Hostname()); err != nil {
		return "", "", "", err
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", "", "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "HelpinDocsAISection/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,text/plain;q=0.7,*/*;q=0.1")
	if client == nil {
		client = newAISectionFetchHTTPClient(nil)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("fetch URL returned HTTP %d", resp.StatusCode)
	}
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1_000_000))
	if err != nil {
		return "", "", finalURL, fmt.Errorf("read URL: %w", err)
	}
	raw := string(body)
	title = extractAISectionHTMLTitle(raw)
	text := normalizeAISectionFetchedText(tiptap.StripHTML(raw))
	return title, excerptAISectionFetchedText(text, instructions, rawURL, 3600), finalURL, nil
}

func newAISectionFetchClients(proxyURLs string) []*http.Client {
	parsedProxyURLs := crawler.ParseProxyURLs(proxyURLs)
	clients := make([]*http.Client, 0, max(1, len(parsedProxyURLs)))
	for _, rawProxyURL := range parsedProxyURLs {
		proxyURL, err := url.Parse(rawProxyURL)
		if err != nil {
			slog.Warn("docs ai section proxy ignored", "proxy_url", rawProxyURL, "error", err)
			continue
		}
		clients = append(clients, newAISectionFetchHTTPClient(proxyURL))
	}
	if len(clients) == 0 {
		clients = append(clients, newAISectionFetchHTTPClient(nil))
	}
	return clients
}

func newAISectionFetchHTTPClient(proxyURL *url.URL) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{
		Timeout:   8 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req == nil || req.URL == nil {
				return fmt.Errorf("invalid redirect URL")
			}
			return validateAISectionFetchHost(req.URL.Hostname())
		},
	}
}

func validateAISectionFetchHost(host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("missing URL host")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("private or local URL hosts are not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("private or local URL hosts are not allowed")
		}
	}
	return nil
}

func extractAISectionHTMLTitle(raw string) string {
	lower := strings.ToLower(raw)
	start := strings.Index(lower, "<title")
	if start == -1 {
		return ""
	}
	openEnd := strings.Index(lower[start:], ">")
	if openEnd == -1 {
		return ""
	}
	start += openEnd
	end := strings.Index(lower[start+1:], "</title>")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(tiptap.StripHTML(raw[start+1 : start+1+end]))
}

func normalizeAISectionFetchedText(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

var aiSectionMoneyPattern = regexp.MustCompile(`(?:[$€£]\s?\d[\d,.]*|\d[\d,.]*\s?(?:/ ?mo|/ ?month|per month|monthly|%))`)

func excerptAISectionFetchedText(text, instructions, rawURL string, maxChars int) string {
	text = strings.TrimSpace(text)
	if text == "" || maxChars <= 0 {
		return ""
	}
	if len(text) <= maxChars {
		return text
	}
	keywords := aiSectionExcerptKeywords(instructions, rawURL)
	if len(keywords) == 0 {
		return truncate(text, maxChars)
	}

	lower := strings.ToLower(text)
	type window struct {
		start int
		end   int
		score int
	}
	windows := make([]window, 0, len(keywords)+8)
	addWindow := func(idx, score int) {
		if idx < 0 {
			return
		}
		start := max(0, idx-420)
		end := min(len(text), idx+780)
		windows = append(windows, window{start: start, end: end, score: score})
	}
	for _, keyword := range keywords {
		searchFrom := 0
		for matches := 0; matches < 4; matches++ {
			idx := strings.Index(lower[searchFrom:], keyword)
			if idx < 0 {
				break
			}
			idx += searchFrom
			addWindow(idx, 10+len(keyword))
			searchFrom = idx + len(keyword)
			if searchFrom >= len(lower) {
				break
			}
		}
	}
	if aiSectionLooksLikePricingRequest(instructions, rawURL) {
		for _, loc := range aiSectionMoneyPattern.FindAllStringIndex(text, 12) {
			addWindow(loc[0], 25)
		}
	}
	if len(windows) == 0 {
		return truncate(text, maxChars)
	}
	for i := range windows {
		segment := strings.ToLower(text[windows[i].start:windows[i].end])
		for _, keyword := range keywords {
			windows[i].score += strings.Count(segment, keyword) * 3
		}
		windows[i].score += len(aiSectionMoneyPattern.FindAllString(segment, -1)) * 8
	}
	slices.SortFunc(windows, func(a, b window) int {
		if a.score != b.score {
			return b.score - a.score
		}
		return a.start - b.start
	})
	selected := make([]window, 0, 4)
	usedChars := 0
	for _, candidate := range windows {
		overlaps := false
		for _, existing := range selected {
			if candidate.start <= existing.end && candidate.end >= existing.start {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}
		length := candidate.end - candidate.start
		if len(selected) > 0 && usedChars+length > maxChars {
			continue
		}
		selected = append(selected, candidate)
		usedChars += length
		if usedChars >= maxChars || len(selected) >= 4 {
			break
		}
	}
	if len(selected) == 0 {
		return truncate(text, maxChars)
	}
	slices.SortFunc(selected, func(a, b window) int {
		return a.start - b.start
	})
	parts := make([]string, 0, len(selected))
	for _, item := range selected {
		parts = append(parts, strings.TrimSpace(text[item.start:item.end]))
	}
	return truncate(strings.Join(parts, " ... "), maxChars)
}

func aiSectionExcerptKeywords(instructions, rawURL string) []string {
	combined := strings.ToLower(strings.TrimSpace(instructions) + " " + strings.TrimSpace(rawURL))
	parts := regexp.MustCompile(`[a-z0-9]+`).FindAllString(combined, 32)
	stop := map[string]bool{
		"https": true, "http": true, "www": true, "com": true, "from": true, "this": true,
		"that": true, "with": true, "into": true, "update": true, "section": true, "fetch": true,
		"info": true, "infor": true, "information": true, "about": true, "page": true, "url": true,
	}
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		if len(part) < 4 || stop[part] || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	if aiSectionLooksLikePricingRequest(instructions, rawURL) {
		for _, keyword := range []string{"pricing", "price", "plans", "monthly", "yearly", "enterprise"} {
			if !seen[keyword] {
				out = append(out, keyword)
				seen[keyword] = true
			}
		}
	}
	return out
}

func aiSectionLooksLikePricingRequest(instructions, rawURL string) bool {
	lower := strings.ToLower(instructions + " " + rawURL)
	return strings.Contains(lower, "pricing") || strings.Contains(lower, "price") || strings.Contains(lower, "plans")
}

func redactAISectionSourcesForStorage(sources []map[string]any) []map[string]any {
	if len(sources) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(sources))
	for _, source := range sources {
		next := make(map[string]any, len(source))
		for key, value := range source {
			next[key] = value
		}
		if sourceType, _ := next["sourceType"].(string); sourceType == "support_conversation" {
			next["title"] = firstNonEmptyAISection(asStringAISection(next["title"]), "Restricted support conversation")
			next["excerpt"] = ""
			next["access"] = "redacted"
			next["redacted"] = true
		}
		out = append(out, next)
	}
	return out
}

func formatAISectionSourceContext(sources []map[string]any) string {
	if len(sources) == 0 {
		return "No source snippets found."
	}
	var b strings.Builder
	for i, source := range sources {
		title, _ := source["title"].(string)
		excerpt, _ := source["excerpt"].(string)
		sourceType, _ := source["sourceType"].(string)
		if access, _ := source["access"].(string); access == "redacted" {
			excerpt = "Restricted source. Do not quote or infer private details from this source."
		}
		fmt.Fprintf(&b, "%d. [%s] %s\n%s\n", i+1, sourceType, title, excerpt)
	}
	return b.String()
}

func asStringAISection(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func aiSectionTitle(raw json.RawMessage) string {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return "AI section"
	}
	attrs, _ := node["attrs"].(map[string]any)
	title, _ := attrs["title"].(string)
	return firstNonEmptyAISection(title, "AI section")
}

func buildAISectionRunContext(doc *model.DocsDocument, block *model.DocsBlock, instructions string) string {
	return fmt.Sprintf("Regenerate AI section block %s in document %s (%q). Current revision: %d. Instructions: %s. Use available read/search/web tools to verify relevant facts. Then call publish_ai_section_candidate with document_id=%s, block_id=%s, and the replacement section body as markdown. Do not call update_document_block or write_document_content; publish_ai_section_candidate stores a review candidate and the document changes only after human approval.",
		block.ID,
		doc.ID,
		doc.Title,
		block.Revision,
		firstNonEmptyAISection(instructions, "Improve this section."),
		doc.ID,
		block.ID,
	)
}

func (s *DocsAISectionService) emitAISectionEvent(ctx context.Context, triggerType string, doc *model.DocsDocument, blockID, runID string) {
	if s.ruleEngine == nil || doc == nil {
		return
	}
	event := model.AutomationEvent{
		WorkspaceID: doc.WorkspaceID,
		TriggerType: triggerType,
		TargetType:  "ai_section",
		TargetID:    blockID,
		RunID:       runID,
	}
	if doc.TeamID != nil {
		event.TeamID = *doc.TeamID
	}
	s.ruleEngine.EvaluateEvent(ctx, event, nil)
}

func (s *DocsAISectionService) logAISectionActivity(ctx context.Context, doc *model.DocsDocument, block *model.DocsBlock, actorID, action string, metadata map[string]interface{}) {
	if s.activity == nil || doc == nil || block == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["block_type"] = block.Type
	metadata["block_title"] = aiSectionTitle(block.Content)
	if err := s.activity.Log(ctx, doc.WorkspaceID, "doc", doc.ID, optionalActor(actorID), action, nil, nil, nil, metadata); err != nil {
		// Activity should not block document editing or candidate review.
		slog.WarnContext(ctx, "failed to log ai section activity", "error", err, "document_id", doc.ID, "block_id", block.ID, "action", action)
	}
}

func firstNonEmptyAISection(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func nilIfEmptyAISection(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}
