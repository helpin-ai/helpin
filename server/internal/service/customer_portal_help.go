package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrPortalHelpArticleNotFound reports a portal help article that does not
// exist or is not live for signed-in customers.
var ErrPortalHelpArticleNotFound = errors.New("help article not found")

const (
	portalHelpSearchLimit = 10
	portalHelpQueryMax    = 200
)

// PortalHelpArticle is a help article for signed-in portal customers, listed
// or found by search. Its ID is the article's public ID.
type PortalHelpArticle struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Excerpt        string    `json:"excerpt,omitempty"`
	Snippet        string    `json:"snippet,omitempty"`
	SpaceName      string    `json:"space_name"`
	CollectionName string    `json:"collection_name,omitempty"`
	PublishedAt    time.Time `json:"published_at"`
	documentID     string
}

// PortalHelpArticleDetail is one help article with its rendered content.
type PortalHelpArticleDetail struct {
	PortalHelpArticle
	ContentHTML string `json:"content_html"`
}

// HelpArticles lists the articles of help spaces for signed-in customers.
// The caller must hold a portal session.
func (s *CustomerPortalService) HelpArticles(ctx context.Context, ws *PortalWorkspace, locale string) ([]PortalHelpArticle, error) {
	repo, locales, err := s.customerHelpSource(ctx, ws.ID, locale)
	if err != nil || repo == nil {
		return []PortalHelpArticle{}, err
	}
	rows, err := repo.ListCustomerHelpArticles(ctx, ws.ID, locales.base, locales.reader)
	if err != nil {
		return nil, err
	}
	articles := make([]PortalHelpArticle, 0, len(rows))
	for _, row := range rows {
		articles = append(articles, portalHelpArticle(row))
	}
	return articles, nil
}

// HelpArticle returns one article of a help space for signed-in customers,
// rendered like a public help center article. The caller must hold a portal
// session.
func (s *CustomerPortalService) HelpArticle(ctx context.Context, ws *PortalWorkspace, publicID, locale string) (*PortalHelpArticleDetail, error) {
	repo, locales, err := s.customerHelpSource(ctx, ws.ID, locale)
	if err != nil {
		return nil, err
	}
	publicID = strings.TrimSpace(publicID)
	if repo == nil || publicID == "" {
		return nil, ErrPortalHelpArticleNotFound
	}
	row, err := repo.GetCustomerHelpArticle(ctx, ws.ID, locales.base, locales.reader, publicID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrPortalHelpArticleNotFound
	}
	content, err := renderPublicArticleHTML(ctx, repo, ws.ID, row.Locale, row.Content)
	if err != nil {
		return nil, err
	}
	return &PortalHelpArticleDetail{PortalHelpArticle: portalHelpArticle(*row), ContentHTML: content}, nil
}

// SearchHelpArticles searches the help spaces for signed-in customers by
// meaning, with keyword matching when embeddings are unavailable. The caller
// must hold a portal session.
func (s *CustomerPortalService) SearchHelpArticles(ctx context.Context, ws *PortalWorkspace, query, locale string) ([]PortalHelpArticle, error) {
	query = strings.Join(strings.Fields(query), " ")
	if runes := []rune(query); len(runes) > portalHelpQueryMax {
		query = string(runes[:portalHelpQueryMax])
	}
	if query == "" {
		return []PortalHelpArticle{}, nil
	}
	return s.searchCustomerHelp(ctx, ws, "portal_help_search", query, locale, portalHelpSearchLimit)
}

// searchCustomerHelp returns relevant articles of help spaces for signed-in
// customers, best first, each with its best matching passage.
func (s *CustomerPortalService) searchCustomerHelp(ctx context.Context, ws *PortalWorkspace, surface, query, locale string, limit int) ([]PortalHelpArticle, error) {
	repo, locales, err := s.customerHelpSource(ctx, ws.ID, locale)
	if err != nil || repo == nil || s.knowledge == nil {
		return []PortalHelpArticle{}, err
	}
	spaceIDs, err := repo.ListCustomerHelpSpaceIDs(ctx, ws.ID)
	if err != nil || len(spaceIDs) == 0 {
		return []PortalHelpArticle{}, err
	}
	groups, err := s.knowledge.searchCustomerChunks(WithKnowledgeSurface(ctx, surface), ws.ID, spaceIDs, query)
	if err != nil || len(groups) == 0 {
		return []PortalHelpArticle{}, err
	}
	rows, err := repo.ListCustomerHelpArticles(ctx, ws.ID, locales.base, locales.reader)
	if err != nil {
		return nil, err
	}
	byDocument := make(map[string]repository.CustomerHelpArticle, len(rows))
	for _, row := range rows {
		byDocument[row.DocumentID] = row
	}
	results := make([]PortalHelpArticle, 0, limit)
	for _, group := range groups {
		row, ok := byDocument[group.documentID]
		if !ok {
			continue
		}
		article := portalHelpArticle(row)
		article.Snippet = excerptText(group.chunks[0].Content, helpcenterSnippetChars)
		results = append(results, article)
		if len(results) == limit {
			break
		}
	}
	return results, nil
}

// helpLocales are the help center's default locale and the locale an
// article is read in when translated.
type helpLocales struct{ base, reader string }

// customerHelpSource returns the help center repository and the locales to
// read in, or a nil repository when the workspace has no published help
// center. requested applies only when the help center offers it.
func (s *CustomerPortalService) customerHelpSource(ctx context.Context, workspaceID, requested string) (*repository.DocsHelpcenterRepository, helpLocales, error) {
	if s.inbox == nil || s.inbox.docsHelpcenterRepo == nil {
		return nil, helpLocales{}, nil
	}
	repo := s.inbox.docsHelpcenterRepo
	cfg, err := repo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, helpLocales{}, err
	}
	if cfg == nil || !cfg.IsPublished {
		return nil, helpLocales{}, nil
	}
	base := defaultHelpcenterLocale(cfg)
	return repo, helpLocales{base: base, reader: readerHelpLocale(cfg, requested)}, nil
}

// readerHelpLocale picks the help center locale closest to a reader's
// language ("de-AT" reads "de-AT", then "de"), or the default locale.
func readerHelpLocale(cfg *model.DocsHelpcenterConfig, requested string) string {
	requested = strings.ToLower(strings.TrimSpace(requested))
	candidates := []string{requested}
	if base, _, found := strings.Cut(requested, "-"); found {
		candidates = append(candidates, base)
	}
	for _, candidate := range candidates {
		for _, enabled := range cfg.EnabledLocales {
			if candidate != "" && strings.EqualFold(enabled, candidate) {
				return enabled
			}
		}
	}
	return defaultHelpcenterLocale(cfg)
}

func portalHelpArticle(row repository.CustomerHelpArticle) PortalHelpArticle {
	return PortalHelpArticle{
		ID:             row.PublicID,
		Title:          row.Title,
		Excerpt:        strings.TrimSpace(derefString(row.Excerpt)),
		SpaceName:      row.SpaceName,
		CollectionName: derefString(row.CollectionName),
		PublishedAt:    row.PublishedAt,
		documentID:     row.DocumentID,
	}
}
