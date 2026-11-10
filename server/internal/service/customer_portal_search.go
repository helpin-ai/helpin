package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrPortalSearchInvalid means the search text is too short or too long.
var ErrPortalSearchInvalid = errors.New("search for 2 to 200 characters")

const (
	portalSearchLimit   = 50
	portalSnippetRadius = 70
)

// PortalSearchResult is a request matching a portal search, with an excerpt
// of the matching message; the excerpt is empty when the subject matched.
type PortalSearchResult struct {
	Reference string `json:"reference"`
	Snippet   string `json:"snippet,omitempty"`
}

// SearchRequests searches the subjects and portal-visible messages of the
// requests in one list the customer can see: their own, a member company's,
// or those copied to them. It only narrows that list, so it can never show
// more than the list does.
func (s *CustomerPortalService) SearchRequests(ctx context.Context, access *PortalAccess, query, scope, companyID string) ([]PortalSearchResult, error) {
	query = strings.TrimSpace(query)
	if length := utf8.RuneCountInString(query); length < 2 || length > 200 {
		return nil, ErrPortalSearchInvalid
	}
	var requests []model.SupportPortalRequest
	var err error
	switch scope {
	case "", PortalScopeMine:
		requests, err = s.repo.ListRequests(ctx, access.Workspace.ID, access.Identity.ID, "")
	case PortalScopeCompany:
		requests, err = s.CompanyRequests(ctx, access, companyID, "")
	case PortalScopeCopied:
		requests, err = s.CopiedRequests(ctx, access, "")
	default:
		return nil, ErrPortalScopeInvalid
	}
	if err != nil {
		return nil, err
	}
	references := make(map[string]string, len(requests))
	ids := make([]string, 0, len(requests))
	for _, request := range requests {
		references[request.ConversationID] = request.Reference
		ids = append(ids, request.ConversationID)
	}
	matches, err := s.repo.SearchPortalRequests(ctx, access.Workspace.ID, ids, query, portalSearchLimit)
	if err != nil {
		return nil, err
	}
	results := []PortalSearchResult{}
	position := map[string]int{}
	for _, match := range matches {
		snippet := portalSearchSnippet(match.Text, query)
		if index, seen := position[match.ConversationID]; seen {
			if results[index].Snippet == "" {
				results[index].Snippet = snippet
			}
			continue
		}
		if len(results) == portalSearchLimit {
			continue
		}
		position[match.ConversationID] = len(results)
		results = append(results, PortalSearchResult{Reference: references[match.ConversationID], Snippet: snippet})
	}
	return results, nil
}

// portalSearchSnippet is plain text around the first query word found in
// text, or its start.
func portalSearchSnippet(text, query string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	plain := []rune(strings.Join(strings.Fields(strings.NewReplacer("**", "", "__", "", "`", "", "#", "").Replace(text)), " "))
	lower := []rune(strings.ToLower(string(plain)))
	start := 0
	for _, word := range strings.Fields(strings.ToLower(query)) {
		if index := strings.Index(string(lower), word); index >= 0 {
			start = utf8.RuneCountInString(string(lower)[:index])
			break
		}
	}
	from := max(0, start-portalSnippetRadius)
	to := min(len(plain), start+portalSnippetRadius*2)
	snippet := strings.TrimSpace(string(plain[from:to]))
	if from > 0 {
		snippet = "…" + snippet
	}
	if to < len(plain) {
		snippet += "…"
	}
	return snippet
}
