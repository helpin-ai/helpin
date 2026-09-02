package agentcontract

import (
	"regexp"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
)

const (
	HelpinMCPServerName = "helpin"
	HelpinMCPToolPrefix = "mcp__" + HelpinMCPServerName + "__"
)

func HelpinMCPRuntimeToolName(alias string) string {
	canonical := CanonicalToolName(alias)
	return canonical
}

func IsHelpinMCPRuntimeToolName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), HelpinMCPToolPrefix)
}

func IsHelpinMCPToolAlias(name string) bool {
	canonical := CanonicalToolName(name)
	if canonical == "" {
		return false
	}
	switch canonical {
	case ToolUpdatePlan,
		ToolRequestUserInput,
		ToolRequestApproval,
		ToolRequestReviewCheckpoint,
		ToolPublishPreview,
		ToolPreviewMarkdown,
		ToolPreviewJSON,
		ToolPublishPRDDraft,
		ToolPublishTaskPlan,
		ToolPublishTaskPlanDoc,
		ToolPublishDocumentChangeProposal:
		return true
	default:
		_, ok := commandtools.ToolMetadataForAlias(canonical)
		return ok
	}
}

func RuntimeToolNameForPrompt(alias string) string {
	return HelpinMCPRuntimeToolName(alias)
}

func HelpinMCPToolAliases() []string {
	aliases := []string{
		ToolUpdatePlan,
		ToolRequestUserInput,
		ToolRequestApproval,
		ToolRequestReviewCheckpoint,
		ToolPublishPreview,
		ToolPreviewMarkdown,
		ToolPreviewJSON,
		ToolPublishPRDDraft,
		ToolPublishTaskPlan,
		ToolPublishTaskPlanDoc,
		ToolPublishDocumentChangeProposal,
	}
	for _, meta := range commandtools.AllRuntimeToolMetadata() {
		aliases = append(aliases, meta.Alias)
	}
	aliases = NormalizeToolNames(aliases)
	sort.SliceStable(aliases, func(i, j int) bool {
		return len(aliases[i]) > len(aliases[j])
	})
	return aliases
}

func RenderRuntimeToolNamesInInstructions(instructions string) string {
	return renderCanonicalRuntimeToolNames(instructions)
}

var backtickedMCPRuntimeToolPattern = regexp.MustCompile("`mcp__[a-zA-Z0-9_-]+__[a-zA-Z0-9_-]+`")

func RenderRuntimeToolNamesInInstructionsForRuntime(instructions, _ string) string {
	return renderCanonicalRuntimeToolNames(instructions)
}

func renderCanonicalRuntimeToolNames(instructions string) string {
	rendered := strings.TrimSpace(instructions)
	return backtickedMCPRuntimeToolPattern.ReplaceAllStringFunc(rendered, func(token string) string {
		qualified := strings.Trim(token, "`")
		logical := CanonicalToolName(qualified)
		if logical == "" || logical == qualified {
			return token
		}
		return "`" + logical + "`"
	})
}
