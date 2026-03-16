package worker

import (
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ResolvedProfile is the single runtime boundary for agent execution.
// It merges class defaults with per-agent overrides so that downstream
// code (tool resolution, target validation, queue selection, approval)
// never needs to inspect agent_class directly.
type ResolvedProfile struct {
	OriginalClass   string
	Tools           []string
	Commands        []string
	TargetTypes     []string
	ApprovalMode    string // "never", "always", "class_default"
	RequiresRepo    bool
	Queue           string
}

// ResolveAgentProfile merges class-level defaults with per-agent overrides.
// Per-agent fields (AllowedTools, AllowedTargets, etc.) take precedence when
// non-empty; otherwise the class profile provides the defaults.
func ResolveAgentProfile(agent *model.Agent) ResolvedProfile {
	classProfile := GetRuntimeProfile(agent.CapabilityProfile)

	resolved := ResolvedProfile{
		OriginalClass: agent.AgentClass,
		Tools:         classProfile.AllowedTools,
		Commands:      classProfile.AllowedCommands,
		TargetTypes:   classProfile.AllowedTargetTypes,
		ApprovalMode:  "class_default",
		RequiresRepo:  classProfile.RequiresRepo,
		Queue:         QueueForClass(agent.AgentClass),
	}

	// Per-agent tool overrides
	if tools := parseJSONStringSlice(agent.AllowedTools); len(tools) > 0 {
		resolved.Tools = tools
	}
	if commands := parseJSONStringSlice(agent.AllowedCommands); len(commands) > 0 {
		resolved.Commands = commands
	}
	if targets := parseJSONStringSlice(agent.AllowedTargets); len(targets) > 0 {
		resolved.TargetTypes = targets
	}

	// Approval mode override
	if agent.ApprovalMode != "" && agent.ApprovalMode != "class_default" {
		resolved.ApprovalMode = agent.ApprovalMode
	}

	// Queue: cross-module agents use the general queue
	if hasCrossModuleTools(resolved.Tools) {
		resolved.Queue = "automation-default"
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
	case "class_default":
		classProfile := GetRuntimeProfile(resolved.OriginalClass)
		if classProfile.ApprovalRequired {
			return "pending"
		}
		return "not_required"
	default:
		return "not_required"
	}
}

// QueueForClass maps an agent class to its Temporal task queue.
// Imported by temporalapp/queues.go — kept here to co-locate with resolution.
func QueueForClass(class string) string {
	switch NormalizeCapabilityProfile(class) {
	case model.AgentClassEngineer:
		return "agent-engineer"
	case model.AgentClassProductPlanner:
		return "agent-planner"
	case model.AgentClassReviewer:
		return "agent-reviewer"
	case model.AgentClassSupport:
		return "agent-support"
	default:
		return "automation-default"
	}
}

// hasCrossModuleTools returns true if the tool set spans more than one module
// (e.g. code tools + support tools + CRM tools).
func hasCrossModuleTools(tools []string) bool {
	modules := 0
	hasCode := false
	hasSupport := false
	hasCRM := false
	hasDocs := false

	for _, t := range tools {
		switch t {
		case "read_file", "write_file", "list_directory", "search_files", "ripgrep",
			"grep", "list_symbols", "run_command", "create_branch", "commit_and_push", "open_pr":
			if !hasCode {
				hasCode = true
				modules++
			}
		case "list_conversation_messages", "draft_support_reply", "update_conversation_status":
			if !hasSupport {
				hasSupport = true
				modules++
			}
		case "list_deals", "update_deal_stage", "add_deal_note", "list_contacts",
			"create_contact", "list_buyer_signals":
			if !hasCRM {
				hasCRM = true
				modules++
			}
		case "list_documents", "read_document", "create_document", "update_document",
			"search_documents":
			if !hasDocs {
				hasDocs = true
				modules++
			}
		}
	}

	return modules > 1
}

// hasRepoTools returns true if any tool in the set requires repository access.
func hasRepoTools(tools []string) bool {
	repoTools := map[string]bool{
		"read_file": true, "read_file_range": true, "write_file": true,
		"list_directory": true, "search_files": true, "ripgrep": true,
		"grep": true, "list_symbols": true, "run_command": true,
		"create_branch": true, "commit_and_push": true, "open_pr": true,
	}
	for _, t := range tools {
		if repoTools[t] {
			return true
		}
	}
	return false
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
