package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

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

// SupportContentSyncService keeps crawled web content indexed in pgvector.
type SupportContentSyncService struct {
	sourceRepo     *repository.SupportContentSourceRepository
	pageRepo       *repository.SupportContentPageRepository
	chunkRepo      *repository.SupportContentChunkRepository
	embedder       llm.EmbeddingProvider
	embeddingModel string
	crawler        crawler.ContentCrawler
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
		starter:        starter,
	}
}

func (s *SupportContentSyncService) QueueSourceSync(ctx context.Context, workspaceID, contentSourceID string) error {
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
	slog.InfoContext(ctx, "queueing support content source sync",
		"workspace_id", workspaceID,
		"content_source_id", source.ID,
		"start_url", source.StartURL,
		"crawl_limit", source.CrawlLimit,
		"crawl_depth", source.CrawlDepth,
		"crawl_source", source.CrawlSource,
	)

	if s.embedder == nil {
		msg := "OpenAI-compatible embedding provider is not configured"
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
		return nil
	}
	if s.crawler == nil {
		msg := "content crawler is not configured"
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
	if err := s.starter.QueueContentSourceSync(ctx, workspaceID, contentSourceID); err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, nil, source.IndexedPages, source.IndexedChunks)
		return err
	}
	return nil
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
	if s.embedder == nil {
		msg := "OpenAI-compatible embedding provider is not configured"
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, source.IndexedPages, source.IndexedChunks, &msg, nil, nil, nil)
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

	onPage := func(record crawler.CrawlRecord) error {
		contentText, format := crawlRecordText(record)
		if strings.TrimSpace(contentText) == "" {
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

		chunks := chunkDocumentText(contentText)
		if len(chunks) == 0 {
			return s.chunkRepo.ReplacePageChunks(ctx, savedPage.ID, nil)
		}

		resp, err := s.embedder.CreateEmbeddings(ctx, llm.EmbeddingRequest{
			Provider: "openai",
			Model:    s.embeddingModel,
			Inputs:   chunks,
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
			safeChunk := strings.ToValidUTF8(chunk, "")
			rows = append(rows, model.SupportContentChunk{
				WorkspaceID:     source.WorkspaceID,
				ContentSourceID: source.ID,
				PageID:          savedPage.ID,
				ChunkIndex:      chunkIndex,
				Title:           safeTitle,
				URL:             safeURL,
				Content:         safeChunk,
				ContentHash:     hashChunk(safeTitle, safeChunk),
				Embedding:       formatVector(resp.Vectors[chunkIndex]),
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
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, pct, source.IndexedPages, source.IndexedChunks, nil, nil, &startedAt, nil)
		return nil
	}

	_, err = s.crawler.Crawl(ctx, *source, onPage)
	if err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt, source.IndexedPages, source.IndexedChunks)
		return err
	}

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
	if s.embedder == nil {
		msg := "OpenAI-compatible embedding provider is not configured"
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
	if s.embedder == nil {
		msg := "OpenAI-compatible embedding provider is not configured"
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

		chunks := chunkDocumentText(page.ContentText)
		if len(chunks) == 0 {
			_ = s.chunkRepo.ReplacePageChunks(ctx, page.ID, nil)
			continue
		}

		resp, err := s.embedder.CreateEmbeddings(ctx, llm.EmbeddingRequest{
			Provider: "openai",
			Model:    s.embeddingModel,
			Inputs:   chunks,
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
			safeChunk := strings.ToValidUTF8(chunk, "")
			rows = append(rows, model.SupportContentChunk{
				WorkspaceID:     source.WorkspaceID,
				ContentSourceID: source.ID,
				PageID:          page.ID,
				ChunkIndex:      chunkIndex,
				Title:           safeTitle,
				URL:             safeURL,
				Content:         safeChunk,
				ContentHash:     hashChunk(safeTitle, safeChunk),
				Embedding:       formatVector(resp.Vectors[chunkIndex]),
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
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, pct, source.IndexedPages, source.IndexedChunks, nil, nil, &startedAt, nil)
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
	htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
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
	replaced := htmlTagPattern.ReplaceAllString(raw, " ")
	replaced = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'", "&quot;", `"`).Replace(replaced)
	return replaced
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

	return strings.Join(strings.Fields(strings.TrimSpace(cleaned)), " ")
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
