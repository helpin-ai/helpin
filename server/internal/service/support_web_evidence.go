package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportSourceExternalWeb = "support_external_web"

// Web citations use the runtime's actual successful fetches, never a child
// summary, a search snippet, or the first URL mentioned in generated text.
// Echo's prompt governs when third-party research is appropriate.
func (s *SupportChatService) prepareSupportWebEvidence(ctx context.Context, plan *model.CommandBarPlanRecord) []model.SupportRunEvidence {
	if s.agentService == nil || s.runRepo == nil {
		return nil
	}
	reader, ok := s.agentService.agentRuntimeClient.(supportMCPToolCallReader)
	if !ok {
		return nil
	}
	var runIDs map[int]string
	if json.Unmarshal(plan.RunIDsByStep, &runIDs) != nil {
		return nil
	}
	indexes := make([]int, 0, len(runIDs))
	for index := range runIDs {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	var evidence []model.SupportRunEvidence
	seen := map[string]bool{}
	for _, index := range indexes {
		run, err := s.runRepo.GetByID(ctx, plan.WorkspaceID, runIDs[index])
		if err != nil {
			slog.WarnContext(ctx, "support web research run unavailable", "run_id", runIDs[index], "error", err)
			continue
		}
		// The completed plan is authoritative. Its finalizer can run before
		// the child's terminal status has been projected into this row.
		if run == nil {
			continue
		}
		runtimeID, valid := agentRuntimeRunID(run)
		if !valid {
			continue
		}
		calls, err := reader.ListToolCalls(ctx, runtimeID)
		if err != nil {
			slog.WarnContext(ctx, "support web evidence unavailable", "run_id", run.ID, "error", err)
			continue
		}
		for _, call := range calls {
			row, valid := supportFetchedPageEvidence(plan, run.ID, runtimeID, call)
			if !valid || seen[row.URL] {
				continue
			}
			seen[row.URL] = true
			evidence = append(evidence, row)
		}
	}
	return evidence
}

func supportFetchedPageEvidence(plan *model.CommandBarPlanRecord, runID, runtimeID string, call AgentRuntimeToolCall) (model.SupportRunEvidence, bool) {
	if call.RunID != runtimeID || call.ToolName != "fetch_url" || call.ID == "" || call.Mutating || call.Error != "" {
		return model.SupportRunEvidence{}, false
	}
	var input struct {
		URL string `json:"url"`
	}
	var page struct {
		URL      string `json:"url"`
		FinalURL string `json:"final_url"`
		Status   int    `json:"status"`
		Title    string `json:"title"`
		Text     string `json:"text"`
	}
	// The native gateway stores an SDK result envelope, while older/direct
	// projections may store the raw JSON tool output.
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structured_content"`
		IsError           bool            `json:"is_error"`
		ApprovalRequired  bool            `json:"approval_required"`
	}
	if json.Unmarshal(call.Output, &result) != nil || result.IsError || result.ApprovalRequired {
		return model.SupportRunEvidence{}, false
	}
	pageOutput := call.Output
	if len(result.StructuredContent) > 0 {
		pageOutput = result.StructuredContent
	} else if len(result.Content) == 1 && result.Content[0].Type == "text" {
		pageOutput = json.RawMessage(result.Content[0].Text)
	}
	if json.Unmarshal(call.Input, &input) != nil || json.Unmarshal(pageOutput, &page) != nil ||
		page.Status < 200 || page.Status >= 300 || strings.TrimSpace(page.Text) == "" {
		return model.SupportRunEvidence{}, false
	}
	requestedURL := supportFetchedPageURL(input.URL)
	if requestedURL == "" || supportFetchedPageURL(page.URL) != requestedURL {
		return model.SupportRunEvidence{}, false
	}
	pageURL := requestedURL
	if page.FinalURL != "" {
		pageURL = supportFetchedPageURL(page.FinalURL)
		if pageURL == "" {
			return model.SupportRunEvidence{}, false
		}
	}
	text, _, _ := boundedDockSummary(page.Text, dockRunResultMaxChars)
	question, _, _ := strings.Cut(plan.Prompt, dockChildHandoffInstruction)
	relevance := termOverlapScore(normalizedTerms(question), page.Title+"\n"+text)
	id := "web-page:" + runID + ":" + call.ID
	return model.SupportRunEvidence{
		WorkspaceID: plan.WorkspaceID, EvidenceID: id, ReferenceID: id,
		SourceType: supportSourceExternalWeb, SourceID: runID, DocumentID: call.ID,
		Title: page.Title, URL: pageURL, Content: text,
		LexicalScore: .35 * relevance, CombinedScore: relevance,
	}, true
}

func supportFetchedPageURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	parsed.Fragment = ""
	return parsed.String()
}
