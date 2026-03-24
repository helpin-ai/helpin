package worker

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	TranscriptSummaryArtifactType    = "transcript_summary"
	transcriptSummaryKeepRecentCount = 8
	transcriptSummaryMinSourceCount  = 4
	artifactContextSoftCharBudget    = 18_000
	artifactContextEntryTrimChars    = 2_500
)

type TranscriptSummaryCheckpoint struct {
	CoveredThroughSequenceNo int       `json:"covered_through_sequence_no"`
	SourceMessageCount       int       `json:"source_message_count"`
	Summary                  string    `json:"summary"`
	GeneratedAt              time.Time `json:"generated_at,omitempty"`
}

func LatestTranscriptSummaryCheckpoint(artifacts []model.AgentRunArtifact) (*TranscriptSummaryCheckpoint, error) {
	var latest *TranscriptSummaryCheckpoint
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != TranscriptSummaryArtifactType || artifact.InlineContent == nil {
			continue
		}
		var checkpoint TranscriptSummaryCheckpoint
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &checkpoint); err != nil {
			return nil, fmt.Errorf("parse transcript summary artifact: %w", err)
		}
		if strings.TrimSpace(checkpoint.Summary) == "" || checkpoint.CoveredThroughSequenceNo <= 0 {
			continue
		}
		if latest == nil || checkpoint.CoveredThroughSequenceNo > latest.CoveredThroughSequenceNo {
			copied := checkpoint
			latest = &copied
		}
	}
	return latest, nil
}

func BuildTranscriptSummaryCheckpoint(messages []model.AgentRunMessage) *TranscriptSummaryCheckpoint {
	history := make([]model.AgentRunMessage, 0, len(messages))
	for _, message := range messages {
		switch strings.TrimSpace(message.MessageType) {
		case "status", "transcript_summary":
			continue
		default:
			history = append(history, message)
		}
	}

	if len(history) <= transcriptSummaryKeepRecentCount {
		return nil
	}
	source := history[:len(history)-transcriptSummaryKeepRecentCount]
	if len(source) < transcriptSummaryMinSourceCount {
		return nil
	}

	summary := summarizeTranscriptMessages(source)
	if strings.TrimSpace(summary) == "" {
		return nil
	}

	return &TranscriptSummaryCheckpoint{
		CoveredThroughSequenceNo: source[len(source)-1].SequenceNo,
		SourceMessageCount:       len(source),
		Summary:                  summary,
		GeneratedAt:              time.Now().UTC(),
	}
}

func BuildExecutionHistory(messages []model.AgentRunMessage, checkpoint *TranscriptSummaryCheckpoint) []ExecutionMessage {
	history := make([]ExecutionMessage, 0, len(messages)+1)
	if checkpoint != nil && strings.TrimSpace(checkpoint.Summary) != "" {
		history = append(history, ExecutionMessage{
			Role:    "user",
			Content: "Resume context from earlier turns:\n" + strings.TrimSpace(checkpoint.Summary),
		})
	}
	for _, message := range messages {
		switch strings.TrimSpace(message.MessageType) {
		case "status", "transcript_summary":
			continue
		}
		if checkpoint != nil && message.SequenceNo <= checkpoint.CoveredThroughSequenceNo {
			continue
		}
		history = append(history, executionMessageFromRunMessage(message))
	}
	return history
}

func TrimArtifactContext(ctx *ArtifactContext) *ArtifactContext {
	if ctx == nil || len(ctx.Entries) == 0 {
		return ctx
	}

	totalChars := 0
	preservedChars := 0
	for _, entry := range ctx.Entries {
		size := len(strings.TrimSpace(entry.Content))
		totalChars += size
		if entry.PreserveFull {
			preservedChars += size
		}
	}
	if totalChars <= artifactContextSoftCharBudget {
		return ctx
	}

	remainingBudget := artifactContextSoftCharBudget - preservedChars
	if remainingBudget < 0 {
		remainingBudget = 0
	}

	trimmableIndexes := make([]int, 0, len(ctx.Entries))
	for i, entry := range ctx.Entries {
		if !entry.PreserveFull && strings.TrimSpace(entry.Content) != "" {
			trimMable := i
			trimmableIndexes = append(trimmableIndexes, trimMable)
		}
	}
	if len(trimmableIndexes) == 0 {
		return ctx
	}

	trimmed := &ArtifactContext{Entries: make([]ArtifactContextEntry, len(ctx.Entries))}
	copy(trimmed.Entries, ctx.Entries)

	perEntryBudget := artifactContextEntryTrimChars
	if remainingBudget > 0 {
		perEntryBudget = remainingBudget / len(trimmableIndexes)
		if perEntryBudget <= 0 {
			perEntryBudget = artifactContextEntryTrimChars / 2
		}
		if perEntryBudget > artifactContextEntryTrimChars {
			perEntryBudget = artifactContextEntryTrimChars
		}
	}

	for _, index := range trimmableIndexes {
		content := strings.TrimSpace(trimmed.Entries[index].Content)
		if len(content) <= perEntryBudget {
			continue
		}
		trimmed.Entries[index].Content = truncateArtifactContent(content, perEntryBudget)
	}
	return trimmed
}

func summarizeTranscriptMessages(messages []model.AgentRunMessage) string {
	firstUser := ""
	userNotes := make([]string, 0, 3)
	assistantNotes := make([]string, 0, 3)
	toolNames := make(map[string]int)

	for _, message := range messages {
		content := compactSummaryText(message.Content, 280)
		switch message.Role {
		case "user":
			if firstUser == "" && content != "" {
				firstUser = content
			}
			if content != "" {
				userNotes = append(userNotes, content)
			}
		case "assistant":
			if content != "" {
				assistantNotes = append(assistantNotes, content)
			}
			for _, block := range parseSummaryBlocks(message.ContentBlocks) {
				if block.Type == ExecutionBlockTypeToolCall && strings.TrimSpace(block.ToolName) != "" {
					toolNames[strings.TrimSpace(block.ToolName)]++
				}
			}
		case "tool":
			for _, block := range parseSummaryBlocks(message.ContentBlocks) {
				if block.Type == ExecutionBlockTypeToolResult && strings.TrimSpace(block.ToolName) != "" {
					toolNames[strings.TrimSpace(block.ToolName)]++
				}
			}
		}
	}

	lines := []string{
		fmt.Sprintf("Earlier transcript covered %d messages through sequence %d.", len(messages), messages[len(messages)-1].SequenceNo),
	}
	if firstUser != "" {
		lines = append(lines, "Initial request: "+firstUser)
	}
	if highlights := lastUniqueItems(userNotes, 3); len(highlights) > 0 {
		lines = append(lines, "User follow-ups:")
		for _, item := range highlights {
			lines = append(lines, "- "+item)
		}
	}
	if highlights := lastUniqueItems(assistantNotes, 3); len(highlights) > 0 {
		lines = append(lines, "Assistant progress:")
		for _, item := range highlights {
			lines = append(lines, "- "+item)
		}
	}
	if len(toolNames) > 0 {
		names := make([]string, 0, len(toolNames))
		for name, count := range toolNames {
			if count > 1 {
				names = append(names, fmt.Sprintf("%s(x%d)", name, count))
			} else {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		lines = append(lines, "Earlier tool activity: "+strings.Join(names, ", "))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func executionMessageFromRunMessage(message model.AgentRunMessage) ExecutionMessage {
	execMessage := ExecutionMessage{
		SequenceNo: message.SequenceNo,
		Role:       message.Role,
		Content:    message.Content,
	}
	if len(message.ContentBlocks) > 0 && string(message.ContentBlocks) != "null" {
		var blocks []ExecutionBlock
		if err := json.Unmarshal(message.ContentBlocks, &blocks); err == nil {
			execMessage.Blocks = NormalizeExecutionBlocks(blocks)
		}
	}
	if strings.TrimSpace(execMessage.Content) == "" && len(execMessage.Blocks) > 0 {
		execMessage.Content = ExtractPersistedContentFromExecutionBlocks(execMessage.Blocks)
	}
	return execMessage
}

func parseSummaryBlocks(raw json.RawMessage) []ExecutionBlock {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var blocks []ExecutionBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}
	return NormalizeExecutionBlocks(blocks)
}

func compactSummaryText(text string, limit int) string {
	compact := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if compact == "" {
		return ""
	}
	if limit > 0 && len(compact) > limit {
		return compact[:limit] + "..."
	}
	return compact
}

func lastUniqueItems(items []string, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	out := make([]string, 0, limit)
	seen := map[string]bool{}
	for i := len(items) - 1; i >= 0 && len(out) < limit; i-- {
		item := strings.TrimSpace(items[i])
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func truncateArtifactContent(content string, limit int) string {
	if limit <= 0 || len(content) <= limit {
		return content
	}
	if limit < 64 {
		limit = 64
	}
	head := limit
	if head > len(content) {
		head = len(content)
	}
	return strings.TrimSpace(content[:head]) + "\n... [trimmed for prompt efficiency; full artifact remains persisted]"
}
