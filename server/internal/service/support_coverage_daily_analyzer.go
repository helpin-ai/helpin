package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
	"golang.org/x/sync/errgroup"
)

var errCoverageNoPublicSegment = errors.New("coverage conversation has no public segment")

const (
	coverageAnalysisMaxMessages             = 80
	coverageAnalysisMaxMessageChars         = 2000
	coverageAnalyzerVersion                 = "v4"
	coverageAnalysisWorkflowID              = "coverage-daily-analysis"
	coverageAnalysisCronSchedule            = "30 4 * * *"
	coverageAnalysisOverlap                 = 2 * time.Hour
	coverageAnalysisSettleDelay             = 10 * time.Minute
	coverageAnalysisBootstrapWindow         = 30 * 24 * time.Hour
	coverageAnalysisWorkspaceLimit          = 1000
	coverageAnalysisConversationConcurrency = 4
	coverageKnowledgeMinRelevanceScore      = 0.1
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
	WorkspaceID           string                        `json:"workspace_id"`
	ConversationID        string                        `json:"conversation_id"`
	Subject               string                        `json:"subject"`
	Status                string                        `json:"status"`
	FlowState             string                        `json:"flow_state"`
	AITurnCount           int                           `json:"ai_turn_count"`
	TranscriptHash        string                        `json:"transcript_hash"`
	SegmentID             string                        `json:"segment_id"`
	SegmentStartMessageID string                        `json:"segment_start_message_id"`
	SegmentEndMessageID   string                        `json:"segment_end_message_id"`
	SegmentStartAt        *time.Time                    `json:"segment_start_at,omitempty"`
	SegmentEndAt          *time.Time                    `json:"segment_end_at,omitempty"`
	SegmentResolved       bool                          `json:"segment_resolved"`
	HasHumanReply         bool                          `json:"has_human_reply"`
	Messages              []CoverageConversationMessage `json:"messages"`
	RetrievalTraces       []CoverageRetrievalTraceInput `json:"retrieval_traces"`
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
	BlockID       string  `json:"block_id,omitempty"`
	PageID        string  `json:"page_id,omitempty"`
	Title         string  `json:"title"`
	URL           string  `json:"url,omitempty"`
	Excerpt       string  `json:"excerpt"`
	CombinedScore float64 `json:"combined_score"`
}

type CoverageConversationAnalysisResult struct {
	IsSupportQuery       bool   `json:"is_support_query"`
	ConversationType     string `json:"conversation_type"`
	ClassificationReason string `json:"classification_reason"`

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

type CoverageFixBundleDecision struct {
	RecommendedFixes []CoverageRecommendedFix `json:"recommended_fixes"`
	DecisionReason   string                   `json:"decision_reason"`
	Confidence       float64                  `json:"confidence"`
}

type CoverageKnowledgeSuggestionDraft struct {
	Title           string `json:"title"`
	MarkdownContent string `json:"markdown_content"`
}

// CoverageConversationSegment represents a lifecycle segment of a support
// conversation delimited by resolved/reopened system events. The daily
// analyzer operates on the latest segment so that unrelated earlier issues
// do not contaminate gap detection.
type CoverageConversationSegment struct {
	ID              string
	StartMessageID  string
	EndMessageID    string
	StartAt         time.Time
	EndAt           *time.Time
	Resolved        bool
	PublicMessages  []model.SupportMessage
	PublicMessageID map[string]bool
}

type CoverageFindingUpsertInput struct {
	WorkspaceID                  string
	ConversationID               string
	MessageID                    string
	AnalysisID                   string
	Result                       CoverageConversationAnalysisResult
	MatchedKnowledgeCandidates   []CoverageKnowledgeCandidate
	RecommendationDecisionReason string
	SegmentID                    string
	SegmentStartMessageID        string
	SegmentEndMessageID          string
	SegmentResolved              bool
	HasHumanReply                bool
}

type SupportCoverageDailyAnalyzer struct {
	llmProvider       llm.Provider
	providerName      string
	modelName         string
	embeddingProvider llm.EmbeddingProvider
	embeddingModel    string
	coverageRepo      *repository.SupportCoverageRepository
	analysisRepo      *repository.SupportCoverageAnalysisRepository
	conversationRepo  *repository.SupportConversationRepository
	messageRepo       *repository.SupportMessageRepository
	knowledgeMatcher  *CoverageKnowledgeMatcher
	docsSpaceRepo     *repository.DocsSpaceRepository
	contentSourceRepo *repository.SupportContentSourceRepository
	temporalClient    tclient.Client
}

func NewSupportCoverageDailyAnalyzer(llmProvider llm.Provider, providerName, modelName string) *SupportCoverageDailyAnalyzer {
	return &SupportCoverageDailyAnalyzer{
		llmProvider:  llmProvider,
		providerName: strings.TrimSpace(providerName),
		modelName:    strings.TrimSpace(modelName),
	}
}

func (s *SupportCoverageDailyAnalyzer) SetCoverageRepositories(coverageRepo *repository.SupportCoverageRepository, analysisRepo *repository.SupportCoverageAnalysisRepository) *SupportCoverageDailyAnalyzer {
	if s == nil {
		return nil
	}
	s.coverageRepo = coverageRepo
	s.analysisRepo = analysisRepo
	return s
}

func (s *SupportCoverageDailyAnalyzer) SetEmbeddingProvider(provider llm.EmbeddingProvider, modelName string) *SupportCoverageDailyAnalyzer {
	if s == nil {
		return nil
	}
	s.embeddingProvider = provider
	s.embeddingModel = coverageEmbeddingModel(modelName)
	return s
}

func (s *SupportCoverageDailyAnalyzer) SetConversationRepositories(conversationRepo *repository.SupportConversationRepository, messageRepo *repository.SupportMessageRepository) *SupportCoverageDailyAnalyzer {
	if s == nil {
		return nil
	}
	s.conversationRepo = conversationRepo
	s.messageRepo = messageRepo
	return s
}

func (s *SupportCoverageDailyAnalyzer) SetKnowledgeMatcher(matcher *CoverageKnowledgeMatcher, docsSpaceRepo *repository.DocsSpaceRepository, contentSourceRepo *repository.SupportContentSourceRepository) *SupportCoverageDailyAnalyzer {
	if s == nil {
		return nil
	}
	s.knowledgeMatcher = matcher
	s.docsSpaceRepo = docsSpaceRepo
	s.contentSourceRepo = contentSourceRepo
	return s
}

func (s *SupportCoverageDailyAnalyzer) SetTemporalClient(client tclient.Client) *SupportCoverageDailyAnalyzer {
	if s == nil {
		return nil
	}
	s.temporalClient = client
	return s
}

func (s *SupportCoverageDailyAnalyzer) EnsureDailyAnalysis(ctx context.Context) error {
	if s == nil || s.temporalClient == nil {
		return nil
	}
	_, err := s.temporalClient.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:           coverageAnalysisWorkflowID,
		TaskQueue:    temporalapp.QueueAutomation,
		CronSchedule: coverageAnalysisCronSchedule,
	}, temporalapp.CoverageDailyAnalysisWorkflowType)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start coverage daily analysis workflow: %w", err)
	}
	return nil
}

func (s *SupportCoverageDailyAnalyzer) ListWorkspacesForDailyAnalysis(ctx context.Context) ([]string, error) {
	if s == nil || s.conversationRepo == nil {
		return nil, nil
	}
	windowEnd := time.Now().UTC().Add(-coverageAnalysisSettleDelay)
	windowStart := windowEnd.Add(-coverageAnalysisBootstrapWindow)
	return s.conversationRepo.ListWorkspacesForCoverageAnalysisCandidates(ctx, windowStart, windowEnd, coverageAnalysisWorkspaceLimit)
}

func (s *SupportCoverageDailyAnalyzer) RunWorkspaceDailyAnalysis(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time) error {
	if s == nil || s.analysisRepo == nil || s.coverageRepo == nil || s.conversationRepo == nil || s.messageRepo == nil {
		return fmt.Errorf("coverage daily analyzer dependencies are not configured")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if windowEnd.IsZero() {
		windowEnd = time.Now().UTC()
	}
	cursorEnd := windowEnd.UTC().Add(-coverageAnalysisSettleDelay)
	if cursorEnd.IsZero() {
		cursorEnd = time.Now().UTC().Add(-coverageAnalysisSettleDelay)
	}
	cursorStart := windowStart.UTC()
	lastCursor, err := s.analysisRepo.LastSuccessfulCursor(ctx, workspaceID, coverageAnalyzerVersion)
	if err != nil {
		return err
	}
	if lastCursor != nil {
		cursorStart = lastCursor.UTC().Add(-coverageAnalysisOverlap)
	} else if cursorStart.IsZero() {
		cursorStart = cursorEnd.Add(-coverageAnalysisBootstrapWindow)
	}
	if !cursorStart.Before(cursorEnd) {
		return nil
	}
	if windowStart.IsZero() {
		windowStart = cursorStart
	}

	run, err := s.analysisRepo.CreateRun(ctx, &model.SupportCoverageAnalysisRun{
		WorkspaceID:     workspaceID,
		WindowStart:     windowStart.UTC(),
		WindowEnd:       windowEnd.UTC(),
		CursorStartedAt: cursorStart,
		CursorEndedAt:   cursorEnd,
		AnalyzerVersion: coverageAnalyzerVersion,
		Status:          model.SupportCoverageAnalysisRunStatusRunning,
		StartedAt:       time.Now().UTC(),
		Metadata:        []byte("{}"),
	})
	if err != nil {
		return err
	}

	conversations, err := s.conversationRepo.ListCoverageAnalysisCandidates(ctx, workspaceID, cursorStart, cursorEnd, coverageAnalysisWorkspaceLimit)
	if err != nil {
		_ = s.analysisRepo.FailRun(ctx, run.ID, err)
		return err
	}
	gapCount := 0
	var gapCountMu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(coverageAnalysisConversationConcurrency)
	for _, conversation := range conversations {
		conversation := conversation
		group.Go(func() error {
			gapCreated, err := s.runConversationCoverageAnalysis(groupCtx, workspaceID, run.ID, conversation)
			if err != nil {
				return err
			}
			if gapCreated {
				gapCountMu.Lock()
				gapCount++
				gapCountMu.Unlock()
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		_ = s.analysisRepo.FailRun(ctx, run.ID, err)
		return err
	}
	materialized, err := s.materializeRunFindings(ctx, workspaceID, run.ID)
	if err != nil {
		_ = s.analysisRepo.FailRun(ctx, run.ID, err)
		return err
	}
	if materialized != nil {
		gapCount = materialized.EvidenceInserted
	}
	return s.analysisRepo.CompleteRun(ctx, run.ID, len(conversations), gapCount)
}

func (s *SupportCoverageDailyAnalyzer) runConversationCoverageAnalysis(ctx context.Context, workspaceID, runID string, conversation model.SupportConversation) (bool, error) {
	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversation.ID, true)
	if err != nil {
		return false, err
	}
	traces, err := s.analysisRepo.ListRetrievalTracesByConversation(ctx, workspaceID, conversation.ID)
	if err != nil {
		return false, err
	}
	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, traces)
	if err != nil {
		if errors.Is(err, errCoverageNoPublicSegment) {
			return false, nil
		}
		return false, err
	}
	alreadyAnalyzed, err := s.analysisRepo.AlreadyAnalyzedConversation(ctx, workspaceID, conversation.ID, input.TranscriptHash, coverageAnalyzerVersion)
	if err != nil {
		return false, err
	}
	if alreadyAnalyzed {
		return false, nil
	}

	// Deterministic prefilter: skip obvious non-support conversations without LLM.
	if localClass, skip := classifyCoverageConversationLocally(input); skip {
		classPayload, _ := json.Marshal(localClass)
		if err := s.analysisRepo.RecordConversationAnalysis(ctx, &model.SupportCoverageConversationAnalysis{
			WorkspaceID:          workspaceID,
			RunID:                runID,
			ConversationID:       conversation.ID,
			Status:               model.SupportCoverageConversationAnalysisStatusSkipped,
			TranscriptHash:       input.TranscriptHash,
			AnalyzerVersion:      coverageAnalyzerVersion,
			IsSupportQuery:       false,
			ConversationType:     localClass.ConversationType,
			ClassificationReason: localClass.ClassificationReason,
			RawOutput:            classPayload,
		}); err != nil {
			return false, err
		}
		return false, nil
	}

	result, raw, err := s.AnalyzeConversation(ctx, input)
	if err != nil {
		analysisErr := err.Error()
		recordErr := s.analysisRepo.RecordConversationAnalysis(ctx, &model.SupportCoverageConversationAnalysis{
			WorkspaceID:     workspaceID,
			RunID:           runID,
			ConversationID:  conversation.ID,
			Status:          model.SupportCoverageConversationAnalysisStatusFailed,
			TranscriptHash:  input.TranscriptHash,
			AnalyzerVersion: coverageAnalyzerVersion,
			ErrorMessage:    &analysisErr,
			RawOutput:       []byte("{}"),
		})
		return false, recordErr
	}
	if result == nil {
		return false, nil
	}

	// Defense-in-depth: override speculative human_resolution when no human actually replied.
	if !input.HasHumanReply && result.HumanResolution != "No human response observed" {
		result.HumanResolution = "No human response observed"
		raw, err = json.Marshal(result)
		if err != nil {
			return false, fmt.Errorf("marshal overridden analyzer result: %w", err)
		}
	}

	// Determine status: non-support conversations are "skipped", others "analyzed".
	analysisStatus := model.SupportCoverageConversationAnalysisStatusAnalyzed
	if !result.IsSupportQuery {
		analysisStatus = model.SupportCoverageConversationAnalysisStatusSkipped
	}

	matchedKnowledge := coverageKnowledgeCandidatesFromTraceInput(input.RetrievalTraces)
	recommendationDecisionReason := result.DecisionReason
	if result.HasGap && result.ShouldRunRetrieval {
		currentMatches, err := s.matchCurrentKnowledgeForAnalysis(ctx, workspaceID, *result)
		if err != nil {
			return false, err
		}
		if len(currentMatches) > 0 {
			matchedKnowledge = currentMatches
			refined, err := s.RefineFixBundleWithKnowledge(ctx, *result, currentMatches)
			if err != nil {
				slog.WarnContext(ctx, "coverage fix-bundle refinement failed; using analyzer recommendations",
					"error", err,
					"workspace_id", workspaceID,
					"conversation_id", conversation.ID,
				)
			} else if refined != nil {
				result.RecommendedFixes = refined.RecommendedFixes
				result.DecisionReason = firstNonEmptyCoverageString(refined.DecisionReason, result.DecisionReason)
				if refined.Confidence > 0 {
					result.Confidence = refined.Confidence
				}
				recommendationDecisionReason = refined.DecisionReason
			}
		}
	}

	if result.HasGap {
		raw, err = json.Marshal(result)
		if err != nil {
			return false, fmt.Errorf("marshal final analyzer result: %w", err)
		}
	}
	materializationMetadata := json.RawMessage(`{}`)
	if result.HasGap {
		payload, err := json.Marshal(map[string]any{
			"matched_knowledge_candidates":   matchedKnowledge,
			"recommendation_decision_reason": recommendationDecisionReason,
			"segment_id":                     input.SegmentID,
			"segment_start_message_id":       input.SegmentStartMessageID,
			"segment_end_message_id":         input.SegmentEndMessageID,
			"segment_resolved":               input.SegmentResolved,
			"has_human_reply":                input.HasHumanReply,
		})
		if err != nil {
			return false, fmt.Errorf("marshal materialization metadata: %w", err)
		}
		materializationMetadata = payload
	}

	analysis := &model.SupportCoverageConversationAnalysis{
		WorkspaceID:               workspaceID,
		RunID:                     runID,
		ConversationID:            conversation.ID,
		Status:                    analysisStatus,
		HasGap:                    result.HasGap,
		GapKind:                   result.GapKind,
		GapCategory:               result.GapCategory,
		PrimaryRecommendationType: primaryRecommendationType(result.RecommendedFixes),
		TranscriptHash:            input.TranscriptHash,
		AnalyzerVersion:           coverageAnalyzerVersion,
		CanonicalTitle:            result.CanonicalTitle,
		IsSupportQuery:            result.IsSupportQuery,
		ConversationType:          result.ConversationType,
		ClassificationReason:      result.ClassificationReason,
		CustomerNeed:              result.CustomerNeed,
		AIFailure:                 result.AIFailure,
		HumanResolution:           result.HumanResolution,
		DecisionReason:            result.DecisionReason,
		Confidence:                result.Confidence,
		RawOutput:                 raw,
		MaterializationMetadata:   materializationMetadata,
	}
	if err := s.analysisRepo.RecordConversationAnalysis(ctx, analysis); err != nil {
		return false, err
	}
	return result.HasGap, nil
}

func BuildCoverageConversationAnalysisInput(conversation model.SupportConversation, messages []model.SupportMessage, traces []model.SupportAIRetrievalTrace) (CoverageConversationAnalysisInput, error) {
	segment := BuildLatestCoverageConversationSegment(messages)
	if segment == nil {
		return CoverageConversationAnalysisInput{}, errCoverageNoPublicSegment
	}
	return BuildCoverageConversationAnalysisInputForSegment(conversation, *segment, traces)
}

// BuildCoverageConversationAnalysisInputForSegment builds analyzer input from
// a specific conversation segment. The IsInternal/system guard is kept as
// defense-in-depth even though segment.PublicMessages should already exclude
// internal messages.
func BuildCoverageConversationAnalysisInputForSegment(conversation model.SupportConversation, segment CoverageConversationSegment, traces []model.SupportAIRetrievalTrace) (CoverageConversationAnalysisInput, error) {
	orderedMessages := sortedCoverageMessages(segment.PublicMessages)
	hash := CoverageSegmentTranscriptHash(segment)
	analysisMessages := make([]CoverageConversationMessage, 0, len(orderedMessages))

	for _, message := range orderedMessages {
		if message.IsInternal || strings.TrimSpace(message.MessageType) == "system" {
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

	// Filter retrieval traces to messages actually sent to the LLM.
	analysisMessageIDs := map[string]bool{}
	for _, message := range analysisMessages {
		analysisMessageIDs[strings.TrimSpace(message.ID)] = true
	}
	filteredTraces := filterCoverageRetrievalTracesByMessageIDs(traces, analysisMessageIDs)

	hasHumanReply := false
	for _, message := range analysisMessages {
		if message.SenderType == "user" {
			hasHumanReply = true
			break
		}
	}

	flowState := ""
	if conversation.FlowState != nil {
		flowState = strings.TrimSpace(*conversation.FlowState)
	}
	traceInputs, err := coverageRetrievalTraceInputs(filteredTraces)
	if err != nil {
		return CoverageConversationAnalysisInput{}, err
	}

	return CoverageConversationAnalysisInput{
		WorkspaceID:           conversation.WorkspaceID,
		ConversationID:        conversation.ID,
		Subject:               strings.TrimSpace(conversation.Subject),
		Status:                strings.TrimSpace(conversation.Status),
		FlowState:             flowState,
		AITurnCount:           conversation.AITurnCount,
		TranscriptHash:        hash,
		SegmentID:             segment.ID,
		SegmentStartMessageID: segment.StartMessageID,
		SegmentEndMessageID:   segment.EndMessageID,
		SegmentStartAt:        timePtrIfNonZero(segment.StartAt),
		SegmentEndAt:          segment.EndAt,
		SegmentResolved:       segment.Resolved,
		HasHumanReply:         hasHumanReply,
		Messages:              analysisMessages,
		RetrievalTraces:       traceInputs,
	}, nil
}

// filterCoverageRetrievalTracesByMessageIDs returns only traces whose
// MessageID appears in the provided set. This ensures traces from earlier
// segments or truncated messages do not leak into analysis.
func filterCoverageRetrievalTracesByMessageIDs(traces []model.SupportAIRetrievalTrace, messageIDs map[string]bool) []model.SupportAIRetrievalTrace {
	if len(traces) == 0 || len(messageIDs) == 0 {
		return nil
	}
	filtered := make([]model.SupportAIRetrievalTrace, 0, len(traces))
	for _, trace := range traces {
		if messageIDs[strings.TrimSpace(trace.MessageID)] {
			filtered = append(filtered, trace)
		}
	}
	return filtered
}

func coverageKnowledgeCandidatesFromTraceInput(traces []CoverageRetrievalTraceInput) []CoverageKnowledgeCandidate {
	candidates := make([]CoverageKnowledgeCandidate, 0)
	for _, trace := range traces {
		candidates = append(candidates, trace.Results...)
	}
	return dedupeCoverageKnowledgeCandidates(candidates)
}

func (s *SupportCoverageDailyAnalyzer) matchCurrentKnowledgeForAnalysis(ctx context.Context, workspaceID string, result CoverageConversationAnalysisResult) ([]CoverageKnowledgeCandidate, error) {
	if s == nil || s.knowledgeMatcher == nil {
		return nil, nil
	}
	query := strings.TrimSpace(result.SearchQuery)
	if query == "" {
		query = strings.TrimSpace(result.CustomerNeed + " " + result.HumanResolution)
	}
	if query == "" {
		query = strings.TrimSpace(result.CanonicalTitle + " " + result.DecisionReason)
	}
	if query == "" {
		return nil, nil
	}

	spaceIDs, err := s.externalDocsSpaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	contentSourceIDs, err := s.supportContentSourceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(spaceIDs) == 0 && len(contentSourceIDs) == 0 {
		return nil, nil
	}
	candidates, err := s.knowledgeMatcher.MatchKnowledge(ctx, workspaceID, spaceIDs, contentSourceIDs, query, 8)
	if err != nil {
		return nil, err
	}
	if len(candidates) > 0 && candidates[0].CombinedScore < coverageKnowledgeMinRelevanceScore {
		slog.InfoContext(ctx, "skipping coverage knowledge refinement: best candidate below threshold",
			"best_score", candidates[0].CombinedScore,
			"threshold", coverageKnowledgeMinRelevanceScore,
			"workspace_id", workspaceID,
		)
		return nil, nil
	}
	return candidates, nil
}

func (s *SupportCoverageDailyAnalyzer) externalDocsSpaceIDs(ctx context.Context, workspaceID string) ([]string, error) {
	if s == nil || s.docsSpaceRepo == nil {
		return nil, nil
	}
	spaces, err := s.docsSpaceRepo.ListPublicByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(spaces))
	for _, space := range spaces {
		if strings.TrimSpace(space.ID) != "" {
			ids = append(ids, space.ID)
		}
	}
	return ids, nil
}

func (s *SupportCoverageDailyAnalyzer) supportContentSourceIDs(ctx context.Context, workspaceID string) ([]string, error) {
	if s == nil || s.contentSourceRepo == nil {
		return nil, nil
	}
	sources, err := s.contentSourceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.ID) != "" {
			ids = append(ids, source.ID)
		}
	}
	return ids, nil
}

// CoverageLocalClassification is the result of the deterministic prefilter.
type CoverageLocalClassification struct {
	ConversationType     string `json:"conversation_type"`
	ClassificationReason string `json:"classification_reason"`
}

// classifyCoverageConversationLocally applies conservative deterministic rules
// to skip obvious non-support conversations before the LLM call. The second
// return value is true when the classification is high-confidence and the
// conversation can be skipped without LLM analysis.
func classifyCoverageConversationLocally(input CoverageConversationAnalysisInput) (CoverageLocalClassification, bool) {
	// Rule 1: No customer messages at all — nothing to analyze.
	hasCustomerMessage := false
	for _, m := range input.Messages {
		if m.SenderType == "customer" {
			hasCustomerMessage = true
			break
		}
	}
	if !hasCustomerMessage {
		return CoverageLocalClassification{
			ConversationType:     "other",
			ClassificationReason: "no customer messages in conversation",
		}, true
	}

	// Gather subject (lowercased) and first customer message for pattern matching.
	subjectLower := strings.ToLower(strings.TrimSpace(input.Subject))
	var firstCustomerContent string
	for _, m := range input.Messages {
		if m.SenderType == "customer" {
			firstCustomerContent = strings.ToLower(m.Content)
			break
		}
	}

	// Rule 2: Auto-reply / bounce patterns in subject.
	autoReplyPrefixes := []string{
		"out of office",
		"automatic reply",
		"auto-reply",
		"auto reply",
		"delivery status notification",
		"undeliverable",
		"mail delivery failed",
		"returned mail",
	}
	for _, prefix := range autoReplyPrefixes {
		if strings.HasPrefix(subjectLower, prefix) || strings.Contains(subjectLower, prefix) {
			return CoverageLocalClassification{
				ConversationType:     "auto_reply",
				ClassificationReason: fmt.Sprintf("subject matches auto-reply pattern: %s", prefix),
			}, true
		}
	}

	// Rule 3: Newsletter / promotional patterns — only if no question mark in content.
	hasQuestion := strings.Contains(firstCustomerContent, "?")
	newsletterSignals := []string{
		"view this email in your browser",
		"manage your preferences",
		"unsubscribe from this list",
		"you are receiving this email because",
	}
	if !hasQuestion {
		for _, signal := range newsletterSignals {
			if strings.Contains(firstCustomerContent, signal) {
				return CoverageLocalClassification{
					ConversationType:     "newsletter",
					ClassificationReason: fmt.Sprintf("content matches newsletter pattern: %s", signal),
				}, true
			}
		}
	}

	// Rule 4: Cold outreach patterns — only if no question mark in content.
	if !hasQuestion {
		coldOutreachSignals := []string{
			"book a call",
			"book a demo",
			"schedule a call",
			"increase your leads",
			"guest post",
			"backlinks",
			"seo services",
			"partnership opportunity",
			"link building",
			"we help companies like yours",
		}
		for _, signal := range coldOutreachSignals {
			if strings.Contains(firstCustomerContent, signal) || strings.Contains(subjectLower, signal) {
				return CoverageLocalClassification{
					ConversationType:     "cold_outreach",
					ClassificationReason: fmt.Sprintf("content matches cold outreach pattern: %s", signal),
				}, true
			}
		}
	}

	// No confident local classification — let the LLM decide.
	return CoverageLocalClassification{}, false
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

func (s *SupportCoverageDailyAnalyzer) RefineFixBundleWithKnowledge(ctx context.Context, result CoverageConversationAnalysisResult, candidates []CoverageKnowledgeCandidate) (*CoverageFixBundleDecision, error) {
	if s == nil || s.llmProvider == nil {
		return nil, fmt.Errorf("coverage analyzer llm provider is not configured")
	}
	payload := struct {
		AnalysisResult CoverageConversationAnalysisResult `json:"analysis_result"`
		Candidates     []CoverageKnowledgeCandidate       `json:"candidates"`
	}{
		AnalysisResult: result,
		Candidates:     candidates,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal fix bundle refinement input: %w", err)
	}
	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: coverageFixBundleRefinementSystemPrompt(),
		Messages: []llm.Message{{
			Role:    "user",
			Content: string(payloadJSON),
		}},
		Provider:    s.providerName,
		Model:       s.modelName,
		Temperature: 0.1,
		MaxTokens:   1400,
		JSONMode:    true,
		JSONSchema:  coverageFixBundleDecisionJSONSchema(),
	})
	if err != nil {
		return nil, fmt.Errorf("coverage fix bundle refinement llm: %w", err)
	}

	var decision CoverageFixBundleDecision
	if err := llm.UnmarshalResponse(resp.Content, &decision); err != nil {
		return nil, fmt.Errorf("parse coverage fix bundle refinement response: %w", err)
	}
	normalizeCoverageFixBundleDecision(&decision)
	return &decision, nil
}

func (s *SupportCoverageDailyAnalyzer) GenerateKnowledgeSuggestion(ctx context.Context, result CoverageConversationAnalysisResult, fix CoverageRecommendedFix) (string, json.RawMessage, error) {
	if !isCoverageDocsFix(fix) {
		return "", nil, fmt.Errorf("knowledge suggestion generation only supports docs fixes")
	}
	if s == nil || s.llmProvider == nil {
		content, err := coverageSuggestionContent(result, fix)
		return firstNonEmptyCoverageString(fix.TargetTitle, result.CanonicalTitle, "Coverage gap fix"), content, err
	}
	payload := struct {
		Result CoverageConversationAnalysisResult `json:"result"`
		Fix    CoverageRecommendedFix             `json:"fix"`
	}{
		Result: result,
		Fix:    fix,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("marshal knowledge suggestion input: %w", err)
	}
	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: coverageKnowledgeSuggestionSystemPrompt(),
		Messages: []llm.Message{{
			Role:    "user",
			Content: string(payloadJSON),
		}},
		Provider:    s.providerName,
		Model:       s.modelName,
		Temperature: 0.1,
		MaxTokens:   1200,
		JSONMode:    true,
		JSONSchema:  coverageKnowledgeSuggestionJSONSchema(),
	})
	if err != nil {
		return "", nil, fmt.Errorf("coverage knowledge suggestion llm: %w", err)
	}
	var draft CoverageKnowledgeSuggestionDraft
	if err := llm.UnmarshalResponse(resp.Content, &draft); err != nil {
		return "", nil, fmt.Errorf("parse coverage knowledge suggestion: %w", err)
	}
	title := firstNonEmptyCoverageString(draft.Title, fix.TargetTitle, result.CanonicalTitle, "Coverage gap fix")
	markdown := strings.TrimSpace(draft.MarkdownContent)
	if markdown == "" {
		markdown = firstNonEmptyCoverageString(fix.SuggestedChange, result.HumanResolution, result.CustomerNeed)
	}
	return title, tiptap.MarkdownToJSON(markdown), nil
}

func (s *SupportCoverageDailyAnalyzer) UpsertFinding(ctx context.Context, input CoverageFindingUpsertInput) (*model.SupportCoverageGap, error) {
	if s == nil || s.coverageRepo == nil || s.analysisRepo == nil {
		return nil, fmt.Errorf("coverage repositories are not configured")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	result := input.Result
	normalizeCoverageConversationAnalysisResult(&result)
	if !result.HasGap {
		return nil, nil
	}
	now := time.Now()
	title := coverageTruncate(firstNonEmptyCoverageString(result.CanonicalTitle, result.CustomerNeed, "Coverage gap"), 160)
	clusterKey := ComputeSupportCoverageClusterKey(
		input.WorkspaceID,
		"daily_conversation_analysis",
		primaryCoverageTargetID(result.RecommendedFixes),
		"",
		result.CanonicalTitle+" "+result.CustomerNeed,
	)

	gap := &model.SupportCoverageGap{
		WorkspaceID:  input.WorkspaceID,
		DedupeKey:    clusterKey,
		GapKind:      firstNonEmptyCoverageString(result.GapKind, "content"),
		GapCategory:  firstNonEmptyCoverageString(result.GapCategory, model.SupportCoverageGapCategoryUnknown),
		V1GapType:    coverageV1GapTypeForFinding(result),
		Title:        title,
		IssueKey:     clusterKey,
		Status:       model.SupportCoverageGapStatusOpen,
		Confidence:   result.Confidence,
		SourceSignal: model.SupportCoverageGapSourceDailyConversationAnalysis,
		FirstSeenAt:  now,
		LastSeenAt:   now,
		Metadata:     []byte(`{"source":"daily_conversation_analysis"}`),
	}

	upserted, err := s.attachFindingToSimilarGap(ctx, input, result, gap, now)
	if err != nil {
		return nil, err
	}
	if upserted == nil {
		var topic *model.SupportCoverageTopic
		topic, err = s.coverageRepo.UpsertTopicByClusterKey(ctx, input.WorkspaceID, clusterKey, title)
		if err != nil {
			return nil, fmt.Errorf("upsert analysis topic: %w", err)
		}
		gap.TopicID = &topic.ID
		upserted, _, err = s.coverageRepo.UpsertOpenGapByTopic(ctx, gap)
		if err != nil {
			return nil, fmt.Errorf("upsert analysis gap: %w", err)
		}
	}

	evidenceMetadata, err := json.Marshal(map[string]any{
		"customer_need":                result.CustomerNeed,
		"ai_failure":                   result.AIFailure,
		"human_resolution":             result.HumanResolution,
		"decision_reason":              result.DecisionReason,
		"recommended_fixes":            result.RecommendedFixes,
		"conversation_analysis_id":     input.AnalysisID,
		"matched_knowledge_candidates": input.MatchedKnowledgeCandidates,
		"segment_id":                   input.SegmentID,
		"segment_start_message_id":     input.SegmentStartMessageID,
		"segment_end_message_id":       input.SegmentEndMessageID,
		"segment_resolved":             input.SegmentResolved,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal evidence metadata: %w", err)
	}
	evidence := &model.SupportGapEvidence{
		GapID:          upserted.ID,
		WorkspaceID:    input.WorkspaceID,
		EvidenceType:   model.SupportCoverageGapSourceDailyConversationAnalysis,
		ConversationID: emptyToNil(input.ConversationID),
		MessageID:      emptyToNil(input.MessageID),
		SourceSignal:   model.SupportCoverageGapSourceDailyConversationAnalysis,
		Excerpt:        coverageTruncate(firstNonEmptyCoverageString(result.CustomerNeed, result.DecisionReason, title), 500),
		Metadata:       evidenceMetadata,
		CreatedAt:      now,
	}
	if err := s.coverageRepo.CreateEvidence(ctx, evidence); err != nil {
		return nil, fmt.Errorf("create analysis evidence: %w", err)
	}

	recommendations, err := s.recommendationRowsForFinding(ctx, input, upserted.ID, now)
	if err != nil {
		return nil, err
	}
	if err := s.analysisRepo.ReplaceRecommendations(ctx, input.WorkspaceID, upserted.ID, recommendations); err != nil {
		return nil, fmt.Errorf("replace analysis recommendations: %w", err)
	}
	if input.AnalysisID != "" {
		if err := s.analysisRepo.SetConversationAnalysisGap(ctx, input.AnalysisID, upserted.ID, primaryRecommendationType(result.RecommendedFixes)); err != nil {
			return nil, err
		}
	}
	return upserted, nil
}

func (s *SupportCoverageDailyAnalyzer) attachFindingToSimilarGap(ctx context.Context, input CoverageFindingUpsertInput, result CoverageConversationAnalysisResult, gap *model.SupportCoverageGap, now time.Time) (*model.SupportCoverageGap, error) {
	items, err := s.coverageRepo.ListOpenGapsForClusterRebuild(ctx, input.WorkspaceID, coverageClusterCreationDedupeLimit)
	if err != nil {
		return nil, fmt.Errorf("list open gaps for similar-gap dedupe: %w", err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	incoming := coverageClusterCandidate{
		Gap: model.SupportCoverageGapListItem{
			SupportCoverageGap: *gap,
			CanonicalTitle:     result.CanonicalTitle,
			CustomerNeedText:   result.CustomerNeed,
			EvidenceText:       firstNonEmptyCoverageString(result.CustomerNeed, result.DecisionReason, result.HumanResolution, gap.Title),
		},
	}
	incoming.Text = coverageClusterComparisonText(incoming.Gap)
	incoming.Tokens = coverageClusterTokens(incoming.Text)

	var best *model.SupportCoverageGapListItem
	bestScore := 0.0
	for _, item := range items {
		if !coverageClusterCompatible(incoming.Gap, item) {
			continue
		}
		candidate := coverageClusterCandidate{
			Gap: item,
		}
		candidate.Text = coverageClusterComparisonText(item)
		candidate.Tokens = coverageClusterTokens(candidate.Text)
		score := coverageClusterSimilarity(incoming, candidate)
		if score > bestScore {
			bestScore = score
			itemCopy := item
			best = &itemCopy
		}
	}
	if best == nil {
		return nil, nil
	}
	bestCandidate := coverageClusterCandidate{Gap: *best}
	bestCandidate.Text = coverageClusterComparisonText(*best)
	bestCandidate.Tokens = coverageClusterTokens(bestCandidate.Text)
	if bestScore < coverageClusterAutoMergeThresholdFor(incoming, bestCandidate) {
		return nil, nil
	}
	return s.coverageRepo.IncrementOpenGapEvidence(ctx, input.WorkspaceID, best.ID, now)
}

func (s *SupportCoverageDailyAnalyzer) recommendationRowsForFinding(ctx context.Context, input CoverageFindingUpsertInput, gapID string, now time.Time) ([]model.SupportCoverageRecommendation, error) {
	result := input.Result
	fixes := result.RecommendedFixes
	rows := make([]model.SupportCoverageRecommendation, 0, len(fixes))
	for _, fix := range fixes {
		metadata, err := json.Marshal(map[string]any{
			"source":                       "daily_conversation_analysis",
			"decision_reason":              firstNonEmptyCoverageString(input.RecommendationDecisionReason, result.DecisionReason),
			"matched_knowledge_candidates": input.MatchedKnowledgeCandidates,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal recommendation metadata: %w", err)
		}
		row := model.SupportCoverageRecommendation{
			WorkspaceID:         input.WorkspaceID,
			GapID:               gapID,
			AnalysisID:          emptyToNil(input.AnalysisID),
			RecommendationType:  fix.Type,
			TargetType:          fix.TargetType,
			TargetID:            emptyToNil(fix.TargetID),
			TargetTitle:         fix.TargetTitle,
			TargetURL:           fix.TargetURL,
			Priority:            firstNonEmptyCoverageString(fix.Priority, model.SupportCoverageRecommendationPrioritySecondary),
			Status:              model.SupportCoverageRecommendationStatusOpen,
			Rationale:           fix.Rationale,
			SuggestedChange:     fix.SuggestedChange,
			ImplementationNotes: fix.ImplementationNotes,
			Metadata:            metadata,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if isCoverageDocsFix(fix) {
			if strings.TrimSpace(fix.TargetID) != "" {
				if err := s.coverageRepo.LinkGapArticle(ctx, gapID, strings.TrimSpace(fix.TargetID), input.WorkspaceID); err != nil {
					return nil, fmt.Errorf("link recommendation article: %w", err)
				}
			}
			if input.HasHumanReply {
				suggestion, err := s.createDocsSuggestionForFix(ctx, input, gapID, fix, metadata, now)
				if err != nil {
					return nil, err
				}
				row.SuggestionID = &suggestion.ID
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *SupportCoverageDailyAnalyzer) createDocsSuggestionForFix(ctx context.Context, input CoverageFindingUpsertInput, gapID string, fix CoverageRecommendedFix, metadata json.RawMessage, now time.Time) (*model.SupportGapSuggestion, error) {
	if err := s.coverageRepo.SupersedeActiveSuggestions(ctx, gapID, now); err != nil {
		return nil, err
	}
	suggestionType := model.SupportCoverageSuggestionCreateArticle
	if fix.Type == model.SupportCoverageFixUpdateArticle {
		suggestionType = model.SupportCoverageSuggestionUpdateArticle
	}
	title, content, err := s.GenerateKnowledgeSuggestion(ctx, input.Result, fix)
	if err != nil {
		return nil, err
	}
	suggestion := &model.SupportGapSuggestion{
		GapID:            gapID,
		WorkspaceID:      input.WorkspaceID,
		SuggestionType:   suggestionType,
		Status:           model.SupportCoverageSuggestionStatusDraft,
		Title:            title,
		Content:          content,
		EvidenceSummary:  coverageTruncate(firstNonEmptyCoverageString(input.Result.HumanResolution, input.Result.DecisionReason, input.Result.CustomerNeed), 500),
		TargetDocumentID: emptyToNil(fix.TargetID),
		IsActive:         true,
		Metadata:         metadata,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	created, err := s.coverageRepo.CreateSuggestion(ctx, suggestion)
	if err != nil {
		return nil, err
	}
	return created, nil
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

// CoverageSegmentTranscriptHash computes a transcript hash that incorporates
// segment boundary information so that different segments of the same
// conversation produce different hashes.
func CoverageSegmentTranscriptHash(segment CoverageConversationSegment) string {
	baseHash := CoverageTranscriptHash(segment.PublicMessages)
	fields := []string{
		strings.TrimSpace(segment.ID),
		strings.TrimSpace(segment.StartMessageID),
		strings.TrimSpace(segment.EndMessageID),
		baseHash,
	}
	if !segment.StartAt.IsZero() {
		fields = append(fields, segment.StartAt.UTC().Format(time.RFC3339Nano))
	}
	if segment.EndAt != nil && !segment.EndAt.IsZero() {
		fields = append(fields, segment.EndAt.UTC().Format(time.RFC3339Nano))
	}
	sum := sha256.Sum256([]byte(strings.Join(fields, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func timePtrIfNonZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func primaryCoverageTargetID(fixes []CoverageRecommendedFix) string {
	for _, fix := range fixes {
		if fix.Priority == model.SupportCoverageRecommendationPriorityPrimary && strings.TrimSpace(fix.TargetID) != "" {
			return strings.TrimSpace(fix.TargetID)
		}
	}
	for _, fix := range fixes {
		if strings.TrimSpace(fix.TargetID) != "" {
			return strings.TrimSpace(fix.TargetID)
		}
	}
	return ""
}

func primaryRecommendationType(fixes []CoverageRecommendedFix) string {
	for _, fix := range fixes {
		if fix.Priority == model.SupportCoverageRecommendationPriorityPrimary && strings.TrimSpace(fix.Type) != "" {
			return strings.TrimSpace(fix.Type)
		}
	}
	for _, fix := range fixes {
		if strings.TrimSpace(fix.Type) != "" {
			return strings.TrimSpace(fix.Type)
		}
	}
	return ""
}

func coverageV1GapTypeForFinding(result CoverageConversationAnalysisResult) string {
	if result.GapCategory != model.SupportCoverageGapCategoryKnowledge {
		return model.SupportCoverageV1GapNeedsReview
	}
	for _, fix := range result.RecommendedFixes {
		switch fix.Type {
		case model.SupportCoverageFixCreateArticle:
			return model.SupportCoverageV1GapMissingArticle
		case model.SupportCoverageFixUpdateArticle:
			return model.SupportCoverageV1GapWeakArticle
		}
	}
	return model.SupportCoverageV1GapNeedsReview
}

func isCoverageDocsFix(fix CoverageRecommendedFix) bool {
	return fix.Type == model.SupportCoverageFixCreateArticle || fix.Type == model.SupportCoverageFixUpdateArticle
}

func coverageSuggestionContent(result CoverageConversationAnalysisResult, fix CoverageRecommendedFix) (json.RawMessage, error) {
	lines := []string{
		firstNonEmptyCoverageString(fix.SuggestedChange, result.HumanResolution, result.CustomerNeed),
	}
	if strings.TrimSpace(result.HumanResolution) != "" {
		lines = append(lines, "Human resolution: "+strings.TrimSpace(result.HumanResolution))
	}
	if strings.TrimSpace(fix.ImplementationNotes) != "" {
		lines = append(lines, "Implementation notes: "+strings.TrimSpace(fix.ImplementationNotes))
	}
	content := map[string]any{
		"type": "doc",
		"content": []map[string]any{
			{
				"type": "paragraph",
				"content": []map[string]any{
					{
						"type": "text",
						"text": strings.Join(lines, "\n\n"),
					},
				},
			},
		},
	}
	raw, err := json.Marshal(content)
	return json.RawMessage(raw), err
}

func firstNonEmptyCoverageString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// validConversationTypes is the set of recognized conversation_type values.
var validConversationTypes = map[string]bool{
	"support_query": true,
	"newsletter":    true,
	"cold_outreach": true,
	"auto_reply":    true,
	"transactional": true,
	"spam":          true,
	"internal":      true,
	"other":         true,
}

func normalizeCoverageConversationAnalysisResult(result *CoverageConversationAnalysisResult) {
	if result == nil {
		return
	}
	// Normalize classification fields.
	result.ConversationType = strings.TrimSpace(result.ConversationType)
	result.ClassificationReason = strings.TrimSpace(result.ClassificationReason)
	if result.ConversationType == "" {
		result.ConversationType = "support_query"
	}
	if !validConversationTypes[result.ConversationType] {
		result.ConversationType = "other"
	}
	// Derive IsSupportQuery from conversation_type to enforce consistency.
	result.IsSupportQuery = result.ConversationType == "support_query"

	// If not a support query, force zero-value gap fields.
	if !result.IsSupportQuery {
		result.HasGap = false
		result.ShouldRunRetrieval = false
		result.RecommendedFixes = nil
		result.GapKind = ""
		result.GapCategory = ""
		result.CanonicalTitle = ""
		result.CustomerNeed = ""
		result.AIFailure = ""
		result.HumanResolution = ""
		result.SearchQuery = ""
		result.Confidence = 0
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

func normalizeCoverageFixBundleDecision(decision *CoverageFixBundleDecision) {
	if decision == nil {
		return
	}
	decision.DecisionReason = strings.TrimSpace(decision.DecisionReason)
	if len(decision.RecommendedFixes) > 3 {
		decision.RecommendedFixes = decision.RecommendedFixes[:3]
	}
	for i := range decision.RecommendedFixes {
		fix := &decision.RecommendedFixes[i]
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

The provided messages may be only the latest lifecycle segment of a longer conversation.
Do not infer gaps from earlier issues that are not present in the provided segment.
Use segment_id and segment boundary fields only as analysis context.

First classify whether the conversation contains a genuine customer or prospect need. Set conversation_type=support_query for customer/prospect questions, support issues, account or billing requests, setup/troubleshooting requests, and product-evaluation questions such as pricing, migration, integration, security, or comparisons.

Set is_support_query=false for newsletters, cold outreach, auto-replies, bounces, transactional notifications with no support request, spam/phishing, internal messages, and emails that are not from someone seeking help or product information. Explain the classification briefly in classification_reason.

If is_support_query=false, set has_gap=false, should_run_retrieval=false, recommended_fixes=[], and leave gap details empty.

If is_support_query=true, proceed with gap analysis:

Decide from the full conversation outcome, not one message. Treat human replies as the best evidence of what was missing. Use live retrieval traces to diagnose what AI actually searched and saw during the conversation.

Do not create a gap if the AI correctly resolved the issue. Set should_run_retrieval=true only when current docs, website, or customer-facing content search can materially improve the recommendation.

Prefer knowledge/content gaps only when customer-facing knowledge could reasonably fix the issue. Use data/context when the human used customer, account, order, subscription, or similar data. Use action when the human performed an operation the AI could not perform. Use policy when the human applied judgment, approval, exception, or escalation policy.

Recommend multiple fixes when one surface alone will not reduce repeated human intervention. Limit to 3 fixes. Mark exactly one fix as primary unless two fixes are equally necessary.

Recommend website/content changes for prospect, sales, pricing, migration, integration, security, comparison, or pre-purchase questions. Recommend docs changes for setup, usage, troubleshooting, and post-signup workflows.

Be concise in all text fields. Each field should be one sentence, two at most:
- customer_need: what the customer needed, not a retelling of the conversation.
- ai_failure: specifically what the AI lacked or got wrong.
- human_resolution: the concrete action the human took. If no message from a human agent (sender_type=user) appears in the conversation, set human_resolution to "No human response observed" exactly. Do not speculate what a human agent would or should do.
- decision_reason: why this gap matters, not a summary of the above fields.
- rationale, suggested_change, implementation_notes in recommended_fixes: one actionable sentence each. Do not repeat information across fields.`
}

func coverageConversationAnalysisJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"is_support_query",
			"conversation_type",
			"classification_reason",
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
			"is_support_query":      map[string]any{"type": "boolean"},
			"conversation_type":     map[string]any{"type": "string", "enum": []string{"support_query", "newsletter", "cold_outreach", "auto_reply", "transactional", "spam", "internal", "other"}},
			"classification_reason": map[string]any{"type": "string"},
			"has_gap":               map[string]any{"type": "boolean"},
			"gap_kind":              map[string]any{"type": "string", "enum": []string{"", "content", "data", "action", "policy"}},
			"gap_category":          map[string]any{"type": "string", "enum": []string{"", model.SupportCoverageGapCategoryKnowledge, model.SupportCoverageGapCategoryStructure, model.SupportCoverageGapCategoryConflict, model.SupportCoverageGapCategoryContext, model.SupportCoverageGapCategoryAction, model.SupportCoverageGapCategoryWorkflow, model.SupportCoverageGapCategoryPolicy, model.SupportCoverageGapCategoryEvaluation, model.SupportCoverageGapCategoryUnknown}},
			"canonical_title":       map[string]any{"type": "string"},
			"customer_need":         map[string]any{"type": "string"},
			"ai_failure":            map[string]any{"type": "string"},
			"human_resolution":      map[string]any{"type": "string"},
			"decision_reason":       map[string]any{"type": "string"},
			"search_query":          map[string]any{"type": "string"},
			"should_run_retrieval":  map[string]any{"type": "boolean"},
			"confidence":            map[string]any{"type": "number"},
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

func coverageFixBundleRefinementSystemPrompt() string {
	return `You refine support coverage gap recommendations using current customer-facing knowledge candidates. Return JSON only.

Use update_article only when a candidate help article is clearly about the same customer need but is missing, outdated, or unclear. Use update_website_page when a website/content page should answer a prospect, sales, pricing, integration, migration, security, comparison, or pre-purchase question. Use create_article or create_website_page when no candidate covers the same topic.

Preserve data, action, policy, workflow, and agent-instruction recommendations from the first-pass analysis if they remain relevant. Mixed cases may produce multiple fixes, capped at 3. Mark exactly one primary fix unless two fixes are equally necessary.

Be concise: decision_reason should be one sentence for a UI card. Each fix field (rationale, suggested_change, implementation_notes) should be one actionable sentence. Do not repeat information across fields.`
}

func coverageFixBundleDecisionJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"recommended_fixes", "decision_reason", "confidence"},
		"properties": map[string]any{
			"decision_reason": map[string]any{"type": "string"},
			"confidence":      map[string]any{"type": "number"},
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

func coverageKnowledgeSuggestionSystemPrompt() string {
	return `You write concise help-center documentation fixes from analyzed support coverage gaps. Return JSON only.

Use the human_resolution as evidence for what solved the customer problem. Do not invent product behavior, policy, pricing, integrations, or steps that are not supported by the customer need, human resolution, or suggested change. If the fix is an update_article, write content that can be appended or merged into the existing article. If the fix is create_article, write a short standalone article draft.

Return a clear title and markdown_content. Keep the markdown practical: headings, short paragraphs, and bullets when useful.`
}

func coverageKnowledgeSuggestionJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"title", "markdown_content"},
		"properties": map[string]any{
			"title":            map[string]any{"type": "string"},
			"markdown_content": map[string]any{"type": "string"},
		},
	}
}

// BuildLatestCoverageConversationSegment derives lifecycle segments from the
// chronologically ordered message list and returns the latest one. Segments
// are delimited by resolved system events. The first public message after a
// resolved event starts a new segment — this handles the email reopen edge
// case where the customer reply is written before the reopened system event.
func BuildLatestCoverageConversationSegment(messages []model.SupportMessage) *CoverageConversationSegment {
	ordered := sortedCoverageMessages(messages)
	var segments []CoverageConversationSegment
	var current *CoverageConversationSegment

	ensureCurrent := func(message model.SupportMessage) {
		if current != nil {
			return
		}
		current = &CoverageConversationSegment{
			ID:              message.ID,
			StartMessageID:  message.ID,
			StartAt:         message.CreatedAt.UTC(),
			PublicMessageID: map[string]bool{},
		}
	}

	closeCurrent := func(event model.SupportMessage) {
		if current == nil || len(current.PublicMessages) == 0 {
			return
		}
		endAt := event.CreatedAt.UTC()
		current.EndAt = &endAt
		current.EndMessageID = event.ID
		current.Resolved = true
		segments = append(segments, *current)
		current = nil
	}

	for _, message := range ordered {
		if isCoverageResolvedSystemMessage(message) {
			closeCurrent(message)
			continue
		}
		if message.IsInternal || strings.TrimSpace(message.MessageType) == "system" {
			continue
		}
		ensureCurrent(message)
		current.PublicMessages = append(current.PublicMessages, message)
		current.PublicMessageID[message.ID] = true
		current.EndMessageID = message.ID
	}

	if current != nil && len(current.PublicMessages) > 0 {
		segments = append(segments, *current)
	}
	if len(segments) == 0 {
		return nil
	}
	segment := segments[len(segments)-1]
	if segment.PublicMessageID == nil {
		segment.PublicMessageID = map[string]bool{}
		for _, message := range segment.PublicMessages {
			segment.PublicMessageID[message.ID] = true
		}
	}
	segment.ID = coverageSegmentID(segment)
	return &segment
}

func isCoverageResolvedSystemMessage(message model.SupportMessage) bool {
	if strings.TrimSpace(message.MessageType) != "system" || message.SystemEventType == nil {
		return false
	}
	return strings.TrimSpace(*message.SystemEventType) == model.SystemEventResolved
}

func coverageSegmentID(segment CoverageConversationSegment) string {
	start := strings.TrimSpace(segment.StartMessageID)
	end := strings.TrimSpace(segment.EndMessageID)
	if start == "" && len(segment.PublicMessages) > 0 {
		start = segment.PublicMessages[0].ID
	}
	if end == "" && len(segment.PublicMessages) > 0 {
		end = segment.PublicMessages[len(segment.PublicMessages)-1].ID
	}
	if end == "" {
		end = "open"
	}
	return start + ":" + end
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
