package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// SupportInboundKindChannelAttachment downloads a customer file from the
// provider after the message is saved.
const SupportInboundKindChannelAttachment = "channel_attachment"

// _channelReopenWindow: a message within this long after resolving reopens the
// conversation; later ones start a new conversation, as Intercom does for
// Facebook, Instagram and SMS.
const _channelReopenWindow = 5 * 24 * time.Hour

// channelAttachmentPayload is the stored job payload for one customer file.
type channelAttachmentPayload struct {
	AccountID      string `json:"account_id"`
	FileID         string `json:"file_id"`
	AttachmentID   string `json:"attachment_id"`
	WorkspaceID    string `json:"workspace_id"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	FileName       string `json:"file_name"`
	ContentType    string `json:"content_type"`
	Size           int64  `json:"size"`
}

// channelReceipt is what processing an event produced.
type channelReceipt struct {
	message             *model.SupportMessage
	reopened            *model.SupportMessage
	conversationCreated bool
	conversationChanged bool
	duplicate           bool
}

// receiveChannelMessage stores a provider event as a customer message: it
// records the person, finds their open conversation (or one resolved within
// the reopen window) or starts one, and queues file downloads. Everything
// commits together; side effects run after commit.
func (s *SupportInboxService) receiveChannelMessage(ctx context.Context, account *model.SupportChannelAccount, event channels.Event, maxDownload int64) (*channelReceipt, error) {
	mailboxID, ownerID, flowState, err := s.channelRouting(ctx, account)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	receipt := &channelReceipt{}
	err = s.conversationRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// One customer's concurrent first messages must open one conversation.
		subject := event.ExternalUserID
		if event.ThreadKey != "" {
			subject = event.ThreadKey
		}
		if err := lockChannelSubject(tx, model.SupportChannelSubjectKey(account.ID, subject)); err != nil {
			return err
		}
		if err := repository.LockContactAnonymization(tx, account.WorkspaceID); err != nil {
			return err
		}
		var existing model.SupportMessage
		if err := tx.Where("channel_account_id = ? AND external_message_key = ?", account.ID, event.ExternalMessageKey).
			Limit(1).Find(&existing).Error; err != nil {
			return err
		}
		if existing.ID != "" {
			receipt.duplicate, receipt.message = true, &existing
			return nil
		}
		var conv *model.SupportConversation
		if event.ThreadKey != "" {
			var thread model.SupportConversation
			err = tx.Where("workspace_id=? AND channel_account_id=? AND provider_thread_key=?", account.WorkspaceID, account.ID, event.ThreadKey).Limit(1).Find(&thread).Error
			if thread.ID != "" {
				if thread.AnonymizedAt != nil || thread.Status == model.SupportConversationStatusSpam {
					receipt.duplicate = true
					return nil
				}
				conv = &thread
			} else if err == nil && event.ExistingThreadOnly {
				receipt.duplicate = true
				return nil
			}
		}
		if err != nil {
			return err
		}
		identity, err := repository.UpsertSupportChannelIdentityTx(ctx, tx, &model.SupportChannelIdentity{
			WorkspaceID: account.WorkspaceID, ChannelAccountID: account.ID, ExternalUserID: event.ExternalUserID,
			DisplayName: event.DisplayName, Username: event.Username,
		}, now)
		if err != nil {
			return err
		}
		displayName := channelCustomerName(identity)
		if event.ThreadKey == "" {
			conv, err = s.channelConversationTx(ctx, tx, identity, now)
		}
		if err != nil {
			return err
		}
		msg := &model.SupportMessage{
			WorkspaceID: account.WorkspaceID, SenderType: "customer", SenderDisplayName: &displayName,
			Content: channelMessageContent(event), MessageType: "reply", ViaChannel: strPtr(account.Provider),
			ChannelAccountID: &account.ID, ExternalMessageKey: &event.ExternalMessageKey,
		}
		if event.Teammate {
			msg.SenderType = "user"
			msg.TrustedInbound = true
		}
		if conv == nil {
			conv = &model.SupportConversation{
				WorkspaceID: account.WorkspaceID, Subject: channelSubject(event), Status: model.SupportConversationStatusOpen,
				FlowState: strPtr(flowState), Priority: "medium", Channel: account.Provider, Source: account.Provider,
				CustomerName: &displayName, CRMContactID: identity.CRMContactID, MailboxID: mailboxID, AssignedUserID: ownerID,
				ChannelAccountID: &account.ID, ChannelIdentityID: &identity.ID, ProviderThreadKey: nilIfEmpty(event.ThreadKey), ProviderChannelName: nilIfEmpty(event.ChannelName),
			}
			if err := s.conversationRepo.WithTx(tx).Create(ctx, conv); err != nil {
				return err
			}
			if event.ThreadKey != "" {
				if err := tx.Table("support_conversations").Where("id=?", conv.ID).Updates(map[string]any{"provider_thread_key": event.ThreadKey, "provider_channel_name": event.ChannelName}).Error; err != nil {
					return err
				}
			}
			if identity.CRMContactID != nil {
				// Shows the conversation on the contact, as linking by hand does.
				if err := tx.Create(&model.CRMAssociation{WorkspaceID: account.WorkspaceID, FromObjectType: model.CRMObjectSupportConversation,
					FromObjectID: conv.ID, ToObjectType: model.CRMObjectContact, ToObjectID: *identity.CRMContactID}).Error; err != nil {
					return err
				}
			}
			msg.ConversationID = conv.ID
			if err := s.messageRepo.WithTx(tx).Create(ctx, msg); err != nil {
				return err
			}
			receipt.conversationCreated = true
		} else {
			msg.ConversationID = conv.ID
			if event.Teammate {
				if err = s.messageRepo.WithTx(tx).Create(ctx, msg); err != nil {
					return err
				}
			} else if receipt.reopened, receipt.conversationChanged, err = s.createCustomerReplyTx(ctx, tx, msg); err != nil {
				return err
			}
		}
		receipt.message = msg
		return enqueueChannelAttachments(ctx, tx, account, msg, event.Files, maxDownload, now)
	})
	if err != nil {
		if isUniqueConstraintViolation(err) {
			// Another worker stored the same provider message first.
			return &channelReceipt{duplicate: true}, nil
		}
		return nil, err
	}
	return receipt, nil
}

// channelRouting places a new conversation: the bot's inbox when set,
// otherwise the workspace default inbox. Triage rules run after the message
// and can still move it.
func (s *SupportInboxService) channelRouting(ctx context.Context, account *model.SupportChannelAccount) (*string, *string, string, error) {
	mailboxID, mailbox, err := s.maybeApplyMailboxRoutingForChannel(ctx, account.WorkspaceID, account.MailboxID, true, account.Provider)
	if err != nil {
		return nil, nil, "", err
	}
	flowState := model.SupportConversationFlowStateWaitingForHuman
	var ownerID *string
	if mailbox != nil {
		if ownerID, flowState, err = s.determineMailboxOwner(ctx, account.WorkspaceID, mailbox, nil); err != nil {
			return nil, nil, "", err
		}
	}
	return mailboxID, ownerID, flowState, nil
}

// channelConversationTx returns the customer's conversation to continue, or
// nil to start a new one.
func (s *SupportInboxService) channelConversationTx(ctx context.Context, tx *gorm.DB, identity *model.SupportChannelIdentity, now time.Time) (*model.SupportConversation, error) {
	var conv model.SupportConversation
	err := tx.WithContext(ctx).Where("workspace_id = ? AND channel_identity_id = ?", identity.WorkspaceID, identity.ID).
		Where("status <> ? AND merged_into_id IS NULL AND anonymized_at IS NULL", model.SupportConversationStatusSpam).
		Order("created_at DESC, id DESC").Limit(1).Find(&conv).Error
	if err != nil || conv.ID == "" {
		return nil, err
	}
	if conv.Status == model.SupportConversationStatusResolved {
		resolved := conv.UpdatedAt
		if conv.ResolvedAt != nil {
			resolved = *conv.ResolvedAt
		}
		if now.Sub(resolved) > _channelReopenWindow {
			return nil, nil
		}
	}
	return &conv, nil
}

func enqueueChannelAttachments(ctx context.Context, tx *gorm.DB, account *model.SupportChannelAccount, msg *model.SupportMessage, files []channels.InboundFile, maxDownload int64, now time.Time) error {
	for i, file := range files {
		id := inboundStableID(fmt.Sprintf("channel-attachment:%s:%s:%d", account.ID, derefString(msg.ExternalMessageKey), i))
		attachment := model.SupportAttachment{
			ID: id, WorkspaceID: msg.WorkspaceID, ConversationID: &msg.ConversationID, MessageID: &msg.ID,
			FileName: file.FileName, FileSize: file.Size, ContentType: file.ContentType, UploadedByType: "customer",
			ProcessingStatus: "processing",
		}
		tooLarge := maxDownload > 0 && file.Size > maxDownload
		if tooLarge {
			attachment.ProcessingStatus = supportAttachmentStatusRejected
			attachment.ProcessingError = fmt.Sprintf("%s exceeds the %s download limit. Ask the customer to send it another way.", file.FileName, formatByteLimit(maxDownload))
		}
		if err := tx.WithContext(ctx).Create(&attachment).Error; err != nil {
			return err
		}
		msg.Attachments = append(msg.Attachments, model.SupportAttachmentPayload{ID: id, FileName: file.FileName, FileType: file.ContentType, FileSize: file.Size, ProcessingStatus: attachment.ProcessingStatus})
		if tooLarge {
			continue
		}
		payload, err := json.Marshal(channelAttachmentPayload{
			AccountID: account.ID, FileID: file.FileID, AttachmentID: id, WorkspaceID: msg.WorkspaceID,
			ConversationID: msg.ConversationID, MessageID: msg.ID, FileName: file.FileName, ContentType: file.ContentType, Size: file.Size,
		})
		if err != nil {
			return err
		}
		if err := repository.NewSupportInboundJobRepository(tx).Create(ctx, &model.SupportInboundJob{
			ID: id, Kind: SupportInboundKindChannelAttachment, WorkspaceID: &msg.WorkspaceID, ConversationID: &msg.ConversationID,
			MessageID: &msg.ID, ChannelAccountID: &account.ID, Payload: string(payload), Status: "pending", AvailableAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

// publishChannelMessage runs the side effects of a stored customer message, in
// the widget's order: realtime events, link previews, the support event, the
// teammate notification, then triage and AI.
func (s *SupportInboxService) publishChannelMessage(ctx context.Context, account *model.SupportChannelAccount, event channels.Event, receipt *channelReceipt) {
	msg := receipt.message
	origin := account.Provider + ":" + account.ID
	if receipt.conversationCreated {
		s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "support_conversation", EntityID: msg.ConversationID, WorkspaceID: msg.WorkspaceID})
	}
	if receipt.conversationChanged {
		s.publishConversationReopened(msg)
	}
	s.wsPublisher.Publish(websocket.SupportMessageEvent(msg.WorkspaceID, msg, origin))
	if receipt.reopened != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(msg.WorkspaceID, receipt.reopened, origin))
	}
	if event.Teammate {
		return
	}
	s.enrichSupportMessageLinksAsync(msg, origin)
	s.recordSupportEvent(SupportEventInput{
		WorkspaceID: msg.WorkspaceID, EventType: model.SupportEventCustomerMessageCreated,
		ConversationID: &msg.ConversationID, MessageID: &msg.ID,
		ActorType: model.SupportEventActorCustomer, Channel: account.Provider,
	})
	if conv, err := s.conversationRepo.GetByID(ctx, msg.WorkspaceID, msg.ConversationID, "", model.RoleOwner); err == nil && conv != nil {
		ProcessSupportCustomerReplyNotification(ctx, s.notificationService, s.pushSenderService, conv, msg.Content, derefString(msg.SenderDisplayName), msg.ID)
	}
	// Something Helpin cannot show is not a question for the AI agent.
	if event.Kind != channels.EventMessage {
		return
	}
	// Slack's assistant setting covers teammate drafts and private answers.
	// Public customer replies are sent explicitly by an agent.
	if account.Provider == "slack" {
		if s.triageService != nil {
			go func() {
				ctx := context.WithoutCancel(ctx)
				if _, err := s.triageService.EvaluateAndRoute(ctx, msg.WorkspaceID, msg.ConversationID, msg.ID); err != nil {
					slog.ErrorContext(ctx, "Slack support triage failed", "workspace_id", msg.WorkspaceID, "conversation_id", msg.ConversationID, "error", err)
				}
			}()
		}
		return
	}
	go s.runWidgetPostMessageAutomation(context.WithoutCancel(ctx), msg.WorkspaceID, msg.ConversationID, msg.ID, msg.Content)
}

func lockChannelSubject(tx *gorm.DB, subjectKey string) error {
	if tx.Dialector.Name() != "postgres" {
		return nil // SQLite is used only by isolated unit tests.
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "support-channel-subject:"+subjectKey).Error
}

func channelCustomerName(identity *model.SupportChannelIdentity) string {
	if name := strings.TrimSpace(identity.DisplayName); name != "" {
		return name
	}
	if username := strings.TrimSpace(identity.Username); username != "" {
		return "@" + username
	}
	return "Telegram user"
}

func channelMessageContent(event channels.Event) string {
	if event.Kind == channels.EventUnsupported {
		return fmt.Sprintf("_Sent a %s on Telegram, which Helpin can't display._", event.UnsupportedType)
	}
	return strings.TrimSpace(event.Text)
}

func channelSubject(event channels.Event) string {
	if text := strings.TrimSpace(event.Text); text != "" {
		return truncate(text, 100)
	}
	if len(event.Files) > 0 {
		return "Attachment"
	}
	return "Telegram message"
}
