package temporalapp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	DocsAssetCleanupWorkflowType = "DocsAssetCleanupWorkflow"
	docsAssetCleanupActivityName = "DocsAssetCleanupActivities.CleanupAssetActivity"
)

type DocsAssetCleanupInput struct {
	WorkspaceID         string   `json:"workspace_id"`
	DeletedDocumentID   string   `json:"deleted_document_id"`
	CandidateAssetKeys  []string `json:"candidate_asset_keys"`
	RequestedAtUnixNano int64    `json:"requested_at_unix_nano,omitempty"`
}

type DocsAssetCleanupAssetInput struct {
	WorkspaceID       string `json:"workspace_id"`
	DeletedDocumentID string `json:"deleted_document_id"`
	AssetKey          string `json:"asset_key"`
}

func WorkflowIDForDocsAssetCleanup(workspaceID, deletedDocumentID string, requestedAtUnixNano int64) string {
	return fmt.Sprintf("docs-asset-cleanup-%s-%s-%d", workspaceID, deletedDocumentID, requestedAtUnixNano)
}

func DocsAssetCleanupWorkflow(ctx workflow.Context, input DocsAssetCleanupInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	type cleanupFuture struct {
		key    string
		future workflow.Future
	}
	cleanups := make([]cleanupFuture, 0, len(input.CandidateAssetKeys))
	for _, key := range dedupeWorkflowAssetKeys(input.CandidateAssetKeys) {
		assetInput := DocsAssetCleanupAssetInput{
			WorkspaceID:       input.WorkspaceID,
			DeletedDocumentID: input.DeletedDocumentID,
			AssetKey:          key,
		}
		cleanups = append(cleanups, cleanupFuture{
			key:    key,
			future: workflow.ExecuteActivity(ctx, docsAssetCleanupActivityName, assetInput),
		})
	}

	for _, cleanup := range cleanups {
		if err := cleanup.future.Get(ctx, nil); err != nil {
			logger.Error("docs asset cleanup failed after retries", "workspace_id", input.WorkspaceID, "deleted_document_id", input.DeletedDocumentID, "asset_key", cleanup.key, "error", err)
		}
	}
	return nil
}

type docsAssetReferenceChecker interface {
	HasSurvivingReference(ctx context.Context, workspaceID, deletedDocumentID, assetKey string) (bool, error)
}

type docsAssetObjectStore interface {
	DeleteObject(ctx context.Context, key string) error
}

type DocsAssetCleanupActivities struct {
	refs  docsAssetReferenceChecker
	store docsAssetObjectStore
}

func NewDocsAssetCleanupActivities(refs docsAssetReferenceChecker, store docsAssetObjectStore) *DocsAssetCleanupActivities {
	if refs == nil || store == nil {
		return nil
	}
	return &DocsAssetCleanupActivities{refs: refs, store: store}
}

func (a *DocsAssetCleanupActivities) CleanupAssetActivity(ctx context.Context, input DocsAssetCleanupAssetInput) error {
	if a == nil || a.refs == nil || a.store == nil {
		return fmt.Errorf("docs asset cleanup activity is not configured")
	}
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	deletedDocumentID := strings.TrimSpace(input.DeletedDocumentID)
	assetKey := strings.TrimSpace(input.AssetKey)
	if workspaceID == "" || deletedDocumentID == "" || assetKey == "" {
		return nil
	}

	stillReferenced, err := a.refs.HasSurvivingReference(ctx, workspaceID, deletedDocumentID, assetKey)
	if err != nil {
		slog.WarnContext(ctx, "docs_asset_cleanup_reference_check_failed", "workspace_id", workspaceID, "deleted_document_id", deletedDocumentID, "asset_key", assetKey, "error", err)
		return err
	}
	if stillReferenced {
		slog.InfoContext(ctx, "docs_asset_cleanup_skipped_referenced", "workspace_id", workspaceID, "deleted_document_id", deletedDocumentID, "asset_key", assetKey, "cleanup_result", "referenced")
		return nil
	}

	if err := a.store.DeleteObject(ctx, assetKey); err != nil {
		slog.WarnContext(ctx, "docs_asset_cleanup_delete_failed", "workspace_id", workspaceID, "deleted_document_id", deletedDocumentID, "asset_key", assetKey, "cleanup_result", "delete_failed", "error", err)
		return err
	}
	slog.InfoContext(ctx, "docs_asset_cleanup_deleted", "workspace_id", workspaceID, "deleted_document_id", deletedDocumentID, "asset_key", assetKey, "cleanup_result", "deleted")
	return nil
}

func dedupeWorkflowAssetKeys(keys []string) []string {
	out := make([]string, 0, len(keys))
	seen := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}
