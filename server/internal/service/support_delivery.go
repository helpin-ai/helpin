package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func validateSupportDeliveryMode(req *model.CreateMessageRequest, senderType string) error {
	if err := validateSupportEmailSubject(req, senderType); err != nil {
		return err
	}
	if req.DeliveryMode == "" {
		return nil
	}
	if req.IsInternal || (req.MessageType != "" && req.MessageType != "reply") || senderType == "customer" {
		return fmt.Errorf("delivery mode is only available for public teammate replies")
	}
	switch req.DeliveryMode {
	case model.SupportDeliveryChatOnly:
		req.Channels = []string{"chat"}
	case model.SupportDeliveryChatAndEmail:
		req.Channels = []string{"chat", "email"}
	case model.SupportDeliveryEmailOnly:
		req.Channels = []string{"email"}
	default:
		return fmt.Errorf("delivery mode must be chat_only, chat_and_email, or email_only")
	}
	return nil
}

func validateSupportEmailSubject(req *model.CreateMessageRequest, senderType string) error {
	if req.EmailSubject == nil {
		return nil
	}
	if senderType != "user" || req.IsInternal || (req.MessageType != "" && req.MessageType != "reply") ||
		(req.DeliveryMode != model.SupportDeliveryEmailOnly && req.DeliveryMode != model.SupportDeliveryChatAndEmail) {
		return fmt.Errorf("email subject is only available for public teammate email replies")
	}
	if strings.ContainsFunc(*req.EmailSubject, unicode.IsControl) {
		return fmt.Errorf("email subject must not contain line breaks or control characters")
	}
	subject := strings.TrimSpace(*req.EmailSubject)
	if subject == "" || utf8.RuneCountInString(subject) > 500 {
		return fmt.Errorf("email subject must be between 1 and 500 characters")
	}
	req.EmailSubject = &subject
	return nil
}

func withSupportDeliveryMode(metadata, mode string) string {
	if mode == "" {
		return metadata
	}
	values := map[string]any{}
	if metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &values); err != nil || values == nil {
			values = map[string]any{}
		}
	}
	values["delivery_mode"] = mode
	if mode == model.SupportDeliveryEmailOnly || mode == model.SupportDeliveryChatAndEmail {
		values["email_delivery_status"] = "queued"
	}
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func (s *EmailFallbackService) validateExplicitEmail(ctx context.Context, workspaceID string, conv *model.SupportConversation) (int, error) {
	if s == nil || s.redis == nil || s.emailClient == nil || s.messageRepo == nil {
		return 0, fmt.Errorf("email delivery is not configured for this workspace")
	}
	if conv == nil || conv.CustomerEmail == nil || strings.TrimSpace(*conv.CustomerEmail) == "" {
		return 0, fmt.Errorf("add a customer email address before sending by email")
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(*conv.CustomerEmail)); err != nil {
		return 0, fmt.Errorf("the customer email address is invalid")
	}
	if conv.PrimaryRecipientState == model.SupportPrimaryRecipientStateUnconfirmed {
		return 0, fmt.Errorf("confirm the primary recipient before sending an email reply")
	}
	if conv.EmailUnsubscribed {
		return 0, fmt.Errorf("this customer has opted out of email replies")
	}
	if strings.EqualFold(conv.Status, "spam") {
		return 0, fmt.Errorf("email replies cannot be sent to a conversation marked as spam")
	}
	settings, err := s.loadSettings(ctx, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("unable to verify email settings; try again")
	}
	if !settings.EmailFallbackEnabled {
		return 0, fmt.Errorf("email delivery is disabled for this workspace")
	}
	if s.contactRepo != nil {
		contact, err := s.contactRepo.GetByEmail(ctx, workspaceID, strings.TrimSpace(*conv.CustomerEmail))
		if err != nil {
			return 0, fmt.Errorf("unable to verify the recipient email address; try again")
		}
		if contact != nil && contact.EmailStatus == model.CRMContactEmailStatusInvalid {
			return 0, fmt.Errorf("the recipient email address is marked invalid; update it before sending")
		}
	}
	if err := s.redis.Ping(ctx).Err(); err != nil {
		return 0, fmt.Errorf("the email delivery queue is unavailable; try again")
	}
	return normalizedEmailFallbackDelaySecs(settings.EmailFallbackDelaySecs), nil
}

func (s *EmailFallbackService) queueExplicitEmail(ctx context.Context, msg *model.SupportMessage, conv *model.SupportConversation, repo *repository.SupportMessageRepository, delaySecs int) error {
	fireAt := s.now().Add(time.Duration(delaySecs) * time.Second)
	// Keep the row and undo deadline atomic. Redis may retain an orphan if the DB
	// commit fails; the existing poller safely drops IDs without a persisted row.
	if err := repo.SetCancellableUntil(ctx, msg.ID, fireAt); err != nil {
		return fmt.Errorf("unable to prepare email delivery; try again")
	}
	pipe := s.redis.TxPipeline()
	pipe.RPush(ctx, s.msgListKey(conv.ID), msg.ID)
	pipe.ZAddArgs(ctx, emailFallbackOutboxKey, redis.ZAddArgs{GT: true, Members: []redis.Z{{Score: float64(fireAt.Unix()), Member: conv.ID}}})
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("unable to queue email delivery; try again")
	}
	msg.CancellableUntil = &fireAt
	return nil
}

func (s *SupportInboxService) createExplicitEmailMessage(ctx context.Context, msg *model.SupportMessage, conv *model.SupportConversation, delaySecs int, emailSubject *string) error {
	values := map[string]any{}
	if err := json.Unmarshal([]byte(msg.Metadata), &values); err != nil {
		return fmt.Errorf("unable to prepare email delivery; try again")
	}
	values["delivery_to_email"] = strings.TrimSpace(derefString(conv.CustomerEmail))
	subject := strings.TrimSpace(conv.Subject)
	if emailSubject != nil {
		subject = *emailSubject
	}
	if subject == "" {
		subject = "Support conversation"
	}
	values["email_subject"] = subject
	recipients := supportMessageEmailRecipientsFromMetadata(msg.Metadata)
	if len(recipients.CC) == 0 {
		recipients.CC = normalizeSupportEmailListExcluding(conv.EmailCC, derefString(conv.CustomerEmail))
	}
	values["email_cc"] = recipients.CC
	values["email_bcc"] = recipients.BCC
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("unable to prepare email delivery; try again")
	}
	msg.Metadata = string(encoded)
	return s.messageRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.messageRepo.WithTx(tx)
		if err := repo.Create(ctx, msg); err != nil {
			return err
		}
		return s.emailFallbackService.queueExplicitEmail(ctx, msg, conv, repo, delaySecs)
	})
}

func widgetVisibleSupportMessages(messages []model.SupportMessage) []model.SupportMessage {
	visible := make([]model.SupportMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.WidgetVisible() {
			visible = append(visible, msg)
		}
	}
	return visible
}
