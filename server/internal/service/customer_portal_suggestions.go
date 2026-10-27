package service

import (
	"context"
	"html"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Article suggestions while a customer writes a portal request.
const (
	portalSuggestionMinChars = 12
	portalSuggestionMinWords = 2
	portalSuggestionMaxChars = 400
	portalSuggestionLimit    = 5
)

// PortalArticleSuggestion is a published help center article that may answer
// the request being written.
type PortalArticleSuggestion struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	SpaceName string `json:"space_name,omitempty"`
	Snippet   string `json:"snippet,omitempty"`
	// URL is the public help center address. Articles for signed-in
	// customers have none; they open in the portal by PortalArticleID.
	URL             string `json:"url,omitempty"`
	PortalArticleID string `json:"portal_article_id,omitempty"`
}

// PortalArticleSuggestions distinguishes completed searches from skipped ones.
type PortalArticleSuggestions struct {
	Articles        []PortalArticleSuggestion `json:"articles"`
	SearchCompleted bool                      `json:"search_completed"`
}

// SetKnowledgeSearch connects the shared public knowledge search, which
// suggests articles while customers write requests.
func (s *CustomerPortalService) SetKnowledgeSearch(search *HelpcenterAISearchService) {
	s.knowledge = search
}

// SuggestArticles returns up to five published help center articles for the
// text of a request being written, from the help spaces chosen for customers:
// the same selection as the Messenger help tab. Text too short to search
// returns none, as does a portal that takes no requests, a workspace without
// a published help center, or one with no help spaces selected. Only articles
// with a public address are suggested, so every suggestion can be opened.
// A signed-in customer also gets articles of help spaces for signed-in
// customers, ahead of public ones.
func (s *CustomerPortalService) SuggestArticles(ctx context.Context, ws *PortalWorkspace, text string, signedIn bool) (PortalArticleSuggestions, error) {
	suggestions := PortalArticleSuggestions{Articles: []PortalArticleSuggestion{}}
	query, ok := portalSuggestionQuery(text)
	intake := ws.Settings.PortalIntakeEnabled || ws.Settings.PortalAnonymousIntakeEnabled
	if !ok || !intake || s.knowledge == nil || s.inbox == nil || s.inbox.docsHelpcenterRepo == nil {
		return suggestions, nil
	}
	cfg, err := s.inbox.docsHelpcenterRepo.GetConfig(ctx, ws.ID)
	if err != nil {
		return suggestions, err
	}
	if cfg == nil || !cfg.IsPublished {
		return suggestions, nil
	}
	if signedIn {
		private, err := s.searchCustomerHelp(ctx, ws, "portal_intake", query, portalSuggestionLimit)
		if err != nil {
			return suggestions, err
		}
		for _, article := range private {
			suggestions.Articles = append(suggestions.Articles, PortalArticleSuggestion{
				ID: article.documentID, Title: article.Title, SpaceName: article.SpaceName,
				Snippet: article.Snippet, PortalArticleID: article.ID,
			})
		}
		suggestions.SearchCompleted = true
	}
	spaces, err := s.inbox.resolveWidgetHelpSpaces(ctx, ws.ID, ws.Settings.WidgetHelpSpaceIDs)
	if err != nil || len(spaces) == 0 {
		return suggestions, err
	}
	remaining := portalSuggestionLimit - len(suggestions.Articles)
	if remaining <= 0 {
		return suggestions, nil
	}
	results, err := s.inbox.searchWidgetHelp(ctx, "portal_intake", ws.ID, defaultHelpcenterLocale(cfg), query, spaces, remaining)
	if err != nil {
		return suggestions, err
	}
	suggestions.SearchCompleted = true
	if len(results) == 0 {
		return suggestions, nil
	}
	if len(results) > remaining {
		results = results[:remaining]
	}
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.ID)
	}
	urls, err := supportPublicArticleURLs(ctx, s.inbox.docsHelpcenterRepo, ws.ID, ids)
	if err != nil {
		return suggestions, err
	}
	for _, result := range results {
		url := urls[result.ID]
		if url == "" {
			continue
		}
		suggestions.Articles = append(suggestions.Articles, PortalArticleSuggestion{
			ID: result.ID, Title: result.Title, SpaceName: result.SpaceName,
			Snippet: portalSuggestionSnippet(result), URL: url,
		})
	}
	return suggestions, nil
}

// portalSuggestionQuery normalizes request text for search: whitespace is
// collapsed and the text is cut to the answer query limit. It reports false
// when the text is too short to search meaningfully.
func portalSuggestionQuery(text string) (string, bool) {
	words := strings.Fields(text)
	query := strings.Join(words, " ")
	if len(words) < portalSuggestionMinWords || utf8.RuneCountInString(query) < portalSuggestionMinChars {
		return "", false
	}
	if runes := []rune(query); len(runes) > portalSuggestionMaxChars {
		query = strings.TrimSpace(string(runes[:portalSuggestionMaxChars]))
	}
	return query, true
}

// portalSuggestionSnippet is the best matching passage, else the excerpt, as
// plain text: full-text matches arrive highlighted with <mark> and escaped.
func portalSuggestionSnippet(result model.PublicSearchResultResponse) string {
	snippet := strings.TrimSpace(derefString(result.Excerpt))
	for _, match := range result.Matches {
		if text := strings.TrimSpace(match.Snippet); text != "" {
			snippet = text
			break
		}
	}
	return html.UnescapeString(portalSnippetMarkup.Replace(snippet))
}

var portalSnippetMarkup = strings.NewReplacer("<mark>", "", "</mark>", "")
