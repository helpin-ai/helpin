package agentcontract

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
		AllowedTargetTypes: []string{"task", "repository", "workspace"},
		ApprovalRequired:   false,
		RequiresRepo:       true,
	},
	{
		Name:               model.AgentPresetEpicPlanner,
		RuntimeKind:        "native_sdk",
		Description:        "Cross-module product planning and review with repository read access, versioned preview artifacts, and optional web research.",
		AllowedTools:       []string{"checkout_repository", "checkout_repositories", "read_file", "read_files", "read_file_range", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "list_available_skills", "search_available_skills", "read_skill", "run_command", "web_search_brave", "web_search_exa", "fetch_url", "crawl_url", "request_user_input", "request_approval", "request_review_checkpoint", "update_plan", "publish_prd_draft", "publish_task_plan", "publish_task_plan_doc", "ensure_epic_spec_doc", "ensure_task_plan_doc", "write_document_content", "approve_epic_spec", "create_task_batch", "publish_document_change_proposal", "publish_ai_section_candidate", "add_task_comment", "list_task_checklist", "list_epic_tasks", "list_workspace_teams", "list_team_workflows_with_stages", "create_task", "search_workspace", "update_task_delivery_target", "update_epic_delivery_target", "list_documents", "list_collections", "read_document", "get_document_blocks", "search_documents", "create_document", "list_deals", "list_contacts", "list_buyer_signals"},
		AllowedCommands:    []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "pwd", "python", "cargo", "rg"},
		AllowedTargetTypes: []string{"epic", "task", "crm_deal", "document", "workspace"},
		ApprovalRequired:   false,
		RequiresRepo:       false,
	},
	{
		Name:               model.AgentPresetReviewAgent,
		RuntimeKind:        "opencode",
		Description:        "Review-first validation agent that can discuss findings and apply agreed fixes in the same branch.",
		AllowedTools:       []string{"read_file", "read_files", "read_file_range", "write_file", "edit_file", "apply_patch", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "run_command", "request_user_input", "request_approval", "request_review_checkpoint", "add_task_comment", "list_task_checklist"},
		AllowedCommands:    []string{"go", "npm", "npx", "node", "make", "git", "ls", "cat", "grep", "find", "head", "tail", "wc", "diff", "echo", "pwd", "python", "cargo", "rg"},
		AllowedTargetTypes: []string{"task", "repository", "workspace"},
		ApprovalRequired:   false,
		RequiresRepo:       true,
	},
	{
		Name:               model.AgentPresetSupportAgent,
		RuntimeKind:        "native_sdk",
		Description:        "Live support conversations: grounded replies via server-validated send, escalation to humans, and read-only sub-agents for live context.",
		AllowedTools:       []string{"list_available_skills", "search_available_skills", "read_skill", "request_user_input", "request_approval", "request_review_checkpoint", "preview_md", "preview_json", "list_conversation_messages", "draft_support_reply", "update_conversation_status", "search_knowledge", "send_support_reply", "escalate_to_human", "list_support_conversations", "get_support_conversation", "list_support_tags", "list_support_inboxes", "list_support_assignees", "assign_support_conversation", "move_support_conversation", "add_support_conversation_tag", "remove_support_conversation_tag", "link_support_conversation_task", "link_support_conversation_contact", "update_support_conversation_subject", "list_tasks", "get_task_context", "update_plan", "list_agents", "start_agent_run", "start_agent_plan", "get_agent_run", "cancel_agent_run"},
		AllowedCommands:    []string{},
		AllowedTargetTypes: []string{"support_conversation"},
		ApprovalRequired:   false,
		RequiresRepo:       false,
	},
	{
		Name:               model.AgentPresetDocumentationAgent,
		RuntimeKind:        "native_sdk",
		Description:        "Documentation maintenance across internal docs, public help center articles, API docs, support gaps, and release-driven updates.",
		AllowedTools:       []string{"list_available_skills", "search_available_skills", "read_skill", "request_user_input", "request_approval", "request_review_checkpoint", "update_plan", "web_search_brave", "web_search_exa", "fetch_url", "crawl_url", "browser_open", "browser_snapshot", "browser_act", "browser_screenshot", "list_repositories", "checkout_repository", "checkout_repositories", "read_file", "read_files", "read_file_range", "list_directory", "search_files", "ripgrep", "grep", "list_symbols", "list_spaces", "search_workspace", "list_documents", "list_collections", "read_document", "get_document_blocks", "search_documents", "create_space", "create_collection", "create_document", "update_space", "update_collection", "move_document", "update_document_metadata", "write_document_content", "update_document_block", "insert_document_block", "insert_document_image", "link_document_to_object", "publish_document_change_proposal", "publish_ai_section_candidate", "complete_support_coverage_gap", "get_release_context", "find_tasks_for_git_changes", "get_task_context", "list_tasks", "add_task_comment", "list_conversation_messages", "list_workspace_teams"},
		AllowedCommands:    []string{},
		AllowedTargetTypes: []string{"workspace", "document", "support_conversation", "support_coverage_gap", "task", "epic", "repository"},
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
	case "planner", "orchestrator", "product_planner", model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner, model.AgentPresetCRMOperator, "marketing", "mira", model.AgentPresetMarketer:
		return model.AgentPresetEpicPlanner
	case "reviewer", "reviewer_tester", model.AgentPresetReviewAgent:
		return model.AgentPresetReviewAgent
	case "support", model.AgentPresetSupportAgent:
		return model.AgentPresetSupportAgent
	case "docs", "documentation", model.AgentPresetDocumentationAgent:
		return model.AgentPresetDocumentationAgent
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
