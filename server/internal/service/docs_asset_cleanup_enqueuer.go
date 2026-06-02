package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"

	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

type temporalDocsAssetCleanupEnqueuer struct {
	client    tclient.Client
	taskQueue string
}

func NewTemporalDocsAssetCleanupEnqueuer(client tclient.Client) DocsAssetCleanupEnqueuer {
	if client == nil {
		return nil
	}
	return &temporalDocsAssetCleanupEnqueuer{client: client, taskQueue: temporalapp.QueueAutomation}
}

func (e *temporalDocsAssetCleanupEnqueuer) EnqueueDocsAssetCleanup(ctx context.Context, workspaceID, deletedDocumentID string, candidateAssetKeys []string) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal client is not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	deletedDocumentID = strings.TrimSpace(deletedDocumentID)
	keys := make([]string, 0, len(candidateAssetKeys))
	seen := map[string]struct{}{}
	for _, key := range candidateAssetKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if workspaceID == "" || deletedDocumentID == "" || len(keys) == 0 {
		return nil
	}

	input := temporalapp.DocsAssetCleanupInput{
		WorkspaceID:         workspaceID,
		DeletedDocumentID:   deletedDocumentID,
		CandidateAssetKeys:  keys,
		RequestedAtUnixNano: time.Now().UTC().UnixNano(),
	}
	_, err := e.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:                    temporalapp.WorkflowIDForDocsAssetCleanup(workspaceID, deletedDocumentID, input.RequestedAtUnixNano),
		TaskQueue:             e.taskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, temporalapp.DocsAssetCleanupWorkflowType, input)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return err
	}
	return nil
}
