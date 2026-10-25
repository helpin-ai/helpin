package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var (
	// ErrSupportCSATInvalid means the rating is outside 1-5.
	ErrSupportCSATInvalid = errors.New("choose a rating from 1 to 5")
	// ErrSupportCSATUnavailable means the conversation cannot be rated now:
	// CSAT is off, the conversation is not resolved, or it is not the caller's.
	ErrSupportCSATUnavailable = errors.New("rating is not available for this conversation")
	// ErrSupportWidgetSessionInvalid means the widget must reconnect.
	ErrSupportWidgetSessionInvalid = errors.New("widget session is not valid")
)

func (s *SupportInboxService) csatRepository() *repository.SupportCSATRepository {
	return repository.NewSupportCSATRepository(s.conversationRepo.DB())
}

// SubmitConversationCSAT records a customer's rating of a resolved
// conversation. Ratings on a merged conversation apply to the conversation it
// was merged into, which is the one the team resolved.
func (s *SupportInboxService) SubmitConversationCSAT(ctx context.Context, submission model.SupportCSATSubmission, settings model.SupportInboxSettings) (*model.SupportCSATSummary, error) {
	if submission.Rating < 1 || submission.Rating > 5 {
		return nil, ErrSupportCSATInvalid
	}
	if !settings.CSATEnabled {
		return nil, ErrSupportCSATUnavailable
	}
	rootID, err := s.mergeRepository().Root(ctx, submission.WorkspaceID, submission.ConversationID)
	if err != nil {
		return nil, err
	}
	conversation, err := s.loadConversationUnscoped(ctx, submission.WorkspaceID, rootID)
	if err != nil {
		return nil, err
	}
	if conversation == nil || conversation.AnonymizedAt != nil || conversation.Status != model.SupportConversationStatusResolved {
		return nil, ErrSupportCSATUnavailable
	}
	if rootID != submission.ConversationID {
		// A conversation merged into another customer's must not rate, or
		// read, that customer's conversation.
		source, err := s.loadConversationUnscoped(ctx, submission.WorkspaceID, submission.ConversationID)
		if err != nil {
			return nil, err
		}
		if !model.SameSupportCustomer(source, conversation) {
			return nil, ErrSupportCSATUnavailable
		}
	}

	comment := normalizeSupportCSATComment(submission.Comment)
	resolvedBy := model.SupportCSATResolvedByHuman
	if conversation.FlowState != nil && *conversation.FlowState == model.SupportConversationFlowStateResolvedByAI {
		resolvedBy = model.SupportCSATResolvedByAI
	}
	saved, previous, changed, err := s.csatRepository().Upsert(ctx, model.SupportCSATResponse{
		WorkspaceID: conversation.WorkspaceID, ConversationID: conversation.ID,
		Rating: submission.Rating, Comment: comment, Channel: submission.Channel,
		ResolvedBy: resolvedBy, AssignedUserID: conversation.AssignedUserID,
		SubmittedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	summary := &model.SupportCSATSummary{Rating: saved.Rating, Comment: saved.Comment, SubmittedAt: saved.SubmittedAt}
	if !changed {
		return summary, nil
	}
	s.recordCSATTimelineEvent(ctx, conversation, saved, previous)
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{
			Action: "updated", Entity: "support_conversation", EntityID: conversation.ID,
			WorkspaceID: conversation.WorkspaceID,
		})
	}
	return summary, nil
}

// conversationAcceptsCSAT reports whether a rating of conversation would be
// accepted now: the conversation that handles it (itself, or the one it was
// merged into for the same customer) is resolved.
func (s *SupportInboxService) conversationAcceptsCSAT(ctx context.Context, workspaceID string, conversation *model.SupportConversation) (bool, error) {
	if conversation == nil || conversation.AnonymizedAt != nil {
		return false, nil
	}
	handler := conversation
	if conversation.MergedIntoID != nil {
		rootID, err := s.mergeRepository().Root(ctx, workspaceID, conversation.ID)
		if err != nil {
			return false, err
		}
		root, err := s.loadConversationUnscoped(ctx, workspaceID, rootID)
		if err != nil {
			return false, err
		}
		if root == nil || root.AnonymizedAt != nil || !model.SameSupportCustomer(conversation, root) {
			return false, nil
		}
		handler = root
	}
	return handler.Status == model.SupportConversationStatusResolved, nil
}

func normalizeSupportCSATComment(raw string) *string {
	comment := strings.TrimSpace(raw)
	if comment == "" {
		return nil
	}
	if utf8.RuneCountInString(comment) > model.SupportCSATCommentMaxRunes {
		comment = strings.TrimSpace(string([]rune(comment)[:model.SupportCSATCommentMaxRunes]))
	}
	return &comment
}

// recordCSATTimelineEvent shows the rating to teammates in the conversation.
// The rating is already stored, so a failure here is logged, not returned.
func (s *SupportInboxService) recordCSATTimelineEvent(ctx context.Context, conversation *model.SupportConversation, saved, previous *model.SupportCSATResponse) {
	verb := "rated"
	if previous != nil && previous.Rating != saved.Rating {
		verb = "changed their rating to"
	} else if previous != nil {
		verb = "updated their feedback:"
	}
	content := fmt.Sprintf("Customer %s %d/5 (%s)", verb, saved.Rating, model.SupportCSATRatingLabel(saved.Rating))
	if verb == "updated their feedback:" {
		content = fmt.Sprintf("Customer updated their feedback on a %d/5 rating", saved.Rating)
	}
	metadata := map[string]any{"rating": saved.Rating, "channel": saved.Channel}
	if saved.Comment != nil {
		metadata["comment"] = *saved.Comment
	}
	if previous != nil && previous.Rating != saved.Rating {
		metadata["previous_rating"] = previous.Rating
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		slog.ErrorContext(ctx, "encode csat timeline metadata", "error", err, "conversation_id", conversation.ID)
		return
	}
	message := &model.SupportMessage{
		WorkspaceID: conversation.WorkspaceID, ConversationID: conversation.ID,
		SenderType: "customer", SenderDisplayName: supportCSATCustomerName(conversation),
		Content: content, MessageType: "system",
		SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventCSATRated),
		IsInternal:      true, Metadata: string(raw),
	}
	if err := s.messageRepo.Create(ctx, message); err != nil {
		slog.ErrorContext(ctx, "create csat timeline event", "error", err, "conversation_id", conversation.ID)
		return
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(conversation.WorkspaceID, message, ""))
	}
}

func supportCSATCustomerName(conversation *model.SupportConversation) *string {
	for _, candidate := range []*string{conversation.CustomerName, conversation.CustomerEmail} {
		if candidate != nil && strings.TrimSpace(*candidate) != "" {
			name := strings.TrimSpace(*candidate)
			return &name
		}
	}
	return nil
}

// SubmitWidgetCSAT records a rating from the chat widget. The session must
// belong to the visitor who started the conversation.
func (s *SupportInboxService) SubmitWidgetCSAT(ctx context.Context, sessionToken, conversationID string, rating int, comment string) (*model.SupportCSATSummary, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSupportWidgetSessionInvalid, err)
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, ErrSupportCSATUnavailable
	}
	conversation, err := s.loadConversationUnscoped(ctx, session.WorkspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil || conversation.AnonymousID == nil || strings.TrimSpace(session.AnonymousID) == "" ||
		strings.TrimSpace(*conversation.AnonymousID) != strings.TrimSpace(session.AnonymousID) {
		return nil, ErrSupportCSATUnavailable
	}
	installation, err := s.installationRepo.GetByWorkspace(ctx, session.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if installation == nil || !installation.Active {
		return nil, ErrSupportCSATUnavailable
	}
	return s.SubmitConversationCSAT(ctx, model.SupportCSATSubmission{
		WorkspaceID: session.WorkspaceID, ConversationID: conversation.ID,
		Channel: model.SupportCSATChannelWidget, Rating: rating, Comment: comment,
	}, parseSettings(installation.Settings))
}

// hydrateConversationCSAT attaches each conversation's rating. A merged
// conversation shows the rating of the conversation it was merged into.
func (s *SupportInboxService) hydrateConversationCSAT(ctx context.Context, workspaceID string, conversations []model.SupportConversation) error {
	if len(conversations) == 0 {
		return nil
	}
	var parents map[string]string
	for _, conversation := range conversations {
		if conversation.MergedIntoID == nil {
			continue
		}
		edges, err := s.mergeRepository().Active(ctx, workspaceID)
		if err != nil {
			return err
		}
		parents = make(map[string]string, len(edges))
		for _, edge := range edges {
			parents[edge.SourceID] = edge.TargetID
		}
		break
	}
	targets := make(map[string]string, len(conversations))
	ids := make([]string, 0, len(conversations))
	rootIDs := []string{}
	for _, conversation := range conversations {
		target := conversation.ID
		if conversation.MergedIntoID != nil {
			root, err := model.SupportMergeRoot(conversation.ID, parents)
			if err != nil {
				return err
			}
			target = root
			rootIDs = append(rootIDs, root)
		}
		targets[conversation.ID] = target
		ids = append(ids, target)
	}
	if len(rootIDs) > 0 {
		// Only the same customer sees the rating of the conversation theirs
		// was merged into.
		roots, err := s.conversationRepo.ListByIDs(ctx, workspaceID, rootIDs, "", model.RoleOwner)
		if err != nil {
			return err
		}
		byID := make(map[string]*model.SupportConversation, len(roots))
		for i := range roots {
			byID[roots[i].ID] = &roots[i]
		}
		for i := range conversations {
			if target := targets[conversations[i].ID]; target != conversations[i].ID && !model.SameSupportCustomer(&conversations[i], byID[target]) {
				delete(targets, conversations[i].ID)
			}
		}
	}
	summaries, err := s.csatRepository().SummariesByConversation(ctx, workspaceID, ids)
	if err != nil {
		return err
	}
	for i := range conversations {
		target, ok := targets[conversations[i].ID]
		if !ok {
			continue
		}
		if summary, ok := summaries[target]; ok {
			value := summary
			conversations[i].CSAT = &value
		}
	}
	return nil
}
