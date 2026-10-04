package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func flowBuilderContextSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"include_agent_options": map[string]any{"type": "boolean"},
		"repository_id":         map[string]any{"type": "string"},
	}}
}

func flowBuilderAgentOverrideSchema() map[string]any {
	text := map[string]any{"type": "string"}
	stringArray := map[string]any{"type": "array", "items": text}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"system_prompt":       text,
		"allowed_tools":       stringArray,
		"allowed_targets":     map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": flowBuilderAgentTargets()}},
		"skills":              map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"key": text, "skill_id": text, "version_key": text, "config": map[string]any{"type": "object"}}}},
		"approval_mode":       map[string]any{"type": "string", "enum": flowBuilderApprovalModes()},
		"max_concurrent_runs": map[string]any{"type": "integer", "minimum": 1},
	}}
}
func flowBuilderAgentTargets() []string {
	return []string{"task", "epic", "repository", "workspace", "crm_deal", "crm_contact", "crm_company", "document", "support_conversation", "support_coverage_gap"}
}
func flowBuilderApprovalModes() []string {
	return []string{"risk_based", "mutating_tools", "always", "never", "preset_default"}
}

// Keep the conversation's settings contract aligned with the former template
// editor. Other agent settings belong to agent management, not a flow draft.
func validateFlowAgentOverrides(d model.FlowBuilderDraft, createsAgent bool) error {
	if d.AgentOverrides == nil && d.AgentName == "" {
		return nil
	}
	if !createsAgent {
		return fmt.Errorf("this flow uses an existing agent; change its settings in agent management")
	}
	o := d.AgentOverrides
	if o == nil {
		return nil
	}
	encoded, err := json.Marshal(o)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return err
	}
	supported := flowBuilderAgentOverrideSchema()["properties"].(map[string]any)
	for key := range fields {
		if supported[key] == nil {
			return fmt.Errorf("unsupported template agent setting %q", key)
		}
	}
	if o.MaxConcurrentRuns != nil && *o.MaxConcurrentRuns < 1 {
		return fmt.Errorf("concurrent runs must be at least one")
	}
	if o.ApprovalMode != nil && !slices.Contains(flowBuilderApprovalModes(), *o.ApprovalMode) {
		return fmt.Errorf("choose a supported approval mode")
	}
	if len(o.AllowedTargets) > 0 {
		var targets []string
		if err := json.Unmarshal(o.AllowedTargets, &targets); err != nil || targets == nil {
			return fmt.Errorf("working areas must be a list")
		}
		for _, target := range targets {
			if !slices.Contains(flowBuilderAgentTargets(), target) {
				return fmt.Errorf("unsupported working area %q", target)
			}
		}
	}
	if len(o.AllowedTools) > 0 {
		var tools []string
		if err := json.Unmarshal(o.AllowedTools, &tools); err != nil || tools == nil {
			return fmt.Errorf("tools must be a list")
		}
	}
	return nil
}

func (s *DockChatService) validateFlowAgentTools(ctx context.Context, workspaceID string, draft model.FlowBuilderDraft) error {
	if draft.AgentOverrides == nil || len(draft.AgentOverrides.AllowedTools) == 0 {
		return nil
	}
	var selected []string
	if err := json.Unmarshal(draft.AgentOverrides.AllowedTools, &selected); err != nil {
		return err
	}
	catalog, err := s.agentService.ListToolCatalogForWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, name := range selected {
		if !slices.ContainsFunc(catalog.Tools, func(tool model.ToolCatalogEntry) bool { return tool.Name == name }) {
			return fmt.Errorf("choose an available tool instead of %q", name)
		}
	}
	return nil
}
