package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) registerSupportOperationalCommands() {
	definitions := []InternalCommandDefinition{
		{Name: "support.list_conversations", Module: "support", SupportedTargetTypes: []string{"workspace", "conversation", "support_conversation"}, Tool: mustCommandToolMetadata("support.list_conversations"), Execute: s.executeSupportListConversations},
		{Name: "support.get_conversation", Module: "support", SupportedTargetTypes: []string{"workspace", "conversation", "support_conversation"}, Tool: mustCommandToolMetadata("support.get_conversation"), Execute: s.executeSupportGetConversation},
		{Name: "support.list_tags", Module: "support", SupportedTargetTypes: []string{"workspace", "conversation", "support_conversation"}, Tool: mustCommandToolMetadata("support.list_tags"), Execute: s.executeSupportListTags},
		{Name: "support.list_inboxes", Module: "support", SupportedTargetTypes: []string{"workspace", "conversation", "support_conversation"}, Tool: mustCommandToolMetadata("support.list_inboxes"), Execute: s.executeSupportListInboxes},
		{Name: "support.list_assignees", Module: "support", SupportedTargetTypes: []string{"conversation", "support_conversation"}, Tool: mustCommandToolMetadata("support.list_assignees"), Execute: s.executeSupportListAssignees},
		{Name: "support.assign_conversation", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.assign_conversation"), Execute: s.executeSupportAssignConversation},
		{Name: "support.move_conversation", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.move_conversation"), Execute: s.executeSupportMoveConversation},
		{Name: "support.add_conversation_tag", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.add_conversation_tag"), Execute: s.executeSupportAddConversationTag},
		{Name: "support.remove_conversation_tag", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.remove_conversation_tag"), Execute: s.executeSupportRemoveConversationTag},
		{Name: "support.link_conversation_task", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.link_conversation_task"), Execute: s.executeSupportLinkConversationTask},
		{Name: "support.link_conversation_contact", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.link_conversation_contact"), Execute: s.executeSupportLinkConversationContact},
		{Name: "support.update_conversation_subject", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.update_conversation_subject"), Execute: s.executeSupportUpdateConversationSubject},
	}
	for _, def := range definitions {
		s.register(def)
	}
}

func (s *InternalCommandService) requireSupportOperationalServices(requireTags bool) error {
	if s.supportInboxService == nil {
		return fmt.Errorf("support operational services are not configured")
	}
	if requireTags && s.supportTagService == nil {
		return fmt.Errorf("support operational services are not configured")
	}
	return nil
}

func (s *InternalCommandService) executeSupportListConversations(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	var req struct {
		Status   string  `json:"status"`
		Priority string  `json:"priority"`
		InboxID  *string `json:"inbox_id"`
		Query    string  `json:"query"`
		Limit    int     `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse support conversation list input: %w", err)
	}
	limit, err := normalizeOperationalLimit(req.Limit)
	if err != nil {
		return nil, err
	}
	mailboxID := normalizeSupportInboxID(req.InboxID)
	response, err := s.supportInboxService.ListConversationsWithMeta(s.supportOperationalContext(ctx, meta), SupportConversationListParams{WorkspaceID: meta.WorkspaceID, UserID: meta.ActorID, Status: req.Status, Priority: req.Priority, Search: strings.TrimSpace(req.Query), MailboxID: mailboxID, Pagination: model.PMPagination{Page: 1, PerPage: limit}})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(response.Data))
	for i := range response.Data {
		items = append(items, compactSupportConversation(&response.Data[i]))
	}
	return mustJSON(map[string]any{"conversations": items, "total": response.Total}), nil
}

func (s *InternalCommandService) executeSupportGetConversation(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse support conversation input: %w", err)
	}
	id, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	conversation, err := s.supportInboxService.GetConversation(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, id)
	if err != nil {
		return nil, err
	}
	return mustJSON(compactSupportConversation(conversation)), nil
}

func (s *InternalCommandService) executeSupportListTags(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requireSupportOperationalServices(true); err != nil {
		return nil, err
	}
	var req struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	limit, err := normalizeOperationalLimit(req.Limit)
	if err != nil {
		return nil, err
	}
	tags, err := s.supportTagService.List(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if len(tags) > limit {
		tags = tags[:limit]
	}
	return mustJSON(map[string]any{"tags": tags}), nil
}

func (s *InternalCommandService) executeSupportListInboxes(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	var req struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	limit, err := normalizeOperationalLimit(req.Limit)
	if err != nil {
		return nil, err
	}
	response, err := s.supportInboxService.ListInboxScopes(s.supportOperationalContext(ctx, meta), meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	items := append([]model.SupportInboxScope{response.SharedInbox}, response.Mailboxes...)
	if len(items) > limit {
		items = items[:limit]
	}
	return mustJSON(map[string]any{"inboxes": items}), nil
}

func (s *InternalCommandService) executeSupportListAssignees(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	var req struct {
		ConversationID string `json:"conversation_id"`
		Limit          int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	id, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	limit, err := normalizeOperationalLimit(req.Limit)
	if err != nil {
		return nil, err
	}
	members, err := s.supportInboxService.ListConversationAssignableUsers(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, id)
	if err != nil {
		return nil, err
	}
	if len(members) > limit {
		members = members[:limit]
	}
	items := make([]map[string]any, 0, len(members))
	for _, member := range members {
		items = append(items, map[string]any{"member_id": member.ID, "user_id": member.UserID, "display_name": member.DisplayName, "role": member.Role, "status": member.Status})
	}
	return mustJSON(map[string]any{"assignees": items}), nil
}

func (s *InternalCommandService) executeSupportAssignConversation(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string  `json:"conversation_id"`
		AssigneeUserID *string `json:"assignee_user_id"`
		Clear          bool    `json:"clear"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	id, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	if err := validateClearPair(req.Clear, req.AssigneeUserID != nil, "clear", "assignee_user_id"); err != nil {
		return nil, err
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	assignee := normalizeOptionalCommandString(req.AssigneeUserID)
	if req.Clear {
		assignee = nil
	}
	if err := s.supportInboxService.AssignConversationUser(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, id, assignee, fallbackActor(meta)); err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"conversation_id": id, "assignee_user_id": assignee}), nil
}

func (s *InternalCommandService) executeSupportMoveConversation(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		InboxID        string `json:"inbox_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	id, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.InboxID) == "" {
		return nil, fmt.Errorf("inbox_id is required")
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	inboxID := normalizeSupportInboxID(&req.InboxID)
	conversation, err := s.supportInboxService.MoveConversation(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, id, inboxID, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(compactSupportConversation(conversation)), nil
}

func (s *InternalCommandService) executeSupportAddConversationTag(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	return s.executeSupportConversationTag(ctx, meta, input, true)
}
func (s *InternalCommandService) executeSupportRemoveConversationTag(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	return s.executeSupportConversationTag(ctx, meta, input, false)
}
func (s *InternalCommandService) executeSupportConversationTag(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage, add bool) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		TagID          string `json:"tag_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	id, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.TagID) == "" {
		return nil, fmt.Errorf("tag_id is required")
	}
	if err := s.requireSupportOperationalServices(true); err != nil {
		return nil, err
	}
	if _, err := s.supportInboxService.GetConversation(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, id); err != nil {
		return nil, err
	}
	if add {
		err = s.supportTagService.AddConversationTag(ctx, meta.WorkspaceID, id, strings.TrimSpace(req.TagID), fallbackActor(meta))
	} else {
		err = s.supportTagService.RemoveConversationTag(ctx, meta.WorkspaceID, id, strings.TrimSpace(req.TagID), fallbackActor(meta))
	}
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"conversation_id": id, "tag_id": strings.TrimSpace(req.TagID), "linked": add}), nil
}

func (s *InternalCommandService) executeSupportLinkConversationTask(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string  `json:"conversation_id"`
		TaskID         *string `json:"task_id"`
		Clear          bool    `json:"clear"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	id, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	if err := validateClearPair(req.Clear, req.TaskID != nil, "clear", "task_id"); err != nil {
		return nil, err
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	taskID := normalizeOptionalCommandString(req.TaskID)
	if !req.Clear && taskID == nil {
		return nil, fmt.Errorf("task_id or clear is required")
	}
	if taskID != nil {
		if s.taskService == nil {
			return nil, fmt.Errorf("task service is not configured")
		}
		detail, loadErr := s.taskService.GetByID(ctx, *taskID)
		if loadErr != nil || detail == nil || detail.Task.WorkspaceID != meta.WorkspaceID {
			return nil, fmt.Errorf("task not found")
		}
	}
	if err := s.supportInboxService.UpdateConversationLinkedTask(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, id, taskID, fallbackActor(meta)); err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"conversation_id": id, "task_id": taskID}), nil
}

func (s *InternalCommandService) executeSupportLinkConversationContact(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string  `json:"conversation_id"`
		ContactID      *string `json:"contact_id"`
		Clear          bool    `json:"clear"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	id, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	if err := validateClearPair(req.Clear, req.ContactID != nil, "clear", "contact_id"); err != nil {
		return nil, err
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	contactID := normalizeOptionalCommandString(req.ContactID)
	if !req.Clear && contactID == nil {
		return nil, fmt.Errorf("contact_id or clear is required")
	}
	conversation, err := s.supportInboxService.UpdateConversationCRMContact(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, id, contactID, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(compactSupportConversation(conversation)), nil
}

func (s *InternalCommandService) executeSupportUpdateConversationSubject(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Subject        string `json:"subject"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	id, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	conversation, err := s.supportInboxService.UpdateConversationSubject(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, id, strings.TrimSpace(req.Subject), fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(compactSupportConversation(conversation)), nil
}

func resolveSupportConversationCommandID(meta model.InternalCommandContext, explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	targetType := strings.TrimSpace(meta.TargetType)
	if targetType == "conversation" || targetType == "support_conversation" {
		targetID := strings.TrimSpace(meta.TargetID)
		if explicit != "" && targetID != "" && explicit != targetID {
			return "", fmt.Errorf("conversation_id conflicts with the current support conversation target")
		}
		if explicit == "" {
			explicit = targetID
		}
	}
	if explicit == "" {
		return "", fmt.Errorf("conversation_id is required")
	}
	return explicit, nil
}

func (s *InternalCommandService) supportOperationalContext(ctx context.Context, meta model.InternalCommandContext) context.Context {
	if authorization.GetActor(ctx) != nil {
		return ctx
	}
	if strings.TrimSpace(meta.ActorRole) == "" {
		return ctx
	}
	return authorization.WithActor(ctx, internalCommandActor(meta))
}

func normalizeSupportInboxID(value *string) *string {
	value = normalizeOptionalCommandString(value)
	if value != nil && strings.EqualFold(*value, "shared") {
		return nil
	}
	return value
}
func compactSupportConversation(item *model.SupportConversation) map[string]any {
	return map[string]any{"conversation_id": item.ID, "display_id": item.DisplayID, "subject": item.Subject, "status": item.Status, "priority": item.Priority, "channel": item.Channel, "mailbox_id": item.MailboxID, "assigned_user_id": item.AssignedUserID, "assigned_agent_id": item.AssignedAgentID, "linked_task_id": item.LinkedTaskID, "crm_contact_id": item.CRMContactID, "crm_company_id": item.CRMCompanyID, "customer_name": item.CustomerName, "customer_email": item.CustomerEmail, "tags": item.Tags, "updated_at": item.UpdatedAt}
}
