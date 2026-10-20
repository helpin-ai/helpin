package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) registerSkillCommands() {
	s.register(InternalCommandDefinition{Name: "skills.list_workspace", Module: "skills", Tool: mustCommandToolMetadata("skills.list_workspace"), Execute: s.listWorkspaceSkillsCommand})
	s.register(InternalCommandDefinition{Name: "skills.update", Module: "skills", Mutating: true, Tool: mustCommandToolMetadata("skills.update"), Execute: s.updateSkillCommand})
}

func (s *InternalCommandService) listWorkspaceSkillsCommand(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.agentService == nil || meta.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace skill management is unavailable")
	}
	var req struct {
		Key     string `json:"key"`
		SkillID string `json:"skill_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	catalog, err := s.agentService.ListSkillCatalog(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	for _, skill := range catalog.Skills {
		if (req.Key != "" && skill.Key == req.Key) || (req.SkillID != "" && skill.ID != nil && *skill.ID == req.SkillID) {
			return json.Marshal(skill)
		}
	}
	if req.Key != "" || req.SkillID != "" {
		return nil, ErrWorkspaceSkillNotFound
	}
	for i := range catalog.Skills {
		catalog.Skills[i].Instructions = ""
		catalog.Skills[i].DefaultContent = nil
	}
	return json.Marshal(catalog)
}

func (s *InternalCommandService) updateSkillCommand(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.agentService == nil || meta.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace skill management is unavailable")
	}
	if strings.TrimSpace(meta.ActorID) == "" || strings.TrimSpace(meta.ActorRole) == "" {
		return nil, fmt.Errorf("skill editing requires an authorized workspace member")
	}
	var req struct {
		SkillID         string  `json:"skill_id"`
		Title           *string `json:"title,omitempty"`
		Description     *string `json:"description,omitempty"`
		Instructions    *string `json:"instructions,omitempty"`
		ExpectedVersion *string `json:"expected_version"`
		RestoreDefault  bool    `json:"restore_default,omitempty"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.ExpectedVersion == nil {
		return nil, fmt.Errorf("read the current skill and supply expected_version")
	}
	if req.Title == nil && req.Description == nil && req.Instructions == nil && !req.RestoreDefault {
		return nil, fmt.Errorf("provide a field to edit or restore_default")
	}
	result, err := s.agentService.UpdateWorkspaceSkill(ctx, meta.WorkspaceID, req.SkillID, model.UpdateWorkspaceSkillRequest{ExpectedVersion: req.ExpectedVersion, Title: req.Title, Description: req.Description, Instructions: req.Instructions, RestoreDefault: req.RestoreDefault})
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
