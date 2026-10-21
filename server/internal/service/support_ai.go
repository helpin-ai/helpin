package service

import (
	"context"

	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// AIRequestEvent is the NATS message payload for AI processing requests.
type AIRequestEvent struct {
	WorkspaceID    string `json:"workspace_id"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	Content        string `json:"content"`
}

// AIResponseContract is the structured JSON output expected from the LLM.
type AIResponseContract struct {
	Content          string              `json:"content"`
	CanAnswer        bool                `json:"can_answer"`
	SourceDocIDs     []string            `json:"source_doc_ids"`
	Confidence       float64             `json:"confidence"`
	Claims           []AIResponseClaim   `json:"claims"`
	EvidenceCoverage map[string][]string `json:"evidence_coverage"`
}

// AIResponseClaim maps one material generated claim to retrieved evidence IDs.
type AIResponseClaim struct {
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type SupportQueryPlanContract struct {
	Route              string   `json:"route"`
	Reply              string   `json:"reply"`
	Intent             string   `json:"intent"`
	Subject            string   `json:"subject"`
	Language           string   `json:"language"`
	Risk               string   `json:"risk"`
	RequiredEvidence   []string `json:"required_evidence"`
	EvidenceMode       string   `json:"evidence_mode"`
	RegistryVersion    int      `json:"registry_version"`
	ContextAction      string   `json:"context_action"`
	Decision           string   `json:"decision"`
	IssueKey           string   `json:"issue_key"`
	IssueSummary       string   `json:"issue_summary"`
	ProgressSignal     string   `json:"progress_signal"`
	StandaloneQuery    string   `json:"standalone_query"`
	SearchQueries      []string `json:"search_queries"`
	ClarifyingQuestion string   `json:"clarifying_question"`
	GreetingReply      string   `json:"greeting_reply"`
	Reason             string   `json:"reason"`
}

// AIMessageMetadata is stored in the SupportMessage.Metadata JSONB field.
type AIMessageMetadata struct {
	AIAutoReply         bool                `json:"ai_auto_reply"`
	AISources           []AISource          `json:"ai_sources"`
	AIConfidence        float64             `json:"ai_confidence"`
	AIModel             string              `json:"ai_model"`
	AITokensUsed        int                 `json:"ai_tokens_used"`
	AIAgentID           string              `json:"ai_agent_id"`
	AIReplyKind         string              `json:"ai_reply_kind,omitempty"`
	AIIssueKey          string              `json:"ai_issue_key,omitempty"`
	AIIssueSummary      string              `json:"ai_issue_summary,omitempty"`
	AIProgressState     string              `json:"ai_progress_state,omitempty"`
	AIPreRoute          string              `json:"ai_pre_route,omitempty"`
	AIIntent            string              `json:"ai_intent,omitempty"`
	AISubject           string              `json:"ai_subject,omitempty"`
	AILanguage          string              `json:"ai_language,omitempty"`
	AIRequiredEvidence  []string            `json:"ai_required_evidence,omitempty"`
	AIEvidenceFound     map[string][]string `json:"ai_evidence_found,omitempty"`
	AIEvidenceMissing   []string            `json:"ai_evidence_missing,omitempty"`
	AIValidationOutcome string              `json:"ai_validation_outcome,omitempty"`
	AIValidationReasons []string            `json:"ai_validation_reasons,omitempty"`
	AIStageLatencyMS    map[string]int64    `json:"ai_stage_latency_ms,omitempty"`
}

// AISource is a single source citation in AI message metadata.
type AISource struct {
	DocID      string  `json:"docId"`
	BlockID    string  `json:"blockId,omitempty"`
	Title      string  `json:"title"`
	Snippet    string  `json:"snippet"`
	Confidence float64 `json:"confidence"`
	SourceType string  `json:"sourceType,omitempty"`
	URL        string  `json:"url,omitempty"`
}

type KnowledgeSearchResult struct {
	ID            string
	ReferenceID   string
	SourceType    string
	IsInternal    bool
	DocumentID    string
	BlockID       string
	SourceID      string
	ChunkIndex    int
	SectionKey    string
	HeadingPath   string
	Title         string
	URL           string
	Content       string
	LexicalScore  float64
	VectorScore   float64
	CombinedScore float64
}

const (
	knowledgeSourceTypeDocs     = "docs"
	knowledgeSourceTypeContent  = "content"
	knowledgeSourceTypeGuidance = "curated_guidance"
	helpinAIDisplayName         = "Helpin AI"
	supportDecisionAnswer       = "answer"

	supportReplyKindAnswer   = "answer"
	supportReplyKindClarify  = "clarify"
	supportReplyKindGreeting = "greeting"
	supportReplyKindConfirm  = "confirmation"

	supportStateProgressing = "progressing"

	// Direct support assistance is metered as a small-tier task. Keep these
	// defaults aligned with the curated pricing catalog so preflight cannot
	// reject grammar rewrites before the provider call is made.
	supportSmallTierProvider = "openai"
	supportSmallTierModel    = "gpt-5.6-luna"
	supportRewriteProvider   = supportSmallTierProvider
	supportRewriteModel      = supportSmallTierModel
	supportRewriteExpand     = "expand"
	supportRewriteRephrase   = "rephrase"
	supportRewriteFixGrammar = "fix_grammar"
	supportRewriteFriendly   = "more_friendly"
	supportRewriteFormal     = "more_formal"
)

var (
	ErrSupportPreviewInvalidInput         = errors.New("invalid support preview input")
	ErrSupportPreviewAgentNotFound        = errors.New("support preview agent not found")
	ErrSupportPreviewConversationNotFound = errors.New("support preview conversation not found")
	ErrSupportRewriteInvalidInput         = errors.New("invalid support rewrite input")
	ErrSupportRewriteConversationNotFound = errors.New("support rewrite conversation not found")
)

// SupportAIService provides Helpin-owned knowledge retrieval, reply publication,
// handoff, and composer assistance. SupportChatService delegates autonomous
// conversation execution to Agent Runtime.
type SupportAIService struct {
	runCloser                      supportChatRunCloser
	llmProvider                    llm.Provider
	embeddingProvider              llm.EmbeddingProvider
	embeddingModel                 string
	queryExpansionModel            string
	queryExpansionProvider         string
	queryExpansionTimeout          time.Duration
	docsChunkRepo                  *repository.DocsChunkRepository
	knowledgeRepo                  *repository.AgentKnowledgeSourceRepository
	contentChunkRepo               *repository.SupportContentChunkRepository
	contentLinkRepo                *repository.AgentContentSourceRepository
	curatedGuidanceRepo            *repository.CuratedGuidanceRepository
	knowledgeReranker              SupportKnowledgeReranker
	processingRepo                 *repository.AIMessageProcessingRepository
	conversationRepo               *repository.SupportConversationRepository
	messageRepo                    *repository.SupportMessageRepository
	attachmentRepo                 *repository.SupportAttachmentRepository
	agentRepo                      *repository.AgentRepository
	handoffRepo                    *repository.AgentHandoffRepository
	installationRepo               *repository.SupportInboxInstallationRepository
	mailboxRepo                    *repository.SupportMailboxRepository
	workspaceRepo                  *repository.WorkspaceRepository
	statusOverrideRepo             *repository.SupportTeammateStatusOverrideRepository
	triageService                  *SupportInboxTriageService
	linkPreviewService             SupportMessageLinkPreviewer
	assignmentSystemMessageEmitter func(ctx context.Context, workspaceID, conversationID, targetUserID string)
	wsPublisher                    *websocket.Publisher
	presence                       websocket.PresenceProvider
	js                             nats.JetStreamContext
	redis                          *redis.Client
	db                             *gorm.DB
	supportEventRecorder           SupportEventRecorder
	traceRecorder                  SupportAIRetrievalTraceRecorder
	portalReplyNotifier            *EmailFallbackService
}

// SetPortalReplyNotifier uses the same delivery policy as teammate replies.
func (s *SupportAIService) SetPortalReplyNotifier(notifier *EmailFallbackService) {
	s.portalReplyNotifier = notifier
}

// NewSupportAIService creates a new SupportAIService with all dependencies.
func NewSupportAIService(
	llmProvider llm.Provider,
	embeddingProvider llm.EmbeddingProvider,
	embeddingModel string,
	docsChunkRepo *repository.DocsChunkRepository,
	knowledgeRepo *repository.AgentKnowledgeSourceRepository,
	contentChunkRepo *repository.SupportContentChunkRepository,
	contentLinkRepo *repository.AgentContentSourceRepository,
	processingRepo *repository.AIMessageProcessingRepository,
	conversationRepo *repository.SupportConversationRepository,
	messageRepo *repository.SupportMessageRepository,
	attachmentRepo *repository.SupportAttachmentRepository,
	agentRepo *repository.AgentRepository,
	handoffRepo *repository.AgentHandoffRepository,
	installationRepo *repository.SupportInboxInstallationRepository,
	wsPublisher *websocket.Publisher,
	js nats.JetStreamContext,
	redisClient *redis.Client,
	db *gorm.DB,
	queryExpansionModel string,
	queryExpansionProvider string,
) *SupportAIService {
	return &SupportAIService{
		llmProvider:            llmProvider,
		embeddingProvider:      embeddingProvider,
		embeddingModel:         strings.TrimSpace(embeddingModel),
		queryExpansionModel:    queryExpansionModel,
		queryExpansionProvider: queryExpansionProvider,
		docsChunkRepo:          docsChunkRepo,
		knowledgeRepo:          knowledgeRepo,
		contentChunkRepo:       contentChunkRepo,
		contentLinkRepo:        contentLinkRepo,
		processingRepo:         processingRepo,
		conversationRepo:       conversationRepo,
		messageRepo:            messageRepo,
		attachmentRepo:         attachmentRepo,
		agentRepo:              agentRepo,
		handoffRepo:            handoffRepo,
		installationRepo:       installationRepo,
		wsPublisher:            wsPublisher,
		js:                     js,
		redis:                  redisClient,
		db:                     db,
	}
}

// SetQueryExpansionTimeout sets the maximum duration of the support query-planning call.
func (s *SupportAIService) SetQueryExpansionTimeout(timeout time.Duration) {
	if s == nil || timeout <= 0 {
		return
	}
	s.queryExpansionTimeout = timeout
}

func (s *SupportAIService) SetSupportRoutingDependencies(
	workspaceRepo *repository.WorkspaceRepository,
	presence websocket.PresenceProvider,
	statusOverrideRepo *repository.SupportTeammateStatusOverrideRepository,
) *SupportAIService {
	if s == nil {
		return nil
	}
	s.workspaceRepo = workspaceRepo
	s.presence = presence
	s.statusOverrideRepo = statusOverrideRepo
	return s
}

func (s *SupportAIService) SetMailboxRepository(mailboxRepo *repository.SupportMailboxRepository) *SupportAIService {
	if s == nil {
		return nil
	}
	s.mailboxRepo = mailboxRepo
	return s
}

// SetCuratedGuidanceRepository enables first-class pinned-answer retrieval.
func (s *SupportAIService) SetCuratedGuidanceRepository(repo *repository.CuratedGuidanceRepository) *SupportAIService {
	if s == nil {
		return nil
	}
	s.curatedGuidanceRepo = repo
	return s
}

// SetKnowledgeReranker enables the fixed-cost semantic reranking stage.
func (s *SupportAIService) SetKnowledgeReranker(reranker SupportKnowledgeReranker) *SupportAIService {
	s.knowledgeReranker = reranker
	return s
}

// SetSupportEventRecorder injects the event recorder for coverage telemetry.
func (s *SupportAIService) SetSupportEventRecorder(r SupportEventRecorder) {
	if s == nil {
		return
	}
	s.supportEventRecorder = r
}

func (s *SupportAIService) SetSupportAIRetrievalTraceRecorder(r SupportAIRetrievalTraceRecorder) *SupportAIService {
	if s == nil {
		return nil
	}
	s.traceRecorder = r
	return s
}

func (s *SupportAIService) recordSupportEvent(input SupportEventInput) {
	if s.supportEventRecorder == nil {
		return
	}
	s.supportEventRecorder.RecordEventBestEffort(input)
}

func (s *SupportAIService) recordSupportAIRetrievalTraceBestEffort(trace *model.SupportAIRetrievalTrace) {
	if s == nil || s.traceRecorder == nil || trace == nil {
		return
	}
	traceCopy := *trace
	go func(trace model.SupportAIRetrievalTrace) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.traceRecorder.RecordSupportAIRetrievalTrace(ctx, &trace); err != nil {
			slog.WarnContext(ctx, "record support AI retrieval trace failed",
				"error", err,
				"workspace_id", trace.WorkspaceID,
				"conversation_id", trace.ConversationID,
				"message_id", trace.MessageID,
			)
		}
	}(traceCopy)
}

func (s *SupportAIService) recordSupportAIAnswerTrace(
	ctx context.Context,
	workspaceID string,
	conversationID string,
	answerMessageID string,
	triggerMessageID string,
	agentID string,
	queryPlan SupportQueryPlanContract,
	searchResults []KnowledgeSearchResult,
	response *AIResponseContract,
	evidenceCoverage supportEvidenceCoverage,
	answerValidation supportAnswerValidation,
	confidence float64,
	replyKind string,
	progressState string,
	retrievalRetried bool,
	stageLatencies map[string]int64,
) {
	canAnswer := "true"
	canResolve := "true"
	trace, err := BuildSupportAIRetrievalTrace(SupportAIRetrievalTraceInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		MessageID:      answerMessageID,
		SearchQueries:  queryPlan.SearchQueries,
		SearchResults:  searchResults,
		CitedSourceIDs: response.SourceDocIDs,
		AIConfidence:   confidence,
		CanAnswer:      &canAnswer,
		CanResolve:     &canResolve,
		Metadata: map[string]any{
			"agent_id":           agentID,
			"trigger_message_id": triggerMessageID,
			"reply_kind":         replyKind,
			"issue_key":          queryPlan.IssueKey,
			"progress_state":     progressState,
			"route":              queryPlan.Route,
			"intent":             queryPlan.Intent,
			"subject":            queryPlan.Subject,
			"language":           queryPlan.Language,
			"required_evidence":  queryPlan.RequiredEvidence,
			"evidence_found":     evidenceCoverage.Found,
			"evidence_missing":   evidenceCoverage.Missing,
			"validation_outcome": answerValidation.Outcome,
			"validation_reasons": answerValidation.Reasons,
			"retrieval_retried":  retrievalRetried,
			"stage_latency_ms":   stageLatencies,
		},
	})
	if err != nil {
		slog.WarnContext(ctx, "build support AI retrieval trace failed",
			"error", err,
			"workspace_id", workspaceID,
			"conversation_id", conversationID,
			"message_id", answerMessageID,
		)
		return
	}
	s.recordSupportAIRetrievalTraceBestEffort(trace)
}

func (s *SupportAIService) SetTriageService(triageService *SupportInboxTriageService) *SupportAIService {
	if s == nil {
		return nil
	}
	s.triageService = triageService
	return s
}

// SetLinkPreviewService injects the support message link preview enricher.
func (s *SupportAIService) SetLinkPreviewService(linkPreviewService SupportMessageLinkPreviewer) *SupportAIService {
	if s == nil {
		return nil
	}
	s.linkPreviewService = linkPreviewService
	return s
}
