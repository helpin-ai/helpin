package agentcontract

import (
	"regexp"
	"strings"
)

const (
	HelpinMCPServerName = "helpin"
	HelpinMCPToolPrefix = "mcp__" + HelpinMCPServerName + "__"
)

func RenderRuntimeToolNamesInInstructions(instructions string) string {
	return renderCanonicalRuntimeToolNames(instructions)
}

var backtickedMCPRuntimeToolPattern = regexp.MustCompile("`mcp__[a-zA-Z0-9_-]+__[a-zA-Z0-9_-]+`")

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
