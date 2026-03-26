package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	contentRepo    *repository.DocsContentRepository
	spaceRepo      *repository.DocsSpaceRepository
	collectionRepo *repository.DocsCollectionRepository
	redirectRepo   *repository.DocsRedirectRepository
	s3Client       *storage.S3Client
	translationSvc *DocsHelpcenterTranslationService
}

// NewDocsHelpcenterService creates a new DocsHelpcenterService.
func NewDocsHelpcenterService(
	hcRepo *repository.DocsHelpcenterRepository,
	docRepo *repository.DocsDocumentRepository,
	contentRepo *repository.DocsContentRepository,
	spaceRepo *repository.DocsSpaceRepository,
	collectionRepo *repository.DocsCollectionRepository,
	redirectRepo *repository.DocsRedirectRepository,
	s3Client *storage.S3Client,
) *DocsHelpcenterService {
	return &DocsHelpcenterService{hcRepo: hcRepo, docRepo: docRepo, contentRepo: contentRepo, spaceRepo: spaceRepo, collectionRepo: collectionRepo, redirectRepo: redirectRepo, s3Client: s3Client}
}

func (s *DocsHelpcenterService) SetTranslationService(translationSvc *DocsHelpcenterTranslationService) {
	s.translationSvc = translationSvc
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
	if req.DefaultLocale != nil {
		updates["default_locale"] = *req.DefaultLocale
	}
	if req.EnabledLocales != nil {
		updates["enabled_locales"] = model.DocsStringArray(req.EnabledLocales)
	}
	if req.ProtectedTerms != nil {
		updates["protected_terms"] = model.DocsStringArray(req.ProtectedTerms)
	}
	if req.ShowLanguageSwitcher != nil {
		updates["show_language_switcher"] = *req.ShowLanguageSwitcher
	}
	if req.FallbackToDefaultLocale != nil {
		updates["fallback_to_default_locale"] = *req.FallbackToDefaultLocale
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
		oldSlug := art.Slug
		_ = s.hcRepo.CreateSlugAlias(ctx, &model.DocsSlugAlias{
			WorkspaceID: doc.WorkspaceID,
			DocumentID:  documentID,
			OldSlug:     oldSlug,
		})

		// Also create a DocsRedirect for canonical path-based redirect resolution.
		collectionSlug := ""
		if doc.CollectionID != nil {
			collection, _ := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
			if collection != nil {
				collectionSlug = collection.Slug
			}
		}
		newSlug := slug
		redirect := &model.DocsRedirect{
			WorkspaceID:          doc.WorkspaceID,
			SourcePath:           "/" + collectionSlug + "/" + oldSlug,
			TargetCollectionSlug: collectionSlug,
			TargetArticleSlug:    &newSlug,
			Type:                 model.RedirectTypeSlugChange,
		}
		if err := s.redirectRepo.Create(ctx, redirect); err != nil {
			slog.ErrorContext(ctx, "create slug change redirect", "error", err, "document_id", documentID)
			// Non-fatal: continue
		}
	}

	// Set slug and public_published_at.
	if err := s.hcRepo.SetSlug(ctx, documentID, slug); err != nil {
		return err
	}
	now := time.Now()
	if err := s.hcRepo.SetPublicPublishedAt(ctx, documentID, &now); err != nil {
		return err
	}
	if s.translationSvc != nil {
		if err := s.translationSvc.RefreshArticleSource(ctx, documentID); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter article translation source after publish", "document_id", documentID, "error", err)
		}
	}
	return nil
}

// UpdateArticleSlug changes the slug of a help center article and creates a redirect from the old slug.
func (s *DocsHelpcenterService) UpdateArticleSlug(ctx context.Context, workspaceID, documentID, newSlug string) error {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return fmt.Errorf("document not found")
	}

	art, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return err
	}
	if art == nil || art.Slug == "" {
		return fmt.Errorf("article has no help center slug")
	}

	oldSlug := art.Slug
	if oldSlug == newSlug {
		return nil
	}

	// Resolve collection slug for redirect target.
	var collectionSlug string
	if doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionSlug = coll.Slug
		}
	}

	// Update the slug.
	if err := s.hcRepo.SetSlug(ctx, documentID, newSlug); err != nil {
		return err
	}

	// Create a redirect from old slug to new slug.
	if collectionSlug != "" {
		redirect := &model.DocsRedirect{
			WorkspaceID:          workspaceID,
			SourcePath:           "/" + collectionSlug + "/" + oldSlug,
			TargetCollectionSlug: collectionSlug,
			TargetArticleSlug:    &newSlug,
			Type:                 "slug_change",
		}
		_ = s.redirectRepo.Create(ctx, redirect)
	}

	return nil
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
	if err := s.hcRepo.SetPublicPublishedAt(ctx, documentID, nil); err != nil {
		return err
	}
	if s.translationSvc != nil {
		if err := s.translationSvc.RefreshArticleSource(ctx, documentID); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter article translation source after unpublish", "document_id", documentID, "error", err)
		}
	}
	return nil
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

// CreateArticle creates a help center article extension record.
func (s *DocsHelpcenterService) CreateArticle(ctx context.Context, art *model.DocsHelpcenterArticle) (*model.DocsHelpcenterArticle, error) {
	return s.hcRepo.CreateArticle(ctx, art)
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

func defaultHelpcenterLocale(cfg *model.DocsHelpcenterConfig) string {
	if cfg != nil && cfg.DefaultLocale != "" {
		return cfg.DefaultLocale
	}
	return "en"
}

func (s *DocsHelpcenterService) getPublicLocaleConfig(ctx context.Context, workspaceID string) (*model.DocsHelpcenterConfig, string, error) {
	cfg, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, "", err
	}
	if cfg == nil {
		return nil, "", fmt.Errorf("help center config not found")
	}
	return cfg, defaultHelpcenterLocale(cfg), nil
}

func (s *DocsHelpcenterService) resolvePublicSpaceTranslationBySlug(ctx context.Context, cfg *model.DocsHelpcenterConfig, workspaceID, requestedLocale, slug string) (*model.DocsHelpcenterSpaceTranslation, string, bool, error) {
	translation, err := s.hcRepo.GetPublicSpaceTranslationBySlug(ctx, workspaceID, requestedLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if translation != nil {
		return translation, requestedLocale, false, nil
	}

	defaultLocale := defaultHelpcenterLocale(cfg)
	if !cfg.FallbackToDefaultLocale || requestedLocale == defaultLocale {
		return nil, "", false, fmt.Errorf("space not found")
	}

	fallback, err := s.hcRepo.GetPublicSpaceTranslationBySlug(ctx, workspaceID, defaultLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if fallback == nil {
		return nil, "", false, fmt.Errorf("space not found")
	}
	return fallback, defaultLocale, true, nil
}

func (s *DocsHelpcenterService) resolvePublicCollectionTranslationBySlug(ctx context.Context, cfg *model.DocsHelpcenterConfig, spaceID, requestedLocale, slug string) (*model.DocsHelpcenterCollectionTranslation, string, bool, error) {
	translation, err := s.hcRepo.GetPublicCollectionTranslationBySlug(ctx, spaceID, requestedLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if translation != nil {
		return translation, requestedLocale, false, nil
	}

	defaultLocale := defaultHelpcenterLocale(cfg)
	if !cfg.FallbackToDefaultLocale || requestedLocale == defaultLocale {
		return nil, "", false, fmt.Errorf("collection not found")
	}

	fallback, err := s.hcRepo.GetPublicCollectionTranslationBySlug(ctx, spaceID, defaultLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if fallback == nil {
		return nil, "", false, fmt.Errorf("collection not found")
	}
	return fallback, defaultLocale, true, nil
}

func (s *DocsHelpcenterService) resolvePublicArticleTranslationBySlug(ctx context.Context, cfg *model.DocsHelpcenterConfig, spaceID string, collectionID *string, requestedLocale, slug string) (*model.DocsHelpcenterArticleTranslation, string, bool, error) {
	translation, err := s.hcRepo.GetPublicArticleTranslationBySlug(ctx, spaceID, collectionID, requestedLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if translation != nil {
		return translation, requestedLocale, false, nil
	}

	defaultLocale := defaultHelpcenterLocale(cfg)
	if !cfg.FallbackToDefaultLocale || requestedLocale == defaultLocale {
		return nil, "", false, fmt.Errorf("article not found")
	}

	fallback, err := s.hcRepo.GetPublicArticleTranslationBySlug(ctx, spaceID, collectionID, defaultLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if fallback == nil {
		return nil, "", false, fmt.Errorf("article not found")
	}
	return fallback, defaultLocale, true, nil
}

// ListPublicSpaces returns external-capable spaces for the public help center.
func (s *DocsHelpcenterService) ListPublicSpaces(ctx context.Context, workspaceID, requestedLocale string) ([]model.PublicSpaceResponse, error) {
	cfg, defaultLocale, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	spaces, err := s.spaceRepo.ListPublicByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	requestedTranslations, err := s.hcRepo.ListPublicSpaceTranslations(ctx, workspaceID, requestedLocale)
	if err != nil {
		return nil, err
	}
	requestedBySpaceID := make(map[string]model.DocsHelpcenterSpaceTranslation, len(requestedTranslations))
	for _, translation := range requestedTranslations {
		requestedBySpaceID[translation.SpaceID] = translation
	}

	fallbackBySpaceID := map[string]model.DocsHelpcenterSpaceTranslation{}
	if cfg.FallbackToDefaultLocale && requestedLocale != defaultLocale {
		fallbackTranslations, err := s.hcRepo.ListPublicSpaceTranslations(ctx, workspaceID, defaultLocale)
		if err != nil {
			return nil, err
		}
		for _, translation := range fallbackTranslations {
			fallbackBySpaceID[translation.SpaceID] = translation
		}
	}

	result := make([]model.PublicSpaceResponse, 0, len(spaces))
	for _, sp := range spaces {
		translation, ok := requestedBySpaceID[sp.ID]
		if !ok {
			translation, ok = fallbackBySpaceID[sp.ID]
			if !ok {
				continue
			}
		}
		result = append(result, model.PublicSpaceResponse{
			ID:          sp.ID,
			Name:        translation.Name,
			Slug:        translation.Slug,
			Icon:        sp.Icon,
			Description: translation.Description,
		})
	}
	return result, nil
}

// GetSpaceNavigation returns the sidebar navigation tree for a space.
func (s *DocsHelpcenterService) GetSpaceNavigation(ctx context.Context, workspaceID, requestedLocale, spaceSlug string) ([]model.PublicNavCollection, error) {
	cfg, defaultLocale, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	spaceTranslation, _, _, err := s.resolvePublicSpaceTranslationBySlug(ctx, cfg, workspaceID, requestedLocale, spaceSlug)
	if err != nil {
		return nil, err
	}

	requestedCollections, err := s.hcRepo.ListPublicCollectionTranslations(ctx, spaceTranslation.SpaceID, requestedLocale)
	if err != nil {
		return nil, err
	}
	requestedCollectionByID := make(map[string]model.DocsHelpcenterCollectionTranslation, len(requestedCollections))
	for _, translation := range requestedCollections {
		requestedCollectionByID[translation.CollectionID] = translation
	}

	requestedArticles, err := s.hcRepo.ListPublicArticleTranslationsBySpace(ctx, spaceTranslation.SpaceID, requestedLocale)
	if err != nil {
		return nil, err
	}
	requestedArticleByID := make(map[string]model.DocsHelpcenterArticleTranslation, len(requestedArticles))
	for _, translation := range requestedArticles {
		requestedArticleByID[translation.DocumentID] = translation
	}

	fallbackCollectionByID := map[string]model.DocsHelpcenterCollectionTranslation{}
	fallbackArticleByID := map[string]model.DocsHelpcenterArticleTranslation{}
	if cfg.FallbackToDefaultLocale && requestedLocale != defaultLocale {
		fallbackCollections, err := s.hcRepo.ListPublicCollectionTranslations(ctx, spaceTranslation.SpaceID, defaultLocale)
		if err != nil {
			return nil, err
		}
		for _, translation := range fallbackCollections {
			fallbackCollectionByID[translation.CollectionID] = translation
		}

		fallbackArticles, err := s.hcRepo.ListPublicArticleTranslationsBySpace(ctx, spaceTranslation.SpaceID, defaultLocale)
		if err != nil {
			return nil, err
		}
		for _, translation := range fallbackArticles {
			fallbackArticleByID[translation.DocumentID] = translation
		}
	}

	collections, err := s.collectionRepo.ListBySpace(ctx, spaceTranslation.SpaceID)
	if err != nil {
		return nil, err
	}

	status := model.DocStatusPublished
	spaceID := spaceTranslation.SpaceID
	docs, err := s.docRepo.List(ctx, workspaceID, &spaceID, nil, &status, nil, "", false)
	if err != nil {
		return nil, err
	}

	articlesByCollection := map[string][]model.PublicNavArticle{}
	uncategorized := make([]model.PublicNavArticle, 0)
	for _, doc := range docs {
		translation, ok := requestedArticleByID[doc.ID]
		if !ok {
			translation, ok = fallbackArticleByID[doc.ID]
			if !ok {
				continue
			}
		}

		article := model.PublicNavArticle{ID: doc.ID, Title: translation.Title, Slug: translation.Slug}
		if doc.CollectionID == nil {
			uncategorized = append(uncategorized, article)
			continue
		}

		if _, ok := requestedCollectionByID[*doc.CollectionID]; !ok {
			if _, ok := fallbackCollectionByID[*doc.CollectionID]; !ok {
				continue
			}
		}
		articlesByCollection[*doc.CollectionID] = append(articlesByCollection[*doc.CollectionID], article)
	}

	result := make([]model.PublicNavCollection, 0, len(collections)+1)
	for _, collection := range collections {
		articles, ok := articlesByCollection[collection.ID]
		if !ok || len(articles) == 0 {
			continue
		}

		translation, ok := requestedCollectionByID[collection.ID]
		if !ok {
			translation, ok = fallbackCollectionByID[collection.ID]
			if !ok {
				continue
			}
		}

		result = append(result, model.PublicNavCollection{
			ID:       collection.ID,
			Name:     translation.Name,
			Slug:     translation.Slug,
			Icon:     collection.Icon,
			Articles: articles,
		})
	}

	if len(uncategorized) > 0 {
		result = append(result, model.PublicNavCollection{
			ID:       "uncategorized",
			Name:     "General",
			Slug:     "uncategorized",
			Articles: uncategorized,
		})
	}

	return result, nil
}

// GetPublicArticle returns the full article detail for the locale-aware public help center.
func (s *DocsHelpcenterService) GetPublicArticle(ctx context.Context, workspaceID, requestedLocale, spaceSlug, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	cfg, _, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	spaceTranslation, _, _, err := s.resolvePublicSpaceTranslationBySlug(ctx, cfg, workspaceID, requestedLocale, spaceSlug)
	if err != nil {
		return nil, err
	}

	var (
		collectionName *string
		collectionID   *string
		resolvedColl   *model.DocsHelpcenterCollectionTranslation
	)
	if collectionSlug != "" {
		resolvedColl, _, _, err = s.resolvePublicCollectionTranslationBySlug(ctx, cfg, spaceTranslation.SpaceID, requestedLocale, collectionSlug)
		if err != nil {
			return nil, err
		}
		collectionID = &resolvedColl.CollectionID
		collectionName = &resolvedColl.Name
	}

	translation, resolvedLocale, fellBack, err := s.resolvePublicArticleTranslationBySlug(ctx, cfg, spaceTranslation.SpaceID, collectionID, requestedLocale, articleSlug)
	if err != nil {
		return nil, err
	}

	doc, err := s.docRepo.GetByID(ctx, translation.DocumentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("article not found")
	}

	if collectionName == nil && doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionName = &coll.Name
		}
	}

	var contentHTML *string
	if len(translation.Content) > 0 {
		rendered, err := tiptap.RenderHTML(translation.Content)
		if err == nil && rendered != "" {
			contentHTML = &rendered
		}
	}

	var publishedAt *string
	if translation.PublishedAt != nil {
		formatted := translation.PublishedAt.Format(time.RFC3339)
		publishedAt = &formatted
	}

	go func() {
		_ = s.hcRepo.IncrementTranslatedViewCount(ctx, translation.DocumentID, resolvedLocale)
	}()

	var collectionSlugValue *string
	if resolvedColl != nil {
		collectionSlugValue = &resolvedColl.Slug
	}

	return &model.PublicArticleResponse{
		ID:              doc.ID,
		Title:           translation.Title,
		Slug:            translation.Slug,
		Locale:          resolvedLocale,
		RequestedLocale: requestedLocale,
		IsFallback:      fellBack,
		Excerpt:         translation.Excerpt,
		Icon:            doc.Icon,
		Status:          doc.Status,
		SpaceSlug:       spaceTranslation.Slug,
		CollectionID:    doc.CollectionID,
		CollectionName:  collectionName,
		CollectionSlug:  collectionSlugValue,
		PublishedAt:     publishedAt,
		SEOTitle:        translation.SEOTitle,
		SEODescription:  translation.SEODescription,
		HelpfulCount:    translation.HelpfulCount,
		NotHelpfulCount: translation.NotHelpfulCount,
		ViewCount:       translation.ViewCount,
		ContentHTML:     contentHTML,
	}, nil
}

// GetPublicLocalizedCollection returns a translated collection page and its translated articles.
func (s *DocsHelpcenterService) GetPublicLocalizedCollection(ctx context.Context, workspaceID, requestedLocale, spaceSlug, collectionSlug string) (*model.PublicNavCollection, []model.PublicNavArticle, error) {
	cfg, defaultLocale, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	spaceTranslation, _, _, err := s.resolvePublicSpaceTranslationBySlug(ctx, cfg, workspaceID, requestedLocale, spaceSlug)
	if err != nil {
		return nil, nil, err
	}

	collectionTranslation, _, _, err := s.resolvePublicCollectionTranslationBySlug(ctx, cfg, spaceTranslation.SpaceID, requestedLocale, collectionSlug)
	if err != nil {
		return nil, nil, err
	}

	requestedArticles, err := s.hcRepo.ListPublicArticleTranslationsBySpace(ctx, spaceTranslation.SpaceID, requestedLocale)
	if err != nil {
		return nil, nil, err
	}
	requestedArticleByID := make(map[string]model.DocsHelpcenterArticleTranslation, len(requestedArticles))
	for _, translation := range requestedArticles {
		requestedArticleByID[translation.DocumentID] = translation
	}

	fallbackArticleByID := map[string]model.DocsHelpcenterArticleTranslation{}
	if cfg.FallbackToDefaultLocale && requestedLocale != defaultLocale {
		fallbackArticles, err := s.hcRepo.ListPublicArticleTranslationsBySpace(ctx, spaceTranslation.SpaceID, defaultLocale)
		if err != nil {
			return nil, nil, err
		}
		for _, translation := range fallbackArticles {
			fallbackArticleByID[translation.DocumentID] = translation
		}
	}

	status := model.DocStatusPublished
	spaceID := spaceTranslation.SpaceID
	collectionID := collectionTranslation.CollectionID
	docs, err := s.docRepo.List(ctx, workspaceID, &spaceID, &collectionID, &status, nil, "", false)
	if err != nil {
		return nil, nil, err
	}

	articles := make([]model.PublicNavArticle, 0, len(docs))
	for _, doc := range docs {
		translation, ok := requestedArticleByID[doc.ID]
		if !ok {
			translation, ok = fallbackArticleByID[doc.ID]
			if !ok {
				continue
			}
		}
		articles = append(articles, model.PublicNavArticle{
			ID:    doc.ID,
			Title: translation.Title,
			Slug:  translation.Slug,
		})
	}

	collection, err := s.collectionRepo.GetByID(ctx, collectionTranslation.CollectionID)
	if err != nil {
		return nil, nil, err
	}
	var icon *string
	if collection != nil {
		icon = collection.Icon
	}

	return &model.PublicNavCollection{
		ID:       collectionTranslation.CollectionID,
		Name:     collectionTranslation.Name,
		Slug:     collectionTranslation.Slug,
		Icon:     icon,
		Articles: articles,
	}, articles, nil
}

// GetPublicArticleByCanonicalPath returns a public article by collection slug and article slug.
func (s *DocsHelpcenterService) GetPublicArticleByCanonicalPath(ctx context.Context, workspaceID, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	doc, ha, content, err := s.hcRepo.GetPublicArticleByCollectionSlug(ctx, workspaceID, collectionSlug, articleSlug)
	if err != nil {
		return nil, err
	}
	if doc == nil || ha == nil {
		return nil, nil
	}

	// Resolve collection name if present.
	var collectionName *string
	if doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionName = &coll.Name
		}
	}

	// Render TipTap JSON -> HTML for public display.
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
	go func() {
		defer func() { recover() }()
		_ = s.hcRepo.IncrementViewCount(ctx, doc.ID)
	}()

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

// GetPublicCollection returns a collection and its published articles by workspace and collection slug.
func (s *DocsHelpcenterService) GetPublicCollection(ctx context.Context, workspaceID, collectionSlug string) (*model.DocsCollection, []model.PublicNavArticle, error) {
	return s.hcRepo.GetPublicCollectionBySlug(ctx, workspaceID, collectionSlug)
}

// PreviewArticleHTML renders a document's TipTap content as HTML for preview, regardless of status.
func (s *DocsHelpcenterService) PreviewArticleHTML(ctx context.Context, workspaceID, docID string) (*model.PreviewArticleResponse, error) {
	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found")
	}

	content, err := s.contentRepo.GetByDocumentID(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("get content: %w", err)
	}

	var contentHTML string
	if content != nil && len(content.Content) > 0 {
		rendered, err := tiptap.RenderHTML(content.Content)
		if err != nil {
			slog.ErrorContext(ctx, "preview render failed", "error", err, "doc_id", docID)
		} else {
			contentHTML = rendered
		}
	}

	// Resolve space name and slug.
	var spaceName, spaceSlug string
	space, err := s.spaceRepo.GetByID(ctx, doc.SpaceID)
	if err == nil && space != nil {
		spaceName = space.Name
		spaceSlug = space.Slug
	}

	// Resolve collection name.
	var collectionName *string
	if doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionName = &coll.Name
		}
	}

	return &model.PreviewArticleResponse{
		ID:             doc.ID,
		Title:          doc.Title,
		Excerpt:        doc.Excerpt,
		Icon:           doc.Icon,
		Status:         doc.Status,
		CollectionID:   doc.CollectionID,
		CollectionName: collectionName,
		SpaceName:      spaceName,
		SpaceSlug:      spaceSlug,
		ContentHTML:    contentHTML,
	}, nil
}

// ListRedirects returns paginated redirects for a workspace.
func (s *DocsHelpcenterService) ListRedirects(ctx context.Context, workspaceID string, filter model.DocsRedirectFilter) ([]model.DocsRedirect, int64, error) {
	return s.redirectRepo.List(ctx, workspaceID, filter)
}

// CreateRedirect creates a manual redirect.
func (s *DocsHelpcenterService) CreateRedirect(ctx context.Context, workspaceID string, req model.CreateDocsRedirectRequest) (*model.DocsRedirect, error) {
	if req.SourcePath == "" || req.SourcePath[0] != '/' {
		return nil, fmt.Errorf("source_path must start with /")
	}
	if req.TargetCollectionSlug == "" {
		return nil, fmt.Errorf("target_collection_slug is required")
	}
	redirect := &model.DocsRedirect{
		WorkspaceID:          workspaceID,
		SourcePath:           req.SourcePath,
		TargetCollectionSlug: req.TargetCollectionSlug,
		TargetArticleSlug:    req.TargetArticleSlug,
		Type:                 model.RedirectTypeManual,
	}
	if err := s.redirectRepo.Create(ctx, redirect); err != nil {
		return nil, err
	}
	return redirect, nil
}

// DeleteRedirect removes a redirect by ID.
// UpdateRedirect updates a redirect's target fields.
func (s *DocsHelpcenterService) UpdateRedirect(ctx context.Context, id string, req model.UpdateDocsRedirectRequest) (*model.DocsRedirect, error) {
	updates := map[string]interface{}{}
	if req.SourcePath != nil {
		if *req.SourcePath == "" || (*req.SourcePath)[0] != '/' {
			return nil, fmt.Errorf("source_path must start with /")
		}
		updates["source_path"] = *req.SourcePath
	}
	if req.TargetCollectionSlug != nil {
		if *req.TargetCollectionSlug == "" {
			return nil, fmt.Errorf("target_collection_slug is required")
		}
		updates["target_collection_slug"] = *req.TargetCollectionSlug
	}
	if req.TargetArticleSlug != nil {
		if *req.TargetArticleSlug == "" {
			updates["target_article_slug"] = nil
		} else {
			updates["target_article_slug"] = *req.TargetArticleSlug
		}
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}
	return s.redirectRepo.Update(ctx, id, updates)
}

func (s *DocsHelpcenterService) DeleteRedirect(ctx context.Context, id string) error {
	return s.redirectRepo.Delete(ctx, id)
}

// ResolvePublicPath resolves a legacy or imported URL path to a redirect target.
func (s *DocsHelpcenterService) ResolvePublicPath(ctx context.Context, workspaceID, path string) (*model.DocsRedirect, error) {
	return s.redirectRepo.GetBySourcePath(ctx, workspaceID, path)
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

func (s *DocsHelpcenterService) SubmitFeedbackForLocale(ctx context.Context, documentID, locale string, req model.DocsArticleFeedbackRequest) error {
	if err := s.SubmitFeedback(ctx, documentID, req); err != nil {
		return err
	}
	if locale == "" {
		return nil
	}
	return s.hcRepo.IncrementTranslatedFeedbackCount(ctx, documentID, locale, req.IsHelpful)
}
