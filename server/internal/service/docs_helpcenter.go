package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// DocsHelpcenterService handles help center publishing, config, and slug management.
type DocsHelpcenterService struct {
	hcRepo          *repository.DocsHelpcenterRepository
	publicationRepo *repository.DocsHelpcenterPublicationRepository
	docRepo         *repository.DocsDocumentRepository
	contentRepo     *repository.DocsContentRepository
	spaceRepo       *repository.DocsSpaceRepository
	collectionRepo  *repository.DocsCollectionRepository
	redirectRepo    *repository.DocsRedirectRepository
	s3Client        *storage.S3Client
	translationSvc  *DocsHelpcenterTranslationService
	wsPublisher     *websocket.Publisher
}

// NewDocsHelpcenterService creates a new DocsHelpcenterService.
func NewDocsHelpcenterService(
	hcRepo *repository.DocsHelpcenterRepository,
	publicationRepo *repository.DocsHelpcenterPublicationRepository,
	docRepo *repository.DocsDocumentRepository,
	contentRepo *repository.DocsContentRepository,
	spaceRepo *repository.DocsSpaceRepository,
	collectionRepo *repository.DocsCollectionRepository,
	redirectRepo *repository.DocsRedirectRepository,
	s3Client *storage.S3Client,
	wsPublisher *websocket.Publisher,
) *DocsHelpcenterService {
	return &DocsHelpcenterService{hcRepo: hcRepo, publicationRepo: publicationRepo, docRepo: docRepo, contentRepo: contentRepo, spaceRepo: spaceRepo, collectionRepo: collectionRepo, redirectRepo: redirectRepo, s3Client: s3Client, wsPublisher: wsPublisher}
}

func (s *DocsHelpcenterService) SetTranslationService(translationSvc *DocsHelpcenterTranslationService) {
	s.translationSvc = translationSvc
}

// UploadAsset uploads a help center asset (logo, dark logo, or favicon) to S3 and returns the public URL.
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
	config, err := s.hcRepo.UpsertConfig(ctx, workspaceID, updates)
	if err == nil && config != nil {
		publishWorkspaceEvent(s.wsPublisher, "updated", "docs_helpcenter_config", workspaceID, workspaceID, "")
	}
	return config, err
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

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return err
	}
	defaultLocale := defaultHelpcenterLocale(cfg)

	currentLive, err := s.publicationRepo.GetArticlePublication(ctx, documentID, defaultLocale)
	if err != nil {
		return err
	}

	if slug == "" {
		slug = strings.TrimSpace(art.Slug)
	}
	slug = normalizedSlugOrFallback(slug, strings.TrimSpace(art.Slug), doc.Title, defaultLocale)
	slug, err = s.ensureUniqueSourcePublicationSlug(ctx, doc.SpaceID, defaultLocale, documentID, slug)
	if err != nil {
		return err
	}

	if err := s.hcRepo.SetSlug(ctx, documentID, slug); err != nil {
		return err
	}

	publication, err := s.buildSourceArticlePublication(ctx, doc, art, defaultLocale, slug)
	if err != nil {
		return err
	}
	if _, err := s.publicationRepo.UpsertArticlePublication(ctx, publication); err != nil {
		return err
	}

	if currentLive != nil && currentLive.Slug != slug {
		if err := s.createSourceArticleRedirect(ctx, doc, currentLive.Slug, slug); err != nil {
			slog.ErrorContext(ctx, "create source slug change redirect", "error", err, "document_id", documentID)
		}
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
	publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", documentID, doc.WorkspaceID, "")
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

	cleaned := slugify(newSlug)
	if cleaned == "" {
		return fmt.Errorf("slug is required")
	}
	if art.Slug == cleaned {
		return nil
	}

	cfg, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return err
	}
	defaultLocale := defaultHelpcenterLocale(cfg)
	slug, err := s.ensureUniqueSourcePublicationSlug(ctx, doc.SpaceID, defaultLocale, documentID, cleaned)
	if err != nil {
		return err
	}
	if err := s.hcRepo.SetSlug(ctx, documentID, slug); err != nil {
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", documentID, workspaceID, "")
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
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	if err := s.hcRepo.SetPublicPublishedAt(ctx, documentID, nil); err != nil {
		return err
	}
	if s.translationSvc != nil {
		if err := s.translationSvc.RefreshArticleSource(ctx, documentID); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter article translation source after unpublish", "document_id", documentID, "error", err)
		}
	}
	if doc != nil {
		publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", documentID, doc.WorkspaceID, "")
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

func (s *DocsHelpcenterService) EnrichDocumentPublishState(ctx context.Context, doc *model.DocsDocument) error {
	if doc == nil {
		return nil
	}
	art, err := s.hcRepo.GetArticle(ctx, doc.ID)
	if err != nil {
		return err
	}
	if art != nil && art.Slug != "" {
		doc.HCSlug = art.Slug
	}
	if art == nil || art.PublicPublishedAt == nil {
		doc.HasUnpublishedChanges = false
		doc.LivePublishedAt = nil
		doc.LiveSlug = nil
		return nil
	}

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return err
	}
	defaultLocale := defaultHelpcenterLocale(cfg)
	publication, err := s.publicationRepo.GetArticlePublication(ctx, doc.ID, defaultLocale)
	if err != nil {
		return err
	}
	if publication == nil {
		doc.HasUnpublishedChanges = false
		doc.LivePublishedAt = art.PublicPublishedAt
		return nil
	}
	doc.LivePublishedAt = &publication.PublishedAt
	doc.LiveSlug = &publication.Slug
	doc.HasUnpublishedChanges, err = s.sourceArticleHasUnpublishedChanges(ctx, doc, art, publication)
	return err
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
	if err := s.ensureDefaultLocaleMirrors(ctx, cfg.WorkspaceID); err != nil {
		return nil, err
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
		if err := s.ensureDefaultLocaleMirrors(ctx, cfg.WorkspaceID); err != nil {
			return nil, err
		}
		s.enrichFeaturedCardTitles(ctx, cfg)
		return cfg, nil
	}

	// Fall back to custom domain lookup.
	cfg, err = s.hcRepo.GetConfigByCustomDomain(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		if err := s.ensureDefaultLocaleMirrors(ctx, cfg.WorkspaceID); err != nil {
			return nil, err
		}
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
		if card.LinkType != "collection" {
			continue
		}
		col, err := s.resolveFeaturedCardCollection(ctx, cfg.WorkspaceID, card)
		if err != nil || col == nil {
			continue
		}
		if col.Name != card.Title {
			hpCfg.FeaturedCards[i].Title = col.Name
			changed = true
		}
		if col.Description != nil && *col.Description != card.Description {
			hpCfg.FeaturedCards[i].Description = *col.Description
			changed = true
		}
		if (col.Icon != nil && *col.Icon != card.Icon) || (col.Icon == nil && card.Icon != "") {
			if col.Icon != nil {
				hpCfg.FeaturedCards[i].Icon = *col.Icon
			} else {
				hpCfg.FeaturedCards[i].Icon = ""
			}
			changed = true
		}
		if col.Slug != "" && col.Slug != card.LinkValue {
			hpCfg.FeaturedCards[i].LinkValue = col.Slug
			changed = true
		}
		space, err := s.spaceRepo.GetByID(ctx, col.SpaceID)
		if err == nil && space != nil && space.Slug != "" && space.Slug != card.SpaceSlug {
			hpCfg.FeaturedCards[i].SpaceSlug = space.Slug
			changed = true
		}
	}

	if changed {
		if enriched, err := json.Marshal(hpCfg); err == nil {
			cfg.HomepageConfig = enriched
		}
	}
}

func (s *DocsHelpcenterService) resolveFeaturedCardCollection(ctx context.Context, workspaceID string, card model.HomepageFeaturedCard) (*model.DocsCollection, error) {
	if card.LinkValue != "" {
		col, err := s.collectionRepo.GetByID(ctx, card.LinkValue)
		if err != nil {
			return nil, err
		}
		if col != nil {
			return col, nil
		}
	}

	if card.SpaceSlug == "" {
		return nil, nil
	}

	space, err := s.spaceRepo.GetBySlug(ctx, workspaceID, card.SpaceSlug)
	if err != nil || space == nil {
		return nil, err
	}

	collections, err := s.collectionRepo.ListBySpace(ctx, space.ID)
	if err != nil {
		return nil, err
	}

	linkValue := strings.TrimSpace(card.LinkValue)
	title := strings.TrimSpace(card.Title)
	for i := range collections {
		collection := collections[i]
		if linkValue != "" && (collection.ID == linkValue || collection.Slug == linkValue) {
			return &collection, nil
		}
		if linkValue == "" && title != "" && strings.EqualFold(strings.TrimSpace(collection.Name), title) {
			return &collection, nil
		}
	}

	return nil, nil
}

func defaultHelpcenterLocale(cfg *model.DocsHelpcenterConfig) string {
	if cfg != nil && cfg.DefaultLocale != "" {
		return cfg.DefaultLocale
	}
	return "en"
}

func compactJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err == nil {
		return buf.Bytes()
	}
	return raw
}

func stringPtrTrimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func derivedSEOTitle(title string, override *string) *string {
	trimmed := stringPtrTrimmed(override)
	if trimmed != "" {
		return &trimmed
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	return &title
}

func derivedSEODescription(excerpt *string, override *string) *string {
	trimmed := stringPtrTrimmed(override)
	if trimmed != "" {
		return &trimmed
	}
	if excerpt == nil {
		return nil
	}
	excerptValue := strings.TrimSpace(*excerpt)
	if excerptValue == "" {
		return nil
	}
	return &excerptValue
}

func (s *DocsHelpcenterService) buildSourceArticlePublication(ctx context.Context, doc *model.DocsDocument, art *model.DocsHelpcenterArticle, locale, slug string) (*model.DocsHelpcenterArticlePublication, error) {
	content, err := s.contentRepo.GetByDocumentID(ctx, doc.ID)
	if err != nil {
		return nil, err
	}

	publication := &model.DocsHelpcenterArticlePublication{
		DocumentID:     doc.ID,
		WorkspaceID:    doc.WorkspaceID,
		SpaceID:        doc.SpaceID,
		CollectionID:   doc.CollectionID,
		Locale:         locale,
		Title:          doc.Title,
		Slug:           slug,
		Excerpt:        doc.Excerpt,
		SEOTitle:       derivedSEOTitle(doc.Title, art.SEOTitle),
		SEODescription: derivedSEODescription(doc.Excerpt, art.SEODescription),
		PublishedAt:    time.Now().UTC(),
	}
	if content != nil {
		publication.Content = content.Content
		publication.ContentText = content.ContentText
	}
	return publication, nil
}

func (s *DocsHelpcenterService) sourceArticleHasUnpublishedChanges(ctx context.Context, doc *model.DocsDocument, art *model.DocsHelpcenterArticle, publication *model.DocsHelpcenterArticlePublication) (bool, error) {
	content, err := s.contentRepo.GetByDocumentID(ctx, doc.ID)
	if err != nil {
		return false, err
	}

	if strings.TrimSpace(doc.Title) != strings.TrimSpace(publication.Title) {
		return true, nil
	}
	if stringPtrTrimmed(doc.Excerpt) != stringPtrTrimmed(publication.Excerpt) {
		return true, nil
	}
	if strings.TrimSpace(art.Slug) != strings.TrimSpace(publication.Slug) {
		return true, nil
	}
	if stringPtrTrimmed(derivedSEOTitle(doc.Title, art.SEOTitle)) != stringPtrTrimmed(publication.SEOTitle) {
		return true, nil
	}
	if stringPtrTrimmed(derivedSEODescription(doc.Excerpt, art.SEODescription)) != stringPtrTrimmed(publication.SEODescription) {
		return true, nil
	}

	var currentContent json.RawMessage
	if content != nil {
		currentContent = content.Content
	}
	if !bytes.Equal(compactJSON(currentContent), compactJSON(publication.Content)) {
		return true, nil
	}
	return false, nil
}

func (s *DocsHelpcenterService) ensureUniqueSourcePublicationSlug(ctx context.Context, spaceID, locale, documentID, base string) (string, error) {
	slug := slugify(base)
	if slug == "" {
		slug = "article"
	}
	baseSlug := slug
	for i := 2; ; i++ {
		taken, err := s.publicationRepo.ArticlePublicationSlugExists(ctx, spaceID, locale, slug, documentID)
		if err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, i)
	}
}

func (s *DocsHelpcenterService) createSourceArticleRedirect(ctx context.Context, doc *model.DocsDocument, oldSlug, newSlug string) error {
	if s.redirectRepo == nil || oldSlug == "" || oldSlug == newSlug {
		return nil
	}
	collectionSlug := ""
	if doc.CollectionID != nil {
		collection, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err != nil {
			return err
		}
		if collection != nil {
			collectionSlug = strings.TrimSpace(collection.Slug)
			if collectionSlug == "" {
				collectionSlug = slugify(collection.Name)
				if collectionSlug == "" {
					return fmt.Errorf("collection slug is missing")
				}
				if _, err := s.collectionRepo.Update(ctx, collection.ID, map[string]interface{}{"slug": collectionSlug}); err != nil {
					return fmt.Errorf("backfill collection slug for redirect: %w", err)
				}
			}
		}
	}
	redirect := &model.DocsRedirect{
		WorkspaceID:          doc.WorkspaceID,
		SourcePath:           buildDocsRedirectPath(collectionSlug, &oldSlug),
		TargetCollectionSlug: collectionSlug,
		TargetArticleSlug:    &newSlug,
		Type:                 model.RedirectTypeSlugChange,
	}
	return s.redirectRepo.Create(ctx, redirect)
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

func (s *DocsHelpcenterService) ensureDefaultLocaleMirrors(ctx context.Context, workspaceID string) error {
	if s.translationSvc == nil {
		return nil
	}
	return s.translationSvc.EnsureDefaultLocaleMirrorsForWorkspace(ctx, workspaceID)
}

func (s *DocsHelpcenterService) resolvePublicSpaceTranslationBySlug(ctx context.Context, cfg *model.DocsHelpcenterConfig, workspaceID, requestedLocale, slug string) (*model.DocsHelpcenterSpaceTranslation, string, bool, error) {
	translation, err := s.hcRepo.GetPublicSpaceTranslationBySlug(ctx, workspaceID, requestedLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if translation != nil {
		return translation, requestedLocale, false, nil
	}

	if err := s.ensureDefaultLocaleMirrors(ctx, workspaceID); err != nil {
		return nil, "", false, err
	}

	translation, err = s.hcRepo.GetPublicSpaceTranslationBySlug(ctx, workspaceID, requestedLocale, slug)
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

func (s *DocsHelpcenterService) resolvePublicCollectionTranslationByCanonicalSlug(ctx context.Context, cfg *model.DocsHelpcenterConfig, workspaceID, requestedLocale, slug string) (*model.DocsHelpcenterCollectionTranslation, string, bool, error) {
	translation, err := s.hcRepo.GetPublicCollectionTranslationByWorkspaceSlug(ctx, workspaceID, requestedLocale, slug)
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

	fallback, err := s.hcRepo.GetPublicCollectionTranslationByWorkspaceSlug(ctx, workspaceID, defaultLocale, slug)
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

	loadRequestedTranslations := func() (map[string]model.DocsHelpcenterSpaceTranslation, int, error) {
		requestedTranslations, err := s.hcRepo.ListPublicSpaceTranslations(ctx, workspaceID, requestedLocale)
		if err != nil {
			return nil, 0, err
		}
		requestedBySpaceID := make(map[string]model.DocsHelpcenterSpaceTranslation, len(requestedTranslations))
		for _, translation := range requestedTranslations {
			requestedBySpaceID[translation.SpaceID] = translation
		}
		return requestedBySpaceID, len(requestedTranslations), nil
	}

	requestedBySpaceID, requestedCount, err := loadRequestedTranslations()
	if err != nil {
		return nil, err
	}
	if requestedCount == 0 {
		if err := s.ensureDefaultLocaleMirrors(ctx, workspaceID); err != nil {
			return nil, err
		}
		requestedBySpaceID, _, err = loadRequestedTranslations()
		if err != nil {
			return nil, err
		}
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
			Slug:        stringValue(translation.Slug),
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

		article := model.PublicNavArticle{ID: doc.ID, Title: translation.Title, Slug: stringValue(translation.Slug)}
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
			Slug:     stringValue(translation.Slug),
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
		collectionSlugValue = resolvedColl.Slug
	}

	return &model.PublicArticleResponse{
		ID:              doc.ID,
		Title:           translation.Title,
		Slug:            stringValue(translation.Slug),
		Locale:          resolvedLocale,
		RequestedLocale: requestedLocale,
		IsFallback:      fellBack,
		Excerpt:         translation.Excerpt,
		Icon:            doc.Icon,
		Status:          doc.Status,
		SpaceSlug:       stringValue(spaceTranslation.Slug),
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
			Slug:  stringValue(translation.Slug),
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
		ID:        collectionTranslation.CollectionID,
		Name:      collectionTranslation.Name,
		Slug:      stringValue(collectionTranslation.Slug),
		SpaceSlug: stringValue(spaceTranslation.Slug),
		Icon:      icon,
		Articles:  articles,
	}, articles, nil
}

func (s *DocsHelpcenterService) GetPublicLocalizedCollectionByCanonicalPath(ctx context.Context, workspaceID, requestedLocale, collectionSlug string) (*model.PublicNavCollection, []model.PublicNavArticle, error) {
	cfg, defaultLocale, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	collectionTranslation, resolvedLocale, _, err := s.resolvePublicCollectionTranslationByCanonicalSlug(ctx, cfg, workspaceID, requestedLocale, collectionSlug)
	if err != nil {
		return nil, nil, err
	}

	requestedArticles, err := s.hcRepo.ListPublicArticleTranslationsByCollection(ctx, collectionTranslation.CollectionID, resolvedLocale)
	if err != nil {
		return nil, nil, err
	}
	requestedArticleByID := make(map[string]model.DocsHelpcenterArticleTranslation, len(requestedArticles))
	for _, translation := range requestedArticles {
		requestedArticleByID[translation.DocumentID] = translation
	}

	fallbackArticleByID := map[string]model.DocsHelpcenterArticleTranslation{}
	if cfg.FallbackToDefaultLocale && resolvedLocale != defaultLocale {
		fallbackArticles, err := s.hcRepo.ListPublicArticleTranslationsByCollection(ctx, collectionTranslation.CollectionID, defaultLocale)
		if err != nil {
			return nil, nil, err
		}
		for _, translation := range fallbackArticles {
			fallbackArticleByID[translation.DocumentID] = translation
		}
	}

	collection, err := s.collectionRepo.GetByID(ctx, collectionTranslation.CollectionID)
	if err != nil {
		return nil, nil, err
	}
	if collection == nil {
		return nil, nil, fmt.Errorf("collection not found")
	}

	status := model.DocStatusPublished
	spaceID := collection.SpaceID
	collectionID := collectionTranslation.CollectionID
	docs, err := s.docRepo.List(ctx, workspaceID, &spaceID, &collectionID, &status, nil, "", false)
	if err != nil {
		return nil, nil, err
	}

	space, err := s.spaceRepo.GetByID(ctx, collection.SpaceID)
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
			Slug:  stringValue(translation.Slug),
		})
	}

	var icon *string
	if collection != nil {
		icon = collection.Icon
	}
	var resolvedSpaceSlug string
	if space != nil {
		resolvedSpaceSlug = space.Slug
	}

	return &model.PublicNavCollection{
		ID:        collectionTranslation.CollectionID,
		Name:      collectionTranslation.Name,
		Slug:      stringValue(collectionTranslation.Slug),
		SpaceSlug: resolvedSpaceSlug,
		Icon:      icon,
		Articles:  articles,
	}, articles, nil
}

func (s *DocsHelpcenterService) GetPublicArticleByLocalizedCanonicalPath(ctx context.Context, workspaceID, requestedLocale, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	cfg, _, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	collectionTranslation, _, _, err := s.resolvePublicCollectionTranslationByCanonicalSlug(ctx, cfg, workspaceID, requestedLocale, collectionSlug)
	if err != nil {
		return nil, err
	}

	translation, resolvedLocale, fellBack, err := func() (*model.DocsHelpcenterArticleTranslation, string, bool, error) {
		translation, err := s.hcRepo.GetPublicArticleTranslationByCollectionSlug(ctx, collectionTranslation.CollectionID, requestedLocale, articleSlug)
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

		fallback, err := s.hcRepo.GetPublicArticleTranslationByCollectionSlug(ctx, collectionTranslation.CollectionID, defaultLocale, articleSlug)
		if err != nil {
			return nil, "", false, err
		}
		if fallback == nil {
			return nil, "", false, fmt.Errorf("article not found")
		}
		return fallback, defaultLocale, true, nil
	}()
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

	collection, err := s.collectionRepo.GetByID(ctx, collectionTranslation.CollectionID)
	if err != nil {
		return nil, err
	}
	var collectionName *string
	var collectionSlugValue *string
	var spaceSlugValue string
	if collection != nil {
		collectionName = &collectionTranslation.Name
		collectionSlugValue = collectionTranslation.Slug
		if space, err := s.spaceRepo.GetByID(ctx, collection.SpaceID); err == nil && space != nil {
			spaceSlugValue = space.Slug
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

	return &model.PublicArticleResponse{
		ID:              doc.ID,
		Title:           translation.Title,
		Slug:            stringValue(translation.Slug),
		Locale:          resolvedLocale,
		RequestedLocale: requestedLocale,
		IsFallback:      fellBack,
		Excerpt:         translation.Excerpt,
		Icon:            doc.Icon,
		Status:          doc.Status,
		SpaceSlug:       spaceSlugValue,
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
	req.SourcePath = normalizeDocsRedirectSourcePath(req.SourcePath)
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
		normalized := normalizeDocsRedirectSourcePath(*req.SourcePath)
		if normalized == "" || normalized[0] != '/' {
			return nil, fmt.Errorf("source_path must start with /")
		}
		updates["source_path"] = normalized
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
