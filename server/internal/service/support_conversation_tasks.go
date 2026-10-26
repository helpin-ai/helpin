package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// commandAgentContextKey marks changes made by an agent run, so the
// conversation thread can name the agent.
type commandAgentContextKey struct{}

// supportTaskLinkChange names the tasks a main-task update newly linked or unlinked.
type supportTaskLinkChange struct {
	linked   string
	unlinked string
}

// LinkConversationTask links a conversation to a task.
func (s *SupportInboxService) LinkConversationTask(ctx context.Context, workspaceID, ticketID, taskID, actorID string) error {
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		return fmt.Errorf("task_id is required")
	}
	return s.UpdateConversationLinkedTask(ctx, workspaceID, ticketID, &trimmed, actorID)
}

// UpdateConversationLinkedTask sets or clears the conversation's main task and
// notes the change in the thread. Other tasks linked to the conversation stay.
func (s *SupportInboxService) UpdateConversationLinkedTask(ctx context.Context, workspaceID, ticketID string, taskID *string, actorID string) error {
	_, change, err := s.setConversationLinkedTask(ctx, workspaceID, ticketID, taskID, actorID)
	if err != nil {
		return err
	}
	if change.linked != "" {
		s.emitTaskLinkSystemMessage(ctx, workspaceID, ticketID, actorID, model.SystemEventTaskLinked, change.linked)
	}
	if change.unlinked != "" {
		s.emitTaskLinkSystemMessage(ctx, workspaceID, ticketID, actorID, model.SystemEventTaskUnlinked, change.unlinked)
	}
	return nil
}

// LinkCreatedConversationTask links a task made for the conversation outside the
// Create task window, copies the conversation's CRM links onto it, and notes the
// new task in the thread.
func (s *SupportInboxService) LinkCreatedConversationTask(ctx context.Context, workspaceID, conversationID, taskID, actorID string) error {
	conversation, _, err := s.setConversationLinkedTask(ctx, workspaceID, conversationID, &taskID, actorID)
	if err != nil {
		return err
	}
	if _, err := s.copyConversationAssociationsToTask(ctx, workspaceID, conversation, taskID); err != nil {
		return err
	}
	s.emitTaskLinkSystemMessage(ctx, workspaceID, conversationID, actorID, model.SystemEventTaskCreated, taskID)
	return nil
}

// RecordConversationTaskLink notes a task linked to or unlinked from the
// conversation through a generic association. Unlinking the main task clears it.
func (s *SupportInboxService) RecordConversationTaskLink(ctx context.Context, workspaceID, conversationID, taskID string, linked bool) {
	var actorID string
	if actor := supportActorFromContext(ctx, workspaceID); actor != nil {
		actorID = actor.UserID
	}
	if linked {
		s.emitTaskLinkSystemMessage(ctx, workspaceID, conversationID, actorID, model.SystemEventTaskLinked, taskID)
		return
	}
	cleared, err := s.conversationRepo.ClearLinkedTask(ctx, workspaceID, conversationID, taskID)
	if err != nil {
		slog.ErrorContext(ctx, "clear support conversation main task",
			"workspace_id", workspaceID, "conversation_id", conversationID, "task_id", taskID, "error", err)
	} else if cleared {
		publishWorkspaceEvent(s.wsPublisher, "updated", "support_conversation", conversationID, workspaceID, actorID)
	}
	s.emitTaskLinkSystemMessage(ctx, workspaceID, conversationID, actorID, model.SystemEventTaskUnlinked, taskID)
}

// setConversationLinkedTask sets or clears the conversation's main task and its
// association row without writing to the thread.
func (s *SupportInboxService) setConversationLinkedTask(ctx context.Context, workspaceID, ticketID string, taskID *string, actorID string) (*model.SupportConversation, supportTaskLinkChange, error) {
	var change supportTaskLinkChange
	if s.assocRepo == nil {
		return nil, change, fmt.Errorf("CRM association repository is unavailable")
	}
	ticket, err := s.loadConversationAccessible(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, change, err
	}
	if ticket == nil {
		return nil, change, fmt.Errorf("ticket not found")
	}

	var next *string
	if taskID != nil && strings.TrimSpace(*taskID) != "" {
		trimmed := strings.TrimSpace(*taskID)
		if s.taskService != nil {
			task, err := s.taskService.GetByID(ctx, trimmed)
			if err != nil || task == nil || task.Task.WorkspaceID != workspaceID {
				return nil, change, fmt.Errorf("task not found")
			}
		}
		next = &trimmed
	}
	linkedTasks, err := s.conversationTaskAssociations(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, change, err
	}
	previous := derefString(ticket.LinkedTaskID)
	if next != nil {
		if _, linked := linkedTasks[*next]; !linked && previous != *next {
			change.linked = *next
		}
	} else if previous != "" {
		change.unlinked = previous
	}

	ticket.LinkedTaskID = next
	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return nil, change, err
	}
	if next != nil {
		assoc := &model.CRMAssociation{WorkspaceID: workspaceID, FromObjectType: model.CRMObjectSupportConversation, FromObjectID: ticketID, ToObjectType: model.CRMObjectTask, ToObjectID: *next}
		if err := s.assocRepo.Create(ctx, assoc); err != nil {
			return nil, change, err
		}
	} else if assocID, ok := linkedTasks[previous]; ok {
		if err := s.assocRepo.Delete(ctx, assocID); err != nil {
			return nil, change, err
		}
	}

	if s.activitySvc != nil {
		if err := s.activitySvc.Log(ctx, workspaceID, "support_conversation", ticketID, &actorID, "updated", strPtr("linked_task_id"), stringPtrOrNil(previous), next, nil); err != nil {
			slog.ErrorContext(ctx, "record support conversation task link activity",
				"workspace_id", workspaceID, "conversation_id", ticketID, "error", err)
		}
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "support_conversation", ticketID, workspaceID, actorID)
	return ticket, change, nil
}

// conversationTaskAssociations maps each task linked to the conversation to its
// association ID.
func (s *SupportInboxService) conversationTaskAssociations(ctx context.Context, workspaceID, conversationID string) (map[string]string, error) {
	existing, err := s.assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectSupportConversation, conversationID)
	if err != nil {
		return nil, err
	}
	tasks := make(map[string]string, len(existing))
	for _, assoc := range existing {
		otherType, otherID := otherAssociationSide(assoc, model.CRMObjectSupportConversation, conversationID)
		if otherType == model.CRMObjectTask {
			tasks[otherID] = assoc.ID
		}
	}
	return tasks, nil
}

// emitTaskLinkSystemMessage loads the task for its key and name, then posts the
// thread line.
func (s *SupportInboxService) emitTaskLinkSystemMessage(ctx context.Context, workspaceID, conversationID, actorUserID string, event model.SupportSystemEventType, taskID string) {
	task := &model.PMTask{ID: taskID}
	if s.taskService != nil {
		detail, err := s.taskService.GetByID(ctx, taskID)
		if err == nil && detail != nil {
			task = &detail.Task
		} else {
			slog.WarnContext(ctx, "load task for support thread line",
				"workspace_id", workspaceID, "conversation_id", conversationID, "task_id", taskID, "error", err)
		}
	}
	s.emitTaskSystemMessage(ctx, workspaceID, conversationID, actorUserID, event, task)
}

// emitTaskSystemMessage posts "<actor> created/linked/unlinked task #KEY: Name."
// An agent working for a teammate is named in the metadata; an agent working on
// its own is the sender.
func (s *SupportInboxService) emitTaskSystemMessage(ctx context.Context, workspaceID, conversationID, actorUserID string, event model.SupportSystemEventType, task *model.PMTask) {
	if s.messageRepo == nil || task == nil {
		return
	}
	msg := &model.SupportMessage{
		WorkspaceID:     workspaceID,
		ConversationID:  conversationID,
		SenderType:      "user",
		IsInternal:      true,
		MessageType:     "system",
		SystemEventType: model.SupportSystemEventTypeStrPtr(event),
	}
	metadata := map[string]string{"task_id": strings.TrimSpace(task.ID)}
	displayName := "A teammate"
	agentID := commandAgentFromContext(ctx)
	switch {
	case strings.TrimSpace(actorUserID) != "":
		msg.SenderUserID = &actorUserID
		if name := s.lookupUserName(ctx, actorUserID); strings.TrimSpace(name) != "" {
			displayName = supportSystemFirstName(name)
			if displayName == "" {
				displayName = name
			}
		}
		if agentName := s.lookupAgentName(ctx, workspaceID, agentID); agentName != "" {
			metadata["via_agent"] = agentName
		}
	case agentID != "":
		displayName = helpinAIDisplayName
		msg.SenderType = "ai"
		msg.SenderAgentID = &agentID
	}
	msg.SenderDisplayName = &displayName
	msg.Content = supportTaskEventContent(displayName, supportTaskEventVerb(event), task.TaskKey, task.Name)
	if encoded, err := json.Marshal(metadata); err == nil {
		msg.Metadata = string(encoded)
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		slog.ErrorContext(ctx, "create support task system message",
			"workspace_id", workspaceID, "conversation_id", conversationID, "event", event, "error", err)
		return
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, msg, derefString(msg.SenderUserID)))
	}
}

func (s *SupportInboxService) lookupAgentName(ctx context.Context, workspaceID, agentID string) string {
	if s.agentRepo == nil || agentID == "" {
		return ""
	}
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return ""
	}
	return strings.TrimSpace(agent.Name)
}

func withCommandAgent(ctx context.Context, agentID string) context.Context {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return ctx
	}
	return context.WithValue(ctx, commandAgentContextKey{}, agentID)
}

func commandAgentFromContext(ctx context.Context) string {
	agentID, _ := ctx.Value(commandAgentContextKey{}).(string)
	return agentID
}

func supportTaskEventVerb(event model.SupportSystemEventType) string {
	switch event {
	case model.SystemEventTaskLinked:
		return "linked"
	case model.SystemEventTaskUnlinked:
		return "unlinked"
	default:
		return "created"
	}
}

func supportTaskEventContent(actor, verb, taskKey, taskName string) string {
	label := strings.TrimSpace(taskKey)
	if label != "" && !strings.HasPrefix(label, "#") {
		label = "#" + label
	}
	name := strings.Join(strings.Fields(taskName), " ")
	switch {
	case label != "" && name != "":
		return fmt.Sprintf("%s %s task %s: %s.", actor, verb, label, name)
	case label != "":
		return fmt.Sprintf("%s %s task %s.", actor, verb, label)
	case name != "":
		return fmt.Sprintf("%s %s task: %s.", actor, verb, name)
	default:
		return fmt.Sprintf("%s %s a task.", actor, verb)
	}
}
