package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/email/inboundhtml"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// emailMarkdown renders agent-authored markdown into safe HTML for outbound email.
// Hard breaks are enabled so single newlines in the editor become <br>, matching
// what the agent sees in the composer preview.
var emailMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(
		gmhtml.WithHardWraps(),
		gmhtml.WithXHTML(),
	),
)

func renderMessageMarkdownToHTML(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := emailMarkdown.Convert([]byte(trimmed), &buf); err != nil {
		return "<p>" + strings.ReplaceAll(html.EscapeString(trimmed), "\n", "<br>") + "</p>"
	}
	rendered := strings.TrimSpace(buf.String())
	if rendered == "" {
		return "<p>" + strings.ReplaceAll(html.EscapeString(trimmed), "\n", "<br>") + "</p>"
	}
	return rendered
}

// inboundPayloadBodies picks the best content source from a Postmark inbound
// payload and returns both a markdown-friendly variant for plaintext display
// and a sanitized HTML variant for rich rendering in a sandboxed iframe.
//
// Preference order for the markdown variant:
//  1. HtmlBody converted to markdown — preserves anchor text so long tracking
//     URLs don't render as plaintext walls.
//  2. StrippedTextReply — Postmark-stripped plain-text reply (quoted history
//     removed), used when HTML is absent or conversion yields nothing.
//  3. TextBody — full plain-text body as final fallback.
//
// htmlBody is populated only when HtmlBody was present and processing
// succeeded; callers should treat an empty string as "no rich body".
func inboundPayloadBodies(payload model.PostmarkInboundPayload) (markdown, htmlBody string) {
	processed := inboundhtml.Process(payload.HtmlBody, "")
	markdown = processed.Markdown
	htmlBody = processed.HTML
	if markdown != "" {
		return markdown, htmlBody
	}
	if stripped := strings.TrimSpace(payload.StrippedTextReply); stripped != "" {
		return stripped, htmlBody
	}
	return strings.TrimSpace(payload.TextBody), htmlBody
}

const (
	emailFallbackOutboxKey     = "email_fallback_outbox"
	emailFallbackLockKey       = "email_fallback_lock"
	emailFallbackMsgsKeyPrefix = "email_fallback_msgs:"
)

// EmailFallbackService manages delayed outbound email delivery and inbound replies.
type EmailFallbackService struct {
	redis               *redis.Client
	hub                 *websocket.Hub
	wsPublisher         *websocket.Publisher
	emailClient         *email.Client
	messageRepo         *repository.SupportMessageRepository
	convRepo            *repository.SupportConversationRepository
	emailLogRepo        *repository.SupportEmailLogRepository
	webhookRepo         *repository.SupportEmailWebhookEventRepository
	installRepo         *repository.SupportInboxInstallationRepository
	sessionRepo         *repository.SupportInboxSessionRepository
	workspaceRepo       *repository.WorkspaceRepository
	contactRepo         *repository.CRMContactRepository
	supportInboxService *SupportInboxService
	notificationService *NotificationService
	replyDomain         string
	appBaseURL          string
	logger              *slog.Logger
	podID               string
	pollInterval        time.Duration
	leaseTTL            time.Duration
	processingTTL       time.Duration
	now                 func() time.Time
}

// SetNotificationService injects the notification service used for support reply alerts.
func (s *EmailFallbackService) SetNotificationService(notificationService *NotificationService) *EmailFallbackService {
	if s == nil {
		return nil
	}
	s.notificationService = notificationService
	return s
}

// SetSupportInboxService injects the support inbox service for mailbox-aware inbound email handling.
func (s *EmailFallbackService) SetSupportInboxService(supportInboxService *SupportInboxService) *EmailFallbackService {
	if s == nil {
		return nil
	}
	s.supportInboxService = supportInboxService
	return s
}

// SetCRMContactRepository injects the CRM contact repository used to flag
// recipient email addresses as invalid after a hard bounce or spam complaint.
func (s *EmailFallbackService) SetCRMContactRepository(contactRepo *repository.CRMContactRepository) *EmailFallbackService {
	if s == nil {
		return nil
	}
	s.contactRepo = contactRepo
	return s
}

// InboundDomain returns the domain used for reply and forwarding aliases.
func (s *EmailFallbackService) InboundDomain() string {
	if s == nil || strings.TrimSpace(s.replyDomain) == "" {
		return "replies.helpin.email"
	}
	return strings.TrimSpace(s.replyDomain)
}

// NewEmailFallbackService constructs the email fallback service.
func NewEmailFallbackService(
	rdb *redis.Client,
	hub *websocket.Hub,
	wsPublisher *websocket.Publisher,
	emailClient *email.Client,
	messageRepo *repository.SupportMessageRepository,
	convRepo *repository.SupportConversationRepository,
	emailLogRepo *repository.SupportEmailLogRepository,
	webhookRepo *repository.SupportEmailWebhookEventRepository,
	installRepo *repository.SupportInboxInstallationRepository,
	sessionRepo *repository.SupportInboxSessionRepository,
	workspaceRepo *repository.WorkspaceRepository,
	replyDomain string,
	appBaseURL string,
	podID string,
) *EmailFallbackService {
	logger := slog.Default().With("service", "email_fallback")
	return &EmailFallbackService{
		redis:         rdb,
		hub:           hub,
		wsPublisher:   wsPublisher,
		emailClient:   emailClient,
		messageRepo:   messageRepo,
		convRepo:      convRepo,
		emailLogRepo:  emailLogRepo,
		webhookRepo:   webhookRepo,
		installRepo:   installRepo,
		sessionRepo:   sessionRepo,
		workspaceRepo: workspaceRepo,
		replyDomain:   strings.TrimSpace(replyDomain),
		appBaseURL:    strings.TrimSpace(appBaseURL),
		logger:        logger,
		podID:         strings.TrimSpace(podID),
		pollInterval:  10 * time.Second,
		leaseTTL:      30 * time.Second,
		processingTTL: 5 * time.Minute,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// StartPoller runs the Redis outbox poller until ctx is cancelled.
func (s *EmailFallbackService) StartPoller(ctx context.Context) {
	if s == nil || s.redis == nil {
		return
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			leaseHeld, err := s.acquireOrRenewLease(ctx)
			if err != nil {
				s.logger.ErrorContext(ctx, "email fallback lease failed", "error", err)
				continue
			}
			if !leaseHeld {
				continue
			}
			if err := s.processDueEntries(ctx); err != nil {
				s.logger.ErrorContext(ctx, "email fallback poll failed", "error", err)
			}
		}
	}
}

// OnAgentReply enqueues an outbound email candidate when the feature is enabled.
func (s *EmailFallbackService) OnAgentReply(ctx context.Context, workspaceID string, msg *model.SupportMessage, conv *model.SupportConversation) error {
	if s == nil || s.redis == nil || s.emailClient == nil || msg == nil || conv == nil {
		return nil
	}
	if conv.CustomerEmail == nil || strings.TrimSpace(*conv.CustomerEmail) == "" {
		return nil
	}
	if isEmailFallbackTerminalStatus(conv.Status) || conv.EmailUnsubscribed {
		return nil
	}

	settings, err := s.loadSettings(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !settings.EmailFallbackEnabled {
		return nil
	}

	delaySecs := settings.EmailFallbackDelaySecs
	if delaySecs < 30 || delaySecs > 600 {
		delaySecs = 120
	}
	fireAt := s.now().Add(time.Duration(delaySecs) * time.Second)
	if _, err := s.redis.ZAddArgs(ctx, emailFallbackOutboxKey, redis.ZAddArgs{
		GT:      true,
		Members: []redis.Z{{Score: float64(fireAt.Unix()), Member: conv.ID}},
	}).Result(); err != nil {
		return fmt.Errorf("enqueue email fallback outbox: %w", err)
	}
	if err := s.redis.RPush(ctx, s.msgListKey(conv.ID), msg.ID).Err(); err != nil {
		return fmt.Errorf("append email fallback message id: %w", err)
	}
	return nil
}

// ProcessInboundEmail converts a Postmark inbound webhook into a support message when valid.
func (s *EmailFallbackService) ProcessInboundEmail(ctx context.Context, payload model.PostmarkInboundPayload, rawPayload string) error {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(rawPayload) == "" {
		rawPayloadBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal inbound payload: %w", err)
		}
		rawPayload = string(rawPayloadBytes)
	}

	resolvedConversationID := inboundConversationID(payload)
	var resolvedConversation *model.SupportConversation
	if resolvedConversationID != "" {
		conv, err := s.findConversationByID(ctx, resolvedConversationID)
		if err != nil {
			return err
		}
		resolvedConversation = conv
	}
	s.recordWebhookEvent(ctx, "inbound", strings.TrimSpace(payload.MessageID), strings.TrimSpace(payload.MessageStream), rawPayload, resolvedConversation, nil, parseInboundWebhookReceivedAt(payload))

	if existing, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, strings.TrimSpace(payload.MessageID)); err != nil {
		return err
	} else if existing != nil {
		s.logger.InfoContext(ctx, "postmark inbound duplicate ignored",
			"message_id", strings.TrimSpace(payload.MessageID),
			"conversation_id", strings.TrimSpace(existing.ConversationID),
			"email_log_id", existing.ID,
		)
		return nil
	}

	mailboxHash := mailboxHashFromInboundPayload(payload)
	if strings.HasPrefix(mailboxHash, "unsubscribe-") {
		conversationID := strings.TrimSpace(strings.TrimPrefix(mailboxHash, "unsubscribe-"))
		if _, err := uuid.Parse(conversationID); err != nil {
			return fmt.Errorf("invalid mailbox hash")
		}
		conv, err := s.findConversationByID(ctx, conversationID)
		if err != nil || conv == nil {
			return err
		}
		s.logger.InfoContext(ctx, "postmark inbound unsubscribe processed",
			"message_id", strings.TrimSpace(payload.MessageID),
			"conversation_id", conv.ID,
		)
		return s.convRepo.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"email_unsubscribed": true})
	}

	if strings.HasPrefix(mailboxHash, "route-") {
		return s.processInboundRouteEmail(ctx, mailboxHash, payload, rawPayload)
	}

	if mailboxHash == "" {
		if route, err := s.findInboundRouteByRecipient(ctx, inboundRecipientAddress(payload)); err != nil {
			return err
		} else if route != nil {
			return s.processInboundRoute(ctx, route, payload, rawPayload)
		}
		s.logger.WarnContext(ctx, "postmark inbound missing mailbox hash",
			"message_id", strings.TrimSpace(payload.MessageID),
			"original_recipient", strings.TrimSpace(payload.OriginalRecipient),
			"to", strings.TrimSpace(payload.To),
		)
		return fmt.Errorf("missing mailbox hash")
	}

	if !strings.HasPrefix(mailboxHash, "conv-") {
		return fmt.Errorf("invalid mailbox hash")
	}
	conversationID := strings.TrimSpace(strings.TrimPrefix(mailboxHash, "conv-"))
	if _, err := uuid.Parse(conversationID); err != nil {
		return fmt.Errorf("invalid mailbox hash")
	}
	conv, err := s.findConversationByID(ctx, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		s.logger.InfoContext(ctx, "postmark inbound conversation not found",
			"message_id", strings.TrimSpace(payload.MessageID),
			"conversation_id", conversationID,
		)
		return nil
	}
	return s.processInboundConversationReply(ctx, conv, nil, payload, rawPayload)
}

func (s *EmailFallbackService) processInboundConversationReply(ctx context.Context, conv *model.SupportConversation, route *model.SupportEmailRoute, payload model.PostmarkInboundPayload, rawPayload string) error {
	if conv == nil {
		return nil
	}
	if isEmailFallbackTerminalStatus(conv.Status) {
		s.logger.InfoContext(ctx, "postmark inbound ignored for terminal conversation",
			"message_id", strings.TrimSpace(payload.MessageID),
			"conversation_id", conv.ID,
			"status", conv.Status,
		)
		return nil
	}

	fromEmail := inboundEmailAddress(payload)
	if conv.CustomerEmail == nil || !strings.EqualFold(strings.TrimSpace(*conv.CustomerEmail), fromEmail) {
		s.logger.InfoContext(ctx, "postmark inbound sender mismatch",
			"message_id", strings.TrimSpace(payload.MessageID),
			"conversation_id", conv.ID,
		)
		return nil
	}

	content, htmlBody := inboundPayloadBodies(payload)
	if content == "" {
		return nil
	}
	if len(content) > 50_000 {
		content = content[:50_000]
	}

	rfcMessageID := inboundRFCMessageID(payload)
	inReplyTo := normalizeRFCHeaderValue(inboundHeaderValue(payload.Headers, "In-Reply-To"))
	referencesHeader := strings.TrimSpace(inboundHeaderValue(payload.Headers, "References"))
	recipientAddress := inboundRecipientAddress(payload)

	senderName := strings.TrimSpace(payload.FromFull.Name)
	if senderName == "" && conv.CustomerName != nil {
		senderName = strings.TrimSpace(*conv.CustomerName)
	}
	if senderName == "" {
		senderName = "Customer"
	}
	viaEmail := "email"
	msg := &model.SupportMessage{
		WorkspaceID:       conv.WorkspaceID,
		ConversationID:    conv.ID,
		SenderType:        "customer",
		SenderDisplayName: &senderName,
		Content:           content,
		IsInternal:        false,
		MessageType:       "reply",
		ViaChannel:        &viaEmail,
	}

	var createdMsg *model.SupportMessage
	txErr := s.convRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.messageRepo.WithTx(tx).Create(ctx, msg); err != nil {
			return err
		}

		logRow := &model.SupportEmailLog{
			WorkspaceID:       conv.WorkspaceID,
			ConversationID:    conv.ID,
			Direction:         "inbound",
			MessageIDs:        model.DocsStringArray{msg.ID},
			FromEmail:         fromEmail,
			ToEmail:           strings.TrimSpace(payload.To),
			RecipientAddress:  recipientAddress,
			Subject:           strings.TrimSpace(payload.Subject),
			RFCMessageID:      rfcMessageID,
			InReplyTo:         inReplyTo,
			ReferencesHeader:  referencesHeader,
			PostmarkMessageID: strPtr(strings.TrimSpace(payload.MessageID)),
			RawBody:           rawPayload,
			StrippedText:      content,
			HTMLBody:          htmlBody,
			Status:            "sent",
		}
		if route != nil {
			logRow.EmailRouteID = &route.ID
		}
		if logRow.PostmarkMessageID != nil && *logRow.PostmarkMessageID == "" {
			logRow.PostmarkMessageID = nil
		}
		if err := s.emailLogRepo.WithTx(tx).Create(ctx, logRow); err != nil {
			return err
		}

		createdMsg = msg
		return nil
	})
	if txErr != nil {
		if isLikelyUniqueConstraintError(txErr) {
			s.logger.InfoContext(ctx, "postmark inbound duplicate ignored after transaction race",
				"message_id", strings.TrimSpace(payload.MessageID),
				"conversation_id", conv.ID,
			)
			return nil
		}
		return txErr
	}

	ProcessSupportCustomerReplyNotification(ctx, s.notificationService, conv, content, senderName)

	s.logger.InfoContext(ctx, "postmark inbound created support message",
		"message_id", strings.TrimSpace(payload.MessageID),
		"conversation_id", conv.ID,
		"support_message_id", createdMsg.ID,
	)
	if route != nil && s.supportInboxService != nil && s.supportInboxService.emailRouteRepo != nil {
		_ = s.supportInboxService.emailRouteRepo.TouchInbound(ctx, route.ID, s.now())
	}
	s.wsPublisher.Publish(websocket.SupportMessageEvent(conv.WorkspaceID, createdMsg, "email:"+createdMsg.ID))
	return nil
}

func (s *EmailFallbackService) processDueEntries(ctx context.Context) error {
	now := s.now().Unix()
	due, err := s.redis.ZRangeByScore(ctx, emailFallbackOutboxKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()
	if err != nil {
		return fmt.Errorf("fetch due fallback entries: %w", err)
	}
	for _, conversationID := range due {
		if err := s.claimAndFire(ctx, conversationID); err != nil {
			s.logger.ErrorContext(ctx, "email fallback processing failed", "conversation_id", conversationID, "error", err)
		}
	}
	return nil
}

func (s *EmailFallbackService) claimAndFire(ctx context.Context, conversationID string) error {
	score, err := s.redis.ZScore(ctx, emailFallbackOutboxKey, conversationID).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get fallback outbox score: %w", err)
	}
	if int64(score) > s.now().Unix() {
		return nil
	}

	claimUntil := s.now().Add(s.processingTTL)
	if _, err := s.redis.ZAddArgs(ctx, emailFallbackOutboxKey, redis.ZAddArgs{
		XX:      true,
		GT:      true,
		Members: []redis.Z{{Score: float64(claimUntil.Unix()), Member: conversationID}},
	}).Result(); err != nil {
		return fmt.Errorf("claim fallback outbox entry: %w", err)
	}

	msgIDs, err := s.redis.LRange(ctx, s.msgListKey(conversationID), 0, -1).Result()
	if err != nil {
		return fmt.Errorf("load fallback message ids: %w", err)
	}

	return s.fireEmail(ctx, conversationID, uniqueEmailFallbackStrings(msgIDs))
}

func (s *EmailFallbackService) fireEmail(ctx context.Context, conversationID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return s.cleanup(ctx, conversationID)
	}

	messages, err := s.messageRepo.GetByIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return s.cleanup(ctx, conversationID)
	}

	allAlreadyNotified := true
	var pending []model.SupportMessage
	for _, msg := range messages {
		if msg.EmailNotifiedAt == nil {
			allAlreadyNotified = false
			pending = append(pending, msg)
		}
	}
	if allAlreadyNotified || len(pending) == 0 {
		return s.cleanup(ctx, conversationID)
	}

	conv, err := s.findConversationByID(ctx, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return s.cleanup(ctx, conversationID)
	}
	if isEmailFallbackTerminalStatus(conv.Status) || conv.EmailUnsubscribed {
		return s.cleanup(ctx, conversationID)
	}
	if conv.CustomerEmail == nil || strings.TrimSpace(*conv.CustomerEmail) == "" {
		return s.cleanup(ctx, conversationID)
	}
	if s.contactRepo != nil {
		if contact, err := s.contactRepo.GetByEmail(ctx, conv.WorkspaceID, strings.TrimSpace(*conv.CustomerEmail)); err == nil && contact != nil && contact.EmailStatus == model.CRMContactEmailStatusInvalid {
			s.logger.InfoContext(ctx, "email fallback skipped — recipient marked invalid",
				"conversation_id", conversationID,
				"email", strings.TrimSpace(*conv.CustomerEmail),
				"reason", derefString(contact.EmailStatusReason),
			)
			return s.cleanup(ctx, conversationID)
		}
	}
	if online, err := s.isVisitorOnline(ctx, conv.WorkspaceID, conv.AnonymousID); err == nil && online {
		return s.cleanup(ctx, conversationID)
	}

	settings, err := s.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return err
	}
	if !settings.EmailFallbackEnabled {
		return s.cleanup(ctx, conversationID)
	}

	workspace, err := s.workspaceRepo.GetByID(ctx, conv.WorkspaceID)
	if err != nil {
		return err
	}
	workspaceName := strings.TrimSpace(settings.EmailFallbackFromName)
	if workspaceName == "" && workspace != nil {
		workspaceName = strings.TrimSpace(workspace.Name)
	}
	if workspaceName == "" {
		workspaceName = "Helpin Support"
	}

	agentName := "Support"
	if lastName := strings.TrimSpace(derefString(pending[len(pending)-1].SenderDisplayName)); lastName != "" {
		agentName = lastName
	}
	fromAddress := s.resolveOutboundFromAddress(ctx, conv)
	from := fmt.Sprintf("%s - %s <%s>", agentName, workspaceName, fromAddress)

	logID := uuid.NewString()
	rfcMessageID := fmt.Sprintf("<helpin-%s@%s>", logID, s.replyDomain)
	headers, err := s.buildThreadHeaders(ctx, conv.WorkspaceID, conversationID, rfcMessageID)
	if err != nil {
		return err
	}
	replyTo := fmt.Sprintf("conv-%s@%s", conversationID, s.replyDomain)
	unsubscribeEmail := s.unsubscribeAddress(conversationID)

	subject, err := s.buildSubject(ctx, conv, pending)
	if err != nil {
		return err
	}
	chatLink, _ := s.buildChatLink(ctx, conv)
	htmlBody, textBody := s.renderBodies(pending, agentName, workspaceName, chatLink, unsubscribeEmail)

	postmarkMessageID, err := s.emailClient.SendEmailWithHeaders(
		from,
		strings.TrimSpace(*conv.CustomerEmail),
		subject,
		htmlBody,
		textBody,
		replyTo,
		headers,
	)
	if err != nil {
		return fmt.Errorf("send fallback email: %w", err)
	}

	inReplyTo := headerValue(headers, "In-Reply-To")
	notifiedAt := s.now()
	messageIDValues := make([]string, 0, len(pending))
	for _, msg := range pending {
		messageIDValues = append(messageIDValues, msg.ID)
	}
	logRow := &model.SupportEmailLog{
		ID:                logID,
		WorkspaceID:       conv.WorkspaceID,
		ConversationID:    conversationID,
		Direction:         "outbound",
		MessageIDs:        model.DocsStringArray(messageIDValues),
		FromEmail:         s.emailClient.FromEmail(),
		ToEmail:           strings.TrimSpace(*conv.CustomerEmail),
		Subject:           subject,
		RFCMessageID:      rfcMessageID,
		InReplyTo:         inReplyTo,
		PostmarkMessageID: strPtr(strings.TrimSpace(postmarkMessageID)),
		StrippedText:      textBody,
		Status:            "sent",
	}

	txErr := s.convRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.emailLogRepo.WithTx(tx).Create(ctx, logRow); err != nil {
			return err
		}
		return s.messageRepo.WithTx(tx).UpdateEmailNotifiedAt(ctx, messageIDValues, notifiedAt)
	})
	if txErr != nil {
		return txErr
	}

	s.publishMessageUpdated(conv.WorkspaceID, conversationID, lastString(messageIDValues), "postmark:sent")

	return s.cleanup(ctx, conversationID)
}

// ProcessOpenEvent records an outbound email open and mirrors it onto the related support messages.
func (s *EmailFallbackService) ProcessOpenEvent(ctx context.Context, payload model.PostmarkOpenPayload, rawPayload string) error {
	if s == nil || s.emailLogRepo == nil || s.messageRepo == nil {
		return nil
	}
	if strings.TrimSpace(rawPayload) == "" {
		rawPayloadBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal open payload: %w", err)
		}
		rawPayload = string(rawPayloadBytes)
	}

	postmarkMessageID := strings.TrimSpace(payload.MessageID)
	if postmarkMessageID == "" {
		s.recordWebhookEvent(ctx, "open", "", strings.TrimSpace(payload.MessageStream), rawPayload, nil, nil, parsePostmarkTimestamp(payload.ReceivedAt))
		s.logger.InfoContext(ctx, "postmark open ignored without message id")
		return nil
	}

	logRow, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, postmarkMessageID)
	if err != nil {
		return err
	}
	var conv *model.SupportConversation
	if logRow != nil && strings.TrimSpace(logRow.ConversationID) != "" {
		conv, _ = s.findConversationByID(ctx, logRow.ConversationID)
	}
	s.recordWebhookEvent(ctx, "open", postmarkMessageID, strings.TrimSpace(payload.MessageStream), rawPayload, conv, logRow, parsePostmarkTimestamp(payload.ReceivedAt))
	if logRow == nil || logRow.Direction != "outbound" || len(logRow.MessageIDs) == 0 {
		s.logger.InfoContext(ctx, "postmark open ignored without matching outbound email",
			"message_id", postmarkMessageID,
		)
		return nil
	}
	if logRow.OpenedAt != nil && !payload.FirstOpen {
		s.logger.InfoContext(ctx, "postmark open duplicate ignored",
			"message_id", postmarkMessageID,
			"conversation_id", logRow.ConversationID,
		)
		return nil
	}

	readAt := s.now()
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.ReceivedAt)); err == nil {
		readAt = parsed.UTC()
	}

	txErr := s.messageRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.emailLogRepo.WithTx(tx).MarkOpened(ctx, logRow.ID, readAt); err != nil {
			return err
		}
		return s.messageRepo.WithTx(tx).UpdateEmailReadAt(ctx, []string(logRow.MessageIDs), readAt)
	})
	if txErr != nil {
		return txErr
	}

	s.logger.InfoContext(ctx, "postmark open marked support message read",
		"message_id", postmarkMessageID,
		"conversation_id", logRow.ConversationID,
		"message_count", len(logRow.MessageIDs),
	)
	s.publishMessageUpdated(logRow.WorkspaceID, logRow.ConversationID, lastString([]string(logRow.MessageIDs)), "postmark:open")
	return nil
}

// ProcessDeliveryEvent records an outbound email delivery and mirrors it onto the related support conversation.
func (s *EmailFallbackService) ProcessDeliveryEvent(ctx context.Context, payload model.PostmarkDeliveryPayload, rawPayload string) error {
	if s == nil || s.emailLogRepo == nil {
		return nil
	}
	if strings.TrimSpace(rawPayload) == "" {
		rawPayloadBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal delivery payload: %w", err)
		}
		rawPayload = string(rawPayloadBytes)
	}

	postmarkMessageID := strings.TrimSpace(payload.MessageID)
	receivedAt := parsePostmarkTimestamp(payload.DeliveredAt)
	if postmarkMessageID == "" {
		s.recordWebhookEvent(ctx, "delivery", "", strings.TrimSpace(payload.MessageStream), rawPayload, nil, nil, receivedAt)
		s.logger.InfoContext(ctx, "postmark delivery ignored without message id")
		return nil
	}

	logRow, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, postmarkMessageID)
	if err != nil {
		return err
	}
	var conv *model.SupportConversation
	if logRow != nil && strings.TrimSpace(logRow.ConversationID) != "" {
		conv, _ = s.findConversationByID(ctx, logRow.ConversationID)
	}
	s.recordWebhookEvent(ctx, "delivery", postmarkMessageID, strings.TrimSpace(payload.MessageStream), rawPayload, conv, logRow, receivedAt)
	if logRow == nil || logRow.Direction != "outbound" || len(logRow.MessageIDs) == 0 {
		s.logger.InfoContext(ctx, "postmark delivery ignored without matching outbound email",
			"message_id", postmarkMessageID,
		)
		return nil
	}
	if logRow.DeliveredAt != nil {
		s.logger.InfoContext(ctx, "postmark delivery duplicate ignored",
			"message_id", postmarkMessageID,
			"conversation_id", logRow.ConversationID,
		)
		return nil
	}
	if logRow.Status == "bounced" || logRow.Status == "spam_complaint" {
		s.logger.InfoContext(ctx, "postmark delivery ignored after terminal delivery failure",
			"message_id", postmarkMessageID,
			"conversation_id", logRow.ConversationID,
			"status", logRow.Status,
		)
		return nil
	}

	deliveredAt := s.now()
	if receivedAt != nil {
		deliveredAt = *receivedAt
	}
	if err := s.emailLogRepo.MarkDelivered(ctx, logRow.ID, deliveredAt); err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "postmark delivery marked support email delivered",
		"message_id", postmarkMessageID,
		"conversation_id", logRow.ConversationID,
	)
	s.publishMessageUpdated(logRow.WorkspaceID, logRow.ConversationID, lastString([]string(logRow.MessageIDs)), "postmark:delivery")
	return nil
}

// ProcessBounceEvent records an outbound email bounce and mirrors it onto the related support conversation.
func (s *EmailFallbackService) ProcessBounceEvent(ctx context.Context, payload model.PostmarkBouncePayload, rawPayload string) error {
	if s == nil || s.emailLogRepo == nil {
		return nil
	}
	if strings.TrimSpace(rawPayload) == "" {
		rawPayloadBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal bounce payload: %w", err)
		}
		rawPayload = string(rawPayloadBytes)
	}

	postmarkMessageID := strings.TrimSpace(payload.MessageID)
	receivedAt := parsePostmarkTimestamp(payload.BouncedAt)
	if postmarkMessageID == "" {
		s.recordWebhookEvent(ctx, "bounce", "", strings.TrimSpace(payload.MessageStream), rawPayload, nil, nil, receivedAt)
		s.logger.InfoContext(ctx, "postmark bounce ignored without message id")
		return nil
	}

	logRow, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, postmarkMessageID)
	if err != nil {
		return err
	}
	var conv *model.SupportConversation
	if logRow != nil && strings.TrimSpace(logRow.ConversationID) != "" {
		conv, _ = s.findConversationByID(ctx, logRow.ConversationID)
	}
	s.recordWebhookEvent(ctx, "bounce", postmarkMessageID, strings.TrimSpace(payload.MessageStream), rawPayload, conv, logRow, receivedAt)
	if logRow == nil || logRow.Direction != "outbound" || len(logRow.MessageIDs) == 0 {
		s.logger.InfoContext(ctx, "postmark bounce ignored without matching outbound email",
			"message_id", postmarkMessageID,
		)
		return nil
	}
	if logRow.Status == "spam_complaint" {
		s.logger.InfoContext(ctx, "postmark bounce ignored after spam complaint",
			"message_id", postmarkMessageID,
			"conversation_id", logRow.ConversationID,
		)
		return nil
	}
	if logRow.BouncedAt != nil && logRow.Status == "bounced" {
		s.logger.InfoContext(ctx, "postmark bounce duplicate ignored",
			"message_id", postmarkMessageID,
			"conversation_id", logRow.ConversationID,
		)
		return nil
	}
	if logRow.Status == "opened" {
		s.logger.InfoContext(ctx, "postmark bounce ignored after open",
			"message_id", postmarkMessageID,
			"conversation_id", logRow.ConversationID,
		)
		return nil
	}

	bouncedAt := s.now()
	if receivedAt != nil {
		bouncedAt = *receivedAt
	}
	description := strings.TrimSpace(payload.Description)
	if description == "" {
		description = strings.TrimSpace(payload.Details)
	}
	if err := s.emailLogRepo.MarkBounced(ctx, logRow.ID, "bounced", bouncedAt, description); err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "postmark bounce marked support email bounced",
		"message_id", postmarkMessageID,
		"conversation_id", logRow.ConversationID,
	)
	if bounceTypeIsPermanent(payload.Type) {
		recipient := strings.TrimSpace(payload.Recipient)
		if recipient == "" {
			recipient = strings.TrimSpace(logRow.ToEmail)
		}
		s.invalidateContactEmail(ctx, logRow.WorkspaceID, recipient, fmt.Sprintf("hard bounce: %s", strings.TrimSpace(payload.Type)))
	}
	s.publishMessageUpdated(logRow.WorkspaceID, logRow.ConversationID, lastString([]string(logRow.MessageIDs)), "postmark:bounce")
	return nil
}

// bounceTypeIsPermanent returns true for Postmark bounce types that indicate
// the recipient's address is permanently undeliverable. Transient bounces
// (SoftBounce, Transient, DnsError) are not permanent.
func bounceTypeIsPermanent(bounceType string) bool {
	switch strings.TrimSpace(bounceType) {
	case "HardBounce", "BadEmailAddress", "Blocked", "ManuallyDeactivated", "Unconfirmed":
		return true
	}
	return false
}

// invalidateContactEmail flags any CRM contact in the workspace whose email
// matches the bounced recipient as undeliverable. Logs and swallows errors —
// the webhook still succeeds if the CRM update fails.
func (s *EmailFallbackService) invalidateContactEmail(ctx context.Context, workspaceID, email, reason string) {
	if s.contactRepo == nil || strings.TrimSpace(email) == "" {
		return
	}
	if err := s.contactRepo.MarkEmailInvalid(ctx, workspaceID, email, reason); err != nil {
		s.logger.ErrorContext(ctx, "mark crm contact email invalid",
			"error", err, "workspace_id", workspaceID, "email", email)
		return
	}
	s.logger.InfoContext(ctx, "crm contact email marked invalid",
		"workspace_id", workspaceID, "email", email, "reason", reason)
}

// ProcessSpamComplaintEvent records an outbound email spam complaint and mirrors it onto the related support conversation.
func (s *EmailFallbackService) ProcessSpamComplaintEvent(ctx context.Context, payload model.PostmarkSpamComplaintPayload, rawPayload string) error {
	if s == nil || s.emailLogRepo == nil {
		return nil
	}
	if strings.TrimSpace(rawPayload) == "" {
		rawPayloadBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal spam complaint payload: %w", err)
		}
		rawPayload = string(rawPayloadBytes)
	}

	postmarkMessageID := strings.TrimSpace(payload.MessageID)
	receivedAt := parsePostmarkTimestamp(payload.BouncedAt)
	if postmarkMessageID == "" {
		s.recordWebhookEvent(ctx, "spam_complaint", "", strings.TrimSpace(payload.MessageStream), rawPayload, nil, nil, receivedAt)
		s.logger.InfoContext(ctx, "postmark spam complaint ignored without message id")
		return nil
	}

	logRow, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, postmarkMessageID)
	if err != nil {
		return err
	}
	var conv *model.SupportConversation
	if logRow != nil && strings.TrimSpace(logRow.ConversationID) != "" {
		conv, _ = s.findConversationByID(ctx, logRow.ConversationID)
	}
	s.recordWebhookEvent(ctx, "spam_complaint", postmarkMessageID, strings.TrimSpace(payload.MessageStream), rawPayload, conv, logRow, receivedAt)
	if logRow == nil || logRow.Direction != "outbound" || len(logRow.MessageIDs) == 0 {
		s.logger.InfoContext(ctx, "postmark spam complaint ignored without matching outbound email",
			"message_id", postmarkMessageID,
		)
		return nil
	}
	if logRow.BouncedAt != nil && logRow.Status == "spam_complaint" {
		s.logger.InfoContext(ctx, "postmark spam complaint duplicate ignored",
			"message_id", postmarkMessageID,
			"conversation_id", logRow.ConversationID,
		)
		return nil
	}

	complaintAt := s.now()
	if receivedAt != nil {
		complaintAt = *receivedAt
	}
	description := strings.TrimSpace(payload.Description)
	if description == "" {
		description = strings.TrimSpace(payload.Details)
	}
	if err := s.emailLogRepo.MarkBounced(ctx, logRow.ID, "spam_complaint", complaintAt, description); err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "postmark spam complaint marked support email complained",
		"message_id", postmarkMessageID,
		"conversation_id", logRow.ConversationID,
	)
	recipient := strings.TrimSpace(payload.Recipient)
	if recipient == "" {
		recipient = strings.TrimSpace(logRow.ToEmail)
	}
	s.invalidateContactEmail(ctx, logRow.WorkspaceID, recipient, "spam complaint")
	s.publishMessageUpdated(logRow.WorkspaceID, logRow.ConversationID, lastString([]string(logRow.MessageIDs)), "postmark:spam_complaint")
	return nil
}

func (s *EmailFallbackService) loadSettings(ctx context.Context, workspaceID string) (model.SupportInboxSettings, error) {
	settings := model.DefaultSupportInboxSettings()
	if s.installRepo == nil {
		return settings, nil
	}
	inst, err := s.installRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return settings, err
	}
	if inst == nil {
		return settings, nil
	}
	return parseSettings(inst.Settings), nil
}

func (s *EmailFallbackService) acquireOrRenewLease(ctx context.Context) (bool, error) {
	if s.podID == "" {
		s.podID = "email-fallback"
	}
	current, err := s.redis.Get(ctx, emailFallbackLockKey).Result()
	if errors.Is(err, redis.Nil) {
		return s.redis.SetNX(ctx, emailFallbackLockKey, s.podID, s.leaseTTL).Result()
	}
	if err != nil {
		return false, err
	}
	if current == s.podID {
		if err := s.redis.Expire(ctx, emailFallbackLockKey, s.leaseTTL).Err(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (s *EmailFallbackService) buildThreadHeaders(ctx context.Context, workspaceID, conversationID, nextMessageID string) ([]email.EmailHeader, error) {
	logs, err := s.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}

	var previous []string
	for _, entry := range logs {
		if entry.Direction != "outbound" || strings.TrimSpace(entry.RFCMessageID) == "" {
			continue
		}
		previous = append(previous, strings.TrimSpace(entry.RFCMessageID))
	}

	headers := []email.EmailHeader{
		{Name: "Message-ID", Value: nextMessageID},
		{Name: "X-Conversation-ID", Value: conversationID},
		{Name: "List-Unsubscribe", Value: fmt.Sprintf("<mailto:%s>", s.unsubscribeAddress(conversationID))},
	}
	if len(previous) > 0 {
		headers = append(headers, email.EmailHeader{Name: "In-Reply-To", Value: previous[len(previous)-1]})
		headers = append(headers, email.EmailHeader{Name: "References", Value: strings.Join(previous, " ")})
	}
	return headers, nil
}

func (s *EmailFallbackService) buildSubject(ctx context.Context, conv *model.SupportConversation, pending []model.SupportMessage) (string, error) {
	base := strings.TrimSpace(conv.Subject)
	if base == "" {
		if len(pending) > 0 {
			base = truncateEmailFallbackString(strings.TrimSpace(pending[0].Content), 60)
		}
		if base == "" {
			base = "Support conversation"
		}
	}
	shortID := conv.ID
	if len(shortID) > 4 {
		shortID = shortID[:4]
	}
	subject := fmt.Sprintf("%s (#%s)", base, shortID)

	logs, err := s.emailLogRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID)
	if err != nil {
		return "", err
	}
	for _, logRow := range logs {
		if logRow.Direction == "outbound" {
			return "Re: " + subject, nil
		}
	}
	return subject, nil
}

func (s *EmailFallbackService) buildChatLink(ctx context.Context, conv *model.SupportConversation) (string, error) {
	pageURL := strings.TrimSpace(s.appBaseURL)
	if s.sessionRepo != nil {
		session, err := s.sessionRepo.GetLatestByConversation(ctx, conv.WorkspaceID, conv.ID)
		if err != nil {
			return "", err
		}
		if session != nil && session.LastPageURL != nil && strings.TrimSpace(*session.LastPageURL) != "" {
			pageURL = strings.TrimSpace(*session.LastPageURL)
		}
	}
	if pageURL == "" {
		return "", nil
	}
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return pageURL, nil
	}
	parsed.Fragment = "helpin-conv=" + conv.ID
	return parsed.String(), nil
}

func (s *EmailFallbackService) renderBodies(messages []model.SupportMessage, agentName, workspaceName, chatLink, unsubscribeEmail string) (string, string) {
	htmlChunks := make([]string, 0, len(messages))
	textChunks := make([]string, 0, len(messages))
	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		textChunks = append(textChunks, text)
		htmlChunks = append(htmlChunks, renderMessageMarkdownToHTML(text))
	}

	var htmlBody strings.Builder
	for _, chunk := range htmlChunks {
		htmlBody.WriteString(chunk)
	}
	htmlBody.WriteString("<p>--<br>")
	htmlBody.WriteString(html.EscapeString(agentName))
	htmlBody.WriteString(" via ")
	htmlBody.WriteString(html.EscapeString(workspaceName))
	htmlBody.WriteString("</p>")
	if chatLink != "" {
		htmlBody.WriteString(`<p>Reply directly to this email, or open the chat:<br><a href="`)
		htmlBody.WriteString(html.EscapeString(chatLink))
		htmlBody.WriteString(`">`)
		htmlBody.WriteString(html.EscapeString(chatLink))
		htmlBody.WriteString("</a></p>")
	} else {
		htmlBody.WriteString("<p>Reply directly to this email.</p>")
	}
	if unsubscribeEmail != "" {
		htmlBody.WriteString(`<p><a href="mailto:`)
		htmlBody.WriteString(html.EscapeString(unsubscribeEmail))
		htmlBody.WriteString(`">Unsubscribe</a> from these emails.</p>`)
	}

	textBody := strings.Join(textChunks, "\n\n")
	if textBody != "" {
		textBody += "\n\n"
	}
	textBody += "--\n" + agentName + " via " + workspaceName + "\n\nReply directly to this email"
	if chatLink != "" {
		textBody += ", or open the chat:\n" + chatLink
	} else {
		textBody += "."
	}
	if unsubscribeEmail != "" {
		textBody += "\nUnsubscribe: mailto:" + unsubscribeEmail
	}
	return htmlBody.String(), textBody
}

func (s *EmailFallbackService) cleanup(ctx context.Context, conversationID string) error {
	if s == nil || s.redis == nil {
		return nil
	}
	if err := s.redis.ZRem(ctx, emailFallbackOutboxKey, conversationID).Err(); err != nil {
		return err
	}
	if err := s.redis.Del(ctx, s.msgListKey(conversationID)).Err(); err != nil {
		return err
	}
	return nil
}

func (s *EmailFallbackService) msgListKey(conversationID string) string {
	return emailFallbackMsgsKeyPrefix + conversationID
}

// ListQueue returns all pending entries in the email fallback outbox for admin inspection.
func (s *EmailFallbackService) ListQueue(ctx context.Context) (*model.EmailQueueResponse, error) {
	if s.redis == nil {
		return &model.EmailQueueResponse{}, nil
	}

	entries, err := s.redis.ZRangeWithScores(ctx, emailFallbackOutboxKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("read email outbox: %w", err)
	}

	now := s.now()
	result := make([]model.EmailQueueEntry, 0, len(entries))

	for _, z := range entries {
		convID, ok := z.Member.(string)
		if !ok {
			continue
		}

		fireAt := time.Unix(int64(z.Score), 0)
		remaining := int(fireAt.Sub(now).Seconds())
		if remaining < 0 {
			remaining = 0
		}

		entry := model.EmailQueueEntry{
			ConversationID:     convID,
			FireAt:             fireAt,
			DelayRemainingSecs: remaining,
		}

		// Load pending message IDs from Redis.
		msgIDs, err := s.redis.LRange(ctx, s.msgListKey(convID), 0, -1).Result()
		if err != nil {
			slog.WarnContext(ctx, "email queue: read msg list", "error", err, "conversation_id", convID)
			msgIDs = nil
		}
		entry.MessageCount = len(msgIDs)

		// Enrich with conversation data.
		conv, err := s.findConversationByID(ctx, convID)
		if err != nil {
			slog.WarnContext(ctx, "email queue: load conversation", "error", err, "conversation_id", convID)
		}
		if conv != nil {
			entry.WorkspaceID = conv.WorkspaceID
			entry.Subject = conv.Subject
			entry.Status = conv.Status
			if conv.CustomerEmail != nil {
				entry.CustomerEmail = *conv.CustomerEmail
			}
			if conv.CustomerName != nil {
				entry.CustomerName = *conv.CustomerName
			}
		}

		// Load message previews.
		if len(msgIDs) > 0 {
			messages, err := s.messageRepo.GetByIDs(ctx, msgIDs)
			if err != nil {
				slog.WarnContext(ctx, "email queue: load messages", "error", err, "conversation_id", convID)
			} else {
				queueMsgs := make([]model.EmailQueueMessage, 0, len(messages))
				for _, m := range messages {
					name := ""
					if m.SenderDisplayName != nil {
						name = *m.SenderDisplayName
					}
					queueMsgs = append(queueMsgs, model.EmailQueueMessage{
						ID:                m.ID,
						Content:           m.Content,
						SenderDisplayName: name,
						CreatedAt:         m.CreatedAt,
					})
				}
				entry.Messages = queueMsgs
			}
		}

		result = append(result, entry)
	}

	return &model.EmailQueueResponse{
		Entries: result,
		Total:   len(result),
	}, nil
}

func (s *EmailFallbackService) unsubscribeAddress(conversationID string) string {
	domain := strings.TrimSpace(s.replyDomain)
	if domain == "" {
		domain = "replies.helpin.email"
	}
	return fmt.Sprintf("unsubscribe-%s@%s", conversationID, domain)
}

// resolveOutboundFromAddress returns the branded sender address for a
// conversation's outbound email. It prefers the mailbox-aware route address
// (<handle>@<slug>.<route_domain>) so replies land on the verified customer
// domain, falling back to the legacy global Postmark sender when the slug or
// route domain is unavailable.
func (s *EmailFallbackService) resolveOutboundFromAddress(ctx context.Context, conv *model.SupportConversation) string {
	fallback := s.emailClient.FromEmail()
	if s.supportInboxService == nil || conv == nil {
		return fallback
	}
	addr, err := s.supportInboxService.BuildOutboundFromAddress(ctx, conv.WorkspaceID, conv.MailboxID)
	if err != nil {
		s.logger.WarnContext(ctx, "mailbox-branded outbound from unavailable, using fallback sender",
			"error", err,
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conv.ID,
		)
		return fallback
	}
	if strings.TrimSpace(addr) == "" {
		return fallback
	}
	return addr
}

func (s *EmailFallbackService) isVisitorOnline(ctx context.Context, workspaceID string, anonymousID *string) (bool, error) {
	if anonymousID == nil || strings.TrimSpace(*anonymousID) == "" || s.hub == nil {
		return false, nil
	}
	visitors, err := s.hub.Presence.GetOnlineVisitors(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	for _, visitorID := range visitors {
		if visitorID == *anonymousID {
			return true, nil
		}
	}
	return false, nil
}

func (s *EmailFallbackService) findConversationByID(ctx context.Context, conversationID string) (*model.SupportConversation, error) {
	if strings.TrimSpace(conversationID) == "" {
		return nil, nil
	}
	var conv model.SupportConversation
	err := s.convRepo.DB().WithContext(ctx).Where("id = ?", conversationID).First(&conv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conv, nil
}

func (s *EmailFallbackService) processInboundRouteEmail(ctx context.Context, mailboxHash string, payload model.PostmarkInboundPayload, rawPayload string) error {
	if s == nil || s.supportInboxService == nil || s.supportInboxService.emailRouteRepo == nil {
		return fmt.Errorf("support email routes are unavailable")
	}

	route, err := s.supportInboxService.emailRouteRepo.GetByRouteKey(ctx, mailboxHash)
	if err != nil {
		return err
	}
	if route == nil || !route.Active {
		s.logger.InfoContext(ctx, "postmark inbound route not found",
			"message_id", strings.TrimSpace(payload.MessageID),
			"route_key", mailboxHash,
		)
		return nil
	}

	return s.processInboundRoute(ctx, route, payload, rawPayload)
}

func (s *EmailFallbackService) processInboundRoute(ctx context.Context, route *model.SupportEmailRoute, payload model.PostmarkInboundPayload, rawPayload string) error {
	if route == nil {
		return nil
	}
	if threadedConversation, err := s.resolveInboundRouteConversation(ctx, route.WorkspaceID, payload); err != nil {
		return err
	} else if threadedConversation != nil {
		return s.processInboundConversationReply(ctx, threadedConversation, route, payload, rawPayload)
	}

	return s.createInboundConversationFromRoute(ctx, route, payload, rawPayload)
}

func (s *EmailFallbackService) findInboundRouteByRecipient(ctx context.Context, recipientAddress string) (*model.SupportEmailRoute, error) {
	if s == nil || s.supportInboxService == nil || s.supportInboxService.emailRouteRepo == nil {
		return nil, nil
	}
	recipientAddress = strings.TrimSpace(recipientAddress)
	if recipientAddress == "" {
		return nil, nil
	}
	return s.supportInboxService.emailRouteRepo.GetActiveByInboundAddress(ctx, recipientAddress)
}

func (s *EmailFallbackService) resolveInboundRouteConversation(ctx context.Context, workspaceID string, payload model.PostmarkInboundPayload) (*model.SupportConversation, error) {
	if s == nil || s.emailLogRepo == nil {
		return nil, nil
	}

	threadLog, err := s.emailLogRepo.FindConversationByRFCReferences(ctx, workspaceID, inboundThreadReferences(payload))
	if err != nil {
		return nil, err
	}
	if threadLog == nil || strings.TrimSpace(threadLog.ConversationID) == "" {
		return nil, nil
	}
	return s.findConversationByID(ctx, threadLog.ConversationID)
}

func (s *EmailFallbackService) createInboundConversationFromRoute(ctx context.Context, route *model.SupportEmailRoute, payload model.PostmarkInboundPayload, rawPayload string) error {
	if route == nil {
		return nil
	}

	fromEmail := inboundEmailAddress(payload)
	if strings.TrimSpace(fromEmail) == "" {
		return nil
	}

	content, htmlBody := inboundPayloadBodies(payload)
	if content == "" {
		return nil
	}
	if len(content) > 50_000 {
		content = content[:50_000]
	}

	senderName := strings.TrimSpace(payload.FromFull.Name)
	if senderName == "" {
		senderName = fromEmail
	}

	subject := strings.TrimSpace(payload.Subject)
	if subject == "" {
		subject = fmt.Sprintf("Email from %s", senderName)
	}

	customerName := senderName
	customerEmail := fromEmail
	viaEmail := "email"
	now := s.now()

	conversation := &model.SupportConversation{
		WorkspaceID:   route.WorkspaceID,
		MailboxID:     route.MailboxID,
		Subject:       subject,
		Status:        "open",
		FlowState:     strPtr(model.SupportConversationFlowStateWaitingForHuman),
		Priority:      "medium",
		Channel:       "email",
		CustomerName:  &customerName,
		CustomerEmail: &customerEmail,
		Source:        "email",
	}

	if conversation.MailboxID == nil && s.supportInboxService != nil {
		if mailboxID, _, mailboxErr := s.supportInboxService.maybeApplyMailboxRoutingForChannel(ctx, route.WorkspaceID, nil, true, "email"); mailboxErr == nil {
			conversation.MailboxID = mailboxID
		}
	}

	var (
		mailbox *model.SupportMailbox
		err     error
	)
	if route.MailboxID != nil && strings.TrimSpace(*route.MailboxID) != "" && s.supportInboxService.mailboxRepo != nil {
		mailbox, err = s.supportInboxService.mailboxRepo.GetByID(ctx, route.WorkspaceID, strings.TrimSpace(*route.MailboxID))
		if err != nil {
			return err
		}
	}
	if mailbox != nil {
		ownerID, flowState, ownerErr := s.supportInboxService.determineMailboxOwner(ctx, route.WorkspaceID, mailbox, nil)
		if ownerErr != nil {
			return ownerErr
		}
		conversation.AssignedUserID = ownerID
		conversation.FlowState = strPtr(flowState)
	}

	message := &model.SupportMessage{
		WorkspaceID:       route.WorkspaceID,
		SenderType:        "customer",
		SenderDisplayName: &customerName,
		Content:           content,
		IsInternal:        false,
		MessageType:       "reply",
		ViaChannel:        &viaEmail,
	}

	rfcMessageID := inboundRFCMessageID(payload)
	inReplyTo := normalizeRFCHeaderValue(inboundHeaderValue(payload.Headers, "In-Reply-To"))
	referencesHeader := strings.TrimSpace(inboundHeaderValue(payload.Headers, "References"))
	recipientAddress := inboundRecipientAddress(payload)

	txErr := s.convRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		convRepoTx := s.convRepo.WithTx(tx)
		msgRepoTx := s.messageRepo.WithTx(tx)
		emailLogRepoTx := s.emailLogRepo.WithTx(tx)

		if err := convRepoTx.Create(ctx, conversation); err != nil {
			return err
		}

		if s.supportInboxService != nil && s.supportInboxService.contactRepo != nil {
			if contactID := s.supportInboxService.matchOrCreateCRMContactTx(ctx, s.supportInboxService.contactRepo.WithTx(tx), conversation.WorkspaceID, conversation.CustomerEmail, conversation.CustomerName, "email_forward"); contactID != nil {
				conversation.CRMContactID = contactID
				if err := convRepoTx.Update(ctx, conversation); err != nil {
					return err
				}
			}
		}

		message.ConversationID = conversation.ID
		if err := msgRepoTx.Create(ctx, message); err != nil {
			return err
		}

		logRow := &model.SupportEmailLog{
			WorkspaceID:       route.WorkspaceID,
			ConversationID:    conversation.ID,
			EmailRouteID:      &route.ID,
			Direction:         "inbound",
			MessageIDs:        model.DocsStringArray{message.ID},
			FromEmail:         fromEmail,
			ToEmail:           strings.TrimSpace(payload.To),
			RecipientAddress:  recipientAddress,
			Subject:           subject,
			RFCMessageID:      rfcMessageID,
			InReplyTo:         inReplyTo,
			ReferencesHeader:  referencesHeader,
			PostmarkMessageID: strPtr(strings.TrimSpace(payload.MessageID)),
			RawBody:           rawPayload,
			StrippedText:      content,
			HTMLBody:          htmlBody,
			Status:            "sent",
		}
		if logRow.PostmarkMessageID != nil && *logRow.PostmarkMessageID == "" {
			logRow.PostmarkMessageID = nil
		}
		if err := emailLogRepoTx.Create(ctx, logRow); err != nil {
			return err
		}

		return nil
	})
	if txErr != nil {
		if isLikelyUniqueConstraintError(txErr) {
			s.logger.InfoContext(ctx, "postmark inbound route duplicate ignored after transaction race",
				"message_id", strings.TrimSpace(payload.MessageID),
				"route_key", route.RouteKey,
			)
			return nil
		}
		return txErr
	}
	if s.supportInboxService.emailRouteRepo != nil {
		_ = s.supportInboxService.emailRouteRepo.TouchInbound(ctx, route.ID, now)
	}

	ProcessSupportCustomerReplyNotification(ctx, s.notificationService, conversation, content, customerName)

	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "created",
			Entity:      "support_conversation",
			EntityID:    conversation.ID,
			WorkspaceID: conversation.WorkspaceID,
			ActorID:     "email:" + conversation.ID,
		})
		s.wsPublisher.Publish(websocket.SupportMessageEvent(conversation.WorkspaceID, message, "email:"+message.ID))
	}

	s.logger.InfoContext(ctx, "postmark inbound created support conversation from route",
		"message_id", strings.TrimSpace(payload.MessageID),
		"conversation_id", conversation.ID,
		"route_key", route.RouteKey,
	)
	if s.supportInboxService != nil && s.supportInboxService.triageService != nil {
		go func(workspaceID, conversationID, messageID string) {
			if _, err := s.supportInboxService.triageService.EvaluateAndRoute(context.Background(), workspaceID, conversationID, messageID); err != nil {
				s.logger.ErrorContext(context.Background(), "support triage failed for inbound email conversation", "workspace_id", workspaceID, "conversation_id", conversationID, "message_id", messageID, "error", err)
			}
		}(conversation.WorkspaceID, conversation.ID, message.ID)
	}
	return nil
}

func inboundEmailAddress(payload model.PostmarkInboundPayload) string {
	if strings.TrimSpace(payload.FromFull.Email) != "" {
		return strings.TrimSpace(payload.FromFull.Email)
	}
	if addr, err := mail.ParseAddress(strings.TrimSpace(payload.From)); err == nil {
		return strings.TrimSpace(addr.Address)
	}
	return strings.TrimSpace(payload.From)
}

func mailboxHashFromInboundPayload(payload model.PostmarkInboundPayload) string {
	if mailboxHash := strings.TrimSpace(payload.MailboxHash); mailboxHash != "" {
		return mailboxHash
	}
	for _, candidate := range []string{
		payload.OriginalRecipient,
		payload.To,
	} {
		if mailboxHash := mailboxHashFromRecipient(candidate); mailboxHash != "" {
			return mailboxHash
		}
	}
	for _, addr := range payload.ToFull {
		if mailboxHash := mailboxHashFromRecipient(addr.Email); mailboxHash != "" {
			return mailboxHash
		}
	}
	return ""
}

func mailboxHashFromRecipient(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if addr, err := mail.ParseAddress(value); err == nil {
		value = strings.TrimSpace(addr.Address)
	}
	at := strings.Index(value, "@")
	if at <= 0 {
		return ""
	}
	local := strings.TrimSpace(value[:at])
	if strings.HasPrefix(local, "conv-") || strings.HasPrefix(local, "unsubscribe-") || strings.HasPrefix(local, "route-") {
		return local
	}
	return ""
}

func inboundRecipientAddress(payload model.PostmarkInboundPayload) string {
	for _, candidate := range []string{payload.OriginalRecipient, payload.To} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if addr, err := mail.ParseAddress(candidate); err == nil {
			return strings.TrimSpace(addr.Address)
		}
		return candidate
	}
	for _, addr := range payload.ToFull {
		if strings.TrimSpace(addr.Email) != "" {
			return strings.TrimSpace(addr.Email)
		}
	}
	return ""
}

func inboundHeaderValue(headers []model.PostmarkHeader, name string) string {
	for _, header := range headers {
		if strings.EqualFold(strings.TrimSpace(header.Name), name) {
			return strings.TrimSpace(header.Value)
		}
	}
	return ""
}

func inboundRFCMessageID(payload model.PostmarkInboundPayload) string {
	if value := normalizeRFCHeaderValue(inboundHeaderValue(payload.Headers, "Message-ID")); value != "" {
		return value
	}
	return normalizeRFCHeaderValue(strings.TrimSpace(payload.MessageID))
}

func inboundThreadReferences(payload model.PostmarkInboundPayload) []string {
	references := parseRFCMessageIDList(inboundHeaderValue(payload.Headers, "In-Reply-To"))
	references = append(references, parseRFCMessageIDList(inboundHeaderValue(payload.Headers, "References"))...)
	return uniqueEmailFallbackStrings(references)
}

func parseRFCMessageIDList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	fields := strings.Fields(value)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(strings.Trim(field, ",;"))
		field = normalizeRFCHeaderValue(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func normalizeRFCHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "<") && strings.Contains(value, "@") {
		value = "<" + strings.Trim(value, "<>") + ">"
	}
	return value
}

func inboundConversationID(payload model.PostmarkInboundPayload) string {
	mailboxHash := mailboxHashFromInboundPayload(payload)
	switch {
	case strings.HasPrefix(mailboxHash, "conv-"):
		return strings.TrimSpace(strings.TrimPrefix(mailboxHash, "conv-"))
	case strings.HasPrefix(mailboxHash, "unsubscribe-"):
		return strings.TrimSpace(strings.TrimPrefix(mailboxHash, "unsubscribe-"))
	default:
		return ""
	}
}

func parseInboundWebhookReceivedAt(payload model.PostmarkInboundPayload) *time.Time {
	return parsePostmarkTimestamp(payload.Date)
}

func parsePostmarkTimestamp(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.RFC1123Z, time.RFC1123} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}

func headerValue(headers []email.EmailHeader, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return header.Value
		}
	}
	return ""
}

func isEmailFallbackTerminalStatus(status string) bool {
	switch model.NormalizeSupportConversationStatus(status) {
	case model.SupportConversationStatusResolved, model.SupportConversationStatusSpam:
		return true
	default:
		return false
	}
}

func isLikelyUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate")
}

func (s *EmailFallbackService) recordWebhookEvent(
	ctx context.Context,
	eventType string,
	postmarkMessageID string,
	messageStream string,
	rawPayload string,
	conv *model.SupportConversation,
	emailLog *model.SupportEmailLog,
	receivedAt *time.Time,
) {
	if s == nil || s.webhookRepo == nil || strings.TrimSpace(rawPayload) == "" {
		return
	}
	event := &model.SupportEmailWebhookEvent{
		Provider:          "postmark",
		EventType:         strings.TrimSpace(eventType),
		PostmarkMessageID: strPtr(strings.TrimSpace(postmarkMessageID)),
		MessageStream:     strPtr(strings.TrimSpace(messageStream)),
		RawPayload:        rawPayload,
		ReceivedAt:        receivedAt,
	}
	if event.PostmarkMessageID != nil && *event.PostmarkMessageID == "" {
		event.PostmarkMessageID = nil
	}
	if event.MessageStream != nil && *event.MessageStream == "" {
		event.MessageStream = nil
	}
	if conv != nil {
		event.WorkspaceID = &conv.WorkspaceID
		event.ConversationID = &conv.ID
	}
	if emailLog != nil {
		event.EmailLogID = &emailLog.ID
		if event.WorkspaceID == nil && strings.TrimSpace(emailLog.WorkspaceID) != "" {
			event.WorkspaceID = &emailLog.WorkspaceID
		}
		if event.ConversationID == nil && strings.TrimSpace(emailLog.ConversationID) != "" {
			event.ConversationID = &emailLog.ConversationID
		}
	}
	if err := s.webhookRepo.Create(ctx, event); err != nil {
		s.logger.WarnContext(ctx, "store support email webhook event failed", "event_type", eventType, "error", err)
	}
}

func (s *EmailFallbackService) publishMessageUpdated(workspaceID, conversationID, messageID, actorID string) {
	if s == nil || s.wsPublisher == nil || strings.TrimSpace(messageID) == "" {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation_message",
		EntityID:    messageID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  "support_conversation",
		ParentID:    conversationID,
	})
}

func lastString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[len(values)-1])
}

func uniqueEmailFallbackStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func truncateEmailFallbackString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
