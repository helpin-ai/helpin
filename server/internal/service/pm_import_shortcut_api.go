package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type shortcutAPIImportDataset struct {
	Rows       []shortcutCSVRow
	Stories    []shortcutAPIStory
	Docs       []shortcutAPIDocSlim
	Enrichment *shortcutAPIEnrichment
	Warnings   []string
}

type shortcutAPIPreviewScanSnapshot struct {
	Request model.ShortcutAPIImportPreviewRequest `json:"request"`
	Dataset *shortcutAPIImportDataset             `json:"dataset,omitempty"`
}

func (s *PMImportService) PreviewShortcutAPI(ctx context.Context, workspaceID, actorID string, req model.ShortcutAPIImportPreviewRequest) (*model.ShortcutImportPreviewResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.APIToken) == "" {
		return nil, fmt.Errorf("Shortcut API token is required")
	}
	s.publishShortcutAPIScanProgress(workspaceID, actorID, req.ScanID, "validating_token", "Validating Shortcut token", 0, 0)
	client := NewShortcutAPIClient(req.APIToken)
	if _, err := client.GetCurrentMember(ctx); err != nil {
		s.publishShortcutAPIScanProgress(workspaceID, actorID, req.ScanID, "failed", "Shortcut token validation failed", 0, 0)
		return nil, err
	}
	dataset, err := s.fetchShortcutAPIImportDataset(ctx, client, req.Options, shortcutAPIScanReporter{
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ScanID:      req.ScanID,
		Service:     s,
	})
	if err != nil {
		s.publishShortcutAPIScanProgress(workspaceID, actorID, req.ScanID, "failed", "Shortcut scan failed", 0, 0)
		return nil, err
	}
	resp, err := s.buildShortcutPreviewFromRows(ctx, workspaceID, filterShortcutRows(dataset.Rows, req.Options), dataset.Enrichment, dataset.Docs, dataset.Warnings)
	if err != nil {
		s.publishShortcutAPIScanProgress(workspaceID, actorID, req.ScanID, "failed", "Shortcut preview build failed", 0, 0)
		return nil, err
	}
	s.publishShortcutAPIScanProgress(workspaceID, actorID, req.ScanID, "ready", "Shortcut preview is ready", len(resp.Workflows), len(resp.Workflows))
	return resp, nil
}

func (s *PMImportService) StartShortcutAPIPreviewScan(ctx context.Context, workspaceID, actorID string, req model.ShortcutAPIImportPreviewRequest) (*model.ShortcutAPIPreviewStartResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.APIToken) == "" {
		return nil, fmt.Errorf("Shortcut API token is required")
	}
	scanID := strings.TrimSpace(req.ScanID)
	if _, err := uuid.Parse(scanID); err != nil {
		scanID = uuid.NewString()
	}
	now := time.Now().UTC()
	job := &model.PMImportJob{
		ID:               scanID,
		WorkspaceID:      workspaceID,
		Source:           model.PMImportSourceShortcutAPIPreview,
		Status:           model.PMImportStatusPending,
		FileName:         "shortcut-api-preview",
		CurrentStep:      "queued",
		StepsTotal:       1,
		StartedBy:        actorID,
		CreatedAt:        now,
		UpdatedAt:        now,
		PayloadEncrypted: nil,
	}
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("create Shortcut preview scan: %w", err)
	}
	req.ScanID = scanID
	go s.runShortcutAPIPreviewScan(scanID, workspaceID, actorID, req)
	return &model.ShortcutAPIPreviewStartResponse{ScanID: scanID, Status: model.PMImportStatusPending}, nil
}

func (s *PMImportService) GetShortcutAPIPreviewScan(ctx context.Context, workspaceID, actorID, scanID string) (*model.ShortcutAPIPreviewScanResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if err := s.reconcileStaleShortcutPreviewScans(ctx, workspaceID); err != nil {
		return nil, err
	}
	var job model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND source = ?", scanID, workspaceID, model.PMImportSourceShortcutAPIPreview).
		First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Shortcut preview scan not found")
		}
		return nil, fmt.Errorf("get Shortcut preview scan: %w", err)
	}
	resp := &model.ShortcutAPIPreviewScanResponse{
		ScanID: job.ID,
		Status: job.Status,
		Progress: model.ShortcutImportStatusProgress{
			CurrentStep:       job.CurrentStep,
			StepsCompleted:    job.StepsCompleted,
			StepsTotal:        job.StepsTotal,
			EntitiesProcessed: job.EntitiesProcessed,
			EntitiesTotal:     job.EntitiesTotal,
		},
		Error:     job.Error,
		CreatedAt: &job.CreatedAt,
		UpdatedAt: &job.UpdatedAt,
	}
	if job.Result != nil && strings.TrimSpace(*job.Result) != "" {
		var preview model.ShortcutImportPreviewResponse
		if err := json.Unmarshal([]byte(*job.Result), &preview); err == nil {
			resp.Preview = &preview
		}
	}
	return resp, nil
}

func (s *PMImportService) reconcileStaleShortcutPreviewScans(ctx context.Context, workspaceID string) error {
	cutoff := time.Now().UTC().Add(-shortcutImportStaleAfter)
	msg := fmt.Sprintf("Shortcut preview scan did not report progress for %s. Start a new preview scan.", shortcutImportStaleAfter)
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&model.PMImportJob{}).
		Where("workspace_id = ? AND source = ? AND status IN ? AND updated_at < ?", workspaceID, model.PMImportSourceShortcutAPIPreview, []string{
			model.PMImportStatusPending,
			model.PMImportStatusScanning,
		}, cutoff).
		Updates(map[string]interface{}{
			"status":       model.PMImportStatusFailed,
			"current_step": "failed",
			"error":        &msg,
			"completed_at": &now,
			"updated_at":   now,
		}).Error; err != nil {
		return fmt.Errorf("reconcile stale Shortcut preview scans: %w", err)
	}
	return nil
}

func (s *PMImportService) runShortcutAPIPreviewScan(scanID, workspaceID, actorID string, req model.ShortcutAPIImportPreviewRequest) {
	ctx := context.Background()
	if err := s.updatePreviewScan(ctx, scanID, map[string]interface{}{
		"status":       model.PMImportStatusScanning,
		"current_step": "validating_token",
		"updated_at":   time.Now().UTC(),
	}); err != nil {
		return
	}
	client := NewShortcutAPIClient(req.APIToken)
	s.publishShortcutAPIScanProgress(workspaceID, actorID, scanID, "validating_token", "Validating Shortcut token", 0, 0)
	if _, err := client.GetCurrentMember(ctx); err != nil {
		s.failShortcutAPIPreviewScan(ctx, workspaceID, actorID, scanID, "Shortcut token validation failed", err)
		return
	}
	reporter := shortcutAPIScanReporter{
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ScanID:      scanID,
		JobID:       scanID,
		Service:     s,
	}
	dataset, err := s.fetchShortcutAPIImportDataset(ctx, client, req.Options, reporter)
	if err != nil {
		s.failShortcutAPIPreviewScan(ctx, workspaceID, actorID, scanID, "Shortcut scan failed", err)
		return
	}
	_ = s.updatePreviewScan(ctx, scanID, map[string]interface{}{
		"current_step":       "building_preview",
		"entities_processed": len(dataset.Rows),
		"entities_total":     len(dataset.Rows),
		"updated_at":         time.Now().UTC(),
	})
	resp, err := s.buildShortcutPreviewFromRows(ctx, workspaceID, filterShortcutRows(dataset.Rows, req.Options), dataset.Enrichment, dataset.Docs, dataset.Warnings)
	if err != nil {
		s.failShortcutAPIPreviewScan(ctx, workspaceID, actorID, scanID, "Shortcut preview build failed", err)
		return
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		s.failShortcutAPIPreviewScan(ctx, workspaceID, actorID, scanID, "Shortcut preview encoding failed", err)
		return
	}
	updates := map[string]interface{}{
		"status":             model.PMImportStatusReady,
		"current_step":       "ready",
		"steps_completed":    1,
		"steps_total":        1,
		"entities_processed": len(dataset.Rows),
		"entities_total":     len(dataset.Rows),
		"total_rows":         len(dataset.Rows),
		"progress":           100,
		"result":             string(raw),
		"updated_at":         time.Now().UTC(),
	}
	if encrypted, ok := s.encryptedShortcutAPIPreviewSnapshot(req, dataset); ok {
		updates["payload_encrypted"] = encrypted
	}
	if err := s.updatePreviewScan(ctx, scanID, updates); err != nil {
		return
	}
	s.publishShortcutAPIScanProgress(workspaceID, actorID, scanID, "ready", "Shortcut preview is ready", len(dataset.Rows), len(dataset.Rows))
}

func (s *PMImportService) failShortcutAPIPreviewScan(ctx context.Context, workspaceID, actorID, scanID, message string, err error) {
	errText := err.Error()
	now := time.Now().UTC()
	_ = s.updatePreviewScan(ctx, scanID, map[string]interface{}{
		"status":       model.PMImportStatusFailed,
		"current_step": "failed",
		"error":        &errText,
		"completed_at": &now,
		"updated_at":   now,
	})
	s.publishShortcutAPIScanProgress(workspaceID, actorID, scanID, "failed", message, 0, 0)
}

func (s *PMImportService) encryptedShortcutAPIPreviewSnapshot(req model.ShortcutAPIImportPreviewRequest, dataset *shortcutAPIImportDataset) (string, bool) {
	if len(s.encryptionKey) != 32 || dataset == nil {
		return "", false
	}
	snapshotDataset := *dataset
	snapshotDataset.Stories = nil
	raw, err := json.Marshal(shortcutAPIPreviewScanSnapshot{Request: req, Dataset: &snapshotDataset})
	if err != nil {
		return "", false
	}
	encrypted, err := appcrypto.EncryptString(string(raw), s.encryptionKey)
	if err != nil {
		return "", false
	}
	return encrypted, true
}

func (s *PMImportService) ExecuteShortcutAPI(ctx context.Context, workspaceID, actorID string, req model.ShortcutAPIImportExecuteRequest) (*model.ShortcutImportExecuteResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.APIToken) == "" && strings.TrimSpace(req.PreviewScanID) == "" {
		return nil, fmt.Errorf("Shortcut API token or preview scan is required")
	}
	if req.UserMappings == nil {
		req.UserMappings = map[string]string{}
	}
	if req.MemberMappings == nil {
		req.MemberMappings = map[string]string{}
	}
	now := time.Now().UTC()
	job := &model.PMImportJob{
		WorkspaceID: workspaceID,
		Source:      model.PMImportSourceShortcut,
		Status:      model.PMImportStatusPending,
		FileName:    "shortcut-api",
		StartedBy:   actorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("create import job: %w", err)
	}
	if started, err := s.startShortcutAPIImportWorkflow(ctx, job, req); err != nil {
		return nil, err
	} else if !started {
		go s.runShortcutAPIImport(job.ID, workspaceID, actorID, req)
	}
	return &model.ShortcutImportExecuteResponse{ImportID: job.ID, Status: model.PMImportStatusProcessing}, nil
}

func (s *PMImportService) runShortcutAPIImport(jobID, workspaceID, actorID string, req model.ShortcutAPIImportExecuteRequest) {
	s.runShortcutAPIImportWithContext(context.Background(), jobID, workspaceID, actorID, req)
}

func (s *PMImportService) runShortcutAPIImportWithContext(ctx context.Context, jobID, workspaceID, actorID string, req model.ShortcutAPIImportExecuteRequest) {
	totalSteps := s.shortcutImportTotalSteps(req.APIToken, req.Options)
	if err := s.updateJob(ctx, jobID, map[string]interface{}{
		"status":             model.PMImportStatusProcessing,
		"current_step":       "api_scan",
		"steps_total":        totalSteps,
		"updated_at":         time.Now().UTC(),
		"entities_total":     0,
		"entities_processed": 0,
	}); err != nil {
		return
	}
	result, totalRows, err := s.executeShortcutAPIImport(ctx, workspaceID, actorID, req, jobID)
	if err != nil {
		errText := err.Error()
		_ = s.updateJob(ctx, jobID, map[string]interface{}{
			"status":       model.PMImportStatusFailed,
			"error":        &errText,
			"completed_at": timePtr(time.Now().UTC()),
			"updated_at":   time.Now().UTC(),
		})
		return
	}
	raw, _ := json.Marshal(result)
	rawStr := string(raw)
	completed := time.Now().UTC()
	_ = s.updateJob(ctx, jobID, map[string]interface{}{
		"status":             model.PMImportStatusCompleted,
		"total_rows":         totalRows,
		"progress":           100,
		"current_step":       "completed",
		"steps_completed":    totalSteps,
		"entities_processed": totalRows,
		"result":             &rawStr,
		"completed_at":       &completed,
		"updated_at":         completed,
	})
}

func (s *PMImportService) startShortcutAPIImportWorkflow(ctx context.Context, job *model.PMImportJob, req model.ShortcutAPIImportExecuteRequest) (bool, error) {
	if s == nil || s.temporalClient == nil {
		return false, nil
	}
	if len(s.encryptionKey) != 32 {
		return false, nil
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Errorf("encode Shortcut import payload: %w", err)
	}
	encrypted, err := appcrypto.EncryptString(string(payload), s.encryptionKey)
	if err != nil {
		return false, fmt.Errorf("encrypt Shortcut import payload: %w", err)
	}
	workflowID := temporalapp.WorkflowIDForShortcutImport(job.ID)
	if err := s.updateJob(ctx, job.ID, map[string]interface{}{
		"payload_encrypted": &encrypted,
		"workflow_id":       &workflowID,
		"updated_at":        time.Now().UTC(),
	}); err != nil {
		return false, err
	}
	_, err = s.temporalClient.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: temporalapp.QueueAutomation,
	}, temporalapp.ShortcutImportWorkflow, temporalapp.ShortcutImportWorkflowInput{ImportID: job.ID})
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if !errors.As(err, &alreadyStarted) {
			return false, fmt.Errorf("start Shortcut import workflow: %w", err)
		}
	}
	return true, nil
}

func (s *PMImportService) executeShortcutAPIImport(ctx context.Context, workspaceID, actorID string, req model.ShortcutAPIImportExecuteRequest, jobID string) (*model.ShortcutImportResult, int, error) {
	var dataset *shortcutAPIImportDataset
	if strings.TrimSpace(req.PreviewScanID) != "" {
		snapshot, err := s.shortcutAPIPreviewSnapshot(ctx, workspaceID, req.PreviewScanID)
		if err != nil && strings.TrimSpace(req.APIToken) == "" {
			return nil, 0, err
		}
		if err == nil && snapshot != nil {
			if strings.TrimSpace(req.APIToken) == "" {
				req.APIToken = snapshot.Request.APIToken
			}
			dataset = snapshot.Dataset
		}
	}
	if strings.TrimSpace(req.APIToken) == "" {
		return nil, 0, fmt.Errorf("Shortcut API token is required")
	}
	client := NewShortcutAPIClient(req.APIToken)
	if _, err := client.GetCurrentMember(ctx); err != nil {
		return nil, 0, err
	}
	_ = s.markStep(ctx, jobID, "api_scan", 1, 0, s.shortcutImportTotalSteps(req.APIToken, req.Options))
	if dataset == nil {
		var err error
		dataset, err = s.fetchShortcutAPIImportDataset(ctx, client, req.Options, shortcutAPIScanReporter{})
		if err != nil {
			return nil, 0, err
		}
	}
	result, totalRows, err := s.executeShortcutRows(ctx, workspaceID, actorID, dataset.Rows, dataset.Warnings, req, jobID, req.APIToken, client, dataset.Enrichment, false)
	if err != nil {
		return nil, totalRows, err
	}
	if req.Options.ImportDocs {
		_ = s.markStep(ctx, jobID, "docs", s.shortcutImportTotalSteps(req.APIToken, req.Options)-1, len(dataset.Docs), s.shortcutImportTotalSteps(req.APIToken, req.Options))
		created, skipped, docWarnings := s.importShortcutDocs(ctx, client, workspaceID, actorID, dataset.Docs, req.Options)
		result.DocsCreated = created
		result.DocsSkipped = skipped
		result.Warnings = appendUniqueWarnings(result.Warnings, docWarnings)
	}
	return result, totalRows, nil
}

type shortcutAPIScanReporter struct {
	WorkspaceID string
	ActorID     string
	ScanID      string
	JobID       string
	Service     *PMImportService
}

func (r shortcutAPIScanReporter) publish(phase, message string, processed, total int) {
	if r.Service == nil {
		return
	}
	if strings.TrimSpace(r.JobID) != "" {
		_ = r.Service.updatePreviewScan(context.Background(), r.JobID, map[string]interface{}{
			"status":             model.PMImportStatusScanning,
			"current_step":       phase,
			"entities_processed": processed,
			"entities_total":     total,
			"updated_at":         time.Now().UTC(),
		})
	}
	r.Service.publishShortcutAPIScanProgress(r.WorkspaceID, r.ActorID, r.ScanID, phase, message, processed, total)
}

func (s *PMImportService) updatePreviewScan(ctx context.Context, scanID string, updates map[string]interface{}) error {
	if strings.TrimSpace(scanID) == "" {
		return nil
	}
	return s.db.WithContext(ctx).
		Model(&model.PMImportJob{}).
		Where("id = ? AND source = ?", scanID, model.PMImportSourceShortcutAPIPreview).
		Updates(updates).Error
}

func (s *PMImportService) shortcutAPIPreviewSnapshot(ctx context.Context, workspaceID, scanID string) (*shortcutAPIPreviewScanSnapshot, error) {
	if len(s.encryptionKey) != 32 {
		return nil, fmt.Errorf("Shortcut preview snapshot is unavailable because import encryption is not configured")
	}
	var job model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND source = ? AND status = ?", scanID, workspaceID, model.PMImportSourceShortcutAPIPreview, model.PMImportStatusReady).
		First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Shortcut preview scan is not ready")
		}
		return nil, fmt.Errorf("get Shortcut preview scan: %w", err)
	}
	if job.PayloadEncrypted == nil || strings.TrimSpace(*job.PayloadEncrypted) == "" {
		return nil, fmt.Errorf("Shortcut preview snapshot is unavailable")
	}
	raw, err := appcrypto.DecryptString(*job.PayloadEncrypted, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("Shortcut preview snapshot could not be decrypted")
	}
	var snapshot shortcutAPIPreviewScanSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, fmt.Errorf("Shortcut preview snapshot is invalid")
	}
	if snapshot.Dataset == nil {
		return nil, fmt.Errorf("Shortcut preview snapshot is empty")
	}
	return &snapshot, nil
}

func (s *PMImportService) fetchShortcutAPIImportDataset(ctx context.Context, client *ShortcutAPIClient, options model.ShortcutImportOptions, reporter shortcutAPIScanReporter) (*shortcutAPIImportDataset, error) {
	reporter.publish("fetching_metadata", "Fetching Shortcut metadata", 0, 0)
	enrichment, warnings := client.FetchEnrichment(ctx)
	queryOptions := shortcutAPIStorySearchOptionsFromImportOptions(options)
	queryOptions.OnDetailFetched = func(processed, total, storyID int) {
		reporter.publish("fetching_stories", fmt.Sprintf("Fetching story details %d of %d", processed, total), processed, total)
	}
	reporter.publish("discovering_stories", "Discovering Shortcut stories", 0, 0)
	stories, _, err := client.ListAllStories(ctx, queryOptions)
	if err != nil {
		return nil, fmt.Errorf("fetch Shortcut stories: %w", err)
	}
	reporter.publish("normalizing_stories", "Normalizing Shortcut stories", len(stories), len(stories))
	iterationStoryIDs, iterationWarnings := s.fetchShortcutIterationStoryIDs(ctx, client, enrichment, reporter)
	warnings = appendUniqueWarnings(warnings, iterationWarnings)
	rows := shortcutAPIStoriesToRows(stories, enrichment, iterationStoryIDs)
	var scopeWarnings []string
	rows, scopeWarnings = filterShortcutAPIRowsByEntityScope(rows, enrichment, options)
	warnings = appendUniqueWarnings(warnings, scopeWarnings)
	var docs []shortcutAPIDocSlim
	if options.ImportDocs {
		reporter.publish("fetching_docs", "Fetching Shortcut docs", 0, 0)
		var err error
		docs, err = client.ListDocs(ctx)
		if err != nil {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut docs could not be scanned: %v", err)})
		} else {
			var docFilterWarnings []string
			docs, docFilterWarnings = filterShortcutDocsByScope(ctx, client, docs, options)
			warnings = appendUniqueWarnings(warnings, docFilterWarnings)
		}
	}
	if queryOptions.MaxStories > 0 && len(stories) >= queryOptions.MaxStories {
		warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut API story scan limited to %d stories", queryOptions.MaxStories)})
	}
	return &shortcutAPIImportDataset{
		Rows:       rows,
		Stories:    stories,
		Docs:       docs,
		Enrichment: enrichment,
		Warnings:   warnings,
	}, nil
}

func (s *PMImportService) fetchShortcutIterationStoryIDs(ctx context.Context, client *ShortcutAPIClient, enrichment *shortcutAPIEnrichment, reporter shortcutAPIScanReporter) (map[int]int, []string) {
	if client == nil || enrichment == nil || len(enrichment.Iterations) == 0 {
		return nil, nil
	}
	iterationKeys := make([]string, 0, len(enrichment.Iterations))
	for key := range enrichment.Iterations {
		iterationKeys = append(iterationKeys, key)
	}
	sort.Slice(iterationKeys, func(i, j int) bool {
		left, _ := strconv.Atoi(iterationKeys[i])
		right, _ := strconv.Atoi(iterationKeys[j])
		return left < right
	})

	storyIterationIDs := map[int]int{}
	warnings := []string{}
	for idx, key := range iterationKeys {
		iterationID, err := strconv.Atoi(key)
		if err != nil || iterationID == 0 {
			continue
		}
		name := enrichment.Iterations[key].Name
		reporter.publish("fetching_iterations", fmt.Sprintf("Mapping Shortcut iteration stories %d of %d", idx+1, len(iterationKeys)), idx+1, len(iterationKeys))
		storyIDs, err := client.ListIterationStoryIDs(ctx, iterationID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Shortcut iteration %q could not be scanned for story membership: %v", fallbackName(name, key), err))
			continue
		}
		for _, storyID := range storyIDs {
			if _, exists := storyIterationIDs[storyID]; !exists {
				storyIterationIDs[storyID] = iterationID
			}
		}
	}
	return storyIterationIDs, warnings
}

func (s *PMImportService) publishShortcutAPIScanProgress(workspaceID, actorID, scanID, phase, message string, processed, total int) {
	if s.publisher == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(scanID) == "" {
		return
	}
	data, _ := json.Marshal(map[string]interface{}{
		"scan_id":   scanID,
		"phase":     phase,
		"message":   message,
		"processed": processed,
		"total":     total,
	})
	s.publisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "pm_import_preview",
		EntityID:    scanID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		Data:        data,
	})
}

func shortcutAPIStorySearchOptionsFromImportOptions(options model.ShortcutImportOptions) shortcutAPIStorySearchOptions {
	out := shortcutAPIStorySearchOptions{
		SortField: normalizeShortcutStoryDateField(options.TaskDateField),
	}
	if options.MaxTasks > 0 {
		out.MaxStories = options.MaxTasks
	}
	if options.TaskLookbackMonths > 0 {
		cutoff := time.Now().UTC().AddDate(0, -options.TaskLookbackMonths, 0).Format(time.RFC3339)
		if out.SortField == "created_at" {
			out.CreatedAtStart = cutoff
		} else {
			out.UpdatedAtStart = cutoff
		}
	}
	return out
}

func normalizeShortcutStoryDateField(raw string) string {
	switch normalizeShortcutName(raw) {
	case "created_at", "created at", "created":
		return "created_at"
	default:
		return "updated_at"
	}
}

func filterShortcutAPIRowsByEntityScope(rows []shortcutCSVRow, enrichment *shortcutAPIEnrichment, options model.ShortcutImportOptions) ([]shortcutCSVRow, []string) {
	if enrichment == nil || (options.EpicLookbackMonths <= 0 && options.ObjectiveLookbackMonths <= 0) {
		return rows, nil
	}
	now := time.Now().UTC()
	var epicCutoff *time.Time
	if options.EpicLookbackMonths > 0 {
		cutoff := now.AddDate(0, -options.EpicLookbackMonths, 0)
		epicCutoff = &cutoff
	}
	var objectiveCutoff *time.Time
	if options.ObjectiveLookbackMonths > 0 {
		cutoff := now.AddDate(0, -options.ObjectiveLookbackMonths, 0)
		objectiveCutoff = &cutoff
	}
	filtered := make([]shortcutCSVRow, 0, len(rows))
	skippedByEpic := 0
	skippedByObjective := 0
	for _, row := range rows {
		if epicCutoff != nil && row.EpicID != "" {
			if epic, ok := enrichment.Epics[row.EpicID]; ok {
				if createdAt := parseShortcutAPITimestamp(epic.CreatedAt); createdAt != nil && createdAt.Before(*epicCutoff) {
					skippedByEpic++
					continue
				}
			}
		}
		if objectiveCutoff != nil && row.ObjectiveID != "" {
			if objective, ok := enrichment.Objectives[row.ObjectiveID]; ok {
				if createdAt := parseShortcutAPITimestamp(objective.CreatedAt); createdAt != nil && createdAt.Before(*objectiveCutoff) {
					skippedByObjective++
					continue
				}
			}
		}
		filtered = append(filtered, row)
	}
	warnings := []string{}
	if skippedByEpic > 0 {
		warnings = append(warnings, fmt.Sprintf("%d stories skipped because their Shortcut epic was outside the selected date scope", skippedByEpic))
	}
	if skippedByObjective > 0 {
		warnings = append(warnings, fmt.Sprintf("%d stories skipped because their Shortcut objective was outside the selected date scope", skippedByObjective))
	}
	return filtered, warnings
}

func (s *PMImportService) buildShortcutPreviewFromRows(ctx context.Context, workspaceID string, rows []shortcutCSVRow, enrichment *shortcutAPIEnrichment, docs []shortcutAPIDocSlim, warnings []string) (*model.ShortcutImportPreviewResponse, error) {
	members, err := s.workspaceRepo.ListAssignableMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	memberByEmail := make(map[string]model.AssignableMember, len(members))
	for _, member := range members {
		memberByEmail[normalizeShortcutName(member.Email)] = member
	}

	storyTypeCounts := map[string]int{}
	epicIDs := map[string]struct{}{}
	objectiveIDs := map[string]struct{}{}
	sprintIDs := map[string]struct{}{}
	labelNames := map[string]struct{}{}
	teamCounts := map[string]int{}
	workflowStateCounts := map[string]*shortcutWorkflowAggregate{}
	emailCounts := map[string]int{}
	ownerCounts := map[string]int{}
	requesterCounts := map[string]int{}
	checklistCount := 0
	commentCount := 0
	externalLinkCount := 0
	storyLinkCount := 0
	for _, row := range rows {
		storyTypeCounts[row.Type]++
		if row.EpicID != "" {
			epicIDs[row.EpicID] = struct{}{}
		}
		if row.ObjectiveID != "" {
			objectiveIDs[row.ObjectiveID] = struct{}{}
		}
		if row.IterationID != "" {
			sprintIDs[row.IterationID] = struct{}{}
		}
		for _, label := range append(shortcutLabelNames(row.Labels), shortcutLabelNames(row.EpicLabels)...) {
			labelNames[normalizeShortcutName(label)] = struct{}{}
		}
		if row.Team != "" {
			teamCounts[row.Team]++
		}
		workflowKey := shortcutWorkflowKey(row.WorkflowID, row.Workflow)
		workflow := workflowStateCounts[workflowKey]
		if workflow == nil {
			workflow = &shortcutWorkflowAggregate{ID: row.WorkflowID, Name: shortcutWorkflowDisplayName(row.WorkflowID, row.Workflow), StateCounts: map[string]int{}}
			workflowStateCounts[workflowKey] = workflow
		}
		workflow.StateCounts[row.State]++
		workflow.TaskCount++
		if row.Requester != "" {
			email := normalizeShortcutName(row.Requester)
			emailCounts[email]++
			requesterCounts[email]++
		}
		for _, owner := range shortcutOwnerEmails(row.Owners) {
			email := normalizeShortcutName(owner)
			emailCounts[email]++
			ownerCounts[email]++
		}
		checklistCount += len(parseShortcutChecklist(row.Tasks))
		commentCount += len(row.APIComments)
		externalLinkCount += len(row.APIExternalLinks)
		storyLinkCount += len(row.APIStoryLinks)
	}
	duplicateTasks, err := s.countTaskDuplicates(ctx, workspaceID, mapKeys(shortcutStoryExternalIDs(rows)))
	if err != nil {
		return nil, err
	}
	users := make([]model.ShortcutUserMatch, 0, len(emailCounts))
	for _, email := range sortKeysByCount(emailCounts) {
		match := model.ShortcutUserMatch{
			Email:          email,
			TaskCount:      emailCounts[email],
			OwnerCount:     ownerCounts[email],
			RequesterCount: requesterCounts[email],
		}
		if member, ok := memberByEmail[email]; ok {
			match.MatchedMemberID = &member.ID
			match.MatchedMemberStatus = &member.Status
			match.MatchedName = &member.DisplayName
			if member.UserID != nil && strings.TrimSpace(*member.UserID) != "" {
				match.MatchedUserID = member.UserID
			}
		}
		if enrichment != nil {
			if scMember, ok := enrichment.MembersByEmail[email]; ok {
				match.ShortcutMemberID = &scMember.ID
				if scMember.Profile.Name != "" {
					match.ShortcutName = &scMember.Profile.Name
				}
			}
		}
		users = append(users, match)
	}
	teams := make([]model.ShortcutTeamPreview, 0, len(teamCounts))
	for _, name := range sortKeysByCount(teamCounts) {
		teams = append(teams, model.ShortcutTeamPreview{Name: name, TaskCount: teamCounts[name]})
	}
	workflows := makeShortcutWorkflowPreview(workflowStateCounts)
	allWarnings := append([]string(nil), warnings...)
	if commentCount > 0 {
		allWarnings = append(allWarnings, fmt.Sprintf("%d Shortcut comments detected and will be imported when comments are enabled", commentCount))
	}
	if externalLinkCount > 0 {
		allWarnings = append(allWarnings, fmt.Sprintf("%d Shortcut file or external links detected", externalLinkCount))
	}
	if storyLinkCount > 0 {
		allWarnings = append(allWarnings, fmt.Sprintf("%d Shortcut story relationships detected", storyLinkCount))
	}
	return &model.ShortcutImportPreviewResponse{
		Summary: model.ShortcutImportPreviewSummary{
			TotalTasks:          len(rows),
			TasksByType:         storyTypeCounts,
			EpicsCount:          len(epicIDs),
			ObjectivesCount:     len(objectiveIDs),
			SprintsCount:        len(sprintIDs),
			LabelsCount:         len(labelNames),
			DocsCount:           len(docs),
			TeamsCount:          len(teamCounts),
			WorkflowsCount:      len(workflowStateCounts),
			WorkflowStatesCount: countWorkflowStates(workflowStateCounts),
			ChecklistItemsCount: checklistCount,
			DuplicateTasks:      duplicateTasks,
		},
		Users:     users,
		Teams:     teams,
		Workflows: workflows,
		Warnings:  allWarnings,
	}, nil
}

func makeShortcutWorkflowPreview(workflowStateCounts map[string]*shortcutWorkflowAggregate) []model.ShortcutWorkflowPreview {
	workflowGroups := make([]*shortcutWorkflowAggregate, 0, len(workflowStateCounts))
	for _, workflow := range workflowStateCounts {
		workflowGroups = append(workflowGroups, workflow)
	}
	sort.Slice(workflowGroups, func(i, j int) bool {
		leftName := normalizeShortcutName(workflowGroups[i].Name)
		rightName := normalizeShortcutName(workflowGroups[j].Name)
		if leftName == rightName {
			return workflowGroups[i].ID < workflowGroups[j].ID
		}
		return leftName < rightName
	})
	stateTypeOrder := map[string]int{
		model.PMStateTypeBacklog:   0,
		model.PMStateTypeUnstarted: 1,
		model.PMStateTypeStarted:   2,
		model.PMStateTypeDone:      3,
	}
	workflows := make([]model.ShortcutWorkflowPreview, 0, len(workflowGroups))
	for _, workflow := range workflowGroups {
		states := make([]model.ShortcutWorkflowStatePreview, 0, len(workflow.StateCounts))
		for _, stateName := range sortKeysByCount(workflow.StateCounts) {
			states = append(states, model.ShortcutWorkflowStatePreview{
				Name:          stateName,
				SuggestedType: suggestedStateType(stateName),
				TaskCount:     workflow.StateCounts[stateName],
			})
		}
		sort.SliceStable(states, func(i, j int) bool {
			return stateTypeOrder[states[i].SuggestedType] < stateTypeOrder[states[j].SuggestedType]
		})
		workflows = append(workflows, model.ShortcutWorkflowPreview{
			ID:        workflow.ID,
			Name:      workflow.Name,
			TaskCount: workflow.TaskCount,
			States:    states,
		})
	}
	return workflows
}

func shortcutAPIStoriesToRows(stories []shortcutAPIStory, enrichment *shortcutAPIEnrichment, storyIterationIDs map[int]int) []shortcutCSVRow {
	rows := make([]shortcutCSVRow, 0, len(stories))
	for idx, story := range stories {
		iterationID := story.IterationID
		if iterationID == nil {
			if mappedIterationID, ok := storyIterationIDs[story.ID]; ok && mappedIterationID != 0 {
				id := mappedIterationID
				iterationID = &id
			}
		}
		row := shortcutCSVRow{
			RowNumber:          idx + 1,
			ID:                 strconv.Itoa(story.ID),
			Name:               story.Name,
			Type:               story.StoryType,
			Description:        story.Description,
			IsCompleted:        strconv.FormatBool(story.Completed),
			CreatedAt:          shortcutAPITimeForCSV(story.CreatedAt),
			StartedAt:          shortcutAPITimeForCSV(story.StartedAt),
			UpdatedAt:          shortcutAPITimeForCSV(story.UpdatedAt),
			MovedAt:            shortcutAPITimeForCSV(story.MovedAt),
			CompletedAt:        shortcutAPITimeForCSV(firstNonEmpty(story.CompletedAtOverride, story.CompletedAt)),
			Estimate:           shortcutAPIIntPtrString(story.Estimate),
			IsBlocked:          strconv.FormatBool(story.Blocked || story.Blocker),
			DueDate:            shortcutAPITimeForCSV(story.Deadline),
			State:              shortcutAPIWorkflowStateName(story.WorkflowID, story.WorkflowStateID, enrichment),
			Workflow:           shortcutAPIWorkflowName(story.WorkflowID, enrichment),
			WorkflowID:         strconv.Itoa(story.WorkflowID),
			IterationID:        shortcutAPIIntPtrID(iterationID),
			Iteration:          shortcutAPIIterationName(iterationID, enrichment),
			UTCOffset:          "+00:00",
			IsArchived:         strconv.FormatBool(story.Archived),
			Team:               shortcutAPIStoryTeamName(story, enrichment),
			AppURL:             story.AppURL,
			APIStoryLinks:      append([]shortcutAPIStoryLink(nil), story.StoryLinks...),
			APIComments:        append([]shortcutAPIComment(nil), story.Comments...),
			APICommentsFetched: true,
		}
		if story.RequestedByID != "" {
			row.Requester = shortcutAPIMemberEmail(story.RequestedByID, enrichment)
		}
		owners := make([]string, 0, len(story.OwnerIDs))
		for _, ownerID := range story.OwnerIDs {
			if email := shortcutAPIMemberEmail(ownerID, enrichment); email != "" {
				owners = append(owners, email)
			}
		}
		row.Owners = strings.Join(owners, ";")
		labelNames := make([]string, 0, len(story.Labels))
		for _, label := range story.Labels {
			if label.Name != "" {
				labelNames = append(labelNames, label.Name)
			}
		}
		row.Labels = strings.Join(labelNames, ";")
		if story.EpicID != nil {
			epicID := strconv.Itoa(*story.EpicID)
			row.EpicID = epicID
			if enrichment != nil {
				if epic, ok := enrichment.Epics[epicID]; ok {
					row.Epic = epic.Name
					row.EpicIsArchived = strconv.FormatBool(epic.Archived)
					row.EpicCreatedAt = shortcutAPITimeForCSV(epic.CreatedAt)
					row.EpicStartedAt = shortcutAPITimeForCSV(epic.StartedAt)
					row.EpicDueDate = shortcutAPITimeForCSV(epic.Deadline)
					row.EpicPlannedStartDate = shortcutAPITimeForCSV(epic.PlannedStartDate)
					switch {
					case epic.Completed:
						row.EpicState = "done"
					case epic.Started:
						row.EpicState = "in progress"
					default:
						row.EpicState = "to do"
					}
					epicLabels := make([]string, 0, len(epic.Labels))
					for _, label := range epic.Labels {
						if label.Name != "" {
							epicLabels = append(epicLabels, label.Name)
						}
					}
					row.EpicLabels = strings.Join(epicLabels, ";")
					if len(epic.ObjectiveIDs) > 0 {
						objectiveID := strconv.Itoa(epic.ObjectiveIDs[0])
						row.ObjectiveID = objectiveID
						if objective, ok := enrichment.Objectives[objectiveID]; ok {
							row.Objective = objective.Name
							row.ObjectiveCreatedAt = shortcutAPITimeForCSV(objective.CreatedAt)
							row.ObjectiveStartedAt = shortcutAPITimeForCSV(objective.StartedAt)
							row.ObjectiveDueDate = shortcutAPITimeForCSV(objective.CompletedAt)
							switch {
							case objective.Completed:
								row.ObjectiveState = "done"
							case objective.Started:
								row.ObjectiveState = "in progress"
							default:
								row.ObjectiveState = "to do"
							}
						}
					}
				}
			}
		}
		checklist := make([]string, 0, len(story.Tasks))
		for _, task := range story.Tasks {
			text := strings.TrimSpace(task.Description)
			if text == "" {
				continue
			}
			prefix := "[ ] "
			if task.Complete {
				prefix = "[X] "
			}
			checklist = append(checklist, prefix+text)
		}
		row.Tasks = strings.Join(checklist, ";")
		row.APIExternalLinks = shortcutAPIStoryExternalLinks(story)
		rows = append(rows, row)
	}
	return rows
}

func shortcutAPIStoryExternalLinks(story shortcutAPIStory) []shortcutAPIExternalLink {
	seen := map[string]struct{}{}
	var out []shortcutAPIExternalLink
	add := func(title, rawURL string) {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			return
		}
		if _, ok := seen[rawURL]; ok {
			return
		}
		seen[rawURL] = struct{}{}
		out = append(out, shortcutAPIExternalLink{Title: fallbackName(title, rawURL), URL: rawURL})
	}
	add("Shortcut Story", story.AppURL)
	for _, rawURL := range story.ExternalLinks {
		add("External Link", rawURL)
	}
	for _, file := range story.Files {
		add(fallbackName(firstNonEmpty(file.Name, file.Filename), "Shortcut File"), file.URL)
	}
	for _, file := range story.LinkedFiles {
		add(fallbackName(file.Name, "Shortcut Linked File"), file.URL)
	}
	return out
}

func shortcutAPITimeForCSV(raw string) string {
	ts := parseShortcutAPITimestamp(raw)
	if ts == nil {
		return ""
	}
	return ts.UTC().Format("2006/01/02 15:04:05")
}

func shortcutAPIIntPtrString(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func shortcutAPIIntPtrID(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func shortcutAPIMemberEmail(memberID string, enrichment *shortcutAPIEnrichment) string {
	if enrichment == nil {
		return ""
	}
	if member, ok := enrichment.Members[memberID]; ok {
		return strings.TrimSpace(member.Profile.EmailAddress)
	}
	return ""
}

func shortcutAPIWorkflowName(workflowID int, enrichment *shortcutAPIEnrichment) string {
	if enrichment == nil {
		return fmt.Sprintf("Workflow %d", workflowID)
	}
	if workflow, ok := enrichment.Workflows[strconv.Itoa(workflowID)]; ok {
		return fallbackName(workflow.Name, fmt.Sprintf("Workflow %d", workflowID))
	}
	return fmt.Sprintf("Workflow %d", workflowID)
}

func shortcutAPIWorkflowStateName(workflowID, stateID int, enrichment *shortcutAPIEnrichment) string {
	if enrichment != nil {
		if workflow, ok := enrichment.Workflows[strconv.Itoa(workflowID)]; ok {
			for _, state := range workflow.States {
				if state.ID == stateID {
					return fallbackName(state.Name, fmt.Sprintf("State %d", stateID))
				}
			}
		}
	}
	return fmt.Sprintf("State %d", stateID)
}

func shortcutAPIIterationName(iterationID *int, enrichment *shortcutAPIEnrichment) string {
	if iterationID == nil || enrichment == nil {
		return ""
	}
	if iteration, ok := enrichment.Iterations[strconv.Itoa(*iterationID)]; ok {
		return iteration.Name
	}
	return ""
}

func shortcutAPIStoryTeamName(story shortcutAPIStory, enrichment *shortcutAPIEnrichment) string {
	if enrichment == nil {
		return ""
	}
	if story.GroupID != "" {
		if group, ok := enrichment.Groups[story.GroupID]; ok {
			return group.Name
		}
	}
	if story.ProjectID != nil {
		if project, ok := enrichment.Projects[strconv.Itoa(*story.ProjectID)]; ok && project.TeamID != 0 {
			if group, ok := enrichment.Groups[strconv.Itoa(project.TeamID)]; ok {
				return group.Name
			}
		}
	}
	return ""
}

func (s *PMImportService) createAPITaskExternalLinks(ctx context.Context, tx *gorm.DB, actorID string, task model.PMTask, links []shortcutAPIExternalLink, result *model.ShortcutImportResult) error {
	for _, link := range links {
		if strings.TrimSpace(link.URL) == "" {
			continue
		}
		var existing int64
		if err := tx.WithContext(ctx).Model(&model.PMExternalLink{}).
			Where("task_id = ? AND url = ?", task.ID, strings.TrimSpace(link.URL)).
			Count(&existing).Error; err != nil {
			return fmt.Errorf("check external link: %w", err)
		}
		if existing > 0 {
			continue
		}
		taskID := task.ID
		record := model.PMExternalLink{
			TaskID:      &taskID,
			EntityType:  "task",
			EntityID:    task.ID,
			Title:       fallbackName(link.Title, link.URL),
			URL:         strings.TrimSpace(link.URL),
			CreatedByID: actorID,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
		}
		if err := tx.WithContext(ctx).Create(&record).Error; err != nil {
			return fmt.Errorf("create external link: %w", err)
		}
		result.ExternalLinksCreated++
	}
	return nil
}

func (s *PMImportService) createAPIStoryLinks(ctx context.Context, tx *gorm.DB, workspaceID, actorID string, rows []shortcutCSVRow, storyMap map[string]string, result *model.ShortcutImportResult) error {
	for _, row := range rows {
		for _, link := range row.APIStoryLinks {
			sourceExternalID := strconv.Itoa(link.SubjectID)
			targetExternalID := strconv.Itoa(link.ObjectID)
			sourceID := storyMap[sourceExternalID]
			targetID := storyMap[targetExternalID]
			if sourceID == "" || targetID == "" {
				continue
			}
			linkType := mapShortcutStoryLinkVerb(link.Verb)
			if linkType == "" {
				continue
			}
			var existing int64
			if err := tx.WithContext(ctx).Model(&model.PMTaskLink{}).
				Where("workspace_id = ? AND source_task_id = ? AND target_task_id = ? AND link_type = ?", workspaceID, sourceID, targetID, linkType).
				Count(&existing).Error; err != nil {
				return fmt.Errorf("check task link: %w", err)
			}
			if existing > 0 {
				continue
			}
			record := model.PMTaskLink{
				WorkspaceID:  workspaceID,
				SourceTaskID: sourceID,
				TargetTaskID: targetID,
				LinkType:     linkType,
				CreatedBy:    actorID,
			}
			if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error; err != nil {
				return fmt.Errorf("create task link: %w", err)
			}
			result.TaskLinksCreated++
		}
	}
	return nil
}

func mapShortcutStoryLinkVerb(raw string) string {
	switch normalizeShortcutName(raw) {
	case "blocks":
		return model.PMTaskLinkTypeBlocks
	case "duplicates":
		return model.PMTaskLinkTypeDuplicates
	case "relates to", "relates_to", "relates":
		return model.PMTaskLinkTypeRelatesTo
	default:
		return ""
	}
}
