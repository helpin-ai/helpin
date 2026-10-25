package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	coverageConnectedToolsLimit     = 12
	coverageConnectedToolDescLength = 160
)

// CoverageConnectedTool is a tool connected to the workspace, given to the
// analyzer so it can tell missing data from a connected tool the AI did not use.
type CoverageConnectedTool struct {
	Name  string   `json:"name"`
	Tools []string `json:"tools,omitempty"`
}

// coverageMissingDataInstruction explains the AI-reported missing data to the
// analyzer. Coverage counts these data gaps across conversations.
const coverageMissingDataInstruction = `ai_reported_missing_data, when present, lists kinds of data the support AI said it needed but none of its tools provide. If that missing data stopped the AI from answering, report a data gap (gap_kind=data, gap_category=context, fix type add_data) and name the kind of data in canonical_title, e.g. "Payment history for refund requests". connected_tools lists the tools connected to this workspace; if one of them covers that data, the AI should have used it, so use gap_category=evaluation instead and name the tool in ai_failure. Do not report a data gap only because more data would have been nice to have.`

// coverageAIReportedMissingData collects the data the support AI and the code
// check said they could not check, from their cards inside the segment.
func coverageAIReportedMissingData(messages []model.SupportMessage, segment CoverageConversationSegment) []string {
	var out []string
	seen := map[string]bool{}
	for _, message := range messages {
		if !message.IsInternal || message.MessageType != "note" || message.DeletedAt.Valid || message.CreatedAt.Before(segment.StartAt) || (segment.EndAt != nil && message.CreatedAt.After(*segment.EndAt)) {
			continue
		}
		var metadata struct {
			Handoff *struct {
				UnavailableData []string `json:"unavailable_data"`
			} `json:"handoff"`
			CodeCheck *struct {
				UnavailableData []string `json:"unavailable_data"`
			} `json:"code_check"`
		}
		if json.Unmarshal([]byte(message.Metadata), &metadata) != nil {
			continue
		}
		var items []string
		if metadata.Handoff != nil {
			items = append(items, metadata.Handoff.UnavailableData...)
		}
		if metadata.CodeCheck != nil {
			items = append(items, metadata.CodeCheck.UnavailableData...)
		}
		for _, item := range items {
			item = briefText(item)
			if key := strings.ToLower(item); item != "" && !seen[key] {
				seen[key] = true
				out = append(out, item)
			}
		}
	}
	return out
}

// coverageConnectedTools lists the workspace's enabled external tools by name
// with short descriptions. Failures leave the list empty; the analysis goes on.
func (s *SupportCoverageDailyAnalyzer) coverageConnectedTools(ctx context.Context, workspaceID string) []CoverageConnectedTool {
	if s == nil || s.conversationRepo == nil {
		return nil
	}
	servers, err := repository.NewExternalMCPRepository(s.conversationRepo.DB()).ListServers(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(ctx, "load connected tools for coverage", "workspace_id", workspaceID, "error", err)
		return nil
	}
	var tools []CoverageConnectedTool
	for _, server := range servers {
		if !server.Enabled {
			continue
		}
		entry := CoverageConnectedTool{Name: strings.TrimSpace(server.Name)}
		for _, tool := range server.Tools {
			if !tool.Enabled || len(entry.Tools) == coverageConnectedToolsLimit {
				continue
			}
			description := []rune(strings.Join(strings.Fields(tool.Description), " "))
			if len(description) > coverageConnectedToolDescLength {
				description = append(description[:coverageConnectedToolDescLength], '…')
			}
			entry.Tools = append(entry.Tools, strings.TrimSpace(tool.RemoteName+": "+string(description)))
		}
		tools = append(tools, entry)
	}
	return tools
}
