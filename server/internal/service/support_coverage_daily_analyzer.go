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

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
)

var errCoverageNoPublicSegment = errors.New("coverage conversation has no public segment")
var errCoverageLLMContract = errors.New("coverage analyzer response violates the actionable finding contract")

const (
	coverageAnalysisMaxMessages             = 80
	coverageAnalysisMaxMessageChars         = 2000
	coverageAnalyzerVersion                 = "v4"
	coverageAnalysisWorkflowID              = "coverage-analysis-v2"
	coverageAnalysisCronSchedule            = "0 */3 * * *"
	coverageAnalysisOverlap                 = 2 * time.Hour
	coverageAnalysisSettleDelay             = 10 * time.Minute
	coverageAnalysisBootstrapWindow         = 30 * 24 * time.Hour
	coverageAnalysisWorkspaceLimit          = 1000
	coverageAnalysisConversationConcurrency = 4
	coverageAnalysisCandidatePageSize       = 25
	// Coverage knowledge matching uses reciprocal-rank fusion scores, not
	// cosine scores. A top lexical-only hit is ~0.016 and a top lexical+vector
	// hit is ~0.033, so keep this floor on that scale.
	coverageKnowledgeMinRelevanceScore = 0.015
)

type CoverageConversationMessage struct {
	VisitorFeedback *model.SupportAnswerFeedback `json:"visitor_feedback,omitempty"`
	ID              string                       `json:"id"`
	SenderType      string                       `json:"sender_type"`
	MessageType     string                       `json:"message_type"`
	Content         string                       `json:"content"`
	CreatedAt       time.Time                    `json:"created_at"`
	AIConfidence    float64                      `json:"ai_confidence,omitempty"`
	AIReplyKind     string                       `json:"ai_reply_kind,omitempty"`
	AIIssueKey      string                       `json:"ai_issue_key,omitempty"`
	AIIssueSummary  string                       `json:"ai_issue_summary,omitempty"`
	AIProgressState string                       `json:"ai_progress_state,omitempty"`
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

type SupportCoverageDailyAnalyzer struct {
	jevDecisions      *JevDecisionService
	llmProvider       llm.Provider
	providerName      string
	modelName         string
	embeddingProvider llm.EmbeddingProvider
	embeddingModel    string
	coverageRepo      *repository.SupportCoverageRepository
	analysisRepo      *repository.SupportCoverageAnalysisRepository
	coverageV2Repo    *repository.CoverageV2Repository
	conversationRepo  *repository.SupportConversationRepository
	messageRepo       *repository.SupportMessageRepository
	knowledgeMatcher  *CoverageKnowledgeMatcher
	docsSpaceRepo     *repository.DocsSpaceRepository
	contentSourceRepo *repository.SupportContentSourceRepository
	temporalClient    tclient.Client
	rolloutPolicy     *CoverageRolloutPolicy
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

func (s *SupportCoverageDailyAnalyzer) SetCoverageV2Repository(repo *repository.CoverageV2Repository) *SupportCoverageDailyAnalyzer {
	if s == nil {
		return nil
	}
	s.coverageV2Repo = repo
	return s
}

func (s *SupportCoverageDailyAnalyzer) SetCoverageRolloutPolicy(policy *CoverageRolloutPolicy) *SupportCoverageDailyAnalyzer {
	if s != nil {
		s.rolloutPolicy = policy
	}
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
	if s == nil {
		return nil, nil
	}
	var candidateWorkspaces []string
	if s.conversationRepo != nil {
		windowEnd := time.Now().UTC().Add(-coverageAnalysisSettleDelay)
		windowStart := windowEnd.Add(-coverageAnalysisBootstrapWindow)
		var err error
		candidateWorkspaces, err = s.conversationRepo.ListWorkspacesForCoverageAnalysisCandidates(ctx, windowStart, windowEnd, coverageAnalysisWorkspaceLimit)
		if err != nil {
			return nil, err
		}
	}
	var queuedWorkspaces []string
	if s.coverageV2Repo != nil {
		var err error
		queuedWorkspaces, err = s.coverageV2Repo.ListQueuedWorkspaces(ctx, coverageAnalysisWorkspaceLimit)
		if err != nil {
			return nil, err
		}
	}
	merged := mergeCoverageWorkspaceIDs(candidateWorkspaces, queuedWorkspaces, coverageAnalysisWorkspaceLimit)
	if s.rolloutPolicy == nil {
		return merged, nil
	}
	filtered := make([]string, 0, len(merged))
	for _, workspaceID := range merged {
		if s.rolloutPolicy.CaptureEnabled(workspaceID) {
			filtered = append(filtered, workspaceID)
		}
	}
	return filtered, nil
}

func mergeCoverageWorkspaceIDs(primary, queued []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	merged := make([]string, 0, min(limit, len(primary)+len(queued)))
	seen := make(map[string]struct{}, len(primary)+len(queued))
	for _, group := range [][]string{primary, queued} {
		for _, workspaceID := range group {
			workspaceID = strings.TrimSpace(workspaceID)
			if workspaceID == "" {
				continue
			}
			if _, exists := seen[workspaceID]; exists {
				continue
			}
			seen[workspaceID] = struct{}{}
			merged = append(merged, workspaceID)
			if len(merged) == limit {
				return merged
			}
		}
	}
	return merged
}

func (s *SupportCoverageDailyAnalyzer) reconcileQueuedCoverageAttempts(ctx context.Context, workspaceID string) error {
	if s.coverageV2Repo == nil {
		return nil
	}
	owner := fmt.Sprintf("coverage-reconcile:%s:%d", workspaceID, time.Now().UTC().UnixNano())
	for {
		attempts, err := s.coverageV2Repo.ClaimAttemptsForWorkspace(ctx, workspaceID, owner, time.Now().UTC(), 15*time.Minute, coverageAnalysisCandidatePageSize)
		if err != nil {
			return err
		}
		if len(attempts) == 0 {
			return nil
		}
		var page sync.WaitGroup
		semaphore := make(chan struct{}, coverageAnalysisConversationConcurrency)
		for index := range attempts {
			attempt := attempts[index]
			page.Add(1)
			go func() {
				defer page.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
				if attempt.SourceKind != "conversation" {
					s.markCoverageV2AttemptRetryable(ctx, &attempt, owner, model.CoverageFailureCandidateQuery, fmt.Errorf("unsupported coverage source kind %q", attempt.SourceKind))
					return
				}
				conversation, loadErr := s.conversationRepo.GetByID(ctx, workspaceID, attempt.SourceID, "", model.RoleOwner)
				if loadErr != nil {
					s.markCoverageV2AttemptRetryable(ctx, &attempt, owner, model.CoverageFailureCandidateQuery, loadErr)
					return
				}
				if conversation == nil {
					s.completeCoverageV2Attempt(ctx, &attempt, owner)
					return
				}
				if _, analyzeErr := s.runConversationCoverageAnalysis(ctx, workspaceID, attempt.BatchID, attempt.BatchID, *conversation, &attempt); analyzeErr != nil {
					slog.WarnContext(ctx, "queued coverage analysis failed", "workspace_id", workspaceID, "attempt_id", attempt.ID, "failure_class", coverageAnalysisFailureClass(analyzeErr))
				}
			}()
		}
		page.Wait()
		batchIDs := make(map[string]struct{}, len(attempts))
		for _, attempt := range attempts {
			batchIDs[attempt.BatchID] = struct{}{}
		}
		for batchID := range batchIDs {
			if err := s.coverageV2Repo.RefreshBatchStatus(ctx, batchID); err != nil {
				return err
			}
		}
	}
}

func (s *SupportCoverageDailyAnalyzer) coverageAssignmentEnabled(ctx context.Context, workspaceID string) bool {
	if s.rolloutPolicy == nil {
		return true
	}
	stats, err := s.coverageV2Repo.CoverageRolloutStats(ctx, workspaceID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		slog.WarnContext(ctx, "coverage rollout guardrail evaluation deferred assignment", "workspace_id", workspaceID, "error", err)
		return false
	}
	return s.rolloutPolicy.Evaluate(workspaceID, CoverageRolloutMetrics{
		AttemptCount: int(stats.AttemptCount), TerminalFailureCount: int(stats.TerminalFailureCount),
		DuplicateIdempotencyViolations: int(stats.DuplicateIdempotencyViolations), CrossWorkspaceInvariantViolations: int(stats.CrossWorkspaceInvariantViolations),
	}).AssignmentEnabled
}

func (s *SupportCoverageDailyAnalyzer) reconcileCoverageAssignments(ctx context.Context, workspaceID string) error {
	if s.coverageV2Repo == nil || !s.coverageAssignmentEnabled(ctx, workspaceID) {
		return nil
	}
	findings, err := s.coverageV2Repo.ClaimFindingsForAssignment(ctx, workspaceID, coverageAnalysisCandidatePageSize)
	if err != nil {
		return err
	}
	for index := range findings {
		finding := &findings[index]
		if err := assignCoverageV2Finding(ctx, s.coverageV2Repo, finding, s.jevDecisions); err != nil {
			_ = s.coverageV2Repo.UpdateFindingAssignmentStatus(ctx, workspaceID, finding.ID, "retryable")
			slog.WarnContext(ctx, "coverage topic assignment retry deferred", "workspace_id", workspaceID, "finding_id", finding.ID, "error", err)
		}
	}
	return nil
}

func (s *SupportCoverageDailyAnalyzer) RunWorkspaceDailyAnalysis(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time) error {
	if s == nil || s.analysisRepo == nil || s.coverageRepo == nil || s.conversationRepo == nil || s.messageRepo == nil {
		return fmt.Errorf("coverage daily analyzer dependencies are not configured")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if s.rolloutPolicy != nil && !s.rolloutPolicy.CaptureEnabled(workspaceID) {
		return nil
	}
	if err := s.reconcileQueuedCoverageAttempts(ctx, workspaceID); err != nil {
		return err
	}
	if err := s.reconcileCoverageAssignments(ctx, workspaceID); err != nil {
		return err
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
	if lastCursor != nil && coverageReanalysisRequest(ctx) == "" {
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
	v2BatchID := ""
	if s.coverageV2Repo != nil {
		batch, batchErr := s.coverageV2Repo.CreateBatch(ctx, &model.CoverageBatch{
			WorkspaceID: workspaceID, WindowStart: cursorStart, WindowEnd: cursorEnd,
			AnalyzerVersion: coverageAnalyzerVersion, PolicyVersion: "v1", Status: model.CoverageBatchRunning,
			CorrelationID: run.ID,
		})
		if batchErr != nil {
			s.markCoverageAnalysisRunFailed(ctx, run.ID, batchErr)
			return batchErr
		}
		v2BatchID = batch.ID
	}

	conversationCount := 0
	gapCount := 0
	var gapCountMu sync.Mutex
	var itemErrors []error
	var itemErrorsMu sync.Mutex
	var candidateCursor *repository.CoverageAnalysisCandidateCursor
	for {
		conversations, nextCursor, pageErr := s.conversationRepo.ListCoverageAnalysisCandidatesPage(ctx, workspaceID, cursorStart, cursorEnd, candidateCursor, coverageAnalysisCandidatePageSize)
		if pageErr != nil {
			s.markCoverageAnalysisRunFailed(ctx, run.ID, pageErr)
			return pageErr
		}
		conversationCount += len(conversations)
		var page sync.WaitGroup
		semaphore := make(chan struct{}, coverageAnalysisConversationConcurrency)
		for _, conversation := range conversations {
			conversation := conversation
			page.Add(1)
			go func() {
				defer page.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
				gapCreated, itemErr := s.runConversationCoverageAnalysis(ctx, workspaceID, run.ID, v2BatchID, conversation, nil)
				if itemErr != nil {
					itemErrorsMu.Lock()
					itemErrors = append(itemErrors, fmt.Errorf("conversation %s: %w", conversation.ID, itemErr))
					itemErrorsMu.Unlock()
					return
				}
				if gapCreated {
					gapCountMu.Lock()
					gapCount++
					gapCountMu.Unlock()
				}
			}()
		}
		page.Wait()
		if nextCursor == nil {
			break
		}
		candidateCursor = nextCursor
	}
	materialized, err := s.materializeRunFindings(ctx, workspaceID, run.ID)
	if err != nil {
		s.markCoverageAnalysisRunFailed(ctx, run.ID, err)
		return err
	}
	if materialized != nil && gapCount == 0 {
		gapCount = materialized.FindingsScanned
	}
	if len(itemErrors) > 0 {
		joined := errors.Join(itemErrors...)
		s.markCoverageAnalysisRunFailed(ctx, run.ID, joined)
		if s.coverageV2Repo != nil && v2BatchID != "" {
			_ = s.coverageV2Repo.CompleteBatch(ctx, v2BatchID, model.CoverageBatchPartialFailed, conversationCount, conversationCount-len(itemErrors), len(itemErrors), 0, model.CoverageFailurePersistence, sanitizeAIActionFailure(joined))
		}
		return joined
	}
	if err := s.analysisRepo.CompleteRun(ctx, run.ID, conversationCount, gapCount); err != nil {
		return err
	}
	if s.coverageV2Repo != nil && v2BatchID != "" {
		return s.coverageV2Repo.CompleteBatch(ctx, v2BatchID, model.CoverageBatchSucceeded, conversationCount, conversationCount, 0, 0, "", "")
	}
	return nil
}

func (s *SupportCoverageDailyAnalyzer) markCoverageAnalysisRunFailed(ctx context.Context, runID string, runErr error) {
	if s == nil || s.analysisRepo == nil {
		return
	}
	if err := s.analysisRepo.FailRun(ctx, runID, runErr); err != nil {
		slog.WarnContext(ctx, "failed to mark coverage analysis run failed", "error", err, "run_id", runID)
	}
}

func (s *SupportCoverageDailyAnalyzer) runConversationCoverageAnalysis(ctx context.Context, workspaceID, runID, v2BatchID string, conversation model.SupportConversation, claimedAttempt *model.CoverageAnalysisAttempt) (bool, error) {
	if claimedAttempt != nil {
		if _, requestID, ok := strings.Cut(claimedAttempt.AnalyzerVersion, ":reanalysis:"); ok {
			ctx = withCoverageReanalysis(ctx, requestID)
		}
	}
	v2Attempt := claimedAttempt
	attemptOwner := runID
	if claimedAttempt != nil {
		attemptOwner = claimedAttempt.LeaseOwner
	}
	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversation.ID, true)
	if err != nil {
		s.markCoverageV2AttemptRetryable(ctx, v2Attempt, attemptOwner, model.CoverageFailurePersistence, err)
		return false, err
	}
	traces, err := s.analysisRepo.ListRetrievalTracesByConversation(ctx, workspaceID, conversation.ID)
	if err != nil {
		s.markCoverageV2AttemptRetryable(ctx, v2Attempt, attemptOwner, model.CoverageFailurePersistence, err)
		return false, err
	}
	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, traces)
	if err != nil {
		if errors.Is(err, errCoverageNoPublicSegment) {
			s.completeCoverageV2Attempt(ctx, v2Attempt, attemptOwner)
			return false, nil
		}
		s.markCoverageV2AttemptRetryable(ctx, v2Attempt, attemptOwner, model.CoverageFailureCandidateQuery, err)
		return false, err
	}
	if v2Attempt == nil && s.coverageV2Repo != nil && v2BatchID != "" {
		logicalWorkKey := aiUsageIdempotencyKey(workspaceID, "coverage_work", conversation.ID, input.SegmentID, input.TranscriptHash, coverageAttemptAnalyzerVersion(ctx), "v1")
		v2Attempt, err = s.coverageV2Repo.StartAnalysisAttempt(ctx, v2BatchID, workspaceID, logicalWorkKey, "conversation", conversation.ID, input.SegmentID, input.TranscriptHash, coverageAttemptAnalyzerVersion(ctx), "v1", runID, 15*time.Minute)
		if err != nil {
			return false, err
		}
		if v2Attempt.Status == model.CoverageAttemptSucceeded {
			return false, nil
		}
		if v2Attempt.Status == model.CoverageAttemptLeased && v2Attempt.LeaseOwner != runID {
			return false, nil
		}
	}
	if claimedAttempt == nil && coverageReanalysisRequest(ctx) == "" {
		alreadyAnalyzed, err := s.analysisRepo.AlreadyAnalyzedConversation(ctx, workspaceID, conversation.ID, input.TranscriptHash, coverageAnalyzerVersion)
		if err != nil {
			return false, err
		}
		if alreadyAnalyzed {
			s.completeCoverageV2Attempt(ctx, v2Attempt, attemptOwner)
			return false, nil
		}
	}

	// Deterministic prefilter: skip obvious non-support conversations without LLM.
	if localClass, skip := classifyCoverageConversationLocally(input); skip {
		classPayload, _ := json.Marshal(localClass)
		if err := s.recordCoverageConversationAnalysis(ctx, &model.SupportCoverageConversationAnalysis{
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
		s.completeCoverageV2Attempt(ctx, v2Attempt, attemptOwner)
		return false, nil
	}

	result, raw, err := s.AnalyzeConversation(ctx, input)
	if err != nil {
		analysisErr := err.Error()
		recordErr := s.recordCoverageConversationAnalysis(ctx, &model.SupportCoverageConversationAnalysis{
			WorkspaceID:     workspaceID,
			RunID:           runID,
			ConversationID:  conversation.ID,
			Status:          model.SupportCoverageConversationAnalysisStatusFailed,
			TranscriptHash:  input.TranscriptHash,
			AnalyzerVersion: coverageAnalyzerVersion,
			ErrorMessage:    &analysisErr,
			RawOutput:       []byte("{}"),
		})
		if v2Attempt != nil {
			failureClass := coverageAnalysisFailureClass(err)
			s.markCoverageV2AttemptRetryable(ctx, v2Attempt, attemptOwner, failureClass, err)
		}
		if recordErr != nil {
			return false, recordErr
		}
		return false, err
	}
	if result == nil {
		s.completeCoverageV2Attempt(ctx, v2Attempt, attemptOwner)
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
	if result.HasGap && (result.ShouldRunRetrieval || coverageReanalysisRequest(ctx) != "") {
		currentMatches, err := s.matchCurrentKnowledgeForAnalysis(ctx, workspaceID, *result)
		if err != nil {
			return false, err
		}
		if len(currentMatches) > 0 {
			matchedKnowledge = currentMatches
			refined, err := s.RefineFixBundleWithKnowledge(WithAIUsageMetering(ctx, AIUsageMeteringContext{
				WorkspaceID:    workspaceID,
				FeatureKey:     BillingFeatureCoverageGapAnalysis,
				IdempotencyKey: aiUsageIdempotencyKey(workspaceID, BillingFeatureCoverageGapAnalysis, "refine", conversation.ID, input.TranscriptHash, coverageReanalysisRequest(ctx)),
				Metadata: map[string]interface{}{
					"conversation_id": conversation.ID,
					"action":          "refine",
				},
			}), *result, currentMatches)
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
	if err := s.recordCoverageConversationAnalysis(ctx, analysis); err != nil {
		return false, err
	}
	if result.HasGap && v2Attempt != nil {
		if err := s.persistCoverageV2Finding(ctx, v2Attempt, conversation, input, *result); err != nil {
			s.markCoverageV2AttemptRetryable(ctx, v2Attempt, attemptOwner, model.CoverageFailurePersistence, err)
			return false, err
		}
	}
	s.completeCoverageV2Attempt(ctx, v2Attempt, attemptOwner)
	return result.HasGap, nil
}

func coverageAnalysisFailureClass(err error) string {
	if errors.Is(err, errCoverageLLMContract) {
		return model.CoverageFailureLLMContract
	}
	if strings.Contains(strings.ToLower(err.Error()), "not configured") {
		return model.CoverageFailureConfiguration
	}
	return model.CoverageFailureLLMProvider
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
			ID:              message.ID,
			SenderType:      strings.TrimSpace(message.SenderType),
			MessageType:     strings.TrimSpace(message.MessageType),
			Content:         truncateCoverageAnalysisContent(normalizeCoverageTranscriptContent(message.Content), coverageAnalysisMaxMessageChars),
			CreatedAt:       message.CreatedAt.UTC(),
			VisitorFeedback: message.VisitorFeedback(),
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
	classification := s.classifyCoverageWithJev(ctx, input)
	schema, prompt := classification.constrain(coverageConversationAnalysisJSONSchema(), coverageConversationAnalysisSystemPrompt())
	generationKey := aiUsageIdempotencyKey(input.WorkspaceID, BillingFeatureCoverageGapAnalysis, "analyze", input.ConversationID, input.TranscriptHash, coverageReanalysisRequest(ctx))
	if classification != nil {
		generationKey = aiUsageIdempotencyKey(generationKey, classification.assessmentID)
	}
	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID:    input.WorkspaceID,
		ActionKey:      aipolicy.ActionSupportCoverageAnalyze,
		FeatureKey:     BillingFeatureCoverageGapAnalysis,
		IdempotencyKey: generationKey,
		Metadata: map[string]interface{}{
			"conversation_id": input.ConversationID,
			"action":          "analyze",
		},
		Chat: llm.ChatRequest{
			SystemPrompt: prompt,
			Messages: []llm.Message{{
				Role:    "user",
				Content: string(inputJSON),
			}},
			Temperature: 0.1,
			MaxTokens:   1800,
			JSONMode:    true,
			JSONSchema:  schema,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("coverage conversation analyzer llm: %w", err)
	}

	var result CoverageConversationAnalysisResult
	if err := llm.UnmarshalResponse(resp.Content, &result); err != nil {
		return nil, nil, fmt.Errorf("parse coverage conversation analyzer response: %w", err)
	}
	normalizeCoverageConversationAnalysisResult(&result)
	if err := classification.validate(result); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", errCoverageLLMContract, err)
	}
	if err := validateActionableCoverageResult(result); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", errCoverageLLMContract, err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal normalized analyzer result: %w", err)
	}
	return &result, raw, nil
}

func validateActionableCoverageResult(result CoverageConversationAnalysisResult) error {
	if !result.HasGap {
		return nil
	}
	if result.CustomerNeed == "" {
		return fmt.Errorf("customer_need is required")
	}
	if result.AIFailure == "" {
		return fmt.Errorf("ai_failure is required")
	}
	if result.Confidence <= 0 || result.Confidence > 1 {
		return fmt.Errorf("confidence must be between zero and one")
	}
	if len(result.RecommendedFixes) == 0 {
		return fmt.Errorf("at least one recommended fix is required")
	}
	for index, fix := range result.RecommendedFixes {
		if fix.Type == "" || firstNonEmptyCoverageString(fix.TargetTitle, fix.TargetID, fix.TargetURL, fix.TargetType) == "" || fix.Rationale == "" || fix.SuggestedChange == "" {
			return fmt.Errorf("recommended_fixes[%d] requires type, target, rationale, and suggested_change", index)
		}
	}
	return nil
}

func (s *SupportCoverageDailyAnalyzer) completeCoverageV2Attempt(ctx context.Context, attempt *model.CoverageAnalysisAttempt, owner string) {
	if s == nil || s.coverageV2Repo == nil || attempt == nil || attempt.Status == model.CoverageAttemptSucceeded {
		return
	}
	if err := s.coverageV2Repo.CompleteAttempt(ctx, attempt.ID, owner, time.Now().UTC()); err != nil {
		slog.WarnContext(ctx, "failed to complete coverage v2 attempt", "attempt_id", attempt.ID, "error", err)
	}
}

func (s *SupportCoverageDailyAnalyzer) markCoverageV2AttemptRetryable(ctx context.Context, attempt *model.CoverageAnalysisAttempt, owner, failureClass string, attemptErr error) {
	if s == nil || s.coverageV2Repo == nil || attempt == nil || attempt.Status == model.CoverageAttemptSucceeded {
		return
	}
	next, transitionErr := TransitionCoverageAttempt(CoverageAttemptState{
		Status:          model.CoverageAttemptLeased,
		RetryBudgetUsed: attempt.RetryBudgetUsed,
		FailureClass:    attempt.FailureClass,
	}, CoverageAttemptEvent{Kind: CoverageEventFailure, FailureClass: failureClass, MaxRetries: 3})
	if transitionErr != nil {
		slog.WarnContext(ctx, "failed to transition coverage v2 attempt", "attempt_id", attempt.ID, "error", transitionErr)
		return
	}
	var retryAt *time.Time
	if next.Status == model.CoverageAttemptRetryable {
		at := time.Now().UTC().Add(time.Minute)
		retryAt = &at
	}
	if err := s.coverageV2Repo.FailAttempt(ctx, attempt.ID, owner, next.Status, failureClass, sanitizeAIActionFailure(attemptErr), retryAt, next.RetryBudgetUsed); err != nil && !errors.Is(err, repository.ErrCoverageLeaseLost) {
		slog.WarnContext(ctx, "failed to mark coverage v2 attempt retryable", "attempt_id", attempt.ID, "error", err)
	}
}

func (s *SupportCoverageDailyAnalyzer) persistCoverageV2Finding(ctx context.Context, attempt *model.CoverageAnalysisAttempt, conversation model.SupportConversation, input CoverageConversationAnalysisInput, result CoverageConversationAnalysisResult) error {
	if s == nil || s.coverageV2Repo == nil || attempt == nil {
		return nil
	}
	fix := result.RecommendedFixes[0]
	aiAnswer, humanAnswer := coverageTranscriptAnswers(input.Messages)
	finding := &model.CoverageFinding{
		WorkspaceID: attempt.WorkspaceID, LogicalWorkKey: aiUsageIdempotencyKey(attempt.WorkspaceID, "coverage_work", conversation.ID, input.SegmentID, input.TranscriptHash, coverageAnalyzerVersion, "v1"), AnalysisAttemptID: attempt.ID,
		SourceKind: attempt.SourceKind, SourceID: attempt.SourceID, ConversationID: &conversation.ID,
		CustomerID: conversation.CRMContactID, CustomerNeed: result.CustomerNeed, AIAnswer: aiAnswer,
		AIFailure: result.AIFailure, HumanAnswer: firstNonEmptyCoverageString(humanAnswer, result.HumanResolution),
		FixType: fix.Type, FixTarget: firstNonEmptyCoverageString(fix.TargetTitle, fix.TargetID, fix.TargetURL, fix.TargetType),
		Rationale: fix.Rationale, SuggestedChange: fix.SuggestedChange, Confidence: result.Confidence,
		EmbeddingStatus: "pending", AssignmentStatus: "pending", Metadata: []byte("{}"),
	}
	if coverageReanalysisRequest(ctx) != "" {
		reviewed, err := s.coverageV2Repo.ReplaceCurrentFindingPreservingReview(ctx, finding)
		if err != nil {
			return err
		}
		if reviewed {
			return nil
		}
	} else if err := s.coverageV2Repo.ReplaceCurrentFinding(ctx, finding); err != nil {
		return err
	}
	if !s.coverageAssignmentEnabled(ctx, finding.WorkspaceID) {
		return nil
	}
	// The durable finding is committed first. Assignment is independently
	// retryable and must never erase or roll back the analysis result.
	if err := assignCoverageV2Finding(ctx, s.coverageV2Repo, finding, s.jevDecisions); err != nil {
		_ = s.coverageV2Repo.UpdateFindingAssignmentStatus(ctx, finding.WorkspaceID, finding.ID, "retryable")
		slog.WarnContext(ctx, "coverage topic assignment deferred", "finding_id", finding.ID, "error", err)
	}
	return nil
}

func coverageTranscriptAnswers(messages []CoverageConversationMessage) (string, string) {
	var aiAnswer, humanAnswer string
	for _, message := range messages {
		switch strings.ToLower(strings.TrimSpace(message.SenderType)) {
		case "ai":
			aiAnswer = strings.TrimSpace(message.Content)
		case "user", "agent", "human", "teammate":
			humanAnswer = strings.TrimSpace(message.Content)
		}
	}
	return aiAnswer, humanAnswer
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
	metering, _ := AIUsageMeteringFromContext(ctx)
	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID: metering.WorkspaceID, ActionKey: aipolicy.ActionSupportCoverageRefine,
		FeatureKey:     BillingFeatureCoverageGapAnalysis,
		IdempotencyKey: metering.IdempotencyKey, Metadata: metering.Metadata,
		Chat: llm.ChatRequest{
			SystemPrompt: coverageFixBundleRefinementSystemPrompt(),
			Messages: []llm.Message{{
				Role:    "user",
				Content: string(payloadJSON),
			}},
			Temperature: 0.1,
			MaxTokens:   1400,
			JSONMode:    true,
			JSONSchema:  coverageFixBundleDecisionJSONSchema(),
		},
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

Visitor feedback is a satisfaction signal attached to a specific AI answer. A thumbs-down alone is not proof of missing or incorrect knowledge: distinguish answer quality, retrieval, account context, unavailable actions, policy, and product limitations using the transcript and traces. A thumbs-up is not proof of factual correctness or resolution. Never create or close a gap solely because of a vote.

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
