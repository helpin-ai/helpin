package service

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	dockChildResultSummaryChars = 3000
	dockRunResultDefaultChars   = 6000
	dockRunResultMaxChars       = 12000
)

const dockChildHandoffInstruction = "Result handoff: End with a self-contained summary of at most 2,500 characters. Lead with the concrete findings the parent needs, and reference durable artifacts or changed files for supporting detail."

const supportChildHandoffInstruction = "Support research handoff: Report only directly observed facts. Use configured knowledge sources for public product facts; do not perform web research or fetch outside pages. For repository research, include the relevant file paths and symbols and distinguish observed behavior from anything not found. Distinguish current behavior from historical descriptions and unconfirmed availability; do not infer discontinuation from missing documentation. Do not mention internal tools or run IDs."

type dockRunArtifactReference struct {
	ArtifactID   string `json:"artifact_id"`
	ResourceURI  string `json:"resource_uri"`
	ArtifactType string `json:"artifact_type"`
	Format       string `json:"format"`
	StorageMode  string `json:"storage_mode"`
}

type dockRunResultExcerpt struct {
	Content    string `json:"content"`
	Offset     int    `json:"offset"`
	CharCount  int    `json:"char_count"`
	Truncated  bool   `json:"truncated"`
	NextOffset *int   `json:"next_offset,omitempty"`
}

func withDockChildHandoffInstruction(instructions string) string {
	instructions = strings.TrimSpace(instructions)
	if instructions == "" || strings.Contains(instructions, dockChildHandoffInstruction) {
		return instructions
	}
	return instructions + "\n\n" + dockChildHandoffInstruction
}

func withSupportChildHandoffInstruction(instructions string) string {
	instructions = withDockChildHandoffInstruction(instructions)
	if instructions == "" || strings.Contains(instructions, supportChildHandoffInstruction) {
		return instructions
	}
	return instructions + "\n\n" + supportChildHandoffInstruction
}

func latestAssistantResponse(messages []model.AgentRunMessage) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if strings.TrimSpace(messages[index].Role) != "assistant" {
			continue
		}
		if content := strings.TrimSpace(messages[index].Content); content != "" {
			return content
		}
	}
	return ""
}

// boundedDockSummary keeps the complete handoff when it fits and otherwise
// replaces the final rune with an ellipsis so the returned summary never
// exceeds maxChars. CharCount always describes the unabridged response.
func boundedDockSummary(content string, maxChars int) (summary string, charCount int, truncated bool) {
	runes := []rune(strings.TrimSpace(content))
	charCount = len(runes)
	if maxChars <= 0 || charCount <= maxChars {
		return string(runes), charCount, false
	}
	if maxChars == 1 {
		return "…", charCount, true
	}
	return string(runes[:maxChars-1]) + "…", charCount, true
}

func dockRunResultWindow(content string, offset, limit int) dockRunResultExcerpt {
	runes := []rune(strings.TrimSpace(content))
	total := len(runes)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	excerpt := dockRunResultExcerpt{
		Content:   string(runes[offset:end]),
		Offset:    offset,
		CharCount: total,
		Truncated: end < total,
	}
	if end < total {
		next := end
		excerpt.NextOffset = &next
	}
	return excerpt
}

func dockRunArtifactReferences(artifacts []model.AgentRunArtifact) []dockRunArtifactReference {
	references := make([]dockRunArtifactReference, 0, len(artifacts))
	for _, artifact := range artifacts {
		typeName := strings.TrimSpace(artifact.ArtifactType)
		if typeName == "" || dockRunArtifactIsInternal(typeName) {
			continue
		}
		references = append(references, dockRunArtifactReference{
			ArtifactID:   artifact.ID,
			ResourceURI:  artifactReference(artifact.ID),
			ArtifactType: typeName,
			Format:       strings.TrimSpace(artifact.Format),
			StorageMode:  strings.TrimSpace(artifact.StorageMode),
		})
	}
	return references
}

func dockRunArtifactIsInternal(artifactType string) bool {
	switch strings.TrimSpace(artifactType) {
	case model.AgentRunArtifactTypeToolCall,
		model.AgentRunArtifactTypeAgentTurnDebug,
		model.AgentRunArtifactTypeAgentRepairState,
		model.AgentRunArtifactTypeProviderResponseCheckpoint,
		model.AgentRunArtifactTypeCodexAuthState,
		model.AgentRunArtifactTypeHumanInputRequest,
		model.AgentRunArtifactTypeHumanApprovalRequest:
		return true
	default:
		return false
	}
}
