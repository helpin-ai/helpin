package service

import (
	"context"
	"errors"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type CoverageTopicDetailV2 struct {
	Topic    model.CoverageTopicV2   `json:"topic"`
	Findings []model.CoverageFinding `json:"findings"`
}

type CoveragePipelineHealth struct {
	LatestBatch *model.CoverageBatch            `json:"latest_batch"`
	Failures    []model.CoverageAnalysisAttempt `json:"failures"`
	Healthy     bool                            `json:"healthy"`
	Rollout     CoverageRolloutDecision         `json:"rollout"`
}

var ErrCoverageV2ReadDisabled = errors.New("coverage v2 reads are not enabled for this workspace")

type SupportCoverageV2Service struct {
	repo          *repository.CoverageV2Repository
	rolloutPolicy *CoverageRolloutPolicy
}

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
	return &CoveragePipelineHealth{LatestBatch: batch, Failures: failures, Healthy: healthy, Rollout: rollout}, nil
}

func (s *SupportCoverageV2Service) ReplayAttempt(ctx context.Context, workspaceID, attemptID string) error {
	return s.repo.ReplayAttempt(ctx, workspaceID, attemptID)
}

func (s *SupportCoverageV2Service) ListArchivedV1(ctx context.Context, workspaceID string, limit, offset int) ([]model.SupportCoverageGap, error) {
	return s.repo.ListArchivedV1Gaps(ctx, workspaceID, limit, offset)
}
