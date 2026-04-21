package worker

import (
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var runtimeProfiles = []model.RuntimeProfile{
	{
		Name:               model.AgentPresetCodeBuilder,
		RuntimeKind:        "opencode",
		Description:        "Task-only code implementation with repository, git, and validation tools.",
		AllowedTools:       []string{"read_file", "read_files", "read_file_range", "write_file", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "run_command", "create_branch", "commit_and_push", "open_pr", "add_task_comment", "update_task_state", "list_task_checklist"},
		AllowedCommands:    []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "mkdir", "cp", "mv", "pwd", "python", "pip", "cargo", "rustc", "rg"},
		AllowedTargetTypes: []string{"task", "repository"},
		ApprovalRequired:   false,
		RequiresRepo:       true,
	},
	{
		Name:               model.AgentPresetEpicPlanner,
		RuntimeKind:        "native_sdk",
		Description:        "Cross-module product planning and review with repository read access, versioned preview artifacts, and optional web research.",
		AllowedTools:       []string{"read_file", "read_files", "read_file_range", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "run_command", "web_search_brave", "web_search_exa", "request_user_input", "request_review_checkpoint", "update_plan", "publish_prd_draft", "publish_task_plan", "publish_task_plan_doc", "add_task_comment", "list_task_checklist", "list_epic_tasks", "list_workspace_teams", "list_documents", "read_document", "search_documents", "list_deals", "list_contacts", "list_buyer_signals"},
		AllowedCommands:    []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "pwd", "python", "cargo", "rg"},
		AllowedTargetTypes: []string{"epic", "task", "crm_deal"},
		ApprovalRequired:   false,
		RequiresRepo:       false,
	},
	{
		Name:               model.AgentPresetReviewAgent,
		RuntimeKind:        "opencode",
		Description:        "Review-first validation agent that can discuss findings and apply agreed fixes in the same branch.",
		AllowedTools:       []string{"read_file", "read_files", "read_file_range", "write_file", "edit_file", "apply_patch", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "run_command", "request_user_input", "request_review_checkpoint", "add_task_comment", "list_task_checklist"},
		AllowedCommands:    []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "pwd", "python", "cargo", "rg"},
		AllowedTargetTypes: []string{"task", "repository"},
		ApprovalRequired:   false,
		RequiresRepo:       true,
	},
	{
		Name:               model.AgentPresetSupportAgent,
		RuntimeKind:        "native_sdk",
		Description:        "Support conversation triage and draft replies with human approval before customer-visible sends.",
		AllowedTools:       []string{"request_user_input", "request_review_checkpoint", "preview_md", "preview_json", "list_conversation_messages", "draft_support_reply", "update_conversation_status"},
		AllowedCommands:    []string{},
		AllowedTargetTypes: []string{"support_conversation"},
		ApprovalRequired:   true,
		RequiresRepo:       false,
	},
}

// ListRuntimeProfiles returns the supported runtime profiles.
func ListRuntimeProfiles() []model.RuntimeProfile {
	out := make([]model.RuntimeProfile, len(runtimeProfiles))
	copy(out, runtimeProfiles)
	return out
}

// GetRuntimeProfile returns the named runtime profile, defaulting to code_builder.
func GetRuntimeProfile(name string) model.RuntimeProfile {
	name = normalizeRuntimeProfileName(name)
	for _, profile := range runtimeProfiles {
		if profile.Name == name {
			return profile
		}
	}
	return runtimeProfiles[0]
}

func normalizeRuntimeProfileName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "engineer", "coder", model.AgentPresetCodeBuilder:
		return model.AgentPresetCodeBuilder
	case "planner", "orchestrator", "product_planner", model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner, model.AgentPresetCRMOperator:
		return model.AgentPresetEpicPlanner
	case "reviewer", "reviewer_tester", model.AgentPresetReviewAgent:
		return model.AgentPresetReviewAgent
	case "support", model.AgentPresetSupportAgent:
		return model.AgentPresetSupportAgent
	default:
		return strings.TrimSpace(name)
	}
}

func allowedToolSet(resolved ResolvedProfile) map[string]bool {
	set := make(map[string]bool, len(resolved.Tools))
	for _, toolName := range resolved.Tools {
		set[toolName] = true
	}
	return set
}

func allowedCommandsFor(resolved ResolvedProfile, config *WorkflowConfig) []string {
	if config == nil || len(config.AllowedCommands) == 0 {
		return slices.Clone(resolved.Commands)
	}

	allowed := make([]string, 0, len(config.AllowedCommands))
	for _, command := range config.AllowedCommands {
		if slices.Contains(resolved.Commands, command) {
			allowed = append(allowed, command)
		}
	}
	return allowed
}
