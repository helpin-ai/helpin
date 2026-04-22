package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// mentionPattern matches @username patterns that are not embedded inside emails or other handles.
var mentionPattern = regexp.MustCompile(`(^|[^a-zA-Z0-9._-])@([a-zA-Z0-9][a-zA-Z0-9._-]*)`)

// extractMentions returns deduplicated @mention usernames from the given body text.
func extractMentions(body string) []string {
	if body == "" {
		return nil
	}
	plainText := tiptap.StripHTML(body)
	matches := mentionPattern.FindAllStringSubmatch(plainText, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	var result []string
	for _, m := range matches {
		username := strings.ToLower(m[2])
		if !seen[username] {
			seen[username] = true
			result = append(result, username)
		}
	}
	return result
}

func normalizeMentionHandle(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		}
	}

	normalized := strings.Join(strings.Fields(b.String()), ".")
	return strings.Trim(normalized, ".")
}

func mentionHandleVariants(value string) []string {
	normalized := normalizeMentionHandle(value)
	if normalized == "" {
		return nil
	}

	tokens := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	if len(tokens) == 0 {
		return nil
	}

	if len(tokens) == 1 {
		return []string{tokens[0]}
	}

	variants := []string{
		strings.Join(tokens, "."),
		strings.Join(tokens, "-"),
		strings.Join(tokens, "_"),
	}

	seen := make(map[string]struct{}, len(variants))
	deduped := make([]string, 0, len(variants))
	for _, variant := range variants {
		if variant == "" {
			continue
		}
		if _, ok := seen[variant]; ok {
			continue
		}
		seen[variant] = struct{}{}
		deduped = append(deduped, variant)
	}
	return deduped
}

func appendMentionLookup(lookup map[string][]string, userID string, values ...string) {
	if userID == "" {
		return
	}
	for _, value := range values {
		for _, handle := range mentionHandleVariants(value) {
			recipients := lookup[handle]
			alreadyPresent := false
			for _, existingUserID := range recipients {
				if existingUserID == userID {
					alreadyPresent = true
					break
				}
			}
			if alreadyPresent {
				continue
			}
			lookup[handle] = append(recipients, userID)
		}
	}
}

func diffMentionHandles(previous, current []string) []string {
	if len(current) == 0 {
		return nil
	}

	seenPrevious := make(map[string]struct{}, len(previous))
	for _, handle := range previous {
		if handle != "" {
			seenPrevious[handle] = struct{}{}
		}
	}

	var added []string
	for _, handle := range current {
		if handle == "" {
			continue
		}
		if _, exists := seenPrevious[handle]; exists {
			continue
		}
		added = append(added, handle)
	}
	return added
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
	return resolveMentionRecipientsForHandles(ctx, workspaceRepo, workspaceID, mentions, authorID, readableTeamIDs)
}

func resolveMentionRecipientsForHandles(ctx context.Context, workspaceRepo *repository.WorkspaceRepository, workspaceID string, mentions []string, authorID string, readableTeamIDs []string) ([]string, error) {
	if len(mentions) == 0 {
		return nil, nil
	}

	members, err := workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Build a lookup of normalized display-name, email-prefix, and user-id handles to user IDs.
	nameToUserIDs := make(map[string][]string, len(members))
	for _, m := range members {
		if m.UserID == "" {
			continue
		}

		appendMentionLookup(nameToUserIDs, m.UserID, m.FullName, m.UserID)
		if at := strings.Index(m.Email, "@"); at > 0 {
			appendMentionLookup(nameToUserIDs, m.UserID, m.Email[:at])
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
		for _, uid := range nameToUserIDs[lower] {
			if uid == "" || uid == authorID || seen[uid] {
				continue
			}
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
	return emitMentionNotificationForHandles(ctx, notifService, workspaceRepo, input, mentions)
}

func emitMentionNotificationForHandles(ctx context.Context, notifService *NotificationService, workspaceRepo *repository.WorkspaceRepository, input pmMentionNotificationInput, mentions []string) ([]string, error) {
	if len(mentions) == 0 {
		return nil, nil
	}

	mentionedUserIDs, err := resolveMentionRecipientsForHandles(ctx, workspaceRepo, input.WorkspaceID, mentions, input.ActorID, input.ReadableTeamIDs)
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
