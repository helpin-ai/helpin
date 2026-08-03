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
	supportKnowledgeDefaultMaxChunks = 12
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
			Description: "Search the workspace's support knowledge base (help docs, crawled content, curated guidance) with hybrid semantic search. Returns chunks with evidence_id values — cite these ids in send_support_reply claims. Chunks marked is_internal may inform your reasoning but must never be quoted or referenced to the visitor.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"queries": map[string]any{
						"type":        "array",
						"description": "1-3 search query variants (rephrase the visitor's question; add one variant with key product terms).",
						"items":       map[string]any{"type": "string"},
					},
					"language":    map[string]any{"type": "string", "description": "Optional ISO language code of the conversation."},
					"max_results": map[string]any{"type": "integer", "description": "Maximum chunks to return (default 12)."},
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

			type knowledgeRow struct {
				EvidenceID  string  `json:"evidence_id"`
				Title       string  `json:"title,omitempty"`
				URL         string  `json:"url,omitempty"`
				SourceType  string  `json:"source_type"`
				IsInternal  bool    `json:"is_internal,omitempty"`
				HeadingPath string  `json:"heading_path,omitempty"`
				Content     string  `json:"content"`
				Score       float64 `json:"score"`
			}
			rows := make([]knowledgeRow, 0, len(results))
			for _, result := range results {
				content := result.Content
				if len(content) > supportKnowledgeContentExcerpt {
					content = content[:supportKnowledgeContentExcerpt] + "…"
				}
				rows = append(rows, knowledgeRow{
					EvidenceID:  result.ReferenceID,
					Title:       result.Title,
					URL:         result.URL,
					SourceType:  result.SourceType,
					IsInternal:  result.IsInternal,
					HeadingPath: result.HeadingPath,
					Content:     content,
					Score:       result.CombinedScore,
				})
			}
			return mustJSON(map[string]any{
				"results": rows,
				"total":   len(rows),
				"note":    "Cite evidence_id values in send_support_reply claims. Do not expose is_internal content to the visitor.",
			}), nil
		},
	})
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
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
			EvidenceID:  result.ReferenceID,
			SourceType:  result.SourceType,
			SourceID:    result.SourceID,
			DocumentID:  result.DocumentID,
			Title:       result.Title,
			URL:         result.URL,
			IsInternal:  result.IsInternal,
			Content:     result.Content,
		})
	}
	if err := s.supportRunEvidenceRepo.UpsertBatch(ctx, rows); err != nil {
		slog.WarnContext(ctx, "search_knowledge: persist evidence failed",
			"error", err, "workspace_id", run.WorkspaceID, "run_id", run.ID)
	}
}
