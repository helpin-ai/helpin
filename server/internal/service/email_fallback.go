package service

import (
	"context"
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
	redis         *redis.Client
	hub           *websocket.Hub
	wsPublisher   *websocket.Publisher
	emailClient   *email.Client
	messageRepo   *repository.SupportMessageRepository
	convRepo      *repository.SupportConversationRepository
	emailLogRepo  *repository.SupportEmailLogRepository
	installRepo   *repository.SupportInboxInstallationRepository
	sessionRepo   *repository.SupportInboxSessionRepository
	workspaceRepo *repository.WorkspaceRepository
	replyDomain   string
	appBaseURL    string
	logger        *slog.Logger
	podID         string
	pollInterval  time.Duration
	leaseTTL      time.Duration
	processingTTL time.Duration
	now           func() time.Time
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
func (s *EmailFallbackService) ProcessInboundEmail(ctx context.Context, payload model.PostmarkInboundPayload) error {
	if s == nil {
		return nil
	}

	mailboxHash := strings.TrimSpace(payload.MailboxHash)
	if mailboxHash == "" {
		return fmt.Errorf("missing mailbox hash")
	}
	if existing, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, strings.TrimSpace(payload.MessageID)); err != nil {
		return err
	} else if existing != nil {
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
		return nil
	}
	if isEmailFallbackTerminalStatus(conv.Status) {
		return nil
	}

	fromEmail := inboundEmailAddress(payload)
	if conv.CustomerEmail == nil || !strings.EqualFold(strings.TrimSpace(*conv.CustomerEmail), fromEmail) {
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
			RawBody:           strings.TrimSpace(payload.HtmlBody),
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
			return nil
		}
		return txErr
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

	subject, err := s.buildSubject(ctx, conv, pending)
	if err != nil {
		return err
	}
	chatLink, _ := s.buildChatLink(ctx, conv)
	htmlBody, textBody := s.renderBodies(pending, agentName, workspaceName, chatLink)

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

	return s.cleanup(ctx, conversationID)
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
		{Name: "List-Unsubscribe", Value: fmt.Sprintf("<mailto:unsubscribe-%s@%s>", conversationID, s.replyDomain)},
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

func (s *EmailFallbackService) renderBodies(messages []model.SupportMessage, agentName, workspaceName, chatLink string) (string, string) {
	chunks := make([]string, 0, len(messages))
	textChunks := make([]string, 0, len(messages))
	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		textChunks = append(textChunks, text)
		chunks = append(chunks, strings.ReplaceAll(html.EscapeString(text), "\n", "<br>"))
	}

	htmlBody := `<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:600px;margin:0 auto;color:#1a1a1a;background:#ffffff;padding:24px;">`
	for _, chunk := range chunks {
		htmlBody += `<p style="margin:0 0 16px;line-height:1.6;font-size:15px;">` + chunk + `</p>`
	}
	htmlBody += `<div style="margin:24px 0 16px;color:#6b7280;">--</div>`
	htmlBody += `<p style="margin:0 0 12px;color:#6b7280;font-size:13px;">&#9679; ` + html.EscapeString(agentName) + ` via ` + html.EscapeString(workspaceName) + `.</p>`
	if chatLink != "" {
		htmlBody += `<p style="margin:0 0 12px;color:#6b7280;font-size:13px;">Reply directly to this email, or go to <a href="` + html.EscapeString(chatLink) + `">chat</a>.</p>`
	} else {
		htmlBody += `<p style="margin:0 0 12px;color:#6b7280;font-size:13px;">Reply directly to this email.</p>`
	}
	htmlBody += `<p style="margin:0;color:#6b7280;font-size:13px;">Sent from <a href="https://helpin.ai">Helpin</a>. Unsubscribe from these emails.</p></div>`

	textBody := strings.Join(textChunks, "\n\n")
	if textBody != "" {
		textBody += "\n\n"
	}
	textBody += "--\n" + agentName + " via " + workspaceName + "\n\nReply directly to this email"
	if chatLink != "" {
		textBody += ", or go to chat:\n" + chatLink
	} else {
		textBody += "."
	}
	textBody += "\n\nSent from Helpin (https://helpin.ai)."
	return htmlBody, textBody
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
