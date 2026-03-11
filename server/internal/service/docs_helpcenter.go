package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// DocsHelpcenterService handles help center publishing, config, and slug management.
type DocsHelpcenterService struct {
	hcRepo         *repository.DocsHelpcenterRepository
	docRepo        *repository.DocsDocumentRepository
	spaceRepo      *repository.DocsSpaceRepository
	collectionRepo *repository.DocsCollectionRepository
	s3Client       *storage.S3Client
}

// NewDocsHelpcenterService creates a new DocsHelpcenterService.
func NewDocsHelpcenterService(
	hcRepo *repository.DocsHelpcenterRepository,
	docRepo *repository.DocsDocumentRepository,
	spaceRepo *repository.DocsSpaceRepository,
	collectionRepo *repository.DocsCollectionRepository,
	s3Client *storage.S3Client,
) *DocsHelpcenterService {
	return &DocsHelpcenterService{hcRepo: hcRepo, docRepo: docRepo, spaceRepo: spaceRepo, collectionRepo: collectionRepo, s3Client: s3Client}
}

// UploadAsset uploads a help center asset (logo or favicon) to S3 and returns the public URL.
func (s *DocsHelpcenterService) UploadAsset(ctx context.Context, workspaceID, assetType, contentType string, size int64, body io.Reader) (string, error) {
	if s.s3Client == nil {
		return "", fmt.Errorf("file storage not configured")
	}

	ext := ".png"
	switch contentType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/svg+xml":
		ext = ".svg"
	case "image/x-icon", "image/vnd.microsoft.icon":
		ext = ".ico"
	}

	key := fmt.Sprintf("helpcenter/%s/%s/%s%s", workspaceID, assetType, uuid.New().String(), ext)
	if err := s.s3Client.PutObject(ctx, key, contentType, size, body, true); err != nil {
		return "", fmt.Errorf("upload helpcenter asset: %w", err)
	}

	return s.s3Client.PublicURL(key), nil
}

// GetConfig returns the help center config for a workspace.
func (s *DocsHelpcenterService) GetConfig(ctx context.Context, workspaceID string) (*model.DocsHelpcenterConfig, error) {
	return s.hcRepo.GetConfig(ctx, workspaceID)
}

// UpsertConfig creates or updates the help center config.
func (s *DocsHelpcenterService) UpsertConfig(ctx context.Context, workspaceID string, req model.UpdateDocsHelpcenterConfigRequest) (*model.DocsHelpcenterConfig, error) {
	updates := map[string]interface{}{}
	if req.Subdomain != nil {
		updates["subdomain"] = *req.Subdomain
	}
	if req.CustomDomain != nil {
		updates["custom_domain"] = req.CustomDomain
	}
	if req.BrandName != nil {
		updates["brand_name"] = *req.BrandName
	}
	if req.BrandLogoURL != nil {
		updates["brand_logo_url"] = req.BrandLogoURL
	}
	if req.BrandLogoDarkURL != nil {
		updates["brand_logo_dark_url"] = req.BrandLogoDarkURL
	}
	if req.BrandColor != nil {
		updates["brand_color"] = *req.BrandColor
	}
	if req.IsPublished != nil {
		updates["is_published"] = *req.IsPublished
	}
	if req.SEOTitle != nil {
		updates["seo_title"] = req.SEOTitle
	}
	if req.SEODescription != nil {
		updates["seo_description"] = req.SEODescription
	}
	if req.SupportEmail != nil {
		updates["support_email"] = req.SupportEmail
	}
	if req.FaviconURL != nil {
		updates["favicon_url"] = req.FaviconURL
	}
	if req.ThemeMode != nil {
		updates["theme_mode"] = *req.ThemeMode
	}
	if req.HeaderLinks != nil {
		updates["header_links"] = req.HeaderLinks
	}
	if req.FooterConfig != nil {
		updates["footer_config"] = req.FooterConfig
	}
	if req.HomepageConfig != nil {
		updates["homepage_config"] = req.HomepageConfig
	}
	if req.SpaceNavConfig != nil {
		updates["space_nav_config"] = req.SpaceNavConfig
	}
	if req.SearchPlaceholder != nil {
		updates["search_placeholder"] = req.SearchPlaceholder
	}
	return s.hcRepo.UpsertConfig(ctx, workspaceID, updates)
}

// PublishExternally publishes a help center article externally.
func (s *DocsHelpcenterService) PublishExternally(ctx context.Context, documentID string, slug string) error {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	// Validate eligibility.
	if doc.Status != model.DocStatusPublished {
		return fmt.Errorf("document must be internally published first")
	}

	space, err := s.spaceRepo.GetByID(ctx, doc.SpaceID)
	if err != nil {
		return err
	}
	if space == nil || space.Type != model.SpaceTypeExternalCapable {
		return fmt.Errorf("document must be in an external-capable space")
	}

	if slug == "" {
		slug = slugify(doc.Title)
	}

	// Ensure article extension exists.
	art, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return err
	}
	if art == nil {
		art = &model.DocsHelpcenterArticle{DocumentID: documentID}
		if _, err := s.hcRepo.CreateArticle(ctx, art); err != nil {
			return err
		}
	}

	// Enforce slug uniqueness within the space — append -2, -3, etc. on collision.
	baseSlug := slug
	for i := 2; ; i++ {
		taken, err := s.hcRepo.SlugExistsInSpace(ctx, doc.SpaceID, slug, documentID)
		if err != nil {
			return fmt.Errorf("check slug uniqueness: %w", err)
		}
		if !taken {
			break
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, i)
	}

	// Alias the old slug if it changed (so old URLs redirect).
	if art.Slug != "" && art.Slug != slug {
		_ = s.hcRepo.CreateSlugAlias(ctx, &model.DocsSlugAlias{
			WorkspaceID: doc.WorkspaceID,
			DocumentID:  documentID,
			OldSlug:     art.Slug,
		})
	}

	// Set slug and public_published_at.
	if err := s.hcRepo.SetSlug(ctx, documentID, slug); err != nil {
		return err
	}
	now := time.Now()
	return s.hcRepo.SetPublicPublishedAt(ctx, documentID, &now)
}

// UnpublishExternally removes a help center article from public access.
func (s *DocsHelpcenterService) UnpublishExternally(ctx context.Context, documentID string) error {
	art, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return err
	}
	if art == nil {
		return nil // Not published, no-op.
	}
	return s.hcRepo.SetPublicPublishedAt(ctx, documentID, nil)
}

// CreateSlugAlias records an old slug alias for redirect.
func (s *DocsHelpcenterService) CreateSlugAlias(ctx context.Context, workspaceID, documentID, oldSlug string) error {
	alias := &model.DocsSlugAlias{
		WorkspaceID: workspaceID,
		DocumentID:  documentID,
		OldSlug:     oldSlug,
	}
	return s.hcRepo.CreateSlugAlias(ctx, alias)
}

// ResolveSlug resolves a slug to a document ID, checking aliases if needed.
func (s *DocsHelpcenterService) ResolveSlug(ctx context.Context, workspaceID, slug string) (documentID string, isAlias bool, err error) {
	// First check slug aliases.
	alias, err := s.hcRepo.FindAliasBySlug(ctx, workspaceID, slug)
	if err != nil {
		return "", false, err
	}
	if alias != nil {
		return alias.DocumentID, true, nil
	}
	return "", false, nil
}

// GetArticle returns the help center article extension for a document.
func (s *DocsHelpcenterService) GetArticle(ctx context.Context, documentID string) (*model.DocsHelpcenterArticle, error) {
	return s.hcRepo.GetArticle(ctx, documentID)
}

// IncrementViewCount increments article view count.
func (s *DocsHelpcenterService) IncrementViewCount(ctx context.Context, documentID string) error {
	return s.hcRepo.IncrementViewCount(ctx, documentID)
}

// ─── Public Help Center API ──────────────────────────────────────────────────

// GetConfigBySubdomain returns the help center config by subdomain.
// It enriches featured card titles with current collection names so
// renaming a collection is immediately reflected on the public homepage.
func (s *DocsHelpcenterService) GetConfigBySubdomain(ctx context.Context, subdomain string) (*model.DocsHelpcenterConfig, error) {
	cfg, err := s.hcRepo.GetConfigBySubdomain(ctx, subdomain)
	if err != nil || cfg == nil {
		return cfg, err
	}
	s.enrichFeaturedCardTitles(ctx, cfg)
	return cfg, nil
}

// ResolveConfig resolves a help center config by subdomain first, then by custom domain.
func (s *DocsHelpcenterService) ResolveConfig(ctx context.Context, identifier string) (*model.DocsHelpcenterConfig, error) {
	// Try subdomain first.
	cfg, err := s.hcRepo.GetConfigBySubdomain(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		s.enrichFeaturedCardTitles(ctx, cfg)
		return cfg, nil
	}

	// Fall back to custom domain lookup.
	cfg, err = s.hcRepo.GetConfigByCustomDomain(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		s.enrichFeaturedCardTitles(ctx, cfg)
		return cfg, nil
	}

	return nil, nil
}

// GetConfigByCustomDomain returns a help center config by its custom domain.
func (s *DocsHelpcenterService) GetConfigByCustomDomain(ctx context.Context, domain string) (*model.DocsHelpcenterConfig, error) {
	return s.hcRepo.GetConfigByCustomDomain(ctx, domain)
}

// enrichFeaturedCardTitles resolves current collection names into
// homepage_config.featured_cards so the public site stays up-to-date.
func (s *DocsHelpcenterService) enrichFeaturedCardTitles(ctx context.Context, cfg *model.DocsHelpcenterConfig) {
	if len(cfg.HomepageConfig) == 0 {
		return
	}
	var hpCfg model.HelpcenterHomepageConfig
	if err := json.Unmarshal(cfg.HomepageConfig, &hpCfg); err != nil {
		return
	}
	if len(hpCfg.FeaturedCards) == 0 {
		return
	}

	changed := false
	for i, card := range hpCfg.FeaturedCards {
		if card.LinkType != "collection" || card.LinkValue == "" {
			continue
		}
		col, err := s.collectionRepo.GetByID(ctx, card.LinkValue)
		if err != nil || col == nil {
			continue
		}
		if col.Name != card.Title {
			hpCfg.FeaturedCards[i].Title = col.Name
			changed = true
		}
	}

	if changed {
		if enriched, err := json.Marshal(hpCfg); err == nil {
			cfg.HomepageConfig = enriched
		}
	}
}

// ListPublicSpaces returns external-capable spaces for the public help center.
func (s *DocsHelpcenterService) ListPublicSpaces(ctx context.Context, workspaceID string) ([]model.PublicSpaceResponse, error) {
	spaces, err := s.spaceRepo.ListPublicByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]model.PublicSpaceResponse, len(spaces))
	for i, sp := range spaces {
		result[i] = model.PublicSpaceResponse{
			ID:          sp.ID,
			Name:        sp.Name,
			Slug:        sp.Slug,
			Icon:        sp.Icon,
			Description: nil, // DocsSpace doesn't have Description — omit
		}
	}
	return result, nil
}

// GetSpaceNavigation returns the sidebar navigation tree for a space.
func (s *DocsHelpcenterService) GetSpaceNavigation(ctx context.Context, workspaceID, spaceSlug string) ([]model.PublicNavCollection, error) {
	space, err := s.spaceRepo.GetBySlug(ctx, workspaceID, spaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}
	return s.hcRepo.ListSpaceNavigation(ctx, space.ID)
}

// GetPublicArticle returns the full article detail for the help center.
func (s *DocsHelpcenterService) GetPublicArticle(ctx context.Context, workspaceID, spaceSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	space, err := s.spaceRepo.GetBySlug(ctx, workspaceID, spaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}

	doc, ha, content, err := s.hcRepo.GetPublicArticleBySlug(ctx, space.ID, articleSlug)
	if err != nil {
		return nil, err
	}
	if doc == nil || ha == nil {
		return nil, fmt.Errorf("article not found")
	}

	// Resolve collection name if present.
	var collectionName *string
	if doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionName = &coll.Name
		}
	}

	// Render TipTap JSON → HTML for public display.
	var contentHTML *string
	if content != nil && len(content.Content) > 0 {
		rendered, err := tiptap.RenderHTML(content.Content)
		if err == nil && rendered != "" {
			contentHTML = &rendered
		}
	}

	var publishedAt *string
	if ha.PublicPublishedAt != nil {
		s := ha.PublicPublishedAt.Format(time.RFC3339)
		publishedAt = &s
	}

	// Increment view count asynchronously.
	go func() { _ = s.hcRepo.IncrementViewCount(ctx, doc.ID) }()

	return &model.PublicArticleResponse{
		ID:              doc.ID,
		Title:           doc.Title,
		Slug:            ha.Slug,
		Excerpt:         doc.Excerpt,
		Icon:            doc.Icon,
		Status:          doc.Status,
		CollectionID:    doc.CollectionID,
		CollectionName:  collectionName,
		PublishedAt:     publishedAt,
		SEOTitle:        ha.SEOTitle,
		SEODescription:  ha.SEODescription,
		HelpfulCount:    ha.HelpfulCount,
		NotHelpfulCount: ha.NotHelpfulCount,
		ViewCount:       ha.ViewCount,
		ContentHTML:     contentHTML,
	}, nil
}

// SubmitFeedback records article feedback and updates counts.
func (s *DocsHelpcenterService) SubmitFeedback(ctx context.Context, documentID string, req model.DocsArticleFeedbackRequest) error {
	fb := &model.DocsArticleFeedback{
		DocumentID: documentID,
		IsHelpful:  req.IsHelpful,
		Comment:    req.Comment,
		SessionID:  req.SessionID,
	}
	if _, err := s.hcRepo.CreateFeedback(ctx, fb); err != nil {
		return err
	}
	return s.hcRepo.IncrementFeedbackCount(ctx, documentID, req.IsHelpful)
}
