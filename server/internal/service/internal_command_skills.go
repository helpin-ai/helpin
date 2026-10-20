package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) registerSkillCommands() {
	s.register(InternalCommandDefinition{Name: "skills.list_workspace", Module: "skills", Tool: mustCommandToolMetadata("skills.list_workspace"), Execute: s.listWorkspaceSkillsCommand})
	s.register(InternalCommandDefinition{Name: "skills.validate_update", Module: "skills", Tool: mustCommandToolMetadata("skills.validate_update"), Execute: s.validateSkillUpdateCommand})
	s.register(InternalCommandDefinition{Name: "skills.update", Module: "skills", Mutating: true, Tool: mustCommandToolMetadata("skills.update"), Validate: s.validateSkillUpdate, Execute: s.updateSkillCommand})
}

func decodeSkillCommand(input json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected a single JSON object")
	}
	return nil
}

// Protect bounded pages from the runtime's ordinary tool-output summarization.
func marshalSkillToolResult(value any) (json.RawMessage, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	hint, _ := json.Marshal(map[string]any{"exempt": true, "max_runes": len([]rune(string(payload))) + 256})
	// Append rather than marshal a map so metadata stays ahead of instructions.
	return append(append(payload[:len(payload)-1], []byte(`,"_agent_runtime_compaction":`)...), append(hint, '}')...), nil
}

type skillToolMetadata struct {
	SkillID           string   `json:"skill_id"`
	Key               string   `json:"key"`
	VersionKey        string   `json:"version_key"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	SourceKind        string   `json:"source_kind"`
	IsCustomized      bool     `json:"is_customized"`
	ID                *string  `json:"id,omitempty"`
	SourceRuntime     *string  `json:"source_runtime,omitempty"`
	RequiredTools     []string `json:"required_tools,omitempty"`
	SupportedRuntimes []string `json:"supported_runtimes,omitempty"`
	Presets           []string `json:"presets,omitempty"`
}

func skillMetadata(skill model.SkillCatalogEntry) skillToolMetadata {
	id := skill.Key
	if skill.ID != nil {
		id = *skill.ID
	}
	return skillToolMetadata{SkillID: id, Key: skill.Key, VersionKey: skill.VersionKey, Title: skill.Title, Description: skill.Description, SourceKind: skill.SourceKind, IsCustomized: skill.IsCustomized, ID: skill.ID, SourceRuntime: skill.SourceRuntime, RequiredTools: skill.RequiredTools, SupportedRuntimes: skill.SupportedRuntimes, Presets: skill.Presets}
}
func (s *InternalCommandService) listWorkspaceSkillsCommand(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.agentService == nil || meta.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace skill management is unavailable")
	}
	var req struct {
		Key             string `json:"key"`
		SkillID         string `json:"skill_id"`
		Offset          int    `json:"offset"`
		Limit           int    `json:"limit"`
		ExpectedVersion string `json:"expected_version"`
	}
	if err := decodeSkillCommand(input, &req); err != nil {
		return nil, err
	}
	if req.Offset < 0 || req.Limit < 0 {
		return nil, fmt.Errorf("offset and limit must be nonnegative")
	}
	if req.Key != "" && req.SkillID != "" {
		return nil, fmt.Errorf("supply key or skill_id, not both")
	}
	catalog, err := s.agentService.ListSkillCatalog(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if req.Key != "" || req.SkillID != "" {
		for _, skill := range catalog.Skills {
			if skill.Key != req.Key && skillMetadata(skill).SkillID != req.SkillID && !(skill.SourceKind == model.WorkspaceSkillSourceBuiltIn && strings.TrimPrefix(req.SkillID, agentRuntimeHelpinBuiltInSkillIDPrefix) == skill.Key) {
				continue
			}
			if req.Offset > 0 && req.ExpectedVersion == "" {
				return nil, fmt.Errorf("supply expected_version from the first page to continue reading")
			}
			if req.ExpectedVersion != "" && req.ExpectedVersion != skill.VersionKey {
				return nil, fmt.Errorf("skill changed; restart the read")
			}
			text := []rune(skill.Instructions)
			if req.Offset > len(text) {
				return nil, fmt.Errorf("offset exceeds instruction length")
			}
			limit := req.Limit
			if limit == 0 {
				limit = 4000
			}
			limit = min(limit, 4000)
			end := min(req.Offset+limit, len(text))
			var next *int
			if end < len(text) {
				next = &end
			}
			return marshalSkillToolResult(struct {
				skillToolMetadata
				Offset       int    `json:"offset"`
				Total        int    `json:"total_characters"`
				Complete     bool   `json:"complete"`
				Next         *int   `json:"next_offset"`
				Instructions string `json:"instructions"`
			}{skillMetadata(skill), req.Offset, len(text), next == nil, next, string(text[req.Offset:end])})
		}
		return nil, ErrWorkspaceSkillNotFound
	}
	if req.Offset > len(catalog.Skills) {
		return nil, fmt.Errorf("offset exceeds catalog length")
	}
	limit := req.Limit
	if limit == 0 {
		limit = 10
	}
	limit = min(limit, 10)
	end := min(req.Offset+limit, len(catalog.Skills))
	var next *int
	if end < len(catalog.Skills) {
		next = &end
	}
	entries := make([]skillToolMetadata, 0, end-req.Offset)
	for _, skill := range catalog.Skills[req.Offset:end] {
		entry := skillMetadata(skill)
		entry.Description = truncateCommandBarText(entry.Description, 500)
		entries = append(entries, entry)
	}
	return marshalSkillToolResult(struct {
		Skills   []skillToolMetadata `json:"skills"`
		Total    int                 `json:"total"`
		Complete bool                `json:"complete"`
		Next     *int                `json:"next_offset"`
	}{entries, len(catalog.Skills), next == nil, next})
}

type skillUpdateInput struct {
	SkillID         string  `json:"skill_id"`
	ExpectedVersion *string `json:"expected_version"`
	Title           *string `json:"title,omitempty"`
	Description     *string `json:"description,omitempty"`
	Instructions    *string `json:"instructions,omitempty"`
	RestoreDefault  bool    `json:"restore_default,omitempty"`
}

func (s *InternalCommandService) validateSkillUpdate(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) error {
	if s.agentService == nil || meta.WorkspaceID == "" {
		return fmt.Errorf("workspace skill management is unavailable")
	}
	if strings.TrimSpace(meta.ActorID) == "" || strings.TrimSpace(meta.ActorRole) == "" {
		return fmt.Errorf("skill editing requires an authorized workspace member")
	}
	// Validation is read-only but checks the same permissions as saving.
	if err := s.authorizeCommandActor(meta, InternalCommandDefinition{Name: "skills.update", Module: "skills", Mutating: true}); err != nil {
		return err
	}
	var req skillUpdateInput
	if err := decodeSkillCommand(input, &req); err != nil {
		return err
	}
	if req.SkillID == "" || req.ExpectedVersion == nil || strings.TrimSpace(*req.ExpectedVersion) == "" {
		return fmt.Errorf("read list_workspace_skills and supply its skill_id and expected_version; never guess a version")
	}
	if req.RestoreDefault && (req.Title != nil || req.Description != nil || req.Instructions != nil) {
		return fmt.Errorf("restore_default cannot be combined with edited fields")
	}
	for _, value := range []*string{req.Title, req.Description, req.Instructions} {
		if value != nil && strings.TrimSpace(*value) == "" {
			return fmt.Errorf("edited fields must not be empty")
		}
	}
	catalog, err := s.agentService.ListSkillCatalog(ctx, meta.WorkspaceID)
	if err != nil {
		return err
	}
	for _, skill := range catalog.Skills {
		if skillMetadata(skill).SkillID != req.SkillID && !(skill.SourceKind == model.WorkspaceSkillSourceBuiltIn && strings.TrimPrefix(req.SkillID, agentRuntimeHelpinBuiltInSkillIDPrefix) == skill.Key) {
			continue
		}
		if *req.ExpectedVersion != skill.VersionKey {
			return fmt.Errorf("skill changed or expected_version is invalid; read list_workspace_skills again, never guess a version")
		}
		changed := req.RestoreDefault && skill.IsCustomized
		for _, field := range []struct {
			value   *string
			current string
		}{{req.Title, skill.Title}, {req.Description, skill.Description}, {req.Instructions, skill.Instructions}} {
			if field.value != nil && strings.TrimSpace(*field.value) != field.current {
				changed = true
			}
		}
		if !changed {
			return fmt.Errorf("no skill changes supplied; prepare the actual edit before requesting approval")
		}
		return nil
	}
	return ErrWorkspaceSkillNotFound
}
func (s *InternalCommandService) validateSkillUpdateCommand(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.validateSkillUpdate(ctx, meta, input); err != nil {
		return nil, err
	}
	return json.RawMessage(`{"valid":true,"saved":false,"next_step":"Call update_skill with these exact arguments. Validation does not authorize or save the edit."}`), nil
}
func (s *InternalCommandService) updateSkillCommand(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.validateSkillUpdate(ctx, meta, input); err != nil {
		return nil, err
	}
	var req skillUpdateInput
	if err := decodeSkillCommand(input, &req); err != nil {
		return nil, err
	}
	result, err := s.agentService.UpdateWorkspaceSkill(ctx, meta.WorkspaceID, req.SkillID, model.UpdateWorkspaceSkillRequest{ExpectedVersion: req.ExpectedVersion, Title: req.Title, Description: req.Description, Instructions: req.Instructions, RestoreDefault: req.RestoreDefault})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"saved": true, "id": result.ID, "skill_id": result.ID, "key": result.Key, "version_key": result.VersionKey})
}
