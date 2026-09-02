package service

import (
	"context"

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
}

type SupportCoverageV2Service struct {
	repo *repository.CoverageV2Repository
}

func NewSupportCoverageV2Service(repo *repository.CoverageV2Repository) *SupportCoverageV2Service {
	return &SupportCoverageV2Service{repo: repo}
}

func (s *SupportCoverageV2Service) ListTopics(ctx context.Context, workspaceID string, limit, offset int) ([]model.CoverageTopicV2, error) {
	return s.repo.ListTopics(ctx, workspaceID, limit, offset)
}

func (s *SupportCoverageV2Service) GetTopic(ctx context.Context, workspaceID, topicID string) (*CoverageTopicDetailV2, error) {
	topic, findings, err := s.repo.GetTopic(ctx, workspaceID, topicID)
	if err != nil || topic == nil {
		return nil, err
	}
	return &CoverageTopicDetailV2{Topic: *topic, Findings: findings}, nil
}

func (s *SupportCoverageV2Service) ListSignals(ctx context.Context, workspaceID string, limit, offset int) ([]model.CoverageUnreviewedSignal, error) {
	return s.repo.ListUnreviewedSignals(ctx, workspaceID, limit, offset)
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
	healthy := len(failures) == 0 && (batch == nil || batch.Status == model.CoverageBatchSucceeded)
	return &CoveragePipelineHealth{LatestBatch: batch, Failures: failures, Healthy: healthy}, nil
}

func (s *SupportCoverageV2Service) ReplayAttempt(ctx context.Context, workspaceID, attemptID string) error {
	return s.repo.ReplayAttempt(ctx, workspaceID, attemptID)
}

func (s *SupportCoverageV2Service) ListArchivedV1(ctx context.Context, workspaceID string, limit, offset int) ([]model.SupportCoverageGap, error) {
	return s.repo.ListArchivedV1Gaps(ctx, workspaceID, limit, offset)
}
