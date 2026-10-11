package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// ErrSupportMergeDenied is returned when the caller cannot perform a merge.
var ErrSupportMergeDenied = errors.New("you do not have permission to merge conversations")

func requireSupportMergeActor(ctx context.Context, workspaceID string) (*authorization.Actor, error) {
	actor := supportActorFromContext(ctx, workspaceID)
	if actor == nil || actor.UserID == "" || !authorization.NewRBACEngine().Can(actor.Role, authorization.PermSupportMerge) {
		return nil, ErrSupportMergeDenied
	}
	return actor, nil
}

func (s *SupportInboxService) mergeRepository() *repository.SupportMergeRepository {
	return repository.NewSupportMergeRepository(s.conversationRepo.DB())
}

// PreviewConversationMerge resolves the complete affected group and audience.
func (s *SupportInboxService) PreviewConversationMerge(ctx context.Context, workspaceID, targetID string, req model.SupportMergeRequest) (*model.SupportMergePreview, error) {
	actor, err := requireSupportMergeActor(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.previewConversationMerge(ctx, s.mergeRepository(), workspaceID, targetID, req.SourceIDs, actor)
}

func (s *SupportInboxService) previewConversationMerge(ctx context.Context, repo *repository.SupportMergeRepository, workspaceID, targetID string, sourceIDs []string, actor *authorization.Actor) (*model.SupportMergePreview, error) {
	if len(sourceIDs) < 1 || len(sourceIDs) > 49 {
		return nil, fmt.Errorf("select between 2 and 50 conversations")
	}
	ids := append([]string{targetID}, sourceIDs...)
	slices.Sort(ids)
	if len(slices.Compact(slices.Clone(ids))) != len(ids) {
		return nil, fmt.Errorf("select each conversation once")
	}
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return nil, repository.ErrSupportMergeNotFound
		}
	}
	records, err := repo.Conversations(ctx, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	if len(records) != len(ids) {
		return nil, repository.ErrSupportMergeNotFound
	}
	preview := &model.SupportMergePreview{Sources: []model.SupportConversation{}, Participants: []string{}, StopsAI: true}
	var primary model.SupportConversation
	for _, c := range records {
		if c.ID == targetID {
			primary = c
		}
	}
	installation, err := repository.NewSupportInboxInstallationRepository(repo.DB()).GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	settings := model.DefaultSupportInboxSettings()
	if installation != nil {
		settings = parseSettings(installation.Settings)
	}
	affected := []string{}
	for _, selected := range records {
		if !s.isMailboxAccessible(ctx, workspaceID, selected.MailboxID) {
			return nil, repository.ErrSupportMergeNotFound
		}
		if selected.MergedIntoID != nil {
			return nil, repository.ErrSupportMerged
		}
		group, err := repo.Group(ctx, workspaceID, selected.ID)
		if err != nil {
			return nil, err
		}
		for _, c := range group {
			if c.Status == model.SupportConversationStatusSpam {
				return nil, fmt.Errorf("move spam conversations back to the inbox before merging")
			}
			if c.AnonymizedAt != nil {
				return nil, fmt.Errorf("deleted customers cannot be merged")
			}
			if c.MailboxID != nil && s.mailboxRepo != nil {
				inbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, *c.MailboxID)
				if err != nil {
					return nil, err
				}
				if inbox != nil {
					c.MailboxName = &inbox.Name
				}
			}
			affected = append(affected, c.ID)
			if !model.SameSupportCustomer(&primary, &c) && c.ID != primary.ID {
				preview.CrossCustomer = true
			}
			if !sameMailboxID(primary.MailboxID, c.MailboxID) {
				preview.TransfersAccess = true
			}
			if c.AIActiveRunID != nil || c.AssignedAgentID != nil {
				preview.StopsAI = true
			}
			if c.Status == model.SupportConversationStatusOpen || c.Status == model.SupportConversationStatusWaitingOnCustomer {
				preview.Reopens = primary.Status == model.SupportConversationStatusResolved
			}
			for _, email := range model.SupportMergeEmailAudience([]model.SupportConversation{c}) {
				if !slices.Contains(preview.Participants, email) {
					preview.Participants = append(preview.Participants, email)
				}
			}
			if c.ID == targetID {
				preview.Target = c
			} else {
				preview.Sources = append(preview.Sources, c)
			}
		}
	}
	if len(affected) > 50 {
		return nil, fmt.Errorf("select up to 50 conversations including merged history")
	}
	if preview.CrossCustomer && !settings.AllowCrossCustomerMerge {
		return nil, fmt.Errorf("different customer; ask an admin to enable this")
	}
	if err := repo.EnsureNoPendingSends(ctx, workspaceID, affected); err != nil {
		return nil, err
	}
	slices.Sort(preview.Participants)
	slices.SortFunc(preview.Sources, func(a, b model.SupportConversation) int { return strings.Compare(a.ID, b.ID) })
	preview.PreviewToken = supportMergeFingerprint(workspaceID, actor.UserID, *preview)
	return preview, nil
}

func supportMergeFingerprint(workspaceID, actorID string, p model.SupportMergePreview) string {
	// Exclude message timestamps so an ordinary inbound reply does not invalidate
	// review. Routing, identity, status, assignment, and AI control must match.
	type record struct {
		ID                                                              string
		Parent                                                          *string
		Revision                                                        int64
		Mailbox, Contact, Email, Anonymous, AssignedUser, AssignedAgent *string
		Status                                                          string
		AIControl                                                       int64
	}
	convert := func(c model.SupportConversation) record {
		return record{c.ID, c.MergedIntoID, c.MergeRevision, c.MailboxID, c.CRMContactID, c.CustomerEmail, c.AnonymousID, c.AssignedUserID, c.AssignedAgentID, c.Status, c.AIControlVersion}
	}
	records := []record{convert(p.Target)}
	for _, c := range p.Sources {
		records = append(records, convert(c))
	}
	payload, _ := json.Marshal(struct {
		Workspace, Actor         string
		Records                  []record
		Participants             []string
		Cross, Reopens, Transfer bool
	}{workspaceID, actorID, records, p.Participants, p.CrossCustomer, p.Reopens, p.TransfersAccess})
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

// MergeConversations rechecks the review and permissions in the routing transaction.
func (s *SupportInboxService) MergeConversations(ctx context.Context, workspaceID, targetID string, req model.SupportMergeRequest) (*model.SupportMergeResult, error) {
	actor, err := requireSupportMergeActor(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(req.OperationID); err != nil {
		return nil, fmt.Errorf("a valid operation_id is required")
	}
	result := &model.SupportMergeResult{ConversationID: targetID}
	err = s.mergeRepository().Transaction(ctx, workspaceID, func(repo *repository.SupportMergeRepository) error {
		previous, err := repo.Operation(ctx, workspaceID, req.OperationID)
		if err != nil {
			return err
		}
		if len(previous) > 0 {
			// Authorize a retry against current effective access, not an expired preview.
			for _, edge := range previous {
				if edge.ActorID != actor.UserID || edge.TargetID != targetID || !slices.Contains(req.SourceIDs, edge.SourceID) {
					return repository.ErrSupportMergeChanged
				}
			}
			if len(previous) != len(req.SourceIDs) {
				return repository.ErrSupportMergeChanged
			}
			c, err := s.loadConversationAccessible(ctx, workspaceID, targetID)
			if err != nil {
				return err
			}
			if c == nil {
				return repository.ErrSupportMergeNotFound
			}
			result.Merges = previous
			return nil
		}
		preview, err := s.previewConversationMerge(ctx, repo, workspaceID, targetID, req.SourceIDs, actor)
		if err != nil {
			return err
		}
		if req.PreviewToken == "" || req.PreviewToken != preview.PreviewToken {
			return repository.ErrSupportMergeChanged
		}
		result.Merges, err = repo.Merge(ctx, workspaceID, targetID, req.SourceIDs, actor.UserID, req.OperationID, preview.CrossCustomer)
		result.AffectedIDs = []string{targetID}
		for _, c := range preview.Sources {
			result.AffectedIDs = append(result.AffectedIDs, c.ID)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "support merge routing updated", "workspace_id", workspaceID, "actor_id", actor.UserID, "operation_id", req.OperationID, "conversation_id", result.ConversationID, "affected_ids", result.AffectedIDs)
	s.publishMergeChange(workspaceID, actor.UserID, result)
	return result, nil
}

func (s *SupportInboxService) publishMergeChange(workspaceID, actorID string, result *model.SupportMergeResult) {
	if s.wsPublisher == nil {
		return
	}
	for _, id := range result.AffectedIDs {
		s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID})
	}
}

// ListConversationMerges returns accessible originals, never inaccessible snippets.
func (s *SupportInboxService) ListConversationMerges(ctx context.Context, workspaceID, conversationID string) ([]model.SupportConversationMerge, error) {
	conv, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, repository.ErrSupportMergeNotFound
	}
	rows, err := s.mergeRepository().List(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		source, err := s.loadConversationAccessible(ctx, workspaceID, rows[i].SourceID)
		if err != nil {
			return nil, err
		}
		target, err := s.loadConversationAccessible(ctx, workspaceID, rows[i].TargetID)
		if err != nil {
			return nil, err
		}
		rows[i].Source = source
		rows[i].Target = target
		if s.userRepo != nil {
			u, e := s.userRepo.GetByID(ctx, rows[i].ActorID)
			if e != nil {
				return nil, e
			}
			if u != nil {
				rows[i].ActorName = u.FullName
			}
		}
	}
	return rows, nil
}

// PreviewConversationUnmerge reviews restored ownership and retained history.
func (s *SupportInboxService) PreviewConversationUnmerge(ctx context.Context, workspaceID, mergeID string, restoreMailboxID ...*string) (*model.SupportMergePreview, error) {
	actor, err := requireSupportMergeActor(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var restore *string
	if len(restoreMailboxID) > 0 {
		restore = restoreMailboxID[0]
	}
	return s.previewConversationUnmerge(ctx, s.mergeRepository(), workspaceID, mergeID, actor, restore)
}

func (s *SupportInboxService) previewConversationUnmerge(ctx context.Context, repo *repository.SupportMergeRepository, workspaceID, mergeID string, actor *authorization.Actor, restore *string) (*model.SupportMergePreview, error) {
	edge, err := repo.Get(ctx, workspaceID, mergeID)
	if err != nil {
		return nil, err
	}
	if edge.EndedAt != nil {
		return nil, repository.ErrSupportMergeChanged
	}
	root, err := repo.Root(ctx, workspaceID, edge.TargetID)
	if err != nil {
		return nil, err
	}
	records, err := repo.Conversations(ctx, workspaceID, []string{edge.SourceID, root})
	if err != nil {
		return nil, err
	}
	if len(records) != 2 {
		return nil, repository.ErrSupportMergeNotFound
	}
	p := &model.SupportMergePreview{Sources: []model.SupportConversation{}, Participants: []string{}, StopsAI: true}
	for _, c := range records {
		if c.AnonymizedAt != nil {
			return nil, fmt.Errorf("this conversation was deleted and cannot be restored")
		}
		if c.ID == edge.SourceID {
			mailbox := c.MailboxID
			if restore != nil {
				if *restore == "shared" {
					mailbox = nil
				} else {
					mailbox = restore
				}
			}
			if mailbox != nil {
				if s.mailboxRepo == nil {
					p.RestoreInboxRequired = true
				} else {
					inbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, *mailbox)
					if err != nil {
						return nil, err
					}
					if inbox == nil || !inbox.Active || !s.isMailboxAccessible(ctx, workspaceID, mailbox) {
						p.RestoreInboxRequired = true
					} else {
						c.MailboxName = &inbox.Name
					}
				}
			} else {
				c.MailboxName = nil
			}
			c.MailboxID = mailbox
			if c.AssignedUserID != nil && !s.mergeAssigneeEligible(ctx, workspaceID, mailbox, *c.AssignedUserID) {
				c.AssignedUserID = nil
			}
			p.Sources = append(p.Sources, c)
		} else {
			if !s.isMailboxAccessible(ctx, workspaceID, c.MailboxID) {
				return nil, repository.ErrSupportMergeNotFound
			}
			p.Target = c
		}
	}
	group, err := repo.Group(ctx, workspaceID, root)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(group))
	for _, c := range group {
		ids = append(ids, c.ID)
	}
	if err := repo.EnsureNoPendingSends(ctx, workspaceID, ids); err != nil {
		return nil, err
	}
	if !p.RestoreInboxRequired {
		p.PreviewToken = supportMergeFingerprint(workspaceID, actor.UserID, *p)
	}
	return p, nil
}

// UnmergeConversation restores one source and its existing descendants.
func (s *SupportInboxService) UnmergeConversation(ctx context.Context, workspaceID, mergeID string, req model.SupportMergeRequest) (*model.SupportMergeResult, error) {
	actor, err := requireSupportMergeActor(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(req.OperationID); err != nil {
		return nil, fmt.Errorf("a valid operation_id is required")
	}
	result := &model.SupportMergeResult{}
	err = s.mergeRepository().Transaction(ctx, workspaceID, func(repo *repository.SupportMergeRepository) error {
		edge, err := repo.Get(ctx, workspaceID, mergeID)
		if err != nil {
			return err
		}
		if edge.EndedAt != nil && edge.EndedBy != nil && *edge.EndedBy == actor.UserID && edge.UnmergeOperationID != nil && *edge.UnmergeOperationID == req.OperationID {
			source, err := s.loadConversationAccessible(ctx, workspaceID, edge.SourceID)
			if err != nil {
				return err
			}
			if source == nil {
				return repository.ErrSupportMergeNotFound
			}
			result.ConversationID = source.ID
			return nil
		}
		p, err := s.previewConversationUnmerge(ctx, repo, workspaceID, mergeID, actor, req.RestoreMailboxID)
		if err != nil {
			return err
		}
		if req.PreviewToken == "" || req.PreviewToken != p.PreviewToken {
			return repository.ErrSupportMergeChanged
		}
		result.ConversationID = p.Sources[0].ID
		result.AffectedIDs = []string{p.Target.ID, result.ConversationID}
		if err := repo.Unmerge(ctx, workspaceID, mergeID, actor.UserID, req.OperationID); err != nil {
			return err
		}
		return repo.RestoreLocation(ctx, workspaceID, p.Sources[0])
	})
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "support merge routing updated", "workspace_id", workspaceID, "actor_id", actor.UserID, "operation_id", req.OperationID, "conversation_id", result.ConversationID, "affected_ids", result.AffectedIDs)
	s.publishMergeChange(workspaceID, actor.UserID, result)
	return result, nil
}

// RequireWritableSupportConversation guards every staff mutation entry point.
func (s *SupportInboxService) RequireWritableSupportConversation(ctx context.Context, workspaceID, id string) error {
	c, err := s.loadConversationAccessible(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if c == nil {
		return repository.ErrSupportMergeNotFound
	}
	if c.MergedIntoID != nil {
		return repository.ErrSupportMerged
	}
	return nil
}

func (s *SupportInboxService) mergeAssigneeEligible(ctx context.Context, workspaceID string, mailboxID *string, userID string) bool {
	if s.workspaceRepo == nil {
		return false
	}
	member, err := s.workspaceRepo.GetMembership(ctx, workspaceID, userID)
	return err == nil && member != nil && member.Status == "active" && s.userCanAccessMailbox(ctx, workspaceID, mailboxID, userID)
}

// refreshMergedVisitors keeps original widget sessions on the active status.
func (s *SupportInboxService) refreshMergedVisitors(ctx context.Context, c *model.SupportConversation) {
	if c == nil || c.MergeRevision == 0 || s.wsPublisher == nil {
		return
	}
	group, err := s.mergeRepository().Group(ctx, c.WorkspaceID, c.ID)
	if err != nil {
		slog.WarnContext(ctx, "refresh merged widget conversations", "error", err, "conversation_id", c.ID)
		return
	}
	seen := map[string]bool{}
	for _, member := range group {
		if member.AnonymousID != nil && !seen[*member.AnonymousID] {
			seen[*member.AnonymousID] = true
			s.pushVisitorConversationsRefresh(ctx, c.WorkspaceID, *member.AnonymousID)
		}
	}
}
