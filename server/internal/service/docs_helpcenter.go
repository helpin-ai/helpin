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
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/cache"
	"github.com/helpin-ai/helpin/server/internal/iconcatalog"
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
	searchRepo      *repository.DocsHelpcenterSearchRepository
	s3Client        *storage.S3Client
	translationSvc  *DocsHelpcenterTranslationService
	wsPublisher     *websocket.Publisher
	hcCache         cache.Cache
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

func (s *DocsHelpcenterService) SetSearchRepository(searchRepo *repository.DocsHelpcenterSearchRepository) {
	s.searchRepo = searchRepo
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
	cfg, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil || cfg == nil {
		return cfg, err
	}
	s.enrichFeaturedCardTitles(ctx, cfg)
	return cfg, nil
}

// UpsertConfig creates or updates the help center config.
func (s *DocsHelpcenterService) UpsertConfig(ctx context.Context, workspaceID string, req model.UpdateDocsHelpcenterConfigRequest) (*model.DocsHelpcenterConfig, error) {
	existing, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	mode := model.HelpcenterPublicURLModeHostedSubdomain
	var customDomain, reverseProxyHost, reverseProxyBasePath *string
	if existing != nil {
		mode = strings.TrimSpace(existing.PublicURLMode)
		customDomain = existing.CustomDomain
		reverseProxyHost = existing.ReverseProxyHost
		reverseProxyBasePath = existing.ReverseProxyBasePath
		if mode == "" {
			mode = model.HelpcenterPublicURLModeHostedSubdomain
		}
	}

	updates := map[string]interface{}{}
	if req.Subdomain != nil {
		updates["subdomain"] = *req.Subdomain
	}
	if req.CustomDomain != nil {
		normalized, err := normalizeHelpcenterPublicHost(req.CustomDomain, "custom domain")
		if err != nil {
			return nil, err
		}
		// Stored value must match what the TLS ask endpoint looks up, or the
		// domain can never get a certificate issued.
		if normalized != nil {
			if _, err := NormalizeTLSAskDomain(*normalized); err != nil {
				return nil, fmt.Errorf("custom domain must be a valid hostname without port or wildcard")
			}
		}
		customDomain = normalized
		updates["custom_domain"] = normalized
	}
	if req.PublicURLMode != nil {
		mode = strings.TrimSpace(*req.PublicURLMode)
		updates["public_url_mode"] = mode
	}
	if req.ReverseProxyHost != nil {
		normalized, err := normalizeHelpcenterPublicHost(req.ReverseProxyHost, "reverse proxy host")
		if err != nil {
			return nil, err
		}
		reverseProxyHost = normalized
		updates["reverse_proxy_host"] = normalized
	}
	if req.ReverseProxyBasePath != nil {
		normalized, err := normalizeHelpcenterBasePath(req.ReverseProxyBasePath)
		if err != nil {
			return nil, err
		}
		reverseProxyBasePath = normalized
		updates["reverse_proxy_base_path"] = normalized
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
	if req.ChatWidgetEnabled != nil {
		updates["chat_widget_enabled"] = *req.ChatWidgetEnabled
	}
	if req.AIAnswersEnabled != nil {
		updates["ai_answers_enabled"] = *req.AIAnswersEnabled
	}
	if req.SEOTitle != nil {
		updates["seo_title"] = req.SEOTitle
	}
	if req.SEODescription != nil {
		updates["seo_description"] = req.SEODescription
	}
	if req.OGTitle != nil {
		updates["og_title"] = nullableTrimmedString(req.OGTitle)
	}
	if req.OGDescription != nil {
		updates["og_description"] = nullableTrimmedString(req.OGDescription)
	}
	if req.OGImageURL != nil {
		updates["og_image_url"] = nullableTrimmedString(req.OGImageURL)
	}
	if req.OGImageAlt != nil {
		updates["og_image_alt"] = nullableTrimmedString(req.OGImageAlt)
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
		var currentHomepageConfig json.RawMessage
		if existing != nil {
			currentHomepageConfig = existing.HomepageConfig
		}
		normalized, err := normalizeHomepageConfigIconWrites(req.HomepageConfig, currentHomepageConfig)
		if err != nil {
			return nil, err
		}
		updates["homepage_config"] = normalized
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
	if req.PublicURLMode == nil && existing == nil && mode == model.HelpcenterPublicURLModeHostedSubdomain && stringPtrTrimmed(customDomain) != "" {
		mode = model.HelpcenterPublicURLModeCustomDomain
		updates["public_url_mode"] = mode
	}
	if err := validateHelpcenterPublicURLConfig(mode, customDomain, reverseProxyHost, reverseProxyBasePath); err != nil {
		return nil, err
	}
	if _, ok := updates["public_url_mode"]; !ok && existing == nil {
		updates["public_url_mode"] = mode
	}
	config, err := s.hcRepo.UpsertConfig(ctx, workspaceID, updates)
	if err == nil && config != nil {
		publishWorkspaceEvent(s.wsPublisher, "updated", "docs_helpcenter_config", workspaceID, workspaceID, "")
		s.InvalidateHelpcenterCacheForWorkspace(ctx, workspaceID)
	}
	return config, err
}

// PublishExternally publishes a help center article externally.
func (s *DocsHelpcenterService) PublishExternally(ctx context.Context, documentID string, slug string, publishedContent json.RawMessage) error {
	publishedContent, err := validatePublicationSnapshotContent(publishedContent)
	if err != nil {
		return err
	}

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
		publicID, err := s.ensureUniqueHelpcenterPublicID(ctx, documentID)
		if err != nil {
			return err
		}
		art = &model.DocsHelpcenterArticle{
			DocumentID: documentID,
			PublicID:   publicID,
		}
		if _, err := s.hcRepo.CreateArticle(ctx, art); err != nil {
			return err
		}
	} else if strings.TrimSpace(art.PublicID) == "" {
		publicID, err := s.ensureUniqueHelpcenterPublicID(ctx, documentID)
		if err != nil {
			return err
		}
		if err := s.hcRepo.SetPublicID(ctx, documentID, publicID); err != nil {
			return err
		}
		art.PublicID = publicID
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

	publication, err := s.buildSourceArticlePublication(ctx, doc, art, defaultLocale, slug, publishedContent)
	if err != nil {
		return err
	}
	if _, err := s.publicationRepo.UpsertArticlePublication(ctx, publication); err != nil {
		return err
	}
	if err := s.rebuildArticleSearchEntries(ctx, publication); err != nil {
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
	s.InvalidateHelpcenterCacheForWorkspace(ctx, doc.WorkspaceID)
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
	s.InvalidateHelpcenterCacheForWorkspace(ctx, workspaceID)
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
	if s.searchRepo != nil {
		if err := s.searchRepo.DeleteArticleEntriesByDocumentIDs(ctx, []string{documentID}); err != nil {
			return err
		}
	}
	if s.translationSvc != nil {
		if err := s.translationSvc.RefreshArticleSource(ctx, documentID); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter article translation source after unpublish", "document_id", documentID, "error", err)
		}
	}
	if doc != nil {
		publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", documentID, doc.WorkspaceID, "")
		s.InvalidateHelpcenterCacheForWorkspace(ctx, doc.WorkspaceID)
	}
	return nil
}

func (s *DocsHelpcenterService) rebuildArticleSearchEntries(ctx context.Context, publication *model.DocsHelpcenterArticlePublication) error {
	if s.searchRepo == nil || publication == nil {
		return nil
	}
	entries := BuildHelpcenterSearchEntries(*publication)
	if err := s.searchRepo.ReplaceArticleEntries(ctx, publication.DocumentID, publication.Locale, entries); err != nil {
		return fmt.Errorf("rebuild helpcenter search entries: %w", err)
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

func (s *DocsHelpcenterService) UpdateArticleMetadata(ctx context.Context, workspaceID, documentID string, req model.UpdateDocsHelpcenterArticleMetadataRequest) (*model.DocsHelpcenterArticle, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found")
	}

	art, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if art == nil {
		publicID, err := s.ensureUniqueHelpcenterPublicID(ctx, documentID)
		if err != nil {
			return nil, err
		}
		art = &model.DocsHelpcenterArticle{
			DocumentID: documentID,
			PublicID:   publicID,
		}
		if _, err := s.hcRepo.CreateArticle(ctx, art); err != nil {
			return nil, err
		}
	}

	updates := map[string]interface{}{}
	if req.OGTitle != nil {
		updates["og_title"] = nullableTrimmedString(req.OGTitle)
	}
	if req.OGDescription != nil {
		updates["og_description"] = nullableTrimmedString(req.OGDescription)
	}
	if req.OGImageURL != nil {
		updates["og_image_url"] = nullableTrimmedString(req.OGImageURL)
	}
	if req.OGImageAlt != nil {
		updates["og_image_alt"] = nullableTrimmedString(req.OGImageAlt)
	}
	if len(updates) == 0 {
		return art, nil
	}

	updated, err := s.hcRepo.UpdateArticleMetadata(ctx, documentID, updates)
	if err != nil {
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", documentID, workspaceID, "")
	s.InvalidateHelpcenterCacheForWorkspace(ctx, workspaceID)
	return updated, nil
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
	if art != nil {
		doc.HCOGTitle = art.OGTitle
		doc.HCOGDescription = art.OGDescription
		doc.HCOGImageURL = art.OGImageURL
		doc.HCOGImageAlt = art.OGImageAlt
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
	if art != nil && strings.TrimSpace(art.PublicID) == "" {
		publicID, err := s.ensureUniqueHelpcenterPublicID(ctx, art.DocumentID)
		if err != nil {
			return nil, err
		}
		art.PublicID = publicID
	}
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
	normalizePublicHomepageIcons(cfg)
	return cfg, nil
}

// ResolveConfig resolves a help center config by subdomain first, then by custom domain.
func (s *DocsHelpcenterService) resolveConfigUncached(ctx context.Context, identifier string) (*model.DocsHelpcenterConfig, error) {
	// Try subdomain first.
	cfg, err := s.hcRepo.GetConfigBySubdomain(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		s.enrichFeaturedCardTitles(ctx, cfg)
		normalizePublicHomepageIcons(cfg)
		return cfg, nil
	}

	// Fall back to custom domain lookup.
	cfg, err = s.hcRepo.GetConfigByCustomDomain(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		s.enrichFeaturedCardTitles(ctx, cfg)
		normalizePublicHomepageIcons(cfg)
		return cfg, nil
	}

	return nil, nil
}

// GetConfigByCustomDomain returns a help center config by its custom domain.
func (s *DocsHelpcenterService) GetConfigByCustomDomain(ctx context.Context, domain string) (*model.DocsHelpcenterConfig, error) {
	cfg, err := s.hcRepo.GetConfigByCustomDomain(ctx, domain)
	if err != nil || cfg == nil {
		return cfg, err
	}
	s.enrichFeaturedCardTitles(ctx, cfg)
	normalizePublicHomepageIcons(cfg)
	return cfg, nil
}

func normalizeHomepageConfigIconWrites(raw, currentRaw json.RawMessage) (json.RawMessage, error) {
	var homepage model.HelpcenterHomepageConfig
	if err := json.Unmarshal(raw, &homepage); err != nil {
		return nil, fmt.Errorf("homepage_config: invalid JSON: %w", err)
	}
	var current model.HelpcenterHomepageConfig
	if len(currentRaw) > 0 {
		_ = json.Unmarshal(currentRaw, &current)
	}
	for index := range homepage.FeaturedCards {
		card := &homepage.FeaturedCards[index]
		var currentIcon *string
		if index < len(current.FeaturedCards) {
			currentIcon = &current.FeaturedCards[index].Icon
		}
		normalized, changed, err := iconcatalog.NormalizeUpdate(&card.Icon, currentIcon)
		if err != nil {
			return nil, fmt.Errorf("homepage_config.featured_cards[%d].icon: %w", index, err)
		}
		switch {
		case changed:
			if normalized == nil {
				card.Icon = ""
			} else {
				card.Icon = *normalized
			}
		case currentIcon != nil:
			card.Icon = *currentIcon
		}
	}
	normalized, err := json.Marshal(homepage)
	if err != nil {
		return nil, fmt.Errorf("homepage_config: encode: %w", err)
	}
	return normalized, nil
}

func normalizePublicHomepageIcons(cfg *model.DocsHelpcenterConfig) {
	if cfg == nil || len(cfg.HomepageConfig) == 0 {
		return
	}
	var homepage model.HelpcenterHomepageConfig
	if err := json.Unmarshal(cfg.HomepageConfig, &homepage); err != nil {
		return
	}
	for index := range homepage.FeaturedCards {
		icon := homepage.FeaturedCards[index].Icon
		resolved := iconcatalog.ResolvePublicValue(&icon, "folder")
		if resolved == nil {
			homepage.FeaturedCards[index].Icon = ""
		} else {
			homepage.FeaturedCards[index].Icon = *resolved
		}
	}
	if normalized, err := json.Marshal(homepage); err == nil {
		cfg.HomepageConfig = normalized
	}
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
	enrichedCards := make([]model.HomepageFeaturedCard, 0, len(hpCfg.FeaturedCards))
	for _, rawCard := range hpCfg.FeaturedCards {
		card := model.HomepageFeaturedCard{
			Title:       strings.TrimSpace(rawCard.Title),
			Description: strings.TrimSpace(rawCard.Description),
			Icon:        strings.TrimSpace(rawCard.Icon),
			LinkType:    strings.TrimSpace(rawCard.LinkType),
			LinkValue:   strings.TrimSpace(rawCard.LinkValue),
			SpaceSlug:   strings.TrimSpace(rawCard.SpaceSlug),
			PublicID:    strings.TrimSpace(rawCard.PublicID),
		}
		if card != rawCard {
			changed = true
		}

		switch card.LinkType {
		case "collection":
			col, err := s.resolveFeaturedCardCollection(ctx, cfg.WorkspaceID, card)
			if err != nil {
				enrichedCards = append(enrichedCards, card)
				continue
			}
			if col == nil {
				changed = true
				continue
			}
			if col.Name != card.Title {
				card.Title = col.Name
				changed = true
			}
			nextDescription := ""
			if col.Description != nil {
				nextDescription = *col.Description
			}
			if nextDescription != card.Description {
				card.Description = nextDescription
				changed = true
			}
			nextIcon := ""
			if col.Icon != nil {
				nextIcon = *col.Icon
			}
			if nextIcon != card.Icon {
				card.Icon = nextIcon
				changed = true
			}
			if col.Slug != "" && col.Slug != card.LinkValue {
				card.LinkValue = col.Slug
				changed = true
			}
			if col.PublicID != "" && col.PublicID != card.PublicID {
				card.PublicID = col.PublicID
				changed = true
			}
			space, err := s.spaceRepo.GetByID(ctx, col.SpaceID)
			if err == nil && space != nil && space.Slug != "" && space.Slug != card.SpaceSlug {
				card.SpaceSlug = space.Slug
				changed = true
			}
			if card.LinkValue == "" {
				changed = true
				continue
			}
		case "space", "article", "url":
			if card.LinkValue == "" {
				changed = true
				continue
			}
		default:
			changed = true
			continue
		}

		enrichedCards = append(enrichedCards, card)
	}
	if len(enrichedCards) != len(hpCfg.FeaturedCards) {
		changed = true
	}
	hpCfg.FeaturedCards = enrichedCards

	if changed {
		if enriched, err := json.Marshal(hpCfg); err == nil {
			cfg.HomepageConfig = enriched
		}
	}
}

func looksLikeUUID(s string) bool {
	// UUID v4: 8-4-4-4-12 hex chars
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func (s *DocsHelpcenterService) resolveFeaturedCardCollection(ctx context.Context, workspaceID string, card model.HomepageFeaturedCard) (*model.DocsCollection, error) {
	if card.LinkValue != "" && looksLikeUUID(card.LinkValue) {
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

func validatePublicationSnapshotContent(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("published_content must be valid JSON")
	}
	var node tiptap.Node
	if err := json.Unmarshal(trimmed, &node); err != nil {
		return nil, fmt.Errorf("published_content must be TipTap JSON: %w", err)
	}
	if node.Type != "doc" {
		return nil, fmt.Errorf("published_content root must be a doc node")
	}
	if _, err := tiptap.RenderHTML(trimmed); err != nil {
		return nil, fmt.Errorf("published_content cannot be rendered: %w", err)
	}
	return json.RawMessage(compactJSON(trimmed)), nil
}

func publicationContentEqual(current json.RawMessage, published json.RawMessage) bool {
	return bytes.Equal(compactJSON(normalizePublishedSourceContent(current)), compactJSON(normalizePublishedSourceContent(published)))
}

func normalizePublishedSourceContent(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var node tiptap.Node
	if err := json.Unmarshal(raw, &node); err != nil {
		return raw
	}
	normalized := normalizePublishedSourceNode(node)
	payload, err := json.Marshal(normalized)
	if err != nil {
		return raw
	}
	return payload
}

func normalizePublishedSourceNode(node tiptap.Node) tiptap.Node {
	if node.Attrs != nil {
		if source, ok := node.Attrs["publishedFrom"]; ok {
			payload, err := json.Marshal(source)
			if err == nil {
				var sourceNode tiptap.Node
				if err := json.Unmarshal(payload, &sourceNode); err == nil && sourceNode.Type != "" {
					return normalizePublishedSourceNode(sourceNode)
				}
			}
		}
	}
	if len(node.Content) > 0 {
		for i := range node.Content {
			node.Content[i] = normalizePublishedSourceNode(node.Content[i])
		}
	}
	return node
}

func stringPtrTrimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func normalizeHelpcenterPublicHost(value *string, label string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	lower := strings.TrimSuffix(strings.ToLower(trimmed), ".")
	if strings.Contains(lower, "://") || strings.ContainsAny(lower, `/\`) || strings.ContainsAny(lower, " \t\r\n") {
		return nil, fmt.Errorf("%s must be a hostname without scheme or path", label)
	}
	return &lower, nil
}

func normalizeHelpcenterBasePath(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" || trimmed == "/" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" || trimmed == "/" {
		return nil, nil
	}
	if strings.Contains(trimmed, "//") || strings.Contains(trimmed, `\`) {
		return nil, fmt.Errorf("reverse proxy base path is invalid")
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "." || segment == ".." {
			return nil, fmt.Errorf("reverse proxy base path is invalid")
		}
	}
	return &trimmed, nil
}

func validateHelpcenterPublicURLConfig(mode string, customDomain, reverseProxyHost, reverseProxyBasePath *string) error {
	switch mode {
	case "", model.HelpcenterPublicURLModeHostedSubdomain:
		return nil
	case model.HelpcenterPublicURLModeCustomDomain:
		if stringPtrTrimmed(customDomain) == "" {
			return fmt.Errorf("custom domain public URL mode requires a custom domain")
		}
		return nil
	case model.HelpcenterPublicURLModeReverseProxy:
		if stringPtrTrimmed(reverseProxyHost) == "" {
			return fmt.Errorf("reverse proxy public URL mode requires a public host")
		}
		if stringPtrTrimmed(reverseProxyBasePath) == "" {
			return fmt.Errorf("reverse proxy public URL mode requires a public base path")
		}
		return nil
	default:
		return fmt.Errorf("invalid public URL mode %q", mode)
	}
}

func nullableTrimmedString(value *string) interface{} {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
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

func (s *DocsHelpcenterService) buildSourceArticlePublication(ctx context.Context, doc *model.DocsDocument, art *model.DocsHelpcenterArticle, locale, slug string, publishedContent json.RawMessage) (*model.DocsHelpcenterArticlePublication, error) {
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
		OGTitle:        art.OGTitle,
		OGDescription:  art.OGDescription,
		OGImageURL:     art.OGImageURL,
		OGImageAlt:     art.OGImageAlt,
		PublishedAt:    time.Now().UTC(),
	}
	if content != nil {
		publication.Content = content.Content
		publication.ContentText = content.ContentText
	}
	if len(publishedContent) > 0 {
		publication.Content = publishedContent
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
	if stringPtrTrimmed(art.OGTitle) != stringPtrTrimmed(publication.OGTitle) {
		return true, nil
	}
	if stringPtrTrimmed(art.OGDescription) != stringPtrTrimmed(publication.OGDescription) {
		return true, nil
	}
	if stringPtrTrimmed(art.OGImageURL) != stringPtrTrimmed(publication.OGImageURL) {
		return true, nil
	}
	if stringPtrTrimmed(art.OGImageAlt) != stringPtrTrimmed(publication.OGImageAlt) {
		return true, nil
	}

	var currentContent json.RawMessage
	if content != nil {
		currentContent = content.Content
	}
	if !publicationContentEqual(currentContent, publication.Content) {
		return true, nil
	}
	return false, nil
}

func (s *DocsHelpcenterService) ensureUniqueSourcePublicationSlug(ctx context.Context, spaceID, locale, documentID, base string) (string, error) {
	slug := slugify(base)
	if slug == "" {
		slug = "article"
	}
	return slug, nil
}

func (s *DocsHelpcenterService) ensureUniqueHelpcenterPublicID(ctx context.Context, documentID string) (string, error) {
	for attempts := 0; attempts < 16; attempts++ {
		publicID, err := generateDocsHelpcenterPublicID()
		if err != nil {
			return "", err
		}
		exists, err := s.hcRepo.PublicIDExists(ctx, publicID, documentID)
		if err != nil {
			return "", err
		}
		if !exists {
			return publicID, nil
		}
	}
	return "", fmt.Errorf("generate unique helpcenter public id: exhausted retries")
}

func (s *DocsHelpcenterService) createSourceArticleRedirect(ctx context.Context, doc *model.DocsDocument, oldSlug, newSlug string) error {
	if s.redirectRepo == nil || oldSlug == "" || oldSlug == newSlug {
		return nil
	}
	collectionSlug, err := s.ensureCollectionSlugForRedirect(ctx, doc.CollectionID)
	if err != nil {
		return err
	}
	redirect := &model.DocsRedirect{
		WorkspaceID:          doc.WorkspaceID,
		SourcePath:           buildDocsRedirectPath(collectionSlug, &oldSlug),
		TargetCollectionSlug: collectionSlug,
		TargetArticleSlug:    &newSlug,
		Type:                 model.RedirectTypeSlugChange,
	}
	if ha, err := s.hcRepo.GetArticle(ctx, doc.ID); err == nil && ha != nil {
		setRedirectTargetPath(redirect, "", ha.PublicID)
	}
	return s.redirectRepo.UpsertWithReconciliation(ctx, redirect)
}

// ensureCollectionSlugForRedirect returns the canonical slug of the
// collection referenced by id, back-filling the slug from the collection
// name when the row predates the slug-hardening migration. Returns an
// empty string when id is nil (article lives in the uncategorized bucket).
func (s *DocsHelpcenterService) ensureCollectionSlugForRedirect(ctx context.Context, collectionID *string) (string, error) {
	if collectionID == nil {
		return "", nil
	}
	collection, err := s.collectionRepo.GetByID(ctx, *collectionID)
	if err != nil {
		return "", err
	}
	if collection == nil {
		return "", nil
	}
	slug := strings.TrimSpace(collection.Slug)
	if slug != "" {
		return slug, nil
	}
	slug = slugify(collection.Name)
	if slug == "" {
		return "", fmt.Errorf("collection slug is missing")
	}
	if _, err := s.collectionRepo.Update(ctx, collection.ID, map[string]interface{}{"slug": slug}); err != nil {
		return "", fmt.Errorf("backfill collection slug for redirect: %w", err)
	}
	return slug, nil
}

// EmitArticleMoveRedirect writes an auto_article_move redirect when a
// document has moved to a different collection. The redirect maps the
// old canonical public path to the new one. Cycles are broken by the
// repository's UpsertWithReconciliation helper, so repeated back-and-
// forth moves collapse instead of chaining.
//
// It is safe to call this with an unchanged collection, a missing
// article, or a document that has no help center presence — in those
// cases the method is a no-op.
func (s *DocsHelpcenterService) EmitArticleMoveRedirect(ctx context.Context, doc *model.DocsDocument, oldCollectionID *string) error {
	if s.redirectRepo == nil || doc == nil {
		return nil
	}
	if sameCollectionPointer(oldCollectionID, doc.CollectionID) {
		return nil
	}
	art, err := s.hcRepo.GetArticle(ctx, doc.ID)
	if err != nil {
		return err
	}
	if art == nil || art.Slug == "" || art.PublicPublishedAt == nil {
		return nil
	}

	oldCollectionSlug, err := s.ensureCollectionSlugForRedirect(ctx, oldCollectionID)
	if err != nil {
		return err
	}
	newCollectionSlug, err := s.ensureCollectionSlugForRedirect(ctx, doc.CollectionID)
	if err != nil {
		return err
	}
	slug := art.Slug
	oldPath := buildDocsRedirectPath(oldCollectionSlug, &slug)
	newPath := buildDocsRedirectPath(newCollectionSlug, &slug)
	if oldPath == newPath {
		return nil
	}
	redirect := &model.DocsRedirect{
		WorkspaceID:          doc.WorkspaceID,
		SourcePath:           oldPath,
		TargetCollectionSlug: newCollectionSlug,
		TargetArticleSlug:    &slug,
		Type:                 model.RedirectTypeAutoArticleMove,
	}
	setRedirectTargetPath(redirect, "", art.PublicID)
	return s.redirectRepo.UpsertWithReconciliation(ctx, redirect)
}

// UpdateCollectionSlug changes the slug of a collection and emits
// auto_collection_rename redirects for the collection itself and for
// every published article directly attached to it.
//
// The entire operation — redirect writes plus slug update — runs in a
// single GORM transaction so a partial failure cannot leave redirects
// pointing at a slug that never became canonical. If the slug update
// fails after redirect writes, the transaction rolls back and the DB
// state is unchanged.
//
// Collection slugs are no longer required to be unique — duplicate slugs
// are allowed. The method updates the slug and creates redirects from the
// old slug to the new one.
func (s *DocsHelpcenterService) UpdateCollectionSlug(ctx context.Context, collectionID, rawNewSlug string) (*model.DocsCollection, error) {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, ErrDocsCollectionNotFound
	}
	newSlug := strings.TrimSpace(slugify(rawNewSlug))
	if newSlug == "" {
		return nil, fmt.Errorf("collection slug is required")
	}
	if newSlug == collection.Slug {
		return collection, nil
	}

	oldSlug := collection.Slug

	var updated *model.DocsCollection
	txErr := s.collectionRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCollectionRepo := repository.NewDocsCollectionRepository(tx, false)
		txRedirectRepo := repository.NewDocsRedirectRepository(tx)
		txHcRepo := repository.NewDocsHelpcenterRepository(tx, false)

		// Collection-level redirect: old /:slug -> new /:slug.
		if oldSlug != "" {
			collectionRedirect := &model.DocsRedirect{
				WorkspaceID:          collection.WorkspaceID,
				SourcePath:           buildDocsRedirectPath(oldSlug, nil),
				TargetCollectionSlug: newSlug,
				Type:                 model.RedirectTypeAutoCollectionRename,
			}
			setRedirectTargetPath(collectionRedirect, collection.PublicID, "")
			if err := txRedirectRepo.UpsertWithReconciliation(ctx, collectionRedirect); err != nil {
				return err
			}

			// Per-article redirects for every directly-published article.
			// ListPublishedArticleSlugsInCollection reads the canonical
			// source slug from docs_helpcenter_articles.slug so we cover
			// articles that have not yet synthesised a publication row.
			articles, err := txHcRepo.ListPublishedArticleSlugsInCollection(ctx, collection.ID)
			if err != nil {
				return err
			}
			for i := range articles {
				slug := articles[i].Slug
				if slug == "" {
					continue
				}
				articleRedirect := &model.DocsRedirect{
					WorkspaceID:          collection.WorkspaceID,
					SourcePath:           buildDocsRedirectPath(oldSlug, &slug),
					TargetCollectionSlug: newSlug,
					TargetArticleSlug:    &slug,
					Type:                 model.RedirectTypeAutoCollectionRename,
				}
				setRedirectTargetPath(articleRedirect, "", articles[i].PublicID)
				if err := txRedirectRepo.UpsertWithReconciliation(ctx, articleRedirect); err != nil {
					return err
				}
			}
		}

		slugUpdated, err := txCollectionRepo.Update(ctx, collection.ID, map[string]interface{}{"slug": newSlug})
		if err != nil {
			return err
		}
		updated = slugUpdated
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_collection", collection.ID, collection.WorkspaceID, "", "docs_space", collection.SpaceID, nil)
	return updated, nil
}

func sameCollectionPointer(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
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

func (s *DocsHelpcenterService) ensureDefaultLocaleCollectionMirrorsForSpace(ctx context.Context, spaceID string) error {
	if s.translationSvc == nil {
		return nil
	}

	collections, err := s.collectionRepo.ListBySpace(ctx, spaceID)
	if err != nil {
		return err
	}

	for _, collection := range collections {
		if err := s.translationSvc.RefreshCollectionSource(ctx, collection.ID); err != nil {
			return err
		}
	}

	return nil
}

func (s *DocsHelpcenterService) ensureDefaultLocaleCollectionMirrorsForWorkspace(ctx context.Context, workspaceID string) error {
	if s.translationSvc == nil {
		return nil
	}

	spaces, err := s.spaceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}

	for _, space := range spaces {
		if space.Type != model.SpaceTypeExternalCapable {
			continue
		}
		if err := s.ensureDefaultLocaleCollectionMirrorsForSpace(ctx, space.ID); err != nil {
			return err
		}
	}

	return nil
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
	if _, publicID, ok := parseDocsHelpcenterCollectionKey(slug); ok {
		coll, err := s.collectionRepo.GetByPublicID(ctx, publicID)
		if err != nil {
			return nil, "", false, err
		}
		if coll == nil || coll.SpaceID != spaceID {
			return nil, "", false, fmt.Errorf("collection not found")
		}
		return s.resolvePublicCollectionTranslationByID(ctx, cfg, coll.ID, requestedLocale, func() error {
			return s.ensureDefaultLocaleCollectionMirrorsForSpace(ctx, spaceID)
		})
	}

	translation, err := s.hcRepo.GetPublicCollectionTranslationBySlug(ctx, spaceID, requestedLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if translation != nil {
		return translation, requestedLocale, false, nil
	}

	if err := s.ensureDefaultLocaleCollectionMirrorsForSpace(ctx, spaceID); err != nil {
		return nil, "", false, err
	}

	translation, err = s.hcRepo.GetPublicCollectionTranslationBySlug(ctx, spaceID, requestedLocale, slug)
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
	if _, publicID, ok := parseDocsHelpcenterCollectionKey(slug); ok {
		coll, err := s.collectionRepo.GetByPublicID(ctx, publicID)
		if err != nil {
			return nil, "", false, err
		}
		if coll == nil || coll.WorkspaceID != workspaceID {
			return nil, "", false, fmt.Errorf("collection not found")
		}
		space, err := s.spaceRepo.GetByID(ctx, coll.SpaceID)
		if err != nil {
			return nil, "", false, err
		}
		if space == nil || space.Type != model.SpaceTypeExternalCapable {
			return nil, "", false, fmt.Errorf("collection not found")
		}
		return s.resolvePublicCollectionTranslationByID(ctx, cfg, coll.ID, requestedLocale, func() error {
			return s.ensureDefaultLocaleCollectionMirrorsForWorkspace(ctx, workspaceID)
		})
	}

	translation, err := s.hcRepo.GetPublicCollectionTranslationByWorkspaceSlug(ctx, workspaceID, requestedLocale, slug)
	if err != nil {
		return nil, "", false, err
	}
	if translation != nil {
		return translation, requestedLocale, false, nil
	}

	if err := s.ensureDefaultLocaleCollectionMirrorsForWorkspace(ctx, workspaceID); err != nil {
		return nil, "", false, err
	}

	translation, err = s.hcRepo.GetPublicCollectionTranslationByWorkspaceSlug(ctx, workspaceID, requestedLocale, slug)
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

func (s *DocsHelpcenterService) resolvePublicCollectionTranslationByID(ctx context.Context, cfg *model.DocsHelpcenterConfig, collectionID, requestedLocale string, ensureDefaultMirrors func() error) (*model.DocsHelpcenterCollectionTranslation, string, bool, error) {
	translation, err := s.hcRepo.GetPublicCollectionTranslationByCollectionID(ctx, collectionID, requestedLocale)
	if err != nil {
		return nil, "", false, err
	}
	if translation != nil {
		return translation, requestedLocale, false, nil
	}

	if ensureDefaultMirrors != nil {
		if err := ensureDefaultMirrors(); err != nil {
			return nil, "", false, err
		}
	}

	translation, err = s.hcRepo.GetPublicCollectionTranslationByCollectionID(ctx, collectionID, requestedLocale)
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

	fallback, err := s.hcRepo.GetPublicCollectionTranslationByCollectionID(ctx, collectionID, defaultLocale)
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
func (s *DocsHelpcenterService) listPublicSpacesUncached(ctx context.Context, workspaceID, requestedLocale string) ([]model.PublicSpaceResponse, error) {
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
			Icon:        iconcatalog.ResolvePublicValue(sp.Icon, "folder"),
			Description: translation.Description,
		})
	}
	return result, nil
}

// getSpaceNavigationUncached is the uncached implementation of GetSpaceNavigation.
// The exported GetSpaceNavigation in docs_helpcenter_cache.go wraps this with
// a cache-aside layer when the service has been configured with a cache.
func (s *DocsHelpcenterService) getSpaceNavigationUncached(ctx context.Context, workspaceID, requestedLocale, spaceSlug string) ([]model.PublicNavCollection, error) {
	cfg, defaultLocale, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureDefaultLocaleMirrors(ctx, workspaceID); err != nil {
		return nil, err
	}

	spaceTranslation, _, _, err := s.resolvePublicSpaceTranslationBySlug(ctx, cfg, workspaceID, requestedLocale, spaceSlug)
	if err != nil {
		return nil, err
	}

	if err := s.ensureDefaultLocaleCollectionMirrorsForSpace(ctx, spaceTranslation.SpaceID); err != nil {
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
	publicIDs := s.loadHelpcenterPublicIDs(ctx, docs)

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

		article := model.PublicNavArticle{
			ID:          doc.ID,
			Title:       translation.Title,
			Slug:        stringValue(translation.Slug),
			PublicID:    publicIDs[doc.ID],
			Position:    doc.Position,
			PublishedAt: formatPublicPublishedAt(translation.PublishedAt),
		}
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

	// Build a set of collection IDs that have articles so we can also
	// include ancestor collections that have no direct articles but
	// contain subcollections with articles.
	collectionsWithArticles := make(map[string]bool, len(articlesByCollection))
	for id := range articlesByCollection {
		collectionsWithArticles[id] = true
	}
	// Walk up parent chains so parent collections are included even
	// when they have no direct articles.
	collectionByID := make(map[string]model.DocsCollection, len(collections))
	for _, c := range collections {
		collectionByID[c.ID] = c
	}
	for id := range collectionsWithArticles {
		cur := collectionByID[id]
		for cur.ParentCollectionID != nil && *cur.ParentCollectionID != "" {
			pid := *cur.ParentCollectionID
			if collectionsWithArticles[pid] {
				break
			}
			collectionsWithArticles[pid] = true
			parent, ok := collectionByID[pid]
			if !ok {
				break
			}
			cur = parent
		}
	}

	result := make([]model.PublicNavCollection, 0, len(collections)+1)
	for _, collection := range collections {
		if !collectionsWithArticles[collection.ID] {
			continue
		}

		translation, ok := requestedCollectionByID[collection.ID]
		if !ok {
			translation, ok = fallbackCollectionByID[collection.ID]
			if !ok {
				continue
			}
		}

		articles := articlesByCollection[collection.ID]
		if articles == nil {
			articles = []model.PublicNavArticle{}
		}
		result = append(result, model.PublicNavCollection{
			ID:                 collection.ID,
			Name:               translation.Name,
			Slug:               stringValue(translation.Slug),
			PublicID:           collection.PublicID,
			Icon:               iconcatalog.ResolvePublicValue(collection.Icon, "folder"),
			ParentCollectionID: collection.ParentCollectionID,
			Depth:              collection.Depth,
			Position:           collection.Position,
			Articles:           articles,
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
func (s *DocsHelpcenterService) getPublicArticleUncached(ctx context.Context, workspaceID, requestedLocale, spaceSlug, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
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
	articleExt, err := s.hcRepo.GetArticle(ctx, translation.DocumentID)
	if err != nil {
		return nil, err
	}
	publicID := ""
	if articleExt != nil {
		publicID = articleExt.PublicID
	}

	if collectionName == nil && doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionName = &coll.Name
		}
	}

	var contentHTML *string
	if len(translation.Content) > 0 {
		rendered, err := RenderPublicDocsHTML(translation.Content)
		if err == nil && rendered != "" {
			contentHTML = &rendered
		}
	}

	publishedAt := formatPublicPublishedAt(translation.PublishedAt)

	go func() {
		_ = s.hcRepo.IncrementTranslatedViewCount(ctx, translation.DocumentID, resolvedLocale)
	}()

	var collectionSlugValue *string
	var collectionPublicIDValue *string
	if resolvedColl != nil {
		collectionSlugValue = resolvedColl.Slug
		if publicID := s.collectionPublicID(ctx, resolvedColl.CollectionID); publicID != "" {
			collectionPublicIDValue = &publicID
		}
	}

	return &model.PublicArticleResponse{
		ID:                 doc.ID,
		Title:              translation.Title,
		Slug:               stringValue(translation.Slug),
		PublicID:           publicID,
		Locale:             resolvedLocale,
		RequestedLocale:    requestedLocale,
		IsFallback:         fellBack,
		Excerpt:            translation.Excerpt,
		Icon:               iconcatalog.ResolvePublicValue(doc.Icon, "file01"),
		Status:             doc.Status,
		SpaceSlug:          stringValue(spaceTranslation.Slug),
		CollectionID:       doc.CollectionID,
		CollectionName:     collectionName,
		CollectionSlug:     collectionSlugValue,
		CollectionPublicID: collectionPublicIDValue,
		PublishedAt:        publishedAt,
		SEOTitle:           translation.SEOTitle,
		SEODescription:     translation.SEODescription,
		OGTitle:            translation.OGTitle,
		OGDescription:      translation.OGDescription,
		OGImageURL:         translation.OGImageURL,
		OGImageAlt:         translation.OGImageAlt,
		HelpfulCount:       translation.HelpfulCount,
		NotHelpfulCount:    translation.NotHelpfulCount,
		ViewCount:          translation.ViewCount,
		ContentHTML:        contentHTML,
	}, nil
}

// GetPublicLocalizedCollection returns a translated collection page and its translated articles.
func (s *DocsHelpcenterService) getPublicLocalizedCollectionUncached(ctx context.Context, workspaceID, requestedLocale, spaceSlug, collectionSlug string) (*model.PublicNavCollection, []model.PublicNavArticle, error) {
	cfg, defaultLocale, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	if err := s.ensureDefaultLocaleMirrors(ctx, workspaceID); err != nil {
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
	publicIDs := s.loadHelpcenterPublicIDs(ctx, docs)
	for _, doc := range docs {
		translation, ok := requestedArticleByID[doc.ID]
		if !ok {
			translation, ok = fallbackArticleByID[doc.ID]
			if !ok {
				continue
			}
		}
		articles = append(articles, model.PublicNavArticle{
			ID:          doc.ID,
			Title:       translation.Title,
			Slug:        stringValue(translation.Slug),
			PublicID:    publicIDs[doc.ID],
			PublishedAt: formatPublicPublishedAt(translation.PublishedAt),
		})
	}

	collection, err := s.collectionRepo.GetByID(ctx, collectionTranslation.CollectionID)
	if err != nil {
		return nil, nil, err
	}
	var icon *string
	var collectionPublicID string
	if collection != nil {
		icon = collection.Icon
		collectionPublicID = collection.PublicID
	}

	return &model.PublicNavCollection{
		ID:        collectionTranslation.CollectionID,
		Name:      collectionTranslation.Name,
		Slug:      stringValue(collectionTranslation.Slug),
		PublicID:  collectionPublicID,
		SpaceSlug: stringValue(spaceTranslation.Slug),
		Icon:      iconcatalog.ResolvePublicValue(icon, "folder"),
		Articles:  articles,
	}, articles, nil
}

func (s *DocsHelpcenterService) getPublicLocalizedCollectionByCanonicalPathUncached(ctx context.Context, workspaceID, requestedLocale, collectionSlug string) (*model.PublicNavCollection, []model.PublicNavArticle, error) {
	cfg, defaultLocale, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	if err := s.ensureDefaultLocaleMirrors(ctx, workspaceID); err != nil {
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
	publicIDs := s.loadHelpcenterPublicIDs(ctx, docs)
	for _, doc := range docs {
		translation, ok := requestedArticleByID[doc.ID]
		if !ok {
			translation, ok = fallbackArticleByID[doc.ID]
			if !ok {
				continue
			}
		}
		articles = append(articles, model.PublicNavArticle{
			ID:          doc.ID,
			Title:       translation.Title,
			Slug:        stringValue(translation.Slug),
			PublicID:    publicIDs[doc.ID],
			PublishedAt: formatPublicPublishedAt(translation.PublishedAt),
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
		PublicID:  collection.PublicID,
		SpaceSlug: resolvedSpaceSlug,
		Icon:      iconcatalog.ResolvePublicValue(icon, "folder"),
		Articles:  articles,
	}, articles, nil
}

func (s *DocsHelpcenterService) getPublicArticleByLocalizedCanonicalPathUncached(ctx context.Context, workspaceID, requestedLocale, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	cfg, _, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureDefaultLocaleMirrors(ctx, workspaceID); err != nil {
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
	articleExt, err := s.hcRepo.GetArticle(ctx, translation.DocumentID)
	if err != nil {
		return nil, err
	}
	publicID := ""
	if articleExt != nil {
		publicID = articleExt.PublicID
	}

	collection, err := s.collectionRepo.GetByID(ctx, collectionTranslation.CollectionID)
	if err != nil {
		return nil, err
	}
	var collectionName *string
	var collectionSlugValue *string
	var collectionPublicIDValue *string
	var spaceSlugValue string
	if collection != nil {
		collectionName = &collectionTranslation.Name
		collectionSlugValue = collectionTranslation.Slug
		if collection.PublicID != "" {
			collectionPublicIDValue = &collection.PublicID
		}
		if space, err := s.spaceRepo.GetByID(ctx, collection.SpaceID); err == nil && space != nil {
			spaceSlugValue = space.Slug
		}
	}

	var contentHTML *string
	if len(translation.Content) > 0 {
		rendered, err := RenderPublicDocsHTML(translation.Content)
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
		ID:                 doc.ID,
		Title:              translation.Title,
		Slug:               stringValue(translation.Slug),
		PublicID:           publicID,
		Locale:             resolvedLocale,
		RequestedLocale:    requestedLocale,
		IsFallback:         fellBack,
		Excerpt:            translation.Excerpt,
		Icon:               iconcatalog.ResolvePublicValue(doc.Icon, "file01"),
		Status:             doc.Status,
		SpaceSlug:          spaceSlugValue,
		CollectionID:       doc.CollectionID,
		CollectionName:     collectionName,
		CollectionSlug:     collectionSlugValue,
		CollectionPublicID: collectionPublicIDValue,
		PublishedAt:        publishedAt,
		SEOTitle:           translation.SEOTitle,
		SEODescription:     translation.SEODescription,
		OGTitle:            translation.OGTitle,
		OGDescription:      translation.OGDescription,
		OGImageURL:         translation.OGImageURL,
		OGImageAlt:         translation.OGImageAlt,
		HelpfulCount:       translation.HelpfulCount,
		NotHelpfulCount:    translation.NotHelpfulCount,
		ViewCount:          translation.ViewCount,
		ContentHTML:        contentHTML,
	}, nil
}

func (s *DocsHelpcenterService) getPublicArticleByLocalizedCanonicalKeyUncached(ctx context.Context, workspaceID, requestedLocale, articleKey string) (*model.PublicArticleResponse, error) {
	_, publicID, ok := parseDocsHelpcenterArticleKey(articleKey)
	if !ok {
		return nil, fmt.Errorf("article not found")
	}

	cfg, _, err := s.getPublicLocaleConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureDefaultLocaleMirrors(ctx, workspaceID); err != nil {
		return nil, err
	}

	translation, resolvedLocale, fellBack, err := func() (*model.DocsHelpcenterArticleTranslation, string, bool, error) {
		translation, err := s.hcRepo.GetPublicArticleTranslationByPublicID(ctx, workspaceID, requestedLocale, publicID)
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

		fallback, err := s.hcRepo.GetPublicArticleTranslationByPublicID(ctx, workspaceID, defaultLocale, publicID)
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

	articleExt, err := s.hcRepo.GetArticle(ctx, translation.DocumentID)
	if err != nil {
		return nil, err
	}

	var collectionName *string
	var collectionSlugValue *string
	var collectionPublicIDValue *string
	var spaceSlugValue string
	if doc.CollectionID != nil {
		collection, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err != nil {
			return nil, err
		}
		if collection != nil {
			collectionName = &collection.Name
			if collection.Slug != "" {
				collectionSlugValue = &collection.Slug
			}
			if collection.PublicID != "" {
				collectionPublicIDValue = &collection.PublicID
			}
			space, err := s.spaceRepo.GetByID(ctx, collection.SpaceID)
			if err == nil && space != nil {
				spaceSlugValue = space.Slug
			}
		}
	} else {
		space, err := s.spaceRepo.GetByID(ctx, doc.SpaceID)
		if err == nil && space != nil {
			spaceSlugValue = space.Slug
		}
	}

	var contentHTML *string
	if len(translation.Content) > 0 {
		rendered, err := RenderPublicDocsHTML(translation.Content)
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

	publicIDValue := ""
	if articleExt != nil {
		publicIDValue = articleExt.PublicID
	}

	return &model.PublicArticleResponse{
		ID:                 doc.ID,
		Title:              translation.Title,
		Slug:               stringValue(translation.Slug),
		PublicID:           publicIDValue,
		Locale:             resolvedLocale,
		RequestedLocale:    requestedLocale,
		IsFallback:         fellBack,
		Excerpt:            translation.Excerpt,
		Icon:               iconcatalog.ResolvePublicValue(doc.Icon, "file01"),
		Status:             doc.Status,
		SpaceSlug:          spaceSlugValue,
		CollectionID:       doc.CollectionID,
		CollectionName:     collectionName,
		CollectionSlug:     collectionSlugValue,
		CollectionPublicID: collectionPublicIDValue,
		PublishedAt:        publishedAt,
		SEOTitle:           translation.SEOTitle,
		SEODescription:     translation.SEODescription,
		OGTitle:            translation.OGTitle,
		OGDescription:      translation.OGDescription,
		OGImageURL:         translation.OGImageURL,
		OGImageAlt:         translation.OGImageAlt,
		HelpfulCount:       translation.HelpfulCount,
		NotHelpfulCount:    translation.NotHelpfulCount,
		ViewCount:          translation.ViewCount,
		ContentHTML:        contentHTML,
	}, nil
}

// GetPublicArticleByCanonicalPath returns a public article by collection slug and article slug.
func (s *DocsHelpcenterService) getPublicArticleByCanonicalPathUncached(ctx context.Context, workspaceID, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	var (
		doc     *model.DocsDocument
		ha      *model.DocsHelpcenterArticle
		content *model.DocsContent
		err     error
	)
	if _, publicID, ok := parseDocsHelpcenterCollectionKey(collectionSlug); ok {
		collection, lookupErr := s.collectionRepo.GetByPublicID(ctx, publicID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if collection == nil || collection.WorkspaceID != workspaceID {
			return nil, nil
		}
		doc, ha, content, err = s.hcRepo.GetPublicArticleByCollectionIDAndSlug(ctx, collection.ID, articleSlug)
	} else {
		doc, ha, content, err = s.hcRepo.GetPublicArticleByCollectionSlug(ctx, workspaceID, collectionSlug, articleSlug)
	}
	if err != nil {
		return nil, err
	}
	if doc == nil || ha == nil {
		return nil, nil
	}

	// Resolve collection name, slug, and space slug if present.
	var collectionName *string
	var resolvedCollSlug *string
	var resolvedCollPublicID *string
	var spaceSlug string
	if doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionName = &coll.Name
			slug := coll.Slug
			resolvedCollSlug = &slug
			if coll.PublicID != "" {
				resolvedCollPublicID = &coll.PublicID
			}
			if space, err := s.spaceRepo.GetByID(ctx, coll.SpaceID); err == nil && space != nil {
				spaceSlug = space.Slug
			}
		}
	}

	// Render TipTap JSON -> HTML for public display.
	var contentHTML *string
	if content != nil && len(content.Content) > 0 {
		rendered, err := RenderPublicDocsHTML(content.Content)
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
		ID:                 doc.ID,
		Title:              doc.Title,
		Slug:               ha.Slug,
		PublicID:           ha.PublicID,
		Excerpt:            doc.Excerpt,
		Icon:               iconcatalog.ResolvePublicValue(doc.Icon, "file01"),
		Status:             doc.Status,
		SpaceSlug:          spaceSlug,
		CollectionID:       doc.CollectionID,
		CollectionName:     collectionName,
		CollectionSlug:     resolvedCollSlug,
		CollectionPublicID: resolvedCollPublicID,
		PublishedAt:        publishedAt,
		SEOTitle:           ha.SEOTitle,
		SEODescription:     ha.SEODescription,
		OGTitle:            ha.OGTitle,
		OGDescription:      ha.OGDescription,
		OGImageURL:         ha.OGImageURL,
		OGImageAlt:         ha.OGImageAlt,
		HelpfulCount:       ha.HelpfulCount,
		NotHelpfulCount:    ha.NotHelpfulCount,
		ViewCount:          ha.ViewCount,
		ContentHTML:        contentHTML,
	}, nil
}

func (s *DocsHelpcenterService) getPublicArticleByCanonicalKeyUncached(ctx context.Context, workspaceID, articleKey string) (*model.PublicArticleResponse, error) {
	_, publicID, ok := parseDocsHelpcenterArticleKey(articleKey)
	if !ok {
		return nil, nil
	}

	doc, ha, content, err := s.hcRepo.GetPublicArticleByPublicID(ctx, workspaceID, publicID)
	if err != nil {
		return nil, err
	}
	if doc == nil || ha == nil {
		return nil, nil
	}

	var collectionName *string
	var resolvedCollSlug *string
	var resolvedCollPublicID *string
	var spaceSlug string
	if doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionName = &coll.Name
			if coll.Slug != "" {
				resolvedCollSlug = &coll.Slug
			}
			if coll.PublicID != "" {
				resolvedCollPublicID = &coll.PublicID
			}
			if space, err := s.spaceRepo.GetByID(ctx, coll.SpaceID); err == nil && space != nil {
				spaceSlug = space.Slug
			}
		}
	} else if space, err := s.spaceRepo.GetByID(ctx, doc.SpaceID); err == nil && space != nil {
		spaceSlug = space.Slug
	}

	var contentHTML *string
	if content != nil && len(content.Content) > 0 {
		rendered, err := RenderPublicDocsHTML(content.Content)
		if err == nil && rendered != "" {
			contentHTML = &rendered
		}
	}

	var publishedAt *string
	if ha.PublicPublishedAt != nil {
		formatted := ha.PublicPublishedAt.Format(time.RFC3339)
		publishedAt = &formatted
	}

	go func() {
		defer func() { recover() }()
		_ = s.hcRepo.IncrementViewCount(ctx, doc.ID)
	}()

	return &model.PublicArticleResponse{
		ID:                 doc.ID,
		Title:              doc.Title,
		Slug:               ha.Slug,
		PublicID:           ha.PublicID,
		Excerpt:            doc.Excerpt,
		Icon:               iconcatalog.ResolvePublicValue(doc.Icon, "file01"),
		Status:             doc.Status,
		SpaceSlug:          spaceSlug,
		CollectionID:       doc.CollectionID,
		CollectionName:     collectionName,
		CollectionSlug:     resolvedCollSlug,
		CollectionPublicID: resolvedCollPublicID,
		PublishedAt:        publishedAt,
		SEOTitle:           ha.SEOTitle,
		SEODescription:     ha.SEODescription,
		OGTitle:            ha.OGTitle,
		OGDescription:      ha.OGDescription,
		OGImageURL:         ha.OGImageURL,
		OGImageAlt:         ha.OGImageAlt,
		HelpfulCount:       ha.HelpfulCount,
		NotHelpfulCount:    ha.NotHelpfulCount,
		ViewCount:          ha.ViewCount,
		ContentHTML:        contentHTML,
	}, nil
}

// GetPublicCollection returns a collection, its published articles, and the parent space slug by workspace and collection key.
func (s *DocsHelpcenterService) getPublicCollectionUncached(ctx context.Context, workspaceID, collectionKey string) (*model.DocsCollection, []model.PublicNavArticle, string, error) {
	var (
		coll     *model.DocsCollection
		articles []model.PublicNavArticle
		err      error
	)
	if _, publicID, ok := parseDocsHelpcenterCollectionKey(collectionKey); ok {
		coll, err = s.collectionRepo.GetByPublicID(ctx, publicID)
		if err != nil {
			return nil, nil, "", err
		}
		if coll != nil && coll.WorkspaceID != workspaceID {
			coll = nil
		}
		if coll != nil {
			articles, err = s.hcRepo.ListPublicCollectionArticles(ctx, coll.ID)
			if err != nil {
				return nil, nil, "", err
			}
		}
	} else {
		coll, articles, err = s.hcRepo.GetPublicCollectionBySlug(ctx, workspaceID, collectionKey)
	}
	if err != nil {
		return nil, nil, "", err
	}
	if coll == nil {
		return nil, nil, "", nil
	}
	var spaceSlug string
	if space, err := s.spaceRepo.GetByID(ctx, coll.SpaceID); err == nil && space != nil {
		spaceSlug = space.Slug
	}
	publicCollection := *coll
	publicCollection.Icon = iconcatalog.ResolvePublicValue(coll.Icon, "folder")
	return &publicCollection, articles, spaceSlug, nil
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
		Icon:           iconcatalog.ResolvePublicValue(doc.Icon, "file01"),
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
	items, total, err := s.redirectRepo.List(ctx, workspaceID, filter)
	if err != nil {
		return nil, 0, err
	}
	s.hydrateRedirectTargetPaths(ctx, workspaceID, items)
	return items, total, nil
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
	// Resolve and persist canonical target_path after creation.
	s.hydrateRedirectTargetPath(ctx, workspaceID, redirect)
	if redirect.TargetPath != nil && *redirect.TargetPath != "" {
		if _, err := s.redirectRepo.Update(ctx, redirect.ID, map[string]interface{}{"target_path": *redirect.TargetPath}); err != nil {
			slog.WarnContext(ctx, "persist redirect target_path failed", "redirect_id", redirect.ID, "error", err)
		}
	}
	s.InvalidateHelpcenterCacheForWorkspace(ctx, workspaceID)
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
	slugChanged := false
	if req.TargetCollectionSlug != nil {
		if *req.TargetCollectionSlug == "" {
			return nil, fmt.Errorf("target_collection_slug is required")
		}
		updates["target_collection_slug"] = *req.TargetCollectionSlug
		slugChanged = true
	}
	if req.TargetArticleSlug != nil {
		if *req.TargetArticleSlug == "" {
			updates["target_article_slug"] = nil
		} else {
			updates["target_article_slug"] = *req.TargetArticleSlug
		}
		slugChanged = true
	}
	// Clear stale target_path when slug fields change so resolution
	// falls back to slug rebuild until recomputed.
	if slugChanged {
		updates["target_path"] = nil
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}
	redirect, err := s.redirectRepo.Update(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	// Recompute and persist target_path after slug changes.
	s.hydrateRedirectTargetPath(ctx, redirect.WorkspaceID, redirect)
	if slugChanged && redirect.TargetPath != nil && *redirect.TargetPath != "" {
		if _, err := s.redirectRepo.Update(ctx, redirect.ID, map[string]interface{}{"target_path": *redirect.TargetPath}); err != nil {
			slog.WarnContext(ctx, "persist redirect target_path failed", "redirect_id", redirect.ID, "error", err)
		}
	}
	s.InvalidateHelpcenterCacheForWorkspace(ctx, redirect.WorkspaceID)
	return redirect, nil
}

func (s *DocsHelpcenterService) DeleteRedirect(ctx context.Context, id string) error {
	redirect, err := s.redirectRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.redirectRepo.Delete(ctx, id); err != nil {
		return err
	}
	if redirect != nil {
		s.InvalidateHelpcenterCacheForWorkspace(ctx, redirect.WorkspaceID)
	}
	return nil
}

// ResolvePublicPath resolves a legacy or imported URL path to a redirect target.
func (s *DocsHelpcenterService) resolvePublicPathUncached(ctx context.Context, workspaceID, path string) (string, error) {
	cfg, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if cfg == nil {
		return "", nil
	}

	return s.resolvePublicPathTarget(ctx, cfg, workspaceID, path, map[string]struct{}{}, 0)
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
	if err := s.hcRepo.IncrementFeedbackCount(ctx, documentID, req.IsHelpful); err != nil {
		return err
	}
	s.invalidateHelpcenterCacheForDocument(ctx, documentID)
	return nil
}

func (s *DocsHelpcenterService) SubmitFeedbackForLocale(ctx context.Context, documentID, locale string, req model.DocsArticleFeedbackRequest) error {
	if err := s.SubmitFeedback(ctx, documentID, req); err != nil {
		return err
	}
	if locale == "" {
		return nil
	}
	if err := s.hcRepo.IncrementTranslatedFeedbackCount(ctx, documentID, locale, req.IsHelpful); err != nil {
		return err
	}
	s.invalidateHelpcenterCacheForDocument(ctx, documentID)
	return nil
}

func (s *DocsHelpcenterService) invalidateHelpcenterCacheForDocument(ctx context.Context, documentID string) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil || doc == nil {
		if err != nil {
			slog.WarnContext(ctx, "hc cache: document invalidation lookup failed", "document_id", documentID, "error", err)
		}
		return
	}
	s.InvalidateHelpcenterCacheForWorkspace(ctx, doc.WorkspaceID)
}

func formatPublicPublishedAt(value *time.Time) *string {
	if value == nil {
		return nil
	}

	formatted := value.Format(time.RFC3339)
	return &formatted
}

func (s *DocsHelpcenterService) loadHelpcenterPublicIDs(ctx context.Context, docs []model.DocsDocument) map[string]string {
	result := make(map[string]string, len(docs))
	for _, doc := range docs {
		article, err := s.hcRepo.GetArticle(ctx, doc.ID)
		if err != nil || article == nil {
			continue
		}
		result[doc.ID] = article.PublicID
	}
	return result
}

func (s *DocsHelpcenterService) collectionPublicID(ctx context.Context, collectionID string) string {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil || collection == nil {
		return ""
	}
	return collection.PublicID
}

func (s *DocsHelpcenterService) hydrateRedirectTargetPaths(ctx context.Context, workspaceID string, redirects []model.DocsRedirect) {
	for i := range redirects {
		s.hydrateRedirectTargetPath(ctx, workspaceID, &redirects[i])
	}
}

// setRedirectTargetPath computes and sets the canonical PublicID-backed
// target_path on a redirect from the given entity PublicIDs. If a
// PublicID is empty, target_path is left nil (legacy slug fallback).
// cfg and locale are optional — pass nil/"" for default-locale paths.
func setRedirectTargetPath(redirect *model.DocsRedirect, collectionPublicID string, articlePublicID string) {
	setRedirectTargetPathLocalized(redirect, nil, "", collectionPublicID, articlePublicID)
}

// setRedirectTargetPathLocalized is like setRedirectTargetPath but
// produces locale-aware canonical paths for multilingual deployments.
func setRedirectTargetPathLocalized(redirect *model.DocsRedirect, cfg *model.DocsHelpcenterConfig, locale string, collectionPublicID string, articlePublicID string) {
	if redirect == nil {
		return
	}
	if redirect.TargetArticleSlug != nil && *redirect.TargetArticleSlug != "" && articlePublicID != "" {
		p := buildDocsHelpcenterArticleCanonicalPath(cfg, locale, *redirect.TargetArticleSlug, articlePublicID)
		redirect.TargetPath = &p
	} else if redirect.TargetCollectionSlug != "" && collectionPublicID != "" {
		p := buildDocsHelpcenterCollectionCanonicalPath(cfg, locale, redirect.TargetCollectionSlug, collectionPublicID)
		redirect.TargetPath = &p
	}
}

func (s *DocsHelpcenterService) hydrateRedirectTargetPath(ctx context.Context, workspaceID string, redirect *model.DocsRedirect) {
	if redirect == nil {
		return
	}

	targetPath, err := s.resolveRedirectTargetPath(ctx, workspaceID, redirect)
	if err != nil || targetPath == "" {
		return
	}
	redirect.TargetPath = &targetPath
}

func (s *DocsHelpcenterService) resolveRedirectTargetPath(ctx context.Context, workspaceID string, redirect *model.DocsRedirect) (string, error) {
	if redirect == nil {
		return "", nil
	}
	// Prefer the pre-computed PublicID-backed target path when present.
	// Fall back to legacy slug-based path rebuild for old rows.
	var target string
	if redirect.TargetPath != nil && *redirect.TargetPath != "" {
		target = *redirect.TargetPath
	} else {
		target = buildDocsRedirectPath(redirect.TargetCollectionSlug, redirect.TargetArticleSlug)
	}
	if target == "" || target == "/" {
		return target, nil
	}

	resolved, err := s.ResolvePublicPath(ctx, workspaceID, target)
	if err != nil {
		return "", err
	}
	if resolved != "" {
		return resolved, nil
	}
	return target, nil
}

func (s *DocsHelpcenterService) resolvePublicPathTarget(
	ctx context.Context,
	cfg *model.DocsHelpcenterConfig,
	workspaceID string,
	path string,
	seen map[string]struct{},
	depth int,
) (string, error) {
	normalizedPath := normalizeDocsRedirectSourcePath(path)
	if normalizedPath == "" {
		return "", nil
	}
	if depth > 8 {
		return "", nil
	}
	if _, exists := seen[normalizedPath]; exists {
		return "", nil
	}
	seen[normalizedPath] = struct{}{}

	if redirect, err := s.redirectRepo.GetBySourcePath(ctx, workspaceID, normalizedPath); err != nil {
		return "", err
	} else if redirect != nil {
		// Prefer pre-computed PublicID-backed target path; fall back to
		// slug-based rebuild for legacy rows without target_path.
		var target string
		if redirect.TargetPath != nil && *redirect.TargetPath != "" {
			target = *redirect.TargetPath
		} else {
			target = buildDocsRedirectPath(redirect.TargetCollectionSlug, redirect.TargetArticleSlug)
		}
		if target == "" || target == normalizedPath {
			return "", nil
		}
		if resolved, err := s.resolveDynamicPublicPath(ctx, cfg, workspaceID, target); err != nil {
			return "", err
		} else if resolved != "" && resolved != normalizedPath {
			return resolved, nil
		}
		if resolved, err := s.resolvePublicPathTarget(ctx, cfg, workspaceID, target, seen, depth+1); err != nil {
			return "", err
		} else if resolved != "" && resolved != normalizedPath {
			return resolved, nil
		}
		return target, nil
	}

	return s.resolveDynamicPublicPath(ctx, cfg, workspaceID, normalizedPath)
}

func (s *DocsHelpcenterService) resolveDynamicPublicPath(
	ctx context.Context,
	cfg *model.DocsHelpcenterConfig,
	workspaceID string,
	path string,
) (string, error) {
	multilingual := docsHelpcenterMultilingualEnabled(cfg)
	defaultLocale := defaultHelpcenterLocale(cfg)
	locale, hasLocalePrefix, segments := parseDocsHelpcenterPublicPath(cfg, path)
	normalizedPath := normalizeDocsRedirectSourcePath(path)

	if hasLocalePrefix && !multilingual {
		trimmed := "/" + strings.Join(segments, "/")
		if trimmed == "/" {
			return "/", nil
		}
		if trimmed != normalizedPath {
			if resolved, err := s.resolveDynamicPublicPath(ctx, cfg, workspaceID, trimmed); err != nil {
				return "", err
			} else if resolved != "" {
				return resolved, nil
			}
			return trimmed, nil
		}
	}

	if len(segments) == 0 {
		if multilingual && !hasLocalePrefix {
			return buildCanonicalHomePath(defaultLocale, true), nil
		}
		if !multilingual && hasLocalePrefix {
			return "/", nil
		}
		return "", nil
	}

	switch segments[0] {
	case "c":
		if len(segments) != 2 {
			return "", nil
		}
		collectionSlug := segments[1]
		if multilingual {
			translation, resolvedLocale, _, err := s.resolvePublicCollectionTranslationByCanonicalSlug(ctx, cfg, workspaceID, locale, collectionSlug)
			if err != nil || translation == nil {
				return "", nil
			}
			target := buildDocsHelpcenterCollectionCanonicalPath(cfg, resolvedLocale, stringValue(translation.Slug), s.collectionPublicID(ctx, translation.CollectionID))
			if target != normalizedPath {
				return target, nil
			}
			return "", nil
		}
		coll, _, _, err := s.GetPublicCollection(ctx, workspaceID, collectionSlug)
		if err != nil || coll == nil {
			return "", nil
		}
		target := buildDocsHelpcenterCollectionCanonicalPath(cfg, defaultLocale, coll.Slug, coll.PublicID)
		if target != normalizedPath {
			return target, nil
		}
		return "", nil
	case "articles":
		if len(segments) != 2 {
			return "", nil
		}
		if multilingual {
			article, err := s.GetPublicArticleByLocalizedCanonicalKey(ctx, workspaceID, locale, segments[1])
			if err != nil || article == nil {
				return "", nil
			}
			target := buildDocsHelpcenterArticleCanonicalPath(cfg, article.Locale, article.Slug, article.PublicID)
			if target != normalizedPath {
				return target, nil
			}
			return "", nil
		}
		article, err := s.GetPublicArticleByCanonicalKey(ctx, workspaceID, segments[1])
		if err != nil || article == nil {
			return "", nil
		}
		target := buildDocsHelpcenterArticleCanonicalPath(cfg, defaultLocale, article.Slug, article.PublicID)
		if target != normalizedPath {
			return target, nil
		}
		return "", nil
	}

	if len(segments) == 1 {
		collectionSlug := segments[0]
		if multilingual {
			translation, resolvedLocale, _, err := s.resolvePublicCollectionTranslationByCanonicalSlug(ctx, cfg, workspaceID, locale, collectionSlug)
			if err == nil && translation != nil {
				return buildDocsHelpcenterCollectionCanonicalPath(cfg, resolvedLocale, stringValue(translation.Slug), s.collectionPublicID(ctx, translation.CollectionID)), nil
			}
		} else if coll, _, _, err := s.GetPublicCollection(ctx, workspaceID, collectionSlug); err == nil && coll != nil {
			return buildDocsHelpcenterCollectionCanonicalPath(cfg, defaultLocale, coll.Slug, coll.PublicID), nil
		}
		return "", nil
	}

	if len(segments) >= 2 {
		legacyCollectionSlug := segments[len(segments)-2]
		legacyArticleSlug := segments[len(segments)-1]

		if multilingual {
			article, err := s.GetPublicArticleByLocalizedCanonicalPath(ctx, workspaceID, locale, legacyCollectionSlug, legacyArticleSlug)
			if err == nil && article != nil {
				return buildDocsHelpcenterArticleCanonicalPath(cfg, article.Locale, article.Slug, article.PublicID), nil
			}
		} else if article, err := s.GetPublicArticleByCanonicalPath(ctx, workspaceID, legacyCollectionSlug, legacyArticleSlug); err == nil && article != nil {
			return buildDocsHelpcenterArticleCanonicalPath(cfg, defaultLocale, article.Slug, article.PublicID), nil
		}
	}

	return "", nil
}

func parseDocsHelpcenterPublicPath(cfg *model.DocsHelpcenterConfig, path string) (locale string, hasLocalePrefix bool, segments []string) {
	normalized := normalizeDocsRedirectSourcePath(path)
	if normalized == "" || normalized == "/" {
		return defaultHelpcenterLocale(cfg), false, nil
	}

	parts := strings.Split(strings.Trim(normalized, "/"), "/")
	if len(parts) == 0 {
		return defaultHelpcenterLocale(cfg), false, nil
	}

	first := strings.TrimSpace(strings.ToLower(parts[0]))
	for _, enabled := range cfg.EnabledLocales {
		if strings.TrimSpace(strings.ToLower(enabled)) == first {
			return first, true, parts[1:]
		}
	}

	return defaultHelpcenterLocale(cfg), false, parts
}

func buildCanonicalHomePath(locale string, multilingual bool) string {
	if multilingual {
		return "/" + strings.Trim(strings.TrimSpace(locale), "/")
	}
	return "/"
}
