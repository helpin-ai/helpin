package service

import (
	"context"
	"slices"
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

// supportRecipientAvailability reports whether a candidate can take a conversation now.
type supportRecipientAvailability func(teammate supportTeammatePresence) bool

type supportRecipientSelectorInput struct {
	WorkspaceID            string
	MailboxID              *string
	OwnerUserID            *string
	HandoffBehavior        string
	HandoffTeamID          *string
	EventType              string
	Channel                string
	RequirePreferences     bool
	RequireAvailability    bool
	UseMailboxAssignment   bool
	PreserveCandidateOrder bool
	Now                    time.Time
}

func selectSupportConversationRecipient(
	ctx context.Context,
	workspaceRepo *repository.WorkspaceRepository,
	mailboxRepo *repository.SupportMailboxRepository,
	installationRepo *repository.SupportInboxInstallationRepository,
	prefRepo *repository.NotificationPreferenceRepository,
	presence websocket.PresenceProvider,
	statusOverrideRepo *repository.SupportTeammateStatusOverrideRepository,
	responderDirectory *SupportResponderDirectory,
	input supportRecipientSelectorInput,
) (*supportRecipientSelection, error) {
	if workspaceRepo == nil || strings.TrimSpace(input.WorkspaceID) == "" {
		return nil, nil
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	settings, officeHours, err := loadSupportAvailability(ctx, installationRepo, input.WorkspaceID, now)
	if err != nil {
		return nil, err
	}
	if input.HandoffBehavior == "" {
		input.HandoffBehavior = settings.HandoffBehavior
	}
	if input.HandoffTeamID == nil {
		input.HandoffTeamID = settings.HandoffTeamID
	}

	teammates, err := resolveSupportTeammatePresence(ctx, workspaceRepo, presence, statusOverrideRepo, input.WorkspaceID, now)
	if err != nil {
		return nil, err
	}
	if len(teammates) == 0 {
		return nil, nil
	}

	presenceByUserID := make(map[string]supportTeammatePresence, len(teammates))
	allUserIDs := make([]string, 0, len(teammates))
	for _, teammate := range teammates {
		presenceByUserID[teammate.status.UserID] = teammate
		allUserIDs = append(allUserIDs, teammate.status.UserID)
	}

	// Within business hours anyone can take a conversation; outside them only a
	// teammate who is available to customers right now.
	availability := supportRecipientAvailability(func(supportTeammatePresence) bool { return true })
	if input.RequireAvailability && !officeHours.IsWithinOfficeHours {
		customerTeammates, err := newSupportCustomerTeammates(ctx, responderDirectory, teammates, input.WorkspaceID, settings.BusinessHoursTimezone, now)
		if err != nil {
			return nil, err
		}
		availability = customerTeammates.available
	}

	ownerID := strings.TrimSpace(derefString(input.OwnerUserID))
	mailboxID := strings.TrimSpace(derefString(input.MailboxID))
	if mailboxID != "" && mailboxRepo != nil {
		if input.UseMailboxAssignment {
			mailbox, err := mailboxRepo.GetByID(ctx, input.WorkspaceID, mailboxID)
			if err != nil {
				return nil, err
			}
			if mailbox == nil || !mailbox.Active {
				return nil, nil
			}
			if mailbox.AssignmentMemberIDs != nil {
				candidates, err := mailboxRepo.ListAssignmentCandidateUserIDs(ctx, input.WorkspaceID, mailbox)
				if err != nil {
					return nil, err
				}
				input.PreserveCandidateOrder = true
				return buildSupportRecipientSelection(ctx, prefRepo, availability, input, presenceByUserID, candidates, "", "mailbox_assignment")
			}
		}
		mailboxUserIDs, err := mailboxRepo.ListActiveMemberUserIDs(ctx, input.WorkspaceID, mailboxID)
		if err != nil {
			return nil, err
		}
		adminIDs, err := workspaceRepo.ListActiveUserIDsByRoles(ctx, input.WorkspaceID, []string{model.RoleOwner, model.RoleAdmin})
		if err != nil {
			return nil, err
		}
		mailboxCandidates := append(mailboxUserIDs, adminIDs...)
		if ownerID != "" && slices.Contains(mailboxCandidates, ownerID) {
			if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, presenceByUserID, []string{ownerID}, "", "owner"); err != nil || selection != nil {
				return selection, err
			}
		}
		if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, presenceByUserID, mailboxCandidates, "", "mailbox"); err != nil || selection != nil {
			return selection, err
		}
		return nil, nil
	}

	if ownerID != "" {
		if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, presenceByUserID, []string{ownerID}, "", "owner"); err != nil || selection != nil {
			return selection, err
		}
	}

	teamID := strings.TrimSpace(derefString(input.HandoffTeamID))
	if teamID != "" && input.HandoffBehavior == "assign_to_team" {
		teamUserIDs, err := workspaceRepo.ListActiveTeamUserIDs(ctx, input.WorkspaceID, teamID)
		if err != nil {
			return nil, err
		}
		if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, presenceByUserID, teamUserIDs, teamID, "team"); err != nil || selection != nil {
			return selection, err
		}
	}

	if selection, err := buildSupportRecipientSelection(ctx, prefRepo, availability, input, presenceByUserID, allUserIDs, "", "workspace"); err != nil || selection != nil {
		return selection, err
	}

	return nil, nil
}

func buildSupportRecipientSelection(
	ctx context.Context,
	prefRepo *repository.NotificationPreferenceRepository,
	availability supportRecipientAvailability,
	input supportRecipientSelectorInput,
	presenceByUserID map[string]supportTeammatePresence,
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
		if _, ok := presenceByUserID[userID]; !ok {
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

	if !input.PreserveCandidateOrder {
		sort.Slice(uniqueIDs, func(i, j int) bool {
			left := presenceByUserID[uniqueIDs[i]]
			right := presenceByUserID[uniqueIDs[j]]
			leftRank := supportRecipientStatusRank(left.status.Status)
			rightRank := supportRecipientStatusRank(right.status.Status)
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			return uniqueIDs[i] < uniqueIDs[j]
		})
	}

	for _, userID := range uniqueIDs {
		teammate := presenceByUserID[userID]
		if input.RequireAvailability && !availability(teammate) {
			continue
		}
		if input.RequirePreferences {
			if prefRepo == nil || input.Channel == "" || input.EventType == "" {
				continue
			}
			channel := input.Channel
			if channel == "any" {
				channel = "in_app"
			}
			shouldNotify, err := prefRepo.ShouldNotify(ctx, userID, input.WorkspaceID, input.EventType, channel, teamID)
			if err == nil && !shouldNotify && input.Channel == "any" {
				shouldNotify, err = prefRepo.ShouldNotify(ctx, userID, input.WorkspaceID, input.EventType, "email", teamID)
			}
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
			Status: teammate.status,
		}, nil
	}

	return nil, nil
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
