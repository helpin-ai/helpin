package service

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const notificationReasonKey = "notification_reason"
const notificationActionKey = "notification_action"

func taskRecipientEvent(event model.NotificationEventInput, task *repository.TaskNotificationContext, recipientID string, follows bool) model.NotificationEventInput {
	if task == nil || strings.TrimSpace(task.Title) == "" {
		return event
	}
	event.Metadata = cloneNotificationMetadata(event.Metadata)
	event.EntitySnapshot = cloneNotificationMetadata(event.EntitySnapshot)
	event.EntitySnapshot["title"] = task.Title
	subject := task.Title
	reason := ""
	if task.RequesterID == recipientID {
		subject += " you requested"
		reason = "You requested this task."
	} else {
		for _, owner := range task.Owners {
			if owner.UserID == recipientID {
				subject += " assigned to you"
				reason = "This task is assigned to you."
				break
			}
		}
		if reason == "" && follows {
			subject += " you follow"
			reason = "You follow this task."
		}
	}
	explicit := false
	for _, id := range event.ExplicitRecipients {
		if id == recipientID {
			explicit = true
			break
		}
	}
	action := ""
	switch event.EventType {
	case "task.status_changed":
		action = "updated the status of " + subject
		if task.StateName != "" {
			action = "moved " + subject + " to " + task.StateName
		}
	case "task.assigned":
		if explicit {
			action = "assigned you to " + task.Title
			reason = "This task was assigned to you."
		} else {
			names := []string{}
			for _, owner := range task.Owners {
				for _, id := range event.ExplicitRecipients {
					if owner.UserID == id && strings.TrimSpace(owner.Name) != "" {
						names = append(names, owner.Name)
						break
					}
				}
			}
			target := "a teammate"
			if len(names) > 0 {
				target = strings.Join(names, ", ")
			}
			action = "assigned " + subject + " to " + target
		}
	case "comment.created", "task.comment":
		action = "commented on " + subject
	case "comment.mention", "task.mention", "checklist.mention":
		if explicit {
			action = "mentioned you in " + subject
			if event.EventType == "comment.mention" {
				action = "mentioned you in a comment on " + subject
			}
			if event.EventType == "checklist.mention" {
				action = "mentioned you in a checklist on " + subject
			}
			reason = "You were mentioned in this task."
		} else {
			action = "added a mention in " + subject
		}
	case "task.updated":
		action = "updated " + subject
	case "task.blocked":
		action = "marked " + subject + " as blocked"
	case "task.created":
		action = "created " + subject
	}
	if reason != "" {
		event.Metadata[notificationReasonKey] = reason
	}
	if action == "" && reason != "" {
		action = event.Title
		if name, _ := event.ActorSnapshot["name"].(string); strings.TrimSpace(name) != "" {
			action = strings.TrimPrefix(action, strings.TrimSpace(name)+" ")
		}
		action += " — " + strings.TrimSuffix(reason, ".")
	}
	if action != "" {
		event.Metadata[notificationActionKey] = action
		event.Title = action
		if name, _ := event.ActorSnapshot["name"].(string); strings.TrimSpace(name) != "" {
			event.Title = strings.TrimSpace(name) + " " + action
		}
	}
	return event
}
