package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ToolListAvailableSkills   = "list_available_skills"
	ToolSearchAvailableSkills = "search_available_skills"
	ToolReadSkill             = "read_skill"
)

type availableSkillToolInput struct {
	Key     string `json:"key,omitempty"`
	SkillID string `json:"skill_id,omitempty"`
}

type searchAvailableSkillsToolInput struct {
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type availableSkillToolEntry struct {
	ID                *string  `json:"id,omitempty"`
	Key               string   `json:"key"`
	VersionKey        string   `json:"version_key,omitempty"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	SourceKind        string   `json:"source_kind"`
	RequiredTools     []string `json:"required_tools,omitempty"`
	SupportedRuntimes []string `json:"supported_runtimes,omitempty"`
	Instructions      string   `json:"instructions,omitempty"`
}

type availableSkillsToolResponse struct {
	Total  int                       `json:"total"`
	Skills []availableSkillToolEntry `json:"skills"`
}

type searchAvailableSkillsToolResponse struct {
	Query  string                    `json:"query"`
	Total  int                       `json:"total"`
	Skills []availableSkillToolEntry `json:"skills"`
}

func toolListAvailableSkills(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	entries := availableRuntimeSkillEntries(ctx, false)
	return marshalToolJSON(availableSkillsToolResponse{
		Total:  len(entries),
		Skills: entries,
	})
}

func toolSearchAvailableSkills(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var req searchAvailableSkillsToolInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return "", fmt.Errorf("parse search_available_skills input: %w", err)
		}
	}

	query := strings.TrimSpace(req.Query)
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	all := availableRuntimeSkillEntries(ctx, false)
	matches := make([]availableSkillToolEntry, 0, minInt(len(all), limit))
	for _, entry := range all {
		if query == "" || availableSkillMatches(entry, query) {
			matches = append(matches, entry)
			if len(matches) >= limit {
				break
			}
		}
	}

	return marshalToolJSON(searchAvailableSkillsToolResponse{
		Query:  query,
		Total:  len(matches),
		Skills: matches,
	})
}

func toolReadSkill(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var req availableSkillToolInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return "", fmt.Errorf("parse read_skill input: %w", err)
		}
	}
	key := strings.TrimSpace(req.Key)
	skillID := strings.TrimSpace(req.SkillID)
	if key == "" && skillID == "" {
		return "", fmt.Errorf("read_skill requires key or skill_id")
	}

	var matches []availableSkillToolEntry
	for _, entry := range availableRuntimeSkillEntries(ctx, true) {
		if skillID != "" {
			if entry.ID == nil || strings.TrimSpace(*entry.ID) != skillID {
				continue
			}
			return marshalToolJSON(entry)
		}
		if strings.TrimSpace(entry.Key) == key {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 1 {
		return marshalToolJSON(matches[0])
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple available skills use key %q; call read_skill with skill_id", key)
	}
	if skillID != "" {
		return "", fmt.Errorf("skill %q is not available to this agent", skillID)
	}
	return "", fmt.Errorf("skill %q is not available to this agent", key)
}

func availableRuntimeSkillEntries(ctx *ExecutionContext, includeInstructions bool) []availableSkillToolEntry {
	if ctx == nil {
		return nil
	}

	refs := ctx.RuntimeSkillRefs.Normalize()
	if len(refs) == 0 && ctx.Agent != nil {
		refs = ctx.Agent.Skills.Normalize()
	}

	definitions := ctx.RuntimeSkillDefinitions
	entries := make([]availableSkillToolEntry, 0, maxInt(len(refs), len(definitions)))
	seen := make(map[string]struct{}, maxInt(len(refs), len(definitions)))

	if len(definitions) > 0 {
		for idx, definition := range definitions {
			var ref model.AgentSkillRef
			if idx < len(refs) {
				ref = refs[idx]
			}
			entry := availableSkillEntryFromDefinition(ref, definition, includeInstructions)
			appendAvailableSkillEntry(&entries, seen, entry)
		}
		return entries
	}

	for _, ref := range refs {
		if definition, ok := GetBuiltInSkill(ref.Key); ok {
			entry := availableSkillEntryFromDefinition(ref, definition, includeInstructions)
			appendAvailableSkillEntry(&entries, seen, entry)
			continue
		}
		entry := availableSkillToolEntry{
			ID:         cloneStringPtr(ref.SkillID),
			Key:        strings.TrimSpace(ref.Key),
			SourceKind: "workspace",
		}
		if ref.VersionKey != nil {
			entry.VersionKey = strings.TrimSpace(*ref.VersionKey)
		}
		appendAvailableSkillEntry(&entries, seen, entry)
	}
	return entries
}

func availableSkillEntryFromDefinition(ref model.AgentSkillRef, definition SkillDefinition, includeInstructions bool) availableSkillToolEntry {
	entry := availableSkillToolEntry{
		ID:                cloneStringPtr(ref.SkillID),
		Key:               strings.TrimSpace(definition.Key),
		Title:             strings.TrimSpace(definition.Title),
		Description:       strings.TrimSpace(definition.Description),
		SourceKind:        strings.TrimSpace(definition.SourceKind),
		RequiredTools:     append([]string(nil), definition.RequiredTools...),
		SupportedRuntimes: append([]string(nil), definition.SupportedRuntimes...),
	}
	if entry.Key == "" {
		entry.Key = strings.TrimSpace(ref.Key)
	}
	if entry.SourceKind == "" {
		if ref.SkillID != nil && strings.TrimSpace(*ref.SkillID) != "" {
			entry.SourceKind = "workspace"
		} else {
			entry.SourceKind = "built_in"
		}
	}
	if ref.VersionKey != nil {
		entry.VersionKey = strings.TrimSpace(*ref.VersionKey)
	}
	if includeInstructions {
		entry.Instructions = strings.TrimSpace(definition.Instructions)
	}
	return entry
}

func appendAvailableSkillEntry(entries *[]availableSkillToolEntry, seen map[string]struct{}, entry availableSkillToolEntry) {
	if strings.TrimSpace(entry.Key) == "" && (entry.ID == nil || strings.TrimSpace(*entry.ID) == "") {
		return
	}
	identity := strings.TrimSpace(entry.SourceKind) + ":" + strings.TrimSpace(entry.Key)
	if entry.ID != nil && strings.TrimSpace(*entry.ID) != "" {
		identity = "id:" + strings.TrimSpace(*entry.ID)
	}
	if _, ok := seen[identity]; ok {
		return
	}
	seen[identity] = struct{}{}
	*entries = append(*entries, entry)
}

func availableSkillMatches(entry availableSkillToolEntry, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		entry.Key,
		entry.Title,
		entry.Description,
		entry.SourceKind,
		strings.Join(entry.RequiredTools, " "),
		strings.Join(entry.SupportedRuntimes, " "),
	}, " "))
	return strings.Contains(haystack, needle)
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
