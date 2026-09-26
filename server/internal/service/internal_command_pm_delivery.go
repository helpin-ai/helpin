package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type pmDeliveryCommandInput struct {
	TaskID        string  `json:"task_id"`
	EpicID        string  `json:"epic_id"`
	RepositoryID  *string `json:"repository_id"`
	BaseBranch    *string `json:"base_branch"`
	WorkingBranch *string `json:"working_branch"`
	EpicBranch    *string `json:"epic_branch"`
	ClearTarget   bool    `json:"clear_target"`
}

func (s *InternalCommandService) registerPMDeliveryCommands() {
	s.register(InternalCommandDefinition{
		Name: "pm.update_task_delivery_target", Module: "pm", Mutating: true,
		SupportedTargetTypes: []string{"workspace", "task"},
		Tool:                 mustCommandToolMetadata("pm.update_task_delivery_target"), Execute: s.executePMUpdateTaskDeliveryTarget,
	})
	s.register(InternalCommandDefinition{
		Name: "pm.update_epic_delivery_target", Module: "pm", Mutating: true,
		SupportedTargetTypes: []string{"workspace", "epic"},
		Tool:                 mustCommandToolMetadata("pm.update_epic_delivery_target"), Execute: s.executePMUpdateEpicDeliveryTarget,
	})
}

func (s *InternalCommandService) executePMUpdateTaskDeliveryTarget(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req pmDeliveryCommandInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse task delivery target input: %w", err)
	}
	taskID, err := resolveCommandEntityID(meta, req.TaskID, "task")
	if err != nil {
		return nil, err
	}
	if s.gitService == nil {
		return nil, fmt.Errorf("git service is not configured")
	}
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	task, err := s.resolveWritableCommandTask(ctx, meta, taskID)
	if err != nil {
		return nil, err
	}
	if err := validateDeliveryCommandInput(req.RepositoryID, req.BaseBranch, req.WorkingBranch, req.ClearTarget); err != nil {
		return nil, err
	}
	target, err := s.gitService.UpdateTaskDeliveryTarget(ctx, meta.WorkspaceID, task.ID, model.UpdateTaskDeliveryTargetRequest{
		RepositoryID: normalizeOptionalCommandString(req.RepositoryID), BaseBranch: normalizeOptionalCommandString(req.BaseBranch),
		WorkingBranch: normalizeOptionalCommandString(req.WorkingBranch), ClearTarget: req.ClearTarget,
	}, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(target), nil
}

func (s *InternalCommandService) executePMUpdateEpicDeliveryTarget(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req pmDeliveryCommandInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse epic delivery target input: %w", err)
	}
	epicID, err := resolvePMCommandEpicID(meta, req.EpicID, true)
	if err != nil {
		return nil, err
	}
	if s.gitService == nil {
		return nil, fmt.Errorf("git service is not configured")
	}
	if s.epicService == nil {
		return nil, fmt.Errorf("epic service is not configured")
	}
	epic, err := s.epicService.GetByID(ctx, epicID)
	if err != nil {
		return nil, err
	}
	if epic == nil || epic.Epic.WorkspaceID != meta.WorkspaceID {
		return nil, errCommandNotFound("epic")
	}
	if err := requireCommandAgentTeam(meta, epic.Epic.TeamID); err != nil {
		return nil, err
	}
	if err := validateDeliveryCommandInput(req.RepositoryID, req.BaseBranch, req.EpicBranch, req.ClearTarget); err != nil {
		return nil, err
	}
	target, err := s.gitService.UpdateEpicDeliveryTarget(ctx, meta.WorkspaceID, epicID, model.UpdateEpicDeliveryTargetRequest{
		RepositoryID: normalizeOptionalCommandString(req.RepositoryID), BaseBranch: normalizeOptionalCommandString(req.BaseBranch),
		EpicBranch: normalizeOptionalCommandString(req.EpicBranch), ClearTarget: req.ClearTarget,
	}, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(target), nil
}

func validateDeliveryCommandInput(repositoryID, baseBranch, targetBranch *string, clear bool) error {
	if clear {
		if repositoryID != nil || baseBranch != nil || targetBranch != nil {
			return fmt.Errorf("clear_target cannot be combined with repository or branch fields")
		}
		return nil
	}
	if strings.TrimSpace(derefCommandString(repositoryID)) == "" && strings.TrimSpace(derefCommandString(baseBranch)) == "" && strings.TrimSpace(derefCommandString(targetBranch)) == "" {
		return errCommandInput("at least one delivery target field is required")
	}
	return nil
}

func derefCommandString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
