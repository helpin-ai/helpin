package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/netip"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/geoip"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/requestmeta"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// SupportInboxService contains support business logic.
type SupportInboxService struct {
	conversationRepo        *repository.SupportConversationRepository
	mailboxRepo             *repository.SupportMailboxRepository
	emailRouteRepo          *repository.SupportEmailRouteRepository
	emailSenderRepo         *repository.SupportEmailSenderRepository
	emailSenderDomainRepo   *repository.SupportEmailSenderDomainRepository
	messageRepo             *repository.SupportMessageRepository
	tagRepo                 *repository.SupportTagRepository
	agentRepo               *repository.AgentRepository
	assocRepo               *repository.CRMAssociationRepository
	installationRepo        *repository.SupportInboxInstallationRepository
	sessionRepo             *repository.SupportInboxSessionRepository
	cannedResponseRepo      *repository.SupportCannedResponseRepository
	activitySvc             *PMActivityService
	wsPublisher             *websocket.Publisher
	contactRepo             *repository.CRMContactRepository
	userRepo                *repository.UserRepository
	docsSpaceRepo           *repository.DocsSpaceRepository
	docsCollectionRepo      *repository.DocsCollectionRepository
	docsHelpcenterRepo      *repository.DocsHelpcenterRepository
	conversationAgentRunner func(ctx context.Context, workspaceID, conversationID string) (*model.AgentRun, error)
	supportAIService        *SupportAIService
	emailFallbackService    *EmailFallbackService
	notificationService     *NotificationService
	workspaceRepo           *repository.WorkspaceRepository
	authzService            *authorization.AuthzService
	attachmentService       *SupportAttachmentService
	linkPreviewService      SupportMessageLinkPreviewer
	presence                websocket.PresenceProvider
	statusOverrideRepo      *repository.SupportTeammateStatusOverrideRepository
	emailLogRepo            *repository.SupportEmailLogRepository
	postmarkDomainClient    *email.DomainClient
	triageService           *SupportInboxTriageService
	taskService             *PMTaskService
	geoIPResolver           geoip.Resolver
	supportEventRecorder    SupportEventRecorder
	routeDomain             string
}

type supportConversationTaskDraft struct {
	Title       string
	Summary     string
	Description string
	TaskType    string
	Priority    string
}

type supportTaskAssociationCopyCounts struct {
	Contacts  int
	Companies int
	Deals     int
}

var ErrSupportTaskInsufficientContext = errors.New("Not enough support context to create a useful task. Add more internal notes with the issue, impact, and expected outcome, then try again.")

// NewSupportInboxService creates a new SupportInboxService.
func NewSupportInboxService(
	conversationRepo *repository.SupportConversationRepository,
	mailboxRepo *repository.SupportMailboxRepository,
	messageRepo *repository.SupportMessageRepository,
	agentRepo *repository.AgentRepository,
	assocRepo *repository.CRMAssociationRepository,
	installationRepo *repository.SupportInboxInstallationRepository,
	sessionRepo *repository.SupportInboxSessionRepository,
	cannedResponseRepo *repository.SupportCannedResponseRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
	contactRepo *repository.CRMContactRepository,
	userRepo *repository.UserRepository,
	docsSpaceRepo *repository.DocsSpaceRepository,
	docsCollectionRepo *repository.DocsCollectionRepository,
	docsHelpcenterRepo *repository.DocsHelpcenterRepository,
) *SupportInboxService {
	return &SupportInboxService{
		conversationRepo:   conversationRepo,
		mailboxRepo:        mailboxRepo,
		messageRepo:        messageRepo,
		agentRepo:          agentRepo,
		assocRepo:          assocRepo,
		installationRepo:   installationRepo,
		sessionRepo:        sessionRepo,
		cannedResponseRepo: cannedResponseRepo,
		activitySvc:        activitySvc,
		wsPublisher:        wsPublisher,
		contactRepo:        contactRepo,
		userRepo:           userRepo,
		docsSpaceRepo:      docsSpaceRepo,
		docsCollectionRepo: docsCollectionRepo,
		docsHelpcenterRepo: docsHelpcenterRepo,
	}
}

func supportActorFromContext(ctx context.Context, workspaceID string) *authorization.Actor {
	actor := authorization.GetActor(ctx)
	if actor == nil || actor.WorkspaceID != workspaceID {
		return nil
	}
	return actor
}

func (s *SupportInboxService) SetRouteDomain(domain string) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.routeDomain = strings.TrimSpace(domain)
	return s
}

func (s *SupportInboxService) SetSupportTagRepo(repo *repository.SupportTagRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.tagRepo = repo
	return s
}

// SetEmailLogRepo wires the email log repository used by GetMessageEmailDetail.
func (s *SupportInboxService) SetEmailLogRepo(repo *repository.SupportEmailLogRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.emailLogRepo = repo
	return s
}

// GetMessageEmailDetail returns the email log tied to a support message,
// scoped to the workspace. Returns ErrRecordNotFound-style nil when the
// message does not exist, is not in this workspace, or has no email log.
func (s *SupportInboxService) GetMessageEmailDetail(ctx context.Context, workspaceID, messageID string) (*model.SupportMessageEmailDetail, error) {
	if s == nil || s.emailLogRepo == nil || s.messageRepo == nil {
		return nil, fmt.Errorf("support inbox service not configured for email detail")
	}
	if workspaceID == "" || messageID == "" {
		return nil, fmt.Errorf("workspace_id and message_id are required")
	}

	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("get support message: %w", err)
	}
	if msg == nil || msg.WorkspaceID != workspaceID {
		return nil, nil
	}

	log, err := s.emailLogRepo.GetByMessageID(ctx, workspaceID, messageID)
	if err != nil {
		return nil, err
	}
	if log == nil {
		return nil, nil
	}
	var metadata map[string]any
	if strings.TrimSpace(msg.Metadata) != "" {
		_ = json.Unmarshal([]byte(msg.Metadata), &metadata)
	}

	return &model.SupportMessageEmailDetail{
		ID:                   log.ID,
		MessageID:            messageID,
		Direction:            log.Direction,
		Subject:              log.Subject,
		FromEmail:            log.FromEmail,
		ToEmail:              log.ToEmail,
		RFCMessageID:         log.RFCMessageID,
		InReplyTo:            log.InReplyTo,
		ReferencesHeader:     log.ReferencesHeader,
		StrippedText:         log.StrippedText,
		HTMLBody:             log.HTMLBody,
		Status:               log.Status,
		DeliveredAt:          log.DeliveredAt,
		OpenedAt:             log.OpenedAt,
		BouncedAt:            log.BouncedAt,
		ErrorMessage:         log.ErrorMessage,
		CreatedAt:            log.CreatedAt,
		ForwardedAttribution: forwardedAttributionFromMetadata(metadata),
	}, nil
}

func (s *SupportInboxService) actorMailboxScope(ctx context.Context, workspaceID string) (workspaceMemberID, role string) {
	actor := supportActorFromContext(ctx, workspaceID)
	if actor == nil {
		return "", ""
	}
	return actor.WorkspaceMemberID, actor.Role
}

func renderWidgetArticleHTML(content json.RawMessage) *string {
	if len(content) == 0 {
		return nil
	}

	rendered, err := tiptap.RenderHTML(content)
	if err != nil || rendered == "" {
		return nil
	}

	return &rendered
}

func formatSupportTranscriptTimestamp(ts time.Time) string {
	return ts.UTC().Format("Jan 2, 2006 15:04 UTC")
}

// SetSupportEventRecorder injects the event recorder for coverage telemetry.
func (s *SupportInboxService) SetSupportEventRecorder(r SupportEventRecorder) {
	if s == nil {
		return
	}
	s.supportEventRecorder = r
}

func (s *SupportInboxService) recordSupportEvent(input SupportEventInput) {
	if s.supportEventRecorder == nil {
		return
	}
	s.supportEventRecorder.RecordEventBestEffort(input)
}

// firstCustomerMessageExcerpt returns a short excerpt of the first
// customer-authored message on a conversation, used as evidence for a
// coverage gap. Returns "" when the message can't be loaded — callers
// must handle an empty summary gracefully.
func (s *SupportInboxService) firstCustomerMessageExcerpt(ctx context.Context, workspaceID, conversationID string) string {
	if s.messageRepo == nil {
		return ""
	}
	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		return ""
	}
	for _, m := range messages {
		if m.SenderType != "customer" || m.IsInternal {
			continue
		}
		excerpt := strings.TrimSpace(m.Content)
		if excerpt == "" {
			continue
		}
		if len(excerpt) > 500 {
			excerpt = excerpt[:500]
		}
		return excerpt
	}
	return ""
}

func (s *SupportInboxService) SetGeoIPResolver(resolver geoip.Resolver) {
	s.geoIPResolver = resolver
}

func (s *SupportInboxService) widgetSessionGeoContext(ctx context.Context) (*string, *geoip.Result, error) {
	if s.geoIPResolver == nil {
		return nil, nil, nil
	}

	clientIP, ok := requestmeta.ClientIPFromContext(ctx)
	if !ok || !requestmeta.IsPublicIP(clientIP.Addr) {
		return nil, nil, nil
	}

	ipText := clientIP.Addr.String()
	result, err := s.geoIPResolver.Lookup(clientIP.Addr)
	if err != nil {
		return nil, nil, err
	}
	return &ipText, result, nil
}

func (s *SupportInboxService) refreshWidgetSessionGeo(ctx context.Context, session *model.SupportWidgetSession) {
	if s == nil || session == nil {
		return
	}

	clientIP, ok := requestmeta.ClientIPFromContext(ctx)
	if !ok || !requestmeta.IsPublicIP(clientIP.Addr) {
		return
	}

	ipText := clientIP.Addr.Unmap().String()
	changed := false
	if strings.TrimSpace(derefString(session.IPAddress)) != ipText {
		session.IPAddress = &ipText
		changed = true
	}

	if s.geoIPResolver != nil {
		result, err := s.geoIPResolver.Lookup(clientIP.Addr)
		if err != nil {
			slog.WarnContext(ctx, "widget session geoip refresh failed", "session_id", session.ID, "ip_address", ipText, "error", err)
		} else if result != nil {
			if next := stringPtrOrNil(result.CountryCode); derefString(session.CountryCode) != derefString(next) {
				session.CountryCode = next
				changed = true
			}
			if next := stringPtrOrNil(result.CountryName); derefString(session.CountryName) != derefString(next) {
				session.CountryName = next
				changed = true
			}
			if next := stringPtrOrNil(result.RegionName); derefString(session.RegionName) != derefString(next) {
				session.RegionName = next
				changed = true
			}
			if next := stringPtrOrNil(result.CityName); derefString(session.CityName) != derefString(next) {
				session.CityName = next
				changed = true
			}
		}
	}

	if !changed || s.sessionRepo == nil {
		return
	}
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		slog.WarnContext(ctx, "widget session geoip refresh save failed", "session_id", session.ID, "ip_address", ipText, "error", err)
	}
}

func (s *SupportInboxService) BackfillWidgetSessionGeo(ctx context.Context, limit int) (int, error) {
	if s == nil || s.geoIPResolver == nil || s.sessionRepo == nil {
		return 0, nil
	}

	sessions, err := s.sessionRepo.ListGeoBackfillCandidates(ctx, limit)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, session := range sessions {
		ipText := strings.TrimSpace(derefString(session.IPAddress))
		if ipText == "" {
			continue
		}

		addr, err := netip.ParseAddr(ipText)
		if err != nil {
			slog.WarnContext(ctx, "skip widget geoip backfill for invalid ip", "session_id", session.ID, "ip_address", ipText, "error", err)
			continue
		}
		addr = addr.Unmap()
		if !requestmeta.IsPublicIP(addr) {
			continue
		}

		result, err := s.geoIPResolver.Lookup(addr)
		if err != nil {
			slog.WarnContext(ctx, "widget geoip backfill lookup failed", "session_id", session.ID, "ip_address", ipText, "error", err)
			continue
		}
		if result == nil {
			continue
		}

		if err := s.sessionRepo.UpdateGeoLocation(
			ctx,
			session.ID,
			stringPtrOrNil(result.CountryCode),
			stringPtrOrNil(result.CountryName),
			stringPtrOrNil(result.RegionName),
			stringPtrOrNil(result.CityName),
		); err != nil {
			return updated, err
		}
		updated++
	}

	return updated, nil
}

func (s *SupportInboxService) ListConversationAssignableUsers(ctx context.Context, workspaceID, conversationID string) ([]model.AssignableMember, error) {
	if s.workspaceRepo == nil {
		return nil, fmt.Errorf("workspace repository unavailable")
	}

	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	if s.authzService == nil {
		return s.workspaceRepo.ListSupportAssignableMembers(ctx, workspaceID, nil)
	}

	members, err := s.workspaceRepo.ListAssignableMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	assignable := make([]model.AssignableMember, 0, len(members))
	for _, member := range members {
		if member.UserID == nil || strings.TrimSpace(*member.UserID) == "" {
			continue
		}
		if member.Status != model.WorkspaceMemberStatusActive {
			continue
		}
		if s.userHasSupportModuleAccess(ctx, workspaceID, strings.TrimSpace(*member.UserID)) {
			assignable = append(assignable, member)
		}
	}

	return assignable, nil
}

func (s *SupportInboxService) userHasSupportModuleAccess(ctx context.Context, workspaceID, userID string) bool {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return false
	}

	if s.authzService != nil {
		actor, err := s.authzService.ResolveActor(ctx, workspaceID, trimmedUserID)
		if err == nil && actor != nil {
			allowed, err := s.authzService.CanAccessModule(ctx, actor, model.ModuleSupport)
			return err == nil && allowed
		}
	}

	if s.workspaceRepo == nil {
		return false
	}

	members, err := s.workspaceRepo.ListSupportAssignableMembers(ctx, workspaceID, nil)
	if err != nil {
		return false
	}
	for _, member := range members {
		if member.UserID != nil && strings.TrimSpace(*member.UserID) == trimmedUserID {
			return true
		}
	}
	return false
}

func (s *SupportInboxService) isConversationAssignableUser(ctx context.Context, workspaceID string, mailboxID *string, userID string) bool {
	return s.userHasSupportModuleAccess(ctx, workspaceID, userID)
}

func supportTranscriptSenderName(workspaceName string, msg model.SupportMessage) string {
	switch msg.SenderType {
	case "customer":
		if strings.TrimSpace(derefString(msg.SenderDisplayName)) != "" {
			return strings.TrimSpace(derefString(msg.SenderDisplayName))
		}
		return "You"
	case "ai":
		return "Helpin AI"
	default:
		if strings.TrimSpace(derefString(msg.SenderDisplayName)) != "" {
			return strings.TrimSpace(derefString(msg.SenderDisplayName))
		}
		if strings.TrimSpace(workspaceName) != "" {
			return workspaceName
		}
		return "Support"
	}
}

func renderSupportTranscriptBodies(workspaceName string, conversation *model.SupportConversation, messages []model.SupportMessage) (string, string) {
	subject := strings.TrimSpace(conversation.Subject)
	if subject == "" {
		subject = "Conversation transcript"
	}

	var text strings.Builder
	text.WriteString(workspaceName)
	text.WriteString(" conversation transcript\n")
	text.WriteString("Conversation #")
	text.WriteString(fmt.Sprintf("%d", conversation.DisplayID))
	text.WriteString("\n")
	text.WriteString("Subject: ")
	text.WriteString(subject)
	text.WriteString("\n\n")

	var htmlBody strings.Builder
	htmlBody.WriteString("<p>")
	htmlBody.WriteString(html.EscapeString(workspaceName))
	htmlBody.WriteString(" conversation transcript</p>")
	htmlBody.WriteString("<p><strong>Conversation #")
	htmlBody.WriteString(fmt.Sprintf("%d", conversation.DisplayID))
	htmlBody.WriteString("</strong><br />Subject: ")
	htmlBody.WriteString(html.EscapeString(subject))
	htmlBody.WriteString("</p>")

	for _, msg := range messages {
		sender := supportTranscriptSenderName(workspaceName, msg)
		timestamp := formatSupportTranscriptTimestamp(msg.CreatedAt)
		content := strings.TrimSpace(msg.Content)

		text.WriteString(sender)
		text.WriteString(" — ")
		text.WriteString(timestamp)
		text.WriteString("\n")
		if content != "" {
			text.WriteString(content)
			text.WriteString("\n")
		}
		for _, attachment := range msg.Attachments {
			text.WriteString("[Attachment: ")
			text.WriteString(strings.TrimSpace(attachment.FileName))
			text.WriteString("]\n")
		}
		text.WriteString("\n")

		htmlBody.WriteString("<p><strong>")
		htmlBody.WriteString(html.EscapeString(sender))
		htmlBody.WriteString("</strong> <span style=\"color:#6b7280\">")
		htmlBody.WriteString(html.EscapeString(timestamp))
		htmlBody.WriteString("</span><br />")
		if content != "" {
			htmlBody.WriteString(strings.ReplaceAll(html.EscapeString(content), "\n", "<br />"))
			if len(msg.Attachments) > 0 {
				htmlBody.WriteString("<br />")
			}
		}
		for i, attachment := range msg.Attachments {
			if i > 0 {
				htmlBody.WriteString("<br />")
			}
			htmlBody.WriteString("[Attachment: ")
			htmlBody.WriteString(html.EscapeString(strings.TrimSpace(attachment.FileName)))
			htmlBody.WriteString("]")
		}
		htmlBody.WriteString("</p>")
	}

	return htmlBody.String(), text.String()
}

// SetSupportAIService injects the AI-first auto-reply service.
func (s *SupportInboxService) SetSupportAIService(aiService *SupportAIService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.supportAIService = aiService
	return s
}

// SetEmailFallbackService injects the email fallback service used for offline visitor replies.
func (s *SupportInboxService) SetEmailFallbackService(emailFallbackService *EmailFallbackService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.emailFallbackService = emailFallbackService
	return s
}

// SetEmailRouteRepository injects the support email route repository.
func (s *SupportInboxService) SetEmailRouteRepository(emailRouteRepo *repository.SupportEmailRouteRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.emailRouteRepo = emailRouteRepo
	return s
}

// SetEmailSenderDomainRepository injects the custom support sender domain repository.
func (s *SupportInboxService) SetEmailSenderDomainRepository(repo *repository.SupportEmailSenderDomainRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.emailSenderDomainRepo = repo
	return s
}

// SetEmailSenderRepository injects the custom support sender-address repository.
func (s *SupportInboxService) SetEmailSenderRepository(repo *repository.SupportEmailSenderRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.emailSenderRepo = repo
	return s
}

// SetPostmarkDomainClient injects the Postmark Account API client for sender domains.
func (s *SupportInboxService) SetPostmarkDomainClient(client *email.DomainClient) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.postmarkDomainClient = client
	return s
}

// SetNotificationService injects the notification service and workspace repo for @mention support.
func (s *SupportInboxService) SetNotificationService(ns *NotificationService, wr *repository.WorkspaceRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.notificationService = ns
	s.workspaceRepo = wr
	return s
}

// SetAttachmentService injects the support attachment service for file upload support.
func (s *SupportInboxService) SetAttachmentService(attachmentService *SupportAttachmentService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.attachmentService = attachmentService
	return s
}

// SetLinkPreviewService injects the support message link preview enricher.
func (s *SupportInboxService) SetLinkPreviewService(linkPreviewService SupportMessageLinkPreviewer) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.linkPreviewService = linkPreviewService
	return s
}

// SetWorkspaceRepo injects the workspace repo for member/status lookups.
func (s *SupportInboxService) SetWorkspaceRepo(workspaceRepo *repository.WorkspaceRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.workspaceRepo = workspaceRepo
	return s
}

// SetAuthzService injects the canonical authorization service for module access checks.
func (s *SupportInboxService) SetAuthzService(authzService *authorization.AuthzService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.authzService = authzService
	return s
}

// SetPresenceProvider injects the live support presence provider.
func (s *SupportInboxService) SetPresenceProvider(presence websocket.PresenceProvider) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.presence = presence
	return s
}

// SetStatusOverrideRepo injects the manual teammate status override repository.
func (s *SupportInboxService) SetStatusOverrideRepo(repo *repository.SupportTeammateStatusOverrideRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.statusOverrideRepo = repo
	return s
}

// SetTriageService injects the support conversation triage service.
func (s *SupportInboxService) SetTriageService(triageService *SupportInboxTriageService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.triageService = triageService
	return s
}

// SetConversationAgentRunner injects the agent-run startup hook used for widget auto-replies.
func (s *SupportInboxService) SetConversationAgentRunner(runner func(ctx context.Context, workspaceID, conversationID string) (*model.AgentRun, error)) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.conversationAgentRunner = runner
	return s
}

// SetTaskService injects the PM task service used for support-created tasks.
func (s *SupportInboxService) SetTaskService(taskService *PMTaskService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.taskService = taskService
	return s
}

func (s *SupportInboxService) ListTriageRules(ctx context.Context, workspaceID string) ([]model.SupportTriageRule, error) {
	if s.triageService == nil {
		return []model.SupportTriageRule{}, nil
	}
	return s.triageService.ListRules(ctx, workspaceID)
}

func (s *SupportInboxService) CreateTriageRule(ctx context.Context, workspaceID, actorID string, req model.CreateSupportTriageRuleRequest) (*model.SupportTriageRule, error) {
	if s.triageService == nil {
		return nil, fmt.Errorf("support triage is unavailable")
	}
	return s.triageService.CreateRule(ctx, workspaceID, actorID, req)
}

func (s *SupportInboxService) UpdateTriageRule(ctx context.Context, workspaceID, ruleID string, req model.UpdateSupportTriageRuleRequest) (*model.SupportTriageRule, error) {
	if s.triageService == nil {
		return nil, fmt.Errorf("support triage is unavailable")
	}
	return s.triageService.UpdateRule(ctx, workspaceID, ruleID, req)
}

func (s *SupportInboxService) DeleteTriageRule(ctx context.Context, workspaceID, ruleID string) error {
	if s.triageService == nil {
		return fmt.Errorf("support triage is unavailable")
	}
	return s.triageService.DeleteRule(ctx, workspaceID, ruleID)
}

func (s *SupportInboxService) DismissConversationTriage(ctx context.Context, workspaceID, conversationID, actorID string) (*model.SupportConversationTriage, error) {
	if s.triageService == nil {
		return nil, fmt.Errorf("support triage is unavailable")
	}
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	return s.triageService.DismissConversationTriage(ctx, workspaceID, conversationID, actorID)
}

// ListConversations returns conversations with optional filters.
func (s *SupportInboxService) ListConversations(ctx context.Context, workspaceID, status, priority string, pagination model.PMPagination, search string) ([]model.SupportConversation, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	status = model.NormalizeSupportConversationStatus(status)
	workspaceMemberID, role := s.actorMailboxScope(ctx, workspaceID)
	return s.conversationRepo.List(ctx, repository.ConversationRepositoryListParams{
		ConversationListParams: repository.ConversationListParams{
			WorkspaceID: workspaceID,
			Status:      status,
			Priority:    priority,
			Pagination:  pagination,
			Search:      search,
		},
		WorkspaceMemberID: workspaceMemberID,
		Role:              role,
	})
}

type SupportConversationListParams = repository.ConversationListParams

func normalizeSupportConversationStatuses(statuses []string) []string {
	normalized := make([]string, 0, len(statuses))
	seen := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		value := model.NormalizeSupportConversationStatus(status)
		if value == "" || !model.IsValidSupportConversationStatus(value) || seen[value] {
			continue
		}
		normalized = append(normalized, value)
		seen[value] = true
	}
	return normalized
}

// ListConversationsWithMeta returns conversations plus aggregate unread stats.
func (s *SupportInboxService) ListConversationsWithMeta(ctx context.Context, params SupportConversationListParams) (*model.ConversationListResponse, error) {
	if params.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	params.Status = model.NormalizeSupportConversationStatus(params.Status)
	params.Statuses = normalizeSupportConversationStatuses(params.Statuses)
	if err := s.requireMailboxAccess(ctx, params.WorkspaceID, params.MailboxID); err != nil {
		return nil, err
	}
	for _, mailboxID := range params.MailboxIDs {
		trimmed := strings.TrimSpace(mailboxID)
		if trimmed == "" || trimmed == "shared" {
			continue
		}
		if err := s.requireMailboxAccess(ctx, params.WorkspaceID, &trimmed); err != nil {
			return nil, err
		}
	}
	workspaceMemberID, role := s.actorMailboxScope(ctx, params.WorkspaceID)
	conversations, total, err := s.conversationRepo.List(ctx, repository.ConversationRepositoryListParams{
		ConversationListParams: params,
		WorkspaceMemberID:      workspaceMemberID,
		Role:                   role,
	})
	if err != nil {
		return nil, err
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	if s.triageService != nil {
		if err := s.triageService.HydrateConversations(ctx, conversations); err != nil {
			slog.ErrorContext(ctx, "hydrate support conversation triage list", "error", err, "workspace_id", params.WorkspaceID)
		}
	}
	s.hydrateConversationTags(ctx, params.WorkspaceID, conversations)

	stats, err := s.conversationRepo.GetUnreadStats(ctx, params.WorkspaceID, params.UserID, workspaceMemberID, role, params.MailboxID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get unread stats", "error", err, "workspace_id", params.WorkspaceID)
		// Non-fatal: return conversations with zero stats
	}

	perPage := params.Pagination.PerPage
	if perPage <= 0 {
		perPage = 50
	}
	page := params.Pagination.Page
	if page <= 0 {
		page = 1
	}
	totalPages := 0
	if perPage > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}

	return &model.ConversationListResponse{
		Data:       conversations,
		Total:      int(total),
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		Meta: model.ConversationListMeta{
			Unread: stats,
		},
	}, nil
}

func (s *SupportInboxService) hydrateConversationTags(ctx context.Context, workspaceID string, conversations []model.SupportConversation) {
	if len(conversations) == 0 {
		return
	}
	ids := make([]string, 0, len(conversations))
	for i := range conversations {
		conversations[i].SystemTags = supportConversationSystemTags(conversations[i])
		ids = append(ids, conversations[i].ID)
	}
	if s.tagRepo == nil {
		return
	}
	tagsByConversation, err := s.tagRepo.ListByConversationIDs(ctx, workspaceID, ids)
	if err != nil {
		slog.ErrorContext(ctx, "hydrate support conversation tags", "error", err, "workspace_id", workspaceID)
		return
	}
	for i := range conversations {
		conversations[i].Tags = tagsByConversation[conversations[i].ID]
	}
}

func supportConversationSystemTags(conversation model.SupportConversation) []string {
	tags := make([]string, 0, 2)
	if supportConversationHasAIHandoff(conversation) {
		tags = append(tags, model.SupportSystemTagAIHandoff)
	}
	if conversationResolvedByAI(conversation) {
		tags = append(tags, model.SupportSystemTagAIResolved)
	}
	return tags
}

func supportConversationHasAIHandoff(conversation model.SupportConversation) bool {
	aiInvolved := conversation.AIState != nil || conversation.AITurnCount > 0
	if conversation.AIState != nil && *conversation.AIState == "escalated" {
		return true
	}
	if conversation.AIEscalatedAt != nil {
		return true
	}
	if conversation.CustomerRequestedHumanAt != nil && aiInvolved {
		return true
	}
	if conversation.FlowState == nil || !aiInvolved {
		return false
	}
	switch *conversation.FlowState {
	case model.SupportConversationFlowStateWaitingForHuman,
		model.SupportConversationFlowStateQueuedForHuman,
		model.SupportConversationFlowStateAfterHoursQueue:
		return true
	default:
		return false
	}
}

func conversationResolvedByAI(conversation model.SupportConversation) bool {
	if conversation.HumanTakeover != nil && *conversation.HumanTakeover {
		return false
	}
	if conversation.FlowState != nil && *conversation.FlowState == model.SupportConversationFlowStateResolvedByAI {
		return true
	}
	return conversation.FlowState == nil && conversation.AIState != nil && *conversation.AIState == "resolved"
}

// GetUnreadStats returns aggregate unread conversation counts for sidebar badges.
func (s *SupportInboxService) GetUnreadStats(ctx context.Context, workspaceID, userID string, mailboxID *string) (model.UnreadStats, error) {
	if err := s.requireMailboxAccess(ctx, workspaceID, mailboxID); err != nil {
		return model.UnreadStats{}, err
	}
	workspaceMemberID, role := s.actorMailboxScope(ctx, workspaceID)
	return s.conversationRepo.GetUnreadStats(ctx, workspaceID, userID, workspaceMemberID, role, mailboxID)
}

// MarkConversationRead updates the team read cursor and broadcasts a read event.
func (s *SupportInboxService) MarkConversationRead(ctx context.Context, workspaceID, conversationID, userID string) error {
	conv, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.MarkInternalRead(ctx, conversationID); err != nil {
		return err
	}
	if s.notificationService != nil {
		recipients := []string{userID}
		if conv.OpenedByUserID != nil && strings.TrimSpace(*conv.OpenedByUserID) != "" && *conv.OpenedByUserID != userID {
			recipients = append(recipients, *conv.OpenedByUserID)
		}
		for _, recipientID := range recipients {
			if err := s.notificationService.MarkEntityCategoryAsRead(ctx, recipientID, workspaceID, "support_conversation", conversationID, model.NotifCategorySupportReplies); err != nil {
				slog.ErrorContext(ctx, "mark support reply notifications read", "error", err, "conversation_id", conversationID, "user_id", recipientID)
			}
		}
	}

	// Broadcast read event so other tabs/users can invalidate
	reasonJSON, _ := json.Marshal(map[string]string{"reason": "read"})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     userID,
		Data:        reasonJSON,
	})
	return nil
}

// MarkConversationReadByVisitor updates the contact read cursor and broadcasts a list refresh.
func (s *SupportInboxService) MarkConversationReadByVisitor(ctx context.Context, workspaceID, conversationID, anonymousID string) error {
	conv, err := s.loadConversationUnscoped(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}
	if conv.AnonymousID == nil || *conv.AnonymousID != anonymousID {
		return fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.MarkContactRead(ctx, conversationID); err != nil {
		return err
	}

	// Push authoritative conversations:listed refresh to all visitor widget sessions
	s.pushVisitorConversationsRefresh(ctx, workspaceID, anonymousID)

	// Broadcast to agent dashboard so read receipts update in real-time
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
	})

	return nil
}

// EscalateConversation delegates to the AI service to escalate a conversation to a human agent.
func (s *SupportInboxService) EscalateConversation(ctx context.Context, workspaceID, conversationID, reason string) error {
	if s.supportAIService == nil {
		return fmt.Errorf("ai service not configured")
	}
	return s.supportAIService.EscalateToHuman(ctx, workspaceID, conversationID, reason)
}

// pushVisitorConversationsRefresh sends an updated conversation list to all widget sessions for a visitor.
func (s *SupportInboxService) pushVisitorConversationsRefresh(ctx context.Context, workspaceID, anonymousID string) {
	conversations, err := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, anonymousID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch visitor conversations for refresh", "error", err)
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	listJSON, _ := json.Marshal(map[string]any{"conversations": conversations})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_visitor_conversations",
		EntityID:    anonymousID,
		WorkspaceID: workspaceID,
		Data:        listJSON,
	})
}

// MarkConversationUnread resets the team read cursor so the conversation appears unread.
func (s *SupportInboxService) MarkConversationUnread(ctx context.Context, workspaceID, conversationID, userID string) error {
	conv, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.MarkUnread(ctx, conversationID); err != nil {
		return err
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     userID,
	})
	return nil
}

// UpdateConversationSubject changes the conversation subject.
func (s *SupportInboxService) UpdateConversationSubject(ctx context.Context, workspaceID, conversationID, subject, actorID string) (*model.SupportConversation, error) {
	if strings.TrimSpace(subject) == "" {
		return nil, fmt.Errorf("subject is required")
	}

	conv, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.UpdateSubject(ctx, conversationID, strings.TrimSpace(subject)); err != nil {
		return nil, err
	}
	conv.Subject = strings.TrimSpace(subject)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return conv, nil
}

// DeleteConversation permanently deletes a conversation and its messages.
func (s *SupportInboxService) DeleteConversation(ctx context.Context, workspaceID, conversationID, actorID string) error {
	conv, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.Delete(ctx, workspaceID, conversationID); err != nil {
		return err
	}

	slog.InfoContext(ctx, "conversation deleted", "conversation_id", conversationID, "workspace_id", workspaceID, "actor_id", actorID)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "deleted",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})
	return nil
}

// GetConversation returns a single conversation.
func (s *SupportInboxService) GetConversation(ctx context.Context, workspaceID, id string) (*model.SupportConversation, error) {
	ticket, err := s.loadConversationAccessible(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}
	if s.triageService != nil {
		if err := s.triageService.HydrateConversation(ctx, ticket); err != nil {
			slog.ErrorContext(ctx, "hydrate support conversation triage", "error", err, "workspace_id", workspaceID, "conversation_id", id)
		}
	}
	hydrated := []model.SupportConversation{*ticket}
	s.hydrateConversationTags(ctx, workspaceID, hydrated)
	ticket.Tags = hydrated[0].Tags
	ticket.SystemTags = hydrated[0].SystemTags
	return ticket, nil
}

// CreateConversation creates a new support conversation.
func (s *SupportInboxService) CreateConversation(ctx context.Context, req model.CreateConversationRequest, actorID string) (*model.SupportConversation, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Subject) == "" {
		return nil, fmt.Errorf("workspace_id and subject are required")
	}

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}
	source := req.Source
	if source == "" {
		source = "internal"
	}

	ticket := &model.SupportConversation{
		WorkspaceID:    req.WorkspaceID,
		MailboxID:      req.MailboxID,
		Subject:        strings.TrimSpace(req.Subject),
		Status:         "open",
		FlowState:      strPtr(defaultConversationFlowState(&actorID, &actorID, nil)),
		Priority:       priority,
		Channel:        source,
		CustomerName:   req.CustomerName,
		CustomerEmail:  req.CustomerEmail,
		OpenedByUserID: &actorID,
		AssignedUserID: &actorID,
		Source:         source,
	}

	mailboxID, mailbox, err := s.maybeApplyMailboxRouting(ctx, req.WorkspaceID, req.MailboxID, false)
	if err != nil {
		return nil, err
	}
	if err := s.requireMailboxAccess(ctx, req.WorkspaceID, mailboxID); err != nil {
		return nil, err
	}
	ticket.MailboxID = mailboxID
	if mailbox != nil {
		ownerID, flowState, ownerErr := s.determineMailboxOwner(ctx, req.WorkspaceID, mailbox, ticket.AssignedUserID)
		if ownerErr != nil {
			return nil, ownerErr
		}
		ticket.AssignedUserID = ownerID
		ticket.FlowState = strPtr(flowState)
	}

	if err := s.conversationRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}

	// Auto-match or create CRM contact by email.
	if contactID := s.matchOrCreateCRMContact(ctx, ticket.WorkspaceID, ticket.CustomerEmail, ticket.CustomerName); contactID != nil {
		ticket.CRMContactID = contactID
		if err := s.conversationRepo.Update(ctx, ticket); err != nil {
			slog.ErrorContext(ctx, "failed to link CRM contact to ticket", "error", err, "ticket_id", ticket.ID)
		}
	}

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, ticket.WorkspaceID, "support_conversation", ticket.ID, &actorID, "created", nil, nil, &ticket.Subject, nil)
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_conversation",
		EntityID:    ticket.ID,
		WorkspaceID: ticket.WorkspaceID,
		ActorID:     actorID,
	})
	if s.triageService != nil {
		if err := s.triageService.HydrateConversation(ctx, ticket); err != nil {
			slog.ErrorContext(ctx, "hydrate created support conversation triage", "error", err, "workspace_id", ticket.WorkspaceID, "conversation_id", ticket.ID)
		}
	}

	return ticket, nil
}

// validConversationStatuses defines allowed status transitions.
var validConversationStatuses = map[string]bool{
	model.SupportConversationStatusOpen:              true,
	model.SupportConversationStatusWaitingOnCustomer: true,
	model.SupportConversationStatusResolved:          true,
	model.SupportConversationStatusSpam:              true,
}

// UpdateConversationStatus changes conversation status.
func (s *SupportInboxService) UpdateConversationStatus(ctx context.Context, workspaceID, ticketID, status, actorID string) (*model.SupportConversation, error) {
	status = model.NormalizeSupportConversationStatus(status)
	if !validConversationStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	ticket, err := s.loadConversationAccessible(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}

	oldStatus := ticket.Status
	ticket.Status = status

	now := time.Now()
	switch status {
	case model.SupportConversationStatusResolved:
		ticket.ResolvedAt = &now
		if ticket.FlowState == nil || *ticket.FlowState != model.SupportConversationFlowStateResolvedByAI {
			ticket.FlowState = strPtr(model.SupportConversationFlowStateResolvedByHuman)
		}
	case model.SupportConversationStatusOpen:
		if ticket.HumanTakeover != nil && *ticket.HumanTakeover {
			ticket.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
		} else {
			ticket.FlowState = strPtr(defaultConversationFlowState(ticket.OpenedByUserID, ticket.AssignedUserID, ticket.AssignedAgentID))
		}
	case model.SupportConversationStatusSpam:
		ticket.ClosedAt = &now
	}

	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return nil, err
	}

	if status == model.SupportConversationStatusResolved {
		input := SupportEventInput{
			WorkspaceID:    workspaceID,
			EventType:      model.SupportEventConversationResolved,
			ConversationID: &ticketID,
			ActorType:      model.SupportEventActorAgent,
			Channel:        "inbox",
		}
		// Tag as a coverage gap signal only when AI engaged but a human
		// finished the conversation. This mirrors Intercom Fin's model:
		// assumed/confirmed AI resolutions are not treated as gaps; only
		// conversations a human had to step into are flagged for review.
		if ticket.FlowState != nil &&
			*ticket.FlowState == model.SupportConversationFlowStateResolvedByHuman &&
			ticket.AITurnCount > 0 {
			input.SourceSignal = model.SupportCoverageSourceConversationResolvedByHuman
			input.IssueSummary = s.firstCustomerMessageExcerpt(ctx, workspaceID, ticketID)
		}
		s.recordSupportEvent(input)
	}

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", ticketID, &actorID, "updated", strPtr("status"), &oldStatus, &status, nil)
	}

	// Insert a system message for admin status transitions. These stay internal
	// so the widget does not surface resolved/reopened thread notices.
	if oldStatus != status && (status == model.SupportConversationStatusResolved || (oldStatus == model.SupportConversationStatusResolved && status == model.SupportConversationStatusOpen)) {
		label := "Resolved conversation"
		if status == model.SupportConversationStatusOpen && oldStatus == model.SupportConversationStatusResolved {
			label = "Reopened conversation"
		}

		// Resolve actor display name and avatar.
		var senderDisplayName *string
		var senderAvatarURL *string
		if actorID != "" && s.userRepo != nil {
			user, _ := s.userRepo.GetByID(ctx, actorID)
			if user != nil {
				senderDisplayName = &user.FullName
				senderAvatarURL = user.AvatarURL
			}
		}
		senderUserID := &actorID

		eventType := model.SystemEventResolved
		if status == model.SupportConversationStatusOpen && oldStatus == model.SupportConversationStatusResolved {
			eventType = model.SystemEventReopened
		}
		sysMsg := &model.SupportMessage{
			WorkspaceID:       workspaceID,
			ConversationID:    ticketID,
			SenderType:        "user",
			SenderUserID:      senderUserID,
			SenderDisplayName: senderDisplayName,
			SenderAvatarURL:   senderAvatarURL,
			Content:           label,
			MessageType:       "system",
			SystemEventType:   model.SupportSystemEventTypeStrPtr(eventType),
			IsInternal:        true,
		}
		if err := s.messageRepo.Create(ctx, sysMsg); err != nil {
			slog.ErrorContext(ctx, "create system message for status change", "error", err, "conversation_id", ticketID)
		} else {
			s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, sysMsg, actorID))
		}
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    ticketID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return ticket, nil
}

// ListConversationMessages returns messages for a conversation.
func (s *SupportInboxService) ListConversationMessages(ctx context.Context, workspaceID, ticketID string, includeInternal bool) ([]model.SupportMessage, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	conv, err := s.loadConversationAccessible(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, ticketID, includeInternal)
	if err != nil {
		return nil, err
	}

	// Hydrate file attachments onto messages.
	if s.attachmentService != nil && len(messages) > 0 {
		if err := s.attachmentService.HydrateMessages(ctx, messages); err != nil {
			slog.ErrorContext(ctx, "hydrate support message attachments", "error", err, "conversation_id", ticketID)
		}
	}

	// Hydrate inbound email bodies onto messages so the thread bubbles can
	// render rich HTML without a per-message round-trip. Only email messages
	// need this — widget/in-app chat messages have no email log.
	if s.emailLogRepo != nil {
		hydrateEmailBodies(ctx, s.emailLogRepo, workspaceID, ticketID, messages)
	}

	return messages, nil
}

// hydrateEmailBodies joins support_email_logs onto messages by message_ids.
// For inbound email messages (ViaChannel == "email") it populates HTMLBody +
// StrippedText. For outbound messages (agent replies sent via the email
// fallback), it surfaces EmailDeliveryStatus + EmailDeliveryError so the UI
// can render delivery/bounce indicators. Safe to call with a nil repo.
func hydrateEmailBodies(
	ctx context.Context,
	repo supportEmailLogReader,
	workspaceID, conversationID string,
	messages []model.SupportMessage,
) {
	if repo == nil || len(messages) == 0 {
		return
	}

	logs, err := repo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		slog.ErrorContext(ctx, "hydrate support email bodies",
			"error", err, "conversation_id", conversationID)
		return
	}
	if len(logs) == 0 {
		return
	}

	byMessageID := make(map[string]*model.SupportEmailLog, len(logs))
	for i := range logs {
		for _, mid := range logs[i].MessageIDs {
			if mid == "" {
				continue
			}
			// Prefer the first log encountered per message_id. Messages are
			// usually 1:1 with logs, but MessageIDs can list several.
			if _, exists := byMessageID[mid]; !exists {
				byMessageID[mid] = &logs[i]
			}
		}
	}

	for i := range messages {
		log := byMessageID[messages[i].ID]
		if log == nil {
			continue
		}
		if messages[i].ViaChannel != nil && *messages[i].ViaChannel == "email" {
			messages[i].HTMLBody = log.HTMLBody
			messages[i].StrippedText = log.StrippedText
		}
		if log.Direction == "outbound" {
			messages[i].EmailDeliveryStatus = log.Status
			messages[i].EmailDeliveryError = log.ErrorMessage
		}
	}
}

// supportEmailLogReader is the subset of SupportEmailLogRepository needed
// by hydrateEmailBodies. Defined at the consumer so the helper can be
// unit-tested without a real repo.
type supportEmailLogReader interface {
	ListByConversation(ctx context.Context, workspaceID, conversationID string) ([]model.SupportEmailLog, error)
}

// CreateConversationMessage creates a message on a conversation.
func (s *SupportInboxService) CreateConversationMessage(ctx context.Context, workspaceID, ticketID string, req model.CreateMessageRequest, senderType string, senderUserID, senderAgentID *string, senderDisplayName *string) (*model.SupportMessage, error) {
	if strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0 {
		return nil, fmt.Errorf("content is required")
	}
	conv, err := s.loadConversationAccessible(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	messageType := req.MessageType
	if messageType == "" {
		messageType = "reply"
	}

	// Auto-resolve sender display name and avatar from user record.
	var senderAvatarURL *string
	if senderUserID != nil && s.userRepo != nil {
		user, _ := s.userRepo.GetByID(ctx, *senderUserID)
		if user != nil {
			if senderDisplayName == nil {
				senderDisplayName = &user.FullName
			}
			senderAvatarURL = user.AvatarURL
		}
	}

	// Pre-resolve teammate @mentions for support messages.
	var mentionedUserIDs []string
	if senderType == "user" && s.notificationService != nil && s.workspaceRepo != nil {
		ids, err := resolveMentionRecipients(ctx, s.workspaceRepo, workspaceID, strings.TrimSpace(req.Content), derefString(senderUserID), nil)
		if err != nil {
			slog.ErrorContext(ctx, "resolve support mentions", "error", err, "conversation_id", ticketID)
		}
		if len(ids) > 0 {
			filtered := make([]string, 0, len(ids))
			for _, mentionedID := range ids {
				// Mailbox membership does not imply support module access — legacy mailboxes
				// may include users (e.g. marketing team) who should not get support notifications.
				if s.userCanAccessMailbox(ctx, workspaceID, conv.MailboxID, mentionedID) &&
					s.userHasSupportModuleAccess(ctx, workspaceID, mentionedID) {
					filtered = append(filtered, mentionedID)
				}
			}
			ids = filtered
		}
		mentionedUserIDs = ids
	}

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    ticketID,
		SenderType:        senderType,
		SenderUserID:      senderUserID,
		SenderAgentID:     senderAgentID,
		SenderDisplayName: senderDisplayName,
		SenderAvatarURL:   senderAvatarURL,
		Content:           strings.TrimSpace(req.Content),
		IsInternal:        req.IsInternal,
		MessageType:       messageType,
	}

	if len(mentionedUserIDs) > 0 {
		metaJSON, _ := json.Marshal(map[string]any{"mentioned_user_ids": mentionedUserIDs})
		msg.Metadata = string(metaJSON)
	}
	if s.linkPreviewService != nil {
		s.linkPreviewService.EnrichMessage(ctx, msg)
	}

	// Before persisting a teammate's first public reply, emit a widget-visible
	// "{name} joined the conversation" system message so the customer sees a
	// centered pill immediately ahead of the reply — Intercom's pattern.
	if senderType == "user" && !req.IsInternal && messageType == "reply" && senderUserID != nil {
		s.emitTeammateJoinedIfFirstReply(ctx, workspaceID, ticketID, strings.TrimSpace(*senderUserID), derefString(senderDisplayName), senderAvatarURL)
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	// Link pre-uploaded attachments to this message.
	if s.attachmentService != nil && len(req.AttachmentIDs) > 0 {
		if err := s.attachmentService.LinkToMessage(ctx, req.AttachmentIDs, msg.ID); err != nil {
			slog.ErrorContext(ctx, "link attachments to support message", "error", err, "message_id", msg.ID)
		}
		// Hydrate for WS broadcast.
		msgs := []model.SupportMessage{*msg}
		if err := s.attachmentService.HydrateMessages(ctx, msgs); err == nil {
			msg.Attachments = msgs[0].Attachments
		}
	}

	s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, msg, derefString(senderUserID)))

	// Emit mention notifications after message creation.
	if len(mentionedUserIDs) > 0 {
		ProcessSupportMentions(ctx, s.notificationService, conv, msg.Content, derefString(senderUserID), mentionedUserIDs)
	}

	previousOwnerID := ""
	if conv != nil && conv.OpenedByUserID != nil {
		previousOwnerID = strings.TrimSpace(*conv.OpenedByUserID)
	}

	if !msg.IsInternal && msg.MessageType == "reply" && msg.SenderType == "customer" {
		senderName := derefString(msg.SenderDisplayName)
		ProcessSupportCustomerReplyNotification(ctx, s.notificationService, conv, msg.Content, senderName)
		if conv.Status == model.SupportConversationStatusWaitingOnCustomer || conv.Status == model.SupportConversationStatusResolved {
			conv.Status = model.SupportConversationStatusOpen
			if conv.HumanTakeover != nil && *conv.HumanTakeover {
				conv.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
			} else {
				conv.FlowState = strPtr(defaultConversationFlowState(conv.OpenedByUserID, conv.AssignedUserID, conv.AssignedAgentID))
			}
			conv.ClosedAt = nil
			if err := s.conversationRepo.UpdateFields(ctx, workspaceID, ticketID, map[string]any{
				"status":      conv.Status,
				"flow_state":  derefString(conv.FlowState),
				"resolved_at": nil,
				"closed_at":   nil,
				"updated_at":  time.Now(),
			}); err != nil {
				slog.ErrorContext(ctx, "failed to reopen support conversation after customer reply", "error", err, "conversation_id", ticketID)
			}
		}
		if s.triageService != nil && !supportConversationHumanOwned(conv) {
			go func(workspaceID, conversationID, messageID string) {
				if _, triageErr := s.triageService.EvaluateAndRoute(context.WithoutCancel(ctx), workspaceID, conversationID, messageID); triageErr != nil {
					slog.ErrorContext(ctx, "support triage failed after customer reply", "workspace_id", workspaceID, "conversation_id", conversationID, "message_id", messageID, "error", triageErr)
				}
			}(workspaceID, ticketID, msg.ID)
		}
	}

	if !msg.IsInternal && msg.MessageType == "reply" && msg.SenderType == "user" && senderUserID != nil && conv != nil {
		if conv.OpenedByUserID == nil || *conv.OpenedByUserID != *senderUserID {
			conv.OpenedByUserID = senderUserID
			conv.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
			conv.HumanTakeover = boolPtr(true)
			if err := s.conversationRepo.Update(ctx, conv); err != nil {
				slog.ErrorContext(ctx, "failed to set support conversation owner", "error", err, "conversation_id", ticketID)
			}
		}
	}

	if !msg.IsInternal && msg.MessageType == "reply" && msg.SenderType != "customer" && conv != nil {
		conv.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
		conv.HumanTakeover = boolPtr(true)
		updates := map[string]any{
			"flow_state":        model.SupportConversationFlowStateAssignedToHuman,
			"opened_by_user_id": conv.OpenedByUserID,
			"human_takeover":    true,
		}
		if conv.Status == model.SupportConversationStatusResolved {
			conv.Status = model.SupportConversationStatusWaitingOnCustomer
			conv.ResolvedAt = nil
			conv.ClosedAt = nil
			updates["status"] = model.SupportConversationStatusWaitingOnCustomer
			updates["resolved_at"] = nil
			updates["closed_at"] = nil
			updates["updated_at"] = time.Now()
		}
		if err := s.conversationRepo.UpdateFields(ctx, workspaceID, ticketID, updates); err != nil {
			slog.ErrorContext(ctx, "failed to update support conversation flow state after teammate reply", "error", err, "conversation_id", ticketID)
		}

		// Emit human_reply_after_ai when a human agent replies to a conversation that was escalated from AI.
		if conv.AIState != nil && *conv.AIState == "escalated" && msg.SenderType == "user" {
			// Include reply text so draft generator can use real agent answers.
			// IssueKey comes from triage intent if loaded; otherwise coverage
			// creates a needs_review gap that can be reclassified later.
			replyExcerpt := strings.TrimSpace(msg.Content)
			if len(replyExcerpt) > 500 {
				replyExcerpt = replyExcerpt[:500]
			}
			issueSummary := replyExcerpt
			issueKey := ""
			if conv.Triage != nil && conv.Triage.Intent != nil {
				issueKey = *conv.Triage.Intent
				issueSummary = replyExcerpt // keep reply as summary for drafts
			}
			s.recordSupportEvent(SupportEventInput{
				WorkspaceID:    workspaceID,
				EventType:      model.SupportEventHumanReplyAfterAI,
				ConversationID: &ticketID,
				MessageID:      &msg.ID,
				IssueKey:       issueKey,
				IssueSummary:   issueSummary,
				SourceSignal:   model.SupportCoverageSourceHumanReply,
				ActorType:      model.SupportEventActorAgent,
				Channel:        "inbox",
			})
		}
	}

	if !msg.IsInternal && msg.MessageType == "reply" && msg.SenderType != "customer" && previousOwnerID != "" && s.notificationService != nil {
		if err := s.notificationService.MarkEntityCategoryAsRead(ctx, previousOwnerID, workspaceID, "support_conversation", ticketID, model.NotifCategorySupportReplies); err != nil {
			slog.ErrorContext(ctx, "mark support reply notifications handled after teammate reply", "error", err, "conversation_id", ticketID, "recipient_id", previousOwnerID)
		}
	}

	// Push visitor-scoped list refresh for widget unread when an agent/user/AI reply is sent.
	if !msg.IsInternal && msg.SenderType != "customer" && msg.MessageType == "reply" && conv != nil {
		if conv.AnonymousID != nil && *conv.AnonymousID != "" {
			s.pushVisitorConversationsRefresh(ctx, workspaceID, *conv.AnonymousID)
		}
		if s.emailFallbackService != nil {
			go func(convSnapshot *model.SupportConversation) {
				if err := s.emailFallbackService.OnAgentReply(context.WithoutCancel(ctx), workspaceID, msg, convSnapshot); err != nil {
					slog.ErrorContext(ctx, "enqueue email fallback failed", "conversation_id", ticketID, "message_id", msg.ID, "error", err)
				}
			}(conv)
		}
	}

	return msg, nil
}

// LinkConversationStory links a conversation to a task.
func (s *SupportInboxService) LinkConversationStory(ctx context.Context, workspaceID, ticketID, storyID, actorID string) error {
	ticket, err := s.loadConversationAccessible(ctx, workspaceID, ticketID)
	if err != nil {
		return err
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	ticket.LinkedTaskID = &storyID
	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return err
	}

	assoc := &model.CRMAssociation{
		WorkspaceID:    workspaceID,
		FromObjectType: model.CRMObjectSupportConversation,
		FromObjectID:   ticketID,
		ToObjectType:   model.CRMObjectTask,
		ToObjectID:     storyID,
	}
	if err := s.assocRepo.Create(ctx, assoc); err != nil {
		return err
	}

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", ticketID, &actorID, "updated", strPtr("linked_task_id"), nil, &storyID, nil)
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    ticketID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return nil
}

// CreateTaskFromConversation summarizes a support conversation and creates a linked PM task.
func (s *SupportInboxService) CreateTaskFromConversation(
	ctx context.Context,
	workspaceID, conversationID, actorID string,
	req model.CreateTaskFromConversationRequest,
) (*model.CreateTaskFromConversationResponse, error) {
	if s == nil || s.taskService == nil {
		return nil, fmt.Errorf("task service is unavailable")
	}

	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	messages, err := s.ListConversationMessages(ctx, workspaceID, conversationID, true)
	if err != nil {
		return nil, err
	}

	draft, err := s.generateTaskDraftFromConversation(ctx, workspaceID, conversation, messages)
	if err != nil {
		return nil, err
	}
	if trimPtrValue(req.Name) == "" {
		if err := validateSupportTaskDraft(conversation, messages, draft); err != nil {
			slog.WarnContext(ctx, "support task creation blocked due to weak draft context",
				"workspace_id", workspaceID,
				"conversation_id", conversationID,
				"title", strings.TrimSpace(draft.Title),
				"summary_preview", truncateLog(strings.TrimSpace(draft.Summary), 160),
				"error", err,
			)
			return nil, err
		}
	}

	taskType := normalizeSupportTaskType(firstNonEmptyString(trimPtrValue(req.TaskType), draft.TaskType))
	priority := normalizeSupportTaskPriority(firstNonEmptyString(trimPtrValue(req.Priority), draft.Priority))
	description := trimRichTextPtr(req.Description)
	if description == nil {
		description = supportTaskDescriptionToRichText(draft.Description)
	}
	requesterMemberID := trimOptionalPtr(req.RequesterMemberID)
	createReq := model.CreateTaskRequest{
		WorkspaceID:       workspaceID,
		Name:              firstNonEmptyString(trimPtrValue(req.Name), draft.Title),
		Description:       description,
		TaskType:          taskType,
		WorkflowID:        trimPtrValue(req.WorkflowID),
		WorkflowStateID:   trimPtrValue(req.WorkflowStateID),
		EpicID:            trimOptionalPtr(req.EpicID),
		SprintID:          trimOptionalPtr(req.SprintID),
		TeamID:            trimOptionalPtr(req.TeamID),
		OwnerMemberIDs:    optionalTrimmedStringSlice(req.OwnerMemberID),
		RequesterID:       nil,
		RequesterMemberID: requesterMemberID,
		Estimate:          req.Estimate,
		Severity:          req.Severity,
		Deadline:          req.Deadline,
		Position:          req.Position,
		Blocked:           req.Blocked,
		Blocker:           trimOptionalPtr(req.Blocker),
		TemplateID:        trimOptionalPtr(req.TemplateID),
		ExternalID:        trimOptionalPtr(req.ExternalID),
		OwnerIDs:          req.OwnerIDs,
		FollowerIDs:       req.FollowerIDs,
		LabelIDs:          req.LabelIDs,
		AttachmentIDs:     req.AttachmentIDs,
		ChecklistItems:    req.ChecklistItems,
		ExternalLinks:     req.ExternalLinks,
	}
	if requesterMemberID == nil {
		createReq.RequesterID = strPtr(actorID)
	}
	if priority != "" {
		createReq.Priority = &priority
	}

	detail, err := s.taskService.Create(ctx, createReq, actorID)
	if err != nil {
		return nil, err
	}

	taskID := detail.Task.ID
	if err := s.LinkConversationStory(ctx, workspaceID, conversationID, taskID, actorID); err != nil {
		return nil, err
	}

	counts, err := s.copyConversationAssociationsToTask(ctx, workspaceID, conversation, taskID)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "created task from support conversation",
		"workspace_id", workspaceID,
		"conversation_id", conversationID,
		"task_id", taskID,
		"task_key", detail.Task.TaskKey,
		"contact_associations", counts.Contacts,
		"company_associations", counts.Companies,
		"deal_associations", counts.Deals,
	)

	return &model.CreateTaskFromConversationResponse{
		TaskID:                    taskID,
		DisplayID:                 detail.Task.DisplayID,
		TaskKey:                   detail.Task.TaskKey,
		TaskName:                  detail.Task.Name,
		Summary:                   strings.TrimSpace(draft.Summary),
		CopiedContactAssociations: counts.Contacts,
		CopiedCompanyAssociations: counts.Companies,
		CopiedDealAssociations:    counts.Deals,
	}, nil
}

func (s *SupportInboxService) generateTaskDraftFromConversation(
	ctx context.Context,
	workspaceID string,
	conversation *model.SupportConversation,
	messages []model.SupportMessage,
) (*supportConversationTaskDraft, error) {
	if s.supportAIService != nil {
		draft, err := s.supportAIService.GenerateTaskDraftFromConversation(ctx, workspaceID, conversation, messages)
		if err == nil && draft != nil {
			titleEmpty := strings.TrimSpace(draft.Title) == ""
			descriptionEmpty := strings.TrimSpace(draft.Description) == ""
			if titleEmpty || descriptionEmpty {
				slog.WarnContext(ctx, "llm returned incomplete task draft; using deterministic fallback for missing fields",
					"workspace_id", workspaceID,
					"conversation_id", conversation.ID,
					"title_empty", titleEmpty,
					"description_empty", descriptionEmpty,
					"summary_len", len(strings.TrimSpace(draft.Summary)),
				)
			}
			normalized := *draft
			normalized.Title = fallbackSupportTaskTitle(conversation, messages, normalized.Title)
			normalized.Summary = strings.TrimSpace(normalized.Summary)
			normalized.Description = firstNonEmptyString(strings.TrimSpace(normalized.Description), fallbackSupportTaskDescription(conversation, messages, normalized))
			normalized.TaskType = normalizeSupportTaskType(normalized.TaskType)
			normalized.Priority = normalizeSupportTaskPriority(normalized.Priority)
			return &normalized, nil
		}
		if err != nil {
			slog.WarnContext(ctx, "support task draft generation fell back to deterministic summary",
				"workspace_id", workspaceID,
				"conversation_id", conversation.ID,
				"error", err,
			)
		}
	}

	fallback := fallbackSupportConversationTaskDraft(conversation, messages)
	return &fallback, nil
}

func (s *SupportInboxService) copyConversationAssociationsToTask(
	ctx context.Context,
	workspaceID string,
	conversation *model.SupportConversation,
	taskID string,
) (supportTaskAssociationCopyCounts, error) {
	var counts supportTaskAssociationCopyCounts
	if conversation == nil {
		return counts, nil
	}

	contactIDs := map[string]struct{}{}
	companyIDs := map[string]struct{}{}
	dealIDs := map[string]struct{}{}

	if conversation.CRMContactID != nil && strings.TrimSpace(*conversation.CRMContactID) != "" {
		contactIDs[strings.TrimSpace(*conversation.CRMContactID)] = struct{}{}
	}

	assocs, err := s.assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectSupportConversation, conversation.ID)
	if err != nil {
		return counts, err
	}
	for _, assoc := range assocs {
		otherType, otherID := supportAssociationPeer(assoc, model.CRMObjectSupportConversation, conversation.ID)
		switch otherType {
		case model.CRMObjectContact:
			contactIDs[otherID] = struct{}{}
		case model.CRMObjectCompany:
			companyIDs[otherID] = struct{}{}
		case model.CRMObjectDeal:
			dealIDs[otherID] = struct{}{}
		}
	}

	for _, id := range supportSortedSetKeys(contactIDs) {
		assoc := &model.CRMAssociation{
			WorkspaceID:    workspaceID,
			FromObjectType: model.CRMObjectTask,
			FromObjectID:   taskID,
			ToObjectType:   model.CRMObjectContact,
			ToObjectID:     id,
		}
		if err := s.assocRepo.Create(ctx, assoc); err != nil {
			return counts, err
		}
		counts.Contacts++
	}
	for _, id := range supportSortedSetKeys(companyIDs) {
		assoc := &model.CRMAssociation{
			WorkspaceID:    workspaceID,
			FromObjectType: model.CRMObjectTask,
			FromObjectID:   taskID,
			ToObjectType:   model.CRMObjectCompany,
			ToObjectID:     id,
		}
		if err := s.assocRepo.Create(ctx, assoc); err != nil {
			return counts, err
		}
		counts.Companies++
	}
	for _, id := range supportSortedSetKeys(dealIDs) {
		assoc := &model.CRMAssociation{
			WorkspaceID:    workspaceID,
			FromObjectType: model.CRMObjectTask,
			FromObjectID:   taskID,
			ToObjectType:   model.CRMObjectDeal,
			ToObjectID:     id,
		}
		if err := s.assocRepo.Create(ctx, assoc); err != nil {
			return counts, err
		}
		counts.Deals++
	}

	return counts, nil
}

func fallbackSupportConversationTaskDraft(
	conversation *model.SupportConversation,
	messages []model.SupportMessage,
) supportConversationTaskDraft {
	summary := fallbackSupportTaskSummary(conversation, messages)
	return supportConversationTaskDraft{
		Title:       fallbackSupportTaskTitle(conversation, messages, ""),
		Summary:     summary,
		Description: fallbackSupportTaskDescription(conversation, messages, supportConversationTaskDraft{Summary: summary}),
		TaskType:    model.PMTaskTypeFeature,
		Priority:    fallbackSupportTaskPriority(conversation),
	}
}

func fallbackSupportTaskTitle(conversation *model.SupportConversation, messages []model.SupportMessage, proposed string) string {
	if candidate := cleanSupportTaskTitleCandidate(proposed); candidate != "" && !isWeakSupportTaskTitle(candidate) {
		return truncateSupportTaskTitle(candidate, supportTaskTitleMaxLen)
	}
	if conversation != nil {
		if candidate := cleanSupportTaskTitleCandidate(conversation.Subject); candidate != "" && !isWeakSupportTaskTitle(candidate) {
			return truncateSupportTaskTitle(candidate, supportTaskTitleMaxLen)
		}
	}
	if candidate := supportTaskTitleFromMessages(messages); candidate != "" {
		return truncateSupportTaskTitle(candidate, supportTaskTitleMaxLen)
	}
	if conversation != nil {
		return fmt.Sprintf("Customer-reported issue in conversation #%d", conversation.DisplayID)
	}
	return "Customer-reported issue"
}

const supportTaskTitleMaxLen = 90

// truncateSupportTaskTitle caps a title at maxLen characters, cutting at the
// last word boundary and appending an ellipsis. Stack-trace-style subjects
// like the full "SERP analysis failed: Token is not valid; SERP Knowledge..."
// error chain are never useful as task names at full length.
func truncateSupportTaskTitle(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return trimmed
	}
	if cut := strings.IndexAny(trimmed[:maxLen], ";|"); cut >= 16 {
		return strings.TrimRight(trimmed[:cut], " \t-:,;") + "..."
	}
	if space := strings.LastIndexAny(trimmed[:maxLen], " \t"); space >= maxLen/2 {
		return strings.TrimRight(trimmed[:space], " \t-:,;") + "..."
	}
	return strings.TrimRight(trimmed[:maxLen], " \t-:,;") + "..."
}

func fallbackSupportTaskSummary(conversation *model.SupportConversation, messages []model.SupportMessage) string {
	if candidate := supportPreferredContextExcerpt(messages, 220); candidate != "" {
		return candidate
	}
	if candidate := supportLastNonEmptyMessageExcerpt(messages, 220); candidate != "" {
		return candidate
	}
	if conversation == nil {
		return ""
	}
	if subject := cleanSupportTaskTitleCandidate(conversation.Subject); subject != "" && !isWeakSupportTaskTitle(subject) {
		return subject
	}
	return fmt.Sprintf("Support conversation #%d follow-up.", conversation.DisplayID)
}

func fallbackSupportTaskDescription(
	conversation *model.SupportConversation,
	messages []model.SupportMessage,
	draft supportConversationTaskDraft,
) string {
	var b strings.Builder
	problem := strings.TrimSpace(draft.Summary)
	if problem == "" && conversation != nil {
		problem = strings.TrimSpace(conversation.Subject)
	}
	if problem != "" {
		b.WriteString("## Problem\n")
		b.WriteString(problem)
		b.WriteString("\n\n")
	}
	if impact := fallbackSupportTaskImpact(conversation, messages); impact != "" {
		b.WriteString("## Impact\n")
		b.WriteString(impact)
		b.WriteString("\n\n")
	}
	if outcome := specificSupportRequestedOutcome(conversation); outcome != "" {
		b.WriteString("## Requested Outcome\n")
		b.WriteString(outcome)
		b.WriteString("\n\n")
	}
	if conversation != nil {
		b.WriteString("## Customer Context\n")
		b.WriteString(fmt.Sprintf("- Conversation: #%d\n", conversation.DisplayID))
		if subject := strings.TrimSpace(conversation.Subject); subject != "" {
			b.WriteString("- Subject: ")
			b.WriteString(subject)
			b.WriteString("\n")
		}
		if name := strings.TrimSpace(derefString(conversation.CustomerName)); name != "" {
			b.WriteString("- Customer: ")
			b.WriteString(name)
			b.WriteString("\n")
		}
		if email := strings.TrimSpace(derefString(conversation.CustomerEmail)); email != "" {
			b.WriteString("- Email: ")
			b.WriteString(email)
			b.WriteString("\n")
		}
		if priority := strings.TrimSpace(conversation.Priority); priority != "" {
			b.WriteString("- Support Priority: ")
			b.WriteString(priority)
			b.WriteString("\n")
		}
		b.WriteString("\n\n")
	}
	if len(messages) == 0 {
		return strings.TrimSpace(b.String())
	}
	b.WriteString("## Conversation Notes\n")
	start := 0
	if len(messages) > 12 {
		start = len(messages) - 12
	}
	for _, msg := range messages[start:] {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		label := strings.TrimSpace(msg.SenderType)
		if label == "" {
			label = "message"
		}
		b.WriteString("- ")
		b.WriteString(label)
		if msg.IsInternal {
			b.WriteString(" (internal)")
		}
		b.WriteString(": ")
		b.WriteString(excerptSupportText(content, 400))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func fallbackSupportTaskImpact(conversation *model.SupportConversation, messages []model.SupportMessage) string {
	if candidate := supportPreferredContextExcerpt(messages, 240); candidate != "" {
		return candidate
	}
	if candidate := supportLastNonEmptyMessageExcerpt(messages, 240); candidate != "" {
		return candidate
	}
	if conversation == nil {
		return ""
	}
	if subject := cleanSupportTaskTitleCandidate(conversation.Subject); subject != "" && !isWeakSupportTaskTitle(subject) {
		return subject
	}
	return fmt.Sprintf("Customer-facing issue reported in support conversation #%d.", conversation.DisplayID)
}

// specificSupportRequestedOutcome returns a subject-derived outcome sentence
// only when the subject matches a known keyword. For generic conversations it
// returns "" so the description builder can omit the section rather than
// emit boilerplate filler.
func specificSupportRequestedOutcome(conversation *model.SupportConversation) string {
	if conversation == nil {
		return ""
	}
	subject := strings.ToLower(strings.TrimSpace(conversation.Subject))
	switch {
	case strings.Contains(subject, "request"), strings.Contains(subject, "feature"):
		return "Assess the requested capability and decide the appropriate product follow-up for the customer request."
	case strings.Contains(subject, "billing"), strings.Contains(subject, "invoice"), strings.Contains(subject, "payment"):
		return "Identify the underlying issue and complete the internal follow-up needed to unblock the customer."
	default:
		return ""
	}
}

func cleanSupportTaskTitleCandidate(value string) string {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return ""
	}
	candidate = strings.Join(strings.Fields(candidate), " ")
	candidate = trimSupportEmailPrefixes(candidate)
	candidate = trimSupportDiagnosticPrefixes(candidate)
	candidate = trimSupportTaskActionPrefixes(candidate)
	candidate = strings.Trim(candidate, " \t\r\n-:;,.")
	if candidate == "" {
		return ""
	}
	return titleCaseSupportIssue(candidate)
}

// trimSupportDiagnosticPrefixes strips "Error:", "Exception:", "Warning:",
// and bracketed tags like "[ApplicationError]" that customers often paste
// in as the subject line. Without this the raw log prefix becomes the PM
// task name.
func trimSupportDiagnosticPrefixes(value string) string {
	candidate := strings.TrimSpace(value)
	for {
		trimmed := false
		for strings.HasPrefix(candidate, "[") {
			if end := strings.IndexByte(candidate, ']'); end > 0 {
				candidate = strings.TrimSpace(candidate[end+1:])
				trimmed = true
				continue
			}
			break
		}
		lower := strings.ToLower(candidate)
		for _, prefix := range []string{"error:", "exception:", "warning:", "fatal:", "panic:"} {
			if strings.HasPrefix(lower, prefix) {
				candidate = strings.TrimSpace(candidate[len(prefix):])
				trimmed = true
				break
			}
		}
		if !trimmed {
			return candidate
		}
	}
}

func trimSupportEmailPrefixes(value string) string {
	candidate := strings.TrimSpace(value)
	for {
		lower := strings.ToLower(candidate)
		switch {
		case strings.HasPrefix(lower, "re:"):
			candidate = strings.TrimSpace(candidate[3:])
		case strings.HasPrefix(lower, "fw:"):
			candidate = strings.TrimSpace(candidate[3:])
		case strings.HasPrefix(lower, "fwd:"):
			candidate = strings.TrimSpace(candidate[4:])
		default:
			return candidate
		}
	}
}

func trimSupportTaskActionPrefixes(value string) string {
	candidate := strings.TrimSpace(value)
	prefixes := []string{
		"investigate ",
		"fix ",
		"handle ",
		"follow up on ",
		"follow up ",
		"follow-up on ",
		"follow-up ",
		"look into ",
		"review ",
		"resolve ",
		"debug ",
		"triage ",
		"check ",
	}
	for {
		lower := strings.ToLower(candidate)
		trimmed := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(lower, prefix) {
				candidate = strings.TrimSpace(candidate[len(prefix):])
				trimmed = true
				break
			}
		}
		if !trimmed {
			return candidate
		}
	}
}

func isWeakSupportTaskTitle(value string) bool {
	candidate := strings.ToLower(strings.TrimSpace(value))
	if candidate == "" {
		return true
	}
	if len(candidate) < 4 {
		return true
	}
	switch candidate {
	case "new convo", "new conversation", "support conversation", "customer issue", "customer request", "support request", "follow up", "follow-up", "what about me", "help", "hi", "hello", "thanks", "thank you", "test", "testing":
		return true
	}
	if strings.HasPrefix(candidate, "conversation #") {
		return true
	}
	return false
}

func supportTaskTitleFromMessages(messages []model.SupportMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].SenderType != "customer" {
			continue
		}
		if !isSubstantiveSupportContextText(messages[i].Content) {
			continue
		}
		if candidate := supportTaskTitleFromText(messages[i].Content); candidate != "" {
			return candidate
		}
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if !isSubstantiveSupportContextText(messages[i].Content) {
			continue
		}
		if candidate := supportTaskTitleFromText(messages[i].Content); candidate != "" {
			return candidate
		}
	}
	return ""
}

func supportTaskTitleFromText(value string) string {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return ""
	}
	if newline := strings.Index(candidate, "\n"); newline >= 0 {
		candidate = candidate[:newline]
	}
	if sentence := strings.Index(candidate, ". "); sentence >= 0 {
		candidate = candidate[:sentence]
	}
	candidate = excerptSupportText(candidate, 90)
	candidate = cleanSupportTaskTitleCandidate(candidate)
	if isWeakSupportTaskTitle(candidate) {
		return ""
	}
	return candidate
}

func titleCaseSupportIssue(value string) string {
	if value == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError && size == 0 {
		return ""
	}
	if !unicode.IsLetter(r) || unicode.IsUpper(r) {
		return value
	}
	return string(unicode.ToUpper(r)) + value[size:]
}

func validateSupportTaskDraft(
	conversation *model.SupportConversation,
	messages []model.SupportMessage,
	draft *supportConversationTaskDraft,
) error {
	if draft == nil {
		return ErrSupportTaskInsufficientContext
	}
	title := cleanSupportTaskTitleCandidate(draft.Title)
	if isWeakSupportTaskTitle(title) {
		return ErrSupportTaskInsufficientContext
	}
	summary := strings.TrimSpace(draft.Summary)
	if !isSubstantiveSupportContextText(summary) {
		if fallback := supportPreferredContextExcerpt(messages, 220); !isSubstantiveSupportContextText(fallback) {
			if conversation == nil {
				return ErrSupportTaskInsufficientContext
			}
			subject := cleanSupportTaskTitleCandidate(conversation.Subject)
			if !isSubstantiveSupportContextText(subject) || isWeakSupportTaskTitle(subject) {
				return ErrSupportTaskInsufficientContext
			}
		}
	}
	return nil
}

func supportPreferredContextExcerpt(messages []model.SupportMessage, maxLen int) string {
	if candidate := pickSupportContextExcerpt(messages, maxLen, false); candidate != "" {
		return candidate
	}
	// If every message is a pleasantry/sign-off, allow one as a last resort
	// so we don't return empty when the customer genuinely sent nothing else.
	return pickSupportContextExcerpt(messages, maxLen, true)
}

func pickSupportContextExcerpt(messages []model.SupportMessage, maxLen int, allowPleasantry bool) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].SenderType != "customer" {
			continue
		}
		if !isSubstantiveSupportContextText(messages[i].Content) {
			continue
		}
		if !allowPleasantry && looksLikeSupportPleasantry(messages[i].Content) {
			continue
		}
		return excerptSupportText(messages[i].Content, maxLen)
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if !messages[i].IsInternal {
			continue
		}
		if !isSubstantiveSupportContextText(messages[i].Content) {
			continue
		}
		if !allowPleasantry && looksLikeSupportPleasantry(messages[i].Content) {
			continue
		}
		return excerptSupportText(messages[i].Content, maxLen)
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if !isSubstantiveSupportContextText(messages[i].Content) {
			continue
		}
		if !allowPleasantry && looksLikeSupportPleasantry(messages[i].Content) {
			continue
		}
		return excerptSupportText(messages[i].Content, maxLen)
	}
	return ""
}

// looksLikeSupportPleasantry detects sign-offs ("thanks in advance", "kind
// regards") and greetings that technically pass the substantive-text bar on
// length alone but shouldn't be surfaced as the Problem/Impact excerpt.
func looksLikeSupportPleasantry(value string) bool {
	candidate := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
	if candidate == "" {
		return true
	}
	leadingPleasantries := []string{
		"thanks in advance",
		"thank you in advance",
		"thank you so much",
		"thank you for",
		"thanks for",
		"thanks,",
		"thanks.",
		"thank you",
		"kind regards",
		"best regards",
		"warm regards",
		"regards,",
		"regards.",
		"cheers,",
		"cheers.",
		"grazie in anticipo",
		"grazie mille",
		"grazie,",
		"ciao,",
		"hello there",
		"hi there",
		"please help",
	}
	for _, prefix := range leadingPleasantries {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}
	return false
}

func supportLastNonEmptyMessageExcerpt(messages []model.SupportMessage, maxLen int) string {
	for i := len(messages) - 1; i >= 0; i-- {
		content := strings.TrimSpace(messages[i].Content)
		if content == "" {
			continue
		}
		return excerptSupportText(content, maxLen)
	}
	return ""
}

func isSubstantiveSupportContextText(value string) bool {
	candidate := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
	if candidate == "" {
		return false
	}
	switch candidate {
	case "new convo", "new conversation", "what about me", "help", "hi", "hello", "thanks", "thank you", "test", "testing":
		return false
	}
	if len(candidate) >= 18 {
		return true
	}
	if strings.Count(candidate, " ") >= 3 {
		return true
	}
	if strings.ContainsAny(candidate, "0123456789") && len(candidate) >= 10 {
		return true
	}
	for _, keyword := range []string{
		"error", "fail", "issue", "broken", "request", "problem", "cannot", "can't",
		"mismatch", "missing", "wrong", "incorrect", "bug", "sync", "import", "export",
		"billing", "invoice", "payment", "attribution", "conversion",
	} {
		if strings.Contains(candidate, keyword) {
			return true
		}
	}
	return false
}

func fallbackSupportTaskPriority(conversation *model.SupportConversation) string {
	if conversation == nil {
		return model.PMTaskPriorityNone
	}
	switch strings.TrimSpace(conversation.Priority) {
	case model.PMTaskPriorityUrgent, model.PMTaskPriorityHigh, model.PMTaskPriorityMedium, model.PMTaskPriorityLow:
		return strings.TrimSpace(conversation.Priority)
	default:
		return model.PMTaskPriorityNone
	}
}

func normalizeSupportTaskType(value string) string {
	switch strings.TrimSpace(value) {
	case model.PMTaskTypeBug, model.PMTaskTypeChore, model.PMTaskTypeFeature:
		return strings.TrimSpace(value)
	default:
		return model.PMTaskTypeFeature
	}
}

func normalizeSupportTaskPriority(value string) string {
	switch strings.TrimSpace(value) {
	case model.PMTaskPriorityUrgent, model.PMTaskPriorityHigh, model.PMTaskPriorityMedium, model.PMTaskPriorityLow, model.PMTaskPriorityNone:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func supportTaskDescriptionToRichText(markdown string) *string {
	return normalizeTaskDescriptionRichText(strPtr(markdown))
}

func trimRichTextPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func trimOptionalPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalTrimmedStringSlice(value *string) []string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return []string{trimmed}
}

func trimPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func supportAssociationPeer(assoc model.CRMAssociation, currentType, currentID string) (string, string) {
	if assoc.FromObjectType == currentType && assoc.FromObjectID == currentID {
		return assoc.ToObjectType, assoc.ToObjectID
	}
	return assoc.FromObjectType, assoc.FromObjectID
}

func supportSortedSetKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		result = append(result, strings.TrimSpace(value))
	}
	slices.Sort(result)
	return result
}

func excerptSupportText(value string, maxLen int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if maxLen <= 0 || len(normalized) <= maxLen {
		return normalized
	}
	return strings.TrimSpace(normalized[:maxLen]) + "..."
}

// AssignConversationAgent assigns an agent to a conversation.
func (s *SupportInboxService) AssignConversationAgent(ctx context.Context, workspaceID, ticketID, agentID, actorID string) error {
	return s.assignConversationAgent(ctx, workspaceID, ticketID, agentID, &actorID)
}

// AssignConversationUser assigns a teammate to a conversation, or clears the assignee when userID is empty.
func (s *SupportInboxService) AssignConversationUser(ctx context.Context, workspaceID, ticketID string, userID *string, actorID string) error {
	return s.assignConversationUser(ctx, workspaceID, ticketID, userID, &actorID)
}

// UpdateConversationCRMContact sets or clears the primary CRM contact link on a conversation.
func (s *SupportInboxService) UpdateConversationCRMContact(ctx context.Context, workspaceID, conversationID string, contactID *string, actorID string) (*model.SupportConversation, error) {
	return s.updateConversationCRMContact(ctx, workspaceID, conversationID, contactID, &actorID)
}

// ListContactConversations returns support conversations linked to a CRM contact.
func (s *SupportInboxService) ListContactConversations(ctx context.Context, workspaceID, contactID string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	conversations, total, err := s.conversationRepo.ListByContact(ctx, workspaceID, contactID, pagination)
	if err != nil {
		return nil, 0, err
	}
	workspaceMemberID, role := s.actorMailboxScope(ctx, workspaceID)
	ids := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}
	filtered, err := s.conversationRepo.ListByIDs(ctx, workspaceID, ids, workspaceMemberID, role)
	if err != nil {
		return nil, 0, err
	}
	if s.triageService != nil {
		if err := s.triageService.HydrateConversations(ctx, filtered); err != nil {
			slog.ErrorContext(ctx, "hydrate support contact conversation triage", "error", err, "workspace_id", workspaceID)
		}
	}
	return filtered, total, nil
}

// matchOrCreateCRMContact looks up a CRM contact by email; if not found,
// creates one as lead with source=live_chat. Always promotes subscriber→lead.
func (s *SupportInboxService) matchOrCreateCRMContact(ctx context.Context, workspaceID string, email, name *string) *string {
	return s.matchOrCreateCRMContactTx(ctx, s.contactRepo, workspaceID, email, name, "widget_prechat")
}

// matchOrCreateCRMContactTx is the transactional version of matchOrCreateCRMContact.
// It uses the provided contactRepo (which may be wrapped in a transaction).
// The source param controls lifecycle promotion: "identify" promotes lead→customer.
func (s *SupportInboxService) matchOrCreateCRMContactTx(ctx context.Context, contactRepo *repository.CRMContactRepository, workspaceID string, email, name *string, source string) *string {
	return s.matchOrCreateCRMContactIdentityTx(ctx, contactRepo, workspaceID, model.WidgetIdentityPayload{
		Email:  derefString(email),
		Name:   derefString(name),
		Source: source,
	})
}

func (s *SupportInboxService) matchOrCreateCRMContactIdentityTx(ctx context.Context, contactRepo *repository.CRMContactRepository, workspaceID string, identity model.WidgetIdentityPayload) *string {
	resolved := resolveWidgetIdentityPayload(identity)
	if resolved.email == "" {
		return nil
	}

	// Look up existing contact by exact email match
	existing, err := contactRepo.GetByEmail(ctx, workspaceID, resolved.email)
	if err != nil {
		slog.ErrorContext(ctx, "CRM contact lookup failed", "error", err, "workspace_id", workspaceID)
		return nil
	}

	if existing != nil {
		if s.syncCRMContactIdentity(existing, resolved) {
			if err := contactRepo.Update(ctx, existing); err != nil {
				slog.ErrorContext(ctx, "CRM contact identity sync failed", "error", err, "contact_id", existing.ID)
			}
		}

		// Promote lifecycle stage if appropriate (never downgrade)
		promoted := s.promoteContactLifecycle(ctx, contactRepo, existing, resolved.source)
		if promoted {
			slog.InfoContext(ctx, "promoted CRM contact lifecycle",
				"contact_id", existing.ID, "stage", existing.LifecycleStage, "source", resolved.source)
		}
		return &existing.ID
	}

	// Auto-create new contact as lead with source=live_chat
	firstName, lastName := resolved.contactNames()
	if firstName == "" {
		firstName = "Unknown"
	}
	contactSource := "live_chat"
	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		FirstName:      firstName,
		LastName:       lastName,
		Email:          &resolved.email,
		LifecycleStage: model.CRMLifecycleLead,
		LeadStatus:     model.CRMLeadStatusNew,
		Source:         &contactSource,
	}
	displayID, err := contactRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "get next display ID for CRM contact", "error", err, "workspace_id", workspaceID)
		return nil
	}
	contact.DisplayID = displayID
	if err := contactRepo.Create(ctx, contact); err != nil {
		slog.ErrorContext(ctx, "auto-create CRM contact from widget", "error", err, "workspace_id", workspaceID)
		return nil
	}

	// If source is "identify", promote new lead → customer
	if resolved.source == "sdk_identify" {
		s.promoteContactLifecycle(ctx, contactRepo, contact, resolved.source)
	}

	slog.InfoContext(ctx, "auto-created CRM lead from widget",
		"contact_id", contact.ID, "workspace_id", workspaceID, "source", contactSource)
	return &contact.ID
}

type resolvedWidgetCompany struct {
	externalID       string
	name             string
	domain           string
	industry         *string
	employeeCount    *int
	annualRevenue    *float64
	description      *string
	logoURL          *string
	customProperties model.JSONB
}

func (s *SupportInboxService) matchOrCreateCRMCompanyIdentityTx(ctx context.Context, companyRepo *repository.CRMCompanyRepository, workspaceID string, identity model.WidgetIdentityPayload) (*string, error) {
	resolved := resolveWidgetCompanyPayload(identity.Company)
	if resolved == nil {
		return nil, nil
	}

	var company *model.CRMCompany
	var err error
	if resolved.externalID != "" {
		company, err = companyRepo.GetByExternalID(ctx, workspaceID, resolved.externalID)
		if err != nil {
			return nil, err
		}
	}
	if company == nil && resolved.domain != "" {
		company, err = companyRepo.GetByDomain(ctx, workspaceID, resolved.domain)
		if err != nil {
			return nil, err
		}
	}
	if company == nil && resolved.name != "" {
		company, err = companyRepo.GetByName(ctx, workspaceID, resolved.name)
		if err != nil {
			return nil, err
		}
	}

	if company != nil {
		if syncCRMCompanyIdentity(company, *resolved) {
			if err := companyRepo.Update(ctx, company); err != nil {
				return nil, err
			}
		}
		return &company.ID, nil
	}

	if resolved.name == "" {
		return nil, nil
	}

	displayID, err := companyRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	company = &model.CRMCompany{
		WorkspaceID:      workspaceID,
		DisplayID:        displayID,
		ExternalID:       stringPtrOrNil(resolved.externalID),
		Name:             resolved.name,
		Domain:           stringPtrOrNil(resolved.domain),
		Industry:         resolved.industry,
		EmployeeCount:    resolved.employeeCount,
		AnnualRevenue:    resolved.annualRevenue,
		Description:      resolved.description,
		LogoURL:          resolved.logoURL,
		CustomProperties: resolved.customProperties,
	}
	if company.CustomProperties == nil {
		company.CustomProperties = model.JSONB{}
	}
	if err := companyRepo.Create(ctx, company); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "auto-created CRM company from widget identity",
		"company_id", company.ID, "workspace_id", workspaceID, "external_id", resolved.externalID)
	return &company.ID, nil
}

func syncCRMCompanyIdentity(company *model.CRMCompany, identity resolvedWidgetCompany) bool {
	updated := false
	if identity.externalID != "" && strings.TrimSpace(derefString(company.ExternalID)) == "" {
		company.ExternalID = &identity.externalID
		updated = true
	}
	if identity.name != "" && strings.TrimSpace(company.Name) != identity.name {
		company.Name = identity.name
		updated = true
	}
	if identity.domain != "" && strings.TrimSpace(derefString(company.Domain)) != identity.domain {
		company.Domain = &identity.domain
		updated = true
	}
	if identity.industry != nil && strings.TrimSpace(derefString(company.Industry)) != *identity.industry {
		company.Industry = identity.industry
		updated = true
	}
	if identity.employeeCount != nil && (company.EmployeeCount == nil || *company.EmployeeCount != *identity.employeeCount) {
		company.EmployeeCount = identity.employeeCount
		updated = true
	}
	if identity.annualRevenue != nil && (company.AnnualRevenue == nil || *company.AnnualRevenue != *identity.annualRevenue) {
		company.AnnualRevenue = identity.annualRevenue
		updated = true
	}
	if identity.description != nil && strings.TrimSpace(derefString(company.Description)) != *identity.description {
		company.Description = identity.description
		updated = true
	}
	if identity.logoURL != nil && strings.TrimSpace(derefString(company.LogoURL)) != *identity.logoURL {
		company.LogoURL = identity.logoURL
		updated = true
	}

	merged := mergeCRMCustomProperties(company.CustomProperties, identity.customProperties)
	if len(merged) != len(company.CustomProperties) {
		company.CustomProperties = merged
		return true
	}
	for key, value := range merged {
		if !reflect.DeepEqual(company.CustomProperties[key], value) {
			company.CustomProperties = merged
			return true
		}
	}
	return updated
}

func (s *SupportInboxService) ensurePrimaryContactCompanyAssociationTx(ctx context.Context, assocRepo *repository.CRMAssociationRepository, workspaceID, contactID, companyID string) error {
	assoc := &model.CRMAssociation{
		WorkspaceID:      workspaceID,
		FromObjectType:   model.CRMObjectContact,
		FromObjectID:     contactID,
		ToObjectType:     model.CRMObjectCompany,
		ToObjectID:       companyID,
		AssociationLabel: crmAssociationStringPtr(primaryCompanyAssociationLabel),
	}
	if err := assocRepo.Create(ctx, assoc); err != nil {
		return err
	}

	assocs, err := assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contactID)
	if err != nil {
		return err
	}
	for _, existing := range assocs {
		otherType, otherID := otherAssociationSide(existing, model.CRMObjectContact, contactID)
		if otherType != model.CRMObjectCompany {
			continue
		}
		if existing.ID == assoc.ID || otherID == companyID {
			if !isPrimaryCompanyAssociationLabel(existing.AssociationLabel) {
				if err := assocRepo.UpdateLabel(ctx, existing.ID, crmAssociationStringPtr(primaryCompanyAssociationLabel)); err != nil {
					return err
				}
			}
			continue
		}
		if isPrimaryCompanyAssociationLabel(existing.AssociationLabel) {
			if err := assocRepo.UpdateLabel(ctx, existing.ID, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveWidgetCompanyPayload(payload model.JSONB) *resolvedWidgetCompany {
	if len(payload) == 0 {
		return nil
	}

	company := &resolvedWidgetCompany{
		externalID:       widgetPayloadString(payload, "id"),
		name:             widgetPayloadString(payload, "name"),
		domain:           normalizeWidgetCompanyDomain(widgetPayloadString(payload, "domain")),
		industry:         stringPtrOrNil(widgetPayloadString(payload, "industry")),
		description:      stringPtrOrNil(widgetPayloadString(payload, "description")),
		logoURL:          stringPtrOrNil(widgetPayloadString(payload, "logo_url")),
		employeeCount:    widgetPayloadInt(payload, "employee_count"),
		annualRevenue:    widgetPayloadFloat(payload, "annual_revenue"),
		customProperties: model.JSONB{},
	}
	if company.name == "" && company.domain != "" {
		company.name = company.domain
	}
	if company.name == "" && company.externalID != "" {
		company.name = company.externalID
	}

	if company.externalID != "" {
		company.customProperties["sdk_company_id"] = company.externalID
	}
	if createdAt := widgetPayloadString(payload, "created_at"); createdAt != "" {
		company.customProperties["sdk_created_at"] = createdAt
	}

	for _, key := range []string{"custom", "custom_properties", "properties"} {
		switch nested := payload[key].(type) {
		case map[string]interface{}:
			for nestedKey, value := range nested {
				company.customProperties[nestedKey] = value
			}
		case model.JSONB:
			for nestedKey, value := range nested {
				company.customProperties[nestedKey] = value
			}
		}
	}
	for key, value := range payload {
		switch key {
		case "id", "name", "domain", "created_at", "custom", "custom_properties", "properties", "industry", "description", "logo_url", "employee_count", "annual_revenue":
			continue
		default:
			company.customProperties[key] = value
		}
	}

	if company.name == "" && company.externalID == "" && company.domain == "" {
		return nil
	}
	return company
}

func mergeCRMCustomProperties(existing, incoming model.JSONB) model.JSONB {
	merged := model.JSONB{}
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range incoming {
		merged[key] = value
	}
	return merged
}

func widgetPayloadString(payload model.JSONB, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func widgetPayloadInt(payload model.JSONB, key string) *int {
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case int:
		return &typed
	case int64:
		converted := int(typed)
		return &converted
	case float64:
		converted := int(typed)
		return &converted
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func widgetPayloadFloat(payload model.JSONB, key string) *float64 {
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		return &typed
	case float32:
		converted := float64(typed)
		return &converted
	case int:
		converted := float64(typed)
		return &converted
	case int64:
		converted := float64(typed)
		return &converted
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func normalizeWidgetCompanyDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "www.")
	if idx := strings.Index(domain, "/"); idx >= 0 {
		domain = domain[:idx]
	}
	return strings.TrimSpace(domain)
}

func (s *SupportInboxService) syncCRMContactIdentity(contact *model.CRMContact, identity resolvedWidgetIdentity) bool {
	updated := false

	if identity.email != "" {
		if contact.Email == nil || strings.TrimSpace(*contact.Email) != identity.email {
			contact.Email = &identity.email
			updated = true
		}
	}

	firstName, lastName := identity.contactNames()
	if identity.hasExplicitName {
		if firstName != "" && strings.TrimSpace(contact.FirstName) != firstName {
			contact.FirstName = firstName
			updated = true
		}
		if currentLast := strings.TrimSpace(derefString(contact.LastName)); currentLast != strings.TrimSpace(derefString(lastName)) {
			contact.LastName = lastName
			updated = true
		}
		return updated
	}

	if firstName != "" && contactNameIsPlaceholder(contact) && strings.TrimSpace(contact.FirstName) != firstName {
		contact.FirstName = firstName
		contact.LastName = lastName
		updated = true
	}

	return updated
}

type resolvedWidgetIdentity struct {
	email           string
	source          string
	displayName     string
	firstName       string
	lastName        *string
	hasExplicitName bool
}

func (i resolvedWidgetIdentity) contactNames() (string, *string) {
	return i.firstName, i.lastName
}

func resolveWidgetIdentityPayload(identity model.WidgetIdentityPayload) resolvedWidgetIdentity {
	resolved := resolvedWidgetIdentity{
		email:  strings.TrimSpace(identity.Email),
		source: strings.TrimSpace(identity.Source),
	}

	firstName := normalizeWidgetName(identity.FirstName)
	lastName := normalizeOptionalWidgetName(identity.LastName)
	if firstName != "" || lastName != nil {
		if firstName == "" && lastName != nil {
			firstName = *lastName
			lastName = nil
		}
		resolved.firstName = firstName
		resolved.lastName = lastName
		resolved.displayName = joinWidgetNameParts(firstName, derefString(lastName))
		resolved.hasExplicitName = true
		return resolved
	}

	if fullName := normalizeWidgetName(identity.Name); fullName != "" {
		resolved.displayName = fullName
		resolved.firstName, resolved.lastName = splitCRMContactName(fullName)
		resolved.hasExplicitName = true
		return resolved
	}

	if derivedName := deriveWidgetNameFromEmail(resolved.email); derivedName != "" {
		resolved.displayName = derivedName
		resolved.firstName, resolved.lastName = splitCRMContactName(derivedName)
	}

	return resolved
}

func splitCRMContactName(name string) (string, *string) {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "", nil
	}
	if len(parts) == 1 {
		return parts[0], nil
	}

	lastName := strings.Join(parts[1:], " ")
	return parts[0], &lastName
}

func normalizeWidgetName(name string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
}

func normalizeOptionalWidgetName(name string) *string {
	normalized := normalizeWidgetName(name)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func joinWidgetNameParts(firstName, lastName string) string {
	return strings.TrimSpace(strings.Join([]string{strings.TrimSpace(firstName), strings.TrimSpace(lastName)}, " "))
}

func contactNameIsPlaceholder(contact *model.CRMContact) bool {
	firstName := strings.TrimSpace(contact.FirstName)
	return firstName == "" || strings.EqualFold(firstName, "unknown")
}

func deriveWidgetNameFromEmail(email string) string {
	localPart := strings.TrimSpace(email)
	if at := strings.Index(localPart, "@"); at >= 0 {
		localPart = localPart[:at]
	}
	if plus := strings.Index(localPart, "+"); plus >= 0 {
		localPart = localPart[:plus]
	}
	localPart = strings.TrimSpace(localPart)
	if localPart == "" {
		return ""
	}

	var normalized strings.Builder
	for _, r := range localPart {
		switch {
		case unicode.IsLetter(r):
			normalized.WriteRune(unicode.ToLower(r))
		case unicode.IsDigit(r):
			normalized.WriteRune(r)
		default:
			normalized.WriteRune(' ')
		}
	}

	blocked := map[string]struct{}{
		"admin":   {},
		"billing": {},
		"contact": {},
		"hello":   {},
		"help":    {},
		"info":    {},
		"mail":    {},
		"noreply": {},
		"no":      {},
		"office":  {},
		"reply":   {},
		"sales":   {},
		"service": {},
		"support": {},
		"team":    {},
	}

	parts := make([]string, 0, 4)
	for _, token := range strings.Fields(normalized.String()) {
		token = strings.TrimFunc(token, func(r rune) bool { return unicode.IsDigit(r) })
		if token == "" {
			continue
		}
		if _, skip := blocked[token]; skip {
			continue
		}
		parts = append(parts, titleCaseWidgetNameToken(token))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ")
}

func titleCaseWidgetNameToken(token string) string {
	runes := []rune(token)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

// promoteContactLifecycle promotes a CRM contact's lifecycle stage based on the event source.
// subscriber → lead (always), lead → customer (only on "identify" source).
// Never downgrades.
func (s *SupportInboxService) promoteContactLifecycle(ctx context.Context, contactRepo *repository.CRMContactRepository, contact *model.CRMContact, source string) bool {
	var targetStage string

	switch {
	case contact.LifecycleStage == model.CRMLifecycleSubscriber:
		// Always promote subscriber → lead
		targetStage = model.CRMLifecycleLead
	case contact.LifecycleStage == model.CRMLifecycleLead && source == "sdk_identify":
		// SDK identify() promotes lead → customer
		targetStage = model.CRMLifecycleCustomer
	default:
		return false
	}

	// Guard: never downgrade
	if model.CRMLifecycleIsHigherOrEqual(contact.LifecycleStage, targetStage) {
		return false
	}

	contact.LifecycleStage = targetStage
	if err := contactRepo.Update(ctx, contact); err != nil {
		slog.ErrorContext(ctx, "promote CRM contact lifecycle", "error", err, "contact_id", contact.ID)
		return false
	}
	return true
}

func generateSecureToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// ListConversationsWithMentions returns conversations where the given user was mentioned.
func (s *SupportInboxService) ListConversationsWithMentions(ctx context.Context, workspaceID, userID, search string) (*model.ConversationListResponse, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	ids, err := s.conversationRepo.ListConversationIDsWithMentions(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return &model.ConversationListResponse{
			Data: []model.SupportConversation{},
		}, nil
	}
	workspaceMemberID, role := s.actorMailboxScope(ctx, workspaceID)
	conversations, err := s.conversationRepo.ListByIDs(ctx, workspaceID, ids, workspaceMemberID, role)
	if err != nil {
		return nil, err
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	if trimmedSearch := strings.TrimSpace(search); trimmedSearch != "" {
		filtered := conversations[:0]
		for _, conversation := range conversations {
			if supportConversationMatchesSearch(conversation, trimmedSearch) {
				filtered = append(filtered, conversation)
			}
		}
		conversations = filtered
	}
	if s.triageService != nil {
		if err := s.triageService.HydrateConversations(ctx, conversations); err != nil {
			slog.ErrorContext(ctx, "hydrate support mention conversation triage", "error", err, "workspace_id", workspaceID)
		}
	}
	return &model.ConversationListResponse{
		Data:       conversations,
		Total:      len(conversations),
		Page:       1,
		PerPage:    len(conversations),
		TotalPages: 1,
		Meta:       model.ConversationListMeta{},
	}, nil
}

func supportConversationMatchesSearch(conversation model.SupportConversation, search string) bool {
	query := strings.ToLower(strings.TrimSpace(search))
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(conversation.Subject), query) {
		return true
	}
	if strings.Contains(fmt.Sprintf("%d", conversation.DisplayID), query) {
		return true
	}
	if conversation.CustomerName != nil && strings.Contains(strings.ToLower(*conversation.CustomerName), query) {
		return true
	}
	if conversation.CustomerEmail != nil && strings.Contains(strings.ToLower(*conversation.CustomerEmail), query) {
		return true
	}
	return false
}

func (s *SupportInboxService) assignConversationAgent(ctx context.Context, workspaceID, conversationID, agentID string, actorID *string) error {
	ticket, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	if err := validateAgentTarget(agent, "support_conversation"); err != nil {
		return err
	}

	previousAgentID := derefString(ticket.AssignedAgentID)
	ticket.AssignedAgentID = &agentID
	ticket.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
	if s.installationRepo != nil {
		inst, _ := s.installationRepo.GetByWorkspace(ctx, workspaceID)
		if inst != nil {
			settings := parseSettings(inst.Settings)
			if settings.AIAgentID != nil && strings.TrimSpace(*settings.AIAgentID) == agentID {
				pending := "pending"
				ticket.HumanTakeover = boolPtr(false)
				ticket.AIState = &pending
				ticket.AIResolvedAt = nil
				ticket.AIResolutionType = nil
				ticket.FlowState = strPtr(model.SupportConversationFlowStateAIHandling)
			}
		}
	}
	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return err
	}

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversationID, actorID, "updated", strPtr("assigned_agent_id"), nil, &agentID, nil)
	}

	if previousAgentID != agentID {
		s.emitAssignmentSystemMessage(ctx, workspaceID, conversationID, derefString(actorID), assignmentTargetAgent, agent.Name, "")
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     derefString(actorID),
	})

	return nil
}

func (s *SupportInboxService) assignConversationUser(ctx context.Context, workspaceID, conversationID string, userID *string, actorID *string) error {
	ticket, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	var normalizedUserID *string
	if userID != nil {
		trimmed := strings.TrimSpace(*userID)
		if trimmed != "" {
			normalizedUserID = &trimmed
		}
	}

	if normalizedUserID != nil {
		if !s.isConversationAssignableUser(ctx, workspaceID, ticket.MailboxID, *normalizedUserID) {
			return fmt.Errorf("user is not eligible for this conversation")
		}
	}

	previousAssignedUserID := derefString(ticket.AssignedUserID)
	ticket.AssignedUserID = normalizedUserID
	if normalizedUserID != nil {
		ticket.HumanTakeover = boolPtr(true)
	}
	ticket.FlowState = strPtr(defaultConversationFlowState(ticket.OpenedByUserID, ticket.AssignedUserID, ticket.AssignedAgentID))
	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return err
	}

	if s.activitySvc != nil {
		var oldValue *string
		if previousAssignedUserID != "" {
			oldValue = &previousAssignedUserID
		}
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversationID, actorID, "updated", strPtr("assigned_user_id"), oldValue, normalizedUserID, nil)
	}

	newUserID := derefString(normalizedUserID)
	if newUserID != previousAssignedUserID {
		switch {
		case newUserID != "":
			s.emitAssignmentSystemMessage(ctx, workspaceID, conversationID, derefString(actorID), assignmentTargetUser, "", newUserID)
		default:
			s.emitAssignmentSystemMessage(ctx, workspaceID, conversationID, derefString(actorID), assignmentTargetUnassign, "", previousAssignedUserID)
		}
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     derefString(actorID),
	})

	return nil
}

func (s *SupportInboxService) updateConversationCRMContact(ctx context.Context, workspaceID, conversationID string, contactID *string, actorID *string) (*model.SupportConversation, error) {
	ticket, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}

	var normalizedContactID *string
	if contactID != nil {
		trimmed := strings.TrimSpace(*contactID)
		if trimmed != "" {
			contact, err := s.contactRepo.GetByID(ctx, trimmed)
			if err != nil {
				return nil, err
			}
			if contact == nil || contact.WorkspaceID != workspaceID {
				return nil, fmt.Errorf("contact not found")
			}
			normalizedContactID = &trimmed
		}
	}

	oldContactID := ticket.CRMContactID
	if err := s.syncConversationContactAssociation(ctx, workspaceID, conversationID, normalizedContactID); err != nil {
		return nil, err
	}

	fields := map[string]any{
		"crm_contact_id": normalizedContactID,
		"updated_at":     time.Now().UTC(),
	}
	if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversationID, fields); err != nil {
		return nil, err
	}
	ticket.CRMContactID = normalizedContactID
	ticket.UpdatedAt = time.Now().UTC()

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversationID, actorID, "updated", strPtr("crm_contact_id"), oldContactID, normalizedContactID, nil)
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     derefString(actorID),
	})

	return ticket, nil
}

func (s *SupportInboxService) syncConversationContactAssociation(ctx context.Context, workspaceID, conversationID string, contactID *string) error {
	assocs, err := s.assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectSupportConversation, conversationID)
	if err != nil {
		return err
	}
	for _, assoc := range assocs {
		otherType, otherID := otherAssociationSide(assoc, model.CRMObjectSupportConversation, conversationID)
		if otherType != model.CRMObjectContact {
			continue
		}
		if contactID != nil && otherID == *contactID {
			continue
		}
		if err := s.assocRepo.Delete(ctx, assoc.ID); err != nil {
			return err
		}
	}

	if contactID == nil {
		return nil
	}

	return s.assocRepo.Create(ctx, &model.CRMAssociation{
		WorkspaceID:    workspaceID,
		FromObjectType: model.CRMObjectSupportConversation,
		FromObjectID:   conversationID,
		ToObjectType:   model.CRMObjectContact,
		ToObjectID:     *contactID,
	})
}

// assignmentTargetKind describes the kind of assignee referenced in an
// assignment system message so the rendered copy can match intent (user,
// agent, or unassignment).
type assignmentTargetKind int

const (
	assignmentTargetUser assignmentTargetKind = iota
	assignmentTargetAgent
	assignmentTargetUnassign
)

// emitAssignmentSystemMessage writes an internal (admin-only) system message
// describing an assignment change on the conversation. The message is marked
// is_internal=true so it surfaces in the teammate-facing inbox thread but is
// never broadcast or returned to the widget — avoiding a "joined" UX on the
// customer side when the assignee may not actually reply for a while.
//
// targetUserID is the assignee's user ID (for user assignments) or the
// previously assigned user ID (for unassignments); it is used to resolve the
// target's display name and avatar.
func (s *SupportInboxService) emitAssignmentSystemMessage(
	ctx context.Context,
	workspaceID, conversationID, actorUserID string,
	target assignmentTargetKind,
	fallbackTargetName, targetUserID string,
) {
	if s.messageRepo == nil {
		return
	}

	actorName := s.lookupUserName(ctx, actorUserID)
	actorAvatar := s.lookupUserAvatar(ctx, actorUserID)
	targetName := strings.TrimSpace(fallbackTargetName)
	if target != assignmentTargetAgent {
		if name := s.lookupUserName(ctx, targetUserID); name != "" {
			targetName = name
		}
	}

	content := formatAssignmentSystemMessage(target, actorName, targetName, actorUserID, targetUserID)
	if content == "" {
		return
	}

	// Display the actor's identity alongside the system message so the pill
	// shows who performed the action in the admin thread.
	displayName := actorName
	if displayName == "" {
		displayName = "System"
	}

	var senderUserID *string
	if actorUserID != "" {
		senderUserID = &actorUserID
	}

	eventType := assignmentTargetSystemEventType(target, actorUserID, targetUserID)

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderUserID:      senderUserID,
		SenderDisplayName: &displayName,
		SenderAvatarURL:   actorAvatar,
		Content:           content,
		IsInternal:        true,
		MessageType:       "system",
		SystemEventType:   model.SupportSystemEventTypeStrPtr(eventType),
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		slog.ErrorContext(ctx, "create support assignment system message", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		return
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, msg, derefString(senderUserID)))
	}
}

// lookupUserName returns the trimmed FullName for a user ID, or "" on miss.
func (s *SupportInboxService) lookupUserName(ctx context.Context, userID string) string {
	if userID == "" || s.userRepo == nil {
		return ""
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return ""
	}
	return strings.TrimSpace(user.FullName)
}

// lookupUserAvatar returns the avatar URL pointer for a user ID, or nil on miss.
func (s *SupportInboxService) lookupUserAvatar(ctx context.Context, userID string) *string {
	if userID == "" || s.userRepo == nil {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil
	}
	return user.AvatarURL
}

// assignmentTargetSystemEventType maps an assignment action (plus actor /
// target identity) onto the canonical system_event_type constant. Self
// assignments surface as took; user vs. agent assignments surface distinctly.
func assignmentTargetSystemEventType(target assignmentTargetKind, actorUserID, targetUserID string) model.SupportSystemEventType {
	switch target {
	case assignmentTargetUnassign:
		return model.SystemEventUnassigned
	case assignmentTargetAgent:
		return model.SystemEventAgentAssigned
	case assignmentTargetUser:
		if actorUserID != "" && actorUserID == targetUserID {
			return model.SystemEventTook
		}
		return model.SystemEventAssigned
	}
	return model.SystemEventAssigned
}

// formatAssignmentSystemMessage produces the human-readable copy for an
// assignment system message. It handles self-assignment, auto-assignment
// (no actor), and unassignment so the thread reads naturally.
func formatAssignmentSystemMessage(target assignmentTargetKind, actorName, targetName, actorUserID, targetUserID string) string {
	actor := strings.TrimSpace(actorName)
	name := strings.TrimSpace(targetName)

	switch target {
	case assignmentTargetUnassign:
		if actor != "" {
			return fmt.Sprintf("%s moved this conversation to unassigned", actor)
		}
		return "Moved to unassigned"

	case assignmentTargetAgent:
		if name == "" {
			name = "an AI agent"
		}
		if actor != "" {
			return fmt.Sprintf("%s assigned this conversation to %s", actor, name)
		}
		return fmt.Sprintf("Assigned to %s", name)

	case assignmentTargetUser:
		if name == "" {
			name = "a teammate"
		}
		if actorUserID != "" && actorUserID == targetUserID {
			if actor == "" {
				actor = name
			}
			return fmt.Sprintf("%s took this conversation", actor)
		}
		if actor != "" {
			return fmt.Sprintf("%s assigned this conversation to %s", actor, name)
		}
		return fmt.Sprintf("Assigned to %s", name)
	}
	return ""
}

// emitTeammateJoinedIfFirstReply emits a public "{name} joined the conversation"
// system message on the widget-visible side the first time a given teammate
// sends a non-internal reply on the conversation. Matches Intercom's behavior
// of surfacing a "joined" pill on first engagement rather than on assignment.
func (s *SupportInboxService) emitTeammateJoinedIfFirstReply(ctx context.Context, workspaceID, conversationID, senderUserID, displayName string, senderAvatar *string) {
	if s.messageRepo == nil || senderUserID == "" {
		return
	}

	priorMessages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		slog.ErrorContext(ctx, "list messages for first-reply check", "error", err, "conversation_id", conversationID)
		return
	}
	for _, prior := range priorMessages {
		if prior.SenderType != "user" || prior.IsInternal {
			continue
		}
		if prior.MessageType == "system" {
			continue
		}
		if prior.SenderUserID != nil && strings.TrimSpace(*prior.SenderUserID) == senderUserID {
			return
		}
	}

	name := strings.TrimSpace(displayName)
	if name == "" {
		name = s.lookupUserName(ctx, senderUserID)
	}
	if name == "" {
		name = "A teammate"
	}
	avatarURL := senderAvatar
	if avatarURL == nil {
		avatarURL = s.lookupUserAvatar(ctx, senderUserID)
	}

	content := fmt.Sprintf("%s joined the conversation", name)
	userID := senderUserID
	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderUserID:      &userID,
		SenderDisplayName: &name,
		SenderAvatarURL:   avatarURL,
		Content:           content,
		IsInternal:        false,
		MessageType:       "system",
		SystemEventType:   model.SupportSystemEventTypeStrPtr(model.SystemEventTeammateJoined),
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		slog.ErrorContext(ctx, "create teammate-joined system message", "error", err, "conversation_id", conversationID)
		return
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, msg, userID))
	}
}
