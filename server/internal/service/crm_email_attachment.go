package service

import (
	"context"
	"fmt"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/sync"
)

const (
	crmEmailAttachmentMaxFiles     = 10
	crmEmailAttachmentMaxFileSize  = 10 * 1024 * 1024
	crmEmailAttachmentMaxTotalSize = 16 * 1024 * 1024
)

func (s *CRMEmailService) SetAttachmentStorage(repo *repository.CRMEmailAttachmentRepository, s3 *storage.S3Client) {
	s.attachmentRepo = repo
	s.attachmentStorage = s3
}

func (s *CRMEmailService) CreateAttachment(ctx context.Context, workspaceID, userID string, req model.CreateCRMEmailAttachmentRequest) (*model.CRMEmailAttachmentUploadResponse, error) {
	if s.attachmentRepo == nil || s.attachmentStorage == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}
	if _, err := uuid.Parse(req.DraftID); err != nil {
		return nil, fmt.Errorf("draft_id must be a UUID")
	}
	rawFileName := strings.TrimSpace(req.FileName)
	if rawFileName == "" {
		return nil, fmt.Errorf("file_name and content_type are required")
	}
	if len(rawFileName) > 255 || strings.ContainsAny(rawFileName, "\r\n\x00") {
		return nil, fmt.Errorf("file_name is invalid")
	}
	req.FileName = path.Base(strings.ReplaceAll(rawFileName, "\\", "/"))
	req.ContentType = strings.TrimSpace(req.ContentType)
	if req.FileName == "." || req.FileName == "/" || req.ContentType == "" {
		return nil, fmt.Errorf("file_name and content_type are required")
	}
	if req.FileSize <= 0 || req.FileSize > crmEmailAttachmentMaxFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of 10MB")
	}
	mediaType, _, err := mime.ParseMediaType(req.ContentType)
	if err != nil || strings.TrimSpace(mediaType) == "" {
		return nil, fmt.Errorf("content_type is invalid")
	}
	req.ContentType = mediaType
	attachment := &model.CRMEmailAttachment{WorkspaceID: workspaceID, DraftID: req.DraftID, UploadedByID: userID, FileName: req.FileName, FileSize: req.FileSize, ContentType: req.ContentType}
	if err := s.attachmentRepo.Create(ctx, attachment); err != nil {
		return nil, err
	}
	attachment.StorageKey = fmt.Sprintf("workspaces/%s/crm/email-drafts/%s/%s-%s", workspaceID, req.DraftID, attachment.ID, req.FileName)
	// StorageKey is intentionally private; persist it after GORM has generated the ID.
	if err := s.attachmentRepo.UpdateStorageKey(ctx, attachment.ID, attachment.StorageKey); err != nil {
		return nil, err
	}
	uploadURL, err := s.attachmentStorage.GeneratePresignedPutURL(attachment.StorageKey, attachment.ContentType, attachment.FileSize, false)
	if err != nil {
		return nil, fmt.Errorf("generate upload URL: %w", err)
	}
	return &model.CRMEmailAttachmentUploadResponse{Attachment: *attachment, UploadURL: uploadURL}, nil
}

func (s *CRMEmailService) ConfirmAttachment(ctx context.Context, workspaceID, id, userID string) error {
	if s.attachmentRepo == nil {
		return fmt.Errorf("file storage is not configured")
	}
	return s.attachmentRepo.Confirm(ctx, workspaceID, id, userID)
}

func (s *CRMEmailService) DeleteAttachment(ctx context.Context, workspaceID, id, userID string) error {
	if s.attachmentRepo == nil || s.attachmentStorage == nil {
		return fmt.Errorf("file storage is not configured")
	}
	attachment, err := s.attachmentRepo.Get(ctx, workspaceID, id)
	if err != nil || attachment == nil || attachment.UploadedByID != userID || attachment.MessageID != nil {
		return fmt.Errorf("attachment not found")
	}
	if attachment.StorageKey != "" {
		_ = s.attachmentStorage.DeleteObject(ctx, attachment.StorageKey)
	}
	return s.attachmentRepo.Delete(ctx, workspaceID, id, userID)
}

func (s *CRMEmailService) GetAttachmentDownloadURL(ctx context.Context, workspaceID, id string) (string, error) {
	if s.attachmentRepo == nil || s.attachmentStorage == nil {
		return "", fmt.Errorf("file storage is not configured")
	}
	attachment, err := s.attachmentRepo.Get(ctx, workspaceID, id)
	if err != nil || attachment == nil || attachment.MessageID == nil || !attachment.IsUploaded {
		return "", fmt.Errorf("attachment not found")
	}
	return s.attachmentStorage.GeneratePresignedGetURL(attachment.StorageKey, attachment.FileName)
}

func (s *CRMEmailService) prepareAttachments(ctx context.Context, workspaceID, draftID, userID string, ids []string) ([]sync.GmailAttachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > crmEmailAttachmentMaxFiles {
		return nil, fmt.Errorf("at most 10 attachments are allowed")
	}
	if s.attachmentRepo == nil || s.attachmentStorage == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}
	attachments, err := s.attachmentRepo.ListDraft(ctx, workspaceID, draftID, userID, ids)
	if err != nil {
		return nil, err
	}
	if len(attachments) != len(ids) {
		return nil, fmt.Errorf("one or more attachments are unavailable")
	}
	var total int64
	result := make([]sync.GmailAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		total += attachment.FileSize
		if total > crmEmailAttachmentMaxTotalSize {
			return nil, fmt.Errorf("attachments exceed the 16MB total limit")
		}
		data, err := s.attachmentStorage.GetObject(ctx, attachment.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("load attachment %s: %w", attachment.FileName, err)
		}
		result = append(result, sync.GmailAttachment{FileName: attachment.FileName, ContentType: attachment.ContentType, Data: data})
	}
	return result, nil
}

func (s *CRMEmailService) hydrateAttachments(ctx context.Context, workspaceID string, messages []model.CRMEmailMessage) error {
	if s.attachmentRepo == nil || len(messages) == 0 {
		return nil
	}
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	attachments, err := s.attachmentRepo.ListByMessageIDs(ctx, workspaceID, ids)
	if err != nil {
		return err
	}
	byMessage := map[string][]model.CRMEmailAttachment{}
	for _, attachment := range attachments {
		if attachment.MessageID != nil {
			byMessage[*attachment.MessageID] = append(byMessage[*attachment.MessageID], attachment)
		}
	}
	for i := range messages {
		messages[i].Attachments = byMessage[messages[i].ID]
	}
	return nil
}

func (s *CRMEmailService) CleanupStaleAttachments(ctx context.Context) error {
	if s.attachmentRepo == nil || s.attachmentStorage == nil {
		return nil
	}
	attachments, err := s.attachmentRepo.ListStale(ctx, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return err
	}
	for _, attachment := range attachments {
		if attachment.StorageKey != "" {
			_ = s.attachmentStorage.DeleteObject(ctx, attachment.StorageKey)
		}
		_ = s.attachmentRepo.Delete(ctx, attachment.WorkspaceID, attachment.ID, attachment.UploadedByID)
	}
	return nil
}
