package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func feedMenuAction(kind string) bool {
	switch kind {
	case "feed_priority", "feed_move", "feed_snooze", "feed_wake", "feed_status", "feed_pause", "feed_resume":
		return true
	}
	return false
}
func feedFormAction(kind string) bool {
	return kind == "feed_reply" || kind == "feed_manage" || kind == "feed_assign" || feedMenuAction(kind)
}

func (f *feedContext) replyLabel() string {
	c := f.conv
	options := feedOptions(c)
	if len(options) > 1 || c.MergeRevision > 0 || c.ChannelIdentityID != nil {
		return "Reply…"
	}
	if f.contact != nil && f.contact.LifecycleStage == "customer" {
		return "Reply to customer"
	}
	if len(options) == 1 && options[0]["value"] == "email" {
		return "Reply by email"
	}
	if c.AnonymousID != nil || model.SupportChannelHasChatSession(c.Source) || c.Channel == "widget" {
		return "Reply to visitor"
	}
	return "Reply…"
}

func (s *SlackService) feedAIAction(ctx context.Context, c *model.SupportConversation) (string, error) {
	if c.Status == "resolved" || c.Status == "spam" || c.Channel == "portal" || !supportAIConversationSupported(c) {
		return "", nil
	}
	inst, err := s.channels.inbox.installationRepo.GetByWorkspace(ctx, c.WorkspaceID)
	if err != nil || inst == nil || !inst.Active {
		return "", err
	}
	settings := effectiveSupportAISettings(parseSettings(inst.Settings), c)
	if !shouldAutomaticallyProcessSupportAI(settings) || derefString(settings.AIAgentID) == "" || !model.SupportAIReplyAllowed(settings, c, nil) {
		return "", nil
	}
	if model.SupportAIConversationBlocked(c) {
		return "feed_resume", nil
	}
	return "feed_pause", nil
}

func (s *SlackService) feedTriageOptions(f *feedContext) []map[string]any {
	c, id := f.conv, f.thread.ID
	menu := []map[string]any{
		feedOption("Set priority…", id+":feed_priority"),
		feedOption("Move to inbox…", id+":feed_move"),
	}
	if c.Status != model.SupportConversationStatusResolved {
		if c.SnoozedUntil != nil && c.SnoozedUntil.After(s.channels.now()) {
			menu = append(menu, feedOption("Wake from snooze", id+":feed_wake"))
		} else {
			menu = append(menu, feedOption("Snooze…", id+":feed_snooze"))
		}
	}
	return append(menu, feedOption("Change status…", id+":feed_status"))
}

func (s *SlackService) feedCardActions(ctx context.Context, f *feedContext, owner string) (map[string]any, error) {
	c, id := f.conv, f.thread.ID
	assign := "Assign…"
	if c.AssignedUserID != nil {
		assign = truncate("Assigned to "+feedOneLine(owner), 70) + " ▾"
	}
	status := feedButton("feed_resolve", "Resolve", id+":"+c.Status)
	menu := s.feedTriageOptions(f)
	ai, err := s.feedAIAction(ctx, c)
	if err != nil {
		return nil, err
	}
	switch ai {
	case "feed_resume":
		menu = append(menu, feedOption("Resume AI…", id+":"+ai))
	case "feed_pause":
		menu = append(menu, feedOption("Pause AI…", id+":"+ai))
	}
	return slackButtons(feedButton("feed_reply", f.replyLabel(), id), feedButton("feed_assign", assign, id), status, map[string]any{"type": "overflow", "action_id": "feed_more", "options": menu}), nil
}

func feedSnoozeStamp(c *model.SupportConversation) string {
	if c.SnoozedUntil == nil {
		return ""
	}
	return c.SnoozedUntil.UTC().Format(time.RFC3339Nano)
}

func feedSnoozeChoices(now time.Time) []map[string]any {
	atNine := func(days int) time.Time {
		d := now.AddDate(0, 0, days)
		return time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, d.Location())
	}
	opt := func(label string, t time.Time) map[string]any {
		return feedOption(label+" · "+t.Format("Mon 15:04 MST"), strconv.FormatInt(t.Unix(), 10))
	}
	var choices []map[string]any
	if later := now.Add(3 * time.Hour); later.Day() == now.Day() {
		choices = append(choices, opt("Later today", later))
	}
	monday := (8 - int(now.Weekday())) % 7
	if monday == 0 {
		monday = 7
	}
	return append(choices, opt("Tomorrow", atNine(1)), opt("Next Monday", atNine(monday)), opt("In one week", atNine(7)), feedOption("Pick a date and time", "custom"))
}

func (s *SlackService) feedTriageForm(ctx context.Context, f *feedContext, p *feedActionJob) (string, string, []any, error) {
	c, inbox := f.conv, s.channels.inbox
	field := func(label string, options []map[string]any, current string) []any {
		return []any{feedSelectBlock("choice", label, options, current, false)}
	}
	info := func(text string) []any { return []any{map[string]any{"type": "section", "text": slackText(text)}} }
	switch p.Kind {
	case "feed_assign":
		members, err := inbox.ListConversationAssignableUsers(ctx, c.WorkspaceID, c.ID)
		if err != nil {
			return "", "", nil, err
		}
		actor := authorization.GetActor(ctx)
		var options []map[string]any
		for _, m := range members {
			if m.UserID == nil || m.AssignmentDisabledReason != nil {
				continue
			}
			name := firstNonEmptyString(m.DisplayName, m.Email)
			if actor != nil && *m.UserID == actor.UserID {
				options = append([]map[string]any{feedOption("Assign to me · "+name, *m.UserID)}, options...)
			} else {
				options = append(options, feedOption(name, *m.UserID))
			}
		}
		options = append(options, feedOption("Unassigned", "unassigned"))
		return "Assign conversation", "Save", field("Assignee", options, firstNonEmptyString(derefString(c.AssignedUserID), "unassigned")), nil
	case "feed_priority":
		return "Set priority", "Save", field("Priority", []map[string]any{feedOption("🔴 Urgent", "urgent"), feedOption("🟠 High", "high"), feedOption("Medium", "medium"), feedOption("Low", "low")}, c.Priority), nil
	case "feed_move":
		actor := authorization.GetActor(ctx)
		boxes, err := inbox.mailboxRepo.ListAccessible(ctx, c.WorkspaceID, actor.WorkspaceMemberID, actor.Role, false)
		if err != nil {
			return "", "", nil, err
		}
		var options []map[string]any
		if c.MailboxID != nil || derefString(c.FlowState) == model.SupportConversationFlowStateAIHandling {
			options = append(options, feedOption("Shared Inbox", "shared"))
		}
		for _, b := range boxes {
			if b.Active && b.ID != derefString(c.MailboxID) {
				options = append(options, feedOption(b.Name, b.ID))
			}
		}
		if len(options) == 0 {
			return "Move to inbox", "", info("There are no other available inboxes."), nil
		}
		fields := field("Destination inbox", options, "")
		if derefString(c.FlowState) == model.SupportConversationFlowStateAIHandling {
			fields = append(fields, info("Moving to Shared Inbox pauses AI replies and follow-ups."))
		}
		return "Move to inbox", "Move", fields, nil
	case "feed_status":
		options := []map[string]any{feedOption("Open", "open"), feedOption("Waiting on customer", "waiting_on_customer"), feedOption("Resolved", "resolved"), feedOption("Spam", "spam")}
		return "Change status", "Save", field("Status", options, c.Status), nil
	case "feed_snooze":
		if c.Status == "resolved" || c.SnoozedUntil != nil && c.SnoozedUntil.After(s.channels.now()) {
			return "Snooze", "", info("Check the updated status on the conversation card."), nil
		}
		now := s.channels.now().UTC()
		workspace, err := inbox.workspaceRepo.GetByID(ctx, c.WorkspaceID)
		if err != nil {
			return "", "", nil, err
		}
		if workspace != nil && workspace.Timezone != "" {
			if zone, e := time.LoadLocation(workspace.Timezone); e == nil {
				now = now.In(zone)
			}
		}
		fields := field("Snooze until", feedSnoozeChoices(now), "")
		fields = append(fields, map[string]any{"type": "input", "block_id": "custom", "label": slackText("Custom date and time"), "optional": true, "element": map[string]any{"type": "datetimepicker", "action_id": "value"}})
		return "Snooze conversation", "Snooze", fields, nil
	case "feed_wake":
		if c.SnoozedUntil == nil {
			return "Wake conversation", "", info("This conversation is already awake."), nil
		}
		return "Wake conversation", "Wake", info("Return this conversation to the inbox now?"), nil
	case "feed_pause", "feed_resume":
		allowed, err := s.feedAIAction(ctx, c)
		if err != nil {
			return "", "", nil, err
		}
		if allowed != p.Kind {
			return "AI handling", "", info("AI handling has changed. Check the updated conversation card."), nil
		}
		if allowed == "feed_pause" {
			return "Pause AI", "Pause AI", info("Pause AI replies and follow-ups so the team can handle this conversation."), nil
		}
		prompt := "Resuming releases the human assignment and allows AI to answer the next incoming message."
		if c.CustomerRequestedHumanAt != nil {
			prompt = "This person asked for a teammate. " + prompt
		}
		return "Resume AI", "Resume AI", info(prompt), nil
	}
	return "", "", nil, ErrChannelInvalid
}

func (s *SlackService) applyFeedTriage(ctx context.Context, f *feedContext, p *feedActionJob, actor string) error {
	c, inbox := f.conv, s.channels.inbox
	stale := errors.New("The conversation changed. Review its current settings before saving.")
	switch p.Kind {
	case "feed_assign":
		if p.Draft.Owner != derefString(c.AssignedUserID) {
			return stale
		}
		var next *string
		if p.Choice != "unassigned" {
			if !isUUIDString(p.Choice) {
				return ErrChannelInvalid
			}
			next = &p.Choice
		}
		return inbox.AssignConversationUser(context.WithValue(ctx, expectedSupportAssigneeKey{}, p.Draft.Owner), c.WorkspaceID, c.ID, next, actor)
	case "feed_priority":
		if c.Priority != p.Draft.Priority {
			return stale
		}
		_, err := inbox.UpdateConversationPriority(ctx, c.WorkspaceID, c.ID, p.Choice, actor)
		return err
	case "feed_status":
		if c.Status != p.Draft.Status {
			return stale
		}
		_, err := inbox.UpdateConversationStatus(ctx, c.WorkspaceID, c.ID, p.Choice, actor)
		return err
	case "feed_move":
		if derefString(c.MailboxID) != p.Draft.Mailbox {
			return stale
		}
		var dest *string
		if p.Choice != "shared" {
			if !isUUIDString(p.Choice) {
				return ErrChannelInvalid
			}
			dest = &p.Choice
		}
		_, err := inbox.MoveConversation(ctx, c.WorkspaceID, c.ID, dest, actor)
		return err
	case "feed_snooze", "feed_wake":
		if feedSnoozeStamp(c) != p.Draft.Snooze || c.Status != p.Draft.Status {
			return stale
		}
		if p.Kind == "feed_wake" {
			_, err := inbox.UnsnoozeConversation(ctx, c.WorkspaceID, c.ID, actor)
			return err
		}
		stamp := p.CustomTime
		if p.Choice != "custom" {
			v, err := strconv.ParseInt(p.Choice, 10, 64)
			if err != nil {
				return ErrChannelInvalid
			}
			stamp = v
		}
		if stamp == 0 {
			return fmt.Errorf("Choose a custom date and time.")
		}
		_, err := inbox.SnoozeConversation(ctx, c.WorkspaceID, c.ID, time.Unix(stamp, 0), actor)
		return err
	case "feed_pause", "feed_resume":
		allowed, err := s.feedAIAction(ctx, c)
		if err != nil {
			return err
		}
		if allowed != p.Kind {
			return stale
		}
		req := model.SupportAIControlRequest{Action: "pause", ExpectedVersion: p.Draft.AIControlVersion}
		if p.Kind == "feed_resume" {
			req.Action = "return"
			req.ConfirmHumanRequest = p.Draft.HumanRequested
		}
		return inbox.ChangeConversationAIControl(ctx, c.WorkspaceID, c.ID, actor, req)
	}
	return ErrChannelInvalid
}
