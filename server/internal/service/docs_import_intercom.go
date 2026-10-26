package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

const docsImportSourceIntercom = "intercom"

// SetIntercomBaseURL replaces the Intercom API host (INTERCOM_API_BASE_URL).
func (s *DocsImportService) SetIntercomBaseURL(baseURL string) *DocsImportService {
	s.intercomBaseURLOverride = baseURL
	return s
}

// ErrIntercomImportInvalid wraps a caller-fixable Intercom import problem.
var ErrIntercomImportInvalid = errors.New("invalid Intercom import")

// intercomRegionBaseURLs lists the Intercom API hosts; tokens only work in
// their own region.
var intercomRegionBaseURLs = map[string]string{
	"us": intercom.DefaultBaseURL,
	"eu": "https://api.eu.intercom.io",
	"au": "https://api.au.intercom.io",
}

// intercomBaseURL returns the API host for a region, or the operator's
// override.
func intercomBaseURL(region, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		region = "us"
	}
	base, ok := intercomRegionBaseURLs[region]
	if !ok {
		return "", fmt.Errorf("%w: region must be us, eu or au", ErrIntercomImportInvalid)
	}
	return base, nil
}

func (s *DocsImportService) intercomClient(token, region string) (*intercom.Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("%w: an access token is required", ErrIntercomImportInvalid)
	}
	base, err := intercomBaseURL(region, s.intercomBaseURLOverride)
	if err != nil {
		return nil, err
	}
	return intercom.NewClient(token, base), nil
}

// intercomAccessError turns API errors into caller-fixable messages.
func intercomAccessError(err error) error {
	if errors.Is(err, intercom.ErrUnauthorized) {
		return fmt.Errorf("%w: Intercom rejected the access token; check it and its region", ErrIntercomImportInvalid)
	}
	return err
}

// PreviewIntercom counts what an Intercom Help Center import would bring in.
func (s *DocsImportService) PreviewIntercom(ctx context.Context, req model.IntercomDocsImportRequest) (*model.IntercomDocsImportPreview, error) {
	client, err := s.intercomClient(req.AccessToken, req.Region)
	if err != nil {
		return nil, err
	}
	me, err := client.Me(ctx)
	if err != nil {
		return nil, intercomAccessError(err)
	}
	collections, err := client.ListCollections(ctx)
	if err != nil {
		return nil, intercomAccessError(err)
	}
	articles, err := client.ListArticles(ctx)
	if err != nil {
		return nil, intercomAccessError(err)
	}
	preview := &model.IntercomDocsImportPreview{WorkspaceName: me.App.Name, Collections: len(collections)}
	for _, article := range articles {
		if article.State == "published" {
			preview.ArticlesPublished++
		} else {
			preview.ArticlesDraft++
		}
	}
	return preview, nil
}

// StartIntercom creates an import job for an Intercom Help Center and starts
// the durable import workflow.
func (s *DocsImportService) StartIntercom(ctx context.Context, req model.IntercomDocsImportRequest, workspaceID, userID string) (string, error) {
	if req.TargetSpaceID == nil && (req.NewSpaceName == nil || strings.TrimSpace(*req.NewSpaceName) == "") {
		return "", fmt.Errorf("%w: choose a space or name a new one", ErrIntercomImportInvalid)
	}
	client, err := s.intercomClient(req.AccessToken, req.Region)
	if err != nil {
		return "", err
	}
	if _, err := client.Me(ctx); err != nil {
		return "", intercomAccessError(err)
	}
	if s.temporalClient == nil {
		return "", fmt.Errorf("docs import worker is not configured")
	}
	if len(s.encryptionKey) != 32 {
		return "", fmt.Errorf("docs import encryption key is not configured")
	}
	spaceID := ""
	if req.NewSpaceName != nil && strings.TrimSpace(*req.NewSpaceName) != "" {
		space, err := s.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{
			Name: strings.TrimSpace(*req.NewSpaceName), Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable,
		}, userID)
		if err != nil {
			return "", fmt.Errorf("create space: %w", err)
		}
		spaceID = space.ID
	} else {
		spaceID = strings.TrimSpace(*req.TargetSpaceID)
		space, err := s.spaceSvc.GetUnfiltered(ctx, spaceID)
		if err != nil || space == nil || space.WorkspaceID != workspaceID {
			return "", fmt.Errorf("%w: the space was not found", ErrIntercomImportInvalid)
		}
	}
	stored := model.DocsImportStartRequest{APIKey: req.AccessToken, IntercomRegion: req.Region, TargetSpaceID: &spaceID, ImportStatus: req.ImportStatus}
	payload, err := json.Marshal(stored)
	if err != nil {
		return "", fmt.Errorf("marshal import payload: %w", err)
	}
	encrypted, err := appcrypto.EncryptString(string(payload), s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt import payload: %w", err)
	}
	config, err := json.Marshal(docsImportJobConfig{ImportStatus: normalizeDocsImportStatus(req.ImportStatus)})
	if err != nil {
		return "", fmt.Errorf("marshal import config: %w", err)
	}
	now := time.Now()
	job := &model.DocsImportJob{
		ID: uuid.NewString(), WorkspaceID: workspaceID, SpaceID: &spaceID, Source: docsImportSourceIntercom,
		Status: model.DocsImportStatusPending, Failures: json.RawMessage("[]"), Config: config,
		StartedBy: userID, StartedAt: &now, PayloadEncrypted: &encrypted,
	}
	workflowID := temporalapp.WorkflowIDForDocsImport(job.ID)
	job.WorkflowID = &workflowID
	if err := s.importRepo.Create(ctx, job); err != nil {
		return "", fmt.Errorf("create import job: %w", err)
	}
	if _, err := s.temporalClient.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{ID: workflowID, TaskQueue: temporalapp.QueueAutomation},
		temporalapp.DocsImportWorkflow, temporalapp.DocsImportWorkflowInput{ImportID: job.ID}); err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if !errors.As(err, &alreadyStarted) {
			s.failJob(ctx, job.ID, fmt.Sprintf("start docs import workflow: %v", err))
			return "", fmt.Errorf("start docs import workflow: %w", err)
		}
	}
	return job.ID, nil
}

// intercomCollectionTarget is the Helpin collection an Intercom collection
// became.
type intercomCollectionTarget struct{ id, slug string }

var _intercomSlugPrefix = regexp.MustCompile(`^\d+-`)

// intercomSlug takes the slug from an Intercom Help Center URL such as
// .../articles/123-reset-password, falling back to the title.
func intercomSlug(rawURL, title string) string {
	if parsed, err := url.Parse(rawURL); err == nil {
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if last := segments[len(segments)-1]; last != "" {
			if slug := importSlug(_intercomSlugPrefix.ReplaceAllString(last, "")); slug != "" {
				return slug
			}
		}
	}
	if slug := importSlug(title); slug != "" {
		return slug
	}
	return "article"
}

var _importSlugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

func importSlug(value string) string {
	slug := strings.Trim(_importSlugInvalid.ReplaceAllString(strings.ToLower(value), "-"), "-")
	if len(slug) > 80 {
		slug = strings.Trim(slug[:80], "-")
	}
	return slug
}

func intercomURLPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Path == "" {
		return ""
	}
	return parsed.Path
}

// intercomCollectionsInOrder sorts collections so parents come first.
func intercomCollectionsInOrder(collections []intercom.Collection) []intercom.Collection {
	byID := make(map[string]intercom.Collection, len(collections))
	for _, collection := range collections {
		byID[collection.ID] = collection
	}
	depth := func(collection intercom.Collection) int {
		d := 0
		for parent := collection.ParentID; parent != nil && d < 10; d++ {
			next, ok := byID[*parent]
			if !ok {
				break
			}
			parent = next.ParentID
		}
		return d
	}
	ordered := slices.Clone(collections)
	slices.SortStableFunc(ordered, func(a, b intercom.Collection) int {
		if da, db := depth(a), depth(b); da != db {
			return da - db
		}
		return a.Order - b.Order
	})
	return ordered
}

// runIntercomImport imports collections and articles, resuming from stored
// provenance.
func (s *DocsImportService) runIntercomImport(ctx context.Context, jobID string, req model.DocsImportStartRequest, spaceID, workspaceID, userID string, heartbeat func(int)) error {
	if err := s.importRepo.UpdateStatus(ctx, jobID, model.DocsImportStatusRunning, nil); err != nil {
		return err
	}
	client, err := s.intercomClient(req.APIKey, req.IntercomRegion)
	if err != nil {
		s.failJob(ctx, jobID, err.Error())
		return nil
	}
	collections, err := client.ListCollections(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.failJob(ctx, jobID, fmt.Sprintf("list collections: %v", intercomAccessError(err)))
		return nil
	}
	articles, err := client.ListArticles(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.failJob(ctx, jobID, fmt.Sprintf("list articles: %v", intercomAccessError(err)))
		return nil
	}
	if err := s.importRepo.SetTotal(ctx, jobID, len(articles)); err != nil {
		s.logger.Error("failed to set job total", "job_id", jobID, "error", err)
	}

	existing, err := s.collectionSvc.List(ctx, spaceID)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("list existing collections: %v", err))
		return nil
	}
	existingBySlug := make(map[string]model.DocsCollection, len(existing))
	for _, collection := range existing {
		existingBySlug[collection.Slug] = collection
	}
	targets := map[string]intercomCollectionTarget{}
	collectionsCreated, redirectsCreated := 0, 0
	for _, source := range intercomCollectionsInOrder(collections) {
		slug := intercomSlug(source.URL, source.Name)
		collection, ok := existingBySlug[slug]
		if !ok {
			var parentID *string
			if source.ParentID != nil {
				if parent, ok := targets[*source.ParentID]; ok {
					parentID = &parent.id
				}
			}
			var description *string
			if text := strings.TrimSpace(source.Description); text != "" {
				description = &text
			}
			created, err := s.collectionSvc.Create(ctx, workspaceID, spaceID, model.CreateDocsCollectionRequest{
				Name: source.Name, Slug: &slug, Description: description, ParentCollectionID: parentID,
			}, userID)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				s.failJob(ctx, jobID, fmt.Sprintf("create collection %q: %v", source.Name, err))
				return nil
			}
			collection = *created
			existingBySlug[slug] = collection
			collectionsCreated++
		}
		targets[source.ID] = intercomCollectionTarget{id: collection.ID, slug: collection.Slug}
		if sourcePath := intercomURLPath(source.URL); sourcePath != "" {
			canonical := buildDocsHelpcenterCollectionCanonicalPath(nil, "", collection.Slug, collection.PublicID)
			created, err := s.redirectRepo.UpsertImported(ctx, &model.DocsRedirect{
				WorkspaceID: workspaceID, SourcePath: sourcePath, TargetCollectionSlug: collection.Slug, TargetPath: &canonical,
				Type: model.RedirectTypeImported, SourceSystem: stringPtr(docsImportSourceIntercom),
				SourceObjectType: stringPtr("collection"), SourceObjectID: stringPtr(source.ID),
			})
			if err != nil {
				s.logger.Error("create collection redirect", "error", err, "collection_id", source.ID)
			} else if created {
				redirectsCreated++
			}
		}
	}

	done, err := s.contentSvc.contentRepo.ListImportSourceObjectIDs(ctx, spaceID, docsImportSourceIntercom)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("load import checkpoints: %v", err))
		return nil
	}
	imported := make(map[string]bool, len(done))
	for _, id := range done {
		imported[id] = true
	}
	var uploader helpscout.ImageUploader
	if s.s3Client != nil {
		uploader = &s3ImageUploader{store: s.s3Client}
	}
	mode := normalizeDocsImportStatus(req.ImportStatus)
	var (
		completed, failed, published, drafted, uncategorized, withWarnings int
		failures                                                           []model.ImportFailure
	)
	for i, article := range articles {
		if err := ctx.Err(); err != nil {
			return err
		}
		if imported[article.ID] {
			completed++
		} else {
			var collection *intercomCollectionTarget
			if article.ParentID != nil {
				if found, ok := targets[strconv.FormatInt(*article.ParentID, 10)]; ok {
					collection = &found
				}
			}
			stats, err := s.importIntercomArticle(ctx, article, collection, spaceID, workspaceID, userID, uploader, mode)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				failed++
				failures = append(failures, model.ImportFailure{ArticleID: article.ID, Title: article.Title, Error: err.Error()})
			} else {
				completed++
				imported[article.ID] = true
				if stats.Published {
					published++
				} else {
					drafted++
				}
				if stats.RedirectCreated {
					redirectsCreated++
				}
				if stats.Uncategorized {
					uncategorized++
				}
				if stats.HadConversionWarning {
					withWarnings++
				}
			}
		}
		if (i+1)%5 == 0 || i == len(articles)-1 {
			failuresJSON, _ := json.Marshal(failures)
			if failures == nil {
				failuresJSON = json.RawMessage("[]")
			}
			if err := s.importRepo.UpdateProgress(ctx, jobID, completed, failed, failuresJSON); err != nil {
				s.logger.Error("failed to update progress", "job_id", jobID, "error", err)
			}
		}
		if heartbeat != nil {
			heartbeat(completed + failed)
		}
	}
	summary, _ := json.Marshal(model.ImportSummary{
		CollectionsCreated: collectionsCreated, ArticlesPublished: published, ArticlesDrafted: drafted,
		RedirectsCreated: redirectsCreated, ArticlesUncategorized: uncategorized, ArticlesWithConversionWarnings: withWarnings,
	})
	if err := s.importRepo.SetSummary(ctx, jobID, summary); err != nil {
		s.logger.Error("failed to set import summary", "job_id", jobID, "error", err)
	}
	status := model.DocsImportStatusDone
	if failed > 0 && completed == 0 {
		status = model.DocsImportStatusFailed
	}
	doneAt := time.Now()
	if err := s.importRepo.UpdateStatus(ctx, jobID, status, &doneAt); err != nil {
		s.logger.Error("failed to mark job done", "job_id", jobID, "error", err)
	}
	return nil
}

func (s *DocsImportService) importIntercomArticle(
	ctx context.Context,
	article intercom.Article,
	collection *intercomCollectionTarget,
	spaceID, workspaceID, userID string,
	uploader helpscout.ImageUploader,
	mode string,
) (*articleStats, error) {
	html := article.Body
	var imageWarnings []docsimport.Warning
	if uploader != nil && html != "" {
		processed, kept, err := helpscout.ProcessImagesDetailed(ctx, html, uploader, workspaceID)
		if err != nil {
			s.logger.Warn("image processing failed, using original HTML", "article_id", article.ID, "error", err)
		} else {
			html = processed
			for _, keptURL := range kept {
				imageWarnings = append(imageWarnings, docsimport.Warning{Type: "image_url_kept", Message: "image URL kept without rewrite: " + keptURL})
			}
		}
	}
	converted, warnings, err := s.convertHelpScoutHTML(ctx, workspaceID, article.ID, article.Title, html)
	if err != nil {
		return nil, fmt.Errorf("convert article %s: %w", article.ID, err)
	}
	warnings = append(imageWarnings, warnings...)
	var collectionID *string
	if collection != nil {
		collectionID = &collection.id
	}
	document, err := s.documentSvc.Create(ctx, workspaceID, model.CreateDocsDocumentRequest{SpaceID: spaceID, CollectionID: collectionID, Title: article.Title}, userID)
	if err != nil {
		return nil, fmt.Errorf("create document for article %s: %w", article.ID, err)
	}
	content, err := json.Marshal(converted.Doc)
	if err != nil {
		return nil, fmt.Errorf("marshal content for article %s: %w", article.ID, err)
	}
	saved, err := s.contentSvc.Save(ctx, document.ID, json.RawMessage(content), userID)
	if err != nil {
		return nil, fmt.Errorf("save content for article %s: %w", article.ID, err)
	}
	slug := intercomSlug(article.URL, article.Title)
	hcArticle, err := s.helpcenterSvc.CreateArticle(ctx, &model.DocsHelpcenterArticle{DocumentID: document.ID, Slug: slug})
	if err != nil {
		return nil, fmt.Errorf("create help center article for %s: %w", article.ID, err)
	}
	stats := summarizeImportWarnings(warnings)
	stats.Uncategorized = collection == nil
	if sourcePath := intercomURLPath(article.URL); sourcePath != "" && collection != nil && hcArticle != nil {
		canonical := buildDocsHelpcenterArticleCanonicalPath(nil, "", hcArticle.Slug, hcArticle.PublicID)
		articleSlug := hcArticle.Slug
		created, err := s.redirectRepo.UpsertImported(ctx, &model.DocsRedirect{
			WorkspaceID: workspaceID, SourcePath: sourcePath, TargetCollectionSlug: collection.slug, TargetArticleSlug: &articleSlug,
			TargetPath: &canonical, Type: model.RedirectTypeImported, SourceSystem: stringPtr(docsImportSourceIntercom),
			SourceObjectType: stringPtr("article"), SourceObjectID: stringPtr(article.ID),
		})
		if err != nil {
			s.logger.Error("create article redirect", "error", err, "article_id", article.ID)
		} else {
			stats.RedirectCreated = created
		}
	}
	publish := mode == "published" || (mode == "match_source" && article.State == "published")
	if publish {
		if _, err := s.documentSvc.Publish(ctx, document.ID); err != nil {
			return nil, fmt.Errorf("publish imported article internally %s: %w", article.ID, err)
		}
		externalSlug := slug
		if hcArticle != nil {
			externalSlug = hcArticle.Slug
		}
		if err := s.helpcenterSvc.PublishExternally(ctx, document.ID, externalSlug, nil); err != nil {
			return nil, fmt.Errorf("publish imported article externally %s: %w", article.ID, err)
		}
		stats.Published = true
	}
	// The provenance is the resume checkpoint; store it last.
	if err := s.contentSvc.SetImportProvenance(ctx, saved.ID, html, docsImportSourceIntercom, article.ID); err != nil {
		return nil, fmt.Errorf("store import checkpoint for article %s: %w", article.ID, err)
	}
	return stats, nil
}
