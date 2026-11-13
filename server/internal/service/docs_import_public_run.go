package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const (
	// publicImportLinkedCollectionName holds public articles that sit in no
	// source collection and are reachable only through links.
	publicImportLinkedCollectionName = "Other articles"
	// publicImportMaxLinkedLookups bounds how many linked article numbers one
	// import follows, so a help center full of stale links cannot stall it.
	publicImportMaxLinkedLookups = 1000
	publicImportProgressEvery    = 5
)

// publicImportTarget is the Helpin collection a source collection became.
type publicImportTarget struct {
	id, slug, publicID string
}

// publicImportRun carries one execution of a public help center import. Every
// step reads what earlier runs stored, so a retried run continues where the
// last one stopped.
type publicImportRun struct {
	s                                  *DocsImportService
	jobID, spaceID, workspaceID, actor string
	mode                               string
	client                             *intercom.PublicClient
	site                               *intercom.PublicHelpCenter
	uploader                           helpscout.ImageUploader
	heartbeat                          func(int)

	collections []publicImportCollection
	// targets maps a source collection ID to its Helpin collection.
	targets map[string]publicImportTarget
	// documents maps a source article ID to its Helpin document.
	documents map[string]string
	// documentCollections maps a checkpointed document to its collection.
	documentCollections map[string]string
	// titles names documents in the report.
	titles map[string]string
	// order lists source article IDs as this run met them, for stable reports.
	order []string
	// linked lists the article IDs found through links, in discovery order.
	linked     []string
	imageCache map[string]string

	summary           model.ImportSummary
	failures          []model.ImportFailure
	total             int
	completed, failed int
}

// runPublicIntercomImport imports a public Intercom help center: collections,
// articles with their images, links between them, and the source order.
func (s *DocsImportService) runPublicIntercomImport(ctx context.Context, jobID string, req model.DocsImportStartRequest, spaceID, workspaceID, userID string, heartbeat func(int)) error {
	if err := s.importRepo.UpdateStatus(ctx, jobID, model.DocsImportStatusRunning, nil); err != nil {
		return err
	}
	run := &publicImportRun{
		s: s, jobID: jobID, spaceID: spaceID, workspaceID: workspaceID, actor: userID,
		mode:       normalizeDocsImportStatus(req.ImportStatus),
		client:     s.publicIntercomClient(),
		heartbeat:  heartbeat,
		targets:    map[string]publicImportTarget{},
		documents:  map[string]string{},
		titles:     map[string]string{},
		imageCache: map[string]string{},
	}
	switch {
	case s.imageUploader != nil:
		run.uploader = s.imageUploader
	case s.s3Client != nil:
		run.uploader = &s3ImageUploader{store: s.s3Client}
	}
	steps := []func(context.Context) error{
		func(ctx context.Context) error { return run.loadSite(ctx, req.SourceURL) },
		run.importCollections,
		run.loadCheckpoints,
		run.importListedArticles,
		run.importLinkedArticles,
		run.rewriteLinks,
		run.restoreOrder,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var fatal *publicImportFatalError
			if errors.As(err, &fatal) {
				s.failJob(ctx, jobID, fatal.message)
				return nil
			}
			return err
		}
	}
	run.finish(ctx)
	return nil
}

// publicImportFatalError ends the job with a message for the person who
// started it. Retrying the run would not help.
type publicImportFatalError struct{ message string }

func (e *publicImportFatalError) Error() string { return e.message }

func publicImportFatal(format string, args ...any) error {
	return &publicImportFatalError{message: fmt.Sprintf(format, args...)}
}

func (r *publicImportRun) beat() {
	if r.heartbeat != nil {
		r.heartbeat(r.completed + r.failed)
	}
}

func (r *publicImportRun) loadSite(ctx context.Context, sourceURL string) error {
	site, err := r.client.LoadHelpCenter(ctx, sourceURL, r.beat)
	if err != nil {
		friendly := publicImportError(err)
		if errors.Is(friendly, ErrPublicImportInvalid) {
			return publicImportFatal("%s", strings.TrimPrefix(friendly.Error(), ErrPublicImportInvalid.Error()+": "))
		}
		return fmt.Errorf("read help center: %w", err)
	}
	r.site = site
	r.collections = flattenPublicCollections(site.Collections)
	for _, entry := range r.collections {
		r.total += len(entry.source.Articles)
		if entry.merged() {
			r.summary.CollectionsMerged++
		}
	}
	r.summary.SourceSystem = docsImportSourceIntercomPublic
	r.summary.SourceURL = site.BaseURL
	r.summary.SourceName = site.Name
	if err := r.s.importRepo.SetTotal(ctx, r.jobID, r.total); err != nil {
		r.s.logger.ErrorContext(ctx, "set import total", "job_id", r.jobID, "error", err)
	}
	return nil
}

// importCollections creates the Helpin collection for every source
// collection that fits Helpin's nesting, reusing the ones an earlier run made.
func (r *publicImportRun) importCollections(ctx context.Context) error {
	existing, err := r.s.collectionSvc.List(ctx, r.spaceID)
	if err != nil {
		return publicImportFatal("The space's collections could not be read. Try the import again.")
	}
	slugs := make(map[string]bool, len(existing))
	for _, collection := range existing {
		slugs[collection.Slug] = true
	}
	claimed := map[string]bool{}
	for _, entry := range r.collections {
		if entry.merged() {
			continue
		}
		target, created, err := r.ensureCollection(ctx, &existing, slugs, claimed, entry)
		if err != nil {
			return err
		}
		if created {
			r.summary.CollectionsCreated++
		}
		r.targets[entry.source.ID] = target
		r.redirectCollection(ctx, entry.source, target)
		r.beat()
	}
	return nil
}

func (r *publicImportRun) ensureCollection(
	ctx context.Context,
	existing *[]model.DocsCollection,
	slugs, claimed map[string]bool,
	entry publicImportCollection,
) (publicImportTarget, bool, error) {
	var parentID *string
	if parent, ok := r.targets[entry.parentID]; ok {
		parentID = &parent.id
	}
	slug := importSlug(entry.source.Slug)
	if slug == "" {
		slug = intercomSlug(entry.source.URL, entry.source.Name)
	}
	if collection, ok := matchIntercomCollection(*existing, claimed, slug, entry.source.Name, parentID); ok {
		claimed[collection.ID] = true
		return publicImportTarget{id: collection.ID, slug: collection.Slug, publicID: collection.PublicID}, false, nil
	}
	slug = uniqueImportSlug(slug, slugs)
	var description *string
	if entry.source.Description != "" {
		description = &entry.source.Description
	}
	created, err := r.s.collectionSvc.Create(ctx, r.workspaceID, r.spaceID, model.CreateDocsCollectionRequest{
		Name: entry.source.Name, Slug: &slug, Description: description, ParentCollectionID: parentID,
	}, r.actor)
	if err != nil {
		if ctx.Err() != nil {
			return publicImportTarget{}, false, ctx.Err()
		}
		r.s.logger.ErrorContext(ctx, "create imported collection", "job_id", r.jobID, "collection", entry.source.Name, "error", err)
		return publicImportTarget{}, false, publicImportFatal("The collection %q could not be created. Try the import again.", entry.source.Name)
	}
	*existing = append(*existing, *created)
	slugs[created.Slug] = true
	claimed[created.ID] = true
	return publicImportTarget{id: created.ID, slug: created.Slug, publicID: created.PublicID}, true, nil
}

func (r *publicImportRun) redirectCollection(ctx context.Context, source intercom.PublicCollection, target publicImportTarget) {
	sourcePath := intercomURLPath(source.URL)
	if sourcePath == "" {
		return
	}
	canonical := buildDocsHelpcenterCollectionCanonicalPath(nil, "", target.slug, target.publicID)
	created, err := r.s.redirectRepo.UpsertImported(ctx, &model.DocsRedirect{
		WorkspaceID: r.workspaceID, SourcePath: sourcePath, TargetCollectionSlug: target.slug, TargetPath: &canonical,
		Type: model.RedirectTypeImported, SourceSystem: stringPtr(docsImportSourceIntercomPublic),
		SourceObjectType: stringPtr("collection"), SourceObjectID: stringPtr(source.ID),
	})
	if err != nil {
		r.s.logger.ErrorContext(ctx, "create collection redirect", "job_id", r.jobID, "collection_id", source.ID, "error", err)
		return
	}
	if created {
		r.summary.RedirectsCreated++
	}
}

// loadCheckpoints reads the articles earlier runs finished.
func (r *publicImportRun) loadCheckpoints(ctx context.Context) error {
	rows, err := r.s.contentSvc.contentRepo.ListImportSourceDocuments(ctx, r.spaceID, docsImportSourceIntercomPublic)
	if err != nil {
		return fmt.Errorf("load import checkpoints: %w", err)
	}
	r.documentCollections = make(map[string]string, len(rows))
	for _, row := range rows {
		r.documents[row.SourceObjectID] = row.DocumentID
		r.documentCollections[row.DocumentID] = row.CollectionID
	}
	return nil
}

// importListedArticles imports the articles that collections list.
func (r *publicImportRun) importListedArticles(ctx context.Context) error {
	seen := 0
	for _, entry := range r.collections {
		target, ok := r.targets[entry.targetID]
		if !ok {
			continue
		}
		for _, summary := range entry.source.Articles {
			if err := ctx.Err(); err != nil {
				return err
			}
			seen++
			r.importArticle(ctx, summary, nil, &target, false)
			r.reportProgress(ctx, seen%publicImportProgressEvery == 0)
		}
	}
	r.reportProgress(ctx, true)
	return nil
}

// importArticle imports one article unless an earlier run already did, and
// counts the outcome.
func (r *publicImportRun) importArticle(ctx context.Context, summary intercom.PublicArticleSummary, fetched *intercom.PublicArticle, target *publicImportTarget, linked bool) {
	r.order = append(r.order, summary.ID)
	if documentID, done := r.documents[summary.ID]; done {
		r.completed++
		r.titles[documentID] = summary.Title
		r.beat()
		return
	}
	documentID, err := r.createArticle(ctx, summary, fetched, target)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		r.failed++
		r.failures = append(r.failures, model.ImportFailure{ArticleID: summary.ID, Title: summary.Title, Error: publicArticleFailure(err)})
		r.s.logger.WarnContext(ctx, "import public article", "job_id", r.jobID, "article_id", summary.ID, "error", err)
		r.beat()
		return
	}
	r.completed++
	r.documents[summary.ID] = documentID
	if linked {
		r.summary.ArticlesOutsideCollections++
	}
	r.beat()
}

// publicArticleFailure words an article failure for the import report.
func publicArticleFailure(err error) string {
	switch {
	case errors.Is(err, intercom.ErrNotFound):
		return "The article is no longer available on the source help center."
	case errors.Is(err, intercom.ErrPublicHelpCenterRestricted):
		return "The article requires sign-in on the source help center."
	case errors.Is(err, intercom.ErrPublicHelpCenterUnreachable):
		return "The source help center did not respond while this article was read. Retry to import it."
	}
	var invalid *publicArticleContentError
	if errors.As(err, &invalid) {
		return "The article's content could not be converted: " + invalid.reason
	}
	return "The article could not be saved. Retry to import it."
}

// publicArticleContentError means an article converted to content Helpin
// does not accept.
type publicArticleContentError struct{ reason string }

func (e *publicArticleContentError) Error() string { return "invalid converted content: " + e.reason }

// createArticle converts and stores one article. fetched is the article when
// the caller already read it; otherwise it is read from summary.URL.
func (r *publicImportRun) createArticle(ctx context.Context, summary intercom.PublicArticleSummary, fetched *intercom.PublicArticle, target *publicImportTarget) (string, error) {
	article := fetched
	if article == nil {
		read, err := r.client.Article(ctx, summary.URL)
		if err != nil {
			return "", err
		}
		article = read
	}
	var failedImages []model.ImportFailedImage
	converted := docsimport.ConvertIntercomBlocks(article.Blocks, docsimport.IntercomBlockOptions{
		CopyImage: func(source string) string { return r.copyImage(ctx, source, article.Title, &failedImages) },
	})
	content, err := converted.Doc.ToJSON()
	if err != nil {
		return "", fmt.Errorf("encode article %s: %w", article.ID, err)
	}
	if err := tiptap.ValidateDocument(content); err != nil {
		return "", &publicArticleContentError{reason: err.Error()}
	}
	var collectionID *string
	if target != nil {
		collectionID = &target.id
	}
	document, err := r.s.documentSvc.Create(ctx, r.workspaceID, model.CreateDocsDocumentRequest{
		SpaceID: r.spaceID, CollectionID: collectionID, Title: article.Title,
	}, r.actor)
	if err != nil {
		return "", fmt.Errorf("create document for article %s: %w", article.ID, err)
	}
	excerpt := article.Description
	if excerpt == "" {
		excerpt = summary.Summary
	}
	published, err := r.fillArticle(ctx, summary.ID, article, target, document.ID, content, excerpt)
	if err != nil {
		// Without its checkpoint the article is imported again on the next
		// run, so remove the partial document rather than leave a duplicate.
		if deleteErr := r.s.documentSvc.Delete(context.WithoutCancel(ctx), document.ID); deleteErr != nil {
			r.s.logger.ErrorContext(ctx, "delete partially imported article", "article_id", article.ID, "document_id", document.ID, "error", deleteErr)
		}
		return "", err
	}
	r.titles[document.ID] = article.Title
	if published {
		r.summary.ArticlesPublished++
	} else {
		r.summary.ArticlesDrafted++
	}
	if len(converted.Warnings) > 0 {
		r.summary.ArticlesWithConversionWarnings++
	}
	for _, image := range failedImages {
		image.DocumentID = document.ID
		r.addFailedImage(image)
	}
	return document.ID, nil
}

// fillArticle writes an article's content, excerpt, help center entry,
// redirect, and publication, and stores its checkpoint last.
//
// sourceID is the number in the article's address. It is the checkpoint key
// because it is what collection listings and links carry; the ID inside the
// article data is not guaranteed to be the same number.
func (r *publicImportRun) fillArticle(
	ctx context.Context,
	sourceID string,
	article *intercom.PublicArticle,
	target *publicImportTarget,
	documentID string,
	content json.RawMessage,
	excerpt string,
) (bool, error) {
	saved, err := r.s.contentSvc.Save(ctx, documentID, content, r.actor)
	if err != nil {
		return false, fmt.Errorf("save content for article %s: %w", article.ID, err)
	}
	if excerpt != "" {
		if _, err := r.s.documentSvc.Update(ctx, documentID, model.UpdateDocsDocumentRequest{Excerpt: &excerpt}); err != nil {
			return false, fmt.Errorf("set excerpt for article %s: %w", article.ID, err)
		}
	}
	slug := intercomSlug(article.URL, article.Title)
	hcArticle, err := r.s.helpcenterSvc.CreateArticle(ctx, &model.DocsHelpcenterArticle{DocumentID: documentID, Slug: slug})
	if err != nil {
		return false, fmt.Errorf("create help center article for %s: %w", article.ID, err)
	}
	if hcArticle != nil {
		slug = hcArticle.Slug
	}
	if sourcePath := intercomURLPath(article.URL); sourcePath != "" && target != nil && hcArticle != nil {
		canonical := buildDocsHelpcenterArticleCanonicalPath(nil, "", hcArticle.Slug, hcArticle.PublicID)
		created, err := r.s.redirectRepo.UpsertImported(ctx, &model.DocsRedirect{
			WorkspaceID: r.workspaceID, SourcePath: sourcePath, TargetCollectionSlug: target.slug, TargetArticleSlug: &slug,
			TargetPath: &canonical, Type: model.RedirectTypeImported, SourceSystem: stringPtr(docsImportSourceIntercomPublic),
			SourceObjectType: stringPtr("article"), SourceObjectID: stringPtr(sourceID),
		})
		if err != nil {
			r.s.logger.ErrorContext(ctx, "create article redirect", "job_id", r.jobID, "article_id", sourceID, "error", err)
		} else if created {
			r.summary.RedirectsCreated++
		}
	}
	// Everything a public help center shows is published at the source, so
	// "match source" publishes too.
	publish := r.mode != "draft"
	if publish {
		if _, err := r.s.documentSvc.Publish(ctx, documentID); err != nil {
			return false, fmt.Errorf("publish imported article internally %s: %w", article.ID, err)
		}
		if err := r.s.helpcenterSvc.PublishExternally(ctx, documentID, slug, nil); err != nil {
			return false, fmt.Errorf("publish imported article externally %s: %w", article.ID, err)
		}
	}
	// The provenance is the resume checkpoint; store it last. No source HTML
	// is kept: the article was converted from blocks, not HTML.
	if err := r.s.contentSvc.SetImportProvenance(ctx, saved.ID, "", docsImportSourceIntercomPublic, sourceID); err != nil {
		return false, fmt.Errorf("store import checkpoint for article %s: %w", article.ID, err)
	}
	return publish, nil
}

// copyImage stores an image once per import and returns its new address, or
// "" when it could not be copied.
func (r *publicImportRun) copyImage(ctx context.Context, source, articleTitle string, failed *[]model.ImportFailedImage) string {
	key := imageCacheKey(source)
	if stored, ok := r.imageCache[key]; ok {
		return stored
	}
	fail := func(reason string) string {
		*failed = append(*failed, model.ImportFailedImage{ArticleTitle: articleTitle, URL: stripURLQuery(source), Reason: reason})
		return ""
	}
	if r.uploader == nil {
		return fail("File storage is not set up, so images stay on the source help center.")
	}
	// An article with many large images can take minutes; say the import is
	// alive before each one.
	r.beat()
	stored, err := r.s.importExternalImageWithUploader(ctx, r.workspaceID, source, r.uploader)
	if err != nil {
		r.s.logger.WarnContext(ctx, "copy imported image", "job_id", r.jobID, "image_host", hostOfURL(source), "error", err)
		return fail(publicImageFailure(err))
	}
	r.imageCache[key] = stored
	r.summary.ImagesCopied++
	return stored
}

func publicImageFailure(err error) string {
	switch {
	case errors.Is(err, helpscout.ErrNotImage):
		return "The address did not return an image."
	case errors.Is(err, helpscout.ErrImageTooLarge):
		return "The image is larger than 25 MB."
	}
	return "The image could not be downloaded."
}

func (r *publicImportRun) addFailedImage(image model.ImportFailedImage) {
	r.summary.FailedImagesTotal++
	r.summary.ImageRewriteFailures++
	if len(r.summary.FailedImages) < model.ImportReportListLimit {
		r.summary.FailedImages = append(r.summary.FailedImages, image)
	}
}

// imageCacheKey identifies an image without the signature Intercom's CDN adds
// to every address, which differs between two reads of the same image.
func imageCacheKey(source string) string { return stripURLQuery(source) }

func stripURLQuery(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.RawQuery, parsed.Fragment = "", ""
	return parsed.String()
}

func hostOfURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// reportProgress stores the running counts so the import screen can follow
// along.
func (r *publicImportRun) reportProgress(ctx context.Context, write bool) {
	if !write {
		return
	}
	failures := json.RawMessage("[]")
	if len(r.failures) > 0 {
		encoded, err := json.Marshal(r.failures)
		if err != nil {
			r.s.logger.ErrorContext(ctx, "encode import failures", "job_id", r.jobID, "error", err)
			return
		}
		failures = encoded
	}
	if err := r.s.importRepo.UpdateProgress(ctx, r.jobID, r.completed, r.failed, failures); err != nil {
		r.s.logger.ErrorContext(ctx, "update import progress", "job_id", r.jobID, "error", err)
	}
}

// finish stores the report and closes the job.
func (r *publicImportRun) finish(ctx context.Context) {
	r.reportProgress(ctx, true)
	summary, err := json.Marshal(r.summary)
	if err != nil {
		r.s.logger.ErrorContext(ctx, "encode import summary", "job_id", r.jobID, "error", err)
	} else if err := r.s.importRepo.SetSummary(ctx, r.jobID, summary); err != nil {
		r.s.logger.ErrorContext(ctx, "set import summary", "job_id", r.jobID, "error", err)
	}
	status := model.DocsImportStatusDone
	if r.failed > 0 && r.completed == 0 {
		status = model.DocsImportStatusFailed
	}
	doneAt := time.Now()
	if err := r.s.importRepo.UpdateStatus(ctx, r.jobID, status, &doneAt); err != nil {
		r.s.logger.ErrorContext(ctx, "close import job", "job_id", r.jobID, "error", err)
	}
	r.s.logger.InfoContext(ctx, "public help center import finished",
		"job_id", r.jobID, "workspace_id", r.workspaceID, "status", status,
		"completed", r.completed, "failed", r.failed,
		"links_rewritten", r.summary.LinksRewritten, "links_unresolved", r.summary.UnresolvedLinksTotal,
		"images_failed", r.summary.FailedImagesTotal)
}
