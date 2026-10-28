package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// supportInboxPeople lists who looks after the conversations in one inbox,
// most responsible first. A team inbox has its own people; the Shared Inbox
// relies on the teammates who answer customers anywhere in the workspace.
type supportInboxPeople struct {
	tiers []supportInboxTier
	// allowed is everyone who may own a conversation in this inbox, so an
	// assignee or last replier from another inbox is skipped after a move.
	allowed map[string]struct{}
	// answerers decide whether the inbox's team can reply now. Nil means the
	// workspace's responders decide, as for the Shared Inbox.
	answerers map[string]struct{}
}

// supportInboxTier is one step of an inbox's people, tried in order.
type supportInboxTier struct {
	reason  string
	userIDs []string
}

// allows reports whether userID may own a conversation in this inbox.
func (p supportInboxPeople) allows(userID string) bool {
	_, ok := p.allowed[userID]
	return ok
}

// resolveSupportInboxPeople lists the people for a conversation's inbox.
//
// A team inbox tries its assignment people ("Assign to" or "Assign between"),
// then the people who answer customers in that inbox, then any inbox member,
// and admins only when the inbox has no members. The Shared Inbox, and an inbox
// that no longer exists, tries the people who answer the Shared Inbox, then the
// workspace's responders, then everyone with support access.
func resolveSupportInboxPeople(
	ctx context.Context,
	workspaceRepo *repository.WorkspaceRepository,
	mailboxRepo *repository.SupportMailboxRepository,
	index supportResponderIndex,
	workspaceID, mailboxID string,
	supportUserIDs []string,
) (supportInboxPeople, error) {
	mailboxID = strings.TrimSpace(mailboxID)
	if mailboxID == "" || mailboxRepo == nil || workspaceRepo == nil {
		return sharedSupportInboxPeople(index, supportUserIDs), nil
	}
	mailbox, err := mailboxRepo.GetByID(ctx, workspaceID, mailboxID)
	if err != nil {
		return supportInboxPeople{}, fmt.Errorf("load inbox: %w", err)
	}
	if mailbox == nil || !mailbox.Active {
		return sharedSupportInboxPeople(index, supportUserIDs), nil
	}
	pool, err := mailboxRepo.ListAssignmentCandidateUserIDs(ctx, workspaceID, mailbox)
	if err != nil {
		return supportInboxPeople{}, fmt.Errorf("load inbox assignment people: %w", err)
	}
	members, err := mailboxRepo.ListActiveMemberUserIDs(ctx, workspaceID, mailboxID)
	if err != nil {
		return supportInboxPeople{}, fmt.Errorf("load inbox members: %w", err)
	}
	admins, err := workspaceRepo.ListActiveUserIDsByRoles(ctx, workspaceID, []string{model.RoleOwner, model.RoleAdmin})
	if err != nil {
		return supportInboxPeople{}, fmt.Errorf("load workspace admins: %w", err)
	}

	// Anyone with access who keeps answering this inbox looks after it, admins included.
	answered := supportRespondersAmong(index.inbox(mailboxID), append(append([]string(nil), members...), admins...))

	people := supportInboxPeople{
		tiers: []supportInboxTier{
			{reason: "mailbox_assignment", userIDs: pool},
			{reason: "mailbox_responders", userIDs: answered},
			{reason: "mailbox", userIDs: members},
		},
		allowed: supportUserIDSet(pool, members, admins),
	}
	if len(members) == 0 {
		people.tiers = append(people.tiers, supportInboxTier{reason: "admins", userIDs: admins})
	}
	switch {
	case len(pool) > 0 || len(answered) > 0:
		people.answerers = supportUserIDSet(pool, answered)
	case len(members) > 0:
		// A new inbox without replies yet: its members are its team.
		people.answerers = supportUserIDSet(members)
	}
	return people, nil
}

// sharedSupportInboxPeople lists the Shared Inbox's people: teammates who answer
// the Shared Inbox, then teammates who answer customers anywhere, then everyone
// with support access. Someone who only works a team inbox is not its team.
func sharedSupportInboxPeople(index supportResponderIndex, supportUserIDs []string) supportInboxPeople {
	people := supportInboxPeople{allowed: supportUserIDSet(supportUserIDs)}
	if shared := index.inbox(""); len(shared.userIDs) > 0 {
		people.tiers = append(people.tiers, supportInboxTier{reason: "inbox_responders", userIDs: supportRespondersAmong(shared, supportUserIDs)})
		people.answerers = shared.userIDs
	}
	if index.workspace.hasReplyHistory {
		people.tiers = append(people.tiers, supportInboxTier{reason: "responders", userIDs: supportRespondersAmong(index.workspace, supportUserIDs)})
	}
	people.tiers = append(people.tiers, supportInboxTier{reason: "workspace", userIDs: supportUserIDs})
	return people
}

func supportRespondersAmong(responders supportResponders, userIDs []string) []string {
	var answering []string
	for _, userID := range userIDs {
		if _, ok := responders.userIDs[userID]; ok {
			answering = append(answering, userID)
		}
	}
	return answering
}

func supportUserIDSet(groups ...[]string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, group := range groups {
		for _, userID := range group {
			if userID = strings.TrimSpace(userID); userID != "" {
				set[userID] = struct{}{}
			}
		}
	}
	return set
}

// inboxTeamAvailable reports whether someone who looks after the inbox can
// reply now, so a billing teammate who is around does not make a support
// handoff say "connecting you". An inbox with nobody to go on uses the
// workspace's responders. Lookup failures count as nobody, so customers are
// never promised a reply that may not come.
func (s *SupportAIService) inboxTeamAvailable(ctx context.Context, customers supportCustomerTeammates, workspaceID, mailboxID, timezone string, now time.Time) bool {
	if !customers.anyAround() {
		return false
	}
	index, err := s.responderDirectory.index(ctx, workspaceID, timezone, now)
	if err != nil {
		slog.WarnContext(ctx, "load inbox responders failed; treating the inbox team as unavailable", "error", err, "workspace_id", workspaceID, "mailbox_id", mailboxID)
		return false
	}
	people, err := resolveSupportInboxPeople(ctx, s.workspaceRepo, s.mailboxRepo, index, workspaceID, mailboxID, nil)
	if err != nil {
		slog.WarnContext(ctx, "load inbox people failed; treating the inbox team as unavailable", "error", err, "workspace_id", workspaceID, "mailbox_id", mailboxID)
		return false
	}
	if people.answerers == nil {
		return customers.anyAvailable()
	}
	return customers.anyAvailableAmong(people.answerers)
}

// selectSupportReplyAlertRecipient picks the one teammate told that a customer
// replied: the assignee, then whoever last replied to the customer unless the
// conversation moved inbox since, then the people of the inbox the
// conversation is in now. Teammates who turned the alert off are skipped.
func selectSupportReplyAlertRecipient(
	ctx context.Context,
	workspaceRepo *repository.WorkspaceRepository,
	mailboxRepo *repository.SupportMailboxRepository,
	messageRepo *repository.SupportMessageRepository,
	installationRepo *repository.SupportInboxInstallationRepository,
	prefRepo *repository.NotificationPreferenceRepository,
	presence websocket.PresenceProvider,
	statusOverrideRepo *repository.SupportTeammateStatusOverrideRepository,
	responderDirectory *SupportResponderDirectory,
	conv *model.SupportConversation,
	eventType string,
	now time.Time,
) (*supportRecipientSelection, error) {
	if workspaceRepo == nil || conv == nil {
		return nil, nil
	}
	settings, _, err := loadSupportAvailability(ctx, installationRepo, conv.WorkspaceID, now)
	if err != nil {
		return nil, err
	}
	teammates, err := resolveSupportTeammatePresence(ctx, workspaceRepo, presence, statusOverrideRepo, conv.WorkspaceID, now)
	if err != nil || len(teammates) == 0 {
		return nil, err
	}
	presenceByUserID := make(map[string]supportTeammatePresence, len(teammates))
	supportUserIDs := make([]string, 0, len(teammates))
	for _, teammate := range teammates {
		presenceByUserID[teammate.status.UserID] = teammate
		supportUserIDs = append(supportUserIDs, teammate.status.UserID)
	}
	index, err := responderDirectory.index(ctx, conv.WorkspaceID, settings.BusinessHoursTimezone, now)
	if err != nil {
		return nil, err
	}
	people, err := resolveSupportInboxPeople(ctx, workspaceRepo, mailboxRepo, index, conv.WorkspaceID, derefString(conv.MailboxID), supportUserIDs)
	if err != nil {
		return nil, err
	}

	owners := []supportInboxTier{{reason: "assignee", userIDs: []string{derefString(conv.AssignedUserID)}}}
	if lastReplier := strings.TrimSpace(derefString(conv.OpenedByUserID)); lastReplier != "" {
		moved := false
		if messageRepo != nil {
			if moved, err = messageRepo.MovedSinceLastTeammateReply(ctx, conv.WorkspaceID, conv.ID); err != nil {
				return nil, err
			}
		}
		// After a move the reply belongs to the earlier inbox's team.
		if !moved {
			owners = append(owners, supportInboxTier{reason: "last_replier", userIDs: []string{lastReplier}})
		}
	}

	input := supportRecipientSelectorInput{WorkspaceID: conv.WorkspaceID, EventType: eventType, Channel: "any", RequirePreferences: true, Now: now}
	for _, owner := range owners {
		if userID := strings.TrimSpace(owner.userIDs[0]); userID == "" || !people.allows(userID) {
			continue
		}
		if selection, err := buildSupportRecipientSelection(ctx, prefRepo, nil, input, presenceByUserID, owner.userIDs, "", owner.reason); err != nil || selection != nil {
			return selection, err
		}
	}
	for _, tier := range people.tiers {
		if selection, err := buildSupportRecipientSelection(ctx, prefRepo, nil, input, presenceByUserID, tier.userIDs, "", tier.reason); err != nil || selection != nil {
			return selection, err
		}
	}
	return nil, nil
}
