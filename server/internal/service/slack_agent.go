package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type slackAskAgent interface {
	SendMessage(context.Context, string, string, string, model.SendDockChatMessageRequest) (*model.DockChatDetail, error)
}

const SlackAgentReplyJob = "slack_agent_reply"

var errSlackAgentPending = errors.New("waiting for Slack agent answer")

func slackDirect(m slack.Message) bool {
	return m.Type == "message" && m.ChannelType == "im" && strings.HasPrefix(m.Channel, "D")
}

func slackDraftRequest(text, handle string) bool {
	q := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(text, "<@"+handle+">", "")))
	return (strings.HasPrefix(q, "create ") || strings.HasPrefix(q, "draft ") || strings.HasPrefix(q, "make ") || strings.HasPrefix(q, "file ")) && (strings.Contains(q, "task") || strings.Contains(q, "bug"))
}

func slackHelp(text, handle string) bool {
	q := strings.ToLower(strings.Trim(strings.TrimSpace(strings.ReplaceAll(text, "<@"+handle+">", "")), " ?!."))
	return q == "" || q == "help" || q == "what can you do" || q == "how can you help"
}

type slackAccess struct{ PM, Support bool }

func slackMemberAccess(ctx context.Context, authz *authorization.AuthzService, actor *authorization.Actor) (slackAccess, error) {
	pm, err := authz.CanAccessModule(ctx, actor, model.ModulePM)
	if err != nil {
		return slackAccess{}, err
	}
	support, err := authz.CanAccessModule(ctx, actor, model.ModuleSupport)
	return slackAccess{pm && authz.Can(actor, authorization.PermPMRead), support && authz.Can(actor, authorization.PermSupportRead)}, err
}

func (access slackAccess) chatID(account string, m slack.Message, actor *authorization.Actor) string {
	teams := actor.TeamIDs()
	sort.Strings(teams)
	return inboundStableID(fmt.Sprintf("slack-chat:%s:%s:%s:%s:%s:%s:%s:%t:%t", account, m.Channel, m.Root(), actor.UserID, actor.WorkspaceMemberID, actor.Role, strings.Join(teams, ","), access.PM, access.Support))
}

func (access slackAccess) help(edit, ai bool) string {
	var sections []string
	if access.PM {
		tasks := "*Catch up on tasks*\n• “What’s the latest on ENG-123?” (Use your task’s key.)"
		if ai {
			tasks += "\n• “Which tasks are blocked on my team?”"
		}
		sections = append(sections, tasks)
	}
	if access.Support {
		example := "Paste a Helpin support conversation link and ask for its status."
		if ai {
			example = "Paste a Helpin support conversation link and ask, “What did the customer report, and what’s the next step?”"
		}
		sections = append(sections, "*Get support context*\n"+example)
	}
	if access.PM && edit {
		where := "In a channel with @Helpin"
		if access.Support {
			where = "In a channel with @Helpin or a linked support thread"
		}
		sections = append(sections, "*Turn a discussion into a task*\n"+where+", mention @Helpin and say, “Create a task from this thread.” Review the draft before saving.")
	}
	if len(sections) == 0 {
		return "Your Helpin account needs access to PM or Support to ask about workspace records."
	}
	if ai {
		sections = append(sections, "For a follow-up, mention @Helpin in the same channel thread or reply directly in a DM.")
	}
	return strings.Join(sections, "\n\n")
}

func slackUnlinkedWelcome(ai bool) string {
	const intro = "👋 *Welcome to Helpin!*\nHere are a few things you can try, depending on your workspace access."
	const pairing = "Send your first request here and I’ll help you link your account."
	return strings.Join([]string{intro, (slackAccess{PM: true, Support: true}).help(true, ai), pairing}, "\n\n")
}

func (s *SlackService) welcome(ctx context.Context, a *model.SupportChannelAccount, token string, m slack.Message) error {
	actor, err := s.requestingActor(ctx, a, token, m.User)
	if err != nil {
		return err
	}
	if actor == nil {
		return s.sendPrivate(ctx, token, m, slackUnlinkedWelcome(!a.AIRepliesDisabled), true)
	}
	access, err := slackMemberAccess(ctx, s.authz, actor)
	if err != nil {
		return err
	}
	return s.sendPrivate(ctx, token, m, "👋 *Welcome to Helpin!*\n\n"+access.help(s.authz.Can(actor, authorization.PermPMEdit), !a.AIRepliesDisabled), true)
}

func isSlackDockRun(run *model.AgentRun) bool {
	if run == nil || run.DockChatID == nil || run.TargetType != "workspace" || run.TargetID != run.WorkspaceID {
		return false
	}
	var input model.AgentRunInputPayload
	if json.Unmarshal(run.Input, &input) != nil || input.Trigger == nil || input.Trigger.TriggerType != dockChatTriggerType || input.ExecutionEnabled || len(input.AllowedTools) == 0 {
		return false
	}
	var c struct {
		SlackThread bool   `json:"slack_thread"`
		ChatID      string `json:"dock_chat_id"`
	}
	return json.Unmarshal(input.Trigger.Context, &c) == nil && c.SlackThread && c.ChatID == *run.DockChatID
}

var slackPMReadTools = []string{"list_workspace_members", "list_tasks", "get_task", "get_task_context", "list_task_checklist", "list_epics", "get_epic", "list_epic_tasks", "list_sprints", "list_sprint_tasks", "list_workspace_teams", "list_team_workflows_with_stages"}

var slackPMWriteTools = []string{"update_task", "update_task_state"}
var slackSupportWriteTools = []string{"assign_support_conversation", "set_support_conversation_status", "set_support_conversation_priority"}

func (s *DockChatService) slackAgentTools(ctx context.Context, chat *model.DockChat, userID string, agent *model.Agent) ([]string, error) {
	actor, err := s.authz.ResolveActor(ctx, chat.WorkspaceID, userID)
	if err != nil {
		return nil, err
	}
	access, err := slackMemberAccess(ctx, s.authz, actor)
	if err != nil {
		return nil, err
	}
	projection := *agent
	projection.AllowedTools = mustJSONStringSlice(appendPresetTools(parseJSONStringSlice(agent.AllowedTools), slackSupportWriteTools))
	scoped, err := s.scopedChatTools(ctx, chat.WorkspaceID, userID, &projection)
	if err != nil {
		return nil, err
	}
	var allowed []string
	for _, tool := range scoped {
		pm := access.PM && (slices.Contains(slackPMReadTools, tool) || (s.authz.Can(actor, authorization.PermPMEdit) && slices.Contains(slackPMWriteTools, tool)))
		support := access.Support && (slices.Contains(safeSupportDiscoveryToolAliases, tool) || (s.authz.Can(actor, authorization.PermSupportEdit) && slices.Contains(slackSupportWriteTools, tool)))
		if pm || support {
			allowed = append(allowed, tool)
		}
	}
	if len(allowed) == 0 {
		return nil, errors.New("no permitted Slack agent tools")
	}
	return allowed, nil
}

func (s *SlackService) directAgent(ctx context.Context, a *model.SupportChannelAccount, token string, m slack.Message) error {
	actor, err := s.requestingActor(ctx, a, token, m.User)
	if err != nil {
		return err
	}
	if actor == nil {
		if slackHelp(m.Text, a.Handle) {
			return s.sendPrivate(ctx, token, m, slackUnlinkedWelcome(!a.AIRepliesDisabled), true)
		}
		return s.promptLink(ctx, a, token, m)
	}
	ctx = authorization.WithActor(ctx, actor)
	route := &model.SlackRoute{Purpose: "pm", ChannelID: m.Channel, CurrentChannelID: m.Channel}

	if !slackHelp(m.Text, a.Handle) && (m.ThreadTS != "" || slackSupportLink.MatchString(m.Text) || slackTaskLink.MatchString(m.Text) || slackTaskKey.MatchString(m.Text)) {
		return s.processPM(ctx, a, token, route, m)
	}
	if slackDraftRequest(m.Text, a.Handle) {
		return s.offerDraft(ctx, a, token, route, m, actor, nil)
	}
	return s.agentQuestion(ctx, a, token, m, actor)
}

type slackAgentResult struct {
	Message                slack.Message
	ChatID, UserID, TurnID string
	NativeProgress         bool
	ProgressTS             string
	LastPhase              string
	Records                []slackRecord `json:",omitempty"`
}

type slackRecord struct{ Kind, ID, Label, URL string }

func (s *SlackService) agentQuestion(ctx context.Context, a *model.SupportChannelAccount, token string, m slack.Message, actor *authorization.Actor, records ...slackRecord) error {
	access, err := slackMemberAccess(ctx, s.authz, actor)
	if err != nil {
		return err
	}
	if slackHelp(m.Text, a.Handle) || access == (slackAccess{}) {
		return s.reply(ctx, token, m, access.help(s.authz.Can(actor, authorization.PermPMEdit), !a.AIRepliesDisabled), true)
	}
	if a.AIRepliesDisabled || s.dock == nil {
		return s.sendPrivate(ctx, token, m, "Workspace questions require Slack AI replies to be enabled in Helpin.\n\n"+access.help(s.authz.Can(actor, authorization.PermPMEdit), false), true)
	}
	chatID := access.chatID(a.ID, m, actor)
	content := strings.TrimSpace(strings.ReplaceAll(m.Text, "<@"+a.Handle+">", ""))
	if err = s.repo.EnsureAgentChat(ctx, a.WorkspaceID, actor.UserID, chatID, "Slack · "+truncate(content, 80)); err != nil {
		return err
	}
	clientID := inboundStableID("slack-turn:" + chatID + ":" + m.TS)
	ctx, progress := s.startProgress(ctx, token, m, "")
	queued := false
	defer func() {
		if !queued {
			s.finishProgress(ctx, token, m, progress)
		}
	}()
	req := model.SendDockChatMessageRequest{ClientMessageID: clientID, Content: content}
	if len(records) > 0 {
		req.PageContext = map[string]any{"referenced_records": records}
	}
	ctx = model.WithSlackSource(ctx, s.sourceURL(ctx, token, m.Destination(), m.Root()))
	detail, err := s.dock.SendMessage(ctx, a.WorkspaceID, actor.UserID, chatID, req)
	if err != nil {
		s.channels.logger.WarnContext(ctx, "Slack Ask Agent turn could not start", "workspace_id", a.WorkspaceID, "error", err)
		msg := slackFailureReason(err.Error())
		if strings.Contains(err.Error(), "still working") {
			msg = "I’m still answering your previous question. Try your follow-up after my reply."
		}
		if errors.Is(err, model.ErrAIAllowanceExhausted) || errors.Is(err, model.ErrBillingWorkspaceLocked) || errors.Is(err, model.ErrExtraAIUsageDisabled) {
			msg = "Your workspace needs more AI usage. A workspace admin can review billing in Helpin."
		}
		return s.private(ctx, token, m, msg)
	}
	if detail.AcceptedMessage == nil {
		return errors.New("missing accepted Slack chat turn")
	}
	m.Text = ""
	m.Files = nil
	p, err := json.Marshal(slackAgentResult{Message: m, ChatID: chatID, UserID: actor.UserID, TurnID: detail.AcceptedMessage.ID, ProgressTS: progress.TS, NativeProgress: progress.Native, Records: records})
	if err != nil {
		return err
	}
	subject := model.SupportChannelSubjectKey(a.ID, m.User)
	err = s.channels.jobs.Create(ctx, &model.SupportInboundJob{ID: inboundStableID("slack-result:" + clientID), WorkspaceID: &a.WorkspaceID, ChannelAccountID: &a.ID, Kind: SlackAgentReplyJob, SubjectKey: &subject, Payload: string(p), Status: "pending", AvailableAt: s.channels.now()})
	queued = err == nil
	return err
}

func (s *SlackService) deliverAgentAnswer(ctx context.Context, job *model.SupportInboundJob) error {
	var p slackAgentResult
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return err
	}
	a, err := s.channels.accounts.GetForWebhook(ctx, "slack", *job.ChannelAccountID)
	if err != nil || a == nil {
		return err
	}
	finish := func(answer string, blocks ...any) error {
		token, err := s.channels.decryptToken(a)
		if err != nil {
			return err
		}
		progressCtx := context.WithValue(ctx, slackProgressKey{}, &slackProgress{TS: p.ProgressTS})
		if err = s.reply(progressCtx, token, p.Message, truncate(answer, 3900), false, blocks...); err != nil {
			return err
		}
		s.sessionStatus(ctx, token, p.Message, "active")
		return nil
	}
	if a.AIRepliesDisabled {
		return finish("AI replies were disabled while I was working. A workspace admin can enable them in Helpin’s Slack settings.")
	}
	actor, err := s.linkedActor(ctx, a, p.Message.User)
	if errors.Is(err, authorization.ErrNotAMember) || errors.Is(err, authorization.ErrMembershipInactive) || errors.Is(err, authorization.ErrMembershipRevoked) || errors.Is(err, authorization.ErrMembershipPending) {
		return finish("Your Helpin access changed while I was working. Check your workspace access before asking again.")
	}
	if err != nil {
		return err
	}
	if actor == nil || actor.UserID != p.UserID {
		return finish("Your Helpin access changed while I was working. Check your workspace access before asking again.")
	}
	access, err := slackMemberAccess(ctx, s.authz, actor)
	if err != nil {
		return err
	}
	if access.chatID(a.ID, p.Message, actor) != p.ChatID {
		return finish("Your Helpin access changed while I was working. Check your workspace access before asking again.")
	}
	if !slackDirect(p.Message) {
		route, err := s.repo.EffectiveRoute(ctx, a.ID, p.Message.Channel)
		if err != nil {
			return err
		}
		if route == nil {
			return finish("This channel’s Helpin settings changed. Ask me again in a direct message.")
		}
		p.Message.CurrentChannelID = route.CurrentChannelID
	}
	answer, err := s.repo.AgentAnswer(ctx, a.WorkspaceID, p.ChatID, p.TurnID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(answer) != "" {
		blocks := []any{slackMarkdown(answer)}
		recordCtx := authorization.WithActor(ctx, actor)
		var buttons []any
		for _, record := range p.Records {
			switch record.Kind {
			case "support_conversation":
				conv, err := s.channels.inbox.loadConversationAccessible(recordCtx, a.WorkspaceID, record.ID)
				if err != nil || conv == nil || conv.AnonymizedAt != nil {
					return finish("This conversation is no longer available to you in Helpin.")
				}
			case "task", "task_created_or_linked_in_thread":
				task, err := s.tasks.GetByID(recordCtx, record.ID)
				if err != nil || task == nil || task.Task.WorkspaceID != a.WorkspaceID {
					return finish("This task is no longer available to you in Helpin.")
				}
			default:
				continue
			}
			buttons = append(buttons, map[string]any{"type": "button", "text": slackText("Open " + record.Label), "url": record.URL, "action_id": "open_" + record.Kind})
		}
		if len(buttons) > 0 {
			blocks = append(blocks, slackButtons(buttons...))
		}
		return finish(answer, blocks...)
	}
	run, err := s.repo.AgentTurnStatus(ctx, a.WorkspaceID, p.ChatID, p.TurnID)
	if err != nil {
		return err
	}
	switch {
	case run.Status == model.AgentRunStatusFailed:
		answer = slackFailureReason(derefString(run.ErrorMessage))
	case run.Status == model.AgentRunStatusPaused:
		answer = "This request paused for input. Open the agent run in Helpin to review what it needs."
	case run.Status == model.AgentRunStatusCancelled:
		answer = "This request was cancelled. Send it again to start a new run."
	case s.channels.now().Sub(job.CreatedAt) < 5*time.Minute:
		if err := s.updateReadProgress(ctx, a, &p, job); err != nil {
			s.channels.logger.WarnContext(ctx, "Slack activity update unavailable", "error", err)
		}
		return errSlackAgentPending
	default:
		answer = "I didn’t receive a final answer within five minutes. If you requested a change, check the record in Helpin before retrying."
	}
	return finish(answer)
}

// Report recognized causes without copying runtime diagnostics or customer data into Slack.
func slackFailureReason(detail string) string {
	detail = strings.ToLower(detail)
	switch {
	case strings.Contains(detail, "requires tool") && strings.Contains(detail, "skill"):
		return "The Slack agent’s configuration is missing a required tool. Ask a workspace admin to contact Helpin support."
	case strings.Contains(detail, "rate limit") || strings.Contains(detail, "rate_limit"):
		return "The AI service is rate-limited. Wait a moment, then try again."
	case strings.Contains(detail, "deadline exceeded") || strings.Contains(detail, "timed out"):
		return "The agent timed out before it could answer. Try again with a narrower question."
	case strings.Contains(detail, "ai allowance exhausted") || strings.Contains(detail, "extra ai usage disabled"):
		return "Your workspace has reached its AI usage limit. Ask a workspace admin to review billing in Helpin."
	default:
		return "The agent stopped before it could answer. Try again. If it keeps happening, share this thread with Helpin support."
	}
}
