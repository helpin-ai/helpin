package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func inboundStableID(value string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("helpin:inbound:"+value)).String()
}

// AcceptInboundEmail never acknowledges a receipt that has not been committed.
func (s *EmailFallbackService) AcceptInboundEmail(ctx context.Context, p model.PostmarkInboundPayload, raw string) error {
	if s == nil || s.convRepo == nil {
		return fmt.Errorf("inbound processing unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p.MessageID = strings.TrimSpace(p.MessageID)
	if p.MessageID == "" {
		return fmt.Errorf("missing provider message ID")
	}
	// Use the validated payload, including attachment bytes, rather than a caller's partial audit copy.
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	job := &model.SupportInboundJob{ID: inboundStableID("email:" + p.MessageID), Kind: "email", Payload: string(data), Status: "pending", AvailableAt: time.Now().UTC()}
	if id := inboundConversationID(p); id != "" {
		if _, parseErr := uuid.Parse(id); parseErr == nil {
			conv, lookupErr := s.findConversationByID(ctx, id)
			if lookupErr != nil {
				return lookupErr
			}
			if conv != nil {
				job.WorkspaceID = &conv.WorkspaceID
				job.ConversationID = &conv.ID
				if conv.AnonymizedAt != nil {
					job.Status = "completed"
					job.Payload = ""
				}
			}
		}
	} else if s.supportInboxService != nil && s.supportInboxService.emailRouteRepo != nil {
		route, lookupErr := s.supportInboxService.emailRouteRepo.GetByRouteKey(ctx, mailboxHashFromInboundPayload(p))
		if lookupErr != nil {
			return lookupErr
		}
		if route == nil {
			for _, address := range inboundRecipientAddresses(p) {
				route, lookupErr = s.findInboundRouteByRecipient(ctx, address)
				if lookupErr != nil {
					return lookupErr
				}
				if route != nil {
					break
				}
			}
		}
		if route != nil {
			job.WorkspaceID = &route.WorkspaceID
			conv, lookupErr := s.resolveInboundRouteConversation(ctx, route.WorkspaceID, p)
			if lookupErr != nil {
				return lookupErr
			}
			if conv != nil {
				job.ConversationID = &conv.ID
				if conv.AnonymizedAt != nil {
					job.Status = "completed"
					job.Payload = ""
				}
			}
		}
	}
	return repository.NewSupportInboundJobRepository(s.convRepo.DB()).Create(ctx, job)
}

// Receipt and attachment workers are independent: storage cannot block new mail.
func (s *EmailFallbackService) StartInboundWorker(ctx context.Context, kind string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.processNextInboundJob(ctx, kind); err != nil {
				s.logger.ErrorContext(ctx, "inbound worker failed", "kind", kind, "error", err)
			}
		}
	}
}
func (s *EmailFallbackService) processNextInboundJob(ctx context.Context, kind string) error {
	ctx, cancelJob := context.WithTimeout(ctx, 90*time.Second)
	defer cancelJob()
	repo := repository.NewSupportInboundJobRepository(s.convRepo.DB())
	job, err := repo.Claim(ctx, kind, time.Now().UTC())
	if err != nil || job == nil {
		return err
	}
	workCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	if kind == "email" {
		var p model.PostmarkInboundPayload
		err = json.Unmarshal([]byte(job.Payload), &p)
		if err == nil {
			err = s.ProcessInboundEmail(workCtx, p, job.Payload)
		}
	} else {
		err = s.processInboundAttachmentJob(workCtx, job)
	}
	cancel()
	if kind == "email" {
		var p model.PostmarkInboundPayload
		if json.Unmarshal([]byte(job.Payload), &p) == nil {
			if log, lookupErr := s.emailLogRepo.GetByPostmarkMessageID(ctx, p.MessageID); lookupErr == nil && log != nil {
				if updateErr := s.convRepo.DB().WithContext(ctx).Model(&model.SupportInboundJob{}).Where("id = ? AND lease_token = ?", job.ID, job.LeaseToken).Updates(map[string]any{"workspace_id": log.WorkspaceID, "conversation_id": log.ConversationID}).Error; updateErr != nil {
					return updateErr
				}
			}
		}
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "inbound processing failed", "job_id", job.ID, "kind", kind, "attempt", job.Attempts, "exhausted", job.Attempts >= 10, "error", err)
	}
	// Shutdown/cancellation leaves a recoverable lease, never a false completion.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if kind == "attachment" && err != nil {
		state := "processing"
		if job.Attempts >= 10 {
			state = "failed"
		}
		if updateErr := s.convRepo.DB().WithContext(ctx).Model(&model.SupportAttachment{}).Where("id = ? AND is_uploaded = false", job.ID).Updates(map[string]any{"processing_status": state, "processing_error": "Attachment could not be stored. Retry or contact support."}).Error; updateErr != nil {
			return updateErr
		}
	}
	if finishErr := repo.Finish(ctx, job, err, time.Now().UTC()); finishErr != nil {
		return finishErr
	}
	if job.MessageID != nil && job.WorkspaceID != nil && job.ConversationID != nil {
		s.publishMessageUpdated(*job.WorkspaceID, *job.ConversationID, *job.MessageID, "email-attachment")
	}
	return nil
}

func (s *EmailFallbackService) enqueueInboundAttachments(ctx context.Context, tx *gorm.DB, msg *model.SupportMessage, p model.PostmarkInboundPayload) error {
	for i, a := range p.Attachments {
		id := inboundStableID(fmt.Sprintf("attachment:%s:%d", p.MessageID, i))
		name := strings.TrimSpace(a.Name)
		if name == "" {
			name = "attachment"
		}
		contentType := strings.TrimSpace(a.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		req := supportInboundEmailAttachmentRequest{AttachmentID: id, WorkspaceID: msg.WorkspaceID, ConversationID: msg.ConversationID, MessageID: msg.ID, FileName: name, ContentType: contentType, Base64Content: a.Content, ContentID: a.ContentID, ContentLength: a.ContentLength}
		data, err := json.Marshal(req)
		if err != nil {
			return err
		}
		attachment := model.SupportAttachment{ID: id, WorkspaceID: msg.WorkspaceID, ConversationID: &msg.ConversationID, MessageID: &msg.ID, FileName: name, FileSize: a.ContentLength, ContentType: contentType, UploadedByType: "customer", ProcessingStatus: "processing", ContentID: a.ContentID}
		if err := tx.WithContext(ctx).Create(&attachment).Error; err != nil {
			return err
		}
		if err := repository.NewSupportInboundJobRepository(tx).Create(ctx, &model.SupportInboundJob{ID: id, Kind: "attachment", WorkspaceID: &msg.WorkspaceID, ConversationID: &msg.ConversationID, MessageID: &msg.ID, Payload: string(data), Status: "pending", AvailableAt: time.Now().UTC()}); err != nil {
			return err
		}
		msg.Attachments = append(msg.Attachments, model.SupportAttachmentPayload{ID: id, FileName: name, FileType: contentType, FileSize: a.ContentLength, ProcessingStatus: "processing", ContentID: a.ContentID})
	}
	return nil
}
func (s *EmailFallbackService) processInboundAttachmentJob(ctx context.Context, job *model.SupportInboundJob) error {
	var req supportInboundEmailAttachmentRequest
	if err := json.Unmarshal([]byte(job.Payload), &req); err != nil {
		return err
	}
	msg, err := s.messageRepo.GetByID(ctx, req.MessageID)
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	conv, err := s.findConversationByID(ctx, req.ConversationID)
	if err != nil {
		return err
	}
	if conv == nil || conv.AnonymizedAt != nil {
		return nil
	}
	if s.inboundAttachmentStore == nil {
		return fmt.Errorf("attachment storage unavailable")
	}
	stored, err := s.inboundAttachmentStore.StoreInboundEmailAttachment(ctx, req)
	if err != nil {
		return err
	}
	if stored == nil {
		return fmt.Errorf("attachment storage returned no file")
	}
	return repository.NewSupportAttachmentRepository(s.convRepo.DB()).CompleteInbound(ctx, &model.SupportAttachment{
		ID: req.AttachmentID, WorkspaceID: req.WorkspaceID, MessageID: &req.MessageID,
		StorageKey: stored.FileKey, FileSize: stored.FileSize, ContentType: stored.FileType, PublicURL: stored.URL,
	})
}

func (s *SupportInboxService) RetryInboundAttachment(ctx context.Context, workspaceID, messageID, attachmentID string) error {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg == nil || msg.WorkspaceID != workspaceID {
		return fmt.Errorf("message not found")
	}
	conv, err := s.loadConversationAccessible(ctx, workspaceID, msg.ConversationID)
	if err != nil {
		return err
	}
	if conv == nil || conv.AnonymizedAt != nil {
		return fmt.Errorf("conversation not found")
	}
	db := s.conversationRepo.DB()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.SupportInboundJob{}).Where("id = ? AND workspace_id = ? AND message_id = ? AND kind = ? AND status = ?", attachmentID, workspaceID, messageID, "attachment", "failed").Updates(map[string]any{"status": "pending", "attempts": 0, "available_at": time.Now().UTC(), "last_error": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("attachment is not awaiting retry")
		}
		return tx.Model(&model.SupportAttachment{}).Where("id = ? AND workspace_id = ?", attachmentID, workspaceID).Updates(map[string]any{"processing_status": "processing", "processing_error": ""}).Error
	})
	if err == nil && s.emailFallbackService != nil {
		s.emailFallbackService.publishMessageUpdated(workspaceID, msg.ConversationID, messageID, "email-attachment")
	}
	return err
}
