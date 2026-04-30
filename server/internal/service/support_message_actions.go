package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var (
	ErrCancellableExpired            = errors.New("message can no longer be undone")
	ErrSupportMessageActionForbidden = errors.New("message action forbidden")
	ErrSupportMessageActionNotFound  = errors.New("support message not found")
	ErrSupportMessageActionsMiswired = errors.New("support message actions service is not configured")
)

// SupportMessageActionsService owns the narrow set of Crisp-style actions that
// mutate or inspect an individual support message.
type SupportMessageActionsService struct {
	messageRepo   *repository.SupportMessageRepository
	emailFallback *EmailFallbackService
	emailLogRepo  *repository.SupportEmailLogRepository
	wsPublisher   *websocket.Publisher
	now           func() time.Time
}

type SupportMessageDeleteResult struct {
	ID               string `json:"id"`
	Markdown         string `json:"markdown,omitempty"`
	EmailAlreadySent bool   `json:"email_already_sent"`
}

type SupportMessageInfoSender struct {
	ID        string  `json:"id,omitempty"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type SupportMessageInfoDelivery struct {
	Channel     string    `json:"channel"`
	DeliveredAt time.Time `json:"delivered_at"`
}

type SupportMessageInfo struct {
	ID                 string                      `json:"id"`
	SentAt             time.Time                   `json:"sent_at"`
	Sender             SupportMessageInfoSender    `json:"sender"`
	From               string                      `json:"from"`
	Origin             string                      `json:"origin"`
	Type               string                      `json:"type"`
	Delivered          *SupportMessageInfoDelivery `json:"delivered"`
	NotDeliveredReason *string                     `json:"not_delivered_reason"`
	Read               bool                        `json:"read"`
	ReadAt             *time.Time                  `json:"read_at"`
	Edited             bool                        `json:"edited"`
	Translated         bool                        `json:"translated"`
	Automated          bool                        `json:"automated"`
}

func NewSupportMessageActionsService(
	messageRepo *repository.SupportMessageRepository,
	emailFallback *EmailFallbackService,
	emailLogRepo *repository.SupportEmailLogRepository,
	wsPublisher *websocket.Publisher,
) *SupportMessageActionsService {
	return &SupportMessageActionsService{
		messageRepo:   messageRepo,
		emailFallback: emailFallback,
		emailLogRepo:  emailLogRepo,
		wsPublisher:   wsPublisher,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

func (s *SupportMessageActionsService) Delete(ctx context.Context, workspaceID, conversationID, actorID, messageID string, undo bool) (*SupportMessageDeleteResult, error) {
	if s == nil || s.messageRepo == nil {
		return nil, ErrSupportMessageActionsMiswired
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(conversationID) == "" || strings.TrimSpace(actorID) == "" || strings.TrimSpace(messageID) == "" {
		return nil, ErrSupportMessageActionNotFound
	}

	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg == nil || msg.WorkspaceID != workspaceID || msg.ConversationID != conversationID {
		return nil, ErrSupportMessageActionNotFound
	}

	owned, err := s.messageRepo.GetMessageForActor(ctx, messageID, actorID)
	if err != nil {
		return nil, err
	}
	if owned == nil {
		return nil, ErrSupportMessageActionForbidden
	}
	if undo && (owned.CancellableUntil == nil || !s.now().Before(owned.CancellableUntil.UTC())) {
		return nil, ErrCancellableExpired
	}

	alreadySent := false
	if s.emailFallback != nil {
		alreadySent, err = s.emailFallback.CancelForMessage(ctx, workspaceID, conversationID, messageID)
		if err != nil {
			return nil, err
		}
	}
	if err := s.messageRepo.SoftDeleteMessage(ctx, messageID, actorID); err != nil {
		return nil, err
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.SupportMessageDeletedEvent(workspaceID, conversationID, messageID, actorID))
	}

	result := &SupportMessageDeleteResult{
		ID:               messageID,
		EmailAlreadySent: alreadySent,
	}
	if undo {
		result.Markdown = owned.Content
	}
	return result, nil
}

func (s *SupportMessageActionsService) Info(ctx context.Context, workspaceID, conversationID, actorID, messageID string) (*SupportMessageInfo, error) {
	if s == nil || s.messageRepo == nil {
		return nil, ErrSupportMessageActionsMiswired
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(conversationID) == "" || strings.TrimSpace(messageID) == "" {
		return nil, ErrSupportMessageActionNotFound
	}

	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg == nil || msg.WorkspaceID != workspaceID || msg.ConversationID != conversationID {
		return nil, ErrSupportMessageActionNotFound
	}

	info := &SupportMessageInfo{
		ID:         msg.ID,
		SentAt:     msg.CreatedAt,
		Sender:     supportMessageInfoSender(msg),
		From:       supportMessageInfoFrom(msg, nil),
		Origin:     supportMessageInfoOrigin(msg),
		Type:       supportMessageInfoType(msg),
		Read:       msg.EmailReadAt != nil,
		ReadAt:     msg.EmailReadAt,
		Edited:     false,
		Translated: false,
		Automated:  msg.SenderType == "agent" || msg.SenderType == "ai",
	}

	if s.emailLogRepo != nil {
		logRow, err := s.emailLogRepo.GetByMessageID(ctx, workspaceID, messageID)
		if err != nil {
			return nil, err
		}
		if logRow != nil {
			info.From = supportMessageInfoFrom(msg, logRow)
			if logRow.DeliveredAt != nil {
				info.Delivered = &SupportMessageInfoDelivery{Channel: "email", DeliveredAt: logRow.DeliveredAt.UTC()}
			}
			if strings.TrimSpace(logRow.ErrorMessage) != "" {
				reason := logRow.ErrorMessage
				info.NotDeliveredReason = &reason
			}
		}
	}
	return info, nil
}

func supportMessageInfoSender(msg *model.SupportMessage) SupportMessageInfoSender {
	sender := SupportMessageInfoSender{
		Name:      strings.TrimSpace(derefSupportMessageActionString(msg.SenderDisplayName)),
		Type:      strings.TrimSpace(msg.SenderType),
		AvatarURL: msg.SenderAvatarURL,
	}
	if sender.Name == "" {
		switch msg.SenderType {
		case "customer":
			sender.Name = "Customer"
		case "agent", "ai":
			sender.Name = "AI agent"
		default:
			sender.Name = "Operator"
		}
	}
	if sender.Type == "" {
		sender.Type = "unknown"
	}
	if msg.SenderUserID != nil {
		sender.ID = *msg.SenderUserID
	}
	if msg.SenderAgentID != nil {
		sender.ID = *msg.SenderAgentID
	}
	return sender
}

func supportMessageInfoFrom(msg *model.SupportMessage, logRow *model.SupportEmailLog) string {
	if logRow != nil && strings.TrimSpace(logRow.FromEmail) != "" {
		return strings.TrimSpace(logRow.FromEmail)
	}
	switch msg.SenderType {
	case "customer":
		return "Customer"
	case "agent", "ai":
		return "AI agent"
	default:
		return "Operator"
	}
}

func supportMessageInfoOrigin(msg *model.SupportMessage) string {
	if msg.ViaChannel != nil && strings.TrimSpace(*msg.ViaChannel) != "" {
		return strings.TrimSpace(*msg.ViaChannel)
	}
	return "chat"
}

func supportMessageInfoType(msg *model.SupportMessage) string {
	if strings.TrimSpace(msg.MessageType) == "reply" {
		return "text"
	}
	if strings.TrimSpace(msg.MessageType) != "" {
		return strings.TrimSpace(msg.MessageType)
	}
	return "text"
}

func derefSupportMessageActionString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
