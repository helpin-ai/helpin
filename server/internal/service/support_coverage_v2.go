package service

import (
	"context"
	"errors"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
)

type CoverageTopicDetailV2 struct {
	Topic    model.CoverageTopicV2   `json:"topic"`
	Findings []model.CoverageFinding `json:"findings"`
}

type CoveragePipelineHealth struct {
	ReanalysisStatus string                          `json:"reanalysis_status,omitempty"`
	LatestBatch      *model.CoverageBatch            `json:"latest_batch"`
	Failures         []model.CoverageAnalysisAttempt `json:"failures"`
	Healthy          bool                            `json:"healthy"`
	Rollout          CoverageRolloutDecision         `json:"rollout"`
}

var ErrCoverageV2ReadDisabled = errors.New("coverage v2 reads are not enabled for this workspace")

type SupportCoverageV2Service struct {
	repo          *repository.CoverageV2Repository
	temporal      tclient.Client
	rolloutPolicy *CoverageRolloutPolicy
}

func (s *SupportCoverageV2Service) SetTemporalClient(client tclient.Client) *SupportCoverageV2Service {
	s.temporal = client
	return s
}

func (s *SupportCoverageV2Service) reanalysisStatus(ctx context.Context, workspaceID string) (string, error) {
	if s.temporal == nil {
		return "unavailable", nil
	}
	description, err := s.temporal.DescribeWorkflowExecution(ctx, "coverage-reanalysis-"+workspaceID, "")
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if description == nil || description.WorkflowExecutionInfo == nil {
		return "unavailable", nil
	}
	switch description.WorkflowExecutionInfo.Status {
	case enums.WORKFLOW_EXECUTION_STATUS_RUNNING:
		for _, pending := range description.PendingActivities {
			if pending.State == enums.PENDING_ACTIVITY_STATE_STARTED {
				return "running", nil
			}
		}
		return "queued", nil
	case enums.WORKFLOW_EXECUTION_STATUS_FAILED, enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT, enums.WORKFLOW_EXECUTION_STATUS_TERMINATED, enums.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return "failed", nil
	default:
		return "", nil
	}
}

func (s *SupportCoverageV2Service) EnsureReanalysisEnabled(ctx context.Context, workspaceID string) error {
	decision, err := s.rolloutDecision(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !decision.CaptureEnabled || len(decision.PauseReasons) > 0 {
		return ErrCoverageReanalysisPaused
	}
	return nil
}

var ErrCoverageReanalysisPaused = errors.New("coverage analysis is paused or not enabled")

func (s *SupportCoverageV2Service) SetRolloutPolicy(policy *CoverageRolloutPolicy) *SupportCoverageV2Service {
	if s != nil {
		s.rolloutPolicy = policy
	}
	return s
}

func (s *SupportCoverageV2Service) rolloutDecision(ctx context.Context, workspaceID string) (CoverageRolloutDecision, error) {
	if s.rolloutPolicy == nil {
		return CoverageRolloutDecision{RequestedMode: CoverageRolloutV2Write, CaptureEnabled: true, AssignmentEnabled: true, ReadV2Enabled: true, WriteV2Enabled: true}, nil
	}
	stats, err := s.repo.CoverageRolloutStats(ctx, workspaceID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return CoverageRolloutDecision{}, err
	}
	return s.rolloutPolicy.Evaluate(workspaceID, CoverageRolloutMetrics{
		AttemptCount: int(stats.AttemptCount), TerminalFailureCount: int(stats.TerminalFailureCount),
		DuplicateIdempotencyViolations: int(stats.DuplicateIdempotencyViolations), CrossWorkspaceInvariantViolations: int(stats.CrossWorkspaceInvariantViolations),
	}), nil
}

func (s *SupportCoverageV2Service) requireRead(ctx context.Context, workspaceID string) error {
	decision, err := s.rolloutDecision(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !decision.ReadV2Enabled {
		return ErrCoverageV2ReadDisabled
	}
	return nil
}

func NewSupportCoverageV2Service(repo *repository.CoverageV2Repository) *SupportCoverageV2Service {
	return &SupportCoverageV2Service{repo: repo}
}

func (s *SupportCoverageV2Service) ListTopics(ctx context.Context, workspaceID string, limit, offset int) ([]model.CoverageTopicV2, error) {
	if err := s.requireRead(ctx, workspaceID); err != nil {
		return nil, err
	}
	return s.repo.ListTopics(ctx, workspaceID, limit, offset)
}

func (s *SupportCoverageV2Service) CountTopics(ctx context.Context, workspaceID string) (int64, error) {
	if err := s.requireRead(ctx, workspaceID); err != nil {
		return 0, err
	}
	return s.repo.CountTopics(ctx, workspaceID)
}

func (s *SupportCoverageV2Service) GetTopic(ctx context.Context, workspaceID, topicID string) (*CoverageTopicDetailV2, error) {
	if err := s.requireRead(ctx, workspaceID); err != nil {
		return nil, err
	}
	topic, findings, err := s.repo.GetTopic(ctx, workspaceID, topicID)
	if err != nil || topic == nil {
		return nil, err
	}
	return &CoverageTopicDetailV2{Topic: *topic, Findings: findings}, nil
}

func (s *SupportCoverageV2Service) ListSignals(ctx context.Context, workspaceID string, limit, offset int) ([]model.CoverageUnreviewedSignal, error) {
	if err := s.requireRead(ctx, workspaceID); err != nil {
		return nil, err
	}
	return s.repo.ListUnreviewedSignals(ctx, workspaceID, limit, offset)
}

func (s *SupportCoverageV2Service) CountSignals(ctx context.Context, workspaceID string) (int64, error) {
	if err := s.requireRead(ctx, workspaceID); err != nil {
		return 0, err
	}
	return s.repo.CountUnreviewedSignals(ctx, workspaceID)
}

func (s *SupportCoverageV2Service) ReviewSignal(ctx context.Context, workspaceID, signalID, topicID, actorID string) error {
	return s.repo.ReviewSignal(ctx, workspaceID, signalID, topicID, actorID)
}

func (s *SupportCoverageV2Service) DismissSignal(ctx context.Context, workspaceID, signalID, actorID string) error {
	return s.repo.DismissSignal(ctx, workspaceID, signalID, actorID)
}

func (s *SupportCoverageV2Service) PipelineHealth(ctx context.Context, workspaceID string) (*CoveragePipelineHealth, error) {
	batch, err := s.repo.LatestBatch(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	failures, err := s.repo.ListFailedAttempts(ctx, workspaceID, 25)
	if err != nil {
		return nil, err
	}
	rollout, err := s.rolloutDecision(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	healthy := len(failures) == 0 && (batch == nil || batch.Status == model.CoverageBatchSucceeded)
	if len(rollout.PauseReasons) > 0 {
		healthy = false
	}
	status, err := s.reanalysisStatus(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return &CoveragePipelineHealth{ReanalysisStatus: status, LatestBatch: batch, Failures: failures, Healthy: healthy, Rollout: rollout}, nil
}

func (s *SupportCoverageV2Service) ReplayAttempt(ctx context.Context, workspaceID, attemptID string) error {
	return s.repo.ReplayAttempt(ctx, workspaceID, attemptID)
}

func (s *SupportCoverageV2Service) ListArchivedV1(ctx context.Context, workspaceID string, limit, offset int) ([]model.SupportCoverageGap, error) {
	return s.repo.ListArchivedV1Gaps(ctx, workspaceID, limit, offset)
}
