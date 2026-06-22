package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// ErrLLMUnavailable is returned when no LLM provider is configured.
var ErrLLMUnavailable = fmt.Errorf("LLM provider is not configured")

// CoverageArticleDraft is the parsed LLM output.
type CoverageArticleDraft struct {
	Title           string
	MarkdownContent string
	EvidenceSummary string
}

// SupportCoverageDraftService handles article draft and update
// generation from gap evidence. Separated from the core coverage
// service to isolate LLM and docs dependencies.
type SupportCoverageDraftService struct {
	coverageRepo *repository.SupportCoverageRepository
	documentSvc  *DocsDocumentService
	contentSvc   *DocsContentService
	versionSvc   *DocsVersionService
	llmProvider  llm.Provider
	logger       *slog.Logger
}

// NewSupportCoverageDraftService creates a new draft service.
func NewSupportCoverageDraftService(
	coverageRepo *repository.SupportCoverageRepository,
	documentSvc *DocsDocumentService,
	contentSvc *DocsContentService,
	versionSvc *DocsVersionService,
	llmProvider llm.Provider,
) *SupportCoverageDraftService {
	return &SupportCoverageDraftService{
		coverageRepo: coverageRepo,
		documentSvc:  documentSvc,
		contentSvc:   contentSvc,
		versionSvc:   versionSvc,
		llmProvider:  llmProvider,
		logger:       slog.Default().With("service", "support_coverage_drafts"),
	}
}

// GenerateArticleDraft creates a draft article suggestion from gap evidence.
// Does NOT create the docs document — that happens on Apply.
func (s *SupportCoverageDraftService) GenerateArticleDraft(ctx context.Context, workspaceID, gapID, targetSpaceID string, targetCollectionID *string) (*model.SupportGapSuggestion, error) {
	if s.llmProvider == nil {
		return nil, ErrLLMUnavailable
	}

	detail, err := s.coverageRepo.GetGapDetail(ctx, workspaceID, gapID)
	if err != nil {
		return nil, fmt.Errorf("get gap detail: %w", err)
	}
	if detail == nil {
		return nil, fmt.Errorf("gap not found")
	}

	// Build prompt from evidence.
	draft, err := s.generateDraftFromEvidence(ctx, detail)
	if err != nil {
		return nil, fmt.Errorf("generate draft: %w", err)
	}

	// Convert Markdown to TipTap JSON for storage and preview.
	tiptapContent := tiptap.MarkdownToJSON(draft.MarkdownContent)

	suggestion := &model.SupportGapSuggestion{
		GapID:              gapID,
		WorkspaceID:        workspaceID,
		SuggestionType:     model.SupportCoverageSuggestionCreateArticle,
		Status:             model.SupportCoverageSuggestionStatusDraft,
		Title:              draft.Title,
		Content:            tiptapContent,
		EvidenceSummary:    draft.EvidenceSummary,
		TargetSpaceID:      &targetSpaceID,
		TargetCollectionID: targetCollectionID,
	}

	created, err := s.coverageRepo.CreateSuggestion(ctx, suggestion)
	if err != nil {
		return nil, fmt.Errorf("create suggestion: %w", err)
	}

	return created, nil
}

// GenerateArticleUpdate creates an update suggestion for an existing article.
func (s *SupportCoverageDraftService) GenerateArticleUpdate(ctx context.Context, workspaceID, gapID, targetDocumentID string) (*model.SupportGapSuggestion, error) {
	if s.llmProvider == nil {
		return nil, ErrLLMUnavailable
	}

	detail, err := s.coverageRepo.GetGapDetail(ctx, workspaceID, gapID)
	if err != nil {
		return nil, fmt.Errorf("get gap detail: %w", err)
	}
	if detail == nil {
		return nil, fmt.Errorf("gap not found")
	}

	// Verify the target document belongs to the same workspace.
	doc, err := s.documentSvc.Get(ctx, targetDocumentID)
	if err != nil {
		return nil, fmt.Errorf("get target document: %w", err)
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found in workspace")
	}

	// Load existing article content for context.
	existingContent, err := s.contentSvc.Get(ctx, targetDocumentID)
	if err != nil {
		return nil, fmt.Errorf("get existing content: %w", err)
	}

	draft, err := s.generateUpdateFromEvidence(ctx, detail, existingContent)
	if err != nil {
		return nil, fmt.Errorf("generate update: %w", err)
	}

	tiptapContent := tiptap.MarkdownToJSON(draft.MarkdownContent)

	suggestion := &model.SupportGapSuggestion{
		GapID:            gapID,
		WorkspaceID:      workspaceID,
		SuggestionType:   model.SupportCoverageSuggestionUpdateArticle,
		Status:           model.SupportCoverageSuggestionStatusDraft,
		Title:            draft.Title,
		Content:          tiptapContent,
		EvidenceSummary:  draft.EvidenceSummary,
		TargetDocumentID: &targetDocumentID,
	}

	created, err := s.coverageRepo.CreateSuggestion(ctx, suggestion)
	if err != nil {
		return nil, fmt.Errorf("create suggestion: %w", err)
	}

	return created, nil
}

// ApplySuggestion creates the actual docs document/content from a suggestion.
func (s *SupportCoverageDraftService) ApplySuggestion(ctx context.Context, workspaceID, suggestionID, userID string) error {
	return s.applyImpl(ctx, workspaceID, suggestionID, userID, "", "")
}

func (s *SupportCoverageDraftService) ApplySuggestionWithOverride(ctx context.Context, workspaceID, suggestionID, userID, overrideType, overrideTargetDocID string) error {
	return s.applyImpl(ctx, workspaceID, suggestionID, userID, overrideType, overrideTargetDocID)
}

func (s *SupportCoverageDraftService) applyImpl(ctx context.Context, workspaceID, suggestionID, userID, overrideType, overrideTargetDocID string) error {
	var suggestion model.SupportGapSuggestion
	if err := s.coverageRepo.GetSuggestionByID(ctx, suggestionID, &suggestion); err != nil {
		return fmt.Errorf("get suggestion: %w", err)
	}
	if suggestion.WorkspaceID != workspaceID {
		return fmt.Errorf("suggestion not found")
	}
	if suggestion.Status != model.SupportCoverageSuggestionStatusDraft {
		return fmt.Errorf("suggestion is not in draft state")
	}

	suggestionType := suggestion.SuggestionType
	if overrideType != "" {
		suggestionType = overrideType
		if overrideTargetDocID != "" {
			suggestion.TargetDocumentID = &overrideTargetDocID
		}
	}

	switch suggestionType {
	case model.SupportCoverageSuggestionCreateArticle:
		return s.applyCreateArticle(ctx, &suggestion, userID)
	case model.SupportCoverageSuggestionUpdateArticle:
		return s.applyUpdateArticle(ctx, &suggestion, userID)
	default:
		return fmt.Errorf("unsupported suggestion type: %s", suggestionType)
	}
}

func (s *SupportCoverageDraftService) applyCreateArticle(ctx context.Context, suggestion *model.SupportGapSuggestion, userID string) error {
	spaceID := ""
	if suggestion.TargetSpaceID != nil {
		spaceID = *suggestion.TargetSpaceID
	}
	if spaceID == "" {
		return fmt.Errorf("target_space_id is required")
	}

	doc, err := s.documentSvc.Create(ctx, suggestion.WorkspaceID, model.CreateDocsDocumentRequest{
		SpaceID:      spaceID,
		CollectionID: suggestion.TargetCollectionID,
		Title:        suggestion.Title,
	}, userID)
	if err != nil {
		return fmt.Errorf("create document: %w", err)
	}

	if suggestion.Content != nil {
		if _, err := s.contentSvc.Save(ctx, doc.ID, suggestion.Content, userID); err != nil {
			return fmt.Errorf("save content: %w", err)
		}
	}

	docID := doc.ID
	if err := s.coverageRepo.UpdateSuggestionResult(ctx, suggestion.ID, &docID, nil, model.SupportCoverageSuggestionStatusApplied); err != nil {
		return fmt.Errorf("update suggestion result: %w", err)
	}

	// Close the loop: mark gap as done and link the new article.
	evidence30d, err := s.coverageRepo.CountEvidence30d(ctx, suggestion.GapID)
	if err != nil {
		return fmt.Errorf("count evidence: %w", err)
	}
	if err := s.coverageRepo.MarkGapDone(ctx, suggestion.WorkspaceID, suggestion.GapID, docID, evidence30d); err != nil {
		return fmt.Errorf("mark gap done: %w", err)
	}
	if err := s.coverageRepo.LinkGapArticle(ctx, suggestion.GapID, docID, suggestion.WorkspaceID); err != nil {
		return fmt.Errorf("link gap article: %w", err)
	}

	return nil
}

func (s *SupportCoverageDraftService) applyUpdateArticle(ctx context.Context, suggestion *model.SupportGapSuggestion, userID string) error {
	docID := ""
	if suggestion.TargetDocumentID != nil {
		docID = *suggestion.TargetDocumentID
	}
	if docID == "" {
		return fmt.Errorf("target_document_id is required")
	}

	// Safety check: verify document still belongs to the workspace.
	doc, err := s.documentSvc.Get(ctx, docID)
	if err != nil {
		return fmt.Errorf("get target document: %w", err)
	}
	if doc == nil || doc.WorkspaceID != suggestion.WorkspaceID {
		return fmt.Errorf("document not found in workspace")
	}

	// Snapshot existing content before overwrite.
	if s.versionSvc != nil {
		if _, err := s.versionSvc.SnapshotOnPublish(ctx, docID, userID); err != nil {
			s.logger.Warn("snapshot before update failed", "document_id", docID, "error", err)
		}
	}

	// Append new sections to existing content instead of replacing.
	if suggestion.Content != nil {
		existing, err := s.contentSvc.Get(ctx, docID)
		if err != nil {
			return fmt.Errorf("get existing content: %w", err)
		}
		var existingContent json.RawMessage
		if existing != nil {
			existingContent = existing.Content
		}
		merged, err := tiptap.AppendContent(existingContent, suggestion.Content)
		if err != nil {
			return fmt.Errorf("append content: %w", err)
		}
		if _, err := s.contentSvc.Save(ctx, docID, merged, userID); err != nil {
			return fmt.Errorf("save updated content: %w", err)
		}
	}

	if err := s.coverageRepo.UpdateSuggestionResult(ctx, suggestion.ID, &docID, nil, model.SupportCoverageSuggestionStatusApplied); err != nil {
		return fmt.Errorf("update suggestion result: %w", err)
	}

	// Close the loop: mark gap as done and link the updated article.
	evidence30d, err := s.coverageRepo.CountEvidence30d(ctx, suggestion.GapID)
	if err != nil {
		return fmt.Errorf("count evidence: %w", err)
	}
	if err := s.coverageRepo.MarkGapDone(ctx, suggestion.WorkspaceID, suggestion.GapID, docID, evidence30d); err != nil {
		return fmt.Errorf("mark gap done: %w", err)
	}
	if err := s.coverageRepo.LinkGapArticle(ctx, suggestion.GapID, docID, suggestion.WorkspaceID); err != nil {
		return fmt.Errorf("link gap article: %w", err)
	}

	return nil
}

// ─── LLM Generation ────────────────────────────────────────────────────────

func (s *SupportCoverageDraftService) generateDraftFromEvidence(ctx context.Context, detail *model.SupportCoverageGapDetail) (*CoverageArticleDraft, error) {
	// Build evidence snippets.
	var questions, answers []string
	for _, ev := range detail.Evidence {
		if ev.Excerpt != "" {
			if ev.EvidenceType == model.SupportEventAIHandoffTriggered ||
				ev.EvidenceType == model.SupportEventWidgetSearchPerformed ||
				ev.EvidenceType == model.SupportEventDocsIssueFeedback {
				questions = append(questions, ev.Excerpt)
			} else if ev.EvidenceType == model.SupportEventHumanReplyAfterAI {
				answers = append(answers, ev.Excerpt)
			}
		}
	}

	prompt := buildDraftPrompt(detail.Title, detail.IssueKey, questions, answers)

	draftCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := s.llmProvider.ChatCompletion(WithAIUsageMetering(draftCtx, AIUsageMeteringContext{
		WorkspaceID:    detail.WorkspaceID,
		FeatureKey:     BillingFeatureDocsArticleGeneration,
		IdempotencyKey: aiUsageIdempotencyKey(detail.WorkspaceID, BillingFeatureDocsArticleGeneration, "gap_draft", detail.ID),
		Metadata: map[string]interface{}{
			"gap_id": detail.ID,
		},
	}), llm.ChatRequest{
		SystemPrompt: "You are a technical writer creating help center articles. Write clear, concise documentation that answers the customer's question. Output JSON only.",
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   2000,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM completion: %w", err)
	}

	return parseDraftResponse(resp.Content, detail.Title, len(detail.Evidence))
}

func (s *SupportCoverageDraftService) generateUpdateFromEvidence(ctx context.Context, detail *model.SupportCoverageGapDetail, existing *model.DocsContent) (*CoverageArticleDraft, error) {
	var questions []string
	for _, ev := range detail.Evidence {
		if ev.Excerpt != "" {
			questions = append(questions, ev.Excerpt)
		}
	}

	existingText := ""
	if existing != nil && existing.ContentText != "" {
		existingText = existing.ContentText
	}

	prompt := buildUpdatePrompt(detail.Title, questions, existingText)

	updateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	targetDocumentID := ""
	if existing != nil {
		targetDocumentID = existing.DocumentID
	}
	resp, err := s.llmProvider.ChatCompletion(WithAIUsageMetering(updateCtx, AIUsageMeteringContext{
		WorkspaceID:    detail.WorkspaceID,
		FeatureKey:     BillingFeatureDocsArticleGeneration,
		IdempotencyKey: aiUsageIdempotencyKey(detail.WorkspaceID, BillingFeatureDocsArticleGeneration, "gap_update", detail.ID, targetDocumentID),
		Metadata: map[string]interface{}{
			"gap_id":             detail.ID,
			"target_document_id": targetDocumentID,
		},
	}), llm.ChatRequest{
		SystemPrompt: "You are a technical writer improving existing help center articles. Suggest changes that address the customer questions. Output JSON only.",
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   2000,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM completion: %w", err)
	}

	return parseDraftResponse(resp.Content, detail.Title, len(detail.Evidence))
}

func buildDraftPrompt(title, issueKey string, questions, answers []string) string {
	var b strings.Builder
	b.WriteString("Create a help center article for the following support gap.\n\n")
	b.WriteString(fmt.Sprintf("Topic: %s\n", title))
	if issueKey != "" {
		b.WriteString(fmt.Sprintf("Issue key: %s\n", issueKey))
	}
	if len(questions) > 0 {
		b.WriteString("\nCustomer questions:\n")
		for _, q := range questions[:min(len(questions), 5)] {
			b.WriteString(fmt.Sprintf("- %s\n", q))
		}
	}
	if len(answers) > 0 {
		b.WriteString("\nHuman agent answers:\n")
		for _, a := range answers[:min(len(answers), 3)] {
			b.WriteString(fmt.Sprintf("- %s\n", a))
		}
	}
	b.WriteString("\nWrite the full article in Markdown. Use headings, paragraphs, lists, and code blocks as appropriate.")
	b.WriteString("\nRespond with JSON: {\"title\": \"article title\", \"content\": \"full markdown content\"}")
	return b.String()
}

func buildUpdatePrompt(title string, questions []string, existingContent string) string {
	var b strings.Builder
	b.WriteString("Generate ONLY new sections to add to an existing help center article.\n")
	b.WriteString("Do NOT rewrite or modify existing content. Write sections that will be appended.\n\n")
	b.WriteString(fmt.Sprintf("Topic: %s\n", title))
	if len(questions) > 0 {
		b.WriteString("\nUnanswered customer questions:\n")
		for _, q := range questions[:min(len(questions), 5)] {
			b.WriteString(fmt.Sprintf("- %s\n", q))
		}
	}
	if existingContent != "" {
		truncated := existingContent
		if len(truncated) > 2000 {
			truncated = truncated[:2000] + "..."
		}
		b.WriteString(fmt.Sprintf("\nExisting article content (for context, do not repeat):\n%s\n", truncated))
	}
	b.WriteString("\nWrite new sections in Markdown. Use headings, paragraphs, lists, and code blocks as appropriate.")
	b.WriteString("\nRespond with JSON: {\"title\": \"section title\", \"content\": \"markdown content for new sections\"}")
	return b.String()
}

func parseDraftResponse(content, fallbackTitle string, evidenceCount int) (*CoverageArticleDraft, error) {
	var raw struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	title := raw.Title
	if title == "" {
		title = fallbackTitle
	}
	return &CoverageArticleDraft{
		Title:           title,
		MarkdownContent: raw.Content,
		EvidenceSummary: fmt.Sprintf("Generated from %d evidence items", evidenceCount),
	}, nil
}
