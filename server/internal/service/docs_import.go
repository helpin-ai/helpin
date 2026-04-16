package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

// DocsImportService orchestrates help center article imports.
type DocsImportService struct {
	importRepo    *repository.DocsImportRepository
	spaceSvc      *DocsSpaceService
	collectionSvc *DocsCollectionService
	documentSvc   *DocsDocumentService
	contentSvc    *DocsContentService
	helpcenterSvc *DocsHelpcenterService
	redirectRepo  *repository.DocsRedirectRepository
	s3Client      *storage.S3Client
	logger        *slog.Logger
}

// NewDocsImportService creates a new DocsImportService.
func NewDocsImportService(
	importRepo *repository.DocsImportRepository,
	spaceSvc *DocsSpaceService,
	collectionSvc *DocsCollectionService,
	documentSvc *DocsDocumentService,
	contentSvc *DocsContentService,
	helpcenterSvc *DocsHelpcenterService,
	redirectRepo *repository.DocsRedirectRepository,
	s3Client *storage.S3Client,
) *DocsImportService {
	return &DocsImportService{
		importRepo:    importRepo,
		spaceSvc:      spaceSvc,
		collectionSvc: collectionSvc,
		documentSvc:   documentSvc,
		contentSvc:    contentSvc,
		helpcenterSvc: helpcenterSvc,
		redirectRepo:  redirectRepo,
		s3Client:      s3Client,
		logger:        slog.Default().With("service", "docs_import"),
	}
}

// s3ImageUploader adapts S3Client to the helpscout.ImageUploader interface.
type s3ImageUploader struct {
	store docsImageObjectStore
}

type docsImageObjectStore interface {
	PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error
	PublicURL(key string) string
}

// UploadImage uploads an image to S3 and returns the public URL.
func (u *s3ImageUploader) UploadImage(ctx context.Context, workspaceID, filename string, data io.Reader, contentType string) (string, error) {
	payload, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("read image %q: %w", filename, err)
	}
	key := fmt.Sprintf("docs-import/%s/%s-%s", workspaceID, uuid.New().String(), filename)
	if err := u.store.PutObject(ctx, key, contentType, int64(len(payload)), bytes.NewReader(payload), true); err != nil {
		return "", fmt.Errorf("upload image %q: %w", filename, err)
	}
	return u.store.PublicURL(key), nil
}

// ImportExternalImage downloads a remote image and stores it in our S3-backed docs storage.
func (s *DocsImportService) ImportExternalImage(ctx context.Context, workspaceID, imageURL string) (string, error) {
	if s.s3Client == nil {
		return "", fmt.Errorf("file storage is not configured")
	}
	return s.importExternalImageWithUploader(ctx, workspaceID, imageURL, &s3ImageUploader{store: s.s3Client})
}

func (s *DocsImportService) importExternalImageWithUploader(ctx context.Context, workspaceID, imageURL string, uploader helpscout.ImageUploader) (string, error) {
	if strings.TrimSpace(imageURL) == "" {
		return "", fmt.Errorf("image_url is required")
	}

	parsed, err := url.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("invalid image_url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("image_url must use http or https")
	}

	if s.s3Client != nil {
		publicPrefix := s.s3Client.PublicURL("")
		if publicPrefix != "" && strings.HasPrefix(imageURL, publicPrefix) {
			return imageURL, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("create image request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(path.Ext(parsed.Path))
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("remote URL is not an image")
	}

	filename := path.Base(parsed.Path)
	if filename == "" || filename == "." || filename == "/" {
		filename = "image"
		if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
			filename += exts[0]
		}
	}

	uploadedURL, err := uploader.UploadImage(ctx, workspaceID, filename, resp.Body, contentType)
	if err != nil {
		return "", err
	}
	return uploadedURL, nil
}

// Preview fetches collection metadata from HelpScout for import preview.
func (s *DocsImportService) Preview(ctx context.Context, apiKey string) (*model.DocsImportPreviewResponse, error) {
	client := helpscout.NewClient(apiKey)

	collections, err := client.ListCollections(ctx)
	if err != nil {
		return nil, err
	}

	previews := make([]model.HelpscoutCollectionPreview, 0, len(collections))
	for _, coll := range collections {
		categories, err := client.ListCategories(ctx, coll.ID)
		if err != nil {
			return nil, fmt.Errorf("list categories for collection %s: %w", coll.ID, err)
		}

		articles, err := client.ListArticles(ctx, coll.ID)
		if err != nil {
			return nil, fmt.Errorf("list articles for collection %s: %w", coll.ID, err)
		}

		previews = append(previews, model.HelpscoutCollectionPreview{
			ID:            coll.ID,
			Name:          coll.Name,
			Slug:          coll.Slug,
			CategoryCount: len(categories),
			ArticleCount:  len(articles),
		})
	}

	return &model.DocsImportPreviewResponse{Collections: previews}, nil
}

// Start validates the request, creates an import job, and launches a background import.
func (s *DocsImportService) Start(ctx context.Context, req model.DocsImportStartRequest, workspaceID, userID string) (string, error) {
	if req.HelpscoutCollectionID == "" {
		return "", fmt.Errorf("helpscout collection ID is required")
	}
	if req.TargetSpaceID == nil && req.NewSpaceName == nil {
		return "", fmt.Errorf("either target_space_id or new_space_name is required")
	}

	var spaceID string

	if req.NewSpaceName != nil && *req.NewSpaceName != "" {
		space, err := s.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{
			Name:       *req.NewSpaceName,
			Visibility: model.SpaceVisibilityWorkspaceWide,
			Type:       model.SpaceTypeExternalCapable,
		}, userID)
		if err != nil {
			return "", fmt.Errorf("create space: %w", err)
		}
		spaceID = space.ID
	} else {
		spaceID = *req.TargetSpaceID
	}

	now := time.Now()
	configJSON, err := json.Marshal(docsImportJobConfig{
		HelpScoutCollectionID: req.HelpscoutCollectionID,
		ImportStatus:          normalizeDocsImportStatus(req.ImportStatus),
	})
	if err != nil {
		return "", fmt.Errorf("marshal import config: %w", err)
	}
	job := &model.DocsImportJob{
		WorkspaceID: workspaceID,
		SpaceID:     &spaceID,
		Source:      "helpscout",
		Status:      model.DocsImportStatusPending,
		Failures:    json.RawMessage("[]"),
		Config:      configJSON,
		StartedBy:   userID,
		StartedAt:   &now,
	}
	if err := s.importRepo.Create(ctx, job); err != nil {
		return "", fmt.Errorf("create import job: %w", err)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("import job panicked", "job_id", job.ID, "panic", r)
				_ = s.importRepo.SetError(context.Background(), job.ID, fmt.Sprintf("panic: %v", r))
				failTime := time.Now()
				_ = s.importRepo.UpdateStatus(context.Background(), job.ID, model.DocsImportStatusFailed, &failTime)
			}
		}()
		s.runImport(job.ID, req.APIKey, req, spaceID, workspaceID, userID)
	}()

	return job.ID, nil
}

// redirectEntry is a single entry in the redirect map JSON array.
type redirectEntry struct {
	OldURL  string `json:"old_url"`
	NewSlug string `json:"new_slug"`
}

// articleStats holds per-article outcome counts returned by importArticle.
type articleStats struct {
	Published            bool
	RedirectCreated      bool
	Uncategorized        bool
	HadConversionWarning bool
	HTMLBlockFallbacks   int
	ImageRewriteFailures int
	NormalizedNoteBlocks int
}

type docsImportJobConfig struct {
	HelpScoutCollectionID string `json:"helpscout_collection_id,omitempty"`
	ImportStatus          string `json:"import_status,omitempty"`
}

type helpscoutCategoryTarget struct {
	collectionID   *string
	collectionSlug string
}

type helpscoutCategoryMapping struct {
	byID          map[string]helpscoutCategoryTarget
	bySlug        map[string]helpscoutCategoryTarget
	byName        map[string]helpscoutCategoryTarget
	uncategorized helpscoutCategoryTarget
}

// runImport is the background worker that performs the actual HelpScout import.
func (s *DocsImportService) runImport(jobID, apiKey string, req model.DocsImportStartRequest, spaceID, workspaceID, userID string) {
	ctx := context.Background()

	// Mark job as running.
	if err := s.importRepo.UpdateStatus(ctx, jobID, model.DocsImportStatusRunning, nil); err != nil {
		s.logger.Error("failed to set job running", "job_id", jobID, "error", err)
		return
	}

	client := helpscout.NewClient(apiKey)

	// Fetch categories and create collections.
	categories, err := client.ListCategories(ctx, req.HelpscoutCollectionID)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("list categories: %v", err))
		return
	}

	categoryToCollection := make(map[string]string, len(categories))
	categoryToCollectionSlug := make(map[string]string, len(categories))
	for _, cat := range categories {
		slug := cat.Slug
		coll, err := s.collectionSvc.Create(ctx, workspaceID, spaceID, model.CreateDocsCollectionRequest{
			Name: cat.Name,
			Slug: &slug,
		}, userID)
		if err != nil {
			s.failJob(ctx, jobID, fmt.Sprintf("create collection %q: %v", cat.Name, err))
			return
		}
		categoryToCollection[cat.ID] = coll.ID
		categoryToCollectionSlug[cat.ID] = slug

		// Create legacy redirect for HelpScout category URL.
		collCanonical := buildDocsHelpcenterCollectionCanonicalPath(nil, "", coll.Slug, coll.PublicID)
		catRedirect := &model.DocsRedirect{
			WorkspaceID:          workspaceID,
			SourcePath:           fmt.Sprintf("/category/%d-%s", cat.Number, cat.Slug),
			TargetCollectionSlug: slug,
			TargetPath:           &collCanonical,
			Type:                 model.RedirectTypeImported,
			SourceSystem:         stringPtr("helpscout"),
			SourceObjectType:     stringPtr("category"),
			SourceObjectID:       stringPtr(cat.ID),
		}
		if err := s.redirectRepo.Create(ctx, catRedirect); err != nil {
			s.logger.Error("create category redirect", "error", err, "category_id", cat.ID)
		}
	}

	// Fetch article list.
	articleRefs, err := client.ListArticles(ctx, req.HelpscoutCollectionID)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("list articles: %v", err))
		return
	}

	// Update total count.
	if err := s.importRepo.SetTotal(ctx, jobID, len(articleRefs)); err != nil {
		s.logger.Error("failed to set job total", "job_id", jobID, "error", err)
	}

	// Prepare image uploader.
	var uploader helpscout.ImageUploader
	if s.s3Client != nil {
		uploader = &s3ImageUploader{store: s.s3Client}
	}

	var (
		completed             int
		failed                int
		published             int
		drafted               int
		artRedirects          int
		articlesUncategorized int
		articlesWithWarnings  int
		htmlBlockFallbacks    int
		imageRewriteFailures  int
		normalizedNoteBlocks  int
		failures              []model.ImportFailure
		redirects             []redirectEntry
	)

	categoryMapping := buildHelpScoutCategoryMapping(categories, categoryToCollection, categoryToCollectionSlug)

	for i, ref := range articleRefs {
		stats, err := s.importArticle(ctx, client, ref, spaceID, workspaceID, userID, categoryMapping, uploader, req.ImportStatus, &redirects)
		if err != nil {
			s.logger.Error("article import failed",
				"job_id", jobID,
				"article_id", ref.ID,
				"article_name", ref.Name,
				"error", err,
			)
			failed++
			failures = append(failures, model.ImportFailure{
				ArticleID: ref.ID,
				Title:     ref.Name,
				Error:     err.Error(),
			})
		} else {
			completed++
			if stats.Published {
				published++
			} else {
				drafted++
			}
			if stats.RedirectCreated {
				artRedirects++
			}
			if stats.Uncategorized {
				articlesUncategorized++
			}
			if stats.HadConversionWarning {
				articlesWithWarnings++
			}
			htmlBlockFallbacks += stats.HTMLBlockFallbacks
			imageRewriteFailures += stats.ImageRewriteFailures
			normalizedNoteBlocks += stats.NormalizedNoteBlocks
		}

		// Update progress every 5 articles or on last article.
		if (i+1)%5 == 0 || i == len(articleRefs)-1 {
			failuresJSON, _ := json.Marshal(failures)
			if failuresJSON == nil {
				failuresJSON = json.RawMessage("[]")
			}
			if err := s.importRepo.UpdateProgress(ctx, jobID, completed, failed, failuresJSON); err != nil {
				s.logger.Error("failed to update progress", "job_id", jobID, "error", err)
			}
		}
	}

	// Store redirect map.
	redirectJSON, err := json.Marshal(redirects)
	if err != nil {
		s.logger.Error("failed to marshal redirect map", "job_id", jobID, "error", err)
	} else {
		if err := s.importRepo.SetRedirectMap(ctx, jobID, redirectJSON); err != nil {
			s.logger.Error("failed to set redirect map", "job_id", jobID, "error", err)
		}
	}

	// Store import summary.
	summary := model.ImportSummary{
		CollectionsCreated:             len(categories),
		ArticlesPublished:              published,
		ArticlesDrafted:                drafted,
		RedirectsCreated:               len(categories) + artRedirects, // category + article redirects
		ArticlesUncategorized:          articlesUncategorized,
		ArticlesWithConversionWarnings: articlesWithWarnings,
		HTMLBlockFallbacks:             htmlBlockFallbacks,
		ImageRewriteFailures:           imageRewriteFailures,
		NormalizedNoteBlocks:           normalizedNoteBlocks,
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		s.logger.Error("failed to marshal import summary", "job_id", jobID, "error", err)
	} else {
		if err := s.importRepo.SetSummary(ctx, jobID, summaryJSON); err != nil {
			s.logger.Error("failed to set import summary", "job_id", jobID, "error", err)
		}
	}

	// Mark job done.
	doneTime := time.Now()
	status := model.DocsImportStatusDone
	if failed > 0 && completed == 0 {
		status = model.DocsImportStatusFailed
	}
	if err := s.importRepo.UpdateStatus(ctx, jobID, status, &doneTime); err != nil {
		s.logger.Error("failed to mark job done", "job_id", jobID, "error", err)
	}

	s.logger.Info("import job finished",
		"job_id", jobID,
		"completed", completed,
		"failed", failed,
		"total", len(articleRefs),
	)
}

// importArticle imports a single HelpScout article into Helpin.
func (s *DocsImportService) importArticle(
	ctx context.Context,
	client *helpscout.Client,
	ref helpscout.ArticleRef,
	spaceID, workspaceID, userID string,
	categoryMapping helpscoutCategoryMapping,
	uploader helpscout.ImageUploader,
	importStatus string,
	redirects *[]redirectEntry,
) (*articleStats, error) {
	mode := normalizeDocsImportStatus(importStatus)

	// Preserve the source-published version for live articles, but allow draft
	// content to import when the source article itself is not published.
	useDraft := shouldImportHelpScoutDraft(ref, mode)
	article, err := client.GetArticle(ctx, ref.ID, useDraft)
	if err != nil {
		return nil, fmt.Errorf("fetch article %s: %w", ref.ID, err)
	}

	html := article.Text
	imageWarnings := make([]docsimport.Warning, 0)

	// Process images if uploader is available.
	if uploader != nil && html != "" {
		processed, keptURLs, err := helpscout.ProcessImagesDetailed(ctx, html, uploader, workspaceID)
		if err != nil {
			s.logger.Warn("image processing failed, using original HTML",
				"article_id", ref.ID,
				"error", err,
			)
		} else {
			html = processed
			for _, keptURL := range keptURLs {
				imageWarnings = append(imageWarnings, docsimport.Warning{
					Type:    "image_url_kept",
					Message: fmt.Sprintf("image URL kept without rewrite: %s", keptURL),
				})
			}
		}
	}

	sourceHTML := html
	convResult, allWarnings, err := convertHelpScoutHTML(html)
	if err != nil {
		return nil, fmt.Errorf("convert HTML for article %s: %w", ref.ID, err)
	}
	allWarnings = append(imageWarnings, allWarnings...)
	for _, w := range allWarnings {
		s.logger.Warn("import conversion warning",
			"article_id", ref.ID,
			"warning_type", w.Type,
			"warning", w.Message,
		)
	}

	target := resolveHelpScoutCategoryTarget(categoryMapping, article.Categories)
	collectionID := target.collectionID
	collectionSlug := target.collectionSlug

	// Create document.
	doc, err := s.documentSvc.Create(ctx, workspaceID, model.CreateDocsDocumentRequest{
		SpaceID:      spaceID,
		CollectionID: collectionID,
		Title:        article.Name,
	}, userID)
	if err != nil {
		return nil, fmt.Errorf("create document for article %s: %w", ref.ID, err)
	}

	// Save content as canonical Tiptap JSON.
	contentJSON, err := json.Marshal(convResult.Doc)
	if err != nil {
		return nil, fmt.Errorf("marshal content for article %s: %w", ref.ID, err)
	}
	savedContent, err := s.contentSvc.Save(ctx, doc.ID, json.RawMessage(contentJSON), userID)
	if err != nil {
		return nil, fmt.Errorf("save content for article %s: %w", ref.ID, err)
	}

	// Store import provenance — post-image-rewrite, pre-conversion HTML snapshot.
	sourceSystem := "helpscout"
	s.contentSvc.SetImportProvenance(ctx, savedContent.ID, sourceHTML, sourceSystem, ref.ID)

	// Create helpcenter article record with slug from HelpScout.
	hcArticle := &model.DocsHelpcenterArticle{
		DocumentID: doc.ID,
		Slug:       article.Slug,
	}
	createdHCArticle, err := s.helpcenterSvc.CreateArticle(ctx, hcArticle)
	if err != nil {
		return nil, fmt.Errorf("create helpcenter article for %s: %w", ref.ID, err)
	}

	// Create legacy redirect for HelpScout article URL.
	stats := summarizeImportWarnings(allWarnings)
	stats.Uncategorized = collectionID == nil
	if collectionSlug != "" {
		articleSlug := article.Slug
		var artCanonical string
		if createdHCArticle != nil {
			artCanonical = buildDocsHelpcenterArticleCanonicalPath(nil, "", createdHCArticle.Slug, createdHCArticle.PublicID)
		}
		articleRedirect := &model.DocsRedirect{
			WorkspaceID:          workspaceID,
			SourcePath:           fmt.Sprintf("/article/%d-%s", article.Number, article.Slug),
			TargetCollectionSlug: collectionSlug,
			TargetArticleSlug:    &articleSlug,
			TargetPath:           &artCanonical,
			Type:                 model.RedirectTypeImported,
			SourceSystem:         stringPtr("helpscout"),
			SourceObjectType:     stringPtr("article"),
			SourceObjectID:       stringPtr(ref.ID),
		}
		if err := s.redirectRepo.Create(ctx, articleRedirect); err != nil {
			s.logger.Error("create article redirect", "error", err, "article_id", ref.ID)
			// Non-fatal.
		} else {
			stats.RedirectCreated = true
		}
	}

	if shouldPublishImportedArticle(ref, mode) {
		if _, err := s.documentSvc.Publish(ctx, doc.ID); err != nil {
			return nil, fmt.Errorf("publish imported article internally %s: %w", ref.ID, err)
		}
		if err := s.helpcenterSvc.PublishExternally(ctx, doc.ID, article.Slug); err != nil {
			return nil, fmt.Errorf("publish imported article externally %s: %w", ref.ID, err)
		}
		stats.Published = true
	}

	// Add to redirect map.
	*redirects = append(*redirects, redirectEntry{
		OldURL:  fmt.Sprintf("/article/%s-%s", article.Slug, ref.ID),
		NewSlug: article.Slug,
	})

	return stats, nil
}

// ReconvertResult holds the outcome of a reconversion run.
type ReconvertResult struct {
	Total                int `json:"total"`
	Converted            int `json:"converted"`
	Failed               int `json:"failed"`
	ArticlesWithWarnings int `json:"articles_with_warnings"`
	HTMLBlockFallbacks   int `json:"html_block_fallbacks"`
	NormalizedNoteBlocks int `json:"normalized_note_blocks"`
}

// Reconvert re-runs the HTML-to-Tiptap converter on all documents from a previous import job,
// using the stored import_source_html. This overwrites existing content.
func (s *DocsImportService) Reconvert(ctx context.Context, jobID string) (*ReconvertResult, error) {
	job, err := s.importRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get import job: %w", err)
	}
	if job == nil {
		return nil, fmt.Errorf("import job not found")
	}

	if job.SpaceID == nil || *job.SpaceID == "" {
		return nil, fmt.Errorf("import job has no space ID")
	}
	contents, err := s.contentSvc.ListBySpaceWithImportHTML(ctx, *job.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("list reconvertible docs: %w", err)
	}

	result := &ReconvertResult{Total: len(contents)}

	for _, c := range contents {
		if c.ImportSourceHTML == nil || *c.ImportSourceHTML == "" {
			continue
		}

		sourceSystem := ""
		if c.ImportSourceSystem != nil {
			sourceSystem = *c.ImportSourceSystem
		}

		var contentJSON json.RawMessage
		var warnings []docsimport.Warning

		switch sourceSystem {
		case "nextra":
			// Re-run MDX preprocessing and markdown-to-TipTap conversion.
			contentJSON = nextraContentToTiptapJSON(*c.ImportSourceHTML)
			// No warnings tracked for markdown conversion currently.

		default:
			// HelpScout and other HTML-based sources.
			convResult, convWarnings, err := convertHelpScoutHTML(*c.ImportSourceHTML)
			if err != nil {
				s.logger.Error("reconvert failed", "content_id", c.ID, "error", err)
				result.Failed++
				continue
			}
			var marshalErr error
			contentJSON, marshalErr = json.Marshal(convResult.Doc)
			if marshalErr != nil {
				s.logger.Error("reconvert marshal failed", "content_id", c.ID, "error", marshalErr)
				result.Failed++
				continue
			}
			warnings = convWarnings
		}

		if _, err := s.contentSvc.Save(ctx, c.DocumentID, contentJSON, ""); err != nil {
			s.logger.Error("reconvert save failed", "content_id", c.ID, "error", err)
			result.Failed++
			continue
		}

		result.Converted++
		if len(warnings) > 0 {
			result.ArticlesWithWarnings++
		}
		for _, warning := range warnings {
			switch warning.Type {
			case "html_block_fallback":
				result.HTMLBlockFallbacks++
			case "helpscout_note_block_normalized":
				result.NormalizedNoteBlocks++
			}
		}
	}

	if len(job.Summary) > 0 {
		var summary model.ImportSummary
		if err := json.Unmarshal(job.Summary, &summary); err == nil {
			summary.ArticlesWithConversionWarnings = result.ArticlesWithWarnings
			summary.HTMLBlockFallbacks = result.HTMLBlockFallbacks
			summary.NormalizedNoteBlocks = result.NormalizedNoteBlocks
			if summaryJSON, marshalErr := json.Marshal(summary); marshalErr == nil {
				_ = s.importRepo.SetSummary(ctx, job.ID, summaryJSON)
			}
		}
	}

	return result, nil
}

// GetStatus returns the current state of an import job.
// ListJobs returns all import jobs for a workspace.
func (s *DocsImportService) ListJobs(ctx context.Context, workspaceID string) ([]model.DocsImportJob, error) {
	return s.importRepo.ListByWorkspace(ctx, workspaceID)
}

func (s *DocsImportService) GetStatus(ctx context.Context, jobID string) (*model.DocsImportJob, error) {
	return s.importRepo.GetByID(ctx, jobID)
}

// Retry re-imports only the articles that failed in a previous job run.
func (s *DocsImportService) Retry(ctx context.Context, jobID, apiKey string) error {
	job, err := s.importRepo.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get import job: %w", err)
	}
	if job == nil {
		return fmt.Errorf("import job not found")
	}
	if job.Status != model.DocsImportStatusDone && job.Status != model.DocsImportStatusFailed {
		return fmt.Errorf("can only retry completed or failed jobs")
	}

	var failures []model.ImportFailure
	if err := json.Unmarshal(job.Failures, &failures); err != nil {
		return fmt.Errorf("parse failures: %w", err)
	}
	if len(failures) == 0 {
		return fmt.Errorf("no failures to retry")
	}

	spaceID := ""
	if job.SpaceID != nil {
		spaceID = *job.SpaceID
	}
	if spaceID == "" {
		return fmt.Errorf("job has no target space")
	}

	// Mark job as running again.
	if err := s.importRepo.UpdateStatus(ctx, jobID, model.DocsImportStatusRunning, nil); err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("retry import panicked", "job_id", jobID, "panic", r)
				_ = s.importRepo.SetError(context.Background(), jobID, fmt.Sprintf("panic: %v", r))
				failTime := time.Now()
				_ = s.importRepo.UpdateStatus(context.Background(), jobID, model.DocsImportStatusFailed, &failTime)
			}
		}()
		s.runRetry(jobID, apiKey, failures, spaceID, job.WorkspaceID, job.StartedBy)
	}()

	return nil
}

func readDocsImportJobConfig(job *model.DocsImportJob) docsImportJobConfig {
	cfg := docsImportJobConfig{}
	if job == nil || len(job.Config) == 0 {
		return cfg
	}
	_ = json.Unmarshal(job.Config, &cfg)
	cfg.ImportStatus = normalizeDocsImportStatus(cfg.ImportStatus)
	return cfg
}

// runRetry re-imports failed articles in the background.
func (s *DocsImportService) runRetry(jobID, apiKey string, failures []model.ImportFailure, spaceID, workspaceID, userID string) {
	ctx := context.Background()
	client := helpscout.NewClient(apiKey)
	job, err := s.importRepo.GetByID(ctx, jobID)
	if err != nil {
		s.logger.Error("retry load job failed", "job_id", jobID, "error", err)
		return
	}
	importCfg := readDocsImportJobConfig(job)

	var uploader helpscout.ImageUploader
	if s.s3Client != nil {
		uploader = &s3ImageUploader{store: s.s3Client}
	}

	var (
		retryCompleted int
		retryFailed    int
		newFailures    []model.ImportFailure
		redirects      []redirectEntry
	)

	for i, f := range failures {
		// Fetch article ref by getting the full article.
		article, err := client.GetArticle(ctx, f.ArticleID, false)
		if err != nil {
			s.logger.Error("retry fetch failed", "article_id", f.ArticleID, "error", err)
			retryFailed++
			newFailures = append(newFailures, model.ImportFailure{
				ArticleID: f.ArticleID,
				Title:     f.Title,
				Error:     err.Error(),
			})
			continue
		}

		ref := article.ArticleRef
		if _, err := s.importArticle(ctx, client, ref, spaceID, workspaceID, userID, helpscoutCategoryMapping{uncategorized: helpscoutCategoryTarget{collectionSlug: "uncategorized"}}, uploader, importCfg.ImportStatus, &redirects); err != nil {
			s.logger.Error("retry article import failed", "article_id", f.ArticleID, "error", err)
			retryFailed++
			newFailures = append(newFailures, model.ImportFailure{
				ArticleID: f.ArticleID,
				Title:     f.Title,
				Error:     err.Error(),
			})
		} else {
			retryCompleted++
		}

		// Update progress every 5 articles or on last.
		if (i+1)%5 == 0 || i == len(failures)-1 {
			failuresJSON, _ := json.Marshal(newFailures)
			if failuresJSON == nil {
				failuresJSON = json.RawMessage("[]")
			}
			_ = s.importRepo.UpdateProgress(ctx, jobID, retryCompleted, retryFailed, failuresJSON)
		}
	}

	// Append new redirects to existing redirect map.
	if len(redirects) > 0 {
		redirectJSON, err := json.Marshal(redirects)
		if err == nil {
			_ = s.importRepo.SetRedirectMap(ctx, jobID, redirectJSON)
		}
	}

	doneTime := time.Now()
	status := model.DocsImportStatusDone
	if retryFailed > 0 && retryCompleted == 0 {
		status = model.DocsImportStatusFailed
	}
	_ = s.importRepo.UpdateStatus(ctx, jobID, status, &doneTime)

	s.logger.Info("retry import finished",
		"job_id", jobID,
		"completed", retryCompleted,
		"failed", retryFailed,
	)
}

// GetRedirectMap returns the redirect map for an import job.
func (s *DocsImportService) GetRedirectMap(ctx context.Context, jobID string) (json.RawMessage, error) {
	job, err := s.importRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get import job: %w", err)
	}
	if job == nil {
		return nil, fmt.Errorf("import job not found")
	}
	return job.RedirectMap, nil
}

// failJob sets the job error message and marks it as failed.
func (s *DocsImportService) failJob(ctx context.Context, jobID, errMsg string) {
	s.logger.Error("import job failed", "job_id", jobID, "error", errMsg)
	_ = s.importRepo.SetError(ctx, jobID, errMsg)
	failTime := time.Now()
	_ = s.importRepo.UpdateStatus(ctx, jobID, model.DocsImportStatusFailed, &failTime)
}

func buildHelpScoutCategoryMapping(categories []helpscout.Category, categoryToCollection, categoryToCollectionSlug map[string]string) helpscoutCategoryMapping {
	mapping := helpscoutCategoryMapping{
		byID:   make(map[string]helpscoutCategoryTarget, len(categories)),
		bySlug: make(map[string]helpscoutCategoryTarget, len(categories)),
		byName: make(map[string]helpscoutCategoryTarget, len(categories)),
		uncategorized: helpscoutCategoryTarget{
			collectionSlug: "uncategorized",
		},
	}

	for _, category := range categories {
		target := helpscoutCategoryTarget{
			collectionSlug: categoryToCollectionSlug[category.ID],
		}
		if collectionID, ok := categoryToCollection[category.ID]; ok {
			target.collectionID = stringPtr(collectionID)
		}
		mapping.byID[normalizeHelpScoutCategoryKey(category.ID)] = target
		mapping.bySlug[normalizeHelpScoutCategoryKey(category.Slug)] = target
		mapping.byName[normalizeHelpScoutCategoryKey(category.Name)] = target
	}

	return mapping
}

func resolveHelpScoutCategoryTarget(mapping helpscoutCategoryMapping, categories []string) helpscoutCategoryTarget {
	for _, category := range categories {
		key := normalizeHelpScoutCategoryKey(category)
		if target, ok := mapping.byID[key]; ok {
			return target
		}
		if target, ok := mapping.bySlug[key]; ok {
			return target
		}
		if target, ok := mapping.byName[key]; ok {
			return target
		}
	}
	return mapping.uncategorized
}

func normalizeHelpScoutCategoryKey(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func summarizeImportWarnings(warnings []docsimport.Warning) *articleStats {
	stats := &articleStats{}
	if len(warnings) > 0 {
		stats.HadConversionWarning = true
	}
	for _, warning := range warnings {
		switch warning.Type {
		case "html_block_fallback":
			stats.HTMLBlockFallbacks++
		case "image_url_kept":
			stats.ImageRewriteFailures++
		case "helpscout_note_block_normalized":
			stats.NormalizedNoteBlocks++
		}
	}
	return stats
}

func convertHelpScoutHTML(rawHTML string) (*docsimport.ConversionResult, []docsimport.Warning, error) {
	normalizedHTML, preprocessingWarnings := docsimport.PreprocessHelpScoutHTML(rawHTML)
	convResult, err := docsimport.ConvertHTML(normalizedHTML)
	if err != nil {
		return nil, nil, err
	}
	allWarnings := append(preprocessingWarnings, convResult.Warnings...)
	return convResult, allWarnings, nil
}

func normalizeDocsImportStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "draft", "published", "match_source":
		return strings.TrimSpace(strings.ToLower(status))
	case "all_draft":
		return "draft"
	default:
		return "match_source"
	}
}

func shouldImportHelpScoutDraft(ref helpscout.ArticleRef, importStatus string) bool {
	if !ref.HasDraft {
		return false
	}

	switch normalizeDocsImportStatus(importStatus) {
	case "draft":
		return true
	case "published", "match_source":
		return ref.Status != "published"
	default:
		return ref.Status != "published"
	}
}

func shouldPublishImportedArticle(ref helpscout.ArticleRef, importStatus string) bool {
	switch normalizeDocsImportStatus(importStatus) {
	case "draft":
		return false
	case "published":
		return true
	case "match_source":
		return ref.Status == "published"
	default:
		return ref.Status == "published"
	}
}
