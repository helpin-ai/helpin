package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	defaultDocsEmbeddingModel    = "text-embedding-3-small"
	contentChunkEmbeddingVersion = "content-chunk-v1"
	docsEmbeddingDimensions      = 1536
	chunkSizeChars               = 1200
	chunkOverlapChars            = 200
)

// DocsEmbeddingService keeps pgvector-backed support knowledge chunks in sync.
type DocsEmbeddingService struct {
	chunkRepo      *repository.DocsChunkRepository
	blockRepo      *repository.DocsBlockRepository
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
	blockRepo *repository.DocsBlockRepository,
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
		blockRepo:      blockRepo,
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

// QueueKnowledgeSourceSync schedules a sync for one selected docs source.
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

// QueueDocumentSync schedules a sync for the containing docs space after a doc change.
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

// QueueSpaceSync schedules a sync for all selected knowledge sources on a docs space.
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
	if space == nil || space.WorkspaceID != workspaceID || !isSupportKnowledgeSpaceType(space.Type) {
		msg := "Only internal and help center spaces can be indexed for support AI"
		return s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncDisabled, 0, 0, 0, &msg, nil, nil)
	}

	startedAt := time.Now()
	if err := s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncRunning, 0, 0, 0, nil, &startedAt, nil); err != nil {
		return err
	}

	eligibleDocs, err := s.listEligibleDocuments(ctx, workspaceID, *space)
	if err != nil {
		_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
		return err
	}

	keepDocumentIDs := make([]string, 0, len(eligibleDocs))
	totalChunks := 0

	for idx, doc := range eligibleDocs {
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
		blockIDs := make([]*string, len(chunks))
		if s.blockRepo != nil {
			if blockChunks, ids, err := s.blockChunks(ctx, doc.ID); err != nil {
				_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
				return err
			} else if len(blockChunks) > 0 {
				chunks = blockChunks
				blockIDs = ids
			}
		}
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
				WorkspaceID:         workspaceID,
				SpaceID:             spaceID,
				DocumentID:          doc.ID,
				BlockID:             blockIDs[chunkIndex],
				ChunkIndex:          chunkIndex,
				Title:               doc.Title,
				Content:             chunk,
				ContentHash:         hashChunk(doc.Title, chunk),
				Embedding:           formatVector(resp.Vectors[chunkIndex]),
				EmbeddingProvider:   "openai",
				EmbeddingModel:      s.embeddingModel,
				EmbeddingVersion:    contentChunkEmbeddingVersion,
				EmbeddingDimensions: docsEmbeddingDimensions,
			})
		}
		if err := s.chunkRepo.ReplaceDocumentChunks(ctx, doc.ID, rows); err != nil {
			_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
			return err
		}

		totalChunks += len(rows)
		progress := progressFor(idx+1, len(eligibleDocs))
		if err := s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncRunning, progress, idx+1, totalChunks, nil, &startedAt, nil); err != nil {
			return err
		}
	}

	if err := s.chunkRepo.DeleteBySpaceExceptDocuments(ctx, workspaceID, spaceID, keepDocumentIDs); err != nil {
		_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
		return err
	}

	completedAt := time.Now()
	return s.updateAllSyncStates(ctx, sources, model.KnowledgeSourceSyncReady, 100, len(eligibleDocs), totalChunks, nil, &startedAt, &completedAt)
}

func (s *DocsEmbeddingService) listEligibleDocuments(ctx context.Context, workspaceID string, space model.DocsSpace) ([]model.DocsDocument, error) {
	switch space.Type {
	case model.SpaceTypeInternal:
		if s.documentRepo == nil {
			return nil, fmt.Errorf("docs document repository is not configured")
		}
		return s.documentRepo.ListPublishedBySpace(ctx, workspaceID, space.ID)
	case model.SpaceTypeExternalCapable:
		if s.helpcenterRepo == nil {
			return nil, fmt.Errorf("docs help center repository is not configured")
		}
		return s.helpcenterRepo.ListPublicDocumentsBySpace(ctx, workspaceID, space.ID)
	default:
		return nil, fmt.Errorf("unsupported docs space type %q", space.Type)
	}
}

// RunSpaceSync performs a single full sync for a selected docs space.
// This is intended to run inside a durable Temporal activity.
func (s *DocsEmbeddingService) RunSpaceSync(ctx context.Context, workspaceID, spaceID string) error {
	if s == nil {
		return nil
	}
	return s.syncSpace(ctx, workspaceID, spaceID)
}

func (s *DocsEmbeddingService) blockChunks(ctx context.Context, documentID string) ([]string, []*string, error) {
	blocks, err := s.blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		return nil, nil, err
	}
	chunks := []string{}
	blockIDs := []*string{}
	for _, block := range blocks {
		text := strings.TrimSpace(block.ContentText)
		if text == "" {
			text = strings.TrimSpace(extractEmbeddableBlockText(block.Content))
		}
		if text == "" {
			continue
		}
		parts := chunkDocumentText(text)
		if len(parts) == 0 {
			continue
		}
		for _, part := range parts {
			id := block.ID
			chunks = append(chunks, part)
			blockIDs = append(blockIDs, &id)
		}
	}
	return chunks, blockIDs, nil
}

func extractEmbeddableBlockText(raw []byte) string {
	var node map[string]json.RawMessage
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	var sb strings.Builder
	extractTextFromEmbeddableNode(node, &sb)
	return sb.String()
}

func extractTextFromEmbeddableNode(node map[string]json.RawMessage, sb *strings.Builder) {
	if textRaw, ok := node["text"]; ok {
		var text string
		if err := json.Unmarshal(textRaw, &text); err == nil {
			sb.WriteString(text)
		}
	}
	if attrsRaw, ok := node["attrs"]; ok {
		var attrs map[string]json.RawMessage
		if err := json.Unmarshal(attrsRaw, &attrs); err == nil {
			if htmlRaw, ok := attrs["html"]; ok {
				var htmlStr string
				if err := json.Unmarshal(htmlRaw, &htmlStr); err == nil && strings.TrimSpace(htmlStr) != "" {
					sb.WriteString(stripEmbeddableHTMLTags(htmlStr))
					sb.WriteString(" ")
				}
			}
		}
	}
	if contentRaw, ok := node["content"]; ok {
		var children []map[string]json.RawMessage
		if err := json.Unmarshal(contentRaw, &children); err == nil {
			for _, child := range children {
				extractTextFromEmbeddableNode(child, sb)
				sb.WriteString(" ")
			}
		}
	}
}

func stripEmbeddableHTMLTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
			sb.WriteByte(' ')
		default:
			if !inTag {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
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
