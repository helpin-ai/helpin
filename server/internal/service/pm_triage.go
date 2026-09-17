package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/pmtriage"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// PMTriageConfig controls the independently capped PM decision pilot.
type PMTriageConfig struct {
	Mode         string
	Threshold    float64
	DailyLimit   int
	WorkspaceIDs []string
}

// PMTriageService evaluates existing tasks and public support feedback.
// Mutations are intentionally handled through the existing reviewed task flows.
type PMTriageService struct {
	config      PMTriageConfig
	provider    decision.Provider
	assessments *repository.PMTriageRepository
	usage       AIExecutionUsageStore
	tasks       *repository.PMTaskRepository
	workspaces  *repository.WorkspaceRepository
	labels      *repository.PMLabelRepository
	support     *SupportInboxService
}

// NewPMTriageService constructs the independently controlled PM integration.
func NewPMTriageService(config PMTriageConfig, provider decision.Provider, assessments *repository.PMTriageRepository, usage AIExecutionUsageStore, tasks *repository.PMTaskRepository, workspaces *repository.WorkspaceRepository, labels *repository.PMLabelRepository, support *SupportInboxService) (*PMTriageService, error) {
	if config.Mode == "" {
		config.Mode = "primary"
	}
	if config.Mode != "primary" && config.Mode != "shadow" && config.Mode != "off" {
		return nil, errors.New("invalid PM triage mode")
	}
	if math.IsNaN(config.Threshold) || math.IsInf(config.Threshold, 0) || config.Threshold < .5 || config.Threshold > 1 || config.DailyLimit < 1 {
		return nil, errors.New("invalid PM triage threshold or daily limit")
	}
	if assessments == nil || usage == nil || tasks == nil || workspaces == nil || labels == nil || support == nil {
		return nil, errors.New("PM triage dependencies required")
	}
	return &PMTriageService{config: config, provider: provider, assessments: assessments, usage: usage, tasks: tasks, workspaces: workspaces, labels: labels, support: support}, nil
}

// Analyze resolves current permissions before retrieving source or candidate text.
// The HTTP entry point must also enforce module permissions.
func (s *PMTriageService) Analyze(ctx context.Context, workspaceID, sourceKind, sourceID string) (*model.PMTriageView, error) {
	actor := authorization.GetActor(ctx)
	if actor == nil || actor.UserID == "" || actor.WorkspaceID != workspaceID || !authorization.NewRBACEngine().Can(actor.Role, authorization.PermPMEdit) {
		return nil, &model.ErrForbidden{Message: "permission to edit product work is required"}
	}
	view := &model.PMTriageView{Status: "disabled", SourceKind: sourceKind, SourceID: sourceID, Teams: []pmtriage.Option{}, Labels: []pmtriage.Option{}, Candidates: []model.PMTriageCandidateView{}}
	if !s.enabled(workspaceID) {
		return view, nil
	}
	source, err := s.loadSource(ctx, workspaceID, sourceKind, sourceID)
	if err != nil {
		return nil, err
	}
	view.SourceHash = source.hash
	if len(source.input.Text) > 8000 || strings.TrimSpace(source.input.Text) == "" {
		view.Status = "input_limit"
		return view, nil
	}
	if err := s.addOptions(ctx, workspaceID, source); err != nil {
		return nil, err
	}
	candidates, err := s.tasks.FindTriageCandidates(ctx, repository.PMTriageScope{WorkspaceID: workspaceID, TeamIDs: actor.TeamIDs(), AllTeams: isPrivileged(actor)}, source.taskID, source.input.Text)
	if err != nil {
		return nil, err
	}
	request, err := pmtriage.Build(source.input)
	if err != nil {
		view.Status = "input_limit"
		return view, nil
	}
	for _, task := range candidates {
		candidate := pmtriage.Candidate{ID: task.ID, Name: task.Name, Description: pmTriageText(task.Description)}
		source.input.Candidates = append(source.input.Candidates, candidate)
		next, buildErr := pmtriage.Build(source.input)
		if buildErr != nil {
			source.input.Candidates = source.input.Candidates[:len(source.input.Candidates)-1]
			continue
		}
		request = next
		view.Candidates = append(view.Candidates, model.PMTriageCandidateView{ID: task.ID, Name: task.Name, DisplayID: task.DisplayID})
	}
	view.Teams = source.input.Teams
	view.Labels = source.input.Labels
	return s.evaluate(ctx, actor, source.hash, request, view)
}

func (s *PMTriageService) enabled(workspaceID string) bool {
	if s == nil || s.provider == nil || s.config.Mode == "off" {
		return false
	}
	configured := false
	for _, id := range s.config.WorkspaceIDs {
		if id = strings.TrimSpace(id); id != "" {
			configured = true
			if id == workspaceID {
				return true
			}
		}
	}
	return !configured
}

func (s *PMTriageService) evaluate(ctx context.Context, actor *authorization.Actor, sourceHash string, request *pmtriage.Request, view *model.PMTriageView) (*model.PMTriageView, error) {
	identity, err := json.Marshal(struct {
		Version, Model, Mode, SourceHash, State string
		Threshold                               float64
		Questions                               map[string]decision.Question
	}{Version: pmtriage.Version, Model: decision.Model, Mode: s.config.Mode, SourceHash: sourceHash, State: request.State, Threshold: s.config.Threshold, Questions: request.Questions})
	if err != nil {
		return nil, err
	}
	admission, err := s.assessments.Reserve(ctx, model.PMTriageAssessment{WorkspaceID: actor.WorkspaceID, ActorID: actor.UserID, SourceKind: view.SourceKind, SourceID: view.SourceID, SourceHash: sourceHash, ContextHash: pmTriageHash(string(identity)), Mode: s.config.Mode}, s.config.DailyLimit)
	if err != nil {
		return nil, err
	}
	if admission.Limited {
		view.Status = "daily_limit"
		return view, nil
	}
	record := admission.Assessment
	view.ID = record.ID
	view.Status = record.Status
	view.Reviewed = record.Reviewed
	if !admission.CallProvider {
		if record.Status == "ready" && s.config.Mode == "primary" {
			encoded, err := json.Marshal(record.Outcome["assessment"])
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(encoded, &view.Assessment); err != nil {
				return nil, fmt.Errorf("decode cached PM assessment: %w", err)
			}
		}
		if s.config.Mode == "shadow" {
			view.Status = "shadow"
		}
		return view, nil
	}
	assessment, usage, callErr := request.Evaluate(ctx, s.provider, s.config.Threshold)
	settle, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if usage != nil {
		entry := model.AIExecutionUsage{WorkspaceID: actor.WorkspaceID, IdempotencyKey: "jev-pm:" + record.ID, FeatureKey: "pm_triage", Provider: "typesafe", Model: usage.Model, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, MeasurementStatus: "provider_reported", PaidTools: model.JSONBlob(`[]`)}
		if err := s.usage.RecordExecutionUsage(settle, entry, nil); err != nil {
			callErr = errors.Join(callErr, fmt.Errorf("record PM decision usage: %w", err))
		}
	}
	outcome := model.JSONB{"version": pmtriage.Version, "cost_policy": "operator_funded_pilot", "confidence_kind": "provider_probability_not_locally_calibrated"}
	status := "failed"
	if callErr == nil {
		status = "ready"
		outcome["assessment"] = assessment
		outcome["usage"] = usage
	}
	if err := s.assessments.Finish(settle, actor.WorkspaceID, actor.UserID, record.ID, status, outcome); err != nil {
		return nil, errors.Join(callErr, err)
	}
	if callErr != nil {
		return nil, callErr
	}
	view.Status = "ready"
	if s.config.Mode == "shadow" {
		view.Status = "shadow"
	} else {
		view.Assessment = assessment
	}
	return view, nil
}

func pmTriageHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
