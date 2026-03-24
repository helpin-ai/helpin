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
	storyCount := len(ctx.EpicStories)

	// Check if spec doc has draft content (written but not approved).
	hasDraftContent := false
	if !hasApprovedSpec && ctx.Services != nil && ctx.Services.GetDocumentContent != nil {
		if contentText, err := ctx.Services.GetDocumentContent(ctx.Context, doc.ID); err == nil && strings.TrimSpace(contentText) != "" {
			hasDraftContent = true
		}
	}

	var planningHint string
	switch {
	case hasApprovedSpec && storyCount > 0:
		planningHint = "prd_approved_stories_exist"
	case hasApprovedSpec:
		planningHint = "prd_approved_no_stories"
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
		"story_count":       storyCount,
		"planning_hint":     planningHint,
	}
	if hasApprovedSpec {
		result["approved_spec_version_id"] = strings.TrimSpace(*ctx.Epic.ApprovedSpecVersionID)
	}

	payload, _ := json.MarshalIndent(result, "", "  ")
	return string(payload), nil
}

func toolEnsureStoryPlanDoc(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "story" || ctx.TargetID == "" {
		return "", fmt.Errorf("ensure_story_plan_doc is only available for story runs")
	}
	if ctx.Services == nil || ctx.Services.EnsureStoryPlanDoc == nil {
		return "", fmt.Errorf("story plan document service is not available")
	}

	doc, err := ctx.Services.EnsureStoryPlanDoc(ctx.Context, ctx.WorkspaceID, ctx.TargetID, ctx.AgentID)
	if err != nil {
		return "", fmt.Errorf("ensure story plan doc: %w", err)
	}
	if doc == nil {
		return "", fmt.Errorf("story plan document was not created")
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
	payload, _ := json.MarshalIndent(result, "", "  ")
	return string(payload), nil
}

func toolApproveEpicSpec(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "epic" || ctx.TargetID == "" {
		return "", fmt.Errorf("approve_epic_spec is only available for epic runs")
	}
	if ctx.Services == nil || ctx.Services.ApproveEpicSpec == nil {
		return "", fmt.Errorf("epic spec approval is not available")
	}

	var params struct {
		VersionID *string `json:"version_id"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return "", plannerToolInputError("approve_epic_spec", err)
		}
	}

	summary, err := ctx.Services.ApproveEpicSpec(ctx.Context, ctx.WorkspaceID, ctx.TargetID, ctx.AgentID, params.VersionID)
	if err != nil {
		return "", fmt.Errorf("approve epic spec: %w", err)
	}
	payload, _ := json.MarshalIndent(summary, "", "  ")
	return string(payload), nil
}

func toolCreateStoryBatch(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "epic" || ctx.TargetID == "" {
		return "", fmt.Errorf("create_story_batch is only available for epic runs")
	}
	if ctx.Services == nil || ctx.Services.CreateStoryBatch == nil {
		return "", fmt.Errorf("story batch creation is not available")
	}

	var params struct {
		Stories         []model.ProposedStory `json:"stories"`
		ProposedStories []model.ProposedStory `json:"proposed_stories"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", plannerToolInputError("create_story_batch", err)
	}
	if len(params.Stories) == 0 && len(params.ProposedStories) > 0 {
		params.Stories = params.ProposedStories
	}
	if len(params.Stories) == 0 {
		return "", fmt.Errorf("create_story_batch requires \"stories\" (legacy alias: \"proposed_stories\")")
	}
	if err := model.NormalizeProposedStories(params.Stories); err != nil {
		return "", fmt.Errorf("create_story_batch stories are invalid: %w", err)
	}

	// Hard guard: stories require an approved spec.
	if ctx.Epic == nil || ctx.Epic.ApprovedSpecVersionID == nil || strings.TrimSpace(*ctx.Epic.ApprovedSpecVersionID) == "" {
		return "", fmt.Errorf("create_story_batch requires an approved PRD first; call approve_epic_spec before creating stories")
	}

	result, err := ctx.Services.CreateStoryBatch(ctx.Context, ctx.WorkspaceID, ctx.TargetID, ctx.AgentID, params.Stories)
	if err != nil {
		return "", fmt.Errorf("create story batch: %w", err)
	}
	payload, _ := json.MarshalIndent(result, "", "  ")
	return string(payload), nil
}

func toolAssignStoryAgent(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.AssignStoryAgent == nil {
		return "", fmt.Errorf("story assignment is not available")
	}
	var params struct {
		StoryID string `json:"story_id"`
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", plannerToolInputError("assign_story_agent", err)
	}
	if strings.TrimSpace(params.StoryID) == "" || strings.TrimSpace(params.AgentID) == "" {
		return "", fmt.Errorf("assign_story_agent requires \"story_id\" and \"agent_id\"")
	}
	if err := ctx.Services.AssignStoryAgent(ctx.Context, ctx.WorkspaceID, ctx.AgentID, params.StoryID, params.AgentID); err != nil {
		return "", fmt.Errorf("assign story agent: %w", err)
	}
	return fmt.Sprintf("Assigned story %s to agent %s.", params.StoryID, params.AgentID), nil
}

func toolSetStoryDependencies(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.SetStoryDependencies == nil {
		return "", fmt.Errorf("story dependency updates are not available")
	}
	var params struct {
		Dependencies []StoryDependencyLink `json:"dependencies"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", plannerToolInputError("set_story_dependencies", err)
	}
	if len(params.Dependencies) == 0 {
		return "", fmt.Errorf("set_story_dependencies requires \"dependencies\"")
	}
	for idx, link := range params.Dependencies {
		if strings.TrimSpace(link.SourceStoryID) == "" || strings.TrimSpace(link.TargetStoryID) == "" {
			return "", fmt.Errorf("set_story_dependencies dependencies[%d] must include \"source_story_id\" and \"target_story_id\"", idx)
		}
	}
	if err := ctx.Services.SetStoryDependencies(ctx.Context, ctx.WorkspaceID, ctx.AgentID, params.Dependencies); err != nil {
		return "", fmt.Errorf("set story dependencies: %w", err)
	}
	return fmt.Sprintf("Created %d dependency links.", len(params.Dependencies)), nil
}

func toolListEpicStories(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TargetType != "epic" || ctx.TargetID == "" {
		return "", fmt.Errorf("list_epic_stories is only available for epic runs")
	}
	if ctx.Services == nil || ctx.Services.ListEpicStories == nil {
		return "", fmt.Errorf("epic story listing is not available")
	}

	stories, err := ctx.Services.ListEpicStories(ctx.Context, ctx.WorkspaceID, ctx.TargetID)
	if err != nil {
		return "", fmt.Errorf("list epic stories: %w", err)
	}

	payload, _ := json.MarshalIndent(map[string]any{
		"epic_id":     ctx.TargetID,
		"story_count": len(stories),
		"stories":     stories,
	}, "", "  ")
	return string(payload), nil
}
