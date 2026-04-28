package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type EvidenceSnippet struct {
	Excerpt      string
	SourceSignal string
	CreatedAt    time.Time
}

type KBCandidate struct {
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	Score      int    `json:"score"`
}

type KBContext struct {
	Titles     []string      `json:"titles"`
	Candidates []KBCandidate `json:"candidates"`
}

type EnrichmentInput struct {
	TopicID   string
	Evidence  []EvidenceSnippet
	KBContext KBContext
}

type EnrichmentResult struct {
	CanonicalTitle   string          `json:"canonical_title"`
	GapSubtype       string          `json:"gap_subtype"`
	Route            string          `json:"route"`
	TargetDocumentID string          `json:"target_document_id"`
	DraftContent     json.RawMessage `json:"draft_content"`
	Confidence       float64         `json:"confidence"`
}

type SupportCoverageEnrichmentService struct {
	db          *gorm.DB
	llmProvider llm.Provider
	now         func() time.Time
}

func NewSupportCoverageEnrichmentService(db *gorm.DB, llmProvider llm.Provider) *SupportCoverageEnrichmentService {
	return &SupportCoverageEnrichmentService{
		db:          db,
		llmProvider: llmProvider,
		now:         time.Now,
	}
}

func (s *SupportCoverageEnrichmentService) LoadKBContext(ctx context.Context, workspaceID, summary string) (KBContext, error) {
	var docs []struct {
		ID    string
		Title string
	}
	if err := s.db.WithContext(ctx).
		Table("docs_documents").
		Select("id, title").
		Where("workspace_id = ? AND status = ?", workspaceID, model.DocStatusPublished).
		Find(&docs).Error; err != nil {
		return KBContext{}, fmt.Errorf("load kb docs: %w", err)
	}

	summaryTokens := tokenSet(normalizeForCluster(summary))
	ctxOut := KBContext{Titles: make([]string, 0, len(docs))}
	for _, doc := range docs {
		ctxOut.Titles = append(ctxOut.Titles, doc.Title)
		score := overlapScore(summaryTokens, tokenSet(normalizeForCluster(doc.Title)))
		ctxOut.Candidates = append(ctxOut.Candidates, KBCandidate{
			DocumentID: doc.ID,
			Title:      doc.Title,
			Score:      score,
		})
	}
	sort.SliceStable(ctxOut.Candidates, func(i, j int) bool {
		if ctxOut.Candidates[i].Score == ctxOut.Candidates[j].Score {
			return ctxOut.Candidates[i].Title < ctxOut.Candidates[j].Title
		}
		return ctxOut.Candidates[i].Score > ctxOut.Candidates[j].Score
	})
	if len(ctxOut.Candidates) > 5 {
		ctxOut.Candidates = ctxOut.Candidates[:5]
	}
	return ctxOut, nil
}

func (s *SupportCoverageEnrichmentService) EnrichTopic(ctx context.Context, topicID string) error {
	if s.llmProvider == nil {
		return fmt.Errorf("coverage enrichment llm provider is not configured")
	}

	var topic model.SupportCoverageTopic
	if err := s.db.WithContext(ctx).Where("id = ?", topicID).First(&topic).Error; err != nil {
		return fmt.Errorf("load topic: %w", err)
	}
	now := s.now()
	if topic.CooldownUntil != nil && topic.CooldownUntil.After(now) {
		return nil
	}

	gap, err := s.openGapForTopic(ctx, topicID)
	if err != nil {
		return err
	}
	if gap == nil {
		return nil
	}

	evidence, summary, err := s.loadEvidence(ctx, topicID)
	if err != nil {
		return err
	}
	kbContext, err := s.LoadKBContext(ctx, topic.WorkspaceID, summary)
	if err != nil {
		return err
	}

	promptBytes, _ := json.Marshal(EnrichmentInput{
		TopicID:   topicID,
		Evidence:  evidence,
		KBContext: kbContext,
	})
	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: "You generate support coverage gap article suggestions. Return JSON only.",
		Messages: []llm.Message{{
			Role:    "user",
			Content: string(promptBytes),
		}},
		Temperature: 0.1,
		MaxTokens:   1600,
		JSONMode:    true,
		JSONSchema:  coverageEnrichmentJSONSchema(),
	})
	if err != nil {
		return fmt.Errorf("coverage enrichment llm: %w", err)
	}

	var result EnrichmentResult
	if err := llm.UnmarshalResponse(resp.Content, &result); err != nil {
		return fmt.Errorf("parse coverage enrichment response: %w", err)
	}
	if len(result.DraftContent) == 0 || string(result.DraftContent) == "null" {
		result.DraftContent = json.RawMessage(`{"type":"doc","content":[]}`)
	}
	if result.Route == "" {
		result.Route = model.SupportCoverageSuggestionCreateArticle
	}
	if result.GapSubtype == "" {
		result.GapSubtype = gap.V1GapType
	}

	return s.writeEnrichmentResult(ctx, &topic, gap, result, now)
}

func (s *SupportCoverageEnrichmentService) openGapForTopic(ctx context.Context, topicID string) (*model.SupportCoverageGap, error) {
	var gap model.SupportCoverageGap
	err := s.db.WithContext(ctx).
		Where("topic_id = ? AND status = ?", topicID, model.SupportCoverageGapStatusOpen).
		Order("last_seen_at DESC").
		First(&gap).Error
	if err == nil {
		return &gap, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, fmt.Errorf("load open topic gap: %w", err)
}

func (s *SupportCoverageEnrichmentService) loadEvidence(ctx context.Context, topicID string) ([]EvidenceSnippet, string, error) {
	var rows []EvidenceSnippet
	if err := s.db.WithContext(ctx).
		Table("support_gap_evidence e").
		Select("e.excerpt, e.source_signal, e.created_at").
		Joins("JOIN support_coverage_gaps g ON g.id = e.gap_id").
		Where("g.topic_id = ?", topicID).
		Order("e.created_at DESC").
		Limit(20).
		Find(&rows).Error; err != nil {
		return nil, "", fmt.Errorf("load topic evidence: %w", err)
	}
	var summaryParts []string
	for _, row := range rows {
		if strings.TrimSpace(row.Excerpt) != "" {
			summaryParts = append(summaryParts, row.Excerpt)
		}
	}
	return rows, strings.Join(summaryParts, "\n"), nil
}

func (s *SupportCoverageEnrichmentService) writeEnrichmentResult(ctx context.Context, topic *model.SupportCoverageTopic, gap *model.SupportCoverageGap, result EnrichmentResult, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SupportGapSuggestion{}).
			Where("gap_id = ? AND is_active", gap.ID).
			Updates(map[string]interface{}{
				"is_active":     false,
				"superseded_at": now,
				"updated_at":    now,
			}).Error; err != nil {
			return fmt.Errorf("supersede active suggestions: %w", err)
		}

		suggestionType := result.Route
		if suggestionType == "create_new" {
			suggestionType = model.SupportCoverageSuggestionCreateArticle
		}
		if suggestionType == "update_existing" {
			suggestionType = model.SupportCoverageSuggestionUpdateArticle
		}
		suggestion := model.SupportGapSuggestion{
			ID:               uuid.New().String(),
			GapID:            gap.ID,
			WorkspaceID:      gap.WorkspaceID,
			SuggestionType:   suggestionType,
			Status:           model.SupportCoverageSuggestionStatusDraft,
			Title:            result.CanonicalTitle,
			Content:          result.DraftContent,
			EvidenceSummary:  strings.TrimSpace(result.CanonicalTitle),
			TargetDocumentID: emptyToNil(result.TargetDocumentID),
			IsActive:         true,
			Metadata:         []byte("{}"),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := tx.Create(&suggestion).Error; err != nil {
			return fmt.Errorf("create enrichment suggestion: %w", err)
		}

		cooldown := now.Add(time.Hour)
		if err := tx.Model(&model.SupportCoverageTopic{}).
			Where("id = ?", topic.ID).
			Updates(map[string]interface{}{
				"canonical_title":  result.CanonicalTitle,
				"last_enriched_at": now,
				"cooldown_until":   cooldown,
				"updated_at":       now,
			}).Error; err != nil {
			return fmt.Errorf("update enriched topic: %w", err)
		}
		if result.GapSubtype != "" {
			if err := tx.Model(&model.SupportCoverageGap{}).
				Where("id = ?", gap.ID).
				Updates(map[string]interface{}{
					"v1_gap_type": result.GapSubtype,
					"confidence":  result.Confidence,
					"updated_at":  now,
				}).Error; err != nil {
				return fmt.Errorf("update enriched gap: %w", err)
			}
		}
		return nil
	})
}

func coverageEnrichmentJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"canonical_title":    map[string]any{"type": "string"},
			"gap_subtype":        map[string]any{"type": "string"},
			"route":              map[string]any{"type": "string"},
			"target_document_id": map[string]any{"type": "string"},
			"draft_content":      map[string]any{"type": "object"},
			"confidence":         map[string]any{"type": "number"},
		},
		"required":             []string{"canonical_title", "gap_subtype", "route", "draft_content", "confidence"},
		"additionalProperties": false,
	}
}

func tokenSet(normalized string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range strings.Fields(normalized) {
		out[token] = struct{}{}
	}
	return out
}

func overlapScore(a, b map[string]struct{}) int {
	score := 0
	for token := range a {
		if _, ok := b[token]; ok {
			score++
		}
	}
	return score
}

func emptyToNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
