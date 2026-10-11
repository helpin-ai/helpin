package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ConversationMergeCandidates returns an authorized same-customer search by default.
func (s *SupportInboxService) ConversationMergeCandidates(ctx context.Context, workspaceID, sourceID, search string, allCustomers bool, page int) (*model.SupportMergeCandidates, error) {
	actor, err := requireSupportMergeActor(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	source, err := s.loadConversationAccessible(ctx, workspaceID, sourceID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, repository.ErrSupportMergeNotFound
	}
	installation, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	allowed := false
	if installation != nil {
		allowed = parseSettings(installation.Settings).AllowCrossCustomerMerge
	}
	if allCustomers && !allowed {
		return nil, fmt.Errorf("different customer; ask an admin to enable this")
	}
	rows, more, err := s.conversationRepo.MergeCandidates(ctx, workspaceID, actor.WorkspaceMemberID, actor.Role, *source, search, allCustomers, page)
	if err != nil {
		return nil, err
	}
	return &model.SupportMergeCandidates{Data: rows, HasMore: more, CrossCustomerAllowed: allowed}, nil
}

// ConversationSharedHistory returns a retained shared interval. Viewing a
// container is not permission to see a separately restricted original thread.
func (s *SupportInboxService) ConversationSharedHistory(ctx context.Context, workspaceID, conversationID, mergeID, cursor string) (*model.SupportMessagePage, error) {
	container, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if container == nil {
		return nil, repository.ErrSupportMergeNotFound
	}
	edge, err := s.mergeRepository().Get(ctx, workspaceID, mergeID)
	if err != nil {
		return nil, err
	}
	if edge.SourceID != conversationID {
		return nil, repository.ErrSupportMergeNotFound
	}
	target, err := s.loadConversationAccessible(ctx, workspaceID, edge.TargetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, repository.ErrSupportMergeNotFound
	}
	var before *model.SupportMessage
	if cursor != "" {
		at, id, err := decodeSupportMessageCursor(cursor)
		if err != nil {
			return nil, err
		}
		before = &model.SupportMessage{ID: id, CreatedAt: at}
	}
	messages, more, err := s.mergeRepository().SharedMessages(ctx, workspaceID, conversationID, mergeID, false, before, 50)
	if err != nil {
		return nil, err
	}
	filtered := make([]model.SupportMessage, 0, len(messages))
	for _, message := range messages {
		owner, err := s.loadConversationAccessible(ctx, workspaceID, message.ConversationID)
		if err != nil {
			return nil, err
		}
		if owner != nil && owner.AnonymizedAt == nil {
			filtered = append(filtered, message)
		}
	}
	if s.attachmentService != nil {
		if err := s.attachmentService.HydrateMessages(ctx, filtered); err != nil {
			return nil, err
		}
	}
	if s.emailLogRepo != nil && len(filtered) > 0 {
		ids := make([]string, 0, len(filtered))
		for _, m := range filtered {
			ids = append(ids, m.ID)
		}
		logs, err := s.emailLogRepo.ListByMessageIDs(ctx, workspaceID, ids)
		if err != nil {
			return nil, err
		}
		hydrateEmailBodiesFromLogs(filtered, logs)
	}
	result := &model.SupportMessagePage{Data: filtered, HasMore: more}
	if more && len(messages) > 0 {
		next := encodeSupportMessageCursor(messages[0].CreatedAt, messages[0].ID)
		result.NextCursor = &next
	}
	return result, nil
}
