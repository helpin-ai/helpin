package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type supportRecipientSelection struct {
	UserID string
	TeamID string
	Reason string
	Status model.SupportTeammatePresenceStatus
}

type supportRecipientSelectorInput struct {
	WorkspaceID         string
	MailboxID           *string
	OwnerUserID         *string
	HandoffBehavior     string
	HandoffTeamID       *string
	EventType           string
	Channel             string
	RequirePreferences  bool
	RequireAvailability bool
	Now                 time.Time
}

func selectSupportConversationRecipient(
	ctx context.Context,
	workspaceRepo *repository.WorkspaceRepository,
	mailboxRepo *repository.SupportMailboxRepository,
	installationRepo *repository.SupportInboxInstallationRepository,
	prefRepo *repository.NotificationPreferenceRepository,
	presence websocket.PresenceProvider,
	statusOverrideRepo *repository.SupportTeammateStatusOverrideRepository,
	input supportRecipientSelectorInput,
) (*supportRecipientSelection, error) {
	if workspaceRepo == nil || strings.TrimSpace(input.WorkspaceID) == "" {
		return nil, nil
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	settings, availability, err := loadSupportAvailability(ctx, installationRepo, input.WorkspaceID, now)
	if err != nil {
		return nil, err
	}
	if input.HandoffBehavior == "" {
		input.HandoffBehavior = settings.HandoffBehavior
	}
	if input.HandoffTeamID == nil {
		input.HandoffTeamID = settings.HandoffTeamID
	}

	statuses, err := resolveSupportTeammatePresenceStatuses(ctx, workspaceRepo, presence, statusOverrideRepo, input.WorkspaceID, now)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, nil
	}

	statusByUserID := make(map[string]model.SupportTeammatePresenceStatus, len(statuses))
	allUserIDs := make([]string, 0, len(statuses))
	for _, status := range statuses {
		statusByUserID[status.UserID] = status
		allUserIDs = append(allUserIDs, status.UserID)
	}

	ownerID := strings.TrimSpace(derefString(input.OwnerUserID))
	mailboxID := strings.TrimSpace(derefString(input.MailboxID))
	if mailboxID != "" && mailboxRepo != nil {
		mailboxUserIDs, err := mailboxRepo.ListActiveMemberUserIDs(ctx, input.WorkspaceID, mailboxID)
		if err != nil {
			return nil, err
		}
		adminIDs, err := workspaceRepo.ListActiveUserIDsByRoles(ctx, input.WorkspaceID, []string{model.RoleOwner, model.RoleAdmin})
		if err != nil {
			return nil, err
		}
		mailboxCandidates := append(mailboxUserIDs, adminIDs...)
		if ownerID != "" {
			if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, statusByUserID, []string{ownerID}, "", "owner"); err != nil || selection != nil {
				return selection, err
			}
		}
		if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, statusByUserID, mailboxCandidates, "", "mailbox"); err != nil || selection != nil {
			return selection, err
		}
		return nil, nil
	}

	if ownerID != "" {
		if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, statusByUserID, []string{ownerID}, "", "owner"); err != nil || selection != nil {
			return selection, err
		}
	}

	teamID := strings.TrimSpace(derefString(input.HandoffTeamID))
	if teamID != "" && input.HandoffBehavior == "assign_to_team" {
		teamUserIDs, err := workspaceRepo.ListActiveTeamUserIDs(ctx, input.WorkspaceID, teamID)
		if err != nil {
			return nil, err
		}
		if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, statusByUserID, teamUserIDs, teamID, "team"); err != nil || selection != nil {
			return selection, err
		}
	}

	if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, statusByUserID, allUserIDs, "", "workspace"); err != nil || selection != nil {
		return selection, err
	}

	return nil, nil
}

func buildSupportRecipientSelection(
	ctx context.Context,
	prefRepo *repository.NotificationPreferenceRepository,
	availability supportAvailabilitySnapshot,
	input supportRecipientSelectorInput,
	statusByUserID map[string]model.SupportTeammatePresenceStatus,
	candidateUserIDs []string,
	teamID string,
	reason string,
) (*supportRecipientSelection, error) {
	uniqueIDs := make([]string, 0, len(candidateUserIDs))
	seen := make(map[string]struct{}, len(candidateUserIDs))
	for _, userID := range candidateUserIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, ok := statusByUserID[userID]; !ok {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		uniqueIDs = append(uniqueIDs, userID)
	}
	if len(uniqueIDs) == 0 {
		return nil, nil
	}

	sort.Slice(uniqueIDs, func(i, j int) bool {
		left := statusByUserID[uniqueIDs[i]]
		right := statusByUserID[uniqueIDs[j]]
		leftRank := supportRecipientStatusRank(left.Status)
		rightRank := supportRecipientStatusRank(right.Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return uniqueIDs[i] < uniqueIDs[j]
	})

	for _, userID := range uniqueIDs {
		status := statusByUserID[userID]
		if input.RequireAvailability && !supportRecipientIsAvailable(availability, status) {
			continue
		}
		if input.RequirePreferences {
			if prefRepo == nil || input.Channel == "" || input.EventType == "" {
				continue
			}
			shouldNotify, err := prefRepo.ShouldNotify(ctx, userID, input.WorkspaceID, input.EventType, input.Channel, teamID)
			if err != nil {
				return nil, err
			}
			if !shouldNotify {
				continue
			}
		}

		return &supportRecipientSelection{
			UserID: userID,
			TeamID: teamID,
			Reason: reason,
			Status: status,
		}, nil
	}

	return nil, nil
}

func supportRecipientIsAvailable(availability supportAvailabilitySnapshot, status model.SupportTeammatePresenceStatus) bool {
	if availability.IsWithinOfficeHours {
		return true
	}
	return status.Status == model.SupportTeammateStatusOnline
}

func supportRecipientStatusRank(status string) int {
	switch status {
	case model.SupportTeammateStatusOnline:
		return 0
	case model.SupportTeammateStatusAway:
		return 1
	default:
		return 2
	}
}
