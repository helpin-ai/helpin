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
	s.register(InternalCommandDefinition{Name: "skills.update", Module: "skills", Mutating: true, Tool: mustCommandToolMetadata("skills.update"), Validate: s.validatePreparedSkillUpdate, Execute: s.updateSkillCommand})
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
	ContractVersion   string   `json:"contract_version"`
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
	return skillToolMetadata{ContractVersion: "2", SkillID: id, Key: skill.Key, VersionKey: skill.VersionKey, Title: skill.Title, Description: skill.Description, SourceKind: skill.SourceKind, IsCustomized: skill.IsCustomized, ID: skill.ID, SourceRuntime: skill.SourceRuntime, RequiredTools: skill.RequiredTools, SupportedRuntimes: skill.SupportedRuntimes, Presets: skill.Presets}
}
func (s *InternalCommandService) listWorkspaceSkillsCommand(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.agentService == nil || meta.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace skill management is unavailable")
	}
	var req skillReadInput
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
			section, sections, err := skillInstructionSection(skill.Instructions, req.SectionHeading)
			if err != nil {
				return nil, err
			}
			text := []rune(section)
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
			var nextRead *skillReadInput
			if next != nil {
				nextRead = &skillReadInput{SkillID: skillMetadata(skill).SkillID, Offset: *next, Limit: limit, ExpectedVersion: skill.VersionKey, SectionHeading: req.SectionHeading}
			}
			return marshalSkillToolResult(struct {
				skillToolMetadata
				Offset         int             `json:"offset"`
				Total          int             `json:"total_characters"`
				Complete       bool            `json:"complete"`
				Next           *int            `json:"next_offset"`
				NextRead       *skillReadInput `json:"next_read"`
				Sections       []string        `json:"sections,omitempty"`
				SectionHeading string          `json:"section_heading,omitempty"`
				Instructions   string          `json:"instructions"`
			}{skillMetadata(skill), req.Offset, len(text), next == nil, next, nextRead, sections, req.SectionHeading, string(text[req.Offset:end])})
		}
		return nil, ErrWorkspaceSkillNotFound
	}
	if req.SectionHeading != "" {
		return nil, fmt.Errorf("section_heading requires key or skill_id")
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
	var nextRead *skillReadInput
	if next != nil {
		nextRead = &skillReadInput{Offset: *next, Limit: limit}
	}
	return marshalSkillToolResult(struct {
		ContractVersion string              `json:"contract_version"`
		Skills          []skillToolMetadata `json:"skills"`
		Total           int                 `json:"total"`
		Complete        bool                `json:"complete"`
		Next            *int                `json:"next_offset"`
		NextRead        *skillReadInput     `json:"next_read"`
	}{"2", entries, len(catalog.Skills), next == nil, next, nextRead})
}

type skillUpdateInput struct {
	SkillID         string                 `json:"skill_id"`
	ExpectedVersion *string                `json:"expected_version"`
	Title           *string                `json:"title,omitempty"`
	Description     *string                `json:"description,omitempty"`
	Instructions    *string                `json:"instructions,omitempty"`
	RestoreDefault  bool                   `json:"restore_default,omitempty"`
	Replacements    []skillTextReplacement `json:"replacements,omitempty"`
	reviewTitle     string
}

func (s *InternalCommandService) resolveSkillUpdate(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (*skillUpdateInput, error) {
	if s.agentService == nil || meta.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace skill management is unavailable")
	}
	if strings.TrimSpace(meta.ActorID) == "" || strings.TrimSpace(meta.ActorRole) == "" {
		return nil, fmt.Errorf("skill editing requires an authorized workspace member")
	}
	// Validation is read-only but checks the same permissions as saving.
	if err := s.authorizeCommandActor(meta, InternalCommandDefinition{Name: "skills.update", Module: "skills", Mutating: true}); err != nil {
		return nil, err
	}
	var req skillUpdateInput
	if err := decodeSkillCommand(input, &req); err != nil {
		return nil, err
	}
	if req.SkillID == "" || req.ExpectedVersion == nil || strings.TrimSpace(*req.ExpectedVersion) == "" {
		return nil, fmt.Errorf("read list_workspace_skills and supply its skill_id and expected_version; never guess a version")
	}
	if req.RestoreDefault && (req.Title != nil || req.Description != nil || req.Instructions != nil || len(req.Replacements) > 0) {
		return nil, fmt.Errorf("restore_default cannot be combined with edited fields")
	}
	for _, value := range []*string{req.Title, req.Description, req.Instructions} {
		if value != nil && strings.TrimSpace(*value) == "" {
			return nil, fmt.Errorf("edited fields must not be empty")
		}
	}
	catalog, err := s.agentService.ListSkillCatalog(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	for _, skill := range catalog.Skills {
		if skillMetadata(skill).SkillID != req.SkillID && !(skill.SourceKind == model.WorkspaceSkillSourceBuiltIn && strings.TrimPrefix(req.SkillID, agentRuntimeHelpinBuiltInSkillIDPrefix) == skill.Key) {
			continue
		}
		if *req.ExpectedVersion != skill.VersionKey {
			return nil, fmt.Errorf("skill changed or expected_version is invalid; read list_workspace_skills again, never guess a version")
		}

		if len(req.Replacements) > 0 {
			if req.Instructions != nil {
				return nil, fmt.Errorf("use replacements or full instructions, not both")
			}
			updated, err := applySkillReplacements(skill.Instructions, req.Replacements)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(updated) == "" {
				return nil, fmt.Errorf("instructions must not be empty")
			}
			req.Instructions = &updated
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
			return nil, fmt.Errorf("no skill changes supplied; prepare the actual edit before requesting approval")
		}
		req.reviewTitle = skill.Title
		return &req, nil
	}
	return nil, ErrWorkspaceSkillNotFound
}
