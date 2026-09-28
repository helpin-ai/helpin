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
	"sync"
	"time"
)

var ErrInvalidSupportReply = errors.New("invalid support reply")

type supportSendProgressKey struct{}
type supportSendGuardKey struct{}
type supportSendPolicyKey struct{}
type supportSendPolicySnapshot struct {
	workspaceID, conversationID, userID string
	options                             *model.SupportTranslationOptions
}

func (s *SupportInboxService) supportSendTranslationOptions(ctx context.Context, ws, conv, user string) (*model.SupportTranslationOptions, error) {
	if snapshot, ok := ctx.Value(supportSendPolicyKey{}).(supportSendPolicySnapshot); ok && snapshot.workspaceID == ws && snapshot.conversationID == conv && snapshot.userID == user {
		return snapshot.options, nil
	}
	return s.translationOptions(ctx, ws, conv, user, true)
}

func reportSupportSendProgress(ctx context.Context, status string) {
	if fn, ok := ctx.Value(supportSendProgressKey{}).(func(string)); ok {
		fn(status)
	}
}
func (s *SupportInboxService) QueueSupportSend(ctx context.Context, ws, conv, user string, req model.CreateMessageRequest) (*model.SupportMessage, error) {
	if _, err := uuid.Parse(req.ClientMessageID); err != nil {
		return nil, ErrInvalidSupportReply
	}
	if req.IsInternal || (req.MessageType != "" && req.MessageType != "reply") || (strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0) {
		return nil, ErrInvalidSupportReply
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
	options, err := s.translationOptions(ctx, ws, conv, user, true)
	if err != nil {
		return nil, err
	}
	conversation, err := s.loadConversationAccessible(ctx, ws, conv)
	if err != nil || conversation == nil {
		return nil, ErrSupportTranslation
	}
	target := supportReplyTarget(options)
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
		return nil, ErrInvalidSupportReply
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
	s.wakePendingSupportSends()
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
	options, err := s.translationOptions(ctx, ws, conv, user, true)
	if err != nil {
		return err
	}
	conversation, err := s.loadConversationAccessible(ctx, ws, conv)
	if err != nil || conversation == nil {
		return ErrSupportTranslation
	}
	err = s.messageRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	if err == nil {
		s.wakePendingSupportSends()
	}
	return err
}

const supportSendWorkers = 4

func (s *SupportInboxService) pendingSupportSendWake() chan struct{} {
	s.pendingSendWakeOnce.Do(func() { s.pendingSendWake = make(chan struct{}, supportSendWorkers) })
	return s.pendingSendWake
}
func (s *SupportInboxService) wakePendingSupportSends() {
	wake := s.pendingSupportSendWake()
	for i := 0; i < supportSendWorkers; i++ {
		select {
		case wake <- struct{}{}:
		default:
			return
		}
	}
}

const supportSendUnblockedSQL = `NOT EXISTS (SELECT 1 FROM support_pending_sends earlier
    WHERE earlier.workspace_id = support_pending_sends.workspace_id AND earlier.conversation_id = support_pending_sends.conversation_id
    AND earlier.id <> support_pending_sends.id
    AND earlier.status IN ('queued','preparing','translating','sending')
    AND ((earlier.status <> 'queued' AND earlier.updated_at >= ?)
      OR earlier.created_at < support_pending_sends.created_at
      OR (earlier.created_at = support_pending_sends.created_at AND earlier.id < support_pending_sends.id)))`

// Claim the earliest unfinished reply per conversation, across all API replicas.
// A locked earlier row remains visible to NOT EXISTS, so SKIP LOCKED cannot
// allow another worker to jump ahead in the same conversation.
func (s *SupportInboxService) claimPendingSupportSend(ctx context.Context) (*model.SupportPendingSend, error) {
	var job model.SupportPendingSend
	cutoff := time.Now().Add(-2 * time.Minute)
	err := s.messageRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Where("(status = 'queued' OR (status IN ('preparing','translating','sending') AND updated_at < ?))", cutoff).
			Where(supportSendUnblockedSQL, cutoff).
			Order("created_at ASC, id ASC")
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := q.First(&job).Error; err != nil {
			return err
		}
		if tx.Dialector.Name() == "postgres" {
			// Also fence claims selected from older statement snapshots (for
			// example when an earlier failed reply is concurrently retried).
			var locked bool
			if err := tx.Raw("SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))", "support-send:"+job.WorkspaceID+":"+job.ConversationID).Scan(&locked).Error; err != nil {
				return err
			}
			if !locked {
				return gorm.ErrRecordNotFound
			}
			var eligible int64
			if err := tx.Model(&model.SupportPendingSend{}).Where("id = ?", job.ID).Where(supportSendUnblockedSQL, cutoff).Count(&eligible).Error; err != nil {
				return err
			}
			if eligible != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		job.Attempts++
		return tx.Model(&job).Updates(map[string]any{"status": "preparing", "attempts": job.Attempts, "updated_at": time.Now().UTC()}).Error
	})
	return &job, err
}

func (s *SupportInboxService) RunPendingSupportSends(ctx context.Context) {
	wake := s.pendingSupportSendWake()
	var workers sync.WaitGroup
	for i := 0; i < supportSendWorkers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			tick := time.NewTicker(500 * time.Millisecond)
			defer tick.Stop()
			for ctx.Err() == nil {
				job, err := s.claimPendingSupportSend(ctx)
				if err == nil {
					started := time.Now()
					age := started.Sub(job.CreatedAt)
					s.executePendingSupportSend(ctx, job)
					slog.InfoContext(ctx, "support send processed", "send_id", job.ID, "attempt", job.Attempts, "queue_age_ms", age.Milliseconds(), "processing_ms", time.Since(started).Milliseconds())
					s.wakePendingSupportSends()
					continue
				}
				if !errors.Is(err, gorm.ErrRecordNotFound) && ctx.Err() == nil {
					slog.WarnContext(ctx, "claim support send failed", "error", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-wake:
				case <-tick.C:
				}
			}
		}()
	}
	workers.Wait()
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
	options, err := s.translationOptions(work, job.WorkspaceID, job.ConversationID, job.UserID, true)
	if err != nil || options.Conversation.Revision != job.Revision || (options.Preference.AutoTranslateOutgoing && job.TargetLanguage != "" && job.TargetLanguage != supportReplyTarget(options)) {
		fail("translation")
		return
	}
	if !options.Preference.AutoTranslateOutgoing {
		// Older queued jobs may retain a detected language even though Live
		// Translate is off. It is not a delivery requirement for original text.
		job.TargetLanguage = ""
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
	work = context.WithValue(work, supportSendPolicyKey{}, supportSendPolicySnapshot{job.WorkspaceID, job.ConversationID, job.UserID, options})
	work = context.WithValue(work, supportSendGuardKey{}, job)
	work = context.WithValue(work, supportSendProgressKey{}, progress)
	msg, err := s.CreateConversationMessage(work, job.WorkspaceID, job.ConversationID, req, "user", &job.UserID, nil, nil)
	if err != nil {
		slog.WarnContext(ctx, "support pending reply could not be delivered", "send_id", job.ID, "conversation_id", job.ConversationID, "error", err)
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
	if !options.Preference.AutoTranslateOutgoing {
		return ""
	}
	if options.Conversation.CustomerLanguage != "" {
		return options.Conversation.CustomerLanguage
	}
	return options.DetectedCustomerLanguage
}
