package service

import (
	"context"
	"errors"
	"strings"
	"time"

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
func (s *CustomerPortalService) HelpArticles(ctx context.Context, ws *PortalWorkspace) ([]PortalHelpArticle, error) {
	repo, locale, err := s.customerHelpSource(ctx, ws.ID)
	if err != nil || repo == nil {
		return []PortalHelpArticle{}, err
	}
	rows, err := repo.ListCustomerHelpArticles(ctx, ws.ID, locale)
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
func (s *CustomerPortalService) HelpArticle(ctx context.Context, ws *PortalWorkspace, publicID string) (*PortalHelpArticleDetail, error) {
	repo, locale, err := s.customerHelpSource(ctx, ws.ID)
	if err != nil {
		return nil, err
	}
	publicID = strings.TrimSpace(publicID)
	if repo == nil || publicID == "" {
		return nil, ErrPortalHelpArticleNotFound
	}
	row, err := repo.GetCustomerHelpArticle(ctx, ws.ID, locale, publicID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrPortalHelpArticleNotFound
	}
	content, err := renderPublicArticleHTML(ctx, repo, ws.ID, locale, row.Content)
	if err != nil {
		return nil, err
	}
	return &PortalHelpArticleDetail{PortalHelpArticle: portalHelpArticle(*row), ContentHTML: content}, nil
}

// SearchHelpArticles searches the help spaces for signed-in customers by
// meaning, with keyword matching when embeddings are unavailable. The caller
// must hold a portal session.
func (s *CustomerPortalService) SearchHelpArticles(ctx context.Context, ws *PortalWorkspace, query string) ([]PortalHelpArticle, error) {
	query = strings.Join(strings.Fields(query), " ")
	if runes := []rune(query); len(runes) > portalHelpQueryMax {
		query = string(runes[:portalHelpQueryMax])
	}
	if query == "" {
		return []PortalHelpArticle{}, nil
	}
	return s.searchCustomerHelp(ctx, ws, "portal_help_search", query, portalHelpSearchLimit)
}

// searchCustomerHelp returns relevant articles of help spaces for signed-in
// customers, best first, each with its best matching passage.
func (s *CustomerPortalService) searchCustomerHelp(ctx context.Context, ws *PortalWorkspace, surface, query string, limit int) ([]PortalHelpArticle, error) {
	repo, locale, err := s.customerHelpSource(ctx, ws.ID)
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
	rows, err := repo.ListCustomerHelpArticles(ctx, ws.ID, locale)
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

// customerHelpSource returns the help center repository and the help center
// default locale, or a nil repository when the workspace has no published
// help center.
func (s *CustomerPortalService) customerHelpSource(ctx context.Context, workspaceID string) (*repository.DocsHelpcenterRepository, string, error) {
	if s.inbox == nil || s.inbox.docsHelpcenterRepo == nil {
		return nil, "", nil
	}
	repo := s.inbox.docsHelpcenterRepo
	cfg, err := repo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, "", err
	}
	if cfg == nil || !cfg.IsPublished {
		return nil, "", nil
	}
	return repo, defaultHelpcenterLocale(cfg), nil
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
