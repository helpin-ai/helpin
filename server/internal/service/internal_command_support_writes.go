package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Support writes for teammates and automation accounts: status, priority,
// snooze, replies and new conversations. They run through the same inbox
// methods as the web app, so permissions, inbox access, timeline events and
// delivery behave the same.

func (s *InternalCommandService) supportWriteDefinitions() []InternalCommandDefinition {
	return []InternalCommandDefinition{
		{Name: "support.set_conversation_status", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.set_conversation_status"), Execute: s.executeSupportSetConversationStatus},
		{Name: "support.set_conversation_priority", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.set_conversation_priority"), Execute: s.executeSupportSetConversationPriority},
		{Name: "support.snooze_conversation", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.snooze_conversation"), Execute: s.executeSupportSnoozeConversation},
		{Name: "support.unsnooze_conversation", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.unsnooze_conversation"), Execute: s.executeSupportUnsnoozeConversation},
		{Name: "support.reply_to_conversation", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes, Tool: mustCommandToolMetadata("support.reply_to_conversation"), Execute: s.executeSupportReplyToConversation},
		{Name: "support.create_conversation", Module: "support", Mutating: true, SupportedTargetTypes: []string{"workspace"}, Tool: mustCommandToolMetadata("support.create_conversation"), Execute: s.executeSupportCreateConversation},
	}
}

// supportWritableConversation resolves the target and refuses merged or
// inaccessible conversations.
func (s *InternalCommandService) supportWritableConversation(ctx context.Context, meta model.InternalCommandContext, explicit string) (string, context.Context, error) {
	id, err := resolveSupportConversationCommandID(meta, explicit)
	if err != nil {
		return "", nil, err
	}
	if _, err := canonicalActionID(id); err != nil {
		return "", nil, errCommandNotFound("conversation")
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return "", nil, err
	}
	scoped := s.supportOperationalContext(ctx, meta)
	if err := s.supportInboxService.RequireWritableSupportConversation(scoped, meta.WorkspaceID, id); err != nil {
		return "", nil, supportCommandError(err)
	}
	return id, scoped, nil
}

// supportCommandError turns inbox errors the caller can act on into command
// errors with safe messages.
func supportCommandError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrSupportMergeNotFound), strings.HasSuffix(err.Error(), "not found"):
		return errCommandNotFound("conversation")
	case errors.Is(err, repository.ErrSupportMerged):
		return errCommandInput("the conversation was merged into another; act on that conversation instead")
	case errors.Is(err, ErrSupportInvalidPriority), errors.Is(err, ErrSupportInvalidSnooze), errors.Is(err, ErrSupportSnoozeClosed):
		return errCommandInput("%s", err.Error())
	}
	return err
}

func supportConversationWriteResult(conversation *model.SupportConversation) json.RawMessage {
	result := compactSupportConversation(conversation)
	result["snoozed_until"] = conversation.SnoozedUntil
	return mustJSON(result)
}

func (s *InternalCommandService) executeSupportSetConversationStatus(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid input")
	}
	status := model.NormalizeSupportConversationStatus(req.Status)
	switch status {
	case model.SupportConversationStatusOpen, model.SupportConversationStatusWaitingOnCustomer,
		model.SupportConversationStatusResolved, model.SupportConversationStatusSpam:
	default:
		return nil, errCommandInput("status must be open, waiting_on_customer, resolved or spam")
	}
	id, scoped, err := s.supportWritableConversation(ctx, meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	conversation, err := s.supportInboxService.UpdateConversationStatus(scoped, meta.WorkspaceID, id, status, fallbackActor(meta))
	if err != nil {
		return nil, supportCommandError(err)
	}
	return supportConversationWriteResult(conversation), nil
}

func (s *InternalCommandService) executeSupportSetConversationPriority(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Priority       string `json:"priority"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid input")
	}
	id, scoped, err := s.supportWritableConversation(ctx, meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	conversation, err := s.supportInboxService.UpdateConversationPriority(scoped, meta.WorkspaceID, id, strings.ToLower(strings.TrimSpace(req.Priority)), fallbackActor(meta))
	if err != nil {
		return nil, supportCommandError(err)
	}
	return supportConversationWriteResult(conversation), nil
}

func (s *InternalCommandService) executeSupportSnoozeConversation(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Until          string `json:"until"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid input")
	}
	until, err := time.Parse(time.RFC3339, strings.TrimSpace(req.Until))
	if err != nil {
		return nil, errCommandInput("until must be an RFC 3339 time, for example 2026-11-02T09:00:00Z")
	}
	id, scoped, err := s.supportWritableConversation(ctx, meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	conversation, err := s.supportInboxService.SnoozeConversation(scoped, meta.WorkspaceID, id, until, fallbackActor(meta))
	if err != nil {
		return nil, supportCommandError(err)
	}
	return supportConversationWriteResult(conversation), nil
}

func (s *InternalCommandService) executeSupportUnsnoozeConversation(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid input")
	}
	id, scoped, err := s.supportWritableConversation(ctx, meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	conversation, err := s.supportInboxService.UnsnoozeConversation(scoped, meta.WorkspaceID, id, fallbackActor(meta))
	if err != nil {
		return nil, supportCommandError(err)
	}
	return supportConversationWriteResult(conversation), nil
}

// requireTeammateActor refuses customer-visible sends without a person to
// attribute them to.
func requireTeammateActor(meta model.InternalCommandContext) (string, error) {
	actor := strings.TrimSpace(meta.ActorID)
	if actor == "" {
		return "", errCommandInput("replies need a teammate or automation account to send them")
	}
	return actor, nil
}

func (s *InternalCommandService) executeSupportReplyToConversation(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Content        string `json:"content"`
		DeliveryMode   string `json:"delivery_mode"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid input")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errCommandInput("content is required")
	}
	mode := strings.TrimSpace(req.DeliveryMode)
	switch mode {
	case "", model.SupportDeliveryChatOnly, model.SupportDeliveryChatAndEmail, model.SupportDeliveryEmailOnly:
	default:
		return nil, errCommandInput("delivery_mode must be chat_only, chat_and_email or email_only")
	}
	actor, err := requireTeammateActor(meta)
	if err != nil {
		return nil, err
	}
	id, scoped, err := s.supportWritableConversation(ctx, meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	message, err := s.supportInboxService.CreateConversationMessage(scoped, meta.WorkspaceID, id,
		model.CreateMessageRequest{Content: content, MessageType: "reply", DeliveryMode: mode}, "user", &actor, nil, nil)
	if err != nil {
		return nil, supportReplyError(err)
	}
	return mustJSON(map[string]any{
		"status": "sent", "conversation_id": id, "message_id": message.ID, "delivery_mode": message.DeliveryMode(), "created_at": message.CreatedAt,
	}), nil
}

// supportReplyError keeps validation messages and hides internal failures.
func supportReplyError(err error) error {
	if mapped := supportCommandError(err); mapped != err {
		return mapped
	}
	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		return err
	}
	if errors.Is(err, ErrSupportTranslation) {
		return errCommandInput("the reply could not be translated")
	}
	// The inbox explains why a send is refused (no chat session, recipient
	// not confirmed, email not set up); the web app shows the same text.
	if message := err.Error(); !strings.Contains(strings.ToLower(message), "sql") && !strings.Contains(message, ": ") {
		return errCommandInput("%s", message)
	}
	return fmt.Errorf("send support reply: %w", err)
}

func (s *InternalCommandService) executeSupportCreateConversation(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		CustomerEmail string  `json:"customer_email"`
		CustomerName  *string `json:"customer_name"`
		Subject       string  `json:"subject"`
		Content       string  `json:"content"`
		InboxID       *string `json:"inbox_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid input")
	}
	address, err := mail.ParseAddress(strings.TrimSpace(req.CustomerEmail))
	if err != nil || address.Name != "" {
		return nil, errCommandInput("customer_email must be an email address")
	}
	subject, content := strings.TrimSpace(req.Subject), strings.TrimSpace(req.Content)
	if subject == "" || content == "" {
		return nil, errCommandInput("subject and content are required")
	}
	actor, err := requireTeammateActor(meta)
	if err != nil {
		return nil, err
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	email := address.Address
	created, err := s.supportInboxService.CreateConversationWithMessage(s.supportOperationalContext(ctx, meta), model.CreateConversationWithMessageRequest{
		WorkspaceID: meta.WorkspaceID, MailboxID: normalizeSupportInboxID(req.InboxID), Subject: subject,
		CustomerName: normalizeOptionalCommandString(req.CustomerName), CustomerEmail: &email,
		Channels: []string{"email"}, Content: content,
	}, actor)
	if err != nil {
		return nil, supportReplyError(err)
	}
	result := compactSupportConversation(created.Conversation)
	result["message_id"] = created.Message.ID
	return mustJSON(result), nil
}
