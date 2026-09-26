package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type chatHistoryPart struct {
	Content    string `json:"content"`
	NextOffset *int   `json:"next_offset,omitempty"`
}

// Offsets are Unicode code points so clients can retrieve complete messages
// without splitting characters, duplicating text, or skipping a truncated tail.
func chatHistoryExcerpt(content string, offset, limit int) chatHistoryPart {
	chars := []rune(content)
	if offset < 0 {
		offset = 0
	}
	if offset > len(chars) {
		offset = len(chars)
	}
	end := offset + limit
	if end > len(chars) {
		end = len(chars)
	}
	part := chatHistoryPart{Content: string(chars[offset:end])}
	if end < len(chars) {
		part.NextOffset = &end
	}
	return part
}

func chatHistoryContent(message model.AgentRunMessage) string {
	content := message.Content
	// Successor prompts contain old handoffs. Never recursively quote them or
	// turn attached context into a new user instruction.
	if message.Role == "user" {
		for _, pattern := range dockChatUntrustedContextPatterns {
			content = pattern.ReplaceAllString(content, "")
		}
	}
	return strings.TrimSpace(content)
}

func historyByteExcerpt(content string, limit int) string {
	if len(content) <= limit {
		return content
	}
	end := limit
	for end > 0 && !utf8.RuneStart(content[end]) {
		end--
	}
	return content[:end] + "\n[Excerpt truncated.]"
}

func (s *DockChatService) carryForwardMessages(ctx context.Context, run *model.AgentRun) []model.AgentRunMessage {
	if s.runMessageRepo == nil {
		return nil
	}
	if run.DockChatID == nil {
		messages, err := s.runMessageRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			return nil
		}
		return messages
	}
	messages, _, err := s.runMessageRepo.ListChatHistory(ctx, run.WorkspaceID, *run.DockChatID, 0, dockChatCarryForwardTurns)
	if err != nil {
		return nil
	}
	original, err := s.runMessageRepo.FirstChatRequest(ctx, run.WorkspaceID, *run.DockChatID)
	if err == nil && original != nil && (len(messages) == 0 || messages[0].ID != original.ID) {
		messages = append([]model.AgentRunMessage{*original}, messages...)
	}
	return messages
}

func renderChatCarryForward(messages []model.AgentRunMessage) string {
	substantive := make([]model.AgentRunMessage, 0, len(messages))
	for _, message := range messages {
		if message.Role != "user" && !(message.Role == "assistant" && message.MessageType == "assistant_final") {
			continue
		}
		message.Content = chatHistoryContent(message)
		if message.Content != "" {
			substantive = append(substantive, message)
		}
	}
	// Reserve the original request, then allocate the remainder newest first.
	// Render the selected excerpts chronologically to preserve conversational order.
	lines := make([]string, len(substantive))
	remaining := dockChatCarryForwardTotal
	for n := 0; n < len(substantive); n++ {
		i := len(substantive) - n
		if n == 0 {
			i = 0
		}
		if remaining < 256 {
			break
		}
		message := substantive[i]
		maxBytes := dockChatCarryForwardChars
		if n == 0 && maxBytes > 4000 {
			maxBytes = 4000
		}
		if maxBytes > remaining-192 {
			maxBytes = remaining - 192
		}
		sequence := int64(0)
		if message.DockChatSequence != nil {
			sequence = *message.DockChatSequence
		}
		line := fmt.Sprintf("[message %d, %s]\n%s\n", sequence, message.Role, historyByteExcerpt(message.Content, maxBytes))
		lines[i] = line
		remaining -= len(line)
	}
	return strings.Join(lines, "")
}

func (s *DockChatService) carryForwardPlan(ctx context.Context, run *model.AgentRun) string {
	if s.agentService == nil || s.agentService.sessionSnapshotRepo == nil {
		return ""
	}
	var plan *model.CodingSessionRunPlan
	var err error
	if run.DockChatID != nil {
		plan, err = s.agentService.sessionSnapshotRepo.LatestPlanByDockChat(ctx, run.WorkspaceID, *run.DockChatID)
	} else {
		snapshot, readErr := s.agentService.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
		if readErr != nil || snapshot == nil {
			return ""
		}
		state, decodeErr := model.DecodeCodingSessionStreamSnapshot(snapshot.SnapshotPayload)
		if decodeErr != nil || state == nil {
			return ""
		}
		plan = state.CurrentPlan
	}
	if err != nil || plan == nil {
		return ""
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	return "Recorded work plan (verify current state before acting):\n" + historyByteExcerpt(string(encoded), 8000) + "\n"
}

func (s *DockChatService) carryForwardArtifacts(ctx context.Context, run *model.AgentRun) string {
	if s.artifactRepo == nil || run.DockChatID == nil {
		return ""
	}
	artifacts, err := s.artifactRepo.ListRecentChatArtifactReferences(ctx, run.WorkspaceID, *run.DockChatID)
	if err != nil || len(artifacts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Published artifact references (files are not in the new execution workspace):\n")
	if len(artifacts) == 20 {
		b.WriteString("Showing the latest 20 artifacts.\n")
	}
	for _, artifact := range artifacts {
		// Deliberately omit object keys, signed URLs and arbitrary metadata.
		fmt.Fprintf(&b, "- %s (artifact %s, run %s)\n", historyByteExcerpt(artifact.ArtifactType, 100), artifact.ID, artifact.RunID)
	}
	return b.String()
}
