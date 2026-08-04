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
	contentChunkEmbeddingVersion = "content-chunk-v2"
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
	collectionRepo *repository.DocsCollectionRepository
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
	return NewDocsEmbeddingServiceWithCollections(chunkRepo, blockRepo, knowledgeRepo, contentRepo, spaceRepo, nil, helpcenterRepo, documentRepo, embedder, embeddingModel, starter)
}

// NewDocsEmbeddingServiceWithCollections creates an embedding service that
// also calculates per-collection knowledge-source sync statistics.
func NewDocsEmbeddingServiceWithCollections(
	chunkRepo *repository.DocsChunkRepository,
	blockRepo *repository.DocsBlockRepository,
	knowledgeRepo *repository.AgentKnowledgeSourceRepository,
	contentRepo *repository.DocsContentRepository,
	spaceRepo *repository.DocsSpaceRepository,
	collectionRepo *repository.DocsCollectionRepository,
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
		collectionRepo: collectionRepo,
		helpcenterRepo: helpcenterRepo,
		documentRepo:   documentRepo,
		embedder:       embedder,
		embeddingModel: strings.TrimSpace(embeddingModel),
		starter:        starter,
	}
}

// spaceIsAutoIndexable reports whether a space should be chunked even without
// any agent knowledge-source link: live external-capable (help center) spaces.
func (s *DocsEmbeddingService) spaceIsAutoIndexable(ctx context.Context, workspaceID, spaceID string) bool {
	if s.spaceRepo == nil {
		return false
	}
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil || space == nil || space.WorkspaceID != workspaceID {
		return false
	}
	return space.Type == model.SpaceTypeExternalCapable && space.DeletedAt == nil
}

// QueueHelpcenterAutoIndex queues an embedding sync for every external-capable
// space in the workspace. Used as a lazy backfill for workspaces that
// published a help center before auto-indexing existed (or before any agent
// linked their spaces).
func (s *DocsEmbeddingService) QueueHelpcenterAutoIndex(ctx context.Context, workspaceID string) error {
	if s == nil || s.spaceRepo == nil {
		return nil
	}
	spaces, err := s.spaceRepo.ListPublicByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, space := range spaces {
		if err := s.QueueSpaceSync(ctx, workspaceID, space.ID); err != nil {
			return err
		}
	}
	return nil
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

// QueueSpaceSync schedules a sync for a docs space. Spaces linked as agent
// knowledge sources always sync; help-center (external-capable) spaces sync
// even without any linked agent — public semantic search and AI answers
// depend on their chunks, so indexing is not gated on agent configuration.
func (s *DocsEmbeddingService) QueueSpaceSync(ctx context.Context, workspaceID, spaceID string) error {
	if s == nil || s.knowledgeRepo == nil {
		return nil
	}
	sources, err := s.knowledgeRepo.ListBySpaceID(ctx, workspaceID, spaceID)
	if err != nil {
		return err
	}
	if len(sources) == 0 && !s.spaceIsAutoIndexable(ctx, workspaceID, spaceID) {
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
	if len(sources) == 0 && !s.spaceIsAutoIndexable(ctx, workspaceID, spaceID) {
		return nil
	}

	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return err
	}
	if space == nil || space.WorkspaceID != workspaceID || !isDocsKnowledgeSpaceType(space.Type) {
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
	chunksByDocumentID := make(map[string]int, len(eligibleDocs))

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
			chunksByDocumentID[doc.ID] = 0
			continue
		}

		chunks := chunkStructuredDocument(doc.Title, content.ContentText)
		if s.blockRepo != nil {
			if blocks, err := s.blockRepo.ListByDocument(ctx, doc.ID, false); err != nil {
				_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
				return err
			} else if structuredBlocks := chunkStructuredBlocks(doc.Title, blocks); len(structuredBlocks) > 0 {
				chunks = structuredBlocks
			}
		}
		if len(chunks) == 0 {
			if err := s.chunkRepo.DeleteByDocumentID(ctx, doc.ID); err != nil {
				_ = s.markSourcesFailed(ctx, sources, err, &startedAt)
				return err
			}
			chunksByDocumentID[doc.ID] = 0
			continue
		}

		resp, err := s.embedder.CreateEmbeddings(ctx, llm.EmbeddingRequest{
			Provider: "openai",
			Model:    s.embeddingModel,
			Inputs:   structuredChunkSearchInputs(chunks),
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
			previous, next := neighborChunkIndexes(chunkIndex, len(chunks))
			rows = append(rows, model.DocsChunk{
				WorkspaceID:         workspaceID,
				SpaceID:             spaceID,
				DocumentID:          doc.ID,
				BlockID:             chunk.BlockID,
				ChunkIndex:          chunkIndex,
				SectionKey:          chunk.SectionKey,
				HeadingPath:         chunk.HeadingPath,
				Title:               doc.Title,
				Content:             chunk.Content,
				SearchContent:       chunk.SearchContent,
				PreviousChunkIndex:  previous,
				NextChunkIndex:      next,
				ContentHash:         hashChunk(doc.Title, chunk.SearchContent),
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
		chunksByDocumentID[doc.ID] = len(rows)
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
	return s.updateScopedSyncStates(ctx, sources, eligibleDocs, chunksByDocumentID, model.KnowledgeSourceSyncReady, 100, nil, &startedAt, &completedAt)
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

func (s *DocsEmbeddingService) listIndexableDocumentsBySpace(ctx context.Context, workspaceID string, space *model.DocsSpace) ([]model.DocsDocument, error) {
	if space.Type == model.SpaceTypeExternalCapable {
		return s.helpcenterRepo.ListPublicDocumentsBySpace(ctx, workspaceID, space.ID)
	}
	status := model.DocStatusPublished
	return s.documentRepo.List(ctx, workspaceID, &space.ID, nil, &status, nil, "", false)
}

func (s *DocsEmbeddingService) updateScopedSyncStates(
	ctx context.Context,
	sources []model.AgentKnowledgeSource,
	docs []model.DocsDocument,
	chunksByDocumentID map[string]int,
	status string,
	progress int,
	errMessage *string,
	startedAt *time.Time,
	completedAt *time.Time,
) error {
	collectionDescendants := map[string]map[string]struct{}{}
	if s.collectionRepo != nil && len(sources) > 0 {
		spaceCollections := map[string][]model.DocsCollection{}
		for _, source := range sources {
			if source.CollectionID == nil || *source.CollectionID == "" {
				continue
			}
			if _, ok := spaceCollections[source.SpaceID]; !ok {
				collections, err := s.collectionRepo.ListBySpace(ctx, source.SpaceID)
				if err != nil {
					return err
				}
				spaceCollections[source.SpaceID] = collections
			}
			collectionDescendants[*source.CollectionID] = docsCollectionDescendantSet(*source.CollectionID, spaceCollections[source.SpaceID])
		}
	}

	for _, source := range sources {
		documentCount := 0
		chunkCount := 0
		for _, doc := range docs {
			if !docsKnowledgeSourceMatchesDocument(source, doc, collectionDescendants) {
				continue
			}
			documentCount++
			chunkCount += chunksByDocumentID[doc.ID]
		}
		if err := s.knowledgeRepo.UpdateSyncState(ctx, source.ID, status, progress, documentCount, chunkCount, errMessage, startedAt, completedAt); err != nil {
			return err
		}
	}
	return nil
}

func docsKnowledgeSourceMatchesDocument(source model.AgentKnowledgeSource, doc model.DocsDocument, collectionDescendants map[string]map[string]struct{}) bool {
	scopeType := source.ScopeType
	if scopeType == "" {
		scopeType = model.KnowledgeSourceScopeSpace
	}
	switch scopeType {
	case model.KnowledgeSourceScopeSpace:
		return doc.SpaceID == source.SpaceID
	case model.KnowledgeSourceScopeCollection:
		if source.CollectionID == nil || doc.CollectionID == nil {
			return false
		}
		if *doc.CollectionID == *source.CollectionID {
			return true
		}
		descendants := collectionDescendants[*source.CollectionID]
		_, ok := descendants[*doc.CollectionID]
		return ok
	case model.KnowledgeSourceScopeArticle:
		return source.DocumentID != nil && doc.ID == *source.DocumentID
	default:
		return false
	}
}

func docsCollectionDescendantSet(rootID string, collections []model.DocsCollection) map[string]struct{} {
	children := map[string][]string{}
	for _, collection := range collections {
		if collection.ParentCollectionID == nil {
			continue
		}
		children[*collection.ParentCollectionID] = append(children[*collection.ParentCollectionID], collection.ID)
	}
	result := map[string]struct{}{}
	queue := append([]string{}, children[rootID]...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, seen := result[id]; seen {
			continue
		}
		result[id] = struct{}{}
		queue = append(queue, children[id]...)
	}
	return result
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
