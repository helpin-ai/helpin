package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type slackShortcutJob struct {
	Command, Requester string
	Message            slack.Message
}

func (s *SlackService) acceptShortcut(ctx context.Context, v SlackAction) (any, error) {
	switch v.Callback {
	case "create_support", "create_task", "link_task":
	default:
		return nil, ErrChannelInvalid
	}
	a, err := s.channels.accounts.FindLive(ctx, "slack", v.Team.ID)
	if a == nil || err != nil {
		return nil, err
	}
	m := v.Message
	m.Type = "message"
	m.Channel = v.Channel.ID
	if !m.HumanMessage() {
		return map[string]any{}, nil
	}
	route, err := s.repo.Route(ctx, a.ID, m.Channel)
	if err != nil || route == nil {
		return map[string]any{}, err
	}
	m.Channel, m.CurrentChannelID = route.ChannelID, route.CurrentChannelID
	if v.Callback == "link_task" {
		return s.linkView(ctx, a, v, route, m)
	}
	payload, err := json.Marshal(slackShortcutJob{Command: v.Callback, Requester: v.User.ID, Message: m})
	if err != nil {
		return nil, err
	}
	subject := model.SupportChannelSubjectKey(a.ID, m.User)
	job := model.SupportInboundJob{ID: inboundStableID("slack_shortcut:" + v.Trigger + ":" + v.Callback), Kind: "slack_shortcut", WorkspaceID: &a.WorkspaceID, ChannelAccountID: &a.ID, SubjectKey: &subject, Payload: string(payload), Status: "pending", AvailableAt: s.channels.now()}
	return map[string]any{}, s.channels.jobs.Create(ctx, &job)
}

func (s *SlackService) shortcutJob(ctx context.Context, job *model.SupportInboundJob) error {
	var payload slackShortcutJob
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return err
	}
	a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *job.ChannelAccountID)
	if a == nil || err != nil {
		return err
	}
	m := payload.Message
	route, err := s.repo.Route(ctx, a.ID, m.Channel)
	if err != nil || route == nil {
		return err
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return err
	}
	m.Channel, m.CurrentChannelID = route.ChannelID, route.CurrentChannelID
	request := m
	request.User = payload.Requester
	if payload.Command == "create_support" {
		if route.Purpose == "pm" {
			return s.private(ctx, token, request, "Choose a channel configured for Support.")
		}
		return s.receiveSupport(ctx, a, token, route, m, job, true)
	}
	actor, err := s.linkedActor(ctx, a, payload.Requester)
	if err != nil {
		return err
	}
	if actor == nil {
		return s.promptLink(ctx, a, token, request)
	}
	ctx = authorization.WithActor(ctx, actor)
	allowed, err := s.authz.CanAccessModule(ctx, actor, model.ModulePM)
	if err != nil {
		return err
	}
	if !allowed || !s.authz.Can(actor, authorization.PermPMEdit) {
		return s.private(ctx, token, request, "You need permission to edit tasks in Helpin.")
	}
	id, err := s.referencedConversation(ctx, token, request)
	if err != nil {
		return s.private(ctx, token, request, err.Error())
	}
	if id != "" {
		return s.internalSupportAnswer(ctx, a, token, route, request, actor, id, true)
	}
	if route.Purpose == "pm" {
		return s.offerDraft(ctx, a, token, route, request, actor, nil)
	}
	conv, err := s.repo.SupportThread(ctx, a, m.Channel+":"+m.Root())
	if err != nil {
		return err
	}
	if conv == nil {
		return s.private(ctx, token, request, "Create a support conversation for this thread first.")
	}
	return s.internalSupportAnswer(ctx, a, token, route, request, actor, conv.ID, true)
}

func slackCommand(text string) string {
	fields := strings.Fields(strings.ToLower(text))
	if len(fields) > 0 && strings.HasPrefix(fields[0], "<@") {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (s *SlackService) linkView(ctx context.Context, a *model.SupportChannelAccount, v SlackAction, route *model.SlackRoute, m slack.Message) (any, error) {
	actor, err := s.linkedActor(ctx, a, v.User.ID)
	if err != nil {
		return nil, err
	}
	token, err := s.channels.decryptToken(a)
	if err != nil {
		return nil, err
	}
	if actor == nil {
		m.User = v.User.ID
		return map[string]any{}, s.promptLink(ctx, a, token, m)
	}
	meta, err := json.Marshal(model.SlackThread{AccountID: a.ID, Channel: route.ChannelID, Root: m.Root()})
	if err != nil {
		return nil, err
	}
	view := map[string]any{"type": "modal", "callback_id": "helpin_link_task", "private_metadata": string(meta), "title": slackText("Link task"), "submit": slackText("Link"), "close": slackText("Cancel"), "blocks": []any{inputBlock("key", "Task key", "", false)}}
	return map[string]any{}, s.api.Call(ctx, token, "views.open", map[string]any{"trigger_id": v.Trigger, "view": view}, nil)
}

func (s *SlackService) submitLink(ctx context.Context, v SlackAction) (any, error) {
	invalid := func(message string) (any, error) {
		return map[string]any{"response_action": "errors", "errors": map[string]string{"key": message}}, nil
	}
	var thread model.SlackThread
	if err := json.Unmarshal([]byte(v.View.Metadata), &thread); err != nil {
		return nil, ErrChannelInvalid
	}
	a, err := s.channels.accounts.FindLive(ctx, "slack", v.Team.ID)
	if err != nil {
		return nil, err
	}
	if a == nil || a.ID != thread.AccountID {
		return invalid("This installation is unavailable.")
	}
	actor, err := s.linkedActor(ctx, a, v.User.ID)
	if err != nil {
		return nil, err
	}
	if actor == nil || !s.authz.Can(actor, authorization.PermPMEdit) {
		return invalid("You need permission to edit tasks.")
	}
	ctx = authorization.WithActor(ctx, actor)
	pm, err := s.authz.CanAccessModule(ctx, actor, model.ModulePM)
	if err != nil {
		return nil, err
	}
	if !pm {
		return invalid("Project management is unavailable.")
	}
	key := strings.TrimSpace(v.View.State.Values["key"]["value"].Value)
	match := slackTaskKey.FindStringSubmatch(key)
	if len(match) != 3 || match[0] != key || !strings.EqualFold(match[1], s.tasks.GetWorkspaceKey(ctx, a.WorkspaceID)) {
		return invalid("Enter a task key from this workspace.")
	}
	id, err := strconv.Atoi(match[2])
	if err != nil {
		return invalid("Enter a valid task key.")
	}
	task, err := s.tasks.GetByDisplayID(ctx, a.WorkspaceID, id)
	if err != nil || task == nil {
		return invalid("This task is unavailable.")
	}
	route, err := s.repo.Route(ctx, a.ID, thread.Channel)
	if err != nil {
		return nil, err
	}
	if route == nil {
		return invalid("This channel is no longer connected.")
	}
	if route.Purpose == "pm" {
		current, err := s.repo.ThreadTask(ctx, a.ID, thread.Channel, thread.Root)
		if err != nil {
			return nil, err
		}
		if current != "" && current != task.Task.ID {
			return invalid("This thread is already linked to a different task.")
		}
		if current == "" {
			if err := s.repo.LinkTask(ctx, a.ID, thread.Channel, thread.Root, task.Task.ID, actor.UserID); err != nil {
				return nil, err
			}
		}
	} else {
		allowed, err := s.authz.CanAccessModule(ctx, actor, model.ModuleSupport)
		if err != nil {
			return nil, err
		}
		if !allowed || !s.authz.Can(actor, authorization.PermSupportEdit) {
			return invalid("You need permission to edit this support conversation.")
		}
		c, err := s.repo.SupportThread(ctx, a, thread.Channel+":"+thread.Root)
		if err != nil {
			return nil, err
		}
		if c == nil {
			return invalid("Create a support conversation for this thread first.")
		}
		if err = s.channels.inbox.LinkConversationTask(ctx, a.WorkspaceID, c.ID, task.Task.ID, actor.UserID); err != nil {
			return invalid("The conversation or task is unavailable.")
		}
	}
	return map[string]string{"response_action": "clear"}, nil
}
