package worker

import (
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
	if canonical == "" {
		return ""
	}
	if IsHelpinMCPRuntimeToolName(canonical) {
		return canonical
	}
	if !IsHelpinMCPToolAlias(canonical) {
		return canonical
	}
	return HelpinMCPToolPrefix + canonical
}

func HelpinMCPDisplayToolName(name string) string {
	return CanonicalToolName(name)
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

func RuntimeToolNamesForPrompt(aliases ...string) []string {
	names := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		name := RuntimeToolNameForPrompt(alias)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
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
	rendered := strings.TrimSpace(instructions)
	if rendered == "" {
		return ""
	}
	for _, alias := range HelpinMCPToolAliases() {
		runtimeName := HelpinMCPRuntimeToolName(alias)
		if runtimeName == "" || runtimeName == alias {
			continue
		}
		rendered = strings.ReplaceAll(rendered, "`"+alias+"`", "`"+runtimeName+"`")
	}
	return rendered
}
