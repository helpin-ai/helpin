package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	coverageAnalysisMaxMessages     = 80
	coverageAnalysisMaxMessageChars = 2000
)

type CoverageConversationMessage struct {
	ID              string    `json:"id"`
	SenderType      string    `json:"sender_type"`
	MessageType     string    `json:"message_type"`
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"created_at"`
	AIConfidence    float64   `json:"ai_confidence,omitempty"`
	AIReplyKind     string    `json:"ai_reply_kind,omitempty"`
	AIIssueKey      string    `json:"ai_issue_key,omitempty"`
	AIIssueSummary  string    `json:"ai_issue_summary,omitempty"`
	AIProgressState string    `json:"ai_progress_state,omitempty"`
}

type CoverageConversationAnalysisInput struct {
	WorkspaceID     string                        `json:"workspace_id"`
	ConversationID  string                        `json:"conversation_id"`
	Subject         string                        `json:"subject"`
	Status          string                        `json:"status"`
	FlowState       string                        `json:"flow_state"`
	AITurnCount     int                           `json:"ai_turn_count"`
	TranscriptHash  string                        `json:"transcript_hash"`
	Messages        []CoverageConversationMessage `json:"messages"`
	RetrievalTraces []CoverageRetrievalTraceInput `json:"retrieval_traces"`
}

type CoverageRetrievalTraceInput struct {
	MessageID      string                       `json:"message_id"`
	SearchQueries  []string                     `json:"search_queries"`
	Results        []CoverageKnowledgeCandidate `json:"results"`
	CitedSourceIDs []string                     `json:"cited_source_ids"`
	AIConfidence   float64                      `json:"ai_confidence"`
	FailureMode    string                       `json:"failure_mode"`
}

type CoverageKnowledgeCandidate struct {
	SourceType    string  `json:"source_type"`
	TargetType    string  `json:"target_type"`
	DocumentID    string  `json:"document_id,omitempty"`
	PageID        string  `json:"page_id,omitempty"`
	Title         string  `json:"title"`
	URL           string  `json:"url,omitempty"`
	Excerpt       string  `json:"excerpt"`
	CombinedScore float64 `json:"combined_score"`
}

type CoverageConversationAnalysisResult struct {
	HasGap             bool                     `json:"has_gap"`
	GapKind            string                   `json:"gap_kind"`
	GapCategory        string                   `json:"gap_category"`
	CanonicalTitle     string                   `json:"canonical_title"`
	CustomerNeed       string                   `json:"customer_need"`
	AIFailure          string                   `json:"ai_failure"`
	HumanResolution    string                   `json:"human_resolution"`
	DecisionReason     string                   `json:"decision_reason"`
	SearchQuery        string                   `json:"search_query"`
	ShouldRunRetrieval bool                     `json:"should_run_retrieval"`
	RecommendedFixes   []CoverageRecommendedFix `json:"recommended_fixes"`
	Confidence         float64                  `json:"confidence"`
}

type CoverageRecommendedFix struct {
	Type                string `json:"type"`
	TargetType          string `json:"target_type"`
	TargetID            string `json:"target_id"`
	TargetTitle         string `json:"target_title"`
	TargetURL           string `json:"target_url"`
	Priority            string `json:"priority"`
	Rationale           string `json:"rationale"`
	SuggestedChange     string `json:"suggested_change"`
	ImplementationNotes string `json:"implementation_notes"`
}

type SupportCoverageDailyAnalyzer struct {
	llmProvider  llm.Provider
	providerName string
	modelName    string
}

func NewSupportCoverageDailyAnalyzer(llmProvider llm.Provider, providerName, modelName string) *SupportCoverageDailyAnalyzer {
	return &SupportCoverageDailyAnalyzer{
		llmProvider:  llmProvider,
		providerName: strings.TrimSpace(providerName),
		modelName:    strings.TrimSpace(modelName),
	}
}

func BuildCoverageConversationAnalysisInput(conversation model.SupportConversation, messages []model.SupportMessage, traces []model.SupportAIRetrievalTrace) (CoverageConversationAnalysisInput, error) {
	orderedMessages := sortedCoverageMessages(messages)
	hash := CoverageTranscriptHash(orderedMessages)
	analysisMessages := make([]CoverageConversationMessage, 0, len(orderedMessages))

	for _, message := range orderedMessages {
		if message.IsInternal {
			continue
		}
		analysisMessage := CoverageConversationMessage{
			ID:          message.ID,
			SenderType:  strings.TrimSpace(message.SenderType),
			MessageType: strings.TrimSpace(message.MessageType),
			Content:     truncateCoverageAnalysisContent(normalizeCoverageTranscriptContent(message.Content), coverageAnalysisMaxMessageChars),
			CreatedAt:   message.CreatedAt.UTC(),
		}
		if strings.TrimSpace(message.SenderType) == "ai" {
			if metadata, ok := parseCoverageAIMessageMetadata(message.Metadata); ok {
				analysisMessage.AIConfidence = metadata.AIConfidence
				analysisMessage.AIReplyKind = metadata.AIReplyKind
				analysisMessage.AIIssueKey = metadata.AIIssueKey
				analysisMessage.AIIssueSummary = metadata.AIIssueSummary
				analysisMessage.AIProgressState = metadata.AIProgressState
			}
		}
		analysisMessages = append(analysisMessages, analysisMessage)
	}

	if len(analysisMessages) > coverageAnalysisMaxMessages {
		analysisMessages = analysisMessages[len(analysisMessages)-coverageAnalysisMaxMessages:]
	}

	flowState := ""
	if conversation.FlowState != nil {
		flowState = strings.TrimSpace(*conversation.FlowState)
	}
	traceInputs, err := coverageRetrievalTraceInputs(traces)
	if err != nil {
		return CoverageConversationAnalysisInput{}, err
	}

	return CoverageConversationAnalysisInput{
		WorkspaceID:     conversation.WorkspaceID,
		ConversationID:  conversation.ID,
		Subject:         strings.TrimSpace(conversation.Subject),
		Status:          strings.TrimSpace(conversation.Status),
		FlowState:       flowState,
		AITurnCount:     conversation.AITurnCount,
		TranscriptHash:  hash,
		Messages:        analysisMessages,
		RetrievalTraces: traceInputs,
	}, nil
}

func (s *SupportCoverageDailyAnalyzer) AnalyzeConversation(ctx context.Context, input CoverageConversationAnalysisInput) (*CoverageConversationAnalysisResult, json.RawMessage, error) {
	if s == nil || s.llmProvider == nil {
		return nil, nil, fmt.Errorf("coverage analyzer llm provider is not configured")
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal analyzer input: %w", err)
	}
	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: coverageConversationAnalysisSystemPrompt(),
		Messages: []llm.Message{{
			Role:    "user",
			Content: string(inputJSON),
		}},
		Provider:    s.providerName,
		Model:       s.modelName,
		Temperature: 0.1,
		MaxTokens:   1800,
		JSONMode:    true,
		JSONSchema:  coverageConversationAnalysisJSONSchema(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("coverage conversation analyzer llm: %w", err)
	}

	var result CoverageConversationAnalysisResult
	if err := llm.UnmarshalResponse(resp.Content, &result); err != nil {
		return nil, nil, fmt.Errorf("parse coverage conversation analyzer response: %w", err)
	}
	normalizeCoverageConversationAnalysisResult(&result)
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal normalized analyzer result: %w", err)
	}
	return &result, raw, nil
}

func CoverageTranscriptHash(messages []model.SupportMessage) string {
	ordered := sortedCoverageMessages(messages)
	records := make([]string, 0, len(ordered))
	for _, message := range ordered {
		if message.IsInternal {
			continue
		}
		fields := []string{
			message.ID,
			strings.TrimSpace(message.SenderType),
			strings.TrimSpace(message.MessageType),
			normalizeCoverageTranscriptContent(message.Content),
			message.CreatedAt.UTC().Format(time.RFC3339Nano),
			normalizeCoverageTranscriptMetadata(message.Metadata),
		}
		records = append(records, strings.Join(fields, "\x1f"))
	}
	sum := sha256.Sum256([]byte(strings.Join(records, "\x1e")))
	return hex.EncodeToString(sum[:])
}

func normalizeCoverageConversationAnalysisResult(result *CoverageConversationAnalysisResult) {
	if result == nil {
		return
	}
	result.GapKind = strings.TrimSpace(result.GapKind)
	result.GapCategory = strings.TrimSpace(result.GapCategory)
	result.CanonicalTitle = strings.TrimSpace(result.CanonicalTitle)
	result.CustomerNeed = strings.TrimSpace(result.CustomerNeed)
	result.AIFailure = strings.TrimSpace(result.AIFailure)
	result.HumanResolution = strings.TrimSpace(result.HumanResolution)
	result.DecisionReason = strings.TrimSpace(result.DecisionReason)
	result.SearchQuery = strings.TrimSpace(result.SearchQuery)
	if len(result.RecommendedFixes) > 3 {
		result.RecommendedFixes = result.RecommendedFixes[:3]
	}
	for i := range result.RecommendedFixes {
		fix := &result.RecommendedFixes[i]
		fix.Type = strings.TrimSpace(fix.Type)
		fix.TargetType = strings.TrimSpace(fix.TargetType)
		fix.TargetID = strings.TrimSpace(fix.TargetID)
		fix.TargetTitle = strings.TrimSpace(fix.TargetTitle)
		fix.TargetURL = strings.TrimSpace(fix.TargetURL)
		fix.Priority = strings.TrimSpace(fix.Priority)
		if fix.Priority == "" {
			fix.Priority = model.SupportCoverageRecommendationPrioritySecondary
		}
		fix.Rationale = strings.TrimSpace(fix.Rationale)
		fix.SuggestedChange = strings.TrimSpace(fix.SuggestedChange)
		fix.ImplementationNotes = strings.TrimSpace(fix.ImplementationNotes)
	}
}

func coverageConversationAnalysisSystemPrompt() string {
	return `You analyze support conversations to find durable AI coverage gaps. Return JSON only.

Decide from the full conversation outcome, not one message. Treat human replies as the best evidence of what was missing. Use live retrieval traces to diagnose what AI actually searched and saw during the conversation.

Do not create a gap if the AI correctly resolved the issue. Set should_run_retrieval=true only when current docs, website, or customer-facing content search can materially improve the recommendation.

Prefer knowledge/content gaps only when customer-facing knowledge could reasonably fix the issue. Use data/context when the human used customer, account, order, subscription, or similar data. Use action when the human performed an operation the AI could not perform. Use policy when the human applied judgment, approval, exception, or escalation policy.

Recommend multiple fixes when one surface alone will not reduce repeated human intervention. Limit to 3 fixes. Mark exactly one fix as primary unless two fixes are equally necessary.

Recommend website/content changes for prospect, sales, pricing, migration, integration, security, comparison, or pre-purchase questions. Recommend docs changes for setup, usage, troubleshooting, and post-signup workflows.`
}

func coverageConversationAnalysisJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"has_gap",
			"gap_kind",
			"gap_category",
			"canonical_title",
			"customer_need",
			"ai_failure",
			"human_resolution",
			"decision_reason",
			"search_query",
			"should_run_retrieval",
			"recommended_fixes",
			"confidence",
		},
		"properties": map[string]any{
			"has_gap":              map[string]any{"type": "boolean"},
			"gap_kind":             map[string]any{"type": "string", "enum": []string{"", "content", "data", "action", "policy"}},
			"gap_category":         map[string]any{"type": "string", "enum": []string{"", model.SupportCoverageGapCategoryKnowledge, model.SupportCoverageGapCategoryStructure, model.SupportCoverageGapCategoryConflict, model.SupportCoverageGapCategoryContext, model.SupportCoverageGapCategoryAction, model.SupportCoverageGapCategoryWorkflow, model.SupportCoverageGapCategoryPolicy, model.SupportCoverageGapCategoryEvaluation, model.SupportCoverageGapCategoryUnknown}},
			"canonical_title":      map[string]any{"type": "string"},
			"customer_need":        map[string]any{"type": "string"},
			"ai_failure":           map[string]any{"type": "string"},
			"human_resolution":     map[string]any{"type": "string"},
			"decision_reason":      map[string]any{"type": "string"},
			"search_query":         map[string]any{"type": "string"},
			"should_run_retrieval": map[string]any{"type": "boolean"},
			"confidence":           map[string]any{"type": "number"},
			"recommended_fixes": map[string]any{
				"type":     "array",
				"maxItems": 3,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"type", "target_type", "target_id", "target_title", "target_url", "priority", "rationale", "suggested_change", "implementation_notes"},
					"properties": map[string]any{
						"type":                 map[string]any{"type": "string", "enum": []string{model.SupportCoverageFixCreateArticle, model.SupportCoverageFixUpdateArticle, model.SupportCoverageFixUpdateWebsitePage, model.SupportCoverageFixCreateWebsitePage, model.SupportCoverageFixAddData, model.SupportCoverageFixAddAction, model.SupportCoverageFixDefinePolicy, model.SupportCoverageFixImproveWorkflow, model.SupportCoverageFixNoFix}},
						"target_type":          map[string]any{"type": "string", "enum": []string{"", "docs", "website_page", "content_source", "data_source", "tool_action", "policy", "workflow", "agent_instruction"}},
						"target_id":            map[string]any{"type": "string"},
						"target_title":         map[string]any{"type": "string"},
						"target_url":           map[string]any{"type": "string"},
						"priority":             map[string]any{"type": "string", "enum": []string{"", model.SupportCoverageRecommendationPriorityPrimary, model.SupportCoverageRecommendationPrioritySecondary}},
						"rationale":            map[string]any{"type": "string"},
						"suggested_change":     map[string]any{"type": "string"},
						"implementation_notes": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func sortedCoverageMessages(messages []model.SupportMessage) []model.SupportMessage {
	ordered := append([]model.SupportMessage(nil), messages...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})
	return ordered
}

func parseCoverageAIMessageMetadata(raw string) (AIMessageMetadata, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return AIMessageMetadata{}, false
	}
	var metadata AIMessageMetadata
	if err := json.Unmarshal([]byte(trimmed), &metadata); err != nil {
		return AIMessageMetadata{}, false
	}
	return metadata, true
}

func normalizeCoverageTranscriptContent(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.TrimSpace(normalized)
}

func normalizeCoverageTranscriptMetadata(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}"
	}
	var value any
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return trimmed
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return trimmed
	}
	return string(normalized)
}

func truncateCoverageAnalysisContent(content string, maxChars int) string {
	if maxChars <= 0 || len(content) <= maxChars {
		return content
	}
	return strings.TrimSpace(content[:maxChars]) + "..."
}

func coverageRetrievalTraceInputs(traces []model.SupportAIRetrievalTrace) ([]CoverageRetrievalTraceInput, error) {
	inputs := make([]CoverageRetrievalTraceInput, 0, len(traces))
	for _, trace := range traces {
		searchQueries, err := decodeStringJSONList(trace.SearchQueries)
		if err != nil {
			return nil, fmt.Errorf("decode trace search queries for message %s: %w", trace.MessageID, err)
		}
		citedSourceIDs, err := decodeStringJSONList(trace.CitedSourceIDs)
		if err != nil {
			return nil, fmt.Errorf("decode trace cited source ids for message %s: %w", trace.MessageID, err)
		}
		results, err := decodeCoverageKnowledgeCandidates(trace.Results)
		if err != nil {
			return nil, fmt.Errorf("decode trace results for message %s: %w", trace.MessageID, err)
		}
		inputs = append(inputs, CoverageRetrievalTraceInput{
			MessageID:      trace.MessageID,
			SearchQueries:  searchQueries,
			Results:        results,
			CitedSourceIDs: citedSourceIDs,
			AIConfidence:   trace.AIConfidence,
			FailureMode:    strings.TrimSpace(trace.FailureMode),
		})
	}
	sort.SliceStable(inputs, func(i, j int) bool {
		return inputs[i].MessageID < inputs[j].MessageID
	})
	return inputs, nil
}

func decodeStringJSONList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

type coverageTraceResult struct {
	SourceType    string  `json:"source_type"`
	TargetType    string  `json:"target_type"`
	DocumentID    string  `json:"document_id"`
	PageID        string  `json:"page_id"`
	SourceID      string  `json:"source_id"`
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Snippet       string  `json:"snippet"`
	Excerpt       string  `json:"excerpt"`
	CombinedScore float64 `json:"combined_score"`
}

func decodeCoverageKnowledgeCandidates(raw json.RawMessage) ([]CoverageKnowledgeCandidate, error) {
	if len(raw) == 0 {
		return []CoverageKnowledgeCandidate{}, nil
	}
	var results []coverageTraceResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	candidates := make([]CoverageKnowledgeCandidate, 0, len(results))
	for _, result := range results {
		sourceType := strings.TrimSpace(result.SourceType)
		targetType := strings.TrimSpace(result.TargetType)
		if targetType == "" {
			switch sourceType {
			case knowledgeSourceTypeDocs:
				targetType = "docs"
			case knowledgeSourceTypeContent:
				sourceType = "website"
				targetType = "website_page"
			}
		}
		excerpt := strings.TrimSpace(result.Excerpt)
		if excerpt == "" {
			excerpt = strings.TrimSpace(result.Snippet)
		}
		pageID := strings.TrimSpace(result.PageID)
		if pageID == "" && sourceType == "website" {
			pageID = strings.TrimSpace(result.SourceID)
		}
		candidates = append(candidates, CoverageKnowledgeCandidate{
			SourceType:    sourceType,
			TargetType:    targetType,
			DocumentID:    strings.TrimSpace(result.DocumentID),
			PageID:        pageID,
			Title:         strings.TrimSpace(result.Title),
			URL:           strings.TrimSpace(result.URL),
			Excerpt:       excerpt,
			CombinedScore: result.CombinedScore,
		})
	}
	return candidates, nil
}
