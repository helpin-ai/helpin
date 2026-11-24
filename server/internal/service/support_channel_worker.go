package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// SetInboxService lets the channel worker store messages and attachments
// through the support inbox.
func (s *SupportChannelService) SetInboxService(inbox *SupportInboxService) *SupportChannelService {
	s.inbox = inbox
	return s
}

// StartWorker processes inbound jobs of one channel kind until ctx ends.
func (s *SupportChannelService) StartWorker(ctx context.Context, kind string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Drain what is due before waiting again.
			for {
				processed, err := s.processNextJob(ctx, kind)
				if err != nil {
					s.logger.ErrorContext(ctx, "channel worker failed", "kind", kind, "error", err)
				}
				if !processed || ctx.Err() != nil {
					break
				}
			}
		}
	}
}

// processNextJob claims and processes one job; processed reports whether a
// job was claimed.
func (s *SupportChannelService) processNextJob(ctx context.Context, kind string) (bool, error) {
	ctx, cancelJob := context.WithTimeout(ctx, 90*time.Second)
	defer cancelJob()
	job, err := s.jobs.Claim(ctx, kind, s.now())
	if err != nil || job == nil {
		return false, err
	}
	workCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	var workErr error
	switch kind {
	case "slack_shortcut":
		workErr = s.slack.shortcutJob(workCtx, job)
	case SlackPMDeliveryJob:
		workErr = s.slack.deliverDiscussion(workCtx, job)
	case SlackAgentReplyJob:
		workErr = s.slack.deliverAgentAnswer(workCtx, job)
	case SlackEventJob:
		workErr = s.slack.process(workCtx, job)
	case SupportInboundKindChannelEvent:
		workErr = s.processEventJob(workCtx, job)
	case SupportInboundKindChannelAttachment:
		workErr = s.processAttachmentJob(workCtx, job)
	default:
		workErr = fmt.Errorf("unknown channel job kind %q", kind)
	}
	cancel()
	// Shutdown leaves a recoverable lease, never a false completion.
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	if errors.Is(workErr, errSlackAgentPending) {
		return true, s.jobs.WaitForResult(ctx, job, s.now().Add(3*time.Second))
	}
	if workErr != nil {
		s.logger.ErrorContext(ctx, "channel job failed", "job_id", job.ID, "kind", kind, "attempt", job.Attempts, "error", workErr)
		if (kind == SlackEventJob || kind == SlackAgentReplyJob) && s.slack.reportRequestFailure(ctx, job, workErr) {
			workErr = &repository.InboundUndeliverableError{Reason: "Slack request failed after retry; requester notified"}
		}
		if kind == SupportInboundKindChannelAttachment {
			s.markAttachmentRetrying(ctx, job)
		}
	}
	return true, s.jobs.Finish(ctx, job, workErr, s.now())
}

func (s *SupportChannelService) processEventJob(ctx context.Context, job *model.SupportInboundJob) error {
	var payload channelEventPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode channel event: %w", err)
	}
	account, adapter, err := s.webhookAccount(ctx, payload.Provider, payload.AccountID)
	if err != nil || account == nil {
		// A disconnected account's queued events are dropped.
		return err
	}
	if s.inbox == nil {
		return errors.New("support inbox unavailable")
	}
	receipt, err := s.inbox.receiveChannelMessage(ctx, account, payload.Event, adapter.Capabilities().MaxDownloadSize)
	if err != nil {
		return err
	}
	if receipt.duplicate {
		return nil
	}
	// Linking the job to its conversation lets the privacy trigger clear it.
	if err := s.jobs.SetConversation(ctx, job, receipt.message.ConversationID, receipt.message.ID); err != nil {
		s.logger.WarnContext(ctx, "link channel job to conversation", "error", err, "job_id", job.ID)
	}
	s.inbox.publishChannelMessage(ctx, account, payload.Event, receipt)
	return nil
}

func (s *SupportChannelService) processAttachmentJob(ctx context.Context, job *model.SupportInboundJob) error {
	var payload channelAttachmentPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode channel attachment: %w", err)
	}
	if s.inbox == nil || s.inbox.attachmentService == nil {
		return errors.New("attachment storage unavailable")
	}
	account, err := s.accounts.Get(ctx, payload.WorkspaceID, payload.AccountID)
	if err != nil {
		return err
	}
	adapter, ok := s.adapters.Lookup(account.Provider)
	if !ok && account.Provider != "slack" {
		return fmt.Errorf("%w: unsupported provider", ErrChannelInvalid)
	}
	token, err := s.decryptToken(account)
	if err != nil {
		return err
	}
	var data []byte
	if account.Provider == "slack" {
		var file slack.File
		file, data, err = s.slack.api.DownloadFile(ctx, token, payload.FileID, 25<<20)
		payload.FileName = firstNonEmptyString(file.Name, payload.FileName, "Attachment")
		payload.ContentType = firstNonEmptyString(file.Mime, payload.ContentType)
		if errors.Is(err, slack.ErrFileTooLarge) {
			err = rejectSupportAttachment("%s exceeds the %s download limit.", payload.FileName, formatByteLimit(25<<20))
		}
	} else {
		data, err = adapter.DownloadFile(ctx, token, payload.FileID, adapter.Capabilities().MaxDownloadSize)
	}
	var stored *model.SupportAttachmentPayload
	if err == nil {
		stored, err = s.inbox.attachmentService.StoreInboundAttachmentBytes(ctx, supportInboundEmailAttachmentRequest{
			AttachmentID: payload.AttachmentID, WorkspaceID: payload.WorkspaceID, ConversationID: payload.ConversationID,
			MessageID: payload.MessageID, FileName: payload.FileName, ContentType: payload.ContentType,
		}, data)
	}
	if message, rejected := IsSupportAttachmentRejected(err); rejected {
		// A refused file is refused again on every retry, so finish the job.
		return s.inbox.conversationRepo.DB().WithContext(ctx).Model(&model.SupportAttachment{}).
			Where("id = ? AND workspace_id = ? AND is_uploaded = false", payload.AttachmentID, payload.WorkspaceID).
			Updates(map[string]any{"processing_status": supportAttachmentStatusRejected, "processing_error": message, "file_name": payload.FileName}).Error
	}
	if err != nil {
		return err
	}
	if err := repository.NewSupportAttachmentRepository(s.inbox.conversationRepo.DB()).CompleteInbound(ctx, &model.SupportAttachment{
		ID: payload.AttachmentID, WorkspaceID: payload.WorkspaceID, MessageID: &payload.MessageID,
		FileName: stored.FileName, StorageKey: stored.FileKey, FileSize: stored.FileSize, ContentType: stored.FileType, PublicURL: stored.URL,
	}); err != nil {
		return err
	}
	s.inbox.wsPublisher.Publish(websocket.Event{
		Action: "updated", Entity: "support_conversation_message", EntityID: payload.MessageID,
		WorkspaceID: payload.WorkspaceID, ActorID: "channel-attachment", ParentType: "support_conversation", ParentID: payload.ConversationID,
	})
	return nil
}

// markAttachmentRetrying shows the file as still processing, or failed once
// the job has used its attempts.
func (s *SupportChannelService) markAttachmentRetrying(ctx context.Context, job *model.SupportInboundJob) {
	if s.inbox == nil {
		return
	}
	state := "processing"
	if job.Attempts >= 10 {
		state = "failed"
	}
	if err := s.inbox.conversationRepo.DB().WithContext(ctx).Model(&model.SupportAttachment{}).
		Where("id = ? AND is_uploaded = false", job.ID).
		Updates(map[string]any{"processing_status": state, "processing_error": "Attachment could not be downloaded from Telegram. Retry or ask the customer to send it again."}).Error; err != nil {
		s.logger.WarnContext(ctx, "mark channel attachment state", "error", err, "attachment_id", job.ID)
	}
}

// webhookAccount loads a live account and its adapter, or nil when the account
// was disconnected after the event arrived.
func (s *SupportChannelService) webhookAccount(ctx context.Context, provider, accountID string) (*model.SupportChannelAccount, channels.Adapter, error) {
	adapter, ok := s.adapters.Lookup(provider)
	if !ok {
		return nil, nil, nil
	}
	account, err := s.accounts.GetForWebhook(ctx, provider, accountID)
	if err != nil || account == nil {
		return nil, nil, err
	}
	return account, adapter, nil
}
