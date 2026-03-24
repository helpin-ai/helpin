package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

// SupportAttachmentService contains support attachment business logic.
type SupportAttachmentService struct {
	attachmentRepo *repository.SupportAttachmentRepository
	s3Client       *storage.S3Client
}

// NewSupportAttachmentService creates a new SupportAttachmentService.
func NewSupportAttachmentService(
	attachmentRepo *repository.SupportAttachmentRepository,
	s3Client *storage.S3Client,
) *SupportAttachmentService {
	return &SupportAttachmentService{
		attachmentRepo: attachmentRepo,
		s3Client:       s3Client,
	}
}

// Create validates file metadata, creates a DB record, and returns a presigned PUT URL.
func (s *SupportAttachmentService) Create(
	ctx context.Context,
	req model.CreateSupportAttachmentRequest,
	workspaceID, conversationID, uploaderType string,
	uploaderUserID, sessionID *string,
) (*model.SupportAttachmentResponse, error) {
	if s.s3Client == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}
	if strings.TrimSpace(req.FileName) == "" {
		return nil, fmt.Errorf("file_name is required")
	}
	if req.FileSize <= 0 {
		return nil, fmt.Errorf("file_size must be positive")
	}
	if req.FileSize > maxFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of 10MB")
	}
	if req.ContentType == "" {
		return nil, fmt.Errorf("content_type is required")
	}
	if !allowedMIMETypes[req.ContentType] {
		return nil, fmt.Errorf("file type %s is not allowed", req.ContentType)
	}

	attachment := &model.SupportAttachment{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		FileName:       strings.TrimSpace(req.FileName),
		FileSize:       req.FileSize,
		ContentType:    req.ContentType,
		UploadedByType: uploaderType,
		UploadedByID:   uploaderUserID,
		SessionID:      sessionID,
	}

	if err := s.attachmentRepo.Create(ctx, attachment); err != nil {
		return nil, err
	}

	// Build storage key: workspaces/{ws_id}/support/{conv_id}/{attachment_id}-{filename}
	storageKey := fmt.Sprintf("workspaces/%s/support/%s/%s-%s",
		workspaceID, conversationID, attachment.ID, attachment.FileName)

	var publicURL string
	if s.s3Client.HasPublicURL() {
		publicURL = s.s3Client.PublicURL(storageKey)
	}

	if err := s.attachmentRepo.UpdateStorageKey(ctx, attachment.ID, storageKey, publicURL); err != nil {
		return nil, err
	}
	attachment.StorageKey = storageKey
	attachment.PublicURL = publicURL

	uploadURL, err := s.s3Client.GeneratePresignedPutURL(storageKey, attachment.ContentType, attachment.FileSize, s.s3Client.HasPublicURL())
	if err != nil {
		return nil, fmt.Errorf("generate upload URL: %w", err)
	}

	return &model.SupportAttachmentResponse{
		Attachment: *attachment,
		UploadURL:  uploadURL,
		PublicURL:  publicURL,
	}, nil
}

// ConfirmUpload marks a support attachment as successfully uploaded.
func (s *SupportAttachmentService) ConfirmUpload(ctx context.Context, id string) error {
	attachment, err := s.attachmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if attachment == nil {
		return fmt.Errorf("attachment not found")
	}
	if attachment.IsUploaded {
		return nil // idempotent
	}
	return s.attachmentRepo.ConfirmUpload(ctx, id)
}

// LinkToMessage associates uploaded attachments with a message.
func (s *SupportAttachmentService) LinkToMessage(ctx context.Context, attachmentIDs []string, messageID string) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	return s.attachmentRepo.LinkToMessage(ctx, attachmentIDs, messageID)
}

// HydrateMessages populates the Attachments field on each message.
func (s *SupportAttachmentService) HydrateMessages(ctx context.Context, messages []model.SupportMessage) error {
	if len(messages) == 0 {
		return nil
	}

	messageIDs := make([]string, len(messages))
	for i, m := range messages {
		messageIDs[i] = m.ID
	}

	attachments, err := s.attachmentRepo.ListByMessageIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	if len(attachments) == 0 {
		return nil
	}

	// Group by message_id.
	byMsg := make(map[string][]model.SupportAttachmentPayload, len(attachments))
	for _, a := range attachments {
		if a.MessageID == nil {
			continue
		}
		byMsg[*a.MessageID] = append(byMsg[*a.MessageID], model.SupportAttachmentPayload{
			ID:       a.ID,
			FileKey:  a.StorageKey,
			FileName: a.FileName,
			FileType: a.ContentType,
			FileSize: a.FileSize,
			URL:      a.PublicURL,
		})
	}

	for i := range messages {
		if payloads, ok := byMsg[messages[i].ID]; ok {
			messages[i].Attachments = payloads
		}
	}
	return nil
}

// Delete deletes a support attachment (S3 object + DB record).
func (s *SupportAttachmentService) Delete(ctx context.Context, id, uploaderType string, uploaderUserID, sessionID *string) error {
	attachment, err := s.attachmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if attachment == nil {
		return fmt.Errorf("attachment not found")
	}

	// Authorization: verify the caller owns the attachment.
	switch uploaderType {
	case "user":
		if uploaderUserID == nil || attachment.UploadedByID == nil || *attachment.UploadedByID != *uploaderUserID {
			return fmt.Errorf("only the uploader can delete this attachment")
		}
	case "customer":
		if sessionID == nil || attachment.SessionID == nil || *attachment.SessionID != *sessionID {
			return fmt.Errorf("only the uploader can delete this attachment")
		}
	default:
		return fmt.Errorf("invalid uploader type")
	}

	if s.s3Client != nil && attachment.StorageKey != "" && attachment.IsUploaded {
		if err := s.s3Client.DeleteObject(ctx, attachment.StorageKey); err != nil {
			slog.ErrorContext(ctx, "delete support attachment from S3", "error", err, "storage_key", attachment.StorageKey)
		}
	}

	return s.attachmentRepo.Delete(ctx, id)
}
