package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var (
	// ErrInboundRecoveryConflict indicates a receipt that is already processing or complete.
	ErrInboundRecoveryConflict = errors.New("This email is already processing or has been recovered. Refresh to check its status.")
	// ErrInboundRecoveryDestination rejects an unsafe or stale destination.
	ErrInboundRecoveryDestination = errors.New("Choose an active conversation with this sender in the email's workspace.")
	// ErrInboundRecoveryNotFound keeps erased or missing receipts out of the admin UI.
	ErrInboundRecoveryNotFound = errors.New("Email no longer available.")
)

// ListAdminInboundEmails lists receipts that need attention and manual recovery results.
func (s *EmailFallbackService) ListAdminInboundEmails(ctx context.Context, status, query string, page int) (*model.AdminInboundEmailList, error) {
	if page < 1 {
		page = 1
	}
	return repository.NewSupportInboundJobRepository(s.convRepo.DB()).ListAdmin(ctx, status, strings.TrimSpace(query), page)
}

func (s *EmailFallbackService) adminInboundPayload(ctx context.Context, id string) (*model.SupportInboundJob, model.PostmarkInboundPayload, error) {
	var payload model.PostmarkInboundPayload
	if _, err := uuid.Parse(id); err != nil {
		return nil, payload, ErrInboundRecoveryNotFound
	}
	job, err := repository.NewSupportInboundJobRepository(s.convRepo.DB()).GetEmail(ctx, id)
	if err != nil {
		return nil, payload, err
	}
	if job == nil {
		return nil, payload, ErrInboundRecoveryNotFound
	}
	raw := job.Payload
	if raw == "" && job.EmailLogID != nil {
		log, err := repository.NewSupportInboundJobRepository(s.convRepo.DB()).AdminEmailLog(ctx, job)
		if err != nil {
			return nil, payload, err
		}
		if log != nil {
			raw = log.RawBody
		}
	}
	if raw == "" {
		return nil, payload, ErrInboundRecoveryNotFound
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, payload, fmt.Errorf("decode retained inbound email: %w", err)
	}
	return job, payload, nil
}

// GetAdminInboundEmail returns original content as text, never untrusted executable HTML.
func (s *EmailFallbackService) GetAdminInboundEmail(ctx context.Context, id string) (*model.AdminInboundEmailDetail, error) {
	job, payload, err := s.adminInboundPayload(ctx, id)
	if err != nil {
		return nil, err
	}
	repo := repository.NewSupportInboundJobRepository(s.convRepo.DB())
	summary, err := repo.AdminSummary(ctx, id)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		return nil, ErrInboundRecoveryNotFound
	}
	summary.SenderEmail = inboundEmailAddress(payload)
	body := strings.TrimSpace(payload.TextBody)
	if body == "" {
		body = inboundPayloadProjection(payload).VisibleText
	}
	result := &model.AdminInboundEmailDetail{AdminInboundEmail: *summary, Body: body, ProviderID: payload.MessageID, InReplyTo: inboundHeaderValue(payload.Headers, "In-Reply-To"), LastError: job.LastError, Attachments: []model.AdminInboundAttachment{}}
	for i, attachment := range payload.Attachments {
		result.Attachments = append(result.Attachments, model.AdminInboundAttachment{ID: inboundStableID(fmt.Sprintf("attachment:%s:%d", payload.MessageID, i)), Name: attachment.Name, ContentType: attachment.ContentType, Size: attachment.ContentLength})
	}
	if job.ConversationID != nil {
		result.Destination, err = repo.AdminConversation(ctx, *job.ConversationID)
	}
	return result, err
}

// DownloadAdminInboundAttachment retrieves a retained attachment only through its parent receipt.
func (s *EmailFallbackService) DownloadAdminInboundAttachment(ctx context.Context, id, attachmentID string) ([]byte, string, error) {
	_, payload, err := s.adminInboundPayload(ctx, id)
	if err != nil {
		return nil, "", err
	}
	for i, attachment := range payload.Attachments {
		if inboundStableID(fmt.Sprintf("attachment:%s:%d", payload.MessageID, i)) == attachmentID {
			data, err := base64.StdEncoding.DecodeString(attachment.Content)
			return data, attachment.Name, err
		}
	}
	return nil, "", ErrInboundRecoveryNotFound
}

// FindAdminInboundConversations suggests threads belonging to the actual sender, without selecting one.
func (s *EmailFallbackService) FindAdminInboundConversations(ctx context.Context, id, query string) ([]model.AdminInboundConversation, error) {
	job, payload, err := s.adminInboundPayload(ctx, id)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	targetID := query
	if parsed, err := url.Parse(query); err == nil && parsed.Host != "" {
		targetID = strings.TrimSuffix(parsed.Path, "/")
		targetID = targetID[strings.LastIndex(targetID, "/")+1:]
	}
	if _, err := uuid.Parse(targetID); err != nil {
		targetID = ""
	} else {
		query = ""
	}
	repo := repository.NewSupportInboundJobRepository(s.convRepo.DB())
	conversations, err := repo.AdminCandidates(ctx, job.WorkspaceID, strings.ToLower(inboundEmailAddress(payload)), query, targetID)
	if err != nil {
		return nil, err
	}
	result := []model.AdminInboundConversation{}
	for i := range conversations {
		if !validInboundRecoveryDestination(job, &conversations[i], payload) {
			continue
		}
		candidate, err := repo.AdminConversation(ctx, conversations[i].ID)
		if err != nil {
			return nil, err
		}
		if candidate != nil {
			result = append(result, *candidate)
		}
	}
	return result, nil
}

func validInboundRecoveryDestination(job *model.SupportInboundJob, conv *model.SupportConversation, payload model.PostmarkInboundPayload) bool {
	if conv == nil || conv.AnonymizedAt != nil || conv.MergedIntoID != nil || isEmailFallbackInboundTerminalStatus(conv.Status) {
		return false
	}
	if job.WorkspaceID != nil && *job.WorkspaceID != conv.WorkspaceID {
		return false
	}
	if job.ConversationID != nil && *job.ConversationID != conv.ID {
		return false
	}
	// Verification and unsubscribe messages retain their original control-message semantics.
	hash := mailboxHashFromInboundPayload(payload)
	if strings.HasPrefix(hash, "verify-") || strings.HasPrefix(hash, "unsubscribe-") {
		return false
	}
	sender := inboundEmailAddress(payload)
	if sender == "" {
		return false
	}
	participants := append([]string{derefString(conv.CustomerEmail)}, conv.EmailCC...)
	participants = append(participants, conv.EmailThreadParticipants...)
	for _, participant := range participants {
		if strings.EqualFold(strings.TrimSpace(participant), sender) {
			return true
		}
	}
	return false
}

// RecoverAdminInboundEmail queues one operator-confirmed retry and records who requested it.
func (s *EmailFallbackService) RecoverAdminInboundEmail(ctx context.Context, id, actor, conversationID string) error {
	job, payload, err := s.adminInboundPayload(ctx, id)
	if err != nil {
		return err
	}
	if job.Status != "failed" && job.Status != "pending" || job.Attempts == 0 || job.Payload == "" {
		return ErrInboundRecoveryConflict
	}
	var target *model.SupportConversation
	if conversationID != "" {
		if _, err := uuid.Parse(conversationID); err != nil {
			return ErrInboundRecoveryDestination
		}
		target, err = s.findConversationByID(ctx, conversationID)
		if err != nil {
			return err
		}
		if !validInboundRecoveryDestination(job, target, payload) {
			return ErrInboundRecoveryDestination
		}
	} else if job.WorkspaceID == nil {
		return ErrInboundRecoveryDestination
	}
	ok, err := repository.NewSupportInboundJobRepository(s.convRepo.DB()).RequestRecovery(ctx, job, actor, target, time.Now().UTC())
	if err != nil {
		return err
	}
	if !ok {
		return ErrInboundRecoveryConflict
	}
	s.logger.InfoContext(ctx, "admin requested inbound email recovery", "job_id", id, "user_id", actor, "conversation_id", conversationID)
	return nil
}

func (s *EmailFallbackService) processRecoveredInboundEmail(ctx context.Context, job *model.SupportInboundJob, payload model.PostmarkInboundPayload) error {
	conv, err := s.findConversationByID(ctx, derefString(job.RecoveryConversationID))
	if err != nil {
		return err
	}
	if !validInboundRecoveryDestination(job, conv, payload) {
		return ErrInboundRecoveryDestination
	}
	existing, err := s.emailLogRepo.GetByPostmarkMessageID(ctx, payload.MessageID)
	if err != nil {
		return err
	}
	if existing != nil {
		if existing.WorkspaceID != conv.WorkspaceID || existing.ConversationID != conv.ID {
			return ErrInboundRecoveryDestination
		}
		return s.retryInboundCustomerAIRequest(ctx, existing)
	}
	if inboundPayloadProjection(payload).VisibleText == "" && len(payload.Attachments) == 0 {
		return errors.New("email has no recoverable content")
	}
	return s.processInboundConversationReplyWithRecovery(ctx, conv, nil, payload, job.Payload, job)
}
