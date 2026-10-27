package service

import (
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// supportReportTopics summarizes each tag's conversations with the headline
// rules, so a topic row reads like the report narrowed to that tag. Tags
// without conversations in either period are left out. Conversations without
// a tag form a final row with an empty tag ID.
func supportReportTopics(
	facts []repository.SupportReportConversationFact,
	links []repository.SupportReportTagLink,
	ratings []repository.SupportReportCSATRow,
	current, previous supportReportPeriod,
) []model.SupportReportTopic {
	tagsByConversation := map[string][]repository.SupportReportTagLink{}
	for _, link := range links {
		tagsByConversation[link.ConversationID] = append(tagsByConversation[link.ConversationID], link)
	}
	ratingsByConversation := map[string][]repository.SupportReportCSATRow{}
	for _, rating := range ratings {
		ratingsByConversation[rating.ConversationID] = append(ratingsByConversation[rating.ConversationID], rating)
	}

	type group struct {
		tag   repository.SupportReportTagLink
		facts []repository.SupportReportConversationFact
	}
	byTag := map[string]*group{}
	untagged := &group{}
	for _, fact := range facts {
		tags := tagsByConversation[fact.ID]
		if len(tags) == 0 {
			untagged.facts = append(untagged.facts, fact)
			continue
		}
		for _, tag := range tags {
			entry, ok := byTag[tag.TagID]
			if !ok {
				entry = &group{tag: tag}
				byTag[tag.TagID] = entry
			}
			entry.facts = append(entry.facts, fact)
		}
	}

	summarize := func(g *group) model.SupportReportTopic {
		var topicRatings []repository.SupportReportCSATRow
		for _, fact := range g.facts {
			topicRatings = append(topicRatings, ratingsByConversation[fact.ID]...)
		}
		summary := summarizeSupportReport(g.facts, topicRatings, current)
		return model.SupportReportTopic{
			TagID: g.tag.TagID, Name: g.tag.Name, Color: g.tag.Color,
			ConversationsCreated: summary.ConversationsCreated,
			PreviousCreated:      summarizeSupportReport(g.facts, nil, previous).ConversationsCreated,
			AIInvolved:           summary.AIInvolved,
			AIResolutionRate:     summary.AIResolutionRate,
			AIHandoffRate:        summary.AIHandoffRate,
			FirstHumanResponse:   summary.FirstHumanResponse,
			Resolution:           summary.Resolution,
			CSATResponses:        summary.CSATResponses,
			CSATAverage:          summary.CSATAverage,
		}
	}

	result := make([]model.SupportReportTopic, 0, len(byTag)+1)
	for _, g := range byTag {
		if topic := summarize(g); topic.ConversationsCreated > 0 || topic.PreviousCreated > 0 {
			result = append(result, topic)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.ConversationsCreated != b.ConversationsCreated {
			return a.ConversationsCreated > b.ConversationsCreated
		}
		if a.PreviousCreated != b.PreviousCreated {
			return a.PreviousCreated > b.PreviousCreated
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	if topic := summarize(untagged); topic.ConversationsCreated > 0 || topic.PreviousCreated > 0 {
		result = append(result, topic)
	}
	return result
}

// supportReportTagNames lists each conversation's tag names for the CSV
// export, alphabetically and comma separated.
func supportReportTagNames(links []repository.SupportReportTagLink) map[string]string {
	names := map[string][]string{}
	for _, link := range links {
		names[link.ConversationID] = append(names[link.ConversationID], link.Name)
	}
	result := make(map[string]string, len(names))
	for conversationID, list := range names {
		sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i]) < strings.ToLower(list[j]) })
		result[conversationID] = strings.Join(list, ", ")
	}
	return result
}
