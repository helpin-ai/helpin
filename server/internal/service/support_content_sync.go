package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	htmlstd "html"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SupportContentSyncWorkflowStarter queues durable crawl+embedding work.
type SupportContentSyncWorkflowStarter interface {
	QueueContentSourceSync(ctx context.Context, workspaceID, contentSourceID string) error
	QueueContentSourceReindex(ctx context.Context, workspaceID, contentSourceID string) error
}

// SupportContentSyncWorkflowCanceller is implemented by durable workflow
// starters that can stop an in-flight source sync before its data is deleted.
type SupportContentSyncWorkflowCanceller interface {
	CancelContentSourceSync(ctx context.Context, workspaceID, contentSourceID string) error
}

// SupportContentSyncService keeps crawled web content indexed in pgvector.
type SupportContentSyncService struct {
	sourceRepo     *repository.SupportContentSourceRepository
	pageRepo       *repository.SupportContentPageRepository
	chunkRepo      *repository.SupportContentChunkRepository
	embedder       llm.EmbeddingProvider
	embeddingModel string
	crawler        crawler.ContentCrawler
	objectStore    supportContentObjectStore
	starter        SupportContentSyncWorkflowStarter
}

// NewSupportContentSyncService creates a sync service that uses the provided
// ContentCrawler to fetch pages and the embedding provider to index them.
func NewSupportContentSyncService(
	sourceRepo *repository.SupportContentSourceRepository,
	pageRepo *repository.SupportContentPageRepository,
	chunkRepo *repository.SupportContentChunkRepository,
	embedder llm.EmbeddingProvider,
	embeddingModel string,
	contentCrawler crawler.ContentCrawler,
	objectStore supportContentObjectStore,
	starter SupportContentSyncWorkflowStarter,
) *SupportContentSyncService {
	if strings.TrimSpace(embeddingModel) == "" {
		embeddingModel = defaultDocsEmbeddingModel
	}
	return &SupportContentSyncService{
		sourceRepo:     sourceRepo,
		pageRepo:       pageRepo,
		chunkRepo:      chunkRepo,
		embedder:       embedder,
		embeddingModel: strings.TrimSpace(embeddingModel),
		crawler:        contentCrawler,
		objectStore:    objectStore,
		starter:        starter,
	}
}

func (s *SupportContentSyncService) QueueSourceSync(ctx context.Context, workspaceID, contentSourceID string) error {
	return s.queueSourceSync(ctx, workspaceID, contentSourceID, nil)
}

func (s *SupportContentSyncService) queueSourceSync(ctx context.Context, workspaceID, contentSourceID string, autoSyncSince *time.Time) error {
	if s == nil || s.sourceRepo == nil {
		return nil
	}
	source, err := s.sourceRepo.GetByID(ctx, contentSourceID)
	if err != nil {
		return err
	}
	if source == nil || source.WorkspaceID != workspaceID {
		return fmt.Errorf("content source not found in workspace")
	}
	if autoSyncSince != nil {
		claimed, err := s.sourceRepo.ClaimAutoSync(ctx, source.ID, *autoSyncSince)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
	}
	slog.InfoContext(ctx, "queueing support content source sync",
		"workspace_id", workspaceID,
		"content_source_id", source.ID,
		"start_url", source.StartURL,
		"crawl_limit", source.CrawlLimit,
		"crawl_depth", source.CrawlDepth,
		"crawl_source", source.CrawlSource,
	)

	refreshEmbeddingSource(s.embedder, source.WorkspaceID)
	if !embeddingsAvailable(ctx, s.embedder, source.WorkspaceID) {
		msg := embeddingProviderMissingMessage
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
		return nil
	}
	if source.SourceType == model.ContentSourceTypeFile {
		if s.objectStore == nil {
			msg := "file storage is not configured"
			_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
			return nil
		}
	} else if s.crawler == nil {
		msg := "content crawler is not configured"
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
		return nil
	}
	if autoSyncSince == nil {
		if err := s.sourceRepo.MarkSyncQueued(ctx, source.ID); err != nil {
			return err
		}
	}
	if s.starter == nil {
		err = fmt.Errorf("Temporal content sync pipeline is not configured")
		_ = s.markSourceFailed(ctx, source.ID, err, nil, source.IndexedPages, source.IndexedChunks)
		return err
	}
	if err := s.starter.QueueContentSourceSync(ctx, workspaceID, contentSourceID); err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, nil, source.IndexedPages, source.IndexedChunks)
		return err
	}
	return nil
}

// CancelSourceSync stops the deterministic durable workflow for a content
// source. A missing starter means no workflow was queued by this service.
func (s *SupportContentSyncService) CancelSourceSync(ctx context.Context, workspaceID, contentSourceID string) error {
	if s == nil || s.starter == nil {
		return nil
	}
	canceller, ok := s.starter.(SupportContentSyncWorkflowCanceller)
	if !ok {
		return fmt.Errorf("Temporal content sync cancellation is not configured")
	}
	return canceller.CancelContentSourceSync(ctx, workspaceID, contentSourceID)
}

// RunSourceSync performs the full crawl + embedding pipeline for a content source.
// It uses the configured ContentCrawler to fetch pages via a callback and indexes
// each page incrementally.
func (s *SupportContentSyncService) RunSourceSync(ctx context.Context, workspaceID, contentSourceID string) error {
	if s == nil {
		return nil
	}
	source, err := s.sourceRepo.GetByID(ctx, contentSourceID)
	if err != nil {
		return err
	}
	if source == nil || source.WorkspaceID != workspaceID {
		return fmt.Errorf("content source not found in workspace")
	}
	slog.InfoContext(ctx, "starting support content source sync",
		"workspace_id", workspaceID,
		"content_source_id", source.ID,
		"start_url", source.StartURL,
		"crawl_limit", source.CrawlLimit,
		"crawl_depth", source.CrawlDepth,
		"crawl_source", source.CrawlSource,
		"include_subdomains", source.IncludeSubdomains,
		"include_external_links", source.IncludeExternalLinks,
	)
	refreshEmbeddingSource(s.embedder, source.WorkspaceID)
	if !embeddingsAvailable(ctx, s.embedder, source.WorkspaceID) {
		msg := embeddingProviderMissingMessage
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
	}
	if source.SourceType == model.ContentSourceTypeFile {
		return s.runFileSourceSync(ctx, *source)
	}
	if s.crawler == nil {
		msg := "content crawler is not configured"
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
	}

	startedAt := time.Now()
	if err := s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, 0, source.IndexedPages, source.IndexedChunks, nil, nil, &startedAt, nil); err != nil {
		return err
	}

	// Mutable state shared with the onPage callback.
	var (
		keepPageIDs   []string
		keepURLs      []string
		indexedPages  int
		indexedChunks int
	)

	if err := s.sourceRepo.UpdateSyncWarning(ctx, source.ID, nil); err != nil {
		return err
	}
	skippedURLs := map[string]bool{}
	onPage := func(record crawler.CrawlRecord) error {
		if record.SkipReason != "" {
			skippedURLs[record.URL] = true
			return nil
		}
		contentText, format := crawlRecordText(record)
		if strings.TrimSpace(contentText) == "" {
			slog.DebugContext(ctx, "support content page skipped: empty content",
				"content_source_id", source.ID,
				"url", record.URL,
				"status", record.HTTPStatus,
			)
			return nil
		}

		title := crawlRecordTitle(record)
		pageURL := strings.ToValidUTF8(strings.TrimSpace(record.URL), "")
		page := &model.SupportContentPage{
			WorkspaceID:     source.WorkspaceID,
			ContentSourceID: source.ID,
			URL:             pageURL,
			Title:           title,
			HTTPStatus:      record.HTTPStatus,
			ContentFormat:   format,
			ContentText:     contentText,
			ContentHash:     hashChunk(title+"\n"+pageURL, contentText),
			Metadata:        sanitizeJSONUTF8(mustMarshalJSON(record.Metadata)),
			LastCrawledAt:   time.Now(),
		}
		savedPage, err := s.pageRepo.Upsert(ctx, page)
		if err != nil {
			return err
		}

		chunks := chunkStructuredDocument(title, contentText)
		if len(chunks) == 0 {
			return s.chunkRepo.ReplacePageChunks(ctx, savedPage.ID, nil)
		}
		states, err := s.chunkRepo.ListReusableEmbeddingStates(ctx, source.WorkspaceID, savedPage.ID, s.embeddingModel, contentChunkEmbeddingVersion, docsEmbeddingDimensions)
		if err != nil {
			return err
		}
		if matchingKnowledgeIndex(title, chunks, states) {
			indexedPages++
			indexedChunks += len(chunks)
			keepPageIDs = append(keepPageIDs, savedPage.ID)
			keepURLs = append(keepURLs, savedPage.URL)
			return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, progressFor(indexedPages, source.CrawlLimit), indexedPages, indexedChunks, nil, nil, &startedAt, nil)
		}

		embedCtx := withAIActionMetering(ctx, source.WorkspaceID, aipolicy.ActionSupportKnowledgeEmbed, "support_content_crawl_embed", savedPage.ContentHash, map[string]interface{}{
			"surface": "support_content_sync", "source_id": source.ID, "page_id": savedPage.ID,
		})
		resp, err := s.embedder.CreateEmbeddings(embedCtx, llm.EmbeddingRequest{
			Provider: "openai",
			Model:    s.embeddingModel,
			Inputs:   structuredChunkSearchInputs(chunks),
		})
		if err != nil {
			slog.WarnContext(ctx, "skipping page: embedding failed", "page_id", savedPage.ID, "url", savedPage.URL, "error", err)
			keepPageIDs = append(keepPageIDs, savedPage.ID)
			keepURLs = append(keepURLs, savedPage.URL)
			return nil
		}
		if len(resp.Vectors) != len(chunks) {
			slog.WarnContext(ctx, "skipping page: embedding count mismatch", "page_id", savedPage.ID, "got", len(resp.Vectors), "want", len(chunks))
			keepPageIDs = append(keepPageIDs, savedPage.ID)
			keepURLs = append(keepURLs, savedPage.URL)
			return nil
		}
		rows := make([]model.SupportContentChunk, 0, len(chunks))
		validEmbeddings := true
		for chunkIndex, chunk := range chunks {
			if len(resp.Vectors[chunkIndex]) != docsEmbeddingDimensions {
				slog.WarnContext(ctx, "skipping page: embedding dimension mismatch", "page_id", savedPage.ID, "got", len(resp.Vectors[chunkIndex]), "want", docsEmbeddingDimensions)
				validEmbeddings = false
				break
			}
			safeTitle := strings.ToValidUTF8(savedPage.Title, "")
			safeURL := strings.ToValidUTF8(savedPage.URL, "")
			safeChunk := strings.ToValidUTF8(chunk.Content, "")
			safeSearchContent := strings.ToValidUTF8(chunk.SearchContent, "")
			previous, next := neighborChunkIndexes(chunkIndex, len(chunks))
			rows = append(rows, model.SupportContentChunk{
				WorkspaceID:         source.WorkspaceID,
				ContentSourceID:     source.ID,
				PageID:              savedPage.ID,
				ChunkIndex:          chunkIndex,
				SectionKey:          chunk.SectionKey,
				HeadingPath:         chunk.HeadingPath,
				Title:               safeTitle,
				URL:                 safeURL,
				Content:             safeChunk,
				SearchContent:       safeSearchContent,
				PreviousChunkIndex:  previous,
				NextChunkIndex:      next,
				ContentHash:         hashChunk(safeTitle, safeSearchContent),
				Embedding:           formatVector(resp.Vectors[chunkIndex]),
				EmbeddingProvider:   "openai",
				EmbeddingModel:      s.embeddingModel,
				EmbeddingVersion:    contentChunkEmbeddingVersion,
				EmbeddingDimensions: docsEmbeddingDimensions,
			})
		}
		if !validEmbeddings {
			keepPageIDs = append(keepPageIDs, savedPage.ID)
			keepURLs = append(keepURLs, savedPage.URL)
			return nil
		}
		if err := s.chunkRepo.ReplacePageChunks(ctx, savedPage.ID, rows); err != nil {
			return err
		}

		indexedPages++
		indexedChunks += len(rows)
		keepPageIDs = append(keepPageIDs, savedPage.ID)
		keepURLs = append(keepURLs, savedPage.URL)

		// Update progress incrementally.
		pct := 0
		if source.CrawlLimit > 0 {
			pct = (indexedPages * 100) / source.CrawlLimit
		}
		if pct > 99 {
			pct = 99 // Reserve 100 for final completion.
		}
		slog.InfoContext(ctx, "support content page indexed",
			"workspace_id", source.WorkspaceID,
			"content_source_id", source.ID,
			"page_id", savedPage.ID,
			"url", savedPage.URL,
			"title", savedPage.Title,
			"chunks", len(rows),
			"progress_pct", pct,
			"indexed_pages", indexedPages,
			"indexed_chunks", indexedChunks,
		)
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, pct, indexedPages, indexedChunks, nil, nil, &startedAt, nil)
		return nil
	}

	slog.InfoContext(ctx, "support content crawl starting",
		"workspace_id", workspaceID,
		"content_source_id", source.ID,
		"start_url", source.StartURL,
	)
	crawledCount, err := s.crawler.Crawl(ctx, *source, onPage)
	if len(skippedURLs) > 0 {
		warning := fmt.Sprintf("%d URLs skipped because the site disallows crawling. Check robots.txt and the crawler access instructions before re-syncing.", len(skippedURLs))
		if warningErr := s.sourceRepo.UpdateSyncWarning(ctx, source.ID, &warning); warningErr != nil {
			return warningErr
		}
	}
	if err != nil {
		slog.ErrorContext(ctx, "support content crawl failed",
			"workspace_id", workspaceID,
			"content_source_id", source.ID,
			"crawled_pages", crawledCount,
			"error", err,
		)
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}
	slog.InfoContext(ctx, "support content crawl finished",
		"workspace_id", workspaceID,
		"content_source_id", source.ID,
		"crawled_pages", crawledCount,
		"indexed_pages", indexedPages,
		"indexed_chunks", indexedChunks,
	)

	// Clean up stale pages that were not seen in this crawl (full crawl only).
	if source.ModifiedSince == nil {
		if err := s.chunkRepo.DeleteByContentSourceExceptPages(ctx, source.WorkspaceID, source.ID, keepPageIDs); err != nil {
			_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
			return err
		}
		if err := s.pageRepo.DeleteByContentSourceExceptURLs(ctx, source.WorkspaceID, source.ID, keepURLs); err != nil {
			_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
			return err
		}
	}

	completedAt := time.Now()
	slog.InfoContext(ctx, "completed support content source sync",
		"workspace_id", workspaceID,
		"content_source_id", source.ID,
		"crawl_limit", source.CrawlLimit,
		"indexed_pages", indexedPages,
		"indexed_chunks", indexedChunks,
	)
	return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncReady, 100, indexedPages, indexedChunks, nil, nil, &startedAt, &completedAt)
}

func (s *SupportContentSyncService) runFileSourceSync(ctx context.Context, source model.SupportContentSource) error {
	if s.objectStore == nil {
		msg := "file storage is not configured"
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
	}
	if source.StorageKey == nil || strings.TrimSpace(*source.StorageKey) == "" {
		err := fmt.Errorf("file source storage key is missing")
		_ = s.markSourceFailed(ctx, source.ID, err, nil, source.IndexedPages, source.IndexedChunks)
		return err
	}

	startedAt := time.Now()
	if err := s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, 0, source.IndexedPages, source.IndexedChunks, nil, nil, &startedAt, nil); err != nil {
		return err
	}

	data, err := s.objectStore.GetObject(ctx, strings.TrimSpace(*source.StorageKey))
	if err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}
	fileName := source.Name
	if source.FileName != nil && strings.TrimSpace(*source.FileName) != "" {
		fileName = strings.TrimSpace(*source.FileName)
	}
	contentType := ""
	if source.ContentType != nil {
		contentType = *source.ContentType
	}
	contentText, format, err := extractUploadedContentText(data, contentType, fileName)
	if err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}
	if strings.TrimSpace(contentText) == "" {
		err := fmt.Errorf("uploaded file contains no indexable text")
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}

	pageURL := strings.ToValidUTF8(source.StartURL, "")
	page := &model.SupportContentPage{
		WorkspaceID:     source.WorkspaceID,
		ContentSourceID: source.ID,
		URL:             pageURL,
		Title:           fileName,
		HTTPStatus:      200,
		ContentFormat:   format,
		ContentText:     contentText,
		ContentHash:     hashChunk(fileName+"\n"+pageURL, contentText),
		Metadata: sanitizeJSONUTF8(mustMarshalJSON(map[string]any{
			"source_type":  source.SourceType,
			"file_name":    fileName,
			"content_type": contentType,
			"file_size":    source.FileSize,
		})),
		LastCrawledAt: time.Now(),
	}
	savedPage, err := s.pageRepo.Upsert(ctx, page)
	if err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}

	chunks := chunkDocumentText(contentText)
	if len(chunks) == 0 {
		if err := s.chunkRepo.ReplacePageChunks(ctx, savedPage.ID, nil); err != nil {
			_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
			return err
		}
		completedAt := time.Now()
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncReady, 100, 1, 0, nil, nil, &startedAt, &completedAt)
	}
	states, err := s.chunkRepo.ListReusableEmbeddingStates(ctx, source.WorkspaceID, savedPage.ID, s.embeddingModel, contentChunkEmbeddingVersion, docsEmbeddingDimensions)
	if err != nil {
		return errors.Join(err, s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks))
	}
	if matchingKnowledgeIndex(fileName, fileKnowledgeChunks(contentText), states) {
		completedAt := time.Now()
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncReady, 100, 1, len(chunks), nil, nil, &startedAt, &completedAt)
	}

	embedCtx := withAIActionMetering(ctx, source.WorkspaceID, aipolicy.ActionSupportKnowledgeEmbed, "support_upload_embed", savedPage.ContentHash, map[string]interface{}{
		"surface": "support_content_upload", "source_id": source.ID, "page_id": savedPage.ID,
	})
	resp, err := s.embedder.CreateEmbeddings(embedCtx, llm.EmbeddingRequest{
		Provider: "openai",
		Model:    s.embeddingModel,
		Inputs:   chunks,
	})
	if err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}
	if len(resp.Vectors) != len(chunks) {
		err := fmt.Errorf("embedding count mismatch: got %d, want %d", len(resp.Vectors), len(chunks))
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}
	rows := make([]model.SupportContentChunk, 0, len(chunks))
	for chunkIndex, chunk := range chunks {
		if len(resp.Vectors[chunkIndex]) != docsEmbeddingDimensions {
			err := fmt.Errorf("embedding dimension mismatch: got %d, want %d", len(resp.Vectors[chunkIndex]), docsEmbeddingDimensions)
			_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
			return err
		}
		safeTitle := strings.ToValidUTF8(savedPage.Title, "")
		safeURL := strings.ToValidUTF8(savedPage.URL, "")
		safeChunk := strings.ToValidUTF8(chunk, "")
		rows = append(rows, model.SupportContentChunk{
			WorkspaceID:         source.WorkspaceID,
			ContentSourceID:     source.ID,
			PageID:              savedPage.ID,
			ChunkIndex:          chunkIndex,
			Title:               safeTitle,
			URL:                 safeURL,
			Content:             safeChunk,
			ContentHash:         hashChunk(safeTitle, safeChunk),
			Embedding:           formatVector(resp.Vectors[chunkIndex]),
			EmbeddingProvider:   "openai",
			EmbeddingModel:      s.embeddingModel,
			EmbeddingVersion:    contentChunkEmbeddingVersion,
			EmbeddingDimensions: docsEmbeddingDimensions,
		})
	}
	if err := s.chunkRepo.ReplacePageChunks(ctx, savedPage.ID, rows); err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}
	if err := s.chunkRepo.DeleteByContentSourceExceptPages(ctx, source.WorkspaceID, source.ID, []string{savedPage.ID}); err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}
	if err := s.pageRepo.DeleteByContentSourceExceptURLs(ctx, source.WorkspaceID, source.ID, []string{savedPage.URL}); err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}

	completedAt := time.Now()
	return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncReady, 100, 1, len(rows), nil, nil, &startedAt, &completedAt)
}

// QueueSourceReindex queues a re-embedding job that reads existing pages from
// the database instead of re-crawling the website.
func (s *SupportContentSyncService) QueueSourceReindex(ctx context.Context, workspaceID, contentSourceID string) error {
	if s == nil || s.sourceRepo == nil {
		return nil
	}
	source, err := s.sourceRepo.GetByID(ctx, contentSourceID)
	if err != nil {
		return err
	}
	if source == nil || source.WorkspaceID != workspaceID {
		return fmt.Errorf("content source not found in workspace")
	}
	refreshEmbeddingSource(s.embedder, source.WorkspaceID)
	if !embeddingsAvailable(ctx, s.embedder, source.WorkspaceID) {
		msg := embeddingProviderMissingMessage
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
		return nil
	}
	if err := s.sourceRepo.MarkSyncQueued(ctx, source.ID); err != nil {
		return err
	}
	if s.starter == nil {
		err = fmt.Errorf("Temporal content sync pipeline is not configured")
		_ = s.markSourceFailed(ctx, source.ID, err, nil, source.IndexedPages, source.IndexedChunks)
		return err
	}
	if err := s.starter.QueueContentSourceReindex(ctx, workspaceID, contentSourceID); err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, nil, source.IndexedPages, source.IndexedChunks)
		return err
	}
	return nil
}

// RunSourceReindex re-chunks and re-embeds existing pages stored in the
// database without re-crawling the website. This is faster and avoids
// crawler-related issues (rate limits, encoding errors from remote content).
func (s *SupportContentSyncService) RunSourceReindex(ctx context.Context, workspaceID, contentSourceID string) error {
	if s == nil {
		return nil
	}
	source, err := s.sourceRepo.GetByID(ctx, contentSourceID)
	if err != nil {
		return err
	}
	if source == nil || source.WorkspaceID != workspaceID {
		return fmt.Errorf("content source not found in workspace")
	}
	refreshEmbeddingSource(s.embedder, source.WorkspaceID)
	if !embeddingsAvailable(ctx, s.embedder, source.WorkspaceID) {
		msg := embeddingProviderMissingMessage
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
	}

	startedAt := time.Now()
	if err := s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, 0, source.IndexedPages, source.IndexedChunks, nil, nil, &startedAt, nil); err != nil {
		return err
	}

	pages, err := s.pageRepo.ListByContentSourceIDWithContent(ctx, contentSourceID)
	if err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}

	var (
		indexedPages  int
		indexedChunks int
	)

	for _, page := range pages {
		if strings.TrimSpace(page.ContentText) == "" {
			_ = s.chunkRepo.ReplacePageChunks(ctx, page.ID, nil)
			continue
		}

		chunks := chunkStructuredDocument(page.Title, page.ContentText)
		if source.SourceType == model.ContentSourceTypeFile {
			chunks = fileKnowledgeChunks(page.ContentText)
		}
		if len(chunks) == 0 {
			_ = s.chunkRepo.ReplacePageChunks(ctx, page.ID, nil)
			continue
		}
		states, err := s.chunkRepo.ListReusableEmbeddingStates(ctx, source.WorkspaceID, page.ID, s.embeddingModel, contentChunkEmbeddingVersion, docsEmbeddingDimensions)
		if err != nil {
			return errors.Join(err, s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks))
		}
		if matchingKnowledgeIndex(page.Title, chunks, states) {
			indexedPages++
			indexedChunks += len(chunks)
			continue
		}

		embedCtx := withAIActionMetering(ctx, source.WorkspaceID, aipolicy.ActionSupportKnowledgeEmbed, "support_content_reindex_embed", page.ContentHash, map[string]interface{}{
			"surface": "support_content_reindex", "source_id": source.ID, "page_id": page.ID,
		})
		resp, err := s.embedder.CreateEmbeddings(embedCtx, llm.EmbeddingRequest{
			Provider: "openai",
			Model:    s.embeddingModel,
			Inputs:   structuredChunkSearchInputs(chunks),
		})
		if err != nil {
			slog.WarnContext(ctx, "skipping page: embedding failed", "page_id", page.ID, "url", page.URL, "error", err)
			continue
		}
		if len(resp.Vectors) != len(chunks) {
			slog.WarnContext(ctx, "skipping page: embedding count mismatch", "page_id", page.ID, "got", len(resp.Vectors), "want", len(chunks))
			continue
		}

		rows := make([]model.SupportContentChunk, 0, len(chunks))
		validEmbeddings := true
		for chunkIndex, chunk := range chunks {
			if len(resp.Vectors[chunkIndex]) != docsEmbeddingDimensions {
				slog.WarnContext(ctx, "skipping page: embedding dimension mismatch", "page_id", page.ID, "got", len(resp.Vectors[chunkIndex]), "want", docsEmbeddingDimensions)
				validEmbeddings = false
				break
			}
			safeTitle := strings.ToValidUTF8(page.Title, "")
			safeURL := strings.ToValidUTF8(page.URL, "")
			safeChunk := strings.ToValidUTF8(chunk.Content, "")
			safeSearchContent := strings.ToValidUTF8(chunk.SearchContent, "")
			previous, next := neighborChunkIndexes(chunkIndex, len(chunks))
			rows = append(rows, model.SupportContentChunk{
				WorkspaceID:         source.WorkspaceID,
				ContentSourceID:     source.ID,
				PageID:              page.ID,
				ChunkIndex:          chunkIndex,
				SectionKey:          chunk.SectionKey,
				HeadingPath:         chunk.HeadingPath,
				Title:               safeTitle,
				URL:                 safeURL,
				Content:             safeChunk,
				SearchContent:       safeSearchContent,
				PreviousChunkIndex:  previous,
				NextChunkIndex:      next,
				ContentHash:         hashChunk(safeTitle, safeSearchContent),
				Embedding:           formatVector(resp.Vectors[chunkIndex]),
				EmbeddingProvider:   "openai",
				EmbeddingModel:      s.embeddingModel,
				EmbeddingVersion:    contentChunkEmbeddingVersion,
				EmbeddingDimensions: docsEmbeddingDimensions,
			})
		}
		if !validEmbeddings {
			continue
		}
		if err := s.chunkRepo.ReplacePageChunks(ctx, page.ID, rows); err != nil {
			slog.WarnContext(ctx, "skipping page: chunk upsert failed", "page_id", page.ID, "error", err)
			continue
		}

		indexedPages++
		indexedChunks += len(rows)

		pct := 0
		if len(pages) > 0 {
			pct = (indexedPages * 100) / len(pages)
		}
		if pct > 99 {
			pct = 99
		}
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, pct, indexedPages, indexedChunks, nil, nil, &startedAt, nil)
	}

	completedAt := time.Now()
	return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncReady, 100, indexedPages, indexedChunks, nil, nil, &startedAt, &completedAt)
}

func (s *SupportContentSyncService) markSourceFailed(ctx context.Context, sourceID string, err error, startedAt *time.Time, indexedPages, indexedChunks int) error {
	errMsg := err.Error()
	completedAt := time.Now()
	return s.sourceRepo.UpdateSyncState(ctx, sourceID, model.KnowledgeSourceSyncFailed, 0, indexedPages, indexedChunks, &errMsg, nil, startedAt, &completedAt)
}

// crawlRecordText extracts the best available text from a CrawlRecord.
// It prefers Markdown, then falls back to sanitized HTML.
func crawlRecordText(record crawler.CrawlRecord) (string, string) {
	if strings.TrimSpace(record.Markdown) != "" {
		return normalizeContentText(record.Markdown), model.ContentSourceFormatMarkdown
	}
	if strings.TrimSpace(record.HTML) != "" {
		return normalizeContentText(stripHTML(record.HTML)), model.ContentSourceFormatHTML
	}
	return "", ""
}

// crawlRecordTitle extracts the page title from a CrawlRecord.
func crawlRecordTitle(record crawler.CrawlRecord) string {
	if strings.TrimSpace(record.Title) != "" {
		return strings.TrimSpace(strings.ToValidUTF8(record.Title, ""))
	}
	if title, ok := record.Metadata["title"].(string); ok && strings.TrimSpace(title) != "" {
		return strings.TrimSpace(strings.ToValidUTF8(title, ""))
	}
	return strings.TrimSpace(record.URL)
}

func extractUploadedContentText(data []byte, contentType, fileName string) (string, string, error) {
	normalizedType, err := normalizeKnowledgeFileContentType(contentType, fileName)
	if err != nil {
		return "", "", err
	}
	switch normalizedType {
	case "text/plain", "text/markdown", "text/x-markdown", "text/csv", "application/json":
		return normalizeContentText(string(data)), model.ContentSourceFormatMarkdown, nil
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		text, err := extractDOCXText(data)
		if err != nil {
			return "", "", err
		}
		return normalizeContentText(text), model.ContentSourceFormatMarkdown, nil
	case "application/pdf":
		text, err := extractPDFText(data)
		if err != nil {
			return "", "", err
		}
		return normalizeContentText(text), model.ContentSourceFormatMarkdown, nil
	default:
		return "", "", fmt.Errorf("unsupported file type %s", normalizedType)
	}
}

func extractDOCXText(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open DOCX: %w", err)
	}
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		if file.UncompressedSize64 > 8*1024*1024 {
			return "", fmt.Errorf("DOCX document text is too large")
		}
		stream, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("open DOCX document: %w", err)
		}
		defer func() { _ = stream.Close() }()

		decoder := xml.NewDecoder(io.LimitReader(stream, 8*1024*1024+1))
		var text strings.Builder
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", fmt.Errorf("decode DOCX document: %w", err)
			}
			switch node := token.(type) {
			case xml.StartElement:
				if node.Name.Local == "t" {
					var value string
					if err := decoder.DecodeElement(&value, &node); err != nil {
						return "", fmt.Errorf("decode DOCX text: %w", err)
					}
					text.WriteString(value)
				} else if node.Name.Local == "tab" {
					text.WriteByte('\t')
				}
			case xml.EndElement:
				if node.Name.Local == "p" {
					text.WriteByte('\n')
				}
			}
		}
		return strings.TrimSpace(text.String()), nil
	}
	return "", fmt.Errorf("DOCX document.xml is missing")
}

func extractPDFText(data []byte) (string, error) {
	tmp, err := os.CreateTemp("", "helpin-knowledge-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create temporary PDF: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temporary PDF: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary PDF: %w", err)
	}
	file, reader, err := pdf.Open(tmpPath)
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}
	defer func() { _ = file.Close() }()
	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w", err)
	}
	payload, err := io.ReadAll(textReader)
	if err != nil {
		return "", fmt.Errorf("read PDF text: %w", err)
	}
	return string(payload), nil
}

func mustMarshalJSON(value any) json.RawMessage {
	if value == nil {
		return json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

var (
	htmlTagPattern          = regexp.MustCompile(`<[^>]+>`)
	htmlBlockPattern        = regexp.MustCompile(`(?i)</?(?:p|div|li|br|section|article|header|footer|table|tr|ul|ol)[^>]*>`)
	htmlHeadingBlockPattern = regexp.MustCompile(`(?is)<h([1-6])[^>]*>(.*?)</h[1-6]>`)
	// Markdown images: ![alt text](url) → keep alt text only.
	mdImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	// HTML <img> tags (self-closing or not).
	htmlImgPattern = regexp.MustCompile(`(?i)<img[^>]*>`)
	// Inline SVG blocks.
	svgPattern = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
	// Markdown image-only links: [![alt](img-url)](link-url) → keep alt text.
	mdImageLinkPattern = regexp.MustCompile(`\[!\[([^\]]*)\]\([^)]+\)\]\([^)]+\)`)
)

func stripHTML(raw string) string {
	withHeadings := htmlHeadingBlockPattern.ReplaceAllStringFunc(raw, func(match string) string {
		parts := htmlHeadingBlockPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		level, err := strconv.Atoi(parts[1])
		if err != nil || level < 1 || level > 6 {
			level = 2
		}
		heading := strings.Join(strings.Fields(htmlTagPattern.ReplaceAllString(parts[2], " ")), " ")
		return "\n" + strings.Repeat("#", level) + " " + heading + "\n"
	})
	withBlocks := htmlBlockPattern.ReplaceAllString(withHeadings, "\n")
	replaced := htmlTagPattern.ReplaceAllString(withBlocks, " ")
	return htmlstd.UnescapeString(replaced)
}

func normalizeContentText(value string) string {
	// Strip invalid UTF-8 bytes — crawlers sometimes return Windows-1252
	// encoded characters that PostgreSQL rejects.
	cleaned := strings.ToValidUTF8(value, "")

	// Remove images and SVGs — they are noise for text embeddings.
	cleaned = svgPattern.ReplaceAllString(cleaned, " ")
	cleaned = htmlImgPattern.ReplaceAllString(cleaned, " ")
	cleaned = mdImageLinkPattern.ReplaceAllString(cleaned, "$1")
	cleaned = mdImagePattern.ReplaceAllString(cleaned, "$1")

	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	lines := strings.Split(cleaned, "\n")
	normalized := make([]string, 0, len(lines))
	lastBlank := true
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line == "" {
			if !lastBlank {
				normalized = append(normalized, "")
				lastBlank = true
			}
			continue
		}
		normalized = append(normalized, line)
		lastBlank = false
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

// sanitizeJSONUTF8 strips invalid UTF-8 byte sequences from a JSON payload.
// Cloudflare and other crawlers may return metadata with Windows-1252 or other
// non-UTF-8 characters that PostgreSQL jsonb columns reject.
func sanitizeJSONUTF8(raw json.RawMessage) json.RawMessage {
	s := string(raw)
	cleaned := strings.ToValidUTF8(s, "")
	if len(cleaned) == len(s) {
		return raw // already valid
	}
	return json.RawMessage(cleaned)
}
