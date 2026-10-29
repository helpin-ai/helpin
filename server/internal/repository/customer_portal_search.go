package repository

import (
	"context"
	"strings"
	"unicode"

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

// portalSearchTerms splits a query into lowercase words of letters and
// digits, at most eight; anything else is dropped, so terms are safe in a
// tsquery.
func portalSearchTerms(query string) []string {
	words := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(words) > 8 {
		words = words[:8]
	}
	return words
}

// SearchPortalRequests finds which of conversationIDs match query in their
// subject or in a message the portal shows: public replies by customers or
// the team, never notes, system events or email-only replies, including
// replies merged in. Merged messages match on their public text. Every term
// must match (as a word prefix on Postgres). Matches are newest first.
func (r *CustomerPortalRepository) SearchPortalRequests(ctx context.Context, workspaceID string, conversationIDs []string, query string, limit int) ([]PortalSearchMatch, error) {
	terms := portalSearchTerms(query)
	if len(conversationIDs) == 0 || len(terms) == 0 {
		return nil, nil
	}
	postgres := r.db.Dialector.Name() == "postgres"
	publicText := "COALESCE(" + supportMetadataValueSQL(r.db, "msg", "merge_public_text") + ", msg.content)"
	var match func(column string) (string, []any)
	if postgres {
		tsquery := strings.Join(terms, ":* & ") + ":*"
		match = func(column string) (string, []any) {
			return "to_tsvector('simple', " + column + ") @@ to_tsquery('simple', ?)", []any{tsquery}
		}
	} else {
		match = func(column string) (string, []any) {
			clauses := make([]string, 0, len(terms))
			args := make([]any, 0, len(terms))
			for _, term := range terms {
				clauses = append(clauses, "lower("+column+") LIKE ?")
				args = append(args, "%"+term+"%")
			}
			return strings.Join(clauses, " AND "), args
		}
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

	messageSQL, messageArgs := match(publicText)
	var messages []PortalSearchMatch
	query2 := r.db.WithContext(ctx).Table("support_messages AS msg").
		Joins(`JOIN support_conversations AS conv ON conv.workspace_id = msg.workspace_id AND (conv.id = msg.conversation_id OR EXISTS (
			SELECT 1 FROM support_merge_messages AS grant_row
			WHERE grant_row.workspace_id = msg.workspace_id AND grant_row.conversation_id = conv.id AND grant_row.message_id = msg.id))`).
		Where("conv.workspace_id = ? AND conv.id IN ?", workspaceID, conversationIDs).
		Where("msg.deleted_at IS NULL AND msg.is_internal = false AND msg.message_type = 'reply' AND msg.sender_type IN ('customer', 'user', 'agent', 'ai')").
		Where(supportDeliveryModeSQL(r.db, "msg")+" <> ?", model.SupportDeliveryEmailOnly).
		Where(`NOT EXISTS (SELECT 1 FROM support_conversations AS origin
			WHERE origin.id = COALESCE(msg.origin_conversation_id, msg.conversation_id) AND origin.anonymized_at IS NOT NULL)`)
	if postgres {
		// The message index narrows candidates; the public text decides.
		query2 = query2.Where("("+supportMetadataValueSQL(r.db, "msg", "merge_public_text")+" IS NOT NULL OR msg.search_vector @@ to_tsquery('simple', ?))", messageArgs[0])
	}
	if err := query2.Where(messageSQL, messageArgs...).
		Select("conv.id AS conversation_id, " + publicText + " AS text").
		Order("msg.created_at DESC").Limit(limit * 4).Scan(&messages).Error; err != nil {
		return nil, err
	}
	return append(matches, messages...), nil
}
