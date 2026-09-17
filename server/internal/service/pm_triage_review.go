package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/pmtriage"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrPMTriageStale requires refreshing suggestions before reviewing changed work.
var ErrPMTriageStale = errors.New("triage evidence changed")

// PMTriageReviewService applies explicitly reviewed suggestions through canonical services.
type PMTriageReviewService struct {
	triage       *PMTriageService
	tasks        *PMTaskService
	associations *AssociationsService
}

// NewPMTriageReviewService constructs the review-only mutation path.
func NewPMTriageReviewService(triage *PMTriageService, tasks *PMTaskService, associations *AssociationsService) *PMTriageReviewService {
	return &PMTriageReviewService{triage: triage, tasks: tasks, associations: associations}
}

// Review rechecks ownership, source revision and current task access before applying.
func (s *PMTriageReviewService) Review(ctx context.Context, workspaceID, kind, sourceID string, req model.PMTriageReviewRequest) (*model.PMTriageReviewResult, error) {
	actor := authorization.GetActor(ctx)
	if actor == nil || actor.WorkspaceID != workspaceID || !authorization.NewRBACEngine().Can(actor.Role, authorization.PermPMEdit) {
		return nil, &model.ErrForbidden{Message: "PM edit permission required"}
	}
	if !s.triage.enabled(workspaceID) || s.triage.config.Mode != "primary" {
		return nil, ErrPMTriageStale
	}
	record, err := s.triage.assessments.Get(ctx, workspaceID, actor.UserID, req.AssessmentID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.SourceKind != kind || record.SourceID != sourceID || record.Status != "ready" || record.Mode != "primary" {
		return nil, ErrPMTriageStale
	}
	source, err := s.triage.loadSource(ctx, workspaceID, kind, sourceID)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(record.Outcome["assessment"])
	if err != nil {
		return nil, err
	}
	var assessment pmtriage.Assessment
	if err := json.Unmarshal(encoded, &assessment); err != nil {
		return nil, err
	}
	key, relationship, err := pmTriageReviewChoice(assessment, req, kind)
	if err != nil {
		return nil, err
	}
	status := "accepted"
	if req.Dismiss {
		status = "dismissed"
	}
	if previous, ok := record.Reviewed[key]; ok {
		if previous != status {
			return nil, ErrPMTriageStale
		}
		return &model.PMTriageReviewResult{Key: key, Status: status}, nil
	}
	if source.hash != record.SourceHash {
		return nil, ErrPMTriageStale
	}
	if !req.Dismiss {
		if err := s.apply(ctx, workspaceID, actor.UserID, sourceID, kind, record, req, relationship); err != nil {
			return nil, err
		}
	}
	if err := s.triage.assessments.MarkReviewed(ctx, workspaceID, actor.UserID, record.ID, key, status); err != nil {
		return nil, err
	}
	return &model.PMTriageReviewResult{Key: key, Status: status}, nil
}

func pmTriageReviewChoice(assessment pmtriage.Assessment, req model.PMTriageReviewRequest, kind string) (string, string, error) {
	if !assessment.Actionable || req.Value == "" {
		return "", "", ErrPMTriageStale
	}
	switch req.Action {
	case "task_type":
		if kind == "task" && assessment.TaskType != nil && assessment.TaskType.ID == req.Value {
			return "task_type:" + req.Value, "", nil
		}
	case "team":
		if kind == "task" && assessment.Team != nil && assessment.Team.ID == req.Value {
			return "team:" + req.Value, "", nil
		}
	case "match":
		for _, match := range assessment.Matches {
			if match.TaskID == req.Value && (match.Relationship == "duplicates" || match.Relationship == "relates_to") {
				return "match:" + req.Value, match.Relationship, nil
			}
		}
	}
	return "", "", ErrPMTriageStale
}

func (s *PMTriageReviewService) apply(ctx context.Context, workspaceID, actorID, sourceID, kind string, record *model.PMTriageAssessment, req model.PMTriageReviewRequest, relationship string) error {
	if req.Action == "match" {
		target, err := s.triage.tasks.GetRawByID(ctx, req.Value)
		if err != nil {
			return err
		}
		if target == nil || target.WorkspaceID != workspaceID || target.Archived || !canAccessTeam(ctx, target.TeamID) {
			return ErrPMTriageStale
		}
		encoded, err := json.Marshal(record.Outcome["candidate_hashes"])
		if err != nil {
			return err
		}
		var hashes map[string]string
		if err := json.Unmarshal(encoded, &hashes); err != nil {
			return err
		}
		if hashes[target.ID] != pmTriageHash(target.Name+"\n"+pmTriageText(target.Description)) {
			return ErrPMTriageStale
		}
		if kind == "support_conversation" {
			return s.triage.support.linkReviewedConversationTask(ctx, workspaceID, sourceID, target.ID, actorID, record.SourceHash, target.UpdatedAt)
		}
		source, err := s.triage.tasks.GetRawByID(ctx, sourceID)
		if err != nil {
			return err
		}
		if source == nil || pmTriageHash(source.Name+"\n"+pmTriageText(source.Description)+"\n"+source.UpdatedAt.String()) != record.SourceHash {
			return ErrPMTriageStale
		}
		_, err = s.associations.CreateTaskRelationship(ctx, workspaceID, sourceID, actorID, model.CreateTaskRelationshipRequest{OtherTaskID: target.ID, RelationshipType: relationship, ExpectedTaskRevisions: map[string]time.Time{source.ID: source.UpdatedAt, target.ID: target.UpdatedAt}})
		if errors.Is(err, repository.ErrPMTaskRevisionChanged) {
			return ErrPMTriageStale
		}
		return err
	}
	task, err := s.triage.tasks.GetRawByID(ctx, sourceID)
	if err != nil {
		return err
	}
	if task == nil || pmTriageHash(task.Name+"\n"+pmTriageText(task.Description)+"\n"+task.UpdatedAt.String()) != record.SourceHash {
		return ErrPMTriageStale
	}
	patch := model.UpdateTaskRequest{ExpectedUpdatedAt: &task.UpdatedAt}
	switch req.Action {
	case "task_type":
		patch.TaskType = &req.Value
	case "team":
		patch.TeamID = &req.Value
	default:
		return fmt.Errorf("unsupported review action")
	}
	_, err = s.tasks.Update(ctx, sourceID, patch, actorID)
	if errors.Is(err, repository.ErrPMTaskRevisionChanged) {
		return ErrPMTriageStale
	}
	return err
}
