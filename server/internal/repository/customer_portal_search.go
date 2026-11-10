package repository

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PortalSearchMatch is a request that matches a portal search: by subject,
// or by the text of a message the portal shows.
type PortalSearchMatch struct {
	ConversationID string
	// Text is the matching message's customer-safe text; empty for a
	// subject match.
	Text string
}

// portalSearchTerms splits a query into at most eight lowercase words.
// Words keep their punctuation, so addresses and links match as typed.
func portalSearchTerms(query string) []string {
	words := strings.Fields(strings.ToLower(query))
	if len(words) > 8 {
		words = words[:8]
	}
	return words
}

// likeContains is a LIKE pattern matching text that contains term.
func likeContains(term string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(term) + "%"
}

// SearchPortalRequests finds which of conversationIDs match query in their
// subject or in a message the portal shows: public replies by customers or
// the team, never notes, system events or email-only replies, including
// replies merged in. Merged messages match on their public text. Every word
// must appear (case-insensitive). The search is bounded by conversationIDs,
// one list a customer can see, so it reads text rather than an index and
// behaves the same on every database and language. It returns at most limit
// subject matches and limit conversations matching by message, each with its
// newest matching message.
func (r *CustomerPortalRepository) SearchPortalRequests(ctx context.Context, workspaceID string, conversationIDs []string, query string, limit int) ([]PortalSearchMatch, error) {
	terms := portalSearchTerms(query)
	if len(conversationIDs) == 0 || len(terms) == 0 {
		return nil, nil
	}
	match := func(column string) (string, []any) {
		clauses := make([]string, 0, len(terms))
		args := make([]any, 0, len(terms))
		for _, term := range terms {
			clauses = append(clauses, "lower("+column+") LIKE ? ESCAPE '\\'")
			args = append(args, likeContains(term))
		}
		return strings.Join(clauses, " AND "), args
	}

	var matches []PortalSearchMatch
	subjectSQL, subjectArgs := match("conv.subject")
	if err := r.db.WithContext(ctx).Table("support_conversations AS conv").
		Where("conv.workspace_id = ? AND conv.id IN ?", workspaceID, conversationIDs).
		Where(subjectSQL, subjectArgs...).
		Select("conv.id AS conversation_id, '' AS text").Order("conv.created_at DESC").Limit(limit).
		Scan(&matches).Error; err != nil {
		return nil, err
	}

	publicText := "COALESCE(" + supportMetadataValueSQL(r.db, "msg", "merge_public_text") + ", msg.content)"
	messageSQL, messageArgs := match(publicText)
	ranked := r.db.Table("support_messages AS msg").
		Joins(`JOIN support_conversations AS conv ON conv.workspace_id = msg.workspace_id AND (conv.id = msg.conversation_id OR EXISTS (
			SELECT 1 FROM support_merge_messages AS grant_row
			WHERE grant_row.workspace_id = msg.workspace_id AND grant_row.conversation_id = conv.id AND grant_row.message_id = msg.id))`).
		Where("conv.workspace_id = ? AND conv.id IN ?", workspaceID, conversationIDs).
		Where("msg.deleted_at IS NULL AND msg.is_internal = false AND msg.message_type = 'reply' AND msg.sender_type IN ('customer', 'user', 'agent', 'ai')").
		Where(supportDeliveryModeSQL(r.db, "msg")+" <> ?", model.SupportDeliveryEmailOnly).
		Where(`NOT EXISTS (SELECT 1 FROM support_conversations AS origin
			WHERE origin.id = COALESCE(msg.origin_conversation_id, msg.conversation_id) AND origin.anonymized_at IS NOT NULL)`).
		Where(messageSQL, messageArgs...).
		Select("conv.id AS conversation_id, " + publicText + " AS text, msg.created_at AS matched_at, " +
			"ROW_NUMBER() OVER (PARTITION BY conv.id ORDER BY msg.created_at DESC, msg.id DESC) AS match_rank")
	var messages []PortalSearchMatch
	if err := r.db.WithContext(ctx).Table("(?) AS ranked", ranked).Where("match_rank = 1").
		Select("conversation_id, text").Order("matched_at DESC").Limit(limit).Scan(&messages).Error; err != nil {
		return nil, err
	}
	return append(matches, messages...), nil
}
