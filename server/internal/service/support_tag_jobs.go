package service

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SupportTagJobService assesses each customer reply for automatic tags and a
// raised priority, independently of routing and replies.
type SupportTagJobService struct {
	jobs          *repository.SupportTagJobRepository
	messages      *repository.SupportMessageRepository
	conversations *repository.SupportConversationRepository
	installations *repository.SupportInboxInstallationRepository
	jev           *SupportJevService
	entitlements  EntitlementPolicy
}

// NewSupportTagJobService binds the existing classifier and current entitlement policy.
func NewSupportTagJobService(jobs *repository.SupportTagJobRepository, messages *repository.SupportMessageRepository, conversations *repository.SupportConversationRepository, installations *repository.SupportInboxInstallationRepository, jev *SupportJevService, entitlements EntitlementPolicy) *SupportTagJobService {
	return &SupportTagJobService{jobs: jobs, messages: messages, conversations: conversations, installations: installations, jev: jev, entitlements: entitlements}
}

// supportTagJobPolicy is the classifier policy a job's decisions were made under.
type supportTagJobPolicy struct {
	tagsMode, priorityMode          string
	tagThreshold, priorityThreshold float64
}

func (s *SupportTagJobService) policy() supportTagJobPolicy {
	c := s.jev.config
	return supportTagJobPolicy{tagsMode: c.TagsMode, priorityMode: c.PriorityMode, tagThreshold: c.TagThreshold, priorityThreshold: c.PriorityThreshold}
}

// Run drains bounded batches until shutdown. Unfinished leases survive restarts.
func (s *SupportTagJobService) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for range 8 {
				worked, err := s.ProcessNext(ctx)
				if err != nil {
					slog.WarnContext(ctx, "support tagging job unavailable", "error", err)
					break
				}
				if !worked || ctx.Err() != nil {
					break
				}
			}
		}
	}
}

// ProcessNext executes one leased attempt with a deadline independent of replies.
func (s *SupportTagJobService) ProcessNext(ctx context.Context) (bool, error) {
	job, err := s.jobs.ClaimNext(ctx, time.Now().UTC())
	if err != nil || job == nil {
		return false, err
	}
	if job.Status == "failed" {
		return true, nil
	}
	attempt, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = s.process(attempt, *job)
	cancel()
	if err == nil || errors.Is(err, repository.ErrSupportTagLeaseLost) {
		return true, nil
	}
	settle, done := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer done()
	retryErr := s.jobs.Retry(settle, *job, time.Now().UTC())
	if errors.Is(retryErr, repository.ErrSupportTagLeaseLost) {
		retryErr = nil
	}
	return true, errors.Join(err, retryErr)
}

func (s *SupportTagJobService) allowed(ctx context.Context, workspaceID string) bool {
	return s.jev.enabled(workspaceID) && (s.jev.config.TagsMode != "off" || s.jev.config.PriorityMode != "off") &&
		(s.entitlements == nil || s.entitlements.RequireFeature(ctx, workspaceID, EntitlementFeatureAIConversationRouting) == nil)
}

func (s *SupportTagJobService) process(ctx context.Context, job model.SupportTagJob) error {
	complete := func(code string) error {
		return s.jobs.Finalize(ctx, job, func(*gorm.DB) (string, error) { return code, nil })
	}
	if !s.allowed(ctx, job.WorkspaceID) {
		return complete("disabled")
	}
	message, err := s.messages.GetByID(ctx, job.MessageID)
	if err != nil {
		return err
	}
	conversation, err := s.conversations.GetByID(ctx, job.WorkspaceID, job.ConversationID, "", model.RoleOwner)
	if err != nil {
		return err
	}
	history, err := s.messages.ListByConversation(ctx, job.WorkspaceID, job.ConversationID, true)
	if err != nil {
		return err
	}
	if !supportTagJobEligible(job, conversation, message, history) {
		return complete("ineligible")
	}
	askPriority, err := s.priorityWanted(ctx, conversation, history)
	if err != nil {
		return err
	}
	state := supportJevTagState(message, history)
	policy := s.policy()
	assessment, err := s.jev.assessCustomerReply(ctx, job.WorkspaceID, job.ConversationID, message, history, askPriority)
	if err != nil {
		return err
	}
	if !s.allowed(ctx, job.WorkspaceID) || policy != s.policy() {
		return complete("policy_changed")
	}
	tags := assessment.Tags
	var notes []*model.SupportMessage
	err = s.jobs.Finalize(ctx, job, func(tx *gorm.DB) (string, error) {
		var current model.SupportConversation
		if err := tx.Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ConversationID).Take(&current).Error; err != nil {
			return "", err
		}
		var source model.SupportMessage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND conversation_id = ? AND id = ?", job.WorkspaceID, job.ConversationID, job.MessageID).Take(&source).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "stale", nil
			}
			return "", err
		}
		latest, err := s.messages.WithTx(tx).ListByConversation(ctx, job.WorkspaceID, job.ConversationID, true)
		if err != nil {
			return "", err
		}
		if !supportTagJobEligible(job, &current, &source, latest) || source.Content != message.Content || !source.CreatedAt.Equal(message.CreatedAt) || supportJevTagState(&source, latest) != state {
			return "stale", nil
		}
		// Stable lock ordering also serializes tag renames/deletions during commit.
		sort.Slice(tags, func(i, j int) bool { return tags[i].ID < tags[j].ID })
		bound := s.jev.tags.withTx(tx)
		for _, selected := range tags {
			var tag model.SupportTag
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", job.WorkspaceID, selected.ID).Take(&tag).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				return "", err
			}
			if tag.Name != selected.Name || supportTagManuallyRemoved(latest, tag) {
				continue
			}
			note, err := bound.addAutomaticTag(ctx, job.WorkspaceID, job.ConversationID, tag.ID)
			if err != nil {
				return "", err
			}
			if note != nil {
				notes = append(notes, note)
			}
		}
		if assessment.Priority != "" {
			note, err := s.applyPriority(ctx, tx, &current, latest, assessment.Priority)
			if err != nil {
				return "", err
			}
			if note != nil {
				notes = append(notes, note)
			}
		}
		return "assessed", nil
	})
	if err == nil {
		for _, note := range notes {
			s.jev.tags.publishTagChange(job.WorkspaceID, job.ConversationID, note)
		}
	}
	return err
}

// priorityWanted reports whether Jev should also judge this conversation's priority.
func (s *SupportTagJobService) priorityWanted(ctx context.Context, conversation *model.SupportConversation, history []model.SupportMessage) (bool, error) {
	if s.jev.config.PriorityMode == "off" || !supportAIPriorityEligible(conversation, history) {
		return false, nil
	}
	return supportAIPriorityEnabled(ctx, s.installations, conversation.WorkspaceID)
}

// applyPriority rechecks the setting and teammate changes under the job's
// conversation lock before raising the priority.
func (s *SupportTagJobService) applyPriority(ctx context.Context, tx *gorm.DB, conversation *model.SupportConversation, latest []model.SupportMessage, priority string) (*model.SupportMessage, error) {
	enabled, err := supportAIPriorityEnabled(ctx, repository.NewSupportInboxInstallationRepository(tx), conversation.WorkspaceID)
	if err != nil || !enabled || !supportAIPriorityEligible(conversation, latest) {
		return nil, err
	}
	return raiseConversationPriority(ctx, tx, s.messages, conversation, priority)
}

func supportTagJobEligible(job model.SupportTagJob, conversation *model.SupportConversation, message *model.SupportMessage, history []model.SupportMessage) bool {
	if conversation == nil || conversation.AnonymizedAt != nil || conversation.WorkspaceID != job.WorkspaceID || conversation.ID != job.ConversationID ||
		message == nil || message.DeletedAt.Valid || message.ID != job.MessageID || message.WorkspaceID != job.WorkspaceID || message.ConversationID != job.ConversationID ||
		message.IsInternal || message.SenderType != "customer" || message.MessageType != "reply" || strings.TrimSpace(message.Content) == "" ||
		message.IsImported() {
		return false
	}
	for _, candidate := range history {
		if candidate.SenderType == "customer" && candidate.MessageType == "reply" && !candidate.IsInternal && !candidate.DeletedAt.Valid &&
			(candidate.CreatedAt.After(message.CreatedAt) || (candidate.CreatedAt.Equal(message.CreatedAt) && candidate.ID > message.ID)) {
			return false
		}
	}
	return true
}
