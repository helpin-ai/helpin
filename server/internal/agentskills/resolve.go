package agentskills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type WorkspaceSkillLookup interface {
	GetByID(ctx context.Context, workspaceID, id string) (*model.WorkspaceSkill, error)
	GetActiveByKey(ctx context.Context, workspaceID, key string) (*model.WorkspaceSkill, error)
}

type Resolution struct {
	Refs        model.AgentSkillRefs
	Definitions []agentcontract.SkillDefinition
}

func Resolve(ctx context.Context, workspaceID string, refs model.AgentSkillRefs, lookup WorkspaceSkillLookup) (Resolution, error) {
	normalized := refs.Normalize()
	if len(normalized) == 0 {
		return Resolution{Refs: model.AgentSkillRefs{}, Definitions: nil}, nil
	}

	canonical := make(model.AgentSkillRefs, 0, len(normalized))
	definitions := make([]agentcontract.SkillDefinition, 0, len(normalized))
	seen := make(map[string]struct{}, len(normalized))
	for _, ref := range normalized {
		resolvedRef, definition, identity, err := resolveOne(ctx, workspaceID, ref, lookup)
		if err != nil {
			return Resolution{}, err
		}
		if _, exists := seen[identity]; exists {
			return Resolution{}, fmt.Errorf("duplicate skill reference %q", identity)
		}
		seen[identity] = struct{}{}
		canonical = append(canonical, resolvedRef)
		definitions = append(definitions, definition)
	}

	return Resolution{Refs: canonical, Definitions: definitions}, nil
}

func CompileInstructions(definitions []agentcontract.SkillDefinition) string {
	sections := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		instructions := strings.TrimSpace(definition.Instructions)
		if instructions == "" {
			continue
		}
		sections = append(sections, instructions)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func AggregatePolicy(definitions []agentcontract.SkillDefinition) agentcontract.SkillPolicy {
	if len(definitions) == 0 {
		return agentcontract.SkillPolicy{}
	}

	var policy agentcontract.SkillPolicy
	var allowImplicit *bool
	requiredInteractionKinds := make([]string, 0, len(definitions))
	interactionContracts := make([]agentcontract.SkillInteractionContract, 0, len(definitions))

	for _, definition := range definitions {
		if definition.Policy.AllowImplicitInvocation != nil {
			value := *definition.Policy.AllowImplicitInvocation
			switch {
			case allowImplicit == nil:
				allowImplicit = &value
			case !value:
				allowImplicit = &value
			}
		}
		requiredInteractionKinds = append(requiredInteractionKinds, definition.Policy.CompletionRequiresInteractionKinds...)
		interactionContracts = append(interactionContracts, definition.Policy.InteractionContracts...)
	}

	policy.AllowImplicitInvocation = allowImplicit
	policy.CompletionRequiresInteractionKinds = agentcontract.SortedUniqueStrings(requiredInteractionKinds)
	policy.InteractionContracts = agentcontract.NormalizeInteractionContracts(interactionContracts)
	return policy
}

func ValidateRuntimeAndTools(runtimeKind string, allowedTools []string, definitions []agentcontract.SkillDefinition) error {
	runtimeKind = strings.TrimSpace(runtimeKind)
	allowedSet := make(map[string]struct{}, len(allowedTools))
	for _, toolName := range agentcontract.NormalizeToolNames(allowedTools) {
		allowedSet[toolName] = struct{}{}
	}
	for _, definition := range definitions {
		if len(definition.SupportedRuntimes) > 0 && runtimeKind != "" && !runtimeSupportedBySkill(definition.SupportedRuntimes, runtimeKind) {
			return fmt.Errorf("skill %q does not support runtime %q", definition.Key, runtimeKind)
		}
		for _, toolName := range agentcontract.NormalizeToolNames(definition.RequiredTools) {
			if _, ok := allowedSet[toolName]; !ok {
				return fmt.Errorf("skill %q requires tool %q", definition.Key, toolName)
			}
		}
	}
	return nil
}

func runtimeSupportedBySkill(supported []string, runtimeKind string) bool {
	if contains(supported, runtimeKind) {
		return true
	}
	// Migration compatibility: existing workspace skills were authored for the
	// in-process native runtime, but Codex now stages and uses the same skill
	// contract for default agent execution.
	return false
}

func resolveOne(ctx context.Context, workspaceID string, ref model.AgentSkillRef, lookup WorkspaceSkillLookup) (model.AgentSkillRef, agentcontract.SkillDefinition, string, error) {
	if ref.SkillID != nil {
		if lookup == nil {
			return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", fmt.Errorf("workspace skill lookup is not configured")
		}
		skill, err := lookup.GetByID(ctx, workspaceID, *ref.SkillID)
		if err != nil {
			return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", err
		}
		if skill == nil || skill.IsArchived {
			return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", fmt.Errorf("workspace skill not found")
		}
		if ref.VersionKey != nil && strings.TrimSpace(*ref.VersionKey) != "" && strings.TrimSpace(*ref.VersionKey) != strings.TrimSpace(skill.VersionKey) {
			return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", fmt.Errorf("workspace skill %q version mismatch", skill.Key)
		}
		definition, err := workspaceSkillDefinition(skill)
		if err != nil {
			return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", err
		}
		canonical := ref.Normalize()
		canonical.SkillID = stringPtr(skill.ID)
		canonical.Key = skill.Key
		canonical.VersionKey = stringPtr(skill.VersionKey)
		return canonical, definition, "workspace:" + skill.ID, nil
	}

	key := strings.TrimSpace(ref.Key)
	if key == "" {
		return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", fmt.Errorf("skill key is required")
	}
	// Built-in skills take precedence for key-only refs. Workspace skills that reuse a
	// built-in key must be referenced by skill_id to avoid ambiguous resolution.
	if definition, ok := agentcontract.GetBuiltInSkill(key); ok {
		canonical := ref.Normalize()
		canonical.Key = definition.Key
		return canonical, definition, "builtin:" + definition.Key, nil
	}
	if lookup == nil {
		return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", fmt.Errorf("unknown skill %q", key)
	}
	workspaceSkill, err := lookup.GetActiveByKey(ctx, workspaceID, key)
	if err != nil {
		return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", err
	}
	if workspaceSkill == nil || workspaceSkill.IsArchived {
		return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", fmt.Errorf("unknown skill %q", key)
	}
	if ref.VersionKey != nil && strings.TrimSpace(*ref.VersionKey) != "" && strings.TrimSpace(*ref.VersionKey) != strings.TrimSpace(workspaceSkill.VersionKey) {
		return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", fmt.Errorf("workspace skill %q version mismatch", workspaceSkill.Key)
	}
	definition, err := workspaceSkillDefinition(workspaceSkill)
	if err != nil {
		return model.AgentSkillRef{}, agentcontract.SkillDefinition{}, "", err
	}
	canonical := ref.Normalize()
	canonical.SkillID = stringPtr(workspaceSkill.ID)
	canonical.Key = workspaceSkill.Key
	canonical.VersionKey = stringPtr(workspaceSkill.VersionKey)
	return canonical, definition, "workspace:" + workspaceSkill.ID, nil
}

func workspaceSkillDefinition(skill *model.WorkspaceSkill) (agentcontract.SkillDefinition, error) {
	if skill == nil {
		return agentcontract.SkillDefinition{}, fmt.Errorf("workspace skill is required")
	}
	definition := agentcontract.SkillDefinition{
		Key:               strings.TrimSpace(skill.Key),
		Title:             strings.TrimSpace(skill.Title),
		Description:       strings.TrimSpace(stringOrDefault(skill.Description)),
		SourceKind:        strings.TrimSpace(skill.SourceKind),
		Instructions:      strings.TrimSpace(skill.Instructions),
		RequiredTools:     parseStringArray(skill.RequiredTools),
		SupportedRuntimes: parseStringArray(skill.SupportedRuntimes),
	}
	if err := decodeJSONBlob(skill.InterfaceConfig, &definition.Interface); err != nil {
		return agentcontract.SkillDefinition{}, fmt.Errorf("decode workspace skill interface config: %w", err)
	}
	if err := decodeJSONBlob(skill.PolicyConfig, &definition.Policy); err != nil {
		return agentcontract.SkillDefinition{}, fmt.Errorf("decode workspace skill policy config: %w", err)
	}
	return definition, nil
}

func decodeJSONBlob(raw model.JSONBlob, dest any) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func parseStringArray(raw model.JSONBlob) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func stringPtr(value string) *string {
	return &value
}

func stringOrDefault(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
