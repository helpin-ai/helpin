package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func plannerToolInputError(toolName string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s input must be valid JSON: %w", toolName, err)
}

func toolEnsureEpicSpecDoc(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "epic" || ctx.TargetID == "" {
		return "", fmt.Errorf("ensure_epic_spec_doc is only available for epic runs")
	}
	if ctx.Services == nil || ctx.Services.EnsureEpicSpecDoc == nil {
		return "", fmt.Errorf("epic spec document service is not available")
	}

	doc, err := ctx.Services.EnsureEpicSpecDoc(ctx.Context, ctx.WorkspaceID, ctx.TargetID, ctx.AgentID)
	if err != nil {
		return "", fmt.Errorf("ensure epic spec doc: %w", err)
	}
	if doc == nil {
		return "", fmt.Errorf("epic spec document was not created")
	}

	hasApprovedSpec := ctx.Epic != nil && ctx.Epic.ApprovedSpecVersionID != nil && strings.TrimSpace(*ctx.Epic.ApprovedSpecVersionID) != ""
	taskCount := len(ctx.EpicTasks)

	// Check if spec doc has draft content (written but not approved).
	hasDraftContent := false
	if !hasApprovedSpec && ctx.Services != nil && ctx.Services.GetDocumentContent != nil {
		if contentText, err := ctx.Services.GetDocumentContent(ctx.Context, doc.ID); err == nil && strings.TrimSpace(contentText) != "" {
			hasDraftContent = true
		}
	}

	var planningHint string
	switch {
	case hasApprovedSpec && taskCount > 0:
		planningHint = "prd_approved_tasks_exist"
	case hasApprovedSpec:
		planningHint = "prd_approved_no_tasks"
	case hasDraftContent:
		planningHint = "prd_draft_exists"
	default:
		planningHint = "no_prd"
	}

	result := map[string]any{
		"document_id":       doc.ID,
		"title":             doc.Title,
		"status":            doc.Status,
		"has_approved_spec": hasApprovedSpec,
		"has_draft_content": hasDraftContent,
		"task_count":        taskCount,
		"planning_hint":     planningHint,
	}
	if hasApprovedSpec {
		result["approved_spec_version_id"] = strings.TrimSpace(*ctx.Epic.ApprovedSpecVersionID)
	}

	return toCompactJSONString(result), nil
}

func toolEnsureTaskPlanDoc(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "task" || ctx.TargetID == "" {
		return "", fmt.Errorf("ensure_task_plan_doc is only available for task runs")
	}
	if ctx.Services == nil || ctx.Services.EnsureTaskPlanDoc == nil {
		return "", fmt.Errorf("task plan document service is not available")
	}

	doc, err := ctx.Services.EnsureTaskPlanDoc(ctx.Context, ctx.WorkspaceID, ctx.TargetID, ctx.AgentID)
	if err != nil {
		return "", fmt.Errorf("ensure task plan doc: %w", err)
	}
	if doc == nil {
		return "", fmt.Errorf("task plan document was not created")
	}

	hasDraftContent := false
	if ctx.Services != nil && ctx.Services.GetDocumentContent != nil {
		if contentText, err := ctx.Services.GetDocumentContent(ctx.Context, doc.ID); err == nil && strings.TrimSpace(contentText) != "" {
			hasDraftContent = true
		}
	}

	result := map[string]any{
		"document_id":       doc.ID,
		"title":             doc.Title,
		"status":            doc.Status,
		"has_draft_content": hasDraftContent,
	}
	return toCompactJSONString(result), nil
}

func toolApproveEpicSpec(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "epic" || ctx.TargetID == "" {
		return "", fmt.Errorf("approve_epic_spec is only available for epic runs")
	}

	var params struct {
		VersionID *string `json:"version_id"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return "", plannerToolInputError("approve_epic_spec", err)
		}
	}
	if output, ok, err := executeInternalCommand(ctx, "epic", ctx.TargetID, "pm.approve_epic_spec", input); ok {
		if err != nil {
			return "", fmt.Errorf("approve epic spec: %w", err)
		}
		return string(output), nil
	}
	if ctx.Services == nil || ctx.Services.ApproveEpicSpec == nil {
		return "", fmt.Errorf("epic spec approval is not available")
	}

	summary, err := ctx.Services.ApproveEpicSpec(ctx.Context, ctx.WorkspaceID, ctx.TargetID, ctx.AgentID, params.VersionID)
	if err != nil {
		return "", fmt.Errorf("approve epic spec: %w", err)
	}
	return toCompactJSONString(summary), nil
}

func toolCreateTaskBatch(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "epic" || ctx.TargetID == "" {
		return "", fmt.Errorf("create_task_batch is only available for epic runs")
	}

	var params struct {
		Stories         []model.ProposedStory `json:"stories"`
		ProposedStories []model.ProposedStory `json:"proposed_stories"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", plannerToolInputError("create_task_batch", err)
	}
	if len(params.Stories) == 0 && len(params.ProposedStories) > 0 {
		params.Stories = params.ProposedStories
	}
	if len(params.Stories) == 0 {
		return "", fmt.Errorf("create_task_batch requires \"stories\" (legacy alias: \"proposed_stories\")")
	}
	if err := model.NormalizeProposedStories(params.Stories); err != nil {
		return "", fmt.Errorf("create_task_batch stories are invalid: %w", err)
	}

	// Hard guard: stories require an approved spec.
	if ctx.Epic == nil || ctx.Epic.ApprovedSpecVersionID == nil || strings.TrimSpace(*ctx.Epic.ApprovedSpecVersionID) == "" {
		return "", fmt.Errorf("create_task_batch requires an approved PRD first; call approve_epic_spec before creating tasks")
	}
	commandInput, _ := json.Marshal(map[string]any{
		"stories": params.Stories,
	})
	if output, ok, err := executeInternalCommand(ctx, "epic", ctx.TargetID, "pm.create_task_batch", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("create task batch: %w", err)
		}
		return string(output), nil
	}
	if ctx.Services == nil || ctx.Services.CreateTaskBatch == nil {
		return "", fmt.Errorf("task batch creation is not available")
	}

	result, err := ctx.Services.CreateTaskBatch(ctx.Context, ctx.WorkspaceID, ctx.TargetID, ctx.AgentID, params.Stories)
	if err != nil {
		return "", fmt.Errorf("create task batch: %w", err)
	}
	return toCompactJSONString(result), nil
}

func toolAssignTaskAgent(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		StoryID string `json:"story_id"`
		TaskID  string `json:"task_id"`
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", plannerToolInputError("assign_task_agent", err)
	}
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(params.StoryID)
	}
	if taskID == "" || strings.TrimSpace(params.AgentID) == "" {
		return "", fmt.Errorf("assign_task_agent requires \"task_id\" and \"agent_id\"")
	}
	if output, ok, err := executeInternalCommand(ctx, "task", taskID, "pm.assign_task_agent", input); ok {
		if err != nil {
			return "", fmt.Errorf("assign task agent: %w", err)
		}
		return string(output), nil
	}
	if ctx.Services == nil || ctx.Services.AssignTaskAgent == nil {
		return "", fmt.Errorf("task assignment is not available")
	}
	if err := ctx.Services.AssignTaskAgent(ctx.Context, ctx.WorkspaceID, ctx.AgentID, taskID, params.AgentID); err != nil {
		return "", fmt.Errorf("assign task agent: %w", err)
	}
	return fmt.Sprintf("Assigned task %s to agent %s.", taskID, params.AgentID), nil
}

func toolSetTaskDependencies(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Dependencies []TaskDependencyLink `json:"dependencies"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", plannerToolInputError("set_task_dependencies", err)
	}
	if len(params.Dependencies) == 0 {
		return "", fmt.Errorf("set_task_dependencies requires \"dependencies\"")
	}
	for idx, link := range params.Dependencies {
		if strings.TrimSpace(link.SourceTaskID) == "" || strings.TrimSpace(link.TargetTaskID) == "" {
			return "", fmt.Errorf("set_task_dependencies dependencies[%d] must include \"source_task_id\"/\"source_story_id\" and \"target_task_id\"/\"target_story_id\"", idx)
		}
	}
	if output, ok, err := executeInternalCommand(ctx, "epic", ctx.TargetID, "pm.set_task_dependencies", input); ok {
		if err != nil {
			return "", fmt.Errorf("set task dependencies: %w", err)
		}
		return string(output), nil
	}
	if ctx.Services == nil || ctx.Services.SetTaskDependencies == nil {
		return "", fmt.Errorf("task dependency updates are not available")
	}
	if err := ctx.Services.SetTaskDependencies(ctx.Context, ctx.WorkspaceID, ctx.AgentID, params.Dependencies); err != nil {
		return "", fmt.Errorf("set task dependencies: %w", err)
	}
	return fmt.Sprintf("Created %d dependency links.", len(params.Dependencies)), nil
}

func toolListEpicTasks(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "epic" || ctx.TargetID == "" {
		return "", fmt.Errorf("list_epic_tasks is only available for epic runs")
	}
	if ctx.Services == nil || ctx.Services.ListEpicTasks == nil {
		return "", fmt.Errorf("epic story listing is not available")
	}

	tasks, err := ctx.Services.ListEpicTasks(ctx.Context, ctx.WorkspaceID, ctx.TargetID)
	if err != nil {
		return "", fmt.Errorf("list epic tasks: %w", err)
	}

	return toCompactJSONString(map[string]any{
		"epic_id":    ctx.TargetID,
		"task_count": len(tasks),
		"tasks":      tasks,
	}), nil
}
