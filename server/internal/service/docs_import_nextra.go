package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const (
	// maxNextraUncompressedSize is the maximum allowed uncompressed
	// size of a Nextra archive (500 MB).
	maxNextraUncompressedSize = 500 * 1024 * 1024
)

// PreviewNextra reads a Nextra zip archive, parses it into an import
// plan, stores the archive in S3, creates a pending import job, and
// returns a preview response. No docs are created during preview.
func (s *DocsImportService) PreviewNextra(ctx context.Context, workspaceID, userID, archiveName, sourceCommit string, archive io.Reader, size int64) (*model.DocsNextraImportPreviewResponse, error) {
	if s.s3Client == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}

	// Read archive into memory.
	data, err := io.ReadAll(io.LimitReader(archive, size+1))
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}

	// Parse and validate the archive.
	nextraArchive, archiveWarnings, err := docsimport.ReadNextraArchiveFromBytes(data, maxNextraUncompressedSize)
	if err != nil {
		return nil, fmt.Errorf("invalid archive: %w", err)
	}

	// Build the import plan.
	adapter := docsimport.NextraAdapter{}
	plan, planWarnings, err := adapter.Preview(ctx, nextraArchive, docsimport.NextraPreviewOptions{
		SourceCommit: sourceCommit,
	})
	if err != nil {
		return nil, fmt.Errorf("build import plan: %w", err)
	}

	allWarnings := append(archiveWarnings, planWarnings...)

	// Store archive in S3.
	jobID := uuid.New().String()
	archiveKey := fmt.Sprintf("docs-import-archives/%s/%s.zip", workspaceID, jobID)
	if err := s.s3Client.PutObject(ctx, archiveKey, "application/zip", int64(len(data)), bytes.NewReader(data), false); err != nil {
		return nil, fmt.Errorf("store archive: %w", err)
	}

	// Build job config.
	config := model.DocsImportJobConfig{
		SourceSystem: "nextra",
		ArchiveKey:   archiveKey,
		ArchiveName:  archiveName,
		DetectedRoot: plan.RootPath,
		SourceCommit: sourceCommit,
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	// Build summary for preview.
	summary := model.ImportSummary{
		SourceSystem:       "nextra",
		CollectionsCreated: len(plan.Collections),
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return nil, fmt.Errorf("marshal summary: %w", err)
	}

	// Create pending import job.
	job := &model.DocsImportJob{
		ID:          jobID,
		WorkspaceID: workspaceID,
		Source:      "nextra",
		Status:      model.DocsImportStatusPending,
		Total:       len(plan.Articles),
		Config:      configJSON,
		Summary:     summaryJSON,
		StartedBy:   userID,
	}
	if err := s.importRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create import job: %w", err)
	}

	// Build preview response.
	resp := &model.DocsNextraImportPreviewResponse{
		JobID:        jobID,
		ArchiveName:  archiveName,
		SourceCommit: sourceCommit,
		DetectedRoot: plan.RootPath,
		Collections:  len(plan.Collections),
		Articles:     len(plan.Articles),
		Assets:       len(plan.Assets),
		Redirects:    len(plan.Redirects),
	}

	// Build space previews.
	for _, sp := range plan.Spaces {
		colCount := 0
		artCount := 0
		for _, c := range plan.Collections {
			if c.SpaceSourceID == sp.SourceID {
				colCount++
			}
		}
		for _, a := range plan.Articles {
			if a.SpaceSourceID == sp.SourceID {
				artCount++
			}
		}
		resp.Spaces = append(resp.Spaces, model.DocsNextraImportSpacePreview{
			SourceID:        sp.SourceID,
			Name:            sp.Name,
			CollectionCount: colCount,
			ArticleCount:    artCount,
		})
	}

	// Build warning responses.
	for _, w := range allWarnings {
		resp.Warnings = append(resp.Warnings, model.DocsImportWarningResponse{
			Type:    w.Type,
			Message: w.Message,
		})
	}

	// Build unsupported component summary.
	compCounts := map[string]int{}
	for _, a := range plan.Articles {
		for _, c := range a.UnsupportedComponents {
			compCounts[c]++
		}
	}
	for name, count := range compCounts {
		resp.UnsupportedComponents = append(resp.UnsupportedComponents, model.DocsImportUnsupportedComponentResponse{
			Name:  name,
			Count: count,
		})
	}

	// Build broken link summary.
	for _, w := range allWarnings {
		if w.Type == "broken_internal_link" {
			resp.BrokenLinks = append(resp.BrokenLinks, model.DocsImportBrokenLinkResponse{
				Message: w.Message,
			})
		}
	}

	s.logger.InfoContext(ctx, "nextra import preview created",
		"job_id", jobID,
		"workspace_id", workspaceID,
		"articles", len(plan.Articles),
		"collections", len(plan.Collections),
	)

	return resp, nil
}

// StartNextra loads a pending Nextra import job, re-reads the archive
// from S3, rebuilds the plan, and launches a background executor.
func (s *DocsImportService) StartNextra(ctx context.Context, req model.DocsNextraImportStartRequest, workspaceID, userID string) (string, error) {
	job, err := s.importRepo.GetByID(ctx, req.JobID)
	if err != nil {
		return "", fmt.Errorf("get import job: %w", err)
	}
	if job == nil {
		return "", fmt.Errorf("import job not found")
	}
	if job.WorkspaceID != workspaceID {
		return "", fmt.Errorf("import job does not belong to this workspace")
	}
	if job.Source != "nextra" {
		return "", fmt.Errorf("import job is not a Nextra import")
	}
	if job.Status != model.DocsImportStatusPending {
		return "", fmt.Errorf("import job is not in pending state (current: %s)", job.Status)
	}

	// Resolve target space.
	if req.TargetSpaceID == nil && req.NewSpaceName == nil {
		return "", fmt.Errorf("target_space_id or new_space_name is required")
	}

	// Parse config to get archive key.
	var config model.DocsImportJobConfig
	if err := json.Unmarshal(job.Config, &config); err != nil {
		return "", fmt.Errorf("parse job config: %w", err)
	}

	// Update config with start request.
	if req.TargetSpaceID != nil {
		config.TargetSpaceID = *req.TargetSpaceID
	}
	if req.NewSpaceName != nil {
		config.NewSpaceName = *req.NewSpaceName
	}
	config.ImportStatus = req.ImportStatus
	if config.ImportStatus == "" {
		config.ImportStatus = "draft"
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	// Mark job as running via repo methods.
	now := time.Now()
	job.Config = configJSON
	job.StartedAt = &now
	if err := s.importRepo.UpdateStatus(ctx, job.ID, model.DocsImportStatusRunning, nil); err != nil {
		return "", fmt.Errorf("update import job status: %w", err)
	}

	// Launch background import.
	go s.executeNextraImport(context.Background(), job.ID, workspaceID, userID, config)

	s.logger.InfoContext(ctx, "nextra import started",
		"job_id", job.ID,
		"workspace_id", workspaceID,
	)

	return job.ID, nil
}

// executeNextraImport runs the Nextra import in the background.
func (s *DocsImportService) executeNextraImport(ctx context.Context, jobID, workspaceID, userID string, config model.DocsImportJobConfig) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("nextra import panic", "job_id", jobID, "panic", r)
			_ = s.importRepo.SetError(ctx, jobID, fmt.Sprintf("internal error: %v", r))
		}
	}()

	// Download archive from S3.
	archiveData, err := s.s3Client.GetObject(ctx, config.ArchiveKey)
	if err != nil {
		s.logger.Error("nextra import: download archive failed", "job_id", jobID, "error", err)
		_ = s.importRepo.SetError(ctx, jobID, fmt.Sprintf("download archive: %v", err))
		return
	}

	// Re-parse archive.
	nextraArchive, _, err := docsimport.ReadNextraArchiveFromBytes(archiveData, maxNextraUncompressedSize)
	if err != nil {
		s.logger.Error("nextra import: parse archive failed", "job_id", jobID, "error", err)
		_ = s.importRepo.SetError(ctx, jobID, fmt.Sprintf("parse archive: %v", err))
		return
	}

	// Rebuild plan.
	adapter := docsimport.NextraAdapter{}
	plan, _, err := adapter.Preview(ctx, nextraArchive, docsimport.NextraPreviewOptions{
		SourceCommit: config.SourceCommit,
	})
	if err != nil {
		s.logger.Error("nextra import: build plan failed", "job_id", jobID, "error", err)
		_ = s.importRepo.SetError(ctx, jobID, fmt.Sprintf("build plan: %v", err))
		return
	}

	// Execute the import plan.
	if err := s.executeNextraImportPlan(ctx, jobID, workspaceID, userID, config, plan, nextraArchive); err != nil {
		s.logger.Error("nextra import: execution failed", "job_id", jobID, "error", err)
		_ = s.importRepo.SetError(ctx, jobID, fmt.Sprintf("execution: %v", err))
		return
	}

	s.logger.Info("nextra import completed", "job_id", jobID, "workspace_id", workspaceID)
}

// executeNextraImportPlan creates spaces, collections, articles,
// uploads assets, and creates redirects from a normalised ImportPlan.
func (s *DocsImportService) executeNextraImportPlan(ctx context.Context, jobID, workspaceID, userID string, config model.DocsImportJobConfig, plan *docsimport.ImportPlan, archive *docsimport.NextraArchive) error {
	summary := model.ImportSummary{SourceSystem: "nextra"}
	var failures []model.ImportFailure
	completed := 0
	failed := 0

	// 1. Resolve or create target space.
	spaceID := config.TargetSpaceID
	if spaceID == "" && config.NewSpaceName != "" {
		space, err := s.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{
			Name:       config.NewSpaceName,
			Visibility: model.SpaceVisibilityWorkspaceWide,
			Type:       model.SpaceTypeExternalCapable,
		}, userID)
		if err != nil {
			return fmt.Errorf("create space %q: %w", config.NewSpaceName, err)
		}
		spaceID = space.ID
	}

	// 2. Create collections parent-before-child (plan is topo-sorted).
	sourceToCollectionID := map[string]string{}
	sourceToCollectionSlug := map[string]string{}
	s.logger.Info("nextra import: creating collections",
		"job_id", jobID, "count", len(plan.Collections))
	for _, ic := range plan.Collections {
		var parentID *string
		if ic.ParentSourceID != "" {
			if pid, ok := sourceToCollectionID[ic.ParentSourceID]; ok {
				parentID = &pid
			} else {
				s.logger.Warn("nextra import: parent collection not found",
					"job_id", jobID, "collection", ic.Name,
					"source_id", ic.SourceID, "parent_source_id", ic.ParentSourceID)
			}
		}
		slug := ic.Slug
		s.logger.Info("nextra import: creating collection",
			"job_id", jobID, "name", ic.Name, "slug", slug,
			"source_id", ic.SourceID, "parent_source_id", ic.ParentSourceID,
			"has_parent_id", parentID != nil)

		coll, err := s.collectionSvc.Create(ctx, workspaceID, spaceID, model.CreateDocsCollectionRequest{
			Name:               ic.Name,
			Slug:               &slug,
			ParentCollectionID: parentID,
		}, userID)
		if err != nil {
			s.logger.Error("nextra import: create collection failed",
				"job_id", jobID, "collection", ic.Name, "slug", slug, "error", err)
			continue
		}
		sourceToCollectionID[ic.SourceID] = coll.ID
		sourceToCollectionSlug[ic.SourceID] = coll.Slug
		summary.CollectionsCreated++
		s.logger.Info("nextra import: collection created",
			"job_id", jobID, "name", ic.Name, "helpin_id", coll.ID)
	}

	// 3. Upload all images and build rewrite maps.
	contentFiles := archive.ContentFiles()
	publicFiles := archive.PublicFiles()

	// Upload images to S3 and build resolved archive path → public URL map.
	imageURLMap := map[string]string{}
	if s.s3Client != nil {
		seen := map[string]bool{}
		for _, asset := range plan.Assets {
			if seen[asset.SourcePath] {
				continue
			}
			seen[asset.SourcePath] = true

			assetData := contentFiles[asset.SourcePath]
			if assetData == nil {
				pubPath := asset.SourcePath
				if len(pubPath) > 7 && pubPath[:7] == "public/" {
					pubPath = pubPath[7:]
				}
				assetData = publicFiles[pubPath]
			}
			if assetData == nil {
				summary.AssetRewriteFailures++
				continue
			}
			key := fmt.Sprintf("docs-import/%s/%s/%s", workspaceID, jobID, asset.SourcePath)
			if err := s.s3Client.PutObject(ctx, key, asset.ContentType, int64(len(assetData)), bytes.NewReader(assetData), true); err != nil {
				summary.AssetRewriteFailures++
				s.logger.Error("nextra import: upload asset failed",
					"job_id", jobID, "asset", asset.SourcePath, "error", err)
				continue
			}
			publicURL := s.s3Client.PublicURL(key)
			if strings.TrimSpace(publicURL) == "" {
				summary.AssetRewriteFailures++
				s.logger.Error("nextra import: public asset URL is empty",
					"job_id", jobID, "asset", asset.SourcePath)
				continue
			}
			// Map both the content-relative path and the /assets/... absolute path.
			imageURLMap[asset.SourcePath] = publicURL
			// For public/ images referenced as /filename.png
			if len(asset.SourcePath) > 7 && asset.SourcePath[:7] == "public/" {
				imageURLMap["/"+asset.SourcePath[7:]] = publicURL
			}
		}
		s.logger.Info("nextra import: images uploaded",
			"job_id", jobID, "count", len(imageURLMap))
	}

	// 4. Create all articles and HC records, collecting route→canonical path map.
	type createdArticle struct {
		docID         string
		contentID     string
		rawContent    string
		sourceRoute   string
		slug          string
		publicID      string
		collSourceID  string
		title         string
		hidden        bool
		shouldPublish bool
		sourceID      string
	}
	var created []createdArticle
	routeToCanonical := map[string]string{} // source route → /articles/slug-publicID

	for _, ia := range plan.Articles {
		// Resolve collection ID.
		var collectionID *string
		if ia.CollectionSourceID != "" {
			if cid, ok := sourceToCollectionID[ia.CollectionSourceID]; ok {
				collectionID = &cid
			} else {
				s.logger.Warn("nextra import: article collection not found, will be uncategorized",
					"job_id", jobID, "title", ia.Title,
					"collection_source_id", ia.CollectionSourceID,
					"known_collections", len(sourceToCollectionID))
			}
		}

		// Create document with placeholder content.
		doc, err := s.documentSvc.Create(ctx, workspaceID, model.CreateDocsDocumentRequest{
			SpaceID:      spaceID,
			CollectionID: collectionID,
			Title:        ia.Title,
		}, userID)
		if err != nil {
			failed++
			failures = append(failures, model.ImportFailure{
				Title: ia.Title,
				Error: fmt.Sprintf("create document: %v", err),
			})
			continue
		}

		// Save initial content (will be rewritten in phase 5).
		contentJSON := nextraContentToTiptapJSON(ia.RawContent)
		savedContent, err := s.contentSvc.Save(ctx, doc.ID, contentJSON, userID)
		if err != nil {
			failed++
			failures = append(failures, model.ImportFailure{
				ArticleID: doc.ID,
				Title:     ia.Title,
				Error:     fmt.Sprintf("save content: %v", err),
			})
			continue
		}

		// Store import provenance.
		s.contentSvc.SetImportProvenance(ctx, savedContent.ID, ia.RawContent, "nextra", ia.SourceID)

		// Create helpcenter article.
		hcArticle := &model.DocsHelpcenterArticle{
			DocumentID: doc.ID,
			Slug:       ia.Slug,
		}
		createdHC, err := s.helpcenterSvc.CreateArticle(ctx, hcArticle)
		if err != nil {
			s.logger.Error("nextra import: create HC article failed",
				"job_id", jobID, "title", ia.Title, "error", err)
		}

		// Collect for link rewriting.
		ca := createdArticle{
			docID:         doc.ID,
			contentID:     savedContent.ID,
			rawContent:    ia.RawContent,
			sourceRoute:   ia.SourceRoute,
			slug:          ia.Slug,
			collSourceID:  ia.CollectionSourceID,
			title:         ia.Title,
			hidden:        ia.Hidden,
			shouldPublish: config.ImportStatus == "published" && !ia.Hidden,
			sourceID:      ia.SourceID,
		}
		if createdHC != nil {
			ca.publicID = createdHC.PublicID
			ca.slug = createdHC.Slug
			if ia.SourceRoute != "" {
				canonical := buildDocsHelpcenterArticleCanonicalPath(nil, "", createdHC.Slug, createdHC.PublicID)
				routeToCanonical[ia.SourceRoute] = canonical
			}
		}
		created = append(created, ca)

		// Create redirect.
		if ia.SourceRoute != "" && createdHC != nil {
			articleSlug := createdHC.Slug
			canonicalPath := buildDocsHelpcenterArticleCanonicalPath(nil, "", createdHC.Slug, createdHC.PublicID)
			redirect := &model.DocsRedirect{
				WorkspaceID:       workspaceID,
				SourcePath:        ia.SourceRoute,
				TargetArticleSlug: &articleSlug,
				TargetPath:        &canonicalPath,
				Type:              model.RedirectTypeImported,
				SourceSystem:      stringPtr("nextra"),
				SourceObjectType:  stringPtr("page"),
				SourceObjectID:    stringPtr(ia.SourceID),
			}
			if ia.CollectionSourceID != "" {
				if cs, ok := sourceToCollectionSlug[ia.CollectionSourceID]; ok {
					redirect.TargetCollectionSlug = cs
				}
			}
			if err := s.redirectRepo.Create(ctx, redirect); err != nil {
				s.logger.Error("nextra import: create redirect failed",
					"job_id", jobID, "source_route", ia.SourceRoute, "error", err)
			} else {
				summary.RedirectsCreated++
			}
		}

		completed++
		if completed%5 == 0 {
			failuresJSON, _ := json.Marshal(failures)
			_ = s.importRepo.UpdateProgress(ctx, jobID, completed, failed, failuresJSON)
		}
	}

	// 5. Rewrite content with correct image URLs and internal links before publishing.
	if len(created) > 0 {
		rewriteFailed := map[string]bool{}
		if len(imageURLMap) > 0 || len(routeToCanonical) > 0 {
			s.logger.Info("nextra import: rewriting content",
				"job_id", jobID, "images", len(imageURLMap), "links", len(routeToCanonical))
		}

		for _, ca := range created {
			rewritten := rewriteNextraImportedContent(ca.rawContent, imageURLMap, routeToCanonical)
			if rewritten == ca.rawContent {
				s.contentSvc.SetImportProvenance(ctx, ca.contentID, rewritten, "nextra", ca.sourceID)
				continue
			}

			contentJSON := nextraContentToTiptapJSON(rewritten)
			savedContent, err := s.contentSvc.Save(ctx, ca.docID, contentJSON, userID)
			if err != nil {
				s.logger.Error("nextra import: rewrite content failed",
					"job_id", jobID, "title", ca.title, "error", err)
				rewriteFailed[ca.docID] = true
				continue
			}
			s.contentSvc.SetImportProvenance(ctx, savedContent.ID, rewritten, "nextra", ca.sourceID)
		}

		// 6. Publish after rewrites so public snapshots contain final image URLs.
		for _, ca := range created {
			if !ca.shouldPublish || rewriteFailed[ca.docID] {
				summary.ArticlesDrafted++
				continue
			}
			if _, err := s.documentSvc.Publish(ctx, ca.docID); err != nil {
				s.logger.Error("nextra import: internal publish failed",
					"job_id", jobID, "title", ca.title, "error", err)
				summary.ArticlesDrafted++
			} else if err := s.helpcenterSvc.PublishExternally(ctx, ca.docID, ca.slug, nil); err != nil {
				s.logger.Error("nextra import: external publish failed",
					"job_id", jobID, "title", ca.title, "error", err)
				summary.ArticlesDrafted++
			} else {
				summary.ArticlesPublished++
			}
		}
	}

	// 7. Finalize.
	summaryJSON, _ := json.Marshal(summary)
	_ = s.importRepo.SetSummary(ctx, jobID, summaryJSON)

	failuresJSON, _ := json.Marshal(failures)
	_ = s.importRepo.UpdateProgress(ctx, jobID, completed, failed, failuresJSON)

	now := time.Now()
	if err := s.importRepo.UpdateStatus(ctx, jobID, model.DocsImportStatusDone, &now); err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	return nil
}

// rewriteInternalLinks replaces Nextra route references in markdown
// content with canonical Helpin paths. It handles:
//   - [text](/route) → [text](/articles/slug-publicID)
//   - [text](/route#anchor) → [text](/articles/slug-publicID#anchor)
//
// Routes are matched longest-first to prevent /foo matching /foo/bar.
func rewriteInternalLinks(content string, routeToCanonical map[string]string) string {
	// Sort routes longest-first.
	routes := make([]string, 0, len(routeToCanonical))
	for r := range routeToCanonical {
		routes = append(routes, r)
	}
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i]) > len(routes[j])
	})

	for _, oldRoute := range routes {
		newPath := routeToCanonical[oldRoute]
		// Exact match: [text](/route)
		content = strings.ReplaceAll(content, "("+oldRoute+")", "("+newPath+")")
		// With anchor: [text](/route#section)
		content = strings.ReplaceAll(content, "("+oldRoute+"#", "("+newPath+"#")
	}
	return content
}

func rewriteNextraImportedContent(content string, imageURLMap, routeToCanonical map[string]string) string {
	rewritten := content
	for oldPath, newURL := range imageURLMap {
		if strings.TrimSpace(newURL) == "" {
			continue
		}
		rewritten = strings.ReplaceAll(rewritten, "("+oldPath+")", "("+newURL+")")
		rewritten = strings.ReplaceAll(rewritten, "\""+oldPath+"\"", "\""+newURL+"\"")
		rewritten = strings.ReplaceAll(rewritten, "'"+oldPath+"'", "'"+newURL+"'")
	}
	return rewriteInternalLinks(rewritten, routeToCanonical)
}

func nextraContentToTiptapJSON(content string) json.RawMessage {
	return tiptap.MarkdownToJSON(convertNextraHTMLImagesToMarkdown(content))
}

var nextraHTMLImageTagPattern = regexp.MustCompile(`(?i)<img\b[^>]*>`)

func convertNextraHTMLImagesToMarkdown(content string) string {
	return nextraHTMLImageTagPattern.ReplaceAllStringFunc(content, func(tag string) string {
		src := htmlAttrValue(tag, "src")
		if strings.TrimSpace(src) == "" {
			return tag
		}
		alt := escapeMarkdownImageAlt(htmlAttrValue(tag, "alt"))
		return "\n\n![" + alt + "](" + src + ")\n\n"
	})
}

func htmlAttrValue(tag, attr string) string {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(attr) + `\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	match := re.FindStringSubmatch(tag)
	if len(match) == 0 {
		return ""
	}
	if match[1] != "" {
		return match[1]
	}
	return match[2]
}

func escapeMarkdownImageAlt(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `]`, `\]`)
	return value
}
