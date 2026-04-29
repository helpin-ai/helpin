package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SupportCoverageAnalysisRepository stores daily analyzer runs, per-conversation
// decisions, live AI retrieval traces, and recommended fixes.
type SupportCoverageAnalysisRepository struct {
	db *gorm.DB
}

func NewSupportCoverageAnalysisRepository(db *gorm.DB) *SupportCoverageAnalysisRepository {
	return &SupportCoverageAnalysisRepository{db: db}
}

func (r *SupportCoverageAnalysisRepository) CreateRun(ctx context.Context, run *model.SupportCoverageAnalysisRun) (*model.SupportCoverageAnalysisRun, error) {
	if run.WorkspaceID == "" || run.WindowStart.IsZero() || run.WindowEnd.IsZero() {
		return nil, fmt.Errorf("workspace_id, window_start, and window_end are required")
	}
	if run.AnalyzerVersion == "" {
		run.AnalyzerVersion = "v1"
	}

	var existing model.SupportCoverageAnalysisRun
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND window_start = ? AND window_end = ? AND analyzer_version = ?",
			run.WorkspaceID, run.WindowStart, run.WindowEnd, run.AnalyzerVersion).
		First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("lookup analysis run: %w", err)
	}

	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	if run.Status == "" {
		run.Status = model.SupportCoverageAnalysisRunStatusRunning
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now()
	}
	if run.Metadata == nil {
		run.Metadata = []byte("{}")
	}

	if err := r.db.WithContext(ctx).Create(run).Error; err != nil {
		var raceExisting model.SupportCoverageAnalysisRun
		if findErr := r.db.WithContext(ctx).
			Where("workspace_id = ? AND window_start = ? AND window_end = ? AND analyzer_version = ?",
				run.WorkspaceID, run.WindowStart, run.WindowEnd, run.AnalyzerVersion).
			First(&raceExisting).Error; findErr == nil {
			return &raceExisting, nil
		}
		return nil, fmt.Errorf("create analysis run: %w", err)
	}
	return run, nil
}

func (r *SupportCoverageAnalysisRepository) CompleteRun(ctx context.Context, runID string, conversationCount, gapCount int) error {
	if runID == "" {
		return fmt.Errorf("run_id is required")
	}
	now := time.Now()
	updates := map[string]interface{}{
		"status":           model.SupportCoverageAnalysisRunStatusCompleted,
		"conversation_cnt": conversationCount,
		"gap_count":        gapCount,
		"completed_at":     now,
		"updated_at":       now,
		"error_message":    nil,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportCoverageAnalysisRun{}).
		Where("id = ?", runID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("complete analysis run: %w", err)
	}
	return nil
}

func (r *SupportCoverageAnalysisRepository) FailRun(ctx context.Context, runID string, runErr error) error {
	if runID == "" {
		return fmt.Errorf("run_id is required")
	}
	now := time.Now()
	message := ""
	if runErr != nil {
		message = runErr.Error()
	}
	updates := map[string]interface{}{
		"status":        model.SupportCoverageAnalysisRunStatusFailed,
		"error_message": message,
		"completed_at":  now,
		"updated_at":    now,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportCoverageAnalysisRun{}).
		Where("id = ?", runID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("fail analysis run: %w", err)
	}
	return nil
}

func (r *SupportCoverageAnalysisRepository) RecordConversationAnalysis(ctx context.Context, analysis *model.SupportCoverageConversationAnalysis) error {
	if analysis.WorkspaceID == "" || analysis.RunID == "" || analysis.ConversationID == "" {
		return fmt.Errorf("workspace_id, run_id, and conversation_id are required")
	}
	if analysis.TranscriptHash == "" {
		return fmt.Errorf("transcript_hash is required")
	}
	if analysis.AnalyzerVersion == "" {
		analysis.AnalyzerVersion = "v1"
	}

	var existing model.SupportCoverageConversationAnalysis
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ? AND transcript_hash = ? AND analyzer_version = ?",
			analysis.WorkspaceID, analysis.ConversationID, analysis.TranscriptHash, analysis.AnalyzerVersion).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("lookup conversation analysis: %w", err)
	}

	if analysis.ID == "" {
		analysis.ID = uuid.New().String()
	}
	if analysis.Status == "" {
		analysis.Status = model.SupportCoverageConversationAnalysisStatusAnalyzed
	}
	if analysis.RawOutput == nil {
		analysis.RawOutput = []byte("{}")
	}

	if err := r.db.WithContext(ctx).Create(analysis).Error; err != nil {
		var raceExisting model.SupportCoverageConversationAnalysis
		if findErr := r.db.WithContext(ctx).
			Where("workspace_id = ? AND conversation_id = ? AND transcript_hash = ? AND analyzer_version = ?",
				analysis.WorkspaceID, analysis.ConversationID, analysis.TranscriptHash, analysis.AnalyzerVersion).
			First(&raceExisting).Error; findErr == nil {
			return nil
		}
		return fmt.Errorf("record conversation analysis: %w", err)
	}
	return nil
}

func (r *SupportCoverageAnalysisRepository) AlreadyAnalyzedConversation(ctx context.Context, workspaceID, conversationID, transcriptHash, analyzerVersion string) (bool, error) {
	if workspaceID == "" || conversationID == "" || transcriptHash == "" {
		return false, fmt.Errorf("workspace_id, conversation_id, and transcript_hash are required")
	}
	if analyzerVersion == "" {
		analyzerVersion = "v1"
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.SupportCoverageConversationAnalysis{}).
		Where("workspace_id = ? AND conversation_id = ? AND transcript_hash = ? AND analyzer_version = ? AND status != ?",
			workspaceID, conversationID, transcriptHash, analyzerVersion, model.SupportCoverageConversationAnalysisStatusFailed).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check conversation analysis: %w", err)
	}
	return count > 0, nil
}

func (r *SupportCoverageAnalysisRepository) LastSuccessfulCursor(ctx context.Context, workspaceID, analyzerVersion string) (*time.Time, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if analyzerVersion == "" {
		analyzerVersion = "v1"
	}
	var run model.SupportCoverageAnalysisRun
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND analyzer_version = ? AND status = ?",
			workspaceID, analyzerVersion, model.SupportCoverageAnalysisRunStatusCompleted).
		Order("cursor_ended_at DESC").
		First(&run).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get last successful cursor: %w", err)
	}
	return &run.CursorEndedAt, nil
}

func (r *SupportCoverageAnalysisRepository) UpsertRetrievalTrace(ctx context.Context, trace *model.SupportAIRetrievalTrace) error {
	if trace.WorkspaceID == "" || trace.ConversationID == "" || trace.MessageID == "" {
		return fmt.Errorf("workspace_id, conversation_id, and message_id are required")
	}
	if trace.ID == "" {
		trace.ID = uuid.New().String()
	}
	if trace.SearchQueries == nil {
		trace.SearchQueries = []byte("[]")
	}
	if trace.Results == nil {
		trace.Results = []byte("[]")
	}
	if trace.CitedSourceIDs == nil {
		trace.CitedSourceIDs = []byte("[]")
	}
	if trace.Metadata == nil {
		trace.Metadata = []byte("{}")
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "workspace_id"}, {Name: "message_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"conversation_id",
				"search_queries",
				"results",
				"cited_source_ids",
				"ai_confidence",
				"can_answer",
				"can_resolve",
				"failure_mode",
				"metadata",
			}),
		}).
		Create(trace).Error; err != nil {
		return fmt.Errorf("upsert retrieval trace: %w", err)
	}
	return nil
}

func (r *SupportCoverageAnalysisRepository) ListRetrievalTracesByConversation(ctx context.Context, workspaceID, conversationID string) ([]model.SupportAIRetrievalTrace, error) {
	if workspaceID == "" || conversationID == "" {
		return nil, fmt.Errorf("workspace_id and conversation_id are required")
	}
	var traces []model.SupportAIRetrievalTrace
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).
		Order("created_at ASC").
		Find(&traces).Error; err != nil {
		return nil, fmt.Errorf("list retrieval traces: %w", err)
	}
	return traces, nil
}

func (r *SupportCoverageAnalysisRepository) ReplaceRecommendations(ctx context.Context, workspaceID, gapID string, recommendations []model.SupportCoverageRecommendation) error {
	if workspaceID == "" || gapID == "" {
		return fmt.Errorf("workspace_id and gap_id are required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("workspace_id = ? AND gap_id = ? AND status = ?",
				workspaceID, gapID, model.SupportCoverageRecommendationStatusOpen).
			Delete(&model.SupportCoverageRecommendation{}).Error; err != nil {
			return fmt.Errorf("delete open recommendations: %w", err)
		}

		for i := range recommendations {
			rec := &recommendations[i]
			if rec.ID == "" {
				rec.ID = uuid.New().String()
			}
			rec.WorkspaceID = workspaceID
			rec.GapID = gapID
			if rec.Status == "" {
				rec.Status = model.SupportCoverageRecommendationStatusOpen
			}
			if rec.Priority == "" {
				rec.Priority = model.SupportCoverageRecommendationPrioritySecondary
			}
			if rec.Metadata == nil {
				rec.Metadata = []byte("{}")
			}
		}
		if len(recommendations) == 0 {
			return nil
		}
		if err := tx.Create(&recommendations).Error; err != nil {
			return fmt.Errorf("create replacement recommendations: %w", err)
		}
		return nil
	})
}
