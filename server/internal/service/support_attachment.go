package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

const maxSupportFileSize = 100 * 1024 * 1024 // 100 MiB, displayed as 100 MB.

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
	if req.FileSize > maxSupportFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %s", formatByteLimit(maxSupportFileSize))
	}
	if req.ContentType == "" {
		return nil, fmt.Errorf("content_type is required")
	}
	if !allowedMIMETypes[req.ContentType] {
		return nil, fmt.Errorf("file type %s is not allowed", req.ContentType)
	}

	conversationID = strings.TrimSpace(conversationID)
	var conversationIDPtr *string
	storageScope := ""
	if conversationID != "" {
		conversationIDPtr = &conversationID
		storageScope = conversationID
	} else {
		if sessionID == nil || strings.TrimSpace(*sessionID) == "" {
			return nil, fmt.Errorf("conversation_id is required")
		}
		storageScope = "sessions/" + strings.TrimSpace(*sessionID)
	}

	attachment := &model.SupportAttachment{
		WorkspaceID:    workspaceID,
		ConversationID: conversationIDPtr,
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

	// A widget upload can precede its first message. Stage it under the widget
	// session until WidgetCreateMessage assigns the durable conversation.
	storageKey := fmt.Sprintf("workspaces/%s/support/%s/%s-%s",
		workspaceID, storageScope, attachment.ID, attachment.FileName)

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
func (s *SupportAttachmentService) ConfirmUpload(ctx context.Context, id, uploaderType string, uploaderUserID, sessionID *string) error {
	attachment, err := s.attachmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if attachment == nil {
		return fmt.Errorf("attachment not found")
	}
	if err := authorizeSupportAttachment(attachment, uploaderType, uploaderUserID, sessionID); err != nil {
		return err
	}
	if attachment.IsUploaded {
		return nil // idempotent for the owning uploader
	}
	return s.attachmentRepo.ConfirmUpload(ctx, id)
}

// ValidateWidgetAttachments confirms that every requested attachment is an
// uploaded, unconsumed attachment owned by this widget session. A staged
// attachment has no conversation; an attachment uploaded in an existing
// thread must belong to that exact active conversation.
func (s *SupportAttachmentService) ValidateWidgetAttachments(
	ctx context.Context,
	attachmentIDs []string,
	workspaceID, sessionID string,
	conversationID *string,
) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	return s.attachmentRepo.ValidateWidgetAttachments(ctx, attachmentIDs, workspaceID, sessionID, conversationID)
}

// LinkWidgetAttachments atomically claims staged widget uploads for the
// successful message and its newly-created (or existing) conversation.
func (s *SupportAttachmentService) LinkWidgetAttachments(
	ctx context.Context,
	attachmentIDs []string,
	workspaceID, sessionID, conversationID, messageID string,
	expectedConversationID *string,
) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	return s.attachmentRepo.LinkWidgetAttachments(
		ctx, attachmentIDs, workspaceID, sessionID, conversationID, messageID, expectedConversationID,
	)
}

// LinkToMessage associates uploaded attachments with a message.
func (s *SupportAttachmentService) LinkToMessage(ctx context.Context, attachmentIDs []string, messageID string) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	return s.attachmentRepo.LinkToMessage(ctx, attachmentIDs, messageID)
}

// StoreInboundEmailAttachment decodes a Postmark inbound attachment, uploads it,
// and creates a message-linked support attachment record.
func (s *SupportAttachmentService) StoreInboundEmailAttachment(ctx context.Context, req supportInboundEmailAttachmentRequest) (*model.SupportAttachmentPayload, error) {
	if s == nil || s.s3Client == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}
	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = "attachment"
	}
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !allowedMIMETypes[contentType] {
		return nil, fmt.Errorf("file type %s is not allowed", contentType)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.Base64Content))
	if err != nil {
		return nil, fmt.Errorf("decode inbound attachment: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("attachment content is empty")
	}
	fileSize := int64(len(data))
	if fileSize > maxSupportFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %s", formatByteLimit(maxSupportFileSize))
	}

	messageID := strings.TrimSpace(req.MessageID)
	attachmentID := uuid.NewString()
	storageKey := fmt.Sprintf("workspaces/%s/support/%s/%s-%s",
		strings.TrimSpace(req.WorkspaceID), strings.TrimSpace(req.ConversationID), attachmentID, fileName)
	publicURL := ""
	if s.s3Client.HasPublicURL() {
		publicURL = s.s3Client.PublicURL(storageKey)
	}
	if err := s.s3Client.PutObject(ctx, storageKey, contentType, fileSize, bytes.NewReader(data), s.s3Client.HasPublicURL()); err != nil {
		return nil, err
	}

	attachment := &model.SupportAttachment{
		ID:             attachmentID,
		WorkspaceID:    strings.TrimSpace(req.WorkspaceID),
		ConversationID: supportAttachmentStringPtr(strings.TrimSpace(req.ConversationID)),
		MessageID:      &messageID,
		FileName:       fileName,
		FileSize:       fileSize,
		ContentType:    contentType,
		StorageKey:     storageKey,
		PublicURL:      publicURL,
		UploadedByType: "customer",
		IsUploaded:     true,
	}

	if err := s.attachmentRepo.Create(ctx, attachment); err != nil {
		return nil, err
	}

	return &model.SupportAttachmentPayload{
		ID:       attachment.ID,
		FileKey:  storageKey,
		FileName: attachment.FileName,
		FileType: attachment.ContentType,
		FileSize: attachment.FileSize,
		URL:      publicURL,
	}, nil
}

// DownloadContent returns the stored bytes for a support attachment payload.
func (s *SupportAttachmentService) DownloadContent(ctx context.Context, attachment model.SupportAttachmentPayload) ([]byte, error) {
	if s == nil || s.s3Client == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}
	key := strings.TrimSpace(attachment.FileKey)
	if key == "" {
		return nil, fmt.Errorf("attachment storage key is required")
	}
	return s.s3Client.GetObject(ctx, key)
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

	if err := authorizeSupportAttachment(attachment, uploaderType, uploaderUserID, sessionID); err != nil {
		return err
	}

	if s.s3Client != nil && attachment.StorageKey != "" && attachment.IsUploaded {
		if err := s.s3Client.DeleteObject(ctx, attachment.StorageKey); err != nil {
			slog.ErrorContext(ctx, "delete support attachment from S3", "error", err, "storage_key", attachment.StorageKey)
		}
	}

	return s.attachmentRepo.Delete(ctx, id)
}

func authorizeSupportAttachment(attachment *model.SupportAttachment, uploaderType string, uploaderUserID, sessionID *string) error {
	switch uploaderType {
	case "user":
		if uploaderUserID == nil || attachment.UploadedByID == nil || *attachment.UploadedByID != *uploaderUserID {
			return fmt.Errorf("only the uploader can modify this attachment")
		}
	case "customer":
		if sessionID == nil || attachment.SessionID == nil || *attachment.SessionID != *sessionID {
			return fmt.Errorf("only the uploader can modify this attachment")
		}
	default:
		return fmt.Errorf("invalid uploader type")
	}
	return nil
}

func supportAttachmentStringPtr(value string) *string {
	return &value
}
