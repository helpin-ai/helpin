package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// mentionPattern matches @username patterns (alphanumeric, dots, hyphens, underscores).
var mentionPattern = regexp.MustCompile(`@([a-zA-Z0-9][a-zA-Z0-9._-]*)`)

// extractMentions returns deduplicated @mention usernames from the given body text.
func extractMentions(body string) []string {
	if body == "" {
		return nil
	}
	matches := mentionPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	var result []string
	for _, m := range matches {
		username := m[1]
		if !seen[username] {
			seen[username] = true
			result = append(result, username)
		}
	}
	return result
}

// mentionScopeForTeamID returns a slice containing the team ID if non-nil,
// used to scope mention resolution to specific teams.
func mentionScopeForTeamID(teamID *string) []string {
	if teamID == nil || *teamID == "" {
		return nil
	}
	return []string{*teamID}
}

// pmMentionNotificationInput holds the parameters for emitting a mention notification.
type pmMentionNotificationInput struct {
	WorkspaceID      string
	ActorID          string
	Body             string
	EventType        string
	EntityType       string
	EntityID         string
	Title            string
	TeamID           string
	ReadableTeamIDs  []string
	EntitySnapshot   model.JSONB
	NotificationBody string
}

// resolveMentionRecipients extracts @mentions from body text and resolves them
// to workspace member user IDs. It excludes the author and returns only IDs of
// members who exist in the workspace.
func resolveMentionRecipients(ctx context.Context, workspaceRepo *repository.WorkspaceRepository, workspaceID, body, authorID string, readableTeamIDs []string) ([]string, error) {
	mentions := extractMentions(body)
	if len(mentions) == 0 {
		return nil, nil
	}

	members, err := workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Build a lookup of display_name (lowered) and email prefix to user ID.
	nameToUserID := make(map[string]string, len(members))
	for _, m := range members {
		if m.UserID != "" {
			lower := strings.ToLower(m.FullName)
			// Map full name variants: "John Doe" -> "john.doe", "john-doe", "john_doe"
			normalized := strings.ReplaceAll(lower, " ", ".")
			nameToUserID[normalized] = m.UserID
			nameToUserID[strings.ReplaceAll(lower, " ", "-")] = m.UserID
			nameToUserID[strings.ReplaceAll(lower, " ", "_")] = m.UserID

			// Also map by email prefix (part before @).
			if m.Email != "" {
				if at := strings.Index(m.Email, "@"); at > 0 {
					nameToUserID[strings.ToLower(m.Email[:at])] = m.UserID
				}
			}
		}
	}

	seen := make(map[string]bool)
	var userIDs []string
	teamScope := make(map[string]struct{}, len(readableTeamIDs))
	for _, teamID := range readableTeamIDs {
		if trimmed := strings.TrimSpace(teamID); trimmed != "" {
			teamScope[trimmed] = struct{}{}
		}
	}
	for _, mention := range mentions {
		lower := strings.ToLower(mention)
		if team, err := workspaceRepo.GetTeamByHandle(ctx, workspaceID, lower); err != nil {
			return nil, fmt.Errorf("resolve mention recipients: %w", err)
		} else if team != nil {
			if _, allowed := teamScope[team.ID]; !allowed {
				// Team handles should not fall through to similarly named user handles.
				continue
			}
			teamUserIDs, err := workspaceRepo.ListActiveTeamUserIDs(ctx, workspaceID, team.ID)
			if err != nil {
				return nil, fmt.Errorf("resolve mention recipients: %w", err)
			}
			for _, uid := range teamUserIDs {
				if uid == "" || uid == authorID || seen[uid] {
					continue
				}
				seen[uid] = true
				userIDs = append(userIDs, uid)
			}
			continue
		}
		if uid, ok := nameToUserID[lower]; ok && uid != authorID && !seen[uid] {
			seen[uid] = true
			userIDs = append(userIDs, uid)
		}
	}
	return userIDs, nil
}

// emitMentionNotification resolves mentions and emits a notification for them.
// Returns the list of mentioned user IDs that were notified.
func emitMentionNotification(ctx context.Context, notifService *NotificationService, workspaceRepo *repository.WorkspaceRepository, input pmMentionNotificationInput) ([]string, error) {
	mentions := extractMentions(input.Body)
	if len(mentions) == 0 {
		return nil, nil
	}

	mentionedUserIDs, err := resolveMentionRecipients(ctx, workspaceRepo, input.WorkspaceID, input.Body, input.ActorID, input.ReadableTeamIDs)
	if err != nil {
		return nil, err
	}
	if len(mentionedUserIDs) == 0 {
		return nil, nil
	}

	slog.InfoContext(ctx, "emitting mention notification",
		"event_type", input.EventType,
		"entity_id", input.EntityID,
		"mentioned_user_ids", mentionedUserIDs,
	)

	body := input.NotificationBody
	if body == "" {
		body = truncate(tiptap.StripHTML(input.Body), 200)
	}

	if err := notifService.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        input.WorkspaceID,
		ActorID:            input.ActorID,
		EventType:          input.EventType,
		EntityType:         input.EntityType,
		EntityID:           input.EntityID,
		Title:              input.Title,
		Body:               body,
		Category:           "mention",
		Priority:           "high",
		TeamID:             input.TeamID,
		EntitySnapshot:     input.EntitySnapshot,
		ExplicitRecipients: mentionedUserIDs,
		SkipFollowers:      true,
	}); err != nil {
		return mentionedUserIDs, err
	}

	return mentionedUserIDs, nil
}
