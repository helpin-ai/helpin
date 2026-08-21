package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const maxCommandBarPlanSteps = 50
const maxCommandBarDAGInitialFanOut = 10

type commandBarTriggerContextPayload struct {
	PlanID      string                      `json:"plan_id,omitempty"`
	Prompt      string                      `json:"prompt,omitempty"`
	PageContext model.CommandBarPageContext `json:"page_context"`
	Steps       []model.CommandBarPlanStep  `json:"steps"`
	RunCount    int                         `json:"run_count"`
	StepIndex   int                         `json:"step_index"`
}

// CommandBarService owns command-bar plan dispatch, plan lifecycle, and
// one-shot-run promotion. The conversational chat layer that used to live
// here was replaced by dock chats (DockChatService) backed by agent-runtime
// chat-mode runs.
type CommandBarService struct {
	agentService        *AgentService
	planRepo            *repository.CommandBarPlanRepository
	dismissalRepo       *repository.CommandBarPlanDismissalRepository
	commandService      *InternalCommandService
	docsDocumentService *DocsDocumentService
	crmDealService      *CRMDealService
	crmContactService   *CRMContactService
	crmCompanyService   *CRMCompanyService
	wsPublisher         *websocket.Publisher
}

// NewCommandBarService creates a CommandBarService.
func NewCommandBarService(agentService *AgentService, planRepo *repository.CommandBarPlanRepository, dismissalRepo *repository.CommandBarPlanDismissalRepository) *CommandBarService {
	return &CommandBarService{
		agentService:  agentService,
		planRepo:      planRepo,
		dismissalRepo: dismissalRepo,
	}
}

// SetWebsocketPublisher wires the websocket publisher used for plan events.
func (s *CommandBarService) SetWebsocketPublisher(publisher *websocket.Publisher) *CommandBarService {
	if s != nil {
		s.wsPublisher = publisher
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
	modelTier := firstNonEmptyString(strings.TrimSpace(draft.ModelTier), "small")
	approvalMode := firstNonEmptyString(strings.TrimSpace(draft.ApprovalMode), "mutating_tools")
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
		ModelTier:             &modelTier,
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
			"read_files",
			"list_directory",
			"repository_search",
			"list_symbols",
			"read_symbol",
			"trace_symbol":
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
		Status:         agent.Status,
		RuntimeKind:    agent.RuntimeKind,
		IsSystem:       agent.IsSystem,
		SupportedModes: slices.Clone(agent.SupportedModes),
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

func shortCommandBarID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[:8]
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

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
