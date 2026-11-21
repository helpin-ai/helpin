package service

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type slackDraft struct {
	Matches                                   []model.PMTriageCandidateView
	Thread                                    model.SlackThread
	ConversationID, SourceHash, HistoryNotice string
	Title, Description, TaskType, Priority    string
	TeamID, EpicID                            *string
}

func (s *SlackService) offerDraft(ctx context.Context, a *model.SupportChannelAccount, token string, route *model.SlackRoute, m slack.Message, actor *authorization.Actor, conv *model.SupportConversation) error {
	allowed, err := s.authz.CanAccessModule(ctx, actor, model.ModulePM)
	if err != nil {
		return err
	}
	if !allowed || !s.authz.Can(actor, authorization.PermPMEdit) {
		return s.private(ctx, token, m, "You need permission to create tasks in Helpin.")
	}
	d := slackDraft{Thread: model.SlackThread{AccountID: a.ID, Channel: m.Channel, Root: m.Root()}, TeamID: route.TeamID, EpicID: route.EpicID}
	var history []model.SupportMessage
	var source *string
	if conv != nil {
		source = &conv.ID
		if conv.MailboxID != nil {
			box, err := s.channels.inbox.mailboxRepo.GetByID(ctx, a.WorkspaceID, *conv.MailboxID)
			if err != nil {
				return err
			}
			if box != nil {
				d.TeamID = box.LinkedTeamID
			}
		}
		if d.TeamID == nil {
			member, err := s.tasks.workspaceRepo.GetMembership(ctx, a.WorkspaceID, actor.UserID)
			if err != nil {
				return err
			}
			if member != nil {
				inboxKey := derefString(conv.MailboxID)
				if inboxKey == "" {
					inboxKey = "shared"
				}
				d.TeamID = nilIfEmpty(member.SupportTaskTeamByInbox[inboxKey])
				if d.TeamID == nil {
					d.TeamID = member.SupportDefaultTeamID
				}
			}
		}
		if d.TeamID != nil && !canAccessTeam(ctx, d.TeamID) {
			return s.private(ctx, token, m, "The inbox team is unavailable to you.")
		}

		d.ConversationID = conv.ID
		history, err = s.channels.inbox.ListConversationMessages(ctx, a.WorkspaceID, conv.ID, false)
		if err != nil {
			return err
		}
		_, d.SourceHash = supportPMTriageEvidence(conv, history)
		d.Title = conv.Subject
	} else {
		history, d.HistoryNotice = s.draftThreadHistory(ctx, token, m)
		if len(history) > 0 {
			d.Title = truncate(history[0].Content, 120)
		}
	}
	var text strings.Builder
	for _, msg := range history {
		if !msg.IsInternal && msg.MessageType == "reply" {
			text.WriteString(tiptap.RichTextToMarkdown(msg.Content))
			text.WriteByte('\n')
		}
	}
	description := strings.TrimSpace(text.String())
	d.Description = truncate(description, 2800)
	manualExcerpt := d.Description != description
	if !a.AIRepliesDisabled {
		evidence := conv
		if evidence == nil {
			evidence = &model.SupportConversation{ID: inboundStableID(a.ID + ":" + m.Channel + ":" + m.Root()), WorkspaceID: a.WorkspaceID, Subject: d.Title}
		}
		generated, err := s.channels.inbox.generateTaskDraftFromConversation(ctx, a.WorkspaceID, evidence, history)
		if err == nil && generated != nil {
			d.Title, d.Description, d.TaskType, d.Priority = generated.Title, generated.Description, generated.TaskType, generated.Priority
			manualExcerpt = false
		}
	}
	if conv == nil && manualExcerpt && d.HistoryNotice == "" {
		d.HistoryNotice = "The thread was shortened to fit the description. Review it for missing details."
	}
	if s.triage != nil && !a.AIRepliesDisabled {
		var match *model.PMTriageView
		var err error
		if conv != nil {
			match, err = s.triage.Analyze(ctx, a.WorkspaceID, "support_conversation", conv.ID)
		} else {
			match, err = s.triage.AnalyzeDraft(ctx, a.WorkspaceID, model.PMTriageDraftRequest{DraftID: inboundStableID(a.ID + m.Channel + m.Root()), Name: d.Title, Description: d.Description, TeamID: d.TeamID})
		}
		if err == nil && match != nil && match.Status == "ready" && match.Assessment != nil && match.Assessment.Actionable {
			for _, suggestion := range match.Assessment.Matches {
				for _, candidate := range match.Candidates {
					if candidate.ID == suggestion.TaskID && len(d.Matches) < 3 {
						d.Matches = append(d.Matches, candidate)
					}
				}
			}
		}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	if err = s.repo.SaveDraft(ctx, id, a.ID, actor.UserID, source, raw); err != nil {
		return err
	}
	return s.private(ctx, token, m, "Review a task for this thread.", slackButtons(map[string]any{"type": "button", "text": slackText("Review task"), "action_id": "draft_task", "value": id}))
}

// Bound Slack API calls made by a single review, including on repeated cursors.
// Always retain the selected message when later parts of a thread are unavailable.
func (s *SlackService) draftThreadHistory(ctx context.Context, token string, m slack.Message) ([]model.SupportMessage, string) {
	const maxPages = 4
	var history []model.SupportMessage
	seen := map[string]bool{}
	add := func(msg slack.Message) {
		if msg.BotID != "" || strings.TrimSpace(msg.Text) == "" || seen[msg.TS] {
			return
		}
		seen[msg.TS] = true
		history = append(history, model.SupportMessage{SenderType: "customer", MessageType: "reply", Content: msg.Text})
	}
	cursor := ""
	visited := map[string]bool{}
	for range maxPages {
		var page struct {
			Messages []slack.Message
			More     bool `json:"has_more"`
			Metadata struct {
				Next string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
		params := url.Values{"channel": {m.Destination()}, "ts": {m.Root()}, "limit": {"50"}}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		if err := s.api.Get(ctx, token, "conversations.replies", params, &page); err != nil {
			break
		}
		for _, msg := range page.Messages {
			add(msg)
		}
		cursor = page.Metadata.Next
		if cursor == "" && !page.More && len(history) > 0 {
			add(m)
			return history, ""
		}
		if cursor == "" || visited[cursor] {
			break
		}
		visited[cursor] = true
	}
	add(m)
	if len(history) == 0 {
		return history, "Slack provided no message text. Add the task details before saving."
	}
	if len(history) == 1 && seen[m.TS] {
		return history, "Draft uses the selected message only. Slack thread history was unavailable."
	}
	return history, "Draft uses part of this thread. Review it for missing details."
}

func selectBlock(id, label string, options []map[string]any, initial string, optional bool) map[string]any {
	field := map[string]any{"type": "static_select", "action_id": "value", "options": options, "placeholder": slackText("Choose " + strings.ToLower(label))}
	for _, opt := range options {
		if opt["value"] == initial {
			field["initial_option"] = opt
		}
	}
	return map[string]any{"type": "input", "block_id": id, "label": slackText(label), "element": field, "optional": optional}
}

func enumOptions(values ...string) []map[string]any {
	opts := make([]map[string]any, 0, len(values))
	for _, v := range values {
		opts = append(opts, map[string]any{"text": slackText(v), "value": v})
	}
	return opts
}
