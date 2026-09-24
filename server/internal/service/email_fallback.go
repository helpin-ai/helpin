package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/helpin-ai/helpin/server/internal/deployment"
	"html"
	"log/slog"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

type inboundEmailProjection struct {
	VisibleText          string
	QuotedText           string
	HTMLBody             string
	HasQuotedContent     bool
	ProjectionConfidence string
	Version              int
}

// inboundPayloadProjection derives one lossless display projection for every
// consumer. Canonical provider bodies remain on the raw payload/log; this
// value contains sanitized HTML plus independently renderable visible and
// quoted Markdown.
func inboundPayloadProjection(payload model.PostmarkInboundPayload) inboundEmailProjection {
	strippedReply := stripSupportEmailReplyDelimiter(payload.StrippedTextReply)
	processed := inboundhtml.Project(inboundhtml.ProjectionInput{
		HTML:              payload.HtmlBody,
		TextBody:          payload.TextBody,
		StrippedTextReply: strippedReply,
	})

	visible := stripSupportEmailReplyDelimiter(processed.Markdown)
	quoted := strings.TrimSpace(processed.QuotedMarkdown)
	hasQuoted := processed.HasQuotedContent && quoted != ""
	confidence := processed.ProjectionConfidence

	// Helpin's own reply delimiter is deterministic even when an email client
	// has flattened the message into otherwise unstructured HTML.
	if before, after, ok := splitSupportEmailReplyDelimiter(processed.Markdown); ok {
		visible = stripInboundCIDTextPlaceholders(before)
		quoted = stripInboundCIDTextPlaceholders(after)
		hasQuoted = quoted != ""
		if hasQuoted {
			confidence = inboundhtml.ProjectionConfidenceHigh
		}
	}
	if visible == "" {
		visible = strippedReply
	}
	if visible == "" {
		visible = stripSupportEmailReplyDelimiter(payload.TextBody)
	}

	return inboundEmailProjection{
		VisibleText:          visible,
		QuotedText:           quoted,
		HTMLBody:             processed.HTML,
		HasQuotedContent:     hasQuoted,
		ProjectionConfidence: confidence,
		Version:              processed.ProjectionVersion,
	}
}

// inboundPayloadBodies is retained for callers/tests that only need the two
// legacy display variants.
func inboundPayloadBodies(payload model.PostmarkInboundPayload) (markdown, htmlBody string) {
	projection := inboundPayloadProjection(payload)
	return projection.VisibleText, projection.HTMLBody
}

var (
	inboundCIDImageSrcRe        = regexp.MustCompile(`(?i)(\bsrc\s*=\s*["'])cid:([^"']+)(["'])`)
	inboundCIDTextPlaceholderRe = regexp.MustCompile(`(?i)\[cid:[^\]\s]+]`)
)

func inboundPayloadProjectionWithHTML(payload model.PostmarkInboundPayload, htmlBody string) inboundEmailProjection {
	payload.HtmlBody = htmlBody
	return inboundPayloadProjection(payload)
}

func applyForwardedEmailProjection(projection *inboundEmailProjection, visibleText string) {
	if projection == nil {
		return
	}
	projection.VisibleText = cleanForwardedEmailProjectionText(visibleText)
	projection.QuotedText = ""
	projection.HasQuotedContent = false
	projection.ProjectionConfidence = inboundhtml.ProjectionConfidenceNone
	projection.Version = inboundhtml.CurrentProjectionVersion
}

func cleanForwardedEmailProjectionText(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	markerIndex, _ := firstForwardedEmailMarker(lines)
	if markerIndex < 0 {
		return strings.TrimSpace(content)
	}

	note := strings.TrimSpace(strings.Join(lines[:markerIndex], "\n"))
	bodyStart := -1
	firstNonMetadata := -1
	sawMetadata := false
	headerEnded := false
	for i := markerIndex + 1; i < len(lines); i++ {
		if sawMetadata && strings.TrimSpace(lines[i]) == "" {
			headerEnded = true
			continue
		}
		if headerEnded {
			bodyStart = i
			break
		}
		if isForwardedEmailMetadataLine(lines[i]) {
			sawMetadata = true
			continue
		}
		if sawMetadata && firstNonMetadata < 0 {
			firstNonMetadata = i
		}
	}
	if bodyStart < 0 {
		bodyStart = firstNonMetadata
	}
	if bodyStart < 0 {
		if note != "" {
			return note
		}
		return strings.TrimSpace(content)
	}

	bodyEnd := len(lines)
	for i := bodyStart + 1; i < len(lines); i++ {
		if nextMarker, _ := firstForwardedEmailMarker([]string{lines[i]}); nextMarker == 0 {
			bodyEnd = i
			break
		}
	}
	body := strings.TrimSpace(strings.Join(lines[bodyStart:bodyEnd], "\n"))
	return strings.TrimSpace(strings.Join(nonEmptyForwardedEmailParts(note, body), "\n\n"))
}

func isForwardedEmailMetadataLine(line string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "> ")))
	for _, prefix := range []string{
		"from:", "date:", "sent:", "subject:", "to:", "cc:", "bcc:",
		"de:", "fecha:", "enviado:", "para:", "asunto:",
		"von:", "datum:", "gesendet:", "an:", "betreff:",
		"expéditeur:", "envoyé:", "à:", "objet:",
	} {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func nonEmptyForwardedEmailParts(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}

func splitSupportEmailReplyDelimiter(content string) (before, after string, ok bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	index := strings.Index(normalized, supportEmailReplyDelimiter)
	if index < 0 {
		return "", "", false
	}
	return strings.TrimSpace(normalized[:index]), strings.TrimSpace(normalized[index+len(supportEmailReplyDelimiter):]), true
}

func stripInboundCIDTextPlaceholders(content string) string {
	content = inboundCIDTextPlaceholderRe.ReplaceAllString(content, "")
	return strings.TrimSpace(content)
}

func rewriteInboundCIDImageSources(htmlBody string, cidURLs map[string]string) string {
	if strings.TrimSpace(htmlBody) == "" || len(cidURLs) == 0 {
		return htmlBody
	}
	return inboundCIDImageSrcRe.ReplaceAllStringFunc(htmlBody, func(match string) string {
		parts := inboundCIDImageSrcRe.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		if replacement := cidURLs[normalizeInboundContentID(parts[2])]; replacement != "" {
			return parts[1] + replacement + parts[3]
		}
		return match
	})
}

func normalizeInboundContentID(contentID string) string {
	contentID = strings.TrimSpace(contentID)
	contentID = strings.TrimPrefix(strings.TrimSuffix(contentID, ">"), "<")
	contentID = strings.TrimPrefix(strings.ToLower(contentID), "cid:")
	return contentID
}

func stripSupportEmailReplyDelimiter(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if index := strings.Index(normalized, supportEmailReplyDelimiter); index >= 0 {
		normalized = normalized[:index]
	}
	return stripInboundCIDTextPlaceholders(normalized)
}

func inboundForwardedEmailScanText(payload model.PostmarkInboundPayload, fallback string) string {
	var parts []string
	for _, part := range []string{
		payload.TextBody,
		payload.StrippedTextReply,
		fallback,
	} {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		duplicate := false
		for _, existing := range parts {
			if existing == part {
				duplicate = true
				break
			}
		}
		if !duplicate {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n\n")
}

func inboundForwardedEmailDisplaySource(payload model.PostmarkInboundPayload, fallback string, attribution forwardedEmailDetectionResult) string {
	if !attribution.Applied {
		return fallback
	}
	for _, candidate := range []string{
		payload.TextBody,
		payload.StrippedTextReply,
		fallback,
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(candidate), strings.ToLower(strings.TrimSpace(attribution.OriginalEmail))) {
			continue
		}
		if markerIndex, _ := firstForwardedEmailMarker(strings.Split(strings.ReplaceAll(candidate, "\r\n", "\n"), "\n")); markerIndex >= 0 {
			return candidate
		}
	}
	return fallback
}

const postmarkInboundAutoSpamScoreThreshold = 5.0

type postmarkInboundSpamSignals struct {
	score  *float64
	status string
	tests  string
}

func postmarkInboundSpamSignalsFromHeaders(headers []model.PostmarkHeader) postmarkInboundSpamSignals {
	signals := postmarkInboundSpamSignals{
		status: strings.TrimSpace(inboundHeaderValue(headers, "X-Spam-Status")),
		tests:  strings.TrimSpace(inboundHeaderValue(headers, "X-Spam-Tests")),
	}
	if scoreValue := strings.TrimSpace(inboundHeaderValue(headers, "X-Spam-Score")); scoreValue != "" {
		fields := strings.Fields(scoreValue)
		if len(fields) > 0 {
			if score, err := strconv.ParseFloat(fields[0], 64); err == nil {
				signals.score = &score
			}
		}
	}
	return signals
}

func (signals postmarkInboundSpamSignals) shouldAutoSpamNewConversation() bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(signals.status)), "yes") {
		return true
	}
	return signals.score != nil && *signals.score >= postmarkInboundAutoSpamScoreThreshold
}

func (signals postmarkInboundSpamSignals) messageMetadata() string {
	metadata := map[string]any{}
	if signals.score != nil {
		metadata["postmark_spam_score"] = *signals.score
	}
	if signals.status != "" {
		metadata["postmark_spam_status"] = signals.status
	}
	if signals.tests != "" {
		metadata["postmark_spam_tests"] = signals.tests
	}
	if len(metadata) == 0 {
		return "{}"
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(data)
}

const (
	emailFallbackOutboxKey     = "email_fallback_outbox"
	emailFallbackLockKey       = "email_fallback_lock"
	emailFallbackReconcileKey  = "email_fallback_reconcile_lock"
	emailFallbackMsgsKeyPrefix = "email_fallback_msgs:"
	emailFallbackOnlineRetry   = 30 * time.Second
	emailFallbackReconcileTick = time.Minute
)

const (
	supportEmailAttachmentMaxFileBytes     = 5 * 1024 * 1024
	supportEmailAttachmentMaxTotalRawBytes = 7 * 1024 * 1024
	supportEmailReplyDelimiter             = "-- Please type your reply above this line --"
	supportEmailPreviewMaxRunes            = 160
)

func helpinAttributionSlug(value, fallback string) string {
	var slug strings.Builder
	lastWasSeparator := false
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			slug.WriteRune(char)
			lastWasSeparator = false
			continue
		}
		if slug.Len() > 0 && !lastWasSeparator {
			slug.WriteByte('-')
			lastWasSeparator = true
		}
	}
	cleaned := strings.Trim(slug.String(), "-")
	if cleaned == "" {
		return fallback
	}
	return cleaned
}

func buildHelpinAttributionSource(workspaceName, workspaceID string) string {
	workspaceSlug := helpinAttributionSlug(workspaceName, "workspace")
	idPrefix := strings.TrimSpace(workspaceID)
	if len(idPrefix) > 8 {
		idPrefix = idPrefix[:8]
	}
	idPrefix = helpinAttributionSlug(idPrefix, "")
	if idPrefix == "" {
		return workspaceSlug
	}
	return workspaceSlug + "-" + idPrefix
}

func buildHelpinEmailAttributionURL(workspaceName, workspaceID string) string {
	params := []string{
		"utm_source=" + url.QueryEscape(buildHelpinAttributionSource(workspaceName, workspaceID)),
		"utm_medium=email",
		"utm_campaign=powered_by_helpin",
		"utm_content=support_email_footer",
	}
	return "https://helpin.ai/?" + strings.Join(params, "&")
}

type supportEmailAttachmentDownloader interface {
	DownloadContent(ctx context.Context, attachment model.SupportAttachmentPayload) ([]byte, error)
}

type supportEmailInboundAttachmentStore interface {
	StoreInboundEmailAttachment(ctx context.Context, req supportInboundEmailAttachmentRequest) (*model.SupportAttachmentPayload, error)
}

type supportInboundEmailAttachmentRequest struct {
	AttachmentID   string
	WorkspaceID    string
	ConversationID string
	MessageID      string
	FileName       string
	ContentType    string
	Base64Content  string
	ContentID      string
	ContentLength  int64
}

// EmailFallbackService manages delayed outbound email delivery and inbound replies.
type EmailFallbackService struct {
	redis                  *redis.Client
	hub                    *websocket.Hub
	wsPublisher            *websocket.Publisher
	emailClient            *email.Client
	messageRepo            *repository.SupportMessageRepository
	convRepo               *repository.SupportConversationRepository
	emailLogRepo           *repository.SupportEmailLogRepository
	webhookRepo            *repository.SupportEmailWebhookEventRepository
	installRepo            *repository.SupportInboxInstallationRepository
	sessionRepo            *repository.SupportInboxSessionRepository
	workspaceRepo          *repository.WorkspaceRepository
	contactRepo            *repository.CRMContactRepository
	supportInboxService    *SupportInboxService
	attachmentService      *SupportAttachmentService
	attachmentDownloader   supportEmailAttachmentDownloader
	inboundAttachmentStore supportEmailInboundAttachmentStore
	notificationService    *NotificationService
	pushSenderService      *PushSenderService
	replyDomain            string
	appBaseURL             string
	logger                 *slog.Logger
	podID                  string
	pollInterval           time.Duration
	leaseTTL               time.Duration
	processingTTL          time.Duration
	now                    func() time.Time
}

type EmailFallbackBackfillOptions struct {
	From   time.Time
	To     time.Time
	Limit  int
	DryRun bool
}

type EmailFallbackBackfillResult struct {
	DryRun                 bool
	From                   time.Time
	To                     time.Time
	CandidateMessages      int
	CandidateConversations int
	SentConversations      int
	SentMessages           int
	SkippedMessages        int
}

// SetNotificationService injects the notification service used for support reply alerts.
func (s *EmailFallbackService) SetNotificationService(notificationService *NotificationService) *EmailFallbackService {
	if s == nil {
		return nil
	}
	s.notificationService = notificationService
	return s
}

// SetPushSenderService injects the push sender used to fan out mobile push
// notifications for customer replies routed through the email fallback
// path. Safe to leave unset (nil) — ProcessSupportCustomerReplyNotification
// tolerates a nil *PushSenderService.
func (s *EmailFallbackService) SetPushSenderService(ps *PushSenderService) *EmailFallbackService {
	if s == nil {
		return nil
	}
	s.pushSenderService = ps
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

// SetAttachmentService injects support attachment hydration for outbound fallback emails.
func (s *EmailFallbackService) SetAttachmentService(attachmentService *SupportAttachmentService) *EmailFallbackService {
	if s == nil {
		return nil
	}
	s.attachmentService = attachmentService
	s.attachmentDownloader = attachmentService
	s.inboundAttachmentStore = attachmentService
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
		return deployment.DefaultReplyDomain
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
	if s == nil {
		return
	}
	if s.redis == nil {
		s.logger.WarnContext(ctx, "email fallback poller not started — redis is not configured")
		return
	}

	s.logger.InfoContext(ctx, "email fallback poller started",
		"poll_interval", s.pollInterval.String(),
		"pod_id", s.podID,
	)
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.InfoContext(ctx, "email fallback poller stopped", "error", ctx.Err())
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

// StartReconciler periodically repairs missed outbound email fallback sends.
// It scans Postgres for recent unread outbound replies that were not marked as
// emailed and sends each eligible message through the normal fallback path.
func (s *EmailFallbackService) StartReconciler(ctx context.Context) {
	if s == nil {
		return
	}
	if s.redis == nil || s.emailClient == nil {
		s.logger.WarnContext(ctx, "email fallback reconciler not started — dependency not configured",
			"redis_configured", s.redis != nil,
			"postmark_reply_configured", s.emailClient != nil,
		)
		return
	}

	s.logger.InfoContext(ctx, "email fallback reconciler started",
		"interval", emailFallbackReconcileTick.String(),
		"pod_id", s.podID,
	)
	ticker := time.NewTicker(emailFallbackReconcileTick)
	defer ticker.Stop()

	run := func() {
		leaseHeld, err := s.acquireOrRenewNamedLease(ctx, emailFallbackReconcileKey)
		if err != nil {
			s.logger.ErrorContext(ctx, "email fallback reconcile lease failed", "error", err)
			return
		}
		if !leaseHeld {
			return
		}
		sent, err := s.ReconcileMissedOutboundEmails(ctx, 25)
		if err != nil {
			s.logger.ErrorContext(ctx, "email fallback reconcile failed", "error", err)
			return
		}
		if sent > 0 {
			s.logger.InfoContext(ctx, "email fallback reconcile completed", "sent_count", sent)
		}
	}

	run()
	for {
		select {
		case <-ctx.Done():
			s.logger.InfoContext(ctx, "email fallback reconciler stopped", "error", ctx.Err())
			return
		case <-ticker.C:
			run()
		}
	}
}

// OnAgentReply enqueues an outbound email candidate when the feature is enabled.
func (s *EmailFallbackService) OnAgentReply(ctx context.Context, workspaceID string, msg *model.SupportMessage, conv *model.SupportConversation) error {
	if s == nil {
		return nil
	}
	if msg == nil || conv == nil {
		s.logger.WarnContext(ctx, "email fallback enqueue skipped — missing message or conversation",
			"workspace_id", workspaceID,
			"has_message", msg != nil,
			"has_conversation", conv != nil,
		)
		return nil
	}
	if msg.DeliveryMode() == model.SupportDeliveryChatOnly {
		return nil
	}
	if msg.ExplicitEmailDelivery() {
		delaySecs, err := s.validateExplicitEmail(ctx, workspaceID, conv)
		if err != nil {
			return err
		}
		return s.queueExplicitEmail(ctx, msg, conv, s.messageRepo, delaySecs)
	}
	if s.redis == nil || s.emailClient == nil {
		s.logger.WarnContext(ctx, "email fallback enqueue skipped — dependency not configured",
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
			"redis_configured", s.redis != nil,
			"postmark_reply_configured", s.emailClient != nil,
		)
		return nil
	}
	if conv.CustomerEmail == nil || strings.TrimSpace(*conv.CustomerEmail) == "" {
		s.logger.InfoContext(ctx, "email fallback enqueue skipped — conversation has no customer email",
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
		)
		return nil
	}
	if strings.TrimSpace(conv.PrimaryRecipientState) == model.SupportPrimaryRecipientStateUnconfirmed {
		s.logger.InfoContext(ctx, "email fallback enqueue skipped — primary recipient unconfirmed",
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
		)
		return nil
	}
	if isEmailFallbackTerminalStatus(conv.Status) {
		s.logger.InfoContext(ctx, "email fallback enqueue skipped — conversation is terminal",
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
			"status", conv.Status,
		)
		return nil
	}
	if conv.EmailUnsubscribed {
		s.logger.InfoContext(ctx, "email fallback enqueue skipped — conversation is unsubscribed",
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
		)
		return nil
	}

	settings, err := s.loadSettings(ctx, workspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "email fallback enqueue skipped — settings load failed",
			"error", err,
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
		)
		return err
	}
	if !settings.EmailFallbackEnabled {
		s.logger.InfoContext(ctx, "email fallback enqueue skipped — workspace setting disabled",
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
		)
		return nil
	}
	if online, err := s.isVisitorOnline(ctx, workspaceID, conv.AnonymousID); err == nil && online {
		s.logger.InfoContext(ctx, "email fallback enqueue skipped — visitor online",
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
		)
		return nil
	} else if err != nil {
		s.logger.WarnContext(ctx, "email fallback enqueue visitor presence lookup failed",
			"error", err,
			"workspace_id", workspaceID,
			"conversation_id", conv.ID,
			"message_id", msg.ID,
		)
	}

	delaySecs := normalizedEmailFallbackDelaySecs(settings.EmailFallbackDelaySecs)
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
	if err := s.messageRepo.SetCancellableUntil(ctx, msg.ID, fireAt); err != nil {
		s.logger.WarnContext(ctx, "set support message cancellable_until failed", "error", err, "message_id", msg.ID)
	} else {
		s.publishMessageUpdated(workspaceID, conv.ID, msg.ID, "email_fallback:queued")
	}
	s.logger.InfoContext(ctx, "email fallback enqueued",
		"workspace_id", workspaceID,
		"conversation_id", conv.ID,
		"message_id", msg.ID,
		"to_email", strings.TrimSpace(*conv.CustomerEmail),
		"fire_at", fireAt,
		"delay_secs", delaySecs,
	)
	return nil
}

// CancelForMessage removes a single queued message from its conversation email
// fallback batch. If an outbound email log already references this message, the
// email has fired and the caller can only hide the in-app message.
func (s *EmailFallbackService) CancelForMessage(ctx context.Context, workspaceID, conversationID, messageID string) (bool, error) {
	if s == nil || s.redis == nil {
		return false, nil
	}
	if s.emailLogRepo != nil {
		logRow, err := s.emailLogRepo.GetByMessageID(ctx, workspaceID, messageID)
		if err != nil {
			return false, err
		}
		if logRow != nil && strings.TrimSpace(logRow.Direction) == "outbound" {
			return true, nil
		}
	}

	msgKey := s.msgListKey(conversationID)
	if err := s.redis.LRem(ctx, msgKey, 0, messageID).Err(); err != nil {
		return false, fmt.Errorf("remove queued email fallback message: %w", err)
	}
	remaining, err := s.redis.LLen(ctx, msgKey).Result()
	if err != nil {
		return false, fmt.Errorf("count queued email fallback messages: %w", err)
	}
	if remaining > 0 {
		return false, nil
	}
	if err := s.redis.ZRem(ctx, emailFallbackOutboxKey, conversationID).Err(); err != nil {
		return false, fmt.Errorf("remove email fallback outbox entry: %w", err)
	}
	if err := s.redis.Del(ctx, msgKey).Err(); err != nil {
		return false, fmt.Errorf("delete email fallback message list: %w", err)
	}
	return false, nil
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

	if existing, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, strings.TrimSpace(payload.MessageID)); err != nil {
		return err
	} else if existing != nil {
		conv, err := s.findConversationByID(ctx, existing.ConversationID)
		if err != nil {
			return err
		}
		if conv == nil || conv.AnonymizedAt != nil {
			return nil
		}
		// Link duplicate receipts too; never retain an unowned copy of identity.
		s.recordWebhookEvent(ctx, "inbound", strings.TrimSpace(payload.MessageID), strings.TrimSpace(payload.MessageStream), rawPayload, conv, existing, parseInboundWebhookReceivedAt(payload))
		s.logger.InfoContext(ctx, "postmark inbound duplicate ignored",
			"message_id", strings.TrimSpace(payload.MessageID),
			"conversation_id", strings.TrimSpace(existing.ConversationID),
			"email_log_id", existing.ID,
		)
		return s.retryInboundCustomerAIRequest(ctx, existing)
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
	if strings.HasPrefix(mailboxHash, "verify-") {
		return s.processInboundSenderForwardingVerification(ctx, mailboxHash, payload)
	}

	if mailboxHash == "" {
		for _, recipientAddress := range inboundRecipientAddresses(payload) {
			if route, err := s.findInboundRouteByRecipient(ctx, recipientAddress); err != nil {
				return err
			} else if route != nil {
				return s.processInboundRoute(ctx, route, payload, rawPayload)
			}
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
		return fmt.Errorf("inbound conversation not found")
	}
	return s.processInboundConversationReply(ctx, conv, nil, payload, rawPayload)
}

func (s *EmailFallbackService) processInboundConversationReply(ctx context.Context, conv *model.SupportConversation, route *model.SupportEmailRoute, payload model.PostmarkInboundPayload, rawPayload string) error {
	if conv == nil || conv.AnonymizedAt != nil {
		return nil
	}
	if isEmailFallbackInboundTerminalStatus(conv.Status) {
		s.logger.InfoContext(ctx, "postmark inbound ignored for terminal conversation",
			"message_id", strings.TrimSpace(payload.MessageID),
			"conversation_id", conv.ID,
			"status", conv.Status,
		)
		return nil
	}

	fromEmail := inboundEmailAddress(payload)
	projection := inboundPayloadProjection(payload)
	content, htmlBody := projection.VisibleText, projection.HTMLBody
	if content == "" && len(payload.Attachments) == 0 {
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
	replyToRaw, replyToEmail, replyToName := inboundReplyToAddress(payload)

	settings, err := s.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return err
	}
	routeDomain := ""
	if s.supportInboxService != nil {
		routeDomain = s.supportInboxService.routeDomain
	}
	forwardedAttribution := forwardedEmailDetectionResult{}
	if settings.ForwardedEmailDetectionEnabled && strings.TrimSpace(settings.ForwardedEmailDetectionMode) == "high_confidence_any_sender" {
		recipientEmails := []string{
			payload.To,
			payload.OriginalRecipient,
			inboundRecipientAddress(payload),
		}
		if route != nil {
			recipientEmails = append(recipientEmails, route.InboundAddress)
		}
		forwardedAttribution = detectForwardedEmailAttribution(forwardedEmailDetectionInput{
			ForwarderEmail:  fromEmail,
			ForwarderName:   senderName,
			RecipientEmails: recipientEmails,
			ReplyDomain:     s.InboundDomain(),
			RouteDomain:     routeDomain,
			Text:            inboundForwardedEmailScanText(payload, content),
			MinConfidence:   settings.ForwardedEmailMinConfidence,
		})
	}
	customerEmail := ""
	if conv.CustomerEmail != nil {
		customerEmail = strings.TrimSpace(*conv.CustomerEmail)
	}
	isTeammateReply := false
	unknownSender := false
	knownParticipant := false
	for _, address := range append(append([]string{}, conv.EmailCC...), conv.EmailThreadParticipants...) {
		if strings.EqualFold(strings.TrimSpace(address), fromEmail) {
			knownParticipant = true
			break
		}
	}
	var teammate *model.WorkspaceMember
	if !strings.EqualFold(customerEmail, fromEmail) {
		replyToMatchesCustomer := replyToEmail != "" && strings.EqualFold(customerEmail, replyToEmail)
		forwardedMatchesCustomer := forwardedAttribution.Applied && strings.EqualFold(customerEmail, strings.TrimSpace(forwardedAttribution.OriginalEmail))
		if !replyToMatchesCustomer && !forwardedMatchesCustomer && !knownParticipant {
			if s.workspaceRepo != nil && inboundPayloadIncludesRecipient(payload, customerEmail) {
				teammate, err = s.workspaceRepo.GetActiveMembershipByEmail(ctx, conv.WorkspaceID, fromEmail)
				if err != nil {
					return err
				}
			}
			if teammate == nil || teammate.UserID == nil {
				unknownSender = true
			} else {
				isTeammateReply = true
				senderName = strings.TrimSpace(teammate.DisplayName)
				if senderName == "" {
					senderName = fromEmail
				}
			}
		}
		if !isTeammateReply && forwardedMatchesCustomer && strings.TrimSpace(forwardedAttribution.OriginalName) != "" {
			senderName = strings.TrimSpace(forwardedAttribution.OriginalName)
		} else if !isTeammateReply && replyToMatchesCustomer && strings.TrimSpace(replyToName) != "" {
			senderName = strings.TrimSpace(replyToName)
		} else if !isTeammateReply && !knownParticipant && !unknownSender && conv.CustomerName != nil && strings.TrimSpace(*conv.CustomerName) != "" {
			senderName = strings.TrimSpace(*conv.CustomerName)
		}
	}
	if forwardedAttribution.Applied {
		content = inboundForwardedEmailDisplaySource(payload, content, forwardedAttribution)
		if len(content) > 50_000 {
			content = content[:50_000]
		}
		applyForwardedEmailProjection(&projection, content)
	}

	rfcMessageID := inboundRFCMessageID(payload)
	inReplyTo := normalizeRFCHeaderValue(inboundHeaderValue(payload.Headers, "In-Reply-To"))
	referencesHeader := strings.TrimSpace(inboundHeaderValue(payload.Headers, "References"))
	recipientAddress := inboundRecipientAddress(payload)
	ccEmails := inboundCCEmails(payload)
	bccEmails := inboundBCCEmails(payload)

	viaEmail := "email"
	spamSignals := postmarkInboundSpamSignalsFromHeaders(payload.Headers)
	messageMetadata := inboundEmailAIMetadata(spamSignals.messageMetadata(), payload, content)
	if forwardedAttribution.Applied {
		messageMetadata = mergeForwardedAttributionMetadata(messageMetadata, forwardedAttribution)
	}
	if knownParticipant || unknownSender {
		senderName = strings.TrimSpace(payload.FromFull.Name)
		if senderName == "" {
			senderName = fromEmail
		}
	}
	var inboundMeta map[string]any
	_ = json.Unmarshal([]byte(messageMetadata), &inboundMeta)
	if inboundMeta == nil {
		inboundMeta = map[string]any{}
	}
	inboundMeta["email_sender"] = fromEmail
	inboundMeta["email_participant_sender"] = knownParticipant || unknownSender
	if unknownSender {
		inboundMeta["email_unknown_sender"] = true
		inboundMeta["email_ai_request"] = false
	}
	encodedMeta, _ := json.Marshal(inboundMeta)
	messageMetadata = string(encodedMeta)
	isNotice := inboundEmailHasAbsenceNotice(messageMetadata)
	if isTeammateReply && !isNotice {
		messageMetadata = mergeExternalEmailReplyMetadata(messageMetadata)
	}
	senderType := "customer"
	var senderUserID *string
	emailDirection := "inbound"
	emailStatus := "sent"
	if isTeammateReply {
		senderType = "user"
		senderUserID = teammate.UserID
		if !isNotice {
			emailDirection = "outbound"
			emailStatus = "external"
		}
	}
	msg := &model.SupportMessage{
		ID:                inboundStableID("message:" + payload.MessageID),
		WorkspaceID:       conv.WorkspaceID,
		ConversationID:    conv.ID,
		SenderType:        senderType,
		SenderUserID:      senderUserID,
		SenderDisplayName: &senderName,
		Content:           content,
		IsInternal:        unknownSender,
		MessageType:       "reply",
		Metadata:          messageMetadata,
		ViaChannel:        &viaEmail,
	}
	if isTeammateReply {
		if sentAt := parseInboundWebhookReceivedAt(payload); sentAt != nil {
			msg.CreatedAt = *sentAt
			msg.UpdatedAt = *sentAt
		}
	}

	if isNotice {
		msg.MessageType = model.SupportMessageTypeEmailNotice
	}

	normalizedStatus := model.NormalizeSupportConversationStatus(conv.Status)
	wasResolved := normalizedStatus == model.SupportConversationStatusResolved
	shouldReopenCustomerReply := !isTeammateReply && !isNotice && (wasResolved || normalizedStatus == model.SupportConversationStatusWaitingOnCustomer)
	shouldMoveTeammateReplyToWaiting := isTeammateReply && !isNotice && wasResolved

	var createdMsg *model.SupportMessage
	txErr := s.convRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgRepoTx := s.messageRepo.WithTx(tx)
		emailLogRepoTx := s.emailLogRepo.WithTx(tx)
		convRepoTx := s.convRepo.WithTx(tx)

		if err := msgRepoTx.Create(ctx, msg); err != nil {
			return err
		}
		// Learn visible participants only from an accepted sender, without changing
		// the primary customer or the team's selected outbound CC recipients.
		if !unknownSender && !isNotice {
			var current model.SupportConversation
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", conv.ID, conv.WorkspaceID).First(&current).Error; err != nil {
				return err
			}
			routeAddress := recipientAddress
			if route != nil {
				routeAddress = route.InboundAddress
			}
			participants := normalizedEmailAddressList(append([]string(current.EmailThreadParticipants), inboundVisibleThreadParticipants(payload, routeAddress, customerEmail)...))
			if err := convRepoTx.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"email_thread_participants": model.DocsStringArray(participants)}); err != nil {
				return err
			}
		}
		if err := s.enqueueInboundAttachments(ctx, tx, msg, payload); err != nil {
			return err
		}
		msg.EmailFrom = fromEmail
		msg.EmailTo = strings.TrimSpace(payload.To)
		msg.EmailCC = model.DocsStringArray(ccEmails)
		msg.HTMLBody = htmlBody
		msg.StrippedText = content
		msg.EmailVisibleText = projection.VisibleText
		msg.EmailQuotedText = projection.QuotedText
		msg.EmailHasQuotedContent = boolPtr(projection.HasQuotedContent)
		msg.EmailProjectionConfidence = projection.ProjectionConfidence
		msg.EmailProjectionVersion = projection.Version

		logRow := &model.SupportEmailLog{
			WorkspaceID:               conv.WorkspaceID,
			ConversationID:            conv.ID,
			Direction:                 emailDirection,
			MessageIDs:                model.DocsStringArray{msg.ID},
			FromEmail:                 fromEmail,
			ToEmail:                   strings.TrimSpace(payload.To),
			ReplyTo:                   replyToRaw,
			RecipientAddress:          recipientAddress,
			CCEmails:                  model.DocsStringArray(ccEmails),
			BCCEmails:                 model.DocsStringArray(bccEmails),
			Subject:                   strings.TrimSpace(payload.Subject),
			RFCMessageID:              rfcMessageID,
			InReplyTo:                 inReplyTo,
			ReferencesHeader:          referencesHeader,
			PostmarkMessageID:         strPtr(strings.TrimSpace(payload.MessageID)),
			RawBody:                   rawPayload,
			StrippedText:              content,
			HTMLBody:                  htmlBody,
			EmailVisibleText:          projection.VisibleText,
			EmailQuotedText:           projection.QuotedText,
			EmailHasQuotedContent:     projection.HasQuotedContent,
			EmailProjectionConfidence: projection.ProjectionConfidence,
			EmailProjectionVersion:    projection.Version,
			Status:                    emailStatus,
		}
		if route != nil {
			logRow.EmailRouteID = &route.ID
		}
		if logRow.PostmarkMessageID != nil && *logRow.PostmarkMessageID == "" {
			logRow.PostmarkMessageID = nil
		}
		if err := emailLogRepoTx.Create(ctx, logRow); err != nil {
			return err
		}

		if unknownSender {
			if err := convRepoTx.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{
				"human_takeover": true, "assigned_agent_id": nil, "ai_state": "escalated", "flow_state": supportEmailReopenFlowState(conv),
			}); err != nil {
				return err
			}
			conv.HumanTakeover = boolPtr(true)
			conv.AssignedAgentID = nil
			conv.AIState = strPtr("escalated")
		}
		// Email replies belong to the human inbox. Cancel AI ownership silently:
		// sending a handoff email here can itself feed an autoresponder loop.
		if !isTeammateReply && !isNotice && (!model.SupportAIReplyAllowed(settings, conv, msg) || !shouldAutomaticallyProcessSupportAI(settings)) && (derefString(conv.AssignedAgentID) != "" || derefString(conv.AIState) == "pending" || derefString(conv.AIActiveRunID) != "") {
			flow := supportEmailReopenFlowState(conv)
			if err := convRepoTx.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{
				"assigned_agent_id": nil, "human_takeover": true, "ai_state": "escalated", "flow_state": flow,
			}); err != nil {
				return err
			}
			conv.AssignedAgentID = nil
			conv.HumanTakeover = boolPtr(true)
			conv.AIState = strPtr("escalated")
			conv.FlowState = &flow
		}

		if shouldReopenCustomerReply {
			reopenFlowState := supportEmailReopenFlowState(conv)
			if model.SupportAIReplyAllowed(settings, conv, msg) && shouldAutomaticallyProcessSupportAI(settings) && !supportConversationHumanOwned(conv) && conv.CustomerRequestedHumanAt == nil && derefString(conv.AIState) != "escalated" {
				reopenFlowState = model.SupportConversationFlowStateAIHandling
			}
			if err := convRepoTx.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{
				"status":             model.SupportConversationStatusOpen,
				"flow_state":         reopenFlowState,
				"resolved_at":        nil,
				"closed_at":          nil,
				"ai_resolved_at":     nil,
				"ai_resolution_type": nil,
				"updated_at":         s.now(),
			}); err != nil {
				return err
			}
			conv.Status = model.SupportConversationStatusOpen
			flowState := reopenFlowState
			conv.FlowState = &flowState
			conv.ResolvedAt = nil
			conv.ClosedAt = nil
			conv.AIResolvedAt = nil
			conv.AIResolutionType = nil
			if wasResolved {
				if err := createEmailReopenedSystemMessage(ctx, msgRepoTx, conv); err != nil {
					return err
				}
			}
		}
		if isTeammateReply && !isNotice {
			updates := map[string]any{
				"flow_state":        model.SupportConversationFlowStateAssignedToHuman,
				"opened_by_user_id": teammate.UserID,
				"human_takeover":    true,
				"updated_at":        s.now(),
			}
			if shouldMoveTeammateReplyToWaiting {
				updates["status"] = model.SupportConversationStatusWaitingOnCustomer
				updates["resolved_at"] = nil
				updates["closed_at"] = nil
			}
			if err := convRepoTx.UpdateFields(ctx, conv.WorkspaceID, conv.ID, updates); err != nil {
				return err
			}
			flowState := model.SupportConversationFlowStateAssignedToHuman
			conv.FlowState = &flowState
			conv.OpenedByUserID = teammate.UserID
			conv.HumanTakeover = boolPtr(true)
			if shouldMoveTeammateReplyToWaiting {
				conv.Status = model.SupportConversationStatusWaitingOnCustomer
				conv.ResolvedAt = nil
				conv.ClosedAt = nil
			}
		}

		createdMsg = msg
		return nil
	})
	if txErr != nil {
		if isLikelyUniqueConstraintError(txErr) {
			existing, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, strings.TrimSpace(payload.MessageID))
			if err != nil {
				return err
			}
			// A constraint error alone does not prove this email was saved.
			if existing != nil {
				return s.retryInboundCustomerAIRequest(ctx, existing)
			}
		}
		return txErr
	}

	if !isTeammateReply && !isNotice {
		ProcessSupportCustomerReplyNotification(ctx, s.notificationService, s.pushSenderService, conv, content, senderName)
	}
	if !isTeammateReply && !isNotice && !unknownSender && s.supportInboxService != nil {
		s.supportInboxService.recordSupportEvent(SupportEventInput{
			WorkspaceID: conv.WorkspaceID, EventType: model.SupportEventCustomerMessageCreated,
			ConversationID: &conv.ID, MessageID: &createdMsg.ID,
			ActorType: model.SupportEventActorCustomer, Channel: "email",
		})
	}

	s.logger.InfoContext(ctx, "postmark inbound created support message",
		"message_id", strings.TrimSpace(payload.MessageID),
		"conversation_id", conv.ID,
		"support_message_id", createdMsg.ID,
	)
	if route != nil && s.supportInboxService != nil && s.supportInboxService.emailRouteRepo != nil {
		_ = s.supportInboxService.emailRouteRepo.TouchInbound(ctx, route.ID, s.now())
	}
	actorID := "email:" + createdMsg.ID
	if isTeammateReply && teammate.UserID != nil {
		actorID = *teammate.UserID
	}
	s.wsPublisher.Publish(websocket.SupportMessageEvent(conv.WorkspaceID, createdMsg, actorID))
	if shouldReopenCustomerReply || unknownSender || (isTeammateReply && !isNotice) {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "support_conversation",
			EntityID:    conv.ID,
			WorkspaceID: conv.WorkspaceID,
		})
	}
	if !isTeammateReply && !unknownSender {
		return s.publishInboundCustomerAIRequest(ctx, conv.WorkspaceID, conv.ID, createdMsg)
	}
	return nil
}

// ErrInboundEmailAIDispatchRetry tells the webhook handler that a saved customer
// message still needs AI dispatch and the provider should retry delivery.
var ErrInboundEmailAIDispatchRetry = errors.New("inbound email AI dispatch needs retry")

// Inbound email has already been persisted and deduplicated before automation.
// Keep the existing mailbox; this is a reply, not a new routing decision.
func (s *EmailFallbackService) retryInboundCustomerAIRequest(ctx context.Context, logRow *model.SupportEmailLog) error {
	if logRow == nil || logRow.Direction != "inbound" {
		return nil
	}
	for _, id := range logRow.MessageIDs {
		msg, err := s.messageRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("%w: load saved inbound customer message: %w", ErrInboundEmailAIDispatchRetry, err)
		}
		if msg == nil || msg.WorkspaceID != logRow.WorkspaceID || msg.ConversationID != logRow.ConversationID {
			continue
		}
		if err := s.publishInboundCustomerAIRequest(ctx, logRow.WorkspaceID, logRow.ConversationID, msg); err != nil {
			return err
		}
	}
	return nil
}

func (s *EmailFallbackService) publishInboundCustomerAIRequest(ctx context.Context, workspaceID, conversationID string, msg *model.SupportMessage) error {
	var metadata struct {
		AIRequest bool `json:"email_ai_request"`
	}
	if !supportAIMessageEligible(msg) || json.Unmarshal([]byte(msg.Metadata), &metadata) != nil || !metadata.AIRequest {
		return nil
	}
	if s.supportInboxService == nil || s.supportInboxService.supportAIService == nil || s.installRepo == nil {
		return nil
	}
	inst, err := s.installRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("%w: load inbound email AI settings: %w", ErrInboundEmailAIDispatchRetry, err)
	}
	if inst == nil || !shouldAutomaticallyProcessSupportAI(parseSettings(inst.Settings)) || !model.SupportAIReplyAllowed(parseSettings(inst.Settings), nil, msg) {
		return nil
	}
	conv, err := s.convRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return fmt.Errorf("%w: load inbound email AI ownership: %w", ErrInboundEmailAIDispatchRetry, err)
	}
	if conv == nil || conv.AnonymizedAt != nil || supportConversationHumanOwned(conv) || conv.CustomerRequestedHumanAt != nil || derefString(conv.AIState) == "escalated" || derefString(conv.FlowState) == model.SupportConversationFlowStateAssignedToHuman || conv.PrimaryRecipientState == model.SupportPrimaryRecipientStateUnconfirmed || isEmailFallbackInboundTerminalStatus(conv.Status) {
		return nil
	}
	if err := s.supportInboxService.supportAIService.PublishAIRequest(ctx, workspaceID, conversationID, msg.ID, msg.Content); err != nil {
		return fmt.Errorf("%w: publish inbound email AI request: %w", ErrInboundEmailAIDispatchRetry, err)
	}
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
	// Claim atomically: two pollers that observed the same due entry cannot
	// both move its deadline and deliver the same conversation batch.
	const claimScript = `
local score = redis.call('ZSCORE', KEYS[1], ARGV[1])
if not score or tonumber(score) > tonumber(ARGV[2]) then return 0 end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[1])
return 1`
	claimed, err := s.redis.Eval(ctx, claimScript, []string{emailFallbackOutboxKey}, conversationID, s.now().Unix(), s.now().Add(s.processingTTL).Unix()).Int()
	if err != nil {
		return fmt.Errorf("claim fallback outbox entry: %w", err)
	}
	if claimed == 0 {
		return nil
	}

	msgIDs, err := s.redis.LRange(ctx, s.msgListKey(conversationID), 0, -1).Result()
	if err != nil {
		return fmt.Errorf("load fallback message ids: %w", err)
	}

	return s.fireEmail(ctx, conversationID, uniqueEmailFallbackStrings(msgIDs))
}

// ReconcileMissedOutboundEmails sends recent unread outbound replies that were
// not queued or marked as emailed. It intentionally skips conversations still
// present in Redis outbox so the normal delayed queue remains the single owner
// of active pending fallback sends.
func (s *EmailFallbackService) ReconcileMissedOutboundEmails(ctx context.Context, limit int) (int, error) {
	if s == nil || s.messageRepo == nil || s.emailLogRepo == nil || s.emailClient == nil {
		return 0, nil
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}

	now := s.now()
	candidates, err := s.messageRepo.ListEmailFallbackReconciliationCandidates(ctx, now.Add(-1800*time.Second), now.Add(-30*time.Second), limit)
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	s.logger.InfoContext(ctx, "email fallback reconcile candidates found", "candidate_count", len(candidates))
	sent := 0
	for _, msg := range candidates {
		if ctx.Err() != nil {
			return sent, ctx.Err()
		}

		if s.redis != nil {
			if _, err := s.redis.ZScore(ctx, emailFallbackOutboxKey, msg.ConversationID).Result(); err == nil {
				s.logger.InfoContext(ctx, "email fallback reconcile skipped — conversation still has active redis outbox",
					"workspace_id", msg.WorkspaceID,
					"conversation_id", msg.ConversationID,
					"message_id", msg.ID,
				)
				continue
			} else if err != nil && !errors.Is(err, redis.Nil) {
				s.logger.WarnContext(ctx, "email fallback reconcile redis outbox lookup failed",
					"error", err,
					"workspace_id", msg.WorkspaceID,
					"conversation_id", msg.ConversationID,
					"message_id", msg.ID,
				)
				continue
			}
		}

		existingLog, err := s.emailLogRepo.GetByMessageID(ctx, msg.WorkspaceID, msg.ID)
		if err != nil {
			s.logger.WarnContext(ctx, "email fallback reconcile email log lookup failed",
				"error", err,
				"workspace_id", msg.WorkspaceID,
				"conversation_id", msg.ConversationID,
				"message_id", msg.ID,
			)
			continue
		}
		if existingLog != nil {
			s.logger.InfoContext(ctx, "email fallback reconcile skipped — message already has email log",
				"workspace_id", msg.WorkspaceID,
				"conversation_id", msg.ConversationID,
				"message_id", msg.ID,
				"email_log_id", existingLog.ID,
			)
			continue
		}

		settings, err := s.loadSettings(ctx, msg.WorkspaceID)
		if err != nil {
			s.logger.WarnContext(ctx, "email fallback reconcile settings load failed",
				"error", err,
				"workspace_id", msg.WorkspaceID,
				"conversation_id", msg.ConversationID,
				"message_id", msg.ID,
			)
			continue
		}
		delaySecs := normalizedEmailFallbackDelaySecs(settings.EmailFallbackDelaySecs)
		maxAgeSecs := normalizedEmailFallbackMaxDeliveryAgeSecs(settings.EmailFallbackMaxDeliveryAgeSecs, delaySecs)
		fireAt := msg.CreatedAt.Add(time.Duration(delaySecs) * time.Second)
		if msg.CancellableUntil != nil {
			fireAt = *msg.CancellableUntil
		}
		if now.Before(fireAt) {
			continue
		}
		if !msg.ExplicitEmailDelivery() && now.Sub(msg.CreatedAt) > time.Duration(maxAgeSecs)*time.Second {
			s.logger.DebugContext(ctx, "email fallback reconcile skipped — message is stale",
				"workspace_id", msg.WorkspaceID,
				"conversation_id", msg.ConversationID,
				"message_id", msg.ID,
				"max_delivery_age_secs", maxAgeSecs,
			)
			continue
		}

		s.logger.InfoContext(ctx, "email fallback reconcile sending missed message",
			"workspace_id", msg.WorkspaceID,
			"conversation_id", msg.ConversationID,
			"message_id", msg.ID,
			"fire_at", fireAt,
			"delay_secs", delaySecs,
		)
		beforeSent := countMessagesWithEmailNotifiedAt(ctx, s.messageRepo, []string{msg.ID})
		if err := s.fireEmailWithoutRedisCleanup(ctx, msg.ConversationID, []string{msg.ID}); err != nil {
			s.logger.ErrorContext(ctx, "email fallback reconcile send failed",
				"error", err,
				"workspace_id", msg.WorkspaceID,
				"conversation_id", msg.ConversationID,
				"message_id", msg.ID,
			)
			continue
		}
		afterSent := countMessagesWithEmailNotifiedAt(ctx, s.messageRepo, []string{msg.ID})
		if afterSent > beforeSent {
			sent++
		}
	}

	return sent, nil
}

// BackfillMissedOutboundEmails sends missed support replies for a known outage
// window. Unlike the realtime reconciler, this intentionally bypasses the stale
// delivery window after the caller has supplied an explicit recovery range.
func (s *EmailFallbackService) BackfillMissedOutboundEmails(ctx context.Context, opts EmailFallbackBackfillOptions) (*EmailFallbackBackfillResult, error) {
	if s == nil || s.messageRepo == nil || s.emailLogRepo == nil || s.emailClient == nil {
		return nil, fmt.Errorf("email fallback service is not fully configured")
	}
	if opts.From.IsZero() || opts.To.IsZero() {
		return nil, fmt.Errorf("from and to are required")
	}
	if !opts.From.Before(opts.To) {
		return nil, fmt.Errorf("from must be before to")
	}
	if opts.Limit < 1 || opts.Limit > 1000 {
		opts.Limit = 250
	}

	candidates, err := s.messageRepo.ListEmailFallbackReconciliationCandidates(ctx, opts.From, opts.To, opts.Limit)
	if err != nil {
		return nil, err
	}

	result := &EmailFallbackBackfillResult{
		DryRun:            opts.DryRun,
		From:              opts.From,
		To:                opts.To,
		CandidateMessages: len(candidates),
	}
	if len(candidates) == 0 {
		return result, nil
	}

	grouped := make(map[string][]string)
	for _, msg := range candidates {
		existingLog, err := s.emailLogRepo.GetByMessageID(ctx, msg.WorkspaceID, msg.ID)
		if err != nil {
			s.logger.WarnContext(ctx, "email fallback backfill email log lookup failed",
				"error", err,
				"workspace_id", msg.WorkspaceID,
				"conversation_id", msg.ConversationID,
				"message_id", msg.ID,
			)
			result.SkippedMessages++
			continue
		}
		if existingLog != nil {
			result.SkippedMessages++
			continue
		}
		grouped[msg.ConversationID] = append(grouped[msg.ConversationID], msg.ID)
	}
	result.CandidateConversations = len(grouped)
	if opts.DryRun {
		for conversationID, messageIDs := range grouped {
			s.logger.InfoContext(ctx, "email fallback backfill dry run candidate",
				"conversation_id", conversationID,
				"message_count", len(messageIDs),
			)
		}
		return result, nil
	}

	for conversationID, messageIDs := range grouped {
		beforeSent := countMessagesWithEmailNotifiedAt(ctx, s.messageRepo, messageIDs)
		s.logger.InfoContext(ctx, "email fallback backfill sending conversation",
			"conversation_id", conversationID,
			"message_count", len(messageIDs),
		)
		if err := s.fireEmailForBackfill(ctx, conversationID, messageIDs); err != nil {
			s.logger.ErrorContext(ctx, "email fallback backfill send failed",
				"error", err,
				"conversation_id", conversationID,
				"message_count", len(messageIDs),
			)
			result.SkippedMessages += len(messageIDs)
			continue
		}
		afterSent := countMessagesWithEmailNotifiedAt(ctx, s.messageRepo, messageIDs)
		newlySent := afterSent - beforeSent
		if newlySent <= 0 {
			result.SkippedMessages += len(messageIDs)
			continue
		}
		result.SentConversations++
		result.SentMessages += newlySent
		result.SkippedMessages += len(messageIDs) - newlySent
	}

	return result, nil
}

// DiagnoseConversation explains the email fallback decision state for one
// support conversation. It is read-only and intended for platform-admin
// troubleshooting.
func (s *EmailFallbackService) DiagnoseConversation(ctx context.Context, conversationID string) (*model.EmailFallbackConversationDiagnosticsResponse, error) {
	if s == nil || s.convRepo == nil || s.messageRepo == nil {
		return nil, nil
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}

	conv, err := s.findConversationByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, nil
	}

	settings, err := s.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return nil, err
	}
	delaySecs := normalizedEmailFallbackDelaySecs(settings.EmailFallbackDelaySecs)
	maxAgeSecs := normalizedEmailFallbackMaxDeliveryAgeSecs(settings.EmailFallbackMaxDeliveryAgeSecs, delaySecs)

	messages, err := s.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, true)
	if err != nil {
		return nil, err
	}

	queue := model.EmailFallbackConversationQueueSummary{
		RedisChecked:       s.redis != nil,
		DelayRemainingSecs: 0,
	}
	queuedMessageIDs := map[string]bool{}
	if s.redis != nil {
		if score, err := s.redis.ZScore(ctx, emailFallbackOutboxKey, conv.ID).Result(); err == nil {
			queue.Queued = true
			fireAt := time.Unix(int64(score), 0).UTC()
			queue.FireAt = strPtr(fireAt.Format(time.RFC3339))
			if remaining := int(fireAt.Sub(s.now()).Seconds()); remaining > 0 {
				queue.DelayRemainingSecs = remaining
			}
		} else if err != nil && !errors.Is(err, redis.Nil) {
			queue.RedisError = err.Error()
		}
		if ids, err := s.redis.LRange(ctx, s.msgListKey(conv.ID), 0, -1).Result(); err == nil {
			queue.MessageIDs = uniqueEmailFallbackStrings(ids)
			for _, id := range queue.MessageIDs {
				queuedMessageIDs[id] = true
			}
		} else if err != nil && queue.RedisError == "" {
			queue.RedisError = err.Error()
		}
	}

	visitorOnline := false
	if online, err := s.isVisitorOnline(ctx, conv.WorkspaceID, conv.AnonymousID); err == nil {
		visitorOnline = online
	}

	now := s.now()
	diagnostics := make([]model.EmailFallbackMessageDiagnostics, 0, len(messages))
	for _, msg := range messages {
		diag := model.EmailFallbackMessageDiagnostics{
			ID:             msg.ID,
			CreatedAt:      msg.CreatedAt.UTC().Format(time.RFC3339),
			SenderType:     msg.SenderType,
			MessageType:    msg.MessageType,
			IsInternal:     msg.IsInternal,
			ContentPreview: truncateEmailFallbackString(strings.TrimSpace(msg.Content), 120),
			Queued:         queuedMessageIDs[msg.ID],
			Reasons:        []string{},
		}
		if msg.CancellableUntil != nil {
			diag.CancellableUntil = strPtr(msg.CancellableUntil.UTC().Format(time.RFC3339))
		}
		if msg.EmailNotifiedAt != nil {
			diag.EmailNotifiedAt = strPtr(msg.EmailNotifiedAt.UTC().Format(time.RFC3339))
		}
		if msg.EmailReadAt != nil {
			diag.EmailReadAt = strPtr(msg.EmailReadAt.UTC().Format(time.RFC3339))
		}

		if s.emailLogRepo != nil {
			logRow, err := s.emailLogRepo.GetByMessageID(ctx, msg.WorkspaceID, msg.ID)
			if err == nil && logRow != nil {
				diag.EmailLogID = logRow.ID
				diag.EmailLogStatus = logRow.Status
				diag.PostmarkMessageID = derefString(logRow.PostmarkMessageID)
			} else if err != nil {
				diag.Reasons = append(diag.Reasons, "email log lookup failed: "+err.Error())
			}
		}

		messageType := strings.TrimSpace(msg.MessageType)
		if messageType == "" {
			messageType = "reply"
		}
		fireAt := msg.CreatedAt.Add(time.Duration(delaySecs) * time.Second)
		if msg.CancellableUntil != nil {
			fireAt = *msg.CancellableUntil
		}
		diag.Due = !now.Before(fireAt)

		if msg.EmailNotifiedAt != nil {
			diag.Reasons = append(diag.Reasons, "already marked email_notified_at")
		}
		if diag.EmailLogID != "" {
			diag.Reasons = append(diag.Reasons, "already has outbound email log")
		}
		if msg.IsInternal {
			diag.Reasons = append(diag.Reasons, "internal note")
		}
		if messageType != "reply" {
			diag.Reasons = append(diag.Reasons, "message_type is not reply")
		}
		if strings.TrimSpace(msg.SenderType) == "customer" {
			diag.Reasons = append(diag.Reasons, "customer-authored message")
		}
		if conv.CustomerEmail == nil || strings.TrimSpace(*conv.CustomerEmail) == "" {
			diag.Reasons = append(diag.Reasons, "conversation has no customer_email")
		}
		if isEmailFallbackTerminalStatus(conv.Status) {
			diag.Reasons = append(diag.Reasons, "conversation status is terminal: "+conv.Status)
		}
		if conv.EmailUnsubscribed {
			diag.Reasons = append(diag.Reasons, "conversation is unsubscribed")
		}
		if !settings.EmailFallbackEnabled {
			diag.Reasons = append(diag.Reasons, "email fallback setting disabled")
		}
		if conv.ContactLastSeenAt != nil && !msg.CreatedAt.After(*conv.ContactLastSeenAt) {
			diag.Reasons = append(diag.Reasons, "visitor already saw this message")
		}
		if !msg.ExplicitEmailDelivery() && now.Sub(msg.CreatedAt) > time.Duration(maxAgeSecs)*time.Second {
			diag.Reasons = append(diag.Reasons, "message is older than fallback send window")
		}
		if visitorOnline {
			diag.Reasons = append(diag.Reasons, "visitor currently online")
		}
		if !diag.Due {
			diag.Reasons = append(diag.Reasons, "fallback delay has not elapsed")
		}
		if queue.RedisError != "" {
			diag.Reasons = append(diag.Reasons, "redis queue check failed")
		}

		diag.Eligible = len(diag.Reasons) == 0 || (len(diag.Reasons) == 1 && diag.Queued)
		diag.ReconcileCandidate = diag.Eligible && !diag.Queued && diag.EmailLogID == "" && msg.EmailNotifiedAt == nil
		if diag.Queued {
			diag.Reasons = append([]string{"currently queued in Redis"}, diag.Reasons...)
		}
		if len(diag.Reasons) == 0 {
			diag.Reasons = append(diag.Reasons, "eligible to send")
		}
		diagnostics = append(diagnostics, diag)
	}

	var contactLastSeenAt *string
	if conv.ContactLastSeenAt != nil {
		contactLastSeenAt = strPtr(conv.ContactLastSeenAt.UTC().Format(time.RFC3339))
	}

	return &model.EmailFallbackConversationDiagnosticsResponse{
		ConversationID:    conv.ID,
		WorkspaceID:       conv.WorkspaceID,
		Subject:           conv.Subject,
		Status:            conv.Status,
		CustomerEmail:     strings.TrimSpace(derefString(conv.CustomerEmail)),
		EmailUnsubscribed: conv.EmailUnsubscribed,
		ContactLastSeenAt: contactLastSeenAt,
		VisitorOnline:     visitorOnline,
		Settings: model.EmailFallbackConversationSettingsSummary{
			EmailFallbackEnabled:            settings.EmailFallbackEnabled,
			EmailFallbackDelaySecs:          delaySecs,
			EmailFallbackMaxDeliveryAgeSecs: maxAgeSecs,
		},
		Queue:    queue,
		Messages: diagnostics,
	}, nil
}

func (s *EmailFallbackService) fireEmail(ctx context.Context, conversationID string, messageIDs []string) error {
	return s.fireEmailWithOptions(ctx, conversationID, messageIDs, emailFallbackFireOptions{
		cleanupRedis:     true,
		enforceFreshness: true,
		checkOnline:      true,
	})
}

func (s *EmailFallbackService) fireEmailWithoutRedisCleanup(ctx context.Context, conversationID string, messageIDs []string) error {
	return s.fireEmailWithOptions(ctx, conversationID, messageIDs, emailFallbackFireOptions{
		cleanupRedis:     false,
		enforceFreshness: true,
		checkOnline:      true,
	})
}

func (s *EmailFallbackService) fireEmailForBackfill(ctx context.Context, conversationID string, messageIDs []string) error {
	return s.fireEmailWithOptions(ctx, conversationID, messageIDs, emailFallbackFireOptions{
		cleanupRedis:     false,
		enforceFreshness: false,
		checkOnline:      false,
	})
}

type emailFallbackFireOptions struct {
	cleanupRedis     bool
	enforceFreshness bool
	checkOnline      bool
	explicitEmail    bool
}

func (s *EmailFallbackService) fireEmailBatch(ctx context.Context, conversationID string, messageIDs []string, opts emailFallbackFireOptions) error {
	if len(messageIDs) > 0 && s.InboundDomain() == "" {
		return fmt.Errorf("support email is not configured: set SUPPORT_EMAIL_REPLY_DOMAIN")
	}
	if len(messageIDs) == 0 {
		s.logger.InfoContext(ctx, "email fallback cleaned up — no queued message ids",
			"conversation_id", conversationID,
		)
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}

	messages, err := s.messageRepo.GetByIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		s.logger.InfoContext(ctx, "email fallback cleaned up — queued messages no longer exist",
			"conversation_id", conversationID,
			"queued_message_count", len(messageIDs),
		)
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}

	conv, err := s.findConversationByID(ctx, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		s.logger.InfoContext(ctx, "email fallback cleaned up — conversation no longer exists",
			"conversation_id", conversationID,
			"queued_message_count", len(messageIDs),
		)
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}
	if conv.AnonymizedAt != nil {
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}

	if opts.explicitEmail {
		if reason := explicitRecipientChanged(messages[0], conv); reason != "" {
			return s.blockExplicitEmail(ctx, conv, messageIDs, reason, opts.cleanupRedis)
		}
		if _, err := s.validateExplicitEmail(ctx, conv.WorkspaceID, conv); err != nil {
			// Infrastructure failures remain retryable. Recipient/configuration
			// changes are visible to the teammate as blocked delivery.
			if strings.Contains(err.Error(), "try again") {
				return err
			}
			return s.blockExplicitEmail(ctx, conv, messageIDs, err.Error(), opts.cleanupRedis)
		}
	}
	if isEmailFallbackTerminalStatus(conv.Status) && !opts.explicitEmail {
		s.logger.InfoContext(ctx, "email fallback cleaned up — conversation is terminal",
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conversationID,
			"status", conv.Status,
			"queued_message_count", len(messageIDs),
		)
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}
	if conv.EmailUnsubscribed {
		s.logger.InfoContext(ctx, "email fallback cleaned up — conversation is unsubscribed",
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conversationID,
			"queued_message_count", len(messageIDs),
		)
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}
	if conv.CustomerEmail == nil || strings.TrimSpace(*conv.CustomerEmail) == "" {
		s.logger.InfoContext(ctx, "email fallback cleaned up — conversation has no customer email",
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conversationID,
			"queued_message_count", len(messageIDs),
		)
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}

	settings, err := s.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return err
	}
	if !settings.EmailFallbackEnabled {
		s.logger.InfoContext(ctx, "email fallback cleaned up — workspace setting disabled",
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conversationID,
			"queued_message_count", len(messageIDs),
		)
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}

	if s.contactRepo != nil {
		if contact, err := s.contactRepo.GetByEmail(ctx, conv.WorkspaceID, strings.TrimSpace(*conv.CustomerEmail)); err == nil && contact != nil && contact.EmailStatus == model.CRMContactEmailStatusInvalid {
			s.logger.InfoContext(ctx, "email fallback skipped — recipient marked invalid",
				"workspace_id", conv.WorkspaceID,
				"conversation_id", conversationID,
				"reason", derefString(contact.EmailStatusReason),
			)
			return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
		} else if err != nil {
			s.logger.WarnContext(ctx, "email fallback contact status lookup failed",
				"error", err,
				"workspace_id", conv.WorkspaceID,
				"conversation_id", conversationID,
			)
		}
	}

	pending := unreadFallbackMessages(conv, messages)
	if len(pending) == 0 {
		s.logger.InfoContext(ctx, "email fallback cleaned up — no unread outbound replies remain",
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conversationID,
			"queued_message_count", len(messageIDs),
			"contact_last_seen_at", conv.ContactLastSeenAt,
		)
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}
	if s.attachmentService != nil {
		if err := s.attachmentService.HydrateMessages(ctx, pending); err != nil {
			s.logger.ErrorContext(ctx, "hydrate fallback email attachments",
				"error", err,
				"workspace_id", conv.WorkspaceID,
				"conversation_id", conversationID,
			)
		}
	}

	if opts.enforceFreshness && !opts.explicitEmail {
		maxAgeSecs := normalizedEmailFallbackMaxDeliveryAgeSecs(settings.EmailFallbackMaxDeliveryAgeSecs, normalizedEmailFallbackDelaySecs(settings.EmailFallbackDelaySecs))
		freshPending := freshEmailFallbackMessages(pending, s.now(), time.Duration(maxAgeSecs)*time.Second)
		if len(freshPending) == 0 {
			s.logger.InfoContext(ctx, "email fallback skipped — unread reply is stale",
				"conversation_id", conversationID,
				"max_delivery_age_secs", maxAgeSecs,
			)
			return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
		}
		pending = freshPending
	}

	if opts.checkOnline && !opts.explicitEmail {
		if online, err := s.isVisitorOnline(ctx, conv.WorkspaceID, conv.AnonymousID); err == nil && online {
			s.logger.InfoContext(ctx, "email fallback postponed — visitor online",
				"workspace_id", conv.WorkspaceID,
				"conversation_id", conversationID,
				"retry_secs", int(emailFallbackOnlineRetry.Seconds()),
			)
			return s.postpone(ctx, conversationID, emailFallbackOnlineRetry)
		} else if err != nil {
			s.logger.WarnContext(ctx, "email fallback visitor presence lookup failed",
				"error", err,
				"workspace_id", conv.WorkspaceID,
				"conversation_id", conversationID,
			)
		}
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
	outboundFrom := s.resolveOutboundFromAddress(ctx, conv)
	fromAddress := outboundFrom.Email
	fromDisplayName := fmt.Sprintf("%s - %s", agentName, workspaceName)
	var replyMetadata struct {
		Kind string `json:"ai_reply_kind"`
	}
	if json.Unmarshal([]byte(pending[len(pending)-1].Metadata), &replyMetadata) == nil && replyMetadata.Kind == "inactivity_follow_up" && workspace != nil && strings.TrimSpace(workspace.Name) != "" {
		fromDisplayName = strings.TrimSpace(workspace.Name) + " Support"
	}
	from := fmt.Sprintf("%s <%s>", fromDisplayName, fromAddress)

	logID := uuid.NewString()
	rfcMessageID := fmt.Sprintf("<helpin-%s@%s>", logID, s.replyDomain)
	headers, err := s.buildThreadHeaders(ctx, conv.WorkspaceID, conversationID, rfcMessageID)
	if err != nil {
		return err
	}
	// Tell other mail systems this batch was generated automatically so their
	// autoresponders can decline it as well (RFC 3834).
	aiOnly := len(pending) > 0
	for _, message := range pending {
		if message.SenderType != "ai" {
			aiOnly = false
			break
		}
	}
	if aiOnly {
		headers = append(headers, email.EmailHeader{Name: "Auto-Submitted", Value: "auto-replied"})
	}
	replyTo := s.resolveConversationReplyTo(ctx, conv, workspaceName)
	unsubscribeEmail := s.unsubscribeAddress(conversationID)

	subject, err := s.buildSubject(ctx, conv, pending)
	if err != nil {
		return err
	}
	emailRecipients := supportMessageEmailRecipientsFromMetadata(pending[len(pending)-1].Metadata)
	if !opts.explicitEmail && len(emailRecipients.CC) == 0 && len(conv.EmailCC) > 0 {
		emailRecipients.CC = normalizeSupportEmailListExcluding([]string(conv.EmailCC), strings.TrimSpace(derefString(conv.CustomerEmail)))
	}
	if recipientCount := 1 + len(emailRecipients.CC) + len(emailRecipients.BCC); recipientCount > 50 {
		return fmt.Errorf("email recipient count exceeds provider limit")
	}
	chatLink, _ := s.buildChatLink(ctx, conv)
	if pending[len(pending)-1].DeliveryMode() == model.SupportDeliveryEmailOnly {
		chatLink = ""
	}
	preparedPending, emailAttachments := s.prepareEmailAttachments(ctx, pending)
	htmlBody, textBody := s.renderBodies(preparedPending, agentName, workspaceName, chatLink, unsubscribeEmail)

	var postmarkMessageID, sentFromAddress, fromFallbackReason, fromSource string
	var sendErr error
	skipped := false

	messageIDValues := make([]string, 0, len(pending))
	for _, msg := range pending {
		messageIDValues = append(messageIDValues, msg.ID)
	}
	// Hold the same row lock as deletion through the external send and its receipt.
	// Attachment fetching/rendering above remains outside this critical section.
	txErr := s.convRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.SupportConversation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", conv.ID, conv.WorkspaceID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			skipped = true
			return nil
		}
		if err != nil {
			return err
		}
		if current.AnonymizedAt != nil || current.EmailUnsubscribed {
			skipped = true
			return nil
		}
		if derefString(current.CustomerEmail) != derefString(conv.CustomerEmail) {
			return fmt.Errorf("email recipient changed during preparation; retry batch")
		}

		s.logger.InfoContext(ctx, "email fallback sending via postmark",
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conversationID,
			"from_email", fromAddress,
			"reply_to", replyTo,
			"message_count", len(pending),
			"attachment_count", len(emailAttachments),
		)
		postmarkMessageID, sentFromAddress, fromFallbackReason, sendErr = s.sendFallbackEmailWithSenderFallback(
			ctx,
			conv.WorkspaceID,
			conversationID,
			from,
			fromAddress,
			fromDisplayName,
			strings.TrimSpace(*conv.CustomerEmail),
			subject,
			htmlBody,
			textBody,
			replyTo,
			headers,
			emailAttachments,
			emailRecipients,
		)
		if sendErr != nil {
			return nil // Handle retry status after releasing the conversation lock.
		}

		fromSource = outboundFrom.Source
		if strings.TrimSpace(fromFallbackReason) != "" {
			fromSource = "verified_fallback_sender"
		}

		inReplyTo := headerValue(headers, "In-Reply-To")
		notifiedAt := s.now()
		logRow := &model.SupportEmailLog{
			ID:                 logID,
			WorkspaceID:        conv.WorkspaceID,
			ConversationID:     conversationID,
			Direction:          "outbound",
			MessageIDs:         model.DocsStringArray(messageIDValues),
			FromEmail:          sentFromAddress,
			FromDisplayName:    fromDisplayName,
			FromSource:         fromSource,
			FromFallbackReason: fromFallbackReason,
			ToEmail:            strings.TrimSpace(*conv.CustomerEmail),
			CCEmails:           model.DocsStringArray(emailRecipients.CC),
			BCCEmails:          model.DocsStringArray(emailRecipients.BCC),
			ReplyTo:            replyTo,
			Subject:            subject,
			RFCMessageID:       rfcMessageID,
			InReplyTo:          inReplyTo,
			PostmarkMessageID:  strPtr(strings.TrimSpace(postmarkMessageID)),
			StrippedText:       textBody,
			Status:             "sent",
		}

		if err := s.emailLogRepo.WithTx(tx).Create(ctx, logRow); err != nil {
			return err
		}
		repo := s.messageRepo.WithTx(tx)
		if err := repo.UpdateEmailNotifiedAt(ctx, messageIDValues, notifiedAt); err != nil {
			return err
		}
		if opts.explicitEmail {
			return repo.UpdateExplicitEmailStatus(ctx, messageIDValues, "sent", "")
		}
		return nil
	})
	if txErr != nil {
		if postmarkMessageID != "" && sendErr == nil {
			s.logger.ErrorContext(ctx, "email fallback sent but failed to persist email log",
				"error", txErr, "workspace_id", conv.WorkspaceID,
				"conversation_id", conversationID, "postmark_message_id", postmarkMessageID)
		}
		return txErr
	}
	if skipped {
		return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
	}

	if sendErr != nil {
		s.logger.ErrorContext(ctx, "email fallback send failed",
			"error", sendErr,
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conversationID,
			"from_email", fromAddress,
			"to_email", strings.TrimSpace(*conv.CustomerEmail),
			"message_count", len(pending),
		)
		if opts.explicitEmail {
			if statusErr := s.setExplicitEmailStatus(ctx, conv, messageIDs, "failed", "Email delivery failed; we will retry."); statusErr != nil {
				return statusErr
			}
		}
		return fmt.Errorf("send fallback email: %w", sendErr)
	}

	s.logger.InfoContext(ctx, "email fallback accepted by postmark",
		"workspace_id", conv.WorkspaceID,
		"conversation_id", conversationID,
		"email_log_id", logID,
		"postmark_message_id", strings.TrimSpace(postmarkMessageID),
		"from_email", sentFromAddress,
		"from_source", fromSource,
		"to_email", strings.TrimSpace(*conv.CustomerEmail),
		"message_count", len(messageIDValues),
	)
	s.publishMessageUpdated(conv.WorkspaceID, conversationID, lastString(messageIDValues), "postmark:sent")

	return s.finishEmailBatch(ctx, conversationID, messageIDs, opts.cleanupRedis)
}

func unreadFallbackMessages(conv *model.SupportConversation, messages []model.SupportMessage) []model.SupportMessage {
	if conv == nil || len(messages) == 0 {
		return []model.SupportMessage{}
	}
	lastSeen := time.Time{}
	if conv.ContactLastSeenAt != nil {
		lastSeen = *conv.ContactLastSeenAt
	}
	pending := make([]model.SupportMessage, 0, len(messages))
	for _, msg := range messages {
		messageType := strings.TrimSpace(msg.MessageType)
		if messageType == "" {
			messageType = "reply"
		}
		if msg.EmailNotifiedAt != nil ||
			msg.IsInternal ||
			messageType != "reply" ||
			strings.TrimSpace(msg.SenderType) == "customer" {
			continue
		}
		if msg.DeliveryMode() == model.SupportDeliveryChatOnly || explicitEmailBlocked(msg) {
			continue
		}
		if !msg.ExplicitEmailDelivery() && !lastSeen.IsZero() && !msg.CreatedAt.After(lastSeen) {
			continue
		}
		pending = append(pending, msg)
	}
	return pending
}

func freshEmailFallbackMessages(messages []model.SupportMessage, now time.Time, maxAge time.Duration) []model.SupportMessage {
	if len(messages) == 0 {
		return []model.SupportMessage{}
	}
	if maxAge <= 0 || now.IsZero() {
		return messages
	}

	cutoff := now.Add(-maxAge)
	fresh := make([]model.SupportMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.CreatedAt.IsZero() || !msg.CreatedAt.Before(cutoff) {
			fresh = append(fresh, msg)
		}
	}
	return fresh
}

func countMessagesWithEmailNotifiedAt(ctx context.Context, repo *repository.SupportMessageRepository, ids []string) int {
	if repo == nil || len(ids) == 0 {
		return 0
	}
	messages, err := repo.GetByIDs(ctx, ids)
	if err != nil {
		return 0
	}
	count := 0
	for _, msg := range messages {
		if msg.EmailNotifiedAt != nil {
			count++
		}
	}
	return count
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
		conv, err = s.findConversationByID(ctx, logRow.ConversationID)
		if err != nil {
			return err
		}
		if conv != nil && conv.AnonymizedAt != nil {
			return nil
		}
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
		"workspace_id", logRow.WorkspaceID,
		"conversation_id", logRow.ConversationID,
		"from_email", logRow.FromEmail,
		"to_email", logRow.ToEmail,
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
		conv, err = s.findConversationByID(ctx, logRow.ConversationID)
		if err != nil {
			return err
		}
		if conv != nil && conv.AnonymizedAt != nil {
			return nil
		}
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
		"workspace_id", logRow.WorkspaceID,
		"conversation_id", logRow.ConversationID,
		"from_email", logRow.FromEmail,
		"to_email", logRow.ToEmail,
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
		conv, err = s.findConversationByID(ctx, logRow.ConversationID)
		if err != nil {
			return err
		}
		if conv != nil && conv.AnonymizedAt != nil {
			return nil
		}
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
		"workspace_id", logRow.WorkspaceID,
		"conversation_id", logRow.ConversationID,
		"from_email", logRow.FromEmail,
		"to_email", logRow.ToEmail,
		"bounce_type", strings.TrimSpace(payload.Type),
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
		conv, err = s.findConversationByID(ctx, logRow.ConversationID)
		if err != nil {
			return err
		}
		if conv != nil && conv.AnonymizedAt != nil {
			return nil
		}
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
		"workspace_id", logRow.WorkspaceID,
		"conversation_id", logRow.ConversationID,
		"from_email", logRow.FromEmail,
		"to_email", logRow.ToEmail,
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
	return s.acquireOrRenewNamedLease(ctx, emailFallbackLockKey)
}

func (s *EmailFallbackService) acquireOrRenewNamedLease(ctx context.Context, key string) (bool, error) {
	if s.podID == "" {
		s.podID = "email-fallback"
	}
	current, err := s.redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return s.redis.SetNX(ctx, key, s.podID, s.leaseTTL).Result()
	}
	if err != nil {
		return false, err
	}
	if current == s.podID {
		if err := s.redis.Expire(ctx, key, s.leaseTTL).Err(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func supportConversationReplyTo(conversationID, replyDomain, displayName string) string {
	address := fmt.Sprintf("conv-%s@%s", strings.TrimSpace(conversationID), strings.TrimSpace(replyDomain))
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return address
	}
	return (&mail.Address{Name: displayName, Address: address}).String()
}

// supportRouteReplyTo formats a Reply-To header using a workspace's route-backed
// inbound address with an optional friendly display name.
func supportRouteReplyTo(inboundAddress, displayName string) string {
	inboundAddress = strings.TrimSpace(inboundAddress)
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return inboundAddress
	}
	return (&mail.Address{Name: displayName, Address: inboundAddress}).String()
}

func (s *EmailFallbackService) resolveConversationReplyTo(ctx context.Context, conv *model.SupportConversation, displayName string) string {
	if conv == nil {
		return supportRouteReplyTo("", displayName)
	}
	legacy := supportConversationReplyTo(conv.ID, s.replyDomain, displayName)
	if s.supportInboxService == nil || s.supportInboxService.emailRouteRepo == nil {
		return legacy
	}
	route, err := s.supportInboxService.emailRouteRepo.GetActiveByMailbox(ctx, conv.WorkspaceID, conv.MailboxID)
	if err != nil {
		s.logger.WarnContext(ctx, "resolve route reply-to failed, using legacy conv address",
			"error", err,
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conv.ID,
		)
		return legacy
	}
	if route == nil || strings.TrimSpace(route.InboundAddress) == "" {
		return legacy
	}
	return supportRouteReplyTo(route.InboundAddress, displayName)
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
	}
	if len(previous) > 0 {
		headers = append(headers, email.EmailHeader{Name: "In-Reply-To", Value: previous[len(previous)-1]})
		headers = append(headers, email.EmailHeader{Name: "References", Value: strings.Join(previous, " ")})
	}
	return headers, nil
}

func (s *EmailFallbackService) buildSubject(ctx context.Context, conv *model.SupportConversation, pending []model.SupportMessage) (string, error) {
	if len(pending) > 0 && pending[0].ExplicitEmailDelivery() {
		if subject := explicitDeliveryMetadata(pending[0]).Subject; subject != "" {
			return subject, nil
		}
	}
	base := strings.TrimSpace(conv.Subject)
	if base == "" {
		if len(pending) > 0 {
			base = truncateEmailFallbackString(strings.TrimSpace(pending[0].Content), 60)
		}
		if base == "" {
			base = "Support conversation"
		}
	}
	shortID := ""
	if conv.DisplayID > 0 {
		shortID = strconv.Itoa(conv.DisplayID)
	} else {
		shortID = conv.ID
		if len(shortID) > 4 {
			shortID = shortID[:4]
		}
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
	if s.sessionRepo == nil {
		return "", nil
	}
	session, err := s.sessionRepo.GetLatestByConversation(ctx, conv.WorkspaceID, conv.ID)
	if err != nil {
		return "", err
	}
	if session == nil || session.LastPageURL == nil {
		return "", nil
	}
	pageURL := strings.TrimSpace(*session.LastPageURL)
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
		if text == "" && len(msg.Attachments) == 0 {
			continue
		}
		var textChunk strings.Builder
		if text != "" {
			textChunk.WriteString(text)
			htmlChunks = append(htmlChunks, renderMessageMarkdownToHTML(text))
		}
		if len(msg.Attachments) > 0 {
			attachmentText := renderAttachmentTextList(msg.Attachments)
			if attachmentText != "" {
				if textChunk.Len() > 0 {
					textChunk.WriteString("\n\n")
				}
				textChunk.WriteString(attachmentText)
			}
			if attachmentHTML := renderAttachmentHTMLList(msg.Attachments); attachmentHTML != "" {
				htmlChunks = append(htmlChunks, attachmentHTML)
			}
		}
		if textChunk.Len() > 0 {
			textChunks = append(textChunks, textChunk.String())
		}
	}

	var htmlBody strings.Builder
	workspaceID := ""
	for _, message := range messages {
		if strings.TrimSpace(message.WorkspaceID) != "" {
			workspaceID = message.WorkspaceID
			break
		}
	}
	attributionURL := buildHelpinEmailAttributionURL(workspaceName, workspaceID)
	htmlBody.WriteString(renderSupportEmailHiddenPreheader(messages, workspaceName))
	htmlBody.WriteString(`<p style="margin:0 0 16px;color:#9ca3af;font-size:12px;line-height:18px;">`)
	htmlBody.WriteString(html.EscapeString(supportEmailReplyDelimiter))
	htmlBody.WriteString("</p>")
	for _, chunk := range htmlChunks {
		htmlBody.WriteString(chunk)
	}
	htmlBody.WriteString("<p>--<br>")
	htmlBody.WriteString(html.EscapeString(agentName))
	htmlBody.WriteString(" via ")
	htmlBody.WriteString(html.EscapeString(workspaceName))
	htmlBody.WriteString("</p>")
	if chatLink != "" {
		htmlBody.WriteString(`<p><a href="`)
		htmlBody.WriteString(html.EscapeString(chatLink))
		htmlBody.WriteString(`">View conversation in browser</a></p>`)
	}
	htmlBody.WriteString(`<p style="border-top:1px solid #e5e7eb;margin-top:20px;padding-top:12px;color:#6b7280;font-size:12px;line-height:18px;">Powered by <a href="`)
	htmlBody.WriteString(html.EscapeString(attributionURL))
	htmlBody.WriteString(`" style="color:#6b7280;text-decoration:none;"><strong>Helpin AI</strong></a></p>`)

	textBody := supportEmailReplyDelimiter + "\n\n" + strings.Join(textChunks, "\n\n")
	if strings.TrimSpace(strings.Join(textChunks, "")) != "" {
		textBody += "\n\n"
	}
	textBody += "--\n" + agentName + " via " + workspaceName
	if chatLink != "" {
		textBody += "\n\nView conversation in browser:\n" + chatLink
	}
	textBody += "\n\nPowered by Helpin AI: " + attributionURL
	return htmlBody.String(), textBody
}

func renderSupportEmailHiddenPreheader(messages []model.SupportMessage, workspaceName string) string {
	preview := supportEmailPreviewText(messages, workspaceName)
	if preview == "" {
		return ""
	}
	return `<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;mso-hide:all;line-height:1px;font-size:1px;">` +
		html.EscapeString(preview) +
		`</div>`
}

func supportEmailPreviewText(messages []model.SupportMessage, workspaceName string) string {
	for _, msg := range messages {
		if preview := truncateSupportEmailPreview(collapseSupportEmailPreviewWhitespace(msg.Content)); preview != "" {
			return preview
		}
	}
	for _, msg := range messages {
		if len(msg.Attachments) > 0 {
			name := strings.TrimSpace(workspaceName)
			if name == "" {
				name = "Helpin Support"
			}
			return "Attachment from " + name
		}
	}
	return ""
}

func collapseSupportEmailPreviewWhitespace(content string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
}

func truncateSupportEmailPreview(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= supportEmailPreviewMaxRunes {
		return string(runes)
	}
	if supportEmailPreviewMaxRunes <= 1 {
		return string(runes[:supportEmailPreviewMaxRunes])
	}
	return strings.TrimSpace(string(runes[:supportEmailPreviewMaxRunes-1])) + "…"
}

func (s *EmailFallbackService) prepareEmailAttachments(ctx context.Context, messages []model.SupportMessage) ([]model.SupportMessage, []email.Attachment) {
	if len(messages) == 0 || s == nil || s.attachmentDownloader == nil {
		return messages, nil
	}

	prepared := make([]model.SupportMessage, len(messages))
	copy(prepared, messages)

	var totalRawBytes int64
	postmarkAttachments := make([]email.Attachment, 0)
	for msgIndex := range prepared {
		if len(prepared[msgIndex].Attachments) == 0 {
			continue
		}
		msgAttachments := make([]model.SupportAttachmentPayload, len(prepared[msgIndex].Attachments))
		copy(msgAttachments, prepared[msgIndex].Attachments)
		for attachmentIndex := range msgAttachments {
			attachment := msgAttachments[attachmentIndex]
			if !isSupportEmailAttachmentCandidate(attachment, totalRawBytes) {
				continue
			}
			data, err := s.attachmentDownloader.DownloadContent(ctx, attachment)
			if err != nil {
				if s.logger != nil {
					s.logger.WarnContext(ctx, "download support attachment for fallback email",
						"error", err,
						"attachment_id", attachment.ID,
						"file_name", attachment.FileName,
					)
				}
				continue
			}
			rawSize := int64(len(data))
			if rawSize <= 0 || rawSize > supportEmailAttachmentMaxFileBytes || totalRawBytes+rawSize > supportEmailAttachmentMaxTotalRawBytes {
				continue
			}
			name := strings.TrimSpace(attachment.FileName)
			if name == "" {
				name = "attachment"
			}
			contentType := strings.TrimSpace(attachment.FileType)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			postmarkAttachments = append(postmarkAttachments, email.Attachment{
				Name:        name,
				ContentType: contentType,
				Content:     base64.StdEncoding.EncodeToString(data),
			})
			totalRawBytes += rawSize
			msgAttachments[attachmentIndex].URL = ""
		}
		prepared[msgIndex].Attachments = msgAttachments
	}

	return prepared, postmarkAttachments
}

func isSupportEmailAttachmentCandidate(attachment model.SupportAttachmentPayload, currentTotalBytes int64) bool {
	return strings.TrimSpace(attachment.FileKey) != "" &&
		attachment.FileSize > 0 &&
		attachment.FileSize <= supportEmailAttachmentMaxFileBytes &&
		currentTotalBytes+attachment.FileSize <= supportEmailAttachmentMaxTotalRawBytes
}

func renderAttachmentTextList(attachments []model.SupportAttachmentPayload) string {
	attachments = supportEmailLinkedAttachments(attachments)
	if len(attachments) == 0 {
		return ""
	}
	var text strings.Builder
	text.WriteString("Attachments:")
	for _, attachment := range attachments {
		name := strings.TrimSpace(attachment.FileName)
		if name == "" {
			name = "Attachment"
		}
		text.WriteString("\n- ")
		text.WriteString(name)
		if url := strings.TrimSpace(attachment.URL); url != "" {
			text.WriteString(": ")
			text.WriteString(url)
		}
	}
	return text.String()
}

func renderAttachmentHTMLList(attachments []model.SupportAttachmentPayload) string {
	attachments = supportEmailLinkedAttachments(attachments)
	if len(attachments) == 0 {
		return ""
	}
	var htmlBody strings.Builder
	htmlBody.WriteString("<p><strong>Attachments:</strong></p><ul>")
	for _, attachment := range attachments {
		name := strings.TrimSpace(attachment.FileName)
		if name == "" {
			name = "Attachment"
		}
		htmlBody.WriteString("<li>")
		if url := strings.TrimSpace(attachment.URL); url != "" {
			htmlBody.WriteString(`<a href="`)
			htmlBody.WriteString(html.EscapeString(url))
			htmlBody.WriteString(`">`)
			htmlBody.WriteString(html.EscapeString(name))
			htmlBody.WriteString("</a>")
		} else {
			htmlBody.WriteString(html.EscapeString(name))
		}
		htmlBody.WriteString("</li>")
	}
	htmlBody.WriteString("</ul>")
	return htmlBody.String()
}

func supportEmailLinkedAttachments(attachments []model.SupportAttachmentPayload) []model.SupportAttachmentPayload {
	if len(attachments) == 0 {
		return nil
	}
	linked := make([]model.SupportAttachmentPayload, 0, len(attachments))
	for _, attachment := range attachments {
		if strings.TrimSpace(attachment.URL) != "" {
			linked = append(linked, attachment)
		}
	}
	return linked
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

func (s *EmailFallbackService) cleanupIfRequested(ctx context.Context, conversationID string, cleanupRedis bool) error {
	if !cleanupRedis {
		return nil
	}
	return s.cleanup(ctx, conversationID)
}

func (s *EmailFallbackService) postpone(ctx context.Context, conversationID string, delay time.Duration) error {
	if s == nil || s.redis == nil {
		return nil
	}
	if delay <= 0 {
		delay = emailFallbackOnlineRetry
	}
	fireAt := s.now().Add(delay)
	if _, err := s.redis.ZAddArgs(ctx, emailFallbackOutboxKey, redis.ZAddArgs{
		XX:      true,
		Members: []redis.Z{{Score: float64(fireAt.Unix()), Member: conversationID}},
	}).Result(); err != nil {
		return fmt.Errorf("postpone email fallback outbox: %w", err)
	}
	return nil
}

func normalizedEmailFallbackDelaySecs(delaySecs int) int {
	if delaySecs < 10 || delaySecs > 600 {
		return model.DefaultSupportInboxSettings().EmailFallbackDelaySecs
	}
	return delaySecs
}

func normalizedEmailFallbackMaxDeliveryAgeSecs(maxAgeSecs, delaySecs int) int {
	if maxAgeSecs < 120 || maxAgeSecs > 1800 || maxAgeSecs < delaySecs {
		return model.DefaultSupportInboxSettings().EmailFallbackMaxDeliveryAgeSecs
	}
	return maxAgeSecs
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
		domain = deployment.DefaultReplyDomain
	}
	if domain == "" {
		return ""
	}
	return fmt.Sprintf("unsubscribe-%s@%s", conversationID, domain)
}

func (s *EmailFallbackService) sendFallbackEmailWithSenderFallback(
	ctx context.Context,
	workspaceID string,
	conversationID string,
	from string,
	fromAddress string,
	fromDisplayName string,
	to string,
	subject string,
	htmlBody string,
	textBody string,
	replyTo string,
	headers []email.EmailHeader,
	attachments []email.Attachment,
	recipients supportMessageEmailRecipients,
) (string, string, string, error) {
	options := email.SendEmailOptions{CC: recipients.CC, BCC: recipients.BCC}
	postmarkMessageID, err := s.emailClient.SendEmailWithHeadersAttachmentsAndOptionsContext(ctx, from, to, subject, htmlBody, textBody, replyTo, headers, attachments, options)
	if err == nil {
		return postmarkMessageID, fromAddress, "", nil
	}
	if !email.IsSenderSignatureError(err) {
		return "", fromAddress, "", err
	}

	fallbackFromAddress := strings.TrimSpace(s.emailClient.FromEmail())
	if fallbackFromAddress == "" || strings.EqualFold(fallbackFromAddress, strings.TrimSpace(fromAddress)) {
		return "", fromAddress, "", err
	}
	fallbackFrom := fmt.Sprintf("%s <%s>", fromDisplayName, fallbackFromAddress)
	s.logger.WarnContext(ctx, "email fallback branded sender rejected, retrying with verified sender",
		"error", err,
		"workspace_id", workspaceID,
		"conversation_id", conversationID,
		"branded_from_email", fromAddress,
		"fallback_from_email", fallbackFromAddress,
		"reply_to", replyTo,
	)
	postmarkMessageID, fallbackErr := s.emailClient.SendEmailWithHeadersAttachmentsAndOptionsContext(ctx, fallbackFrom, to, subject, htmlBody, textBody, replyTo, headers, attachments, options)
	if fallbackErr != nil {
		return "", fallbackFromAddress, "postmark_sender_signature_rejected", fmt.Errorf("retry with verified sender after branded sender rejection: %w", fallbackErr)
	}
	s.logger.InfoContext(ctx, "email fallback sent with verified sender fallback",
		"workspace_id", workspaceID,
		"conversation_id", conversationID,
		"branded_from_email", fromAddress,
		"fallback_from_email", fallbackFromAddress,
		"reply_to", replyTo,
	)
	return postmarkMessageID, fallbackFromAddress, "postmark_sender_signature_rejected", nil
}

// resolveOutboundFromAddress returns the preferred branded sender address for a
// conversation's outbound email. Postmark may still reject this address until
// the workspace/domain is verified for outbound sending; the send path retries
// with the configured verified sender while keeping the conversation Reply-To.
func (s *EmailFallbackService) resolveOutboundFromAddress(ctx context.Context, conv *model.SupportConversation) SupportOutboundFromAddressResult {
	fallback := SupportOutboundFromAddressResult{Email: s.emailClient.FromEmail(), Source: "verified_fallback_sender"}
	if s.supportInboxService == nil || conv == nil {
		return fallback
	}
	result, err := s.supportInboxService.ResolveOutboundFromAddress(ctx, conv.WorkspaceID, conv.MailboxID)
	if err != nil {
		s.logger.WarnContext(ctx, "mailbox-branded outbound from unavailable, using fallback sender",
			"error", err,
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conv.ID,
		)
		return fallback
	}
	if strings.TrimSpace(result.Email) == "" {
		return fallback
	}
	return result
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
		return fmt.Errorf("inbound route not found or inactive")
	}

	return s.processInboundRoute(ctx, route, payload, rawPayload)
}

func (s *EmailFallbackService) processInboundSenderForwardingVerification(ctx context.Context, mailboxHash string, payload model.PostmarkInboundPayload) error {
	if s == nil || s.supportInboxService == nil {
		return nil
	}
	sender, verified, err := s.supportInboxService.CompleteEmailSenderForwardingVerification(ctx, mailboxHash, payload)
	if err != nil {
		return err
	}
	if sender == nil {
		s.logger.InfoContext(ctx, "postmark inbound sender forwarding verification not found",
			"message_id", strings.TrimSpace(payload.MessageID),
			"mailbox_hash", mailboxHash,
		)
		return nil
	}
	if verified {
		s.logger.InfoContext(ctx, "postmark inbound sender forwarding verified",
			"message_id", strings.TrimSpace(payload.MessageID),
			"workspace_id", sender.WorkspaceID,
			"sender_id", sender.ID,
			"sender_email", sender.Email,
		)
		return nil
	}
	s.logger.InfoContext(ctx, "postmark inbound sender forwarding verification did not match sender address",
		"message_id", strings.TrimSpace(payload.MessageID),
		"workspace_id", sender.WorkspaceID,
		"sender_id", sender.ID,
		"sender_email", sender.Email,
	)
	return nil
}

func (s *EmailFallbackService) processInboundRoute(ctx context.Context, route *model.SupportEmailRoute, payload model.PostmarkInboundPayload, rawPayload string) error {
	if route == nil {
		return nil
	}
	isConfirmation := isProviderForwardingConfirmation(payload)
	consumed, err := s.inspectInboundRouteVerification(ctx, route, payload)
	if err != nil {
		return err
	}
	if consumed {
		return nil
	}

	conversation, err := s.resolveInboundRouteConversation(ctx, route.WorkspaceID, payload)
	if err != nil {
		return err
	}
	if conversation != nil {
		if isConfirmation {
			if err := s.storeRouteConfirmationConversation(ctx, route, conversation.ID); err != nil {
				return err
			}
		}
		return s.processInboundConversationReply(ctx, conversation, route, payload, rawPayload)
	}

	return s.createInboundConversationFromRoute(ctx, route, payload, rawPayload)
}

func (s *EmailFallbackService) inspectInboundRouteVerification(ctx context.Context, route *model.SupportEmailRoute, payload model.PostmarkInboundPayload) (bool, error) {
	if s == nil || route == nil || s.supportInboxService == nil || s.supportInboxService.emailRouteRepo == nil {
		return false, nil
	}
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}

	if isProviderForwardingConfirmation(payload) {
		if route.ConfirmationReceivedAt == nil {
			route.ConfirmationReceivedAt = &now
			if err := s.supportInboxService.emailRouteRepo.Update(ctx, route); err != nil {
				return false, err
			}
		}
		return false, nil
	}

	token := strings.TrimSpace(route.ForwardingVerificationToken)
	if strings.Contains(payload.Subject, supportEmailRouteVerificationSubject) {
		if token != "" && strings.Contains(payload.Subject, "["+token+"]") {
			route.ForwardingVerifiedAt = &now
			route.LastInboundAt = &now
			route.ForwardingVerificationToken = ""
			route.ForwardingLastError = nil
			if err := s.supportInboxService.emailRouteRepo.Update(ctx, route); err != nil {
				return false, err
			}
			s.logger.InfoContext(ctx, "postmark inbound forwarding test verified",
				"message_id", strings.TrimSpace(payload.MessageID),
				"workspace_id", route.WorkspaceID,
				"route_id", route.ID,
			)
		}
		return true, nil
	}

	if route.ForwardingVerifiedAt == nil && route.SourceAddress != nil && inboundPayloadMentionsAddress(payload, *route.SourceAddress) {
		route.ForwardingVerifiedAt = &now
		route.ForwardingLastError = nil
		if err := s.supportInboxService.emailRouteRepo.Update(ctx, route); err != nil {
			return false, err
		}
	}
	return false, nil
}

func isProviderForwardingConfirmation(payload model.PostmarkInboundPayload) bool {
	sender := strings.ToLower(strings.TrimSpace(payload.FromFull.Email))
	subject := strings.ToLower(strings.TrimSpace(payload.Subject))
	body := strings.ToLower(payload.TextBody + "\n" + payload.HtmlBody)
	if sender == "forwarding-noreply@google.com" || strings.Contains(body, "mail-settings.google.com/mail/vf-") {
		return true
	}
	return strings.Contains(sender, "zoho") && strings.Contains(subject, "forward") &&
		(strings.Contains(subject, "confirm") || strings.Contains(subject, "verif"))
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

	projection := inboundPayloadProjection(payload)
	content, htmlBody := projection.VisibleText, projection.HTMLBody
	if content == "" && len(payload.Attachments) == 0 {
		return nil
	}
	if len(content) > 50_000 {
		content = content[:50_000]
	}

	senderName := strings.TrimSpace(payload.FromFull.Name)
	replyToRaw, replyToEmail, replyToName := inboundReplyToAddress(payload)

	settings, err := s.loadSettings(ctx, route.WorkspaceID)
	if err != nil {
		return err
	}
	routeDomain := ""
	if s.supportInboxService != nil {
		routeDomain = s.supportInboxService.routeDomain
	}
	forwardedAttribution := forwardedEmailDetectionResult{}
	if settings.ForwardedEmailDetectionEnabled && strings.TrimSpace(settings.ForwardedEmailDetectionMode) == "high_confidence_any_sender" {
		forwardedAttribution = detectForwardedEmailAttribution(forwardedEmailDetectionInput{
			ForwarderEmail: fromEmail,
			ForwarderName:  senderName,
			RecipientEmails: []string{
				payload.To,
				payload.OriginalRecipient,
				route.InboundAddress,
				inboundRecipientAddress(payload),
			},
			ReplyDomain:   s.InboundDomain(),
			RouteDomain:   routeDomain,
			Text:          inboundForwardedEmailScanText(payload, content),
			MinConfidence: settings.ForwardedEmailMinConfidence,
		})
	}
	effectiveSenderName := senderName
	effectiveSenderEmail := fromEmail
	if forwardedAttribution.Applied {
		content = inboundForwardedEmailDisplaySource(payload, content, forwardedAttribution)
		if len(content) > 50_000 {
			content = content[:50_000]
		}
		applyForwardedEmailProjection(&projection, content)
		effectiveSenderName = strings.TrimSpace(forwardedAttribution.OriginalName)
		effectiveSenderEmail = strings.TrimSpace(forwardedAttribution.OriginalEmail)
		if effectiveSenderName == "" {
			effectiveSenderName = effectiveSenderEmail
		}
	} else if replyToEmail != "" {
		effectiveSenderEmail = replyToEmail
		effectiveSenderName = replyToName
	}
	if strings.TrimSpace(effectiveSenderName) == "" || strings.EqualFold(strings.TrimSpace(effectiveSenderName), effectiveSenderEmail) {
		if derivedName := deriveWidgetNameFromEmail(effectiveSenderEmail); derivedName != "" {
			effectiveSenderName = derivedName
		}
	}
	if strings.TrimSpace(effectiveSenderName) == "" {
		effectiveSenderName = effectiveSenderEmail
	}

	subject := strings.TrimSpace(payload.Subject)
	if subject == "" {
		subject = fmt.Sprintf("Email from %s", effectiveSenderName)
	}

	customerName := effectiveSenderName
	customerEmail := effectiveSenderEmail
	primaryRecipientState := model.SupportPrimaryRecipientStateConfirmed
	var suggestedPrimaryRecipientEmail *string
	var suggestedPrimaryRecipientName *string
	threadParticipants := inboundVisibleThreadParticipants(payload, route.InboundAddress, customerEmail)
	routeAddressInCC := inboundRouteAddressInCC(payload, route.InboundAddress)
	emailCC := model.DocsStringArray{}
	if routeAddressInCC {
		if email, name := inboundSuggestedPrimaryRecipient(payload, route.InboundAddress, customerEmail); email != "" {
			primaryRecipientState = model.SupportPrimaryRecipientStateUnconfirmed
			suggestedPrimaryRecipientEmail = strPtr(email)
			if strings.TrimSpace(name) != "" {
				suggestedPrimaryRecipientName = strPtr(strings.TrimSpace(name))
			}
		}
	} else if inboundRouteAddressInTo(payload, route.InboundAddress) {
		emailCC = model.DocsStringArray(normalizeSupportEmailListExcluding(inboundCCEmails(payload), route.InboundAddress, customerEmail))
	}
	viaEmail := "email"
	now := s.now()
	spamSignals := postmarkInboundSpamSignalsFromHeaders(payload.Headers)
	messageMetadata := inboundEmailAIMetadata(spamSignals.messageMetadata(), payload, content)
	if forwardedAttribution.Applied {
		messageMetadata = mergeForwardedAttributionMetadata(messageMetadata, forwardedAttribution)
	}
	isNotice := inboundEmailHasAbsenceNotice(messageMetadata)
	status := model.SupportConversationStatusOpen
	var closedAt *time.Time
	if spamSignals.shouldAutoSpamNewConversation() {
		status = model.SupportConversationStatusSpam
		closedAt = &now
	}

	if isNotice {
		status = model.SupportConversationStatusResolved
		closedAt = &now
	}

	routeMailboxID := route.MailboxID
	var (
		mailbox    *model.SupportMailbox
		mailboxErr error
	)
	if status != model.SupportConversationStatusSpam && route.MailboxID != nil && strings.TrimSpace(*route.MailboxID) != "" && s.supportInboxService.mailboxRepo != nil {
		mailbox, mailboxErr = s.supportInboxService.mailboxRepo.GetByID(ctx, route.WorkspaceID, strings.TrimSpace(*route.MailboxID))
		if mailboxErr != nil {
			return mailboxErr
		}
		if mailbox == nil || !mailbox.Active {
			routeMailboxID = nil
			mailbox = nil
		}
	}

	conversation := &model.SupportConversation{
		ID:                             inboundStableID("conversation:" + payload.MessageID),
		WorkspaceID:                    route.WorkspaceID,
		MailboxID:                      routeMailboxID,
		Subject:                        subject,
		Status:                         status,
		ClosedAt:                       closedAt,
		FlowState:                      strPtr(model.SupportConversationFlowStateWaitingForHuman),
		Priority:                       "medium",
		Channel:                        "email",
		CustomerName:                   &customerName,
		CustomerEmail:                  &customerEmail,
		PrimaryRecipientState:          primaryRecipientState,
		SuggestedPrimaryRecipientEmail: suggestedPrimaryRecipientEmail,
		SuggestedPrimaryRecipientName:  suggestedPrimaryRecipientName,
		EmailCC:                        emailCC,
		EmailThreadParticipants:        model.DocsStringArray(threadParticipants),
		Source:                         "email",
	}

	if !isNotice && conversation.Status != model.SupportConversationStatusSpam && conversation.MailboxID == nil && s.supportInboxService != nil {
		if mailboxID, _, mailboxErr := s.supportInboxService.maybeApplyMailboxRoutingForChannel(ctx, route.WorkspaceID, nil, true, "email"); mailboxErr == nil {
			conversation.MailboxID = mailboxID
		}
	}

	if mailbox != nil && !isNotice {
		ownerID, flowState, ownerErr := s.supportInboxService.determineMailboxOwner(ctx, route.WorkspaceID, mailbox, nil)
		if ownerErr != nil {
			return ownerErr
		}
		conversation.AssignedUserID = ownerID
		conversation.FlowState = strPtr(flowState)
	}

	message := &model.SupportMessage{
		ID:                inboundStableID("message:" + payload.MessageID),
		WorkspaceID:       route.WorkspaceID,
		SenderType:        "customer",
		SenderDisplayName: &customerName,
		Content:           content,
		IsInternal:        false,
		MessageType:       "reply",
		Metadata:          messageMetadata,
		ViaChannel:        &viaEmail,
	}

	if isNotice {
		// System closure is not an AI or human resolution. Do not stamp
		// resolved_at or emit a resolution event used by performance metrics.
		conversation.FlowState = nil
		message.MessageType = model.SupportMessageTypeEmailNotice
	}

	rfcMessageID := inboundRFCMessageID(payload)
	inReplyTo := normalizeRFCHeaderValue(inboundHeaderValue(payload.Headers, "In-Reply-To"))
	referencesHeader := strings.TrimSpace(inboundHeaderValue(payload.Headers, "References"))
	recipientAddress := inboundRecipientAddress(payload)
	ccEmails := inboundCCEmails(payload)
	bccEmails := inboundBCCEmails(payload)

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
				// Snapshot an unambiguous CRM membership for this new email
				// conversation. Replies preserve any later selection or clear.
				if conversation.CRMCompanyID == nil {
					companyID, err := repository.NewCRMAssociationRepository(tx).SingleCompanyForContact(ctx, conversation.WorkspaceID, *contactID)
					if err != nil {
						return err
					}
					conversation.CRMCompanyID = companyID
				}
				if err := convRepoTx.Update(ctx, conversation); err != nil {
					return err
				}
			}
		}

		message.ConversationID = conversation.ID
		if err := msgRepoTx.Create(ctx, message); err != nil {
			return err
		}
		if err := s.enqueueInboundAttachments(ctx, tx, message, payload); err != nil {
			return err
		}
		message.EmailFrom = fromEmail
		message.EmailTo = strings.TrimSpace(payload.To)
		message.EmailCC = model.DocsStringArray(ccEmails)
		message.HTMLBody = htmlBody
		message.StrippedText = content
		message.EmailVisibleText = projection.VisibleText
		message.EmailQuotedText = projection.QuotedText
		message.EmailHasQuotedContent = boolPtr(projection.HasQuotedContent)
		message.EmailProjectionConfidence = projection.ProjectionConfidence
		message.EmailProjectionVersion = projection.Version

		logRow := &model.SupportEmailLog{
			WorkspaceID:               route.WorkspaceID,
			ConversationID:            conversation.ID,
			EmailRouteID:              &route.ID,
			Direction:                 "inbound",
			MessageIDs:                model.DocsStringArray{message.ID},
			FromEmail:                 fromEmail,
			ToEmail:                   strings.TrimSpace(payload.To),
			ReplyTo:                   replyToRaw,
			RecipientAddress:          recipientAddress,
			CCEmails:                  model.DocsStringArray(ccEmails),
			BCCEmails:                 model.DocsStringArray(bccEmails),
			Subject:                   subject,
			RFCMessageID:              rfcMessageID,
			InReplyTo:                 inReplyTo,
			ReferencesHeader:          referencesHeader,
			PostmarkMessageID:         strPtr(strings.TrimSpace(payload.MessageID)),
			RawBody:                   rawPayload,
			StrippedText:              content,
			HTMLBody:                  htmlBody,
			EmailVisibleText:          projection.VisibleText,
			EmailQuotedText:           projection.QuotedText,
			EmailHasQuotedContent:     projection.HasQuotedContent,
			EmailProjectionConfidence: projection.ProjectionConfidence,
			EmailProjectionVersion:    projection.Version,
			Status:                    "sent",
		}
		if logRow.PostmarkMessageID != nil && *logRow.PostmarkMessageID == "" {
			logRow.PostmarkMessageID = nil
		}
		if err := emailLogRepoTx.Create(ctx, logRow); err != nil {
			return err
		}

		if isNotice {
			event := model.SystemEventClosed
			if err := msgRepoTx.Create(ctx, &model.SupportMessage{
				ID: uuid.NewString(), WorkspaceID: conversation.WorkspaceID,
				ConversationID: conversation.ID, SenderType: "system",
				MessageType: "system", SystemEventType: &event, IsInternal: true,
				Content:  "Automatically closed: out-of-office reply.",
				Metadata: `{"closure_reason":"out_of_office","closure_actor":"system"}`,
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if txErr != nil {
		if isLikelyUniqueConstraintError(txErr) {
			existing, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, strings.TrimSpace(payload.MessageID))
			if err != nil {
				return err
			}
			// A constraint error alone does not prove this email was saved.
			if existing != nil {
				return s.retryInboundCustomerAIRequest(ctx, existing)
			}
		}
		return txErr
	}
	if s.supportInboxService.emailRouteRepo != nil {
		route.LastInboundAt = &now
		if err := s.supportInboxService.emailRouteRepo.TouchInbound(ctx, route.ID, now); err != nil {
			return err
		}
		if isProviderForwardingConfirmation(payload) {
			if err := s.storeRouteConfirmationConversation(ctx, route, conversation.ID); err != nil {
				return err
			}
		}
	}

	if !isNotice {
		ProcessSupportCustomerReplyNotification(ctx, s.notificationService, s.pushSenderService, conversation, content, customerName)
	}
	if !isNotice && s.supportInboxService != nil {
		s.supportInboxService.recordSupportEvent(SupportEventInput{
			WorkspaceID: conversation.WorkspaceID, EventType: model.SupportEventCustomerMessageCreated,
			ConversationID: &conversation.ID, MessageID: &message.ID,
			ActorType: model.SupportEventActorCustomer, Channel: "email",
		})
	}

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
	if !isNotice && s.supportInboxService != nil && s.supportInboxService.triageService != nil {
		// Routing can assign a human. Complete it before reloading ownership for AI.
		if _, err := s.supportInboxService.triageService.EvaluateAndRoute(ctx, conversation.WorkspaceID, conversation.ID, message.ID); err != nil {
			s.logger.ErrorContext(ctx, "support triage failed for inbound email conversation", "error", err, "conversation_id", conversation.ID)
		}
	}

	return s.publishInboundCustomerAIRequest(ctx, conversation.WorkspaceID, conversation.ID, message)
}

func (s *EmailFallbackService) storeRouteConfirmationConversation(ctx context.Context, route *model.SupportEmailRoute, conversationID string) error {
	if s == nil || route == nil || s.supportInboxService == nil || s.supportInboxService.emailRouteRepo == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	route.ConfirmationConversationID = strPtr(conversationID)
	if err := s.supportInboxService.emailRouteRepo.Update(ctx, route); err != nil {
		return fmt.Errorf("store forwarding confirmation conversation: %w", err)
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

func inboundReplyToAddress(payload model.PostmarkInboundPayload) (raw, emailAddress, displayName string) {
	raw = strings.TrimSpace(payload.ReplyTo)
	if raw == "" {
		raw = strings.TrimSpace(inboundHeaderValue(payload.Headers, "Reply-To"))
	}
	if raw == "" {
		return "", "", ""
	}
	if addresses, err := mail.ParseAddressList(raw); err == nil && len(addresses) > 0 {
		return raw, strings.TrimSpace(addresses[0].Address), strings.TrimSpace(addresses[0].Name)
	}
	if addr, err := mail.ParseAddress(raw); err == nil {
		return raw, strings.TrimSpace(addr.Address), strings.TrimSpace(addr.Name)
	}
	return raw, "", ""
}

func mailboxHashFromInboundPayload(payload model.PostmarkInboundPayload) string {
	if mailboxHash := strings.TrimSpace(payload.MailboxHash); mailboxHash != "" {
		return mailboxHash
	}
	for _, candidate := range []string{
		payload.OriginalRecipient,
		payload.To,
		payload.Cc,
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
	for _, addr := range payload.CcFull {
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
	if strings.HasPrefix(local, "conv-") || strings.HasPrefix(local, "unsubscribe-") || strings.HasPrefix(local, "route-") || strings.HasPrefix(local, "verify-") {
		return local
	}
	return ""
}

func inboundRecipientAddress(payload model.PostmarkInboundPayload) string {
	recipients := inboundRecipientAddresses(payload)
	if len(recipients) == 0 {
		return ""
	}
	return recipients[0]
}

func inboundRecipientAddresses(payload model.PostmarkInboundPayload) []string {
	var candidates []string
	candidates = append(candidates, payload.OriginalRecipient, payload.To)
	for _, addr := range payload.ToFull {
		candidates = append(candidates, addr.Email)
	}
	candidates = append(candidates, payload.Cc)
	for _, addr := range payload.CcFull {
		candidates = append(candidates, addr.Email)
	}
	return normalizedEmailAddressList(candidates)
}

func inboundPayloadIncludesRecipient(payload model.PostmarkInboundPayload, email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	for _, recipient := range inboundRecipientAddresses(payload) {
		if strings.EqualFold(recipient, email) {
			return true
		}
	}
	return false
}

func mergeExternalEmailReplyMetadata(existing string) string {
	metadata := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		_ = json.Unmarshal([]byte(existing), &metadata)
	}
	metadata["external_email_reply"] = true
	metadata["external_email_capture"] = "support_email_copy"
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return existing
	}
	return string(encoded)
}

func inboundCCEmails(payload model.PostmarkInboundPayload) []string {
	values := []string{payload.Cc}
	for _, addr := range payload.CcFull {
		values = append(values, addr.Email)
	}
	return normalizedEmailAddressList(values)
}

func inboundBCCEmails(payload model.PostmarkInboundPayload) []string {
	values := []string{payload.Bcc}
	for _, addr := range payload.BccFull {
		values = append(values, addr.Email)
	}
	return normalizedEmailAddressList(values)
}

func inboundRouteAddressInCC(payload model.PostmarkInboundPayload, routeAddress string) bool {
	routeAddress = strings.ToLower(strings.TrimSpace(routeAddress))
	if routeAddress == "" {
		return false
	}
	for _, email := range inboundCCEmails(payload) {
		if strings.EqualFold(email, routeAddress) {
			return true
		}
	}
	return false
}

func inboundRouteAddressInTo(payload model.PostmarkInboundPayload, routeAddress string) bool {
	routeAddress = strings.ToLower(strings.TrimSpace(routeAddress))
	if routeAddress == "" {
		return false
	}
	values := []string{payload.To}
	for _, addr := range payload.ToFull {
		values = append(values, addr.Email)
	}
	for _, email := range normalizedEmailAddressList(values) {
		if strings.EqualFold(email, routeAddress) {
			return true
		}
	}
	return false
}

func inboundSuggestedPrimaryRecipient(payload model.PostmarkInboundPayload, routeAddress, currentPrimary string) (string, string) {
	routeAddress = strings.ToLower(strings.TrimSpace(routeAddress))
	currentPrimary = strings.ToLower(strings.TrimSpace(currentPrimary))
	for _, addr := range payload.ToFull {
		email := strings.ToLower(strings.TrimSpace(addr.Email))
		if email == "" || email == routeAddress || email == currentPrimary {
			continue
		}
		return email, strings.TrimSpace(addr.Name)
	}
	for _, email := range normalizedEmailAddressList([]string{payload.To}) {
		if email == "" || email == routeAddress || email == currentPrimary {
			continue
		}
		return email, ""
	}
	return "", ""
}

func inboundVisibleThreadParticipants(payload model.PostmarkInboundPayload, routeAddress, currentPrimary string) []string {
	routeAddress = strings.ToLower(strings.TrimSpace(routeAddress))
	currentPrimary = strings.ToLower(strings.TrimSpace(currentPrimary))
	values := []string{payload.To, payload.Cc}
	for _, addr := range payload.ToFull {
		values = append(values, addr.Email)
	}
	for _, addr := range payload.CcFull {
		values = append(values, addr.Email)
	}
	result := make([]string, 0, len(values))
	for _, email := range normalizedEmailAddressList(values) {
		if email == "" || email == routeAddress || email == currentPrimary {
			continue
		}
		result = append(result, email)
	}
	return result
}

func normalizedEmailAddressList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		var addresses []string
		if parsed, err := mail.ParseAddressList(value); err == nil {
			for _, addr := range parsed {
				addresses = append(addresses, addr.Address)
			}
		} else if addr, err := mail.ParseAddress(value); err == nil {
			addresses = append(addresses, addr.Address)
		} else {
			addresses = append(addresses, value)
		}
		for _, address := range addresses {
			email := strings.ToLower(strings.TrimSpace(address))
			if email == "" {
				continue
			}
			if _, exists := seen[email]; exists {
				continue
			}
			seen[email] = struct{}{}
			result = append(result, email)
		}
	}
	return result
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

// isEmailFallbackInboundTerminalStatus returns true only for statuses that
// should reject inbound customer email replies. Resolved conversations accept
// replies and reopen; only spam is truly terminal for inbound.
func isEmailFallbackInboundTerminalStatus(status string) bool {
	switch model.NormalizeSupportConversationStatus(status) {
	case model.SupportConversationStatusSpam:
		return true
	default:
		return false
	}
}

// supportEmailReopenFlowState computes the flow state a conversation should
// transition to when an inbound customer email reply makes it actionable again.
func supportEmailReopenFlowState(conv *model.SupportConversation) string {
	if conv == nil {
		return model.SupportConversationFlowStateWaitingForHuman
	}
	if conv.AssignedUserID != nil || conv.OpenedByUserID != nil || derefString(conv.FlowState) == model.SupportConversationFlowStateAssignedToHuman {
		return model.SupportConversationFlowStateAssignedToHuman
	}
	return model.SupportConversationFlowStateWaitingForHuman
}

// createEmailReopenedSystemMessage records an internal system event marking
// the conversation as reopened by an inbound email reply.
func createEmailReopenedSystemMessage(ctx context.Context, msgRepo *repository.SupportMessageRepository, conv *model.SupportConversation) error {
	if msgRepo == nil || conv == nil {
		return nil
	}
	sysMsg := &model.SupportMessage{
		WorkspaceID:     conv.WorkspaceID,
		ConversationID:  conv.ID,
		SenderType:      "user",
		Content:         "Customer reply reopened this conversation.",
		MessageType:     "system",
		SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventReopened),
		IsInternal:      true,
	}
	return msgRepo.Create(ctx, sysMsg)
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
