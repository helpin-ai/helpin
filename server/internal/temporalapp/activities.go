package temporalapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/helpin-ai/helpin/server/internal/agentskills"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

const (
	productSpecsSpaceSlug   = "product-specs"
	productSpecsSpaceName   = "Product Specs"
	githubAPIRequestTimeout = 30 * time.Second
)

var codexLivePausePollEvery = time.Second
var errBranchSyncUnrelatedHistory = errors.New("working branch does not share history with base branch")

type planningRunInput struct {
	Stage               string   `json:"stage,omitempty"`
	AdditionalContext   string   `json:"additional_context,omitempty"`
	PlanDocumentID      string   `json:"plan_document_id,omitempty"`
	SpecDocumentID      string   `json:"spec_document_id,omitempty"`
	SpecVersionID       string   `json:"spec_version_id,omitempty"`
	PlanningMethodology string   `json:"planning_methodology,omitempty"`
	AllowedTools        []string `json:"allowed_tools,omitempty"`
	FlowOutputKind      string   `json:"flow_output_kind,omitempty"`
}

type planningRunSummary struct {
	Stage               string                        `json:"stage"`
	SpecDocumentID      string                        `json:"spec_document_id,omitempty"`
	SpecVersionID       string                        `json:"spec_version_id,omitempty"`
	PlanningMethodology string                        `json:"planning_methodology,omitempty"`
	PlanDocumentID      string                        `json:"plan_document_id,omitempty"`
	Summary             string                        `json:"summary,omitempty"`
	Risks               []string                      `json:"risks,omitempty"`
	Assumptions         []string                      `json:"assumptions,omitempty"`
	OpenQuestions       []string                      `json:"open_questions,omitempty"`
	Clarifications      []model.SpecClarificationItem `json:"clarifications,omitempty"`
	Proposal            *model.OrchestrationProposal  `json:"proposal,omitempty"`
}

type supportRunActivitySummary struct {
	DraftReply    *supportRunActivityDraft `json:"draft_reply,omitempty"`
	SentMessageID *string                  `json:"sent_message_id,omitempty"`
}

type supportRunActivityDraft struct {
	Content           string  `json:"content"`
	IsInternal        bool    `json:"is_internal"`
	SenderDisplayName *string `json:"sender_display_name,omitempty"`
	ApprovalRequired  bool    `json:"approval_required"`
}

type InternalCommandExecutor interface {
	Execute(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error)
}

type NotificationEmitter interface {
	Emit(ctx context.Context, event model.NotificationEventInput) error
}

// AgentRunActivities contains the Temporal activities that execute an agent run.
type AgentRunActivities struct {
	runRepo             *repository.AgentRunRepository
	runMessageRepo      *repository.AgentRunMessageRepository
	agentRepo           *repository.AgentRepository
	workspaceSkillRepo  *repository.WorkspaceSkillRepository
	skillPackageStore   agentskills.SkillPackageStore
	artifactRepo        *repository.AgentRunArtifactRepository
	interactionRepo     *repository.AgentRunInteractionRepository
	sessionSnapshotRepo *repository.CodingSessionStateSnapshotRepository
	taskRepo            *repository.PMTaskRepository
	taskLinkRepo        *repository.PMTaskLinkRepository
	epicRepo            *repository.PMEpicRepository
	conversationRepo    *repository.SupportConversationRepository
	commentRepo         *repository.PMCommentRepository
	checklistRepo       *repository.PMChecklistItemRepository
	messageRepo         *repository.SupportMessageRepository
	gitIntRepo          *repository.GitIntegrationRepository
	gitRepo             *repository.GitRepositoryRepository
	gitLinkRepo         *repository.TaskGitLinkRepository
	deliveryRepo        *repository.TaskDeliveryTargetRepository
	settingsRepo        *repository.SettingsRepository
	workspaceRepo       *repository.WorkspaceRepository
	docsSpaceRepo       *repository.DocsSpaceRepository
	docsDocRepo         *repository.DocsDocumentRepository
	docsContentRepo     *repository.DocsContentRepository
	docsVersionRepo     *repository.DocsVersionRepository
	docsLinkRepo        *repository.DocsLinkRepository
	docsSearchRepo      *repository.DocsSearchRepository
	crmDealRepo         *repository.CRMDealRepository
	crmContactRepo      *repository.CRMContactRepository
	crmSignalRepo       *repository.CRMSignalRepository
	crmActivityRepo     *repository.CRMActivityRepository
	commandExecutor     InternalCommandExecutor
	notificationEmitter NotificationEmitter
	wsPublisher         websocket.EventPublisher
	runtimes            *workerpkg.RuntimeRegistry
	githubApp           *githubapp.Client
	runEngine           *RunEngine
}

// NewAgentRunActivities creates the activity set used by shared Temporal workers.
func NewAgentRunActivities(
	runRepo *repository.AgentRunRepository,
	runMessageRepo *repository.AgentRunMessageRepository,
	agentRepo *repository.AgentRepository,
	workspaceSkillRepo *repository.WorkspaceSkillRepository,
	skillPackageStore agentskills.SkillPackageStore,
	artifactRepo *repository.AgentRunArtifactRepository,
	interactionRepo *repository.AgentRunInteractionRepository,
	sessionSnapshotRepo *repository.CodingSessionStateSnapshotRepository,
	taskRepo *repository.PMTaskRepository,
	taskLinkRepo *repository.PMTaskLinkRepository,
	epicRepo *repository.PMEpicRepository,
	conversationRepo *repository.SupportConversationRepository,
	commentRepo *repository.PMCommentRepository,
	checklistRepo *repository.PMChecklistItemRepository,
	messageRepo *repository.SupportMessageRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	gitRepo *repository.GitRepositoryRepository,
	gitLinkRepo *repository.TaskGitLinkRepository,
	deliveryRepo *repository.TaskDeliveryTargetRepository,
	settingsRepo *repository.SettingsRepository,
	workspaceRepo *repository.WorkspaceRepository,
	docsSpaceRepo *repository.DocsSpaceRepository,
	docsDocRepo *repository.DocsDocumentRepository,
	docsContentRepo *repository.DocsContentRepository,
	docsVersionRepo *repository.DocsVersionRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	docsSearchRepo *repository.DocsSearchRepository,
	crmDealRepo *repository.CRMDealRepository,
	crmContactRepo *repository.CRMContactRepository,
	crmSignalRepo *repository.CRMSignalRepository,
	crmActivityRepo *repository.CRMActivityRepository,
	commandExecutor InternalCommandExecutor,
	notificationEmitter NotificationEmitter,
	wsPublisher websocket.EventPublisher,
	runtimes *workerpkg.RuntimeRegistry,
	githubApp *githubapp.Client,
	runEngine *RunEngine,
) *AgentRunActivities {
	return &AgentRunActivities{
		runRepo:             runRepo,
		runMessageRepo:      runMessageRepo,
		agentRepo:           agentRepo,
		workspaceSkillRepo:  workspaceSkillRepo,
		skillPackageStore:   skillPackageStore,
		artifactRepo:        artifactRepo,
		interactionRepo:     interactionRepo,
		sessionSnapshotRepo: sessionSnapshotRepo,
		taskRepo:            taskRepo,
		taskLinkRepo:        taskLinkRepo,
		epicRepo:            epicRepo,
		conversationRepo:    conversationRepo,
		commentRepo:         commentRepo,
		checklistRepo:       checklistRepo,
		messageRepo:         messageRepo,
		gitIntRepo:          gitIntRepo,
		gitRepo:             gitRepo,
		gitLinkRepo:         gitLinkRepo,
		deliveryRepo:        deliveryRepo,
		settingsRepo:        settingsRepo,
		workspaceRepo:       workspaceRepo,
		docsSpaceRepo:       docsSpaceRepo,
		docsDocRepo:         docsDocRepo,
		docsContentRepo:     docsContentRepo,
		docsVersionRepo:     docsVersionRepo,
		docsLinkRepo:        docsLinkRepo,
		docsSearchRepo:      docsSearchRepo,
		crmDealRepo:         crmDealRepo,
		crmContactRepo:      crmContactRepo,
		crmSignalRepo:       crmSignalRepo,
		crmActivityRepo:     crmActivityRepo,
		commandExecutor:     commandExecutor,
		notificationEmitter: notificationEmitter,
		wsPublisher:         wsPublisher,
		runtimes:            runtimes,
		githubApp:           githubApp,
		runEngine:           runEngine,
	}
}

type resolvedRunState struct {
	run                        *model.AgentRun
	agent                      *model.Agent
	runtimeSkillRefs           model.AgentSkillRefs
	runtimeSkillDefinitions    []workerpkg.SkillDefinition
	skillPolicy                workerpkg.SkillPolicy
	nativeSelectivePathEnabled bool
	task                       *model.PMTask
	workspaceKey               string
	epic                       *model.PMEpic
	epicTasks                  []model.PMTask
	conversation               *model.SupportConversation
	resolved                   workerpkg.ResolvedProfile // merged class+agent overrides — use this for decisions
	deliveryTarget             *model.TaskDeliveryTarget
	repository                 *model.GitRepository
	integration                *model.GitIntegration
	teamDefault                *model.PMTeamRepoDefault
	accessToken                string
	branchSync                 branchSyncState
}

type branchSyncState struct {
	Status        string
	BaseBranch    string
	WorkingBranch string
	ConflictFiles []string
	BackupBranch  string
}

func executionRuntimeKind(state *resolvedRunState) string {
	if state == nil {
		return ""
	}
	if state.run != nil && strings.TrimSpace(state.run.RuntimeKind) != "" {
		return strings.TrimSpace(state.run.RuntimeKind)
	}
	if state.agent != nil {
		return strings.TrimSpace(state.agent.RuntimeKind)
	}
	return ""
}

func shouldPersistExecutionWorkspace(run *model.AgentRun, runtimeKind string) bool {
	if run == nil {
		return false
	}
	return strings.TrimSpace(runtimeKind) == "codex" && strings.TrimSpace(run.InvocationMode) == model.InvocationModeInteractive
}

func nativeSelectivePlannerPathRolloutEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_NATIVE_SELECTIVE_PLANNER_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func resolveNativeSelectivePlannerPathEnabled(run *model.AgentRun, agent *model.Agent) bool {
	if run == nil || agent == nil {
		return false
	}
	if !nativeSelectivePlannerPathRolloutEnabled() {
		return false
	}
	if !agent.IsSystem {
		return false
	}
	runtimeKind := firstNonEmptyString(strings.TrimSpace(run.RuntimeKind), strings.TrimSpace(agent.RuntimeKind))
	if runtimeKind != "native_sdk" {
		return false
	}
	switch strings.TrimSpace(agent.EffectivePresetKey()) {
	case model.AgentPresetEpicPlanner:
		return strings.TrimSpace(run.TargetType) == "epic"
	case model.AgentPresetTaskPlanner:
		return strings.TrimSpace(run.TargetType) == "task"
	default:
		return false
	}
}

func runtimeSkillRefKeys(refs model.AgentSkillRefs) []string {
	keys := make([]string, 0, len(refs))
	for _, ref := range refs {
		key := strings.TrimSpace(ref.Key)
		if key == "" && ref.SkillID != nil && strings.TrimSpace(*ref.SkillID) != "" {
			key = "workspace:" + strings.TrimSpace(*ref.SkillID)
		}
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func selectNativeActiveSkills(state *resolvedRunState, planningStage string) agentskills.NativeActiveSelection {
	if state == nil {
		return agentskills.NativeActiveSelection{}
	}
	if !state.nativeSelectivePathEnabled {
		return agentskills.NativeActiveSelection{
			Refs:         append(model.AgentSkillRefs(nil), state.runtimeSkillRefs...),
			Definitions:  append([]workerpkg.SkillDefinition(nil), state.runtimeSkillDefinitions...),
			Instructions: agentskills.CompileInstructions(state.runtimeSkillDefinitions),
		}
	}
	planningStage = nativeActiveSkillPlanningStage(state, planningStage)
	return agentskills.SelectNativeActiveSkills(state.runtimeSkillRefs, state.runtimeSkillDefinitions, agentskills.NativeActiveSelectionContext{
		PresetKey:     strings.TrimSpace(state.agent.EffectivePresetKey()),
		TargetType:    strings.TrimSpace(state.run.TargetType),
		PlanningStage: strings.TrimSpace(planningStage),
	})
}

func nativeActiveSkillPlanningStage(state *resolvedRunState, planningStage string) string {
	planningStage = strings.TrimSpace(planningStage)
	if planningStage != "" || state == nil || state.run == nil {
		return planningStage
	}

	switch strings.TrimSpace(state.run.TargetType) {
	case "epic":
		if state.epic == nil {
			return planningStage
		}
		if strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID)) != "" {
			return model.PlanningStagePlanTasks
		}
		switch strings.TrimSpace(state.epic.PlanningState) {
		case model.EpicPlanningStateReadyForTaskPlanning,
			model.EpicPlanningStateReadyForStoryPlanning,
			model.EpicPlanningStateAwaitingPlanApproval,
			model.EpicPlanningStateStoriesCreated,
			model.EpicPlanningStateExecutionStarted,
			model.EpicPlanningStateReadyForExecution:
			return model.PlanningStagePlanTasks
		default:
			return model.PlanningStageDraftSpec
		}
	case "task":
		if state.task != nil && strings.TrimSpace(state.agent.EffectivePresetKey()) == model.AgentPresetTaskPlanner {
			return model.PlanningStageTaskPlanDoc
		}
	}

	return planningStage
}

func effectiveExecutionSkillPolicy(state *resolvedRunState, selection agentskills.NativeActiveSelection) workerpkg.SkillPolicy {
	if state == nil || !state.nativeSelectivePathEnabled {
		if state == nil {
			return workerpkg.SkillPolicy{}
		}
		return state.skillPolicy
	}
	if len(selection.Definitions) == 0 && len(state.runtimeSkillDefinitions) > 0 {
		return state.skillPolicy
	}
	return agentskills.AggregatePolicy(selection.Definitions)
}

type nativeRepairInstruction struct {
	Source       string
	Class        string
	Instructions string
}

type nativeTurnDebugArtifact struct {
	RuntimeKind                string   `json:"runtime_kind"`
	NativeSelectivePathEnabled bool     `json:"native_selective_path_enabled"`
	PlanningStage              string   `json:"planning_stage,omitempty"`
	ContinuationMode           string   `json:"continuation_mode"`
	RuntimeSkillRefs           []string `json:"runtime_skill_refs,omitempty"`
	ActiveSkillRefs            []string `json:"active_skill_refs,omitempty"`
	RequiredInteractions       []string `json:"required_interactions,omitempty"`
	RepairGuidancePresent      bool     `json:"repair_guidance_present"`
	RepairGuidanceSource       string   `json:"repair_guidance_source,omitempty"`
	RepairGuidanceClass        string   `json:"repair_guidance_class,omitempty"`
}

func latestUnresolvedNativeToolFailure(messages []model.AgentRunMessage) *workerpkg.ExecutionBlock {
	if len(messages) == 0 {
		return nil
	}

	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if strings.TrimSpace(message.Role) == "assistant" && shouldIncludeRunMessageInExecutionHistory(message) {
			break
		}
		if strings.TrimSpace(message.Role) != "tool" || strings.TrimSpace(message.MessageType) != "tool_result" {
			continue
		}
		for _, block := range parsePersistedExecutionBlocks(message.ContentBlocks) {
			if block.Type != workerpkg.ExecutionBlockTypeToolResult || !block.IsError || strings.TrimSpace(block.ToolName) == "" {
				continue
			}
			copied := block
			if strings.TrimSpace(copied.Output) == "" {
				copied.Output = strings.TrimSpace(message.Content)
			}
			return &copied
		}
	}
	return nil
}

func latestExecutionToolFailure(messages []workerpkg.ExecutionMessage) *workerpkg.ExecutionBlock {
	toolMessages := finalRoundToolMessages(messages)
	for i := len(toolMessages) - 1; i >= 0; i-- {
		for j := len(toolMessages[i].Blocks) - 1; j >= 0; j-- {
			block := toolMessages[i].Blocks[j]
			if block.Type != workerpkg.ExecutionBlockTypeToolResult || !block.IsError {
				continue
			}
			copied := block
			return &copied
		}
	}
	return nil
}

func parsePersistedExecutionBlocks(raw json.RawMessage) []workerpkg.ExecutionBlock {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var blocks []workerpkg.ExecutionBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}
	return workerpkg.NormalizeExecutionBlocks(blocks)
}

func classifyNativeToolFailureRepair(state *resolvedRunState, failure *workerpkg.ExecutionBlock) nativeRepairInstruction {
	if state == nil || !state.nativeSelectivePathEnabled || failure == nil {
		return nativeRepairInstruction{}
	}
	toolName := strings.TrimSpace(failure.ToolName)
	output := strings.TrimSpace(failure.Output)
	if toolName == "" || output == "" {
		return nativeRepairInstruction{}
	}

	switch toolName {
	case workerpkg.ToolPublishTaskPlan:
		return classifyPublishTaskPlanRepair(output)
	case workerpkg.ToolPublishTaskPlanDoc, workerpkg.ToolPublishPRDDraft:
		return classifyMarkdownPreviewRepair(toolName, output)
	default:
		return nativeRepairInstruction{}
	}
}

func classifyPublishTaskPlanRepair(output string) nativeRepairInstruction {
	switch {
	case strings.Contains(output, "publish_task_plan content must be a JSON object with summary and proposed_tasks"):
		return nativeRepairInstruction{
			Class: "publish_task_plan_object_shape",
			Instructions: strings.Join([]string{
				"Your previous publish_task_plan call failed validation because content was not a structured JSON object.",
				"Retry publish_task_plan with one complete JSON object in content.",
				`The content object must include a non-empty "summary" string and a "proposed_tasks" array of task objects.`,
				"Do not send markdown, prose wrappers, or stringified JSON blobs inside content.",
			}, "\n"),
		}
	case strings.Contains(output, "publish_task_plan requires content.proposed_tasks to be an array of task objects"):
		return nativeRepairInstruction{
			Class: "publish_task_plan_task_array_shape",
			Instructions: strings.Join([]string{
				"Your previous publish_task_plan call failed validation because proposed_tasks was not an array of task objects.",
				`Retry publish_task_plan with content.proposed_tasks as an array of full task objects, not strings, refs, placeholders, or partial fragments.`,
				"Each task entry should include the normal structured task fields expected by the task-plan contract.",
			}, "\n"),
		}
	case strings.Contains(output, `publish_task_plan is missing content; include the task plan JSON object in "content"`):
		return nativeRepairInstruction{
			Class: "publish_task_plan_missing_content",
			Instructions: strings.Join([]string{
				"Your previous publish_task_plan call failed validation because the content field was missing.",
				`Retry publish_task_plan with the full task-plan JSON object under "content".`,
				`Do not send title-only payloads; include content.summary and content.proposed_tasks in the same tool call.`,
			}, "\n"),
		}
	case strings.Contains(output, "publish_task_plan input must be a JSON object with structured fields; do not send a raw string wrapper"):
		return nativeRepairInstruction{
			Class: "publish_task_plan_raw_wrapper",
			Instructions: strings.Join([]string{
				"Your previous publish_task_plan call failed validation because the tool input was wrapped as raw text instead of structured fields.",
				"Retry publish_task_plan with a normal JSON object input, not a raw wrapper string.",
				`Put the task plan under the structured "content" object with summary and proposed_tasks.`,
			}, "\n"),
		}
	default:
		return nativeRepairInstruction{}
	}
}

func classifyMarkdownPreviewRepair(toolName, output string) nativeRepairInstruction {
	toolName = strings.TrimSpace(toolName)
	contentLabel := "full markdown draft"
	switch toolName {
	case workerpkg.ToolPublishPRDDraft:
		contentLabel = "full PRD markdown draft"
	case workerpkg.ToolPublishTaskPlanDoc:
		contentLabel = "full task planning markdown draft"
	}

	switch {
	case strings.Contains(output, fmt.Sprintf(`%s is missing content; include markdown in "content"`, toolName)):
		return nativeRepairInstruction{
			Class: toolName + "_missing_content",
			Instructions: strings.Join([]string{
				fmt.Sprintf("Your previous %s call failed validation because the content field was missing.", toolName),
				fmt.Sprintf(`Retry %s with the %s under "content".`, toolName, contentLabel),
				"Do not send title-only payloads when publishing a markdown preview for review.",
			}, "\n"),
		}
	case strings.Contains(output, fmt.Sprintf(`%s content must be a markdown string in "content"`, toolName)):
		return nativeRepairInstruction{
			Class: toolName + "_markdown_type",
			Instructions: strings.Join([]string{
				fmt.Sprintf("Your previous %s call failed validation because content was not a markdown string.", toolName),
				fmt.Sprintf(`Retry %s with the %s as a plain markdown string in "content".`, toolName, contentLabel),
				"Do not send JSON objects, arrays, or other non-string content to markdown preview tools.",
			}, "\n"),
		}
	default:
		return nativeRepairInstruction{}
	}
}

func latestUnresolvedPolicyRetryMessage(messages []model.AgentRunMessage) *model.AgentRunMessage {
	sawLaterAssistant := false
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if strings.TrimSpace(message.Role) == "assistant" && shouldIncludeRunMessageInExecutionHistory(message) {
			sawLaterAssistant = true
		}
		if strings.TrimSpace(message.MessageType) != "policy_retry" {
			continue
		}
		if sawLaterAssistant {
			return nil
		}
		copied := message
		return &copied
	}
	return nil
}

func classifyNativeRepairInstruction(state *resolvedRunState, message *model.AgentRunMessage) nativeRepairInstruction {
	if state == nil || !state.nativeSelectivePathEnabled || message == nil {
		return nativeRepairInstruction{}
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return nativeRepairInstruction{}
	}

	switch {
	case strings.Contains(content, "multiple same-turn previews") && strings.Contains(content, "preview_panel_key"):
		return nativeRepairInstruction{
			Source: "policy_retry",
			Class:  "approval_preview_panel_key_required",
			Instructions: strings.Join([]string{
				"Continue from your last assistant turn instead of restarting the run.",
				"If this turn requests approval or a review checkpoint after publishing multiple previews, include preview_panel_key so the handoff binds to the intended preview.",
				"Publish the target preview in the same turn before the approval handoff, then treat request_approval or request_review_checkpoint as the final action in that turn.",
			}, "\n"),
		}
	case strings.Contains(content, "required same-turn ") && strings.Contains(content, " preview"):
		requiredPreviewKey := extractRequiredSameTurnPreviewKey(content)
		lines := []string{
			"Continue from your last assistant turn instead of restarting the run.",
			"If this turn requests approval or a review checkpoint, first publish the required preview in the same turn before the handoff.",
		}
		if requiredPreviewKey != "" {
			lines = append(lines,
				fmt.Sprintf("Use preview_panel_key=%q so the approval request binds to the %s preview.", requiredPreviewKey, requiredPreviewKey),
				fmt.Sprintf("If that %s preview is missing or stale, republish it in the same turn before requesting approval.", requiredPreviewKey),
			)
		} else {
			lines = append(lines, "Include preview_panel_key when needed so the approval request binds to the intended preview.")
		}
		lines = append(lines, "Treat request_approval or request_review_checkpoint as the final action in that turn.")
		return nativeRepairInstruction{
			Source:       "policy_retry",
			Class:        "approval_specific_preview_required",
			Instructions: strings.Join(lines, "\n"),
		}
	case strings.Contains(content, "same-turn preview") || strings.Contains(content, "preview_panel_key"):
		return nativeRepairInstruction{
			Source: "policy_retry",
			Class:  "approval_preview_binding",
			Instructions: strings.Join([]string{
				"Continue from your last assistant turn instead of restarting the run.",
				"If this turn requests approval or a review checkpoint, first publish the preview in the same turn before the approval handoff.",
				"When multiple same-turn previews exist, include preview_panel_key so the approval request binds to the correct preview.",
				"Treat request_approval or request_review_checkpoint as the final action in that turn.",
			}, "\n"),
		}
	case strings.Contains(content, "review_checkpoint handoff"):
		return nativeRepairInstruction{
			Source: "policy_retry",
			Class:  "review_checkpoint_handoff",
			Instructions: strings.Join([]string{
				"Continue from your last assistant turn instead of restarting the review.",
				"Before the run stops, emit a review_checkpoint handoff using the runtime-appropriate mechanism.",
				"Only emit request_user_input instead if the human explicitly closed the review or asked a blocking follow-up question.",
				"Do not end the turn with prose only.",
			}, "\n"),
		}
	default:
		requiredKinds := sortedCompletionInteractionKinds(completionRequiredInteractionKinds(state.skillPolicy))
		requiredKindsText := "the required interaction handoff"
		if len(requiredKinds) > 0 {
			requiredKindsText = fmt.Sprintf("one of the required interaction handoffs [%s]", strings.Join(requiredKinds, ", "))
		}
		return nativeRepairInstruction{
			Source: "policy_retry",
			Class:  "required_interaction_handoff",
			Instructions: strings.Join([]string{
				"Continue from your last assistant turn instead of restarting the run.",
				fmt.Sprintf("Before the run stops, emit %s declared by the active skill policy.", requiredKindsText),
				"Do not end the turn with prose only.",
			}, "\n"),
		}
	}
}

func extractRequiredSameTurnPreviewKey(content string) string {
	return extractPreviewKeyAfterMarker(content, "required same-turn ", " preview")
}

func latestNativeRepairInstruction(state *resolvedRunState, messages []model.AgentRunMessage) nativeRepairInstruction {
	if state == nil || !state.nativeSelectivePathEnabled {
		return nativeRepairInstruction{}
	}
	if failure := latestUnresolvedNativeToolFailure(messages); failure != nil {
		if instruction := classifyNativeToolFailureRepair(state, failure); strings.TrimSpace(instruction.Instructions) != "" {
			if strings.TrimSpace(instruction.Source) == "" {
				instruction.Source = "tool_result_history"
			}
			return instruction
		}
	}
	message := latestUnresolvedPolicyRetryMessage(messages)
	if message == nil {
		return nativeRepairInstruction{}
	}
	instruction := classifyNativeRepairInstruction(state, message)
	if strings.TrimSpace(instruction.Instructions) != "" {
		if strings.TrimSpace(instruction.Source) == "" {
			instruction.Source = "policy_retry"
		}
		return instruction
	}
	return nativeRepairInstruction{
		Source:       "policy_retry",
		Class:        "raw_policy_retry",
		Instructions: strings.TrimSpace(message.Content),
	}
}

func latestNativeRepairInstructionFromArtifacts(messages []model.AgentRunMessage, artifacts []model.AgentRunArtifact) nativeRepairInstruction {
	if len(messages) == 0 || len(artifacts) == 0 {
		return nativeRepairInstruction{}
	}
	latestAssistantSeq := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(messages[i].Role) != "assistant" {
			continue
		}
		latestAssistantSeq = messages[i].SequenceNo
		break
	}
	if latestAssistantSeq <= 0 {
		return nativeRepairInstruction{}
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeNativeRepairState || artifact.InlineContent == nil {
			continue
		}
		if artifactAssistantMessageSequenceNo(artifact) != latestAssistantSeq {
			continue
		}
		var payload model.NativeRepairState
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			continue
		}
		hint := strings.TrimSpace(payload.RepairHint)
		if hint == "" {
			continue
		}
		return nativeRepairInstruction{
			Source:       "native_repair_state:" + strings.TrimSpace(payload.Source),
			Class:        strings.TrimSpace(payload.RepairClass),
			Instructions: hint,
		}
	}
	return nativeRepairInstruction{}
}

func resolveLatestNativeRepairInstruction(state *resolvedRunState, messages []model.AgentRunMessage, artifacts []model.AgentRunArtifact) nativeRepairInstruction {
	if instruction := latestNativeRepairInstructionFromArtifacts(messages, artifacts); strings.TrimSpace(instruction.Instructions) != "" {
		return instruction
	}
	return latestNativeRepairInstruction(state, messages)
}

func normalizedCompletionRetryInstruction(state *resolvedRunState, cause error) nativeRepairInstruction {
	if cause == nil || strings.TrimSpace(cause.Error()) == "" {
		return nativeRepairInstruction{}
	}
	causeText := strings.TrimSpace(cause.Error())
	if state != nil && state.agent != nil && strings.TrimSpace(state.agent.EffectivePresetKey()) == model.AgentPresetReviewAgent {
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "review_checkpoint_handoff",
			Instructions: "System correction: the previous review turn ended without the required interaction. Continue from your last assistant message instead of restarting. Do not end with prose only. In this next turn, emit a review_checkpoint handoff using the runtime-appropriate mechanism, or emit request_user_input only if the human explicitly closed the review or asked a blocking follow-up.",
		}
	}
	switch {
	case strings.Contains(causeText, "requires preview_panel_key when multiple same-turn previews exist"):
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "approval_preview_panel_key_required",
			Instructions: approvalPreviewRetryInstruction(causeText),
		}
	case strings.Contains(causeText, "requires a same-turn ") && strings.Contains(causeText, " preview before requesting approval"):
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "approval_specific_preview_required",
			Instructions: approvalPreviewRetryInstruction(causeText),
		}
	case strings.Contains(causeText, "same-turn") || strings.Contains(causeText, "preview_panel_key"):
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "approval_preview_binding",
			Instructions: "System correction: the previous turn requested approval without binding it to a same-turn preview. Continue from your last assistant message instead of restarting. Do not end with prose only. If you emit request_approval or request_review_checkpoint, first publish the preview in the same turn. When multiple previews exist in that turn, include preview_panel_key so it binds to the correct preview.",
		}
	default:
		policy := workerpkg.SkillPolicy{}
		if state != nil {
			policy = state.skillPolicy
		}
		requiredKinds := sortedCompletionInteractionKinds(completionRequiredInteractionKinds(policy))
		instruction := "System correction: the previous turn ended without creating the required interaction. Continue from your last assistant message instead of restarting. Do not end with prose only. Before this run stops, emit one of the required interaction handoffs declared by the active skill policy."
		if len(requiredKinds) > 0 {
			instruction = instruction + " Required interaction kinds for this turn: " + strings.Join(requiredKinds, ", ") + "."
		}
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "required_interaction_handoff",
			Instructions: instruction,
		}
	}
}

func replayMessagesForExecution(state *resolvedRunState, messages []model.AgentRunMessage) []model.AgentRunMessage {
	if state == nil || !state.nativeSelectivePathEnabled {
		return messages
	}
	filtered := make([]model.AgentRunMessage, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.MessageType) == "policy_retry" {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}

func splitNativePhaseGuidance(runtimeKind string, state *resolvedRunState, initialInstructions string) (string, string) {
	initialInstructions = strings.TrimSpace(initialInstructions)
	if initialInstructions == "" {
		return "", ""
	}
	if state != nil && state.nativeSelectivePathEnabled && strings.TrimSpace(runtimeKind) == "native_sdk" {
		return "", initialInstructions
	}
	return initialInstructions, ""
}

func providerContinuationMode(continuation *workerpkg.ProviderContinuation) string {
	if continuation == nil {
		return "fresh"
	}
	if strings.TrimSpace(continuation.ResponseID) != "" {
		return "response_id"
	}
	if strings.TrimSpace(continuation.PreviousResponseID) != "" {
		return "previous_response_id"
	}
	return "fresh"
}

func recordActivityHeartbeatSafe(ctx context.Context, details ...interface{}) {
	defer func() {
		if recover() != nil {
			// Some unit tests call activities directly without a Temporal activity context.
		}
	}()
	activity.RecordHeartbeat(ctx, details...)
}

// PrepareRunActivity resolves repo state, snapshots delivery metadata, and creates the working branch if needed.
func (a *AgentRunActivities) PrepareRunActivity(ctx context.Context, runID string) error {
	state, err := a.loadRunState(ctx, runID)
	if err != nil {
		return err
	}
	if err := ensureRunNotTerminal(state.run); err != nil {
		return err
	}

	now := time.Now()
	state.run.Status = model.AgentRunStatusRunning
	state.run.PauseReason = model.AgentRunPauseReasonNone
	if state.run.StartedAt == nil {
		state.run.StartedAt = &now
	}
	state.run.ExecutionStage = strPtr("preparing")
	state.run.LastHeartbeatAt = &now

	if state.task != nil {
		if err := a.prepareTaskDelivery(ctx, state); err != nil {
			_ = a.failRun(ctx, state, err.Error())
			return err
		}
	}

	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)

	recordActivityHeartbeatSafe(ctx, "prepared")
	return nil
}

// ExecuteRunActivity executes the agent loop on a shared runner workspace.
func (a *AgentRunActivities) ExecuteRunActivity(ctx context.Context, runID string) (ExecuteRunResult, error) {
	state, err := a.loadRunState(ctx, runID)
	if err != nil {
		return ExecuteRunResult{}, err
	}
	if err := ensureRunNotTerminal(state.run); err != nil {
		return ExecuteRunResult{}, err
	}
	planningInput, err := a.resolvePlanningRunInput(ctx, state)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if state.run.TargetType == "support_conversation" && state.run.ApprovalState == "approved" {
		if err := a.finalizeSupportConversationRun(ctx, state); err != nil {
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}

		completedAt := time.Now()
		state.run.Status = model.AgentRunStatusCompleted
		state.run.PauseReason = model.AgentRunPauseReasonNone
		state.run.CompletedAt = &completedAt
		state.run.ExecutionStage = strPtr("completed")
		state.run.LastHeartbeatAt = &completedAt
		if err := a.runRepo.Update(ctx, state.run); err != nil {
			return ExecuteRunResult{}, err
		}
		a.runRepo.Notify(ctx, state.run)
		if err := a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed); err != nil {
			return ExecuteRunResult{}, err
		}
		return ExecuteRunResult{}, nil
	}
	approvedPreviewAction, err := a.applyApprovedInteractivePreview(ctx, state, &planningInput)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	switch approvedPreviewAction {
	case "create_tasks":
		return ExecuteRunResult{}, nil
	case "persist_task_doc":
		return ExecuteRunResult{}, nil
	case "persist_prd":
		now := time.Now()
		state.run.Status = model.AgentRunStatusRunning
		state.run.PauseReason = model.AgentRunPauseReasonNone
		state.run.ExecutionStage = strPtr("continuing")
		state.run.LastHeartbeatAt = &now
		state.run.CompletedAt = nil
		if err := a.runRepo.Update(ctx, state.run); err != nil {
			return ExecuteRunResult{}, err
		}
		a.runRepo.Notify(ctx, state.run)
		return ExecuteRunResult{ContinueExecution: true}, nil
	}

	if err := a.preparePlanningRepository(ctx, state, planningInput); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	runtimeKind := executionRuntimeKind(state)
	initialInstructions, err := a.buildInitialInstructions(ctx, state, planningInput)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	legacyInitialInstructions, phaseGuidance := splitNativePhaseGuidance(runtimeKind, state, initialInstructions)

	now := time.Now()
	state.run.Status = "running"
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.StartedAt = &now
	state.run.ExecutionStage = strPtr("starting")
	state.run.LastHeartbeatAt = &now
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}
	if err := a.ensureRunBootstrapStatusMessage(ctx, state.run, "Preparing workspace and loading run context."); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	slog.InfoContext(ctx, "agent run ensuring initial conversation",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"target_type", state.run.TargetType,
		"runtime_kind", state.run.RuntimeKind,
	)
	history, artifactContext, providerContinuation, repairInstruction, err := a.ensureRunConversation(ctx, state, legacyInitialInstructions, planningInput)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	slog.InfoContext(ctx, "agent run preparing workspace",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"repo", repoFullName(state),
	)
	persistWorkspace := shouldPersistExecutionWorkspace(state.run, runtimeKind)
	var (
		workDir       string
		reusedWorkDir bool
	)
	if persistWorkspace {
		workDir, reusedWorkDir, err = workerpkg.PrepareWorkspaceForRun(ctx, state.integration, repoFullName(state), state.accessToken, state.run.ID)
	} else {
		workDir, err = workerpkg.PrepareWorkspace(ctx, state.integration, repoFullName(state), state.accessToken)
	}
	if err != nil {
		_ = a.failRun(ctx, state, fmt.Sprintf("prepare workspace: %v", err))
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if !persistWorkspace {
		defer os.RemoveAll(workDir)
	}
	slog.InfoContext(ctx, "agent run workspace ready",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"repo", repoFullName(state),
		"reused", reusedWorkDir,
	)

	if state.repository != nil {
		if !reusedWorkDir {
			slog.InfoContext(ctx, "agent run checking out run ref",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"repo", state.repository.FullName,
			)
			if err := a.checkoutRunRef(ctx, workDir, state); err != nil {
				if persistWorkspace {
					_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
				}
				_ = a.failRun(ctx, state, err.Error())
				return ExecuteRunResult{}, nonRetryableRunError(err)
			}
			slog.InfoContext(ctx, "agent run checked out run ref",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"repo", state.repository.FullName,
			)
			if err := a.syncBaseIntoWorkingBranch(ctx, workDir, state); err != nil {
				if persistWorkspace {
					_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
				}
				_ = a.failRun(ctx, state, err.Error())
				return ExecuteRunResult{}, nonRetryableRunError(err)
			}
		} else {
			slog.InfoContext(ctx, "agent run reusing existing workspace checkout",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"repo", state.repository.FullName,
				"work_dir", workDir,
			)
		}
	}

	config := workerpkg.ParseWorkflowConfigForAgent(workDir, state.agent)
	if config == nil {
		config = workerpkg.DefaultWorkflowConfigForAgent(state.agent)
	}

	allowedTools := effectiveToolSet(state.resolved, planningInput.AllowedTools)
	activeSkillSelection := selectNativeActiveSkills(state, planningInput.Stage)
	state.skillPolicy = effectiveExecutionSkillPolicy(state, activeSkillSelection)

	bridge := a.serviceBridge()
	execCtx := &workerpkg.ExecutionContext{
		Context:                    ctx,
		WorkDir:                    workDir,
		WorkspaceID:                state.run.WorkspaceID,
		AgentID:                    state.run.AgentID,
		RunID:                      state.run.ID,
		TargetType:                 state.run.TargetType,
		TargetID:                   state.run.TargetID,
		Agent:                      state.agent,
		Task:                       state.task,
		Epic:                       state.epic,
		EpicTasks:                  state.epicTasks,
		Conversation:               state.conversation,
		GitIntegration:             state.integration,
		GitAccessToken:             state.accessToken,
		Repo:                       repoFullName(state),
		BaseBranch:                 derefString(state.run.BaseBranch),
		WorkingBranch:              derefString(state.run.WorkingBranch),
		BranchSyncStatus:           strings.TrimSpace(state.branchSync.Status),
		BranchSyncConflictFiles:    slices.Clone(state.branchSync.ConflictFiles),
		InitialInstructions:        legacyInitialInstructions,
		PhaseGuidance:              phaseGuidance,
		RepairGuidance:             repairInstruction.Instructions,
		RepairGuidanceSource:       repairInstruction.Source,
		RepairGuidanceClass:        repairInstruction.Class,
		PlanningStage:              planningInput.Stage,
		PlanningMethodology:        planningInput.PlanningMethodology,
		PlanningSpecDocumentID:     planningInput.SpecDocumentID,
		PlanningSpecVersionID:      planningInput.SpecVersionID,
		RunFacts:                   buildDurableRunFacts(state, planningInput),
		Config:                     config,
		ResolvedProfile:            state.resolved,
		RuntimeSkillRefs:           state.runtimeSkillRefs,
		ActiveRuntimeSkillRefs:     activeSkillSelection.Refs,
		ActiveSkillInstructions:    activeSkillSelection.Instructions,
		SkillPolicy:                state.skillPolicy,
		NativeSelectivePathEnabled: state.nativeSelectivePathEnabled,
		AllowedTools:               allowedTools,
		Services:                   bridge,
		ArtifactContext:            artifactContext,
		ProviderContinuation:       providerContinuation,
		ConversationHistory:        history,
		Heartbeat: func(stage string) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			now := time.Now()
			recordActivityHeartbeatSafe(ctx, stage)
			state.run.ExecutionStage = &stage
			state.run.LastHeartbeatAt = &now
			if err := a.runRepo.UpdateStage(ctx, state.run.WorkspaceID, state.run.ID, stage, &now); err != nil {
				return err
			}
			a.runRepo.Notify(ctx, state.run)
			return nil
		},
		OnExecutionEvent: func(event workerpkg.ExecutionEvent) {
			a.publishRunStreamEvent(state.run, event)
		},
		OnGitPush: func(branch, sha string) error {
			return a.recordPushAndEnsureDeliveryPR(ctx, state, branch, sha)
		},
		OnPROpen: func(metadata workerpkg.PRMetadata, title string) error {
			return a.recordPR(ctx, state, metadata, title)
		},
	}
	if state.task != nil {
		execCtx.TaskID = state.task.ID
	}
	if state.conversation != nil {
		execCtx.ConversationID = state.conversation.ID
	}
	heartbeatStage := "codex_running"
	var heartbeatStageMu sync.RWMutex
	setHeartbeatStage := func(stage string) {
		heartbeatStageMu.Lock()
		defer heartbeatStageMu.Unlock()
		if strings.TrimSpace(stage) == "" {
			heartbeatStage = "codex_running"
			return
		}
		heartbeatStage = strings.TrimSpace(stage)
	}
	execCtx.HeartbeatStageProvider = func() string {
		heartbeatStageMu.RLock()
		defer heartbeatStageMu.RUnlock()
		return heartbeatStage
	}
	if runtimeKind == "codex" && state.run.InvocationMode == model.InvocationModeInteractive {
		execCtx.HandleInteractivePause = func(result *workerpkg.ExecutionResult) (*workerpkg.LiveExecutionResumeSignal, error) {
			return a.handleLiveCodexInteractivePause(ctx, state, execCtx, result, setHeartbeatStage)
		}
	}

	adapter, err := a.runtimes.Get(runtimeKind)
	if err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	slog.InfoContext(ctx, "agent run runtime execution starting",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"runtime_kind", runtimeKind,
		"agent_id", state.run.AgentID,
		"work_dir", workDir,
		"preset_key", strings.TrimSpace(state.agent.EffectivePresetKey()),
		"target_type", strings.TrimSpace(state.run.TargetType),
		"native_selective_path_enabled", state.nativeSelectivePathEnabled,
		"continuation_mode", providerContinuationMode(providerContinuation),
		"runtime_skill_refs", runtimeSkillRefKeys(state.runtimeSkillRefs),
		"active_skill_refs", runtimeSkillRefKeys(activeSkillSelection.Refs),
		"active_policy_required_interactions", completionRequiredInteractionKinds(state.skillPolicy),
	)
	var repoSkillMask *workerpkg.RepoSkillMask
	if runtimeKind == "codex" || runtimeKind == "opencode" {
		repoSkillMask, err = workerpkg.MaskRepoSkillRoots(workDir, state.run.ID)
		if err != nil {
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
		if repoSkillMask != nil {
			defer func() {
				if restoreErr := repoSkillMask.Restore(); restoreErr != nil && !errors.Is(restoreErr, os.ErrNotExist) {
					slog.WarnContext(ctx, "failed to restore masked repo skill roots",
						"error", restoreErr,
						"run_id", state.run.ID,
						"runtime_kind", runtimeKind)
				}
			}()
		}
	}
	if runtimeKind == "codex" || runtimeKind == "opencode" {
		if err := a.stageRuntimeSkills(ctx, state, execCtx, runtimeKind); err != nil {
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
	}

	err = adapter.Execute(execCtx, state.run)
	if err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		if ctx.Err() != nil {
			bgCtx := context.Background()
			_ = a.markAgentIdle(bgCtx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
			return ExecuteRunResult{}, nil
		}
		if err == workerpkg.ErrRunCancelled {
			bgCtx := context.Background()
			explicitlyCancelled, lookupErr := a.isRunExplicitlyCancelled(bgCtx, state.run.ID)
			if lookupErr != nil {
				_ = a.failRun(bgCtx, state, lookupErr.Error())
				return ExecuteRunResult{}, nonRetryableRunError(lookupErr)
			}
			if explicitlyCancelled {
				_ = a.markAgentIdle(bgCtx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
				return ExecuteRunResult{}, nil
			}
			unexpectedErr := fmt.Errorf("runtime reported cancellation without a cancelled run state")
			slog.ErrorContext(ctx, "agent run runtime cancelled unexpectedly",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"runtime_kind", runtimeKind,
			)
			_ = a.failRun(bgCtx, state, unexpectedErr.Error())
			return ExecuteRunResult{}, nonRetryableRunError(unexpectedErr)
		}
		bgCtx := context.Background()
		_ = a.failRun(bgCtx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	slog.InfoContext(ctx, "agent run runtime execution completed",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"runtime_kind", runtimeKind,
	)
	if runtimeKind == "codex" {
		if err := a.pushCodexLocalCommit(ctx, workDir, state, execCtx); err != nil {
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
	}
	assistantMessage, err := a.persistAssistantRunMessage(ctx, state, execCtx)
	if err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if err := a.captureTranscriptPlanningArtifacts(ctx, state, execCtx, assistantMessage, planningInput); err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	approvalRequest := latestExecutionApprovalRequest(execCtx)
	reviewRequest := latestExecutionReviewCheckpointRequest(execCtx)
	humanInputRequest := latestExecutionHumanInputRequest(execCtx)
	authRequest := latestExecutionCodexAuthState(execCtx)
	if approvalRequest == nil && reviewRequest == nil && humanInputRequest == nil && authRequest == nil {
		synthesizedReview, synthesizedInput, err := a.synthesizeCompletionInteractionFallback(ctx, state, assistantMessage)
		if err != nil {
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
		if synthesizedReview != nil {
			reviewRequest = synthesizedReview
		}
		if synthesizedInput != nil {
			humanInputRequest = synthesizedInput
		}
	}
	if err := a.finalizePlanningRun(ctx, state, planningInput); err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if err := a.finalizeSupportConversationRun(ctx, state); err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	if state.task != nil && execCtx.WorkingBranch != "" {
		state.run.WorkingBranch = &execCtx.WorkingBranch
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}

	waitForApproval, waitForInput, waitForAuth := resolveExecutionWaitState(state.run, humanInputRequest, approvalRequest, reviewRequest, authRequest)
	continueExecution := false
	if state.run.InvocationMode == model.InvocationModeInteractive && humanInputRequest != nil {
	}
	if !waitForApproval && !waitForInput && !waitForAuth && !continueExecution {
		if err := a.enforceCompletionInteractionPolicy(ctx, state, assistantMessage); err != nil {
			retried, retryErr := a.retryInvalidCompletionTurn(ctx, state, assistantMessage, err)
			if retryErr != nil {
				if persistWorkspace {
					_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
				}
				_ = a.failRun(ctx, state, retryErr.Error())
				return ExecuteRunResult{}, nonRetryableRunError(retryErr)
			}
			if retried {
				now := time.Now()
				state.run.Status = model.AgentRunStatusRunning
				state.run.PauseReason = model.AgentRunPauseReasonNone
				state.run.CompletedAt = nil
				state.run.ExecutionStage = strPtr("continuing")
				state.run.LastHeartbeatAt = &now
				if err := a.runRepo.Update(ctx, state.run); err != nil {
					return ExecuteRunResult{}, err
				}
				a.runRepo.Notify(ctx, state.run)
				return ExecuteRunResult{ContinueExecution: true}, nil
			}
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
	}
	completedAt := time.Now()
	normalizeApprovalStateAfterExecution(state.run, waitForApproval)
	if waitForApproval {
		state.run.Status = model.AgentRunStatusPaused
		state.run.PauseReason = model.AgentRunPauseReasonHumanApproval
		state.run.CompletedAt = nil
		if approvalRequest != nil && strings.TrimSpace(approvalRequest.Phase) != "" {
			state.run.ExecutionStage = strPtr(strings.TrimSpace(approvalRequest.Phase))
		} else {
			state.run.ExecutionStage = strPtr("awaiting_approval")
		}
	} else if waitForInput {
		state.run.Status = model.AgentRunStatusPaused
		state.run.PauseReason = model.AgentRunPauseReasonHumanInput
		state.run.CompletedAt = nil
		state.run.ExecutionStage = strPtr("awaiting_input")
	} else if waitForAuth {
		state.run.Status = model.AgentRunStatusPaused
		state.run.PauseReason = model.AgentRunPauseReasonAuthentication
		state.run.CompletedAt = nil
		state.run.ExecutionStage = strPtr("awaiting_auth")
	} else if continueExecution {
		state.run.Status = model.AgentRunStatusRunning
		state.run.PauseReason = model.AgentRunPauseReasonNone
		state.run.CompletedAt = nil
	} else {
		state.run.Status = "completed"
		state.run.PauseReason = model.AgentRunPauseReasonNone
		state.run.CompletedAt = &completedAt
		state.run.ExecutionStage = strPtr("completed")
	}
	state.run.LastHeartbeatAt = &completedAt
	if isEmptySummary(state.run.OutputSummary) {
		state.run.OutputSummary = json.RawMessage(`{"status":"success"}`)
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}
	a.runRepo.Notify(ctx, state.run)
	if persistWorkspace && !waitForApproval && !waitForInput && !waitForAuth && !continueExecution {
		_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
	}

	if waitForApproval || waitForInput || waitForAuth {
		if err := a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed); err != nil {
			return ExecuteRunResult{}, err
		}
	} else if !continueExecution {
		if err := a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed); err != nil {
			return ExecuteRunResult{}, err
		}
	}

	return ExecuteRunResult{
		WaitForApproval:   waitForApproval,
		AwaitingInput:     waitForInput && !waitForApproval,
		AwaitingAuth:      waitForAuth && !waitForApproval && !waitForInput,
		ContinueExecution: continueExecution && !waitForApproval && !waitForInput && !waitForAuth,
	}, nil
}

func (a *AgentRunActivities) stageRuntimeSkills(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, runtimeKind string) error {
	if a == nil || state == nil || state.run == nil || state.agent == nil || execCtx == nil {
		return nil
	}
	if len(agentskills.EffectiveRuntimeRefs(state.agent)) == 0 {
		return nil
	}
	stageRoot := workerpkg.RuntimeSkillRootPathForRun(state.run.ID, runtimeKind)
	resolution, err := agentskills.StageInto(
		ctx,
		state.run.WorkspaceID,
		state.agent,
		state.resolved.Tools,
		a.workspaceSkillRepo,
		a.skillPackageStore,
		stageRoot,
	)
	if err != nil {
		return fmt.Errorf("stage runtime skills: %w", err)
	}
	execCtx.StagedRuntimeSkillRoot = stageRoot
	a.persistRuntimeSkillManifest(ctx, state.run, runtimeKind, stageRoot, resolution)
	return nil
}

func (a *AgentRunActivities) persistRuntimeSkillManifest(ctx context.Context, run *model.AgentRun, runtimeKind, stageRoot string, resolution agentskills.Resolution) {
	if a == nil || a.artifactRepo == nil || run == nil {
		return
	}
	type skillManifestEntry struct {
		Key        string  `json:"key"`
		SourceKind string  `json:"source_kind"`
		VersionKey *string `json:"version_key,omitempty"`
		SkillID    *string `json:"skill_id,omitempty"`
	}
	entries := make([]skillManifestEntry, 0, len(resolution.Refs))
	for idx, ref := range resolution.Refs {
		definition := resolution.Definitions[idx]
		entries = append(entries, skillManifestEntry{
			Key:        definition.Key,
			SourceKind: definition.SourceKind,
			VersionKey: ref.VersionKey,
			SkillID:    ref.SkillID,
		})
	}
	payload, err := json.Marshal(map[string]any{
		"runtime_kind": runtimeKind,
		"staged_root":  stageRoot,
		"skills":       entries,
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to marshal runtime skill manifest", "error", err, "run_id", run.ID)
		return
	}
	seqNo, err := a.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		slog.WarnContext(ctx, "failed to allocate runtime skill manifest sequence", "error", err, "run_id", run.ID)
		return
	}
	content := string(payload)
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  "runtime_skill_manifest",
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    seqNo,
	}
	if err := a.artifactRepo.Create(ctx, artifact); err != nil {
		slog.WarnContext(ctx, "failed to persist runtime skill manifest", "error", err, "run_id", run.ID)
	}
}

func resolveExecutionWaitState(run *model.AgentRun, humanInputRequest *workerpkg.UserInputRequest, approvalRequest *model.ApprovalRequest, reviewRequest *model.ReviewCheckpointRequest, authRequest *model.CodexAuthState) (waitForApproval bool, waitForInput bool, waitForAuth bool) {
	if run == nil {
		return false, false, false
	}

	if approvalRequest != nil || reviewRequest != nil {
		return true, false, false
	}
	if humanInputRequest != nil {
		return false, true, false
	}
	if authRequest != nil {
		return false, false, true
	}

	waitForApproval = run.ApprovalState == "pending"
	if run.InvocationMode != model.InvocationModeInteractive {
		return waitForApproval, false, false
	}
	return waitForApproval, false, false
}

func normalizeApprovalStateAfterExecution(run *model.AgentRun, waitForApproval bool) {
	if run == nil {
		return
	}
	if waitForApproval {
		run.ApprovalState = "pending"
		return
	}
	if run.ApprovalState != "approved" {
		run.ApprovalState = "not_required"
	}
}

func (a *AgentRunActivities) enforceCompletionInteractionPolicy(ctx context.Context, state *resolvedRunState, assistantMessage *model.AgentRunMessage) error {
	if a == nil || state == nil || state.run == nil {
		return nil
	}

	requiredKinds := completionRequiredInteractionKinds(state.skillPolicy)
	if len(requiredKinds) == 0 {
		return nil
	}
	if a.interactionRepo == nil {
		kinds := sortedCompletionInteractionKinds(requiredKinds)
		return fmt.Errorf("run cannot complete because active skills require one of [%s] before completion, but interaction storage is unavailable", strings.Join(kinds, ", "))
	}

	interactions, err := a.interactionRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return fmt.Errorf("verify completion interaction requirements: %w", err)
	}
	currentAssistantSequenceNo := 0
	if assistantMessage != nil {
		currentAssistantSequenceNo = assistantMessage.SequenceNo
	}
	for _, interaction := range interactions {
		if currentAssistantSequenceNo > 0 {
			if interaction.AssistantMessageSequenceNo == nil || *interaction.AssistantMessageSequenceNo != currentAssistantSequenceNo {
				continue
			}
		}
		if _, ok := requiredKinds[strings.TrimSpace(interaction.InteractionKind)]; ok {
			if err := a.validateApprovalInteractionPreviewContract(ctx, state, interaction, currentAssistantSequenceNo); err != nil {
				return err
			}
			return nil
		}
	}
	if allowsTerminalCleanImplementationReview(state, assistantMessage) {
		return nil
	}
	if allowsPostApprovalImplementationCompletion(interactions, currentAssistantSequenceNo) {
		return nil
	}

	kinds := sortedCompletionInteractionKinds(requiredKinds)
	return fmt.Errorf("run cannot complete because active skills require one of [%s] before completion", strings.Join(kinds, ", "))
}

func (a *AgentRunActivities) validateApprovalInteractionPreviewContract(ctx context.Context, state *resolvedRunState, interaction model.AgentRunInteraction, currentAssistantSequenceNo int) error {
	if a == nil || state == nil || state.run == nil || a.artifactRepo == nil {
		return nil
	}
	interactionKind := strings.TrimSpace(interaction.InteractionKind)
	if interactionKind != model.AgentRunInteractionKindReviewCheckpoint && interactionKind != model.AgentRunInteractionKindApprovalRequest {
		return nil
	}

	var approval model.ApprovalRequest
	if err := json.Unmarshal(interaction.RequestPayload, &approval); err != nil {
		return fmt.Errorf("parse approval payload: %w", err)
	}
	approval.PreviewPanelKey = normalizeApprovalPreviewPanelKey(approval.PreviewPanelKey)

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return fmt.Errorf("verify approval preview artifacts: %w", err)
	}
	matchCount := 0
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != workerpkg.RunPreviewArtifactType || artifact.InlineContent == nil {
			continue
		}
		if currentAssistantSequenceNo > 0 && artifactAssistantMessageSequenceNo(artifact) != currentAssistantSequenceNo {
			continue
		}
		var preview workerpkg.PublishedPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
			return fmt.Errorf("parse approval preview artifact: %w", err)
		}
		if approval.PreviewPanelKey != "" && normalizeApprovalPreviewPanelKey(preview.PanelKey) != approval.PreviewPanelKey {
			continue
		}
		matchCount++
		if approval.PreviewPanelKey != "" {
			return nil
		}
	}
	if approval.PreviewPanelKey != "" {
		return fmt.Errorf("%s requires a same-turn %s preview before requesting approval", interactionKind, approval.PreviewPanelKey)
	}
	if matchCount == 1 {
		return nil
	}
	if matchCount > 1 {
		return fmt.Errorf("%s requires preview_panel_key when multiple same-turn previews exist", interactionKind)
	}
	return fmt.Errorf("%s requires a same-turn preview before requesting approval", interactionKind)
}

func completionRequiredInteractionKinds(policy workerpkg.SkillPolicy) map[string]struct{} {
	if len(policy.CompletionRequiresInteractionKinds) == 0 {
		return nil
	}

	declaredContracts := make(map[string]struct{}, len(policy.InteractionContracts))
	for _, contract := range workerpkg.NormalizeInteractionContracts(policy.InteractionContracts) {
		if kind := strings.TrimSpace(contract.Kind); kind != "" {
			declaredContracts[kind] = struct{}{}
		}
	}

	out := make(map[string]struct{}, len(policy.CompletionRequiresInteractionKinds))
	for _, value := range policy.CompletionRequiresInteractionKinds {
		normalized := strings.TrimSpace(value)
		switch normalized {
		case model.AgentRunInteractionKindRequestUserInput:
			out[model.AgentRunInteractionKindRequestUserInput] = struct{}{}
		case model.AgentRunInteractionKindApprovalRequest:
			out[model.AgentRunInteractionKindApprovalRequest] = struct{}{}
		case model.AgentRunInteractionKindReviewCheckpoint:
			out[model.AgentRunInteractionKindReviewCheckpoint] = struct{}{}
		case model.AgentRunInteractionKindCommandExecutionApproval,
			model.AgentRunInteractionKindFileChangeApproval,
			model.AgentRunInteractionKindPermissionsApproval,
			model.AgentRunInteractionKindAuthRequired:
			out[normalized] = struct{}{}
		default:
			if _, ok := declaredContracts[normalized]; ok {
				out[normalized] = struct{}{}
			}
		}
	}
	return out
}

func sortedCompletionInteractionKinds(values map[string]struct{}) []string {
	kinds := make([]string, 0, len(values))
	for kind := range values {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

func (a *AgentRunActivities) synthesizeCompletionInteractionFallback(ctx context.Context, state *resolvedRunState, assistantMessage *model.AgentRunMessage) (*model.ReviewCheckpointRequest, *workerpkg.UserInputRequest, error) {
	if a == nil || state == nil || state.run == nil || assistantMessage == nil {
		return nil, nil, nil
	}
	switch strings.TrimSpace(state.run.RuntimeKind) {
	case "codex", "opencode":
	default:
		return nil, nil, nil
	}

	requiredKinds := completionRequiredInteractionKinds(state.skillPolicy)
	if len(requiredKinds) == 0 {
		return nil, nil, nil
	}

	content := strings.TrimSpace(assistantMessage.Content)
	if content == "" {
		return nil, nil, nil
	}

	if _, ok := requiredKinds[model.AgentRunInteractionKindReviewCheckpoint]; ok {
		if a.interactionRepo != nil {
			interactions, err := a.interactionRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
			if err != nil {
				return nil, nil, err
			}
			if allowsPostApprovalImplementationCompletion(interactions, assistantMessage.SequenceNo) {
				return nil, nil, nil
			}
		}
		if terminalReview := terminalCleanImplementationReviewRequest(state, assistantMessage); terminalReview != nil {
			metadata := buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo)
			if a.artifactRepo != nil {
				if reviewFindings := reviewFindingsArtifactFromReviewCheckpointRequest(terminalReview, assistantMessage.SequenceNo); reviewFindings != nil {
					if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeReviewFindings, "json", reviewFindings, metadata); err != nil {
						return nil, nil, err
					}
				}
			}
			return nil, nil, nil
		}
		reviewRequest := synthesizedReviewCheckpointFromAssistantMessage(state, assistantMessage)
		if reviewRequest == nil {
			return nil, nil, nil
		}
		metadata := buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", reviewRequest, metadata); err != nil {
				return nil, nil, err
			}
			if reviewFindings := reviewFindingsArtifactFromReviewCheckpointRequest(reviewRequest, assistantMessage.SequenceNo); reviewFindings != nil {
				if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeReviewFindings, "json", reviewFindings, metadata); err != nil {
					return nil, nil, err
				}
			}
		}
		interaction, err := a.persistHumanApprovalInteraction(ctx, state, model.AgentRunInteractionKindReviewCheckpoint, reviewRequest.Title, reviewRequest.Summary, reviewRequest, metadata, assistantMessage.SequenceNo)
		if err != nil {
			return nil, nil, err
		}
		if interaction != nil {
			a.maybeNotifyAgentAttentionRequired(ctx, state, interaction)
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "approval.requested", map[string]any{
				"content": reviewRequest,
			})
		}
		return reviewRequest, nil, nil
	}

	return nil, nil, nil
}

func allowsTerminalCleanImplementationReview(state *resolvedRunState, assistantMessage *model.AgentRunMessage) bool {
	return terminalCleanImplementationReviewRequest(state, assistantMessage) != nil
}

func allowsPostApprovalImplementationCompletion(interactions []model.AgentRunInteraction, currentAssistantSequenceNo int) bool {
	if len(interactions) == 0 {
		return false
	}
	for idx := len(interactions) - 1; idx >= 0; idx-- {
		interaction := interactions[idx]
		if strings.TrimSpace(interaction.Status) != model.AgentRunInteractionStatusResolved {
			continue
		}
		if strings.TrimSpace(interaction.InteractionKind) != model.AgentRunInteractionKindReviewCheckpoint {
			continue
		}
		var response model.ReviewCheckpointResponse
		if err := json.Unmarshal(interaction.ResponsePayload, &response); err != nil {
			return false
		}
		if strings.TrimSpace(response.Decision) != "approve" {
			return false
		}
		if currentAssistantSequenceNo > 0 && interaction.AssistantMessageSequenceNo != nil && *interaction.AssistantMessageSequenceNo >= currentAssistantSequenceNo {
			return false
		}
		return true
	}
	return false
}

func terminalCleanImplementationReviewRequest(state *resolvedRunState, assistantMessage *model.AgentRunMessage) *model.ReviewCheckpointRequest {
	if state == nil || state.agent == nil || assistantMessage == nil {
		return nil
	}
	if strings.TrimSpace(state.agent.EffectivePresetKey()) != model.AgentPresetReviewAgent {
		return nil
	}
	request := synthesizedReviewCheckpointFromAssistantMessage(state, assistantMessage)
	if request == nil {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(request.Phase), "implementation") {
		return nil
	}
	if len(request.Findings) > 0 {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(request.OverallCorrectness), "correct") {
		return nil
	}
	return request
}

func synthesizedReviewCheckpointFromAssistantMessage(state *resolvedRunState, assistantMessage *model.AgentRunMessage) *model.ReviewCheckpointRequest {
	if assistantMessage == nil {
		return nil
	}
	content := strings.TrimSpace(assistantMessage.Content)
	if content == "" {
		return nil
	}

	if structured, ok := parseStructuredReviewApprovalRequest(state.skillPolicy, strings.TrimSpace(state.run.RuntimeKind), content); ok {
		if strings.TrimSpace(structured.Phase) == "" {
			if state != nil && state.agent != nil && strings.TrimSpace(state.agent.EffectivePresetKey()) == model.AgentPresetReviewAgent {
				structured.Phase = "review_findings"
			} else {
				structured.Phase = "review"
			}
		}
		return structured
	}

	title := "Review findings"
	phase := "review_findings"
	if state != nil && state.agent != nil && strings.TrimSpace(state.agent.EffectivePresetKey()) != model.AgentPresetReviewAgent {
		title = "Checkpoint review"
		phase = "review"
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		if first != "" && len(first) <= 80 {
			title = first
		}
	}

	return &model.ReviewCheckpointRequest{
		Phase:   phase,
		Title:   title,
		Summary: content,
	}
}

func parseStructuredReviewApprovalRequest(policy workerpkg.SkillPolicy, runtimeKind, content string) (*model.ReviewCheckpointRequest, bool) {
	payload, prose := extractStructuredReviewPayload(policy, runtimeKind, content)
	if strings.TrimSpace(payload) == "" {
		return nil, false
	}

	var req model.ReviewCheckpointRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return nil, false
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Summary = strings.TrimSpace(req.Summary)
	req.Phase = strings.TrimSpace(req.Phase)
	req.PreviewPanelKey = normalizeApprovalPreviewPanelKey(req.PreviewPanelKey)
	req.OverallCorrectness = strings.TrimSpace(req.OverallCorrectness)
	req.OverallExplanation = strings.TrimSpace(req.OverallExplanation)
	normalizeStructuredReviewFindings(req.Findings)
	for idx := range req.Findings {
		req.Findings[idx].Title = strings.TrimSpace(req.Findings[idx].Title)
		req.Findings[idx].Body = strings.TrimSpace(req.Findings[idx].Body)
		req.Findings[idx].Priority = strings.TrimSpace(req.Findings[idx].Priority)
		req.Findings[idx].CodeLocation = strings.TrimSpace(req.Findings[idx].CodeLocation)
	}

	if req.Title == "" {
		req.Title = "Review findings"
	}
	if req.Summary == "" {
		req.Summary = prose
	}
	if req.Summary == "" && len(req.Findings) == 0 && req.OverallCorrectness == "" && req.OverallExplanation == "" {
		return nil, false
	}
	return &req, true
}

func extractStructuredReviewPayload(policy workerpkg.SkillPolicy, runtimeKind, content string) (payload string, prose string) {
	label := workerpkg.ReviewCheckpointFencedBlockLabel(policy, runtimeKind)
	pattern := regexp.MustCompile(fmt.Sprintf("(?s)```%s\\s*\\n(.*?)\\n```", regexp.QuoteMeta(label)))
	matches := pattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return "", strings.TrimSpace(content)
	}
	last := matches[len(matches)-1]
	if len(last) < 4 {
		return "", strings.TrimSpace(content)
	}
	payload = strings.TrimSpace(content[last[2]:last[3]])
	prose = strings.TrimSpace(strings.TrimSpace(content[:last[0]]) + "\n" + strings.TrimSpace(content[last[1]:]))
	return payload, prose
}

func normalizeStructuredReviewFindings(findings []model.ReviewFinding) {
	for idx := range findings {
		findings[idx].ID = strings.TrimSpace(findings[idx].ID)
		if findings[idx].ID == "" {
			findings[idx].ID = fmt.Sprintf("finding_%d", idx+1)
		}
	}
}

func (a *AgentRunActivities) retryInvalidCompletionTurn(ctx context.Context, state *resolvedRunState, assistantMessage *model.AgentRunMessage, cause error) (bool, error) {
	if a == nil || state == nil || state.run == nil || a.runMessageRepo == nil {
		return false, nil
	}
	if cause == nil || strings.TrimSpace(cause.Error()) == "" {
		return false, nil
	}

	messages, err := a.runMessageRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return false, err
	}
	for _, message := range messages {
		if strings.TrimSpace(message.MessageType) == "policy_retry" {
			return false, nil
		}
	}

	retryInstruction := normalizedCompletionRetryInstruction(state, cause)
	instruction := strings.TrimSpace(retryInstruction.Instructions)
	if instruction == "" {
		return false, nil
	}
	if _, err := a.createRunMessage(ctx, state.run, "user", "policy_retry", instruction, nil, nil, nil, nil); err != nil {
		return false, err
	}
	if err := a.persistNativeCompletionRetryRepairState(ctx, state, assistantMessage, cause, retryInstruction); err != nil {
		return false, err
	}
	return true, nil
}

func (a *AgentRunActivities) persistNativeCompletionRetryRepairState(ctx context.Context, state *resolvedRunState, assistantMessage *model.AgentRunMessage, cause error, repairInstruction nativeRepairInstruction) error {
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || assistantMessage == nil || cause == nil {
		return nil
	}
	runtimeKind := executionRuntimeKind(state)
	if !state.nativeSelectivePathEnabled || strings.TrimSpace(runtimeKind) != "native_sdk" {
		return nil
	}
	if strings.TrimSpace(repairInstruction.Class) == "" || strings.TrimSpace(repairInstruction.Instructions) == "" {
		return nil
	}
	payload := model.NativeRepairState{
		Source:       "completion_retry",
		RepairClass:  strings.TrimSpace(repairInstruction.Class),
		RepairHint:   strings.TrimSpace(repairInstruction.Instructions),
		ErrorSummary: strings.TrimSpace(cause.Error()),
	}
	_, err := a.appendRunArtifactWithMetadata(
		ctx,
		state.run,
		model.AgentRunArtifactTypeNativeRepairState,
		"json",
		payload,
		buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo),
	)
	return err
}

func approvalPreviewRetryInstruction(causeText string) string {
	causeText = strings.TrimSpace(causeText)
	switch {
	case strings.Contains(causeText, "requires preview_panel_key when multiple same-turn previews exist"):
		return "System correction: the previous turn requested approval after publishing multiple same-turn previews but did not include preview_panel_key. Continue from your last assistant message instead of restarting. Do not end with prose only. If you emit request_approval or request_review_checkpoint, publish the intended preview in the same turn and include preview_panel_key so the handoff binds to the correct preview."
	case strings.Contains(causeText, "requires a same-turn ") && strings.Contains(causeText, " preview before requesting approval"):
		requiredPreviewKey := extractRequiredPreviewKeyFromCause(causeText)
		if requiredPreviewKey == "" {
			return "System correction: the previous turn requested approval without binding it to the required same-turn preview. Continue from your last assistant message instead of restarting. Do not end with prose only. Publish the intended preview in the same turn before the approval handoff, and include preview_panel_key when needed so it binds to the correct preview."
		}
		return fmt.Sprintf("System correction: the previous turn requested approval without binding it to the required same-turn %s preview. Continue from your last assistant message instead of restarting. Do not end with prose only. Publish the %s preview in the same turn before the approval handoff, and set preview_panel_key=%q on request_approval or request_review_checkpoint so it binds to the correct preview.", requiredPreviewKey, requiredPreviewKey, requiredPreviewKey)
	default:
		return ""
	}
}

func extractRequiredPreviewKeyFromCause(causeText string) string {
	return extractPreviewKeyAfterMarker(causeText, "requires a same-turn ", " preview before requesting approval")
}

func extractPreviewKeyAfterMarker(content, marker, terminator string) string {
	content = strings.TrimSpace(content)
	if content == "" || marker == "" || terminator == "" {
		return ""
	}
	start := strings.Index(content, marker)
	if start == -1 {
		return ""
	}
	remaining := content[start+len(marker):]
	end := strings.Index(remaining, terminator)
	if end == -1 {
		return ""
	}
	return normalizeApprovalPreviewPanelKey(strings.TrimSpace(remaining[:end]))
}

func (a *AgentRunActivities) finalizeSupportConversationRun(ctx context.Context, state *resolvedRunState) error {
	if state == nil || state.run == nil || state.run.TargetType != "support_conversation" || state.conversation == nil {
		return nil
	}
	if state.run.ApprovalState == "pending" {
		return nil
	}

	var summary supportRunActivitySummary
	if len(state.run.OutputSummary) == 0 {
		return nil
	}
	if err := json.Unmarshal(state.run.OutputSummary, &summary); err != nil {
		return fmt.Errorf("parse support run summary: %w", err)
	}
	if summary.SentMessageID != nil && strings.TrimSpace(*summary.SentMessageID) != "" {
		return nil
	}
	if summary.DraftReply == nil || strings.TrimSpace(summary.DraftReply.Content) == "" {
		return nil
	}

	messageID := state.run.ID
	existing, err := a.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("lookup support reply message: %w", err)
	}
	createdMessage := false
	if existing == nil {
		createdMessage = true
		existing = &model.SupportMessage{
			ID:                messageID,
			WorkspaceID:       state.run.WorkspaceID,
			ConversationID:    state.conversation.ID,
			SenderType:        "agent",
			SenderAgentID:     &state.run.AgentID,
			SenderDisplayName: summary.DraftReply.SenderDisplayName,
			Content:           strings.TrimSpace(summary.DraftReply.Content),
			IsInternal:        summary.DraftReply.IsInternal,
			MessageType:       "reply",
		}
		if err := a.messageRepo.Create(ctx, existing); err != nil {
			return fmt.Errorf("create support reply message: %w", err)
		}
	}

	summary.SentMessageID = &existing.ID
	payload, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal support run summary: %w", err)
	}
	state.run.OutputSummary = payload
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return fmt.Errorf("persist support run summary: %w", err)
	}

	if a.wsPublisher != nil {
		event := websocket.SupportMessageEvent(state.run.WorkspaceID, existing, "")
		if !createdMessage {
			event.Data = nil
		}
		a.wsPublisher.Publish(event)
	}
	a.pushVisitorConversationRefresh(ctx, state.run.WorkspaceID, state.conversation)
	return nil
}

func (a *AgentRunActivities) ensureRunConversation(ctx context.Context, state *resolvedRunState, initialInstructions string, planningInput planningRunInput) ([]workerpkg.ExecutionMessage, *workerpkg.ArtifactContext, *workerpkg.ProviderContinuation, nativeRepairInstruction, error) {
	artifactContext, err := a.loadRunArtifactContext(ctx, state)
	if err != nil {
		return nil, nil, nil, nativeRepairInstruction{}, err
	}
	providerContinuation, err := a.loadProviderContinuation(ctx, state)
	if err != nil {
		return nil, nil, nil, nativeRepairInstruction{}, err
	}
	if a.runMessageRepo == nil {
		return nil, artifactContext, providerContinuation, nativeRepairInstruction{}, nil
	}

	messages, err := a.runMessageRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, nil, nil, nativeRepairInstruction{}, err
	}
	var artifacts []model.AgentRunArtifact
	if state.nativeSelectivePathEnabled && a.artifactRepo != nil {
		artifacts, err = a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
		if err != nil {
			return nil, nil, nil, nativeRepairInstruction{}, err
		}
	}
	repairInstruction := resolveLatestNativeRepairInstruction(state, messages, artifacts)
	replayMessages := replayMessagesForExecution(state, messages)
	if !hasExecutionHistoryMessages(replayMessages) {
		prompt, err := a.buildInitialRunUserPrompt(ctx, state, artifactContext, planningInput, initialInstructions)
		if err != nil {
			return nil, nil, nil, nativeRepairInstruction{}, err
		}
		created, err := a.createRunMessage(ctx, state.run, "user", "prompt", prompt, nil, nil, nil, nil)
		if err != nil {
			return nil, nil, nil, nativeRepairInstruction{}, err
		}
		messages = append(messages, *created)
		replayMessages = append(replayMessages, *created)
	}

	transcriptSummary, err := a.ensureTranscriptSummaryCheckpoint(ctx, state, replayMessages)
	if err != nil {
		return nil, nil, nil, nativeRepairInstruction{}, err
	}
	history := workerpkg.BuildExecutionHistory(replayMessages, transcriptSummary)
	slog.InfoContext(ctx, "agent run prepared execution history",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"native_selective_path_enabled", state.nativeSelectivePathEnabled,
		"repair_guidance_present", strings.TrimSpace(repairInstruction.Instructions) != "",
		"repair_guidance_class", strings.TrimSpace(repairInstruction.Class),
		"filtered_policy_retry_messages", len(messages)-len(replayMessages),
		"history_messages", len(history),
		"transcript_summary_present", transcriptSummary != nil && strings.TrimSpace(transcriptSummary.Summary) != "",
	)
	return history, artifactContext, providerContinuation, repairInstruction, nil
}

func (a *AgentRunActivities) buildInitialRunUserPrompt(ctx context.Context, state *resolvedRunState, artifactContext *workerpkg.ArtifactContext, planningInput planningRunInput, initialInstructions string) (string, error) {
	var checklist []model.PMChecklistItem
	if state.task != nil {
		items, err := a.checklistRepo.List(ctx, state.task.ID)
		if err != nil {
			return "", err
		}
		checklist = items
	}

	var ticketMessages []model.SupportMessage
	if state.conversation != nil {
		messages, err := a.messageRepo.ListByConversation(ctx, state.run.WorkspaceID, state.conversation.ID, true)
		if err != nil {
			return "", err
		}
		ticketMessages = messages
	}

	return workerpkg.BuildUserPrompt(
		state.agent,
		state.task,
		state.epic,
		state.epicTasks,
		state.conversation,
		ticketMessages,
		checklist,
		artifactContext,
		planningInput.Stage,
		initialInstructions,
	), nil
}

func (a *AgentRunActivities) loadProviderContinuation(ctx context.Context, state *resolvedRunState) (*workerpkg.ProviderContinuation, error) {
	if state == nil || state.run == nil || state.agent == nil || a.artifactRepo == nil {
		return nil, nil
	}
	provider := strings.TrimSpace(derefString(state.agent.Provider))
	if !workerpkg.ProviderSupportsResponseContinuation(provider) {
		return nil, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, err
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeProviderResponseCheckpoint || artifact.InlineContent == nil {
			continue
		}
		var checkpoint model.ProviderResponseCheckpoint
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &checkpoint); err != nil {
			return nil, fmt.Errorf("parse provider response checkpoint: %w", err)
		}
		if strings.TrimSpace(checkpoint.ResponseID) == "" {
			continue
		}
		continuation := &workerpkg.ProviderContinuation{
			Provider:           strings.TrimSpace(checkpoint.Provider),
			ResponseID:         strings.TrimSpace(checkpoint.ResponseID),
			PreviousResponseID: strings.TrimSpace(checkpoint.PreviousResponseID),
			AfterSequenceNo:    checkpoint.AssistantMessageSeqNo,
		}
		slog.InfoContext(ctx, "loaded provider continuation checkpoint",
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"provider", provider,
			"response_id", continuation.ResponseID,
			"after_sequence_no", continuation.AfterSequenceNo,
		)
		return continuation, nil
	}
	return nil, nil
}

func (a *AgentRunActivities) loadRunArtifactContext(ctx context.Context, state *resolvedRunState) (*workerpkg.ArtifactContext, error) {
	if state == nil || state.run == nil {
		return nil, nil
	}

	artifactContext := &workerpkg.ArtifactContext{}
	if parentRunID := strings.TrimSpace(derefString(state.run.ParentRunID)); parentRunID != "" && a.runRepo != nil {
		parentRun, err := a.runRepo.GetByID(ctx, state.run.WorkspaceID, parentRunID)
		if err != nil {
			return nil, err
		}
		parentEntries, err := a.loadParentRunArtifactContext(ctx, parentRun)
		if err != nil {
			return nil, err
		}
		artifactContext.Entries = append(artifactContext.Entries, parentEntries...)
	}
	if a.artifactRepo != nil {
		artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
		if err != nil {
			return nil, err
		}
		entries, err := buildArtifactContextEntries(artifacts)
		if err != nil {
			return nil, err
		}
		artifactContext.Entries = append(artifactContext.Entries, entries...)
	}
	if state.epic != nil && state.epic.SpecDocumentID != nil && strings.TrimSpace(*state.epic.SpecDocumentID) != "" && a.docsContentRepo != nil {
		content, err := a.docsContentRepo.GetByDocumentID(ctx, *state.epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if markdown := docsContentMarkdown(content); markdown != "" {
			artifactContext.Entries = append(artifactContext.Entries, workerpkg.ArtifactContextEntry{
				Label:        "Linked epic spec document",
				Source:       "spec_document",
				Status:       "approved",
				Format:       "markdown",
				Content:      markdown,
				PreserveFull: true,
			})
		}
	}
	if len(artifactContext.Entries) == 0 {
		return nil, nil
	}
	return workerpkg.TrimArtifactContext(artifactContext), nil
}

func (a *AgentRunActivities) loadParentRunArtifactContext(ctx context.Context, parentRun *model.AgentRun) ([]workerpkg.ArtifactContextEntry, error) {
	if parentRun == nil || a.artifactRepo == nil {
		return nil, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, parentRun.WorkspaceID, parentRun.ID)
	if err != nil {
		return nil, err
	}

	entries := make([]workerpkg.ArtifactContextEntry, 0, 4)
	if reason := strings.TrimSpace(derefString(parentRun.ErrorMessage)); reason != "" {
		entries = append(entries, workerpkg.ArtifactContextEntry{
			Label:        "Previous run failure reason",
			Source:       "previous_run_error",
			Status:       strings.TrimSpace(parentRun.Status),
			Format:       "text",
			Content:      reason,
			PreserveFull: true,
		})
	}
	if checkpoint, err := workerpkg.LatestTranscriptSummaryCheckpoint(artifacts); err != nil {
		return nil, err
	} else if checkpoint != nil && strings.TrimSpace(checkpoint.Summary) != "" {
		entries = append(entries, workerpkg.ArtifactContextEntry{
			Label:   "Previous run transcript summary",
			Source:  workerpkg.TranscriptSummaryArtifactType,
			Status:  strings.TrimSpace(parentRun.Status),
			Format:  "text",
			Content: strings.TrimSpace(checkpoint.Summary),
		})
	}

	parentEntries, err := buildArtifactContextEntries(artifacts)
	if err != nil {
		return nil, err
	}
	for _, entry := range parentEntries {
		entry.Label = "Previous run: " + entry.Label
		if strings.TrimSpace(entry.Status) == "" {
			entry.Status = strings.TrimSpace(parentRun.Status)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func buildArtifactContextEntries(artifacts []model.AgentRunArtifact) ([]workerpkg.ArtifactContextEntry, error) {
	if len(artifacts) == 0 {
		return nil, nil
	}
	sorted := append([]model.AgentRunArtifact(nil), artifacts...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].SequenceNo == sorted[j].SequenceNo {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].SequenceNo < sorted[j].SequenceNo
	})

	applied := make(map[string]model.AppliedApprovedRunPreview)
	latestRunPreview := make(map[string]workerpkg.PublishedPreview)
	latestApproved := make(map[string]approvedPreviewContextEntry)
	var latestRunPlan *workerpkg.ArtifactContextEntry
	otherEntries := make([]workerpkg.ArtifactContextEntry, 0)

	for _, artifact := range sorted {
		if artifact.InlineContent == nil {
			continue
		}
		switch strings.TrimSpace(artifact.ArtifactType) {
		case model.AgentRunArtifactTypeApprovedPreviewApplied:
			var marker model.AppliedApprovedRunPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &marker); err != nil {
				return nil, fmt.Errorf("parse approved preview applied artifact: %w", err)
			}
			if id := strings.TrimSpace(marker.ApprovedArtifactID); id != "" {
				applied[id] = marker
			}
		case workerpkg.RunPreviewArtifactType:
			var preview workerpkg.PublishedPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
				return nil, fmt.Errorf("parse run preview artifact: %w", err)
			}
			latestRunPreview[strings.TrimSpace(preview.PanelKey)] = preview
		case model.AgentRunArtifactTypeApprovedPreview:
			var preview model.ApprovedRunPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
				return nil, fmt.Errorf("parse approved preview artifact: %w", err)
			}
			latestApproved[strings.TrimSpace(preview.PanelKey)] = approvedPreviewContextEntry{
				ArtifactID: artifact.ID,
				Preview:    preview,
			}
		case model.AgentRunArtifactTypeRunPlan:
			entry, err := buildRunPlanArtifactContextEntry(artifact)
			if err != nil {
				return nil, err
			}
			latestRunPlan = entry
		default:
			if entry := buildOtherArtifactContextEntry(artifact); entry != nil {
				otherEntries = append(otherEntries, *entry)
			}
		}
	}

	panelKeys := make([]string, 0, len(latestRunPreview)+len(latestApproved))
	seenPanels := make(map[string]bool)
	for panelKey := range latestRunPreview {
		if panelKey == "" || seenPanels[panelKey] {
			continue
		}
		panelKeys = append(panelKeys, panelKey)
		seenPanels[panelKey] = true
	}
	for panelKey := range latestApproved {
		if panelKey == "" || seenPanels[panelKey] {
			continue
		}
		panelKeys = append(panelKeys, panelKey)
		seenPanels[panelKey] = true
	}
	sort.Strings(panelKeys)

	entries := make([]workerpkg.ArtifactContextEntry, 0, len(panelKeys)*2+len(otherEntries))
	for _, panelKey := range panelKeys {
		if preview, ok := latestRunPreview[panelKey]; ok && latestApproved[panelKey].Preview.Phase == "" {
			content, err := renderArtifactContextContent(preview.Format, preview.Content)
			if err != nil {
				return nil, err
			}
			entries = append(entries, workerpkg.ArtifactContextEntry{
				Label:        fmt.Sprintf("Current preview for %s", panelKey),
				Source:       workerpkg.RunPreviewArtifactType,
				Status:       "draft",
				Format:       preview.Format,
				Content:      content,
				PreserveFull: true,
			})
		}
		if approved, ok := latestApproved[panelKey]; ok {
			status := "approved"
			if marker, ok := applied[approved.ArtifactID]; ok && strings.TrimSpace(marker.Action) != "" {
				status = "approved_and_" + strings.TrimSpace(marker.Action)
			}
			content, err := renderArtifactContextContent(approved.Preview.Format, approved.Preview.Content)
			if err != nil {
				return nil, err
			}
			entries = append(entries, workerpkg.ArtifactContextEntry{
				Label:        fmt.Sprintf("Approved preview for %s", panelKey),
				Source:       model.AgentRunArtifactTypeApprovedPreview,
				Status:       status,
				Format:       approved.Preview.Format,
				Content:      content,
				PreserveFull: true,
			})
		}
	}
	if latestRunPlan != nil {
		entries = append(entries, *latestRunPlan)
	}
	entries = append(entries, otherEntries...)
	return entries, nil
}

type approvedPreviewContextEntry struct {
	ArtifactID string
	Preview    model.ApprovedRunPreview
}

func buildRunPlanArtifactContextEntry(artifact model.AgentRunArtifact) (*workerpkg.ArtifactContextEntry, error) {
	content := strings.TrimSpace(derefString(artifact.InlineContent))
	if content == "" {
		return nil, nil
	}
	var plan workerpkg.RunPlanArtifact
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("parse run plan artifact: %w", err)
	}
	if err := workerpkg.ValidateRunPlanArtifactForContext(&plan); err != nil {
		return nil, nil
	}
	rendered := workerpkg.FormatRunPlanArtifactContentForContext(&plan)
	if rendered == "" {
		return nil, nil
	}
	return &workerpkg.ArtifactContextEntry{
		Label:   "Current execution plan",
		Source:  model.AgentRunArtifactTypeRunPlan,
		Status:  "active",
		Format:  "text",
		Content: rendered,
	}, nil
}

func buildOtherArtifactContextEntry(artifact model.AgentRunArtifact) *workerpkg.ArtifactContextEntry {
	content := strings.TrimSpace(derefString(artifact.InlineContent))
	if content == "" {
		return nil
	}
	switch strings.TrimSpace(artifact.ArtifactType) {
	case "product_spec_draft":
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured product spec draft artifact",
			Source:       artifact.ArtifactType,
			Status:       "draft",
			Format:       artifact.Format,
			Content:      content,
			PreserveFull: true,
		}
	case "story_plan_proposal":
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured task plan proposal artifact",
			Source:       artifact.ArtifactType,
			Status:       "draft",
			Format:       artifact.Format,
			Content:      content,
			PreserveFull: true,
		}
	case model.AgentRunArtifactTypeReviewFindings:
		rendered := formatReviewFindingsArtifactContent(content)
		if rendered == "" {
			return nil
		}
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured review findings artifact",
			Source:       artifact.ArtifactType,
			Status:       "active",
			Format:       "text",
			Content:      rendered,
			PreserveFull: true,
		}
	case model.AgentRunArtifactTypeReviewDecision:
		rendered := formatReviewDecisionArtifactContent(content)
		if rendered == "" {
			return nil
		}
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured review decision artifact",
			Source:       artifact.ArtifactType,
			Status:       "active",
			Format:       "text",
			Content:      rendered,
			PreserveFull: true,
		}
	default:
		return nil
	}
}

func formatReviewFindingsArtifactContent(content string) string {
	var artifact model.ReviewFindingsArtifact
	if err := json.Unmarshal([]byte(content), &artifact); err != nil {
		return ""
	}
	lines := make([]string, 0, len(artifact.Findings)+4)
	if title := strings.TrimSpace(artifact.Title); title != "" {
		lines = append(lines, title)
	}
	if summary := strings.TrimSpace(artifact.Summary); summary != "" {
		lines = append(lines, summary)
	}
	if overall := strings.TrimSpace(artifact.OverallCorrectness); overall != "" {
		lines = append(lines, "Overall correctness: "+overall)
	}
	if explanation := strings.TrimSpace(artifact.OverallExplanation); explanation != "" {
		lines = append(lines, "Overall explanation: "+explanation)
	}
	for _, finding := range artifact.Findings {
		line := "- "
		if priority := strings.TrimSpace(finding.Priority); priority != "" {
			line += priority + " "
		}
		line += strings.TrimSpace(finding.Title)
		if location := strings.TrimSpace(finding.CodeLocation); location != "" {
			line += " (" + location + ")"
		}
		body := strings.TrimSpace(finding.Body)
		if body != "" {
			line += ": " + body
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func formatReviewDecisionArtifactContent(content string) string {
	var artifact model.ReviewDecisionArtifact
	if err := json.Unmarshal([]byte(content), &artifact); err != nil {
		return ""
	}
	lines := make([]string, 0, len(artifact.Findings)+4)
	if title := strings.TrimSpace(artifact.Title); title != "" {
		lines = append(lines, title)
	}
	if decision := strings.TrimSpace(artifact.Decision); decision != "" {
		lines = append(lines, "Decision: "+decision)
	}
	if message := strings.TrimSpace(artifact.Message); message != "" {
		lines = append(lines, "Message: "+message)
	}
	for _, finding := range artifact.Findings {
		line := "- " + strings.TrimSpace(finding.Title)
		if status := strings.TrimSpace(finding.Status); status != "" {
			line += " [" + status + "]"
		}
		if location := strings.TrimSpace(finding.CodeLocation); location != "" {
			line += " (" + location + ")"
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (a *AgentRunActivities) ensureTranscriptSummaryCheckpoint(ctx context.Context, state *resolvedRunState, messages []model.AgentRunMessage) (*workerpkg.TranscriptSummaryCheckpoint, error) {
	if a.artifactRepo == nil || state == nil || state.run == nil {
		return nil, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, err
	}
	latest, err := workerpkg.LatestTranscriptSummaryCheckpoint(artifacts)
	if err != nil {
		return nil, err
	}

	next := workerpkg.BuildTranscriptSummaryCheckpoint(messages)
	if next == nil {
		return latest, nil
	}
	if latest != nil && latest.CoveredThroughSequenceNo >= next.CoveredThroughSequenceNo {
		return latest, nil
	}

	if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, workerpkg.TranscriptSummaryArtifactType, "json", next, buildAssistantSequenceArtifactMetadata(lastAssistantSequenceNoUpTo(messages, next.CoveredThroughSequenceNo))); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "saved transcript summary checkpoint",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"covered_through_sequence_no", next.CoveredThroughSequenceNo,
		"source_message_count", next.SourceMessageCount,
	)
	return next, nil
}

func renderArtifactContextContent(format string, raw json.RawMessage) (string, error) {
	switch strings.TrimSpace(format) {
	case workerpkg.PreviewFormatMarkdown:
		var markdown string
		if err := json.Unmarshal(raw, &markdown); err != nil {
			return "", fmt.Errorf("parse markdown artifact content: %w", err)
		}
		return strings.TrimSpace(markdown), nil
	case workerpkg.PreviewFormatJSON:
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("parse json artifact content: %w", err)
		}
		pretty, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return "", fmt.Errorf("format json artifact content: %w", err)
		}
		return string(pretty), nil
	default:
		return strings.TrimSpace(string(raw)), nil
	}
}

func shouldIncludeRunMessageInExecutionHistory(message model.AgentRunMessage) bool {
	return strings.TrimSpace(message.MessageType) != "status"
}

func hasExecutionHistoryMessages(messages []model.AgentRunMessage) bool {
	for _, message := range messages {
		if shouldIncludeRunMessageInExecutionHistory(message) {
			return true
		}
	}
	return false
}

func (a *AgentRunActivities) ensureRunBootstrapStatusMessage(ctx context.Context, run *model.AgentRun, content string) error {
	if a.runMessageRepo == nil || run == nil || strings.TrimSpace(content) == "" {
		return nil
	}
	messages, err := a.runMessageRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	if len(messages) > 0 {
		return nil
	}
	_, err = a.createRunMessage(ctx, run, "assistant", "status", strings.TrimSpace(content), nil, nil, nil, nil)
	return err
}

func (a *AgentRunActivities) persistAssistantRunMessage(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext) (*model.AgentRunMessage, error) {
	if a.runMessageRepo == nil || execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil, nil
	}
	result := execCtx.LastExecutionResult
	snapshot, err := a.loadCodingSessionStreamSnapshot(ctx, state.run)
	if err != nil {
		return nil, err
	}
	assistantMessageInput, err := buildPersistedAssistantRunMessage(result, snapshot)
	if err != nil {
		return nil, err
	}
	if assistantMessageInput == nil {
		return nil, nil
	}

	assistantMessage, err := a.createRunMessage(
		ctx,
		state.run,
		assistantMessageInput.Role,
		assistantMessageInput.MessageType,
		assistantMessageInput.Content,
		assistantMessageInput.ContentBlocks,
		assistantMessageInput.TurnSegments,
		assistantMessageInput.ToolInvocations,
		assistantMessageInput.TokenUsage,
	)
	if err != nil {
		return nil, err
	}
	if err := a.persistProviderResponseCheckpoint(ctx, state, result, assistantMessage); err != nil {
		return nil, err
	}
	if err := a.persistHumanInteractionArtifacts(ctx, state, result, assistantMessage); err != nil {
		return nil, err
	}
	if err := a.persistNativeTurnDebugArtifact(ctx, state, execCtx, assistantMessage); err != nil {
		return nil, err
	}
	if err := a.persistNativeRepairStateArtifact(ctx, state, execCtx, assistantMessage); err != nil {
		return nil, err
	}

	for _, toolMessage := range buildPersistedToolResultMessages(result.Messages) {
		if _, err := a.createRunMessage(ctx, state.run, toolMessage.Role, toolMessage.MessageType, toolMessage.Content, toolMessage.ContentBlocks, nil, nil, nil); err != nil {
			return nil, err
		}
	}

	return assistantMessage, nil
}

func (a *AgentRunActivities) persistNativeTurnDebugArtifact(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, assistantMessage *model.AgentRunMessage) error {
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || execCtx == nil || assistantMessage == nil {
		return nil
	}
	runtimeKind := executionRuntimeKind(state)
	if !state.nativeSelectivePathEnabled || !execCtx.NativeSelectivePathEnabled || strings.TrimSpace(runtimeKind) != "native_sdk" {
		return nil
	}

	payload := nativeTurnDebugArtifact{
		RuntimeKind:                runtimeKind,
		NativeSelectivePathEnabled: execCtx.NativeSelectivePathEnabled,
		PlanningStage:              strings.TrimSpace(execCtx.PlanningStage),
		ContinuationMode:           providerContinuationMode(execCtx.ProviderContinuation),
		RuntimeSkillRefs:           runtimeSkillRefKeys(execCtx.RuntimeSkillRefs),
		ActiveSkillRefs:            runtimeSkillRefKeys(execCtx.ActiveRuntimeSkillRefs),
		RequiredInteractions:       sortedCompletionInteractionKinds(completionRequiredInteractionKinds(execCtx.SkillPolicy)),
		RepairGuidancePresent:      strings.TrimSpace(execCtx.RepairGuidance) != "",
		RepairGuidanceSource:       strings.TrimSpace(execCtx.RepairGuidanceSource),
		RepairGuidanceClass:        strings.TrimSpace(execCtx.RepairGuidanceClass),
	}

	_, err := a.appendRunArtifactWithMetadata(
		ctx,
		state.run,
		model.AgentRunArtifactTypeNativeTurnDebug,
		"json",
		payload,
		buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo),
	)
	return err
}

func (a *AgentRunActivities) persistNativeRepairStateArtifact(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, assistantMessage *model.AgentRunMessage) error {
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || execCtx == nil || execCtx.LastExecutionResult == nil || assistantMessage == nil {
		return nil
	}
	runtimeKind := executionRuntimeKind(state)
	if !state.nativeSelectivePathEnabled || !execCtx.NativeSelectivePathEnabled || strings.TrimSpace(runtimeKind) != "native_sdk" {
		return nil
	}
	failure := latestExecutionToolFailure(execCtx.LastExecutionResult.Messages)
	if failure == nil {
		return nil
	}
	repair := classifyNativeToolFailureRepair(state, failure)
	if strings.TrimSpace(repair.Class) == "" || strings.TrimSpace(repair.Instructions) == "" {
		return nil
	}
	payload := model.NativeRepairState{
		Source:       "tool_failure",
		RepairClass:  strings.TrimSpace(repair.Class),
		ToolName:     strings.TrimSpace(failure.ToolName),
		RepairHint:   strings.TrimSpace(repair.Instructions),
		ErrorSummary: strings.TrimSpace(failure.Output),
	}
	_, err := a.appendRunArtifactWithMetadata(
		ctx,
		state.run,
		model.AgentRunArtifactTypeNativeRepairState,
		"json",
		payload,
		buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo),
	)
	return err
}

type persistedRunMessageInput struct {
	Role            string
	MessageType     string
	Content         string
	ContentBlocks   json.RawMessage
	TurnSegments    json.RawMessage
	ToolInvocations json.RawMessage
	TokenUsage      json.RawMessage
}

func buildPersistedAssistantRunMessage(result *workerpkg.ExecutionResult, snapshot *model.CodingSessionStreamSnapshot) (*persistedRunMessageInput, error) {
	if result == nil {
		return nil, nil
	}
	content := persistedMessageContent(strings.TrimSpace(result.AssistantText), result.AssistantBlocks)
	if content == "" && len(result.AssistantBlocks) == 0 {
		return nil, nil
	}

	blocks, err := marshalExecutionBlocks(result.AssistantBlocks)
	if err != nil {
		return nil, fmt.Errorf("marshal assistant blocks: %w", err)
	}
	turnSegments, err := marshalCodingSessionTurnSegments(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal turn segments: %w", err)
	}
	invocations, err := marshalToolInvocations(result.ToolInvocations)
	if err != nil {
		return nil, fmt.Errorf("marshal tool invocations: %w", err)
	}
	usagePayload, err := marshalTokenUsage(result.Usage)
	if err != nil {
		return nil, fmt.Errorf("marshal usage: %w", err)
	}

	return &persistedRunMessageInput{
		Role:            "assistant",
		MessageType:     "assistant_turn",
		Content:         content,
		ContentBlocks:   blocks,
		TurnSegments:    turnSegments,
		ToolInvocations: invocations,
		TokenUsage:      usagePayload,
	}, nil
}

func buildPersistedToolResultMessages(messages []workerpkg.ExecutionMessage) []persistedRunMessageInput {
	toolMessages := finalRoundToolMessages(messages)
	results := make([]persistedRunMessageInput, 0, len(toolMessages))
	for _, toolMessage := range toolMessages {
		results = append(results, persistedRunMessageInput{
			Role:          "tool",
			MessageType:   "tool_result",
			Content:       persistedMessageContent(strings.TrimSpace(toolMessage.Content), toolMessage.Blocks),
			ContentBlocks: mustMarshalExecutionBlocks(toolMessage.Blocks),
		})
	}
	return results
}

func buildAssistantSequenceArtifactMetadata(sequenceNo int) json.RawMessage {
	if sequenceNo <= 0 {
		return json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(map[string]any{
		"assistant_message_sequence_no": sequenceNo,
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func artifactAssistantMessageSequenceNo(artifact model.AgentRunArtifact) int {
	if len(artifact.Metadata) == 0 || string(artifact.Metadata) == "null" {
		return 0
	}
	var metadata struct {
		AssistantMessageSequenceNo int `json:"assistant_message_sequence_no"`
	}
	if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
		return 0
	}
	return metadata.AssistantMessageSequenceNo
}

func normalizeApprovalPreviewPanelKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mergeArtifactMetadata(parts ...json.RawMessage) json.RawMessage {
	merged := map[string]any{}
	for _, part := range parts {
		if len(part) == 0 || string(part) == "null" {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(part, &decoded); err != nil {
			continue
		}
		for key, value := range decoded {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func marshalExecutionBlocks(blocks []workerpkg.ExecutionBlock) (json.RawMessage, error) {
	if len(blocks) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(workerpkg.NormalizeExecutionBlocks(blocks))
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func marshalCodingSessionTurnSegments(snapshot *model.CodingSessionStreamSnapshot) (json.RawMessage, error) {
	if snapshot == nil || len(snapshot.LiveTurnSegments) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(snapshot.LiveTurnSegments)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func mustMarshalExecutionBlocks(blocks []workerpkg.ExecutionBlock) json.RawMessage {
	payload, err := marshalExecutionBlocks(blocks)
	if err != nil {
		return nil
	}
	return payload
}

func marshalToolInvocations(invocations []model.ToolInvocation) (json.RawMessage, error) {
	if len(invocations) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(invocations)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func marshalTokenUsage(usage workerpkg.ExecutionUsage) (json.RawMessage, error) {
	return json.Marshal(map[string]int{
		"cached_input_tokens": usage.CachedInputTokens,
		"input_tokens":        usage.InputTokens,
		"output_tokens":       usage.OutputTokens,
	})
}

func persistedMessageContent(fallback string, blocks []workerpkg.ExecutionBlock) string {
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	text := strings.TrimSpace(workerpkg.ExtractPersistedContentFromExecutionBlocks(blocks))
	if text != "" {
		return text
	}
	return ""
}

func finalRoundToolMessages(messages []workerpkg.ExecutionMessage) []workerpkg.ExecutionMessage {
	lastAssistantIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			lastAssistantIndex = i
			break
		}
	}
	if lastAssistantIndex == -1 || lastAssistantIndex >= len(messages)-1 {
		return nil
	}

	results := make([]workerpkg.ExecutionMessage, 0, len(messages)-lastAssistantIndex-1)
	for _, message := range messages[lastAssistantIndex+1:] {
		if message.Role != "tool" {
			continue
		}
		results = append(results, message)
	}
	return results
}

func (a *AgentRunActivities) persistProviderResponseCheckpoint(ctx context.Context, state *resolvedRunState, result *workerpkg.ExecutionResult, assistantMessage *model.AgentRunMessage) error {
	if a.artifactRepo == nil || state == nil || state.run == nil || state.agent == nil || result == nil || assistantMessage == nil || result.ProviderContinuation == nil {
		return nil
	}
	if strings.TrimSpace(result.ProviderContinuation.ResponseID) == "" {
		return nil
	}

	provider := strings.TrimSpace(derefString(state.agent.Provider))
	if provider == "" {
		provider = strings.TrimSpace(result.ProviderContinuation.Provider)
	}

	_, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeProviderResponseCheckpoint, "json", model.ProviderResponseCheckpoint{
		Provider:              provider,
		ResponseID:            strings.TrimSpace(result.ProviderContinuation.ResponseID),
		PreviousResponseID:    strings.TrimSpace(result.ProviderContinuation.PreviousResponseID),
		AssistantMessageSeqNo: assistantMessage.SequenceNo,
		RecordedAt:            time.Now().UTC(),
	}, buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo))
	if err == nil {
		slog.InfoContext(ctx, "saved provider continuation checkpoint",
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"provider", provider,
			"response_id", strings.TrimSpace(result.ProviderContinuation.ResponseID),
			"assistant_sequence_no", assistantMessage.SequenceNo,
		)
	}
	return err
}

func (a *AgentRunActivities) persistHumanInteractionArtifacts(ctx context.Context, state *resolvedRunState, result *workerpkg.ExecutionResult, assistantMessage *model.AgentRunMessage) error {
	if (a.artifactRepo == nil && a.interactionRepo == nil) || state == nil || state.run == nil || result == nil || assistantMessage == nil {
		return nil
	}

	metadata := buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo)

	if inputRequest := latestHumanInputRequestFromResult(result); inputRequest != nil {
		inputMetadata := mergeArtifactMetadata(metadata, result.HumanInputMetadata)
		artifactPayload := humanInputArtifactFromWorker(inputRequest)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanInputRequest, "json", artifactPayload, inputMetadata); err != nil {
				return err
			}
		}
		interaction, err := a.persistHumanInputInteraction(ctx, state, inputRequest, inputMetadata, assistantMessage.SequenceNo)
		if err != nil {
			return err
		}
		if interaction != nil {
			a.maybeNotifyAgentAttentionRequired(ctx, state, interaction)
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "input.requested", map[string]any{
				"content": artifactPayload,
			})
		}
	}

	if reviewRequest := latestHumanReviewCheckpointRequestFromResult(result); reviewRequest != nil {
		approvalMetadata := mergeArtifactMetadata(metadata, result.HumanApprovalMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", reviewRequest, approvalMetadata); err != nil {
				return err
			}
			if reviewFindings := reviewFindingsArtifactFromReviewCheckpointRequest(reviewRequest, assistantMessage.SequenceNo); reviewFindings != nil {
				if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeReviewFindings, "json", reviewFindings, approvalMetadata); err != nil {
					return err
				}
			}
		}
		interaction, err := a.persistHumanApprovalInteraction(ctx, state, model.AgentRunInteractionKindReviewCheckpoint, reviewRequest.Title, reviewRequest.Summary, reviewRequest, approvalMetadata, assistantMessage.SequenceNo)
		if err != nil {
			return err
		}
		if interaction != nil {
			a.maybeNotifyAgentAttentionRequired(ctx, state, interaction)
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "approval.requested", map[string]any{
				"content": reviewRequest,
			})
		}
	}

	if approvalRequest := latestHumanApprovalRequestFromResult(result); approvalRequest != nil {
		approvalMetadata := mergeArtifactMetadata(metadata, result.HumanApprovalMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", approvalRequest, approvalMetadata); err != nil {
				return err
			}
		}
		interaction, err := a.persistHumanApprovalInteraction(ctx, state, model.AgentRunInteractionKindApprovalRequest, approvalRequest.Title, approvalRequest.Summary, approvalRequest, approvalMetadata, assistantMessage.SequenceNo)
		if err != nil {
			return err
		}
		if interaction != nil {
			a.maybeNotifyAgentAttentionRequired(ctx, state, interaction)
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "approval.requested", map[string]any{
				"content": approvalRequest,
			})
		}
	}

	if authState := latestCodexAuthStateFromResult(result); authState != nil {
		authMetadata := mergeArtifactMetadata(metadata, result.CodexAuthMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeCodexAuthState, "json", authState, authMetadata); err != nil {
				return err
			}
		}
		a.publishCodingSessionEvent(state.run, "auth.updated", map[string]any{
			"content": authState,
		})
	}

	if runPlan := latestRunPlanFromResult(result); runPlan != nil {
		runPlanMetadata := mergeArtifactMetadata(metadata, result.RunPlanMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeRunPlan, "json", runPlan, runPlanMetadata); err != nil {
				return err
			}
		}
		a.publishCodingSessionEvent(state.run, "activity.updated", map[string]any{
			"content": runPlan,
		})
	}

	return nil
}

func (a *AgentRunActivities) maybeNotifyAgentAttentionRequired(ctx context.Context, state *resolvedRunState, interaction *model.AgentRunInteraction) {
	if a == nil || a.notificationEmitter == nil || state == nil || state.run == nil || interaction == nil {
		return
	}
	if strings.TrimSpace(interaction.Status) != model.AgentRunInteractionStatusPending {
		return
	}
	if strings.TrimSpace(state.run.TargetType) != "task" || strings.TrimSpace(state.run.TargetID) == "" {
		return
	}
	task, recipients, err := a.loadTaskAgentAttentionNotificationTarget(ctx, state.run.TargetID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to resolve task attention notification target",
			"error", err,
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"task_id", state.run.TargetID,
		)
		return
	}
	if task == nil || len(recipients) == 0 {
		return
	}

	attentionType := "input"
	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindCommandExecutionApproval,
		model.AgentRunInteractionKindFileChangeApproval,
		model.AgentRunInteractionKindPermissionsApproval,
		model.AgentRunInteractionKindReviewCheckpoint:
		attentionType = "approval"
	}

	title := fmt.Sprintf("%s is waiting for %s on %s", a.agentAttentionAgentName(state), attentionType, strings.TrimSpace(task.Name))
	body := strings.TrimSpace(derefString(interaction.Summary))
	if body == "" {
		body = fmt.Sprintf("Open the task run to respond to the pending %s request.", attentionType)
	}

	teamID := ""
	if task.TeamID != nil {
		teamID = strings.TrimSpace(*task.TeamID)
	}

	if err := a.notificationEmitter.Emit(ctx, model.NotificationEventInput{
		WorkspaceID: state.run.WorkspaceID,
		EventType:   "task.agent_attention_required",
		EntityType:  "agent_run",
		EntityID:    state.run.ID,
		Title:       title,
		Body:        body,
		Category:    model.NotifCategoryAgentAttention,
		Priority:    "high",
		TeamID:      teamID,
		Metadata: model.JSONB{
			"run_id":           state.run.ID,
			"task_id":          task.ID,
			"target_type":      state.run.TargetType,
			"target_id":        state.run.TargetID,
			"interaction_id":   interaction.ID,
			"interaction_kind": interaction.InteractionKind,
			"pause_reason":     agentAttentionPauseReason(interaction),
			"agent_id":         strings.TrimSpace(state.run.AgentID),
			"agent_name":       a.agentAttentionAgentName(state),
		},
		EntitySnapshot: model.JSONB{
			"title":      task.Name,
			"identifier": taskAttentionDisplayIdentifier(task),
			"type":       "task",
		},
		ParentEntitySnapshot: model.JSONB{
			"type":       "task",
			"id":         task.ID,
			"title":      task.Name,
			"identifier": taskAttentionDisplayIdentifier(task),
		},
		ExplicitRecipients: recipients,
		SkipFollowers:      true,
		SkipEmailDelivery:  true,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to emit task agent attention notification",
			"error", err,
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"task_id", task.ID,
			"recipient_count", len(recipients),
		)
	}
}

func (a *AgentRunActivities) loadTaskAgentAttentionNotificationTarget(ctx context.Context, taskID string) (*model.PMTask, []string, error) {
	if a == nil || a.taskRepo == nil {
		return nil, nil, nil
	}
	task, err := a.taskRepo.GetRawByID(ctx, taskID)
	if err != nil || task == nil {
		return task, nil, err
	}

	if ownerID := strings.TrimSpace(derefString(task.OwnerID)); ownerID != "" {
		return task, []string{ownerID}, nil
	}

	ownerUserIDs, err := a.taskRepo.ListOwnerUserIDs(ctx, task.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(ownerUserIDs) > 0 {
		return task, ownerUserIDs, nil
	}

	teamID := strings.TrimSpace(derefString(task.TeamID))
	if teamID == "" || a.workspaceRepo == nil {
		return task, nil, nil
	}

	teamUserIDs, err := a.workspaceRepo.ListActiveTeamUserIDs(ctx, task.WorkspaceID, teamID)
	if err != nil {
		return nil, nil, err
	}
	return task, teamUserIDs, nil
}

func (a *AgentRunActivities) agentAttentionAgentName(state *resolvedRunState) string {
	if state != nil && state.agent != nil {
		if name := strings.TrimSpace(state.agent.Name); name != "" {
			return name
		}
	}
	return "Agent"
}

func agentAttentionPauseReason(interaction *model.AgentRunInteraction) string {
	if interaction == nil {
		return model.AgentRunPauseReasonHumanInput
	}
	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindCommandExecutionApproval,
		model.AgentRunInteractionKindFileChangeApproval,
		model.AgentRunInteractionKindPermissionsApproval,
		model.AgentRunInteractionKindReviewCheckpoint:
		return model.AgentRunPauseReasonHumanApproval
	default:
		return model.AgentRunPauseReasonHumanInput
	}
}

func taskAttentionDisplayIdentifier(task *model.PMTask) string {
	if task == nil || task.DisplayID <= 0 {
		return ""
	}
	return "#" + strconv.Itoa(task.DisplayID)
}

type interactionRuntimeMetadata struct {
	AssistantMessageSequenceNo int             `json:"assistant_message_sequence_no,omitempty"`
	RuntimeKind                string          `json:"runtime_kind,omitempty"`
	CodexRequestKind           string          `json:"codex_request_kind,omitempty"`
	CodexRequestID             string          `json:"codex_request_id,omitempty"`
	CodexThreadID              string          `json:"codex_thread_id,omitempty"`
	CodexTurnID                string          `json:"codex_turn_id,omitempty"`
	CodexItemID                string          `json:"codex_item_id,omitempty"`
	CodexApprovalID            *string         `json:"codex_approval_id,omitempty"`
	CodexRequestPayload        json.RawMessage `json:"codex_request_payload,omitempty"`
}

func (a *AgentRunActivities) persistHumanInputInteraction(ctx context.Context, state *resolvedRunState, inputRequest *workerpkg.UserInputRequest, metadata json.RawMessage, assistantSequenceNo int) (*model.AgentRunInteraction, error) {
	if a.interactionRepo == nil || state == nil || state.run == nil || inputRequest == nil {
		return nil, nil
	}

	runtimeMetadata := decodeInteractionRuntimeMetadata(metadata)
	interaction := &model.AgentRunInteraction{
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                firstNonEmptyString(strings.TrimSpace(runtimeMetadata.RuntimeKind), executionRuntimeKind(state), strings.TrimSpace(state.run.RuntimeKind)),
		InteractionKind:            model.AgentRunInteractionKindRequestUserInput,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionCodexV2,
		RequestPayload:             mustMarshalJSON(inputRequest),
		RuntimeMetadata:            defaultInteractionRuntimeMetadata(metadata),
		AssistantMessageSequenceNo: intPtrIfPositive(firstPositiveInt(runtimeMetadata.AssistantMessageSequenceNo, assistantSequenceNo)),
		Title:                      strPtr("User input required"),
	}

	if interaction.RuntimeKind == "codex" && len(runtimeMetadata.CodexRequestPayload) > 0 {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionCodexV2
		interaction.RequestPayload = copyRawJSON(runtimeMetadata.CodexRequestPayload)
		interaction.RequestID = strPtrIfNotEmpty(runtimeMetadata.CodexRequestID)
		interaction.ThreadID = strPtrIfNotEmpty(runtimeMetadata.CodexThreadID)
		interaction.TurnID = strPtrIfNotEmpty(runtimeMetadata.CodexTurnID)
		interaction.ItemID = strPtrIfNotEmpty(runtimeMetadata.CodexItemID)
	}

	interaction.Summary = strPtrIfNotEmpty(workerpkg.UserInputSummary(inputRequest))

	if err := a.appendRunInteraction(ctx, interaction); err != nil {
		return nil, err
	}
	return interaction, nil
}

func (a *AgentRunActivities) persistHumanApprovalInteraction(ctx context.Context, state *resolvedRunState, interactionKind, title, summary string, requestPayload any, metadata json.RawMessage, assistantSequenceNo int) (*model.AgentRunInteraction, error) {
	if a.interactionRepo == nil || state == nil || state.run == nil || requestPayload == nil {
		return nil, nil
	}

	runtimeMetadata := decodeInteractionRuntimeMetadata(metadata)
	interaction := &model.AgentRunInteraction{
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                firstNonEmptyString(strings.TrimSpace(runtimeMetadata.RuntimeKind), executionRuntimeKind(state), strings.TrimSpace(state.run.RuntimeKind)),
		InteractionKind:            strings.TrimSpace(interactionKind),
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload:             mustMarshalJSON(requestPayload),
		RuntimeMetadata:            defaultInteractionRuntimeMetadata(metadata),
		AssistantMessageSequenceNo: intPtrIfPositive(firstPositiveInt(runtimeMetadata.AssistantMessageSequenceNo, assistantSequenceNo)),
		Title:                      strPtrIfNotEmpty(strings.TrimSpace(title)),
		Summary:                    strPtrIfNotEmpty(strings.TrimSpace(summary)),
	}

	if interaction.RuntimeKind == "codex" && len(runtimeMetadata.CodexRequestPayload) > 0 {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionCodexV2
		interaction.RequestPayload = copyRawJSON(runtimeMetadata.CodexRequestPayload)
		interaction.RequestID = strPtrIfNotEmpty(runtimeMetadata.CodexRequestID)
		interaction.ThreadID = strPtrIfNotEmpty(runtimeMetadata.CodexThreadID)
		interaction.TurnID = strPtrIfNotEmpty(runtimeMetadata.CodexTurnID)
		interaction.ItemID = strPtrIfNotEmpty(runtimeMetadata.CodexItemID)
		interaction.ApprovalID = runtimeMetadata.CodexApprovalID
		switch strings.TrimSpace(runtimeMetadata.CodexRequestKind) {
		case "command_execution":
			interaction.InteractionKind = model.AgentRunInteractionKindCommandExecutionApproval
		case "file_change":
			interaction.InteractionKind = model.AgentRunInteractionKindFileChangeApproval
		case "permissions":
			interaction.InteractionKind = model.AgentRunInteractionKindPermissionsApproval
		}
	}

	if err := a.appendRunInteraction(ctx, interaction); err != nil {
		return nil, err
	}
	return interaction, nil
}

func (a *AgentRunActivities) appendRunInteraction(ctx context.Context, interaction *model.AgentRunInteraction) error {
	if a.interactionRepo == nil || interaction == nil {
		return nil
	}
	if len(interaction.RequestPayload) == 0 {
		interaction.RequestPayload = json.RawMessage(`{}`)
	}
	if len(interaction.RuntimeMetadata) == 0 {
		interaction.RuntimeMetadata = json.RawMessage(`{}`)
	}
	if strings.TrimSpace(interaction.Status) == "" {
		interaction.Status = model.AgentRunInteractionStatusPending
	}
	if strings.TrimSpace(interaction.RequestSchemaVersion) == "" {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionHelpinV1
	}
	return a.interactionRepo.Create(ctx, interaction)
}

func decodeInteractionRuntimeMetadata(raw json.RawMessage) interactionRuntimeMetadata {
	var metadata interactionRuntimeMetadata
	if len(raw) == 0 || string(raw) == "null" {
		return metadata
	}
	_ = json.Unmarshal(raw, &metadata)
	return metadata
}

func defaultInteractionRuntimeMetadata(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return copyRawJSON(raw)
}

func mustMarshalJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func copyRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func intPtrIfPositive(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func strPtrIfNotEmpty(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func latestHumanApprovalRequestFromResult(result *workerpkg.ExecutionResult) *model.ApprovalRequest {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestApprovalRequest(result.ToolInvocations)
}

func latestHumanReviewCheckpointRequestFromResult(result *workerpkg.ExecutionResult) *model.ReviewCheckpointRequest {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestReviewCheckpointRequest(result.ToolInvocations)
}

func reviewFindingsArtifactFromReviewCheckpointRequest(reviewRequest *model.ReviewCheckpointRequest, assistantSequenceNo int) *model.ReviewFindingsArtifact {
	if reviewRequest == nil {
		return nil
	}
	if len(reviewRequest.Findings) == 0 && strings.TrimSpace(reviewRequest.OverallCorrectness) == "" && strings.TrimSpace(reviewRequest.OverallExplanation) == "" && reviewRequest.OverallConfidenceScore == nil {
		return nil
	}
	return &model.ReviewFindingsArtifact{
		Phase:                      strings.TrimSpace(reviewRequest.Phase),
		Title:                      strings.TrimSpace(reviewRequest.Title),
		Summary:                    strings.TrimSpace(reviewRequest.Summary),
		Findings:                   slices.Clone(reviewRequest.Findings),
		OverallCorrectness:         strings.TrimSpace(reviewRequest.OverallCorrectness),
		OverallExplanation:         strings.TrimSpace(reviewRequest.OverallExplanation),
		OverallConfidenceScore:     reviewRequest.OverallConfidenceScore,
		AssistantMessageSequenceNo: assistantSequenceNo,
		RecordedAt:                 time.Now().UTC(),
	}
}

func latestHumanInputRequestFromResult(result *workerpkg.ExecutionResult) *workerpkg.UserInputRequest {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanInputRequest(result.ToolInvocations)
}

func latestCodexAuthStateFromResult(result *workerpkg.ExecutionResult) *model.CodexAuthState {
	if result == nil || result.CodexAuthState == nil {
		return nil
	}
	return result.CodexAuthState
}

func latestRunPlanFromResult(result *workerpkg.ExecutionResult) *workerpkg.RunPlanArtifact {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestRunPlan(result.ToolInvocations)
}

func humanInputArtifactFromWorker(req *workerpkg.UserInputRequest) model.HumanInputArtifact {
	return workerpkg.HumanInputArtifactFromUserInputRequest(req)
}

func (a *AgentRunActivities) createRunMessage(ctx context.Context, run *model.AgentRun, role, messageType, content string, blocks, turnSegments, toolInvocations, tokenUsage json.RawMessage) (*model.AgentRunMessage, error) {
	if a.runMessageRepo == nil {
		return nil, nil
	}
	sequenceNo, err := a.runMessageRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	message := &model.AgentRunMessage{
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		Role:            role,
		Content:         content,
		MessageType:     messageType,
		ContentBlocks:   blocks,
		TurnSegments:    turnSegments,
		ToolInvocations: toolInvocations,
		TokenUsage:      tokenUsage,
		SequenceNo:      sequenceNo,
	}
	if err := a.runMessageRepo.Create(ctx, message); err != nil {
		return nil, err
	}
	if shouldClearCodingSessionStreamSnapshot(role, messageType) {
		if err := a.clearCodingSessionStreamSnapshot(ctx, run); err != nil {
			slog.WarnContext(ctx, "clear coding session stream snapshot failed",
				"run_id", run.ID,
				"workspace_id", run.WorkspaceID,
				"error", err,
			)
		}
	}
	a.publishRunMessageEvent(run, message)
	return message, nil
}

func (a *AgentRunActivities) publishRunMessageEvent(run *model.AgentRun, message *model.AgentRunMessage) {
	if a.wsPublisher == nil || run == nil || message == nil {
		return
	}
	data, _ := json.Marshal(message)
	a.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "agent_run_message",
		EntityID:    message.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  "agent_run",
		ParentID:    run.ID,
		Data:        data,
	})
	eventType := "user.message.completed"
	switch strings.TrimSpace(message.Role) {
	case "assistant":
		eventType = "assistant.message.completed"
	case "tool":
		eventType = "tool.call.completed"
	}
	a.publishCodingSessionEvent(run, eventType, map[string]any{
		"message_id":       message.ID,
		"role":             message.Role,
		"message_type":     message.MessageType,
		"content":          message.Content,
		"sequence_no":      message.SequenceNo,
		"content_blocks":   json.RawMessage(message.ContentBlocks),
		"turn_segments":    json.RawMessage(message.TurnSegments),
		"tool_invocations": json.RawMessage(message.ToolInvocations),
	})
}

func (a *AgentRunActivities) publishRunStreamEvent(run *model.AgentRun, event workerpkg.ExecutionEvent) {
	if run == nil {
		return
	}

	sentAt := time.Now().UTC()
	codingEventType := codingSessionEventTypeFromExecutionEvent(event)
	codingPayload := map[string]any{
		"message_id":        strings.TrimSpace(event.MessageID),
		"parent_message_id": strings.TrimSpace(event.ParentMessageID),
		"result_message_id": strings.TrimSpace(event.ResultMessageID),
		"text":              event.Text,
		"content":           coalesceRaw(event.Content, event.Text),
		"tool_call_id":      event.ToolCallID,
		"tool_name":         event.ToolName,
		"tool_input":        event.ToolInput,
		"args_delta":        event.ArgsDelta,
		"args_text":         event.ArgsText,
		"activity_id":       event.ActivityID,
		"activity_type":     event.ActivityType,
		"encrypted_value":   event.EncryptedValue,
		"output_summary":    event.OutputSummary,
		"duration_ms":       event.DurationMs,
		"error":             event.Error,
	}
	if codingEventType != "" {
		a.persistCodingSessionStreamSnapshot(context.Background(), run, codingEventType, codingPayload, sentAt)
	}
	if a.wsPublisher == nil {
		return
	}

	payload, err := json.Marshal(model.AgentRunStreamEvent{
		SentAt:          sentAt,
		Type:            event.Type,
		RunID:           run.ID,
		MessageID:       event.MessageID,
		ParentMessageID: event.ParentMessageID,
		ResultMessageID: event.ResultMessageID,
		Text:            event.Text,
		Content:         event.Content,
		ToolCallID:      event.ToolCallID,
		ToolName:        event.ToolName,
		ToolInput:       event.ToolInput,
		ArgsDelta:       event.ArgsDelta,
		ArgsText:        event.ArgsText,
		ActivityID:      event.ActivityID,
		ActivityType:    event.ActivityType,
		EncryptedValue:  event.EncryptedValue,
		OutputSummary:   event.OutputSummary,
		DurationMs:      event.DurationMs,
		Error:           event.Error,
	})
	if err != nil {
		return
	}

	a.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "agent_run_stream",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  "agent_run",
		ParentID:    run.ID,
		Data:        payload,
	})
	if codingEventType != "" {
		a.publishCodingSessionEvent(run, codingEventType, codingPayload)
	}
}

func shouldClearCodingSessionStreamSnapshot(role, messageType string) bool {
	return strings.TrimSpace(role) == "assistant" && strings.TrimSpace(messageType) == "assistant_turn"
}

func (a *AgentRunActivities) clearCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun) error {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil {
		return nil
	}
	return a.sessionSnapshotRepo.DeleteByRun(ctx, run.WorkspaceID, run.ID)
}

func (a *AgentRunActivities) loadCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun) (*model.CodingSessionStreamSnapshot, error) {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil {
		return nil, nil
	}
	record, err := a.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil || record == nil {
		return nil, err
	}
	return model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
}

func (a *AgentRunActivities) persistCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun, eventType string, payload map[string]any, timestamp time.Time) {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil || strings.TrimSpace(eventType) == "" {
		return
	}

	record, err := a.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		slog.WarnContext(ctx, "load coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
		return
	}

	var snapshot *model.CodingSessionStreamSnapshot
	if record != nil {
		snapshot, err = model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
		if err != nil {
			slog.WarnContext(ctx, "decode coding session stream snapshot failed",
				"run_id", run.ID,
				"workspace_id", run.WorkspaceID,
				"error", err,
			)
			record = nil
		}
	}

	snapshot = model.ApplyCodingSessionStreamEvent(snapshot, eventType, payload, timestamp)
	if snapshot == nil || snapshot.IsEmpty() {
		if record != nil {
			if err := a.sessionSnapshotRepo.DeleteByRun(ctx, run.WorkspaceID, run.ID); err != nil {
				slog.WarnContext(ctx, "delete empty coding session stream snapshot failed",
					"run_id", run.ID,
					"workspace_id", run.WorkspaceID,
					"error", err,
				)
			}
		}
		return
	}

	encoded, err := model.EncodeCodingSessionStreamSnapshot(snapshot)
	if err != nil {
		slog.WarnContext(ctx, "encode coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
		return
	}

	nextRecord := &model.CodingSessionStateSnapshot{
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		SchemaVersion:   model.CodingSessionStateSnapshotSchemaVersionV1,
		SnapshotPayload: encoded,
	}
	if record != nil {
		nextRecord.ID = record.ID
		nextRecord.CreatedAt = record.CreatedAt
	}
	if err := a.sessionSnapshotRepo.Upsert(ctx, nextRecord); err != nil {
		slog.WarnContext(ctx, "persist coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
	}
}

func (a *AgentRunActivities) publishCodingSessionEvent(run *model.AgentRun, eventType string, payload map[string]any) {
	if a.wsPublisher == nil || run == nil || strings.TrimSpace(eventType) == "" {
		return
	}
	envelope, _ := json.Marshal(model.CodingSessionEvent{
		ID:          fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano()),
		SessionID:   run.ID,
		RunID:       run.ID,
		SequenceNo:  int(time.Now().UTC().UnixMilli()),
		Timestamp:   time.Now().UTC(),
		Type:        eventType,
		RuntimeKind: run.RuntimeKind,
		Payload:     payload,
	})
	a.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "coding_session_event",
		EntityID:    fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano()),
		WorkspaceID: run.WorkspaceID,
		ParentType:  "coding_session",
		ParentID:    run.ID,
		Data:        envelope,
	})
}

func (a *AgentRunActivities) publishCodingSessionInteractionEvent(run *model.AgentRun, interaction *model.AgentRunInteraction) {
	if interaction == nil {
		return
	}
	eventType, payload := codingSessionInteractionEventPayload(interaction)
	a.publishCodingSessionEvent(run, eventType, payload)
}

func codingSessionInteractionEventPayload(interaction *model.AgentRunInteraction) (string, map[string]any) {
	if interaction == nil {
		return "", nil
	}

	eventType := "interaction.updated"
	switch strings.TrimSpace(interaction.Status) {
	case model.AgentRunInteractionStatusPending:
		eventType = "interaction.requested"
	case model.AgentRunInteractionStatusResolved:
		eventType = "interaction.resolved"
	case model.AgentRunInteractionStatusCancelled:
		eventType = "interaction.cancelled"
	}

	payload := map[string]any{
		"interaction_id":         interaction.ID,
		"interaction_kind":       interaction.InteractionKind,
		"status":                 interaction.Status,
		"request_schema_version": interaction.RequestSchemaVersion,
		"request_payload":        json.RawMessage(interaction.RequestPayload),
		"title":                  derefString(interaction.Title),
		"summary":                derefString(interaction.Summary),
		"request_id":             derefString(interaction.RequestID),
		"thread_id":              derefString(interaction.ThreadID),
		"turn_id":                derefString(interaction.TurnID),
		"item_id":                derefString(interaction.ItemID),
		"approval_id":            derefString(interaction.ApprovalID),
	}
	if interaction.AssistantMessageSequenceNo != nil {
		payload["assistant_message_sequence_no"] = *interaction.AssistantMessageSequenceNo
	}
	if interaction.ResponseSchemaVersion != nil && strings.TrimSpace(*interaction.ResponseSchemaVersion) != "" {
		payload["response_schema_version"] = strings.TrimSpace(*interaction.ResponseSchemaVersion)
	}
	if len(interaction.ResponsePayload) > 0 && string(interaction.ResponsePayload) != "null" {
		payload["response_payload"] = json.RawMessage(interaction.ResponsePayload)
	}
	if interaction.ResolvedAt != nil {
		payload["resolved_at"] = interaction.ResolvedAt.UTC()
	}
	if interaction.ResolvedBy != nil && strings.TrimSpace(*interaction.ResolvedBy) != "" {
		payload["resolved_by"] = strings.TrimSpace(*interaction.ResolvedBy)
	}

	return eventType, payload
}

func codingSessionEventTypeFromExecutionEvent(event workerpkg.ExecutionEvent) string {
	switch strings.TrimSpace(event.Type) {
	case "assistant_message_started":
		return "assistant.message.started"
	case "assistant_message_delta":
		return "assistant.message.delta"
	case "assistant_message_completed":
		return "assistant.message.completed"
	case "reasoning_message_started":
		return "reasoning.message.started"
	case "reasoning_message_delta":
		return "reasoning.message.delta"
	case "reasoning_message_completed":
		return "reasoning.message.completed"
	case "tool_call_started":
		return "tool.call.started"
	case "tool_call_args_delta":
		return "tool.call.args.delta"
	case "tool_call_result":
		return "tool.call.result"
	case "tool_call_finished":
		if strings.TrimSpace(event.Error) != "" {
			return "tool.call.failed"
		}
		return "tool.call.completed"
	case "activity_snapshot":
		return "activity.snapshot"
	case "activity_delta":
		return "activity.delta"
	case "plan_updated":
		return "plan.updated"
	default:
		return ""
	}
}

func latestExecutionApprovalRequest(execCtx *workerpkg.ExecutionContext) *model.ApprovalRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestApprovalRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionReviewCheckpointRequest(execCtx *workerpkg.ExecutionContext) *model.ReviewCheckpointRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestReviewCheckpointRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionHumanInputRequest(execCtx *workerpkg.ExecutionContext) *workerpkg.UserInputRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanInputRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionCodexAuthState(execCtx *workerpkg.ExecutionContext) *model.CodexAuthState {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return execCtx.LastExecutionResult.CodexAuthState
}

func (a *AgentRunActivities) handleLiveCodexInteractivePause(
	ctx context.Context,
	state *resolvedRunState,
	execCtx *workerpkg.ExecutionContext,
	result *workerpkg.ExecutionResult,
	setHeartbeatStage func(string),
) (*workerpkg.LiveExecutionResumeSignal, error) {
	if state == nil || state.run == nil || execCtx == nil || result == nil {
		return nil, fmt.Errorf("live codex pause context is incomplete")
	}

	pauseReason, pauseStage, err := liveCodexPauseState(result)
	if err != nil {
		return nil, err
	}

	execCtx.LastExecutionResult = result
	assistantMessage, err := a.persistAssistantRunMessage(ctx, state, execCtx)
	if err != nil {
		return nil, err
	}
	afterSequenceNo, err := a.currentRunMessageSequence(ctx, state.run)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	state.run.Status = model.AgentRunStatusPaused
	state.run.PauseReason = pauseReason
	state.run.ExecutionStage = strPtr(pauseStage)
	state.run.LastHeartbeatAt = &now
	state.run.CompletedAt = nil
	if pauseReason == model.AgentRunPauseReasonHumanApproval {
		state.run.ApprovalState = "pending"
	} else if state.run.ApprovalState != "approved" {
		state.run.ApprovalState = "not_required"
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return nil, err
	}
	a.runRepo.Notify(ctx, state.run)
	if setHeartbeatStage != nil {
		setHeartbeatStage(pauseStage)
	}

	if assistantMessage != nil && assistantMessage.SequenceNo > afterSequenceNo {
		afterSequenceNo = assistantMessage.SequenceNo
	}
	signal, err := a.waitForLiveCodexResumeSignal(ctx, state.run, afterSequenceNo, pauseReason)
	if err != nil {
		return nil, err
	}
	return &workerpkg.LiveExecutionResumeSignal{
		Intent:          signal.Intent,
		Content:         signal.Content,
		ResponsePayload: copyRawJSON(signal.ResponsePayload),
		Acknowledge: func() error {
			if setHeartbeatStage != nil {
				setHeartbeatStage("codex_running")
			}
			now := time.Now()
			state.run.Status = model.AgentRunStatusRunning
			state.run.PauseReason = model.AgentRunPauseReasonNone
			state.run.ExecutionStage = strPtr(liveCodexResumeStage(signal, pauseReason))
			state.run.LastHeartbeatAt = &now
			state.run.CompletedAt = nil
			switch strings.TrimSpace(signal.Intent) {
			case model.AgentRunResumeIntentApprove:
				state.run.ApprovalState = "approved"
			case model.AgentRunResumeIntentRequestChanges:
				state.run.ApprovalState = "rejected"
			default:
				if pauseReason != model.AgentRunPauseReasonHumanApproval && state.run.ApprovalState != "pending" && state.run.ApprovalState != "rejected" {
					state.run.ApprovalState = "not_required"
				}
			}
			if err := a.runRepo.Update(ctx, state.run); err != nil {
				return err
			}
			a.runRepo.Notify(ctx, state.run)
			return nil
		},
	}, nil
}

func (a *AgentRunActivities) currentRunMessageSequence(ctx context.Context, run *model.AgentRun) (int, error) {
	if a == nil || a.runMessageRepo == nil || run == nil {
		return 0, nil
	}
	nextSequenceNo, err := a.runMessageRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return 0, err
	}
	if nextSequenceNo <= 1 {
		return 0, nil
	}
	return nextSequenceNo - 1, nil
}

func liveCodexPauseState(result *workerpkg.ExecutionResult) (pauseReason, stage string, err error) {
	if reviewRequest := latestHumanReviewCheckpointRequestFromResult(result); reviewRequest != nil {
		stage = "awaiting_review"
		if strings.TrimSpace(reviewRequest.Phase) != "" {
			stage = strings.TrimSpace(reviewRequest.Phase)
		}
		return model.AgentRunPauseReasonHumanApproval, stage, nil
	}
	if approvalRequest := latestHumanApprovalRequestFromResult(result); approvalRequest != nil {
		stage = "awaiting_approval"
		if strings.TrimSpace(approvalRequest.Phase) != "" {
			stage = strings.TrimSpace(approvalRequest.Phase)
		}
		return model.AgentRunPauseReasonHumanApproval, stage, nil
	}
	if latestHumanInputRequestFromResult(result) != nil {
		return model.AgentRunPauseReasonHumanInput, "awaiting_input", nil
	}
	return "", "", fmt.Errorf("execution result did not include a live human interaction request")
}

func liveCodexResumeStage(signal *workerpkg.LiveExecutionResumeSignal, pauseReason string) string {
	if signal == nil {
		return "resuming"
	}
	switch strings.TrimSpace(signal.Intent) {
	case model.AgentRunResumeIntentApprove:
		return "approved"
	case model.AgentRunResumeIntentRequestChanges:
		return "feedback_received"
	case model.AgentRunResumeIntentReply:
		if pauseReason == model.AgentRunPauseReasonHumanApproval {
			return "feedback_received"
		}
		return "input_received"
	default:
		return "resuming"
	}
}

func (a *AgentRunActivities) waitForLiveCodexResumeSignal(ctx context.Context, run *model.AgentRun, afterSequenceNo int, pauseReason string) (*workerpkg.LiveExecutionResumeSignal, error) {
	ticker := time.NewTicker(codexLivePausePollEvery)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return nil, workerpkg.ErrRunCancelled
		}

		currentRun, err := a.runRepo.GetByIDAny(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		if currentRun == nil {
			return nil, fmt.Errorf("agent run %s was not found while waiting for live codex input", run.ID)
		}
		model.NormalizeAgentRunPauseState(currentRun)

		switch currentRun.Status {
		case model.AgentRunStatusCancelled:
			return nil, workerpkg.ErrRunCancelled
		case model.AgentRunStatusFailed:
			return nil, fmt.Errorf("agent run failed while waiting for live codex input")
		case model.AgentRunStatusCompleted:
			return nil, fmt.Errorf("agent run completed while waiting for live codex input")
		}

		if signal, err := a.latestResolvedLiveCodexInteractionSignal(ctx, currentRun, afterSequenceNo); err != nil {
			return nil, err
		} else if signal != nil {
			return signal, nil
		}

		if pauseReason == model.AgentRunPauseReasonHumanApproval {
			switch strings.TrimSpace(currentRun.ApprovalState) {
			case "approved":
				content, err := a.latestLiveCodexUserMessage(ctx, currentRun, afterSequenceNo)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(content) == "" {
					content = "approve"
				}
				return &workerpkg.LiveExecutionResumeSignal{
					Intent:  model.AgentRunResumeIntentApprove,
					Content: content,
				}, nil
			case "rejected":
				content, err := a.latestLiveCodexUserMessage(ctx, currentRun, afterSequenceNo)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(content) == "" {
					return nil, fmt.Errorf("approval feedback message is missing for live codex resume")
				}
				return &workerpkg.LiveExecutionResumeSignal{
					Intent:  model.AgentRunResumeIntentRequestChanges,
					Content: content,
				}, nil
			}
		} else {
			content, err := a.latestLiveCodexUserMessage(ctx, currentRun, afterSequenceNo)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(content) != "" {
				return &workerpkg.LiveExecutionResumeSignal{
					Intent:  model.AgentRunResumeIntentReply,
					Content: content,
				}, nil
			}
		}

		select {
		case <-ctx.Done():
			return nil, workerpkg.ErrRunCancelled
		case <-ticker.C:
		}
	}
}

func (a *AgentRunActivities) latestResolvedLiveCodexInteractionSignal(ctx context.Context, run *model.AgentRun, afterSequenceNo int) (*workerpkg.LiveExecutionResumeSignal, error) {
	if a == nil || a.interactionRepo == nil || run == nil {
		return nil, nil
	}

	interaction, err := a.interactionRepo.GetLatestResolvedByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil || interaction == nil {
		return nil, err
	}
	if len(interaction.ResponsePayload) == 0 {
		return nil, nil
	}
	if interaction.AssistantMessageSequenceNo != nil && *interaction.AssistantMessageSequenceNo < afterSequenceNo {
		return nil, nil
	}

	followupInput, err := a.latestLiveCodexUserMessage(ctx, run, afterSequenceNo)
	if err != nil {
		return nil, err
	}
	return &workerpkg.LiveExecutionResumeSignal{
		Intent:          liveCodexResumeIntentForInteraction(interaction),
		Content:         strings.TrimSpace(followupInput),
		ResponsePayload: copyRawJSON(interaction.ResponsePayload),
	}, nil
}

func liveCodexResumeIntentForInteraction(interaction *model.AgentRunInteraction) string {
	if interaction == nil {
		return model.AgentRunResumeIntentReply
	}

	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindRequestUserInput:
		return model.AgentRunResumeIntentReply
	case model.AgentRunInteractionKindPermissionsApproval:
		var payload struct {
			Permissions map[string]any `json:"permissions"`
		}
		if err := json.Unmarshal(interaction.ResponsePayload, &payload); err == nil && len(payload.Permissions) > 0 {
			return model.AgentRunResumeIntentApprove
		}
		return model.AgentRunResumeIntentRequestChanges
	case model.AgentRunInteractionKindCommandExecutionApproval, model.AgentRunInteractionKindFileChangeApproval:
		var payload struct {
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(interaction.ResponsePayload, &payload); err == nil {
			switch strings.TrimSpace(payload.Decision) {
			case "accept", "acceptForSession", "acceptWithExecpolicyAmendment", "applyNetworkPolicyAmendment":
				return model.AgentRunResumeIntentApprove
			}
		}
		return model.AgentRunResumeIntentRequestChanges
	default:
		return model.AgentRunResumeIntentRequestChanges
	}
}

func (a *AgentRunActivities) latestLiveCodexUserMessage(ctx context.Context, run *model.AgentRun, afterSequenceNo int) (string, error) {
	if a.runMessageRepo == nil || run == nil {
		return "", nil
	}
	messages, err := a.runMessageRepo.ListByRunAfterSequence(ctx, run.WorkspaceID, run.ID, afterSequenceNo)
	if err != nil {
		return "", err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(messages[i].Role) != "user" {
			continue
		}
		content := strings.TrimSpace(messages[i].Content)
		if content != "" {
			return content, nil
		}
	}
	return "", nil
}

func (a *AgentRunActivities) captureTranscriptPlanningArtifacts(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, assistantMessage *model.AgentRunMessage, _ planningRunInput) error {
	if state == nil || state.run == nil || execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	switch state.run.TargetType {
	case "epic":
		if state.epic == nil {
			return nil
		}
	case "story", "task":
		if state.task == nil {
			return nil
		}
	default:
		return nil
	}

	assistantSequenceNo := 0
	if assistantMessage != nil {
		assistantSequenceNo = assistantMessage.SequenceNo
	}
	previews := workerpkg.ExtractPublishedPreviews(execCtx.LastExecutionResult.ToolInvocations)
	for index, preview := range previews {
		if err := a.createRunArtifactWithMetadata(ctx, state.run, workerpkg.RunPreviewArtifactType, "json", map[string]any{
			"panel_key": preview.PanelKey,
			"title":     preview.Title,
			"format":    preview.Format,
			"content":   json.RawMessage(preview.Content),
			"replace":   preview.Replace,
		}, 999900+index, buildAssistantSequenceArtifactMetadata(assistantSequenceNo)); err != nil {
			return err
		}
	}

	return nil
}

func (a *AgentRunActivities) applyApprovedInteractivePreview(ctx context.Context, state *resolvedRunState, input *planningRunInput) (string, error) {
	if state == nil || state.run == nil || input == nil {
		return "", nil
	}
	if state.run.InvocationMode != model.InvocationModeInteractive {
		return "", nil
	}
	if a.artifactRepo == nil {
		return "", nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return "", err
	}
	approvedArtifact, approvedPreview, err := nextUnappliedApprovedPreview(artifacts)
	if err != nil || approvedArtifact == nil || approvedPreview == nil {
		return "", err
	}

	var appliedAction string
	switch strings.ToLower(strings.TrimSpace(approvedPreview.Phase)) {
	case "prd":
		if state.epic == nil || state.run.TargetType != "epic" {
			return "", fmt.Errorf("approved PRD preview requires an epic target")
		}
		if err := a.applyApprovedPRDPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "persist_prd"
	case "task_doc", "story_doc":
		if state.task == nil {
			return "", fmt.Errorf("approved task planning doc preview requires a task target")
		}
		if err := a.applyApprovedTaskDocPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "persist_task_doc"
	case "tasks", "stories":
		if state.epic == nil || state.run.TargetType != "epic" {
			return "", fmt.Errorf("approved task plan preview requires an epic target")
		}
		if err := a.applyApprovedTaskPlanPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "create_tasks"
	default:
		return "", fmt.Errorf("unsupported approved preview phase %q", approvedPreview.Phase)
	}

	if _, err := a.appendRunArtifact(ctx, state.run, model.AgentRunArtifactTypeApprovedPreviewApplied, "json", model.AppliedApprovedRunPreview{
		ApprovedArtifactID: approvedArtifact.ID,
		Phase:              strings.TrimSpace(approvedPreview.Phase),
		Action:             appliedAction,
		AppliedAt:          time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return appliedAction, nil
}

func nextUnappliedApprovedPreview(artifacts []model.AgentRunArtifact) (*model.AgentRunArtifact, *model.ApprovedRunPreview, error) {
	applied := make(map[string]bool)
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreviewApplied || artifact.InlineContent == nil {
			continue
		}
		var marker model.AppliedApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &marker); err != nil {
			return nil, nil, fmt.Errorf("parse applied approved preview artifact: %w", err)
		}
		if strings.TrimSpace(marker.ApprovedArtifactID) != "" {
			applied[strings.TrimSpace(marker.ApprovedArtifactID)] = true
		}
	}

	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreview || artifact.InlineContent == nil {
			continue
		}
		if applied[artifact.ID] {
			continue
		}
		var preview model.ApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
			return nil, nil, fmt.Errorf("parse approved preview artifact: %w", err)
		}
		if approvedPreviewDebugEnabled() {
			slog.Info("selected approved preview for application",
				"artifact_id", artifact.ID,
				"sequence_no", artifact.SequenceNo,
				"phase", strings.TrimSpace(preview.Phase),
				"panel_key", strings.TrimSpace(preview.PanelKey),
				"format", strings.TrimSpace(preview.Format),
				"source_message_id", strings.TrimSpace(preview.SourceMessageID),
				"content_preview", previewDebugSnippet(preview.Content, 1600),
			)
		}
		return &artifact, &preview, nil
	}
	return nil, nil, nil
}

func decodeApprovedTaskPlanPreviewContent(raw json.RawMessage) (model.OrchestrationProposal, error) {
	var proposal model.OrchestrationProposal
	var payload map[string]json.RawMessage
	normalized, err := workerpkg.NormalizeTaskPlanPreviewContent(raw)
	if err != nil {
		if !decodeLooseApprovedTaskPlanPayload(raw, &payload) {
			if approvedPreviewDebugEnabled() {
				slog.Error("approved task plan preview normalization failed during apply",
					"raw_preview", previewDebugSnippet(raw, 1600),
					"error", err,
				)
			}
			return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; legacy proposed_stories is still accepted")
		}
	} else if err := json.Unmarshal(normalized, &payload); err != nil {
		if approvedPreviewDebugEnabled() {
			slog.Error("approved task plan preview payload unmarshal failed during apply",
				"normalized_preview", previewDebugSnippet(normalized, 1600),
				"error", err,
			)
		}
		return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; legacy proposed_stories is still accepted")
	}

	proposal.EpicID = decodeLooseJSONString(payload["epic_id"])
	proposal.Summary = decodeLooseJSONString(payload["summary"])
	proposal.SpecVersionID = decodeLooseJSONString(payload["spec_version_id"])
	proposal.OpenQuestions = decodeLooseJSONStringArray(payload["open_questions"])
	proposal.Risks = decodeLooseJSONStringArray(payload["risks"])
	if verticalCoverage, ok := decodeLooseVerticalCoverage(payload["vertical_coverage"]); ok {
		proposal.VerticalCoverage = verticalCoverage
	}

	var taskItems []json.RawMessage
	if err := json.Unmarshal(payload["proposed_stories"], &taskItems); err != nil {
		if err := json.Unmarshal(payload["proposed_tasks"], &taskItems); err != nil {
			if approvedPreviewDebugEnabled() {
				slog.Error("approved task plan preview proposed_tasks decode failed during apply",
					"normalized_preview", previewDebugSnippet(normalized, 1600),
					"error", err,
				)
			}
			return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; legacy proposed_stories is still accepted")
		}
	}
	proposal.ProposedTasks = make([]model.ProposedTask, 0, len(taskItems))
	for index, item := range taskItems {
		task, ok := decodeLooseApprovedProposedTask(item)
		if !ok {
			if approvedPreviewDebugEnabled() {
				slog.Error("approved task plan preview item decode failed during apply",
					"task_index", index,
					"task_preview", previewDebugSnippet(item, 1200),
				)
			}
			return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; legacy proposed_stories is still accepted")
		}
		proposal.ProposedTasks = append(proposal.ProposedTasks, task)
	}
	if approvedPreviewDebugEnabled() {
		slog.Info("decoded approved task plan preview",
			"summary_preview", truncateString(strings.TrimSpace(proposal.Summary), 240),
			"task_count", len(proposal.ProposedTasks),
		)
	}
	return proposal, nil
}

func decodeLooseApprovedTaskPlanPayload(raw json.RawMessage, payload *map[string]json.RawMessage) bool {
	if payload == nil {
		return false
	}
	if err := json.Unmarshal(raw, payload); err == nil {
		_, hasSummary := (*payload)["summary"]
		_, hasTasks := (*payload)["proposed_tasks"]
		_, hasLegacyTasks := (*payload)["proposed_stories"]
		return hasSummary && (hasTasks || hasLegacyTasks)
	}

	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return false
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" || !json.Valid([]byte(encoded)) {
		return false
	}
	if err := json.Unmarshal([]byte(encoded), payload); err != nil {
		return false
	}
	_, hasSummary := (*payload)["summary"]
	_, hasTasks := (*payload)["proposed_tasks"]
	_, hasLegacyTasks := (*payload)["proposed_stories"]
	return hasSummary && (hasTasks || hasLegacyTasks)
}

func decodeLooseApprovedProposedTask(raw json.RawMessage) (model.ProposedTask, bool) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return model.ProposedTask{}, false
	}

	task := model.ProposedTask{
		Ref:                decodeLooseJSONString(payload["ref"]),
		Name:               firstNonEmptyString(decodeLooseJSONString(payload["name"]), decodeLooseJSONString(payload["title"])),
		Description:        decodeLooseJSONString(payload["description"]),
		TaskType:           firstNonEmptyString(decodeLooseJSONString(payload["task_type"]), decodeLooseJSONString(payload["story_type"]), decodeLooseJSONString(payload["type"])),
		SliceType:          decodeLooseJSONString(payload["slice_type"]),
		AcceptanceCriteria: decodeLooseJSONStringArray(payload["acceptance_criteria"]),
		DependencyRefs:     decodeLooseJSONStringArray(payload["dependency_refs"]),
	}
	if estimate, ok := decodeLooseJSONInt(payload["estimate"]); ok {
		task.Estimate = &estimate
	}
	if priority := decodeLooseJSONString(payload["priority"]); priority != "" {
		task.Priority = &priority
	}
	if assignAgentID := decodeLooseJSONString(payload["assign_agent_id"]); assignAgentID != "" {
		task.AssignAgentID = &assignAgentID
	}
	if sourceRefs, ok := decodeLoosePlanningSourceRefs(payload["source_refs"]); ok {
		task.SourceRefs = sourceRefs
	}
	if brief, ok := decodeLooseTaskImplementationBrief(payload["implementation_brief"]); ok {
		task.ImplementationBrief = brief
	}
	return task, true
}

func decodeLooseJSONString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return ""
}

func decodeLooseJSONStringArray(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		filtered := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				filtered = append(filtered, item)
			}
		}
		return filtered
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil
		}
		return []string{single}
	}
	var mixed []any
	if err := json.Unmarshal(raw, &mixed); err == nil {
		filtered := make([]string, 0, len(mixed))
		for _, item := range mixed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text != "" {
				filtered = append(filtered, text)
			}
		}
		return filtered
	}
	return nil
}

func decodeLooseJSONInt(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var value int
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(text, "%d", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func decodeLoosePlanningSourceRefs(raw json.RawMessage) ([]model.PlanningSourceRef, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var refs []model.PlanningSourceRef
	if err := json.Unmarshal(raw, &refs); err == nil {
		return refs, true
	}
	return nil, false
}

func decodeLooseTaskImplementationBrief(raw json.RawMessage) (*model.TaskImplementationBrief, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}

	brief := &model.TaskImplementationBrief{
		Approach:       decodeLooseJSONString(payload["approach"]),
		FilesToModify:  decodeLooseFileChanges(payload["files_to_modify"]),
		TestStrategy:   decodeLooseJSONText(payload["test_strategy"]),
		VerticalLayers: decodeLooseJSONStringArray(payload["vertical_layers"]),
		DependsOnFiles: decodeLooseJSONStringArray(payload["depends_on_files"]),
	}
	return brief, true
}

func decodeLooseFileChanges(raw json.RawMessage) []model.FileChange {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	changes := make([]model.FileChange, 0, len(items))
	for _, item := range items {
		var change model.FileChange
		if err := json.Unmarshal(item, &change); err != nil {
			continue
		}
		if strings.TrimSpace(change.Path) == "" {
			continue
		}
		changes = append(changes, change)
	}
	return changes
}

func decodeLooseJSONText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		filtered := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				filtered = append(filtered, item)
			}
		}
		return strings.Join(filtered, "\n")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		return compact.String()
	}
	return strings.TrimSpace(string(raw))
}

func decodeLooseVerticalCoverage(raw json.RawMessage) ([]model.VerticalCoverageEntry, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var entries []model.VerticalCoverageEntry
	if err := json.Unmarshal(raw, &entries); err == nil {
		return entries, true
	}
	return nil, false
}

func approvedPreviewDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_PREVIEW_DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func previewDebugSnippet(raw json.RawMessage, max int) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		trimmed = compact.String()
	}
	if max > 0 && len(trimmed) > max {
		return trimmed[:max] + "...(truncated)"
	}
	return trimmed
}

func truncateString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max > 0 && len(value) > max {
		return value[:max] + "...(truncated)"
	}
	return value
}

func decodeApprovedMarkdownPreviewContent(raw json.RawMessage, previewLabel string) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", fmt.Errorf("%s content is empty; publish a non-empty markdown draft before requesting approval", previewLabel)
	}

	var markdown string
	if err := json.Unmarshal(raw, &markdown); err != nil {
		return "", fmt.Errorf("%s content must be a markdown string", previewLabel)
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return "", fmt.Errorf("%s content is empty; publish a non-empty markdown draft before requesting approval", previewLabel)
	}
	return markdown, nil
}

func (a *AgentRunActivities) applyApprovedPRDPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved PRD preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatMarkdown {
		return fmt.Errorf("approved PRD preview must use format %q", workerpkg.PreviewFormatMarkdown)
	}

	markdown, err := decodeApprovedMarkdownPreviewContent(preview.Content, "approved PRD preview")
	if err != nil {
		return err
	}

	doc, err := a.ensureEpicSpecDocument(ctx, state, runActorID(state.run))
	if err != nil {
		return err
	}
	if a.commandExecutor != nil {
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			TargetType:  "document",
			TargetID:    doc.ID,
		}, "docs.write_document_content", mustJSON(map[string]any{
			"document_id": doc.ID,
			"content":     markdown,
		})); err != nil {
			return err
		}
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			ActorID:     runActorID(state.run),
			TargetType:  "epic",
			TargetID:    state.epic.ID,
		}, "pm.approve_epic_spec", json.RawMessage(`{}`)); err != nil {
			return err
		}
	} else {
		savedContent, err := a.docsContentRepo.Upsert(ctx, doc.ID, tiptap.MarkdownToJSON(markdown))
		if err != nil {
			return err
		}
		label := "Approved Spec"
		version, err := a.docsVersionRepo.Create(ctx, doc.ID, runActorID(state.run), savedContent.Content, savedContent.ContentText, &label, "manual", len(strings.Fields(savedContent.ContentText)))
		if err != nil {
			return err
		}
		state.epic.SpecDocumentID = &doc.ID
		state.epic.ApprovedSpecVersionID = &version.ID
		if err := a.epicRepo.Update(ctx, state.epic); err != nil {
			return err
		}
	}

	epicWithStats, err := a.epicRepo.GetByID(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	if epicWithStats == nil {
		return fmt.Errorf("epic not found after applying approved PRD preview")
	}
	state.epic = &epicWithStats.Epic
	input.SpecDocumentID = strings.TrimSpace(derefString(state.epic.SpecDocumentID))
	input.SpecVersionID = strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID))
	return nil
}

func (a *AgentRunActivities) applyApprovedTaskPlanPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved task plan preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatJSON {
		return fmt.Errorf("approved task plan preview must use format %q", workerpkg.PreviewFormatJSON)
	}
	if a.commandExecutor == nil {
		return fmt.Errorf("planner commands are not available")
	}

	proposal, err := decodeApprovedTaskPlanPreviewContent(preview.Content)
	if err != nil {
		return err
	}
	if proposal.EpicID == "" {
		proposal.EpicID = state.epic.ID
	}
	if proposal.SpecVersionID == "" {
		proposal.SpecVersionID = strings.TrimSpace(firstNonEmptyString(input.SpecVersionID, derefString(state.epic.ApprovedSpecVersionID)))
	}
	if err := validatePlanningProposalTasks(proposal.ProposedTasks); err != nil {
		return err
	}

	output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: state.run.WorkspaceID,
		ActorID:     runActorID(state.run),
		TargetType:  "epic",
		TargetID:    state.epic.ID,
	}, "pm.create_task_batch", mustJSON(map[string]any{
		"tasks": proposal.ProposedTasks,
	}))
	if err != nil {
		return err
	}

	var result workerpkg.CreateTaskBatchResult
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("parse created task batch: %w", err)
	}

	tasks, err := a.epicRepo.ListTasks(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	state.epicTasks = tasks

	epicWithStats, err := a.epicRepo.GetByID(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	if epicWithStats == nil {
		return fmt.Errorf("epic not found after applying approved task plan")
	}
	state.epic = &epicWithStats.Epic
	state.epic.LastPlanningRunID = &state.run.ID
	if err := a.epicRepo.Update(ctx, state.epic); err != nil {
		return err
	}

	state.run.OutputSummary, _ = json.Marshal(planningRunSummary{
		Stage:               model.PlanningStagePlanTasks,
		SpecDocumentID:      strings.TrimSpace(firstNonEmptyString(input.SpecDocumentID, derefString(state.epic.SpecDocumentID))),
		SpecVersionID:       proposal.SpecVersionID,
		PlanningMethodology: input.PlanningMethodology,
		Summary:             strings.TrimSpace(proposal.Summary),
		Risks:               append([]string(nil), proposal.Risks...),
		OpenQuestions:       append([]string(nil), proposal.OpenQuestions...),
		Proposal:            &proposal,
	})

	createdCount := len(result.Tasks)
	summaryText := fmt.Sprintf("Applied the approved task plan and created %d tasks.", createdCount)
	if _, err := a.createRunMessage(ctx, state.run, "assistant", "assistant_turn", summaryText, nil, nil, nil, nil); err != nil {
		return err
	}

	completedAt := time.Now()
	state.run.Status = model.AgentRunStatusCompleted
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.CompletedAt = &completedAt
	state.run.ExecutionStage = strPtr("completed")
	state.run.LastHeartbeatAt = &completedAt
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)
	return a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
}

func (a *AgentRunActivities) applyApprovedTaskDocPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved task planning document preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatMarkdown {
		return fmt.Errorf("approved task planning document preview must use format %q", workerpkg.PreviewFormatMarkdown)
	}
	if state.task == nil {
		return fmt.Errorf("approved task planning document preview requires a task target")
	}

	markdown, err := decodeApprovedMarkdownPreviewContent(preview.Content, "approved task planning document preview")
	if err != nil {
		return err
	}

	doc, err := a.ensureTaskPlanDocument(ctx, state, runActorID(state.run))
	if err != nil {
		return err
	}
	if a.commandExecutor != nil {
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			TargetType:  "document",
			TargetID:    doc.ID,
		}, "docs.write_document_content", mustJSON(map[string]any{
			"document_id": doc.ID,
			"content":     markdown,
		})); err != nil {
			return err
		}
	} else {
		if _, err := a.docsContentRepo.Upsert(ctx, doc.ID, tiptap.MarkdownToJSON(markdown)); err != nil {
			return err
		}
		if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, runActorID(state.run)); err != nil {
			return err
		}
	}

	if content, err := a.docsContentRepo.GetByDocumentID(ctx, doc.ID); err == nil && content != nil {
		label := "Approved Task Plan"
		_, _ = a.docsVersionRepo.Create(ctx, doc.ID, runActorID(state.run), content.Content, content.ContentText, &label, "manual", len(strings.Fields(content.ContentText)))
	}

	state.task.PlanDocumentID = &doc.ID
	if err := a.taskRepo.Update(ctx, state.task); err != nil {
		return err
	}
	input.PlanDocumentID = doc.ID

	state.run.OutputSummary, _ = json.Marshal(planningRunSummary{
		Stage:          model.PlanningStageStoryPlanDoc,
		PlanDocumentID: doc.ID,
		Summary:        strings.TrimSpace(preview.ApprovalSummary),
	})

	summaryText := "Persisted the approved task plan to Docs and linked it to the task."
	if _, err := a.createRunMessage(ctx, state.run, "assistant", "assistant_turn", summaryText, nil, nil, nil, nil); err != nil {
		return err
	}

	completedAt := time.Now()
	state.run.Status = model.AgentRunStatusCompleted
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.CompletedAt = &completedAt
	state.run.ExecutionStage = strPtr("completed")
	state.run.LastHeartbeatAt = &completedAt
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)
	return a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
}

func (a *AgentRunActivities) loadRunState(ctx context.Context, runID string) (*resolvedRunState, error) {
	run, err := a.runRepo.GetByIDAny(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	model.NormalizeAgentRunPauseState(run)

	agent, err := a.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	resolved := workerpkg.ResolveAgentProfile(agent, run.InvocationMode)
	skillRefs := agentskills.EffectiveRuntimeRefs(agent)
	skillResolution, err := agentskills.Resolve(ctx, run.WorkspaceID, skillRefs, a.workspaceSkillRepo)
	if err != nil {
		return nil, err
	}
	agent.Skills = skillResolution.Refs
	if err := agentskills.ValidateRuntimeAndTools(agent.RuntimeKind, resolved.Tools, skillResolution.Definitions); err != nil {
		return nil, err
	}
	agent.ResolvedSkillInstructions = agentskills.CompileInstructions(skillResolution.Definitions)

	if run.TargetType == "" {
		switch {
		case run.TaskID != nil:
			run.TargetType = "task"
			run.TargetID = *run.TaskID
		case run.ConversationID != nil:
			run.TargetType = "support_conversation"
			run.TargetID = *run.ConversationID
		}
	}

	nativeSelectivePathEnabled := resolveNativeSelectivePlannerPathEnabled(run, agent)
	state := &resolvedRunState{
		run:                        run,
		agent:                      agent,
		runtimeSkillRefs:           skillResolution.Refs,
		runtimeSkillDefinitions:    skillResolution.Definitions,
		skillPolicy:                agentskills.AggregatePolicy(skillResolution.Definitions),
		nativeSelectivePathEnabled: nativeSelectivePathEnabled,
		resolved:                   resolved,
	}
	slog.InfoContext(ctx, "resolved run state",
		"workspace_id", run.WorkspaceID,
		"run_id", run.ID,
		"agent_id", run.AgentID,
		"runtime_kind", executionRuntimeKind(state),
		"preset_key", strings.TrimSpace(agent.EffectivePresetKey()),
		"target_type", strings.TrimSpace(run.TargetType),
		"native_selective_path_enabled", nativeSelectivePathEnabled,
		"runtime_skill_refs", runtimeSkillRefKeys(skillResolution.Refs),
	)

	if run.TaskID != nil {
		task, err := a.taskRepo.GetRawByID(ctx, *run.TaskID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, fmt.Errorf("task not found")
		}
		state.task = task

		if a.workspaceRepo != nil {
			ws, wsErr := a.workspaceRepo.GetByID(ctx, run.WorkspaceID)
			if wsErr == nil && ws != nil {
				state.workspaceKey = ws.WorkspaceKey
			}
		}

		target, teamDefault, err := a.resolveDeliveryTarget(ctx, run.WorkspaceID, task)
		if err != nil {
			return nil, err
		}
		state.deliveryTarget = target
		state.teamDefault = teamDefault

		if target != nil && target.RepositoryID != nil && *target.RepositoryID != "" {
			repo, err := a.gitRepo.GetByID(ctx, run.WorkspaceID, *target.RepositoryID)
			if err != nil {
				return nil, err
			}
			state.repository = repo
		}
		if target != nil && target.IntegrationID != nil && *target.IntegrationID != "" {
			integration, err := a.gitIntRepo.GetByID(ctx, run.WorkspaceID, *target.IntegrationID)
			if err != nil {
				return nil, err
			}
			state.integration = integration
		}
		if state.integration != nil {
			token, err := a.mintAccessToken(ctx, state.integration)
			if err != nil {
				return nil, err
			}
			state.accessToken = token
		}
		if task.EpicID != nil && strings.TrimSpace(*task.EpicID) != "" {
			epicWithStats, err := a.epicRepo.GetByID(ctx, *task.EpicID)
			if err != nil {
				return nil, err
			}
			if epicWithStats != nil {
				state.epic = &epicWithStats.Epic
			}
		}
	}

	if run.ConversationID != nil {
		conversation, err := a.conversationRepo.GetByID(ctx, run.WorkspaceID, *run.ConversationID, "", model.RoleOwner)
		if err != nil {
			return nil, err
		}
		if conversation == nil {
			return nil, fmt.Errorf("conversation not found")
		}
		state.conversation = conversation
	}

	if run.TargetType == "epic" {
		epicWithStats, err := a.epicRepo.GetByID(ctx, run.TargetID)
		if err != nil {
			return nil, err
		}
		if epicWithStats == nil {
			return nil, fmt.Errorf("epic not found")
		}
		state.epic = &epicWithStats.Epic
		epicTasks, err := a.epicRepo.ListTasks(ctx, run.TargetID)
		if err != nil {
			return nil, err
		}
		state.epicTasks = epicTasks
	}

	return state, nil
}

func (a *AgentRunActivities) resolveDeliveryTarget(ctx context.Context, workspaceID string, task *model.PMTask) (*model.TaskDeliveryTarget, *model.PMTeamRepoDefault, error) {
	target, err := a.deliveryRepo.GetByTask(ctx, workspaceID, task.ID)
	if err != nil {
		return nil, nil, err
	}
	if target != nil {
		var teamDefault *model.PMTeamRepoDefault
		if task.TeamID != nil && *task.TeamID != "" {
			teamDefault, err = a.settingsRepo.GetTeamRepoDefault(ctx, *task.TeamID)
			if err != nil {
				return nil, nil, err
			}
		}
		return target, teamDefault, nil
	}

	target = &model.TaskDeliveryTarget{
		WorkspaceID:   workspaceID,
		TaskID:        task.ID,
		DeliveryState: "unconfigured",
	}

	var teamDefault *model.PMTeamRepoDefault
	if task.TeamID != nil && *task.TeamID != "" {
		teamDefault, err = a.settingsRepo.GetTeamRepoDefault(ctx, *task.TeamID)
		if err != nil {
			return nil, nil, err
		}
		if teamDefault != nil {
			repo, err := a.gitRepo.GetByID(ctx, workspaceID, teamDefault.RepositoryID)
			if err != nil {
				return nil, nil, err
			}
			if repo != nil {
				target.RepositoryID = &repo.ID
				target.RepoFullName = &repo.FullName
				target.IntegrationID = &repo.IntegrationID
				baseBranch := teamDefault.BaseBranch
				if strings.TrimSpace(baseBranch) == "" {
					baseBranch = defaultString(repo.DefaultBranch, "main")
				}
				target.BaseBranch = &baseBranch
				target.DeliveryState = "ready"
			}
		}
	}

	if err := a.deliveryRepo.Save(ctx, target); err != nil {
		return nil, nil, err
	}
	return target, teamDefault, nil
}

func (a *AgentRunActivities) prepareTaskDelivery(ctx context.Context, state *resolvedRunState) error {
	target := state.deliveryTarget
	if target == nil {
		return fmt.Errorf("task delivery target is missing")
	}
	if state.repository == nil || state.integration == nil {
		if state.resolved.RequiresRepo {
			return fmt.Errorf("task has no delivery target configured")
		}
		return nil
	}

	if target.RepoFullName == nil {
		target.RepoFullName = &state.repository.FullName
	}
	if target.IntegrationID == nil {
		target.IntegrationID = &state.repository.IntegrationID
	}
	effectiveBaseBranch := strings.TrimSpace(derefString(target.BaseBranch))
	if requestedBaseBranch := strings.TrimSpace(derefString(state.run.BaseBranch)); requestedBaseBranch != "" {
		effectiveBaseBranch = requestedBaseBranch
	} else if effectiveBaseBranch == "" {
		effectiveBaseBranch = defaultString(state.repository.DefaultBranch, "main")
		target.BaseBranch = &effectiveBaseBranch
	}

	effectiveWorkingBranch := strings.TrimSpace(derefString(target.WorkingBranch))
	if requestedWorkingBranch := strings.TrimSpace(derefString(state.run.WorkingBranch)); requestedWorkingBranch != "" {
		effectiveWorkingBranch = requestedWorkingBranch
	} else if state.resolved.RequiresRepo && effectiveWorkingBranch == "" {
		effectiveWorkingBranch = buildWorkingBranch(state.task, state.teamDefault, state.workspaceKey)
		target.WorkingBranch = &effectiveWorkingBranch
	}

	if state.resolved.RequiresRepo && state.accessToken == "" {
		return fmt.Errorf("repository access token is not available")
	}

	if state.resolved.RequiresRepo && effectiveWorkingBranch != "" {
		if err := a.ensureRemoteBranch(ctx, state.integration, state.accessToken, state.repository.FullName, effectiveBaseBranch, effectiveWorkingBranch); err != nil {
			return err
		}
	}

	target.LastRunID = &state.run.ID
	if target.RepositoryID != nil {
		state.run.RepositoryID = target.RepositoryID
	}
	state.run.RepoFullName = target.RepoFullName
	if strings.TrimSpace(effectiveBaseBranch) != "" {
		state.run.BaseBranch = &effectiveBaseBranch
	} else {
		state.run.BaseBranch = nil
	}
	if strings.TrimSpace(effectiveWorkingBranch) != "" {
		state.run.WorkingBranch = &effectiveWorkingBranch
	} else {
		state.run.WorkingBranch = nil
	}
	state.run.DeliveryTargetID = &target.ID
	if state.run.TaskQueue == nil || *state.run.TaskQueue == "" {
		queue := state.resolved.Queue
		state.run.TaskQueue = &queue
		state.run.RunnerPool = &queue
	}
	target.DeliveryState = "in_progress"
	if err := a.deliveryRepo.Save(ctx, target); err != nil {
		return err
	}

	return a.upsertGitLink(ctx, state, "", nil)
}

func (a *AgentRunActivities) checkoutRunRef(ctx context.Context, workDir string, state *resolvedRunState) error {
	baseBranch := derefString(state.run.BaseBranch)
	workingBranch := derefString(state.run.WorkingBranch)
	if workingBranch != "" {
		if refName, err := a.fetchRemoteTrackingBranch(ctx, workDir, state.integration, state.accessToken, workingBranch); err == nil {
			if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", workingBranch, refName); err == nil {
				return nil
			}
		}
	}
	if baseBranch == "" {
		baseBranch = defaultString(state.repository.DefaultBranch, "main")
	}
	baseRefName, err := a.fetchRemoteTrackingBranch(ctx, workDir, state.integration, state.accessToken, baseBranch)
	if err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	if workingBranch != "" {
		if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", workingBranch, baseRefName); err != nil {
			return fmt.Errorf("checkout working branch: %w", err)
		}
		return nil
	}
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", baseBranch, baseRefName); err != nil {
		return fmt.Errorf("checkout base branch: %w", err)
	}
	return nil
}

func (a *AgentRunActivities) syncBaseIntoWorkingBranch(ctx context.Context, workDir string, state *resolvedRunState) error {
	if state == nil {
		return nil
	}
	baseBranch := strings.TrimSpace(derefString(state.run.BaseBranch))
	workingBranch := strings.TrimSpace(derefString(state.run.WorkingBranch))
	state.branchSync = branchSyncState{
		Status:        "not_applicable",
		BaseBranch:    baseBranch,
		WorkingBranch: workingBranch,
	}
	if baseBranch == "" || workingBranch == "" {
		return nil
	}
	if baseBranch == workingBranch {
		state.branchSync.Status = "same_branch"
		return nil
	}

	baseRefName, err := a.fetchRemoteTrackingBranch(ctx, workDir, state.integration, state.accessToken, baseBranch)
	if err != nil {
		return fmt.Errorf("fetch base branch for sync: %w", err)
	}
	needsMerge, err := a.workingBranchNeedsBaseSync(ctx, workDir, state.integration, state.accessToken, baseRefName, baseBranch, workingBranch)
	if err != nil {
		if errors.Is(err, errBranchSyncUnrelatedHistory) {
			if err := a.recoverUnrelatedWorkingBranch(ctx, workDir, state, baseRefName, baseBranch, workingBranch); err != nil {
				state.branchSync.Status = "unrelated_history"
				return err
			}
			return nil
		}
		return fmt.Errorf("check base sync status: %w", err)
	}
	if !needsMerge {
		state.branchSync.Status = "up_to_date"
		return nil
	}

	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "merge", "--no-ff", "--no-edit", baseRefName); err == nil {
		state.branchSync.Status = "merged"
		return nil
	} else {
		conflictFiles, conflictErr := a.gitMergeConflictFiles(ctx, workDir, state.integration, state.accessToken)
		if conflictErr == nil && len(conflictFiles) > 0 {
			state.branchSync.Status = "conflicted"
			state.branchSync.ConflictFiles = conflictFiles
			if executionRuntimeKind(state) == "codex" {
				slog.WarnContext(ctx, "agent run branch sync produced merge conflicts; handing off to codex",
					"workspace_id", state.run.WorkspaceID,
					"run_id", state.run.ID,
					"base_branch", baseBranch,
					"working_branch", workingBranch,
					"conflict_files", conflictFiles,
				)
				return nil
			}
			_, _ = a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "merge", "--abort")
			return fmt.Errorf("sync base branch into working branch: merge conflicts in %s", strings.Join(conflictFiles, ", "))
		}
		_, _ = a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "merge", "--abort")
		return fmt.Errorf("sync base branch into working branch: %w", err)
	}
}

func (a *AgentRunActivities) ensureRemoteBranch(ctx context.Context, integration *model.GitIntegration, accessToken, repoFullName, baseBranch, workingBranch string) error {
	workDir, err := workerpkg.PrepareWorkspace(ctx, integration, repoFullName, accessToken)
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	baseRefName, err := a.fetchRemoteTrackingBranch(ctx, workDir, integration, accessToken, baseBranch)
	if err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	if _, err := a.fetchRemoteTrackingBranch(ctx, workDir, integration, accessToken, workingBranch); err == nil {
		return nil
	}
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "checkout", "-B", workingBranch, baseRefName); err != nil {
		return fmt.Errorf("create working branch: %w", err)
	}
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "push", "-u", "origin", workingBranch); err != nil {
		return fmt.Errorf("push working branch: %w", err)
	}
	return nil
}

func (a *AgentRunActivities) fetchRemoteTrackingBranch(
	ctx context.Context,
	workDir string,
	integration *model.GitIntegration,
	accessToken string,
	branch string,
) (string, error) {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		return "", fmt.Errorf("branch is required")
	}
	refName := "refs/remotes/origin/" + trimmed
	refspec := fmt.Sprintf("refs/heads/%s:%s", trimmed, refName)
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "fetch", "origin", refspec); err != nil {
		return "", err
	}
	return "origin/" + trimmed, nil
}

func (a *AgentRunActivities) workingBranchNeedsBaseSync(
	ctx context.Context,
	workDir string,
	integration *model.GitIntegration,
	accessToken string,
	baseRefName string,
	baseBranch string,
	workingBranch string,
) (bool, error) {
	mergeBaseOutput, err := a.runGitInDir(ctx, workDir, integration, accessToken, "merge-base", "HEAD", strings.TrimSpace(baseRefName))
	if mergeBaseHash := extractMergeBaseHash(mergeBaseOutput, errorText(err)); mergeBaseHash != "" {
		mergeBaseOutput = mergeBaseHash
		err = nil
	}
	if err != nil {
		if strings.TrimSpace(mergeBaseOutput) == "" {
			retried, retryErr := a.retryMergeBaseAfterFetchingHistory(ctx, workDir, integration, accessToken, baseRefName, baseBranch, workingBranch)
			if retryErr == nil {
				mergeBaseOutput = retried
				err = nil
			} else if mergeBaseHash := extractMergeBaseHash(retried, errorText(retryErr)); mergeBaseHash != "" {
				mergeBaseOutput = mergeBaseHash
				err = nil
			}
		}
		if mergeBaseHash := extractMergeBaseHash(mergeBaseOutput, errorText(err)); mergeBaseHash != "" {
			mergeBaseOutput = mergeBaseHash
			err = nil
		}
	}
	if err != nil {
		if strings.TrimSpace(mergeBaseOutput) == "" {
			return false, fmt.Errorf("%w", errBranchSyncUnrelatedHistory)
		}
		detail := strings.TrimSpace(mergeBaseOutput)
		if detail == "" {
			detail = strings.TrimSpace(err.Error())
		}
		return false, fmt.Errorf("determine merge base: %s", detail)
	}
	output, err := a.runGitInDir(ctx, workDir, integration, accessToken, "rev-list", "--count", "HEAD.."+strings.TrimSpace(baseRefName))
	if err != nil {
		return false, err
	}
	count, parseErr := strconv.Atoi(strings.TrimSpace(output))
	if parseErr != nil {
		return false, fmt.Errorf("parse rev-list count: %w", parseErr)
	}
	return count > 0, nil
}

func (a *AgentRunActivities) retryMergeBaseAfterFetchingHistory(
	ctx context.Context,
	workDir string,
	integration *model.GitIntegration,
	accessToken string,
	baseRefName, baseBranch, workingBranch string,
) (string, error) {
	isShallowOutput, err := a.runGitInDir(ctx, workDir, integration, accessToken, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(isShallowOutput) != "true" {
		return "", fmt.Errorf("repository is not shallow")
	}

	trimmedBase := strings.TrimSpace(baseBranch)
	trimmedWorking := strings.TrimSpace(workingBranch)
	refspecs := make([]string, 0, 2)
	if trimmedBase != "" {
		refspecs = append(refspecs, fmt.Sprintf("refs/heads/%s:refs/remotes/origin/%s", trimmedBase, trimmedBase))
	}
	if trimmedWorking != "" && trimmedWorking != trimmedBase {
		refspecs = append(refspecs, fmt.Sprintf("refs/heads/%s:refs/remotes/origin/%s", trimmedWorking, trimmedWorking))
	}

	args := []string{"fetch", "--update-shallow", "--unshallow", "origin"}
	args = append(args, refspecs...)
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, args...); err != nil {
		return "", err
	}

	mergeBaseOutput, err := a.runGitInDir(ctx, workDir, integration, accessToken, "merge-base", "HEAD", strings.TrimSpace(baseRefName))
	if mergeBaseHash := extractMergeBaseHash(mergeBaseOutput, errorText(err)); mergeBaseHash != "" {
		return mergeBaseHash, nil
	}
	if err != nil {
		return mergeBaseOutput, err
	}
	return strings.TrimSpace(mergeBaseOutput), nil
}

func extractMergeBaseHash(texts ...string) string {
	for _, text := range texts {
		fields := strings.Fields(strings.TrimSpace(text))
		for _, field := range fields {
			candidate := strings.TrimSpace(field)
			if len(candidate) < 7 || len(candidate) > 40 {
				continue
			}
			valid := true
			for _, r := range candidate {
				switch {
				case r >= '0' && r <= '9':
				case r >= 'a' && r <= 'f':
				case r >= 'A' && r <= 'F':
				default:
					valid = false
					break
				}
			}
			if valid {
				return candidate
			}
		}
	}
	return ""
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *AgentRunActivities) recoverUnrelatedWorkingBranch(
	ctx context.Context,
	workDir string,
	state *resolvedRunState,
	baseRefName, baseBranch, workingBranch string,
) error {
	if state == nil {
		return fmt.Errorf("sync base branch into working branch: working branch %q does not share history with base branch %q", workingBranch, baseBranch)
	}
	if state.deliveryTarget != nil && state.deliveryTarget.ActivePRNumber != nil {
		state.branchSync.Status = "unrelated_history"
		return fmt.Errorf("sync base branch into working branch: working branch %q does not share history with base branch %q and has an active pull request", workingBranch, baseBranch)
	}

	headSHAOutput, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("sync base branch into working branch: resolve unrelated branch head: %w", err)
	}
	headSHA := strings.TrimSpace(headSHAOutput)
	backupBranch := buildUnrelatedHistoryBackupBranch(headSHA)

	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "branch", "-f", backupBranch, "HEAD"); err != nil {
		return fmt.Errorf("sync base branch into working branch: create backup branch: %w", err)
	}
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "push", "-u", "origin", backupBranch); err != nil {
		return fmt.Errorf("sync base branch into working branch: push backup branch: %w", err)
	}
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", workingBranch, baseRefName); err != nil {
		return fmt.Errorf("sync base branch into working branch: recreate working branch from base: %w", err)
	}
	leaseRef := fmt.Sprintf("--force-with-lease=refs/heads/%s:%s", workingBranch, headSHA)
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "push", leaseRef, "-u", "origin", workingBranch); err != nil {
		return fmt.Errorf("sync base branch into working branch: reset remote working branch from base: %w", err)
	}

	state.branchSync.Status = "recreated_from_base"
	state.branchSync.BackupBranch = backupBranch
	slog.WarnContext(ctx, "agent run recovered unrelated working branch by recreating it from base",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"base_branch", baseBranch,
		"working_branch", workingBranch,
		"backup_branch", backupBranch,
	)
	return nil
}

func buildUnrelatedHistoryBackupBranch(headSHA string) string {
	shortSHA := strings.TrimSpace(headSHA)
	if len(shortSHA) > 12 {
		shortSHA = shortSHA[:12]
	}
	if shortSHA == "" {
		shortSHA = "unknown"
	}
	return fmt.Sprintf("helpin-backup/unrelated-history/%s-%s", time.Now().UTC().Format("20060102150405"), shortSHA)
}

func (a *AgentRunActivities) gitMergeConflictFiles(
	ctx context.Context,
	workDir string,
	integration *model.GitIntegration,
	accessToken string,
) ([]string, error) {
	output, err := a.runGitInDir(ctx, workDir, integration, accessToken, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		files = append(files, trimmed)
	}
	slices.Sort(files)
	return files, nil
}

func (a *AgentRunActivities) recordPush(ctx context.Context, state *resolvedRunState, branch, sha string) error {
	if state.deliveryTarget == nil {
		return nil
	}
	state.deliveryTarget.WorkingBranch = &branch
	state.deliveryTarget.LastCommitSHA = &sha
	state.deliveryTarget.LastRunID = &state.run.ID
	state.deliveryTarget.DeliveryState = "pushed"
	now := time.Now()
	state.deliveryTarget.LastSyncedAt = &now
	state.run.WorkingBranch = &branch
	state.run.LastHeartbeatAt = &now
	if err := a.deliveryRepo.Save(ctx, state.deliveryTarget); err != nil {
		return err
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	return a.upsertGitLink(ctx, state, sha, nil)
}

func (a *AgentRunActivities) recordPushAndEnsureDeliveryPR(ctx context.Context, state *resolvedRunState, branch, sha string) error {
	if err := a.recordPush(ctx, state, branch, sha); err != nil {
		return err
	}
	pr, err := a.ensureDeliveryPullRequest(ctx, state, branch)
	if err != nil {
		baseBranch := ""
		if state != nil && state.run != nil {
			baseBranch = strings.TrimSpace(derefString(state.run.BaseBranch))
		}
		if baseBranch == "" && state != nil && state.deliveryTarget != nil {
			baseBranch = strings.TrimSpace(derefString(state.deliveryTarget.BaseBranch))
		}
		slog.WarnContext(ctx, "failed to ensure delivery pull request after push",
			"workspace_id", safeRunWorkspaceID(state),
			"run_id", safeRunID(state),
			"branch", branch,
			"base_branch", baseBranch,
			"error", err,
		)
		if a.artifactRepo != nil && state != nil && state.run != nil {
			_, _ = a.appendRunArtifact(ctx, state.run, "git_delivery_result", "json", map[string]any{
				"delivery":    "pr_failed",
				"branch":      branch,
				"base_branch": baseBranch,
				"error":       err.Error(),
			})
		}
		if state != nil && state.deliveryTarget != nil {
			now := time.Now()
			state.deliveryTarget.DeliveryState = "pr_failed"
			state.deliveryTarget.LastRunID = &state.run.ID
			state.deliveryTarget.LastSyncedAt = &now
			if saveErr := a.deliveryRepo.Save(ctx, state.deliveryTarget); saveErr != nil {
				slog.WarnContext(ctx, "failed to persist pr_failed delivery state after pull request error",
					"workspace_id", safeRunWorkspaceID(state),
					"run_id", safeRunID(state),
					"branch", branch,
					"error", saveErr,
				)
			}
		}
		return nil
	}
	if pr == nil {
		return nil
	}
	if err := a.recordPR(ctx, state, pr.Metadata, pr.Title); err != nil {
		if a.artifactRepo != nil && state != nil && state.run != nil {
			_, _ = a.appendRunArtifact(ctx, state.run, "git_delivery_result", "json", map[string]any{
				"delivery":    "pr_persistence_failed",
				"created":     !pr.Existing,
				"branch":      branch,
				"base_branch": strings.TrimSpace(pr.Metadata.Base),
				"number":      pr.Metadata.Number,
				"url":         pr.Metadata.URL,
				"title":       pr.Title,
				"error":       err.Error(),
			})
		}
		return fmt.Errorf("persist delivery pull request after upstream create/reuse: %w", err)
	}
	if a.artifactRepo != nil && state != nil && state.run != nil {
		_, _ = a.appendRunArtifact(ctx, state.run, "git_delivery_result", "json", map[string]any{
			"delivery":    "pr_open",
			"created":     !pr.Existing,
			"branch":      branch,
			"base_branch": strings.TrimSpace(pr.Metadata.Base),
			"number":      pr.Metadata.Number,
			"url":         pr.Metadata.URL,
			"title":       pr.Title,
		})
	}
	return nil
}

func (a *AgentRunActivities) pushCodexLocalCommit(ctx context.Context, workDir string, state *resolvedRunState, execCtx *workerpkg.ExecutionContext) error {
	if execCtx == nil || execCtx.LocalGitCommit == nil {
		return nil
	}
	if state == nil || state.run == nil {
		return fmt.Errorf("push codex commit: missing run state")
	}

	branch := strings.TrimSpace(execCtx.LocalGitCommit.Branch)
	if branch == "" {
		branch = strings.TrimSpace(execCtx.WorkingBranch)
	}
	if branch == "" {
		return fmt.Errorf("push codex commit: working branch is empty")
	}

	sha := strings.TrimSpace(execCtx.LocalGitCommit.CommitSHA)
	if sha == "" {
		return fmt.Errorf("push codex commit: commit sha is empty")
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("pushing_changes")
	}

	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push repository changes: %w", err)
	}
	if err := a.recordPushAndEnsureDeliveryPR(ctx, state, branch, sha); err != nil {
		return fmt.Errorf("record pushed branch: %w", err)
	}

	if a.artifactRepo != nil && state != nil && state.run != nil {
		payload := map[string]any{
			"branch":         branch,
			"commit_sha":     sha,
			"commit_message": strings.TrimSpace(execCtx.LocalGitCommit.CommitMessage),
			"changed_files":  slices.Clone(execCtx.LocalGitCommit.ChangedFiles),
			"delivery":       "pushed",
		}
		if _, err := a.appendRunArtifact(ctx, state.run, "git_delivery_result", "json", payload); err != nil {
			slog.WarnContext(ctx, "failed to save git delivery result artifact",
				"error", err,
				"run_id", state.run.ID,
				"branch", branch,
			)
		}
	}

	return nil
}

func (a *AgentRunActivities) ensureDeliveryPullRequest(ctx context.Context, state *resolvedRunState, workingBranch string) (*ensuredDeliveryPR, error) {
	if state == nil || state.run == nil || state.task == nil || state.repository == nil || state.integration == nil || state.deliveryTarget == nil {
		return nil, nil
	}
	if strings.TrimSpace(state.integration.Provider) != "github" {
		return nil, nil
	}

	repoFullName := strings.TrimSpace(state.repository.FullName)
	if repoFullName == "" {
		repoFullName = strings.TrimSpace(derefString(state.deliveryTarget.RepoFullName))
	}
	workingBranch = strings.TrimSpace(workingBranch)
	if workingBranch == "" {
		workingBranch = strings.TrimSpace(derefString(state.deliveryTarget.WorkingBranch))
	}
	if workingBranch == "" {
		workingBranch = strings.TrimSpace(derefString(state.run.WorkingBranch))
	}
	baseBranch := strings.TrimSpace(derefString(state.run.BaseBranch))
	if baseBranch == "" {
		baseBranch = strings.TrimSpace(derefString(state.deliveryTarget.BaseBranch))
	}
	if baseBranch == "" {
		baseBranch = defaultString(state.repository.DefaultBranch, "main")
	}
	if repoFullName == "" || workingBranch == "" || baseBranch == "" || workingBranch == baseBranch {
		return nil, nil
	}
	if strings.TrimSpace(state.accessToken) == "" {
		return nil, fmt.Errorf("repository access token is not available for pull request creation")
	}

	title, body := buildDeliveryPullRequestContent(state, baseBranch, workingBranch)
	pr, err := ensureGitHubPullRequest(ctx, state.integration, state.accessToken, repoFullName, workingBranch, baseBranch, title, body)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

type ensuredDeliveryPR struct {
	Metadata workerpkg.PRMetadata
	Title    string
	Existing bool
}

func ensureGitHubPullRequest(
	ctx context.Context,
	integration *model.GitIntegration,
	accessToken, repoFullName, workingBranch, baseBranch, title, body string,
) (*ensuredDeliveryPR, error) {
	if integration == nil || strings.TrimSpace(integration.Provider) != "github" {
		return nil, nil
	}
	repoFullName = strings.TrimSpace(repoFullName)
	workingBranch = strings.TrimSpace(workingBranch)
	baseBranch = strings.TrimSpace(baseBranch)
	accessToken = strings.TrimSpace(accessToken)
	if repoFullName == "" || workingBranch == "" || baseBranch == "" {
		return nil, nil
	}
	if accessToken == "" {
		return nil, fmt.Errorf("github access token is required")
	}

	owner, repo, err := splitRepoFullName(repoFullName)
	if err != nil {
		return nil, err
	}
	apiBase := model.ResolveGitHubAPIBaseURL(integration.BaseURL)

	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", owner+":"+workingBranch)
	query.Set("base", baseBranch)
	query.Set("per_page", "1")

	var existingPayload []struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := doGitHubAPIRequest(ctx, accessToken, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/pulls?%s", apiBase, owner, repo, query.Encode()), nil, &existingPayload); err != nil {
		return nil, err
	}
	if len(existingPayload) > 0 {
		existing := existingPayload[0]
		return &ensuredDeliveryPR{
			Metadata: workerpkg.PRMetadata{
				Provider: "github",
				URL:      strings.TrimSpace(existing.HTMLURL),
				Number:   existing.Number,
				Head:     strings.TrimSpace(existing.Head.Ref),
				Base:     strings.TrimSpace(existing.Base.Ref),
			},
			Title:    strings.TrimSpace(existing.Title),
			Existing: true,
		}, nil
	}

	var createdPayload struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := doGitHubAPIRequest(ctx, accessToken, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/pulls", apiBase, owner, repo), map[string]string{
		"title": title,
		"body":  body,
		"head":  workingBranch,
		"base":  baseBranch,
	}, &createdPayload); err != nil {
		return nil, err
	}
	return &ensuredDeliveryPR{
		Metadata: workerpkg.PRMetadata{
			Provider: "github",
			URL:      strings.TrimSpace(createdPayload.HTMLURL),
			Number:   createdPayload.Number,
			Head:     strings.TrimSpace(createdPayload.Head.Ref),
			Base:     strings.TrimSpace(createdPayload.Base.Ref),
		},
		Title:    strings.TrimSpace(createdPayload.Title),
		Existing: false,
	}, nil
}

func doGitHubAPIRequest(ctx context.Context, accessToken, method, requestURL string, payload any, out any) error {
	var requestBody []byte
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal github request payload: %w", err)
		}
		requestBody = data
	}
	reqCtx, cancel := context.WithTimeout(ctx, githubAPIRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, requestURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("build github request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("Accept", "application/vnd.github+json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errorPayload struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errorPayload)
		return fmt.Errorf("github api failed (%d): %s", resp.StatusCode, strings.TrimSpace(errorPayload.Message))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

func buildDeliveryPullRequestContent(state *resolvedRunState, baseBranch, workingBranch string) (string, string) {
	taskName := ""
	taskKey := ""
	if state != nil && state.task != nil {
		taskName = strings.TrimSpace(state.task.Name)
		if state.workspaceKey != "" && state.task.DisplayID > 0 {
			taskKey = strings.TrimSpace(model.FormatTaskKey(state.workspaceKey, state.task.DisplayID))
		}
	}

	title := strings.TrimSpace(taskName)
	switch {
	case taskKey != "" && title != "":
		title = fmt.Sprintf("%s: %s", taskKey, title)
	case taskKey != "":
		title = taskKey
	case title == "":
		title = fmt.Sprintf("Automated changes from %s", workingBranch)
	}

	lines := []string{
		"Automated pull request opened by Helpin.",
		"",
		"## Context",
	}
	if taskKey != "" || taskName != "" {
		taskLine := "- Task: "
		switch {
		case taskKey != "" && taskName != "":
			taskLine += fmt.Sprintf("%s — %s", taskKey, taskName)
		case taskKey != "":
			taskLine += taskKey
		default:
			taskLine += taskName
		}
		lines = append(lines, taskLine)
	}
	if state != nil && state.agent != nil && strings.TrimSpace(state.agent.Name) != "" {
		lines = append(lines, "- Agent: "+strings.TrimSpace(state.agent.Name))
	}
	if strings.TrimSpace(workingBranch) != "" {
		lines = append(lines, "- Branch: `"+strings.TrimSpace(workingBranch)+"`")
	}
	if strings.TrimSpace(baseBranch) != "" {
		lines = append(lines, "- Base branch: `"+strings.TrimSpace(baseBranch)+"`")
	}
	if state != nil && state.run != nil && strings.TrimSpace(state.run.ID) != "" {
		lines = append(lines, "- Run ID: `"+state.run.ID+"`")
	}
	return title, strings.Join(lines, "\n")
}

func splitRepoFullName(repoFullName string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(repoFullName), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("invalid repository full name %q", repoFullName)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func safeRunWorkspaceID(state *resolvedRunState) string {
	if state == nil || state.run == nil {
		return ""
	}
	return state.run.WorkspaceID
}

func safeRunID(state *resolvedRunState) string {
	if state == nil || state.run == nil {
		return ""
	}
	return state.run.ID
}

func (a *AgentRunActivities) recordPR(ctx context.Context, state *resolvedRunState, metadata workerpkg.PRMetadata, title string) error {
	if state.deliveryTarget == nil {
		return nil
	}
	state.deliveryTarget.ActivePRNumber = &metadata.Number
	state.deliveryTarget.ActivePRTitle = &title
	state.deliveryTarget.ActivePRURL = &metadata.URL
	openStatus := "open"
	state.deliveryTarget.ActivePRStatus = &openStatus
	state.deliveryTarget.LastRunID = &state.run.ID
	state.deliveryTarget.DeliveryState = "pr_open"
	now := time.Now()
	state.deliveryTarget.LastSyncedAt = &now
	if err := a.deliveryRepo.Save(ctx, state.deliveryTarget); err != nil {
		return err
	}
	return a.upsertGitLink(ctx, state, "", &prUpdate{
		number: metadata.Number,
		title:  title,
		url:    metadata.URL,
		status: openStatus,
	})
}

type prUpdate struct {
	number int
	title  string
	url    string
	status string
}

func (a *AgentRunActivities) upsertGitLink(ctx context.Context, state *resolvedRunState, sha string, pr *prUpdate) error {
	if state.task == nil || state.deliveryTarget == nil || state.repository == nil || state.integration == nil {
		return nil
	}
	branch := derefString(state.deliveryTarget.WorkingBranch)
	if branch == "" {
		return nil
	}

	link, err := a.gitLinkRepo.GetByBranch(ctx, state.run.WorkspaceID, state.repository.FullName, branch)
	if err != nil {
		return err
	}
	if link == nil {
		link = &model.TaskGitLink{
			WorkspaceID:   state.run.WorkspaceID,
			TaskID:        state.task.ID,
			IntegrationID: state.integration.ID,
			RepositoryID:  state.deliveryTarget.RepositoryID,
			RunID:         &state.run.ID,
			Provider:      state.integration.Provider,
			Repo:          state.repository.FullName,
			Branch:        &branch,
		}
		if sha != "" {
			link.CommitSHA = &sha
		}
		if pr != nil {
			link.PRNumber = &pr.number
			link.PRTitle = &pr.title
			link.PRURL = &pr.url
			link.PRStatus = &pr.status
		}
		return a.gitLinkRepo.Create(ctx, link)
	}

	link.RepositoryID = state.deliveryTarget.RepositoryID
	link.RunID = &state.run.ID
	link.Provider = state.integration.Provider
	link.IntegrationID = state.integration.ID
	if sha != "" {
		link.CommitSHA = &sha
	}
	if pr != nil {
		link.PRNumber = &pr.number
		link.PRTitle = &pr.title
		link.PRURL = &pr.url
		link.PRStatus = &pr.status
	}
	return a.gitLinkRepo.Update(ctx, link)
}

func (a *AgentRunActivities) resolvePlanningRunInput(ctx context.Context, state *resolvedRunState) (planningRunInput, error) {
	var input planningRunInput
	if len(state.run.Input) > 0 {
		_ = json.Unmarshal(state.run.Input, &input)
	}

	input.Stage = strings.TrimSpace(input.Stage)
	input.AdditionalContext = strings.TrimSpace(input.AdditionalContext)
	input.PlanDocumentID = strings.TrimSpace(input.PlanDocumentID)
	input.SpecDocumentID = strings.TrimSpace(input.SpecDocumentID)
	input.SpecVersionID = strings.TrimSpace(input.SpecVersionID)
	input.PlanningMethodology = model.NormalizePlanningMethodology(strings.TrimSpace(input.PlanningMethodology))
	input.FlowOutputKind = strings.TrimSpace(input.FlowOutputKind)

	if err := a.normalizeEpicSpecState(ctx, state); err != nil {
		return planningRunInput{}, err
	}

	tools := effectiveToolSet(state.resolved, input.AllowedTools)
	if state.run.TargetType == "task" && state.task != nil && tools[workerpkg.ToolPublishTaskPlanDoc] {
		if input.Stage == "" {
			input.Stage = model.PlanningStageTaskPlanDoc
		}
		if input.PlanDocumentID == "" && state.task.PlanDocumentID != nil {
			input.PlanDocumentID = strings.TrimSpace(*state.task.PlanDocumentID)
		}
		if state.epic != nil {
			input.SpecDocumentID = strings.TrimSpace(derefString(state.epic.SpecDocumentID))
			input.SpecVersionID = strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID))
		}
		payload, _ := json.Marshal(input)
		state.run.Input = payload
		return input, nil
	}

	if state.run.TargetType != "epic" || state.epic == nil {
		return input, nil
	}

	input.SpecDocumentID = strings.TrimSpace(derefString(state.epic.SpecDocumentID))
	input.SpecVersionID = strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID))
	if input.PlanningMethodology == "" {
		input.PlanningMethodology = model.PlanningMethodologyStructuredV1
	}

	payload, _ := json.Marshal(input)
	state.run.Input = payload

	return input, nil
}

func (a *AgentRunActivities) normalizeEpicSpecState(ctx context.Context, state *resolvedRunState) error {
	if state == nil || state.epic == nil || a.epicRepo == nil {
		return nil
	}

	changed := false
	if docID := strings.TrimSpace(derefString(state.epic.SpecDocumentID)); docID != "" && a.docsDocRepo != nil {
		doc, err := a.docsDocRepo.GetByID(ctx, docID)
		if err != nil {
			return err
		}
		if doc == nil {
			state.epic.SpecDocumentID = nil
			state.epic.ApprovedSpecVersionID = nil
			changed = true
		}
	}

	if versionID := strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID)); versionID != "" && a.docsVersionRepo != nil {
		version, err := a.docsVersionRepo.GetByID(ctx, versionID)
		if err != nil {
			return err
		}
		if version == nil || strings.TrimSpace(derefString(state.epic.SpecDocumentID)) == "" || version.DocumentID != strings.TrimSpace(derefString(state.epic.SpecDocumentID)) {
			state.epic.ApprovedSpecVersionID = nil
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return a.epicRepo.Update(ctx, state.epic)
}

func (a *AgentRunActivities) buildInitialInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if state != nil && state.nativeSelectivePathEnabled {
		return a.buildNativeSelectivePhaseGuidance(ctx, state, input)
	}
	tools := effectiveToolSet(state.resolved, input.AllowedTools)
	if strings.TrimSpace(input.FlowOutputKind) != "" {
		return a.buildFlowOutputInstructions(ctx, state, input)
	}
	if state.task != nil {
		if tools[workerpkg.ToolPublishTaskPlanDoc] {
			return a.buildTaskPlannerInstructions(ctx, state, input)
		}
		return a.buildTaskExecutionInstructions(ctx, state, input)
	}
	if state.run.TargetType != "epic" || state.epic == nil {
		return runInputAdditionalContext(state.run.Input), nil
	}
	if !tools[workerpkg.ToolPublishPRDDraft] || (!tools[workerpkg.ToolPublishTaskPlan] && !tools[workerpkg.ToolPublishStoryPlan]) {
		return runInputAdditionalContext(state.run.Input), nil
	}
	return a.buildAgenticEpicPlannerInstructions(ctx, state, input)
}

func (a *AgentRunActivities) buildNativeSelectivePhaseGuidance(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if state == nil || state.run == nil {
		return "", nil
	}
	if state.task != nil {
		return a.buildNativeTaskPlannerPhaseGuidance(ctx, state, input)
	}
	if state.run.TargetType == "epic" && state.epic != nil {
		return a.buildNativeEpicPlannerPhaseGuidance(ctx, state, input)
	}
	return runInputAdditionalContext(state.run.Input), nil
}

func (a *AgentRunActivities) buildFlowOutputInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	switch strings.TrimSpace(input.FlowOutputKind) {
	case "pm.task_completion_followups":
		return a.buildTaskCompletionInstructions(state, input), nil
	case "crm.deal_review_actions":
		return a.buildCRMDealReviewInstructions(ctx, state, input)
	default:
		return runInputAdditionalContext(state.run.Input), nil
	}
}

func (a *AgentRunActivities) buildTaskCompletionInstructions(state *resolvedRunState, input planningRunInput) string {
	var sections []string
	sections = append(sections, "Review this completed task and return JSON only with the shape {\"summary\":\"...\",\"followups\":[{\"title\":\"...\",\"description\":\"...\",\"task_type\":\"chore\",\"priority\":\"medium\"}]}. Legacy story_type is still accepted.")
	sections = append(sections, "Only propose internal PM/docs/support follow-up work. Do not publish customer-facing docs or website changes directly.")
	if state.task != nil {
		sections = append(sections, fmt.Sprintf("Task: %s", state.task.Name))
		if state.task.Description != nil {
			if description := tiptap.RichTextToMarkdown(*state.task.Description); description != "" {
				sections = append(sections, "Task description:\n"+truncatePlanningText(description, 8000))
			}
		}
		if state.task.EpicID != nil && *state.task.EpicID != "" {
			sections = append(sections, fmt.Sprintf("Epic ID: %s", *state.task.EpicID))
		}
	}
	if strings.TrimSpace(input.AdditionalContext) != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}
	return strings.Join(sections, "\n\n")
}

func (a *AgentRunActivities) buildCRMDealReviewInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	deal, err := a.crmDealRepo.GetByID(ctx, state.run.TargetID)
	if err != nil {
		return "", err
	}
	if deal == nil {
		return "", fmt.Errorf("deal not found")
	}
	dealID := deal.ID
	signals, _, err := a.crmSignalRepo.ListSignals(ctx, state.run.WorkspaceID, model.CRMBuyerSignalListFilters{
		DealID: &dealID,
	}, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		return "", err
	}
	var sections []string
	sections = append(sections, "Review this CRM deal and return JSON only with the shape {\"summary\":\"...\",\"recommended_stage_id\":\"optional-stage-id\",\"note\":\"optional internal note\"}.")
	sections = append(sections, "Do not propose outbound messaging, contact creation, or sequence enrollment in this run.")
	sections = append(sections, fmt.Sprintf("Deal: %s", deal.Name))
	if deal.Stage != nil {
		sections = append(sections, fmt.Sprintf("Current stage: %s (%s)", deal.Stage.Name, deal.Stage.ID))
	}
	if deal.Pipeline != nil && len(deal.Pipeline.Stages) > 0 {
		lines := make([]string, 0, len(deal.Pipeline.Stages))
		for _, stage := range deal.Pipeline.Stages {
			lines = append(lines, fmt.Sprintf("- %s (%s)", stage.Name, stage.ID))
		}
		sections = append(sections, "Available stages:\n"+strings.Join(lines, "\n"))
	}
	if len(signals) > 0 {
		lines := make([]string, 0, len(signals))
		for _, signal := range signals {
			lines = append(lines, fmt.Sprintf("- %s: %s (confidence %.2f)", signal.SignalType, signal.Summary, signal.Confidence))
		}
		sections = append(sections, "Recent buyer signals:\n"+strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(input.AdditionalContext) != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}
	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildTaskPlannerInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if state.task == nil {
		return "", fmt.Errorf("task planner requires a task target")
	}

	var sections []string
	sections = append(sections, fmt.Sprintf("Run mode: %s", state.run.InvocationMode))
	if state.run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "The shared run drawer is available for live questions, draft previews, inline approvals, and change requests.")
		sections = append(sections, "Treat this as one transcript-driven planning run. Humans approve and request changes with normal chat replies in this same transcript.")
		sections = append(sections, "Only a clear explicit approval counts as approval. Requested changes, critique, concerns, or ambiguous replies mean the draft is not approved yet.")
	}
	sections = append(sections, "Choose the next step from the transcript, task details, parent epic context, linked docs, comments, code context, and tool results.")
	sections = append(sections, "Use this sequence unless the human explicitly redirects you: clarify scope if needed, draft or refine the task planning doc, publish it with publish_task_plan_doc, wait for inline approval, then stop. The platform will persist and link the approved preview to the canonical task planning doc automatically.")
	sections = append(sections, "Keep approvals soft and inline. When you need approval, call request_approval with phase=\"task_doc\" and stop after the request.")
	sections = append(sections, "Treat request_approval as the final action in that turn. Do not call more tools after it, and do not append extra approval-choice prose after requesting approval.")
	sections = append(sections, "Use publish_task_plan_doc for reviewable right-pane task planning documents.")
	sections = append(sections, "publish_task_plan_doc must receive a JSON object where content is the full markdown planning draft under review. Do not send title-only payloads or empty content.")
	sections = append(sections, "Treat parent epic details, the epic PRD, and epic-linked docs as background context only. Use them to understand constraints, inherited requirements, and non-goals, but do not copy them wholesale into the task planning document unless they directly affect this task's implementation.")
	sections = append(sections, "Ground the planning document primarily in the task description, task comments, task-linked docs, and the current codebase context. Keep the output focused on this task's implementation plan.")

	contextSections, err := a.buildTaskPlannerContextSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	sections = append(sections, contextSections...)

	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildTaskPlannerContextSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	if state.task == nil {
		return nil, fmt.Errorf("task planner requires a task target")
	}

	var sections []string
	if strings.TrimSpace(input.PlanDocumentID) != "" {
		sections = append(sections, fmt.Sprintf("Canonical task planning document ID: %s", input.PlanDocumentID))
		content, err := a.docsContentRepo.GetByDocumentID(ctx, input.PlanDocumentID)
		if err != nil {
			return nil, err
		}
		if content != nil && strings.TrimSpace(content.ContentText) != "" {
			sections = append(sections, "Current task planning draft already in Docs:\n"+truncatePlanningText(content.ContentText, 12000))
			sections = append(sections, "Resume from the existing planning doc draft instead of starting over unless the human explicitly wants a reset.")
		}
	} else {
		sections = append(sections, "No canonical task planning doc exists yet. Keep the draft in chat-backed preview artifacts until approval; the platform will create, persist, and link the approved artifact.")
	}

	if input.AdditionalContext != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}

	sections = append(sections, fmt.Sprintf("Task: %s", state.task.Name))
	if state.task.Description != nil {
		if description := tiptap.RichTextToMarkdown(*state.task.Description); description != "" {
			sections = append(sections, "Task description:\n"+truncatePlanningText(description, 8000))
		}
	}
	if state.task.TeamID != nil && strings.TrimSpace(*state.task.TeamID) != "" {
		sections = append(sections, fmt.Sprintf("Task team ID: %s", strings.TrimSpace(*state.task.TeamID)))
	}

	if state.epic != nil {
		sections = append(sections, fmt.Sprintf("Parent epic: %s", state.epic.Name))
		if state.epic.Description != nil {
			if description := tiptap.RichTextToMarkdown(*state.epic.Description); description != "" {
				sections = append(sections, "Parent epic description:\n"+truncatePlanningText(description, 8000))
			}
		}
	}

	if input.SpecVersionID != "" {
		version, err := a.docsVersionRepo.GetByID(ctx, input.SpecVersionID)
		if err != nil {
			return nil, err
		}
		if version != nil && strings.TrimSpace(version.ContentText) != "" {
			sections = append(sections, fmt.Sprintf("Approved epic PRD version ID: %s", version.ID))
			sections = append(sections, "Approved epic PRD snapshot:\n"+truncatePlanningText(version.ContentText, 16000))
		}
	} else if input.SpecDocumentID != "" {
		content, err := a.docsContentRepo.GetByDocumentID(ctx, input.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if content != nil && strings.TrimSpace(content.ContentText) != "" {
			sections = append(sections, fmt.Sprintf("Parent epic PRD document ID: %s", input.SpecDocumentID))
			sections = append(sections, "Current epic PRD draft:\n"+truncatePlanningText(content.ContentText, 12000))
		}
	}

	taskLinkedDocs, err := a.renderObjectLinkedDocsContext(ctx, state.run.WorkspaceID, model.LinkedObjectTask, state.task.ID, input.PlanDocumentID)
	if err != nil {
		return nil, err
	}
	if taskLinkedDocs != "" {
		sections = append(sections, "Other docs linked directly to this task:\n"+taskLinkedDocs)
	}

	if state.epic != nil {
		epicLinkedDocs, err := a.renderLinkedDocsContext(ctx, state.run.WorkspaceID, state.epic.ID, input.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if epicLinkedDocs != "" {
			sections = append(sections, "Other docs linked to the parent epic:\n"+epicLinkedDocs)
		}
	}

	commentsContext, err := a.renderTaskCommentsContext(ctx, state.task.ID)
	if err != nil {
		return nil, err
	}
	if commentsContext != "" {
		sections = append(sections, "Task comments:\n"+commentsContext)
	}

	repoContext, err := a.buildDraftSpecCodeContext(ctx, state, strings.Join([]string{
		state.task.Name,
		tiptap.RichTextToMarkdown(derefString(state.task.Description)),
		input.AdditionalContext,
	}, "\n\n"))
	if err != nil {
		return nil, err
	}
	if repoContext != "" {
		sections = append(sections, "Current implementation context from the live repository:\n"+repoContext)
	}

	return sections, nil
}

func (a *AgentRunActivities) buildNativeTaskPlannerPhaseGuidance(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	phaseName := strings.TrimSpace(nativeActiveSkillPlanningStage(state, input.Stage))
	if phaseName == "" {
		phaseName = model.PlanningStageTaskPlanDoc
	}
	sections := []string{
		fmt.Sprintf("Current planning phase: %s", phaseName),
		"Phase objective: refine a task-scoped implementation planning document, publish it with publish_task_plan_doc, and stop at inline approval.",
		"Treat this as a transcript-driven task planning run. Continue from the latest human reply, active draft, linked task context, and repository evidence rather than restarting the plan from scratch.",
		"Next-step rule: clarify scope only when blocked, otherwise update the active task planning draft, publish the full replacement preview, and request inline approval when the document is ready.",
		"Approval rule: use request_approval with phase=\"task_doc\" only after publish_task_plan_doc in the same turn. Treat request_approval as the final action in that turn.",
		"Contract reminder: publish_task_plan_doc must receive one JSON object whose content field contains the full markdown draft under review.",
		"Focus rule: keep the planning document grounded in the task description, task comments, task-linked docs, parent-epic constraints that matter to this task, and the current codebase context.",
	}
	if state.run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "Interactive approval semantics: explicit approval advances the run; change requests, critique, concerns, and ambiguous replies mean the draft is still unapproved and must be revised in the same transcript.")
	}
	contextSections, err := a.buildTaskPlannerContextSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	sections = append(sections, contextSections...)
	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildTaskExecutionInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if state.task == nil {
		return runInputAdditionalContext(state.run.Input), nil
	}

	sections, err := a.buildTaskRunContextSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildTaskRunContextSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	var sections []string
	additionalContext := strings.TrimSpace(input.AdditionalContext)
	if additionalContext == "" && state != nil && state.run != nil {
		additionalContext = runInputAdditionalContext(state.run.Input)
	}
	if additionalContext != "" {
		sections = append(sections, "Operator notes:\n"+additionalContext)
	}
	baseBranch := strings.TrimSpace(derefString(state.run.BaseBranch))
	workingBranch := strings.TrimSpace(derefString(state.run.WorkingBranch))
	if baseBranch != "" || workingBranch != "" {
		switch {
		case baseBranch != "" && workingBranch != "":
			sections = append(sections, fmt.Sprintf("Repository branches: base `%s`, working `%s`.", baseBranch, workingBranch))
		case workingBranch != "":
			sections = append(sections, fmt.Sprintf("Repository working branch: `%s`.", workingBranch))
		case baseBranch != "":
			sections = append(sections, fmt.Sprintf("Repository base branch: `%s`.", baseBranch))
		}
	}

	planDocumentID := strings.TrimSpace(derefString(state.task.PlanDocumentID))
	if planDocumentID != "" && a.docsContentRepo != nil {
		title := ""
		if a.docsDocRepo != nil {
			doc, err := a.docsDocRepo.GetByID(ctx, planDocumentID)
			if err != nil {
				return nil, err
			}
			if doc != nil {
				title = strings.TrimSpace(doc.Title)
			}
		}
		content, err := a.docsContentRepo.GetByDocumentID(ctx, planDocumentID)
		if err != nil {
			return nil, err
		}
		if markdown := docsContentMarkdown(content); markdown != "" {
			header := fmt.Sprintf("Canonical task planning document ID: %s", planDocumentID)
			if title != "" {
				header = fmt.Sprintf("Canonical task planning document: %s [%s]", title, planDocumentID)
			}
			sections = append(sections, header+"\n"+truncatePlanningText(markdown, 12000))
		}
	}

	taskLinkedDocs, err := a.renderObjectLinkedDocsContext(ctx, state.run.WorkspaceID, model.LinkedObjectTask, state.task.ID, planDocumentID)
	if err != nil {
		return nil, err
	}
	if taskLinkedDocs != "" {
		sections = append(sections, "Other docs linked directly to this task:\n"+taskLinkedDocs)
	}

	return sections, nil
}

func (a *AgentRunActivities) buildAgenticEpicPlannerInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	var sections []string

	sections = append(sections, fmt.Sprintf("Run mode: %s", state.run.InvocationMode))
	if state.run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "The shared run drawer is available for live questions, draft previews, inline approvals, and change requests.")
		sections = append(sections, "Treat this as one transcript-driven planning run. There is no hidden planner phase machine controlling the next step for you.")
		sections = append(sections, "Humans approve and request changes with normal chat replies in this same transcript. Do not tell them to use a separate approval workflow, button, or UI gate.")
		sections = append(sections, "Only a clear explicit approval counts as approval. Any requested change, concern, critique, follow-up question, or ambiguous reply means the current phase is not approved yet.")
	}
	sections = append(sections, "Choose the next step from the transcript, current epic state, linked docs, existing tasks, and tool results.")
	sections = append(sections, "Use this sequence unless the human explicitly redirects you: clarify scope if needed, draft/refine the PRD, publish it with publish_prd_draft, wait for inline PRD approval, let the platform persist the approved PRD artifact to the canonical epic doc, propose the implementation task plan, publish it with publish_task_plan, wait for inline task approval, then let the platform apply the approved task plan artifact and create tasks.")
	sections = append(sections, "Keep approvals soft and inline. When you need approval, call request_approval with phase=\"prd\" or phase=\"tasks\" and stop after the request.")
	sections = append(sections, "Treat request_approval as the final action in that turn. Do not call more tools after it in the same turn. Do not append extra approval-choice prose after requesting approval.")
	sections = append(sections, "After explicit PRD approval, continue automatically to task planning in the same run. Do not ask whether to proceed to tasks unless the human explicitly redirects scope.")
	sections = append(sections, "After PRD approval is persisted, your next turn must continue into task planning. Either ask the next blocking questions with request_user_input or publish_task_plan. Do not complete the run immediately after PRD approval.")
	sections = append(sections, "If the latest human reply requests changes to the PRD or task plan, revise the active artifact, republish the full replacement preview, and request_approval again when ready. Do not end the run with prose-only acknowledgement after change feedback.")
	sections = append(sections, "Use publish_prd_draft for PRD markdown previews and publish_task_plan for task plan JSON previews.")
	sections = append(sections, "publish_task_plan must receive one complete JSON object payload in that tool call. Do not send title-only payloads, raw string wrappers, partial JSON, or stringified blobs. Put the full plan under content with a non-empty summary and proposed_tasks array.")
	sections = append(sections, "proposed_tasks must be an array of full task objects. Never send arrays of strings, refs, placeholders, key names, or partial fragments. If publish_task_plan fails validation, correct the payload and retry with one complete valid task-plan object before requesting approval.")
	sections = append(sections, "Before approval, keep drafts in chat-backed preview artifacts only. After approval, the platform applies the approved artifact; do not replay approved PRDs or task plans through mutation tools.")

	contextSections, hasSpecContent, err := a.buildEpicPlannerContextSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	sections = append(sections, contextSections...)
	sections = append(sections, formatInteractivePlanningFacts(input, hasSpecContent, len(state.epicTasks)))
	sections = append(sections, nativeEpicPlannerNextStepGuidance(input, hasSpecContent, len(state.epicTasks) > 0))
	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildEpicPlannerContextSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, bool, error) {
	var sections []string
	var hasSpecContent bool
	if input.SpecDocumentID != "" {
		sections = append(sections, fmt.Sprintf("Existing canonical spec document ID: %s", input.SpecDocumentID))

		// Inject durable draft content when a spec doc exists but is not approved.
		if input.SpecVersionID == "" {
			if content, err := a.docsContentRepo.GetByDocumentID(ctx, input.SpecDocumentID); err == nil && content != nil && strings.TrimSpace(content.ContentText) != "" {
				hasSpecContent = true
				sections = append(sections, "IMPORTANT: A PRD draft already exists in the spec document but was never formally approved. Resume from the current draft instead of starting over. Present the draft, revise it if needed, and request PRD approval before any task planning.")
				sections = append(sections, "Current spec draft:\n"+truncatePlanningText(content.ContentText, 12000))
			}
		}
	}
	if input.SpecVersionID != "" {
		sections = append(sections, fmt.Sprintf("Approved spec version ID: %s", input.SpecVersionID))
		sections = append(sections, "IMPORTANT: A previously approved spec already exists. The PRD is LOCKED. Do not redraft, rewrite, or re-approve it. Use it as the read-only source of truth for task planning. If the human asks to revise the PRD, explain the spec is approved and suggest creating a follow-up epic instead, unless they insist.")
		sections = append(sections, "If the current facts show the PRD is already approved, treat persistence as complete and continue from that state. Do not replay the PRD through mutation tools.")
	}
	if state.epic != nil && state.epic.TeamID != nil && strings.TrimSpace(*state.epic.TeamID) != "" {
		sections = append(sections, fmt.Sprintf("Epic team ID: %s", strings.TrimSpace(*state.epic.TeamID)))
	} else {
		sections = append(sections, "This epic does not currently have a team. Before creating tasks, call list_workspace_teams and ask the human to choose the correct team inline in chat.")
	}
	if input.AdditionalContext != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}

	linkedDocs, err := a.renderLinkedDocsContext(ctx, state.run.WorkspaceID, state.epic.ID, input.SpecDocumentID)
	if err != nil {
		return nil, false, err
	}
	if linkedDocs != "" {
		sections = append(sections, "Other docs linked to this epic:\n"+linkedDocs)
	}

	linkedTickets, err := a.renderLinkedTicketsContext(ctx, state)
	if err != nil {
		return nil, false, err
	}
	if linkedTickets != "" {
		sections = append(sections, "Support and customer context already linked to this epic:\n"+linkedTickets)
	}

	repoContext, err := a.buildDraftSpecCodeContext(ctx, state, strings.Join([]string{
		state.epic.Name,
		tiptap.RichTextToMarkdown(derefString(state.epic.Description)),
		linkedDocs,
		linkedTickets,
		input.AdditionalContext,
	}, "\n\n"))
	if err != nil {
		return nil, false, err
	}
	if repoContext != "" {
		sections = append(sections, "Current implementation context from the planning repository:\n"+repoContext)
	}

	if len(state.epicTasks) > 0 {
		lines := make([]string, 0, len(state.epicTasks))
		for _, task := range state.epicTasks {
			lines = append(lines, fmt.Sprintf("- %s [%s]", task.Name, task.ID))
		}
		sections = append(sections, fmt.Sprintf("IMPORTANT: %d tasks already exist under this epic. Do NOT recreate them. Only create new tasks if the human explicitly requests additions.", len(state.epicTasks)))
		sections = append(sections, "Tasks already linked to this epic:\n"+strings.Join(lines, "\n"))
	}

	return sections, hasSpecContent, nil
}

func nativeEpicPlannerNextStepGuidance(input planningRunInput, hasSpecContent bool, hasTasks bool) string {
	hasApprovedSpec := input.SpecVersionID != ""
	hasSpecDoc := input.SpecDocumentID != ""
	switch {
	case hasApprovedSpec && hasTasks:
		return "Next-step guidance: the PRD is approved and tasks already exist. Do not redraft the PRD or recreate existing tasks. Enter clarification or extension mode, inspect current tasks if needed, and only add new tasks if the human explicitly asks for them."
	case hasApprovedSpec && !hasTasks:
		return "Next-step guidance: the PRD is approved and no tasks exist yet. Skip PRD drafting entirely and proceed directly to task planning from the approved spec and current codebase context."
	case hasSpecDoc && hasSpecContent:
		return "Next-step guidance: a draft PRD exists but it is not approved yet. Resume from the current draft, present or revise it, and request PRD approval before any task planning."
	default:
		return "Next-step guidance: no approved PRD exists yet. Follow the full loop from clarification through PRD drafting, preview, revision if needed, and approval."
	}
}

func nativeEpicPlannerPhaseName(input planningRunInput, hasSpecContent bool, hasTasks bool) string {
	hasApprovedSpec := input.SpecVersionID != ""
	hasSpecDoc := input.SpecDocumentID != ""
	switch {
	case hasApprovedSpec && hasTasks:
		return "task_extension"
	case hasApprovedSpec && !hasTasks:
		return model.PlanningStagePlanTasks
	case hasSpecDoc && hasSpecContent:
		return "prd_revision"
	default:
		return model.PlanningStageDraftSpec
	}
}

func (a *AgentRunActivities) buildNativeEpicPlannerPhaseGuidance(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	contextSections, hasSpecContent, err := a.buildEpicPlannerContextSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	hasTasks := len(state.epicTasks) > 0
	phaseName := nativeEpicPlannerPhaseName(input, hasSpecContent, hasTasks)
	sections := []string{
		fmt.Sprintf("Current planning phase: %s", phaseName),
		"Phase objective: move the epic to the next durable planning checkpoint using the current transcript, approved artifacts, linked context, and repository evidence.",
		"Transcript rule: continue from the latest human reply and current planning state rather than restarting the PRD or task plan from scratch.",
		"Approval rule: use request_approval with phase=\"prd\" or phase=\"tasks\" only after publishing the same-turn preview artifact that is being reviewed. Treat request_approval as the final action in that turn.",
		"PRD contract reminder: use publish_prd_draft for markdown previews that the human will review inline.",
		"Task-plan contract reminder: publish_task_plan must receive one complete JSON object with non-empty summary and proposed_tasks fields before task-plan approval is requested.",
		"Post-approval rule: once PRD approval is persisted, continue directly into task planning unless the human explicitly redirects scope. Do not end the run immediately after PRD approval.",
		"Revision rule: if the latest human reply asks for changes to the active PRD or task plan, revise the active artifact, republish the full replacement preview, and request approval again when ready.",
	}
	if state.run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "Interactive approval semantics: only explicit approval advances the phase. Change requests, critique, concerns, and ambiguous replies keep the current phase active.")
	}
	sections = append(sections, formatInteractivePlanningFacts(input, hasSpecContent, len(state.epicTasks)))
	sections = append(sections, nativeEpicPlannerNextStepGuidance(input, hasSpecContent, hasTasks))
	sections = append(sections, contextSections...)
	return strings.Join(sections, "\n\n"), nil
}

func formatInteractivePlanningFacts(input planningRunInput, hasDraftSpec bool, taskCount int) string {
	facts := []string{
		fmt.Sprintf("- approved_spec_exists=%t", strings.TrimSpace(input.SpecVersionID) != ""),
		fmt.Sprintf("- draft_spec_exists=%t", hasDraftSpec),
		fmt.Sprintf("- existing_task_count=%d", taskCount),
	}
	if strings.TrimSpace(input.SpecDocumentID) != "" {
		facts = append(facts, fmt.Sprintf("- spec_document_id=%s", strings.TrimSpace(input.SpecDocumentID)))
	}
	if strings.TrimSpace(input.SpecVersionID) != "" {
		facts = append(facts, fmt.Sprintf("- approved_spec_version_id=%s", strings.TrimSpace(input.SpecVersionID)))
	}
	return "Current durable planning facts:\n" + strings.Join(facts, "\n")
}

func (a *AgentRunActivities) preparePlanningRepository(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	if state.run.TargetType != "epic" || state.epic == nil {
		return nil
	}
	if state.epic.PlanningRepositoryID == nil || strings.TrimSpace(*state.epic.PlanningRepositoryID) == "" {
		return nil
	}

	repo, err := a.gitRepo.GetByID(ctx, state.run.WorkspaceID, *state.epic.PlanningRepositoryID)
	if err != nil {
		return err
	}
	if repo == nil {
		return fmt.Errorf("planning repository not found")
	}
	if repo.Archived || !repo.Selected {
		return fmt.Errorf("planning repository is not available")
	}

	integration, err := a.gitIntRepo.GetByID(ctx, state.run.WorkspaceID, repo.IntegrationID)
	if err != nil {
		return err
	}
	if integration == nil {
		return fmt.Errorf("planning repository integration not found")
	}

	token, err := a.mintAccessToken(ctx, integration)
	if err != nil {
		return fmt.Errorf("planning repository access token: %w", err)
	}

	state.repository = repo
	state.integration = integration
	state.accessToken = token
	state.run.RepositoryID = &repo.ID
	state.run.RepoFullName = &repo.FullName
	baseBranch := repo.DefaultBranch
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
	state.run.BaseBranch = &baseBranch
	return nil
}

func (a *AgentRunActivities) finalizePlanningRun(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	if strings.TrimSpace(input.FlowOutputKind) != "" {
		return a.finalizeFlowOutputRun(ctx, state, input)
	}
	if state.run.TargetType != "epic" || state.epic == nil {
		return nil
	}
	return a.finalizeAgenticEpicPlannerRun(ctx, state)
}

func (a *AgentRunActivities) finalizeAgenticEpicPlannerRun(ctx context.Context, state *resolvedRunState) error {
	if state.epic == nil {
		return nil
	}
	state.epic.LastPlanningRunID = &state.run.ID
	return a.epicRepo.Update(ctx, state.epic)
}

func (a *AgentRunActivities) finalizeFlowOutputRun(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	switch strings.TrimSpace(input.FlowOutputKind) {
	case "pm.task_completion_followups":
		var assessment model.TaskCompletionAssessment
		if err := json.Unmarshal(state.run.OutputSummary, &assessment); err != nil {
			return fmt.Errorf("decode task completion assessment: %w", err)
		}
		if strings.TrimSpace(assessment.Summary) == "" {
			return fmt.Errorf("task completion assessment is missing a summary")
		}
		return nil
	case "crm.deal_review_actions":
		var plan model.CRMDealReviewActionPlan
		if err := json.Unmarshal(state.run.OutputSummary, &plan); err != nil {
			return fmt.Errorf("decode CRM deal review plan: %w", err)
		}
		if strings.TrimSpace(plan.Summary) == "" {
			return fmt.Errorf("CRM deal review plan is missing a summary")
		}
		return nil
	default:
		return nil
	}
}

func (a *AgentRunActivities) ensureEpicSpecDocument(ctx context.Context, state *resolvedRunState, actorID string) (*model.DocsDocument, error) {
	if state.epic == nil {
		return nil, fmt.Errorf("epic context is required")
	}
	if state.epic.SpecDocumentID != nil && strings.TrimSpace(*state.epic.SpecDocumentID) != "" {
		doc, err := a.docsDocRepo.GetByID(ctx, *state.epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			return doc, nil
		}
	}

	space, err := a.docsSpaceRepo.GetBySlug(ctx, state.run.WorkspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = a.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: state.run.WorkspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := state.epic.TeamID
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	doc, err := a.docsDocRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: state.run.WorkspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(state.epic.Name) + " Product Spec",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("product_spec"),
		Tags:        model.DocsStringArray{"product-spec", "epic"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	state.epic.SpecDocumentID = &doc.ID
	if err := a.epicRepo.Update(ctx, state.epic); err != nil {
		return nil, err
	}
	if err := a.ensureEpicSpecLink(ctx, state.run.WorkspaceID, doc.ID, state.epic.ID, actorID); err != nil {
		return nil, err
	}

	return doc, nil
}

func (a *AgentRunActivities) ensureTaskPlanDocument(ctx context.Context, state *resolvedRunState, actorID string) (*model.DocsDocument, error) {
	if state.task == nil {
		return nil, fmt.Errorf("task not found")
	}
	if state.task.PlanDocumentID != nil && strings.TrimSpace(*state.task.PlanDocumentID) != "" {
		doc, err := a.docsDocRepo.GetByID(ctx, *state.task.PlanDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, actorID); err != nil {
				return nil, err
			}
			return doc, nil
		}
	}

	space, err := a.docsSpaceRepo.GetBySlug(ctx, state.run.WorkspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = a.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: state.run.WorkspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := state.task.TeamID
	if teamID == nil && state.epic != nil {
		teamID = state.epic.TeamID
	}
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	doc, err := a.docsDocRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: state.run.WorkspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(state.task.Name) + " Plan",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("task_plan"),
		Tags:        model.DocsStringArray{"task-plan", "task"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	state.task.PlanDocumentID = &doc.ID
	if err := a.taskRepo.Update(ctx, state.task); err != nil {
		return nil, err
	}
	if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, actorID); err != nil {
		return nil, err
	}
	return doc, nil
}

func (a *AgentRunActivities) ensureEpicSpecLink(ctx context.Context, workspaceID, documentID, epicID, actorID string) error {
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectEpic, epicID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.DocumentID == documentID {
			return nil
		}
	}
	_, err = a.docsLinkRepo.Create(ctx, &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: model.LinkedObjectEpic,
		LinkedObjectID:   epicID,
		LinkContext:      model.LinkContextCreatedFrom,
		CreatedBy:        actorID,
	})
	return err
}

func (a *AgentRunActivities) ensureTaskPlanLink(ctx context.Context, workspaceID, documentID, taskID, actorID string) error {
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectTask, taskID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.DocumentID == documentID {
			return nil
		}
	}
	_, err = a.docsLinkRepo.Create(ctx, &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: model.LinkedObjectTask,
		LinkedObjectID:   taskID,
		LinkContext:      model.LinkContextCreatedFrom,
		CreatedBy:        actorID,
	})
	return err
}

func (a *AgentRunActivities) renderLinkedDocsContext(ctx context.Context, workspaceID, epicID, excludeDocumentID string) (string, error) {
	return a.renderObjectLinkedDocsContext(ctx, workspaceID, model.LinkedObjectEpic, epicID, excludeDocumentID)
}

func (a *AgentRunActivities) renderObjectLinkedDocsContext(ctx context.Context, workspaceID, objectType, objectID, excludeDocumentID string) (string, error) {
	if a.docsLinkRepo == nil || a.docsDocRepo == nil || a.docsContentRepo == nil {
		return "", nil
	}
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return "", err
	}
	if len(links) == 0 {
		return "", nil
	}

	seen := make(map[string]bool, len(links))
	entries := make([]string, 0, len(links))
	for _, link := range links {
		if link.DocumentID == excludeDocumentID || seen[link.DocumentID] {
			continue
		}
		seen[link.DocumentID] = true

		doc, err := a.docsDocRepo.GetByID(ctx, link.DocumentID)
		if err != nil {
			return "", err
		}
		if doc == nil {
			continue
		}

		content, err := a.docsContentRepo.GetByDocumentID(ctx, doc.ID)
		if err != nil {
			return "", err
		}
		body := "(no content yet)"
		if markdown := docsContentMarkdown(content); markdown != "" {
			body = truncatePlanningText(markdown, 3000)
		}

		entries = append(entries, fmt.Sprintf("- %s [%s]\n%s", doc.Title, doc.ID, body))
		if len(entries) >= 5 {
			break
		}
	}

	return strings.Join(entries, "\n\n"), nil
}

func docsContentMarkdown(content *model.DocsContent) string {
	if content == nil {
		return ""
	}
	if markdown := tiptap.RichTextToMarkdown(string(content.Content)); markdown != "" {
		return markdown
	}
	return strings.TrimSpace(content.ContentText)
}

func (a *AgentRunActivities) renderTaskCommentsContext(ctx context.Context, taskID string) (string, error) {
	if strings.TrimSpace(taskID) == "" || a.commentRepo == nil {
		return "", nil
	}
	comments, err := a.commentRepo.List(ctx, "task", taskID)
	if err != nil {
		return "", err
	}
	if len(comments) == 0 {
		return "", nil
	}

	entries := make([]string, 0, len(comments))
	for idx, entry := range comments {
		authorName := strings.TrimSpace(entry.Author.FullName)
		if authorName == "" {
			authorName = entry.Comment.AuthorID
		}
		line := fmt.Sprintf("- %s: %s", authorName, truncatePlanningText(entry.Comment.Body, 320))
		if len(entry.Replies) > 0 {
			replyLines := make([]string, 0, len(entry.Replies))
			for _, reply := range entry.Replies {
				replyAuthor := strings.TrimSpace(reply.Author.FullName)
				if replyAuthor == "" {
					replyAuthor = reply.Comment.AuthorID
				}
				replyLines = append(replyLines, fmt.Sprintf("  - %s: %s", replyAuthor, truncatePlanningText(reply.Comment.Body, 220)))
			}
			line += "\nReplies:\n" + strings.Join(replyLines, "\n")
		}
		entries = append(entries, line)
		if idx >= 5 {
			break
		}
	}
	return strings.Join(entries, "\n\n"), nil
}

func (a *AgentRunActivities) renderLinkedTicketsContext(ctx context.Context, state *resolvedRunState) (string, error) {
	if len(state.epicTasks) == 0 {
		return "", nil
	}

	taskIDs := make([]string, 0, len(state.epicTasks))
	taskNames := make(map[string]string, len(state.epicTasks))
	for _, task := range state.epicTasks {
		taskIDs = append(taskIDs, task.ID)
		taskNames[task.ID] = task.Name
	}

	tickets, err := a.conversationRepo.ListByLinkedStoryIDs(ctx, state.run.WorkspaceID, taskIDs)
	if err != nil {
		return "", err
	}
	if len(tickets) == 0 {
		return "", nil
	}

	entries := make([]string, 0, len(tickets))
	for idx, ticket := range tickets {
		taskName := ""
		if ticket.LinkedTaskID != nil {
			taskName = taskNames[*ticket.LinkedTaskID]
		}

		header := fmt.Sprintf("- Ticket #%d: %s [status=%s priority=%s]", ticket.DisplayID, ticket.Subject, ticket.Status, ticket.Priority)
		if taskName != "" {
			header += fmt.Sprintf(" linked_task=%q", taskName)
		}
		if ticket.CustomerEmail != nil && strings.TrimSpace(*ticket.CustomerEmail) != "" {
			header += fmt.Sprintf(" customer=%s", *ticket.CustomerEmail)
		}

		entry := header
		messages, err := a.messageRepo.ListByConversation(ctx, state.run.WorkspaceID, ticket.ID, true)
		if err != nil {
			return "", err
		}
		if len(messages) > 0 {
			start := len(messages) - 2
			if start < 0 {
				start = 0
			}
			lines := make([]string, 0, len(messages)-start)
			for _, message := range messages[start:] {
				scope := "public"
				if message.IsInternal {
					scope = "internal"
				}
				lines = append(lines, fmt.Sprintf("  - [%s/%s] %s", message.SenderType, scope, truncatePlanningText(message.Content, 280)))
			}
			entry += "\nRecent messages:\n" + strings.Join(lines, "\n")
		}

		entries = append(entries, entry)
		if idx >= 4 {
			break
		}
	}

	return strings.Join(entries, "\n\n"), nil
}

func (a *AgentRunActivities) buildPlanningCodeContext(ctx context.Context, state *resolvedRunState, specText string) (string, error) {
	if state.repository == nil || state.integration == nil {
		return "", nil
	}

	workDir, err := workerpkg.PrepareWorkspace(ctx, state.integration, state.repository.FullName, state.accessToken)
	if err != nil {
		return "", fmt.Errorf("prepare planning repository workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	treeEntries, err := collectPlanningTree(workDir, 3, 80)
	if err != nil {
		return "", err
	}

	manifestCandidates := []string{
		"package.json", "pnpm-workspace.yaml", "turbo.json", "tsconfig.json",
		"go.mod", "go.work", "Cargo.toml", "pyproject.toml", "requirements.txt",
		"Dockerfile", "docker-compose.yml", "docker-compose.yaml",
		"README.md", "WORKFLOW.md",
	}
	manifestSnippets := collectStaticFileSnippets(workDir, manifestCandidates, 6, 1200)

	relevantFiles, err := selectRelevantPlanningFiles(workDir, specText)
	if err != nil {
		return "", err
	}
	fileSnippets := collectStaticFileSnippets(workDir, relevantFiles, 8, 1400)

	var sections []string
	sections = append(sections, fmt.Sprintf("Repository root: %s", state.repository.FullName))
	if len(treeEntries) > 0 {
		sections = append(sections, "Repository structure:\n"+strings.Join(treeEntries, "\n"))
	}
	if len(manifestSnippets) > 0 {
		sections = append(sections, "Key manifests and architecture anchors:\n"+strings.Join(manifestSnippets, "\n\n"))
	}
	if len(fileSnippets) > 0 {
		sections = append(sections, "Relevant implementation files:\n"+strings.Join(fileSnippets, "\n\n"))
	} else {
		sections = append(sections, "Relevant implementation files: no confident file matches were found from the approved spec. Treat uncertain areas as risks or open questions.")
	}

	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildDraftSpecCodeContext(ctx context.Context, state *resolvedRunState, seedText string) (string, error) {
	if state.repository == nil || state.integration == nil {
		return "", nil
	}

	workDir, err := workerpkg.PrepareWorkspace(ctx, state.integration, state.repository.FullName, state.accessToken)
	if err != nil {
		return "", fmt.Errorf("prepare draft-spec repository workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	treeEntries, err := collectPlanningTree(workDir, 2, 50)
	if err != nil {
		return "", err
	}

	manifestCandidates := []string{
		"package.json", "go.mod", "go.work", "Cargo.toml", "pyproject.toml",
		"README.md", "WORKFLOW.md",
	}
	manifestSnippets := collectStaticFileSnippets(workDir, manifestCandidates, 4, 900)

	relevantFiles, err := selectRelevantPlanningFiles(workDir, seedText)
	if err != nil {
		return "", err
	}
	fileLimit := 5
	if len(relevantFiles) > fileLimit {
		relevantFiles = relevantFiles[:fileLimit]
	}
	fileSnippets := collectStaticFileSnippets(workDir, relevantFiles, fileLimit, 900)

	var sections []string
	sections = append(sections, fmt.Sprintf("Repository root: %s", state.repository.FullName))
	if len(treeEntries) > 0 {
		sections = append(sections, "High-level repository structure:\n"+strings.Join(treeEntries, "\n"))
	}
	if len(manifestSnippets) > 0 {
		sections = append(sections, "Key product and platform anchors:\n"+strings.Join(manifestSnippets, "\n\n"))
	}
	if len(fileSnippets) > 0 {
		sections = append(sections, "Likely relevant product surface files:\n"+strings.Join(fileSnippets, "\n\n"))
	} else {
		sections = append(sections, "Likely relevant product surface files: no confident matches were found. Treat repo-specific assumptions as risks or open questions.")
	}

	return strings.Join(sections, "\n\n"), nil
}

func collectPlanningTree(root string, maxDepth, maxEntries int) ([]string, error) {
	type item struct {
		path  string
		depth int
	}
	var items []item
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if shouldSkipPlanningPath(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth >= maxDepth && d.IsDir() {
			items = append(items, item{path: rel + "/", depth: depth})
			return filepath.SkipDir
		}
		if len(items) >= maxEntries {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		label := rel
		if d.IsDir() {
			label += "/"
		}
		items = append(items, item{path: label, depth: depth})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].path < items[j].path })
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprintf("%s%s", strings.Repeat("  ", item.depth), item.path))
	}
	return out, nil
}

func collectStaticFileSnippets(root string, candidates []string, limit, maxChars int) []string {
	results := make([]string, 0, limit)
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		fullPath := filepath.Join(root, candidate)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		results = append(results, fmt.Sprintf("[%s]\n%s", candidate, truncatePlanningText(string(content), maxChars)))
		if len(results) >= limit {
			break
		}
	}
	return results
}

func selectRelevantPlanningFiles(root, specText string) ([]string, error) {
	keywords := planningKeywords(specText)
	type candidate struct {
		path  string
		score int
	}
	candidates := make([]candidate, 0, 64)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || shouldSkipPlanningPath(rel) || !looksLikePlanningSourceFile(rel) {
			return nil
		}

		score := planningFileBaseScore(rel)
		lowerRel := strings.ToLower(rel)
		for _, keyword := range keywords {
			if strings.Contains(lowerRel, keyword) {
				score += 8
			}
		}
		if score <= 0 {
			return nil
		}
		candidates = append(candidates, candidate{path: rel, score: score})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].path < candidates[j].path
		}
		return candidates[i].score > candidates[j].score
	})

	limit := 8
	if len(candidates) < limit {
		limit = len(candidates)
	}
	out := make([]string, 0, limit)
	seen := make(map[string]bool, limit)
	for _, item := range candidates {
		if seen[item.path] {
			continue
		}
		seen[item.path] = true
		out = append(out, item.path)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func planningKeywords(specText string) []string {
	normalized := strings.ToLower(specText)
	replacer := strings.NewReplacer(
		"\n", " ", "\t", " ", ",", " ", ".", " ", ":", " ", ";", " ", "(", " ", ")", " ",
		"{", " ", "}", " ", "[", " ", "]", " ", "/", " ", "\\", " ", "-", " ", "_", " ",
	)
	normalized = replacer.Replace(normalized)
	words := strings.Fields(normalized)
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "that": true, "this": true, "from": true,
		"into": true, "will": true, "story": true, "stories": true, "spec": true, "product": true,
		"epic": true, "user": true, "users": true, "should": true, "have": true, "must": true,
		"plan": true, "planning": true, "acceptance": true, "criteria": true, "when": true, "then": true,
		"given": true, "goal": true, "goals": true, "risk": true, "risks": true,
	}
	freq := make(map[string]int)
	for _, word := range words {
		if len(word) < 4 || stop[word] {
			continue
		}
		freq[word]++
	}
	type pair struct {
		word  string
		count int
	}
	pairs := make([]pair, 0, len(freq))
	for word, count := range freq {
		pairs = append(pairs, pair{word: word, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].word < pairs[j].word
		}
		return pairs[i].count > pairs[j].count
	})
	limit := 12
	if len(pairs) < limit {
		limit = len(pairs)
	}
	keywords := make([]string, 0, limit)
	for idx := 0; idx < limit; idx++ {
		keywords = append(keywords, pairs[idx].word)
	}
	return keywords
}

func shouldSkipPlanningPath(rel string) bool {
	rel = filepath.ToSlash(strings.ToLower(rel))
	skipParts := []string{
		".git/", "node_modules/", "vendor/", "dist/", "build/", ".next/", ".turbo/", ".cache/",
		"coverage/", "tmp/", "temp/", "bin/", "public/", "assets/", "storybook-static/",
	}
	for _, part := range skipParts {
		if strings.Contains(rel, part) {
			return true
		}
	}
	return false
}

func looksLikePlanningSourceFile(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".json", ".yaml", ".yml", ".md", ".py", ".rb", ".java", ".kt", ".rs":
		return true
	default:
		return false
	}
}

func planningFileBaseScore(rel string) int {
	lower := filepath.ToSlash(strings.ToLower(rel))
	score := 0
	switch {
	case strings.Contains(lower, "router"), strings.Contains(lower, "routes/"):
		score += 12
	case strings.Contains(lower, "handler"), strings.Contains(lower, "controller"):
		score += 11
	case strings.Contains(lower, "service"):
		score += 10
	case strings.Contains(lower, "model"), strings.Contains(lower, "schema"), strings.Contains(lower, "entity"):
		score += 9
	case strings.Contains(lower, "repository"), strings.Contains(lower, "store"):
		score += 8
	case strings.Contains(lower, "test"):
		score += 6
	case strings.Contains(lower, "component"), strings.Contains(lower, "page"):
		score += 7
	}
	if strings.HasSuffix(lower, "package.json") || strings.HasSuffix(lower, "go.mod") || strings.HasSuffix(lower, "cargo.toml") || strings.HasSuffix(lower, "pyproject.toml") || strings.HasSuffix(lower, "readme.md") {
		score += 14
	}
	return score
}

func runActorID(run *model.AgentRun) string {
	if run.TriggeredByUserID != nil && strings.TrimSpace(*run.TriggeredByUserID) != "" {
		return *run.TriggeredByUserID
	}
	return run.AgentID
}

func truncatePlanningText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... (truncated)"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// coalesceRaw returns the first non-empty string without trimming whitespace.
// Use this instead of firstNonEmptyString when the value may contain meaningful
// leading/trailing whitespace (e.g. LLM streaming token deltas like " found").
func coalesceRaw(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func validatePlanningProposalTasks(stories []model.ProposedTask) error {
	return model.NormalizeProposedTasks(stories)
}

func buildDraftSpecClarifications(draft model.ProductSpecDraft) []model.SpecClarificationItem {
	items := make([]model.SpecClarificationItem, 0, len(draft.OpenQuestions)+len(draft.Assumptions))
	for idx, question := range draft.OpenQuestions {
		question = strings.TrimSpace(question)
		if question == "" {
			continue
		}
		items = append(items, model.SpecClarificationItem{
			ID:          fmt.Sprintf("open_question_%d", idx+1),
			Kind:        model.SpecClarificationKindOpenQuestion,
			Prompt:      question,
			Disposition: model.SpecClarificationDispositionPending,
		})
	}
	for idx, assumption := range draft.Assumptions {
		assumption = strings.TrimSpace(assumption)
		if assumption == "" {
			continue
		}
		items = append(items, model.SpecClarificationItem{
			ID:          fmt.Sprintf("assumption_%d", idx+1),
			Kind:        model.SpecClarificationKindAssumption,
			Prompt:      assumption,
			Disposition: model.SpecClarificationDispositionPending,
		})
	}
	return items
}

func renderSpecClarificationsContext(items []model.SpecClarificationItem) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		switch item.Kind {
		case model.SpecClarificationKindOpenQuestion:
			lines = append(lines, fmt.Sprintf("- %s -> %s", item.Prompt, item.Response))
		case model.SpecClarificationKindAssumption:
			if item.Disposition == model.SpecClarificationDispositionAccepted {
				lines = append(lines, fmt.Sprintf("- %s -> accepted", item.Prompt))
			} else if item.Disposition == model.SpecClarificationDispositionRejected {
				lines = append(lines, fmt.Sprintf("- %s -> rejected: %s", item.Prompt, item.Response))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderProductSpecMarkdown(specMarkdown string, sources []model.PlanningResearchSource) string {
	specMarkdown = strings.TrimSpace(specMarkdown)
	sources = normalizePlanningResearchSources(sources)
	if len(sources) == 0 {
		return specMarkdown
	}

	lines := []string{specMarkdown, "## Research Sources"}
	for _, source := range sources {
		parts := []string{fmt.Sprintf("[%s](%s)", source.Title, source.URL)}
		note := strings.TrimSpace(source.Note)
		if note != "" {
			parts = append(parts, note)
		}
		publishedAt := strings.TrimSpace(source.PublishedAt)
		if publishedAt != "" {
			parts = append(parts, "published "+publishedAt)
		}
		lines = append(lines, "- "+strings.Join(parts, " - "))
	}

	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

func normalizePlanningResearchSources(sources []model.PlanningResearchSource) []model.PlanningResearchSource {
	normalized := make([]model.PlanningResearchSource, 0, len(sources))
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		title := strings.TrimSpace(source.Title)
		sourceURL := strings.TrimSpace(source.URL)
		if title == "" || sourceURL == "" {
			continue
		}
		key := strings.ToLower(sourceURL)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, model.PlanningResearchSource{
			Title:       title,
			URL:         sourceURL,
			Note:        strings.TrimSpace(source.Note),
			PublishedAt: strings.TrimSpace(source.PublishedAt),
		})
	}
	return normalized
}

func (a *AgentRunActivities) createRunArtifact(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any, sequenceNo int) error {
	return a.createRunArtifactWithMetadata(ctx, run, artifactType, format, payload, sequenceNo, nil)
}

func (a *AgentRunActivities) createRunArtifactWithMetadata(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any, sequenceNo int, metadata json.RawMessage) error {
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: strPtr(string(content)),
		Metadata:      metadata,
		SequenceNo:    sequenceNo,
	}
	return a.artifactRepo.Create(ctx, artifact)
}

func (a *AgentRunActivities) appendRunArtifact(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any) (*model.AgentRunArtifact, error) {
	return a.appendRunArtifactWithMetadata(ctx, run, artifactType, format, payload, nil)
}

func (a *AgentRunActivities) appendRunArtifactWithMetadata(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any, metadata json.RawMessage) (*model.AgentRunArtifact, error) {
	content, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	sequenceNo, err := a.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: strPtr(string(content)),
		Metadata:      metadata,
		SequenceNo:    sequenceNo,
	}
	if err := a.artifactRepo.Create(ctx, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

func lastAssistantSequenceNoUpTo(messages []model.AgentRunMessage, maxSequenceNo int) int {
	last := 0
	for _, message := range messages {
		if message.SequenceNo > maxSequenceNo {
			break
		}
		if strings.TrimSpace(message.Role) == "assistant" {
			last = message.SequenceNo
		}
	}
	return last
}

func (a *AgentRunActivities) mintAccessToken(ctx context.Context, integration *model.GitIntegration) (string, error) {
	if integration == nil {
		return "", nil
	}
	if integration.CredentialMode == "github_app" && integration.InstallationID != nil && *integration.InstallationID != "" {
		if a.githubApp == nil {
			return "", fmt.Errorf("github app credentials are not configured")
		}
		return a.githubApp.MintInstallationToken(ctx, *integration.InstallationID)
	}
	if integration.AccessToken != "" {
		return integration.AccessToken, nil
	}
	return "", fmt.Errorf("git integration has no usable credentials")
}

func (a *AgentRunActivities) serviceBridge() *workerpkg.ServiceBridge {
	return &workerpkg.ServiceBridge{
		ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("internal commands are not available")
			}
			return a.commandExecutor.Execute(ctx, meta, name, input)
		},
		AddComment: func(ctx context.Context, workspaceID, taskID, agentID, content string) error {
			comment := &model.PMComment{
				EntityType: "task",
				EntityID:   taskID,
				AuthorID:   agentID,
				Body:       content,
			}
			return a.commentRepo.Create(ctx, comment)
		},
		UpdateTaskState: func(ctx context.Context, workspaceID, taskID, stateID string) error {
			if a.commandExecutor != nil {
				_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					TargetType:  "task",
					TargetID:    taskID,
				}, "pm.update_task_state", mustJSON(map[string]any{
					"task_id":  taskID,
					"state_id": stateID,
				}))
				return err
			}
			// TODO(flow-platform): remove direct fallback once all native runs are command-backed.
			task, err := a.taskRepo.GetRawByID(ctx, taskID)
			if err != nil {
				return err
			}
			if task == nil {
				return fmt.Errorf("task not found")
			}
			task.WorkflowStateID = stateID
			return a.taskRepo.Update(ctx, task)
		},
		ListChecklist: func(ctx context.Context, workspaceID, taskID string) ([]model.PMChecklistItem, error) {
			return a.checklistRepo.List(ctx, taskID)
		},
		CreateTaskBatch: func(ctx context.Context, workspaceID, epicID, actorID string, tasks []model.ProposedTask) (workerpkg.CreateTaskBatchResult, error) {
			if a.commandExecutor != nil {
				output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					ActorID:     actorID,
					TargetType:  "epic",
					TargetID:    epicID,
				}, "pm.create_task_batch", mustJSON(map[string]any{
					"tasks": tasks,
				}))
				if err != nil {
					return workerpkg.CreateTaskBatchResult{}, err
				}
				var result workerpkg.CreateTaskBatchResult
				if err := json.Unmarshal(output, &result); err != nil {
					return workerpkg.CreateTaskBatchResult{}, err
				}
				return result, nil
			}
			return workerpkg.CreateTaskBatchResult{}, fmt.Errorf("planner commands are not available")
		},
		AssignTaskAgent: func(ctx context.Context, workspaceID, actorID, taskID, agentID string) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("planner commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "task",
				TargetID:    taskID,
			}, "pm.assign_task_agent", mustJSON(map[string]any{
				"task_id":  taskID,
				"agent_id": agentID,
			}))
			return err
		},
		SetTaskDependencies: func(ctx context.Context, workspaceID, actorID string, dependencies []workerpkg.TaskDependencyLink) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("planner commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "epic",
			}, "pm.set_task_dependencies", mustJSON(map[string]any{
				"dependencies": dependencies,
			}))
			return err
		},
		ListEpicTasks: func(ctx context.Context, workspaceID, epicID string) ([]workerpkg.EpicTaskSummary, error) {
			tasks, err := a.epicRepo.ListTasks(ctx, epicID)
			if err != nil {
				return nil, err
			}
			summaries := make([]workerpkg.EpicTaskSummary, 0, len(tasks))
			for _, s := range tasks {
				if s.WorkspaceID != workspaceID {
					continue
				}
				status := "not_started"
				if s.Completed {
					status = "done"
				} else if s.Started {
					status = "in_progress"
				}
				summaries = append(summaries, workerpkg.EpicTaskSummary{
					ID:              s.ID,
					Name:            s.Name,
					TaskType:        s.TaskType,
					Status:          status,
					Estimate:        s.Estimate,
					Priority:        s.Priority,
					AssignedAgentID: s.AssignedAgentID,
				})
			}
			return summaries, nil
		},
		ListWorkspaceTeams: func(ctx context.Context, workspaceID string) ([]workerpkg.WorkspaceTeamSummary, error) {
			if a.settingsRepo == nil {
				return nil, fmt.Errorf("workspace settings are not available")
			}
			teams, err := a.settingsRepo.ListTeams(ctx, workspaceID)
			if err != nil {
				return nil, err
			}
			result := make([]workerpkg.WorkspaceTeamSummary, 0, len(teams))
			for _, team := range teams {
				result = append(result, workerpkg.WorkspaceTeamSummary{
					ID:              team.ID,
					Name:            team.Name,
					Handle:          team.Handle,
					TeamType:        team.TeamType,
					DefaultTaskType: team.DefaultStoryType,
				})
			}
			return result, nil
		},
		ApproveEpicSpec: func(ctx context.Context, workspaceID, epicID, actorID string, versionID *string) (*model.ApprovedSpecSummary, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("planner commands are not available")
			}
			input := mustJSON(map[string]any{})
			if versionID != nil && strings.TrimSpace(*versionID) != "" {
				input = mustJSON(map[string]any{"version_id": strings.TrimSpace(*versionID)})
			}
			output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "epic",
				TargetID:    epicID,
			}, "pm.approve_epic_spec", input)
			if err != nil {
				return nil, err
			}
			var summary model.ApprovedSpecSummary
			if err := json.Unmarshal(output, &summary); err != nil {
				return nil, err
			}
			return &summary, nil
		},
		ListConversationMessages: func(ctx context.Context, workspaceID, conversationID string) ([]model.SupportMessage, error) {
			return a.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
		},
		UpdateConversationStatus: func(ctx context.Context, workspaceID, conversationID, status string) error {
			conversation, err := a.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
			if err != nil {
				return err
			}
			if conversation == nil {
				return fmt.Errorf("conversation not found")
			}
			conversation.Status = status
			if err := a.conversationRepo.Update(ctx, conversation); err != nil {
				return err
			}
			if a.wsPublisher != nil {
				a.wsPublisher.Publish(websocket.Event{
					Action:      "updated",
					Entity:      "support_conversation",
					EntityID:    conversationID,
					WorkspaceID: workspaceID,
				})
			}
			return nil
		},

		// CRM
		ListDeals: func(ctx context.Context, workspaceID string, limit int) ([]model.CRMDeal, error) {
			deals, _, err := a.crmDealRepo.List(ctx, workspaceID, model.CRMDealListFilters{}, model.PMPagination{Page: 1, PerPage: limit})
			return deals, err
		},
		GetDeal: func(ctx context.Context, id string) (*model.CRMDeal, error) {
			return a.crmDealRepo.GetByID(ctx, id)
		},
		UpdateDealStage: func(ctx context.Context, dealID, stageID string) error {
			if a.commandExecutor != nil {
				_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					TargetType: "crm_deal",
					TargetID:   dealID,
				}, "crm.update_deal_stage", mustJSON(map[string]any{
					"deal_id":  dealID,
					"stage_id": stageID,
				}))
				return err
			}
			// TODO(flow-platform): remove direct fallback once all native runs are command-backed.
			deal, err := a.crmDealRepo.GetByID(ctx, dealID)
			if err != nil {
				return err
			}
			if deal == nil {
				return fmt.Errorf("deal not found")
			}
			deal.StageID = stageID
			return a.crmDealRepo.Update(ctx, deal)
		},
		AddDealNote: func(ctx context.Context, workspaceID, dealID, agentID, content string) error {
			if a.commandExecutor != nil {
				_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					AgentID:     agentID,
					TargetType:  "crm_deal",
					TargetID:    dealID,
				}, "crm.add_deal_note", mustJSON(map[string]any{
					"deal_id": dealID,
					"content": content,
				}))
				return err
			}
			// TODO(flow-platform): remove direct fallback once all native runs are command-backed.
			now := time.Now()
			activity := &model.CRMActivity{
				WorkspaceID:  workspaceID,
				ActivityType: "note",
				DealID:       &dealID,
				Body:         &content,
				OccurredAt:   now,
			}
			return a.crmActivityRepo.Create(ctx, activity)
		},
		ListContacts: func(ctx context.Context, workspaceID string, limit int) ([]model.CRMContact, error) {
			contacts, _, err := a.crmContactRepo.List(ctx, workspaceID, model.CRMContactListFilters{}, model.PMPagination{Page: 1, PerPage: limit})
			return contacts, err
		},
		ListBuyerSignals: func(ctx context.Context, workspaceID string, dealID *string, limit int) ([]model.CRMBuyerSignal, error) {
			filters := model.CRMBuyerSignalListFilters{DealID: dealID}
			signals, _, err := a.crmSignalRepo.ListSignals(ctx, workspaceID, filters, model.PMPagination{Page: 1, PerPage: limit})
			return signals, err
		},

		// Docs
		GetDocument: func(ctx context.Context, id string) (*model.DocsDocument, error) {
			return a.docsDocRepo.GetByID(ctx, id)
		},
		ListDocuments: func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsDocument, error) {
			published := "published"
			return a.docsDocRepo.List(ctx, workspaceID, spaceID, nil, &published, nil, "", false)
		},
		SearchDocuments: func(ctx context.Context, workspaceID, query string, limit int) ([]workerpkg.DocsSearchHit, error) {
			results, err := a.docsSearchRepo.Search(ctx, workspaceID, query, nil, nil, limit)
			if err != nil {
				return nil, err
			}
			hits := make([]workerpkg.DocsSearchHit, len(results))
			for i, r := range results {
				hits[i] = workerpkg.DocsSearchHit{
					ID:    r.ID,
					Title: r.Title,
				}
			}
			return hits, nil
		},
		EnsureEpicSpecDoc: func(ctx context.Context, workspaceID, epicID, actorID string) (*model.DocsDocument, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("document commands are not available")
			}
			output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "epic",
				TargetID:    epicID,
			}, "docs.ensure_spec_doc", json.RawMessage(`{}`))
			if err != nil {
				return nil, err
			}
			var response struct {
				DocumentID string `json:"document_id"`
			}
			if err := json.Unmarshal(output, &response); err != nil {
				return nil, err
			}
			if strings.TrimSpace(response.DocumentID) == "" {
				return nil, fmt.Errorf("ensure spec doc returned no document_id")
			}
			return a.docsDocRepo.GetByID(ctx, response.DocumentID)
		},
		EnsureTaskPlanDoc: func(ctx context.Context, workspaceID, taskID, actorID string) (*model.DocsDocument, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("document commands are not available")
			}
			output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "task",
				TargetID:    taskID,
			}, "docs.ensure_task_plan_doc", json.RawMessage(`{}`))
			if err != nil {
				return nil, err
			}
			var response struct {
				DocumentID string `json:"document_id"`
			}
			if err := json.Unmarshal(output, &response); err != nil {
				return nil, err
			}
			if strings.TrimSpace(response.DocumentID) == "" {
				return nil, fmt.Errorf("ensure task plan doc returned no document_id")
			}
			return a.docsDocRepo.GetByID(ctx, response.DocumentID)
		},
		GetDocumentContent: func(ctx context.Context, documentID string) (string, error) {
			content, err := a.docsContentRepo.GetByDocumentID(ctx, documentID)
			if err != nil {
				return "", err
			}
			if content == nil {
				return "", nil
			}
			return content.ContentText, nil
		},
		WriteDocumentContent: func(ctx context.Context, workspaceID, documentID string, content json.RawMessage) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("document commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				TargetType:  "document",
				TargetID:    documentID,
			}, "docs.write_document_content", mustJSON(map[string]any{
				"document_id": documentID,
				"content":     json.RawMessage(content),
			}))
			return err
		},
		LinkDocumentToObject: func(ctx context.Context, workspaceID, documentID, linkedObjectType, linkedObjectID, linkContext, actorID string) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("document commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  linkedObjectType,
				TargetID:    linkedObjectID,
			}, "docs.link_document_to_object", mustJSON(map[string]any{
				"document_id":        documentID,
				"linked_object_type": linkedObjectType,
				"linked_object_id":   linkedObjectID,
				"link_context":       linkContext,
			}))
			return err
		},
	}
}

func (a *AgentRunActivities) pushVisitorConversationRefresh(ctx context.Context, workspaceID string, conversation *model.SupportConversation) {
	if conversation == nil || conversation.AnonymousID == nil || strings.TrimSpace(*conversation.AnonymousID) == "" || a.wsPublisher == nil {
		return
	}
	conversations, err := a.conversationRepo.ListByAnonymousID(ctx, workspaceID, *conversation.AnonymousID)
	if err != nil {
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	listJSON, err := json.Marshal(map[string]any{"conversations": conversations})
	if err != nil {
		return
	}
	a.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_visitor_conversations",
		EntityID:    *conversation.AnonymousID,
		WorkspaceID: workspaceID,
		Data:        listJSON,
	})
}

func (a *AgentRunActivities) failRun(ctx context.Context, state *resolvedRunState, errMsg string) error {
	now := time.Now()
	state.run.Status = "failed"
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.CompletedAt = &now
	state.run.ErrorMessage = &errMsg
	state.run.ExecutionStage = strPtr("failed")
	state.run.LastHeartbeatAt = &now
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)
	if state.deliveryTarget != nil {
		state.deliveryTarget.DeliveryState = "failed"
		state.deliveryTarget.LastRunID = &state.run.ID
		state.deliveryTarget.LastSyncedAt = &now
		_ = a.deliveryRepo.Save(ctx, state.deliveryTarget)
	}
	if agent, err := a.agentRepo.GetByID(ctx, state.run.WorkspaceID, state.run.AgentID); err == nil && agent != nil {
		agent.Status = "error"
		agent.ActiveTaskID = nil
		_ = a.agentRepo.Update(ctx, agent)
	}
	return nil
}

func (a *AgentRunActivities) isRunExplicitlyCancelled(ctx context.Context, runID string) (bool, error) {
	if a == nil || a.runRepo == nil || strings.TrimSpace(runID) == "" {
		return false, nil
	}
	currentRun, err := a.runRepo.GetByIDAny(ctx, runID)
	if err != nil {
		return false, err
	}
	if currentRun == nil {
		return false, nil
	}
	return currentRun.Status == model.AgentRunStatusCancelled, nil
}

func (a *AgentRunActivities) MarkRunFailedActivity(ctx context.Context, runID, errMsg string) error {
	if a == nil || a.runRepo == nil {
		return nil
	}

	run, err := a.runRepo.GetByIDAny(ctx, runID)
	if err != nil || run == nil {
		return err
	}
	switch run.Status {
	case model.AgentRunStatusCompleted, model.AgentRunStatusFailed, model.AgentRunStatusCancelled:
		return nil
	}

	now := time.Now()
	run.Status = model.AgentRunStatusFailed
	run.PauseReason = model.AgentRunPauseReasonNone
	run.CompletedAt = &now
	run.ErrorMessage = strPtr(strings.TrimSpace(errMsg))
	run.ExecutionStage = strPtr("failed")
	run.LastHeartbeatAt = &now
	if err := a.runRepo.Update(ctx, run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, run)

	if a.agentRepo != nil {
		if agent, err := a.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID); err == nil && agent != nil {
			agent.Status = "error"
			agent.ActiveTaskID = nil
			_ = a.agentRepo.Update(ctx, agent)
		}
	}
	return nil
}

func ensureRunNotTerminal(run *model.AgentRun) error {
	if run == nil {
		return nil
	}
	switch run.Status {
	case "failed", "completed", "cancelled", "paused":
		return temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("agent run %s is already in terminal state %q", run.ID, run.Status),
			"AgentRunTerminalState",
			nil,
		)
	default:
		return nil
	}
}

func nonRetryableRunError(err error) error {
	if err == nil {
		return nil
	}
	return temporal.NewNonRetryableApplicationError(err.Error(), "AgentRunFailed", err)
}

func (a *AgentRunActivities) markAgentIdle(ctx context.Context, workspaceID, agentID string, tokens int) error {
	agent, err := a.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	agent.Status = "idle"
	agent.ActiveTaskID = nil
	agent.TokensUsedThisMonth += tokens
	return a.agentRepo.Update(ctx, agent)
}

func (a *AgentRunActivities) runGitInDir(ctx context.Context, dir string, integration *model.GitIntegration, accessToken string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmdArgs := append(gitAuthArgs(integration, accessToken), args...)
	cmd := exec.CommandContext(cmdCtx, "git", cmdArgs...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, string(output))
	}
	return string(output), nil
}

func gitAuthArgs(integration *model.GitIntegration, accessToken string) []string {
	if integration == nil || accessToken == "" {
		return nil
	}
	switch integration.Provider {
	case "github":
		auth := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + accessToken))
		return []string{"-c", "http.extraheader=Authorization: Basic " + auth}
	case "gitlab":
		auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + accessToken))
		return []string{"-c", "http.extraheader=Authorization: Basic " + auth}
	default:
		return nil
	}
}

func buildWorkingBranch(task *model.PMTask, teamDefault *model.PMTeamRepoDefault, workspaceKey string) string {
	return model.BuildTaskWorkingBranch(task, teamDefault, workspaceKey)
}

func isEmptySummary(summary json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(summary))
	return trimmed == "" || trimmed == "{}" || trimmed == "null"
}

func runInputAdditionalContext(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var payload struct {
		AdditionalContext string `json:"additional_context"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.AdditionalContext)
}

func buildDurableRunFacts(state *resolvedRunState, input planningRunInput) map[string]string {
	facts := map[string]string{}
	if state == nil || state.run == nil {
		return facts
	}

	collectStructIDFacts(facts, "run", state.run)
	collectStructIDFacts(facts, "agent", state.agent)
	collectStructIDFacts(facts, "task", state.task)
	collectStructIDFacts(facts, "story", state.task) // backward-compat alias for "task"
	collectStructIDFacts(facts, "epic", state.epic)
	collectStructIDFacts(facts, "conversation", state.conversation)
	collectStructIDFacts(facts, "delivery_target", state.deliveryTarget)
	collectStructIDFacts(facts, "repository", state.repository)
	collectStructIDFacts(facts, "integration", state.integration)
	collectStructIDFacts(facts, "team_default", state.teamDefault)
	collectStructIDFacts(facts, "planning_input", input)
	collectJSONIDFacts(facts, "", state.run.Input)

	setFact(facts, "workspace_id", state.run.WorkspaceID)
	setFact(facts, "run_id", state.run.ID)
	setFact(facts, "agent_id", state.run.AgentID)
	setFact(facts, "target_type", state.run.TargetType)
	setFact(facts, "target_id", state.run.TargetID)
	setFact(facts, "task_id", firstNonEmptyString(derefString(state.run.TaskID), structID(state.task)))
	setFact(facts, "story_id", firstNonEmptyString(derefString(state.run.TaskID), structID(state.task)))
	setFact(facts, "conversation_id", firstNonEmptyString(derefString(state.run.ConversationID), structID(state.conversation)))
	setFact(facts, "epic_id", firstNonEmptyString(structID(state.epic), derefString(epicIDOfTask(state.task))))
	setFact(facts, "plan_document_id", firstNonEmptyString(input.PlanDocumentID, derefString(planDocumentIDOfTask(state.task))))
	setFact(facts, "spec_document_id", firstNonEmptyString(input.SpecDocumentID, derefString(specDocumentIDOfEpic(state.epic))))
	setFact(facts, "spec_version_id", input.SpecVersionID)
	setFact(facts, "approved_spec_version_id", derefString(approvedSpecVersionIDOfEpic(state.epic)))
	setFact(facts, "repository_id", firstNonEmptyString(derefString(state.run.RepositoryID), repositoryIDOfDeliveryTarget(state.deliveryTarget), structID(state.repository)))
	setFact(facts, "delivery_target_id", firstNonEmptyString(derefString(state.run.DeliveryTargetID), structID(state.deliveryTarget)))
	setFact(facts, "repo_full_name", repoFullName(state))
	setFact(facts, "base_branch", derefString(state.run.BaseBranch))
	setFact(facts, "working_branch", derefString(state.run.WorkingBranch))
	setFact(facts, "branch_sync_status", strings.TrimSpace(state.branchSync.Status))
	setFact(facts, "branch_sync_base_branch", strings.TrimSpace(state.branchSync.BaseBranch))
	setFact(facts, "branch_sync_working_branch", strings.TrimSpace(state.branchSync.WorkingBranch))
	if len(state.branchSync.ConflictFiles) > 0 {
		setFact(facts, "branch_sync_conflict_files", strings.Join(state.branchSync.ConflictFiles, ","))
	}

	targetType := sanitizeFactKey(state.run.TargetType)
	if targetType != "" && strings.TrimSpace(state.run.TargetID) != "" {
		setFact(facts, targetType+"_id", state.run.TargetID)
	}

	return facts
}

func collectStructIDFacts(facts map[string]string, prefix string, value any) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}

	rt := rv.Type()
	for idx := 0; idx < rv.NumField(); idx++ {
		field := rt.Field(idx)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}

		key := ""
		switch {
		case name == "id":
			key = prefix + "_id"
		case strings.HasSuffix(name, "_id"), strings.HasSuffix(name, "_ids"):
			key = prefix + "_" + name
		default:
			continue
		}

		if list, ok := reflectedStringSlice(rv.Field(idx)); ok && len(list) > 0 {
			setFact(facts, key, strings.Join(list, ", "))
			continue
		}
		if value, ok := reflectedString(rv.Field(idx)); ok {
			setFact(facts, key, value)
		}
	}
}

func collectJSONIDFacts(facts map[string]string, prefix string, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	collectJSONIDFactsValue(facts, prefix, payload)
}

func collectJSONIDFactsValue(facts map[string]string, prefix string, value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			key = sanitizeFactKey(key)
			if key == "" {
				continue
			}
			path := key
			if prefix != "" {
				path = prefix + "_" + key
			}
			if strings.HasSuffix(key, "_id") {
				if stringValue, ok := child.(string); ok {
					setFact(facts, path, stringValue)
				}
			} else if strings.HasSuffix(key, "_ids") {
				if list := jsonStringSlice(child); len(list) > 0 {
					setFact(facts, path, strings.Join(list, ", "))
				}
			}
			collectJSONIDFactsValue(facts, path, child)
		}
	case []any:
		for _, child := range typed {
			collectJSONIDFactsValue(facts, prefix, child)
		}
	}
}

func jsonFieldName(field reflect.StructField) string {
	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "" {
		return sanitizeFactKey(field.Name)
	}
	name := strings.TrimSpace(strings.Split(tag, ",")[0])
	if name == "" {
		return sanitizeFactKey(field.Name)
	}
	return name
}

func reflectedString(value reflect.Value) (string, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.String {
		return "", false
	}
	text := strings.TrimSpace(value.String())
	if text == "" {
		return "", false
	}
	return text, true
}

func reflectedStringSlice(value reflect.Value) ([]string, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Slice {
		return nil, false
	}
	items := make([]string, 0, value.Len())
	for idx := 0; idx < value.Len(); idx++ {
		item, ok := reflectedString(value.Index(idx))
		if ok {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil, false
	}
	return items, true
}

func jsonStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok && strings.TrimSpace(text) != "" {
			result = append(result, strings.TrimSpace(text))
		}
	}
	return result
}

func sanitizeFactKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func setFact(facts map[string]string, key, value string) {
	key = sanitizeFactKey(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	facts[key] = value
}

func structID(value any) string {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return ""
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return ""
	}
	field := rv.FieldByName("ID")
	if !field.IsValid() {
		return ""
	}
	id, ok := reflectedString(field)
	if !ok {
		return ""
	}
	return id
}

func epicIDOfTask(task *model.PMTask) *string {
	if task == nil {
		return nil
	}
	return task.EpicID
}

func planDocumentIDOfTask(task *model.PMTask) *string {
	if task == nil {
		return nil
	}
	return task.PlanDocumentID
}

func specDocumentIDOfEpic(epic *model.PMEpic) *string {
	if epic == nil {
		return nil
	}
	return epic.SpecDocumentID
}

func approvedSpecVersionIDOfEpic(epic *model.PMEpic) *string {
	if epic == nil {
		return nil
	}
	return epic.ApprovedSpecVersionID
}

func repositoryIDOfDeliveryTarget(target *model.TaskDeliveryTarget) string {
	if target == nil {
		return ""
	}
	return derefString(target.RepositoryID)
}

func repoFullName(state *resolvedRunState) string {
	if state.run != nil && state.run.RepoFullName != nil && *state.run.RepoFullName != "" {
		return *state.run.RepoFullName
	}
	if state.deliveryTarget != nil && state.deliveryTarget.RepoFullName != nil {
		return *state.deliveryTarget.RepoFullName
	}
	if state.repository != nil {
		return state.repository.FullName
	}
	return ""
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func strPtr(value string) *string {
	return &value
}

func resolvedAllowedToolSet(resolved workerpkg.ResolvedProfile) map[string]bool {
	set := make(map[string]bool, len(resolved.Tools))
	for _, toolName := range resolved.Tools {
		set[toolName] = true
	}
	return set
}

func effectiveToolSet(resolved workerpkg.ResolvedProfile, explicitAllowed []string) map[string]bool {
	if len(explicitAllowed) > 0 {
		return stringSliceToSet(explicitAllowed)
	}
	return resolvedAllowedToolSet(resolved)
}

func stringSliceToSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = true
		}
	}
	return set
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
