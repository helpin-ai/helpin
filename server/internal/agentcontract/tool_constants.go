package agentcontract

import "strings"

const (
	ToolRequestUserInput        = "request_user_input"
	ToolRequestApproval         = "request_approval"
	ToolRequestReviewCheckpoint = "request_review_checkpoint"
	ToolRequestHumanInput       = "request_human_input"
	ToolRequestHumanApproval    = "request_human_approval"
	ToolUpdatePlan              = "update_plan"

	ToolPublishPreview                = "publish_preview"
	ToolPreviewMarkdown               = "preview_md"
	ToolPreviewJSON                   = "preview_json"
	ToolPublishPRDDraft               = "publish_prd_draft"
	ToolPublishTaskPlan               = "publish_task_plan"
	ToolPublishTaskPlanDoc            = "publish_task_plan_doc"
	ToolPublishDocumentChangeProposal = "publish_document_change_proposal"
	ToolCompleteSupportCoverageGap    = "complete_support_coverage_gap"

	ToolScanSemgrep  = "scan_semgrep"
	ToolScanTrivy    = "scan_trivy"
	ToolScanGitleaks = "scan_gitleaks"

	ToolFindSkills = "find_skills"
	ToolReadSkill  = "read_skill"
	// Deprecated Go identifiers retained for source compatibility. Both resolve
	// to the single model-facing find_skills tool.
	ToolListAvailableSkills   = ToolFindSkills
	ToolSearchAvailableSkills = ToolFindSkills
)

// CanonicalToolName resolves runtime and legacy aliases to the persisted tool name.
func CanonicalToolName(name string) string {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.TrimPrefix(trimmed, HelpinMCPToolPrefix)
	switch trimmed {
	case ToolRequestHumanInput:
		return ToolRequestUserInput
	case ToolRequestHumanApproval:
		return ToolRequestApproval
	case "add_story_comment":
		return "add_task_comment"
	case "list_story_checklist":
		return "list_task_checklist"
	case "update_story_state":
		return "update_task_state"
	case "run_semgrep":
		return ToolScanSemgrep
	case "run_trivy":
		return ToolScanTrivy
	case "run_gitleaks":
		return ToolScanGitleaks
	case "checkout_repository":
		return "checkout_repositories"
	case "read_file", "read_file_range":
		return "read_files"
	case "search_files", "ripgrep", "grep":
		return "repository_search"
	case "find_symbol":
		return "read_symbol"
	case "find_callers", "find_callees":
		return "trace_symbol"
	case "list_available_skills", "search_available_skills":
		return ToolFindSkills
	case "web_search_brave", "web_search_exa":
		return "web_search"
	default:
		return trimmed
	}
}

// NormalizeToolNames canonicalizes and de-duplicates tool names.
func NormalizeToolNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		canonical := CanonicalToolName(name)
		if canonical == "" {
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}
	return normalized
}
