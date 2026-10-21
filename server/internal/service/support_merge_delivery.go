package service

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// mergedDeliveryConversation is a transient transport projection; ownership and
// customer identity on the stored conversation remain unchanged.
func mergedDeliveryConversation(ctx context.Context, repo *repository.SupportConversationRepository, c *model.SupportConversation) (*model.SupportConversation, error) {
	if c == nil || c.MergeRevision == 0 || c.MergedIntoID != nil {
		return c, nil
	}
	group, err := repository.NewSupportMergeRepository(repo.DB()).Group(ctx, c.WorkspaceID, c.ID)
	if err != nil {
		return nil, err
	}
	result := *c
	result.MergeEmailRecipients = model.SupportMergeEmailAudience(group)
	for _, member := range group {
		if member.AnonymousID != nil || member.Source == "widget" {
			result.MergeHasWidget = true
		}
	}
	if strings.TrimSpace(derefString(result.CustomerEmail)) == "" && len(result.MergeEmailRecipients) > 0 && !result.EmailUnsubscribed && result.PrimaryRecipientState != model.SupportPrimaryRecipientStateUnconfirmed {
		result.CustomerEmail = &result.MergeEmailRecipients[0]
	}
	return &result, nil
}
