package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

const maxCommandBarPlanSteps = 50
const maxCommandBarDAGInitialFanOut = 10
const defaultCommandRouterMaxTokens = 900
const defaultCommandRouterTimeout = 2500 * time.Millisecond
const minOpenRouterCommandRouterTimeout = 8 * time.Second
const commandBarPlannerValidationReasonPrefix = "Command Agent planner output invalid: "

var commandBarTaskKeyPattern = regexp.MustCompile(`\b([A-Z][A-Z0-9]{1,11})-(\d{1,9})\b`)
var commandBarTypedUUIDPattern = regexp.MustCompile(`(?i)\b(document|doc|task|story|epic|deal|crm deal|crm_deal|contact|crm contact|crm_contact|repository|repo|git repo|git_repository|git_repo)\s+(?:id|uuid)\s*[:#-]?\s*([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\b`)

type commandBarTriggerContextPayload struct {
	PlanID      string                      `json:"plan_id,omitempty"`
	Prompt      string                      `json:"prompt,omitempty"`
	PageContext model.CommandBarPageContext `json:"page_context"`
	Steps       []model.CommandBarPlanStep  `json:"steps"`
	RunCount    int                         `json:"run_count"`
	StepIndex   int                         `json:"step_index"`
}

type CommandBarChatAccess struct {
	CanReadPM   bool
	CanReadDocs bool
	CanReadCRM  bool
	ActorRole   string
}

type commandBarReadOnlyToolCall struct {
	Tool   string          `json:"tool"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type commandBarReadOnlyToolContext struct {
	Mode           string                       `json:"mode"`
	ToolCalls      []commandBarReadOnlyToolCall `json:"tool_calls,omitempty"`
	WorkingContext *commandBarWorkingContext    `json:"working_context,omitempty"`
}

type commandBarReadOnlyToolTurn struct {
	Type   string          `json:"type"`
	Tool   string          `json:"tool,omitempty"`
	Input  json.RawMessage `json:"input,omitempty"`
	Answer string          `json:"answer,omitempty"`
	Reason string          `json:"reason,omitempty"`
}

type commandBarChatIntentClassification struct {
	Route      string  `json:"route"`
	Reason     string  `json:"reason,omitempty"`
	Answer     string  `json:"answer,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type commandBarWorkingContext struct {
	ReferencedEntities []commandBarWorkingEntityRef `json:"referenced_entities,omitempty"`
	ResultSets         []commandBarWorkingResultSet `json:"result_sets,omitempty"`
	ActiveScope        *commandBarWorkingScope      `json:"active_scope,omitempty"`
}

type commandBarWorkingEntityRef struct {
	Type       string         `json:"type"`
	ID         string         `json:"id,omitempty"`
	Key        string         `json:"key,omitempty"`
	Title      string         `json:"title,omitempty"`
	Status     string         `json:"status,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type commandBarWorkingResultSet struct {
	SourceTool  string                       `json:"source_tool"`
	EntityType  string                       `json:"entity_type,omitempty"`
	Filters     map[string]any               `json:"filters,omitempty"`
	Total       *int64                       `json:"total,omitempty"`
	Returned    int                          `json:"returned,omitempty"`
	EntityRefs  []commandBarWorkingEntityRef `json:"entity_refs,omitempty"`
	Description string                       `json:"description,omitempty"`
}

type commandBarWorkingScope struct {
	SourceTool  string         `json:"source_tool,omitempty"`
	EntityType  string         `json:"entity_type,omitempty"`
	Filters     map[string]any `json:"filters,omitempty"`
	Total       *int64         `json:"total,omitempty"`
	Description string         `json:"description,omitempty"`
}

type commandBarChatClassifierToolCard struct {
	Name        string `json:"name"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
}

type commandBarChatClassifierAgentCard struct {
	Name           string   `json:"name"`
	PresetKey      string   `json:"preset_key,omitempty"`
	Description    string   `json:"description,omitempty"`
	AllowedTargets []string `json:"allowed_targets,omitempty"`
	OneShot        bool     `json:"one_shot,omitempty"`
}

type commandBarChatClassifierTargetResolution struct {
	CurrentTarget       model.CommandBarPageContext  `json:"current_target"`
	PriorTargetState    string                       `json:"prior_target_state"`
	InferredPriorTarget *model.CommandBarPageContext `json:"inferred_prior_target,omitempty"`
	CandidateTargets    []commandBarWorkingEntityRef `json:"candidate_targets,omitempty"`
	Instruction         string                       `json:"instruction,omitempty"`
}

type commandBarRepositoryChoiceContext struct {
	Mode         string                       `json:"mode"`
	Repositories []commandBarRepositoryChoice `json:"repositories"`
}

type commandBarRepositoryChoice struct {
	ID            string `json:"id"`
	FullName      string `json:"full_name"`
	Provider      string `json:"provider,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

type CommandBarService struct {
	agentService                           *AgentService
	planRepo                               *repository.CommandBarPlanRepository
	unmetRepo                              *repository.CommandBarUnmetIntentRepository
	dismissalRepo                          *repository.CommandBarPlanDismissalRepository
	chatRepo                               *repository.CommandBarChatRepository
	commandService                         *InternalCommandService
	docsDocumentService                    *DocsDocumentService
	crmDealService                         *CRMDealService
	crmContactService                      *CRMContactService
	crmCompanyService                      *CRMCompanyService
	llmProvider                            llm.Provider
	commandRouterLLMProvider               string
	commandRouterLLMModel                  string
	commandRouterLLMMaxTokens              int
	commandRouterLLMTimeout                time.Duration
	commandRouterOpenRouterProviderOptions json.RawMessage
}

func NewCommandBarService(agentService *AgentService, planRepo *repository.CommandBarPlanRepository, unmetRepo *repository.CommandBarUnmetIntentRepository, dismissalRepo *repository.CommandBarPlanDismissalRepository, llmProvider llm.Provider) *CommandBarService {
	return &CommandBarService{
		agentService:              agentService,
		planRepo:                  planRepo,
		unmetRepo:                 unmetRepo,
		dismissalRepo:             dismissalRepo,
		llmProvider:               llmProvider,
		commandRouterLLMMaxTokens: defaultCommandRouterMaxTokens,
		commandRouterLLMTimeout:   defaultCommandRouterTimeout,
	}
}

func (s *CommandBarService) SetLLMRouterConfig(provider, modelName string, maxTokens int, timeout time.Duration) *CommandBarService {
	if s == nil {
		return s
	}
	s.commandRouterLLMProvider = strings.TrimSpace(provider)
	s.commandRouterLLMModel = strings.TrimSpace(modelName)
	if maxTokens > 0 {
		s.commandRouterLLMMaxTokens = maxTokens
	}
	if timeout > 0 {
		s.commandRouterLLMTimeout = timeout
	}
	return s
}

func (s *CommandBarService) SetCommandRouterOpenRouterProviderOptions(options json.RawMessage) *CommandBarService {
	if s != nil {
		s.commandRouterOpenRouterProviderOptions = append(json.RawMessage(nil), options...)
	}
	return s
}

func (s *CommandBarService) SetChatRepository(repo *repository.CommandBarChatRepository) *CommandBarService {
	if s != nil {
		s.chatRepo = repo
	}
	return s
}

func (s *CommandBarService) SetInternalCommandService(commandService *InternalCommandService) *CommandBarService {
	if s != nil {
		s.commandService = commandService
	}
	return s
}

func (s *CommandBarService) SetReadOnlyDataServices(docs *DocsDocumentService, deals *CRMDealService, contacts *CRMContactService, companies *CRMCompanyService) *CommandBarService {
	if s != nil {
		s.docsDocumentService = docs
		s.crmDealService = deals
		s.crmContactService = contacts
		s.crmCompanyService = companies
	}
	return s
}

func (s *CommandBarService) ChatTurn(ctx context.Context, workspaceID, actorID string, req model.CommandBarChatTurnRequest) (*model.CommandBarChatTurnResponse, error) {
	return s.ChatTurnWithAccess(ctx, workspaceID, actorID, req, fullCommandBarChatAccess())
}

func (s *CommandBarService) ChatTurnWithAccess(ctx context.Context, workspaceID, actorID string, req model.CommandBarChatTurnRequest, access CommandBarChatAccess) (*model.CommandBarChatTurnResponse, error) {
	if s == nil || s.chatRepo == nil {
		return nil, fmt.Errorf("command bar chat service is not configured")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	pageContext := normalizeCommandBarPageContext(req.PageContext, workspaceID)
	if err := validateCommandBarSupportedTarget(pageContext.EntityType); err != nil {
		return nil, err
	}

	thread, err := s.commandBarThreadForTurn(ctx, workspaceID, actorID, req.ThreadID, text)
	if err != nil {
		return nil, err
	}
	history, err := s.chatRepo.ListRecentMessages(ctx, workspaceID, thread.ID, 10)
	if err != nil {
		return nil, err
	}
	pageContextJSON, _ := json.Marshal(pageContext)
	userMessage := model.CommandBarMessage{
		ID:          uuid.NewString(),
		ThreadID:    thread.ID,
		WorkspaceID: workspaceID,
		ActorID:     optionalActorID(actorID),
		Role:        model.CommandBarMessageRoleUser,
		Content:     text,
		PageContext: pageContextJSON,
	}
	if err := s.chatRepo.CreateMessage(ctx, &userMessage); err != nil {
		return nil, err
	}

	proposal, content, err := s.commandBarChatProposal(ctx, workspaceID, actorID, text, pageContext, access, history)
	if err != nil {
		return nil, err
	}
	proposalJSON, _ := json.Marshal(proposal)
	assistantMessage := model.CommandBarMessage{
		ID:           uuid.NewString(),
		ThreadID:     thread.ID,
		WorkspaceID:  workspaceID,
		ActorID:      optionalActorID(actorID),
		Role:         model.CommandBarMessageRoleAssistant,
		Content:      content,
		PageContext:  pageContextJSON,
		ProposalJSON: proposalJSON,
	}
	if err := s.chatRepo.CreateMessage(ctx, &assistantMessage); err != nil {
		return nil, err
	}

	return &model.CommandBarChatTurnResponse{
		Thread:           commandBarThreadSummary(*thread),
		UserMessage:      commandBarMessageSummary(userMessage),
		AssistantMessage: commandBarMessageSummary(assistantMessage),
		Proposal:         proposal,
	}, nil
}

func (s *CommandBarService) ListChatThreads(ctx context.Context, workspaceID, actorID string, limit int) (*model.ListCommandBarChatThreadsResponse, error) {
	if s == nil || s.chatRepo == nil {
		return nil, fmt.Errorf("command bar chat service is not configured")
	}
	threads, err := s.chatRepo.ListRecentThreads(ctx, workspaceID, actorID, limit)
	if err != nil {
		return nil, err
	}
	resp := &model.ListCommandBarChatThreadsResponse{
		Threads: make([]model.CommandBarThreadDetail, 0, len(threads)),
	}
	for _, thread := range threads {
		messages, err := s.chatRepo.ListRecentMessages(ctx, workspaceID, thread.ID, 30)
		if err != nil {
			return nil, err
		}
		detail := model.CommandBarThreadDetail{
			Thread:   commandBarThreadSummary(thread),
			Messages: make([]model.CommandBarMessageSummary, 0, len(messages)),
		}
		for _, message := range messages {
			detail.Messages = append(detail.Messages, commandBarMessageSummary(message))
		}
		resp.Threads = append(resp.Threads, detail)
	}
	return resp, nil
}

func (s *CommandBarService) ConfirmChatCreateAgent(ctx context.Context, workspaceID, actorID, messageID string, req model.ConfirmCommandBarChatProposalRequest) (*model.ConfirmCommandBarChatCreateAgentResponse, error) {
	if s == nil || s.chatRepo == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar chat service is not configured")
	}
	message, err := s.chatRepo.GetMessage(ctx, workspaceID, strings.TrimSpace(messageID))
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, fmt.Errorf("chat proposal not found")
	}
	thread, err := s.chatRepo.GetThread(ctx, workspaceID, message.ThreadID)
	if err != nil {
		return nil, err
	}
	if thread == nil || thread.Status != model.CommandBarThreadStatusOpen || !commandBarThreadOwnedByActor(thread, actorID) {
		return nil, fmt.Errorf("chat proposal not found")
	}
	proposal, err := decodeCommandBarProposal(message.ProposalJSON)
	if err != nil {
		return nil, err
	}
	if proposal == nil || (proposal.Type != model.CommandBarProposalCreateAgent && proposal.Type != model.CommandBarProposalCreateAgentAndRun) || proposal.Draft == nil {
		return nil, fmt.Errorf("chat message does not contain an agent creation proposal")
	}
	if len(req.AllowedTools) > 0 || len(req.AllowedTargets) > 0 {
		return nil, fmt.Errorf("agent proposal tool and target overrides are not supported")
	}
	agent, err := s.agentService.CreateAgent(ctx, commandBarCreateAgentRequestFromDraft(workspaceID, *proposal.Draft, req), actorID)
	if err != nil {
		return nil, err
	}
	resp := &model.ConfirmCommandBarChatCreateAgentResponse{Agent: *agent}
	if proposal.Type == model.CommandBarProposalCreateAgentAndRun {
		target := proposal.RunTarget
		if target == nil {
			return nil, fmt.Errorf("create-and-run proposal is missing a run target")
		}
		instructions := strings.TrimSpace(proposal.RunInstructions)
		if instructions == "" {
			instructions = "Run the newly created agent for the approved chat proposal."
		}
		run, err := s.agentService.StartTargetRun(ctx, workspaceID, target.EntityType, target.EntityID, model.StartAgentRunRequest{
			AgentID:           agent.ID,
			AdditionalContext: &instructions,
		}, actorID)
		if err != nil {
			return nil, err
		}
		resp.Run = run
	}
	return resp, nil
}

func (s *CommandBarService) commandBarThreadForTurn(ctx context.Context, workspaceID, actorID string, threadID *string, text string) (*model.CommandBarThread, error) {
	if threadID != nil && strings.TrimSpace(*threadID) != "" {
		thread, err := s.chatRepo.GetThread(ctx, workspaceID, strings.TrimSpace(*threadID))
		if err != nil {
			return nil, err
		}
		if thread == nil || !commandBarThreadOwnedByActor(thread, actorID) {
			return nil, fmt.Errorf("command bar chat thread not found")
		}
		return thread, nil
	}
	title := strings.TrimSpace(text)
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	if title == "" {
		title = "Ask Agents"
	}
	thread := &model.CommandBarThread{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		ActorID:     optionalActorID(actorID),
		Title:       title,
		Status:      model.CommandBarThreadStatusOpen,
	}
	if err := s.chatRepo.CreateThread(ctx, thread); err != nil {
		return nil, err
	}
	return thread, nil
}

func (s *CommandBarService) commandBarChatProposal(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, access CommandBarChatAccess, history []model.CommandBarMessage) (*model.CommandBarProposal, string, error) {
	effectiveText := text
	explicitTargetResolved := false
	repositoryChoiceResolved := false
	if explicitTarget, ok, err := s.resolveCommandBarExplicitTarget(ctx, workspaceID, text); err != nil {
		proposal := &model.CommandBarProposal{Type: model.CommandBarProposalNoMatch, Reason: err.Error()}
		return proposal, err.Error(), nil
	} else if ok {
		pageContext = normalizeCommandBarPageContext(explicitTarget, workspaceID)
		explicitTargetResolved = true
	} else if choiceTarget, ok, err := s.resolveCommandBarRepositoryChoiceTarget(ctx, workspaceID, text, history); err != nil {
		proposal := &model.CommandBarProposal{Type: model.CommandBarProposalNoMatch, Reason: err.Error()}
		return proposal, err.Error(), nil
	} else if ok {
		pageContext = normalizeCommandBarPageContext(choiceTarget, workspaceID)
		explicitTargetResolved = true
		repositoryChoiceResolved = true
	} else if inferredTarget, ok, _ := s.inferCommandBarTargetFromHistory(ctx, workspaceID, history); ok {
		pageContext = normalizeCommandBarPageContext(inferredTarget, workspaceID)
	}
	if explicitTargetResolved && (repositoryChoiceResolved || commandBarLooksLikeTargetClarification(text)) {
		if pending := commandBarPendingTargetRequestFromHistory(history); pending != "" {
			effectiveText = pending + "\n\nTarget clarification: " + text
		}
	}

	if denied := commandBarDeniedReadOnlyDomainAnswer(effectiveText, access); denied != "" {
		return &model.CommandBarProposal{Type: model.CommandBarProposalInlineAnswer, Answer: denied}, denied, nil
	}
	if commandBarShouldUseRunPlannerBeforeInline(effectiveText, pageContext) {
		proposal, content, handled, err := s.commandBarRunPlanProposalFromParse(ctx, workspaceID, actorID, effectiveText, pageContext)
		if err != nil {
			return nil, "", err
		}
		if handled {
			return proposal, content, nil
		}
	}

	classification, err := s.classifyCommandBarChatIntent(ctx, workspaceID, effectiveText, pageContext, access, history)
	if err != nil {
		slog.WarnContext(ctx, "ask agents chat intent classification failed", "error", err, "workspace_id", workspaceID)
		answer, inlineContext := s.inlineReadOnlyAnswer(ctx, workspaceID, actorID, effectiveText, pageContext, access, history)
		return &model.CommandBarProposal{Type: model.CommandBarProposalInlineAnswer, Answer: answer, Context: inlineContext}, answer, nil
	}
	route := normalizeCommandBarChatRoute("")
	if classification != nil {
		route = normalizeCommandBarChatRoute(classification.Route)
	}
	switch route {
	case "inline_read_only":
		answer, inlineContext := s.inlineReadOnlyAnswer(ctx, workspaceID, actorID, effectiveText, pageContext, access, history)
		return &model.CommandBarProposal{Type: model.CommandBarProposalInlineAnswer, Answer: answer, Context: inlineContext}, answer, nil
	case "create_agent":
		return s.commandBarCreateAgentChatProposal(ctx, workspaceID, effectiveText, pageContext)
	case "clarification":
		answer := strings.TrimSpace(classification.Answer)
		if answer == "" {
			answer = strings.TrimSpace(classification.Reason)
		}
		if answer == "" {
			answer = "What should I use as the target or scope for this request?"
		}
		proposal := &model.CommandBarProposal{Type: model.CommandBarProposalClarification, Answer: answer, Reason: strings.TrimSpace(classification.Reason)}
		return proposal, answer, nil
	default:
		if classification == nil && shouldCreateReusableAgentFromChat(effectiveText) {
			return s.commandBarCreateAgentChatProposal(ctx, workspaceID, effectiveText, pageContext)
		}
	}

	proposal, content, handled, err := s.commandBarRunPlanProposalFromParse(ctx, workspaceID, actorID, effectiveText, pageContext)
	if err != nil {
		return nil, "", err
	}
	if handled {
		return proposal, content, nil
	}
	return &model.CommandBarProposal{Type: model.CommandBarProposalNoMatch, Reason: "No available agent matched this request."}, "No available agent matched this request.", nil
}

func (s *CommandBarService) commandBarRunPlanProposalFromParse(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext) (*model.CommandBarProposal, string, bool, error) {
	parsed, err := s.ParseIntent(ctx, workspaceID, actorID, model.CommandBarParseRequest{
		Text:        text,
		PageContext: pageContext,
	})
	if err != nil {
		return nil, "", false, err
	}
	if parsed.Status == model.CommandBarParseStatusPlan && parsed.Plan != nil {
		proposal := &model.CommandBarProposal{
			Type:       model.CommandBarProposalRunPlan,
			Plan:       parsed.Plan,
			Guardrails: parsed.Plan.Guardrails,
		}
		return proposal, commandBarPlanProposalContent(*parsed.Plan), true, nil
	}
	if proposal, content, ok := s.commandBarRepositoryTargetClarification(ctx, workspaceID, parsed.Reason); ok {
		return proposal, content, true, nil
	}
	proposal := &model.CommandBarProposal{
		Type:        model.CommandBarProposalNoMatch,
		Reason:      parsed.Reason,
		Suggestions: parsed.Suggestions,
	}
	return proposal, firstNonEmptyString(strings.TrimSpace(parsed.Reason), "No available agent matched this request."), true, nil
}

func (s *CommandBarService) commandBarCreateAgentChatProposal(ctx context.Context, workspaceID, text string, pageContext model.CommandBarPageContext) (*model.CommandBarProposal, string, error) {
	if s == nil || s.agentService == nil {
		return nil, "", fmt.Errorf("agent service is not configured")
	}
	draft, err := s.agentService.DraftCustomAgent(ctx, model.CustomAgentDraftRequest{Description: text})
	if err != nil {
		return nil, "", err
	}
	proposalType := model.CommandBarProposalCreateAgent
	if shouldCreateAndRunReusableAgentFromChat(text) {
		proposalType = model.CommandBarProposalCreateAgentAndRun
	}
	proposal := &model.CommandBarProposal{
		Type:            proposalType,
		Draft:           &draft.Draft,
		Reasons:         draft.Reasons,
		Warnings:        draft.Warnings,
		RunTarget:       &pageContext,
		RunInstructions: text,
		Guardrails: []model.CommandBarGuardrail{{
			Type:     "custom_agent_creation",
			Severity: "info",
			Message:  "This creates a reusable custom agent only after you approve it.",
		}},
	}
	action := "create a reusable agent"
	if proposalType == model.CommandBarProposalCreateAgentAndRun {
		action = "create a reusable agent and run it once"
	}
	return proposal, fmt.Sprintf("I can %s for this. Review the draft before approving.", action), nil
}

func (s *CommandBarService) classifyCommandBarChatIntent(ctx context.Context, workspaceID, text string, pageContext model.CommandBarPageContext, access CommandBarChatAccess, history []model.CommandBarMessage) (*commandBarChatIntentClassification, error) {
	if s == nil || s.llmProvider == nil {
		return nil, nil
	}
	tools := s.executableReadOnlyToolCards(access)
	var agents []model.CommandBarAgent
	if s.agentService != nil && s.agentService.agentRepo != nil {
		list, err := s.agentService.ListAgents(ctx, workspaceID)
		if err == nil {
			agents = commandBarAllAgentCandidates(list)
		} else {
			slog.WarnContext(ctx, "ask agents chat classifier could not list agents", "error", err, "workspace_id", workspaceID)
		}
	}
	contextJSON, _ := json.Marshal(pageContext)
	historyJSON, _ := json.Marshal(commandBarChatHistoryForClassifier(history))
	targetResolutionJSON, _ := json.Marshal(s.commandBarChatTargetResolutionForClassifier(ctx, workspaceID, pageContext, history))
	toolJSON, _ := json.Marshal(commandBarChatClassifierToolCards(tools))
	agentJSON, _ := json.Marshal(commandBarChatClassifierAgentCards(agents))
	timeout := s.commandRouterTimeoutForRequest()
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := s.llmProvider.ChatCompletion(callCtx, llm.ChatRequest{
		Provider: s.commandRouterLLMProvider,
		Model:    s.commandRouterLLMModel,
		SystemPrompt: `You are Helpin's Ask Agents chat intent classifier. Return strict JSON only. Do not answer the user.

Choose exactly one route:
- "inline_read_only": the user asks a factual, status, count, list, search, summary, "how do I", or follow-up question that can be answered directly from chat history or by the available non-mutating tools.
- "run_saved_agent": the user clearly asks to run an available saved, system, or custom agent.
- "one_shot_command": the user asks for ad hoc durable work, action execution, broader investigation, mutation, or tool use that is outside the inline read-only tools.
- "create_agent": the user asks to create, save, or define a reusable agent.
- "clarification": the request is missing the target or scope needed to choose a safe route.

Policy:
- Prefer "inline_read_only" for read-only workspace questions when the available non-mutating tools can fetch the data.
- A resolved task, document, CRM object, or workspace page context is enough target context for inline read-only status questions.
- Do not route to one-shot merely because live data is needed; inline read-only tools are live data tools.
- If the user needs current external evidence, industry trends, online research, web search, or fetched URLs, route "inline_read_only" only when an inline web/search/fetch tool is listed. Otherwise route "one_shot_command" so the user can approve a Command Agent with web tools.
- Use "run_saved_agent" for named-agent invocations such as Forge, Lens, Atlas, or a custom agent name.
- Use "one_shot_command" for changes, writes, long-running execution, chaining, DAGs, fan-out, or requests requiring mutating tools.
- Use target resolution context when deciding whether a target is known. If prior_target_state is "multiple" and the user asks to run agents or perform target-specific work, return "clarification" and ask which target to use.
- Do not return clarification for read-only questions that can operate over an ambiguous prior result set, such as summarizing, counting, ranking, or comparing all referenced entities.`,
		Messages: []llm.Message{{
			Role: "user",
			Content: fmt.Sprintf(`Question: %s
Page context: %s
Recent chat history: %s
Target resolution context: %s
Available inline read-only tools: %s
Available saved/system/custom agents: %s`, text, string(contextJSON), string(historyJSON), string(targetResolutionJSON), string(toolJSON), string(agentJSON)),
		}},
		Temperature:     0,
		MaxTokens:       300,
		JSONMode:        true,
		JSONSchema:      commandBarChatIntentClassificationJSONSchema(),
		ProviderOptions: s.commandRouterProviderOptionsForRequest(),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return nil, nil
	}
	var classification commandBarChatIntentClassification
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &classification); err != nil {
		return nil, err
	}
	classification.Route = normalizeCommandBarChatRoute(classification.Route)
	if classification.Route == "" {
		return nil, nil
	}
	return &classification, nil
}

func (s *CommandBarService) inlineReadOnlyAnswer(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, access CommandBarChatAccess, history []model.CommandBarMessage) (string, json.RawMessage) {
	if denied := commandBarDeniedReadOnlyDomainAnswer(text, access); denied != "" {
		return denied, nil
	}
	if s.llmProvider == nil {
		return fallbackInlineReadOnlyAnswer(text, pageContext), nil
	}
	tools := s.executableReadOnlyToolCards(access)
	contextJSON, _ := json.Marshal(pageContext)
	toolJSON, _ := json.Marshal(tools)
	historyJSON, _ := json.Marshal(commandBarInlineChatHistoryForLLM(history))
	historyWorkingContext := commandBarWorkingContextFromHistory(history)
	toolCalls := make([]commandBarReadOnlyToolCall, 0, 4)
	for i := 0; i < 4; i++ {
		toolResultJSON, _ := json.Marshal(toolCalls)
		workingContextJSON, _ := json.Marshal(commandBarWorkingContextWithToolCalls(historyWorkingContext, toolCalls))
		resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
			Provider: s.commandRouterLLMProvider,
			Model:    s.commandRouterLLMModel,
			SystemPrompt: `You are Helpin's Ask Agents chat assistant.
You are a normal chat, not a deterministic lookup formatter.
Answer read-only questions directly when the answer is general Helpin guidance or can be inferred from chat history.
Use the working context to resolve references such as "these", "those", "all", "top ones", and "out of those" before asking for clarification.
For live workspace data, request one allowed read-only tool at a time, wait for the tool result, then answer from the result.
If the user asks to summarize, compare, rank, or choose from referenced entities and the available context is too shallow, fetch richer read-only detail for the referenced set first.
Respect the domain of the current question. If the user asks about documents/docs, do not answer from PM task result sets; use Docs tools or ask a clarification. If the user asks about tasks, do not answer from Docs result sets.
Never claim that a tool was executed unless a tool result is present.
Never request or simulate mutating actions. If the user asks for mutation, reusable agents, chains, DAGs, or long-running work, do not answer inline.
Return JSON only with one of:
{"type":"tool_call","tool":"tool_name","input":{...}}
{"type":"final","answer":"concise answer"}
{"type":"clarification","answer":"question to ask"}`,
			Messages: []llm.Message{{
				Role: "user",
				Content: fmt.Sprintf(`Current question: %s
Page context: %s
Recent chat history: %s
Working context: %s
Available executable read-only tools: %s
Tool results so far: %s`, text, string(contextJSON), string(historyJSON), string(workingContextJSON), string(toolJSON), string(toolResultJSON)),
			}},
			Temperature:     0,
			MaxTokens:       900,
			JSONMode:        true,
			JSONSchema:      commandBarReadOnlyToolTurnJSONSchema(),
			ProviderOptions: s.commandRouterProviderOptionsForRequest(),
		})
		if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
			if err != nil {
				slog.WarnContext(ctx, "ask agents read-only chat llm failed", "error", err, "workspace_id", workspaceID)
			}
			return fallbackInlineReadOnlyAnswer(text, pageContext), commandBarReadOnlyToolContextJSON(toolCalls, commandBarWorkingContextWithToolCalls(historyWorkingContext, toolCalls))
		}
		turn, err := decodeCommandBarReadOnlyToolTurn(resp.Content)
		if err != nil {
			slog.WarnContext(ctx, "ask agents read-only chat returned invalid json", "error", err, "workspace_id", workspaceID)
			return strings.TrimSpace(resp.Content), commandBarReadOnlyToolContextJSON(toolCalls, commandBarWorkingContextWithToolCalls(historyWorkingContext, toolCalls))
		}
		switch strings.ToLower(strings.TrimSpace(turn.Type)) {
		case "final", "clarification":
			answer := strings.TrimSpace(turn.Answer)
			if answer == "" {
				answer = strings.TrimSpace(turn.Reason)
			}
			if answer == "" {
				answer = fallbackInlineReadOnlyAnswer(text, pageContext)
			}
			return answer, commandBarReadOnlyToolContextJSON(toolCalls, commandBarWorkingContextWithToolCalls(historyWorkingContext, toolCalls))
		case "tool_call":
			call := commandBarReadOnlyToolCall{Tool: strings.TrimSpace(turn.Tool), Input: normalizeCommandBarToolInput(turn.Input)}
			output, err := s.executeCommandBarReadOnlyTool(ctx, workspaceID, actorID, pageContext, access, call.Tool, call.Input)
			if err != nil {
				call.Error = err.Error()
			} else {
				call.Output = truncateCommandBarRawJSON(output, 20000)
			}
			toolCalls = append(toolCalls, call)
		default:
			answer := firstNonEmptyString(strings.TrimSpace(turn.Answer), strings.TrimSpace(resp.Content))
			return answer, commandBarReadOnlyToolContextJSON(toolCalls, commandBarWorkingContextWithToolCalls(historyWorkingContext, toolCalls))
		}
	}
	finalWorkingContext := commandBarWorkingContextWithToolCalls(historyWorkingContext, toolCalls)
	finalContext := commandBarReadOnlyToolContextJSON(toolCalls, finalWorkingContext)
	finalToolJSON, _ := json.Marshal(toolCalls)
	finalWorkingContextJSON, _ := json.Marshal(finalWorkingContext)
	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		Provider:     s.commandRouterLLMProvider,
		Model:        s.commandRouterLLMModel,
		SystemPrompt: `You are Helpin's Ask Agents chat assistant. Write the final concise answer from the provided read-only tool results. Do not request more tools. Return JSON only: {"type":"final","answer":"..."}.`,
		Messages: []llm.Message{{
			Role:    "user",
			Content: fmt.Sprintf("Question: %s\nPage context: %s\nRecent chat history: %s\nWorking context: %s\nTool results: %s", text, string(contextJSON), string(historyJSON), string(finalWorkingContextJSON), string(finalToolJSON)),
		}},
		Temperature:     0,
		MaxTokens:       700,
		JSONMode:        true,
		JSONSchema:      commandBarReadOnlyFinalAnswerJSONSchema(),
		ProviderOptions: s.commandRouterProviderOptionsForRequest(),
	})
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return fallbackInlineReadOnlyAnswer(text, pageContext), finalContext
	}
	turn, err := decodeCommandBarReadOnlyToolTurn(resp.Content)
	if err != nil || strings.TrimSpace(turn.Answer) == "" {
		return strings.TrimSpace(resp.Content), finalContext
	}
	return strings.TrimSpace(turn.Answer), finalContext
}

func commandBarDeniedReadOnlyDomainAnswer(text string, access CommandBarChatAccess) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case !access.CanReadPM && containsAny(lower, "task", "tasks", "story", "stories", "epic", "epics"):
		return "I cannot access PM task data for this workspace with your current permissions."
	case !access.CanReadDocs && containsAny(lower, "doc", "docs", "document", "documents", "knowledge"):
		return "I cannot access Docs data for this workspace with your current permissions."
	case !access.CanReadCRM && containsAny(lower, "crm", "deal", "deals", "contact", "contacts", "company", "companies"):
		return "I cannot access CRM data for this workspace with your current permissions."
	default:
		return ""
	}
}

func decodeCommandBarReadOnlyToolTurn(content string) (commandBarReadOnlyToolTurn, error) {
	var turn commandBarReadOnlyToolTurn
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &turn); err != nil {
		return commandBarReadOnlyToolTurn{}, err
	}
	if strings.TrimSpace(turn.Type) == "" {
		var wrapped struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &wrapped); err == nil && strings.TrimSpace(wrapped.Content) != "" {
			var nested commandBarReadOnlyToolTurn
			if err := json.Unmarshal([]byte(strings.TrimSpace(wrapped.Content)), &nested); err == nil && strings.TrimSpace(nested.Type) != "" {
				return nested, nil
			}
			turn.Type = "final"
			turn.Answer = strings.TrimSpace(wrapped.Content)
		}
	}
	return turn, nil
}

func commandBarReadOnlyToolTurnJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type": "string",
				"enum": []string{"tool_call", "final", "clarification"},
			},
			"tool": map[string]any{
				"type": "string",
			},
			"input": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
			"answer": map[string]any{
				"type": "string",
			},
			"reason": map[string]any{
				"type": "string",
			},
		},
		"required":             []string{"type"},
		"additionalProperties": false,
	}
}

func commandBarReadOnlyFinalAnswerJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type": "string",
				"enum": []string{"final", "clarification"},
			},
			"answer": map[string]any{
				"type": "string",
			},
			"reason": map[string]any{
				"type": "string",
			},
		},
		"required":             []string{"type", "answer"},
		"additionalProperties": false,
	}
}

func commandBarChatIntentClassificationJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"route": map[string]any{
				"type": "string",
				"enum": []string{"inline_read_only", "run_saved_agent", "one_shot_command", "create_agent", "clarification"},
			},
			"reason": map[string]any{
				"type": "string",
			},
			"answer": map[string]any{
				"type": "string",
			},
			"confidence": map[string]any{
				"type": "number",
			},
		},
		"required":             []string{"route"},
		"additionalProperties": false,
	}
}

func normalizeCommandBarChatRoute(route string) string {
	switch strings.ToLower(strings.TrimSpace(route)) {
	case "inline_read_only", "run_saved_agent", "one_shot_command", "create_agent", "clarification":
		return strings.ToLower(strings.TrimSpace(route))
	default:
		return ""
	}
}

func (s *CommandBarService) commandRouterProviderOptionsForRequest() json.RawMessage {
	if s == nil || len(s.commandRouterOpenRouterProviderOptions) == 0 {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(s.commandRouterLLMProvider)) {
	case model.AgentModelProviderOpenRouter, model.AgentModelProviderOpenRouterResponses:
		return append(json.RawMessage(nil), s.commandRouterOpenRouterProviderOptions...)
	default:
		return nil
	}
}

func (s *CommandBarService) commandRouterTimeoutForRequest() time.Duration {
	timeout := defaultCommandRouterTimeout
	if s != nil && s.commandRouterLLMTimeout > 0 {
		timeout = s.commandRouterLLMTimeout
	}
	if s != nil {
		switch strings.ToLower(strings.TrimSpace(s.commandRouterLLMProvider)) {
		case model.AgentModelProviderOpenRouter, model.AgentModelProviderOpenRouterResponses:
			if timeout < minOpenRouterCommandRouterTimeout {
				return minOpenRouterCommandRouterTimeout
			}
		}
	}
	return timeout
}

func normalizeCommandBarToolInput(input json.RawMessage) json.RawMessage {
	if len(input) == 0 || strings.TrimSpace(string(input)) == "" || string(input) == "null" {
		return json.RawMessage(`{}`)
	}
	return input
}

func commandBarReadOnlyToolContextJSON(toolCalls []commandBarReadOnlyToolCall, workingContext *commandBarWorkingContext) json.RawMessage {
	workingContext = commandBarNormalizeWorkingContext(workingContext)
	if len(toolCalls) == 0 && workingContext == nil {
		return nil
	}
	raw, err := json.Marshal(commandBarReadOnlyToolContext{Mode: "read_only_tool_chat", ToolCalls: toolCalls, WorkingContext: workingContext})
	if err != nil {
		return nil
	}
	return raw
}

func truncateCommandBarRawJSON(raw json.RawMessage, maxBytes int) json.RawMessage {
	if maxBytes <= 0 || len(raw) <= maxBytes {
		return raw
	}
	truncated, _ := json.Marshal(map[string]any{
		"truncated": true,
		"preview":   string(raw[:maxBytes]),
	})
	return truncated
}

func commandBarInlineChatHistoryForLLM(history []model.CommandBarMessage) []map[string]string {
	if len(history) == 0 {
		return nil
	}
	const maxInlineHistoryMessages = 10
	if len(history) > maxInlineHistoryMessages {
		history = history[len(history)-maxInlineHistoryMessages:]
	}
	items := make([]map[string]string, 0, len(history))
	for _, message := range history {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		items = append(items, map[string]string{
			"role":    string(message.Role),
			"content": truncateCommandBarText(content, 1600),
		})
	}
	return items
}

func commandBarWorkingContextFromHistory(history []model.CommandBarMessage) *commandBarWorkingContext {
	if len(history) == 0 {
		return nil
	}
	working := &commandBarWorkingContext{}
	for _, message := range history {
		commandBarMergeWorkingContext(working, commandBarWorkingContextFromText(message.Content))
		proposal, err := decodeCommandBarProposal(message.ProposalJSON)
		if err != nil || proposal == nil || len(proposal.Context) == 0 {
			continue
		}
		var toolContext commandBarReadOnlyToolContext
		if err := json.Unmarshal(proposal.Context, &toolContext); err != nil {
			continue
		}
		if toolContext.WorkingContext != nil {
			commandBarMergeWorkingContext(working, toolContext.WorkingContext)
			continue
		}
		commandBarMergeWorkingContext(working, commandBarWorkingContextFromToolCalls(toolContext.ToolCalls))
	}
	return commandBarNormalizeWorkingContext(working)
}

func commandBarPendingTargetRequestFromHistory(history []model.CommandBarMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		message := history[i]
		if message.Role != model.CommandBarMessageRoleAssistant {
			continue
		}
		proposal, err := decodeCommandBarProposal(message.ProposalJSON)
		if err != nil || !commandBarProposalRequestsTarget(proposal, message.Content) {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			prior := history[j]
			if prior.Role != model.CommandBarMessageRoleUser {
				continue
			}
			return strings.TrimSpace(prior.Content)
		}
	}
	return ""
}

func commandBarProposalRequestsTarget(proposal *model.CommandBarProposal, content string) bool {
	if proposal == nil {
		return containsAny(strings.ToLower(strings.TrimSpace(content)), "target", "scope")
	}
	switch proposal.Type {
	case model.CommandBarProposalClarification:
		return true
	case model.CommandBarProposalNoMatch:
		reason := strings.ToLower(strings.TrimSpace(firstNonEmptyString(proposal.Reason, content)))
		return containsAny(reason, "target", "scope", "please name")
	default:
		return false
	}
}

func commandBarLooksLikeTargetClarification(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if containsAny(lower, "changelog", "change log", "release notes", "latest commits", "create document", "create a doc", "write", "summarize", "review", "run forge", "run lens") {
		return false
	}
	return containsAny(lower, "target", "scope", "repository", "repo", "document", "doc", "task", "story", "epic", "deal", "contact")
}

func (s *CommandBarService) resolveCommandBarRepositoryChoiceTarget(ctx context.Context, workspaceID, text string, history []model.CommandBarMessage) (model.CommandBarPageContext, bool, error) {
	index, ok := commandBarRepositoryChoiceIndex(text)
	if !ok {
		return model.CommandBarPageContext{}, false, nil
	}
	choices := commandBarLastRepositoryChoicesFromHistory(history)
	if len(choices) == 0 {
		return model.CommandBarPageContext{}, false, nil
	}
	if index < 0 || index >= len(choices) {
		return model.CommandBarPageContext{}, true, fmt.Errorf("Please choose a repository number from 1 to %d.", len(choices))
	}
	target, err := s.commandBarValidateExplicitTarget(ctx, workspaceID, "repository", choices[index].ID)
	if err != nil {
		return model.CommandBarPageContext{}, true, err
	}
	return target, true, nil
}

func commandBarRepositoryChoiceIndex(text string) (int, bool) {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	trimmed = strings.TrimPrefix(trimmed, "#")
	switch trimmed {
	case "first", "the first one", "first one":
		return 0, true
	case "second", "the second one", "second one":
		return 1, true
	case "third", "the third one", "third one":
		return 2, true
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value - 1, true
}

func commandBarLastRepositoryChoicesFromHistory(history []model.CommandBarMessage) []commandBarRepositoryChoice {
	for i := len(history) - 1; i >= 0; i-- {
		message := history[i]
		if message.Role != model.CommandBarMessageRoleAssistant {
			continue
		}
		proposal, err := decodeCommandBarProposal(message.ProposalJSON)
		if err != nil || proposal == nil || len(proposal.Context) == 0 {
			continue
		}
		var context commandBarRepositoryChoiceContext
		if err := json.Unmarshal(proposal.Context, &context); err != nil {
			continue
		}
		if context.Mode == "repository_target_choices" && len(context.Repositories) > 0 {
			return context.Repositories
		}
	}
	return nil
}

func (s *CommandBarService) inferCommandBarTargetFromHistory(ctx context.Context, workspaceID string, history []model.CommandBarMessage) (model.CommandBarPageContext, bool, bool) {
	working := commandBarWorkingContextFromHistory(history)
	candidates := commandBarRunnableTargetRefs(working)
	if len(candidates) == 0 {
		return model.CommandBarPageContext{}, false, false
	}
	if len(candidates) > 1 {
		return model.CommandBarPageContext{}, false, true
	}
	target, ok := s.commandBarTargetFromWorkingEntity(ctx, workspaceID, candidates[0])
	return target, ok, false
}

func (s *CommandBarService) commandBarChatTargetResolutionForClassifier(ctx context.Context, workspaceID string, pageContext model.CommandBarPageContext, history []model.CommandBarMessage) commandBarChatClassifierTargetResolution {
	resolution := commandBarChatClassifierTargetResolution{
		CurrentTarget:    normalizeCommandBarPageContext(pageContext, workspaceID),
		PriorTargetState: "none",
	}
	candidates := commandBarRunnableTargetRefs(commandBarWorkingContextFromHistory(history))
	switch len(candidates) {
	case 0:
		resolution.Instruction = "No prior runnable target is available from chat history."
	case 1:
		resolution.PriorTargetState = "single"
		if target, ok := s.commandBarTargetFromWorkingEntity(ctx, workspaceID, candidates[0]); ok {
			resolution.InferredPriorTarget = &target
			resolution.Instruction = "A single prior runnable target is available and may be used for follow-up run requests."
		} else {
			resolution.CandidateTargets = candidates
			resolution.Instruction = "One prior target reference exists, but it is not concrete enough to run without clarification."
		}
	default:
		resolution.PriorTargetState = "multiple"
		if len(candidates) > 10 {
			candidates = candidates[:10]
		}
		resolution.CandidateTargets = candidates
		resolution.Instruction = "Multiple prior runnable targets are available. Target-specific agent runs need clarification, while read-only questions may operate over the set."
	}
	return resolution
}

func commandBarRunnableTargetRefs(working *commandBarWorkingContext) []commandBarWorkingEntityRef {
	if working == nil {
		return nil
	}
	seen := map[string]commandBarWorkingEntityRef{}
	for _, entity := range working.ReferencedEntities {
		entity.Type = normalizeCommandBarTargetType(entity.Type)
		switch entity.Type {
		case "task", "epic", "document", "crm_contact", "crm_deal", "repository":
		default:
			continue
		}
		key := entity.Type + "|" + firstNonEmptyString(strings.TrimSpace(entity.ID), strings.TrimSpace(entity.Key), strings.ToLower(strings.TrimSpace(entity.Title)))
		if strings.TrimSpace(key) == "|" {
			continue
		}
		seen[key] = entity
	}
	if len(seen) == 0 {
		return nil
	}
	refs := make([]commandBarWorkingEntityRef, 0, len(seen))
	for _, entity := range seen {
		refs = append(refs, entity)
	}
	slices.SortFunc(refs, func(a, b commandBarWorkingEntityRef) int {
		left := a.Type + "|" + firstNonEmptyString(a.ID, a.Key, a.Title)
		right := b.Type + "|" + firstNonEmptyString(b.ID, b.Key, b.Title)
		return strings.Compare(left, right)
	})
	return refs
}

func (s *CommandBarService) commandBarTargetFromWorkingEntity(ctx context.Context, workspaceID string, entity commandBarWorkingEntityRef) (model.CommandBarPageContext, bool) {
	targetType := normalizeCommandBarTargetType(entity.Type)
	if targetType == "task" && strings.TrimSpace(entity.ID) == "" && strings.TrimSpace(entity.Key) != "" {
		target, ok, err := s.resolveCommandBarExplicitTarget(ctx, workspaceID, entity.Key)
		if err != nil {
			return model.CommandBarPageContext{}, false
		}
		return target, ok
	}
	if strings.TrimSpace(entity.ID) == "" {
		return model.CommandBarPageContext{}, false
	}
	return commandBarFillTargetDisplayTitle(model.CommandBarPageContext{
		EntityType:   targetType,
		EntityID:     strings.TrimSpace(entity.ID),
		DisplayTitle: commandBarWorkingEntityDisplayTitle(entity, targetType),
	}), true
}

func commandBarWorkingEntityDisplayTitle(entity commandBarWorkingEntityRef, fallbackType string) string {
	key := strings.TrimSpace(entity.Key)
	title := strings.TrimSpace(entity.Title)
	switch {
	case key != "" && title != "":
		return key + ": " + title
	case title != "":
		return title
	case key != "":
		return key
	default:
		return fallbackType
	}
}

func commandBarWorkingContextWithToolCalls(base *commandBarWorkingContext, toolCalls []commandBarReadOnlyToolCall) *commandBarWorkingContext {
	working := commandBarCloneWorkingContext(base)
	commandBarMergeWorkingContext(working, commandBarWorkingContextFromToolCalls(toolCalls))
	return commandBarNormalizeWorkingContext(working)
}

func commandBarWorkingContextFromText(text string) *commandBarWorkingContext {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	matches := commandBarTaskKeyPattern.FindAllStringSubmatch(text, 20)
	if len(matches) == 0 {
		return nil
	}
	working := &commandBarWorkingContext{ReferencedEntities: make([]commandBarWorkingEntityRef, 0, len(matches))}
	seen := map[string]bool{}
	for _, match := range matches {
		if len(match) == 0 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(match[0]))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		working.ReferencedEntities = append(working.ReferencedEntities, commandBarWorkingEntityRef{
			Type: "task",
			Key:  key,
		})
	}
	return commandBarNormalizeWorkingContext(working)
}

func commandBarWorkingContextFromToolCalls(toolCalls []commandBarReadOnlyToolCall) *commandBarWorkingContext {
	if len(toolCalls) == 0 {
		return nil
	}
	working := &commandBarWorkingContext{}
	for _, call := range toolCalls {
		if strings.TrimSpace(call.Error) != "" || len(call.Output) == 0 {
			continue
		}
		switch commandBarCommandNameForTool(call.Tool) {
		case "pm.list_tasks":
			commandBarMergeWorkingContext(working, commandBarWorkingContextFromPMListTasks(call))
		case "docs.list_documents":
			commandBarMergeWorkingContext(working, commandBarWorkingContextFromDocsListDocuments(call))
		case "workspace.list_teams":
			commandBarMergeWorkingContext(working, commandBarWorkingContextFromWorkspaceListTeams(call))
		}
	}
	return commandBarNormalizeWorkingContext(working)
}

func commandBarCommandNameForTool(toolName string) string {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return ""
	}
	if meta, ok := commandtools.ToolMetadataForAlias(toolName); ok && meta != nil {
		return meta.CommandName
	}
	if meta, ok := commandtools.ToolMetadataForCommand(toolName); ok && meta != nil {
		return meta.CommandName
	}
	if toolName == "list_documents" {
		return "docs.list_documents"
	}
	return toolName
}

func commandBarWorkingContextFromDocsListDocuments(call commandBarReadOnlyToolCall) *commandBarWorkingContext {
	var payload map[string]any
	if err := json.Unmarshal(call.Output, &payload); err != nil {
		return nil
	}
	rawDocs, _ := payload["documents"].([]any)
	entities := make([]commandBarWorkingEntityRef, 0, min(len(rawDocs), 25))
	for _, rawDoc := range rawDocs {
		doc, _ := rawDoc.(map[string]any)
		if doc == nil {
			continue
		}
		id := commandBarMapString(doc, "document_id")
		title := commandBarMapString(doc, "title")
		status := commandBarMapString(doc, "status")
		if id == "" && title == "" {
			continue
		}
		attrs := commandBarCompactAttributes(doc, "space_id", "collection_id", "team_id", "owner_id", "requires_publish", "published_at", "next_review_at")
		entities = append(entities, commandBarWorkingEntityRef{
			Type:       "document",
			ID:         id,
			Title:      truncateCommandBarText(title, 160),
			Status:     truncateCommandBarText(status, 80),
			Attributes: attrs,
		})
		if len(entities) >= 25 {
			break
		}
	}
	total := commandBarMapInt64Ptr(payload, "total")
	if len(entities) == 0 && total == nil {
		return nil
	}
	resultSet := commandBarWorkingResultSet{
		SourceTool:  "docs.list_documents",
		EntityType:  "document",
		Filters:     commandBarCompactJSONMap(call.Input, "space_id", "collection_id", "team_id", "status", "include_archived", "limit"),
		Total:       total,
		Returned:    len(entities),
		EntityRefs:  entities,
		Description: "Docs document results from list_documents.",
	}
	return &commandBarWorkingContext{
		ReferencedEntities: entities,
		ResultSets:         []commandBarWorkingResultSet{resultSet},
		ActiveScope:        commandBarScopeFromResultSet(resultSet),
	}
}

func commandBarWorkingContextFromPMListTasks(call commandBarReadOnlyToolCall) *commandBarWorkingContext {
	var payload map[string]any
	if err := json.Unmarshal(call.Output, &payload); err != nil {
		return nil
	}
	rawTasks, _ := payload["tasks"].([]any)
	entities := make([]commandBarWorkingEntityRef, 0, min(len(rawTasks), 25))
	for _, rawTask := range rawTasks {
		task, _ := rawTask.(map[string]any)
		if task == nil {
			continue
		}
		id := commandBarMapString(task, "task_id")
		key := commandBarMapString(task, "task_key")
		if key == "" {
			if displayID := commandBarMapInt64(task, "display_id"); displayID > 0 {
				key = strconv.FormatInt(displayID, 10)
			}
		}
		title := commandBarMapString(task, "name")
		if id == "" && key == "" && title == "" {
			continue
		}
		attrs := commandBarCompactAttributes(task, "priority", "severity", "team_id", "state_id", "state_name", "completed", "external_id", "description_excerpt")
		status := commandBarMapString(task, "state_name")
		if status == "" {
			if completed, ok := task["completed"].(bool); ok && completed {
				status = "completed"
			}
		}
		entities = append(entities, commandBarWorkingEntityRef{
			Type:       "task",
			ID:         id,
			Key:        key,
			Title:      truncateCommandBarText(title, 160),
			Status:     truncateCommandBarText(status, 80),
			Attributes: attrs,
		})
		if len(entities) >= 25 {
			break
		}
	}
	total := commandBarMapInt64Ptr(payload, "total")
	if len(entities) == 0 && total == nil {
		return nil
	}
	resultSet := commandBarWorkingResultSet{
		SourceTool:  "pm.list_tasks",
		EntityType:  "task",
		Filters:     commandBarCompactJSONMap(call.Input, "label_id", "team_id", "task_id", "owner_member_ids", "owned_by_actor", "open_only", "detail_level", "include_descriptions", "include_comments", "limit"),
		Total:       total,
		Returned:    len(entities),
		EntityRefs:  entities,
		Description: "PM task results from list_tasks.",
	}
	return &commandBarWorkingContext{
		ReferencedEntities: entities,
		ResultSets:         []commandBarWorkingResultSet{resultSet},
		ActiveScope:        commandBarScopeFromResultSet(resultSet),
	}
}

func commandBarWorkingContextFromWorkspaceListTeams(call commandBarReadOnlyToolCall) *commandBarWorkingContext {
	var payload map[string]any
	if err := json.Unmarshal(call.Output, &payload); err != nil {
		return nil
	}
	rawTeams, _ := payload["teams"].([]any)
	entities := make([]commandBarWorkingEntityRef, 0, min(len(rawTeams), 25))
	for _, rawTeam := range rawTeams {
		team, _ := rawTeam.(map[string]any)
		if team == nil {
			continue
		}
		id := commandBarMapString(team, "id")
		title := commandBarMapString(team, "name")
		if id == "" && title == "" {
			continue
		}
		entities = append(entities, commandBarWorkingEntityRef{
			Type:       "workspace_team",
			ID:         id,
			Key:        commandBarMapString(team, "handle"),
			Title:      truncateCommandBarText(title, 160),
			Attributes: commandBarCompactAttributes(team, "team_type", "default_task_type"),
		})
		if len(entities) >= 25 {
			break
		}
	}
	if len(entities) == 0 {
		return nil
	}
	resultSet := commandBarWorkingResultSet{
		SourceTool:  "workspace.list_teams",
		EntityType:  "workspace_team",
		Returned:    len(entities),
		EntityRefs:  entities,
		Description: "Workspace team results from list_workspace_teams.",
	}
	return &commandBarWorkingContext{
		ReferencedEntities: entities,
		ResultSets:         []commandBarWorkingResultSet{resultSet},
		ActiveScope:        commandBarScopeFromResultSet(resultSet),
	}
}

func commandBarScopeFromResultSet(resultSet commandBarWorkingResultSet) *commandBarWorkingScope {
	return &commandBarWorkingScope{
		SourceTool:  resultSet.SourceTool,
		EntityType:  resultSet.EntityType,
		Filters:     commandBarCloneMap(resultSet.Filters),
		Total:       commandBarCloneInt64Ptr(resultSet.Total),
		Description: resultSet.Description,
	}
}

func commandBarMergeWorkingContext(dst, src *commandBarWorkingContext) {
	if dst == nil || src == nil {
		return
	}
	dst.ReferencedEntities = append(dst.ReferencedEntities, src.ReferencedEntities...)
	dst.ResultSets = append(dst.ResultSets, src.ResultSets...)
	if src.ActiveScope != nil {
		dst.ActiveScope = commandBarCloneWorkingScope(src.ActiveScope)
	}
}

func commandBarNormalizeWorkingContext(working *commandBarWorkingContext) *commandBarWorkingContext {
	if working == nil {
		return nil
	}
	const maxWorkingEntities = 50
	const maxWorkingResultSets = 8
	seen := map[string]bool{}
	entities := make([]commandBarWorkingEntityRef, 0, min(len(working.ReferencedEntities), maxWorkingEntities))
	for i := len(working.ReferencedEntities) - 1; i >= 0; i-- {
		entity := commandBarNormalizeWorkingEntity(working.ReferencedEntities[i])
		if entity.Type == "" || (entity.ID == "" && entity.Key == "" && entity.Title == "") {
			continue
		}
		key := entity.Type + "|" + firstNonEmptyString(entity.ID, entity.Key, strings.ToLower(entity.Title))
		if seen[key] {
			continue
		}
		seen[key] = true
		entities = append(entities, entity)
		if len(entities) >= maxWorkingEntities {
			break
		}
	}
	for i, j := 0, len(entities)-1; i < j; i, j = i+1, j-1 {
		entities[i], entities[j] = entities[j], entities[i]
	}
	working.ReferencedEntities = entities
	if len(working.ResultSets) > maxWorkingResultSets {
		working.ResultSets = working.ResultSets[len(working.ResultSets)-maxWorkingResultSets:]
	}
	for i := range working.ResultSets {
		working.ResultSets[i] = commandBarNormalizeWorkingResultSet(working.ResultSets[i])
	}
	if working.ActiveScope != nil {
		working.ActiveScope.Description = truncateCommandBarText(working.ActiveScope.Description, 240)
		working.ActiveScope.Filters = commandBarCloneMap(working.ActiveScope.Filters)
	}
	if len(working.ReferencedEntities) == 0 && len(working.ResultSets) == 0 && working.ActiveScope == nil {
		return nil
	}
	return working
}

func commandBarNormalizeWorkingEntity(entity commandBarWorkingEntityRef) commandBarWorkingEntityRef {
	entity.Type = normalizeCommandBarTargetType(entity.Type)
	entity.ID = strings.TrimSpace(entity.ID)
	entity.Key = strings.TrimSpace(entity.Key)
	entity.Title = truncateCommandBarText(entity.Title, 180)
	entity.Status = truncateCommandBarText(entity.Status, 100)
	entity.Attributes = commandBarCloneMap(entity.Attributes)
	return entity
}

func commandBarNormalizeWorkingResultSet(resultSet commandBarWorkingResultSet) commandBarWorkingResultSet {
	resultSet.SourceTool = strings.TrimSpace(resultSet.SourceTool)
	resultSet.EntityType = normalizeCommandBarTargetType(resultSet.EntityType)
	resultSet.Filters = commandBarCloneMap(resultSet.Filters)
	resultSet.Description = truncateCommandBarText(resultSet.Description, 240)
	if len(resultSet.EntityRefs) > 25 {
		resultSet.EntityRefs = resultSet.EntityRefs[:25]
	}
	for i := range resultSet.EntityRefs {
		resultSet.EntityRefs[i] = commandBarNormalizeWorkingEntity(resultSet.EntityRefs[i])
	}
	resultSet.Returned = len(resultSet.EntityRefs)
	return resultSet
}

func commandBarCloneWorkingContext(working *commandBarWorkingContext) *commandBarWorkingContext {
	if working == nil {
		return &commandBarWorkingContext{}
	}
	clone := &commandBarWorkingContext{
		ReferencedEntities: make([]commandBarWorkingEntityRef, 0, len(working.ReferencedEntities)),
		ResultSets:         make([]commandBarWorkingResultSet, 0, len(working.ResultSets)),
		ActiveScope:        commandBarCloneWorkingScope(working.ActiveScope),
	}
	for _, entity := range working.ReferencedEntities {
		entity.Attributes = commandBarCloneMap(entity.Attributes)
		clone.ReferencedEntities = append(clone.ReferencedEntities, entity)
	}
	for _, resultSet := range working.ResultSets {
		resultSet.Filters = commandBarCloneMap(resultSet.Filters)
		resultSet.Total = commandBarCloneInt64Ptr(resultSet.Total)
		resultSet.EntityRefs = append([]commandBarWorkingEntityRef(nil), resultSet.EntityRefs...)
		for i := range resultSet.EntityRefs {
			resultSet.EntityRefs[i].Attributes = commandBarCloneMap(resultSet.EntityRefs[i].Attributes)
		}
		clone.ResultSets = append(clone.ResultSets, resultSet)
	}
	return clone
}

func commandBarCloneWorkingScope(scope *commandBarWorkingScope) *commandBarWorkingScope {
	if scope == nil {
		return nil
	}
	return &commandBarWorkingScope{
		SourceTool:  scope.SourceTool,
		EntityType:  scope.EntityType,
		Filters:     commandBarCloneMap(scope.Filters),
		Total:       commandBarCloneInt64Ptr(scope.Total),
		Description: scope.Description,
	}
}

func commandBarCloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func commandBarCloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	clone := make(map[string]any, len(value))
	for key, raw := range value {
		switch typed := raw.(type) {
		case string:
			clone[key] = truncateCommandBarText(typed, 500)
		case []string:
			clone[key] = append([]string(nil), typed...)
		case []any:
			if len(typed) > 20 {
				typed = typed[:20]
			}
			clone[key] = append([]any(nil), typed...)
		default:
			clone[key] = raw
		}
	}
	return clone
}

func commandBarCompactJSONMap(raw json.RawMessage, keys ...string) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	allowed := make(map[string]bool, len(keys))
	for _, key := range keys {
		allowed[key] = true
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		if !allowed[key] || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				out[key] = truncateCommandBarText(typed, 240)
			}
		case bool, float64:
			out[key] = typed
		case []any:
			if len(typed) > 20 {
				typed = typed[:20]
			}
			out[key] = typed
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func commandBarCompactAttributes(values map[string]any, keys ...string) map[string]any {
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				out[key] = truncateCommandBarText(typed, 500)
			}
		case bool, float64:
			out[key] = typed
		case []any:
			if len(typed) > 8 {
				typed = typed[:8]
			}
			out[key] = typed
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func commandBarMapString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func commandBarMapInt64(values map[string]any, key string) int64 {
	switch value := values[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}

func commandBarMapInt64Ptr(values map[string]any, key string) *int64 {
	value, ok := values[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		result := int64(typed)
		return &result
	case int64:
		result := typed
		return &result
	case int:
		result := int64(typed)
		return &result
	default:
		return nil
	}
}

func commandBarChatHistoryForClassifier(history []model.CommandBarMessage) []map[string]string {
	if len(history) == 0 {
		return nil
	}
	const maxClassifierHistoryMessages = 6
	if len(history) > maxClassifierHistoryMessages {
		history = history[len(history)-maxClassifierHistoryMessages:]
	}
	items := make([]map[string]string, 0, len(history))
	for _, message := range history {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		items = append(items, map[string]string{
			"role":    string(message.Role),
			"content": truncateCommandBarText(content, 800),
		})
	}
	return items
}

func commandBarChatClassifierToolCards(tools []commandBarPlannerToolCard) []commandBarChatClassifierToolCard {
	if len(tools) == 0 {
		return nil
	}
	cards := make([]commandBarChatClassifierToolCard, 0, len(tools))
	for _, tool := range tools {
		cards = append(cards, commandBarChatClassifierToolCard{
			Name:        tool.Name,
			Category:    tool.Category,
			Description: truncateCommandBarText(tool.Description, 220),
		})
	}
	return cards
}

func commandBarChatClassifierAgentCards(candidates []model.CommandBarAgent) []commandBarChatClassifierAgentCard {
	if len(candidates) == 0 {
		return nil
	}
	cards := make([]commandBarChatClassifierAgentCard, 0, len(candidates))
	for _, candidate := range candidates {
		cards = append(cards, commandBarChatClassifierAgentCard{
			Name:           candidate.Name,
			PresetKey:      candidate.PresetKey,
			Description:    truncateCommandBarText(candidate.Description, 180),
			AllowedTargets: candidate.AllowedTargets,
			OneShot:        isOneShotCommandAgent(candidate),
		})
	}
	return cards
}

func truncateCommandBarText(value string, maxRunes int) string {
	out, _ := truncateCommandBarTextWithFlag(value, maxRunes)
	return out
}

func truncateCommandBarTextWithFlag(value string, maxRunes int) (string, bool) {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return "", value != ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value, false
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "...", true
}

func (s *CommandBarService) executeCommandBarReadOnlyTool(ctx context.Context, workspaceID, actorID string, pageContext model.CommandBarPageContext, access CommandBarChatAccess, toolName string, input json.RawMessage) (json.RawMessage, error) {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	def, ok := s.commandBarReadOnlyToolDefinition(toolName)
	if !ok {
		return nil, fmt.Errorf("tool %q is not available in Ask Agents chat", toolName)
	}
	if s == nil || s.commandService == nil {
		return nil, fmt.Errorf("read-only tool execution is not configured")
	}
	if def.Mutating || !def.ExposesTool() {
		return nil, fmt.Errorf("tool %q is not an executable read-only chat tool", toolName)
	}
	if !commandBarAccessAllowsModule(access, def.Module) {
		return nil, fmt.Errorf("you do not have permission to use %s tools in this workspace", def.Module)
	}
	targetType := firstNonEmptyString(strings.TrimSpace(pageContext.EntityType), "workspace")
	targetID := firstNonEmptyString(strings.TrimSpace(pageContext.EntityID), workspaceID)
	return s.commandService.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ActorRole:   access.ActorRole,
		TargetType:  targetType,
		TargetID:    targetID,
	}, def.Name, input)
}

func (s *CommandBarService) commandBarReadOnlyToolDefinition(toolName string) (InternalCommandDefinition, bool) {
	if s == nil || s.commandService == nil {
		return InternalCommandDefinition{}, false
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return InternalCommandDefinition{}, false
	}
	if meta, ok := commandtools.ToolMetadataForAlias(toolName); ok && meta != nil {
		return s.commandService.Definition(meta.CommandName)
	}
	if meta, ok := commandtools.ToolMetadataForCommand(toolName); ok && meta != nil {
		return s.commandService.Definition(meta.CommandName)
	}
	if def, ok := s.commandService.Definition(toolName); ok {
		return def, true
	}
	for _, def := range s.commandService.ToolDefinitions() {
		if def.Tool != nil && strings.TrimSpace(def.Tool.Alias) == toolName {
			return def, true
		}
	}
	return InternalCommandDefinition{}, false
}

func (s *CommandBarService) executableReadOnlyToolCards(access CommandBarChatAccess) []commandBarPlannerToolCard {
	if s == nil || s.commandService == nil {
		return nil
	}
	defs := s.commandService.ToolDefinitions()
	cards := make([]commandBarPlannerToolCard, 0)
	for _, def := range defs {
		if def.Mutating || def.Tool == nil || !commandBarAccessAllowsModule(access, def.Module) {
			continue
		}
		cards = append(cards, commandBarPlannerToolCard{
			Name:        def.Tool.Alias,
			Category:    def.Tool.Category,
			Description: def.Tool.Description,
			Mutation:    false,
			InputSchema: def.Tool.InputSchema,
		})
	}
	return cards
}

func commandBarAccessAllowsModule(access CommandBarChatAccess, module string) bool {
	switch strings.ToLower(strings.TrimSpace(module)) {
	case "", "workspace":
		return true
	case "pm":
		return access.CanReadPM
	case "docs":
		return access.CanReadDocs
	case "crm":
		return access.CanReadCRM
	default:
		return false
	}
}

func fallbackInlineReadOnlyAnswer(text string, pageContext model.CommandBarPageContext) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if containsAny(lower, "setting", "settings", "configure", "configuration") {
		return "You can usually manage this from workspace Settings. Use the sidebar to open Settings, then choose the relevant section such as General, Members, Teams, Workflows, Project Delivery, Support, CRM, or Automations."
	}
	if containsAny(lower, "doc", "document", "article", "knowledge") {
		return "I can help with docs questions from this chat. For live ranking or searching across documents, I can use read-only document tools so the answer is based on current workspace data."
	}
	if containsAny(lower, "crm", "deal", "contact", "company") {
		return "I can answer general CRM workflow questions inline. For live deal/contact counts, ranking, or buyer-signal analysis, I can use read-only CRM tools against the current workspace data."
	}
	target := strings.TrimSpace(pageContext.DisplayTitle)
	if target == "" {
		target = strings.TrimSpace(pageContext.EntityType)
	}
	return fmt.Sprintf("I can answer simple read-only questions here. For this request%s, live workspace data may be needed; I can use read-only tools when they are available.", inlineTargetPhrase(target))
}

func inlineTargetPhrase(target string) string {
	if strings.TrimSpace(target) == "" {
		return ""
	}
	return " on " + strings.TrimSpace(target)
}

func fullCommandBarChatAccess() CommandBarChatAccess {
	return CommandBarChatAccess{
		CanReadPM:   true,
		CanReadDocs: true,
		CanReadCRM:  true,
		ActorRole:   "owner",
	}
}

func shouldCreateReusableAgentFromChat(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return containsAny(lower,
		"create an agent",
		"create agent",
		"make an agent",
		"make agent",
		"build an agent",
		"save an agent",
		"reusable agent",
		"agent that",
		"agent to",
	)
}

func shouldCreateAndRunReusableAgentFromChat(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return shouldCreateReusableAgentFromChat(text) && containsAny(lower, "and run", "then run", "run it", "start it")
}

func commandBarPlanProposalContent(plan model.CommandBarPlan) string {
	count := len(plan.Steps)
	if count == 1 {
		step := plan.Steps[0]
		return fmt.Sprintf("I found a plan: run %s on %s. Review it before starting.", firstNonEmptyString(step.AgentName, "this agent"), firstNonEmptyString(step.Target.DisplayTitle, step.Target.EntityType))
	}
	return fmt.Sprintf("I found a %d-step plan. Review the chain before starting.", count)
}

func commandBarShouldUseRunPlannerBeforeInline(text string, pageContext model.CommandBarPageContext) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if normalizeCommandBarTargetType(pageContext.EntityType) == "repository" {
		return true
	}
	if containsAny(lower, "commit", "commits", "changelog", "change log", "release notes", "git history", "git log") &&
		containsAny(lower, "repository", "repo", "git", "one shot", "one-shot", "agent", "create doc", "create document", "docs") {
		return true
	}
	return false
}

func (s *CommandBarService) commandBarRepositoryTargetClarification(ctx context.Context, workspaceID, reason string) (*model.CommandBarProposal, string, bool) {
	if !commandBarReasonNeedsRepositoryTarget(reason) {
		return nil, "", false
	}
	choices, err := s.commandBarRepositoryChoices(ctx, workspaceID)
	if err != nil || len(choices) == 0 {
		return nil, "", false
	}
	lines := []string{"Choose a repository target to run this on:"}
	for i, choice := range choices {
		label := strings.TrimSpace(choice.FullName)
		if choice.DefaultBranch != "" {
			label += " (" + choice.DefaultBranch + ")"
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, label))
	}
	lines = append(lines, "Reply with the repository full name or number.")
	content := strings.Join(lines, "\n")
	raw, _ := json.Marshal(commandBarRepositoryChoiceContext{
		Mode:         "repository_target_choices",
		Repositories: choices,
	})
	return &model.CommandBarProposal{
		Type:    model.CommandBarProposalClarification,
		Answer:  content,
		Reason:  strings.TrimSpace(reason),
		Context: raw,
	}, content, true
}

func commandBarReasonNeedsRepositoryTarget(reason string) bool {
	lower := strings.ToLower(strings.TrimSpace(reason))
	return containsAny(lower, "repository", "repo") && containsAny(lower, "target", "needs", "name")
}

func (s *CommandBarService) commandBarRepositoryChoices(ctx context.Context, workspaceID string) ([]commandBarRepositoryChoice, error) {
	if s == nil || s.agentService == nil || s.agentService.gitService == nil {
		return nil, nil
	}
	repos, err := s.agentService.gitService.ListRepositories(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	choices := make([]commandBarRepositoryChoice, 0, len(repos))
	for _, repo := range repos {
		choices = append(choices, commandBarRepositoryChoice{
			ID:            strings.TrimSpace(repo.ID),
			FullName:      strings.TrimSpace(repo.FullName),
			Provider:      strings.TrimSpace(repo.Provider),
			DefaultBranch: strings.TrimSpace(repo.DefaultBranch),
		})
	}
	slices.SortFunc(choices, func(a, b commandBarRepositoryChoice) int {
		return strings.Compare(strings.ToLower(a.FullName), strings.ToLower(b.FullName))
	})
	if len(choices) > 10 {
		choices = choices[:10]
	}
	return choices, nil
}

func commandBarCreateAgentRequestFromDraft(workspaceID string, draft model.CustomAgentDraft, req model.ConfirmCommandBarChatProposalRequest) model.CreateAgentRequest {
	name := strings.TrimSpace(draft.Name)
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}
	role := strings.TrimSpace(draft.Role)
	if req.Description != nil && strings.TrimSpace(*req.Description) != "" {
		role = strings.TrimSpace(*req.Description)
	}
	if role == "" {
		role = "Custom agent created from Ask Agents."
	}
	allowedTools := append([]string(nil), draft.AllowedTools...)
	if len(req.AllowedTools) > 0 {
		allowedTools = normalizeStringSlice(req.AllowedTools)
	}
	allowedTargets := append([]string(nil), draft.AllowedTargets...)
	if len(req.AllowedTargets) > 0 {
		allowedTargets = normalizeStringSlice(req.AllowedTargets)
	}
	runtimeKind := firstNonEmptyString(strings.TrimSpace(draft.RuntimeKind), "native_sdk")
	provider := strings.TrimSpace(draft.Provider)
	modelName := strings.TrimSpace(draft.Model)
	approvalMode := firstNonEmptyString(strings.TrimSpace(draft.ApprovalMode), "always")
	invocationMode := firstNonEmptyString(strings.TrimSpace(draft.DefaultInvocationMode), "interactive")
	maxRuns := draft.MaxConcurrentRuns
	if maxRuns <= 0 {
		maxRuns = 1
	}
	systemPrompt := strings.TrimSpace(draft.SystemPrompt)
	triggerMode := "manual"
	return model.CreateAgentRequest{
		WorkspaceID:           workspaceID,
		Name:                  firstNonEmptyString(name, "Custom Agent"),
		Role:                  role,
		RuntimeKind:           &runtimeKind,
		Provider:              &provider,
		Model:                 &modelName,
		SystemPrompt:          &systemPrompt,
		Skills:                draft.Skills,
		TriggerMode:           &triggerMode,
		AllowedTools:          mustJSONStringSlice(allowedTools),
		AllowedTargets:        mustJSONStringSlice(allowedTargets),
		ApprovalMode:          &approvalMode,
		MaxConcurrentRuns:     &maxRuns,
		DefaultInvocationMode: &invocationMode,
	}
}

func optionalActorID(actorID string) *string {
	if strings.TrimSpace(actorID) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(actorID)
	return &trimmed
}

func commandBarThreadOwnedByActor(thread *model.CommandBarThread, actorID string) bool {
	if thread == nil {
		return false
	}
	if strings.TrimSpace(actorID) == "" {
		return thread.ActorID == nil
	}
	return thread.ActorID != nil && strings.TrimSpace(*thread.ActorID) == strings.TrimSpace(actorID)
}

func commandBarThreadSummary(thread model.CommandBarThread) model.CommandBarThreadSummary {
	return model.CommandBarThreadSummary{
		ID:          thread.ID,
		WorkspaceID: thread.WorkspaceID,
		ActorID:     thread.ActorID,
		Title:       thread.Title,
		Status:      thread.Status,
		CreatedAt:   thread.CreatedAt,
		UpdatedAt:   thread.UpdatedAt,
	}
}

func commandBarMessageSummary(message model.CommandBarMessage) model.CommandBarMessageSummary {
	var pageContext model.CommandBarPageContext
	if len(message.PageContext) > 0 {
		_ = json.Unmarshal(message.PageContext, &pageContext)
	}
	proposal, _ := decodeCommandBarProposal(message.ProposalJSON)
	return model.CommandBarMessageSummary{
		ID:          message.ID,
		ThreadID:    message.ThreadID,
		Role:        message.Role,
		Content:     message.Content,
		PageContext: pageContext,
		Proposal:    proposal,
		CreatedAt:   message.CreatedAt,
	}
}

func decodeCommandBarProposal(raw json.RawMessage) (*model.CommandBarProposal, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var proposal model.CommandBarProposal
	if err := json.Unmarshal(raw, &proposal); err != nil {
		return nil, fmt.Errorf("decode command bar proposal: %w", err)
	}
	return &proposal, nil
}

func (s *CommandBarService) ParseIntent(ctx context.Context, workspaceID, actorID string, req model.CommandBarParseRequest) (*model.CommandBarParseResponse, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	pageContext := normalizeCommandBarPageContext(req.PageContext, workspaceID)
	if err := validateCommandBarSupportedTarget(pageContext.EntityType); err != nil {
		resp := s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, nil, err.Error())
		return resp, nil
	}

	agents, err := s.agentService.ListAgents(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if explicitTarget, ok, err := s.resolveCommandBarExplicitTarget(ctx, workspaceID, text); err != nil {
		resp := s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, commandBarAllAgentCandidates(agents), err.Error())
		return resp, nil
	} else if ok {
		pageContext = explicitTarget
	}
	allCandidates := commandBarAllAgentCandidates(agents)
	if parsed := s.parseEpicTaskPipelineIntent(ctx, workspaceID, text, pageContext, agents); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, allCandidates), nil
	}
	if parsed := parseExplicitNamedAgents(text, pageContext, allCandidates); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, allCandidates), nil
	}
	candidates := commandBarCandidatesForTarget(agents, pageContext.EntityType)
	if len(candidates) == 0 {
		reason := fmt.Sprintf("No available agent can run on %s.", pageContext.EntityType)
		resp := s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, nil, reason)
		return resp, nil
	}
	narrowCandidates := commandBarNarrowCandidates(candidates)

	if parsed := parseFanOutIntent(text, pageContext, narrowCandidates, candidates); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, candidates), nil
	}
	if parsed := parseExplicitNamedAgents(text, pageContext, candidates); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, candidates), nil
	}
	if parsed := parseTaskPhaseSequenceIntent(text, pageContext, candidates); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, candidates), nil
	}
	var llmNoMatch *model.CommandBarParseResponse
	if parsed := s.parseIntentWithLLM(ctx, text, pageContext, candidates); parsed != nil {
		if parsed.Status == model.CommandBarParseStatusPlan {
			return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, candidates), nil
		}
		llmNoMatch = parsed
		if strings.HasPrefix(strings.TrimSpace(parsed.Reason), commandBarPlannerValidationReasonPrefix) {
			return s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, candidates, strings.TrimPrefix(strings.TrimSpace(parsed.Reason), commandBarPlannerValidationReasonPrefix)), nil
		}
	}
	if parsed := parsePreferredOneShotCommandIntent(text, pageContext, candidates); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, candidates), nil
	}
	if parsed := parseIntentDeterministically(text, pageContext, narrowCandidates); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, candidates), nil
	}
	if parsed := parseOneShotCommandIntent(text, pageContext, candidates); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, candidates), nil
	}
	if parsed := parseSafeOneShotCommandFallback(text, pageContext, candidates); parsed != nil {
		return s.commandBarResolvePlanOrNoMatch(ctx, workspaceID, actorID, text, pageContext, parsed, agents, candidates), nil
	}

	reason := "No available agent matched this request with enough confidence."
	if llmNoMatch != nil && strings.TrimSpace(llmNoMatch.Reason) != "" {
		reason = llmNoMatch.Reason
	}
	return s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, candidates, reason), nil
}

func (s *CommandBarService) commandBarResolvePlanOrNoMatch(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, parsed *model.CommandBarParseResponse, agents []model.Agent, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if parsed == nil || parsed.Status != model.CommandBarParseStatusPlan || parsed.Plan == nil {
		return parsed
	}
	resolved, err := s.resolveCommandBarPlanTargets(ctx, workspaceID, text, pageContext, parsed.Plan.Steps, agents)
	if err != nil {
		return s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, candidates, err.Error())
	}
	parsed.Plan.Steps = resolved
	parsed.Plan.RunCount = len(resolved)
	parsed.Plan.EstimatedRuns = len(resolved)
	return parsed
}

func (s *CommandBarService) resolveCommandBarPlanTargets(ctx context.Context, workspaceID, text string, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep, agents []model.Agent) ([]model.CommandBarPlanStep, error) {
	byID := make(map[string]model.Agent, len(agents))
	for _, agent := range agents {
		byID[agent.ID] = agent
	}
	resolved := make([]model.CommandBarPlanStep, 0, len(steps))
	for i, step := range steps {
		target := normalizeCommandBarPageContext(step.Target, workspaceID)
		if strings.TrimSpace(step.Target.EntityType) == "" || strings.TrimSpace(step.Target.EntityID) == "" {
			target = pageContext
		}
		agent, ok := byID[step.AgentID]
		if !ok {
			return nil, fmt.Errorf("agent not found for step %d", i+1)
		}
		allowedTargets := parseJSONStringSlice(agent.AllowedTargets)
		if requiredTargets := commandBarRequiredTargetTypesForStep(step, agent); len(requiredTargets) > 0 {
			allowedTargets = commandBarIntersectTargetTypes(allowedTargets, requiredTargets)
			if len(allowedTargets) == 0 {
				return nil, fmt.Errorf("%s cannot run with the selected tools on any supported target.", commandBarStepAgentName(step, agent))
			}
		}
		if len(allowedTargets) == 0 {
			step.Target = target
			resolved = append(resolved, step)
			continue
		}
		allowedTargets = normalizeCommandBarTargetTypes(allowedTargets)
		if commandBarTargetAllowedForPrompt(target, allowedTargets, text, step) {
			validatedTarget, err := s.commandBarValidatePlanTargetIfConfigured(ctx, workspaceID, target)
			if err != nil {
				return nil, err
			}
			target = validatedTarget
			step.Target = commandBarFillTargetDisplayTitle(target)
			resolved = append(resolved, step)
			continue
		}
		if inferred, ok, ambiguous := inferCommandBarRelatedTarget(pageContext, allowedTargets); ok {
			step.Target = inferred
			resolved = append(resolved, step)
			continue
		} else if ambiguous {
			return nil, fmt.Errorf("I found multiple possible targets for %s. Please name the exact target to run it on.", commandBarStepAgentName(step, agent))
		}
		return nil, fmt.Errorf("%s needs a %s target. Please name the target to run it on.", commandBarStepAgentName(step, agent), commandBarAllowedTargetPhrase(allowedTargets))
	}
	return resolved, nil
}

func (s *CommandBarService) resolveCommandBarExplicitTarget(ctx context.Context, workspaceID, text string) (model.CommandBarPageContext, bool, error) {
	if target, ok, err := s.resolveCommandBarTypedExplicitTarget(ctx, workspaceID, text); ok || err != nil {
		return target, ok, err
	}
	if target, ok, err := s.resolveCommandBarNamedRepositoryTarget(ctx, workspaceID, text); ok || err != nil {
		return target, ok, err
	}
	matches := commandBarTaskKeyPattern.FindAllStringSubmatch(strings.ToUpper(text), -1)
	if len(matches) == 0 {
		return model.CommandBarPageContext{}, false, nil
	}
	type taskKeyRef struct {
		key       string
		prefix    string
		displayID int
	}
	refsByKey := map[string]taskKeyRef{}
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		displayID, err := strconv.Atoi(match[2])
		if err != nil || displayID <= 0 {
			continue
		}
		key := match[1] + "-" + strconv.Itoa(displayID)
		refsByKey[key] = taskKeyRef{key: key, prefix: match[1], displayID: displayID}
	}
	if len(refsByKey) == 0 {
		return model.CommandBarPageContext{}, false, nil
	}
	if len(refsByKey) > 1 {
		return model.CommandBarPageContext{}, true, fmt.Errorf("Please name one task key to run this on.")
	}
	var ref taskKeyRef
	for _, candidate := range refsByKey {
		ref = candidate
	}
	if s == nil || s.agentService == nil || s.agentService.taskService == nil {
		return model.CommandBarPageContext{}, true, fmt.Errorf("Task key resolution is not configured.")
	}
	taskService := s.agentService.taskService
	if taskService.workspaceRepo != nil {
		workspace, err := taskService.workspaceRepo.GetByID(ctx, workspaceID)
		if err != nil {
			return model.CommandBarPageContext{}, true, err
		}
		if workspace != nil && !strings.EqualFold(strings.TrimSpace(workspace.WorkspaceKey), ref.prefix) {
			return model.CommandBarPageContext{}, true, fmt.Errorf("Task %s is not in this workspace. Please name a task from the current workspace.", ref.key)
		}
	}
	detail, err := taskService.GetByDisplayID(ctx, workspaceID, ref.displayID)
	if err != nil {
		return model.CommandBarPageContext{}, true, fmt.Errorf("I could not find task %s in this workspace.", ref.key)
	}
	task := detail.Task
	taskKey := strings.TrimSpace(task.TaskKey)
	if taskKey == "" {
		taskKey = ref.key
	}
	return model.CommandBarPageContext{
		EntityType:   "task",
		EntityID:     task.ID,
		DisplayTitle: strings.TrimSpace(taskKey + ": " + strings.TrimSpace(task.Name)),
	}, true, nil
}

func (s *CommandBarService) resolveCommandBarTypedExplicitTarget(ctx context.Context, workspaceID, text string) (model.CommandBarPageContext, bool, error) {
	matches := commandBarTypedUUIDPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return model.CommandBarPageContext{}, false, nil
	}
	type typedRef struct {
		targetType string
		id         string
	}
	refsByKey := map[string]typedRef{}
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		targetType := normalizeCommandBarTargetType(match[1])
		id := strings.TrimSpace(match[2])
		if _, err := uuid.Parse(id); err != nil {
			continue
		}
		switch targetType {
		case "task", "document", "epic", "crm_deal", "crm_contact", "repository":
		default:
			continue
		}
		refsByKey[targetType+"|"+id] = typedRef{targetType: targetType, id: id}
	}
	if len(refsByKey) == 0 {
		return model.CommandBarPageContext{}, false, nil
	}
	if len(refsByKey) > 1 {
		return model.CommandBarPageContext{}, true, fmt.Errorf("Please name one target to run this on.")
	}
	var ref typedRef
	for _, candidate := range refsByKey {
		ref = candidate
	}
	target, err := s.commandBarValidateExplicitTarget(ctx, workspaceID, ref.targetType, ref.id)
	if err != nil {
		return model.CommandBarPageContext{}, true, err
	}
	return target, true, nil
}

func (s *CommandBarService) resolveCommandBarNamedRepositoryTarget(ctx context.Context, workspaceID, text string) (model.CommandBarPageContext, bool, error) {
	if !containsAny(strings.ToLower(strings.TrimSpace(text)), "repository", "repo", "git") {
		return model.CommandBarPageContext{}, false, nil
	}
	if s == nil || s.agentService == nil || s.agentService.gitService == nil {
		return model.CommandBarPageContext{}, false, nil
	}
	repos, err := s.agentService.gitService.ListRepositories(ctx, workspaceID)
	if err != nil {
		return model.CommandBarPageContext{}, true, err
	}
	fullMatches := make([]model.GitRepository, 0, 1)
	nameMatches := make([]model.GitRepository, 0, 1)
	for _, repo := range repos {
		fullName := strings.TrimSpace(repo.FullName)
		if fullName == "" {
			continue
		}
		if commandBarTextContainsRepositoryRef(text, fullName) {
			fullMatches = append(fullMatches, repo)
			continue
		}
		baseName := fullName
		if slash := strings.LastIndex(baseName, "/"); slash >= 0 {
			baseName = baseName[slash+1:]
		}
		if baseName != fullName && commandBarTextContainsRepositoryRef(text, baseName) {
			nameMatches = append(nameMatches, repo)
		}
	}
	matches := fullMatches
	if len(matches) == 0 {
		matches = nameMatches
	}
	if len(matches) == 0 {
		return model.CommandBarPageContext{}, false, nil
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, repo := range matches {
			names = append(names, strings.TrimSpace(repo.FullName))
		}
		slices.Sort(names)
		return model.CommandBarPageContext{}, true, fmt.Errorf("Multiple repositories match this request: %s. Please name the owner/repo.", strings.Join(names, ", "))
	}
	return commandBarRepositoryPageContext(matches[0]), true, nil
}

func commandBarTextContainsRepositoryRef(text, ref string) bool {
	normalizedText := " " + commandBarNormalizeRepositoryRefText(text) + " "
	normalizedRef := commandBarNormalizeRepositoryRefText(ref)
	if normalizedRef == "" {
		return false
	}
	return strings.Contains(normalizedText, " "+normalizedRef+" ")
}

func commandBarNormalizeRepositoryRefText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastSpace := true
	for _, r := range value {
		keep := (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '/' ||
			r == '.' ||
			r == '_' ||
			r == '-'
		if keep {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func (s *CommandBarService) commandBarValidateExplicitTarget(ctx context.Context, workspaceID, targetType, targetID string) (model.CommandBarPageContext, error) {
	targetType = normalizeCommandBarTargetType(targetType)
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return model.CommandBarPageContext{}, fmt.Errorf("target id is required")
	}
	switch targetType {
	case "document":
		var doc *model.DocsDocument
		var err error
		if s != nil && s.docsDocumentService != nil {
			doc, err = s.docsDocumentService.Get(ctx, targetID)
		} else if s != nil && s.agentService != nil && s.agentService.docsDocumentRepo != nil {
			doc, err = s.agentService.docsDocumentRepo.GetByID(ctx, targetID)
		} else {
			return model.CommandBarPageContext{}, fmt.Errorf("Document target resolution is not configured.")
		}
		if err != nil {
			return model.CommandBarPageContext{}, err
		}
		if doc == nil || doc.WorkspaceID != workspaceID {
			return model.CommandBarPageContext{}, fmt.Errorf("I could not find document %s in this workspace.", targetID)
		}
		return commandBarFillTargetDisplayTitle(model.CommandBarPageContext{EntityType: "document", EntityID: doc.ID, DisplayTitle: doc.Title}), nil
	case "task":
		if s == nil || s.agentService == nil || s.agentService.taskService == nil {
			return model.CommandBarPageContext{}, fmt.Errorf("Task target resolution is not configured.")
		}
		detail, err := s.agentService.taskService.GetByID(ctx, targetID)
		if err != nil {
			return model.CommandBarPageContext{}, err
		}
		if detail == nil || detail.Task.WorkspaceID != workspaceID {
			return model.CommandBarPageContext{}, fmt.Errorf("I could not find task %s in this workspace.", targetID)
		}
		title := strings.TrimSpace(detail.Task.TaskKey + ": " + strings.TrimSpace(detail.Task.Name))
		return commandBarFillTargetDisplayTitle(model.CommandBarPageContext{EntityType: "task", EntityID: detail.Task.ID, DisplayTitle: title}), nil
	case "epic":
		if s == nil || s.agentService == nil || s.agentService.epicRepo == nil {
			return model.CommandBarPageContext{}, fmt.Errorf("Epic target resolution is not configured.")
		}
		epic, err := s.agentService.epicRepo.GetByID(ctx, targetID)
		if err != nil {
			return model.CommandBarPageContext{}, err
		}
		if epic == nil || epic.Epic.WorkspaceID != workspaceID {
			return model.CommandBarPageContext{}, fmt.Errorf("I could not find epic %s in this workspace.", targetID)
		}
		return commandBarFillTargetDisplayTitle(model.CommandBarPageContext{EntityType: "epic", EntityID: epic.Epic.ID, DisplayTitle: epic.Epic.Name}), nil
	case "crm_deal":
		var deal *model.CRMDeal
		var err error
		if s != nil && s.crmDealService != nil {
			deal, err = s.crmDealService.GetByID(ctx, targetID)
		} else if s != nil && s.agentService != nil && s.agentService.crmDealRepo != nil {
			deal, err = s.agentService.crmDealRepo.GetByID(ctx, targetID)
		} else {
			return model.CommandBarPageContext{}, fmt.Errorf("Deal target resolution is not configured.")
		}
		if err != nil {
			return model.CommandBarPageContext{}, err
		}
		if deal == nil || deal.WorkspaceID != workspaceID {
			return model.CommandBarPageContext{}, fmt.Errorf("I could not find deal %s in this workspace.", targetID)
		}
		return commandBarFillTargetDisplayTitle(model.CommandBarPageContext{EntityType: "crm_deal", EntityID: deal.ID, DisplayTitle: deal.Name}), nil
	case "crm_contact":
		var contact *model.CRMContact
		var err error
		if s != nil && s.crmContactService != nil {
			contact, err = s.crmContactService.GetByID(ctx, targetID)
		} else if s != nil && s.agentService != nil && s.agentService.crmContactRepo != nil {
			contact, err = s.agentService.crmContactRepo.GetByID(ctx, targetID)
		} else {
			return model.CommandBarPageContext{}, fmt.Errorf("Contact target resolution is not configured.")
		}
		if err != nil {
			return model.CommandBarPageContext{}, err
		}
		if contact == nil || contact.WorkspaceID != workspaceID {
			return model.CommandBarPageContext{}, fmt.Errorf("I could not find contact %s in this workspace.", targetID)
		}
		return commandBarFillTargetDisplayTitle(model.CommandBarPageContext{EntityType: "crm_contact", EntityID: contact.ID, DisplayTitle: crmContactDisplayName(contact)}), nil
	case "repository":
		if s == nil || s.agentService == nil || s.agentService.gitService == nil {
			return model.CommandBarPageContext{}, fmt.Errorf("Repository target resolution is not configured.")
		}
		repo, err := s.agentService.gitService.GetRepositoryByID(ctx, workspaceID, targetID)
		if err != nil {
			return model.CommandBarPageContext{}, err
		}
		if repo == nil || repo.WorkspaceID != workspaceID {
			return model.CommandBarPageContext{}, fmt.Errorf("I could not find repository %s in this workspace.", targetID)
		}
		if repo.Archived || !repo.Selected {
			return model.CommandBarPageContext{}, fmt.Errorf("Repository %s is not available for agent runs.", repo.FullName)
		}
		return commandBarRepositoryPageContext(*repo), nil
	default:
		return model.CommandBarPageContext{}, fmt.Errorf("Unsupported target type %q.", targetType)
	}
}

func commandBarRepositoryPageContext(repo model.GitRepository) model.CommandBarPageContext {
	return commandBarFillTargetDisplayTitle(model.CommandBarPageContext{
		EntityType:   "repository",
		EntityID:     strings.TrimSpace(repo.ID),
		DisplayTitle: strings.TrimSpace(repo.FullName),
		Metadata: map[string]interface{}{
			"provider":       strings.TrimSpace(repo.Provider),
			"full_name":      strings.TrimSpace(repo.FullName),
			"default_branch": strings.TrimSpace(repo.DefaultBranch),
		},
	})
}

func (s *CommandBarService) commandBarValidatePlanTargetIfConfigured(ctx context.Context, workspaceID string, target model.CommandBarPageContext) (model.CommandBarPageContext, error) {
	target = commandBarFillTargetDisplayTitle(target)
	targetType := normalizeCommandBarTargetType(target.EntityType)
	targetID := strings.TrimSpace(target.EntityID)
	if targetType == "" || targetID == "" || targetType == "workspace" {
		return target, nil
	}
	switch targetType {
	case "document":
		if s == nil || (s.docsDocumentService == nil && (s.agentService == nil || s.agentService.docsDocumentRepo == nil)) {
			return target, nil
		}
	case "task":
		if s == nil || s.agentService == nil || s.agentService.taskService == nil {
			return target, nil
		}
	case "epic":
		if s == nil || s.agentService == nil || s.agentService.epicRepo == nil {
			return target, nil
		}
	case "crm_deal":
		if s == nil || (s.crmDealService == nil && (s.agentService == nil || s.agentService.crmDealRepo == nil)) {
			return target, nil
		}
	case "crm_contact":
		if s == nil || (s.crmContactService == nil && (s.agentService == nil || s.agentService.crmContactRepo == nil)) {
			return target, nil
		}
	case "repository":
		if s == nil || s.agentService == nil || s.agentService.gitService == nil {
			return target, nil
		}
	default:
		return target, nil
	}
	return s.commandBarValidateExplicitTarget(ctx, workspaceID, targetType, targetID)
}

func crmContactDisplayName(contact *model.CRMContact) string {
	if contact == nil {
		return "Contact"
	}
	name := strings.TrimSpace(contact.FirstName + " " + strings.TrimSpace(stringValue(contact.LastName)))
	if name != "" {
		return name
	}
	if contact.Email != nil && strings.TrimSpace(*contact.Email) != "" {
		return strings.TrimSpace(*contact.Email)
	}
	return "Contact " + shortCommandBarID(contact.ID)
}

func commandBarTargetAllowedForPrompt(target model.CommandBarPageContext, allowedTargets []string, text string, step model.CommandBarPlanStep) bool {
	targetType := normalizeCommandBarTargetType(target.EntityType)
	if targetType == "" || strings.TrimSpace(target.EntityID) == "" || !slices.Contains(allowedTargets, targetType) {
		return false
	}
	if targetType == "workspace" && step.PlanKind != model.CommandBarPlanKindOneShotCommand && commandBarAllowedTargetsIncludeNarrowerTarget(allowedTargets) && !commandBarPromptExplicitlyTargetsWorkspace(text) {
		return false
	}
	return true
}

func commandBarAllowedTargetsIncludeNarrowerTarget(allowedTargets []string) bool {
	for _, targetType := range allowedTargets {
		if normalizeCommandBarTargetType(targetType) != "workspace" {
			return true
		}
	}
	return false
}

func commandBarRequiredTargetTypesForStep(step model.CommandBarPlanStep, agent model.Agent) []string {
	if normalizePresetKey(agent.PresetKey) != model.AgentPresetCommandAgent {
		return nil
	}
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand && step.PlanKind != model.CommandBarPlanKindDAG {
		return nil
	}
	required := []string{}
	for _, tool := range normalizeStringSlice(step.AllowedTools) {
		switch tool {
		case "publish_document_change_proposal", "write_document_content", "update_document_block", "link_document_to_object":
			required = append(required, "document")
		case "get_task_context", "add_task_comment":
			required = append(required, "task")
		case "add_deal_note", "update_deal_stage":
			required = append(required, "crm_deal")
		case "enrich_crm_contact", "ensure_crm_contact_company":
			required = append(required, "crm_contact")
		case "list_commits",
			"read_file",
			"read_file_range",
			"read_files",
			"list_directory",
			"search_files",
			"ripgrep",
			"grep",
			"list_symbols":
			required = append(required, "repository")
		}
	}
	return normalizeCommandBarTargetTypes(required)
}

func commandBarIntersectTargetTypes(base, required []string) []string {
	required = normalizeCommandBarTargetTypes(required)
	if len(required) == 0 {
		return normalizeCommandBarTargetTypes(base)
	}
	if len(base) == 0 {
		return required
	}
	base = normalizeCommandBarTargetTypes(base)
	baseSet := make(map[string]bool, len(base))
	for _, targetType := range base {
		baseSet[targetType] = true
	}
	out := make([]string, 0, len(required))
	for _, targetType := range required {
		if baseSet[targetType] {
			out = append(out, targetType)
		}
	}
	return out
}

func commandBarPromptExplicitlyTargetsWorkspace(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return containsAny(lower, "workspace", "current workspace", "whole workspace", "repository", "repo")
}

func inferCommandBarRelatedTarget(pageContext model.CommandBarPageContext, allowedTargets []string) (model.CommandBarPageContext, bool, bool) {
	matches := make([]model.CommandBarPageContext, 0)
	for _, targetType := range allowedTargets {
		targetType = normalizeCommandBarTargetType(targetType)
		if targetType == "" || targetType == "workspace" {
			continue
		}
		for _, key := range commandBarRelatedTargetKeys(targetType) {
			ids := normalizeStringSlice(pageContext.RelatedIDs[key])
			if len(ids) == 1 {
				matches = append(matches, commandBarFillTargetDisplayTitle(model.CommandBarPageContext{
					EntityType:   targetType,
					EntityID:     ids[0],
					DisplayTitle: targetType,
				}))
			} else if len(ids) > 1 {
				return model.CommandBarPageContext{}, false, true
			}
		}
	}
	if len(matches) == 0 {
		return model.CommandBarPageContext{}, false, false
	}
	if len(matches) > 1 {
		return model.CommandBarPageContext{}, false, true
	}
	return matches[0], true, false
}

func commandBarRelatedTargetKeys(targetType string) []string {
	switch normalizeCommandBarTargetType(targetType) {
	case "task":
		return []string{"task_ids", "story_ids"}
	case "epic":
		return []string{"epic_ids"}
	case "document":
		return []string{"document_ids", "doc_ids"}
	case "crm_contact":
		return []string{"crm_contact_ids", "contact_ids"}
	case "crm_deal":
		return []string{"crm_deal_ids", "deal_ids"}
	default:
		return []string{targetType + "_ids"}
	}
}

func commandBarFillTargetDisplayTitle(target model.CommandBarPageContext) model.CommandBarPageContext {
	target.EntityType = normalizeCommandBarTargetType(target.EntityType)
	target.EntityID = strings.TrimSpace(target.EntityID)
	target.DisplayTitle = strings.TrimSpace(target.DisplayTitle)
	if target.DisplayTitle == "" {
		target.DisplayTitle = target.EntityType
	}
	return target
}

func commandBarStepAgentName(step model.CommandBarPlanStep, agent model.Agent) string {
	return firstNonEmptyString(strings.TrimSpace(step.AgentName), strings.TrimSpace(agent.Name), "This agent")
}

func commandBarAllowedTargetPhrase(allowedTargets []string) string {
	allowedTargets = normalizeCommandBarTargetTypes(allowedTargets)
	allowedTargets = slices.DeleteFunc(allowedTargets, func(targetType string) bool {
		return targetType == "workspace"
	})
	if len(allowedTargets) == 0 {
		return "valid"
	}
	if len(allowedTargets) == 1 {
		return allowedTargets[0]
	}
	return strings.Join(allowedTargets[:len(allowedTargets)-1], ", ") + " or " + allowedTargets[len(allowedTargets)-1]
}

func (s *CommandBarService) DispatchPlan(ctx context.Context, workspaceID, actorID string, req model.CommandBarDispatchRequest) (*model.CommandBarDispatchResponse, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	pageContext := normalizeCommandBarPageContext(req.PageContext, workspaceID)
	if len(req.Steps) == 0 {
		return nil, fmt.Errorf("at least one plan step is required")
	}
	if len(req.Steps) > maxCommandBarPlanSteps {
		return nil, fmt.Errorf("command bar plans are limited to %d steps", maxCommandBarPlanSteps)
	}

	steps := normalizeCommandBarPlanSteps(req.Steps, pageContext)
	for i, step := range steps {
		if err := validateCommandBarSupportedTarget(step.Target.EntityType); err != nil {
			return nil, err
		}
		if strings.TrimSpace(step.AgentID) == "" {
			return nil, fmt.Errorf("agent_id is required for step %d", i+1)
		}
		if step.PlanKind == model.CommandBarPlanKindOneShotCommand && len(step.AllowedTools) == 0 {
			return nil, fmt.Errorf("one-shot command step %d requires at least one enabled tool", i+1)
		}
	}
	if err := s.validateDispatchSteps(ctx, workspaceID, steps); err != nil {
		return nil, err
	}
	planID := uuid.NewString()
	if s.planRepo != nil {
		record, err := newCommandBarPlanRecord(workspaceID, actorID, planID, text, pageContext, steps)
		if err != nil {
			return nil, err
		}
		if err := s.planRepo.Create(ctx, record); err != nil {
			return nil, err
		}
	}
	planKind := commandBarPlanKindForSteps(steps)
	if planKind == model.CommandBarPlanKindTaskPipeline || planKind == model.CommandBarPlanKindDAG {
		if s.agentService.runEngine == nil {
			if s.planRepo != nil {
				_ = s.planRepo.MarkFailed(ctx, workspaceID, planID, "Temporal command-bar orchestration is not configured")
			}
			return nil, fmt.Errorf("temporal command-bar orchestration is not configured")
		}
		if _, _, err := s.agentService.runEngine.StartCommandBarPlan(ctx, temporalapp.CommandBarPlanWorkflowInput{
			WorkspaceID: workspaceID,
			ActorID:     actorID,
			PlanID:      planID,
			Prompt:      text,
			PageContext: pageContext,
			Steps:       steps,
		}); err != nil {
			if s.planRepo != nil {
				_ = s.planRepo.MarkFailed(ctx, workspaceID, planID, err.Error())
			}
			return nil, err
		}
		return &model.CommandBarDispatchResponse{
			PlanID:   planID,
			Steps:    steps,
			RunCount: len(steps),
			Runs:     []model.AgentRun{},
		}, nil
	}
	if planKind == model.CommandBarPlanKindFanOut {
		runs := make([]model.AgentRun, 0, len(steps))
		for index := range steps {
			run, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, text, pageContext, steps, index, planID, nil)
			if err != nil {
				if s.planRepo != nil {
					_ = s.planRepo.MarkFailed(ctx, workspaceID, planID, err.Error())
				}
				return nil, err
			}
			if s.planRepo != nil {
				_ = s.planRepo.SetStepRun(ctx, workspaceID, planID, index, run.ID)
			}
			runs = append(runs, *run)
		}
		return &model.CommandBarDispatchResponse{
			PlanID:   planID,
			Steps:    steps,
			RunCount: len(steps),
			Runs:     runs,
		}, nil
	}
	run, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, text, pageContext, steps, 0, planID, nil)
	if err != nil {
		if s.planRepo != nil {
			_ = s.planRepo.MarkFailed(ctx, workspaceID, planID, err.Error())
		}
		return nil, err
	}
	if s.planRepo != nil {
		_ = s.planRepo.SetStepRun(ctx, workspaceID, planID, 0, run.ID)
	}
	return &model.CommandBarDispatchResponse{
		PlanID:   planID,
		Steps:    steps,
		RunCount: len(steps),
		Runs:     []model.AgentRun{*run},
	}, nil
}

func (s *CommandBarService) validateDispatchSteps(ctx context.Context, workspaceID string, steps []model.CommandBarPlanStep) error {
	agents, err := s.agentService.ListAgents(ctx, workspaceID)
	if err != nil {
		return err
	}
	byID := make(map[string]model.Agent, len(agents))
	for _, agent := range agents {
		byID[agent.ID] = agent
	}
	if err := validateCommandBarStepDependencies(steps, maxCommandBarDAGInitialFanOut); err != nil {
		return err
	}
	for i, step := range steps {
		agent, ok := byID[step.AgentID]
		if !ok {
			return fmt.Errorf("agent not found for step %d", i+1)
		}
		if isCommandBarOrchestrationStep(step) {
			continue
		}
		if err := validateCommandBarStepTargetForAgent(step, &agent, i); err != nil {
			return err
		}
		if err := validateRunAllowedTools(step.AllowedTools, &agent); err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}
		if normalizePresetKey(agent.PresetKey) == model.AgentPresetCommandAgent {
			if step.PlanKind != model.CommandBarPlanKindOneShotCommand && step.PlanKind != model.CommandBarPlanKindDAG {
				return fmt.Errorf("command agent step %d must be dispatched as a one-shot command or DAG step", i+1)
			}
			if len(step.AllowedTools) == 0 {
				return fmt.Errorf("command agent step %d requires a narrowed tool subset", i+1)
			}
		}
	}
	return nil
}

func isCommandBarOrchestrationStep(step model.CommandBarPlanStep) bool {
	switch strings.TrimSpace(step.StepType) {
	case model.CommandBarStepTypeEnsureEpicBranch,
		model.CommandBarStepTypeMergeTaskToEpic,
		model.CommandBarStepTypeResolveMergeConflict,
		model.CommandBarStepTypeOpenEpicPullRequest:
		return true
	default:
		return false
	}
}

func validateCommandBarStepTargetForAgent(step model.CommandBarPlanStep, agent *model.Agent, stepIndex int) error {
	targetType := normalizeCommandBarTargetType(step.Target.EntityType)
	if err := validateCommandBarSupportedTarget(targetType); err != nil {
		return err
	}
	allowedTargets := parseJSONStringSlice(agent.AllowedTargets)
	if requiredTargets := commandBarRequiredTargetTypesForStep(step, *agent); len(requiredTargets) > 0 {
		allowedTargets = commandBarIntersectTargetTypes(allowedTargets, requiredTargets)
		if len(allowedTargets) == 0 {
			return fmt.Errorf("step %d selected tools are outside agent %s target allowlist", stepIndex+1, strings.TrimSpace(agent.Name))
		}
	}
	if len(allowedTargets) == 0 {
		return nil
	}
	allowedTargets = normalizeCommandBarTargetTypes(allowedTargets)
	if !slices.Contains(allowedTargets, targetType) {
		return fmt.Errorf("step %d target %q is outside agent %s target allowlist", stepIndex+1, targetType, strings.TrimSpace(agent.Name))
	}
	return nil
}

func validateCommandBarStepDependencies(steps []model.CommandBarPlanStep, maxFanOut int) error {
	for i, step := range steps {
		for _, dep := range step.DependsOnStepIndexes {
			if dep < 0 || dep >= len(steps) {
				return fmt.Errorf("step %d dependency index %d is out of range", i+1, dep)
			}
			if dep == i {
				return fmt.Errorf("step %d cannot depend on itself", i+1)
			}
		}
	}
	if commandBarHasDependencyCycle(steps) {
		return fmt.Errorf("command bar plan dependencies contain a cycle")
	}
	if maxFanOut > 0 && commandBarPlanKindForSteps(steps) == model.CommandBarPlanKindDAG {
		ready := 0
		for _, step := range steps {
			if len(step.DependsOnStepIndexes) == 0 {
				ready++
			}
		}
		if ready > maxFanOut {
			return fmt.Errorf("DAG plans are limited to %d initially runnable steps", maxFanOut)
		}
	}
	return nil
}

func commandBarHasDependencyCycle(steps []model.CommandBarPlanStep) bool {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make([]int, len(steps))
	var visit func(int) bool
	visit = func(index int) bool {
		if state[index] == visiting {
			return true
		}
		if state[index] == visited {
			return false
		}
		state[index] = visiting
		for _, dep := range steps[index].DependsOnStepIndexes {
			if dep >= 0 && dep < len(steps) && visit(dep) {
				return true
			}
		}
		state[index] = visited
		return false
	}
	for i := range steps {
		if visit(i) {
			return true
		}
	}
	return false
}

func (s *CommandBarService) ListPlans(ctx context.Context, workspaceID, actorID string, limit int) (*model.CommandBarPlanListResponse, error) {
	if s == nil || s.planRepo == nil {
		return &model.CommandBarPlanListResponse{Plans: []model.CommandBarPlanSummary{}}, nil
	}
	trimmedActor := strings.TrimSpace(actorID)
	dismissed := map[string]struct{}{}
	if s.dismissalRepo != nil && trimmedActor != "" {
		ids, err := s.dismissalRepo.ListDismissedPlanIDs(ctx, workspaceID, trimmedActor)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			dismissed[id] = struct{}{}
		}
	}
	// Pull a few extra records so dismissals don't shrink the visible window
	// below the requested limit.
	fetchLimit := limit
	if fetchLimit <= 0 || fetchLimit > 50 {
		fetchLimit = 20
	}
	if len(dismissed) > 0 {
		fetchLimit += len(dismissed)
		if fetchLimit > 50 {
			fetchLimit = 50
		}
	}
	records, err := s.planRepo.ListRecent(ctx, workspaceID, trimmedActor, fetchLimit)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.CommandBarPlanSummary, 0, len(records))
	for _, record := range records {
		if _, hidden := dismissed[record.ID]; hidden {
			continue
		}
		summary, err := s.commandBarPlanSummaryForRecord(ctx, workspaceID, record)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
		if limit > 0 && len(summaries) >= limit {
			break
		}
	}
	return &model.CommandBarPlanListResponse{Plans: summaries}, nil
}

// ListEntityPlans returns command-bar plans targeting the given entity (e.g. an
// epic), hydrated with their agent runs, regardless of which actor triggered
// them. Visibility is enforced at the route by entity-read permissions, so this
// deliberately skips the actor filter and dismissal handling used by ListPlans.
func (s *CommandBarService) ListEntityPlans(ctx context.Context, workspaceID, entityType, entityID string, limit int) (*model.CommandBarPlanListResponse, error) {
	if s == nil || s.planRepo == nil {
		return &model.CommandBarPlanListResponse{Plans: []model.CommandBarPlanSummary{}}, nil
	}
	records, err := s.planRepo.ListByEntity(ctx, workspaceID, entityType, entityID, limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.CommandBarPlanSummary, 0, len(records))
	for _, record := range records {
		summary, err := s.commandBarPlanSummaryForRecord(ctx, workspaceID, record)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return &model.CommandBarPlanListResponse{Plans: summaries}, nil
}

// DismissPlans hides the given plans from the actor's command runs rail. Only
// plans owned by the actor can be dismissed; unknown or unauthorized plan IDs
// are silently skipped so a stale client cannot enumerate other users' runs.
func (s *CommandBarService) DismissPlans(ctx context.Context, workspaceID, actorID string, planIDs []string) error {
	if s == nil || s.dismissalRepo == nil {
		return fmt.Errorf("command bar service is not configured for dismissals")
	}
	trimmedActor := strings.TrimSpace(actorID)
	if trimmedActor == "" {
		return fmt.Errorf("actor is required")
	}
	allowed := make([]string, 0, len(planIDs))
	for _, id := range planIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		plan, err := s.planRepo.GetByID(ctx, workspaceID, id)
		if err != nil {
			return err
		}
		if plan == nil || !commandBarPlanOwnedByActor(plan, trimmedActor) {
			continue
		}
		allowed = append(allowed, id)
	}
	if len(allowed) == 0 {
		return nil
	}
	return s.dismissalRepo.Dismiss(ctx, workspaceID, trimmedActor, allowed)
}

func (s *CommandBarService) GetPlan(ctx context.Context, workspaceID, actorID, planID string) (*model.CommandBarPlanDetailResponse, error) {
	if s == nil || s.planRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	record, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if record == nil || !commandBarPlanOwnedByActor(record, actorID) {
		return nil, fmt.Errorf("command bar plan not found")
	}
	summary, err := s.commandBarPlanSummaryForRecord(ctx, workspaceID, *record)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarPlanDetailResponse{Plan: summary}, nil
}

// GetWorkspacePlan loads a plan without the dock's owner gate, for
// team-actionable flows (epic-page resume/retry) where any member with
// command-bar edit permission may act on another actor's delivery. Callers
// remain responsible for per-step authorization.
func (s *CommandBarService) GetWorkspacePlan(ctx context.Context, workspaceID, planID string) (*model.CommandBarPlanDetailResponse, error) {
	if s == nil || s.planRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	record, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	summary, err := s.commandBarPlanSummaryForRecord(ctx, workspaceID, *record)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarPlanDetailResponse{Plan: summary}, nil
}

func (s *CommandBarService) commandBarPlanSummaryForRecord(ctx context.Context, workspaceID string, record model.CommandBarPlanRecord) (model.CommandBarPlanSummary, error) {
	runIDsByStep := decodeCommandBarPlanRunIDs(record.RunIDsByStep)
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, runIDs)
	if err != nil {
		return model.CommandBarPlanSummary{}, err
	}
	return commandBarPlanSummary(record, runs), nil
}

func (s *CommandBarService) CancelPlan(ctx context.Context, workspaceID, actorID, planID string) (*model.CommandBarCancelPlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	plan, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	if !commandBarPlanOwnedByActor(plan, actorID) {
		return nil, fmt.Errorf("command bar plan not found")
	}
	if err := s.planRepo.MarkCancelled(ctx, workspaceID, plan.ID); err != nil {
		return nil, err
	}
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	updatedRuns := make([]model.AgentRun, 0, len(runs))
	for _, run := range runs {
		if model.IsAgentRunActiveStatus(run.Status) {
			updated, err := s.agentService.CancelRun(ctx, workspaceID, run.ID, actorID)
			if err == nil && updated != nil {
				updatedRuns = append(updatedRuns, *updated)
				continue
			}
		}
		updatedRuns = append(updatedRuns, run)
	}
	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarCancelPlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, updatedRuns),
		Runs: updatedRuns,
	}, nil
}

func (s *CommandBarService) ResumePlan(ctx context.Context, workspaceID, actorID, planID string) (*model.CommandBarResumePlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil || s.agentService.runRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	plan, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	// Resume is intentionally NOT owner-gated: epic delivery plans surface to
	// the whole team on the epic page, and any member with command-bar edit
	// permission (enforced at the router) may revive a stalled delivery.
	if plan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	if plan.Status != model.CommandBarPlanStatusRunning {
		return nil, fmt.Errorf("only running command bar plans can be resumed")
	}
	var pageContext model.CommandBarPageContext
	if err := json.Unmarshal(plan.PageContext, &pageContext); err != nil {
		return nil, fmt.Errorf("decode command bar plan context: %w", err)
	}
	var steps []model.CommandBarPlanStep
	if err := json.Unmarshal(plan.Steps, &steps); err != nil {
		return nil, fmt.Errorf("decode command bar plan steps: %w", err)
	}
	progress, err := s.agentService.StartReadyCommandBarPlanSteps(ctx, temporalapp.CommandBarPlanWorkflowInput{
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		PlanID:      plan.ID,
		Prompt:      plan.Prompt,
		PageContext: pageContext,
		Steps:       steps,
	})
	if err != nil {
		return nil, err
	}
	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	if updatedPlan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	latestRunIDsByStep := decodeCommandBarPlanRunIDs(updatedPlan.RunIDsByStep)
	latestRunIDs := make([]string, 0, len(latestRunIDsByStep))
	for _, runID := range latestRunIDsByStep {
		latestRunIDs = append(latestRunIDs, runID)
	}
	latestRuns, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, latestRunIDs)
	if err != nil {
		return nil, err
	}
	var startedRun *model.AgentRun
	if progress != nil && len(progress.Started) > 0 {
		for i := range latestRuns {
			if latestRuns[i].ID == progress.Started[0] {
				startedRun = &latestRuns[i]
				break
			}
		}
	}
	return &model.CommandBarResumePlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, latestRuns),
		Run:  startedRun,
		Runs: latestRuns,
	}, nil
}

func (s *CommandBarService) RetryPlanFromStep(ctx context.Context, workspaceID, actorID, planID string, req model.CommandBarRetryPlanRequest) (*model.CommandBarRetryPlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil || s.agentService.runRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	plan, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	// Like ResumePlan, retry is team-actionable (not owner-gated): a failed
	// epic delivery can be retried by any member with command-bar edit
	// permission. Retried runs are attributed to the retrying actor.
	//
	// A plan can be left in status "running" with all of its child runs
	// failed or cancelled (e.g. runs cancelled mid-delivery) — a zombie that
	// resume cannot revive because there is nothing ready to start. Retry is
	// the only way out, so only reject while work is genuinely in flight.
	if plan.Status == model.CommandBarPlanStatusRunning {
		activeIDs := make([]string, 0)
		for _, runID := range decodeCommandBarPlanRunIDs(plan.RunIDsByStep) {
			activeIDs = append(activeIDs, runID)
		}
		runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, activeIDs)
		if err != nil {
			return nil, err
		}
		for _, run := range runs {
			if model.IsAgentRunActiveStatus(run.Status) {
				return nil, fmt.Errorf("command bar plan still has active runs; cancel them or wait before retrying")
			}
		}
	}
	var pageContext model.CommandBarPageContext
	if err := json.Unmarshal(plan.PageContext, &pageContext); err != nil {
		return nil, fmt.Errorf("decode command bar plan context: %w", err)
	}
	var steps []model.CommandBarPlanStep
	if err := json.Unmarshal(plan.Steps, &steps); err != nil {
		return nil, fmt.Errorf("decode command bar plan steps: %w", err)
	}
	stepIndex := req.StepIndex
	if stepIndex < 0 || stepIndex >= len(steps) {
		return nil, fmt.Errorf("step_index must be between 0 and %d", len(steps)-1)
	}
	planKind := commandBarPlanKindForSteps(steps)
	if planKind == model.CommandBarPlanKindTaskPipeline || planKind == model.CommandBarPlanKindDAG {
		return s.retryFailedDAGRuns(ctx, workspaceID, actorID, *plan, pageContext, steps)
	}
	if planKind == model.CommandBarPlanKindFanOut {
		return s.retryFailedFanOutRuns(ctx, workspaceID, actorID, *plan, pageContext, steps)
	}
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	for index, runID := range runIDsByStep {
		if index < stepIndex {
			continue
		}
		if run, err := s.agentService.runRepo.GetByID(ctx, workspaceID, runID); err == nil && run != nil && model.IsAgentRunActiveStatus(run.Status) {
			_, _ = s.agentService.CancelRun(ctx, workspaceID, run.ID, actorID)
		}
		delete(runIDsByStep, index)
	}
	var parentRunID *string
	if stepIndex > 0 {
		previousRunID := strings.TrimSpace(runIDsByStep[stepIndex-1])
		if previousRunID == "" {
			return nil, fmt.Errorf("cannot retry from step %d without a previous run", stepIndex+1)
		}
		parentRunID = &previousRunID
	}
	run, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, plan.Prompt, pageContext, steps, stepIndex, plan.ID, parentRunID)
	if err != nil {
		_ = s.planRepo.MarkFailed(ctx, workspaceID, plan.ID, err.Error())
		return nil, err
	}
	runIDsByStep[stepIndex] = run.ID
	rawRunIDs, _ := json.Marshal(runIDsByStep)
	if err := s.planRepo.RestartStepRun(ctx, workspaceID, plan.ID, stepIndex, rawRunIDs); err != nil {
		return nil, err
	}
	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarRetryPlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, []model.AgentRun{*run}),
		Run:  run,
		Runs: []model.AgentRun{*run},
	}, nil
}

func (s *CommandBarService) retryFailedDAGRuns(ctx context.Context, workspaceID, actorID string, plan model.CommandBarPlanRecord, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep) (*model.CommandBarRetryPlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil || s.agentService.runRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	if s.agentService.runEngine == nil {
		return nil, fmt.Errorf("temporal command-bar orchestration is not configured")
	}

	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	runsByID := make(map[string]model.AgentRun, len(runs))
	for _, run := range runs {
		runsByID[run.ID] = run
	}

	failedIndexes := make([]int, 0)
	for stepIndex, runID := range runIDsByStep {
		run, ok := runsByID[runID]
		if !ok {
			continue
		}
		if run.Status == model.AgentRunStatusFailed || run.Status == model.AgentRunStatusCancelled {
			failedIndexes = append(failedIndexes, stepIndex)
		}
	}
	if len(failedIndexes) == 0 {
		return nil, fmt.Errorf("no failed DAG steps to retry")
	}
	slices.Sort(failedIndexes)
	for _, stepIndex := range failedIndexes {
		delete(runIDsByStep, stepIndex)
	}
	updatedRunIDs, err := json.Marshal(runIDsByStep)
	if err != nil {
		return nil, fmt.Errorf("encode retried DAG run ids: %w", err)
	}
	if err := s.planRepo.RestartStepRun(ctx, workspaceID, plan.ID, failedIndexes[0], updatedRunIDs); err != nil {
		return nil, err
	}

	progress, err := s.agentService.StartReadyCommandBarPlanSteps(ctx, temporalapp.CommandBarPlanWorkflowInput{
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		PlanID:      plan.ID,
		Prompt:      plan.Prompt,
		PageContext: pageContext,
		Steps:       steps,
	})
	if err != nil {
		return nil, err
	}

	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	if updatedPlan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	latestRunIDsByStep := decodeCommandBarPlanRunIDs(updatedPlan.RunIDsByStep)
	latestRunIDs := make([]string, 0, len(latestRunIDsByStep))
	for _, runID := range latestRunIDsByStep {
		latestRunIDs = append(latestRunIDs, runID)
	}
	latestRuns, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, latestRunIDs)
	if err != nil {
		return nil, err
	}
	var startedRun *model.AgentRun
	if progress != nil && len(progress.Started) > 0 {
		for i := range latestRuns {
			if latestRuns[i].ID == progress.Started[0] {
				startedRun = &latestRuns[i]
				break
			}
		}
	}
	return &model.CommandBarRetryPlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, latestRuns),
		Run:  startedRun,
		Runs: latestRuns,
	}, nil
}

func (s *CommandBarService) retryFailedFanOutRuns(ctx context.Context, workspaceID, actorID string, plan model.CommandBarPlanRecord, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep) (*model.CommandBarRetryPlanResponse, error) {
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	retrySteps := make([]int, 0)
	for index, runID := range runIDsByStep {
		run, err := s.agentService.runRepo.GetByID(ctx, workspaceID, runID)
		if err != nil {
			return nil, err
		}
		if run == nil {
			continue
		}
		if model.IsAgentRunActiveStatus(run.Status) {
			return nil, fmt.Errorf("fan-out plan still has active runs; wait for them to finish or cancel the plan")
		}
		if run.Status == model.AgentRunStatusFailed || run.Status == model.AgentRunStatusCancelled {
			retrySteps = append(retrySteps, index)
		}
	}
	slices.Sort(retrySteps)
	if len(retrySteps) == 0 {
		return nil, fmt.Errorf("fan-out plan has no failed or cancelled targets to retry")
	}
	started := make([]model.AgentRun, 0, len(retrySteps))
	for _, index := range retrySteps {
		if index < 0 || index >= len(steps) {
			continue
		}
		run, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, plan.Prompt, pageContext, steps, index, plan.ID, nil)
		if err != nil {
			_ = s.planRepo.MarkFailed(ctx, workspaceID, plan.ID, err.Error())
			return nil, err
		}
		runIDsByStep[index] = run.ID
		started = append(started, *run)
	}
	rawRunIDs, _ := json.Marshal(runIDsByStep)
	if err := s.planRepo.RestartStepRun(ctx, workspaceID, plan.ID, retrySteps[0], rawRunIDs); err != nil {
		return nil, err
	}
	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	resp := &model.CommandBarRetryPlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, started),
		Runs: started,
	}
	if len(started) > 0 {
		resp.Run = &started[0]
	}
	return resp, nil
}

func (s *CommandBarService) ListUnmetIntents(ctx context.Context, workspaceID, status string, limit int, includeSensitive bool) (*model.CommandBarUnmetIntentListResponse, error) {
	if s == nil || s.unmetRepo == nil {
		return &model.CommandBarUnmetIntentListResponse{Intents: []model.CommandBarUnmetIntentSummary{}}, nil
	}
	intents, err := s.unmetRepo.List(ctx, workspaceID, strings.TrimSpace(status), limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.CommandBarUnmetIntentSummary, 0, len(intents))
	for _, intent := range intents {
		summaries = append(summaries, commandBarUnmetIntentSummary(intent, includeSensitive))
	}
	return &model.CommandBarUnmetIntentListResponse{Intents: summaries}, nil
}

func (s *CommandBarService) ReviewUnmetIntent(ctx context.Context, workspaceID, id string, req model.ReviewCommandBarUnmetIntentRequest) (*model.CommandBarUnmetIntentSummary, error) {
	if s == nil || s.unmetRepo == nil {
		return nil, fmt.Errorf("command bar unmet intent repository is not configured")
	}
	status := strings.TrimSpace(req.Status)
	switch status {
	case "open", "accepted", "rejected", "deferred":
	default:
		return nil, fmt.Errorf("unsupported unmet intent status %q", status)
	}
	intent, err := s.unmetRepo.Review(ctx, workspaceID, strings.TrimSpace(id), status, req.Notes)
	if err != nil {
		return nil, err
	}
	summary := commandBarUnmetIntentSummary(*intent, false)
	return &summary, nil
}

func (s *CommandBarService) PromoteRunToAgent(ctx context.Context, workspaceID, actorID, runID string, req model.PromoteCommandBarRunRequest) (*model.PromoteCommandBarRunResponse, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	run, err := s.agentService.runRepo.GetByID(ctx, workspaceID, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}
	payload, ok := commandBarRunPayload(run)
	if !ok {
		return nil, fmt.Errorf("only command-bar runs can be promoted")
	}
	if run.Status != model.AgentRunStatusCompleted {
		return nil, fmt.Errorf("only completed command-bar runs can be promoted")
	}
	if payload.StepIndex < 0 || payload.StepIndex >= len(payload.Steps) {
		return nil, fmt.Errorf("command-bar step metadata is invalid")
	}
	sourceAgent, err := s.agentService.agentRepo.GetByID(ctx, workspaceID, run.AgentID)
	if err != nil {
		return nil, err
	}
	if sourceAgent == nil {
		return nil, fmt.Errorf("source agent not found")
	}
	step := payload.Steps[payload.StepIndex]
	allowedTools := step.AllowedTools
	if len(allowedTools) == 0 {
		allowedTools = parseJSONStringSlice(sourceAgent.AllowedTools)
	}
	if len(req.AllowedTools) > 0 {
		allowedTools = normalizeStringSlice(req.AllowedTools)
	}
	if err := validateRunAllowedTools(allowedTools, sourceAgent); err != nil {
		return nil, err
	}
	allowedTargets := []string{strings.TrimSpace(run.TargetType)}
	if len(req.AllowedTargets) > 0 {
		allowedTargets = normalizeStringSlice(req.AllowedTargets)
	}
	if err := validatePromotedAgentTargets(allowedTargets, sourceAgent); err != nil {
		return nil, err
	}
	role := strings.TrimSpace(derefString(req.Description))
	if role == "" {
		role = fmt.Sprintf("Reusable agent promoted from command-bar run %s. Original step instruction: %s", run.ID, strings.TrimSpace(step.Instructions))
	}
	planningNotes := fmt.Sprintf("Promoted from command-bar run %s in plan %s. Source agent: %s. Source target: %s/%s.", run.ID, payload.PlanID, sourceAgent.Name, run.TargetType, run.TargetID)
	agent, err := s.agentService.CreateAgent(ctx, model.CreateAgentRequest{
		WorkspaceID:           workspaceID,
		Name:                  name,
		Role:                  role,
		RuntimeKind:           &sourceAgent.RuntimeKind,
		Provider:              sourceAgent.Provider,
		Model:                 sourceAgent.Model,
		ExecutionConfig:       json.RawMessage(sourceAgent.ExecutionConfig),
		PlanningNotes:         strPtr(planningNotes),
		AllowedTools:          mustJSONStringSlice(allowedTools),
		AllowedCommands:       sourceAgent.AllowedCommands,
		AllowedTargets:        mustJSONStringSlice(allowedTargets),
		DefaultInvocationMode: &sourceAgent.DefaultInvocationMode,
	}, actorID)
	if err != nil {
		return nil, err
	}
	return &model.PromoteCommandBarRunResponse{Agent: *agent}, nil
}

func (s *CommandBarService) GetAgentToolCatalog(ctx context.Context, workspaceID, agentID string, selectedTools []string) (*model.CommandBarToolCatalogResponse, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	agent, err := s.agentService.agentRepo.GetByID(ctx, workspaceID, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	// Tool catalog access is workspace-scoped, not command-bar-candidate scoped,
	// so admins can inspect saved agents before deciding whether to expose them.
	allowedTools := normalizeStringSlice(parseJSONStringSlice(agent.AllowedTools))
	allowedSet := make(map[string]bool, len(allowedTools))
	for _, tool := range allowedTools {
		allowedSet[tool] = true
	}
	selectedTools = normalizeStringSlice(selectedTools)
	selectedSet := make(map[string]bool, len(selectedTools))
	for _, tool := range selectedTools {
		selectedSet[tool] = true
	}
	validation := make([]string, 0)
	validationSet := map[string]bool{}
	addValidation := func(message string) {
		if validationSet[message] {
			return
		}
		validationSet[message] = true
		validation = append(validation, message)
	}
	catalog := s.agentService.ListToolCatalog()
	entries := make([]model.CommandBarToolCatalogEntry, 0, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		allowed := allowedSet[tool.Name]
		entry := model.CommandBarToolCatalogEntry{
			ID:          tool.Name,
			Name:        tool.Name,
			Description: tool.Description,
			Category:    tool.Category,
			InputSchema: tool.InputSchema,
			Allowed:     allowed,
			Selected:    selectedSet[tool.Name],
		}
		if !allowed {
			entry.DisabledReason = "Tool is outside this agent's allowlist."
			if entry.Selected {
				addValidation(fmt.Sprintf("tool %q is outside this agent's allowlist", tool.Name))
			}
		}
		entries = append(entries, entry)
	}
	for _, tool := range selectedTools {
		if !allowedSet[tool] {
			addValidation(fmt.Sprintf("tool %q is outside this agent's allowlist", tool))
		}
	}
	return &model.CommandBarToolCatalogResponse{
		AgentID:        agent.ID,
		AllowedTools:   allowedTools,
		SelectedTools:  selectedTools,
		Tools:          entries,
		Categories:     catalog.Categories,
		Validation:     validation,
		AllowedTargets: parseJSONStringSlice(agent.AllowedTargets),
	}, nil
}

func (s *AgentService) startCommandBarPlanStep(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep, stepIndex int, planID string, parentRunID *string) (*model.AgentRun, error) {
	if s == nil {
		return nil, fmt.Errorf("agent service is not configured")
	}
	if stepIndex < 0 || stepIndex >= len(steps) {
		return nil, fmt.Errorf("command bar step index %d is out of range", stepIndex)
	}
	step := steps[stepIndex]
	target := normalizeCommandBarPageContext(step.Target, workspaceID)
	if target.EntityType == "" || target.EntityID == "" {
		target = pageContext
	}
	if err := validateCommandBarSupportedTarget(target.EntityType); err != nil {
		return nil, err
	}
	agentID := strings.TrimSpace(step.AgentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required for step %d", stepIndex+1)
	}

	triggerContext, err := buildCommandBarTriggerContext(text, pageContext, steps, stepIndex, planID)
	if err != nil {
		return nil, err
	}
	additionalContext := commandBarAdditionalContext(step.Instructions, target, stepIndex, len(steps))
	event := (*model.AgentRunEventContext)(nil)
	if parentRunID != nil && strings.TrimSpace(*parentRunID) != "" {
		reason := "command_bar_next_step"
		event = &model.AgentRunEventContext{
			RunID:  parentRunID,
			Reason: &reason,
		}
	}
	actor := (*string)(nil)
	if strings.TrimSpace(actorID) != "" {
		actor = &actorID
	}
	return s.startTargetRun(ctx, workspaceID, target.EntityType, target.EntityID, model.StartAgentRunRequest{
		AgentID:           agentID,
		AdditionalContext: &additionalContext,
		AllowedTools:      step.AllowedTools,
	}, actor, triggerContext, event, parentRunID)
}

func (s *AgentService) AdvanceCommandBarPlanAfterRun(ctx context.Context, completedRunID string) (*model.AgentRun, error) {
	if s == nil || s.runRepo == nil {
		return nil, nil
	}
	completedRunID = strings.TrimSpace(completedRunID)
	if completedRunID == "" {
		return nil, nil
	}
	run, err := s.runRepo.GetByIDAny(ctx, completedRunID)
	if err != nil || run == nil {
		return nil, err
	}
	payload, ok := commandBarRunPayload(run)
	if !ok {
		return nil, nil
	}
	planKind := commandBarPlanKindForSteps(payload.Steps)
	if planKind == model.CommandBarPlanKindTaskPipeline || planKind == model.CommandBarPlanKindDAG {
		if s.runEngine != nil && payload.PlanID != "" {
			if err := s.runEngine.SignalCommandBarPlanRunCompleted(ctx, payload.PlanID, completedRunID); err == nil {
				return nil, nil
			} else {
				slog.WarnContext(ctx, "command bar plan signal failed; running scheduler fallback",
					"error", err,
					"workspace_id", run.WorkspaceID,
					"plan_id", payload.PlanID,
					"run_id", completedRunID,
				)
			}
		}
		if s.commandBarPlanRepo != nil && payload.PlanID != "" {
			if _, err := s.StartReadyCommandBarPlanSteps(ctx, temporalapp.CommandBarPlanWorkflowInput{
				WorkspaceID: run.WorkspaceID,
				ActorID:     derefString(run.TriggeredByUserID),
				PlanID:      payload.PlanID,
				Prompt:      payload.Prompt,
				PageContext: payload.PageContext,
				Steps:       payload.Steps,
			}); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if planKind == model.CommandBarPlanKindFanOut {
		return s.advanceFanOutCommandBarPlan(ctx, run, payload)
	}
	if run.Status == model.AgentRunStatusFailed {
		if s.commandBarPlanRepo != nil && payload.PlanID != "" {
			_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(run, payload, "failed"))
		}
		return nil, nil
	}
	if run.Status == model.AgentRunStatusCancelled {
		if s.commandBarPlanRepo != nil && payload.PlanID != "" {
			_ = s.commandBarPlanRepo.MarkCancelled(ctx, run.WorkspaceID, payload.PlanID)
		}
		return nil, nil
	}
	if run.Status != model.AgentRunStatusCompleted {
		return nil, nil
	}
	if payload.RunCount <= 1 || payload.StepIndex+1 >= len(payload.Steps) {
		if s.commandBarPlanRepo != nil && payload.PlanID != "" && payload.StepIndex+1 >= len(payload.Steps) {
			_ = s.commandBarPlanRepo.MarkCompleted(ctx, run.WorkspaceID, payload.PlanID)
		}
		return nil, nil
	}
	if s.commandBarPlanRepo != nil && payload.PlanID != "" {
		plan, err := s.commandBarPlanRepo.GetByID(ctx, run.WorkspaceID, payload.PlanID)
		if err != nil {
			return nil, err
		}
		if plan != nil && plan.Status == model.CommandBarPlanStatusCancelled {
			return nil, nil
		}
	}
	if existing, err := s.runRepo.FindByParentRunID(ctx, run.WorkspaceID, run.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	actorID := derefString(run.TriggeredByUserID)
	nextIndex := payload.StepIndex + 1
	planID := firstNonEmptyString(payload.PlanID, uuid.NewString())
	nextRun, err := s.startCommandBarPlanStep(ctx, run.WorkspaceID, actorID, payload.Prompt, payload.PageContext, payload.Steps, nextIndex, planID, &run.ID)
	if err != nil {
		if existing, findErr := s.runRepo.FindByParentRunID(ctx, run.WorkspaceID, run.ID); findErr == nil && existing != nil {
			if s.commandBarPlanRepo != nil && planID != "" {
				_ = s.commandBarPlanRepo.SetStepRun(ctx, run.WorkspaceID, planID, nextIndex, existing.ID)
			}
			return existing, nil
		}
		if s.commandBarPlanRepo != nil && planID != "" {
			_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, planID, err.Error())
		}
		return nil, err
	}
	if s.commandBarPlanRepo != nil && planID != "" && nextRun != nil {
		_ = s.commandBarPlanRepo.SetStepRun(ctx, run.WorkspaceID, planID, nextIndex, nextRun.ID)
	}
	return nextRun, nil
}

func (s *AgentService) StartReadyCommandBarPlanSteps(ctx context.Context, input temporalapp.CommandBarPlanWorkflowInput) (*temporalapp.CommandBarPlanProgress, error) {
	progress := &temporalapp.CommandBarPlanProgress{Status: model.CommandBarPlanStatusRunning}
	if s == nil || s.commandBarPlanRepo == nil || s.runRepo == nil {
		return &temporalapp.CommandBarPlanProgress{Terminal: true, Status: "not_configured"}, nil
	}
	plan, err := s.commandBarPlanRepo.GetByID(ctx, input.WorkspaceID, input.PlanID)
	if err != nil || plan == nil {
		return progress, err
	}
	if plan.Status == model.CommandBarPlanStatusCancelled || plan.Status == model.CommandBarPlanStatusFailed || plan.Status == model.CommandBarPlanStatusCompleted {
		return &temporalapp.CommandBarPlanProgress{Terminal: true, Status: plan.Status}, nil
	}

	steps := input.Steps
	if len(steps) == 0 {
		_ = json.Unmarshal(plan.Steps, &steps)
	}
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.runRepo.ListByIDs(ctx, input.WorkspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	runsByID := make(map[string]model.AgentRun, len(runs))
	activeCount := 0
	for _, run := range runs {
		if stepIndex := commandBarStepIndexForRun(runIDsByStep, run.ID); stepIndex >= 0 && stepIndex < len(steps) && isCommandBarOrchestrationStep(steps[stepIndex]) && model.IsAgentRunActiveStatus(run.Status) {
			updated, err := s.advanceRunningCommandBarOrchestrationRun(ctx, input, steps, stepIndex, &run)
			if err != nil {
				_ = s.commandBarPlanRepo.MarkFailed(ctx, input.WorkspaceID, input.PlanID, err.Error())
				return nil, err
			}
			if updated != nil {
				run = *updated
			}
		}
		runsByID[run.ID] = run
		if model.IsAgentRunActiveStatus(run.Status) {
			activeCount++
		}
		if run.Status == model.AgentRunStatusFailed || run.Status == model.AgentRunStatusCancelled {
			payload := commandBarTriggerContextPayload{PlanID: input.PlanID, Steps: steps, StepIndex: commandBarStepIndexForRun(runIDsByStep, run.ID)}
			_ = s.commandBarPlanRepo.MarkFailed(ctx, input.WorkspaceID, input.PlanID, commandBarTerminalRunMessage(&run, payload, string(run.Status)))
			return &temporalapp.CommandBarPlanProgress{Terminal: true, Status: model.CommandBarPlanStatusFailed}, nil
		}
	}

	allStarted := len(steps) > 0 && len(runIDsByStep) >= len(steps)
	allCompleted := allStarted
	for _, runID := range runIDsByStep {
		run, ok := runsByID[runID]
		if !ok || run.Status != model.AgentRunStatusCompleted {
			allCompleted = false
			break
		}
	}
	if allCompleted {
		_ = s.commandBarPlanRepo.MarkCompleted(ctx, input.WorkspaceID, input.PlanID)
		return &temporalapp.CommandBarPlanProgress{Terminal: true, Status: model.CommandBarPlanStatusCompleted}, nil
	}

	started := 0
	observedConcurrentProgress := false
	for index, step := range steps {
		if strings.TrimSpace(runIDsByStep[index]) != "" {
			continue
		}
		if !commandBarStepDependenciesSatisfied(step, runIDsByStep, runsByID) {
			continue
		}
		latestPlan, err := s.commandBarPlanRepo.GetByID(ctx, input.WorkspaceID, input.PlanID)
		if err != nil {
			return nil, err
		}
		if latestPlan == nil {
			return progress, nil
		}
		if latestPlan.Status == model.CommandBarPlanStatusCancelled || latestPlan.Status == model.CommandBarPlanStatusFailed || latestPlan.Status == model.CommandBarPlanStatusCompleted {
			return &temporalapp.CommandBarPlanProgress{Terminal: true, Status: latestPlan.Status}, nil
		}
		latestRunIDsByStep := decodeCommandBarPlanRunIDs(latestPlan.RunIDsByStep)
		if existingRunID := strings.TrimSpace(latestRunIDsByStep[index]); existingRunID != "" {
			runIDsByStep[index] = existingRunID
			observedConcurrentProgress = true
			if existing, err := s.runRepo.GetByID(ctx, input.WorkspaceID, existingRunID); err == nil && existing != nil {
				runsByID[existing.ID] = *existing
				if model.IsAgentRunActiveStatus(existing.Status) {
					activeCount++
				}
			}
			continue
		}
		parentRunID := commandBarParentRunIDForStep(steps, index, runIDsByStep, runsByID)
		var run *model.AgentRun
		if isCommandBarOrchestrationStep(step) {
			run, err = s.startCommandBarOrchestrationStep(ctx, input, steps, index, parentRunID)
		} else {
			run, err = s.startCommandBarPlanStep(ctx, input.WorkspaceID, input.ActorID, input.Prompt, input.PageContext, steps, index, input.PlanID, parentRunID)
		}
		if err != nil {
			if existing := s.commandBarExistingRunForStep(ctx, input.WorkspaceID, parentRunID, input.PlanID, steps, index); existing != nil {
				run = existing
			} else {
				_ = s.commandBarPlanRepo.MarkFailed(ctx, input.WorkspaceID, input.PlanID, err.Error())
				return nil, err
			}
		}
		runIDsByStep[index] = run.ID
		started++
		progress.Started = append(progress.Started, run.ID)
		if err := s.commandBarPlanRepo.SetStepRun(ctx, input.WorkspaceID, input.PlanID, index, run.ID); err != nil {
			return nil, err
		}
	}
	if started == 0 && activeCount == 0 && !observedConcurrentProgress {
		_ = s.commandBarPlanRepo.MarkFailed(ctx, input.WorkspaceID, input.PlanID, "Command-bar plan has no runnable steps; check task dependencies for a cycle or missing completed prerequisite.")
		return &temporalapp.CommandBarPlanProgress{Terminal: true, Status: model.CommandBarPlanStatusFailed}, nil
	}
	return progress, nil
}

func (s *AgentService) commandBarExistingChildRunForParent(ctx context.Context, workspaceID string, parentRunID *string) *model.AgentRun {
	if s == nil || s.runRepo == nil || parentRunID == nil || strings.TrimSpace(*parentRunID) == "" {
		return nil
	}
	existing, err := s.runRepo.FindByParentRunID(ctx, workspaceID, strings.TrimSpace(*parentRunID))
	if err != nil {
		return nil
	}
	return existing
}

func (s *AgentService) commandBarExistingRunForStep(ctx context.Context, workspaceID string, parentRunID *string, planID string, steps []model.CommandBarPlanStep, stepIndex int) *model.AgentRun {
	existing := s.commandBarExistingChildRunForParent(ctx, workspaceID, parentRunID)
	if existing == nil || !commandBarRunMatchesStep(existing, planID, steps, stepIndex) {
		return nil
	}
	return existing
}

func commandBarRunMatchesStep(run *model.AgentRun, planID string, steps []model.CommandBarPlanStep, stepIndex int) bool {
	if run == nil || stepIndex < 0 || stepIndex >= len(steps) {
		return false
	}
	step := steps[stepIndex]
	if strings.TrimSpace(run.AgentID) != strings.TrimSpace(step.AgentID) {
		return false
	}
	if normalizeCommandBarTargetType(run.TargetType) != normalizeCommandBarTargetType(step.Target.EntityType) || strings.TrimSpace(run.TargetID) != strings.TrimSpace(step.Target.EntityID) {
		return false
	}
	payload, ok := commandBarRunPayload(run)
	if !ok {
		return false
	}
	return strings.TrimSpace(payload.PlanID) == strings.TrimSpace(planID) && payload.StepIndex == stepIndex
}

func commandBarStepDependenciesSatisfied(step model.CommandBarPlanStep, runIDsByStep map[int]string, runsByID map[string]model.AgentRun) bool {
	for _, dep := range step.DependsOnStepIndexes {
		runID := strings.TrimSpace(runIDsByStep[dep])
		if runID == "" {
			return false
		}
		run, ok := runsByID[runID]
		if !ok || run.Status != model.AgentRunStatusCompleted {
			return false
		}
	}
	return true
}

func commandBarParentRunIDForStep(steps []model.CommandBarPlanStep, stepIndex int, runIDsByStep map[int]string, runsByID map[string]model.AgentRun) *string {
	if stepIndex < 0 || stepIndex >= len(steps) {
		return nil
	}
	step := steps[stepIndex]
	if len(step.DependsOnStepIndexes) != 1 {
		return nil
	}
	dependencyIndex := step.DependsOnStepIndexes[0]
	if commandBarDependencyConsumerCount(steps, dependencyIndex) != 1 {
		return nil
	}
	runID := strings.TrimSpace(runIDsByStep[dependencyIndex])
	if runID == "" {
		return nil
	}
	run, ok := runsByID[runID]
	if !ok || run.Status != model.AgentRunStatusCompleted {
		return nil
	}
	id := run.ID
	return &id
}

func commandBarDependencyConsumerCount(steps []model.CommandBarPlanStep, dependencyIndex int) int {
	count := 0
	for _, step := range steps {
		for _, dep := range step.DependsOnStepIndexes {
			if dep == dependencyIndex {
				count++
				break
			}
		}
	}
	return count
}

type commandBarOrchestrationOutput struct {
	Type                string `json:"type,omitempty"`
	Status              string `json:"status,omitempty"`
	Message             string `json:"message,omitempty"`
	EpicBranch          string `json:"epic_branch,omitempty"`
	BaseBranch          string `json:"base_branch,omitempty"`
	TaskBranch          string `json:"task_branch,omitempty"`
	ConflictRunID       string `json:"conflict_run_id,omitempty"`
	ConflictAttempt     int    `json:"conflict_attempt,omitempty"`
	FinalPullRequestURL string `json:"final_pull_request_url,omitempty"`
}

func (s *AgentService) startCommandBarOrchestrationStep(ctx context.Context, input temporalapp.CommandBarPlanWorkflowInput, steps []model.CommandBarPlanStep, stepIndex int, parentRunID *string) (*model.AgentRun, error) {
	step := steps[stepIndex]
	now := time.Now()
	triggerContext, err := buildCommandBarTriggerContext(input.Prompt, input.PageContext, steps, stepIndex, input.PlanID)
	if err != nil {
		return nil, err
	}
	runInput, err := json.Marshal(model.AgentRunInputPayload{
		Trigger: triggerContext,
		Target: &model.AgentRunTargetContext{
			TargetType: step.Target.EntityType,
			TargetID:   step.Target.EntityID,
		},
		AdditionalContext: step.Instructions,
	})
	if err != nil {
		return nil, err
	}
	run := &model.AgentRun{
		ID:                uuid.NewString(),
		WorkspaceID:       input.WorkspaceID,
		AgentID:           step.AgentID,
		TargetType:        firstNonEmptyString(step.Target.EntityType, input.PageContext.EntityType),
		TargetID:          firstNonEmptyString(step.Target.EntityID, input.PageContext.EntityID),
		RuntimeKind:       "internal",
		InvocationMode:    "autonomous",
		ParentRunID:       parentRunID,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonNone,
		TriggeredByUserID: stringPtrIfNotEmpty(input.ActorID),
		Status:            model.AgentRunStatusRunning,
		ExecutionStage:    strPtr(step.StepType),
		StartedAt:         &now,
		LastHeartbeatAt:   &now,
		Input:             runInput,
		OutputSummary:     json.RawMessage(`{}`),
	}
	if step.Target.EntityType == "task" {
		taskID := step.Target.EntityID
		run.TaskID = &taskID
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, err
	}
	updated, err := s.executeCommandBarOrchestrationRun(ctx, input, steps, stepIndex, run)
	if err != nil {
		run.Status = model.AgentRunStatusFailed
		run.ErrorMessage = strPtr(err.Error())
		completedAt := time.Now()
		run.CompletedAt = &completedAt
		run.LastHeartbeatAt = &completedAt
		_ = s.runRepo.Update(ctx, run)
		s.runRepo.Notify(ctx, run)
		return nil, err
	}
	s.runRepo.Notify(ctx, updated)
	return updated, nil
}

func (s *AgentService) advanceRunningCommandBarOrchestrationRun(ctx context.Context, input temporalapp.CommandBarPlanWorkflowInput, steps []model.CommandBarPlanStep, stepIndex int, run *model.AgentRun) (*model.AgentRun, error) {
	if run == nil || run.Status != model.AgentRunStatusRunning {
		return run, nil
	}
	var output commandBarOrchestrationOutput
	_ = json.Unmarshal(run.OutputSummary, &output)
	if output.ConflictRunID == "" {
		return run, nil
	}
	child, err := s.runRepo.GetByIDAny(ctx, output.ConflictRunID)
	if err != nil || child == nil {
		return run, err
	}
	if model.IsAgentRunActiveStatus(child.Status) {
		return run, nil
	}
	if child.Status != model.AgentRunStatusCompleted {
		run.Status = model.AgentRunStatusFailed
		run.ErrorMessage = strPtr("merge conflict resolution run did not complete successfully")
		now := time.Now()
		run.CompletedAt = &now
		run.LastHeartbeatAt = &now
		if err := s.runRepo.Update(ctx, run); err != nil {
			return nil, err
		}
		s.runRepo.Notify(ctx, run)
		return run, nil
	}
	return s.executeCommandBarOrchestrationRun(ctx, input, steps, stepIndex, run)
}

func (s *AgentService) executeCommandBarOrchestrationRun(ctx context.Context, input temporalapp.CommandBarPlanWorkflowInput, steps []model.CommandBarPlanStep, stepIndex int, run *model.AgentRun) (*model.AgentRun, error) {
	if s.gitService == nil {
		return nil, fmt.Errorf("git service is not configured")
	}
	step := steps[stepIndex]
	switch step.StepType {
	case model.CommandBarStepTypeEnsureEpicBranch:
		target, err := s.gitService.EnsureEpicBranch(ctx, input.WorkspaceID, input.PageContext.EntityID, input.ActorID, run.ID)
		if err != nil {
			return nil, err
		}
		preparedTaskIDs := make(map[string]bool)
		for _, candidate := range steps {
			if candidate.Target.EntityType == "task" && candidate.Target.EntityID != "" {
				if preparedTaskIDs[candidate.Target.EntityID] {
					continue
				}
				preparedTaskIDs[candidate.Target.EntityID] = true
				if _, err := s.gitService.PrepareTaskForEpicBranch(ctx, input.WorkspaceID, candidate.Target.EntityID, input.PageContext.EntityID, input.ActorID); err != nil {
					return nil, err
				}
			}
		}
		return s.completeCommandBarOrchestrationRun(ctx, run, commandBarOrchestrationOutput{
			Type:       step.StepType,
			Status:     "completed",
			Message:    "Epic branch is ready and task delivery targets were based on it.",
			EpicBranch: derefString(target.EpicBranch),
			BaseBranch: derefString(target.BaseBranch),
		})
	case model.CommandBarStepTypeMergeTaskToEpic:
		target, err := s.gitService.MergeTaskBranchIntoEpic(ctx, input.WorkspaceID, step.Target.EntityID, input.PageContext.EntityID, run.ID)
		if err != nil {
			if commandBarIsMergeConflict(err) {
				return s.startCommandBarMergeConflictResolution(ctx, input, steps, stepIndex, run, err)
			}
			return nil, err
		}
		return s.completeCommandBarOrchestrationRun(ctx, run, commandBarOrchestrationOutput{
			Type:       step.StepType,
			Status:     "completed",
			Message:    "Task branch merged into epic branch.",
			EpicBranch: derefString(target.EpicBranch),
			BaseBranch: derefString(target.BaseBranch),
		})
	case model.CommandBarStepTypeOpenEpicPullRequest:
		target, err := s.gitService.OpenEpicFinalPullRequest(ctx, input.WorkspaceID, input.PageContext.EntityID, run.ID)
		if err != nil {
			return nil, err
		}
		return s.completeCommandBarOrchestrationRun(ctx, run, commandBarOrchestrationOutput{
			Type:                step.StepType,
			Status:              "completed",
			Message:             "Final epic pull request is open.",
			EpicBranch:          derefString(target.EpicBranch),
			BaseBranch:          derefString(target.BaseBranch),
			FinalPullRequestURL: derefString(target.FinalPRURL),
		})
	default:
		return nil, fmt.Errorf("unsupported command-bar orchestration step %q", step.StepType)
	}
}

func (s *AgentService) completeCommandBarOrchestrationRun(ctx context.Context, run *model.AgentRun, output commandBarOrchestrationOutput) (*model.AgentRun, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	run.Status = model.AgentRunStatusCompleted
	run.PauseReason = model.AgentRunPauseReasonNone
	run.ExecutionStage = strPtr("completed")
	run.OutputSummary = raw
	run.CompletedAt = &now
	run.LastHeartbeatAt = &now
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	s.runRepo.Notify(ctx, run)
	return run, nil
}

func commandBarIsMergeConflict(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "merge conflict")
}

func (s *AgentService) startCommandBarMergeConflictResolution(ctx context.Context, input temporalapp.CommandBarPlanWorkflowInput, steps []model.CommandBarPlanStep, stepIndex int, run *model.AgentRun, mergeErr error) (*model.AgentRun, error) {
	var existing commandBarOrchestrationOutput
	_ = json.Unmarshal(run.OutputSummary, &existing)
	if existing.ConflictAttempt >= 1 && existing.ConflictRunID != "" {
		return nil, fmt.Errorf("task branch still conflicts with epic branch after Forge conflict resolution: %w", mergeErr)
	}
	step := steps[stepIndex]
	forgeStep, ok := commandBarForgeStepForTarget(steps, step.Target.EntityID)
	if !ok {
		return nil, fmt.Errorf("merge conflict requires Forge conflict resolution, but no Forge step was found for task %s", step.Target.DisplayTitle)
	}
	triggerContext, err := buildCommandBarTriggerContext(input.Prompt, input.PageContext, steps, stepIndex, input.PlanID)
	if err != nil {
		return nil, err
	}
	additionalContext := strings.Join([]string{
		"Resolve the merge conflict blocking this epic integration.",
		"Your task branch could not be merged into the epic integration branch.",
		"Update the task branch so it merges cleanly into the epic branch, then push your changes.",
		"Do not merge the task branch into the epic branch yourself; Helpin will retry the backend merge after this run completes.",
		"Merge error: " + mergeErr.Error(),
	}, "\n")
	reason := "command_bar_merge_conflict"
	parentRunID := run.ID
	event := &model.AgentRunEventContext{
		RunID:  &run.ID,
		Reason: &reason,
	}
	actor := stringPtrIfNotEmpty(input.ActorID)
	child, err := s.startTargetRunWithOptions(ctx, input.WorkspaceID, step.Target.EntityType, step.Target.EntityID, model.StartAgentRunRequest{
		AgentID:           forgeStep.AgentID,
		AdditionalContext: &additionalContext,
		AllowedTools:      forgeStep.AllowedTools,
	}, actor, triggerContext, event, &parentRunID, startTargetRunOptions{allowActiveParentRun: true})
	if err != nil {
		return nil, err
	}
	output := commandBarOrchestrationOutput{
		Type:            step.StepType,
		Status:          "resolving_conflict",
		Message:         "Forge is resolving a merge conflict before Helpin retries the task-to-epic merge.",
		ConflictRunID:   child.ID,
		ConflictAttempt: existing.ConflictAttempt + 1,
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	run.Status = model.AgentRunStatusRunning
	run.ExecutionStage = strPtr("resolving_conflict")
	run.OutputSummary = raw
	run.LastHeartbeatAt = &now
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	s.runRepo.Notify(ctx, run)
	return run, nil
}

func commandBarForgeStepForTarget(steps []model.CommandBarPlanStep, taskID string) (model.CommandBarPlanStep, bool) {
	for _, step := range steps {
		if step.Target.EntityID == taskID && normalizePresetKey(step.AgentKey) == model.AgentPresetCodeBuilder {
			return step, true
		}
	}
	for _, step := range steps {
		if step.Target.EntityID == taskID && strings.EqualFold(strings.TrimSpace(step.AgentName), "Forge") {
			return step, true
		}
	}
	return model.CommandBarPlanStep{}, false
}

func commandBarStepIndexForRun(runIDsByStep map[int]string, runID string) int {
	for index, id := range runIDsByStep {
		if id == runID {
			return index
		}
	}
	return -1
}

func (s *AgentService) advanceFanOutCommandBarPlan(ctx context.Context, run *model.AgentRun, payload commandBarTriggerContextPayload) (*model.AgentRun, error) {
	if s.commandBarPlanRepo == nil || payload.PlanID == "" {
		return nil, nil
	}
	if run.Status == model.AgentRunStatusFailed {
		_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(run, payload, "failed"))
		return nil, nil
	}
	if run.Status == model.AgentRunStatusCancelled {
		_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(run, payload, "cancelled"))
		return nil, nil
	}
	if run.Status != model.AgentRunStatusCompleted {
		return nil, nil
	}
	plan, err := s.commandBarPlanRepo.GetByID(ctx, run.WorkspaceID, payload.PlanID)
	if err != nil || plan == nil {
		return nil, err
	}
	if plan.Status == model.CommandBarPlanStatusCancelled || plan.Status == model.CommandBarPlanStatusFailed {
		return nil, nil
	}
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	if len(runIDsByStep) < len(payload.Steps) {
		return nil, nil
	}
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.runRepo.ListByIDs(ctx, run.WorkspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	if len(runs) < len(payload.Steps) {
		return nil, nil
	}
	allCompleted := true
	for _, item := range runs {
		switch item.Status {
		case model.AgentRunStatusCompleted:
		case model.AgentRunStatusFailed:
			_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(&item, payload, "failed"))
			return nil, nil
		case model.AgentRunStatusCancelled:
			_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(&item, payload, "cancelled"))
			return nil, nil
		default:
			allCompleted = false
		}
	}
	if allCompleted {
		_ = s.commandBarPlanRepo.MarkCompleted(ctx, run.WorkspaceID, payload.PlanID)
	}
	return nil, nil
}

func commandBarTerminalRunMessage(run *model.AgentRun, payload commandBarTriggerContextPayload, fallback string) string {
	if run != nil && strings.TrimSpace(derefString(run.ErrorMessage)) != "" {
		return strings.TrimSpace(derefString(run.ErrorMessage))
	}
	stepNumber := payload.StepIndex + 1
	if stepNumber <= 0 {
		stepNumber = 1
	}
	return fmt.Sprintf("Command-bar step %d %s.", stepNumber, fallback)
}

func commandBarPlanOwnedByActor(plan *model.CommandBarPlanRecord, actorID string) bool {
	if plan == nil {
		return false
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" || plan.ActorID == nil {
		return true
	}
	return strings.TrimSpace(*plan.ActorID) == actorID
}

func commandBarRunPayload(run *model.AgentRun) (commandBarTriggerContextPayload, bool) {
	var empty commandBarTriggerContextPayload
	if run == nil || len(run.Input) == 0 {
		return empty, false
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil || input.Trigger == nil {
		return empty, false
	}
	if input.Trigger.Source != model.AgentRunTriggerSourceCommandBar || input.Trigger.TriggerType != model.AgentRunTriggerTypeCommandBar {
		return empty, false
	}
	if len(input.Trigger.Context) == 0 {
		return empty, false
	}
	var payload commandBarTriggerContextPayload
	if err := json.Unmarshal(input.Trigger.Context, &payload); err != nil {
		return empty, false
	}
	if payload.RunCount == 0 {
		payload.RunCount = len(payload.Steps)
	}
	return payload, true
}

func newCommandBarPlanRecord(workspaceID, actorID, planID, prompt string, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep) (*model.CommandBarPlanRecord, error) {
	pageContextRaw, err := json.Marshal(pageContext)
	if err != nil {
		return nil, err
	}
	stepsRaw, err := json.Marshal(steps)
	if err != nil {
		return nil, err
	}
	runIDsRaw, _ := json.Marshal(map[int]string{})
	var actor *string
	if strings.TrimSpace(actorID) != "" {
		actor = &actorID
	}
	return &model.CommandBarPlanRecord{
		ID:               planID,
		WorkspaceID:      workspaceID,
		ActorID:          actor,
		Status:           model.CommandBarPlanStatusRunning,
		Prompt:           strings.TrimSpace(prompt),
		PageContext:      pageContextRaw,
		Steps:            stepsRaw,
		RunIDsByStep:     runIDsRaw,
		CurrentStepIndex: 0,
		RunCount:         len(steps),
	}, nil
}

func decodeCommandBarPlanRunIDs(raw json.RawMessage) map[int]string {
	result := map[int]string{}
	if len(raw) == 0 {
		return result
	}
	var keyed map[string]string
	if err := json.Unmarshal(raw, &keyed); err == nil {
		for key, value := range keyed {
			if idx, scanErr := strconv.Atoi(key); scanErr == nil && strings.TrimSpace(value) != "" {
				result[idx] = value
			}
		}
		return result
	}
	_ = json.Unmarshal(raw, &result)
	return result
}

func commandBarPlanSummary(record model.CommandBarPlanRecord, runs []model.AgentRun) model.CommandBarPlanSummary {
	var pageContext model.CommandBarPageContext
	_ = json.Unmarshal(record.PageContext, &pageContext)
	var steps []model.CommandBarPlanStep
	_ = json.Unmarshal(record.Steps, &steps)
	return model.CommandBarPlanSummary{
		ID:               record.ID,
		Status:           record.Status,
		PlanKind:         commandBarPlanKindForSteps(steps),
		Prompt:           record.Prompt,
		PageContext:      pageContext,
		Steps:            steps,
		RunIDsByStep:     decodeCommandBarPlanRunIDs(record.RunIDsByStep),
		CurrentStepIndex: record.CurrentStepIndex,
		RunCount:         record.RunCount,
		ErrorMessage:     record.ErrorMessage,
		CancelledAt:      record.CancelledAt,
		CompletedAt:      record.CompletedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
		Runs:             runs,
	}
}

func commandBarUnmetIntentSummary(intent model.CommandBarUnmetIntent, includeSensitive bool) model.CommandBarUnmetIntentSummary {
	var pageContext model.CommandBarPageContext
	_ = json.Unmarshal(intent.PageContext, &pageContext)
	var candidates []model.CommandBarAgent
	_ = json.Unmarshal(intent.CandidateAgents, &candidates)
	prompt := ""
	if includeSensitive {
		prompt = intent.Prompt
	}
	return model.CommandBarUnmetIntentSummary{
		ID:              intent.ID,
		WorkspaceID:     intent.WorkspaceID,
		ActorID:         intent.ActorID,
		Prompt:          prompt,
		PromptPreview:   commandBarPromptPreview(intent.Prompt),
		PromptRedacted:  !includeSensitive,
		PageContext:     pageContext,
		CandidateAgents: candidates,
		Reason:          intent.Reason,
		Status:          intent.Status,
		ReviewNotes:     intent.ReviewNotes,
		ReviewedAt:      intent.ReviewedAt,
		CreatedAt:       intent.CreatedAt,
	}
}

func commandBarPromptPreview(prompt string) string {
	prompt = strings.Join(strings.Fields(strings.TrimSpace(prompt)), " ")
	const maxPreviewRunes = 160
	runes := []rune(prompt)
	if len(runes) <= maxPreviewRunes {
		return prompt
	}
	return string(runes[:maxPreviewRunes]) + "..."
}

func validatePromotedAgentTargets(targets []string, sourceAgent *model.Agent) error {
	targets = normalizeStringSlice(targets)
	if len(targets) == 0 {
		return fmt.Errorf("at least one allowed target is required")
	}
	sourceAllowedTargets := parseJSONStringSlice(sourceAgent.AllowedTargets)
	sourceAllowedSet := make(map[string]bool, len(sourceAllowedTargets))
	for _, target := range sourceAllowedTargets {
		sourceAllowedSet[normalizeCommandBarTargetType(target)] = true
	}
	for _, target := range targets {
		normalized := normalizeCommandBarTargetType(target)
		if err := validateCommandBarSupportedTarget(normalized); err != nil {
			return err
		}
		if len(sourceAllowedSet) > 0 && !sourceAllowedSet[normalized] {
			return fmt.Errorf("target %q is outside source agent %s allowlist", normalized, strings.TrimSpace(sourceAgent.Name))
		}
	}
	return nil
}

type commandBarPlannerAgentCard struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	PresetKey      string   `json:"preset_key,omitempty"`
	Role           string   `json:"role,omitempty"`
	AllowedTargets []string `json:"allowed_targets"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
	OneShot        bool     `json:"one_shot"`
}

type commandBarPlannerToolCard struct {
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Mutation    bool           `json:"mutation"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type commandBarPlannerStep struct {
	AgentID              string                       `json:"agent_id"`
	Target               *model.CommandBarPageContext `json:"target,omitempty"`
	Instructions         string                       `json:"instructions"`
	AllowedTools         []string                     `json:"allowed_tools,omitempty"`
	ToolIntent           string                       `json:"tool_intent,omitempty"`
	DependsOnStepIndexes []int                        `json:"depends_on_step_indexes,omitempty"`
}

type commandBarPlannerOutput struct {
	Status             string                  `json:"status"`
	RouteKind          string                  `json:"route_kind"`
	AgentID            string                  `json:"agent_id"`
	Instructions       string                  `json:"instructions"`
	Steps              []commandBarPlannerStep `json:"steps"`
	OneShotTools       []string                `json:"one_shot_tools"`
	ToolIntent         string                  `json:"tool_intent"`
	Rationale          string                  `json:"rationale"`
	Reason             string                  `json:"reason"`
	ClarifyingQuestion string                  `json:"clarifying_question"`
	Confidence         float64                 `json:"confidence"`
}

func (s *CommandBarService) parseIntentWithLLM(ctx context.Context, text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if s == nil || s.llmProvider == nil {
		return nil
	}
	agentCards := commandBarPlannerAgentCards(candidates)
	toolCards := s.commandBarPlannerToolCards(candidates)
	candidateJSON, _ := json.Marshal(agentCards)
	toolJSON, _ := json.Marshal(toolCards)
	contextJSON, _ := json.Marshal(pageContext)
	timeout := s.commandRouterTimeoutForRequest()
	maxTokens := s.commandRouterLLMMaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultCommandRouterMaxTokens
	}
	var validationFeedback string
	for attempt := 0; attempt < 2; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		resp, err := s.llmProvider.ChatCompletion(callCtx, llm.ChatRequest{
			Provider: s.commandRouterLLMProvider,
			Model:    s.commandRouterLLMModel,
			SystemPrompt: fmt.Sprintf(`You are Helpin's command-bar semantic router. Return strict JSON only.

Decide whether the user request should run:
- "known_agent": exactly one saved non-one-shot agent.
- "multi_step": 2-%d saved non-one-shot agents in sequence.
- "one_shot_command": the one-shot Command Agent with a narrow tool subset.
- "dag": a dependency graph of saved-agent and/or one-shot Command Agent steps.
- "no_matching_agent": no safe route or a clarifying question is required.

Routing policy:
- Use saved agents only when the request clearly matches the agent name, role, description, preset, and current target.
- Use multi_step only when the user explicitly asks for staged work or names multiple agents.
- Use one_shot_command for ad hoc data questions, workspace lookup/count/summarization, one-off document/task/CRM changes, or requests that do not fit a reusable saved agent.
- Use dag only when the user asks for orchestration with dependency order, fan-out/fan-in, parallel branches, or multiple phases that should be durably scheduled.
- For one_shot_command, choose the minimum necessary tools from the available one-shot tool catalog. Do not choose mutation tools for read-only questions.
- For one_shot_command requests that need current external evidence, industry trends, online research, web search, or fetched URLs, include web search/fetch tools from the available one-shot catalog.
- For one_shot_command, set tool_intent to "read_only", "propose_change", or "mutate".
- Use tool_intent "propose_change" when the requested outcome is a proposed content change, even conditionally, such as "check if this document needs update" or "update only if research finds new information".
- For Docs proposal/change requests, include publish_document_change_proposal. Read/search tools can gather evidence but cannot submit a Docs proposal.
- For one_shot_command, include steps[0].target when the request names or implies a concrete task, document, epic, CRM deal, or contact.
- Target-bound one_shot_command work must not use workspace as a placeholder; return no_matching_agent with a clarifying_question when the target is not clear.
- For dag Command Agent steps, choose the minimum necessary tools on that step. Saved-agent DAG steps may omit allowed_tools.
- For dag Command Agent steps, set the step tool_intent using the same values.
- For dag steps, use concrete targets only. Do not invent target IDs or use placeholders for targets discovered by prior steps.
- Never route destructive workspace deletes.
- Each step instruction must be scoped to that agent only; do not ask one agent to invoke another agent.`, maxCommandBarPlanSteps),
			Messages: []llm.Message{{
				Role: "user",
				Content: fmt.Sprintf(`Request: %s
Page context: %s
Available agents: %s
Available one-shot tools: %s
Previous planner validation error: %s

Return one JSON object:
{
  "status": "plan" | "no_matching_agent",
  "route_kind": "known_agent" | "multi_step" | "one_shot_command" | "dag" | "no_matching_agent",
  "agent_id": "single saved agent id, or Command Agent id for one_shot_command",
  "instructions": "single-step instruction",
  "steps": [{"agent_id":"agent id","target":{"entity_type":"workspace","entity_id":"...","display_title":"..."},"instructions":"step-scoped instruction","allowed_tools":["tool_name"],"tool_intent":"read_only","depends_on_step_indexes":[0]}],
  "one_shot_tools": ["tool_name"],
  "tool_intent": "read_only | propose_change | mutate",
  "rationale": "short reason",
  "reason": "short no-match reason",
  "clarifying_question": "only when status is no_matching_agent because the request is ambiguous",
  "confidence": 0.0
}`, text, string(contextJSON), string(candidateJSON), string(toolJSON), validationFeedback),
			}},
			Temperature:     0,
			MaxTokens:       maxTokens,
			JSONMode:        true,
			JSONSchema:      commandBarPlannerJSONSchema(),
			ProviderOptions: s.commandRouterProviderOptionsForRequest(),
		})
		cancel()
		if err != nil || resp == nil {
			if err != nil {
				slog.WarnContext(ctx, "command bar llm parse failed", "error", err)
			}
			return nil
		}

		var parsed commandBarPlannerOutput
		if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
			slog.WarnContext(ctx, "command bar llm parse returned invalid json", "error", err)
			return nil
		}
		if parsed.Status == model.CommandBarParseStatusNoMatchingAgent || parsed.RouteKind == model.CommandBarParseStatusNoMatchingAgent {
			reason := firstNonEmptyString(strings.TrimSpace(parsed.Reason), strings.TrimSpace(parsed.ClarifyingQuestion), "No available agent matched this request.")
			return &model.CommandBarParseResponse{
				Status:      model.CommandBarParseStatusNoMatchingAgent,
				Reason:      reason,
				Suggestions: defaultCommandBarSuggestions(pageContext.EntityType),
				Candidates:  candidates,
			}
		}
		if parsed.Status != model.CommandBarParseStatusPlan {
			return nil
		}
		switch strings.TrimSpace(parsed.RouteKind) {
		case model.CommandBarPlanKindDAG:
			return s.commandBarDAGPlanFromLLM(text, pageContext, candidates, parsed)
		case model.CommandBarPlanKindOneShotCommand, "one_shot":
			planned, validationErr := s.commandBarOneShotPlanFromLLMValidated(text, pageContext, candidates, parsed)
			if validationErr != "" {
				validationFeedback = validationErr
				if attempt == 0 {
					continue
				}
				return &model.CommandBarParseResponse{
					Status:      model.CommandBarParseStatusNoMatchingAgent,
					Reason:      commandBarPlannerValidationReasonPrefix + validationErr,
					Suggestions: defaultCommandBarSuggestions(pageContext.EntityType),
					Candidates:  candidates,
				}
			}
			return planned
		case "multi_step":
			return commandBarKnownAgentPlanFromLLM(pageContext, candidates, parsed)
		case "", model.CommandBarPlanKindKnownAgent:
			if len(parsed.Steps) > 1 {
				return commandBarKnownAgentPlanFromLLM(pageContext, candidates, parsed)
			}
			if strings.TrimSpace(parsed.AgentID) == "" && len(parsed.Steps) == 1 {
				parsed.AgentID = parsed.Steps[0].AgentID
				parsed.Instructions = firstNonEmptyString(strings.TrimSpace(parsed.Instructions), parsed.Steps[0].Instructions)
			}
			agent, ok := findCommandBarCandidateByID(candidates, parsed.AgentID)
			if !ok || isOneShotCommandAgent(agent) {
				return nil
			}
			return commandBarPlanResponse(agent, pageContext, text, parsed.Instructions, parsed.Rationale, candidates)
		default:
			return nil
		}
	}
	return nil
}

func commandBarPlannerAgentCards(candidates []model.CommandBarAgent) []commandBarPlannerAgentCard {
	cards := make([]commandBarPlannerAgentCard, 0, len(candidates))
	for _, candidate := range candidates {
		cards = append(cards, commandBarPlannerAgentCard{
			ID:             candidate.ID,
			Name:           candidate.Name,
			Description:    candidate.Description,
			PresetKey:      candidate.PresetKey,
			Role:           candidate.Role,
			AllowedTargets: candidate.AllowedTargets,
			AllowedTools:   candidate.AllowedTools,
			OneShot:        isOneShotCommandAgent(candidate),
		})
	}
	return cards
}

func commandBarPlannerJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"status": map[string]any{
				"type": "string",
				"enum": []string{model.CommandBarParseStatusPlan, model.CommandBarParseStatusNoMatchingAgent},
			},
			"route_kind": map[string]any{
				"type": "string",
				"enum": []string{model.CommandBarPlanKindKnownAgent, "multi_step", model.CommandBarPlanKindOneShotCommand, model.CommandBarPlanKindDAG, model.CommandBarParseStatusNoMatchingAgent},
			},
			"agent_id": map[string]any{"type": "string"},
			"instructions": map[string]any{
				"type": "string",
			},
			"steps": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"agent_id": map[string]any{"type": "string"},
						"target": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"entity_type":   map[string]any{"type": "string"},
								"entity_id":     map[string]any{"type": "string"},
								"display_title": map[string]any{"type": "string"},
								"related_ids":   map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}},
								"metadata":      map[string]any{"type": "object"},
							},
							"required":             []string{"entity_type", "entity_id", "display_title"},
							"additionalProperties": false,
						},
						"instructions":            map[string]any{"type": "string"},
						"allowed_tools":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"tool_intent":             map[string]any{"type": "string", "enum": []string{"", "read_only", "propose_change", "mutate"}},
						"depends_on_step_indexes": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
					},
					"required":             []string{"agent_id", "instructions"},
					"additionalProperties": false,
				},
			},
			"one_shot_tools": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"tool_intent":    map[string]any{"type": "string", "enum": []string{"", "read_only", "propose_change", "mutate"}},
			"rationale":      map[string]any{"type": "string"},
			"reason":         map[string]any{"type": "string"},
			"clarifying_question": map[string]any{
				"type": "string",
			},
			"confidence": map[string]any{"type": "number"},
		},
		"required":             []string{"status", "route_kind", "rationale", "confidence"},
		"additionalProperties": false,
	}
}

func (s *CommandBarService) commandBarPlannerToolCards(candidates []model.CommandBarAgent) []commandBarPlannerToolCard {
	commandAgent, ok := findCommandBarCandidateByPreset(candidates, model.AgentPresetCommandAgent)
	if !ok || s == nil || s.agentService == nil {
		return nil
	}
	allowedSet := make(map[string]bool, len(commandAgent.AllowedTools))
	for _, tool := range commandAgent.AllowedTools {
		allowedSet[strings.TrimSpace(tool)] = true
	}
	catalog := s.agentService.ListToolCatalog()
	cards := make([]commandBarPlannerToolCard, 0, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		if !allowedSet[tool.Name] {
			continue
		}
		description := tool.Description
		if tool.Name == "publish_document_change_proposal" {
			description = strings.TrimSpace(description + " Required when a Docs request may propose a content change, including conditional update requests such as checking whether a known document needs updates after research.")
		}
		cards = append(cards, commandBarPlannerToolCard{
			Name:        tool.Name,
			Category:    tool.Category,
			Description: description,
			Mutation:    commandBarToolIsMutation(tool.Name),
		})
	}
	return cards
}

func commandBarKnownAgentPlanFromLLM(pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent, parsed commandBarPlannerOutput) *model.CommandBarParseResponse {
	if len(parsed.Steps) == 0 {
		return nil
	}
	steps := make([]model.CommandBarPlanStep, 0, min(len(parsed.Steps), maxCommandBarPlanSteps))
	for i, parsedStep := range parsed.Steps {
		if i >= maxCommandBarPlanSteps {
			break
		}
		agent, ok := findCommandBarCandidateByID(candidates, parsedStep.AgentID)
		if !ok || isOneShotCommandAgent(agent) {
			return nil
		}
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:      agent.ID,
			AgentKey:     agent.PresetKey,
			AgentName:    agent.Name,
			Target:       pageContext,
			Instructions: firstNonEmptyString(strings.TrimSpace(parsedStep.Instructions), fmt.Sprintf("Execute your normal %s role for the current target.", agent.Name)),
		})
	}
	if len(steps) == 0 {
		return nil
	}
	return commandBarMultiStepPlanResponse(steps, firstNonEmptyString(strings.TrimSpace(parsed.Rationale), "Matched request to available agents."), candidates)
}

func (s *CommandBarService) commandBarDAGPlanFromLLM(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent, parsed commandBarPlannerOutput) *model.CommandBarParseResponse {
	if len(parsed.Steps) == 0 || len(parsed.Steps) > maxCommandBarPlanSteps {
		return nil
	}
	steps := make([]model.CommandBarPlanStep, 0, len(parsed.Steps))
	for i, parsedStep := range parsed.Steps {
		agent, ok := findCommandBarCandidateByID(candidates, parsedStep.AgentID)
		if !ok {
			return nil
		}
		target := pageContext
		if parsedStep.Target != nil {
			target = *parsedStep.Target
			target.EntityType = normalizeCommandBarTargetType(target.EntityType)
			target.EntityID = strings.TrimSpace(target.EntityID)
			target.DisplayTitle = strings.TrimSpace(target.DisplayTitle)
			if target.DisplayTitle == "" {
				target.DisplayTitle = target.EntityType
			}
		}
		if strings.TrimSpace(target.EntityType) == "" || strings.TrimSpace(target.EntityID) == "" {
			return nil
		}
		allowedTools := normalizeStringSlice(parsedStep.AllowedTools)
		instructions := strings.TrimSpace(parsedStep.Instructions)
		if isOneShotCommandAgent(agent) {
			toolIntent := normalizeCommandBarToolIntent(parsedStep.ToolIntent)
			if toolIntent == "" {
				toolIntent = commandBarToolIntentFromTools(allowedTools)
			}
			if err := commandBarValidatePlannerToolIntent(toolIntent, allowedTools); err != nil {
				return nil
			}
			allowedTools = commandBarFilterOneShotTools(allowedTools, agent.AllowedTools, toolIntent)
			if len(allowedTools) == 0 {
				return nil
			}
			if err := commandBarValidatePlannerToolIntent(toolIntent, allowedTools); err != nil {
				return nil
			}
			base := commandBarOneShotInstructions(text, target, allowedTools)
			if instructions != "" {
				instructions = base + "\n\nPlanner instruction:\n" + instructions
			} else {
				instructions = base
			}
		}
		if instructions == "" {
			instructions = fmt.Sprintf("Execute your normal %s role for the confirmed DAG step.", agent.Name)
		}
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:              agent.ID,
			AgentKey:             agent.PresetKey,
			AgentName:            firstNonEmptyString(strings.TrimSpace(agent.Name), "Agent"),
			PlanKind:             model.CommandBarPlanKindDAG,
			Target:               target,
			Instructions:         instructions,
			AllowedTools:         allowedTools,
			DependsOnStepIndexes: append([]int(nil), parsedStep.DependsOnStepIndexes...),
		})
		if i >= maxCommandBarPlanSteps {
			break
		}
	}
	if len(steps) == 0 {
		return nil
	}
	if err := validateCommandBarStepDependencies(steps, maxCommandBarDAGInitialFanOut); err != nil {
		slog.Warn("command bar llm DAG rejected", "error", err)
		return nil
	}
	resp := commandBarMultiStepPlanResponse(steps, firstNonEmptyString(strings.TrimSpace(parsed.Rationale), "Prepared a dependency-aware command DAG."), candidates)
	resp.Plan.PlanKind = model.CommandBarPlanKindDAG
	resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
		Type:     "temporal_orchestration",
		Severity: "info",
		Message:  "Temporal will run unblocked DAG steps in parallel and start dependent steps as prerequisites complete.",
	})
	if commandBarStepsUseMutationTools(steps) {
		resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
			Type:     "mutation_tools",
			Severity: "warning",
			Message:  "This plan includes one-shot mutation tools. Review the steps before dispatch.",
		})
	}
	return resp
}

func (s *CommandBarService) commandBarOneShotPlanFromLLM(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent, parsed commandBarPlannerOutput) *model.CommandBarParseResponse {
	resp, _ := s.commandBarOneShotPlanFromLLMValidated(text, pageContext, candidates, parsed)
	return resp
}

func (s *CommandBarService) commandBarOneShotPlanFromLLMValidated(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent, parsed commandBarPlannerOutput) (*model.CommandBarParseResponse, string) {
	agent, ok := findCommandBarCandidateByPreset(candidates, model.AgentPresetCommandAgent)
	if !ok {
		return nil, "Command Agent is not available."
	}
	target := pageContext
	if len(parsed.Steps) == 1 && parsed.Steps[0].Target != nil {
		target = commandBarNormalizePlannerTarget(*parsed.Steps[0].Target)
	}
	toolIntent := normalizeCommandBarToolIntent(parsed.ToolIntent)
	if toolIntent == "" && len(parsed.Steps) == 1 {
		toolIntent = normalizeCommandBarToolIntent(parsed.Steps[0].ToolIntent)
	}
	allowedTools := normalizeStringSlice(parsed.OneShotTools)
	if len(allowedTools) == 0 && len(parsed.Steps) == 1 {
		allowedTools = normalizeStringSlice(parsed.Steps[0].AllowedTools)
	}
	if len(allowedTools) == 0 {
		return nil, "one_shot_command requires explicit one_shot_tools or steps[0].allowed_tools from the planner."
	}
	if toolIntent == "" {
		toolIntent = commandBarToolIntentFromTools(allowedTools)
	}
	if err := commandBarValidatePlannerToolIntent(toolIntent, allowedTools); err != nil {
		return nil, err.Error()
	}
	allowedTools = commandBarFilterOneShotTools(allowedTools, agent.AllowedTools, toolIntent)
	allowedTools, toolIntent = commandBarCompleteRepositoryOneShotTools(text, target, allowedTools, agent.AllowedTools, toolIntent)
	if len(allowedTools) == 0 {
		return nil, "selected one-shot tools are not enabled for Command Agent."
	}
	if err := commandBarValidatePlannerToolIntent(toolIntent, allowedTools); err != nil {
		return nil, err.Error()
	}
	instructions := commandBarOneShotInstructions(text, target, allowedTools)
	if extra := strings.TrimSpace(firstNonEmptyString(parsed.Instructions, firstStepInstructions(parsed.Steps))); extra != "" {
		instructions += "\n\nPlanner instruction:\n" + extra
	}
	step := model.CommandBarPlanStep{
		AgentID:      agent.ID,
		AgentKey:     agent.PresetKey,
		AgentName:    firstNonEmptyString(strings.TrimSpace(agent.Name), "Command Agent"),
		PlanKind:     model.CommandBarPlanKindOneShotCommand,
		Target:       target,
		Instructions: instructions,
		AllowedTools: allowedTools,
	}
	resp := commandBarMultiStepPlanResponse(
		[]model.CommandBarPlanStep{step},
		firstNonEmptyString(strings.TrimSpace(parsed.Rationale), "Prepared a one-shot command agent from semantic routing."),
		candidates,
	)
	resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
		Type:     "one_shot_command",
		Severity: "info",
		Message:  "This is a one-shot run. It is not saved as a reusable agent unless you promote it after completion.",
	})
	return resp, ""
}

func firstStepInstructions(steps []commandBarPlannerStep) string {
	if len(steps) == 0 {
		return ""
	}
	return strings.TrimSpace(steps[0].Instructions)
}

func parseIntentDeterministically(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	lower := strings.ToLower(strings.TrimSpace(text))
	if parsed := parseExplicitNamedAgents(text, pageContext, candidates); parsed != nil {
		return parsed
	}
	if parsed := parseTaskPhaseSequenceIntent(text, pageContext, candidates); parsed != nil {
		return parsed
	}

	preferredPresets := []string{}
	if containsAny(lower, "review", "qa", "test", "check quality", "audit") {
		preferredPresets = append(preferredPresets, model.AgentPresetReviewAgent)
	}
	if containsAny(lower, "implement", "build", "code", "fix", "bug", "pull request", "pr") {
		preferredPresets = append(preferredPresets, model.AgentPresetCodeBuilder)
	}
	if containsAny(lower, "break down", "breakdown", "plan", "spec", "acceptance criteria", "tasks", "stories") {
		if pageContext.EntityType == "epic" || pageContext.EntityType == "workspace" {
			preferredPresets = append(preferredPresets, model.AgentPresetEpicPlanner)
		}
		preferredPresets = append(preferredPresets, model.AgentPresetTaskPlanner)
	}
	if containsAny(lower, "deal", "crm", "contact", "buyer", "pipeline") {
		preferredPresets = append(preferredPresets, model.AgentPresetCRMOperator)
	}
	if containsAny(lower, "support", "ticket", "conversation", "reply", "customer") {
		preferredPresets = append(preferredPresets, model.AgentPresetSupportAgent)
	}

	for _, preset := range preferredPresets {
		if agent, ok := findCommandBarCandidateByPreset(candidates, preset); ok {
			return commandBarPlanResponse(agent, pageContext, text, text, "Matched request keywords to an available agent.", candidates)
		}
	}
	return nil
}

func parseTaskPhaseSequenceIntent(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if normalizeCommandBarTargetType(pageContext.EntityType) != "task" || strings.TrimSpace(pageContext.EntityID) == "" {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || !containsAny(lower, "run", "start", "kick off", "then", "and") {
		return nil
	}
	type phaseMatch struct {
		index  int
		preset string
	}
	phasePatterns := []struct {
		preset  string
		phrases []string
	}{
		{preset: model.AgentPresetTaskPlanner, phrases: []string{"task planning", "planning", "plan", "scribe", "task planner"}},
		{preset: model.AgentPresetCodeBuilder, phrases: []string{"coding", "code", "implementation", "implement", "forge", "code builder"}},
		{preset: model.AgentPresetReviewAgent, phrases: []string{"review", "qa", "lens", "review agent"}},
	}
	matches := make([]phaseMatch, 0, len(phasePatterns))
	for _, pattern := range phasePatterns {
		best := -1
		for _, phrase := range pattern.phrases {
			idx := indexCommandBarPhrase(lower, phrase)
			if idx >= 0 && (best < 0 || idx < best) {
				best = idx
			}
		}
		if best >= 0 {
			matches = append(matches, phaseMatch{index: best, preset: pattern.preset})
		}
	}
	if len(matches) < 2 {
		return nil
	}
	slices.SortFunc(matches, func(a, b phaseMatch) int {
		if a.index < b.index {
			return -1
		}
		if a.index > b.index {
			return 1
		}
		return strings.Compare(a.preset, b.preset)
	})
	seen := map[string]bool{}
	steps := make([]model.CommandBarPlanStep, 0, len(matches))
	for _, match := range matches {
		if seen[match.preset] {
			continue
		}
		seen[match.preset] = true
		agent, ok := findCommandBarCandidateByPreset(candidates, match.preset)
		if !ok || isOneShotCommandAgent(agent) {
			continue
		}
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:      agent.ID,
			AgentKey:     agent.PresetKey,
			AgentName:    agent.Name,
			Target:       pageContext,
			Instructions: commandBarTaskPhaseInstruction(agent, len(steps), text),
		})
	}
	if len(steps) < 2 {
		return nil
	}
	return commandBarMultiStepPlanResponse(steps, "Matched requested task phases to available agents in request order.", candidates)
}

func commandBarTaskPhaseInstruction(agent model.CommandBarAgent, stepIndex int, text string) string {
	name := firstNonEmptyString(strings.TrimSpace(agent.Name), strings.TrimSpace(agent.PresetKey), "this agent")
	switch normalizePresetKey(agent.PresetKey) {
	case model.AgentPresetTaskPlanner:
		return "Plan or refine the target task before implementation. Produce the task planning output expected by Scribe, scoped only to this task."
	case model.AgentPresetCodeBuilder:
		instruction := "Implement the target task using the enabled coding workflow."
		if stepIndex > 0 {
			instruction += " Use prior command-bar steps as context when available."
		}
		return instruction
	case model.AgentPresetReviewAgent:
		instruction := "Review the completed work for the target task."
		if stepIndex > 0 {
			instruction += " Use the previous linked run as prior-step context when available."
		}
		return instruction
	default:
		return fmt.Sprintf("Execute the requested %s phase for the target task. Original request: %s", name, strings.TrimSpace(text))
	}
}

func (s *CommandBarService) noMatchResponse(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent, reason string) *model.CommandBarParseResponse {
	reason = firstNonEmptyString(strings.TrimSpace(reason), "No available agent matched this request.")
	_ = s.logUnmetIntent(ctx, workspaceID, actorID, text, pageContext, candidates, reason)
	return &model.CommandBarParseResponse{
		Status:      model.CommandBarParseStatusNoMatchingAgent,
		Reason:      reason,
		Suggestions: defaultCommandBarSuggestions(pageContext.EntityType),
		Candidates:  candidates,
	}
}

func (s *CommandBarService) logUnmetIntent(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent, reason string) error {
	if s == nil || s.unmetRepo == nil {
		return nil
	}
	pageContextJSON, _ := json.Marshal(pageContext)
	candidatesJSON, _ := json.Marshal(candidates)
	var actor *string
	if strings.TrimSpace(actorID) != "" {
		actor = &actorID
	}
	return s.unmetRepo.Create(ctx, &model.CommandBarUnmetIntent{
		WorkspaceID:     workspaceID,
		ActorID:         actor,
		Prompt:          strings.TrimSpace(text),
		PageContext:     pageContextJSON,
		CandidateAgents: candidatesJSON,
		Reason:          reason,
	})
}

func commandBarCandidatesForTarget(agents []model.Agent, targetType string) []model.CommandBarAgent {
	targetType = normalizeCommandBarTargetType(targetType)
	candidates := make([]model.CommandBarAgent, 0, len(agents))
	for _, agent := range agents {
		allowedTargets := normalizeCommandBarTargetTypes(parseJSONStringSlice(agent.AllowedTargets))
		if len(allowedTargets) > 0 && !slices.Contains(allowedTargets, targetType) {
			continue
		}
		candidates = append(candidates, commandBarAgentCandidate(agent, allowedTargets))
	}
	return candidates
}

func commandBarAllAgentCandidates(agents []model.Agent) []model.CommandBarAgent {
	candidates := make([]model.CommandBarAgent, 0, len(agents))
	for _, agent := range agents {
		candidates = append(candidates, commandBarAgentCandidate(agent, parseJSONStringSlice(agent.AllowedTargets)))
	}
	return candidates
}

func commandBarAgentCandidate(agent model.Agent, allowedTargets []string) model.CommandBarAgent {
	return model.CommandBarAgent{
		ID:             agent.ID,
		Name:           agent.Name,
		Description:    commandBarAgentDescription(agent),
		PresetKey:      normalizePresetKey(agent.PresetKey),
		Role:           agent.Role,
		AllowedTargets: normalizeCommandBarTargetTypes(allowedTargets),
		AllowedTools:   parseJSONStringSlice(agent.AllowedTools),
	}
}

func commandBarAgentDescription(agent model.Agent) string {
	if preset, ok := agentPresetDefinition(normalizePresetKey(agent.PresetKey)); ok {
		return strings.TrimSpace(preset.Description)
	}
	if role := strings.TrimSpace(agent.Role); role != "" {
		return role
	}
	if template := strings.TrimSpace(agent.SourceTemplateKey); template != "" {
		return "Custom agent created from template " + template + "."
	}
	return "Custom workspace agent."
}

func commandBarNarrowCandidates(candidates []model.CommandBarAgent) []model.CommandBarAgent {
	narrow := make([]model.CommandBarAgent, 0, len(candidates))
	for _, candidate := range candidates {
		if isOneShotCommandAgent(candidate) {
			continue
		}
		narrow = append(narrow, candidate)
	}
	return narrow
}

func commandBarPlanResponse(agent model.CommandBarAgent, target model.CommandBarPageContext, text, instructions, rationale string, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	instructions = firstNonEmptyString(strings.TrimSpace(instructions), strings.TrimSpace(text))
	step := model.CommandBarPlanStep{
		AgentID:      agent.ID,
		AgentKey:     agent.PresetKey,
		AgentName:    agent.Name,
		Target:       target,
		Instructions: instructions,
	}
	return commandBarMultiStepPlanResponse([]model.CommandBarPlanStep{step}, firstNonEmptyString(strings.TrimSpace(rationale), "Matched request to an available agent."), candidates)
}

func commandBarMultiStepPlanResponse(steps []model.CommandBarPlanStep, rationale string, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	planKind := commandBarPlanKindForSteps(steps)
	guardrails := []model.CommandBarGuardrail{}
	if len(steps) > maxCommandBarPlanSteps {
		steps = steps[:maxCommandBarPlanSteps]
		planKind = commandBarPlanKindForSteps(steps)
		guardrails = append(guardrails, model.CommandBarGuardrail{
			Type:     "run_count_limit",
			Severity: "warning",
			Message:  fmt.Sprintf("Plan was limited to %d steps.", maxCommandBarPlanSteps),
		})
	}
	return &model.CommandBarParseResponse{
		Status: model.CommandBarParseStatusPlan,
		Plan: &model.CommandBarPlan{
			PlanKind:       planKind,
			Steps:          steps,
			RunCount:       len(steps),
			EstimatedRuns:  len(steps),
			MaxAllowedRuns: maxCommandBarPlanSteps,
			Guardrails:     guardrails,
		},
		Rationale:  firstNonEmptyString(strings.TrimSpace(rationale), "Matched request to available agents."),
		Candidates: candidates,
	}
}

func parseOneShotCommandIntent(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	agent, ok := findCommandBarCandidateByPreset(candidates, model.AgentPresetCommandAgent)
	if !ok {
		return nil
	}
	allowedTools, ok := oneShotCommandToolsForIntent(text, pageContext, agent.AllowedTools)
	if !ok || len(allowedTools) == 0 {
		return nil
	}
	instructions := commandBarOneShotInstructions(text, pageContext, allowedTools)
	step := model.CommandBarPlanStep{
		AgentID:      agent.ID,
		AgentKey:     agent.PresetKey,
		AgentName:    firstNonEmptyString(strings.TrimSpace(agent.Name), "Command Agent"),
		PlanKind:     model.CommandBarPlanKindOneShotCommand,
		Target:       pageContext,
		Instructions: instructions,
		AllowedTools: allowedTools,
	}
	resp := commandBarMultiStepPlanResponse(
		[]model.CommandBarPlanStep{step},
		"Prepared a one-shot command agent because no narrower saved agent matched this request.",
		candidates,
	)
	resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
		Type:     "one_shot_command",
		Severity: "info",
		Message:  "This is a one-shot run. It is not saved as a reusable agent unless you promote it after completion.",
	})
	return resp
}

func parseSafeOneShotCommandFallback(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || commandBarUnsafeOneShotPrompt(lower) {
		return nil
	}
	agent, ok := findCommandBarCandidateByPreset(candidates, model.AgentPresetCommandAgent)
	if !ok {
		return nil
	}
	allowedTools := safeOneShotCommandToolsForTarget(pageContext, agent.AllowedTools)
	if len(allowedTools) == 0 || !commandBarHasUsefulOneShotContextTool(allowedTools) {
		return nil
	}
	instructions := commandBarOneShotInstructions(text, pageContext, allowedTools)
	instructions += "\n\nFallback routing:\n- No saved agent matched this request with enough confidence.\n- Treat this as a read-only one-shot unless the user explicitly confirms a later mutation.\n- Answer from the enabled target/context tools, or ask one concise clarification if the context is insufficient."
	step := model.CommandBarPlanStep{
		AgentID:      agent.ID,
		AgentKey:     agent.PresetKey,
		AgentName:    firstNonEmptyString(strings.TrimSpace(agent.Name), "Command Agent"),
		PlanKind:     model.CommandBarPlanKindOneShotCommand,
		Target:       pageContext,
		Instructions: instructions,
		AllowedTools: allowedTools,
	}
	resp := commandBarMultiStepPlanResponse(
		[]model.CommandBarPlanStep{step},
		"No saved agent matched confidently, so this will run once with the Command Agent and read-only context tools.",
		candidates,
	)
	resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
		Type:     "one_shot_command",
		Severity: "info",
		Message:  "This is a one-shot run. It is not saved as a reusable agent unless you promote it after completion.",
	}, model.CommandBarGuardrail{
		Type:     "read_only_fallback",
		Severity: "info",
		Message:  "Fallback routing enabled only read-only context tools for this target.",
	})
	return resp
}

func parsePreferredOneShotCommandIntent(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if !shouldPreferOneShotCommandIntent(text, pageContext) {
		return nil
	}
	return parseOneShotCommandIntent(text, pageContext, candidates)
}

func shouldPreferOneShotCommandIntent(text string, pageContext model.CommandBarPageContext) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	targetType := commandBarEffectiveTargetType(pageContext)
	if shouldPreferOneShotPMTaskAnalysis(lower, targetType) {
		return true
	}
	if targetType != "crm_contact" && targetType != "crm_deal" {
		return false
	}
	if !containsAny(lower, "contact", "company", "account", "crm") {
		return false
	}
	return containsAny(lower,
		"find info",
		"find information",
		"research",
		"enrich",
		"update contact",
		"update company",
		"update account",
		"refresh contact",
		"refresh company",
	)
}

func shouldPreferOneShotPMTaskAnalysis(lower, targetType string) bool {
	if targetType != "workspace" && targetType != "task" && targetType != "epic" && targetType != "all_tasks" {
		return false
	}
	if targetType != "all_tasks" && !containsAny(lower, "task", "tasks", "story", "stories") {
		return false
	}
	if containsAny(lower,
		"create task",
		"create tasks",
		"new task",
		"new tasks",
		"create story",
		"new story",
		"break down",
		"breakdown",
		"turn this into tasks",
		"acceptance criteria",
	) {
		return false
	}
	return containsAny(lower,
		"how many",
		"count",
		"number of",
		"needs attention",
		"need attention",
		"attention",
		"blocked",
		"stale",
		"overdue",
		"due",
		"status",
		"statuses",
		"what tasks",
		"which tasks",
		"what story",
		"what stories",
		"which story",
		"which stories",
		"list tasks",
		"show tasks",
		"open tasks",
		"unassigned",
		"assigned",
		"priority",
		"high priority",
		"important",
		"most important",
		"highest priority",
		"rank",
		"ranking",
		"prioritize",
	)
}

func commandBarFanOutPlanResponse(agent model.CommandBarAgent, targets []model.CommandBarPageContext, text, rationale string, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if len(targets) == 0 {
		return nil
	}
	steps := make([]model.CommandBarPlanStep, 0, min(len(targets), maxCommandBarPlanSteps))
	for i, target := range targets {
		if i >= maxCommandBarPlanSteps {
			break
		}
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:      agent.ID,
			AgentKey:     agent.PresetKey,
			AgentName:    agent.Name,
			PlanKind:     model.CommandBarPlanKindFanOut,
			Target:       target,
			Instructions: firstNonEmptyString(strings.TrimSpace(text), fmt.Sprintf("Run on %s.", target.DisplayTitle)),
		})
	}
	resp := commandBarMultiStepPlanResponse(steps, firstNonEmptyString(strings.TrimSpace(rationale), "Prepared a fan-out plan across selected targets."), candidates)
	resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
		Type:     "fan_out_confirmation",
		Severity: "warning",
		Message:  fmt.Sprintf("Fan-out will start %d independent runs at once. Review the target list before confirming.", len(resp.Plan.Steps)),
	})
	if len(targets) > maxCommandBarPlanSteps {
		resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
			Type:     "fan_out_target_limit",
			Severity: "warning",
			Message:  fmt.Sprintf("Fan-out is limited to the first %d targets.", maxCommandBarPlanSteps),
		})
	}
	return resp
}

func parseFanOutIntent(text string, pageContext model.CommandBarPageContext, narrowCandidates, allCandidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	targets := commandBarFanOutTargetsFromContext(text, pageContext)
	if len(targets) == 0 {
		return nil
	}
	agent, ok := commandBarFanOutAgentForText(text, pageContext, narrowCandidates)
	if !ok {
		return nil
	}
	return commandBarFanOutPlanResponse(agent, targets, text, "Prepared a fan-out plan across concrete related targets.", allCandidates)
}

func (s *CommandBarService) parseEpicTaskPipelineIntent(ctx context.Context, workspaceID, text string, pageContext model.CommandBarPageContext, agents []model.Agent) *model.CommandBarParseResponse {
	if normalizeCommandBarTargetType(pageContext.EntityType) != "epic" {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	if !containsAny(lower, "task", "tasks", "story", "stories") {
		return nil
	}
	if !containsAny(lower, "all", "each", "every", "fan out", "fan-out", "parallel") {
		return nil
	}
	explicitForgeLens := containsAny(lower, "forge", "code builder") && containsAny(lower, "lens", "review agent", "review")
	dependencyAware := commandBarEpicTaskDependencyIntent(lower)
	if !explicitForgeLens && !dependencyAware {
		return nil
	}
	taskCandidates := commandBarCandidatesForTarget(agents, "task")
	forge, okForge := findCommandBarCandidateByPreset(taskCandidates, model.AgentPresetCodeBuilder)
	if !okForge {
		forge, okForge = findCommandBarCandidateByName(taskCandidates, "forge")
	}
	lens, okLens := findCommandBarCandidateByPreset(taskCandidates, model.AgentPresetReviewAgent)
	if !okLens {
		lens, okLens = findCommandBarCandidateByName(taskCandidates, "lens")
	}
	commandAgent, okCommandAgent := findCommandBarCandidateByPreset(commandBarAllAgentCandidates(agents), model.AgentPresetCommandAgent)
	if !okCommandAgent {
		commandAgent, okCommandAgent = findCommandBarCandidateByName(commandBarAllAgentCandidates(agents), "Command Agent")
	}
	if !okForge || !okLens || !okCommandAgent {
		missing := []string{}
		if !okForge {
			missing = append(missing, "Forge")
		}
		if !okLens {
			missing = append(missing, "Lens")
		}
		if !okCommandAgent {
			missing = append(missing, "Command Agent")
		}
		return &model.CommandBarParseResponse{
			Status:      model.CommandBarParseStatusNoMatchingAgent,
			Reason:      fmt.Sprintf("Dependency-aware epic execution requires %s to run on task targets.", strings.Join(missing, " and ")),
			Suggestions: defaultCommandBarSuggestions("task"),
			Candidates:  taskCandidates,
		}
	}
	tasks := s.commandBarEpicTasks(ctx, workspaceID, pageContext)
	var skippedTasks []commandBarSkippedTask
	tasks, skippedTasks = commandBarFilterCompletedTasks(tasks, skippedTasks)
	tasks, skippedTasks = s.commandBarUnmergedEpicTasks(ctx, workspaceID, pageContext.EntityID, tasks, skippedTasks)
	if len(tasks) == 0 {
		return &model.CommandBarParseResponse{
			Status: model.CommandBarParseStatusNoMatchingAgent,
			Reason: commandBarSkippedTasksMessage(
				skippedTasks,
				"All epic tasks are already completed or merged into the epic branch.",
			),
			Suggestions: defaultCommandBarSuggestions("task"),
			Candidates:  taskCandidates,
		}
	}
	slices.SortFunc(tasks, func(a, b model.PMTask) int {
		if a.DisplayID < b.DisplayID {
			return -1
		}
		if a.DisplayID > b.DisplayID {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	links := s.commandBarTaskDependencyLinks(ctx, workspaceID, tasks)
	steps := make([]model.CommandBarPlanStep, 0, min(len(tasks)*3+2, maxCommandBarPlanSteps))
	ensureEpicBranchIndex := len(steps)
	steps = append(steps, model.CommandBarPlanStep{
		AgentID:      commandAgent.ID,
		AgentKey:     commandAgent.PresetKey,
		AgentName:    commandAgent.Name,
		PlanKind:     model.CommandBarPlanKindTaskPipeline,
		StepType:     model.CommandBarStepTypeEnsureEpicBranch,
		Target:       pageContext,
		Instructions: fmt.Sprintf("Create or reuse the epic integration branch for %q from the configured base branch, then configure child task branches to use that epic branch as base.", pageContext.DisplayTitle),
	})
	forgeStepByTask := map[string]int{}
	mergeStepByTask := map[string]int{}
	for _, task := range tasks {
		if len(steps)+3+1 > maxCommandBarPlanSteps {
			break
		}
		target := model.CommandBarPageContext{
			EntityType:   "task",
			EntityID:     task.ID,
			DisplayTitle: commandBarTaskDisplayTitle(task),
			RelatedIDs: map[string][]string{
				"epic_ids": {pageContext.EntityID},
			},
		}
		forgeIndex := len(steps)
		forgeStepByTask[task.ID] = forgeIndex
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:              forge.ID,
			AgentKey:             forge.PresetKey,
			AgentName:            forge.Name,
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               target,
			Instructions:         fmt.Sprintf("Complete the implementation work for %s on its task branch. The task branch is based on this epic's integration branch. Respect task dependencies; this task is part of epic %q. Do not run Lens yourself; Helpin schedules review as the next command-bar step.", commandBarTaskDisplayTitle(task), pageContext.DisplayTitle),
			DependsOnStepIndexes: []int{ensureEpicBranchIndex},
		})
		lensIndex := len(steps)
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:              lens.ID,
			AgentKey:             lens.PresetKey,
			AgentName:            lens.Name,
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               target,
			Instructions:         fmt.Sprintf("Review the completed Forge work for %s. Use the linked Forge run as prior-step context when available.", commandBarTaskDisplayTitle(task)),
			DependsOnStepIndexes: []int{forgeIndex},
		})
		mergeIndex := len(steps)
		mergeStepByTask[task.ID] = mergeIndex
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:              commandAgent.ID,
			AgentKey:             commandAgent.PresetKey,
			AgentName:            commandAgent.Name,
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			StepType:             model.CommandBarStepTypeMergeTaskToEpic,
			Target:               target,
			Instructions:         fmt.Sprintf("Merge %s's task branch into the epic integration branch after Lens completes.", commandBarTaskDisplayTitle(task)),
			DependsOnStepIndexes: []int{lensIndex},
		})
	}
	crossTaskEdges := 0
	for _, link := range links {
		sourceMerge, okSource := mergeStepByTask[link.SourceTaskID]
		targetForge, okTarget := forgeStepByTask[link.TargetTaskID]
		if !okSource || !okTarget {
			continue
		}
		steps[targetForge].DependsOnStepIndexes = appendUniqueInt(steps[targetForge].DependsOnStepIndexes, sourceMerge)
		crossTaskEdges++
	}
	if len(steps) == 0 {
		return nil
	}
	finalDeps := make([]int, 0, len(mergeStepByTask))
	for _, task := range tasks {
		if mergeIndex, ok := mergeStepByTask[task.ID]; ok {
			finalDeps = append(finalDeps, mergeIndex)
		}
	}
	if len(finalDeps) > 0 && len(steps)+1 <= maxCommandBarPlanSteps {
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:              commandAgent.ID,
			AgentKey:             commandAgent.PresetKey,
			AgentName:            commandAgent.Name,
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			StepType:             model.CommandBarStepTypeOpenEpicPullRequest,
			Target:               pageContext,
			Instructions:         fmt.Sprintf("Open or reuse the final pull request from the epic integration branch for %q into the configured base branch.", pageContext.DisplayTitle),
			DependsOnStepIndexes: finalDeps,
		})
	}
	resp := commandBarMultiStepPlanResponse(steps, "Prepared a dependency-aware Forge then Lens pipeline across epic tasks.", taskCandidates)
	resp.Plan.PlanKind = model.CommandBarPlanKindTaskPipeline
	resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
		Type:     "temporal_orchestration",
		Severity: "info",
		Message:  "Temporal will run unblocked task pipelines in parallel and start Lens as soon as each Forge run completes.",
	})
	if len(skippedTasks) > 0 {
		resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
			Type:     "task_pipeline_skipped_tasks",
			Severity: "info",
			Message:  commandBarSkippedTasksMessage(skippedTasks, ""),
		})
	}
	if dependencyAware {
		// Be honest about what the link graph actually contributed: claiming
		// dependencies "were used" when zero blocking links matched reads as
		// the planner ignoring the user's ordering constraints.
		message := "No blocking links found between these tasks — all task pipelines run in parallel."
		if crossTaskEdges == 1 {
			message = "1 blocking link between tasks was used to order the DAG."
		} else if crossTaskEdges > 1 {
			message = fmt.Sprintf("%d blocking links between tasks were used to order the DAG.", crossTaskEdges)
		}
		resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
			Type:     "task_dependency_context",
			Severity: "info",
			Message:  message,
		})
	}
	if len(tasks)*2 > len(steps) {
		resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
			Type:     "run_count_limit",
			Severity: "warning",
			Message:  fmt.Sprintf("Pipeline was limited to %d runs.", maxCommandBarPlanSteps),
		})
	}
	return resp
}

func commandBarEpicTaskDependencyIntent(lower string) bool {
	return containsAny(
		lower,
		"dependency",
		"dependencies",
		"depends on",
		"depend on",
		"blocked",
		"blocker",
		"blocks",
		"blocking",
		"dag",
		"parallel if",
		"if they dont have any dependencies",
		"if they don't have any dependencies",
		"if they do not have any dependencies",
	)
}

func (s *CommandBarService) commandBarEpicTasks(ctx context.Context, workspaceID string, pageContext model.CommandBarPageContext) []model.PMTask {
	ids := normalizeStringSlice(append(pageContext.RelatedIDs["task_ids"], pageContext.RelatedIDs["story_ids"]...))
	if len(ids) > 0 && s != nil && s.agentService != nil && s.agentService.taskRepo != nil {
		tasks, err := s.agentService.taskRepo.ListByIDs(ctx, workspaceID, ids)
		if err == nil {
			return tasks
		}
		slog.WarnContext(ctx, "command bar epic task lookup by ids failed", "error", err, "workspace_id", workspaceID, "epic_id", pageContext.EntityID)
	}
	if s == nil || s.agentService == nil || s.agentService.taskRepo == nil {
		return nil
	}
	tasks, err := s.agentService.taskRepo.ListByEpicID(ctx, workspaceID, pageContext.EntityID)
	if err != nil {
		slog.WarnContext(ctx, "command bar epic task lookup failed", "error", err, "workspace_id", workspaceID, "epic_id", pageContext.EntityID)
		return nil
	}
	return tasks
}

type commandBarSkippedTask struct {
	ID     string
	Title  string
	Reason string
}

func commandBarFilterCompletedTasks(tasks []model.PMTask, skipped []commandBarSkippedTask) ([]model.PMTask, []commandBarSkippedTask) {
	if len(tasks) == 0 {
		return tasks, skipped
	}
	filtered := make([]model.PMTask, 0, len(tasks))
	for _, task := range tasks {
		if task.Completed {
			skipped = append(skipped, commandBarSkippedTask{
				ID:     task.ID,
				Title:  commandBarTaskDisplayTitle(task),
				Reason: "completed",
			})
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered, skipped
}

func (s *CommandBarService) commandBarUnmergedEpicTasks(ctx context.Context, workspaceID, epicID string, tasks []model.PMTask, skipped []commandBarSkippedTask) ([]model.PMTask, []commandBarSkippedTask) {
	if len(tasks) == 0 || s == nil || s.agentService == nil {
		return tasks, skipped
	}
	filtered := make([]model.PMTask, 0, len(tasks))
	for _, task := range tasks {
		merged, err := s.commandBarTaskAlreadyMergedToEpic(ctx, workspaceID, epicID, task.ID)
		if err != nil {
			slog.WarnContext(ctx, "command bar task merge-state lookup failed", "error", err, "workspace_id", workspaceID, "epic_id", epicID, "task_id", task.ID)
			filtered = append(filtered, task)
			continue
		}
		if merged {
			skipped = append(skipped, commandBarSkippedTask{
				ID:     task.ID,
				Title:  commandBarTaskDisplayTitle(task),
				Reason: "merged into the epic branch",
			})
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered, skipped
}

func (s *CommandBarService) commandBarTaskAlreadyMergedToEpic(ctx context.Context, workspaceID, epicID, taskID string) (bool, error) {
	if s == nil || s.agentService == nil || strings.TrimSpace(taskID) == "" {
		return false, nil
	}
	if s.agentService.gitService != nil {
		target, err := s.agentService.gitService.GetTaskDeliveryTarget(ctx, workspaceID, taskID)
		if err != nil {
			return false, err
		}
		if target != nil && commandBarTaskDeliveryTargetIsMerged(*target, epicID) {
			return true, nil
		}
	}
	if s.agentService.runRepo != nil {
		run, err := s.agentService.runRepo.FindCompletedByTargetStage(ctx, workspaceID, "task", taskID, model.CommandBarStepTypeMergeTaskToEpic)
		if err != nil {
			return false, err
		}
		if run != nil {
			return true, nil
		}
		runs, err := s.agentService.runRepo.ListByTarget(ctx, workspaceID, "task", taskID)
		if err != nil {
			return false, err
		}
		if slices.ContainsFunc(runs, commandBarRunLooksLikeCompletedTaskMerge) {
			return true, nil
		}
	}
	return false, nil
}

func commandBarRunLooksLikeCompletedTaskMerge(run model.AgentRun) bool {
	if run.Status != model.AgentRunStatusCompleted {
		return false
	}
	if strings.TrimSpace(derefString(run.ExecutionStage)) == model.CommandBarStepTypeMergeTaskToEpic {
		return true
	}
	var output commandBarOrchestrationOutput
	if len(run.OutputSummary) > 0 {
		_ = json.Unmarshal(run.OutputSummary, &output)
	}
	if strings.TrimSpace(output.Type) == model.CommandBarStepTypeMergeTaskToEpic {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(output.Message))
	if strings.Contains(message, "merged into epic branch") || strings.Contains(message, "task branch merged") {
		return true
	}
	raw := strings.ToLower(string(run.OutputSummary))
	return strings.Contains(raw, model.CommandBarStepTypeMergeTaskToEpic) || strings.Contains(raw, "merged into epic branch")
}

func commandBarSkippedTasksMessage(skipped []commandBarSkippedTask, fallback string) string {
	if len(skipped) == 0 {
		return fallback
	}
	labels := make([]string, 0, min(len(skipped), 3))
	for i, task := range skipped {
		if i >= 3 {
			break
		}
		label := strings.TrimSpace(task.Title)
		if label == "" {
			label = strings.TrimSpace(task.ID)
		}
		if label == "" {
			label = "untitled task"
		}
		reason := strings.TrimSpace(task.Reason)
		if reason != "" {
			label = fmt.Sprintf("%s (%s)", label, reason)
		}
		labels = append(labels, label)
	}
	remainder := len(skipped) - len(labels)
	taskWord := "task"
	if len(skipped) != 1 {
		taskWord = "tasks"
	}
	message := fmt.Sprintf("Skipped %d %s already completed or merged into the epic branch: %s", len(skipped), taskWord, strings.Join(labels, ", "))
	if remainder > 0 {
		message = fmt.Sprintf("%s, and %d more", message, remainder)
	}
	return message + "."
}

func commandBarTaskDeliveryTargetIsMerged(target model.TaskDeliveryTarget, epicID string) bool {
	if strings.TrimSpace(target.DeliveryState) != "merged" && strings.TrimSpace(derefString(target.ActivePRStatus)) != "merged" {
		return false
	}
	sourceEpicID := strings.TrimSpace(derefString(target.SourceEpicID))
	return sourceEpicID == "" || strings.TrimSpace(epicID) == "" || sourceEpicID == strings.TrimSpace(epicID)
}

func (s *CommandBarService) commandBarTaskDependencyLinks(ctx context.Context, workspaceID string, tasks []model.PMTask) []model.PMTaskLink {
	if s == nil || s.agentService == nil || s.agentService.taskLinkRepo == nil || len(tasks) == 0 {
		return nil
	}
	ids := make([]string, 0, len(tasks))
	inSet := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
		inSet[task.ID] = true
	}
	links, err := s.agentService.taskLinkRepo.ListByTasks(ctx, workspaceID, ids)
	if err != nil {
		slog.WarnContext(ctx, "command bar task dependency lookup failed", "error", err, "workspace_id", workspaceID)
		return nil
	}
	return slices.DeleteFunc(links, func(link model.PMTaskLink) bool {
		return link.LinkType != model.PMTaskLinkTypeBlocks || !inSet[link.SourceTaskID] || !inSet[link.TargetTaskID]
	})
}

func commandBarTaskDisplayTitle(task model.PMTask) string {
	if task.DisplayID > 0 {
		return fmt.Sprintf("#%d %s", task.DisplayID, strings.TrimSpace(task.Name))
	}
	return strings.TrimSpace(firstNonEmptyString(task.Name, shortCommandBarID(task.ID)))
}

func appendUniqueInt(values []int, next int) []int {
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

func commandBarFanOutAgentForText(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) (model.CommandBarAgent, bool) {
	if len(candidates) == 0 {
		return model.CommandBarAgent{}, false
	}
	for _, candidate := range candidates {
		if indexCommandBarPhrase(text, candidate.Name) >= 0 {
			return candidate, true
		}
		if candidate.PresetKey != "" && indexCommandBarPhrase(text, strings.ReplaceAll(candidate.PresetKey, "_", " ")) >= 0 {
			return candidate, true
		}
	}
	if parsed := parseIntentDeterministically(text, pageContext, candidates); parsed != nil && parsed.Plan != nil && len(parsed.Plan.Steps) > 0 {
		return findCommandBarCandidateByID(candidates, parsed.Plan.Steps[0].AgentID)
	}
	return model.CommandBarAgent{}, false
}

func commandBarFanOutTargetsFromContext(text string, pageContext model.CommandBarPageContext) []model.CommandBarPageContext {
	lower := strings.ToLower(strings.TrimSpace(text))
	if !containsAny(lower, "all", "each", "every", "across", "fan out", "fan-out", "multiple") {
		return nil
	}
	if len(pageContext.RelatedIDs) == 0 {
		return nil
	}
	type candidate struct {
		keys       []string
		targetType string
		label      string
	}
	candidates := []candidate{
		{keys: []string{"task_ids", "story_ids", "stories", "tasks"}, targetType: "task", label: "Task"},
		{keys: []string{"document_ids", "doc_ids", "documents", "docs"}, targetType: "document", label: "Document"},
		{keys: []string{"crm_deal_ids", "deal_ids", "deals"}, targetType: "crm_deal", label: "Deal"},
		{keys: []string{"crm_contact_ids", "contact_ids", "contacts"}, targetType: "crm_contact", label: "Contact"},
	}
	for _, item := range candidates {
		ids := []string{}
		for _, key := range item.keys {
			ids = append(ids, pageContext.RelatedIDs[key]...)
		}
		ids = normalizeStringSlice(ids)
		if len(ids) == 0 {
			continue
		}
		if !fanOutTextMentionsTarget(lower, item.targetType) {
			continue
		}
		targets := make([]model.CommandBarPageContext, 0, len(ids))
		for _, id := range ids {
			targets = append(targets, model.CommandBarPageContext{
				EntityType:   item.targetType,
				EntityID:     id,
				DisplayTitle: fmt.Sprintf("%s %s", item.label, shortCommandBarID(id)),
			})
		}
		return targets
	}
	return nil
}

func fanOutTextMentionsTarget(text, targetType string) bool {
	switch targetType {
	case "task":
		return containsAny(text, "task", "tasks", "story", "stories", "child")
	case "document":
		return containsAny(text, "doc", "docs", "document", "documents", "article", "articles")
	case "crm_deal":
		return containsAny(text, "deal", "deals")
	case "crm_contact":
		return containsAny(text, "contact", "contacts")
	default:
		return false
	}
}

func shortCommandBarID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func oneShotCommandToolsForIntent(text string, pageContext model.CommandBarPageContext, agentTools []string) ([]string, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || commandBarUnsafeOneShotPrompt(lower) {
		return nil, false
	}
	tools := []string{"update_plan", "request_user_input"}
	recognized := false
	targetType := normalizeCommandBarTargetType(pageContext.EntityType)
	effectiveTargetType := commandBarEffectiveTargetType(pageContext)
	hasConcreteDocument := targetType == "document" && strings.TrimSpace(pageContext.EntityID) != ""

	if targetType == "document" || containsAny(lower, "doc", "document", "article", "knowledge base", "stale") {
		recognized = true
		tools = append(tools, "read_document", "get_document_blocks")
		if !hasConcreteDocument || commandBarPromptRequestsDocumentSearch(lower) {
			tools = append(tools, "list_spaces", "list_documents", "list_collections", "search_documents")
		}
	}
	if containsAny(lower, "web", "website", "url", "internet", "research", "source", "sources", "stale", "latest", "fetch", "crawl", "find info", "find information", "enrich") {
		recognized = true
		tools = append(tools, "web_search_exa", "web_search_brave", "fetch_url", "crawl_url")
	}
	if targetType == "repository" || containsAny(lower, "commit", "commits", "changelog", "change log", "release notes", "repository", "repo", "git history", "git log", "codebase", "source code") {
		recognized = true
		// Read-only repo tools only — no run_command, no writes. list_repositories
		// lets the agent discover/confirm the repo; the rest read the clone once the
		// run targets a repository.
		tools = append(tools, "list_repositories", "list_commits", "read_file", "read_file_range", "read_files", "list_directory", "search_files", "ripgrep", "grep", "list_symbols")
	}
	if targetType == "document" && containsAny(lower, "link", "attach", "associate", "reference") && containsAny(lower, "task", "story", "epic", "deal", "contact", "company", "crm", "support") {
		recognized = true
		tools = append(tools, "request_approval", "link_document_to_object")
	}
	if containsAny(lower, "create doc", "create document", "new doc", "new document", "draft doc", "draft document", "write a doc", "write an article", "changelog", "create a changelog") {
		recognized = true
		// Discovery tools so the agent can resolve a space/collection (and ask the
		// user when several exist) instead of failing on a missing space_id.
		tools = append(tools, "list_spaces", "list_collections", "create_document")
	}
	if effectiveTargetType == "all_tasks" {
		recognized = true
		tools = append(tools, "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks")
	} else if targetType == "task" || containsAny(lower, "task", "story", "comment") {
		tools = append(tools, "get_task_context")
	}
	if shouldPreferOneShotPMTaskAnalysis(lower, effectiveTargetType) {
		recognized = true
		tools = append(tools, "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks")
	}
	if containsAny(lower, "comment", "add note", "write note") && (targetType == "task" || containsAny(lower, "task", "story")) {
		recognized = true
		tools = append(tools, "add_task_comment")
	}
	if containsAny(lower, "create task", "create tasks", "new task", "new tasks", "create story", "new story", "make a task", "turn this into tasks", "follow-up task", "follow up task") {
		recognized = true
		tools = append(tools, "list_workspace_teams", "list_team_workflows_with_stages", "create_task")
	}
	if targetType == "crm_contact" || targetType == "crm_deal" || containsAny(lower, "crm", "deal", "contact", "buyer", "pipeline") {
		recognized = true
		tools = append(tools, "list_deals", "list_contacts", "list_buyer_signals")
	}
	if (targetType == "crm_contact" || targetType == "crm_deal") && containsAny(lower, "update", "refresh", "enrich", "find info", "find information", "research") {
		recognized = true
		tools = append(tools, "request_approval")
		if targetType == "crm_contact" || containsAny(lower, "contact") {
			tools = append(tools, "enrich_crm_contact")
		}
		if containsAny(lower, "company", "account") {
			tools = append(tools, "ensure_crm_contact_company", "enrich_crm_company")
		}
	}
	if containsAny(lower, "deal note", "crm note", "add note to deal") {
		recognized = true
		tools = append(tools, "add_deal_note")
	}
	if containsAny(lower, "update deal", "move deal", "change stage") {
		recognized = true
		tools = append(tools, "request_approval", "update_deal_stage")
	}
	if containsAny(lower, "summarize", "explain", "answer", "compare", "analyze", "find", "check") {
		recognized = true
	}
	if !recognized {
		return nil, false
	}
	allowedSet := make(map[string]bool, len(agentTools))
	for _, tool := range agentTools {
		allowedSet[strings.TrimSpace(tool)] = true
	}
	filtered := make([]string, 0, len(tools))
	seen := map[string]bool{}
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		if len(allowedSet) > 0 && !allowedSet[tool] {
			continue
		}
		seen[tool] = true
		filtered = append(filtered, tool)
	}
	return filtered, len(filtered) > 0
}

func commandBarUnsafeOneShotPrompt(lower string) bool {
	return containsAny(lower,
		"delete workspace",
		"remove workspace",
		"delete task",
		"delete tasks",
		"delete this task",
		"remove task",
		"remove tasks",
		"remove this task",
		"delete epic",
		"delete this epic",
		"remove epic",
		"remove this epic",
		"delete all",
		"remove all",
		"delete all tasks",
		"remove all tasks",
	)
}

func safeOneShotCommandToolsForTarget(pageContext model.CommandBarPageContext, agentTools []string) []string {
	tools := []string{"update_plan", "request_user_input"}
	switch normalizeCommandBarTargetType(pageContext.EntityType) {
	case "workspace", "epic":
		tools = append(tools, "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks")
	case "task":
		tools = append(tools, "get_task_context", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks")
	case "document":
		tools = append(tools, "read_document", "get_document_blocks", "web_search_exa", "web_search_brave", "fetch_url", "crawl_url", "list_deals", "list_contacts", "list_buyer_signals")
		if strings.TrimSpace(pageContext.EntityID) == "" {
			tools = append(tools, "list_spaces", "list_documents", "list_collections", "search_documents")
		}
	case "crm_contact", "crm_deal":
		tools = append(tools, "list_deals", "list_contacts", "list_buyer_signals")
	case "repository":
		tools = append(tools, "list_repositories", "list_commits", "read_file", "read_file_range", "read_files", "list_directory", "search_files", "ripgrep", "grep", "list_symbols")
	default:
		return nil
	}
	return commandBarFilterAllowedTools(tools, agentTools)
}

func commandBarFilterAllowedTools(tools, agentTools []string) []string {
	allowedSet := make(map[string]bool, len(agentTools))
	for _, tool := range agentTools {
		allowedSet[strings.TrimSpace(tool)] = true
	}
	filtered := make([]string, 0, len(tools))
	seen := map[string]bool{}
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		if len(allowedSet) > 0 && !allowedSet[tool] {
			continue
		}
		seen[tool] = true
		filtered = append(filtered, tool)
	}
	return filtered
}

func commandBarPromptRequestsDocumentSearch(lower string) bool {
	return containsAny(lower,
		"search docs",
		"search documents",
		"search the docs",
		"find document",
		"find documents",
		"find a doc",
		"find docs",
		"other docs",
		"other documents",
		"across docs",
		"across documents",
		"knowledge base",
	)
}

func commandBarHasUsefulOneShotContextTool(tools []string) bool {
	for _, tool := range tools {
		switch strings.TrimSpace(tool) {
		case "get_task_context",
			"list_workspace_teams",
			"list_team_workflows_with_stages",
			"list_tasks",
			"list_documents",
			"list_collections",
			"read_document",
			"get_document_blocks",
			"search_documents",
			"list_repositories",
			"list_commits",
			"read_file",
			"read_file_range",
			"read_files",
			"list_directory",
			"search_files",
			"ripgrep",
			"grep",
			"list_symbols",
			"list_deals",
			"list_contacts",
			"list_buyer_signals":
			return true
		}
	}
	return false
}

func commandBarFilterOneShotTools(requestedTools, agentTools []string, toolIntent string) []string {
	allowedSet := make(map[string]bool, len(agentTools))
	for _, tool := range agentTools {
		allowedSet[strings.TrimSpace(tool)] = true
	}
	toolIntent = normalizeCommandBarToolIntent(toolIntent)
	filtered := make([]string, 0, len(requestedTools))
	seen := map[string]bool{}
	for _, tool := range requestedTools {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		if len(allowedSet) > 0 && !allowedSet[tool] {
			continue
		}
		if toolIntent == "read_only" && commandBarToolIsMutation(tool) {
			continue
		}
		seen[tool] = true
		filtered = append(filtered, tool)
	}
	return filtered
}

func commandBarCompleteRepositoryOneShotTools(text string, pageContext model.CommandBarPageContext, selectedTools, agentTools []string, toolIntent string) ([]string, string) {
	if normalizeCommandBarTargetType(pageContext.EntityType) != "repository" {
		return selectedTools, toolIntent
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	historyIntent := containsAny(lower, "commit", "commits", "changelog", "change log", "release notes", "git history", "git log")
	documentCreateIntent := containsAny(lower, "create doc", "create a doc", "create document", "create a document", "new doc", "new document", "write a doc", "changelog doc", "changelog document", "create a changelog")
	if !historyIntent && !documentCreateIntent {
		return selectedTools, toolIntent
	}

	desiredTools := []string{"update_plan", "request_user_input"}
	if historyIntent {
		desiredTools = append(desiredTools,
			"list_repositories",
			"list_commits",
			"read_file",
			"read_file_range",
			"read_files",
			"list_directory",
			"search_files",
			"ripgrep",
			"grep",
			"list_symbols",
		)
	}
	if documentCreateIntent {
		desiredTools = append(desiredTools, "list_spaces", "list_collections", "create_document")
	}

	merged := commandBarFilterAllowedTools(append(append([]string{}, selectedTools...), desiredTools...), agentTools)
	if len(merged) == 0 {
		return selectedTools, toolIntent
	}
	if commandBarToolsIncludeMutation(merged) {
		return merged, "mutate"
	}
	if normalizeCommandBarToolIntent(toolIntent) == "" {
		return merged, "read_only"
	}
	return merged, toolIntent
}

func normalizeCommandBarToolIntent(intent string) string {
	switch strings.ToLower(strings.TrimSpace(intent)) {
	case "read_only", "readonly", "read-only":
		return "read_only"
	case "propose_change", "proposal", "propose-change":
		return "propose_change"
	case "mutate", "mutation", "write":
		return "mutate"
	default:
		return ""
	}
}

func commandBarToolIntentFromTools(tools []string) string {
	if hasAnyTool(tools, "publish_document_change_proposal") {
		return "propose_change"
	}
	if commandBarToolsIncludeMutation(tools) {
		return "mutate"
	}
	return "read_only"
}

func commandBarValidatePlannerToolIntent(toolIntent string, tools []string) error {
	toolIntent = normalizeCommandBarToolIntent(toolIntent)
	switch toolIntent {
	case "read_only":
		if commandBarToolsIncludeMutation(tools) {
			return fmt.Errorf("read_only tool_intent cannot include mutation tools")
		}
	case "propose_change":
		if !hasAnyTool(tools, "publish_document_change_proposal") {
			return fmt.Errorf("propose_change tool_intent requires publish_document_change_proposal")
		}
	case "mutate":
		if !commandBarToolsIncludeMutation(tools) {
			return fmt.Errorf("mutate tool_intent requires at least one mutation tool")
		}
	default:
		return fmt.Errorf("one_shot_command requires tool_intent read_only, propose_change, or mutate")
	}
	return nil
}

func commandBarToolsIncludeMutation(tools []string) bool {
	for _, tool := range tools {
		if commandBarToolIsMutation(tool) {
			return true
		}
	}
	return false
}

func commandBarToolIsMutation(tool string) bool {
	switch strings.TrimSpace(tool) {
	case "add_deal_note",
		"add_task_comment",
		"create_document",
		"create_task",
		"enrich_crm_company",
		"enrich_crm_contact",
		"ensure_crm_contact_company",
		"ensure_task_label",
		"publish_document_change_proposal",
		"update_deal_stage",
		"update_task_state",
		"update_document_block",
		"link_document_to_object",
		"write_document_content":
		return true
	default:
		return false
	}
}

func commandBarOneShotInstructions(text string, pageContext model.CommandBarPageContext, tools []string) string {
	goal, plan, constraints := oneShotExecutionBrief(text, pageContext, tools)
	parts := []string{"One-shot execution brief"}
	parts = append(parts, "Goal:\n- "+goal)
	if len(plan) > 0 {
		lines := make([]string, 0, len(plan))
		for i, step := range plan {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, step))
		}
		parts = append(parts, "Plan:\n"+strings.Join(lines, "\n"))
	}
	constraints = append([]string{
		"Run as a one-shot command agent for the current target.",
		"Do not create or save a reusable agent.",
		"Use only the enabled tools for this run.",
	}, constraints...)
	if hasAnyTool(tools, "publish_document_change_proposal", "write_document_content", "create_document", "create_task", "add_task_comment", "add_deal_note", "update_deal_stage", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company") {
		constraints = append(constraints, "The user confirmed this command-bar plan; keep mutations limited to the requested action and target.")
	}
	if hasAnyTool(tools, "publish_document_change_proposal") {
		constraints = append(constraints, "For Docs edits, call publish_document_change_proposal once with the proposed replacement, then finish. Do not call request_approval for Docs proposals, and do not call direct document write tools for proposed edits.")
		if commandBarPageContextMetadataString(pageContext, "context_scope") == "block" {
			constraints = append(constraints, "For focused Docs block context, submit a block-scoped proposal with the supplied block_id and block_revision unless the user explicitly asks for a whole-document replacement.")
		}
	}
	if len(constraints) > 0 {
		lines := make([]string, 0, len(constraints))
		for _, constraint := range constraints {
			lines = append(lines, "- "+constraint)
		}
		parts = append(parts, "Constraints:\n"+strings.Join(lines, "\n"))
	}
	if pageContext.EntityType != "" || pageContext.EntityID != "" {
		parts = append(parts, fmt.Sprintf("Target: %s %s (%s).", pageContext.EntityType, pageContext.EntityID, pageContext.DisplayTitle))
		if scopeInstruction := commandBarScopeInstruction(pageContext); scopeInstruction != "" {
			parts = append(parts, scopeInstruction)
		}
	}
	parts = append(parts, "User request:\n"+strings.TrimSpace(text))
	return strings.Join(parts, "\n\n")
}

func oneShotExecutionBrief(text string, pageContext model.CommandBarPageContext, tools []string) (string, []string, []string) {
	lower := strings.ToLower(strings.TrimSpace(text))
	targetType := commandBarEffectiveTargetType(pageContext)
	readOnly := !commandBarToolsIncludeMutation(tools)
	switch {
	case targetType == "crm_contact" || targetType == "crm_deal":
		goal := "Research the CRM target and produce high-confidence CRM updates for the requested contact, company, or deal context."
		plan := []string{
			"Review the current CRM target and related CRM context available through the enabled tools.",
			"Search the web for public contact, company, role, domain, and buyer-signal evidence.",
			"Fetch authoritative sources before relying on search snippets.",
			"Extract proposed CRM updates with source URLs and confidence notes.",
			"If the contact has no associated company but the evidence supports one, create or reuse the company and associate it before company enrichment.",
			"Apply only CRM mutations supported by the enabled tools; otherwise return exact proposed field changes for review.",
		}
		constraints := []string{
			"Do not invent contact, company, title, domain, funding, or employment facts.",
			"Use guarded CRM enrichment tools for CRM writes; names, existing email, existing phone, company name, and existing domain are protected server-side.",
			"Ask for approval before any high-impact CRM mutation.",
		}
		return goal, plan, constraints
	case shouldPreferOneShotPMTaskAnalysis(lower, targetType):
		goal := "Answer the requested task question using live workspace PM data."
		plan := []string{
			"Resolve the referenced team or workflow context when the request names one.",
			"List matching workspace tasks with read-only filters first; use compact results unless more detail is needed.",
			"Count and group the tasks according to the user's wording, explaining how ambiguous terms such as needs attention were interpreted.",
			"Return the answer with task names or IDs for any items that need follow-up.",
		}
		constraints := []string{
			"Do not create, update, or move tasks for an analytical question.",
			"Ask for clarification if the team or attention criteria cannot be resolved from available data.",
		}
		return goal, plan, constraints
	case targetType == "repository":
		goal := "Complete the requested repository investigation using the selected repository target and enabled tools."
		plan := []string{
			"Confirm the repository target and inspect recent commits on the requested branch or default branch.",
			"Read only the files needed to understand noteworthy changes behind the selected commits.",
			"Organize the findings into the requested changelog or release-note format.",
			"Create the document only if a document creation tool is enabled; otherwise return the changelog draft.",
		}
		constraints := []string{
			"Do not use workspace as a substitute for the repository target.",
			"Keep code inspection scoped to the commits and files needed for the changelog.",
			"Ask for clarification if the destination docs space or collection cannot be resolved safely.",
		}
		return goal, plan, constraints
	case targetType == "document" || containsAny(lower, "doc", "document", "article", "stale"):
		if readOnly {
			goal := "Answer the requested document question using the current document context and permitted read-only sources."
			plan := []string{
				"Read the current document by its provided document_id and identify the sections relevant to the request.",
				"Use web search only where outside evidence is needed; use document search only if the user asks to find other documents.",
				"Fetch source pages before treating web results as facts.",
				"Report findings and any suggested changes without submitting a Docs proposal.",
			}
			constraints := []string{
				"Do not create, update, or propose document changes because no Docs proposal/write tool is enabled.",
				"Do not replace sourced content with weaker evidence.",
				"Do not use search_documents to rediscover or inspect a known current document.",
			}
			return goal, plan, constraints
		}
		goal := "Research and update the document only where the requested change is supported by the current document context and sources."
		plan := []string{
			"Read the current document by its provided document_id and identify the sections relevant to the request.",
			"Use web search only where outside evidence is needed; use document search only if the user asks to find other documents.",
			"Fetch source pages before treating web results as facts.",
			"Draft the smallest safe content change that satisfies the request.",
			"Write the document only if the enabled tools support it; otherwise return the proposed patch.",
		}
		constraints := []string{
			"Preserve the document's existing structure and tone unless the user requested a rewrite.",
			"Do not replace sourced content with weaker evidence.",
			"Do not use search_documents to rediscover or inspect a known current document.",
			"Ask for approval before broad rewrites or uncertain factual changes.",
		}
		return goal, plan, constraints
	case targetType == "task" || containsAny(lower, "task", "story", "comment"):
		goal := "Complete the requested task-level action using the current task context and the enabled tools."
		plan := []string{
			"Review the current task context and identify the exact requested output.",
			"Gather any missing workspace/team context needed for the action.",
			"Create tasks or add comments only when the request is explicit and the enabled tools support it.",
			"Summarize what changed and any follow-up needed.",
		}
		constraints := []string{
			"Keep mutations limited to the current task or clearly requested workspace target.",
			"Do not create duplicate tasks when an existing task should be updated or referenced.",
		}
		return goal, plan, constraints
	default:
		goal := "Complete the confirmed one-shot command for the current target."
		plan := []string{
			"Review the provided target context and the user's request.",
			"Use the enabled tools to gather only the context needed for this command.",
			"Perform supported mutations carefully, or return proposed changes when a write tool is unavailable.",
			"Summarize the result and any sources or follow-up actions.",
		}
		constraints := []string{
			"Keep the run scoped to the confirmed command and target.",
			"Ask for clarification or approval when the request is ambiguous or risky.",
		}
		return goal, plan, constraints
	}
}

func hasAnyTool(tools []string, needles ...string) bool {
	for _, tool := range tools {
		if slices.Contains(needles, tool) {
			return true
		}
	}
	return false
}

func commandBarStepsUseMutationTools(steps []model.CommandBarPlanStep) bool {
	for _, step := range steps {
		for _, tool := range step.AllowedTools {
			if commandBarToolIsMutation(tool) {
				return true
			}
		}
	}
	return false
}

func commandBarPlanKindForSteps(steps []model.CommandBarPlanStep) string {
	if len(steps) == 0 {
		return model.CommandBarPlanKindKnownAgent
	}
	allTaskPipeline := true
	for _, step := range steps {
		if step.PlanKind != model.CommandBarPlanKindTaskPipeline {
			allTaskPipeline = false
			break
		}
	}
	if allTaskPipeline {
		return model.CommandBarPlanKindTaskPipeline
	}
	allDAG := true
	for _, step := range steps {
		if step.PlanKind != model.CommandBarPlanKindDAG {
			allDAG = false
			break
		}
	}
	if allDAG {
		return model.CommandBarPlanKindDAG
	}
	allFanOut := true
	for _, step := range steps {
		if step.PlanKind != model.CommandBarPlanKindFanOut {
			allFanOut = false
			break
		}
	}
	if allFanOut {
		return model.CommandBarPlanKindFanOut
	}
	for _, step := range steps {
		if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
			return model.CommandBarPlanKindKnownAgent
		}
	}
	return model.CommandBarPlanKindOneShotCommand
}

func isOneShotCommandAgent(agent model.CommandBarAgent) bool {
	return normalizePresetKey(agent.PresetKey) == model.AgentPresetCommandAgent
}

func parseExplicitNamedAgents(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	type match struct {
		index int
		agent model.CommandBarAgent
	}
	matches := make([]match, 0)
	for _, agent := range candidates {
		if idx := indexCommandBarPhrase(text, agent.Name); idx >= 0 {
			matches = append(matches, match{index: idx, agent: agent})
			continue
		}
		if agent.PresetKey != "" {
			presetPhrase := strings.ReplaceAll(strings.ToLower(agent.PresetKey), "_", " ")
			if idx := indexCommandBarPhrase(text, presetPhrase); idx >= 0 {
				matches = append(matches, match{index: idx, agent: agent})
			}
		}
	}
	if len(matches) == 0 {
		return nil
	}
	slices.SortFunc(matches, func(a, b match) int {
		if a.index < b.index {
			return -1
		}
		if a.index > b.index {
			return 1
		}
		return strings.Compare(a.agent.Name, b.agent.Name)
	})

	seen := map[string]bool{}
	agents := make([]model.CommandBarAgent, 0, len(matches))
	for _, item := range matches {
		if seen[item.agent.ID] {
			continue
		}
		seen[item.agent.ID] = true
		agents = append(agents, item.agent)
	}
	if len(agents) == 0 {
		return nil
	}
	if len(agents) == 1 && isOneShotCommandAgent(agents[0]) {
		return parseOneShotCommandIntent(text, pageContext, candidates)
	}
	agents = slices.DeleteFunc(agents, isOneShotCommandAgent)
	if len(agents) == 0 {
		return nil
	}
	steps := make([]model.CommandBarPlanStep, 0, len(agents))
	for i, agent := range agents {
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:      agent.ID,
			AgentKey:     agent.PresetKey,
			AgentName:    agent.Name,
			Target:       pageContext,
			Instructions: commandBarNamedAgentStepInstructions(agent, agents, i),
		})
	}
	rationale := "Matched explicit agent name."
	if len(steps) > 1 {
		rationale = "Matched explicit agent names in request order."
	}
	return commandBarMultiStepPlanResponse(steps, rationale, candidates)
}

func commandBarNamedAgentStepInstructions(agent model.CommandBarAgent, agents []model.CommandBarAgent, stepIndex int) string {
	name := strings.TrimSpace(agent.Name)
	if name == "" {
		name = "this agent"
	}
	parts := []string{
		fmt.Sprintf("Execute your normal %s role for the current target.", name),
	}
	if len(agents) > 1 {
		parts = append(parts, fmt.Sprintf("This is command-bar step %d of %d.", stepIndex+1, len(agents)))
		otherNames := make([]string, 0, len(agents)-1)
		for i, other := range agents {
			if i == stepIndex {
				continue
			}
			if otherName := strings.TrimSpace(other.Name); otherName != "" {
				otherNames = append(otherNames, otherName)
			}
		}
		if len(otherNames) > 0 {
			parts = append(parts, fmt.Sprintf("Do not invoke or run %s yourself; Helpin schedules those as separate command-bar steps.", strings.Join(otherNames, ", ")))
		}
		if stepIndex > 0 {
			parts = append(parts, "Use the previous linked run as prior-step context when it is available.")
		}
	}
	return strings.Join(parts, " ")
}

func indexCommandBarPhrase(text, phrase string) int {
	text = strings.ToLower(strings.TrimSpace(text))
	phrase = strings.ToLower(strings.TrimSpace(phrase))
	if text == "" || phrase == "" {
		return -1
	}
	offset := 0
	for offset < len(text) {
		idx := strings.Index(text[offset:], phrase)
		if idx < 0 {
			return -1
		}
		idx += offset
		beforeOK := idx == 0 || !isCommandBarWordByte(text[idx-1])
		after := idx + len(phrase)
		afterOK := after == len(text) || !isCommandBarWordByte(text[after])
		if beforeOK && afterOK {
			return idx
		}
		offset = idx + 1
	}
	return -1
}

func isCommandBarWordByte(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
}

func normalizeCommandBarPageContext(ctx model.CommandBarPageContext, workspaceID string) model.CommandBarPageContext {
	ctx.EntityType = normalizeCommandBarTargetType(ctx.EntityType)
	ctx.EntityID = strings.TrimSpace(ctx.EntityID)
	ctx.DisplayTitle = strings.TrimSpace(ctx.DisplayTitle)
	if ctx.EntityType == "" {
		ctx.EntityType = "workspace"
	}
	if ctx.EntityType == "workspace" && ctx.EntityID == "" {
		ctx.EntityID = strings.TrimSpace(workspaceID)
	}
	if ctx.DisplayTitle == "" {
		ctx.DisplayTitle = ctx.EntityType
	}
	return ctx
}

func commandBarNormalizePlannerTarget(ctx model.CommandBarPageContext) model.CommandBarPageContext {
	ctx.EntityType = normalizeCommandBarTargetType(ctx.EntityType)
	ctx.EntityID = strings.TrimSpace(ctx.EntityID)
	ctx.DisplayTitle = strings.TrimSpace(ctx.DisplayTitle)
	if ctx.DisplayTitle == "" {
		ctx.DisplayTitle = firstNonEmptyString(ctx.EntityType, "target")
	}
	return ctx
}

func normalizeCommandBarPlanSteps(steps []model.CommandBarPlanStep, fallbackTarget model.CommandBarPageContext) []model.CommandBarPlanStep {
	normalized := make([]model.CommandBarPlanStep, 0, len(steps))
	for _, step := range steps {
		target := step.Target
		if strings.TrimSpace(target.EntityType) == "" || strings.TrimSpace(target.EntityID) == "" {
			target = fallbackTarget
		}
		target.EntityType = normalizeCommandBarTargetType(target.EntityType)
		target.EntityID = strings.TrimSpace(target.EntityID)
		target.DisplayTitle = strings.TrimSpace(target.DisplayTitle)
		if target.DisplayTitle == "" {
			target.DisplayTitle = fallbackTarget.DisplayTitle
		}
		if target.Metadata == nil && target.EntityType == fallbackTarget.EntityType && target.EntityID == fallbackTarget.EntityID {
			target.Metadata = fallbackTarget.Metadata
		}
		step.AgentID = strings.TrimSpace(step.AgentID)
		step.AgentKey = normalizePresetKey(step.AgentKey)
		step.AgentName = strings.TrimSpace(step.AgentName)
		step.PlanKind = normalizeCommandBarPlanKind(step.PlanKind)
		step.Target = target
		step.Instructions = strings.TrimSpace(step.Instructions)
		normalized = append(normalized, step)
	}
	return normalized
}

func normalizeCommandBarPlanKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case model.CommandBarPlanKindOneShotCommand, "one_shot", "one-shot", "one-shot-command":
		return model.CommandBarPlanKindOneShotCommand
	case model.CommandBarPlanKindFanOut, "fanout", "fan-out":
		return model.CommandBarPlanKindFanOut
	case model.CommandBarPlanKindTaskPipeline, "task-pipeline", "task_pipeline", "task-pipeline-fan-out":
		return model.CommandBarPlanKindTaskPipeline
	case model.CommandBarPlanKindDAG, "command_dag", "command-dag", "orchestrated_plan":
		return model.CommandBarPlanKindDAG
	default:
		return ""
	}
}

func normalizeCommandBarTargetType(targetType string) string {
	targetType = strings.TrimSpace(strings.ToLower(targetType))
	targetType = strings.ReplaceAll(targetType, "-", "_")
	targetType = strings.ReplaceAll(targetType, " ", "_")
	switch targetType {
	case "story":
		return "task"
	case "deal":
		return "crm_deal"
	case "contact":
		return "crm_contact"
	case "doc":
		return "document"
	case "repo", "git_repository", "git_repo":
		return "repository"
	default:
		return targetType
	}
}

func normalizeCommandBarTargetTypes(targetTypes []string) []string {
	normalized := make([]string, 0, len(targetTypes))
	seen := map[string]bool{}
	for _, targetType := range targetTypes {
		targetType = normalizeCommandBarTargetType(targetType)
		if targetType == "" || seen[targetType] {
			continue
		}
		seen[targetType] = true
		normalized = append(normalized, targetType)
	}
	return normalized
}

func commandBarEffectiveTargetType(pageContext model.CommandBarPageContext) string {
	if pageContext.Metadata != nil {
		if scope, _ := pageContext.Metadata["context_scope"].(string); strings.TrimSpace(scope) == "all_tasks" {
			return "all_tasks"
		}
	}
	return normalizeCommandBarTargetType(pageContext.EntityType)
}

func validateCommandBarSupportedTarget(targetType string) error {
	switch normalizeCommandBarTargetType(targetType) {
	case "task", "epic", "workspace", "document", "crm_contact", "crm_deal", "repository":
		return nil
	default:
		return fmt.Errorf("command bar v1 does not support %s targets yet", targetType)
	}
}

func buildCommandBarTriggerContext(text string, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep, stepIndex int, planID string) (*model.AgentRunTriggerContext, error) {
	now := time.Now().UTC()
	raw, err := json.Marshal(commandBarTriggerContextPayload{
		PlanID:      strings.TrimSpace(planID),
		Prompt:      strings.TrimSpace(text),
		PageContext: pageContext,
		Steps:       steps,
		RunCount:    len(steps),
		StepIndex:   stepIndex,
	})
	if err != nil {
		return nil, err
	}
	return &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceCommandBar,
		TriggerType: model.AgentRunTriggerTypeCommandBar,
		FiredAt:     &now,
		Context:     raw,
	}, nil
}

func commandBarAdditionalContext(instructions string, pageContext model.CommandBarPageContext, stepIndex, stepCount int) string {
	instructions = strings.TrimSpace(instructions)
	parts := []string{
		"This run was started from the workspace command bar.",
		fmt.Sprintf("Command bar plan step: %d of %d.", stepIndex+1, stepCount),
	}
	if instructions != "" {
		parts = append(parts, "Step instruction:\n"+instructions)
	}
	if stepCount > 1 {
		parts = append(parts, "Other command-bar plan steps are scheduled separately by Helpin. Do not invoke those agents yourself.")
	}
	if pageContext.EntityType != "" || pageContext.EntityID != "" {
		parts = append(parts, fmt.Sprintf("Command bar page context: %s %s (%s).", pageContext.EntityType, pageContext.EntityID, pageContext.DisplayTitle))
		if scopeInstruction := commandBarScopeInstruction(pageContext); scopeInstruction != "" {
			parts = append(parts, scopeInstruction)
		}
	}
	return strings.Join(parts, "\n\n")
}

func commandBarScopeInstruction(pageContext model.CommandBarPageContext) string {
	if pageContext.Metadata == nil {
		return ""
	}
	switch commandBarPageContextMetadataString(pageContext, "context_scope") {
	case "block":
		return commandBarBlockContextInstruction(pageContext)
	case "all_tasks":
		return "PM task collection context:\n- Treat all workspace tasks as the active context.\n- Do not limit the answer to a selected task unless the user explicitly asks for one."
	default:
		return ""
	}
}

func commandBarPageContextMetadataString(pageContext model.CommandBarPageContext, key string) string {
	if pageContext.Metadata == nil {
		return ""
	}
	value, _ := pageContext.Metadata[key].(string)
	return strings.TrimSpace(value)
}

func commandBarBlockContextInstruction(pageContext model.CommandBarPageContext) string {
	if pageContext.Metadata == nil {
		return ""
	}
	blockID, _ := pageContext.Metadata["block_id"].(string)
	blockType, _ := pageContext.Metadata["block_type"].(string)
	excerpt, _ := pageContext.Metadata["block_excerpt"].(string)
	revisionValue := pageContext.Metadata["block_revision"]
	revision := ""
	switch value := revisionValue.(type) {
	case float64:
		revision = fmt.Sprintf("%.0f", value)
	case int:
		revision = fmt.Sprintf("%d", value)
	case string:
		revision = strings.TrimSpace(value)
	}
	lines := []string{
		"Focused Docs block context:",
		"- Treat the full document as reference context.",
		"- The focused block is the primary edit target unless the user explicitly asks for the whole document.",
		"- Use the supplied block_id and block_revision directly for block-scoped proposals; do not rediscover them through document search.",
	}
	if strings.TrimSpace(blockID) != "" {
		lines = append(lines, "- block_id: "+strings.TrimSpace(blockID))
	}
	if strings.TrimSpace(revision) != "" {
		lines = append(lines, "- block_revision: "+strings.TrimSpace(revision))
	}
	if strings.TrimSpace(blockType) != "" {
		lines = append(lines, "- block_type: "+strings.TrimSpace(blockType))
	}
	if strings.TrimSpace(excerpt) != "" {
		lines = append(lines, "- block_excerpt: "+strings.TrimSpace(excerpt))
	}
	return strings.Join(lines, "\n")
}

func findCommandBarCandidateByID(candidates []model.CommandBarAgent, id string) (model.CommandBarAgent, bool) {
	id = strings.TrimSpace(id)
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return model.CommandBarAgent{}, false
}

func findCommandBarCandidateByPreset(candidates []model.CommandBarAgent, preset string) (model.CommandBarAgent, bool) {
	preset = normalizePresetKey(preset)
	for _, candidate := range candidates {
		if normalizePresetKey(candidate.PresetKey) == preset {
			return candidate, true
		}
	}
	return model.CommandBarAgent{}, false
}

func findCommandBarCandidateByName(candidates []model.CommandBarAgent, name string) (model.CommandBarAgent, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(strings.TrimSpace(candidate.Name)), name) {
			return candidate, true
		}
	}
	return model.CommandBarAgent{}, false
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func defaultCommandBarSuggestions(targetType string) []string {
	switch normalizeCommandBarTargetType(targetType) {
	case "task":
		return []string{"Ask Code Builder to implement this task", "Ask Review Agent to review this task", "Ask Task Planner to refine this task"}
	case "epic":
		return []string{"Ask Epic Planner to break this epic into tasks", "Ask Review Agent to review this epic"}
	default:
		return []string{"Try a request that names an existing agent", "Open an epic or task and try again with page context"}
	}
}
