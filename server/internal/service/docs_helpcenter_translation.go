package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsHelpcenterTranslationService handles translation-specific business rules.
type DocsHelpcenterTranslationService struct {
	translationRepo *repository.DocsHelpcenterTranslationRepository
	hcRepo          *repository.DocsHelpcenterRepository
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
	docRepo *repository.DocsDocumentRepository,
	contentRepo *repository.DocsContentRepository,
	spaceRepo *repository.DocsSpaceRepository,
	collectionRepo *repository.DocsCollectionRepository,
	llmProvider llm.Provider,
) *DocsHelpcenterTranslationService {
	return &DocsHelpcenterTranslationService{
		translationRepo: translationRepo,
		hcRepo:          hcRepo,
		docRepo:         docRepo,
		contentRepo:     contentRepo,
		spaceRepo:       spaceRepo,
		collectionRepo:  collectionRepo,
		llmProvider:     llmProvider,
	}
}

type generatedArticleTranslationDraft struct {
	Title          string `json:"title"`
	Slug           string `json:"slug"`
	Excerpt        string `json:"excerpt"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
	Body           string `json:"body"`
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

	return s.hcRepo.UpsertConfig(ctx, workspaceID, map[string]interface{}{
		"default_locale":             defaultLocale,
		"enabled_locales":            enabled,
		"show_language_switcher":     req.ShowLanguageSwitcher,
		"fallback_to_default_locale": req.FallbackToDefaultLocale,
	})
}

func (s *DocsHelpcenterTranslationService) ListArticleTranslations(ctx context.Context, documentID string) ([]model.DocsHelpcenterArticleTranslation, error) {
	return s.translationRepo.ListArticleTranslations(ctx, documentID)
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

	translation := &model.DocsHelpcenterSpaceTranslation{
		SpaceID:         space.ID,
		WorkspaceID:     space.WorkspaceID,
		Locale:          locale,
		Name:            req.Name,
		Slug:            req.Slug,
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

	translation := &model.DocsHelpcenterCollectionTranslation{
		CollectionID:    collection.ID,
		WorkspaceID:     collection.WorkspaceID,
		SpaceID:         collection.SpaceID,
		Locale:          locale,
		Name:            req.Name,
		Description:     req.Description,
		Slug:            req.Slug,
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
		Slug:            req.Slug,
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
	}

	return s.translationRepo.UpsertArticleTranslation(ctx, translation)
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

	article, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return nil, err
	}

	sourcePayload := map[string]any{
		"locale":           locale,
		"source_title":     doc.Title,
		"source_excerpt":   strings.TrimSpace(stringPtrValue(doc.Excerpt)),
		"source_slug":      strings.TrimSpace(articleSlug(article)),
		"source_seo_title": strings.TrimSpace(stringPtrValue(articleSEOTitle(article))),
		"source_seo_desc":  strings.TrimSpace(stringPtrValue(articleSEODescription(article))),
		"source_body":      strings.TrimSpace(contentText(content)),
	}
	sourceJSON, err := json.Marshal(sourcePayload)
	if err != nil {
		return nil, fmt.Errorf("marshal translation source payload: %w", err)
	}

	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: `You generate high-quality help center article drafts for a requested locale.
Return strict JSON with keys: title, slug, excerpt, seo_title, seo_description, body.
Rules:
- Translate the article naturally for the requested locale.
- Keep product names like Helpin unchanged.
- body must be plain text with paragraphs separated by blank lines.
- slug must be URL-safe ASCII; if you cannot produce one, return an empty string.
- Do not include markdown fences or extra commentary.`,
		Messages: []llm.Message{
			{Role: "user", Content: string(sourceJSON)},
		},
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("generate article translation draft: %w", err)
	}

	var generated generatedArticleTranslationDraft
	if err := llm.UnmarshalResponse(resp.Content, &generated); err != nil {
		return nil, fmt.Errorf("parse generated translation draft: %w", err)
	}

	title := strings.TrimSpace(generated.Title)
	if title == "" {
		return nil, fmt.Errorf("generated translation draft is missing a title")
	}

	body := strings.TrimSpace(generated.Body)
	slug := normalizedSlugOrFallback(generated.Slug, articleSlug(article), title, locale)

	req := model.UpsertDocsHelpcenterArticleTranslationRequest{
		Locale:         locale,
		Title:          title,
		Slug:           slug,
		Excerpt:        trimmedStringPointer(generated.Excerpt),
		Content:        plainTextToTipTapDoc(body),
		SEOTitle:       trimmedStringPointer(generated.SEOTitle),
		SEODescription: trimmedStringPointer(generated.SEODescription),
		Status:         model.DocsHelpcenterTranslationStatusDraft,
	}
	return s.UpsertArticleTranslation(ctx, documentID, req)
}

func (s *DocsHelpcenterTranslationService) UnpublishArticleTranslation(ctx context.Context, documentID, locale string) (*model.DocsHelpcenterArticleTranslation, error) {
	if err := s.translationRepo.SetArticleTranslationStatus(ctx, documentID, locale, model.DocsHelpcenterTranslationStatusDraft, nil); err != nil {
		return nil, err
	}
	return s.translationRepo.GetArticleTranslation(ctx, documentID, locale)
}

func (s *DocsHelpcenterTranslationService) PublishSpaceTranslation(ctx context.Context, spaceID, locale string) (*model.DocsHelpcenterSpaceTranslation, error) {
	translation, err := s.translationRepo.GetSpaceTranslation(ctx, spaceID, locale)
	if err != nil {
		return nil, err
	}
	if translation == nil {
		return nil, fmt.Errorf("space translation not found")
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
		Slug:            translation.Slug,
		Description:     translation.Description,
		Status:          status,
		SourceUpdatedAt: translation.SourceUpdatedAt,
		SourceSynced:    true,
		PublishedAt:     translation.PublishedAt,
	})
}

func (s *DocsHelpcenterTranslationService) PublishCollectionTranslation(ctx context.Context, collectionID, locale string) (*model.DocsHelpcenterCollectionTranslation, error) {
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
		Slug:            translation.Slug,
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
		Slug:            translation.Slug,
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
		Slug:            article.Slug,
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
		Slug:            space.Slug,
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

	_, err = s.translationRepo.UpsertCollectionTranslation(ctx, &model.DocsHelpcenterCollectionTranslation{
		CollectionID:    collection.ID,
		WorkspaceID:     collection.WorkspaceID,
		SpaceID:         collection.SpaceID,
		Locale:          defaultLocale,
		Name:            collection.Name,
		Description:     collection.Description,
		Slug:            collection.Slug,
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &collection.UpdatedAt,
		SourceSynced:    true,
		PublishedAt:     &collection.UpdatedAt,
	})
	return err
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

func (s *DocsHelpcenterTranslationService) PublishArticleTranslation(ctx context.Context, documentID, locale string) (*model.DocsHelpcenterArticleTranslation, error) {
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

	now := time.Now().UTC()
	if err := s.translationRepo.SetArticleTranslationStatus(ctx, documentID, locale, model.DocsHelpcenterTranslationStatusPublished, &now); err != nil {
		return nil, err
	}

	return s.translationRepo.GetArticleTranslation(ctx, documentID, locale)
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

func plainTextToTipTapDoc(body string) json.RawMessage {
	body = strings.TrimSpace(body)
	if body == "" {
		return json.RawMessage(`{"type":"doc","content":[]}`)
	}

	parts := strings.Split(body, "\n\n")
	content := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		paragraph := strings.TrimSpace(part)
		if paragraph == "" {
			continue
		}
		content = append(content, map[string]any{
			"type": "paragraph",
			"content": []map[string]any{
				{
					"type": "text",
					"text": paragraph,
				},
			},
		})
	}

	payload, err := json.Marshal(map[string]any{
		"type":    "doc",
		"content": content,
	})
	if err != nil {
		return json.RawMessage(`{"type":"doc","content":[]}`)
	}
	return payload
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

func contentText(content *model.DocsContent) string {
	if content == nil {
		return ""
	}
	return content.ContentText
}
