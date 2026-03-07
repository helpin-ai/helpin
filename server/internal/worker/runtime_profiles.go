package worker

import (
	"slices"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var runtimeProfiles = []model.RuntimeProfile{
	{
		Name:             "engineer",
		RuntimeKind:      "native_claude",
		Description:      "Code implementation with repository, git, and validation tools.",
		AllowedTools:     []string{"read_file", "write_file", "list_directory", "search_files", "run_command", "create_branch", "commit_and_push", "open_pr", "add_story_comment", "update_story_state", "list_story_checklist"},
		AllowedCommands:  []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "mkdir", "cp", "mv", "pwd", "python", "pip", "cargo", "rustc"},
		ApprovalRequired: false,
		RequiresRepo:     true,
	},
	{
		Name:             "planner",
		RuntimeKind:      "native_claude",
		Description:      "Read-heavy planning and PRD generation with repository context but no mutation tools.",
		AllowedTools:     []string{"read_file", "list_directory", "search_files", "run_command", "add_story_comment", "list_story_checklist"},
		AllowedCommands:  []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "pwd", "python", "cargo"},
		ApprovalRequired: false,
		RequiresRepo:     false,
	},
	{
		Name:             "reviewer_tester",
		RuntimeKind:      "native_claude",
		Description:      "Read-heavy validation and test execution with no repository mutation tools.",
		AllowedTools:     []string{"read_file", "list_directory", "search_files", "run_command", "add_story_comment", "list_story_checklist"},
		AllowedCommands:  []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "pwd", "python", "cargo"},
		ApprovalRequired: false,
		RequiresRepo:     true,
	},
	{
		Name:             "support",
		RuntimeKind:      "native_claude",
		Description:      "Ticket triage and draft replies with human approval before customer-visible sends.",
		AllowedTools:     []string{"list_ticket_messages", "draft_support_reply", "update_ticket_status"},
		AllowedCommands:  []string{},
		ApprovalRequired: true,
		RequiresRepo:     false,
	},
	{
		Name:             "orchestrator",
		RuntimeKind:      "native_claude",
		Description:      "Epic planning and decomposition that produces reviewable orchestration proposals.",
		AllowedTools:     []string{},
		AllowedCommands:  []string{},
		ApprovalRequired: true,
		RequiresRepo:     false,
	},
	{
		Name:             "human_proxy",
		RuntimeKind:      "native_claude",
		Description:      "Non-executable placeholder used for explicit handoffs to human participants.",
		AllowedTools:     []string{},
		AllowedCommands:  []string{},
		ApprovalRequired: false,
		RequiresRepo:     false,
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
	for _, profile := range runtimeProfiles {
		if profile.Name == name {
			return profile
		}
	}
	return runtimeProfiles[0]
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
