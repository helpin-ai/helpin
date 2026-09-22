package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log/slog"
	"strings"
	"time"
)

type supportSendProgressKey struct{}
type supportSendGuardKey struct{}

func reportSupportSendProgress(ctx context.Context, status string) {
	if fn, ok := ctx.Value(supportSendProgressKey{}).(func(string)); ok {
		fn(status)
	}
}
func (s *SupportInboxService) QueueSupportSend(ctx context.Context, ws, conv, user string, req model.CreateMessageRequest) (*model.SupportMessage, error) {
	if _, err := uuid.Parse(req.ClientMessageID); err != nil {
		return nil, ErrSupportTranslation
	}
	if req.IsInternal || (req.MessageType != "" && req.MessageType != "reply") || (strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0) {
		return nil, ErrSupportTranslation
	}
	if err := validateSupportDeliveryMode(&req, "user"); err != nil {
		return nil, err
	}
	if len(req.AttachmentIDs) > 0 {
		if s.attachmentService == nil {
			return nil, ErrSupportTranslation
		}
		if _, err := s.attachmentService.PendingReplyAttachments(ctx, ws, conv, user, req.AttachmentIDs); err != nil {
			return nil, err
		}
	}
	options, err := s.TranslationOptions(ctx, ws, conv, user)
	if err != nil {
		return nil, err
	}
	conversation, err := s.loadConversationAccessible(ctx, ws, conv)
	if err != nil || conversation == nil {
		return nil, ErrSupportTranslation
	}
	target := options.Conversation.CustomerLanguage
	if target == "" {
		target = options.DetectedCustomerLanguage
	}
	recipient := strings.ToLower(strings.TrimSpace(derefString(conversation.CustomerEmail)))
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(ws+":"+conv+":"+user+":"+req.ClientMessageID)).String()
	candidate := model.SupportPendingSend{ID: id, WorkspaceID: ws, ConversationID: conv, UserID: user, Request: string(raw), RequestHash: translationHash(string(raw)), RecipientEmail: recipient, TargetLanguage: target, Revision: options.Conversation.Revision, Status: "queued"}
	db := s.messageRepo.DB().WithContext(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return nil, err
	}
	var stored model.SupportPendingSend
	if err := db.Where("id = ? AND user_id = ?", id, user).First(&stored).Error; err != nil {
		return nil, err
	}
	if stored.RequestHash != translationHash(string(raw)) {
		return nil, ErrSupportTranslation
	}
	if stored.Status == "sent" {
		return s.messageRepo.GetByID(ctx, stored.MessageID)
	}
	result := s.pendingSendMessage(&stored)
	if s.attachmentService != nil {
		result.Attachments, err = s.attachmentService.PendingReplyAttachments(ctx, ws, conv, user, req.AttachmentIDs)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (s *SupportInboxService) pendingSendMessage(job *model.SupportPendingSend) *model.SupportMessage {
	var req model.CreateMessageRequest
	if json.Unmarshal([]byte(job.Request), &req) != nil {
		return nil
	}
	metadata := withSupportDeliveryMode(mergeSupportMessageMetadata("{}", req.Channels, req.CCEmails, req.BCCEmails), req.DeliveryMode)
	return &model.SupportMessage{Metadata: metadata, ID: "pending-" + job.ID, ClientMessageID: req.ClientMessageID, WorkspaceID: job.WorkspaceID, ConversationID: job.ConversationID, SenderType: "user", SenderUserID: &job.UserID, Content: req.Content, MessageType: "reply", CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt, PendingSend: job.Status, PendingFailure: job.Failure, PendingSendID: job.ID, PendingRequest: &req}
}
func (s *SupportInboxService) PendingSupportSends(ctx context.Context, ws, conv, user string) ([]*model.SupportMessage, error) {
	if c, err := s.loadConversationAccessible(ctx, ws, conv); err != nil || c == nil || c.AnonymizedAt != nil {
		return nil, ErrSupportTranslation
	}
	jobs := []model.SupportPendingSend{}
	err := s.messageRepo.DB().WithContext(ctx).Where("workspace_id = ? AND conversation_id = ? AND user_id = ? AND (status NOT IN ('sent','dismissed') OR (status='sent' AND updated_at > ?))", ws, conv, user, time.Now().Add(-2*time.Minute)).Order("created_at ASC").Find(&jobs).Error
	result := make([]*model.SupportMessage, 0, len(jobs))
	for i := range jobs {
		if jobs[i].Status == "sent" {
			m, e := s.messageRepo.GetByID(ctx, jobs[i].MessageID)
			if e != nil {
				return nil, e
			}
			if m != nil {
				rows := []model.SupportMessage{*m}
				if s.attachmentService != nil {
					_ = s.attachmentService.HydrateMessages(ctx, rows)
				}
				result = append(result, &rows[0])
			}
			continue
		}
		if m := s.pendingSendMessage(&jobs[i]); m != nil {
			if s.attachmentService != nil {
				attachments, attachmentErr := s.attachmentService.PendingReplyAttachments(ctx, ws, conv, user, m.PendingRequest.AttachmentIDs)
				if attachmentErr != nil {
					return nil, attachmentErr
				}
				m.Attachments = attachments
			}
			result = append(result, m)
		}
	}
	return result, err
}
func (s *SupportInboxService) RetrySupportSend(ctx context.Context, ws, conv, user, id, action string) error {
	options, err := s.TranslationOptions(ctx, ws, conv, user)
	if err != nil {
		return err
	}
	conversation, err := s.loadConversationAccessible(ctx, ws, conv)
	if err != nil || conversation == nil {
		return ErrSupportTranslation
	}
	return s.messageRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job model.SupportPendingSend
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ? AND conversation_id = ? AND user_id = ? AND status = 'failed'", id, ws, conv, user).First(&job).Error; err != nil {
			return err
		}
		status := "queued"
		var req model.CreateMessageRequest
		if err := json.Unmarshal([]byte(job.Request), &req); err != nil {
			return err
		}
		switch action {
		case "original":
			if job.Failure != "translation" {
				return ErrSupportTranslation
			}
			req.SendOriginal = true
		case "retry":
		case "dismiss":
			status = "dismissed"
		default:
			return ErrSupportTranslation
		}
		raw, err := json.Marshal(req)
		if err != nil {
			return err
		}
		return tx.Model(&job).Updates(map[string]any{"status": status, "request": string(raw), "revision": options.Conversation.Revision, "target_language": supportReplyTarget(options), "recipient_email": strings.ToLower(strings.TrimSpace(derefString(conversation.CustomerEmail))), "failure": "", "updated_at": time.Now().UTC()}).Error
	})
}
func (s *SupportInboxService) RunPendingSupportSends(ctx context.Context) {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			var job model.SupportPendingSend
			err := s.messageRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				q := tx.Where("status = 'queued' OR (status IN ('preparing','translating','sending') AND updated_at < ?)", time.Now().Add(-2*time.Minute)).Order("created_at ASC")
				if tx.Dialector.Name() == "postgres" {
					q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
				}
				if err := q.First(&job).Error; err != nil {
					return err
				}
				job.Attempts++
				return tx.Model(&job).Updates(map[string]any{"status": "preparing", "attempts": job.Attempts, "updated_at": time.Now().UTC()}).Error
			})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				slog.WarnContext(ctx, "claim support send failed")
				continue
			}
			s.executePendingSupportSend(ctx, &job)
		}
	}
}
func (s *SupportInboxService) executePendingSupportSend(ctx context.Context, job *model.SupportPendingSend) {
	work, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	progress := func(status string) {
		dbctx, stop := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer stop()
		updates := map[string]any{"status": status, "failure": job.Failure, "message_id": job.MessageID, "updated_at": time.Now().UTC()}
		if status == "sent" {
			updates["request"] = ""
		}
		result := s.messageRepo.DB().WithContext(dbctx).Model(job).Where("attempts = ? AND status NOT IN ('sent','dismissed','failed')", job.Attempts).Updates(updates)
		if result.Error != nil || result.RowsAffected != 1 {
			cancel()
			slog.WarnContext(ctx, "update support send state failed")
			return
		}
		if s.wsPublisher != nil {
			s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_pending_send", EntityID: job.ID, WorkspaceID: job.WorkspaceID, TargetUserID: job.UserID, ParentType: "support_conversation", ParentID: job.ConversationID})
		}
	}
	fail := func(kind string) { job.Failure = kind; progress("failed") }
	if s.authzService == nil {
		fail("access")
		return
	}
	actor, err := s.authzService.ResolveActor(work, job.WorkspaceID, job.UserID)
	if err != nil || !s.authzService.Can(actor, authorization.PermSupportEdit) {
		fail("access")
		return
	}
	work = authorization.WithActor(work, actor)
	options, err := s.TranslationOptions(work, job.WorkspaceID, job.ConversationID, job.UserID)
	if err != nil || options.Conversation.Revision != job.Revision || (job.TargetLanguage != "" && job.TargetLanguage != supportReplyTarget(options)) {
		fail("translation")
		return
	}
	conversation, err := s.loadConversationAccessible(work, job.WorkspaceID, job.ConversationID)
	if err != nil || conversation == nil || strings.ToLower(strings.TrimSpace(derefString(conversation.CustomerEmail))) != job.RecipientEmail {
		fail("delivery")
		return
	}
	var req model.CreateMessageRequest
	if json.Unmarshal([]byte(job.Request), &req) != nil {
		fail("delivery")
		return
	}
	progress("preparing")
	work = context.WithValue(work, supportSendGuardKey{}, job)
	work = context.WithValue(work, supportSendProgressKey{}, progress)
	msg, err := s.CreateConversationMessage(work, job.WorkspaceID, job.ConversationID, req, "user", &job.UserID, nil, nil)
	if err != nil {
		if errors.Is(err, ErrSupportTranslation) || errors.Is(err, repository.ErrTranslationUnavailable) {
			fail("translation")
		} else {
			fail("delivery")
		}
		return
	}
	job.MessageID = msg.ID
	progress("sent")
}

func supportReplyTarget(options *model.SupportTranslationOptions) string {
	if options.Conversation.CustomerLanguage != "" {
		return options.Conversation.CustomerLanguage
	}
	return options.DetectedCustomerLanguage
}
