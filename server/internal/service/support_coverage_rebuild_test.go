package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeCoverageRebuildStore struct {
	preview   *repository.CoverageRebuildPreview
	applies   int
	rollbacks int
}

func (f *fakeCoverageRebuildStore) PreviewRebuild(context.Context, string) (*repository.CoverageRebuildPreview, error) {
	return f.preview, nil
}
func (f *fakeCoverageRebuildStore) ApplyRebuild(context.Context, string, string, string, *repository.CoverageRebuildPreview, []repository.CoverageRebuildSearch) (*repository.CoverageRebuildApplyResult, error) {
	f.applies++
	return &repository.CoverageRebuildApplyResult{AuditID: "audit-1", QueuedWorkCount: 180}, nil
}
func (f *fakeCoverageRebuildStore) ResumeRebuild(context.Context, string, string) (*repository.CoverageRebuildApplyResult, error) {
	return &repository.CoverageRebuildApplyResult{AuditID: "audit-1", QueuedWorkCount: 180}, nil
}
func (f *fakeCoverageRebuildStore) RollbackRebuild(context.Context, string, string) error {
	f.rollbacks++
	return nil
}

func TestCoverageRebuildDryRunIsDefaultAndMatchesCorrectedShape(t *testing.T) {
	conversationIDs := make([]string, 180)
	searches := make([]repository.CoverageRebuildSearch, 105)
	for i := range searches {
		searches[i].Query = "reset account password"
	}
	store := &fakeCoverageRebuildStore{preview: &repository.CoverageRebuildPreview{LegacyGapCount: 259, EvidenceCount: 563, ConversationIDs: conversationIDs, Searches: searches}}
	result, err := NewSupportCoverageRebuildService(store).Run(context.Background(), CoverageRebuildOptions{WorkspaceID: "ws-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || store.applies != 0 || result.ConversationCount != 180 || result.SearchCount != 105 || result.QualifiedSearchCount != 105 {
		t.Fatalf("result/store = %+v/%+v", result, store)
	}
}

func TestCoverageRebuildApplyAndRollbackAreExplicitAndScoped(t *testing.T) {
	store := &fakeCoverageRebuildStore{preview: &repository.CoverageRebuildPreview{}}
	service := NewSupportCoverageRebuildService(store)
	if _, err := service.Run(context.Background(), CoverageRebuildOptions{Apply: true}); err == nil {
		t.Fatal("expected workspace requirement")
	}
	result, err := service.Run(context.Background(), CoverageRebuildOptions{WorkspaceID: "ws-1", Apply: true})
	if err != nil || result.DryRun || store.applies != 1 {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	result, err = service.Run(context.Background(), CoverageRebuildOptions{WorkspaceID: "ws-1", RollbackID: "audit-1"})
	if err != nil || !result.RolledBack || store.rollbacks != 1 {
		t.Fatalf("rollback = %+v, %v", result, err)
	}
}
