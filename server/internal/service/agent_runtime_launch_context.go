package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// delegatedTaskPlanDocumentContextLimit mirrors the Temporal path's plan-doc
// truncation (temporalapp buildTaskRunPlanDocumentContextSections).
const delegatedTaskPlanDocumentContextLimit = 12000

const (
	delegatedEpicSpecContextLimit  = 16000
	delegatedEpicTaskContextLimit  = 50
	agentRepositoryAccessReadOnly  = "read_only"
	agentRepositoryAccessReadWrite = "read_write"
)

// agentRequiresRepositoryWorkspace reports whether delegated runs for this
// agent must execute inside a prepared repository clone. It is keyed strictly
// off the agent's preset runtime profile (code_builder / review_agent set
// RequiresRepo); custom agents never match because
// agentcontract.GetRuntimeProfile falls back to code_builder for unknown names, so
// the resolved profile name must round-trip the preset key exactly.
func agentRequiresRepositoryWorkspace(agent *model.Agent) bool {
	if agent == nil {
		return false
	}
	preset := agent.EffectivePresetKey()
	if preset == "" {
		return false
	}
	profile := agentcontract.GetRuntimeProfile(preset)
	return profile.Name == preset && profile.RequiresRepo
}

// withRepositoryWorkspaceExecutionConfig merges
// {"workspace":{"mode":"repository"}} into an agent execution config. Agent
// Runtime prepares a repository workspace (clone + branch checkout via the
// host repository-spec endpoint) only when the runtime agent record's
// execution_config carries workspace.mode; Helpin's AgentExecutionConfig does
// not model that field, so it is injected at delegation time. An explicit
// pre-existing workspace.mode is preserved.
func withRepositoryWorkspaceExecutionConfig(config json.RawMessage) json.RawMessage {
	values := map[string]interface{}{}
	if len(config) > 0 && strings.TrimSpace(string(config)) != "null" {
		if err := json.Unmarshal(config, &values); err != nil {
			values = map[string]interface{}{}
		}
	}
	workspaceValue, _ := values["workspace"].(map[string]interface{})
	if workspaceValue == nil {
		workspaceValue = map[string]interface{}{}
	}
	if mode, _ := workspaceValue["mode"].(string); strings.TrimSpace(mode) == "" {
		workspaceValue["mode"] = "repository"
	}
	values["workspace"] = workspaceValue
	payload, err := json.Marshal(values)
	if err != nil {
		return config
	}
	return payload
}

// withoutRepositoryWorkspaceExecutionMode removes the startup workspace
// preparation selector while preserving unrelated execution settings. The
// Dock uses checkout_repository dynamically and keeps its primary run target
// as the product workspace, so asking Agent Runtime to prepare that target as
// a repository workspace is invalid.
func withoutRepositoryWorkspaceExecutionMode(config []byte) []byte {
	values := map[string]interface{}{}
	if len(config) > 0 && strings.TrimSpace(string(config)) != "null" {
		if err := json.Unmarshal(config, &values); err != nil {
			return config
		}
	}
	workspaceValue, _ := values["workspace"].(map[string]interface{})
	if workspaceValue == nil {
		return normalizeExecutionConfigJSON(config)
	}
	delete(workspaceValue, "mode")
	if len(workspaceValue) == 0 {
		delete(values, "workspace")
	} else {
		values["workspace"] = workspaceValue
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return config
	}
	return payload
}

// withAgentRuntimeExecutionConfig adds host-owned execution selectors that
// Agent Runtime needs but Helpin does not persist in AgentExecutionConfig.
// Existing user/model routing fields are preserved.
func withAgentRuntimeExecutionConfig(config json.RawMessage, agent *model.Agent) json.RawMessage {
	if agentRequiresRepositoryWorkspace(agent) {
		config = withRepositoryWorkspaceExecutionConfig(config)
	} else if agent != nil && strings.TrimSpace(agent.EffectivePresetKey()) == model.AgentPresetAskAgent {
		// Managed Ask runs are workspace-targeted orchestrators. Repository
		// inspection is attached later by checkout_repository and must not turn
		// the product workspace target into a repository-spec request.
		config = withoutRepositoryWorkspaceExecutionMode(config)
	}
	values := map[string]interface{}{}
	if len(config) > 0 && strings.TrimSpace(string(config)) != "null" {
		if err := json.Unmarshal(config, &values); err != nil {
			values = map[string]interface{}{}
		}
	}
	if agent != nil {
		workspaceValue, _ := values["workspace"].(map[string]interface{})
		if workspaceValue == nil {
			workspaceValue = map[string]interface{}{}
		}
		workspaceValue["access"] = agentRepositoryAccessMode(agent)
		values["workspace"] = workspaceValue
		if strings.TrimSpace(agent.EffectivePresetKey()) == model.AgentPresetTaskPlanner {
			values["completion"] = map[string]interface{}{
				"required_tools": []string{agentcontract.ToolPublishTaskPlanDoc},
			}
		}
		if presetKey := strings.TrimSpace(agent.EffectivePresetKey()); presetKey != "" {
			values["preset_key"] = presetKey
		}
		if policy, ok := runtimePolicyForAgent(agent); ok {
			values["runtime_policy"] = policy
		}
		if strings.TrimSpace(agent.RuntimeKind) == "native_sdk" {
			values["max_tool_steps"] = agentcontract.DefaultWorkflowConfigForAgent(agent).MaxIterations
		}
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return config
	}
	return payload
}

func runtimePolicyForAgent(agent *model.Agent) (agentcontract.SkillPolicy, bool) {
	if agent == nil {
		return agentcontract.SkillPolicy{}, false
	}
	preset, ok := agentPresetVersionDefinition(agent.EffectivePresetKey(), agent.EffectivePresetVersionKey())
	if !ok {
		return agentcontract.SkillPolicy{}, false
	}
	policies := make([]agentcontract.SkillPolicy, 0, len(preset.InstructionSkills))
	for _, key := range preset.InstructionSkills {
		definition, exists := agentcontract.GetBuiltInSkill(key)
		if !exists {
			continue
		}
		policies = append(policies, definition.Policy)
	}
	policy := agentcontract.AggregateSkillPolicies(policies...)
	return policy, policy.AllowImplicitInvocation != nil || len(policy.CompletionRequiresInteractionKinds) > 0 || len(policy.InteractionContracts) > 0
}

func agentRepositoryAccessMode(agent *model.Agent) string {
	if agent == nil {
		return agentRepositoryAccessReadOnly
	}
	switch strings.TrimSpace(agent.EffectivePresetKey()) {
	case model.AgentPresetCodeBuilder, model.AgentPresetReviewAgent:
		return agentRepositoryAccessReadWrite
	}
	for _, toolName := range agentcontract.NormalizeToolNames(parseJSONStringSlice(agent.AllowedTools)) {
		switch toolName {
		case "write_file", "edit_file", "apply_patch", "create_branch", "commit_and_push", "open_pr":
			return agentRepositoryAccessReadWrite
		}
	}
	return agentRepositoryAccessReadOnly
}

func withAgentRunPlanningStage(payload []byte, stage string) ([]byte, error) {
	stage = strings.TrimSpace(stage)
	if stage == "" {
		return payload, nil
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(payload, &input); err != nil {
		return nil, err
	}
	input.Stage = stage
	return json.Marshal(input)
}

func planningStageForDelegatedRun(agent *model.Agent, task *model.PMTask, epic *model.PMEpic) string {
	if agent == nil {
		return ""
	}
	switch strings.TrimSpace(agent.EffectivePresetKey()) {
	case model.AgentPresetTaskPlanner:
		if task != nil {
			return model.PlanningStageTaskPlanDoc
		}
	case model.AgentPresetEpicPlanner:
		if epic == nil {
			return ""
		}
		if strings.TrimSpace(derefString(epic.ApprovedSpecVersionID)) != "" {
			return model.PlanningStagePlanTasks
		}
		switch strings.TrimSpace(epic.PlanningState) {
		case model.EpicPlanningStateReadyForTaskPlanning,
			model.EpicPlanningStateAwaitingPlanApproval,
			model.EpicPlanningStateTasksCreated,
			model.EpicPlanningStateExecutionStarted,
			model.EpicPlanningStateReadyForExecution:
			return model.PlanningStagePlanTasks
		default:
			return model.PlanningStageDraftSpec
		}
	}
	return ""
}

// buildDelegatedTaskLaunchContext assembles, at launch time, the task context
// a local Temporal run builds at execution time (agentcontract.BuildUserPrompt task
// sections + temporalapp buildTaskExecutionInstructions): operator notes,
// task name and description, parent epic background, repository branch
// values, and the canonical plan document content. Delegated runs need this
// stamped into additional_context because Agent Runtime adapters inject only
// the shallow target-context summary ("Task N: name") into prompts, and the
// coding presets' tool grants include no PM read tools to self-gather the
// rest. Assembly is best-effort: enrichment lookups that fail are logged and
// skipped so a launch never fails on optional context.
//
// Known parity gaps versus the Temporal path (documented in
// docs/AGENT_RUNTIME_LOCAL.md): checklist items and task-linked docs other
// than the plan document are not included.
func (s *AgentService) buildDelegatedTaskLaunchContext(ctx context.Context, task *model.PMTask, delivery *model.TaskDeliveryTarget, req model.StartAgentRunRequest) string {
	operatorNotes := strings.TrimSpace(derefString(req.AdditionalContext))
	if task == nil {
		return operatorNotes
	}

	var sections []string
	if operatorNotes != "" {
		sections = append(sections, "Operator notes:\n"+operatorNotes)
	}
	if name := strings.TrimSpace(task.Name); name != "" {
		sections = append(sections, fmt.Sprintf("Task: **%s**", name))
	}
	if task.Description != nil {
		if description := strings.TrimSpace(tiptap.RichTextToMarkdown(*task.Description)); description != "" {
			sections = append(sections, "Task description:\n"+description)
		}
	}
	sections = append(sections, s.delegatedTaskEpicSections(ctx, task)...)
	if repository := delegatedTaskRepositorySection(delivery); repository != "" {
		sections = append(sections, repository)
	}
	if branches := delegatedTaskBranchSection(delivery, req.BaseBranch, req.WorkingBranch); branches != "" {
		sections = append(sections, branches)
	}
	sections = append(sections, s.delegatedTaskPlanDocumentSections(ctx, task)...)

	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func delegatedTaskRepositorySection(delivery *model.TaskDeliveryTarget) string {
	if delivery == nil {
		return ""
	}
	repository := strings.TrimSpace(derefString(delivery.RepoFullName))
	if repository == "" {
		repository = strings.TrimSpace(derefString(delivery.RepositoryID))
	}
	if repository == "" {
		return ""
	}
	return fmt.Sprintf("Prepared repository: `%s`. Agent Runtime checks out this repository before execution. Inspect the current workspace directly; do not ask which repository to use or call repository checkout tools unless filesystem tools report that no checkout exists.", repository)
}

// buildDelegatedEpicLaunchContext carries the durable planning facts that the
// retired Temporal executor assembled before invoking an epic planner. The
// runtime can still use Helpin tools for deeper discovery, but it should not
// spend its first tool rounds rediscovering the epic, approved/draft spec, and
// existing-task guardrails.
func (s *AgentService) buildDelegatedEpicLaunchContext(ctx context.Context, epic *model.PMEpic, extra *string) string {
	operatorNotes := strings.TrimSpace(derefString(extra))
	if epic == nil {
		return operatorNotes
	}
	sections := make([]string, 0, 8)
	if operatorNotes != "" {
		sections = append(sections, "Operator notes:\n"+operatorNotes)
	}
	sections = append(sections, fmt.Sprintf("Epic: **%s** [%s]", strings.TrimSpace(epic.Name), epic.ID))
	if epic.Description != nil {
		if description := strings.TrimSpace(tiptap.RichTextToMarkdown(*epic.Description)); description != "" {
			sections = append(sections, "Epic description:\n"+truncateDelegatedLaunchText(description, 8000))
		}
	}
	sections = append(sections, fmt.Sprintf(
		"Durable planning facts:\n- planning_state=%s\n- team_id=%s\n- spec_document_id=%s\n- approved_spec_version_id=%s",
		strings.TrimSpace(epic.PlanningState),
		strings.TrimSpace(derefString(epic.TeamID)),
		strings.TrimSpace(derefString(epic.SpecDocumentID)),
		strings.TrimSpace(derefString(epic.ApprovedSpecVersionID)),
	))
	if spec := s.delegatedEpicSpecSection(ctx, epic); spec != "" {
		sections = append(sections, spec)
	}
	if tasks := s.delegatedEpicTasksSection(ctx, epic); tasks != "" {
		sections = append(sections, tasks)
	}
	if docs := s.delegatedLinkedDocsSection(ctx, epic.WorkspaceID, model.LinkedObjectEpic, epic.ID, derefString(epic.SpecDocumentID)); docs != "" {
		sections = append(sections, "Other docs linked to this epic:\n"+docs)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func (s *AgentService) delegatedEpicSpecSection(ctx context.Context, epic *model.PMEpic) string {
	if epic == nil {
		return ""
	}
	if versionID := strings.TrimSpace(derefString(epic.ApprovedSpecVersionID)); versionID != "" && s.docsVersionRepo != nil {
		version, err := s.docsVersionRepo.GetByID(ctx, versionID)
		if err != nil {
			slog.WarnContext(ctx, "delegated epic launch context: approved spec lookup failed", "error", err, "epic_id", epic.ID, "version_id", versionID)
		} else if version != nil {
			markdown := strings.TrimSpace(tiptap.RichTextToMarkdown(string(version.Content)))
			if markdown == "" {
				markdown = strings.TrimSpace(version.ContentText)
			}
			if markdown != "" {
				return "Approved epic PRD snapshot [" + versionID + "]:\n" + truncateDelegatedLaunchText(markdown, delegatedEpicSpecContextLimit)
			}
		}
	}
	documentID := strings.TrimSpace(derefString(epic.SpecDocumentID))
	if documentID == "" || s.docsContentRepo == nil {
		return ""
	}
	content, err := s.docsContentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		slog.WarnContext(ctx, "delegated epic launch context: spec draft lookup failed", "error", err, "epic_id", epic.ID, "document_id", documentID)
		return ""
	}
	markdown := delegatedDocsContentMarkdown(content)
	if markdown == "" {
		return ""
	}
	return "Current epic PRD draft [" + documentID + "]:\n" + truncateDelegatedLaunchText(markdown, delegatedEpicSpecContextLimit)
}

func (s *AgentService) delegatedEpicTasksSection(ctx context.Context, epic *model.PMEpic) string {
	if epic == nil || s.taskRepo == nil {
		return ""
	}
	tasks, err := s.taskRepo.ListByEpicID(ctx, epic.WorkspaceID, epic.ID)
	if err != nil {
		slog.WarnContext(ctx, "delegated epic launch context: task lookup failed", "error", err, "epic_id", epic.ID)
		return ""
	}
	if len(tasks) == 0 {
		return "Existing epic tasks: none."
	}
	lines := []string{fmt.Sprintf("Existing epic tasks (%d total; do not duplicate these):", len(tasks))}
	for index, task := range tasks {
		if index >= delegatedEpicTaskContextLimit {
			lines = append(lines, fmt.Sprintf("- ... %d more tasks omitted", len(tasks)-index))
			break
		}
		lines = append(lines, fmt.Sprintf("- #%d %s [id=%s type=%s completed=%t]", task.DisplayID, strings.TrimSpace(task.Name), task.ID, task.TaskType, task.Completed))
	}
	return strings.Join(lines, "\n")
}

func (s *AgentService) delegatedLinkedDocsSection(ctx context.Context, workspaceID, objectType, objectID, excludeDocumentID string) string {
	if s.docsLinkRepo == nil || s.docsDocumentRepo == nil || s.docsContentRepo == nil {
		return ""
	}
	links, err := s.docsLinkRepo.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		slog.WarnContext(ctx, "delegated launch context: linked docs lookup failed", "error", err, "object_type", objectType, "object_id", objectID)
		return ""
	}
	entries := make([]string, 0, 5)
	seen := map[string]bool{}
	for _, link := range links {
		if link.DocumentID == excludeDocumentID || seen[link.DocumentID] {
			continue
		}
		seen[link.DocumentID] = true
		doc, err := s.docsDocumentRepo.GetByID(ctx, link.DocumentID)
		if err != nil || doc == nil {
			continue
		}
		body := "(no content yet)"
		if content, err := s.docsContentRepo.GetByDocumentID(ctx, doc.ID); err == nil {
			if markdown := delegatedDocsContentMarkdown(content); markdown != "" {
				body = truncateDelegatedLaunchText(markdown, 3000)
			}
		}
		entries = append(entries, fmt.Sprintf("- %s [%s]\n%s", doc.Title, doc.ID, body))
		if len(entries) >= 5 {
			break
		}
	}
	return strings.Join(entries, "\n\n")
}

// delegatedTaskEpicSections mirrors the parent-epic background block of
// agentcontract.BuildUserPrompt for a task-scoped run.
func (s *AgentService) delegatedTaskEpicSections(ctx context.Context, task *model.PMTask) []string {
	epicID := strings.TrimSpace(derefString(task.EpicID))
	if epicID == "" || s.epicRepo == nil {
		return nil
	}
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		slog.WarnContext(ctx, "delegated task launch context: epic lookup failed", "error", err, "task_id", task.ID, "epic_id", epicID)
		return nil
	}
	if epicWithStats == nil {
		return nil
	}
	epic := &epicWithStats.Epic
	name := strings.TrimSpace(epic.Name)
	if name == "" {
		return nil
	}
	sections := []string{fmt.Sprintf("Parent epic background: **%s**\nThis run is scoped to the target task. Parent epic context is background only.", name)}
	if epic.Description != nil {
		if description := strings.TrimSpace(tiptap.RichTextToMarkdown(*epic.Description)); description != "" {
			sections = append(sections, "Parent epic description:\n"+description)
		}
	}
	return sections
}

// delegatedTaskBranchSection mirrors temporalapp
// appendTaskRunRepositoryBranchSections with the same value precedence
// createRun uses when stamping the run row: explicit request overrides win
// over the resolved delivery target values.
func delegatedTaskBranchSection(delivery *model.TaskDeliveryTarget, baseOverride, workingOverride *string) string {
	base := ""
	working := ""
	if delivery != nil {
		base = strings.TrimSpace(derefString(delivery.BaseBranch))
		working = strings.TrimSpace(derefString(delivery.WorkingBranch))
	}
	if value := strings.TrimSpace(derefString(baseOverride)); value != "" {
		base = value
	}
	if value := strings.TrimSpace(derefString(workingOverride)); value != "" {
		working = value
	}
	switch {
	case base != "" && working != "":
		return fmt.Sprintf("Repository branches: base `%s`, working `%s`.", base, working)
	case working != "":
		return fmt.Sprintf("Repository working branch: `%s`.", working)
	case base != "":
		return fmt.Sprintf("Repository base branch: `%s`.", base)
	default:
		return ""
	}
}

// delegatedTaskPlanDocumentSections mirrors temporalapp
// buildTaskRunPlanDocumentContextSections: canonical plan-doc header plus the
// truncated document markdown.
func (s *AgentService) delegatedTaskPlanDocumentSections(ctx context.Context, task *model.PMTask) []string {
	planDocumentID := strings.TrimSpace(derefString(task.PlanDocumentID))
	if planDocumentID == "" || s.docsContentRepo == nil {
		return nil
	}
	title := ""
	if s.docsDocumentRepo != nil {
		doc, err := s.docsDocumentRepo.GetByID(ctx, planDocumentID)
		if err != nil {
			slog.WarnContext(ctx, "delegated task launch context: plan document lookup failed", "error", err, "task_id", task.ID, "document_id", planDocumentID)
		} else if doc != nil {
			title = strings.TrimSpace(doc.Title)
		}
	}
	content, err := s.docsContentRepo.GetByDocumentID(ctx, planDocumentID)
	if err != nil {
		slog.WarnContext(ctx, "delegated task launch context: plan document content lookup failed", "error", err, "task_id", task.ID, "document_id", planDocumentID)
		return nil
	}
	markdown := delegatedDocsContentMarkdown(content)
	if markdown == "" {
		return nil
	}
	header := fmt.Sprintf("Canonical task planning document ID: %s", planDocumentID)
	if title != "" {
		header = fmt.Sprintf("Canonical task planning document: %s [%s]", title, planDocumentID)
	}
	return []string{header + "\n" + truncateDelegatedLaunchText(markdown, delegatedTaskPlanDocumentContextLimit)}
}

// delegatedDocsContentMarkdown mirrors temporalapp docsContentMarkdown.
func delegatedDocsContentMarkdown(content *model.DocsContent) string {
	if content == nil {
		return ""
	}
	if markdown := strings.TrimSpace(tiptap.RichTextToMarkdown(string(content.Content))); markdown != "" {
		return markdown
	}
	return strings.TrimSpace(content.ContentText)
}

// truncateDelegatedLaunchText mirrors temporalapp truncatePlanningText.
func truncateDelegatedLaunchText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... (truncated)"
}
