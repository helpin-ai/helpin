package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	defaultDocsEmbeddingModel = "text-embedding-3-small"
	docsEmbeddingDimensions   = 1536
	chunkSizeChars            = 1200
	chunkOverlapChars         = 200
)

// DocsEmbeddingService keeps pgvector-backed help-center chunks in sync.
type DocsEmbeddingService struct {
	chunkRepo      *repository.DocsChunkRepository
	knowledgeRepo  *repository.AgentKnowledgeSourceRepository
	contentRepo    *repository.DocsContentRepository
	spaceRepo      *repository.DocsSpaceRepository
	helpcenterRepo *repository.DocsHelpcenterRepository
	documentRepo   *repository.DocsDocumentRepository
	embedder       llm.EmbeddingProvider
	embeddingModel string
	starter        DocsEmbeddingWorkflowStarter
}

// DocsEmbeddingWorkflowStarter queues durable indexing work.
type DocsEmbeddingWorkflowStarter interface {
	QueueDocsEmbeddingSync(ctx context.Context, workspaceID, spaceID string) error
}

// NewDocsEmbeddingService creates a new DocsEmbeddingService.
func NewDocsEmbeddingService(
	chunkRepo *repository.DocsChunkRepository,
	knowledgeRepo *repository.AgentKnowledgeSourceRepository,
	contentRepo *repository.DocsContentRepository,
	spaceRepo *repository.DocsSpaceRepository,
	helpcenterRepo *repository.DocsHelpcenterRepository,
	documentRepo *repository.DocsDocumentRepository,
	embedder llm.EmbeddingProvider,
	embeddingModel string,
	starter DocsEmbeddingWorkflowStarter,
) *DocsEmbeddingService {
	if strings.TrimSpace(embeddingModel) == "" {
		embeddingModel = defaultDocsEmbeddingModel
	}
	return &DocsEmbeddingService{
		chunkRepo:      chunkRepo,
		knowledgeRepo:  knowledgeRepo,
		contentRepo:    contentRepo,
		spaceRepo:      spaceRepo,
		helpcenterRepo: helpcenterRepo,
		documentRepo:   documentRepo,
		embedder:       embedder,
		embeddingModel: strings.TrimSpace(embeddingModel),
		starter:        starter,
	}
}

// QueueKnowledgeSourceSync schedules a sync for one selected help-center source.
func (s *DocsEmbeddingService) QueueKnowledgeSourceSync(ctx context.Context, knowledgeSourceID string) error {
	if s == nil || s.knowledgeRepo == nil {
		return nil
	}
	source, err := s.knowledgeRepo.GetByID(ctx, knowledgeSourceID)
	if err != nil || source == nil {
		return err
	}
	return s.QueueSpaceSync(ctx, source.WorkspaceID, source.SpaceID)
}

// QueueDocumentSync schedules a sync for the containing help-center space after a doc change.
func (s *DocsEmbeddingService) QueueDocumentSync(ctx context.Context, documentID string) error {
	if s == nil || s.documentRepo == nil {
		return nil
	}
	doc, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil || doc == nil {
		return err
	}
	return s.QueueSpaceSync(ctx, doc.WorkspaceID, doc.SpaceID)
}

// QueueSpaceSync schedules a sync for all selected knowledge sources on a help-center space.
func (s *DocsEmbeddingService) QueueSpaceSync(ctx context.Context, workspaceID, spaceID string) error {
	if s == nil || s.knowledgeRepo == nil {
		return nil
	}
	sources, err := s.knowledgeRepo.ListBySpaceID(ctx, workspaceID, spaceID)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return nil
	}

	if s.embedder == nil {
		errMsg := "OpenAI-compatible embedding provider is not configured"
		for _, source := range sources {
			msg := errMsg
			_ = s.knowledgeRepo.UpdateSyncState(ctx, source.ID, model.KnowledgeSourceSyncDisabled, 0, 0, 0, &msg, nil, nil)
		}
		return nil
	}

	for _, source := range sources {
		_ = s.knowledgeRepo.MarkSyncQueued(ctx, source.ID)
	}

	if s.starter == nil {
		err = fmt.Errorf("Temporal embedding pipeline is not configured")
		_ = s.markSourcesFailed(ctx, sources, err, nil)
		return err
	}

	if err := s.starter.QueueDocsEmbeddingSync(ctx, workspaceID, spaceID); err != nil {
		_ = s.markSourcesFailed(ctx, sources, err, nil)
		return err
	}
	return nil
}

func (s *DocsEmbeddingService) syncSpace(ctx context.Context, workspaceID, spaceID string) error {
	sources, err := s.knowledgeRepo.ListBySpaceID(ctx, workspaceID, spaceID)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return nil
	}

	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return err
	}
	if space == nil || space.Type != model.SpaceTypeExternalCapable {
		msg := "Only help center spaces can be indexed for support AI"
		return s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncDisabled, 0, 0, 0, &msg, nil, nil)
	}

	startedAt := time.Now()
	if err := s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncRunning, 0, 0, 0, nil, &startedAt, nil); err != nil {
		return err
	}

	publicDocs, err := s.helpcenterRepo.ListPublicDocumentsBySpace(ctx, workspaceID, spaceID)
	if err != nil {
		_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
		return err
	}

	keepDocumentIDs := make([]string, 0, len(publicDocs))
	totalChunks := 0

	for idx, doc := range publicDocs {
		keepDocumentIDs = append(keepDocumentIDs, doc.ID)

		content, err := s.contentRepo.GetByDocumentID(ctx, doc.ID)
		if err != nil {
			_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
			return err
		}
		if content == nil || strings.TrimSpace(content.ContentText) == "" {
			if err := s.chunkRepo.DeleteByDocumentID(ctx, doc.ID); err != nil {
				_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
				return err
			}
			continue
		}

		chunks := chunkDocumentText(content.ContentText)
		if len(chunks) == 0 {
			if err := s.chunkRepo.DeleteByDocumentID(ctx, doc.ID); err != nil {
				_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
				return err
			}
			continue
		}

		resp, err := s.embedder.CreateEmbeddings(ctx, llm.EmbeddingRequest{
			Provider: "openai",
			Model:    s.embeddingModel,
			Inputs:   chunks,
		})
		if err != nil {
			_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
			return err
		}
		if len(resp.Vectors) != len(chunks) {
			err = fmt.Errorf("embedding count mismatch for document %s", doc.ID)
			_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
			return err
		}
		for _, vector := range resp.Vectors {
			if len(vector) != docsEmbeddingDimensions {
				err = fmt.Errorf("embedding dimension mismatch for document %s: got %d want %d", doc.ID, len(vector), docsEmbeddingDimensions)
				_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
				return err
			}
		}

		rows := make([]model.DocsChunk, 0, len(chunks))
		for chunkIndex, chunk := range chunks {
			rows = append(rows, model.DocsChunk{
				WorkspaceID: workspaceID,
				SpaceID:     spaceID,
				DocumentID:  doc.ID,
				ChunkIndex:  chunkIndex,
				Title:       doc.Title,
				Content:     chunk,
				ContentHash: hashChunk(doc.Title, chunk),
				Embedding:   formatVector(resp.Vectors[chunkIndex]),
			})
		}
		if err := s.chunkRepo.ReplaceDocumentChunks(ctx, doc.ID, rows); err != nil {
			_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
			return err
		}

		totalChunks += len(rows)
		progress := progressFor(idx+1, len(publicDocs))
		if err := s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncRunning, progress, idx+1, totalChunks, nil, &startedAt, nil); err != nil {
			return err
		}
	}

	if err := s.chunkRepo.DeleteBySpaceExceptDocuments(ctx, workspaceID, spaceID, keepDocumentIDs); err != nil {
		_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
		return err
	}

	completedAt := time.Now()
	return s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncReady, 100, len(publicDocs), totalChunks, nil, &startedAt, &completedAt)
}

// RunSpaceSync performs a single full sync for a help-center space.
// This is intended to run inside a durable Temporal activity.
func (s *DocsEmbeddingService) RunSpaceSync(ctx context.Context, workspaceID, spaceID string) error {
	if s == nil {
		return nil
	}
	return s.syncSpace(ctx, workspaceID, spaceID)
}

func (s *DocsEmbeddingService) updateAllSyncStates(
	ctx context.Context,
	sources []model.AgentKnowledgeSource,
	status string,
	progress int,
	documentCount int,
	chunkCount int,
	errMessage *string,
	startedAt *time.Time,
	completedAt *time.Time,
) error {
	for _, source := range sources {
		if err := s.knowledgeRepo.UpdateSyncState(ctx, source.ID, status, progress, documentCount, chunkCount, errMessage, startedAt, completedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *DocsEmbeddingService) markSourcesFailed(ctx context.Context, sources []model.AgentKnowledgeSource, err error, startedAt *time.Time) error {
	errMsg := err.Error()
	completedAt := time.Now()
	return s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncFailed, 0, 0, 0, &errMsg, startedAt, &completedAt)
}

func chunkDocumentText(text string) []string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(strings.ToValidUTF8(text, ""))), " ")
	if normalized == "" {
		return nil
	}
	if len(normalized) <= chunkSizeChars {
		return []string{normalized}
	}

	var chunks []string
	for start := 0; start < len(normalized); {
		end := start + chunkSizeChars
		if end >= len(normalized) {
			chunks = append(chunks, strings.TrimSpace(normalized[start:]))
			break
		}

		// Avoid splitting multi-byte UTF-8 characters at slice boundaries.
		end = alignRuneBoundary(normalized, end)

		midpoint := alignRuneBoundary(normalized, start+chunkSizeChars/2)
		if midpoint >= end {
			midpoint = start
		}
		split := strings.LastIndexAny(normalized[midpoint:end], ".!?\n ")
		if split > 0 {
			end = midpoint + split + 1
		}

		chunk := strings.TrimSpace(normalized[start:end])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		nextStart := end - chunkOverlapChars
		if nextStart <= start {
			nextStart = end
		}
		nextStart = alignRuneBoundary(normalized, nextStart)
		start = nextStart
	}
	return chunks
}

// alignRuneBoundary advances pos to the next UTF-8 rune start if it currently
// falls inside a multi-byte sequence. This prevents slicing a string in the
// middle of a character, which would produce invalid UTF-8.
func alignRuneBoundary(s string, pos int) int {
	if pos >= len(s) {
		return len(s)
	}
	for pos < len(s) && !utf8.RuneStart(s[pos]) {
		pos++
	}
	return pos
}

func hashChunk(title, content string) string {
	sum := sha256.Sum256([]byte(title + "\n" + content))
	return hex.EncodeToString(sum[:])
}

func formatVector(vector []float32) string {
	if len(vector) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.Grow(len(vector) * 8)
	sb.WriteByte('[')
	for idx, value := range vector {
		if idx > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%g", value))
	}
	sb.WriteByte(']')
	return sb.String()
}

func progressFor(done, total int) int {
	if total <= 0 {
		return 100
	}
	progress := int(float64(done) / float64(total) * 100)
	if progress > 100 {
		return 100
	}
	if progress < 0 {
		return 0
	}
	return progress
}
