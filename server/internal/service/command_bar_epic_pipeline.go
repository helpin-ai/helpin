package service

// Epic delivery pipeline: builds and dispatches the dependency-aware
// Forge → Lens → merge pipeline across an epic's open tasks as a
// task_pipeline_fan_out plan (ensure-epic-branch first, open-epic-PR last).
// This replaces the retired keyword-triggered command-bar parser: the
// pipeline is now started explicitly — from the epic page button, from an
// epic-target agent run via the run_epic_delivery_pipeline tool, or from a
// dock chat behind the usual approval.

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// EpicDeliveryPipelineSkippedTask reports a task excluded from the pipeline.
type EpicDeliveryPipelineSkippedTask struct {
	TaskID string `json:"task_id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// EpicDeliveryPipelinePlan is the constructed (not yet dispatched) pipeline.
type EpicDeliveryPipelinePlan struct {
	Epic            model.CommandBarPageContext       `json:"epic"`
	Steps           []model.CommandBarPlanStep        `json:"steps"`
	SkippedTasks    []EpicDeliveryPipelineSkippedTask `json:"skipped_tasks,omitempty"`
	DependencyEdges int                               `json:"dependency_edges"`
	TaskCount       int                               `json:"task_count"`
}

// EpicDeliveryPipelineResponse is returned by StartEpicDeliveryPipeline.
type EpicDeliveryPipelineResponse struct {
	PlanID          string                            `json:"plan_id"`
	RunCount        int                               `json:"run_count"`
	Runs            []model.AgentRun                  `json:"runs"`
	SkippedTasks    []EpicDeliveryPipelineSkippedTask `json:"skipped_tasks,omitempty"`
	DependencyEdges int                               `json:"dependency_edges"`
	TaskCount       int                               `json:"task_count"`
}

// BuildEpicDeliveryPipeline constructs the pipeline for an epic's open,
// unmerged tasks. Returns an error when the epic has nothing left to deliver.
func (s *CommandBarService) BuildEpicDeliveryPipeline(ctx context.Context, workspaceID, epicID string) (*EpicDeliveryPipelinePlan, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	epicID = strings.TrimSpace(epicID)
	if epicID == "" {
		return nil, fmt.Errorf("epic_id is required")
	}
	epic, err := s.agentService.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epic == nil || epic.Epic.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("epic not found")
	}
	epicContext := model.CommandBarPageContext{
		EntityType:   "epic",
		EntityID:     epic.Epic.ID,
		DisplayTitle: strings.TrimSpace(epic.Epic.Name),
	}

	forge, err := s.agentService.ensureBuiltInAgent(ctx, workspaceID, "", model.AgentPresetCodeBuilder)
	if err != nil {
		return nil, fmt.Errorf("resolve code builder agent: %w", err)
	}
	lens, err := s.agentService.ensureBuiltInAgent(ctx, workspaceID, "", model.AgentPresetReviewAgent)
	if err != nil {
		return nil, fmt.Errorf("resolve review agent: %w", err)
	}
	commandAgent, err := s.agentService.ensureBuiltInAgent(ctx, workspaceID, "", model.AgentPresetCommandAgent)
	if err != nil {
		return nil, fmt.Errorf("resolve sub-agent: %w", err)
	}

	tasks, err := s.agentService.taskRepo.ListByEpicID(ctx, workspaceID, epicID)
	if err != nil {
		return nil, fmt.Errorf("list epic tasks: %w", err)
	}
	var skipped []EpicDeliveryPipelineSkippedTask
	open := make([]model.PMTask, 0, len(tasks))
	for _, task := range tasks {
		if task.Completed {
			skipped = append(skipped, EpicDeliveryPipelineSkippedTask{TaskID: task.ID, Title: epicPipelineTaskTitle(task), Reason: "completed"})
			continue
		}
		merged, mergedErr := s.epicPipelineTaskAlreadyMerged(ctx, workspaceID, epicID, task.ID)
		if mergedErr != nil {
			slog.WarnContext(ctx, "epic pipeline merge-state lookup failed",
				"error", mergedErr, "workspace_id", workspaceID, "epic_id", epicID, "task_id", task.ID)
		} else if merged {
			skipped = append(skipped, EpicDeliveryPipelineSkippedTask{TaskID: task.ID, Title: epicPipelineTaskTitle(task), Reason: "merged into the epic branch"})
			continue
		}
		open = append(open, task)
	}
	if len(open) == 0 {
		return nil, fmt.Errorf("all epic tasks are already completed or merged into the epic branch")
	}
	slices.SortFunc(open, func(a, b model.PMTask) int {
		if a.DisplayID != b.DisplayID {
			if a.DisplayID < b.DisplayID {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})

	steps := make([]model.CommandBarPlanStep, 0, min(len(open)*3+2, maxCommandBarPlanSteps))
	ensureBranchIndex := len(steps)
	steps = append(steps, model.CommandBarPlanStep{
		AgentID:      commandAgent.ID,
		AgentKey:     commandAgent.PresetKey,
		AgentName:    commandAgent.Name,
		PlanKind:     model.CommandBarPlanKindTaskPipeline,
		StepType:     model.CommandBarStepTypeEnsureEpicBranch,
		Target:       epicContext,
		Instructions: fmt.Sprintf("Create or reuse the epic integration branch for %q from the configured base branch, then configure child task branches to use that epic branch as base.", epicContext.DisplayTitle),
	})
	forgeStepByTask := map[string]int{}
	mergeStepByTask := map[string]int{}
	for _, task := range open {
		if len(steps)+3+1 > maxCommandBarPlanSteps {
			skipped = append(skipped, EpicDeliveryPipelineSkippedTask{TaskID: task.ID, Title: epicPipelineTaskTitle(task), Reason: "plan step limit reached"})
			continue
		}
		target := model.CommandBarPageContext{
			EntityType:   "task",
			EntityID:     task.ID,
			DisplayTitle: epicPipelineTaskTitle(task),
			RelatedIDs:   map[string][]string{"epic_ids": {epicID}},
		}
		forgeIndex := len(steps)
		forgeStepByTask[task.ID] = forgeIndex
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:              forge.ID,
			AgentKey:             forge.PresetKey,
			AgentName:            forge.Name,
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               target,
			Instructions:         fmt.Sprintf("Complete the implementation work for %s on its task branch. The task branch is based on this epic's integration branch. Respect task dependencies; this task is part of epic %q. Do not run review yourself; Helpin schedules review as the next step.", epicPipelineTaskTitle(task), epicContext.DisplayTitle),
			DependsOnStepIndexes: []int{ensureBranchIndex},
		})
		lensIndex := len(steps)
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:              lens.ID,
			AgentKey:             lens.PresetKey,
			AgentName:            lens.Name,
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               target,
			Instructions:         fmt.Sprintf("Review the completed implementation work for %s. Use the linked prior run as context when available.", epicPipelineTaskTitle(task)),
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
			Instructions:         fmt.Sprintf("Merge %s's task branch into the epic integration branch after review completes.", epicPipelineTaskTitle(task)),
			DependsOnStepIndexes: []int{lensIndex},
		})
	}

	// Order pipelines by blocking links between tasks in the plan.
	dependencyEdges := 0
	if s.agentService.taskLinkRepo != nil && len(forgeStepByTask) > 1 {
		ids := make([]string, 0, len(open))
		inPlan := make(map[string]bool, len(open))
		for _, task := range open {
			ids = append(ids, task.ID)
			inPlan[task.ID] = true
		}
		links, linkErr := s.agentService.taskLinkRepo.ListByTasks(ctx, workspaceID, ids)
		if linkErr != nil {
			slog.WarnContext(ctx, "epic pipeline dependency lookup failed", "error", linkErr, "workspace_id", workspaceID, "epic_id", epicID)
		}
		for _, link := range links {
			if link.LinkType != model.PMTaskLinkTypeBlocks || !inPlan[link.SourceTaskID] || !inPlan[link.TargetTaskID] {
				continue
			}
			sourceMerge, okSource := mergeStepByTask[link.SourceTaskID]
			targetForge, okTarget := forgeStepByTask[link.TargetTaskID]
			if !okSource || !okTarget {
				continue
			}
			if !slices.Contains(steps[targetForge].DependsOnStepIndexes, sourceMerge) {
				steps[targetForge].DependsOnStepIndexes = append(steps[targetForge].DependsOnStepIndexes, sourceMerge)
				dependencyEdges++
			}
		}
	}

	finalDeps := make([]int, 0, len(mergeStepByTask))
	for _, task := range open {
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
			Target:               epicContext,
			Instructions:         fmt.Sprintf("Open or reuse the final pull request from the epic integration branch for %q into the configured base branch.", epicContext.DisplayTitle),
			DependsOnStepIndexes: finalDeps,
		})
	}

	return &EpicDeliveryPipelinePlan{
		Epic:            epicContext,
		Steps:           steps,
		SkippedTasks:    skipped,
		DependencyEdges: dependencyEdges,
		TaskCount:       len(forgeStepByTask),
	}, nil
}

// DirectDispatchParams is the empty dispatch linkage used by HTTP callers
// (the epic-page button); dock-chat launches pass their chat linkage instead.
func DirectDispatchParams() dispatchPlanParams { return dispatchPlanParams{} }

// StartEpicDeliveryPipeline builds and dispatches the pipeline. params
// carries dock-chat linkage when launched from a chat.
func (s *CommandBarService) StartEpicDeliveryPipeline(ctx context.Context, workspaceID, actorID, epicID string, params dispatchPlanParams) (*EpicDeliveryPipelineResponse, error) {
	pipeline, err := s.BuildEpicDeliveryPipeline(ctx, workspaceID, epicID)
	if err != nil {
		return nil, err
	}
	dispatch, err := s.dispatchPlanCore(ctx, workspaceID, actorID, model.CommandBarDispatchRequest{
		Text:        fmt.Sprintf("Deliver epic %q: implement, review, and merge every open task, then open the epic PR.", pipeline.Epic.DisplayTitle),
		PageContext: pipeline.Epic,
		Steps:       pipeline.Steps,
	}, params)
	if err != nil {
		return nil, err
	}
	return &EpicDeliveryPipelineResponse{
		PlanID:          dispatch.PlanID,
		RunCount:        dispatch.RunCount,
		Runs:            dispatch.Runs,
		SkippedTasks:    pipeline.SkippedTasks,
		DependencyEdges: pipeline.DependencyEdges,
		TaskCount:       pipeline.TaskCount,
	}, nil
}

func (s *CommandBarService) epicPipelineTaskAlreadyMerged(ctx context.Context, workspaceID, epicID, taskID string) (bool, error) {
	if s == nil || s.agentService == nil || strings.TrimSpace(taskID) == "" {
		return false, nil
	}
	if s.agentService.gitService != nil {
		target, err := s.agentService.gitService.GetTaskDeliveryTarget(ctx, workspaceID, taskID)
		if err != nil {
			return false, err
		}
		if target != nil && epicPipelineDeliveryTargetIsMerged(*target, epicID) {
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
	}
	return false, nil
}

func epicPipelineDeliveryTargetIsMerged(target model.TaskDeliveryTarget, epicID string) bool {
	if strings.TrimSpace(target.DeliveryState) != "merged" && strings.TrimSpace(derefString(target.ActivePRStatus)) != "merged" {
		return false
	}
	sourceEpicID := strings.TrimSpace(derefString(target.SourceEpicID))
	return sourceEpicID == "" || strings.TrimSpace(epicID) == "" || sourceEpicID == strings.TrimSpace(epicID)
}

func epicPipelineTaskTitle(task model.PMTask) string {
	if task.DisplayID > 0 {
		return fmt.Sprintf("#%d %s", task.DisplayID, strings.TrimSpace(task.Name))
	}
	return strings.TrimSpace(firstNonEmptyString(task.Name, shortCommandBarID(task.ID)))
}
