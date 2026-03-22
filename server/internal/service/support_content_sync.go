package service

import (
	"context"
	"encoding/json"
	"fmt"
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

	if s.embedder == nil {
		msg := "OpenAI-compatible embedding provider is not configured"
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, 0, 0, &msg, nil, nil, nil)
		return nil
	}
	if s.crawler == nil {
		msg := "content crawler is not configured"
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, 0, 0, &msg, nil, nil, nil)
		return nil
	}
	if err := s.sourceRepo.MarkSyncQueued(ctx, source.ID); err != nil {
		return err
	}
	if s.starter == nil {
		err = fmt.Errorf("Temporal content sync pipeline is not configured")
		_ = s.markSourceFailed(ctx, source.ID, err, nil)
		return err
	}
	if err := s.starter.QueueContentSourceSync(ctx, workspaceID, contentSourceID); err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, nil)
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
	if s.embedder == nil {
		msg := "OpenAI-compatible embedding provider is not configured"
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, 0, 0, &msg, nil, nil, nil)
	}
	if s.crawler == nil {
		msg := "content crawler is not configured"
		return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, 0, 0, &msg, nil, nil, nil)
	}

	startedAt := time.Now()
	if err := s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, 0, 0, 0, nil, nil, &startedAt, nil); err != nil {
		return err
	}

	// Mutable state shared with the onPage callback.
	var (
		keepPageIDs  []string
		keepURLs     []string
		indexedPages int
		indexedChunks int
	)

	onPage := func(record crawler.CrawlRecord) error {
		contentText, format := crawlRecordText(record)
		if strings.TrimSpace(contentText) == "" {
			return nil
		}

		title := crawlRecordTitle(record)
		page := &model.SupportContentPage{
			WorkspaceID:     source.WorkspaceID,
			ContentSourceID: source.ID,
			URL:             strings.TrimSpace(record.URL),
			Title:           title,
			HTTPStatus:      record.HTTPStatus,
			ContentFormat:   format,
			ContentText:     contentText,
			ContentHash:     hashChunk(title+"\n"+strings.TrimSpace(record.URL), contentText),
			Metadata:        mustMarshalJSON(record.Metadata),
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
			return err
		}
		if len(resp.Vectors) != len(chunks) {
			return fmt.Errorf("embedding count mismatch for content page %s", savedPage.ID)
		}
		rows := make([]model.SupportContentChunk, 0, len(chunks))
		for chunkIndex, chunk := range chunks {
			if len(resp.Vectors[chunkIndex]) != docsEmbeddingDimensions {
				return fmt.Errorf("embedding dimension mismatch for content page %s: got %d want %d", savedPage.ID, len(resp.Vectors[chunkIndex]), docsEmbeddingDimensions)
			}
			rows = append(rows, model.SupportContentChunk{
				WorkspaceID:     source.WorkspaceID,
				ContentSourceID: source.ID,
				PageID:          savedPage.ID,
				ChunkIndex:      chunkIndex,
				Title:           savedPage.Title,
				URL:             savedPage.URL,
				Content:         chunk,
				ContentHash:     hashChunk(savedPage.Title, chunk),
				Embedding:       formatVector(resp.Vectors[chunkIndex]),
			})
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
		_ = s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncRunning, pct, indexedPages, indexedChunks, nil, nil, &startedAt, nil)
		return nil
	}

	_, err = s.crawler.Crawl(ctx, *source, onPage)
	if err != nil {
		_ = s.markSourceFailed(ctx, source.ID, err, &startedAt)
		return err
	}

	// Clean up stale pages that were not seen in this crawl (full crawl only).
	if source.ModifiedSince == nil {
		if err := s.chunkRepo.DeleteByContentSourceExceptPages(ctx, source.WorkspaceID, source.ID, keepPageIDs); err != nil {
			_ = s.markSourceFailed(ctx, source.ID, err, &startedAt)
			return err
		}
		if err := s.pageRepo.DeleteByContentSourceExceptURLs(ctx, source.WorkspaceID, source.ID, keepURLs); err != nil {
			_ = s.markSourceFailed(ctx, source.ID, err, &startedAt)
			return err
		}
	}

	completedAt := time.Now()
	return s.sourceRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncReady, 100, indexedPages, indexedChunks, nil, nil, &startedAt, &completedAt)
}

func (s *SupportContentSyncService) markSourceFailed(ctx context.Context, sourceID string, err error, startedAt *time.Time) error {
	errMsg := err.Error()
	completedAt := time.Now()
	return s.sourceRepo.UpdateSyncState(ctx, sourceID, model.KnowledgeSourceSyncFailed, 0, 0, 0, &errMsg, nil, startedAt, &completedAt)
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

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func stripHTML(raw string) string {
	replaced := htmlTagPattern.ReplaceAllString(raw, " ")
	replaced = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'", "&quot;", `"`).Replace(replaced)
	return replaced
}

func normalizeContentText(value string) string {
	// Strip invalid UTF-8 bytes — crawlers sometimes return Windows-1252
	// encoded characters that PostgreSQL rejects.
	cleaned := strings.ToValidUTF8(value, "")
	return strings.Join(strings.Fields(strings.TrimSpace(cleaned)), " ")
}
