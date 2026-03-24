package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	s3Client *storage.S3Client
}

// UploadImage uploads an image to S3 and returns the public URL.
func (u *s3ImageUploader) UploadImage(ctx context.Context, workspaceID, filename string, data io.Reader, contentType string) (string, error) {
	key := fmt.Sprintf("docs-import/%s/%s-%s", workspaceID, uuid.New().String(), filename)
	if err := u.s3Client.PutObject(ctx, key, contentType, -1, data, true); err != nil {
		return "", fmt.Errorf("upload image %q: %w", filename, err)
	}
	return u.s3Client.PublicURL(key), nil
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
	job := &model.DocsImportJob{
		WorkspaceID: workspaceID,
		SpaceID:     &spaceID,
		Source:      "helpscout",
		Status:      model.DocsImportStatusPending,
		Failures:    json.RawMessage("[]"),
		Config:      json.RawMessage("{}"),
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
		catRedirect := &model.DocsRedirect{
			WorkspaceID:          workspaceID,
			SourcePath:           fmt.Sprintf("/category/%d-%s", cat.Number, cat.Slug),
			TargetCollectionSlug: slug,
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
		uploader = &s3ImageUploader{s3Client: s.s3Client}
	}

	var (
		completed int
		failed    int
		failures  []model.ImportFailure
		redirects []redirectEntry
	)

	for i, ref := range articleRefs {
		if err := s.importArticle(ctx, client, ref, spaceID, workspaceID, userID, categoryToCollection, categoryToCollectionSlug, uploader, req.ImportStatus, &redirects); err != nil {
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
	categoryToCollection map[string]string,
	categoryToCollectionSlug map[string]string,
	uploader helpscout.ImageUploader,
	importStatus string,
	redirects *[]redirectEntry,
) error {
	// Fetch full article. Use draft if import_status is "match_source" and article has a draft.
	useDraft := importStatus == "match_source" && ref.HasDraft
	article, err := client.GetArticle(ctx, ref.ID, useDraft)
	if err != nil {
		return fmt.Errorf("fetch article %s: %w", ref.ID, err)
	}

	html := article.Text

	// Process images if uploader is available.
	if uploader != nil && html != "" {
		processed, err := helpscout.ProcessImages(ctx, html, uploader, workspaceID)
		if err != nil {
			s.logger.Warn("image processing failed, using original HTML",
				"article_id", ref.ID,
				"error", err,
			)
		} else {
			html = processed
		}
	}

	// Convert HTML to canonical Tiptap JSON.
	convResult, err := docsimport.ConvertHTML(html)
	if err != nil {
		return fmt.Errorf("convert HTML for article %s: %w", ref.ID, err)
	}
	for _, w := range convResult.Warnings {
		s.logger.Warn("import conversion warning",
			"article_id", ref.ID,
			"warning_type", w.Type,
			"warning", w.Message,
		)
	}

	// Determine collection ID and slug from first category.
	// Use article.Categories (from single-article fetch) rather than
	// ref.Categories (from list endpoint, which omits categories).
	var collectionID *string
	var collectionSlug string
	if len(article.Categories) > 0 {
		if cID, ok := categoryToCollection[article.Categories[0]]; ok {
			collectionID = &cID
		}
		if slug, ok := categoryToCollectionSlug[article.Categories[0]]; ok {
			collectionSlug = slug
		}
	}

	// Create document.
	doc, err := s.documentSvc.Create(ctx, workspaceID, model.CreateDocsDocumentRequest{
		SpaceID:      spaceID,
		CollectionID: collectionID,
		Title:        ref.Name,
	}, userID)
	if err != nil {
		return fmt.Errorf("create document for article %s: %w", ref.ID, err)
	}

	// Save content as canonical Tiptap JSON.
	contentJSON, err := json.Marshal(convResult.Doc)
	if err != nil {
		return fmt.Errorf("marshal content for article %s: %w", ref.ID, err)
	}
	savedContent, err := s.contentSvc.Save(ctx, doc.ID, json.RawMessage(contentJSON))
	if err != nil {
		return fmt.Errorf("save content for article %s: %w", ref.ID, err)
	}

	// Store import provenance — post-image-rewrite, pre-conversion HTML snapshot.
	sourceSystem := "helpscout"
	s.contentSvc.SetImportProvenance(ctx, savedContent.ID, html, sourceSystem, ref.ID)

	// Create helpcenter article record with slug from HelpScout.
	hcArticle := &model.DocsHelpcenterArticle{
		DocumentID: doc.ID,
		Slug:       ref.Slug,
	}
	if _, err := s.helpcenterSvc.CreateArticle(ctx, hcArticle); err != nil {
		return fmt.Errorf("create helpcenter article for %s: %w", ref.ID, err)
	}

	// Create legacy redirect for HelpScout article URL.
	if collectionSlug != "" {
		articleSlug := ref.Slug
		articleRedirect := &model.DocsRedirect{
			WorkspaceID:          workspaceID,
			SourcePath:           fmt.Sprintf("/article/%d-%s", ref.Number, ref.Slug),
			TargetCollectionSlug: collectionSlug,
			TargetArticleSlug:    &articleSlug,
			Type:                 model.RedirectTypeImported,
			SourceSystem:         stringPtr("helpscout"),
			SourceObjectType:     stringPtr("article"),
			SourceObjectID:       stringPtr(ref.ID),
		}
		if err := s.redirectRepo.Create(ctx, articleRedirect); err != nil {
			s.logger.Error("create article redirect", "error", err, "article_id", ref.ID)
			// Non-fatal.
		}
	}

	// Publish if the source article is published and import_status allows it.
	if ref.Status == "published" && importStatus != "all_draft" {
		if _, err := s.documentSvc.Publish(ctx, doc.ID); err != nil {
			s.logger.Warn("failed to publish imported article",
				"article_id", ref.ID,
				"document_id", doc.ID,
				"error", err,
			)
		}
	}

	// Add to redirect map.
	*redirects = append(*redirects, redirectEntry{
		OldURL:  fmt.Sprintf("/article/%s-%s", ref.Slug, ref.ID),
		NewSlug: ref.Slug,
	})

	return nil
}

// ReconvertResult holds the outcome of a reconversion run.
type ReconvertResult struct {
	Total     int `json:"total"`
	Converted int `json:"converted"`
	Failed    int `json:"failed"`
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

		convResult, err := docsimport.ConvertHTML(*c.ImportSourceHTML)
		if err != nil {
			s.logger.Error("reconvert failed", "content_id", c.ID, "error", err)
			result.Failed++
			continue
		}

		contentJSON, err := json.Marshal(convResult.Doc)
		if err != nil {
			s.logger.Error("reconvert marshal failed", "content_id", c.ID, "error", err)
			result.Failed++
			continue
		}

		if _, err := s.contentSvc.Save(ctx, c.DocumentID, json.RawMessage(contentJSON)); err != nil {
			s.logger.Error("reconvert save failed", "content_id", c.ID, "error", err)
			result.Failed++
			continue
		}

		result.Converted++
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

// runRetry re-imports failed articles in the background.
func (s *DocsImportService) runRetry(jobID, apiKey string, failures []model.ImportFailure, spaceID, workspaceID, userID string) {
	ctx := context.Background()
	client := helpscout.NewClient(apiKey)

	var uploader helpscout.ImageUploader
	if s.s3Client != nil {
		uploader = &s3ImageUploader{s3Client: s.s3Client}
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
		if err := s.importArticle(ctx, client, ref, spaceID, workspaceID, userID, nil, nil, uploader, "", &redirects); err != nil {
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
