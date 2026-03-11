package worker

import (
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var runtimeProfiles = []model.RuntimeProfile{
	{
		Name:               model.AgentClassEngineer,
		RuntimeKind:        "opencode",
		Description:        "Story-only code implementation with repository, git, and validation tools.",
		AllowedTools:       []string{"read_file", "read_file_range", "write_file", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "run_command", "create_branch", "commit_and_push", "open_pr", "add_story_comment", "update_story_state", "list_story_checklist"},
		AllowedCommands:    []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "mkdir", "cp", "mv", "pwd", "python", "pip", "cargo", "rustc", "rg"},
		AllowedTargetTypes: []string{"story"},
		ApprovalRequired:   false,
		RequiresRepo:       true,
	},
	{
		Name:               model.AgentClassProductPlanner,
		RuntimeKind:        "opencode",
		Description:        "Epic-only product spec and story planning with repository-aware read access, optional web research, and no mutation tools.",
		AllowedTools:       []string{"read_file", "read_file_range", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "run_command", "web_search", "add_story_comment", "list_story_checklist"},
		AllowedCommands:    []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "pwd", "python", "cargo", "rg"},
		AllowedTargetTypes: []string{"epic"},
		ApprovalRequired:   true,
		RequiresRepo:       false,
	},
	{
		Name:               model.AgentClassReviewer,
		RuntimeKind:        "opencode",
		Description:        "Story-only validation and test execution with no repository mutation tools.",
		AllowedTools:       []string{"read_file", "read_file_range", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "run_command", "add_story_comment", "list_story_checklist"},
		AllowedCommands:    []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "pwd", "python", "cargo", "rg"},
		AllowedTargetTypes: []string{"story"},
		ApprovalRequired:   false,
		RequiresRepo:       true,
	},
	{
		Name:               model.AgentClassSupport,
		RuntimeKind:        "opencode",
		Description:        "Support conversation triage and draft replies with human approval before customer-visible sends.",
		AllowedTools:       []string{"list_conversation_messages", "draft_support_reply", "update_conversation_status"},
		AllowedCommands:    []string{},
		AllowedTargetTypes: []string{"support_conversation"},
		ApprovalRequired:   true,
		RequiresRepo:       false,
	},
	{
		Name:               model.AgentClassHuman,
		RuntimeKind:        "opencode",
		Description:        "Non-executable placeholder used for explicit assignment and handoffs to human participants.",
		AllowedTools:       []string{},
		AllowedCommands:    []string{},
		AllowedTargetTypes: []string{},
		ApprovalRequired:   false,
		RequiresRepo:       false,
	},
}

// ListRuntimeProfiles returns the supported runtime profiles.
func ListRuntimeProfiles() []model.RuntimeProfile {
	out := make([]model.RuntimeProfile, len(runtimeProfiles))
	copy(out, runtimeProfiles)
	return out
}

// GetRuntimeProfile returns the named runtime profile, defaulting to engineer.
func GetRuntimeProfile(name string) model.RuntimeProfile {
	name = NormalizeCapabilityProfile(name)
	for _, profile := range runtimeProfiles {
		if profile.Name == name {
			return profile
		}
	}
	return runtimeProfiles[0]
}

func NormalizeCapabilityProfile(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "engineer":
		return model.AgentClassEngineer
	case "planner", "orchestrator", model.AgentClassProductPlanner:
		return model.AgentClassProductPlanner
	case "reviewer", "reviewer_tester":
		return model.AgentClassReviewer
	case "support":
		return model.AgentClassSupport
	case "human", "human_proxy":
		return model.AgentClassHuman
	default:
		return strings.TrimSpace(name)
	}
}

func allowedToolSet(profile model.RuntimeProfile) map[string]bool {
	set := make(map[string]bool, len(profile.AllowedTools))
	for _, toolName := range profile.AllowedTools {
		set[toolName] = true
	}
	return set
}

func allowedCommandsFor(profile model.RuntimeProfile, config *WorkflowConfig) []string {
	if config == nil || len(config.AllowedCommands) == 0 {
		return slices.Clone(profile.AllowedCommands)
	}

	allowed := make([]string, 0, len(config.AllowedCommands))
	for _, command := range config.AllowedCommands {
		if slices.Contains(profile.AllowedCommands, command) {
			allowed = append(allowed, command)
		}
	}
	return allowed
}
