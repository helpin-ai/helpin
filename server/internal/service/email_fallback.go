package service

import (
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
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

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

	mailboxHash := mailboxHashFromInboundPayload(payload)
	if mailboxHash == "" {
		s.logger.WarnContext(ctx, "postmark inbound missing mailbox hash",
			"message_id", strings.TrimSpace(payload.MessageID),
			"original_recipient", strings.TrimSpace(payload.OriginalRecipient),
			"to", strings.TrimSpace(payload.To),
		)
		return fmt.Errorf("missing mailbox hash")
	}
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

	content := strings.TrimSpace(payload.StrippedTextReply)
	if content == "" {
		content = strings.TrimSpace(payload.TextBody)
	}
	if content == "" {
		return nil
	}
	if len(content) > 50_000 {
		content = content[:50_000]
	}

	senderName := strings.TrimSpace(payload.FromFull.Name)
	if senderName == "" && conv.CustomerName != nil {
		senderName = strings.TrimSpace(*conv.CustomerName)
	}
	if senderName == "" {
		senderName = "Customer"
	}
	viaEmail := "email"

	var createdMsg *model.SupportMessage
	txErr := s.convRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
			Subject:           strings.TrimSpace(payload.Subject),
			PostmarkMessageID: strPtr(strings.TrimSpace(payload.MessageID)),
			RawBody:           rawPayload,
			StrippedText:      content,
			Status:            "sent",
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
	from := fmt.Sprintf("%s - %s <%s>", agentName, workspaceName, s.emailClient.FromEmail())

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
		htmlChunks = append(htmlChunks, strings.ReplaceAll(html.EscapeString(text), "\n", "<br>"))
	}

	var htmlBody strings.Builder
	for _, chunk := range htmlChunks {
		htmlBody.WriteString("<p>")
		htmlBody.WriteString(chunk)
		htmlBody.WriteString("</p>")
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
		domain = "replies.helpin.ai"
	}
	return fmt.Sprintf("unsubscribe-%s@%s", conversationID, domain)
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
	if strings.HasPrefix(local, "conv-") || strings.HasPrefix(local, "unsubscribe-") {
		return local
	}
	return ""
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
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "closed", "spam":
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
