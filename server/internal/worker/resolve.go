package worker

import (
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ResolvedProfile is the single runtime boundary for agent execution.
// It merges preset/tool defaults with per-agent overrides so downstream
// code only deals with concrete policy.
type ResolvedProfile struct {
	Tools            []string
	Commands         []string
	TargetTypes      []string
	ApprovalMode     string // "never", "always", "preset_default"
	ApprovalRequired bool
	RequiresRepo     bool
	Queue            string
}

// ResolveAgentProfile merges preset/runtime defaults with per-agent overrides.
// Per-agent fields (AllowedTools, AllowedTargets, etc.) take precedence when
// non-empty; otherwise the preset-aligned profile provides the defaults.
func ResolveAgentProfile(agent *model.Agent, invocationMode ...string) ResolvedProfile {
	if agent == nil {
		defaultProfile := GetRuntimeProfile("")
		return ResolvedProfile{
			Tools:            NormalizeToolNames(defaultProfile.AllowedTools),
			Commands:         defaultProfile.AllowedCommands,
			TargetTypes:      defaultProfile.AllowedTargetTypes,
			ApprovalMode:     "never",
			ApprovalRequired: defaultProfile.ApprovalRequired,
			RequiresRepo:     defaultProfile.RequiresRepo,
			Queue:            QueueForRuntime(defaultProfile.RuntimeKind, model.InvocationModeAutonomous),
		}
	}
	mode := model.InvocationModeAutonomous
	if len(invocationMode) > 0 {
		mode = invocationMode[0]
	} else if agent != nil && agent.DefaultInvocationMode != "" {
		mode = agent.DefaultInvocationMode
	}
	defaultProfile := GetRuntimeProfile(defaultProfileNameForPreset(agent.EffectivePresetKey(), agent.IsSystem))

	resolved := ResolvedProfile{
		Tools:            NormalizeToolNames(defaultProfile.AllowedTools),
		Commands:         defaultProfile.AllowedCommands,
		TargetTypes:      defaultProfile.AllowedTargetTypes,
		ApprovalMode:     "never",
		ApprovalRequired: defaultProfile.ApprovalRequired,
		RequiresRepo:     defaultProfile.RequiresRepo,
		Queue:            QueueForRuntime(agent.RuntimeKind, mode),
	}

	// Per-agent tool overrides
	if tools := parseJSONStringSlice(agent.AllowedTools); len(tools) > 0 {
		resolved.Tools = NormalizeToolNames(tools)
	}
	if commands := parseJSONStringSlice(agent.AllowedCommands); len(commands) > 0 {
		resolved.Commands = commands
	}
	if targets := parseJSONStringSlice(agent.AllowedTargets); len(targets) > 0 {
		resolved.TargetTypes = targets
	}

	// Approval mode override
	if agent.ApprovalMode != "" && agent.ApprovalMode != "preset_default" {
		resolved.ApprovalMode = agent.ApprovalMode
	}

	if strings.TrimSpace(agent.RuntimeKind) == "native_sdk" && agentHasAvailableSkills(agent) {
		resolved.Tools = appendMissingTools(resolved.Tools, ToolListAvailableSkills, ToolSearchAvailableSkills, ToolReadSkill)
	}

	// Repo: required if agent has any filesystem or git tools
	resolved.RequiresRepo = hasRepoTools(resolved.Tools)

	return resolved
}

// ResolveApprovalState determines the initial approval state for a new run.
func ResolveApprovalState(resolved ResolvedProfile) string {
	switch resolved.ApprovalMode {
	case "never":
		return "not_required"
	case "always":
		return "pending"
	default:
		if resolved.ApprovalRequired {
			return "pending"
		}
		return "not_required"
	}
}

func defaultProfileNameForPreset(presetKey string, isSystem bool) string {
	switch strings.TrimSpace(presetKey) {
	case model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner, model.AgentPresetCRMOperator, model.AgentPresetMarketer, model.AgentPresetCommandAgent:
		return model.AgentPresetEpicPlanner
	case model.AgentPresetSupportAgent:
		return model.AgentPresetSupportAgent
	case model.AgentPresetDocumentationAgent:
		return model.AgentPresetDocumentationAgent
	case model.AgentPresetReviewAgent:
		return model.AgentPresetReviewAgent
	case model.AgentPresetCodeBuilder:
		return model.AgentPresetCodeBuilder
	default:
		if isSystem {
			return model.AgentPresetEpicPlanner
		}
		return model.AgentPresetCodeBuilder
	}
}

// QueueForRuntime maps runtime and invocation mode to a shared Temporal queue.
func QueueForRuntime(runtimeKind, invocationMode string) string {
	switch runtimeKind {
	case "native_sdk":
		if invocationMode == model.InvocationModeInteractive {
			return "agent-native-interactive"
		}
		return "agent-native-autonomous"
	case "opencode":
		return "agent-opencode-autonomous"
	case "codex":
		if invocationMode == model.InvocationModeInteractive {
			return "agent-codex-interactive"
		}
		return "agent-codex-autonomous"
	default:
		return "automation-default"
	}
}

// hasRepoTools returns true if any tool in the set requires repository access.
func hasRepoTools(tools []string) bool {
	repoTools := map[string]bool{
		"read_file": true, "read_file_range": true, "write_file": true,
		"list_directory": true, "search_files": true, "ripgrep": true,
		"grep": true, "list_symbols": true, "run_command": true,
		"create_branch": true, "commit_and_push": true, "open_pr": true,
		ToolScanSemgrep: true, ToolScanTrivy: true, ToolScanGitleaks: true,
	}
	for _, t := range tools {
		if repoTools[t] {
			return true
		}
	}
	return false
}

func agentHasAvailableSkills(agent *model.Agent) bool {
	if agent == nil {
		return false
	}
	if len(agent.Skills.Normalize()) > 0 {
		return true
	}
	bundle, ok := BuiltInPresetSkillBundleForPreset(strings.TrimSpace(agent.EffectivePresetKey()))
	return ok && len(bundle.SkillKeys) > 0
}

func appendMissingTools(tools []string, required ...string) []string {
	out := append([]string(nil), tools...)
	seen := make(map[string]struct{}, len(out)+len(required))
	for _, toolName := range out {
		seen[strings.TrimSpace(toolName)] = struct{}{}
	}
	for _, toolName := range required {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		if _, ok := seen[toolName]; ok {
			continue
		}
		seen[toolName] = struct{}{}
		out = append(out, toolName)
	}
	return out
}

// parseJSONStringSlice safely parses a json.RawMessage into []string.
// Returns nil if the message is nil, empty, or not a valid JSON array.
func parseJSONStringSlice(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return nil
	}
	var result []string
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return result
}
