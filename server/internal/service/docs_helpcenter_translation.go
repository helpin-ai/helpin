package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/docsi18n"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// DocsHelpcenterTranslationService handles translation-specific business rules.
type DocsHelpcenterTranslationService struct {
	translationRepo *repository.DocsHelpcenterTranslationRepository
	hcRepo          *repository.DocsHelpcenterRepository
	publicationRepo *repository.DocsHelpcenterPublicationRepository
	redirectRepo    *repository.DocsRedirectRepository
	docRepo         *repository.DocsDocumentRepository
	contentRepo     *repository.DocsContentRepository
	spaceRepo       *repository.DocsSpaceRepository
	collectionRepo  *repository.DocsCollectionRepository
	llmProvider     llm.Provider
}

// NewDocsHelpcenterTranslationService creates a new multilingual help-center service.
func NewDocsHelpcenterTranslationService(
	translationRepo *repository.DocsHelpcenterTranslationRepository,
	hcRepo *repository.DocsHelpcenterRepository,
	publicationRepo *repository.DocsHelpcenterPublicationRepository,
	redirectRepo *repository.DocsRedirectRepository,
	docRepo *repository.DocsDocumentRepository,
	contentRepo *repository.DocsContentRepository,
	spaceRepo *repository.DocsSpaceRepository,
	collectionRepo *repository.DocsCollectionRepository,
	llmProvider llm.Provider,
) *DocsHelpcenterTranslationService {
	return &DocsHelpcenterTranslationService{
		translationRepo: translationRepo,
		hcRepo:          hcRepo,
		publicationRepo: publicationRepo,
		redirectRepo:    redirectRepo,
		docRepo:         docRepo,
		contentRepo:     contentRepo,
		spaceRepo:       spaceRepo,
		collectionRepo:  collectionRepo,
		llmProvider:     llmProvider,
	}
}

func (s *DocsHelpcenterTranslationService) GetLocales(ctx context.Context, workspaceID string) (*model.DocsHelpcenterConfig, error) {
	return s.hcRepo.GetConfig(ctx, workspaceID)
}

func (s *DocsHelpcenterTranslationService) UpdateLocales(ctx context.Context, workspaceID string, req model.UpdateDocsHelpcenterLocalesRequest) (*model.DocsHelpcenterConfig, error) {
	defaultLocale := strings.TrimSpace(strings.ToLower(req.DefaultLocale))
	if defaultLocale == "" {
		return nil, fmt.Errorf("default locale is required")
	}

	seen := map[string]struct{}{}
	enabled := make(model.DocsStringArray, 0, len(req.EnabledLocales))
	for _, locale := range req.EnabledLocales {
		normalized := strings.TrimSpace(strings.ToLower(locale))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		enabled = append(enabled, normalized)
	}
	if len(enabled) == 0 {
		return nil, fmt.Errorf("at least one enabled locale is required")
	}
	if _, ok := seen[defaultLocale]; !ok {
		return nil, fmt.Errorf("default locale must be included in enabled locales")
	}

	cfg, err := s.hcRepo.UpsertConfig(ctx, workspaceID, map[string]interface{}{
		"default_locale":             defaultLocale,
		"enabled_locales":            enabled,
		"show_language_switcher":     req.ShowLanguageSwitcher,
		"fallback_to_default_locale": req.FallbackToDefaultLocale,
	})
	if err != nil {
		return nil, err
	}

	if err := s.EnsureDefaultLocaleMirrorsForWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (s *DocsHelpcenterTranslationService) ListArticleTranslations(ctx context.Context, documentID string) ([]model.DocsHelpcenterArticleTranslation, error) {
	translations, err := s.translationRepo.ListArticleTranslations(ctx, documentID)
	if err != nil {
		return nil, err
	}
	for i := range translations {
		isPublishedOrNeedsReview := translations[i].Status == model.DocsHelpcenterTranslationStatusPublished ||
			translations[i].Status == model.DocsHelpcenterTranslationStatusNeedsReview
		if !isPublishedOrNeedsReview || translations[i].PublishedAt == nil {
			translations[i].LivePublishedAt = nil
			translations[i].LiveSlug = nil
			translations[i].HasUnpublishedChanges = false
			continue
		}
		publication, err := s.publicationRepo.GetArticlePublication(ctx, documentID, translations[i].Locale)
		if err != nil {
			return nil, err
		}
		if publication != nil {
			translations[i].LivePublishedAt = &publication.PublishedAt
			translations[i].LiveSlug = &publication.Slug
			translations[i].HasUnpublishedChanges = translationHasUnpublishedChanges(&translations[i], publication)
		}
	}
	return translations, nil
}

func (s *DocsHelpcenterTranslationService) ListSpaceTranslations(ctx context.Context, spaceID string) ([]model.DocsHelpcenterSpaceTranslation, error) {
	return s.translationRepo.ListSpaceTranslations(ctx, spaceID)
}

func (s *DocsHelpcenterTranslationService) ListCollectionTranslations(ctx context.Context, collectionID string) ([]model.DocsHelpcenterCollectionTranslation, error) {
	return s.translationRepo.ListCollectionTranslations(ctx, collectionID)
}

func (s *DocsHelpcenterTranslationService) UpsertSpaceTranslation(ctx context.Context, spaceID string, req model.UpsertDocsHelpcenterSpaceTranslationRequest) (*model.DocsHelpcenterSpaceTranslation, error) {
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}
	if space.Type != model.SpaceTypeExternalCapable {
		return nil, fmt.Errorf("space is not external-capable")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, space.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}

	locale, err := validateEditableLocale(cfg, req.Locale)
	if err != nil {
		return nil, err
	}

	existing, err := s.translationRepo.GetSpaceTranslation(ctx, spaceID, locale)
	if err != nil {
		return nil, err
	}

	status := model.DocsHelpcenterTranslationStatusDraft
	if existing != nil {
		status = existing.Status
	}
	if req.Status != "" && req.Status != model.DocsHelpcenterTranslationStatusPublished {
		status = req.Status
	}

	// Translation slugs are frozen after first set. Once a row has a
	// non-nil slug, this upsert path ignores any slug field in the
	// request and preserves the stored value. The public help center
	// routes by slug, and docs_redirects does not track translation
	// slug changes today — mutating the stored slug would silently
	// break localized URLs for every article in the collection. On
	// first write the slug is derived from the explicit request slug
	// or from the name as a fallback. See 2026-04-11 Option C decision.
	var existingSlug *string
	if existing != nil {
		existingSlug = existing.Slug
	}
	storedSlug := freezeOrDeriveTranslationSlug(existingSlug, req.Slug, req.Name)

	translation := &model.DocsHelpcenterSpaceTranslation{
		SpaceID:         space.ID,
		WorkspaceID:     space.WorkspaceID,
		Locale:          locale,
		Name:            req.Name,
		Slug:            storedSlug,
		Description:     req.Description,
		Status:          status,
		SourceUpdatedAt: &space.UpdatedAt,
		SourceSynced:    true,
	}
	if existing != nil {
		translation.PublishedAt = existing.PublishedAt
	}

	return s.translationRepo.UpsertSpaceTranslation(ctx, translation)
}

func (s *DocsHelpcenterTranslationService) UpsertCollectionTranslation(ctx context.Context, collectionID string, req model.UpsertDocsHelpcenterCollectionTranslationRequest) (*model.DocsHelpcenterCollectionTranslation, error) {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, fmt.Errorf("collection not found")
	}

	space, err := s.spaceRepo.GetByID(ctx, collection.SpaceID)
	if err != nil {
		return nil, err
	}
	if space == nil || space.Type != model.SpaceTypeExternalCapable {
		return nil, fmt.Errorf("collection is not in an external-capable space")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, collection.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}

	locale, err := validateEditableLocale(cfg, req.Locale)
	if err != nil {
		return nil, err
	}

	existing, err := s.translationRepo.GetCollectionTranslation(ctx, collectionID, locale)
	if err != nil {
		return nil, err
	}

	status := model.DocsHelpcenterTranslationStatusDraft
	if existing != nil {
		status = existing.Status
	}
	if req.Status != "" && req.Status != model.DocsHelpcenterTranslationStatusPublished {
		status = req.Status
	}

	// See the UpsertSpaceTranslation comment: translation slugs are
	// frozen after first set. On first write we derive the slug from
	// the explicit request slug or from the name as a fallback; every
	// subsequent update leaves the stored slug untouched.
	var existingSlug *string
	if existing != nil {
		existingSlug = existing.Slug
	}
	storedSlug := freezeOrDeriveTranslationSlug(existingSlug, req.Slug, req.Name)

	translation := &model.DocsHelpcenterCollectionTranslation{
		CollectionID:    collection.ID,
		WorkspaceID:     collection.WorkspaceID,
		SpaceID:         collection.SpaceID,
		Locale:          locale,
		Name:            req.Name,
		Description:     req.Description,
		Slug:            storedSlug,
		Status:          status,
		SourceUpdatedAt: &collection.UpdatedAt,
		SourceSynced:    true,
	}
	if existing != nil {
		translation.PublishedAt = existing.PublishedAt
	}

	return s.translationRepo.UpsertCollectionTranslation(ctx, translation)
}

func (s *DocsHelpcenterTranslationService) UpsertArticleTranslation(ctx context.Context, documentID string, req model.UpsertDocsHelpcenterArticleTranslationRequest) (*model.DocsHelpcenterArticleTranslation, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}

	locale := strings.TrimSpace(strings.ToLower(req.Locale))
	if locale == "" {
		return nil, fmt.Errorf("locale is required")
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}
	if locale == defaultLocale {
		return nil, fmt.Errorf("default locale mirrors are managed from the source document")
	}

	if !localeEnabled(cfg.EnabledLocales, locale) {
		return nil, fmt.Errorf("locale %s is not enabled for this help center", locale)
	}

	existing, err := s.translationRepo.GetArticleTranslation(ctx, documentID, locale)
	if err != nil {
		return nil, err
	}

	status := model.DocsHelpcenterTranslationStatusDraft
	if existing != nil {
		status = existing.Status
	}
	if req.Status != "" && req.Status != model.DocsHelpcenterTranslationStatusPublished {
		status = req.Status
	}

	translation := &model.DocsHelpcenterArticleTranslation{
		DocumentID:      documentID,
		WorkspaceID:     doc.WorkspaceID,
		SpaceID:         doc.SpaceID,
		CollectionID:    doc.CollectionID,
		Locale:          locale,
		Title:           req.Title,
		Slug:            normalizeSlugPointer(req.Slug),
		Excerpt:         req.Excerpt,
		Content:         req.Content,
		SEOTitle:        req.SEOTitle,
		SEODescription:  req.SEODescription,
		Status:          status,
		SourceUpdatedAt: &doc.UpdatedAt,
		SourceSynced:    true,
	}
	if existing != nil {
		translation.PublishedAt = existing.PublishedAt
		translation.ViewCount = existing.ViewCount
		translation.HelpfulCount = existing.HelpfulCount
		translation.NotHelpfulCount = existing.NotHelpfulCount
		if req.Slug == nil {
			translation.Slug = existing.Slug
		}
	}

	updated, err := s.translationRepo.UpsertArticleTranslation(ctx, translation)
	if err != nil {
		return nil, err
	}
	if updated == nil || updated.Status != model.DocsHelpcenterTranslationStatusPublished || updated.PublishedAt == nil {
		return updated, nil
	}
	publication, err := s.publicationRepo.GetArticlePublication(ctx, documentID, locale)
	if err != nil {
		return nil, err
	}
	if publication != nil {
		updated.LivePublishedAt = &publication.PublishedAt
		updated.LiveSlug = &publication.Slug
		updated.HasUnpublishedChanges = translationHasUnpublishedChanges(updated, publication)
	}
	return updated, nil
}

func (s *DocsHelpcenterTranslationService) GenerateArticleTranslationDraft(ctx context.Context, documentID, locale string) (*model.DocsHelpcenterArticleTranslation, error) {
	if s.llmProvider == nil {
		return nil, fmt.Errorf("AI translation generation is unavailable")
	}

	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}

	locale, err = validateEditableLocale(cfg, locale)
	if err != nil {
		return nil, err
	}

	content, err := s.contentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("document content not found")
	}

	article, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return nil, err
	}

	var sourceNode tiptap.Node
	if err := json.Unmarshal(content.Content, &sourceNode); err != nil {
		return nil, fmt.Errorf("parse source tiptap content: %w", err)
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	plan, err := docsi18n.PrepareArticleTranslationPlan(docsi18n.ArticleSource{
		SourceLocale:   defaultLocale,
		TargetLocale:   locale,
		Title:          doc.Title,
		Excerpt:        doc.Excerpt,
		SEOTitle:       articleSEOTitle(article),
		SEODescription: articleSEODescription(article),
		Content:        sourceNode,
	}, docsi18n.PrepareOptions{
		ProtectedTerms: []string(cfg.ProtectedTerms),
	})
	if err != nil {
		return nil, fmt.Errorf("prepare article translation plan: %w", err)
	}

	sourceJSON, err := json.Marshal(plan.Request)
	if err != nil {
		return nil, fmt.Errorf("marshal translation segment payload: %w", err)
	}

	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: `You translate structured help-center article segments for a requested locale.
Return strict JSON with shape {"segments":[{"id":"...","translated_text":"..."}]}.
Rules:
- Preserve every segment id exactly.
- Translate only the text provided for each segment.
- Do not add or remove segments.
- Preserve placeholders like __TERM_001__ exactly.
- Keep URLs, code-like tokens, and technical placeholders untouched.
- Do not translate proper nouns, brand names, product names, or widely recognized technical terms (e.g., API, webhook, OAuth, SDK, JSON, REST, URL, HTTP).
- Do not include markdown fences or commentary.`,
		Messages: []llm.Message{
			{Role: "user", Content: string(sourceJSON)},
		},
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("generate article translation draft: %w", err)
	}

	var generated docsi18n.TranslationResponse
	if err := llm.UnmarshalResponse(resp.Content, &generated); err != nil {
		return nil, fmt.Errorf("parse generated translation draft: %w", err)
	}

	draft, err := plan.Apply(generated)
	if err != nil {
		return nil, fmt.Errorf("apply structured translation draft: %w", err)
	}

	title := strings.TrimSpace(draft.Title)
	if title == "" {
		return nil, fmt.Errorf("generated translation draft is missing a title")
	}

	contentJSON, err := json.Marshal(draft.Content)
	if err != nil {
		return nil, fmt.Errorf("marshal translated tiptap content: %w", err)
	}
	req := model.UpsertDocsHelpcenterArticleTranslationRequest{
		Locale:         locale,
		Title:          title,
		Excerpt:        draft.Excerpt,
		Content:        json.RawMessage(contentJSON),
		SEOTitle:       draft.SEOTitle,
		SEODescription: draft.SEODescription,
		Status:         model.DocsHelpcenterTranslationStatusDraft,
	}
	return s.UpsertArticleTranslation(ctx, documentID, req)
}

func (s *DocsHelpcenterTranslationService) UnpublishArticleTranslation(ctx context.Context, documentID, locale string) (*model.DocsHelpcenterArticleTranslation, error) {
	if err := s.translationRepo.SetArticleTranslationStatus(ctx, documentID, locale, model.DocsHelpcenterTranslationStatusDraft, nil); err != nil {
		return nil, err
	}
	updated, err := s.translationRepo.GetArticleTranslation(ctx, documentID, locale)
	if err != nil {
		return nil, err
	}
	if updated != nil {
		updated.LivePublishedAt = nil
		updated.LiveSlug = nil
		updated.HasUnpublishedChanges = false
	}
	return updated, nil
}

func (s *DocsHelpcenterTranslationService) PublishSpaceTranslation(ctx context.Context, spaceID, locale string, requestedSlug *string) (*model.DocsHelpcenterSpaceTranslation, error) {
	translation, err := s.translationRepo.GetSpaceTranslation(ctx, spaceID, locale)
	if err != nil {
		return nil, err
	}
	if translation == nil {
		return nil, fmt.Errorf("space translation not found")
	}

	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}

	requestedSlug = normalizeSlugPointer(requestedSlug)
	if translation.Slug == nil || strings.TrimSpace(*translation.Slug) == "" || requestedSlug != nil {
		slug, err := s.ensureUniqueSpaceTranslationSlug(ctx, space.WorkspaceID, locale, spaceID, normalizedSlugOrFallback(stringPtrValue(requestedSlug), space.Slug, translation.Name, locale))
		if err != nil {
			return nil, err
		}
		if err := s.translationRepo.SetSpaceTranslationSlug(ctx, spaceID, locale, &slug); err != nil {
			return nil, err
		}
		translation.Slug = &slug
	}

	now := time.Now().UTC()
	if err := s.translationRepo.SetSpaceTranslationStatus(ctx, spaceID, locale, model.DocsHelpcenterTranslationStatusPublished, &now); err != nil {
		return nil, err
	}
	return s.translationRepo.GetSpaceTranslation(ctx, spaceID, locale)
}

func (s *DocsHelpcenterTranslationService) UnpublishSpaceTranslation(ctx context.Context, spaceID, locale string) (*model.DocsHelpcenterSpaceTranslation, error) {
	if err := s.translationRepo.SetSpaceTranslationStatus(ctx, spaceID, locale, model.DocsHelpcenterTranslationStatusDraft, nil); err != nil {
		return nil, err
	}
	return s.translationRepo.GetSpaceTranslation(ctx, spaceID, locale)
}

func (s *DocsHelpcenterTranslationService) MarkSpaceTranslationReviewed(ctx context.Context, spaceID, locale string) (*model.DocsHelpcenterSpaceTranslation, error) {
	translation, err := s.translationRepo.GetSpaceTranslation(ctx, spaceID, locale)
	if err != nil {
		return nil, err
	}
	if translation == nil {
		return nil, fmt.Errorf("space translation not found")
	}

	status := model.DocsHelpcenterTranslationStatusDraft
	if translation.PublishedAt != nil {
		status = model.DocsHelpcenterTranslationStatusPublished
	}
	return s.translationRepo.UpsertSpaceTranslation(ctx, &model.DocsHelpcenterSpaceTranslation{
		ID:              translation.ID,
		SpaceID:         translation.SpaceID,
		WorkspaceID:     translation.WorkspaceID,
		Locale:          translation.Locale,
		Name:            translation.Name,
		Slug:            normalizeSlugPointer(translation.Slug),
		Description:     translation.Description,
		Status:          status,
		SourceUpdatedAt: translation.SourceUpdatedAt,
		SourceSynced:    true,
		PublishedAt:     translation.PublishedAt,
	})
}

func (s *DocsHelpcenterTranslationService) PublishCollectionTranslation(ctx context.Context, collectionID, locale string, requestedSlug *string) (*model.DocsHelpcenterCollectionTranslation, error) {
	translation, err := s.translationRepo.GetCollectionTranslation(ctx, collectionID, locale)
	if err != nil {
		return nil, err
	}
	if translation == nil {
		return nil, fmt.Errorf("collection translation not found")
	}

	spaceTranslation, err := s.translationRepo.GetSpaceTranslation(ctx, translation.SpaceID, locale)
	if err != nil {
		return nil, err
	}
	if spaceTranslation == nil || spaceTranslation.Status != model.DocsHelpcenterTranslationStatusPublished {
		return nil, fmt.Errorf("published space translation is required before publishing this collection translation")
	}

	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, fmt.Errorf("collection not found")
	}

	requestedSlug = normalizeSlugPointer(requestedSlug)
	if translation.Slug == nil || strings.TrimSpace(*translation.Slug) == "" || requestedSlug != nil {
		slug, err := s.ensureUniqueCollectionTranslationSlug(ctx, translation.SpaceID, locale, collectionID, normalizedSlugOrFallback(stringPtrValue(requestedSlug), collection.Slug, translation.Name, locale))
		if err != nil {
			return nil, err
		}
		if err := s.translationRepo.SetCollectionTranslationSlug(ctx, collectionID, locale, &slug); err != nil {
			return nil, err
		}
		translation.Slug = &slug
	}

	now := time.Now().UTC()
	if err := s.translationRepo.SetCollectionTranslationStatus(ctx, collectionID, locale, model.DocsHelpcenterTranslationStatusPublished, &now); err != nil {
		return nil, err
	}
	return s.translationRepo.GetCollectionTranslation(ctx, collectionID, locale)
}

func (s *DocsHelpcenterTranslationService) UnpublishCollectionTranslation(ctx context.Context, collectionID, locale string) (*model.DocsHelpcenterCollectionTranslation, error) {
	if err := s.translationRepo.SetCollectionTranslationStatus(ctx, collectionID, locale, model.DocsHelpcenterTranslationStatusDraft, nil); err != nil {
		return nil, err
	}
	return s.translationRepo.GetCollectionTranslation(ctx, collectionID, locale)
}

func (s *DocsHelpcenterTranslationService) MarkCollectionTranslationReviewed(ctx context.Context, collectionID, locale string) (*model.DocsHelpcenterCollectionTranslation, error) {
	translation, err := s.translationRepo.GetCollectionTranslation(ctx, collectionID, locale)
	if err != nil {
		return nil, err
	}
	if translation == nil {
		return nil, fmt.Errorf("collection translation not found")
	}

	status := model.DocsHelpcenterTranslationStatusDraft
	if translation.PublishedAt != nil {
		status = model.DocsHelpcenterTranslationStatusPublished
	}
	return s.translationRepo.UpsertCollectionTranslation(ctx, &model.DocsHelpcenterCollectionTranslation{
		ID:              translation.ID,
		CollectionID:    translation.CollectionID,
		WorkspaceID:     translation.WorkspaceID,
		SpaceID:         translation.SpaceID,
		Locale:          translation.Locale,
		Name:            translation.Name,
		Description:     translation.Description,
		Slug:            normalizeSlugPointer(translation.Slug),
		Status:          status,
		SourceUpdatedAt: translation.SourceUpdatedAt,
		SourceSynced:    true,
		PublishedAt:     translation.PublishedAt,
	})
}

func (s *DocsHelpcenterTranslationService) MarkArticleTranslationReviewed(ctx context.Context, documentID, locale string) (*model.DocsHelpcenterArticleTranslation, error) {
	translation, err := s.translationRepo.GetArticleTranslation(ctx, documentID, locale)
	if err != nil {
		return nil, err
	}
	if translation == nil {
		return nil, fmt.Errorf("article translation not found")
	}

	status := model.DocsHelpcenterTranslationStatusDraft
	if translation.PublishedAt != nil {
		status = model.DocsHelpcenterTranslationStatusPublished
	}
	updated, err := s.translationRepo.UpsertArticleTranslation(ctx, &model.DocsHelpcenterArticleTranslation{
		ID:              translation.ID,
		DocumentID:      translation.DocumentID,
		WorkspaceID:     translation.WorkspaceID,
		SpaceID:         translation.SpaceID,
		CollectionID:    translation.CollectionID,
		Locale:          translation.Locale,
		Title:           translation.Title,
		Slug:            normalizeSlugPointer(translation.Slug),
		Excerpt:         translation.Excerpt,
		Content:         translation.Content,
		ContentText:     translation.ContentText,
		SEOTitle:        translation.SEOTitle,
		SEODescription:  translation.SEODescription,
		Status:          status,
		SourceUpdatedAt: translation.SourceUpdatedAt,
		SourceSynced:    true,
		PublishedAt:     translation.PublishedAt,
		ViewCount:       translation.ViewCount,
		HelpfulCount:    translation.HelpfulCount,
		NotHelpfulCount: translation.NotHelpfulCount,
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *DocsHelpcenterTranslationService) SyncDefaultLocaleArticleMirror(ctx context.Context, documentID string) (*model.DocsHelpcenterArticleTranslation, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}

	article, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, fmt.Errorf("help center article not found")
	}

	content, err := s.contentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		return nil, err
	}

	status := model.DocsHelpcenterTranslationStatusDraft
	if article.PublicPublishedAt != nil {
		status = model.DocsHelpcenterTranslationStatusPublished
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	translation := &model.DocsHelpcenterArticleTranslation{
		DocumentID:      doc.ID,
		WorkspaceID:     doc.WorkspaceID,
		SpaceID:         doc.SpaceID,
		CollectionID:    doc.CollectionID,
		Locale:          defaultLocale,
		Title:           doc.Title,
		Slug:            stringPointerOrNil(article.Slug),
		Excerpt:         doc.Excerpt,
		SEOTitle:        article.SEOTitle,
		SEODescription:  article.SEODescription,
		Status:          status,
		SourceUpdatedAt: &doc.UpdatedAt,
		SourceSynced:    true,
		PublishedAt:     article.PublicPublishedAt,
		ViewCount:       article.ViewCount,
		HelpfulCount:    article.HelpfulCount,
		NotHelpfulCount: article.NotHelpfulCount,
	}
	if content != nil {
		translation.Content = content.Content
		translation.ContentText = content.ContentText
	}

	return s.translationRepo.UpsertArticleTranslation(ctx, translation)
}

func (s *DocsHelpcenterTranslationService) RefreshArticleSource(ctx context.Context, documentID string) error {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return nil
	}

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}

	article, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return err
	}
	if article == nil || article.Slug == "" {
		return nil
	}

	if _, err := s.SyncDefaultLocaleArticleMirror(ctx, documentID); err != nil {
		return err
	}
	return s.MarkArticleTranslationsForSourceChange(ctx, documentID)
}

func (s *DocsHelpcenterTranslationService) RefreshSpaceSource(ctx context.Context, spaceID string) error {
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return err
	}
	if space == nil || space.Type != model.SpaceTypeExternalCapable {
		return nil
	}

	cfg, err := s.hcRepo.GetConfig(ctx, space.WorkspaceID)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	_, err = s.translationRepo.UpsertSpaceTranslation(ctx, &model.DocsHelpcenterSpaceTranslation{
		SpaceID:         space.ID,
		WorkspaceID:     space.WorkspaceID,
		Locale:          defaultLocale,
		Name:            space.Name,
		Slug:            stringPointerOrNil(space.Slug),
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &space.UpdatedAt,
		SourceSynced:    true,
		PublishedAt:     &space.UpdatedAt,
	})
	return err
}

func (s *DocsHelpcenterTranslationService) RefreshCollectionSource(ctx context.Context, collectionID string) error {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return err
	}
	if collection == nil {
		return nil
	}

	space, err := s.spaceRepo.GetByID(ctx, collection.SpaceID)
	if err != nil {
		return err
	}
	if space == nil || space.Type != model.SpaceTypeExternalCapable {
		return nil
	}

	cfg, err := s.hcRepo.GetConfig(ctx, collection.WorkspaceID)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	collectionSlug, err := s.ensureCollectionSourceSlug(ctx, collection)
	if err != nil {
		return err
	}

	_, err = s.translationRepo.UpsertCollectionTranslation(ctx, &model.DocsHelpcenterCollectionTranslation{
		CollectionID:    collection.ID,
		WorkspaceID:     collection.WorkspaceID,
		SpaceID:         collection.SpaceID,
		Locale:          defaultLocale,
		Name:            collection.Name,
		Description:     collection.Description,
		Slug:            stringPointerOrNil(collectionSlug),
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &collection.UpdatedAt,
		SourceSynced:    true,
		PublishedAt:     &collection.UpdatedAt,
	})
	return err
}

func (s *DocsHelpcenterTranslationService) EnsureDefaultLocaleMirrorsForWorkspace(ctx context.Context, workspaceID string) error {
	spaces, err := s.spaceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}

	status := model.DocStatusPublished
	for _, space := range spaces {
		if space.Type != model.SpaceTypeExternalCapable {
			continue
		}

		if err := s.RefreshSpaceSource(ctx, space.ID); err != nil {
			return err
		}

		collections, err := s.collectionRepo.ListBySpace(ctx, space.ID)
		if err != nil {
			return err
		}
		for _, collection := range collections {
			if err := s.RefreshCollectionSource(ctx, collection.ID); err != nil {
				return err
			}
		}

		docs, err := s.docRepo.List(ctx, workspaceID, &space.ID, nil, &status, nil, "", false)
		if err != nil {
			return err
		}
		for _, doc := range docs {
			article, err := s.hcRepo.GetArticle(ctx, doc.ID)
			if err != nil {
				return err
			}
			if article == nil || article.Slug == "" || article.PublicPublishedAt == nil {
				continue
			}

			if err := s.RefreshArticleSource(ctx, doc.ID); err != nil {
				if isArticleTranslationSlugConflict(err) {
					slog.WarnContext(ctx, "skipping conflicting public helpcenter article mirror during workspace backfill", "workspace_id", workspaceID, "document_id", doc.ID, "error", err)
					continue
				}
				return err
			}
		}
	}

	return nil
}

func isArticleTranslationSlugConflict(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	if !strings.Contains(message, "upsert helpcenter article translation") {
		return false
	}

	return strings.Contains(message, "idx_docs_hc_article_space_locale_slug") ||
		strings.Contains(message, "docs_helpcenter_article_translations.space_id, docs_helpcenter_article_translations.locale, docs_helpcenter_article_translations.slug")
}

// GenerateSpaceTranslation uses AI to translate a space's name and description for a locale.
// The resulting translation stays draft until a human publishes it; the first publish derives the slug.
func (s *DocsHelpcenterTranslationService) GenerateSpaceTranslation(ctx context.Context, spaceID, locale string) (*model.DocsHelpcenterSpaceTranslation, error) {
	if s.llmProvider == nil {
		return nil, fmt.Errorf("AI translation generation is unavailable")
	}

	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, space.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}

	locale, err = validateEditableLocale(cfg, locale)
	if err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(map[string]string{
		"source_locale": cfg.DefaultLocale,
		"target_locale": locale,
		"entity_type":   "help_center_space",
		"name":          space.Name,
	})

	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: `You translate metadata for a public help center / knowledge base.
Return strict JSON with shape {"name":"...","description":"..."}.
Rules:
- Translate the name and description into the target locale.
- If description is empty, generate a short helpful description (1-2 sentences) based on the name, suitable for a public help center space.
- Do not translate proper nouns, brand names, product names, or widely recognized technical terms.
- Do not include markdown fences or commentary.`,
		Messages:    []llm.Message{{Role: "user", Content: string(payload)}},
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("generate space translation: %w", err)
	}

	var result struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := llm.UnmarshalResponse(resp.Content, &result); err != nil {
		return nil, fmt.Errorf("parse generated space translation: %w", err)
	}

	desc := strings.TrimSpace(result.Description)
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}

	translation := &model.DocsHelpcenterSpaceTranslation{
		SpaceID:         space.ID,
		WorkspaceID:     space.WorkspaceID,
		Locale:          locale,
		Name:            strings.TrimSpace(result.Name),
		Slug:            nil,
		Description:     descPtr,
		Status:          model.DocsHelpcenterTranslationStatusDraft,
		SourceUpdatedAt: &space.UpdatedAt,
		SourceSynced:    true,
	}
	return s.translationRepo.UpsertSpaceTranslation(ctx, translation)
}

// GenerateCollectionTranslation uses AI to translate a collection's name and description for a locale.
// The resulting translation stays draft until a human publishes it; the first publish derives the slug.
func (s *DocsHelpcenterTranslationService) GenerateCollectionTranslation(ctx context.Context, collectionID, locale string) (*model.DocsHelpcenterCollectionTranslation, error) {
	if s.llmProvider == nil {
		return nil, fmt.Errorf("AI translation generation is unavailable")
	}

	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, fmt.Errorf("collection not found")
	}

	space, err := s.spaceRepo.GetByID(ctx, collection.SpaceID)
	if err != nil {
		return nil, err
	}
	if space == nil || space.Type != model.SpaceTypeExternalCapable {
		return nil, fmt.Errorf("collection is not in an external-capable space")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, space.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}

	locale, err = validateEditableLocale(cfg, locale)
	if err != nil {
		return nil, err
	}

	description := ""
	if collection.Description != nil {
		description = *collection.Description
	}

	payload, _ := json.Marshal(map[string]string{
		"source_locale": cfg.DefaultLocale,
		"target_locale": locale,
		"entity_type":   "help_center_collection",
		"name":          collection.Name,
		"description":   description,
		"space_name":    space.Name,
	})

	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: `You translate metadata for a public help center / knowledge base.
Return strict JSON with shape {"name":"...","description":"..."}.
Rules:
- Translate the name and description into the target locale.
- If description is empty, generate a short helpful description (1-2 sentences) based on the name and parent space name, suitable for a public help center collection.
- Do not translate proper nouns, brand names, product names, or widely recognized technical terms.
- Do not include markdown fences or commentary.`,
		Messages:    []llm.Message{{Role: "user", Content: string(payload)}},
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("generate collection translation: %w", err)
	}

	var result struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := llm.UnmarshalResponse(resp.Content, &result); err != nil {
		return nil, fmt.Errorf("parse generated collection translation: %w", err)
	}

	desc := strings.TrimSpace(result.Description)
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}

	translation := &model.DocsHelpcenterCollectionTranslation{
		CollectionID:    collection.ID,
		WorkspaceID:     space.WorkspaceID,
		SpaceID:         collection.SpaceID,
		Locale:          locale,
		Name:            strings.TrimSpace(result.Name),
		Slug:            nil,
		Description:     descPtr,
		Status:          model.DocsHelpcenterTranslationStatusDraft,
		SourceUpdatedAt: &collection.UpdatedAt,
		SourceSynced:    true,
	}
	return s.translationRepo.UpsertCollectionTranslation(ctx, translation)
}

// GenerateAllSpaceAndCollectionTranslations generates translations for all external spaces
// and their collections for a given locale. Called when a locale is first enabled or
// when a new external space/collection is created.
func (s *DocsHelpcenterTranslationService) GenerateAllSpaceAndCollectionTranslations(ctx context.Context, workspaceID, locale string) error {
	if s.llmProvider == nil {
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

		// Generate space translation if missing
		existing, _ := s.translationRepo.GetSpaceTranslation(ctx, space.ID, locale)
		if existing == nil {
			if _, err := s.GenerateSpaceTranslation(ctx, space.ID, locale); err != nil {
				slog.ErrorContext(ctx, "failed to generate space translation", "space_id", space.ID, "locale", locale, "error", err)
				continue
			}
		}

		// Generate collection translations
		collections, err := s.collectionRepo.ListBySpace(ctx, space.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to list collections for translation", "space_id", space.ID, "error", err)
			continue
		}
		for _, coll := range collections {
			existingColl, _ := s.translationRepo.GetCollectionTranslation(ctx, coll.ID, locale)
			if existingColl == nil {
				if _, err := s.GenerateCollectionTranslation(ctx, coll.ID, locale); err != nil {
					slog.ErrorContext(ctx, "failed to generate collection translation", "collection_id", coll.ID, "locale", locale, "error", err)
				}
			}
		}
	}
	return nil
}

// AutoGenerateSpaceTranslations generates translations for all enabled non-default locales for a space.
// Called when a new external space is created with locales enabled.
func (s *DocsHelpcenterTranslationService) AutoGenerateSpaceTranslations(ctx context.Context, spaceID string) error {
	if s.llmProvider == nil {
		return nil
	}
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil || space == nil || space.Type != model.SpaceTypeExternalCapable {
		return nil
	}
	cfg, err := s.hcRepo.GetConfig(ctx, space.WorkspaceID)
	if err != nil || cfg == nil {
		return nil
	}
	for _, locale := range cfg.EnabledLocales {
		if locale == cfg.DefaultLocale {
			continue
		}
		existing, _ := s.translationRepo.GetSpaceTranslation(ctx, spaceID, locale)
		if existing != nil {
			continue
		}
		if _, err := s.GenerateSpaceTranslation(ctx, spaceID, locale); err != nil {
			slog.ErrorContext(ctx, "auto-generate space translation failed", "space_id", spaceID, "locale", locale, "error", err)
		}
	}
	return nil
}

// AutoGenerateCollectionTranslations generates translations for all enabled non-default locales for a collection.
// Called when a new collection is created in an external space with locales enabled.
func (s *DocsHelpcenterTranslationService) AutoGenerateCollectionTranslations(ctx context.Context, collectionID string) error {
	if s.llmProvider == nil {
		return nil
	}
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil || collection == nil {
		return nil
	}
	space, err := s.spaceRepo.GetByID(ctx, collection.SpaceID)
	if err != nil || space == nil || space.Type != model.SpaceTypeExternalCapable {
		return nil
	}
	cfg, err := s.hcRepo.GetConfig(ctx, space.WorkspaceID)
	if err != nil || cfg == nil {
		return nil
	}
	for _, locale := range cfg.EnabledLocales {
		if locale == cfg.DefaultLocale {
			continue
		}
		existing, _ := s.translationRepo.GetCollectionTranslation(ctx, collectionID, locale)
		if existing != nil {
			continue
		}
		if _, err := s.GenerateCollectionTranslation(ctx, collectionID, locale); err != nil {
			slog.ErrorContext(ctx, "auto-generate collection translation failed", "collection_id", collectionID, "locale", locale, "error", err)
		}
	}
	return nil
}

func (s *DocsHelpcenterTranslationService) MarkArticleTranslationsForSourceChange(ctx context.Context, documentID string) error {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("help center config not found")
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	return s.translationRepo.MarkArticleTranslationsNeedsReview(ctx, documentID, defaultLocale, doc.UpdatedAt)
}

func (s *DocsHelpcenterTranslationService) PublishArticleTranslation(ctx context.Context, documentID, locale string, requestedSlug *string) (*model.DocsHelpcenterArticleTranslation, error) {
	translation, err := s.translationRepo.GetArticleTranslation(ctx, documentID, locale)
	if err != nil {
		return nil, err
	}
	if translation == nil {
		return nil, fmt.Errorf("article translation not found")
	}

	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}
	if locale == defaultLocale {
		return nil, fmt.Errorf("default locale mirrors are managed from the source document")
	}

	spaceTranslation, err := s.translationRepo.GetSpaceTranslation(ctx, doc.SpaceID, locale)
	if err != nil {
		return nil, err
	}
	if spaceTranslation == nil || spaceTranslation.Status != model.DocsHelpcenterTranslationStatusPublished {
		return nil, fmt.Errorf("published space translation is required before publishing this article translation")
	}

	if doc.CollectionID != nil {
		collectionTranslation, err := s.translationRepo.GetCollectionTranslation(ctx, *doc.CollectionID, locale)
		if err != nil {
			return nil, err
		}
		if collectionTranslation == nil || collectionTranslation.Status != model.DocsHelpcenterTranslationStatusPublished {
			return nil, fmt.Errorf("published collection translation is required before publishing this article translation")
		}
	}

	requestedSlug = normalizeSlugPointer(requestedSlug)
	if translation.Slug == nil || strings.TrimSpace(stringPtrValue(translation.Slug)) == "" || requestedSlug != nil {
		base := normalizedSlugOrFallback(stringPtrValue(requestedSlug), "", translation.Title, locale)
		slug, err := s.ensureUniqueArticleTranslationSlug(ctx, doc.SpaceID, locale, documentID, base)
		if err != nil {
			return nil, err
		}
		if err := s.translationRepo.SetArticleTranslationSlug(ctx, documentID, locale, &slug); err != nil {
			return nil, err
		}
		translation.Slug = &slug
	}

	livePublication, err := s.publicationRepo.GetArticlePublication(ctx, documentID, locale)
	if err != nil {
		return nil, err
	}
	publication := buildArticleTranslationPublication(translation)
	if _, err := s.publicationRepo.UpsertArticlePublication(ctx, publication); err != nil {
		return nil, err
	}

	if livePublication != nil && livePublication.Slug != publication.Slug && s.redirectRepo != nil {
		collectionSlug, err := s.localizedCollectionSlugForArticlePath(ctx, translation.CollectionID, locale)
		if err != nil {
			return nil, err
		}
		redirect := &model.DocsRedirect{
			WorkspaceID:          translation.WorkspaceID,
			SourcePath:           buildDocsRedirectPath(collectionSlug, &livePublication.Slug),
			TargetCollectionSlug: collectionSlug,
			TargetArticleSlug:    &publication.Slug,
			Type:                 model.RedirectTypeSlugChange,
		}
		if err := s.redirectRepo.Create(ctx, redirect); err != nil {
			slog.ErrorContext(ctx, "create localized slug change redirect", "error", err, "document_id", documentID, "locale", locale)
		}
	}

	now := time.Now().UTC()
	if err := s.translationRepo.SetArticleTranslationStatus(ctx, documentID, locale, model.DocsHelpcenterTranslationStatusPublished, &now); err != nil {
		return nil, err
	}

	updated, err := s.translationRepo.GetArticleTranslation(ctx, documentID, locale)
	if err != nil {
		return nil, err
	}
	if updated != nil {
		updated.LivePublishedAt = &publication.PublishedAt
		updated.LiveSlug = &publication.Slug
		updated.HasUnpublishedChanges = false
	}
	return updated, nil
}

func (s *DocsHelpcenterTranslationService) UpdateArticleTranslationSlug(ctx context.Context, workspaceID, documentID, locale, newSlug string) error {
	translation, err := s.translationRepo.GetArticleTranslation(ctx, documentID, locale)
	if err != nil {
		return err
	}
	if translation == nil {
		return fmt.Errorf("article translation not found")
	}
	if translation.WorkspaceID != workspaceID {
		return fmt.Errorf("article translation not found")
	}
	if translation.Slug == nil || strings.TrimSpace(*translation.Slug) == "" {
		return fmt.Errorf("article translation has no published slug")
	}

	cleaned := normalizeSlug(newSlug)
	if cleaned == "" {
		return fmt.Errorf("slug is required")
	}
	if cleaned == *translation.Slug {
		return nil
	}

	slug, err := s.ensureUniqueArticleTranslationSlug(ctx, translation.SpaceID, locale, documentID, cleaned)
	if err != nil {
		return err
	}
	return s.translationRepo.SetArticleTranslationSlug(ctx, documentID, locale, &slug)
}

func (s *DocsHelpcenterTranslationService) localizedCollectionSlugForArticlePath(ctx context.Context, collectionID *string, locale string) (string, error) {
	if collectionID == nil {
		return "", nil
	}

	if locale != "" {
		translation, err := s.translationRepo.GetCollectionTranslation(ctx, *collectionID, locale)
		if err != nil {
			return "", err
		}
		if translation != nil && translation.Slug != nil {
			if slug := strings.TrimSpace(*translation.Slug); slug != "" {
				return slug, nil
			}
		}
	}

	collection, err := s.collectionRepo.GetByID(ctx, *collectionID)
	if err != nil {
		return "", err
	}
	if collection == nil {
		return "", fmt.Errorf("collection not found")
	}
	return strings.TrimSpace(collection.Slug), nil
}

// GetLocalizedCollectionBreadcrumb returns the ancestor chain of a
// collection as localized breadcrumb entries ordered from the topmost
// ancestor down to the collection itself (inclusive). Each entry uses
// the requested locale's translated name and slug when available,
// falling back to the canonical source row otherwise.
//
// This is the public helper the help-center and widget routes call to
// render "Root > Parent > Current" paths without touching the repo
// directly.
func (s *DocsHelpcenterTranslationService) GetLocalizedCollectionBreadcrumb(ctx context.Context, collectionID, locale string) ([]model.PublicNavBreadcrumbEntry, error) {
	if collectionID == "" {
		return nil, nil
	}

	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, nil
	}

	ancestors, err := s.collectionRepo.ListAncestors(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	// ListAncestors returns the chain from nearest parent to root. Reverse
	// it so the breadcrumb reads top-down, then append the collection
	// itself as the final entry.
	chain := make([]model.DocsCollection, 0, len(ancestors)+1)
	for i := len(ancestors) - 1; i >= 0; i-- {
		chain = append(chain, ancestors[i])
	}
	chain = append(chain, *collection)

	result := make([]model.PublicNavBreadcrumbEntry, 0, len(chain))
	for i := range chain {
		c := chain[i]
		entry := model.PublicNavBreadcrumbEntry{
			ID:   c.ID,
			Name: c.Name,
			Slug: c.Slug,
		}
		if locale != "" {
			translation, err := s.translationRepo.GetCollectionTranslation(ctx, c.ID, locale)
			if err != nil {
				return nil, err
			}
			if translation != nil {
				if name := strings.TrimSpace(translation.Name); name != "" {
					entry.Name = name
				}
				if translation.Slug != nil {
					if slug := strings.TrimSpace(*translation.Slug); slug != "" {
						entry.Slug = slug
					}
				}
			}
		}
		if entry.Slug == "" {
			entry.Slug = c.ID
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *DocsHelpcenterTranslationService) ResolveArticleTranslation(ctx context.Context, documentID, requestedLocale string) (*model.DocsHelpcenterArticleTranslation, string, bool, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, "", false, err
	}
	if doc == nil {
		return nil, "", false, fmt.Errorf("document not found")
	}

	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return nil, "", false, err
	}
	if cfg == nil {
		return nil, "", false, fmt.Errorf("help center config not found")
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	translation, err := s.translationRepo.GetArticleTranslation(ctx, documentID, requestedLocale)
	if err != nil {
		return nil, "", false, err
	}
	if translation != nil && translation.Status == model.DocsHelpcenterTranslationStatusPublished {
		return translation, requestedLocale, false, nil
	}

	if !cfg.FallbackToDefaultLocale || requestedLocale == defaultLocale {
		return nil, "", false, fmt.Errorf("article translation not available")
	}

	fallback, err := s.translationRepo.GetArticleTranslation(ctx, documentID, defaultLocale)
	if err != nil {
		return nil, "", false, err
	}
	if fallback == nil || fallback.Status != model.DocsHelpcenterTranslationStatusPublished {
		return nil, "", false, fmt.Errorf("article translation not available")
	}

	return fallback, defaultLocale, true, nil
}

func localeEnabled(enabled model.DocsStringArray, locale string) bool {
	for _, candidate := range enabled {
		if candidate == locale {
			return true
		}
	}
	return false
}

func buildArticleTranslationPublication(translation *model.DocsHelpcenterArticleTranslation) *model.DocsHelpcenterArticlePublication {
	publishedAt := time.Now().UTC()
	slug := strings.TrimSpace(stringPtrValue(translation.Slug))
	return &model.DocsHelpcenterArticlePublication{
		DocumentID:     translation.DocumentID,
		WorkspaceID:    translation.WorkspaceID,
		SpaceID:        translation.SpaceID,
		CollectionID:   translation.CollectionID,
		Locale:         translation.Locale,
		Title:          strings.TrimSpace(translation.Title),
		Slug:           slug,
		Excerpt:        translation.Excerpt,
		Content:        translation.Content,
		ContentText:    translation.ContentText,
		SEOTitle:       translation.SEOTitle,
		SEODescription: translation.SEODescription,
		PublishedAt:    publishedAt,
	}
}

func translationHasUnpublishedChanges(translation *model.DocsHelpcenterArticleTranslation, publication *model.DocsHelpcenterArticlePublication) bool {
	if publication == nil {
		return false
	}
	if strings.TrimSpace(translation.Title) != strings.TrimSpace(publication.Title) {
		return true
	}
	if stringPtrValue(translation.Slug) != publication.Slug {
		return true
	}
	if strings.TrimSpace(stringPtrValue(translation.Excerpt)) != strings.TrimSpace(stringPtrValue(publication.Excerpt)) {
		return true
	}
	if strings.TrimSpace(stringPtrValue(translation.SEOTitle)) != strings.TrimSpace(stringPtrValue(publication.SEOTitle)) {
		return true
	}
	if strings.TrimSpace(stringPtrValue(translation.SEODescription)) != strings.TrimSpace(stringPtrValue(publication.SEODescription)) {
		return true
	}
	return !bytes.Equal(compactJSON(translation.Content), compactJSON(publication.Content))
}

func validateEditableLocale(cfg *model.DocsHelpcenterConfig, locale string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(locale))
	if normalized == "" {
		return "", fmt.Errorf("locale is required")
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}
	if normalized == defaultLocale {
		return "", fmt.Errorf("default locale mirrors are managed from the source document")
	}
	if !localeEnabled(cfg.EnabledLocales, normalized) {
		return "", fmt.Errorf("locale %s is not enabled for this help center", normalized)
	}
	return normalized, nil
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9-]+`)

func (s *DocsHelpcenterTranslationService) ensureUniqueSpaceTranslationSlug(ctx context.Context, workspaceID, locale, excludeSpaceID, base string) (string, error) {
	return ensureUniqueSlug(base, func(candidate string) (bool, error) {
		return s.translationRepo.SpaceTranslationSlugExists(ctx, workspaceID, locale, candidate, excludeSpaceID)
	})
}

func (s *DocsHelpcenterTranslationService) ensureUniqueCollectionTranslationSlug(ctx context.Context, spaceID, locale, excludeCollectionID, base string) (string, error) {
	return ensureUniqueSlug(base, func(candidate string) (bool, error) {
		return s.translationRepo.CollectionTranslationSlugExists(ctx, spaceID, locale, candidate, excludeCollectionID)
	})
}

func (s *DocsHelpcenterTranslationService) ensureUniqueArticleTranslationSlug(ctx context.Context, spaceID, locale, excludeDocumentID, base string) (string, error) {
	return ensureUniqueSlug(base, func(candidate string) (bool, error) {
		return s.translationRepo.ArticleTranslationSlugExists(ctx, spaceID, locale, candidate, excludeDocumentID)
	})
}

func ensureUniqueSlug(base string, exists func(candidate string) (bool, error)) (string, error) {
	if base == "" {
		base = "article"
	}
	candidate := base
	for i := 2; ; i++ {
		taken, err := exists(candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

func normalizedSlugOrFallback(candidate, sourceSlug, title, locale string) string {
	if normalized := normalizeSlug(candidate); normalized != "" {
		return normalized
	}
	if normalized := normalizeSlug(title); normalized != "" {
		return normalized
	}
	base := normalizeSlug(sourceSlug)
	if base == "" {
		base = "article"
	}
	return fmt.Sprintf("%s-%s", base, normalizeSlug(locale))
}

// freezeOrDeriveTranslationSlug implements the "set once, then frozen"
// rule for help-center translation slugs. A stored non-nil slug is
// returned as-is — the caller cannot overwrite it. When no slug has
// been set yet, the caller-provided slug is preferred, falling back
// to a slug derived from the translation name so first-time inserts
// never produce a nil URL segment. Returns nil only when neither
// the existing row, the request, nor the name can yield a usable
// slug (e.g. all empty strings).
func freezeOrDeriveTranslationSlug(existing *string, requested *string, name string) *string {
	if existing != nil {
		return existing
	}
	if requested != nil {
		if normalized := normalizeSlugPointer(requested); normalized != nil {
			return normalized
		}
	}
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		if derived := normalizeSlug(trimmed); derived != "" {
			return &derived
		}
	}
	return nil
}

func normalizeSlugPointer(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := normalizeSlug(*value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func stringPointerOrNil(value string) *string {
	normalized := normalizeSlug(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func (s *DocsHelpcenterTranslationService) ensureCollectionSourceSlug(ctx context.Context, collection *model.DocsCollection) (string, error) {
	if collection == nil {
		return "", fmt.Errorf("collection not found")
	}

	existing := normalizeSlug(collection.Slug)
	if existing != "" {
		if collection.Slug != existing {
			updated, err := s.collectionRepo.Update(ctx, collection.ID, map[string]interface{}{"slug": existing})
			if err != nil {
				return "", fmt.Errorf("normalize collection slug: %w", err)
			}
			if updated != nil {
				collection.Slug = updated.Slug
			} else {
				collection.Slug = existing
			}
		}
		return existing, nil
	}

	base := normalizeSlug(collection.Name)
	if base == "" {
		base = "collection"
	}

	collections, err := s.collectionRepo.ListByWorkspace(ctx, collection.WorkspaceID)
	if err != nil {
		return "", fmt.Errorf("list workspace collections for slug backfill: %w", err)
	}

	taken := make(map[string]struct{}, len(collections))
	for _, candidate := range collections {
		if candidate.ID == collection.ID {
			continue
		}
		slug := normalizeSlug(candidate.Slug)
		if slug == "" {
			continue
		}
		taken[slug] = struct{}{}
	}

	slug := base
	for i := 2; ; i++ {
		if _, exists := taken[slug]; !exists {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}

	updated, err := s.collectionRepo.Update(ctx, collection.ID, map[string]interface{}{"slug": slug})
	if err != nil {
		return "", fmt.Errorf("backfill collection slug: %w", err)
	}
	if updated != nil {
		collection.Slug = updated.Slug
	} else {
		collection.Slug = slug
	}
	return slug, nil
}

func normalizeSlug(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "_", "-")
	trimmed = nonSlugChars.ReplaceAllString(trimmed, "-")
	trimmed = strings.Trim(trimmed, "-")
	for strings.Contains(trimmed, "--") {
		trimmed = strings.ReplaceAll(trimmed, "--", "-")
	}
	return trimmed
}

func trimmedStringPointer(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func articleSlug(article *model.DocsHelpcenterArticle) string {
	if article == nil {
		return ""
	}
	return article.Slug
}

func articleSEOTitle(article *model.DocsHelpcenterArticle) *string {
	if article == nil {
		return nil
	}
	return article.SEOTitle
}

func articleSEODescription(article *model.DocsHelpcenterArticle) *string {
	if article == nil {
		return nil
	}
	return article.SEODescription
}
