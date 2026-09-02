package agentcontract

import (
	"strings"
)

const (
	HelpinMCPServerName = "helpin"
	HelpinMCPToolPrefix = "mcp__" + HelpinMCPServerName + "__"

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

// CanonicalToolName normalizes authored and client-provided names before they
// are persisted. Provider discovery and execution accept only canonical names;
// future renames require an explicit data migration instead of provider aliases.
//
// The frontend's canonicalToolName is intentionally NOT a mirror of this: it
// strips the MCP prefix and stops, because it renders stored transcripts and
// must show the tool that actually ran. Aliasing is for deciding what may be
// called now; it would be wrong when replaying what was called before.
func CanonicalToolName(name string) string {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.TrimPrefix(trimmed, HelpinMCPToolPrefix)
	if canonical := legacyToolAliases[trimmed]; canonical != "" {
		return canonical
	}
	return trimmed
}

// legacyToolAliases is write-side hygiene for authored contracts and older
// clients. Provider discovery and execution expose only canonical names.
var legacyToolAliases = map[string]string{
	ToolRequestHumanInput:     ToolRequestUserInput,
	ToolRequestHumanApproval:  ToolRequestApproval,
	"add_story_comment":       "add_task_comment",
	"list_story_checklist":    "list_task_checklist",
	"update_story_state":      "update_task_state",
	"run_semgrep":             ToolScanSemgrep,
	"run_trivy":               ToolScanTrivy,
	"run_gitleaks":            ToolScanGitleaks,
	"checkout_repository":     "checkout_repositories",
	"read_file":               "read_files",
	"read_file_range":         "read_files",
	"search_files":            "repository_search",
	"ripgrep":                 "repository_search",
	"grep":                    "repository_search",
	"list_buyer_signals":      "list_crm_signals",
	"find_symbol":             "read_symbol",
	"find_callers":            "trace_symbol",
	"find_callees":            "trace_symbol",
	"list_available_skills":   ToolFindSkills,
	"search_available_skills": ToolFindSkills,
	"web_search_brave":        "web_search",
	"web_search_exa":          "web_search",
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
