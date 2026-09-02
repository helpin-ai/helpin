package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

type CoverageRebuildStore interface {
	PreviewRebuild(ctx context.Context, workspaceID string) (*repository.CoverageRebuildPreview, error)
	ApplyRebuild(ctx context.Context, workspaceID, analyzerVersion, policyVersion string, preview *repository.CoverageRebuildPreview, qualified []repository.CoverageRebuildSearch) (*repository.CoverageRebuildApplyResult, error)
	ResumeRebuild(ctx context.Context, workspaceID, auditID string) (*repository.CoverageRebuildApplyResult, error)
	RollbackRebuild(ctx context.Context, workspaceID, auditID string) error
}

type CoverageRebuildOptions struct {
	WorkspaceID string
	Apply       bool
	ResumeID    string
	RollbackID  string
}

type CoverageRebuildResult struct {
	DryRun               bool   `json:"dry_run"`
	AuditID              string `json:"audit_id,omitempty"`
	LegacyGapCount       int    `json:"legacy_gap_count"`
	EvidenceCount        int    `json:"evidence_count"`
	ConversationCount    int    `json:"conversation_count"`
	SearchCount          int    `json:"search_count"`
	QualifiedSearchCount int    `json:"qualified_search_count"`
	QueuedWorkCount      int    `json:"queued_work_count"`
	RolledBack           bool   `json:"rolled_back"`
}

type SupportCoverageRebuildService struct{ store CoverageRebuildStore }

func NewSupportCoverageRebuildService(store CoverageRebuildStore) *SupportCoverageRebuildService {
	return &SupportCoverageRebuildService{store: store}
}

func (s *SupportCoverageRebuildService) Run(ctx context.Context, options CoverageRebuildOptions) (*CoverageRebuildResult, error) {
	workspaceID := strings.TrimSpace(options.WorkspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required; all-workspace rebuilds are forbidden")
	}
	if options.Apply && options.RollbackID != "" {
		return nil, fmt.Errorf("apply and rollback are mutually exclusive")
	}
	if options.RollbackID != "" {
		if err := s.store.RollbackRebuild(ctx, workspaceID, options.RollbackID); err != nil {
			return nil, err
		}
		return &CoverageRebuildResult{RolledBack: true, AuditID: options.RollbackID}, nil
	}
	if options.ResumeID != "" {
		resumed, err := s.store.ResumeRebuild(ctx, workspaceID, options.ResumeID)
		if err != nil {
			return nil, err
		}
		return &CoverageRebuildResult{AuditID: resumed.AuditID, QueuedWorkCount: resumed.QueuedWorkCount}, nil
	}
	preview, err := s.store.PreviewRebuild(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	qualified := make([]repository.CoverageRebuildSearch, 0, len(preview.Searches))
	for _, search := range preview.Searches {
		search.MeaningfulTokens = MeaningfulCoverageSearchTokens(search.Query)
		if search.MeaningfulTokens >= 3 {
			qualified = append(qualified, search)
		}
	}
	result := &CoverageRebuildResult{DryRun: !options.Apply, LegacyGapCount: preview.LegacyGapCount, EvidenceCount: preview.EvidenceCount, ConversationCount: len(preview.ConversationIDs), SearchCount: len(preview.Searches), QualifiedSearchCount: len(qualified), QueuedWorkCount: len(preview.ConversationIDs)}
	if !options.Apply {
		return result, nil
	}
	applied, err := s.store.ApplyRebuild(ctx, workspaceID, "v4", "v1", preview, qualified)
	if err != nil {
		return nil, err
	}
	result.DryRun = false
	result.AuditID = applied.AuditID
	result.QueuedWorkCount = applied.QueuedWorkCount
	return result, nil
}
