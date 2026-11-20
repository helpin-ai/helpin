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
	result.ProviderDestinations = mergedProviderDestinations(group)
	result.MergeEmailRecipients = model.SupportMergeEmailAudience(group)
	for _, member := range group {
		if member.AnonymousID != nil || model.SupportChannelHasChatSession(member.Source) {
			result.MergeHasWidget = true
		}
		if member.ChannelIdentityID != nil {
			result.MergeHasChannel = true
		}
	}
	if strings.TrimSpace(derefString(result.CustomerEmail)) == "" && len(result.MergeEmailRecipients) > 0 && !result.EmailUnsubscribed && result.PrimaryRecipientState != model.SupportPrimaryRecipientStateUnconfirmed {
		result.CustomerEmail = &result.MergeEmailRecipients[0]
	}
	return &result, nil
}

// Offer an explicit thread choice when a merge includes Slack.
func mergedProviderDestinations(group []model.SupportConversation) []model.SupportProviderDestination {
	var result []model.SupportProviderDestination
	hasSlack := false
	for _, c := range group {
		if c.ChannelIdentityID == nil || c.AnonymizedAt != nil {
			continue
		}
		name := derefString(c.CustomerName)
		if c.Channel == "slack" {
			hasSlack = true
			name = "#" + derefString(c.ProviderChannelName)
		}
		result = append(result, model.SupportProviderDestination{ID: c.ID, Provider: c.Channel, Name: name, DisplayID: c.DisplayID})
	}
	if !hasSlack {
		return nil
	}
	return result
}
