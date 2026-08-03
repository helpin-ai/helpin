package service

// support.search_knowledge: the runtime tool that gives support chat runs
// access to the workspace's agent-scoped knowledge base (docs embeddings,
// crawled content, curated guidance) with hybrid search + rerank. Results
// carry stable evidence IDs that support.send_reply later re-validates
// against, and every returned chunk is snapshotted per run for that check.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	supportKnowledgeMaxQueries       = 3
	supportKnowledgeContentExcerpt   = 1200
	supportKnowledgeDefaultMaxChunks = 8
)

// supportKnowledgeSearcher is the narrow SupportAIService surface the
// search_knowledge command consumes.
type supportKnowledgeSearcher interface {
	SearchKnowledgeForConversation(ctx context.Context, workspaceID, conversationID, language string, queries []string) (*SupportKnowledgeSearchOutcome, error)
}

// SetSupportKnowledgeDependencies wires the knowledge searcher and the
// per-run evidence store used by the support.search_knowledge tool.
func (s *InternalCommandService) SetSupportKnowledgeDependencies(searcher supportKnowledgeSearcher, evidenceRepo *repository.SupportRunEvidenceRepository) {
	s.supportKnowledgeSearcher = searcher
	s.supportRunEvidenceRepo = evidenceRepo
}

func (s *InternalCommandService) registerSupportKnowledgeCommands() {
	s.register(InternalCommandDefinition{
		Name:                 "support.search_knowledge",
		Module:               "support",
		Mutating:             false,
		SupportedTargetTypes: supportCommandTargetTypes,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.search_knowledge",
			Alias:       "search_knowledge",
			Category:    "Support",
			Description: "Search the workspace's support knowledge base (help docs, crawled content, curated guidance) with hybrid semantic search. The server automatically searches the visitor's exact message first. Query variants must only rephrase that request and must not introduce unverified numbers or facts. Results include evidence_id, URL, and authority — prefer curated/canonical over standard/secondary evidence and cite the used ids in send_support_reply claims. Chunks marked is_internal may inform reasoning but must never be quoted or referenced to the visitor.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"queries": map[string]any{
						"type":        "array",
						"description": "1-3 supplemental rephrasings of the visitor's question. Do not add prices, limits, dates, plan names, or factual assumptions that the visitor did not supply and prior evidence has not verified.",
						"items":       map[string]any{"type": "string"},
					},
					"language":    map[string]any{"type": "string", "description": "Optional ISO language code of the conversation."},
					"max_results": map[string]any{"type": "integer", "description": "Maximum chunks to return (default 8)."},
				},
				"required":             []string{"queries"},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.supportKnowledgeSearcher == nil {
				return nil, fmt.Errorf("support knowledge search is not configured")
			}
			conversationID := commandConversationTargetID(meta)
			if conversationID == "" {
				return nil, fmt.Errorf("search_knowledge requires a support conversation target")
			}
			var req struct {
				Queries    []string `json:"queries"`
				Language   string   `json:"language"`
				MaxResults int      `json:"max_results"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse search knowledge input: %w", err)
			}
			queries := make([]string, 0, supportKnowledgeMaxQueries)
			for _, query := range req.Queries {
				if query = strings.TrimSpace(query); query != "" {
					queries = append(queries, query)
				}
				if len(queries) == supportKnowledgeMaxQueries {
					break
				}
			}
			if len(queries) == 0 {
				return nil, fmt.Errorf("at least one non-empty query is required")
			}

			outcome, err := s.supportKnowledgeSearcher.SearchKnowledgeForConversation(ctx, meta.WorkspaceID, conversationID, strings.TrimSpace(req.Language), queries)
			if err != nil {
				return nil, err
			}
			results := outcome.Results
			maxResults := req.MaxResults
			if maxResults <= 0 || maxResults > supportKnowledgeDefaultMaxChunks {
				maxResults = supportKnowledgeDefaultMaxChunks
			}
			if len(results) > maxResults {
				results = results[:maxResults]
			}

			s.persistSupportRunEvidence(ctx, meta, results)
			requiredConfidence := 0.7
			if s.supportAIService != nil {
				if settings, settingsErr := s.supportAIService.loadSettings(ctx, meta.WorkspaceID); settingsErr == nil && settings != nil && settings.AIConfidenceThreshold > 0 {
					requiredConfidence = settings.AIConfidenceThreshold
				}
			}
			confidenceCeiling := supportEvidenceConfidenceCeiling(results)

			type knowledgeRow struct {
				EvidenceID  string  `json:"evidence_id"`
				Title       string  `json:"title,omitempty"`
				URL         string  `json:"url,omitempty"`
				SourceType  string  `json:"source_type"`
				IsInternal  bool    `json:"is_internal,omitempty"`
				HeadingPath string  `json:"heading_path,omitempty"`
				Content     string  `json:"content"`
				Score       float64 `json:"score"`
				Authority   string  `json:"authority"`
			}
			rows := make([]knowledgeRow, 0, len(results))
			for _, result := range results {
				content := result.Content
				if len(content) > supportKnowledgeContentExcerpt {
					content = content[:supportKnowledgeContentExcerpt] + "…"
				}
				rows = append(rows, knowledgeRow{
					EvidenceID:  result.ID,
					Title:       result.Title,
					URL:         result.URL,
					SourceType:  result.SourceType,
					IsInternal:  result.IsInternal,
					HeadingPath: result.HeadingPath,
					Content:     content,
					Score:       result.CombinedScore,
					Authority:   knowledgeResultAuthority(result),
				})
			}
			return mustJSON(map[string]any{
				"results":                           rows,
				"total":                             len(rows),
				"required_confidence":               requiredConfidence,
				"best_possible_grounded_confidence": confidenceCeiling,
				"note":                              "Cite evidence_id values in send_support_reply claims. Prefer curated and canonical evidence when sources conflict. Preserve the exact scope of prices and other numbers. If the evidence does not directly answer the visitor or its best possible grounded confidence is below required_confidence, gather stronger evidence with the permitted fallback before replying. Do not expose these mechanics or is_internal content to the visitor.",
			}), nil
		},
	})
}

// supportEvidenceConfidenceCeiling tells the runtime whether the strongest
// individual result could clear the workspace threshold if the model were
// fully confident. It is advisory only; send_support_reply recomputes the
// score from the evidence actually cited by the final claims.
func supportEvidenceConfidenceCeiling(results []KnowledgeSearchResult) float64 {
	best := 0.0
	for _, result := range results {
		id := strings.TrimSpace(result.ID)
		if id == "" {
			id = strings.TrimSpace(result.ReferenceID)
		}
		confidence := evaluateConfidence([]KnowledgeSearchResult{result}, &AIResponseContract{
			CanAnswer:    true,
			SourceDocIDs: []string{id},
			Confidence:   1,
			Claims:       []AIResponseClaim{{Text: "candidate", EvidenceIDs: []string{id}}},
		}, false)
		if confidence > best {
			best = confidence
		}
	}
	return best
}

func knowledgeResultAuthority(result KnowledgeSearchResult) string {
	switch {
	case result.SourceType == knowledgeSourceTypeGuidance:
		return "curated"
	case isCanonicalPricingURL(result.URL):
		return "canonical"
	case isArticleKnowledgeURL(result.URL):
		return "secondary"
	default:
		return "standard"
	}
}

// persistSupportRunEvidence snapshots the returned chunks for the calling run
// so send_support_reply can re-validate citations. Best effort: a support
// reply without evidence rows simply fails grounding later.
func (s *InternalCommandService) persistSupportRunEvidence(ctx context.Context, meta model.InternalCommandContext, results []KnowledgeSearchResult) {
	if s.supportRunEvidenceRepo == nil || len(results) == 0 {
		return
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil || run == nil {
		slog.WarnContext(ctx, "search_knowledge: resolve run for evidence failed",
			"error", err, "workspace_id", meta.WorkspaceID, "run_id", meta.RunID)
		return
	}
	rows := make([]model.SupportRunEvidence, 0, len(results))
	for _, result := range results {
		rows = append(rows, model.SupportRunEvidence{
			WorkspaceID:   run.WorkspaceID,
			RunID:         run.ID,
			EvidenceID:    result.ID,
			ReferenceID:   result.ReferenceID,
			SourceType:    result.SourceType,
			SourceID:      result.SourceID,
			DocumentID:    result.DocumentID,
			Title:         result.Title,
			URL:           result.URL,
			IsInternal:    result.IsInternal,
			Content:       result.Content,
			LexicalScore:  result.LexicalScore,
			VectorScore:   result.VectorScore,
			CombinedScore: result.CombinedScore,
		})
	}
	if err := s.supportRunEvidenceRepo.UpsertBatch(ctx, rows); err != nil {
		slog.WarnContext(ctx, "search_knowledge: persist evidence failed",
			"error", err, "workspace_id", run.WorkspaceID, "run_id", run.ID)
	}
}
