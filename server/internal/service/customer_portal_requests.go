package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PortalMessage is a customer-safe conversation message.
type PortalMessage struct {
	ID          string                   `json:"id"`
	Content     string                   `json:"content"`
	SenderType  string                   `json:"sender_type"`
	SenderName  *string                  `json:"sender_name,omitempty"`
	ViaChannel  string                   `json:"via_channel,omitempty"`
	Attachments []model.WidgetAttachment `json:"attachments,omitempty"`
	CreatedAt   time.Time                `json:"created_at"`
}

// PortalRequestDetail is a customer-safe request with its public messages.
type PortalRequestDetail struct {
	Reference      string          `json:"reference"`
	Subject        string          `json:"subject"`
	Status         string          `json:"status"`
	LastActivityAt time.Time       `json:"last_activity_at"`
	CanReply       bool            `json:"can_reply"`
	AIProcessing   bool            `json:"ai_processing"`
	Messages       []PortalMessage `json:"messages"`
}

type portalReplySourceKey struct{}
type portalReplyAuditKey struct{}
type portalReplyAudit struct{ IdentityID, SessionID string }
type portalReplyAIKey struct{}

func portalAIQueueAllowed(settings model.SupportInboxSettings, ownerID *string) bool {
	if !settings.PortalEnabled || (settings.PortalAIMode != "ai_first" && settings.PortalAIMode != "internal_note") {
		return false
	}
	return ownerID == nil || strings.TrimSpace(*ownerID) == ""
}

// StartAnonymousIntake issues a one-use intake token for an unverified address.
func (s *CustomerPortalService) StartAnonymousIntake(ctx context.Context, ws *PortalWorkspace, address string) (string, error) {
	if !s.anonymousIntakeEnabled(ws) {
		return "", ErrPortalAnonymousIntakeDisabled
	}
	address, ok := normalizePortalEmail(address)
	if !ok {
		return "", ErrPortalAuthInvalid
	}
	secret, err := portalSecret()
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateIntakeSession(ctx, &model.PortalIntakeSession{
		ID: uuid.NewString(), WorkspaceID: ws.ID, Email: address,
		TokenHash: portalHash(secret), ExpiresAt: time.Now().Add(portalIntakeTTL),
	}); err != nil {
		return "", err
	}
	return secret, nil
}

// UploadAnonymousAttachment starts an upload scoped to an intake token.
func (s *CustomerPortalService) UploadAnonymousAttachment(ctx context.Context, ws *PortalWorkspace, secret string, req model.CreateSupportAttachmentRequest) (*model.SupportAttachmentResponse, error) {
	session, err := s.anonymousIntakeSession(ctx, ws, secret)
	if err != nil {
		return nil, err
	}
	if !s.uploadsEnabled(ws) {
		return nil, ErrPortalAttachmentsUnavailable
	}
	return s.inbox.attachmentService.Create(ctx, req, ws.ID, "", "customer", nil, &session.ID)
}

// ConfirmAnonymousAttachment confirms an upload made with the same intake token.
func (s *CustomerPortalService) ConfirmAnonymousAttachment(ctx context.Context, ws *PortalWorkspace, secret, attachmentID string) error {
	session, err := s.anonymousIntakeSession(ctx, ws, secret)
	if err != nil {
		return err
	}
	if !s.uploadsEnabled(ws) {
		return ErrPortalAttachmentsUnavailable
	}
	if !s.inbox.attachmentService.PortalAttachmentOwned(ctx, attachmentID, ws.ID, session.ID, "") {
		return ErrPortalAttachmentsInvalid
	}
	return s.inbox.attachmentService.ConfirmUpload(ctx, attachmentID, "customer", nil, &session.ID)
}

// CreateAnonymousRequest creates a support conversation from an intake token.
// The address is unverified, so the request stays out of every portal until
// the address owner exchanges the confirmation link sent here. An address
// that may not use the portal gets a receipt without a link instead, and
// agents reply by email. Submitting never grants portal access.
func (s *CustomerPortalService) CreateAnonymousRequest(ctx context.Context, ws *PortalWorkspace, secret, subject, description string, attachmentIDs []string) error {
	session, err := s.anonymousIntakeSession(ctx, ws, secret)
	if err != nil {
		return err
	}
	subject, description, err = validPortalRequest(subject, description)
	if err != nil {
		return err
	}
	if err := s.validateAttachments(ctx, ws, attachmentIDs, session.ID, ""); err != nil {
		return err
	}
	draft, err := s.routeNewRequest(ctx, ws.ID, session.Email, nil)
	if err != nil {
		return err
	}
	draft.Subject, draft.Description, draft.AttachmentIDs = subject, description, attachmentIDs
	var conversation *model.SupportConversation
	err = s.repo.ConsumeIntakeSession(ctx, ws.ID, portalHash(secret), time.Now(), func(tx *gorm.DB, current *model.PortalIntakeSession) error {
		draft.UploadSessionID = current.ID
		created, _, createErr := s.repo.WithTx(tx).CreateRequest(ctx, draft)
		conversation = created
		return createErr
	})
	if err != nil {
		return err
	}
	s.publishNewRequest(ctx, conversation)
	result, err := s.eligibility(ctx, s.repo, ws, session.Email)
	if err != nil {
		slog.ErrorContext(ctx, "portal eligibility lookup failed", "workspace_id", ws.ID, "conversation_id", conversation.ID, "error", err)
		return nil
	}
	if result.Eligible {
		s.sendLink(ctx, ws, session.Email, &conversation.ID)
	} else {
		s.sendIntakeReceipt(ctx, ws, session.Email)
	}
	return nil
}

// sendIntakeReceipt tells an address without portal access that its request
// arrived. Portal intake requests are the durable throttle record, so the
// same hourly caps as sign-in links apply without a link row.
func (s *CustomerPortalService) sendIntakeReceipt(ctx context.Context, ws *PortalWorkspace, address string) {
	if s.sender == nil {
		return
	}
	forEmail, forWorkspace, err := s.repo.IntakeRequestCountsSince(ctx, ws.ID, address, time.Now().Add(-time.Hour))
	if err != nil {
		slog.ErrorContext(ctx, "portal intake receipt throttle lookup failed", "workspace_id", ws.ID, "error", err)
		return
	}
	if forEmail > portalLinksPerAddressPerHour || forWorkspace > portalLinksPerWorkspaceHour {
		slog.InfoContext(ctx, "portal intake receipt throttled", "workspace_id", ws.ID)
		return
	}
	if err := s.sender.SendEmail(address, "We received your support request",
		"<p>We received your support request. Our team will reply to this email address.</p>",
		"We received your support request. Our team will reply to this email address."); err != nil {
		slog.ErrorContext(ctx, "portal intake receipt delivery failed", "workspace_id", ws.ID, "error", err)
	}
}

// CreateRequest creates a request owned by the signed-in identity.
func (s *CustomerPortalService) CreateRequest(ctx context.Context, access *PortalAccess, subject, description string, attachmentIDs []string) (*model.SupportPortalRequest, error) {
	if !access.Workspace.Settings.PortalIntakeEnabled {
		return nil, ErrPortalIntakeDisabled
	}
	subject, description, err := validPortalRequest(subject, description)
	if err != nil {
		return nil, err
	}
	if err := s.validateAttachments(ctx, &access.Workspace, attachmentIDs, access.Session.ID, ""); err != nil {
		return nil, err
	}
	identity := access.Identity
	draft, err := s.routeNewRequest(ctx, access.Workspace.ID, identity.Email, identity.DisplayName)
	if err != nil {
		return nil, err
	}
	draft.Identity = &identity
	draft.Subject, draft.Description, draft.AttachmentIDs, draft.UploadSessionID = subject, description, attachmentIDs, access.Session.ID
	if portalAIQueueAllowed(access.Workspace.Settings, draft.OwnerID) {
		draft.DispatchAIMode = access.Workspace.Settings.PortalAIMode
	}
	if draft.DispatchAIMode == "ai_first" {
		flow := model.SupportConversationFlowStateAIHandling
		draft.FlowState = &flow
	}
	conversation, request, err := s.repo.CreateRequest(ctx, draft)
	if err != nil {
		return nil, err
	}
	s.publishNewRequest(ctx, conversation)
	return request, nil
}

// Requests lists the identity's requests. Continuity with new email and
// verified widget conversations is re-evaluated at most once per session
// interval rather than on every read.
func (s *CustomerPortalService) Requests(ctx context.Context, access *PortalAccess, status string) ([]model.SupportPortalRequest, error) {
	due, err := s.repo.MarkSessionReconciled(ctx, access.Session.ID, time.Now(), portalReconcileInterval)
	if err != nil {
		slog.ErrorContext(ctx, "portal reconcile claim failed", "workspace_id", access.Workspace.ID, "portal_identity_id", access.Identity.ID, "error", err)
	} else if due {
		if err := s.repo.ReconcileRequests(ctx, access.Workspace.ID, access.Identity.ID); err != nil {
			slog.ErrorContext(ctx, "portal request reconcile failed", "workspace_id", access.Workspace.ID, "portal_identity_id", access.Identity.ID, "error", err)
		}
	}
	return s.repo.ListRequests(ctx, access.Workspace.ID, access.Identity.ID, status)
}

// RequestDetail returns a request the identity may read.
func (s *CustomerPortalService) RequestDetail(ctx context.Context, access *PortalAccess, reference string) (*PortalRequestDetail, error) {
	conv, err := s.repo.FindRequest(ctx, access.Workspace.ID, access.Identity.ID, reference)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, ErrPortalRequestNotFound
	}
	messages, err := s.inbox.ListConversationMessages(ctx, access.Workspace.ID, conv.ID, false)
	if err != nil {
		return nil, err
	}
	detail := &PortalRequestDetail{Reference: reference, Subject: conv.Subject, Status: model.PortalRequestStatus(conv.Status), LastActivityAt: conv.CreatedAt, CanReply: conv.AnonymizedAt == nil, Messages: []PortalMessage{}}
	detail.AIProcessing = conv.FlowState != nil && *conv.FlowState == model.SupportConversationFlowStateAIHandling &&
		!supportConversationHumanOwned(conv) && conv.LastCustomerMessageID != nil &&
		conv.LastPublicMessageID != nil && *conv.LastPublicMessageID == *conv.LastCustomerMessageID
	for i := range messages {
		msg := &messages[i]
		if !portalMessageVisible(msg) {
			continue
		}
		detail.Messages = append(detail.Messages, projectPortalMessage(msg))
		if msg.CreatedAt.After(detail.LastActivityAt) {
			detail.LastActivityAt = msg.CreatedAt
		}
	}
	return detail, nil
}

func projectPortalMessage(msg *model.SupportMessage) PortalMessage {
	public := PortalMessage{ID: msg.ID, Content: msg.Content, SenderType: msg.SenderType, SenderName: msg.SenderDisplayName, CreatedAt: msg.CreatedAt}
	if msg.ViaChannel != nil {
		public.ViaChannel = *msg.ViaChannel
	}
	for _, attachment := range msg.Attachments {
		if attachment.URL == "" || attachment.ProcessingStatus != "" {
			continue
		}
		public.Attachments = append(public.Attachments, model.WidgetAttachment{ID: attachment.ID, FileName: attachment.FileName, FileType: attachment.FileType, FileSize: attachment.FileSize, URL: attachment.URL})
	}
	return public
}

func portalMessageVisible(msg *model.SupportMessage) bool {
	return !msg.IsInternal && msg.DeliveryMode() != model.SupportDeliveryEmailOnly && msg.MessageType == "reply" &&
		(msg.SenderType == "customer" || msg.SenderType == "user" || msg.SenderType == "agent" || msg.SenderType == "ai")
}

// Reply appends a customer reply (text, files, or both), reopening a
// resolved request, and returns the refreshed request.
func (s *CustomerPortalService) Reply(ctx context.Context, access *PortalAccess, reference, content string, attachmentIDs []string) (*PortalRequestDetail, error) {
	// As in chat, a reply may be text, files, or both.
	if strings.TrimSpace(content) == "" && len(attachmentIDs) == 0 {
		return nil, ErrPortalReplyInvalid
	}
	conv, err := s.repo.FindRequest(ctx, access.Workspace.ID, access.Identity.ID, reference)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, ErrPortalRequestNotFound
	}
	if conv.AnonymizedAt != nil {
		return nil, ErrPortalReplyUnavailable
	}
	if err := s.validateAttachments(ctx, &access.Workspace, attachmentIDs, access.Session.ID, conv.ID); err != nil {
		return nil, err
	}
	replyCtx := context.WithValue(ctx, portalReplySourceKey{}, true)
	replyCtx = context.WithValue(replyCtx, portalReplyAuditKey{}, portalReplyAudit{IdentityID: access.Identity.ID, SessionID: access.Session.ID})
	if portalAIQueueAllowed(access.Workspace.Settings, conv.AssignedUserID) && !supportConversationHumanOwned(conv) && conv.CustomerRequestedHumanAt == nil {
		replyCtx = context.WithValue(replyCtx, portalReplyAIKey{}, access.Workspace.Settings.PortalAIMode)
	}
	if _, err := s.inbox.CreateConversationMessage(replyCtx, access.Workspace.ID, conv.ID, model.CreateMessageRequest{
		Content: content, MessageType: "reply", AttachmentIDs: attachmentIDs,
	}, "customer", nil, nil, access.Identity.DisplayName); err != nil {
		return nil, err
	}
	return s.RequestDetail(ctx, access, reference)
}

// UploadAttachment starts an upload for a new request (empty reference) or
// for a reply on one of the identity's requests.
func (s *CustomerPortalService) UploadAttachment(ctx context.Context, access *PortalAccess, reference string, req model.CreateSupportAttachmentRequest) (*model.SupportAttachmentResponse, error) {
	conversationID, err := s.attachmentTarget(ctx, access, reference)
	if err != nil {
		return nil, err
	}
	return s.inbox.attachmentService.Create(ctx, req, access.Workspace.ID, conversationID, "customer", nil, &access.Session.ID)
}

// ConfirmAttachment confirms an upload made by the same session and target.
func (s *CustomerPortalService) ConfirmAttachment(ctx context.Context, access *PortalAccess, reference, attachmentID string) error {
	conversationID, err := s.attachmentTarget(ctx, access, reference)
	if err != nil {
		return err
	}
	if !s.inbox.attachmentService.PortalAttachmentOwned(ctx, attachmentID, access.Workspace.ID, access.Session.ID, conversationID) {
		return ErrPortalAttachmentsInvalid
	}
	return s.inbox.attachmentService.ConfirmUpload(ctx, attachmentID, "customer", nil, &access.Session.ID)
}

// attachmentTarget checks upload availability and returns the target
// conversation, or "" for a new request, which requires portal intake.
func (s *CustomerPortalService) attachmentTarget(ctx context.Context, access *PortalAccess, reference string) (string, error) {
	if !s.uploadsEnabled(&access.Workspace) {
		return "", ErrPortalAttachmentsUnavailable
	}
	if reference == "" {
		if !access.Workspace.Settings.PortalIntakeEnabled {
			return "", ErrPortalIntakeDisabled
		}
		return "", nil
	}
	conv, err := s.repo.FindRequest(ctx, access.Workspace.ID, access.Identity.ID, reference)
	if err != nil {
		return "", err
	}
	if conv == nil || conv.AnonymizedAt != nil {
		return "", ErrPortalRequestNotFound
	}
	return conv.ID, nil
}

func (s *CustomerPortalService) anonymousIntakeSession(ctx context.Context, ws *PortalWorkspace, secret string) (*model.PortalIntakeSession, error) {
	if !s.anonymousIntakeEnabled(ws) {
		return nil, ErrPortalAnonymousIntakeDisabled
	}
	if !validPortalSecret(secret) {
		return nil, ErrPortalAuthInvalid
	}
	session, err := s.repo.FindIntakeSession(ctx, ws.ID, portalHash(secret), time.Now())
	if err != nil {
		return nil, ErrPortalAuthInvalid
	}
	return session, nil
}

// anonymousIntakeEnabled requires the settings and working email: an
// unverified submitter hears back only by email, through a confirmation or
// receipt and then agent replies.
func (s *CustomerPortalService) anonymousIntakeEnabled(ws *PortalWorkspace) bool {
	return ws.Settings.PortalIntakeEnabled && ws.Settings.PortalAnonymousIntakeEnabled && s.inbox.PortalIntakeEmailAvailable()
}

func (s *CustomerPortalService) uploadsEnabled(ws *PortalWorkspace) bool {
	return ws.Settings.FileUploadsEnabled && s.inbox.attachmentService != nil
}

func (s *CustomerPortalService) validateAttachments(ctx context.Context, ws *PortalWorkspace, ids []string, sessionID, conversationID string) error {
	if len(ids) == 0 {
		return nil
	}
	if !s.uploadsEnabled(ws) {
		return ErrPortalAttachmentsUnavailable
	}
	if len(ids) > PortalMaxAttachmentsPerMessage {
		return ErrPortalTooManyAttachments
	}
	if err := s.inbox.attachmentService.ValidatePortalAttachments(ctx, ids, ws.ID, sessionID, conversationID); err != nil {
		return ErrPortalAttachmentsInvalid
	}
	return nil
}

func validPortalRequest(subject, description string) (string, string, error) {
	subject, description = strings.TrimSpace(subject), strings.TrimSpace(description)
	if subject == "" || description == "" || len([]rune(subject)) > 200 {
		return "", "", ErrPortalRequestInvalid
	}
	return subject, description, nil
}

// routeNewRequest applies portal mailbox routing, ownership, and CRM matching.
func (s *CustomerPortalService) routeNewRequest(ctx context.Context, workspaceID, email string, name *string) (repository.PortalRequestDraft, error) {
	draft := repository.PortalRequestDraft{WorkspaceID: workspaceID, Email: email, DisplayName: name}
	mailboxID, mailbox, err := s.inbox.maybeApplyMailboxRoutingForChannel(ctx, workspaceID, nil, true, "portal")
	if err != nil {
		return draft, err
	}
	ownerID, flowState, err := s.inbox.determineMailboxOwner(ctx, workspaceID, mailbox, nil)
	if err != nil {
		return draft, err
	}
	draft.MailboxID, draft.OwnerID, draft.FlowState = mailboxID, ownerID, &flowState
	if s.inbox.contactRepo != nil {
		draft.CRMContactID = s.inbox.matchOrCreateCRMContact(ctx, workspaceID, &email, name)
	}
	return draft, nil
}

func (s *CustomerPortalService) publishNewRequest(ctx context.Context, conversation *model.SupportConversation) {
	if s.inbox.wsPublisher != nil {
		s.inbox.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "support_conversation", EntityID: conversation.ID, WorkspaceID: conversation.WorkspaceID})
	}
	if s.inbox.triageService != nil {
		if err := s.inbox.triageService.HydrateConversation(ctx, conversation); err != nil {
			slog.ErrorContext(ctx, "hydrate portal conversation triage", "error", err, "workspace_id", conversation.WorkspaceID, "conversation_id", conversation.ID)
		}
	}
}
