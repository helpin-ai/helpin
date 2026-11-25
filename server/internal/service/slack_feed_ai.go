package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type slackFeedAIMetadata struct {
	IsHandoff       bool                `json:"ai_handoff_brief"`
	Handoff         *supportHandoffCard `json:"handoff"`
	CodeCheck       *supportCodeCheck   `json:"code_check"`
	InvestigationID string              `json:"investigation_id"`
}

func slackFeedAINote(m *model.SupportMessage) slackFeedAIMetadata {
	var meta slackFeedAIMetadata
	if m == nil || m.DeletedAt.Valid || m.MessageType != "note" || !m.IsInternal || m.SenderType != "agent" || m.SenderUserID != nil {
		return meta
	}
	if json.Unmarshal([]byte(m.Metadata), &meta) != nil {
		return slackFeedAIMetadata{}
	}
	if !meta.IsHandoff {
		meta.Handoff = nil
	}
	return meta
}

func (meta slackFeedAIMetadata) finishedCodeCheck() bool {
	return meta.CodeCheck != nil && (meta.CodeCheck.Status == supportCodeCheckDone || meta.CodeCheck.Status == supportCodeCheckFailed)
}

func slackCodeCheckSummary(check *supportCodeCheck) string {
	if check.Status == supportCodeCheckFailed {
		return firstNonEmptyString(check.Message, supportCodeCheckUnfinishedMessage)
	}
	parts := []string{}
	if check.Found != "" {
		parts = append(parts, truncate(check.Found, 750))
	}
	if len(check.UnavailableData) > 0 {
		parts = append(parts, "**Couldn't check**\n"+truncate(briefList(check.UnavailableData), 400))
	}
	if check.Investigate != "" {
		parts = append(parts, "**Next step**\n"+truncate(check.Investigate, 400))
	}
	if task := check.Task; task != nil {
		label := "Proposed task"
		if task.Kind == "existing" && task.Key != "" {
			label = task.Key
		}
		parts = append(parts, "**"+label+"** "+truncate(task.Text, 300))
	}
	if check.Investigate == "" && check.Task == nil {
		switch {
		case supportCodeCheckConfirmed(*check) && check.Reply != "":
			parts = append(parts, "A suggested customer reply is available in Helpin.")
		case check.Docs != "":
			parts = append(parts, "**Docs to update**\n"+truncate(check.Docs, 400))
		}
	}
	return strings.Join(parts, "\n\n")
}

func slackHandoffSummary(card *supportHandoffCard) string {
	parts := []string{}
	for _, item := range []struct{ label, text string }{
		{"", card.Issue}, {"Reason", card.Reason}, {"AI checked", supportHandoffCheckLines(card.Checked)},
		{"Open questions", briefList(card.OpenQuestions)}, {"Couldn't check", briefList(card.UnavailableData)},
	} {
		if item.text != "" {
			value := truncate(item.text, 650)
			if item.label != "" {
				value = "**" + item.label + "**\n" + value
			}
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (s *SlackService) mirrorFeedAINote(ctx context.Context, f *feedContext, m *model.SupportMessage) error {
	meta := slackFeedAINote(m)
	if meta.Handoff != nil {
		return s.mirrorFeedHandoff(ctx, f, m, meta.Handoff)
	}
	if !meta.finishedCodeCheck() {
		return nil
	}
	handoff, err := s.repo.FeedHandoffForCheck(ctx, f.account.WorkspaceID, f.thread, m.ID)
	if err != nil {
		return err
	}
	if handoff != nil {
		if card := slackFeedAINote(handoff).Handoff; card != nil {
			return s.mirrorFeedHandoff(ctx, f, handoff, card)
		}
	}
	// Background investigations from before explicit handoff linking already
	// belong to a human handoff. A pre-reply check alone never leaves Helpin.
	if meta.InvestigationID == m.ID && feedStillNeedsCodeCheck(f.conv, m.CreatedAt) {
		return s.writeFeedAIUpdate(ctx, f, m.ID, m.ID, "Code check", slackCodeCheckSummary(meta.CodeCheck))
	}
	return nil
}

func (s *SlackService) mirrorFeedHandoff(ctx context.Context, f *feedContext, m *model.SupportMessage, card *supportHandoffCard) error {
	summary := slackHandoffSummary(card)
	var finding string
	if isUUIDString(card.CodeCheckID) {
		check, err := s.channels.inbox.messageRepo.GetByID(ctx, card.CodeCheckID)
		if err != nil {
			return err
		}
		if check != nil && check.ConversationID == m.ConversationID && check.WorkspaceID == m.WorkspaceID {
			if meta := slackFeedAINote(check); meta.finishedCodeCheck() {
				finding = slackCodeCheckSummary(meta.CodeCheck)
			}
		}
	}
	target := m.ID
	if finding != "" && !card.CodeCheckPending {
		summary += "\n\n**Code check**\n" + finding
		target = card.CodeCheckID
	}
	if err := s.writeFeedAIUpdate(ctx, f, m.ID, target, "AI handoff", summary); err != nil {
		return err
	}
	if card.CodeCheckPending && finding != "" && feedStillNeedsCodeCheck(f.conv, m.CreatedAt) {
		return s.writeFeedAIUpdate(ctx, f, card.CodeCheckID, card.CodeCheckID, "Code check", finding)
	}
	return nil
}

// A result arriving after closure or a return to AI no longer needs a teammate's attention.
func feedStillNeedsCodeCheck(c *model.SupportConversation, handedOffAt time.Time) bool {
	return c.Status != model.SupportConversationStatusResolved &&
		(c.AIResumedAt == nil || !c.AIResumedAt.After(handedOffAt)) &&
		(supportConversationHumanOwned(c) || derefString(c.AIState) == "escalated")
}

func (s *SlackService) writeFeedAIUpdate(ctx context.Context, f *feedContext, id, target, kind, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return nil
	}
	title := "Internal · " + kind
	open := feedButton("open_support", "Open in Helpin", f.thread.ID)
	reportURL := f.conversationURL + "#message-" + target
	open["url"] = reportURL
	blocks := []any{
		map[string]any{"type": "context", "elements": []any{slackText(title)}},
		slackMarkdown(summary), slackButtons(open),
	}
	fallback := title + " · " + f.conv.Subject + "\n" + supportTranscriptPlainText(renderSupportTranscriptMessageHTML(summary)) + "\n" + reportURL
	receiptKey := id + ":ai_summary"
	rendered := inboundStableID(strings.Join([]string{fallback, summary, f.conversationURL, target}, "\x00"))
	previous, err := s.repo.FeedReceipt(ctx, f.thread.ID, receiptKey)
	if err != nil || previous == rendered {
		return err
	}
	if err := s.writeFeedReply(ctx, f, id, fallback, blocks); err != nil {
		return err
	}
	return s.repo.SaveFeedReceipt(ctx, f.thread.ID, receiptKey, rendered)
}
