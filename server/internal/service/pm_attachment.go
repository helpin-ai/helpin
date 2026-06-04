package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const maxFileSize = 50 * 1024 * 1024 // 50 MB

var allowedMIMETypes = map[string]bool{
	// Images
	"image/jpeg": true, "image/png": true, "image/gif": true,
	"image/webp": true, "image/svg+xml": true,
	// Video
	"video/mp4": true, "video/quicktime": true, "video/webm": true,
	"video/mpeg": true, "video/x-msvideo": true, "video/x-matroska": true,
	// Documents
	"application/pdf":    true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	// Spreadsheets
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	// Text
	"text/plain": true, "text/csv": true, "text/markdown": true,
	// Archives
	"application/zip": true, "application/gzip": true,
	"application/x-tar": true,
}

var allowedEntityTypes = map[string]bool{
	"task": true, "story": true, "task_template": true, "epic": true, "objective": true, "sprint": true, "comment": true, "editor_upload": true,
}

// PMAttachmentService contains attachment business logic.
type PMAttachmentService struct {
	attachmentRepo *repository.PMAttachmentRepository
	s3Client       pmAttachmentObjectStore
	wsPublisher    *websocket.Publisher
}

type pmAttachmentObjectStore interface {
	HasPublicURL() bool
	PublicURL(key string) string
	GeneratePresignedPutURL(key, contentType string, size int64, publicRead bool) (string, error)
	PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error
	GeneratePresignedGetURL(key, filename string) (string, error)
	GeneratePresignedInlineGetURL(key string) (string, error)
	DeleteObject(ctx context.Context, key string) error
}

// NewPMAttachmentService creates a new PMAttachmentService.
func NewPMAttachmentService(attachmentRepo *repository.PMAttachmentRepository, s3Client pmAttachmentObjectStore, wsPublisher *websocket.Publisher) *PMAttachmentService {
	return &PMAttachmentService{
		attachmentRepo: attachmentRepo,
		s3Client:       s3Client,
		wsPublisher:    wsPublisher,
	}
}

// Create validates file metadata, creates a DB record, and returns a presigned PUT URL.
func (s *PMAttachmentService) Create(ctx context.Context, req model.CreateAttachmentRequest, workspaceID, userID string) (*model.AttachmentResponse, error) {
	attachment, err := s.prepareAttachment(ctx, req, workspaceID, userID)
	if err != nil {
		return nil, err
	}

	uploadURL, err := s.s3Client.GeneratePresignedPutURL(attachment.StorageKey, attachment.ContentType, attachment.FileSize, s.s3Client.HasPublicURL())
	if err != nil {
		return nil, fmt.Errorf("generate upload URL: %w", err)
	}

	resp := &model.AttachmentResponse{
		Attachment: *attachment,
		URL:        uploadURL,
	}
	if s.s3Client.HasPublicURL() {
		resp.PublicURL = s.s3Client.PublicURL(attachment.StorageKey)
	}
	return resp, nil
}

// SupportsPublicURL reports whether imported attachments can be rewritten to stable public URLs.
func (s *PMAttachmentService) SupportsPublicURL() bool {
	return s.s3Client != nil && s.s3Client.HasPublicURL()
}

// CreateImported uploads a file directly from the server and marks it as uploaded.
func (s *PMAttachmentService) CreateImported(ctx context.Context, req model.CreateAttachmentRequest, workspaceID, userID string, body io.Reader) (*model.AttachmentResponse, error) {
	attachment, err := s.prepareAttachment(ctx, req, workspaceID, userID)
	if err != nil {
		return nil, err
	}

	if err := s.s3Client.PutObject(ctx, attachment.StorageKey, attachment.ContentType, attachment.FileSize, body, s.s3Client.HasPublicURL()); err != nil {
		_ = s.attachmentRepo.Delete(ctx, attachment.ID)
		return nil, fmt.Errorf("upload attachment: %w", err)
	}
	if err := s.attachmentRepo.ConfirmUpload(ctx, attachment.ID); err != nil {
		_ = s.s3Client.DeleteObject(ctx, attachment.StorageKey)
		_ = s.attachmentRepo.Delete(ctx, attachment.ID)
		return nil, err
	}
	attachment.IsUploaded = true

	resp := &model.AttachmentResponse{Attachment: *attachment}
	if s.s3Client.HasPublicURL() {
		resp.PublicURL = s.s3Client.PublicURL(attachment.StorageKey)
	} else {
		downloadURL, err := s.s3Client.GeneratePresignedGetURL(attachment.StorageKey, attachment.FileName)
		if err == nil {
			resp.URL = downloadURL
		}
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "attachment", EntityID: attachment.ID, WorkspaceID: attachment.WorkspaceID, ParentType: attachment.EntityType, ParentID: attachment.EntityID})
	}
	return resp, nil
}

func (s *PMAttachmentService) prepareAttachment(ctx context.Context, req model.CreateAttachmentRequest, workspaceID, userID string) (*model.PMAttachment, error) {
	if s.s3Client == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}
	if req.EntityType == "" || req.EntityID == "" {
		return nil, fmt.Errorf("entity_type and entity_id are required")
	}
	if !allowedEntityTypes[req.EntityType] {
		return nil, fmt.Errorf("invalid entity_type: must be task, story, task_template, epic, objective, sprint, comment, or editor_upload")
	}
	if strings.TrimSpace(req.FileName) == "" {
		return nil, fmt.Errorf("file_name is required")
	}
	if req.FileSize <= 0 {
		return nil, fmt.Errorf("file_size must be positive")
	}
	if req.FileSize > maxFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %s", formatByteLimit(maxFileSize))
	}
	if req.ContentType == "" {
		return nil, fmt.Errorf("content_type is required")
	}
	if !allowedMIMETypes[req.ContentType] {
		return nil, fmt.Errorf("file type %s is not allowed", req.ContentType)
	}

	attachment := &model.PMAttachment{
		WorkspaceID:  workspaceID,
		EntityType:   req.EntityType,
		EntityID:     req.EntityID,
		FileName:     strings.TrimSpace(req.FileName),
		FileSize:     req.FileSize,
		ContentType:  req.ContentType,
		UploadedByID: userID,
	}

	if err := s.attachmentRepo.Create(ctx, attachment); err != nil {
		return nil, err
	}

	// Build storage key: {workspace_id}/{attachment_id}-{filename}
	attachment.StorageKey = fmt.Sprintf("%s/%s-%s", workspaceID, attachment.ID, attachment.FileName)
	if err := s.attachmentRepo.UpdateStorageKey(ctx, attachment.ID, attachment.StorageKey); err != nil {
		return nil, err
	}
	return attachment, nil
}

func formatByteLimit(bytes int64) string {
	if bytes > 0 && bytes%(1024*1024) == 0 {
		return fmt.Sprintf("%dMB", bytes/(1024*1024))
	}
	return fmt.Sprintf("%d bytes", bytes)
}

// ConfirmUpload marks an attachment as successfully uploaded.
func (s *PMAttachmentService) ConfirmUpload(ctx context.Context, id string) error {
	attachment, err := s.attachmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if attachment == nil {
		return fmt.Errorf("attachment not found")
	}
	if attachment.IsUploaded {
		return nil // Already confirmed, idempotent
	}
	if err := s.attachmentRepo.ConfirmUpload(ctx, id); err != nil {
		return err
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "attachment", EntityID: id, WorkspaceID: attachment.WorkspaceID, ParentType: attachment.EntityType, ParentID: attachment.EntityID})
	}
	return nil
}

// List returns uploaded attachments for an entity with presigned GET URLs.
func (s *PMAttachmentService) List(ctx context.Context, entityType, entityID string) ([]model.AttachmentResponse, error) {
	if entityType == "" || entityID == "" {
		return nil, fmt.Errorf("entity_type and entity_id are required")
	}
	attachments, err := s.attachmentRepo.List(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}

	result := make([]model.AttachmentResponse, 0, len(attachments))
	for _, a := range attachments {
		resp := model.AttachmentResponse{Attachment: a}
		if s.s3Client != nil && a.StorageKey != "" {
			downloadURL, err := s.s3Client.GeneratePresignedGetURL(a.StorageKey, a.FileName)
			if err == nil {
				resp.URL = downloadURL
			}
			if s.s3Client.HasPublicURL() {
				resp.PublicURL = s.s3Client.PublicURL(a.StorageKey)
			}
		}
		result = append(result, resp)
	}
	return result, nil
}

// Delete deletes an attachment (S3 object + DB record).
func (s *PMAttachmentService) Delete(ctx context.Context, id, userID string, pendingOnly ...bool) error {
	attachment, err := s.attachmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if attachment == nil {
		return fmt.Errorf("attachment not found")
	}
	if attachment.UploadedByID != userID {
		return fmt.Errorf("only the uploader can delete this attachment")
	}

	onlyPending := len(pendingOnly) > 0 && pendingOnly[0]
	if onlyPending {
		deleted, err := s.attachmentRepo.DeleteEditorUpload(ctx, id)
		if err != nil {
			return err
		}
		if !deleted {
			return nil
		}
		if s.shouldDeleteStorageObject(ctx, attachment) {
			_ = s.s3Client.DeleteObject(ctx, attachment.StorageKey)
		}
		if s.wsPublisher != nil {
			s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "attachment", EntityID: id, WorkspaceID: attachment.WorkspaceID, ActorID: userID, ParentType: attachment.EntityType, ParentID: attachment.EntityID})
		}
		return nil
	}

	// Delete from S3 if uploaded
	if s.shouldDeleteStorageObject(ctx, attachment) {
		_ = s.s3Client.DeleteObject(ctx, attachment.StorageKey)
	}

	if err := s.attachmentRepo.Delete(ctx, id); err != nil {
		return err
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "attachment", EntityID: id, WorkspaceID: attachment.WorkspaceID, ActorID: userID, ParentType: attachment.EntityType, ParentID: attachment.EntityID})
	}
	return nil
}

// ContentURL returns a fresh inline content URL for an uploaded attachment.
func (s *PMAttachmentService) ContentURL(ctx context.Context, id string) (string, error) {
	if s.s3Client == nil {
		return "", fmt.Errorf("file storage is not configured")
	}
	attachment, err := s.attachmentRepo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if attachment == nil || !attachment.IsUploaded || attachment.StorageKey == "" {
		return "", fmt.Errorf("attachment not found")
	}
	downloadURL, err := s.s3Client.GeneratePresignedInlineGetURL(attachment.StorageKey)
	if err != nil {
		return "", fmt.Errorf("generate content URL: %w", err)
	}
	return downloadURL, nil
}

func (s *PMAttachmentService) shouldDeleteStorageObject(ctx context.Context, attachment *model.PMAttachment) bool {
	if s.s3Client == nil || attachment == nil || attachment.StorageKey == "" || !attachment.IsUploaded {
		return false
	}
	if s.attachmentRepo == nil {
		return true
	}
	count, err := s.attachmentRepo.CountByStorageKey(ctx, attachment.StorageKey)
	if err != nil {
		return false
	}
	return count <= 1
}
