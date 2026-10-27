package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrPortalIntakeInvalid rejects a malformed intake record.
var ErrPortalIntakeInvalid = errors.New("invalid intake details")

// portalIntakeArticleLimit bounds the article ids one intake records: a
// customer sees at most five articles per search over a few searches.
const portalIntakeArticleLimit = 25

// RecordIntakeEvent stores that suggestions were shown or one was opened
// while a request was written. Submissions are recorded with the request.
func (s *CustomerPortalService) RecordIntakeEvent(ctx context.Context, ws *PortalWorkspace, intakeID, eventType string, articleIDs []string) error {
	if _, err := uuid.Parse(intakeID); err != nil {
		return ErrPortalIntakeInvalid
	}
	ids, err := portalIntakeArticleIDs(articleIDs, portalSuggestionLimit)
	if err != nil {
		return err
	}
	switch eventType {
	case model.PortalIntakeEventShown:
		if len(ids) == 0 {
			return ErrPortalIntakeInvalid
		}
	case model.PortalIntakeEventOpened:
		if len(ids) != 1 {
			return ErrPortalIntakeInvalid
		}
	default:
		return ErrPortalIntakeInvalid
	}
	return s.repo.RecordIntakeEvent(ctx, ws.ID, intakeID, eventType, ids, nil)
}

// normalizePortalIntake validates the intake sent with a new request. A
// request without one is valid.
func normalizePortalIntake(intake *model.PortalIntake) (*model.PortalIntake, error) {
	if intake == nil {
		return nil, nil
	}
	if _, err := uuid.Parse(intake.IntakeID); err != nil {
		return nil, ErrPortalIntakeInvalid
	}
	suggested, err := portalIntakeArticleIDs(intake.SuggestedArticleIDs, portalIntakeArticleLimit)
	if err != nil {
		return nil, err
	}
	opened, err := portalIntakeArticleIDs(intake.OpenedArticleIDs, portalIntakeArticleLimit)
	if err != nil {
		return nil, err
	}
	return &model.PortalIntake{IntakeID: intake.IntakeID, SuggestedArticleIDs: suggested, OpenedArticleIDs: opened}, nil
}

// portalIntakeArticleIDs checks article ids are UUIDs, removes duplicates,
// and enforces a limit.
func portalIntakeArticleIDs(ids []string, limit int) ([]string, error) {
	seen := map[string]bool{}
	cleaned := []string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if _, err := uuid.Parse(id); err != nil {
			return nil, ErrPortalIntakeInvalid
		}
		if !seen[id] {
			seen[id] = true
			cleaned = append(cleaned, id)
		}
	}
	if len(cleaned) > limit {
		return nil, ErrPortalIntakeInvalid
	}
	return cleaned, nil
}

// recordIntakeCoverageSignal reports a request written after the help center
// suggested nothing: the customer looked, found no article, and needed a
// person. Coverage review turns these into missing-article gaps.
func (s *CustomerPortalService) recordIntakeCoverageSignal(ws *PortalWorkspace, conversation *model.SupportConversation, intake *model.PortalIntake) {
	if s.inbox == nil || intake == nil || len(intake.SuggestedArticleIDs) > 0 || conversation == nil {
		return
	}
	s.inbox.recordSupportEvent(SupportEventInput{
		WorkspaceID:    ws.ID,
		EventType:      model.SupportEventWidgetSearchPerformed,
		ConversationID: &conversation.ID,
		ActorType:      model.SupportEventActorCustomer,
		Channel:        "portal",
		SourceSignal:   "no_results",
		IssueSummary:   conversation.Subject,
		Metadata:       map[string]any{"query": conversation.Subject, "result_count": 0, "surface": "portal_intake"},
	})
}
