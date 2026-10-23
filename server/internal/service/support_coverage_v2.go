package service

import (
	"context"
	"errors"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	tclient "go.temporal.io/sdk/client"
)

type CoverageTopicDetailV2 struct {
	Topic    model.CoverageTopicV2   `json:"topic"`
	Findings []model.CoverageFinding `json:"findings"`
}

// CoveragePipelineHealth explains whether coverage analysis is working. Healthy
// means nothing needs a person: retryable failures do not count, but unresolved
// failures from earlier batches and assignment safety pauses do.
type CoveragePipelineHealth struct {
	ReanalysisStatus  string                          `json:"reanalysis_status,omitempty"`
	ReanalysisAttempt int32                           `json:"reanalysis_attempt,omitempty"`
	ReanalysisError   string                          `json:"reanalysis_error,omitempty"`
	ScheduleStatus    string                          `json:"schedule_status,omitempty"`
	NextRunAt         *time.Time                      `json:"next_run_at,omitempty"`
	LatestBatch       *model.CoverageBatch            `json:"latest_batch"`
	Failures          []model.CoverageAnalysisAttempt `json:"failures"`
	Healthy           bool                            `json:"healthy"`
	Rollout           CoverageRolloutDecision         `json:"rollout"`
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
	run, err := s.reanalysisState(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	healthy := len(rollout.PauseReasons) == 0 && run.Status != "failed" &&
		(batch == nil || batch.Status != model.CoverageBatchFailed) && !coverageFailuresNeedPerson(failures)
	schedule := s.scheduleState(ctx)
	return &CoveragePipelineHealth{
		ReanalysisStatus: run.Status, ReanalysisAttempt: run.Attempt, ReanalysisError: run.Error,
		ScheduleStatus: schedule.Status, NextRunAt: schedule.NextRunAt, LatestBatch: batch, Failures: failures,
		Healthy: healthy, Rollout: rollout,
	}, nil
}

// coverageFailuresNeedPerson is true when a failure exhausted its automatic
// retries or is waiting on configuration.
func coverageFailuresNeedPerson(failures []model.CoverageAnalysisAttempt) bool {
	for _, failure := range failures {
		if failure.Status != model.CoverageAttemptRetryable {
			return true
		}
	}
	return false
}

func (s *SupportCoverageV2Service) ReplayAttempt(ctx context.Context, workspaceID, attemptID string) error {
	return s.repo.ReplayAttempt(ctx, workspaceID, attemptID)
}

func (s *SupportCoverageV2Service) ListArchivedV1(ctx context.Context, workspaceID string, limit, offset int) ([]model.SupportCoverageGap, error) {
	return s.repo.ListArchivedV1Gaps(ctx, workspaceID, limit, offset)
}
