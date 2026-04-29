package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

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
