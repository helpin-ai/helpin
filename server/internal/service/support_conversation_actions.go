package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var (
	// ErrSupportActionsInvalid wraps a validation message for a set of actions.
	ErrSupportActionsInvalid = errors.New("invalid conversation actions")
	// ErrSupportMacroWithoutActions means the shortcut only inserts text.
	ErrSupportMacroWithoutActions = errors.New("this shortcut has no actions")
)

const supportMaxSnoozeMinutes = 365 * 24 * 60

// SupportConversationActionService applies macro and bulk actions through
// the same service methods as the individual inbox controls, so access
// checks, timeline events and realtime updates stay identical.
type SupportConversationActionService struct {
	inbox *SupportInboxService
	tags  *SupportTagService
}

func NewSupportConversationActionService(inbox *SupportInboxService, tags *SupportTagService) *SupportConversationActionService {
	return &SupportConversationActionService{inbox: inbox, tags: tags}
}

func invalidSupportActions(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSupportActionsInvalid, fmt.Sprintf(format, args...))
}

func normalizeActionIDs(values []string, label string) ([]string, error) {
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(result, value) {
			continue
		}
		if _, err := uuid.Parse(value); err != nil {
			return nil, invalidSupportActions("%s %q is not an ID", label, value)
		}
		result = append(result, value)
	}
	return result, nil
}

// NormalizeSupportConversationActions validates and tidies a set of actions.
func NormalizeSupportConversationActions(actions model.SupportConversationActions) (model.SupportConversationActions, error) {
	if actions.Status != nil {
		status := model.NormalizeSupportConversationStatus(*actions.Status)
		switch status {
		case model.SupportConversationStatusOpen, model.SupportConversationStatusWaitingOnCustomer, model.SupportConversationStatusResolved:
			actions.Status = &status
		default:
			return actions, invalidSupportActions("status must be open, waiting_on_customer or resolved")
		}
	}
	if actions.Priority != nil {
		priority := strings.ToLower(strings.TrimSpace(*actions.Priority))
		if !model.IsValidSupportConversationPriority(priority) {
			return actions, invalidSupportActions("priority must be low, medium, high or urgent")
		}
		actions.Priority = &priority
	}
	if actions.AssignUserID != nil {
		assignee := strings.TrimSpace(*actions.AssignUserID)
		if assignee != "" {
			if _, err := uuid.Parse(assignee); err != nil {
				return actions, invalidSupportActions("assignee is not a user ID")
			}
		}
		actions.AssignUserID = &assignee
	}
	if actions.MailboxID != nil {
		mailbox := strings.TrimSpace(*actions.MailboxID)
		if mailbox == "" {
			mailbox = "shared"
		}
		if mailbox != "shared" {
			if _, err := uuid.Parse(mailbox); err != nil {
				return actions, invalidSupportActions("inbox is not an ID")
			}
		}
		actions.MailboxID = &mailbox
	}
	var err error
	if actions.AddTagIDs, err = normalizeActionIDs(actions.AddTagIDs, "tag"); err != nil {
		return actions, err
	}
	if actions.RemoveTagIDs, err = normalizeActionIDs(actions.RemoveTagIDs, "tag"); err != nil {
		return actions, err
	}
	for _, id := range actions.AddTagIDs {
		if slices.Contains(actions.RemoveTagIDs, id) {
			return actions, invalidSupportActions("a tag cannot be added and removed at once")
		}
	}
	if actions.SnoozeMinutes != nil {
		if *actions.SnoozeMinutes < 1 || *actions.SnoozeMinutes > supportMaxSnoozeMinutes {
			return actions, invalidSupportActions("snooze must be between 1 minute and 1 year")
		}
		if actions.Status != nil && *actions.Status == model.SupportConversationStatusResolved {
			return actions, invalidSupportActions("a conversation cannot be resolved and snoozed at once")
		}
	}
	if len(actions.AddTagIDs) == 0 {
		actions.AddTagIDs = nil
	}
	if len(actions.RemoveTagIDs) == 0 {
		actions.RemoveTagIDs = nil
	}
	return actions, nil
}

// Apply runs the actions on one conversation in a fixed order: inbox,
// assignee, priority, tags, status, snooze. It stops at the first failure
// and reports what was applied before it.
func (s *SupportConversationActionService) Apply(ctx context.Context, workspaceID, conversationID string, actions model.SupportConversationActions, actorID string) model.SupportActionResult {
	result := model.SupportActionResult{ConversationID: conversationID, Applied: []string{}}
	fail := func(err error) model.SupportActionResult {
		result.Error = supportActionErrorMessage(err)
		return result
	}
	if err := s.inbox.RequireWritableSupportConversation(ctx, workspaceID, conversationID); err != nil {
		return fail(err)
	}
	if actions.MailboxID != nil {
		var mailboxID *string
		if *actions.MailboxID != "shared" {
			mailboxID = actions.MailboxID
		}
		if _, err := s.inbox.MoveConversation(ctx, workspaceID, conversationID, mailboxID, actorID); err != nil {
			return fail(err)
		}
		result.Applied = append(result.Applied, "inbox")
	}
	if actions.AssignUserID != nil {
		var assignee *string
		if *actions.AssignUserID != "" {
			assignee = actions.AssignUserID
		}
		if err := s.inbox.AssignConversationUser(ctx, workspaceID, conversationID, assignee, actorID); err != nil {
			return fail(err)
		}
		result.Applied = append(result.Applied, "assignee")
	}
	if actions.Priority != nil {
		if _, err := s.inbox.UpdateConversationPriority(ctx, workspaceID, conversationID, *actions.Priority, actorID); err != nil {
			return fail(err)
		}
		result.Applied = append(result.Applied, "priority")
	}
	if len(actions.AddTagIDs) > 0 || len(actions.RemoveTagIDs) > 0 {
		if s.tags == nil {
			return fail(errors.New("tags are unavailable"))
		}
		for _, tagID := range actions.AddTagIDs {
			if err := s.tags.AddConversationTag(ctx, workspaceID, conversationID, tagID, actorID); err != nil {
				return fail(err)
			}
		}
		for _, tagID := range actions.RemoveTagIDs {
			if err := s.tags.RemoveConversationTag(ctx, workspaceID, conversationID, tagID, actorID); err != nil {
				return fail(err)
			}
		}
		result.Applied = append(result.Applied, "tags")
	}
	if actions.Status != nil {
		if _, err := s.inbox.UpdateConversationStatus(ctx, workspaceID, conversationID, *actions.Status, actorID); err != nil {
			return fail(err)
		}
		result.Applied = append(result.Applied, "status")
	}
	if actions.SnoozeMinutes != nil {
		until := time.Now().Add(time.Duration(*actions.SnoozeMinutes) * time.Minute)
		if _, err := s.inbox.SnoozeConversation(ctx, workspaceID, conversationID, until, actorID); err != nil {
			return fail(err)
		}
		result.Applied = append(result.Applied, "snooze")
	}
	return result
}

func supportActionErrorMessage(err error) string {
	switch {
	case errors.Is(err, repository.ErrSupportMerged):
		return "merged conversations are read-only"
	case errors.Is(err, repository.ErrSupportMergeNotFound):
		return "conversation not found"
	case errors.Is(err, ErrSupportSnoozeClosed), errors.Is(err, ErrSupportInvalidPriority), errors.Is(err, ErrSupportInvalidSnooze):
		return err.Error()
	}
	message := err.Error()
	if strings.Contains(message, "not found") {
		return message
	}
	return "the change could not be applied"
}

// ApplyMacro runs a shortcut's actions on a conversation.
func (s *SupportConversationActionService) ApplyMacro(ctx context.Context, workspaceID, conversationID, macroID, actorID string) (*model.SupportMacroApplyResponse, error) {
	macro, err := s.inbox.cannedResponseRepo.GetByID(ctx, workspaceID, macroID)
	if err != nil {
		return nil, err
	}
	if macro == nil {
		return nil, fmt.Errorf("canned response not found")
	}
	if macro.Actions.Empty() {
		return nil, ErrSupportMacroWithoutActions
	}
	actions, err := NormalizeSupportConversationActions(macro.Actions)
	if err != nil {
		return nil, err
	}
	result := s.Apply(ctx, workspaceID, conversationID, actions, actorID)
	conversation, err := s.inbox.GetConversation(ctx, workspaceID, conversationID)
	if err != nil && result.Error == "" {
		return nil, err
	}
	return &model.SupportMacroApplyResponse{Conversation: conversation, Result: result}, nil
}

// ApplyBulk runs one set of actions on up to model.SupportBulkActionLimit
// conversations, reporting each result.
func (s *SupportConversationActionService) ApplyBulk(ctx context.Context, workspaceID string, req model.SupportBulkActionRequest, actorID string) (*model.SupportBulkActionResponse, error) {
	ids, err := normalizeActionIDs(req.ConversationIDs, "conversation")
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, invalidSupportActions("choose at least one conversation")
	}
	if len(ids) > model.SupportBulkActionLimit {
		return nil, invalidSupportActions("choose at most %d conversations at a time", model.SupportBulkActionLimit)
	}
	actions, err := NormalizeSupportConversationActions(req.Actions)
	if err != nil {
		return nil, err
	}
	if actions.Empty() {
		return nil, invalidSupportActions("choose a change to apply")
	}
	response := &model.SupportBulkActionResponse{Results: make([]model.SupportActionResult, 0, len(ids))}
	for _, id := range ids {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result := s.Apply(ctx, workspaceID, id, actions, actorID)
		if result.Error == "" {
			response.Succeeded++
		} else {
			response.Failed++
		}
		response.Results = append(response.Results, result)
	}
	return response, nil
}
