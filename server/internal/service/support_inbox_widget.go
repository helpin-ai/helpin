package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const (
	widgetSessionTTL               = 7 * 24 * time.Hour
	widgetSessionActivityExtension = time.Hour
)

// CreateWidgetSession creates a new session for external widget chat.
// Always creates a new session — multiple concurrent sessions per visitor are allowed.
func (s *SupportInboxService) CreateWidgetSession(ctx context.Context, widgetKey string, anonymousID string, customerName, customerEmail *string, userAgent, pageURL, timezone, locale *string) (*model.SupportWidgetSession, error) {
	inst, err := s.installationRepo.GetByWidgetKey(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("invalid widget key")
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	isAnonymous := customerEmail == nil || *customerEmail == ""
	clientIP, geoLookup, geoErr := s.widgetSessionGeoContext(ctx)
	if geoErr != nil {
		slog.WarnContext(ctx, "widget session geoip lookup failed", "error", geoErr)
	}

	session := &model.SupportWidgetSession{
		WorkspaceID:   inst.WorkspaceID,
		SessionToken:  token,
		AnonymousID:   anonymousID,
		IsAnonymous:   isAnonymous,
		CustomerName:  customerName,
		CustomerEmail: customerEmail,
		UserAgent:     userAgent,
		LastPageURL:   pageURL,
		Timezone:      timezone,
		Locale:        locale,
		IPAddress:     clientIP,
		ExpiresAt:     time.Now().Add(widgetSessionTTL),
	}
	if geoLookup != nil {
		session.CountryCode = stringPtrOrNil(geoLookup.CountryCode)
		session.CountryName = stringPtrOrNil(geoLookup.CountryName)
		session.RegionName = stringPtrOrNil(geoLookup.RegionName)
		session.CityName = stringPtrOrNil(geoLookup.CityName)
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// GetWidgetSession returns a valid session by token.
func (s *SupportInboxService) GetWidgetSession(ctx context.Context, token string) (*model.SupportWidgetSession, error) {
	session, err := s.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if session.RevokedAt != nil {
		return nil, fmt.Errorf("session revoked")
	}
	now := time.Now().UTC()
	if now.After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}
	nextExpiry := now.Add(widgetSessionActivityExtension)
	if session.ExpiresAt.Before(nextExpiry) {
		if err := s.sessionRepo.ExtendExpiryByToken(ctx, session.SessionToken, nextExpiry); err != nil {
			return nil, err
		}
		session.ExpiresAt = nextExpiry
	}
	s.refreshWidgetSessionGeo(ctx, session)
	s.touchWidgetSessionActivity(ctx, session.SessionToken)
	return session, nil
}

// TouchWidgetSessionActivity records a direct widget session activity signal.
func (s *SupportInboxService) TouchWidgetSessionActivity(ctx context.Context, sessionToken string) error {
	if s.sessionRepo == nil {
		return nil
	}
	return s.sessionRepo.TouchActivityByToken(ctx, sessionToken)
}

func (s *SupportInboxService) touchWidgetSessionActivity(ctx context.Context, sessionToken string) {
	if err := s.TouchWidgetSessionActivity(ctx, sessionToken); err != nil {
		slog.WarnContext(ctx, "widget session activity touch failed", "error", err)
	}
}

// GetVisitorConversations returns all conversations for a visitor by anonymous_id.
func (s *SupportInboxService) GetVisitorConversations(ctx context.Context, workspaceID, anonymousID string) ([]model.SupportConversation, error) {
	conversations, err := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, anonymousID)
	if err != nil {
		return nil, err
	}
	s.enrichWidgetConversationOwners(ctx, workspaceID, conversations)
	return conversations, nil
}

// UpgradeWidgetSession upgrades an anonymous session with email and name.
// It creates/promotes a CRM contact, backfills all conversations and sessions
// for the same anonymous_id, and broadcasts real-time updates.
// The source parameter controls lifecycle promotion: "identify" promotes lead→customer.
func (s *SupportInboxService) UpgradeWidgetSession(ctx context.Context, sessionToken string, identity model.WidgetIdentityPayload) error {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}
	inst := &model.SupportWidgetInstallation{IdentityVerificationMode: model.IdentityVerificationModeReportOnly}
	if s.installationRepo != nil {
		inst, err = s.installationRepo.GetByWorkspace(ctx, session.WorkspaceID)
		if err != nil {
			return err
		}
		if inst == nil {
			return fmt.Errorf("widget installation not found")
		}
	}
	provenance, err := verifyWidgetIdentity(inst, identity, time.Now().UTC())
	if err != nil {
		return err
	}

	resolved := resolveWidgetIdentityPayload(identity)

	// Run all state changes in a single transaction
	var contactID *string
	var updatedConvIDs []string

	db := s.conversationRepo.DB()
	txErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		convRepoTx := s.conversationRepo.WithTx(tx)
		sessionRepoTx := s.sessionRepo.WithTx(tx)
		contactRepoTx := s.contactRepo.WithTx(tx)
		companyRepoTx := repository.NewCRMCompanyRepository(tx)
		assocRepoTx := repository.NewCRMAssociationRepository(tx)

		// 1. Update current session
		session.CustomerEmail = &resolved.email
		if resolved.displayName != "" {
			session.CustomerName = &resolved.displayName
		}
		session.IsAnonymous = false
		if session.IdentityTrust != model.IdentityTrustVerified || provenance.trust == model.IdentityTrustVerified {
			session.IdentityMethod = provenance.method
			session.IdentityTrust = provenance.trust
			session.IdentityVerifiedAt = provenance.verifiedAt
			session.IdentityVerifierVersion = provenance.verifierVersion
		}
		if err := sessionRepoTx.Update(ctx, session); err != nil {
			return err
		}

		// 2. Create or match CRM contact — always as lead with source=live_chat
		contactID = s.matchOrCreateCRMContactIdentityTx(ctx, contactRepoTx, session.WorkspaceID, identity)
		companyID, companyMatchMethod, err := s.matchOrCreateCRMCompanyIdentityWithMethodTx(ctx, companyRepoTx, session.WorkspaceID, identity)
		if err != nil {
			return err
		}
		if contactID != nil && companyID != nil {
			if err := s.ensureContactCompanyMembershipTx(ctx, assocRepoTx, session.WorkspaceID, *contactID, *companyID); err != nil {
				return err
			}
		}
		if companyID != nil {
			if err := sessionRepoTx.UpdateCompanyByID(ctx, session.WorkspaceID, session.ID, companyID); err != nil {
				return err
			}
			session.CRMCompanyID = companyID
			if session.ConversationID != nil {
				if _, err := convRepoTx.SetCRMCompanyIfUnset(ctx, session.WorkspaceID, *session.ConversationID, *companyID); err != nil {
					return err
				}
			}
		}

		// 3. Backfill ALL conversations for this anonymous_id
		ids, err := convRepoTx.UpdateIdentityByAnonymousID(ctx, session.WorkspaceID, session.AnonymousID, resolved.email, resolved.displayName, contactID)
		if err != nil {
			return err
		}
		updatedConvIDs = ids

		// 4. Backfill ALL sessions for this anonymous_id (multi-tab)
		if err := sessionRepoTx.UpdateSessionsByAnonymousID(ctx, session.WorkspaceID, session.AnonymousID, resolved.email, resolved.displayName); err != nil {
			return err
		}
		if err := sessionRepoTx.UpgradeIdentityProvenanceByAnonymousID(
			ctx, session.WorkspaceID, session.AnonymousID, provenance.method, provenance.trust,
			provenance.verifiedAt, provenance.verifierVersion,
		); err != nil {
			return err
		}
		link := model.CRMIdentityLink{
			WorkspaceID:        session.WorkspaceID,
			AnonymousID:        session.AnonymousID,
			ExternalUserID:     optionalStringPtr(identity.ExternalUserID),
			ContactID:          contactID,
			CompanyID:          companyID,
			IdentityMethod:     provenance.method,
			IdentityTrust:      provenance.trust,
			CompanyMatchMethod: stringPtrOrNil(companyMatchMethod),
			VerifiedAt:         provenance.verifiedAt,
			VerifierVersion:    provenance.verifierVersion,
		}
		if err := tx.Create(&link).Error; err != nil {
			return fmt.Errorf("record CRM identity link: %w", err)
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	// Broadcast WebSocket events AFTER commit for each affected conversation
	for _, convID := range updatedConvIDs {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "support_conversation",
			EntityID:    convID,
			WorkspaceID: session.WorkspaceID,
		})
	}

	slog.InfoContext(ctx, "widget session upgraded",
		"session_id", session.ID,
		"source", resolved.source,
		"identity_method", provenance.method,
		"conversations_backfilled", len(updatedConvIDs),
	)
	return nil
}

// IdentifyByAnonymousID is the HTTP-based identity path for headless SDK usage.
// It looks up sessions by anonymous_id + widget key, then performs the same
// CRM contact creation, lifecycle promotion, and conversation backfill as UpgradeWidgetSession.
func (s *SupportInboxService) IdentifyByAnonymousID(ctx context.Context, widgetKey, anonymousID string, identity model.WidgetIdentityPayload) error {
	inst, err := s.installationRepo.GetByWidgetKey(ctx, widgetKey)
	if err != nil {
		return err
	}
	if inst == nil {
		return fmt.Errorf("invalid widget key")
	}
	provenance, err := verifyWidgetIdentity(inst, identity, time.Now().UTC())
	if err != nil {
		return err
	}

	workspaceID := inst.WorkspaceID
	resolved := resolveWidgetIdentityPayload(identity)

	var contactID *string
	var updatedConvIDs []string

	db := s.conversationRepo.DB()
	txErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		convRepoTx := s.conversationRepo.WithTx(tx)
		sessionRepoTx := s.sessionRepo.WithTx(tx)
		contactRepoTx := s.contactRepo.WithTx(tx)
		companyRepoTx := repository.NewCRMCompanyRepository(tx)
		assocRepoTx := repository.NewCRMAssociationRepository(tx)

		// 1. Create or match CRM contact
		contactID = s.matchOrCreateCRMContactIdentityTx(ctx, contactRepoTx, workspaceID, identity)
		companyID, companyMatchMethod, err := s.matchOrCreateCRMCompanyIdentityWithMethodTx(ctx, companyRepoTx, workspaceID, identity)
		if err != nil {
			return err
		}
		if contactID != nil && companyID != nil {
			if err := s.ensureContactCompanyMembershipTx(ctx, assocRepoTx, workspaceID, *contactID, *companyID); err != nil {
				return err
			}
		}
		if companyID != nil {
			activeSessions, err := sessionRepoTx.UpdateActiveSessionsCompanyByAnonymousID(ctx, workspaceID, anonymousID, *companyID)
			if err != nil {
				return err
			}
			for _, activeSession := range activeSessions {
				if activeSession.ConversationID == nil {
					continue
				}
				if _, err := convRepoTx.SetCRMCompanyIfUnset(ctx, workspaceID, *activeSession.ConversationID, *companyID); err != nil {
					return err
				}
			}
		}

		// 2. Backfill ALL conversations for this anonymous_id
		ids, err := convRepoTx.UpdateIdentityByAnonymousID(ctx, workspaceID, anonymousID, resolved.email, resolved.displayName, contactID)
		if err != nil {
			return err
		}
		updatedConvIDs = ids

		// 3. Backfill ALL sessions for this anonymous_id
		if err := sessionRepoTx.UpdateSessionsByAnonymousID(ctx, workspaceID, anonymousID, resolved.email, resolved.displayName); err != nil {
			return err
		}
		if err := sessionRepoTx.UpgradeIdentityProvenanceByAnonymousID(
			ctx, workspaceID, anonymousID, provenance.method, provenance.trust,
			provenance.verifiedAt, provenance.verifierVersion,
		); err != nil {
			return err
		}
		link := model.CRMIdentityLink{
			WorkspaceID:        workspaceID,
			AnonymousID:        anonymousID,
			ExternalUserID:     optionalStringPtr(identity.ExternalUserID),
			ContactID:          contactID,
			CompanyID:          companyID,
			IdentityMethod:     provenance.method,
			IdentityTrust:      provenance.trust,
			CompanyMatchMethod: stringPtrOrNil(companyMatchMethod),
			VerifiedAt:         provenance.verifiedAt,
			VerifierVersion:    provenance.verifierVersion,
		}
		if err := tx.Create(&link).Error; err != nil {
			return fmt.Errorf("record CRM identity link: %w", err)
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	// Broadcast after commit
	for _, convID := range updatedConvIDs {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "support_conversation",
			EntityID:    convID,
			WorkspaceID: workspaceID,
		})
	}

	slog.InfoContext(ctx, "widget identify via HTTP",
		"anonymous_id", anonymousID,
		"source", resolved.source,
		"identity_method", provenance.method,
		"conversations_backfilled", len(updatedConvIDs),
	)
	return nil
}

// UpdateSessionPageURL updates the last_page_url on a session using a targeted query.
func (s *SupportInboxService) UpdateSessionPageURL(ctx context.Context, sessionToken, url string) error {
	return s.sessionRepo.UpdatePageURL(ctx, sessionToken, url)
}

// RevokeWidgetSession marks a session as revoked.
func (s *SupportInboxService) RevokeWidgetSession(ctx context.Context, sessionToken string) error {
	session, err := s.sessionRepo.GetByToken(ctx, sessionToken)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}

	now := time.Now()
	session.RevokedAt = &now
	return s.sessionRepo.Update(ctx, session)
}

// ClearSessionConversation clears the conversation_id on a session so the next message creates a new conversation.
func (s *SupportInboxService) ClearSessionConversation(ctx context.Context, sessionToken string) error {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}
	session.ConversationID = nil
	return s.sessionRepo.Update(ctx, session)
}

// SetSessionConversation updates the active conversation on a widget session.
func (s *SupportInboxService) SetSessionConversation(ctx context.Context, sessionToken, conversationID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return fmt.Errorf("conversation_id is required")
	}

	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}

	conversation, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return err
	}
	if conversation == nil || conversation.AnonymousID == nil || *conversation.AnonymousID != session.AnonymousID {
		return fmt.Errorf("conversation not found")
	}

	session.ConversationID = &conversationID
	return s.sessionRepo.Update(ctx, session)
}

// GetInstallationByWidgetKey returns an installation by widget key.
func (s *SupportInboxService) GetInstallationByWidgetKey(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	return s.installationRepo.GetByWidgetKey(ctx, widgetKey)
}

func (s *SupportInboxService) SendWidgetConversationTranscript(ctx context.Context, sessionToken, conversationID, email string) (*model.WidgetTranscriptResponse, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(conversationID) == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}

	conversation, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conversation.AnonymousID == nil || strings.TrimSpace(*conversation.AnonymousID) == "" || strings.TrimSpace(*conversation.AnonymousID) != strings.TrimSpace(session.AnonymousID) {
		return nil, fmt.Errorf("conversation not found")
	}

	recipientEmail := strings.TrimSpace(email)
	if recipientEmail == "" {
		recipientEmail = strings.TrimSpace(derefString(session.CustomerEmail))
	}
	if recipientEmail == "" {
		recipientEmail = strings.TrimSpace(derefString(conversation.CustomerEmail))
	}
	if recipientEmail == "" {
		return nil, fmt.Errorf("email is required")
	}
	if _, err := mail.ParseAddress(recipientEmail); err != nil {
		return nil, fmt.Errorf("invalid email address")
	}

	if session.IsAnonymous && strings.TrimSpace(email) != "" {
		name := strings.TrimSpace(derefString(session.CustomerName))
		if name == "" {
			name = strings.TrimSpace(derefString(conversation.CustomerName))
		}
		if err := s.UpgradeWidgetSession(ctx, sessionToken, model.WidgetIdentityPayload{
			Email:  recipientEmail,
			Name:   name,
			Source: "transcript_request",
		}); err != nil {
			slog.ErrorContext(ctx, "failed to upgrade widget session during transcript request", "error", err, "conversation_id", conversationID)
		}
	}

	if err := s.sendConversationTranscript(ctx, session.WorkspaceID, conversation, recipientEmail, strings.TrimSpace(derefString(conversation.CustomerEmail)) == "" && strings.TrimSpace(email) != ""); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "widget transcript sent", "workspace_id", session.WorkspaceID, "conversation_id", conversationID, "email", recipientEmail)

	if strings.TrimSpace(derefString(conversation.CustomerEmail)) == "" && strings.TrimSpace(email) != "" {
		if err := s.conversationRepo.UpdateFields(ctx, conversation.WorkspaceID, conversationID, map[string]any{
			"customer_email": strings.TrimSpace(email),
		}); err != nil {
			slog.WarnContext(ctx, "persist captured visitor email failed", "error", err, "conversation_id", conversationID)
		}
	}

	return &model.WidgetTranscriptResponse{
		Success: true,
		Message: fmt.Sprintf("Transcript sent to %s", recipientEmail),
	}, nil
}

// SendSupportConversationTranscript sends a transcript from the authenticated support inbox.
func (s *SupportInboxService) SendSupportConversationTranscript(ctx context.Context, workspaceID, conversationID, email string, updateCustomerEmail bool) (*model.SendSupportConversationTranscriptResponse, error) {
	if strings.TrimSpace(conversationID) == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}
	conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	recipientEmail := strings.TrimSpace(email)
	if recipientEmail == "" {
		recipientEmail = strings.TrimSpace(derefString(conversation.CustomerEmail))
	}
	if recipientEmail == "" {
		return nil, fmt.Errorf("email is required")
	}
	if _, err := mail.ParseAddress(recipientEmail); err != nil {
		return nil, fmt.Errorf("invalid email address")
	}
	if err := s.sendConversationTranscript(ctx, workspaceID, conversation, recipientEmail, updateCustomerEmail); err != nil {
		return nil, err
	}
	return &model.SendSupportConversationTranscriptResponse{Success: true, Email: recipientEmail, Message: fmt.Sprintf("Transcript sent to %s", recipientEmail)}, nil
}

func (s *SupportInboxService) sendConversationTranscript(ctx context.Context, workspaceID string, conversation *model.SupportConversation, recipientEmail string, updateCustomerEmail bool) error {
	messages, err := s.ListConversationMessages(ctx, workspaceID, conversation.ID, false)
	if err != nil {
		return err
	}
	workspaceName := "Support"
	if s.workspaceRepo != nil {
		if workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID); err == nil && workspace != nil && strings.TrimSpace(workspace.Name) != "" {
			workspaceName = strings.TrimSpace(workspace.Name)
		}
	}
	if s.emailFallbackService == nil || s.emailFallbackService.emailClient == nil {
		return fmt.Errorf("email is not configured")
	}
	htmlBody, textBody := renderSupportTranscriptBodies(workspaceName, conversation, messages)
	if err := s.emailFallbackService.emailClient.SendEmail(recipientEmail, fmt.Sprintf("Your conversation transcript with %s", workspaceName), htmlBody, textBody); err != nil {
		return err
	}
	if updateCustomerEmail && strings.TrimSpace(derefString(conversation.CustomerEmail)) != strings.TrimSpace(recipientEmail) {
		if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversation.ID, map[string]any{"customer_email": recipientEmail}); err != nil {
			slog.WarnContext(ctx, "persist transcript recipient email failed", "error", err, "conversation_id", conversation.ID)
		}
	}
	return nil
}

// WidgetCreateMessage creates a message from an external widget user.
func (s *SupportInboxService) WidgetCreateMessage(ctx context.Context, sessionToken, content string, attachmentIDs []string) (*model.SupportMessage, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}
	initialConversationID := session.ConversationID
	if s.attachmentService != nil && len(attachmentIDs) > 0 {
		if err := s.attachmentService.ValidateWidgetAttachments(
			ctx, attachmentIDs, session.WorkspaceID, session.ID, initialConversationID,
		); err != nil {
			return nil, err
		}
	}
	createdConversationID := ""
	conversationSubject := truncate(strings.TrimSpace(content), 100)
	if conversationSubject == "" && len(attachmentIDs) > 0 {
		conversationSubject = "Attachment"
	}

	// Update conversation subject from first message if it was eagerly created with placeholder.
	if session.ConversationID != nil {
		conv, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, *session.ConversationID, "", model.RoleOwner)
		if err != nil {
			return nil, err
		}
		if conv == nil {
			session.ConversationID = nil
		} else if conv.Subject == "New conversation" {
			if err := s.conversationRepo.UpdateSubject(ctx, conv.ID, conversationSubject); err != nil {
				return nil, err
			}
		}
	}

	// If no conversation yet, create one.
	if session.ConversationID == nil {
		ticket := &model.SupportConversation{
			WorkspaceID:   session.WorkspaceID,
			Subject:       conversationSubject,
			Status:        "open",
			FlowState:     strPtr(model.SupportConversationFlowStateWaitingForHuman),
			Priority:      "medium",
			Channel:       "widget",
			CustomerName:  session.CustomerName,
			CustomerEmail: session.CustomerEmail,
			AnonymousID:   &session.AnonymousID,
			CRMCompanyID:  session.CRMCompanyID,
			Source:        "widget",
		}

		mailboxID, mailbox, mailboxErr := s.maybeApplyMailboxRoutingForChannel(ctx, session.WorkspaceID, nil, true, "widget")
		if mailboxErr != nil {
			return nil, mailboxErr
		}
		ticket.MailboxID = mailboxID
		if mailbox != nil {
			ownerID, flowState, ownerErr := s.determineMailboxOwner(ctx, session.WorkspaceID, mailbox, nil)
			if ownerErr != nil {
				return nil, ownerErr
			}
			ticket.AssignedUserID = ownerID
			ticket.FlowState = strPtr(flowState)
		}

		// Auto-match or create CRM contact by email.
		if contactID := s.matchOrCreateCRMContact(ctx, session.WorkspaceID, session.CustomerEmail, session.CustomerName); contactID != nil {
			ticket.CRMContactID = contactID
		}

		if err := s.conversationRepo.Create(ctx, ticket); err != nil {
			return nil, err
		}
		session.ConversationID = &ticket.ID
		if err := s.sessionRepo.Update(ctx, session); err != nil {
			if cleanupErr := s.conversationRepo.Delete(ctx, session.WorkspaceID, ticket.ID); cleanupErr != nil {
				slog.ErrorContext(ctx, "clean up widget conversation after session update failure", "error", cleanupErr, "conversation_id", ticket.ID)
			}
			return nil, err
		}
		createdConversationID = ticket.ID

		s.wsPublisher.Publish(websocket.Event{
			Action:      "created",
			Entity:      "support_conversation",
			EntityID:    ticket.ID,
			WorkspaceID: session.WorkspaceID,
		})
	}

	displayName := "Customer"
	if session.CustomerName != nil && *session.CustomerName != "" {
		displayName = *session.CustomerName
	}

	msg := &model.SupportMessage{
		WorkspaceID:       session.WorkspaceID,
		ConversationID:    *session.ConversationID,
		SenderType:        "customer",
		SenderDisplayName: &displayName,
		Content:           strings.TrimSpace(content),
		IsInternal:        false,
		MessageType:       "reply",
		ViaChannel:        strPtr("widget"),
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		if createdConversationID != "" {
			session.ConversationID = nil
			if cleanupErr := s.sessionRepo.Update(ctx, session); cleanupErr != nil {
				slog.ErrorContext(ctx, "clear widget session after first-message failure", "error", cleanupErr, "conversation_id", createdConversationID)
			}
			if cleanupErr := s.conversationRepo.Delete(ctx, session.WorkspaceID, createdConversationID); cleanupErr != nil {
				slog.ErrorContext(ctx, "clean up widget conversation after first-message failure", "error", cleanupErr, "conversation_id", createdConversationID)
			} else {
				s.wsPublisher.Publish(websocket.Event{
					Action: "deleted", Entity: "support_conversation", EntityID: createdConversationID, WorkspaceID: session.WorkspaceID,
				})
			}
		}
		return nil, err
	}

	// Link pre-uploaded attachments to this message.
	if s.attachmentService != nil && len(attachmentIDs) > 0 {
		if err := s.attachmentService.LinkWidgetAttachments(
			ctx, attachmentIDs, session.WorkspaceID, session.ID, *session.ConversationID, msg.ID, initialConversationID,
		); err != nil {
			slog.ErrorContext(ctx, "link widget attachments to message", "error", err, "message_id", msg.ID)
		}
		msgs := []model.SupportMessage{*msg}
		if err := s.attachmentService.HydrateMessages(ctx, msgs); err == nil {
			msg.Attachments = msgs[0].Attachments
		}
	}

	s.wsPublisher.Publish(websocket.SupportMessageEvent(session.WorkspaceID, msg, "widget:"+session.ID))
	s.enrichSupportMessageLinksAsync(msg, "widget:"+session.ID)
	s.recordSupportEvent(SupportEventInput{
		WorkspaceID: session.WorkspaceID, EventType: model.SupportEventCustomerMessageCreated,
		ConversationID: session.ConversationID, MessageID: &msg.ID,
		ActorType: model.SupportEventActorCustomer, Channel: "widget",
	})

	if conv, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, *session.ConversationID, "", model.RoleOwner); err == nil {
		if conv != nil && (conv.Status == model.SupportConversationStatusWaitingOnCustomer || conv.Status == model.SupportConversationStatusResolved) {
			conv.Status = model.SupportConversationStatusOpen
			if conv.HumanTakeover != nil && *conv.HumanTakeover {
				conv.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
			} else {
				conv.FlowState = strPtr(defaultConversationFlowState(conv.OpenedByUserID, conv.AssignedUserID, conv.AssignedAgentID))
			}
			conv.ResolvedAt = nil
			conv.ClosedAt = nil
			if err := s.conversationRepo.UpdateFields(ctx, session.WorkspaceID, *session.ConversationID, map[string]any{
				"status":      conv.Status,
				"flow_state":  derefString(conv.FlowState),
				"resolved_at": nil,
				"closed_at":   nil,
				"updated_at":  time.Now(),
			}); err != nil {
				slog.ErrorContext(ctx, "failed to reopen widget support conversation after customer reply", "error", err, "conversation_id", *session.ConversationID)
			} else {
				s.wsPublisher.Publish(websocket.Event{
					Action:      "updated",
					Entity:      "support_conversation",
					EntityID:    *session.ConversationID,
					WorkspaceID: session.WorkspaceID,
				})
			}
		}
		ProcessSupportCustomerReplyNotification(ctx, s.notificationService, s.pushSenderService, conv, msg.Content, displayName)
	}

	go s.runWidgetPostMessageAutomation(context.WithoutCancel(ctx), session.WorkspaceID, *session.ConversationID, msg.ID, msg.Content)

	return msg, nil
}

func (s *SupportInboxService) runWidgetPostMessageAutomation(ctx context.Context, workspaceID, conversationID, messageID, content string) {
	if s == nil || s.installationRepo == nil {
		return
	}

	if s.triageService != nil {
		if _, err := s.triageService.EvaluateAndRoute(ctx, workspaceID, conversationID, messageID); err != nil {
			slog.ErrorContext(ctx, "widget support triage failed", "workspace_id", workspaceID, "conversation_id", conversationID, "message_id", messageID, "error", err)
		}
	}

	inst, instErr := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	settings := model.DefaultSupportInboxSettings()
	if instErr == nil && inst != nil {
		settings = parseSettings(inst.Settings)
	}

	if shouldAutomaticallyProcessSupportAI(settings) && s.supportAIService != nil {
		conv, convErr := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
		if convErr == nil && conv != nil && conv.HumanTakeover != nil && *conv.HumanTakeover {
			return
		}

		if pubErr := s.supportAIService.PublishAIRequest(ctx, workspaceID, conversationID, messageID, content); pubErr != nil {
			slog.ErrorContext(ctx, "failed to publish AI request event",
				"workspace_id", workspaceID,
				"conversation_id", conversationID,
				"error", pubErr,
			)
			if shouldCreatePublicSupportAIReply(settings) {
				go s.maybeAutoRunConversationAgent(context.WithoutCancel(ctx), workspaceID, conversationID)
			}
			return
		}

		agentID := strings.TrimSpace(*settings.AIAgentID)
		pending := "pending"
		if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversationID, map[string]any{
			"ai_state":          &pending,
			"assigned_agent_id": &agentID,
			"flow_state":        model.SupportConversationFlowStateAIHandling,
		}); err != nil {
			slog.ErrorContext(ctx, "set AI handling state after widget triage", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
		return
	}
}

// PublishWidgetTypingIndicator publishes a widget visitor typing event using session context.
func (s *SupportInboxService) PublishWidgetTypingIndicator(ctx context.Context, sessionToken string, isTyping bool) error {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}
	if session.ConversationID == nil || *session.ConversationID == "" {
		return nil
	}

	s.PublishTypingIndicator(ctx, session.WorkspaceID, *session.ConversationID, "widget:"+session.ID, isTyping, "")
	return nil
}

// GetWidgetConfig returns widget config by widget key (public).
func (s *SupportInboxService) GetWidgetConfig(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	return s.installationRepo.GetByWidgetKey(ctx, widgetKey)
}

// GetPublicWidgetConfig returns the public-facing widget config.
func (s *SupportInboxService) GetPublicWidgetConfig(ctx context.Context, widgetKey string) (*model.WidgetConfigResponse, error) {
	inst, err := s.installationRepo.GetByWidgetKey(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("widget not found")
	}
	return s.buildWidgetConfigResponse(ctx, inst)
}

// GetPublicWidgetConfigByID returns the public-facing widget config by installation ID.
func (s *SupportInboxService) GetPublicWidgetConfigByID(ctx context.Context, id string) (*model.WidgetConfigResponse, error) {
	inst, err := s.installationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("widget not found")
	}
	return s.buildWidgetConfigResponse(ctx, inst)
}

func (s *SupportInboxService) maybeAutoRunConversationAgent(ctx context.Context, workspaceID, conversationID string) {
	if s == nil || s.conversationAgentRunner == nil || s.installationRepo == nil || s.agentRepo == nil {
		return
	}

	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil || inst == nil {
		if err != nil {
			slog.ErrorContext(ctx, "support widget auto-run: get installation failed", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
		return
	}

	settings := parseSettings(inst.Settings)
	if !shouldAutomaticallyProcessSupportAI(settings) {
		return
	}

	conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil || conversation == nil {
		if err != nil {
			slog.ErrorContext(ctx, "support widget auto-run: get conversation failed", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
		return
	}

	configuredAgentID := strings.TrimSpace(*settings.AIAgentID)
	if conversation.AssignedAgentID == nil || strings.TrimSpace(*conversation.AssignedAgentID) == "" {
		if err := s.assignConversationAgent(ctx, workspaceID, conversationID, configuredAgentID, nil); err != nil {
			slog.ErrorContext(ctx, "support widget auto-run: assign ai agent failed", "workspace_id", workspaceID, "conversation_id", conversationID, "agent_id", configuredAgentID, "error", err)
			return
		}
	} else if strings.TrimSpace(*conversation.AssignedAgentID) != configuredAgentID {
		assignedAgent, err := s.agentRepo.GetByID(ctx, workspaceID, strings.TrimSpace(*conversation.AssignedAgentID))
		if err != nil {
			slog.ErrorContext(ctx, "support widget auto-run: get assigned agent failed", "workspace_id", workspaceID, "conversation_id", conversationID, "agent_id", *conversation.AssignedAgentID, "error", err)
			return
		}
		if assignedAgent == nil {
			return
		}
		if err := validateAgentTarget(assignedAgent, "support_conversation"); err != nil {
			slog.InfoContext(ctx, "support widget auto-run skipped for non-support assignee", "workspace_id", workspaceID, "conversation_id", conversationID, "agent_id", assignedAgent.ID, "error", err)
			return
		}
	}

	if _, err := s.conversationAgentRunner(ctx, workspaceID, conversationID); err != nil {
		slog.ErrorContext(ctx, "support widget auto-run failed", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
	}
}

func (s *SupportInboxService) enrichWidgetConversationOwners(ctx context.Context, workspaceID string, conversations []model.SupportConversation) {
	if len(conversations) == 0 {
		return
	}

	ownerIDs := make(map[string]struct{}, len(conversations))
	for _, conversation := range conversations {
		if conversation.OpenedByUserID == nil {
			continue
		}
		ownerID := strings.TrimSpace(*conversation.OpenedByUserID)
		if ownerID != "" {
			ownerIDs[ownerID] = struct{}{}
		}
	}
	if len(ownerIDs) == 0 {
		return
	}

	owners := make(map[string]model.WidgetActiveTeammate, len(ownerIDs))
	statusByUserID := make(map[string]string, len(ownerIDs))
	statuses, err := resolveSupportTeammatePresenceStatuses(
		ctx,
		s.workspaceRepo,
		s.presence,
		s.statusOverrideRepo,
		workspaceID,
		time.Now(),
	)
	if err == nil {
		for _, status := range statuses {
			statusByUserID[status.UserID] = status.Status
		}
	}
	if s.workspaceRepo != nil {
		if members, err := s.workspaceRepo.ListMembers(ctx, workspaceID); err == nil {
			for _, member := range members {
				if member.UserID == "" || strings.TrimSpace(member.FullName) == "" {
					continue
				}
				owners[member.UserID] = model.WidgetActiveTeammate{
					UserID:    member.UserID,
					Name:      member.FullName,
					AvatarURL: member.AvatarURL,
					Status:    statusByUserID[member.UserID],
				}
			}
		}
	}

	if s.userRepo != nil {
		for ownerID := range ownerIDs {
			if _, ok := owners[ownerID]; ok {
				continue
			}
			user, err := s.userRepo.GetByID(ctx, ownerID)
			if err != nil || user == nil || strings.TrimSpace(user.FullName) == "" {
				continue
			}
			owners[ownerID] = model.WidgetActiveTeammate{
				UserID:    user.ID,
				Name:      user.FullName,
				AvatarURL: user.AvatarURL,
				Status:    statusByUserID[user.ID],
			}
		}
	}

	for i := range conversations {
		if conversations[i].OpenedByUserID == nil {
			continue
		}
		ownerID := strings.TrimSpace(*conversations[i].OpenedByUserID)
		if ownerID == "" {
			continue
		}
		owner, ok := owners[ownerID]
		if !ok {
			continue
		}
		conversations[i].OpenedByDisplayName = strPtr(owner.Name)
		conversations[i].OpenedByAvatarURL = owner.AvatarURL
		if strings.TrimSpace(owner.Status) != "" {
			conversations[i].OpenedByStatus = strPtr(owner.Status)
		}
	}
}

func supportTeammateStatusRank(status string) int {
	switch strings.TrimSpace(status) {
	case model.SupportTeammateStatusOnline:
		return 0
	case model.SupportTeammateStatusAway:
		return 1
	default:
		return 2
	}
}

// hasOnlineSupportTeammate reports whether at least one support-accessible
// teammate is currently online (presence-based). It drives the widget's
// pre-chat IsOnline flag so availability reflects real presence, not just
// business hours.
func (s *SupportInboxService) hasOnlineSupportTeammate(ctx context.Context, workspaceID string, now time.Time) bool {
	return anySupportTeammateOnline(ctx, s.workspaceRepo, s.presence, s.statusOverrideRepo, workspaceID, now)
}

func (s *SupportInboxService) listWidgetTeammates(ctx context.Context, workspaceID string, limit int) []model.WidgetActiveTeammate {
	if s.workspaceRepo == nil {
		return []model.WidgetActiveTeammate{}
	}

	supportUserIDs, err := s.workspaceRepo.ListSupportAccessibleUserIDs(ctx, workspaceID)
	if err != nil || len(supportUserIDs) == 0 {
		return []model.WidgetActiveTeammate{}
	}
	supportUserIDSet := make(map[string]struct{}, len(supportUserIDs))
	for _, userID := range supportUserIDs {
		supportUserIDSet[userID] = struct{}{}
	}

	statusByUserID := map[string]string{}
	statuses, err := resolveSupportTeammatePresenceStatuses(
		ctx,
		s.workspaceRepo,
		s.presence,
		s.statusOverrideRepo,
		workspaceID,
		time.Now(),
	)
	if err == nil {
		for _, status := range statuses {
			statusByUserID[status.UserID] = status.Status
		}
	}

	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return []model.WidgetActiveTeammate{}
	}

	teammates := make([]model.WidgetActiveTeammate, 0, len(members))
	for _, member := range members {
		if strings.TrimSpace(member.UserID) == "" || strings.TrimSpace(member.FullName) == "" {
			continue
		}
		if _, ok := supportUserIDSet[member.UserID]; !ok {
			continue
		}
		status := statusByUserID[member.UserID]
		if status == "" {
			status = model.SupportTeammateStatusOffline
		}
		teammates = append(teammates, model.WidgetActiveTeammate{
			UserID:    member.UserID,
			Name:      member.FullName,
			AvatarURL: member.AvatarURL,
			Status:    status,
		})
	}

	sort.SliceStable(teammates, func(i, j int) bool {
		left := teammates[i]
		right := teammates[j]
		if supportTeammateStatusRank(left.Status) != supportTeammateStatusRank(right.Status) {
			return supportTeammateStatusRank(left.Status) < supportTeammateStatusRank(right.Status)
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})

	if limit > 0 && len(teammates) > limit {
		return teammates[:limit]
	}
	return teammates
}

func widgetActiveTeammateFromConversation(conversation *model.SupportConversation) *model.WidgetActiveTeammate {
	if conversation == nil || conversation.OpenedByUserID == nil || conversation.OpenedByDisplayName == nil {
		return nil
	}
	userID := strings.TrimSpace(*conversation.OpenedByUserID)
	name := strings.TrimSpace(*conversation.OpenedByDisplayName)
	if userID == "" || name == "" {
		return nil
	}
	return &model.WidgetActiveTeammate{
		UserID:    userID,
		Name:      name,
		AvatarURL: conversation.OpenedByAvatarURL,
		Status: func() string {
			if conversation.OpenedByStatus == nil {
				return ""
			}
			return strings.TrimSpace(*conversation.OpenedByStatus)
		}(),
	}
}

// buildWidgetConfigResponse maps installation settings to the nested WidgetConfig
// shape expected by the widget-core TypeScript interface.
func (s *SupportInboxService) buildWidgetConfigResponse(ctx context.Context, inst *model.SupportWidgetInstallation) (*model.WidgetConfigResponse, error) {
	settings := parseSettings(inst.Settings)

	position := settings.LauncherPosition
	if position == "" || position == "bottom_right" {
		position = "bottom-right"
	} else if position == "bottom_left" {
		position = "bottom-left"
	}

	primaryColor := settings.BrandColor
	if primaryColor == "" {
		primaryColor = "#6366f1"
	}

	helpSpaces, err := s.resolveWidgetHelpSpaces(ctx, inst.WorkspaceID, settings.WidgetHelpSpaceIDs)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	hasOnlineAgent := s.hasOnlineSupportTeammate(ctx, inst.WorkspaceID, now)

	return &model.WidgetConfigResponse{
		WorkspaceID:   inst.WorkspaceID,
		WorkspaceName: settings.WidgetName,
		Branding: model.WidgetConfigBranding{
			PrimaryColor:    primaryColor,
			LogoURL:         settings.LogoURL,
			WelcomeMessage:  settings.WelcomeMessage,
			WidgetPosition:  position,
			ShowBranding:    settings.ShowBranding,
			LauncherIcon:    settings.LauncherIcon,
			ColorScheme:     settings.ColorScheme,
			ButtonColor:     settings.ButtonColor,
			ButtonIconColor: settings.ButtonIconColor,
		},
		Features: model.WidgetConfigFeatures{
			AIEnabled:         settings.AIEnabled,
			AIFirst:           settings.AIEnabled && settings.AIResponseMode == "ai_first",
			ShowTalkToHuman:   settings.ShowTalkToHuman,
			EscalationMessage: settings.EscalationMessage,
			FileUploads:       settings.FileUploadsEnabled,
			PreChatForm:       settings.RequireEmailBeforeChat,
			RequirePhone:      settings.RequirePhoneAfterEmail,
			CSATRating:        settings.CSATEnabled,
			ForceIdentify:     settings.ForceVisitorIdentity,
		},
		Availability:       buildWidgetAvailability(settings, now, hasOnlineAgent),
		AvailableTeammates: s.listWidgetTeammates(ctx, inst.WorkspaceID, 5),
		HelpSpaces:         helpSpaces,
	}, nil
}

func (s *SupportInboxService) resolveWidgetHelpSpaces(ctx context.Context, workspaceID string, configuredIDs []string) ([]model.WidgetHelpSpace, error) {
	if len(configuredIDs) == 0 {
		return []model.WidgetHelpSpace{}, nil
	}
	if s.docsSpaceRepo == nil {
		return nil, fmt.Errorf("docs spaces repository not configured")
	}

	spaces, err := s.docsSpaceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]model.DocsSpace, len(spaces))
	for _, space := range spaces {
		if space.Type == model.SpaceTypeExternalCapable {
			byID[space.ID] = space
		}
	}

	result := make([]model.WidgetHelpSpace, 0, len(configuredIDs))
	seen := make(map[string]struct{}, len(configuredIDs))
	for _, id := range configuredIDs {
		if _, alreadySeen := seen[id]; alreadySeen {
			continue
		}
		space, ok := byID[id]
		if !ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, model.WidgetHelpSpace{
			ID:   space.ID,
			Name: space.Name,
			Slug: space.Slug,
			Icon: space.Icon,
		})
	}

	return result, nil
}

func (s *SupportInboxService) getAllowedWidgetHelpSpaces(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, []model.WidgetHelpSpace, error) {
	inst, err := s.installationRepo.GetByWidgetKey(ctx, widgetKey)
	if err != nil {
		return nil, nil, err
	}
	if inst == nil {
		return nil, nil, fmt.Errorf("widget not found")
	}

	settings := parseSettings(inst.Settings)
	spaces, err := s.resolveWidgetHelpSpaces(ctx, inst.WorkspaceID, settings.WidgetHelpSpaceIDs)
	if err != nil {
		return nil, nil, err
	}

	return inst, spaces, nil
}

func findWidgetHelpSpaceBySlug(spaces []model.WidgetHelpSpace, slug string) *model.WidgetHelpSpace {
	for i := range spaces {
		if spaces[i].Slug == slug {
			return &spaces[i]
		}
	}
	return nil
}

func widgetHelpSpaceIDs(spaces []model.WidgetHelpSpace) []string {
	ids := make([]string, 0, len(spaces))
	for _, space := range spaces {
		ids = append(ids, space.ID)
	}
	return ids
}

func (s *SupportInboxService) resolveWidgetHelpCollection(ctx context.Context, workspaceID, collectionKey string, allowedSpaces []model.WidgetHelpSpace) (*model.DocsCollection, error) {
	if s.docsCollectionRepo == nil {
		return nil, fmt.Errorf("docs collections repository not configured")
	}

	if _, publicID, ok := parseDocsHelpcenterCollectionKey(collectionKey); ok {
		return s.docsCollectionRepo.GetByPublicID(ctx, publicID)
	}

	allowedByID := make(map[string]struct{}, len(allowedSpaces))
	for _, space := range allowedSpaces {
		allowedByID[space.ID] = struct{}{}
	}

	collections, err := s.docsCollectionRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var match *model.DocsCollection
	trimmedKey := strings.TrimSpace(collectionKey)
	for i := range collections {
		collection := collections[i]
		if collection.Slug != trimmedKey {
			continue
		}
		if _, ok := allowedByID[collection.SpaceID]; !ok {
			continue
		}
		if match != nil {
			return nil, nil
		}
		candidate := collection
		match = &candidate
	}

	return match, nil
}

// ListWidgetHelpCollections returns widget-visible collections for a selected space.
func (s *SupportInboxService) ListWidgetHelpCollections(ctx context.Context, widgetKey, spaceSlug string) ([]model.WidgetHelpCollection, error) {
	_, allowedSpaces, err := s.getAllowedWidgetHelpSpaces(ctx, widgetKey)
	if err != nil {
		return nil, err
	}

	space := findWidgetHelpSpaceBySlug(allowedSpaces, spaceSlug)
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}
	if s.docsHelpcenterRepo == nil {
		return nil, fmt.Errorf("docs helpcenter repository not configured")
	}

	return s.docsHelpcenterRepo.ListWidgetCollections(ctx, space.ID)
}

// ListWidgetHelpArticles returns widget-visible articles for a selected collection.
func (s *SupportInboxService) ListWidgetHelpArticles(ctx context.Context, widgetKey, collectionSlug string) ([]model.WidgetHelpArticleSummary, error) {
	inst, allowedSpaces, err := s.getAllowedWidgetHelpSpaces(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if s.docsHelpcenterRepo == nil || s.docsCollectionRepo == nil {
		return nil, fmt.Errorf("docs repositories not configured")
	}

	if strings.HasPrefix(collectionSlug, "uncategorized:") {
		spaceID := strings.TrimPrefix(collectionSlug, "uncategorized:")
		for _, space := range allowedSpaces {
			if space.ID == spaceID {
				articles, err := s.docsHelpcenterRepo.ListWidgetArticlesBySpaceUncategorized(ctx, spaceID)
				if err != nil {
					return nil, err
				}
				return withWidgetArticleKeys(articles), nil
			}
		}
		return nil, fmt.Errorf("collection not found")
	}

	collection, err := s.resolveWidgetHelpCollection(ctx, inst.WorkspaceID, collectionSlug, allowedSpaces)
	if err != nil {
		return nil, err
	}
	if collection == nil || collection.WorkspaceID != inst.WorkspaceID {
		return nil, fmt.Errorf("collection not found")
	}

	allowedByID := make(map[string]struct{}, len(allowedSpaces))
	for _, space := range allowedSpaces {
		allowedByID[space.ID] = struct{}{}
	}
	if _, ok := allowedByID[collection.SpaceID]; !ok {
		return nil, fmt.Errorf("collection not found")
	}

	articles, err := s.docsHelpcenterRepo.ListWidgetArticlesByCollectionID(ctx, collection.ID)
	if err != nil {
		return nil, err
	}
	return withWidgetArticleKeys(articles), nil
}

// SearchWidgetHelpArticles searches published help-center articles in the docs spaces
// selected for the widget.
// SearchWidgetHelpArticles searches widget-visible help articles. anonymousID is
// the caller's durable browser identity when known; it is optional, and is
// recorded on the resulting support event so that self-service searches stay
// attributable to the visitor who made them.
func (s *SupportInboxService) SearchWidgetHelpArticles(ctx context.Context, widgetKey, query string, limit int, anonymousID string, coverageSignal bool) ([]model.WidgetHelpSearchResult, error) {
	inst, allowedSpaces, err := s.getAllowedWidgetHelpSpaces(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if s.docsSearchRepo == nil || s.docsHelpcenterRepo == nil {
		return nil, fmt.Errorf("docs search repository not configured")
	}

	cfg, err := s.docsHelpcenterRepo.GetConfig(ctx, inst.WorkspaceID)
	if err != nil {
		return nil, err
	}
	locale := defaultHelpcenterLocale(cfg)
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	results := make([]model.WidgetHelpSearchResult, 0, limit)
	seen := make(map[string]struct{})
	for _, space := range allowedSpaces {
		if len(results) >= limit {
			break
		}
		spaceResults, err := s.docsSearchRepo.PublicSearch(ctx, inst.WorkspaceID, locale, query, space.Slug, limit)
		if err != nil {
			return nil, err
		}
		for _, result := range spaceResults {
			if len(results) >= limit {
				break
			}
			articleKey := buildDocsHelpcenterArticleKey(result.Slug, result.PublicID)
			if articleKey == "" {
				articleKey = result.ID
			}
			if _, ok := seen[articleKey]; ok {
				continue
			}
			seen[articleKey] = struct{}{}
			results = append(results, model.WidgetHelpSearchResult{
				ID:             result.ID,
				Title:          result.Title,
				Slug:           result.Slug,
				PublicID:       result.PublicID,
				ArticleKey:     articleKey,
				Excerpt:        result.Excerpt,
				CollectionName: result.CollectionName,
				SpaceName:      result.SpaceName,
			})
		}
	}

	searchSourceSignal := model.SupportCoverageSourceSelfService
	if coverageSignal && len(results) == 0 && IsEligibleCoverageWidgetSignal(query) {
		searchSourceSignal = "no_results"
	}
	s.recordSupportEvent(SupportEventInput{
		WorkspaceID:  inst.WorkspaceID,
		EventType:    model.SupportEventWidgetSearchPerformed,
		AnonymousID:  NormalizeVisitorAnonymousID(anonymousID),
		ActorType:    model.SupportEventActorCustomer,
		Channel:      "widget",
		SourceSignal: searchSourceSignal,
		IssueSummary: query,
		Metadata: map[string]any{
			"query":                    query,
			"result_count":             len(results),
			"surface":                  "widget_help",
			"coverage_signal_eligible": coverageSignal,
		},
	})

	return results, nil
}

// GetWidgetHelpArticle returns a widget-visible article with rendered HTML content.
func (s *SupportInboxService) GetWidgetHelpArticle(ctx context.Context, widgetKey, articleKey string) (*model.WidgetHelpArticle, error) {
	inst, allowedSpaces, err := s.getAllowedWidgetHelpSpaces(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if s.docsHelpcenterRepo == nil {
		return nil, fmt.Errorf("docs helpcenter repository not configured")
	}

	spaceIDs := widgetHelpSpaceIDs(allowedSpaces)
	var (
		doc     *model.DocsDocument
		article *model.DocsHelpcenterArticle
		content *model.DocsContent
	)
	if _, publicID, ok := parseDocsHelpcenterArticleKey(articleKey); ok {
		doc, article, content, err = s.docsHelpcenterRepo.GetPublicArticleByPublicIDInSpaces(ctx, spaceIDs, publicID)
	} else {
		doc, article, content, err = s.docsHelpcenterRepo.GetPublicArticleByDocumentIDInSpaces(ctx, spaceIDs, articleKey)
	}
	if err != nil {
		return nil, err
	}
	if doc == nil || article == nil {
		return nil, fmt.Errorf("article not found")
	}

	var contentJSON json.RawMessage
	if content != nil {
		contentJSON = content.Content
	}

	publicPath, err := s.buildWidgetHelpArticlePublicPath(ctx, inst.WorkspaceID, article)
	if err != nil {
		return nil, err
	}

	slug := strings.TrimSpace(article.Slug)
	publicID := strings.TrimSpace(article.PublicID)

	return &model.WidgetHelpArticle{
		ID:          doc.ID,
		Title:       doc.Title,
		Slug:        slug,
		PublicID:    publicID,
		ArticleKey:  buildDocsHelpcenterArticleKey(slug, publicID),
		Excerpt:     doc.Excerpt,
		Icon:        doc.Icon,
		ContentHTML: renderWidgetArticleHTML(contentJSON),
		PublicPath:  publicPath,
	}, nil
}

func withWidgetArticleKeys(articles []model.WidgetHelpArticleSummary) []model.WidgetHelpArticleSummary {
	for i := range articles {
		articles[i].ArticleKey = buildDocsHelpcenterArticleKey(articles[i].Slug, articles[i].PublicID)
	}
	return articles
}

func (s *SupportInboxService) buildWidgetHelpArticlePublicPath(ctx context.Context, workspaceID string, article *model.DocsHelpcenterArticle) (*string, error) {
	if article == nil || s.docsHelpcenterRepo == nil {
		return nil, nil
	}

	cfg, err := s.docsHelpcenterRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get help center config: %w", err)
	}
	if cfg == nil || strings.TrimSpace(cfg.Subdomain) == "" {
		return nil, nil
	}

	articleSlug := strings.TrimSpace(article.Slug)
	publicID := strings.TrimSpace(article.PublicID)
	if articleSlug == "" || publicID == "" {
		return nil, nil
	}

	articleKey := buildDocsHelpcenterArticleKey(articleSlug, publicID)

	// Only build a public URL when the workspace has a custom domain configured.
	// Without a custom domain the help center may not be publicly reachable,
	// so we return nil and the widget hides the external link.
	if cfg.CustomDomain == nil || strings.TrimSpace(*cfg.CustomDomain) == "" {
		return nil, nil
	}

	domain := strings.TrimSpace(*cfg.CustomDomain)
	if !strings.HasPrefix(domain, "http") {
		domain = "https://" + domain
	}
	baseURL := strings.TrimRight(domain, "/")

	path := fmt.Sprintf("%s/articles/%s", baseURL, articleKey)
	return &path, nil
}

// ListWidgetTokens returns all active widget installations formatted as tokens
// for the events-pipeline rust-capture service.
func (s *SupportInboxService) ListWidgetTokens(ctx context.Context) ([]model.WidgetToken, error) {
	installations, err := s.installationRepo.ListAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list widget tokens: %w", err)
	}

	tokens := make([]model.WidgetToken, 0, len(installations))
	for _, inst := range installations {
		workspaceID, err := uuid.Parse(strings.TrimSpace(inst.WorkspaceID))
		if err != nil {
			return nil, fmt.Errorf("widget installation %q has invalid workspace ID", inst.ID)
		}
		tokens = append(tokens, model.WidgetToken{
			ID:                       inst.ID,
			WorkspaceID:              strings.ToLower(workspaceID.String()),
			ClientSecret:             inst.WidgetKey,
			ServerSecret:             inst.SecretKey,
			Origins:                  []string(inst.AllowedOrigins),
			IdentityVerificationMode: inst.IdentityVerificationMode,
		})
	}
	return tokens, nil
}
