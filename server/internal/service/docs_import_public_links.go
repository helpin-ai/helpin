package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// publicImportLinkedTargetKey keys the collection for articles found through
// links in the run's target map. It can never collide with a source ID,
// which is a number.
const publicImportLinkedTargetKey = "linked"

// publicImportContentBatch bounds one read of stored article bodies.
const publicImportContentBatch = 100

// importLinkedArticles finds public articles that sit in no collection.
// Intercom lets an article be public without one; such an article is then
// reachable only through links from other articles. The stored bodies are
// scanned rather than the source, so a retried run finds the same articles.
func (r *publicImportRun) importLinkedArticles(ctx context.Context) error {
	r.countEarlierLinkedArticles(ctx)
	tried := map[string]bool{}
	queue, err := r.linkedArticleIDs(ctx, r.documentIDs())
	if err != nil {
		return err
	}
	lookups := 0
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		id := queue[0]
		queue = queue[1:]
		if tried[id] || r.documents[id] != "" {
			continue
		}
		tried[id] = true
		if lookups++; lookups > publicImportMaxLinkedLookups {
			r.s.logger.WarnContext(ctx, "stopped following article links", "job_id", r.jobID, "lookups", lookups-1)
			break
		}
		documentID, err := r.importLinkedArticle(ctx, id)
		if err != nil {
			return err
		}
		r.beat()
		if documentID == "" {
			continue
		}
		found, err := r.linkedArticleIDs(ctx, []string{documentID})
		if err != nil {
			return err
		}
		queue = append(queue, found...)
	}
	r.reportProgress(ctx, true)
	return nil
}

// countEarlierLinkedArticles adds the articles an earlier run found through
// links to this run's counts and order, so a retried import reports the same
// totals as the first and keeps those articles in place.
//
// Only articles that sit in the "Other articles" collection count. A space
// can also hold articles from another help center, or ones the source has
// since removed; those belong to no part of this run.
func (r *publicImportRun) countEarlierLinkedArticles(ctx context.Context) {
	target, found, err := r.existingLinkedTarget(ctx)
	if err != nil {
		r.s.logger.WarnContext(ctx, "find collection of linked articles", "job_id", r.jobID, "error", err)
		return
	}
	if !found {
		return
	}
	// Known from here on, so the collection keeps its place in the order.
	r.targets[publicImportLinkedTargetKey] = target
	listed := make(map[string]bool, len(r.order))
	for _, sourceID := range r.order {
		listed[sourceID] = true
	}
	var earlier []string
	for sourceID, documentID := range r.documents {
		if !listed[sourceID] && r.documentCollections[documentID] == target.id {
			earlier = append(earlier, sourceID)
		}
	}
	if len(earlier) == 0 {
		return
	}
	slices.Sort(earlier)
	r.order = append(r.order, earlier...)
	r.linked = append(r.linked, earlier...)
	r.total += len(earlier)
	r.completed += len(earlier)
	if err := r.s.importRepo.SetTotal(ctx, r.jobID, r.total); err != nil {
		r.s.logger.ErrorContext(ctx, "set import total", "job_id", r.jobID, "error", err)
	}
}

// importLinkedArticle imports the article with the given source ID when the
// help center still serves it, and returns its document ID or "".
func (r *publicImportRun) importLinkedArticle(ctx context.Context, id string) (string, error) {
	article, err := r.client.Article(ctx, r.site.ArticleURL(id))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if errors.Is(err, intercom.ErrPublicHelpCenterUnreachable) {
			// A link that might lead to an article could not be checked;
			// say so rather than drop it silently.
			r.total++
			r.failed++
			r.failures = append(r.failures, model.ImportFailure{ArticleID: id, Title: "Article " + id, Error: publicArticleFailure(err)})
		}
		// Anything else is a dead link at the source. The link pass reports it.
		return "", nil
	}
	target, err := r.linkedTarget(ctx)
	if err != nil {
		return "", err
	}
	r.total++
	if err := r.s.importRepo.SetTotal(ctx, r.jobID, r.total); err != nil {
		r.s.logger.ErrorContext(ctx, "set import total", "job_id", r.jobID, "error", err)
	}
	summary := intercom.PublicArticleSummary{ID: id, Title: article.Title, URL: article.URL, Summary: article.Description}
	r.importArticle(ctx, summary, article, target, true)
	if documentID := r.documents[id]; documentID != "" {
		r.linked = append(r.linked, id)
		return documentID, nil
	}
	return "", nil
}

// linkedCollectionEntry describes the collection for articles found through
// links, and the space's collections it must be matched against.
func (r *publicImportRun) linkedCollectionEntry(ctx context.Context) (entry publicImportCollection, existing []model.DocsCollection, slugs, claimed map[string]bool, err error) {
	existing, err = r.s.collectionSvc.List(ctx, r.spaceID)
	if err != nil {
		return entry, nil, nil, nil, fmt.Errorf("list collections: %w", err)
	}
	slugs = make(map[string]bool, len(existing))
	claimed = map[string]bool{}
	for _, collection := range existing {
		slugs[collection.Slug] = true
	}
	for _, target := range r.targets {
		claimed[target.id] = true
	}
	entry = publicImportCollection{
		source: intercom.PublicCollection{
			ID: publicImportLinkedTargetKey, Name: publicImportLinkedCollectionName, Slug: importSlug(publicImportLinkedCollectionName),
		},
		targetID: publicImportLinkedTargetKey,
	}
	return entry, existing, slugs, claimed, nil
}

// existingLinkedTarget finds the "Other articles" collection an earlier run
// made, without creating one.
func (r *publicImportRun) existingLinkedTarget(ctx context.Context) (publicImportTarget, bool, error) {
	if target, ok := r.targets[publicImportLinkedTargetKey]; ok {
		return target, true, nil
	}
	entry, existing, _, claimed, err := r.linkedCollectionEntry(ctx)
	if err != nil {
		return publicImportTarget{}, false, err
	}
	collection, ok := matchIntercomCollection(existing, claimed, entry.source.Slug, entry.source.Name, nil)
	if !ok {
		return publicImportTarget{}, false, nil
	}
	return publicImportTarget{id: collection.ID, slug: collection.Slug, publicID: collection.PublicID}, true, nil
}

// linkedTarget returns the collection for articles found through links,
// creating it the first time one is found.
func (r *publicImportRun) linkedTarget(ctx context.Context) (*publicImportTarget, error) {
	if target, ok := r.targets[publicImportLinkedTargetKey]; ok {
		return &target, nil
	}
	entry, existing, slugs, claimed, err := r.linkedCollectionEntry(ctx)
	if err != nil {
		return nil, err
	}
	target, created, err := r.ensureCollection(ctx, &existing, slugs, claimed, entry)
	if err != nil {
		return nil, err
	}
	if created {
		r.summary.CollectionsCreated++
	}
	r.targets[publicImportLinkedTargetKey] = target
	return &target, nil
}

// documentIDs lists every imported document: this run's articles in source
// order, then the ones only earlier runs knew.
func (r *publicImportRun) documentIDs() []string {
	ids := make([]string, 0, len(r.documents))
	seen := make(map[string]bool, len(r.documents))
	add := func(documentID string) {
		if documentID != "" && !seen[documentID] {
			seen[documentID] = true
			ids = append(ids, documentID)
		}
	}
	for _, sourceID := range r.order {
		add(r.documents[sourceID])
	}
	rest := make([]string, 0, len(r.documents))
	for _, documentID := range r.documents {
		if !seen[documentID] {
			rest = append(rest, documentID)
		}
	}
	slices.Sort(rest)
	for _, documentID := range rest {
		add(documentID)
	}
	return ids
}

// eachContent calls visit with the stored body of each document, in the
// order given.
func (r *publicImportRun) eachContent(ctx context.Context, documentIDs []string, visit func(model.DocsContent) error) error {
	for chunk := range slices.Chunk(documentIDs, publicImportContentBatch) {
		if err := ctx.Err(); err != nil {
			return err
		}
		contents, err := r.s.contentSvc.contentRepo.ListByDocumentIDs(ctx, chunk)
		if err != nil {
			return fmt.Errorf("read imported articles: %w", err)
		}
		byDocument := make(map[string]model.DocsContent, len(contents))
		for _, content := range contents {
			byDocument[content.DocumentID] = content
		}
		for _, documentID := range chunk {
			content, ok := byDocument[documentID]
			if !ok {
				continue
			}
			if err := visit(content); err != nil {
				return err
			}
		}
		r.beat()
	}
	return nil
}

// linkedArticleIDs returns the source IDs of articles the given documents
// link to that have not been imported, in the order they appear.
func (r *publicImportRun) linkedArticleIDs(ctx context.Context, documentIDs []string) ([]string, error) {
	var found []string
	seen := map[string]bool{}
	err := r.eachContent(ctx, documentIDs, func(content model.DocsContent) error {
		_, _, err := docsimport.RewriteTiptapLinks(content.Content, func(href string) (string, bool) {
			target, ok := r.site.LinkTarget(href)
			if ok && target.Kind == intercom.PublicLinkArticle && r.documents[target.ID] == "" && !seen[target.ID] {
				seen[target.ID] = true
				found = append(found, target.ID)
			}
			return "", false
		})
		if err != nil {
			r.s.logger.WarnContext(ctx, "scan imported article for links", "job_id", r.jobID, "document_id", content.DocumentID, "error", err)
		}
		return nil
	})
	return found, err
}

// publicImportLinkedArticle is where an imported article now lives.
type publicImportLinkedArticle struct {
	path, slug string
	published  bool
}

// rewriteLinks points every link between imported articles and collections
// at its Helpin address, keeping "#section" anchors, and republishes the
// articles it changed. Links to the source that match nothing imported are
// reported. Rewritten links are relative, so running this again changes
// nothing.
func (r *publicImportRun) rewriteLinks(ctx context.Context) error {
	articles := make(map[string]publicImportLinkedArticle, len(r.documents))
	byDocument := make(map[string]publicImportLinkedArticle, len(r.documents))
	for sourceID, documentID := range r.documents {
		if err := ctx.Err(); err != nil {
			return err
		}
		hcArticle, err := r.s.helpcenterSvc.GetArticle(ctx, documentID)
		if err != nil {
			return fmt.Errorf("read help center article: %w", err)
		}
		if hcArticle == nil || hcArticle.Slug == "" {
			continue
		}
		linked := publicImportLinkedArticle{
			path:      buildDocsHelpcenterArticleCanonicalPath(nil, "", hcArticle.Slug, hcArticle.PublicID),
			slug:      hcArticle.Slug,
			published: hcArticle.PublicPublishedAt != nil,
		}
		articles[sourceID] = linked
		byDocument[documentID] = linked
	}
	collections := make(map[string]string, len(r.collections))
	for _, entry := range r.collections {
		if target, ok := r.targets[entry.targetID]; ok {
			collections[entry.source.ID] = buildDocsHelpcenterCollectionCanonicalPath(nil, "", target.slug, target.publicID)
		}
	}
	return r.eachContent(ctx, r.documentIDs(), func(content model.DocsContent) error {
		return r.rewriteDocumentLinks(ctx, content, articles, collections, byDocument[content.DocumentID])
	})
}

func (r *publicImportRun) rewriteDocumentLinks(
	ctx context.Context,
	content model.DocsContent,
	articles map[string]publicImportLinkedArticle,
	collections map[string]string,
	self publicImportLinkedArticle,
) error {
	var unresolved []string
	rewritten, changed, err := docsimport.RewriteTiptapLinks(content.Content, func(href string) (string, bool) {
		target, ok := r.site.LinkTarget(href)
		if !ok {
			return "", false
		}
		path := collections[target.ID]
		if target.Kind == intercom.PublicLinkArticle {
			path = articles[target.ID].path
		}
		if path == "" {
			// A link on Intercom's shared host may belong to another
			// company's help center; only report the ones that are ours.
			if target.Certain && !slices.Contains(unresolved, href) {
				unresolved = append(unresolved, href)
			}
			return "", false
		}
		if target.Fragment != "" {
			path += "#" + target.Fragment
		}
		return path, true
	})
	if err != nil {
		r.s.logger.WarnContext(ctx, "rewrite links in imported article", "job_id", r.jobID, "document_id", content.DocumentID, "error", err)
		return nil
	}
	for _, href := range unresolved {
		r.summary.UnresolvedLinksTotal++
		if len(r.summary.UnresolvedLinks) < model.ImportReportListLimit {
			r.summary.UnresolvedLinks = append(r.summary.UnresolvedLinks, model.ImportUnresolvedLink{
				DocumentID: content.DocumentID, ArticleTitle: r.titles[content.DocumentID], URL: href,
			})
		}
	}
	if changed == 0 {
		return nil
	}
	if _, err := r.s.contentSvc.Save(ctx, content.DocumentID, rewritten, r.actor); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r.s.logger.ErrorContext(ctx, "save rewritten links", "job_id", r.jobID, "document_id", content.DocumentID, "error", err)
		return nil
	}
	r.summary.LinksRewritten += changed
	if !self.published {
		return nil
	}
	if err := r.s.helpcenterSvc.PublishExternally(ctx, content.DocumentID, self.slug, nil); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r.s.logger.ErrorContext(ctx, "republish article after link rewrite", "job_id", r.jobID, "document_id", content.DocumentID, "error", err)
	}
	return nil
}

// restoreOrder puts collections and articles in the order the source help
// center shows them. Creation order alone is not enough: a retried run
// creates the remaining articles after ones a later collection already has.
// A failure here leaves a complete import in a different order, so it is
// logged and the import still finishes.
func (r *publicImportRun) restoreOrder(ctx context.Context) error {
	var top []string
	articlesByTarget := map[string][]model.ReorderDocsChildItem{}
	childrenByParent := map[string][]model.ReorderDocsChildItem{}
	for _, entry := range r.collections {
		for _, summary := range entry.source.Articles {
			if documentID := r.documents[summary.ID]; documentID != "" {
				articlesByTarget[entry.targetID] = append(articlesByTarget[entry.targetID], model.ReorderDocsChildItem{Kind: "article", ID: documentID})
			}
		}
		if entry.merged() {
			continue
		}
		target, ok := r.targets[entry.source.ID]
		if !ok {
			continue
		}
		if entry.parentID == "" {
			top = append(top, target.id)
			continue
		}
		childrenByParent[entry.parentID] = append(childrenByParent[entry.parentID], model.ReorderDocsChildItem{Kind: "collection", ID: target.id})
	}
	for _, sourceID := range r.linked {
		articlesByTarget[publicImportLinkedTargetKey] = append(articlesByTarget[publicImportLinkedTargetKey],
			model.ReorderDocsChildItem{Kind: "article", ID: r.documents[sourceID]})
	}
	if linked, ok := r.targets[publicImportLinkedTargetKey]; ok {
		top = append(top, linked.id)
	}
	// The top level is reordered only when every collection there came from
	// this import. In a space that already had its own collections, a partial
	// reorder would shuffle them in with the imported ones; the imported
	// collections then simply follow in the order they were created.
	if r.ownsTopLevel(ctx, top) {
		if err := r.s.collectionSvc.ReorderCollections(ctx, r.spaceID, model.ReorderDocsCollectionsRequest{CollectionIDs: top}); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.s.logger.WarnContext(ctx, "restore collection order", "job_id", r.jobID, "error", err)
		}
	}
	for sourceID, target := range r.targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		// An Intercom collection lists its articles, then its subcollections.
		items := slices.Concat(articlesByTarget[sourceID], childrenByParent[sourceID])
		if len(items) == 0 {
			continue
		}
		parentID := target.id
		if err := r.s.collectionSvc.ReorderChildren(ctx, r.spaceID, model.ReorderDocsChildrenRequest{ParentCollectionID: &parentID, Items: items}); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.s.logger.WarnContext(ctx, "restore article order", "job_id", r.jobID, "collection_id", target.id, "error", err)
		}
		r.beat()
	}
	return nil
}

// ownsTopLevel reports whether every top-level collection of the space is one
// of the given imported collections.
func (r *publicImportRun) ownsTopLevel(ctx context.Context, imported []string) bool {
	collections, err := r.s.collectionSvc.List(ctx, r.spaceID)
	if err != nil {
		r.s.logger.WarnContext(ctx, "list collections before reorder", "job_id", r.jobID, "error", err)
		return false
	}
	for _, collection := range collections {
		if collection.ParentCollectionID == nil && !slices.Contains(imported, collection.ID) {
			return false
		}
	}
	return true
}
