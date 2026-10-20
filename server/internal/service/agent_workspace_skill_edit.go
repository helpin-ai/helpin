package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func skillEditableVersion(title, description, instructions string) string {
	payload := fmt.Sprintf("%d:%s%d:%s%d:%s", len(title), title, len(description), description, len(instructions), instructions)
	return agentcontract.SkillVersionForBytes([]byte(payload))
}

func workspaceSkillDefaultContent(skill *model.WorkspaceSkill) *model.SkillEditableContent {
	var content model.SkillEditableContent
	if len(skill.DefaultContent) > 0 && json.Unmarshal(skill.DefaultContent, &content) == nil {
		return &content
	}
	return &model.SkillEditableContent{Title: skill.Title, Description: stringOrDefault(skill.Description, ""), Instructions: skill.Instructions}
}

// UpdateWorkspaceSkill accepts a workspace UUID or a built-in key. Built-in edits
// are explicit workspace overrides; old materialized built-ins remain defaults.
func (s *AgentService) UpdateWorkspaceSkill(ctx context.Context, workspaceID, skillID string, req model.UpdateWorkspaceSkillRequest) (*model.WorkspaceSkillResponse, error) {
	if s.workspaceSkillRepo == nil || s.skillPackageStore == nil {
		return nil, ErrWorkspaceSkillStorageUnavailable
	}
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	var skill *model.WorkspaceSkill
	var err error
	builtIn, isBuiltIn := agentcontract.GetBuiltInSkill(strings.TrimPrefix(skillID, agentRuntimeHelpinBuiltInSkillIDPrefix))
	if isBuiltIn {
		skill, err = s.workspaceSkillRepo.GetActiveByKey(ctx, workspaceID, builtIn.Key)
		if err == nil && skill != nil && skill.SourceKind != model.WorkspaceSkillSourceBuiltIn {
			return nil, fmt.Errorf("a custom skill already uses this key; edit it by ID")
		}
	} else {
		if _, err := uuid.Parse(skillID); err != nil {
			return nil, ErrWorkspaceSkillNotFound
		}
		skill, err = s.workspaceSkillRepo.GetByID(ctx, workspaceID, skillID)
	}
	if err != nil {
		return nil, err
	}
	creating := skill == nil && isBuiltIn
	if creating {
		skill = &model.WorkspaceSkill{ID: uuid.NewString(), WorkspaceID: workspaceID, SourceKind: model.WorkspaceSkillSourceBuiltIn, Key: builtIn.Key}
	}
	if skill == nil || skill.IsArchived {
		return nil, ErrWorkspaceSkillNotFound
	}
	if skill.SourceKind == model.WorkspaceSkillSourceBuiltIn {
		builtIn, isBuiltIn = agentcontract.GetBuiltInSkill(skill.Key)
		if !isBuiltIn {
			return nil, fmt.Errorf("built-in skill is no longer available")
		}
	}
	currentVersion := skill.VersionKey
	if isBuiltIn && !skill.IsCustomized {
		currentVersion = skillEditableVersion(builtIn.Title, builtIn.Description, builtIn.Instructions)
	}
	if req.ExpectedVersion != nil && *req.ExpectedVersion != currentVersion {
		return nil, fmt.Errorf("skill changed since it was opened; reload and try again")
	}
	var original []byte
	if isBuiltIn {
		original, _, _, err = agentcontract.BuildBuiltInSkillArchive(builtIn)
	} else {
		objectKey := skill.PackageObjectKey
		if req.RestoreDefault && skill.DefaultPackageObjectKey != "" {
			objectKey = skill.DefaultPackageObjectKey
		}
		original, err = s.skillPackageStore.GetObject(ctx, objectKey)
	}
	if err != nil {
		return nil, fmt.Errorf("load skill package: %w", err)
	}
	def, err := agentcontract.LoadSkillArchive(original, skill.SourceKind)
	if err != nil {
		return nil, err
	}
	// Snapshot defaults only before the first edit; never overwrite originals.
	if len(skill.DefaultContent) == 0 {
		skill.DefaultContent = marshalJSONBlob(model.SkillEditableContent{Title: def.Title, Description: def.Description, Instructions: def.Instructions}, nil)
	}
	if !isBuiltIn && skill.DefaultPackageObjectKey == "" {
		skill.DefaultPackageObjectKey = skill.PackageObjectKey
	}
	if !req.RestoreDefault && !creating {
		def.Title, def.Description, def.Instructions = skill.Title, stringOrDefault(skill.Description, ""), skill.Instructions
		// Old materialized built-ins are not user overrides.
		if isBuiltIn && !skill.IsCustomized {
			def = builtIn
		}
	}
	def.Key = skill.Key
	if req.Key != nil {
		key, err := normalizeWorkspaceSkillKey(*req.Key)
		if err != nil {
			return nil, err
		}
		if isBuiltIn && key != def.Key {
			return nil, fmt.Errorf("built-in skill keys cannot be changed")
		}
		if key != def.Key {
			if _, reserved := agentcontract.GetBuiltInSkill(key); reserved {
				return nil, fmt.Errorf("skill key is reserved for a built-in skill")
			}
			other, err := s.workspaceSkillRepo.GetActiveByKey(ctx, workspaceID, key)
			if err != nil {
				return nil, err
			}
			if other != nil && other.ID != skill.ID {
				return nil, fmt.Errorf("a workspace skill with key %q already exists", key)
			}
			def.Key = key
		}
	}
	if req.Title != nil {
		def.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		def.Description = strings.TrimSpace(*req.Description)
	}
	if req.Instructions != nil {
		def.Instructions = strings.TrimSpace(*req.Instructions)
	}
	if def.Title == "" || def.Description == "" || def.Instructions == "" {
		return nil, fmt.Errorf("title, description, and instructions are required")
	}
	if isBuiltIn && (req.RequiredTools != nil || req.SupportedRuntimes != nil || req.SourceRuntime != nil) {
		return nil, fmt.Errorf("built-in tool and runtime requirements are managed by Helpin")
	}
	if req.RequiredTools != nil {
		def.RequiredTools = agentcontract.SortedUniqueStrings(req.RequiredTools)
	}
	if req.SupportedRuntimes != nil {
		def.SupportedRuntimes = agentcontract.SortedUniqueStrings(req.SupportedRuntimes)
	}
	if def.Interface.DisplayName != "" {
		def.Interface.DisplayName = def.Title
	}
	if def.Interface.ShortDescription != "" {
		def.Interface.ShortDescription = def.Description
	}
	archive, checksum, filename, err := agentcontract.EditSkillArchive(original, def)
	if err != nil {
		return nil, err
	}
	// Unique immutable objects ensure a failed/concurrent save cannot corrupt the
	// package referenced by the existing row or its restore baseline.
	objectKey := fmt.Sprintf("workspaces/%s/skills/%s/%s/%s", workspaceID, skill.ID, uuid.NewString(), filename)
	if err := s.skillPackageStore.PutObject(ctx, objectKey, "application/zip", int64(len(archive)), bytes.NewReader(archive), false); err != nil {
		return nil, fmt.Errorf("store skill archive: %w", err)
	}
	previousVersion := skill.VersionKey
	skill.Key, skill.Title, skill.Instructions = def.Key, def.Title, def.Instructions
	skill.Description = trimPtr(&def.Description)
	skill.RequiredTools = marshalJSONBlob(def.RequiredTools, []byte("[]"))
	skill.SupportedRuntimes = marshalJSONBlob(def.SupportedRuntimes, []byte("[]"))
	skill.InterfaceConfig = marshalJSONBlob(def.Interface, []byte("{}"))
	skill.PolicyConfig = marshalJSONBlob(def.Policy, []byte("{}"))
	if req.SourceRuntime != nil {
		skill.SourceRuntime = trimPtr(req.SourceRuntime)
	}
	skill.VersionKey, skill.PackageChecksum = agentcontract.SkillVersionForBytes(archive), checksum
	skill.PackageObjectKey, skill.PackageFileName, skill.PackageSize = objectKey, filename, int64(len(archive))
	defaults := workspaceSkillDefaultContent(skill)
	if isBuiltIn {
		defaults = &model.SkillEditableContent{Title: builtIn.Title, Description: builtIn.Description, Instructions: builtIn.Instructions}
	}
	skill.IsCustomized = def.Title != defaults.Title || def.Description != defaults.Description || def.Instructions != defaults.Instructions
	if creating {
		err = s.workspaceSkillRepo.Create(ctx, skill)
	} else {
		err = s.workspaceSkillRepo.UpdateIfVersion(ctx, skill, previousVersion)
	}
	if err != nil {
		if cleanupErr := s.skillPackageStore.DeleteObject(ctx, objectKey); cleanupErr != nil {
			slog.WarnContext(ctx, "failed to clean up unsaved skill package", "workspace_id", workspaceID, "skill_id", skill.ID, "error", cleanupErr)
		}
		return nil, err
	}
	slog.InfoContext(ctx, "workspace skill updated", "workspace_id", workspaceID, "skill_id", skill.ID, "restored_default", req.RestoreDefault)
	s.publishSimpleEvent("updated", "workspace_skill", skill.ID, workspaceID, "")
	resp := workspaceSkillResponse(skill)
	return &resp, nil
}
