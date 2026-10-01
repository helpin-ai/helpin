package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SupportContentSourceService manages workspace content sources and sync queueing.
type SupportContentSourceService struct {
	repo        *repository.SupportContentSourceRepository
	agentRepo   *repository.AgentRepository
	linkRepo    *repository.AgentContentSourceRepository
	pageRepo    *repository.SupportContentPageRepository
	chunkRepo   *repository.SupportContentChunkRepository
	syncService *SupportContentSyncService
	objectStore supportContentObjectStore
}

func NewSupportContentSourceService(
	repo *repository.SupportContentSourceRepository,
	agentRepo *repository.AgentRepository,
	linkRepo *repository.AgentContentSourceRepository,
	pageRepo *repository.SupportContentPageRepository,
	chunkRepo *repository.SupportContentChunkRepository,
	syncService *SupportContentSyncService,
	objectStore supportContentObjectStore,
) *SupportContentSourceService {
	return &SupportContentSourceService{
		repo:        repo,
		agentRepo:   agentRepo,
		linkRepo:    linkRepo,
		pageRepo:    pageRepo,
		chunkRepo:   chunkRepo,
		syncService: syncService,
		objectStore: objectStore,
	}
}

func (s *SupportContentSourceService) List(ctx context.Context, workspaceID string) ([]model.SupportContentSource, error) {
	sources, err := s.repo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	next := model.NextContentSourceAutoSync(time.Now())
	for i := range sources {
		if sources[i].SourceType == model.ContentSourceTypeWebsite && sources[i].SyncStatus != model.KnowledgeSourceSyncDisabled {
			sources[i].NextSyncAt = &next
		}
	}
	return sources, nil
}

func (s *SupportContentSourceService) Create(ctx context.Context, workspaceID string, req model.CreateSupportContentSourceRequest) (*model.SupportContentSource, error) {
	source, err := normalizeSupportContentSourceCreate(workspaceID, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, source); err != nil {
		return nil, err
	}
	if s.syncService != nil {
		if err := s.syncService.QueueSourceSync(ctx, workspaceID, source.ID); err != nil {
			return nil, err
		}
	}
	return s.repo.GetByID(ctx, source.ID)
}

func (s *SupportContentSourceService) CreateFileUpload(ctx context.Context, workspaceID string, req model.CreateSupportContentSourceFileUploadRequest) (*model.CreateSupportContentSourceFileUploadResponse, error) {
	if s.objectStore == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}

	sourceID := uuid.NewString()
	req.StorageKey = buildSupportContentFileStorageKey(workspaceID, sourceID, req.FileName)
	source, err := normalizeSupportContentFileUploadRequest(workspaceID, req)
	if err != nil {
		return nil, err
	}
	source.ID = sourceID
	if err := s.repo.Create(ctx, source); err != nil {
		return nil, err
	}

	uploadURL, err := s.objectStore.GeneratePresignedPutURL(*source.StorageKey, *source.ContentType, source.FileSize, s.objectStore.HasPublicURL())
	if err != nil {
		_ = s.repo.Delete(ctx, source.ID)
		return nil, fmt.Errorf("generate upload URL: %w", err)
	}
	return &model.CreateSupportContentSourceFileUploadResponse{
		Source:    *source,
		UploadURL: uploadURL,
	}, nil
}

func (s *SupportContentSourceService) ConfirmFileUpload(ctx context.Context, workspaceID, id string) (*model.SupportContentSource, error) {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if source == nil || source.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("content source not found in workspace")
	}
	if source.SourceType != model.ContentSourceTypeFile {
		return nil, fmt.Errorf("content source is not a file source")
	}
	if s.syncService != nil {
		if err := s.syncService.QueueSourceSync(ctx, workspaceID, id); err != nil {
			return nil, err
		}
	}
	return s.repo.GetByID(ctx, id)
}

func (s *SupportContentSourceService) Update(ctx context.Context, workspaceID, id string, req model.UpdateSupportContentSourceRequest) (*model.SupportContentSource, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("content source not found in workspace")
	}
	if existing.SourceType == model.ContentSourceTypeFile {
		return nil, fmt.Errorf("file sources cannot be edited")
	}

	updates, err := normalizeSupportContentSourceUpdate(*existing, req)
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.Update(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	if crawlLimit, ok := updates["crawl_limit"]; ok {
		slog.InfoContext(ctx, "support content source crawl limit updated",
			"workspace_id", workspaceID,
			"content_source_id", id,
			"old_crawl_limit", existing.CrawlLimit,
			"new_crawl_limit", crawlLimit,
		)
	}
	if s.syncService != nil {
		if err := s.syncService.QueueSourceSync(ctx, workspaceID, id); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func (s *SupportContentSourceService) Delete(ctx context.Context, workspaceID, id string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil || existing.WorkspaceID != workspaceID {
		return fmt.Errorf("content source not found in workspace")
	}
	// Stop and await the durable workflow before removing indexed data. Without
	// this, an active crawl can recreate pages/chunks after the delete returns.
	if s.syncService != nil {
		if err := s.syncService.CancelSourceSync(ctx, workspaceID, id); err != nil {
			return fmt.Errorf("cancel content source sync: %w", err)
		}
	}
	if err := s.linkRepo.DeleteByContentSourceID(ctx, id); err != nil {
		return err
	}
	if err := s.chunkRepo.DeleteByContentSourceID(ctx, id); err != nil {
		return err
	}
	if err := s.pageRepo.DeleteByContentSourceID(ctx, id); err != nil {
		return err
	}
	if existing.SourceType == model.ContentSourceTypeFile && s.objectStore != nil && existing.StorageKey != nil && strings.TrimSpace(*existing.StorageKey) != "" {
		_ = s.objectStore.DeleteObject(ctx, strings.TrimSpace(*existing.StorageKey))
	}
	return s.repo.Delete(ctx, id)
}

func (s *SupportContentSourceService) Reindex(ctx context.Context, workspaceID, id string) error {
	if s.syncService == nil {
		return nil
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil || existing.WorkspaceID != workspaceID {
		return fmt.Errorf("content source not found in workspace")
	}
	if existing.SourceType == model.ContentSourceTypeWebsite {
		return s.syncService.QueueSourceSync(ctx, workspaceID, id)
	}
	return s.syncService.QueueSourceReindex(ctx, workspaceID, id)
}

// ListPages returns all crawled pages for a content source after verifying
// that the source belongs to the workspace.
func (s *SupportContentSourceService) ListPages(ctx context.Context, workspaceID, contentSourceID string) ([]model.SupportContentPage, error) {
	return s.listPages(ctx, workspaceID, contentSourceID, false)
}

// ListIndexedPages returns only pages with a persisted searchable index.
func (s *SupportContentSourceService) ListIndexedPages(ctx context.Context, workspaceID, contentSourceID string) ([]model.SupportContentPage, error) {
	return s.listPages(ctx, workspaceID, contentSourceID, true)
}

func (s *SupportContentSourceService) listPages(ctx context.Context, workspaceID, contentSourceID string, indexedOnly bool) ([]model.SupportContentPage, error) {
	source, err := s.repo.GetByID(ctx, contentSourceID)
	if err != nil {
		return nil, err
	}
	if source == nil || source.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("content source not found in workspace")
	}
	if indexedOnly {
		return s.pageRepo.ListIndexedByContentSourceID(ctx, workspaceID, contentSourceID)
	}
	return s.pageRepo.ListByContentSourceID(ctx, contentSourceID)
}

// GetPage returns a single crawled page (including content) after verifying
// that its parent content source belongs to the workspace.
func (s *SupportContentSourceService) GetPage(ctx context.Context, workspaceID, contentSourceID, pageID string) (*model.SupportContentPage, error) {
	source, err := s.repo.GetByID(ctx, contentSourceID)
	if err != nil {
		return nil, err
	}
	if source == nil || source.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("content source not found in workspace")
	}
	page, err := s.pageRepo.GetByIDWithContent(ctx, pageID)
	if err != nil {
		return nil, err
	}
	if page == nil || page.ContentSourceID != contentSourceID {
		return nil, fmt.Errorf("page not found in content source")
	}
	return page, nil
}

// AgentContentSourceService manages per-agent content source selection.
type AgentContentSourceService struct {
	repo        *repository.AgentContentSourceRepository
	agentRepo   *repository.AgentRepository
	sourceRepo  *repository.SupportContentSourceRepository
	syncService *SupportContentSyncService
}

func NewAgentContentSourceService(
	repo *repository.AgentContentSourceRepository,
	agentRepo *repository.AgentRepository,
	sourceRepo *repository.SupportContentSourceRepository,
	syncService *SupportContentSyncService,
) *AgentContentSourceService {
	return &AgentContentSourceService{
		repo:        repo,
		agentRepo:   agentRepo,
		sourceRepo:  sourceRepo,
		syncService: syncService,
	}
}

func (s *AgentContentSourceService) ListSelectedIDs(ctx context.Context, workspaceID, agentID string) ([]string, error) {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found in workspace")
	}
	return s.repo.ListContentSourceIDs(ctx, agentID)
}

func (s *AgentContentSourceService) Set(ctx context.Context, workspaceID, agentID string, contentSourceIDs []string) error {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found in workspace")
	}

	existing, err := s.repo.ListByAgentID(ctx, agentID)
	if err != nil {
		return err
	}
	existingByID := make(map[string]model.AgentContentSource, len(existing))
	for _, row := range existing {
		existingByID[row.ContentSourceID] = row
	}

	nextIDs := make([]string, 0, len(contentSourceIDs))
	seen := map[string]struct{}{}
	for _, raw := range contentSourceIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		source, err := s.sourceRepo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if source == nil || source.WorkspaceID != workspaceID {
			return fmt.Errorf("one or more content_source_ids do not belong to this workspace")
		}
		seen[id] = struct{}{}
		nextIDs = append(nextIDs, id)
	}

	for contentSourceID := range existingByID {
		if _, keep := seen[contentSourceID]; keep {
			continue
		}
		if err := s.repo.DeleteByAgentAndSource(ctx, agentID, contentSourceID); err != nil {
			return err
		}
	}

	for _, contentSourceID := range nextIDs {
		if _, ok := existingByID[contentSourceID]; ok {
			continue
		}
		if err := s.repo.Create(ctx, &model.AgentContentSource{
			AgentID:         agentID,
			ContentSourceID: contentSourceID,
			WorkspaceID:     workspaceID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func normalizeSupportContentSourceCreate(workspaceID string, req model.CreateSupportContentSourceRequest) (*model.SupportContentSource, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	startURL, err := normalizeContentSourceURL(req.StartURL)
	if err != nil {
		return nil, err
	}
	formats := normalizeContentSourceFormats(req.Formats)
	purposes := normalizeContentSourcePurposes(req.CrawlPurposes)

	source := &model.SupportContentSource{
		WorkspaceID:          workspaceID,
		Name:                 name,
		SourceType:           model.ContentSourceTypeWebsite,
		StartURL:             startURL,
		CrawlLimit:           defaultIfZero(req.CrawlLimit, 100),
		CrawlDepth:           defaultIfZero(req.CrawlDepth, 2),
		CrawlSource:          normalizeContentDiscoverySource(req.CrawlSource),
		Formats:              model.DocsStringArray(formats),
		Render:               derefBool(req.Render, true),
		IncludeExternalLinks: derefBool(req.IncludeExternalLinks, false),
		IncludeSubdomains:    derefBool(req.IncludeSubdomains, false),
		IncludePatterns:      model.DocsStringArray(normalizeStringList(req.IncludePatterns)),
		ExcludePatterns:      model.DocsStringArray(normalizeStringList(req.ExcludePatterns)),
		CrawlPurposes:        model.DocsStringArray(purposes),
		MaxAgeSeconds:        normalizeMaxAge(req.MaxAgeSeconds),
		ModifiedSince:        req.ModifiedSince,
		JSONPrompt:           trimOptionalString(req.JSONPrompt),
		JSONResponseFormat:   normalizeJSONRaw(req.JSONResponseFormat),
		SyncStatus:           model.KnowledgeSourceSyncQueued,
	}
	if containsContentString(formats, model.ContentSourceFormatJSON) && strings.TrimSpace(derefContentString(source.JSONPrompt)) == "" {
		return nil, fmt.Errorf("json_prompt is required when JSON format is enabled")
	}
	return source, nil
}

const maxKnowledgeFileSize = 50 * 1024 * 1024

func normalizeSupportContentFileUploadRequest(workspaceID string, req model.CreateSupportContentSourceFileUploadRequest) (*model.SupportContentSource, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		return nil, fmt.Errorf("file_name is required")
	}
	if req.FileSize <= 0 {
		return nil, fmt.Errorf("file_size must be positive")
	}
	if req.FileSize > maxKnowledgeFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %s", formatByteLimit(maxKnowledgeFileSize))
	}
	contentType, err := normalizeKnowledgeFileContentType(req.ContentType, fileName)
	if err != nil {
		return nil, err
	}
	storageKey := strings.TrimSpace(req.StorageKey)
	if storageKey == "" {
		return nil, fmt.Errorf("storage_key is required")
	}
	return &model.SupportContentSource{
		WorkspaceID:          workspaceID,
		Name:                 name,
		SourceType:           model.ContentSourceTypeFile,
		StartURL:             "file://" + fileName,
		FileName:             &fileName,
		FileSize:             req.FileSize,
		ContentType:          &contentType,
		StorageKey:           &storageKey,
		CrawlLimit:           1,
		CrawlDepth:           0,
		CrawlSource:          model.ContentSourceDiscoveryAll,
		Formats:              model.DocsStringArray{model.ContentSourceFormatMarkdown},
		Render:               false,
		IncludeExternalLinks: false,
		IncludeSubdomains:    false,
		CrawlPurposes:        model.DocsStringArray{model.ContentSourcePurposeSearch, model.ContentSourcePurposeAIInput},
		MaxAgeSeconds:        86400,
		SyncStatus:           model.KnowledgeSourceSyncQueued,
	}, nil
}

func normalizeKnowledgeFileContentType(raw, fileName string) (string, error) {
	contentType := strings.TrimSpace(strings.ToLower(raw))
	if contentType == "" || contentType == "application/octet-stream" {
		switch strings.ToLower(filepath.Ext(fileName)) {
		case ".pdf":
			contentType = "application/pdf"
		case ".md", ".markdown":
			contentType = "text/markdown"
		case ".txt":
			contentType = "text/plain"
		case ".csv":
			contentType = "text/csv"
		case ".json":
			contentType = "application/json"
		case ".docx":
			contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}
	}
	switch contentType {
	case "application/pdf", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/json", "text/plain", "text/markdown", "text/x-markdown", "text/csv":
		return contentType, nil
	default:
		return "", fmt.Errorf("unsupported file type %s", strings.TrimSpace(raw))
	}
}

func buildSupportContentFileStorageKey(workspaceID, sourceID, fileName string) string {
	safeName := strings.TrimSpace(filepath.Base(fileName))
	safeName = strings.NewReplacer("/", "-", "\\", "-", "\x00", "").Replace(safeName)
	if safeName == "" || safeName == "." {
		safeName = "source-file"
	}
	return fmt.Sprintf("workspaces/%s/knowledge-sources/%s/%s", workspaceID, sourceID, safeName)
}

func normalizeSupportContentSourceUpdate(existing model.SupportContentSource, req model.UpdateSupportContentSourceRequest) (map[string]any, error) {
	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		updates["name"] = name
	}
	if req.StartURL != nil {
		startURL, err := normalizeContentSourceURL(*req.StartURL)
		if err != nil {
			return nil, err
		}
		updates["start_url"] = startURL
	}
	if req.CrawlLimit != nil {
		updates["crawl_limit"] = defaultIfZero(*req.CrawlLimit, 100)
	}
	if req.CrawlDepth != nil {
		updates["crawl_depth"] = defaultIfZero(*req.CrawlDepth, 2)
	}
	if req.CrawlSource != nil {
		updates["crawl_source"] = normalizeContentDiscoverySource(*req.CrawlSource)
	}
	if req.Formats != nil {
		formats := normalizeContentSourceFormats(req.Formats)
		updates["formats"] = model.DocsStringArray(formats)
		if containsContentString(formats, model.ContentSourceFormatJSON) && strings.TrimSpace(derefContentString(req.JSONPrompt)) == "" && strings.TrimSpace(derefContentString(existing.JSONPrompt)) == "" {
			return nil, fmt.Errorf("json_prompt is required when JSON format is enabled")
		}
	}
	if req.Render != nil {
		updates["render"] = *req.Render
	}
	if req.IncludeExternalLinks != nil {
		updates["include_external_links"] = *req.IncludeExternalLinks
	}
	if req.IncludeSubdomains != nil {
		updates["include_subdomains"] = *req.IncludeSubdomains
	}
	if req.IncludePatterns != nil {
		updates["include_patterns"] = model.DocsStringArray(normalizeStringList(req.IncludePatterns))
	}
	if req.ExcludePatterns != nil {
		updates["exclude_patterns"] = model.DocsStringArray(normalizeStringList(req.ExcludePatterns))
	}
	if req.CrawlPurposes != nil {
		updates["crawl_purposes"] = model.DocsStringArray(normalizeContentSourcePurposes(req.CrawlPurposes))
	}
	if req.MaxAgeSeconds != nil {
		updates["max_age_seconds"] = normalizeMaxAge(*req.MaxAgeSeconds)
	}
	if req.ModifiedSince != nil {
		updates["modified_since"] = req.ModifiedSince
	}
	if req.JSONPrompt != nil {
		updates["json_prompt"] = trimOptionalString(req.JSONPrompt)
	}
	if req.JSONResponseFormat != nil {
		updates["json_response_format"] = normalizeJSONRaw(req.JSONResponseFormat)
	}
	updates["sync_status"] = model.KnowledgeSourceSyncQueued
	updates["sync_progress"] = 0
	updates["last_sync_error"] = nil
	return updates, nil
}

func normalizeContentSourceURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("start_url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("start_url must be a valid absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("start_url must use http or https")
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func normalizeContentSourceFormats(input []string) []string {
	valid := map[string]struct{}{
		model.ContentSourceFormatHTML:     {},
		model.ContentSourceFormatMarkdown: {},
		model.ContentSourceFormatJSON:     {},
	}
	formats := normalizeStringList(input)
	result := make([]string, 0, len(formats))
	for _, format := range formats {
		if _, ok := valid[format]; ok {
			result = append(result, format)
		}
	}
	if len(result) == 0 {
		return []string{model.ContentSourceFormatMarkdown}
	}
	return result
}

func normalizeContentSourcePurposes(input []string) []string {
	valid := map[string]struct{}{
		model.ContentSourcePurposeSearch:  {},
		model.ContentSourcePurposeAIInput: {},
		model.ContentSourcePurposeAITrain: {},
	}
	purposes := normalizeStringList(input)
	result := make([]string, 0, len(purposes))
	for _, purpose := range purposes {
		if _, ok := valid[purpose]; ok {
			result = append(result, purpose)
		}
	}
	if len(result) == 0 {
		return []string{model.ContentSourcePurposeSearch, model.ContentSourcePurposeAIInput}
	}
	return result
}

func normalizeContentDiscoverySource(raw string) string {
	switch strings.TrimSpace(raw) {
	case model.ContentSourceDiscoverySitemaps, model.ContentSourceDiscoveryLinks:
		return strings.TrimSpace(raw)
	default:
		return model.ContentSourceDiscoveryAll
	}
}

func normalizeStringList(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeMaxAge(value int) int {
	if value <= 0 {
		return 86400
	}
	if value > 604800 {
		return 604800
	}
	return value
}

func normalizeJSONRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return nil
	}
	return raw
}

func defaultIfZero(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func derefBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func derefContentString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func containsContentString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
