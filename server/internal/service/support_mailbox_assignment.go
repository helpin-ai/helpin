package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func validMailboxAssignmentMode(mode string) bool {
	return mode == "manual" || mode == "specific_member" || mode == "round_robin"
}

func (s *SupportInboxService) validateMailboxAssignment(ctx context.Context, mailbox *model.SupportMailbox, memberIDs []string) error {
	if mailbox.AssignmentMode == "manual" {
		mailbox.AssignmentMemberIDs = []string{}
		return nil
	}
	ids := make([]string, 0, len(mailbox.AssignmentMemberIDs))
	seen := make(map[string]bool, len(mailbox.AssignmentMemberIDs))
	for _, id := range mailbox.AssignmentMemberIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("select a valid assignment member")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("select at least one assignment member")
	}
	if mailbox.AssignmentMode == "specific_member" && len(ids) != 1 {
		return fmt.Errorf("select exactly one assignment member")
	}
	eligible, err := s.mailboxRepo.ListAssignmentEligibleMembers(ctx, mailbox.WorkspaceID, mailbox.LinkedTeamID, memberIDs)
	if err != nil {
		return err
	}
	allowed := make(map[string]bool, len(eligible))
	for _, member := range eligible {
		allowed[member.ID] = true
	}
	for _, id := range ids {
		if !allowed[id] {
			return fmt.Errorf("assignment members must have access to this inbox and permission to handle conversations")
		}
	}
	mailbox.AssignmentMemberIDs = ids
	return nil
}
