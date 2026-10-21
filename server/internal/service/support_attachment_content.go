package service

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrSupportAttachmentNotFound also hides attachments the caller cannot access.
var ErrSupportAttachmentNotFound = errors.New("attachment not found")

// SupportAttachmentContent contains an authorized attachment stream.
type SupportAttachmentContent struct {
	Body        io.ReadCloser
	Size        int64
	ContentType string
	FileName    string
}

// OpenAttachmentContent reads a saved attachment without exposing a storage URL.
// Workspace, inbox access, and message ownership are checked on every request.
func (s *SupportInboxService) OpenAttachmentContent(ctx context.Context, workspaceID, conversationID, attachmentID string) (*SupportAttachmentContent, error) {
	actor := supportActorFromContext(ctx, workspaceID)
	if actor == nil || actor.WorkspaceMemberID == "" {
		return nil, ErrSupportAttachmentNotFound
	}
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, ErrSupportAttachmentNotFound
	}
	if s.attachmentService == nil || s.attachmentService.s3Client == nil {
		return nil, fmt.Errorf("attachment storage unavailable")
	}
	attachment, err := s.attachmentService.attachmentRepo.GetByID(ctx, attachmentID)
	if err != nil {
		return nil, err
	}
	if attachment == nil || attachment.WorkspaceID != workspaceID || attachment.ConversationID == nil ||
		*attachment.ConversationID != conversationID || attachment.MessageID == nil ||
		!attachment.IsUploaded || attachment.StorageKey == "" {
		return nil, ErrSupportAttachmentNotFound
	}
	message, err := s.messageRepo.GetByID(ctx, *attachment.MessageID)
	if err != nil {
		return nil, err
	}
	if message == nil || message.WorkspaceID != workspaceID || message.ConversationID != conversationID {
		return nil, ErrSupportAttachmentNotFound
	}
	body, size, err := s.attachmentService.s3Client.OpenObject(ctx, attachment.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("read support attachment: %w", err)
	}
	return &SupportAttachmentContent{Body: body, Size: size, ContentType: attachment.ContentType, FileName: attachment.FileName}, nil
}
