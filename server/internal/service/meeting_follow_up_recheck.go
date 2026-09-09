package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var ErrMeetingFollowUpRecheckBusy = errors.New("a routing check is already running; please try again shortly")

type meetingFollowUpRecheckStore interface {
	MeetingFollowUpsToRecheck(context.Context, string, string, string) ([]model.MeetingFollowUpRoutingCandidate, error)
	RouteMeetingFollowUpForActor(context.Context, model.CRMSuggestion, string, string) (bool, error)
}

// Runs the same metered classifier as the worker, explicitly and on demand.
// Never launches a Temporal workflow or regenerates meeting artifacts.
type MeetingFollowUpRecheckService struct {
	store     meetingFollowUpRecheckStore
	processor *CRMMeetingProcessingService
	running   sync.Map
}

func NewMeetingFollowUpRecheckService(store meetingFollowUpRecheckStore, processor *CRMMeetingProcessingService) *MeetingFollowUpRecheckService {
	return &MeetingFollowUpRecheckService{store: store, processor: processor}
}
func (s *MeetingFollowUpRecheckService) Recheck(ctx context.Context, ws, cursor string) (*model.MeetingFollowUpRecheckResult, error) {
	actor, err := pmAISuggestionActor(ctx, ws)
	if err != nil {
		return nil, err
	}
	if cursor != "" {
		if _, err := uuid.Parse(cursor); err != nil {
			return nil, ErrPMAISuggestionInput
		}
	}
	if _, busy := s.running.LoadOrStore(ws, struct{}{}); busy {
		return nil, ErrMeetingFollowUpRecheckBusy
	}
	defer s.running.Delete(ws)
	ctx, cancel := context.WithTimeout(ctx, 95*time.Second)
	defer cancel()
	candidates, err := s.store.MeetingFollowUpsToRecheck(ctx, ws, actor.UserID, cursor)
	if err != nil {
		return nil, err
	}
	result := &model.MeetingFollowUpRecheckResult{Items: []model.MeetingFollowUpRecheckItem{}}
	if len(candidates) > 3 {
		result.NextCursor = candidates[2].ID
		candidates = candidates[:3]
	}
	lastChecked := cursor
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			result.NextCursor = lastChecked
			return result, err
		}
		item := model.MeetingFollowUpRecheckItem{ID: candidate.ID, Title: candidate.Title, Outcome: "not_eligible"}
		if candidate.RoutingEligible {
			if s.processor == nil || s.processor.llmProvider == nil {
				result.NextCursor = lastChecked
				return result, errors.New("meeting classifier is unavailable")
			}
			transcript, err := s.processor.repo.GetTranscript(ctx, ws, *candidate.ObjectID)
			if err != nil {
				result.NextCursor = lastChecked
				return result, err
			}
			if transcript == nil || strings.TrimSpace(transcript.PlainText) == "" {
				item.Outcome = "missing_transcript"
			} else {
				scope, err := s.processor.classifyExistingFollowUp(ctx, candidate.CRMSuggestion, transcript)
				if err != nil {
					var entitlementErr *EntitlementError
					if isMeetingUsageBlocked(err) || errors.As(err, &entitlementErr) {
						result.NextCursor = lastChecked
						return result, err
					}
					slog.WarnContext(ctx, "API meeting follow-up routing deferred", "workspace_id", ws, "suggestion_id", candidate.ID, "error", err)
					item.Outcome = "retry_needed"
				} else {
					changed, err := s.store.RouteMeetingFollowUpForActor(ctx, candidate.CRMSuggestion, scope, actor.UserID)
					if err != nil {
						result.NextCursor = lastChecked
						return result, err
					}
					item.Outcome = scope
					if !changed {
						item.Outcome = "changed"
					}
				}
			}
		}
		result.Items = append(result.Items, item)
		lastChecked = candidate.ID
	}
	return result, nil
}
