package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SupportCoverageRepository provides data access for coverage topics,
// gaps, evidence, suggestions, snapshots, and digest deliveries.
type SupportCoverageRepository struct {
	db *gorm.DB
}

var ErrGapAlreadyClosed = errors.New("gap is no longer open")

func CoverageGapPairKey(leftGapID, rightGapID string) string {
	ids := []string{strings.TrimSpace(leftGapID), strings.TrimSpace(rightGapID)}
	sort.Strings(ids)
	if ids[0] == "" || ids[1] == "" {
		return ""
	}
	return ids[0] + ":" + ids[1]
}

// NewSupportCoverageRepository creates a new SupportCoverageRepository.
func NewSupportCoverageRepository(db *gorm.DB) *SupportCoverageRepository {
	return &SupportCoverageRepository{db: db}
}

func (r *SupportCoverageRepository) WithTx(tx *gorm.DB) *SupportCoverageRepository {
	return &SupportCoverageRepository{db: tx}
}

func (r *SupportCoverageRepository) ListGapsByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.SupportCoverageGap, error) {
	if len(ids) == 0 {
		return []model.SupportCoverageGap{}, nil
	}
	var gaps []model.SupportCoverageGap
	if err := r.db.WithContext(ctx).
		Select("id", "title").
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&gaps).Error; err != nil {
		return nil, fmt.Errorf("list coverage gaps by ids: %w", err)
	}
	return gaps, nil
}

func (r *SupportCoverageRepository) ListOpenGapsForClusterRebuild(ctx context.Context, workspaceID string, limit int) ([]model.SupportCoverageGapListItem, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	evidenceCutoff := time.Now().AddDate(0, 0, -30)
	customerNeedExpr := "''"
	if r.db.Dialector.Name() == "postgres" {
		customerNeedExpr = "COALESCE(latest_evidence.metadata ->> 'customer_need', '')"
	} else if r.db.Dialector.Name() == "sqlite" {
		customerNeedExpr = "COALESCE(json_extract(latest_evidence.metadata, '$.customer_need'), '')"
	}
	var items []model.SupportCoverageGapListItem
	err := r.db.WithContext(ctx).
		Table("support_coverage_gaps g").
		Select(`g.*,
			COALESCE(t.title, '') AS topic_title,
			COALESCE(t.canonical_title, t.title, '') AS canonical_title,
			COALESCE(latest_evidence.excerpt, '') AS evidence_text,
			`+customerNeedExpr+` AS customer_need_text,
			(SELECT COUNT(*) FROM support_gap_suggestions s WHERE s.gap_id = g.id) AS suggestion_count,
			(SELECT ga.document_id FROM support_coverage_gap_articles ga WHERE ga.gap_id = g.id LIMIT 1) AS related_article_id,
			(SELECT COUNT(DISTINCT COALESCE(e.conversation_id, e.id)) FROM support_gap_evidence e WHERE e.gap_id = g.id AND e.created_at > ?) AS evidence_30d`, evidenceCutoff).
		Joins("LEFT JOIN support_coverage_topics t ON t.id = g.topic_id").
		Joins("LEFT JOIN support_gap_evidence latest_evidence ON latest_evidence.id = (SELECT e2.id FROM support_gap_evidence e2 WHERE e2.gap_id = g.id ORDER BY e2.created_at DESC LIMIT 1)").
		Where("g.workspace_id = ? AND g.status = ?", workspaceID, model.SupportCoverageGapStatusOpen).
		Order("g.evidence_count DESC, g.last_seen_at DESC").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list open gaps for cluster rebuild: %w", err)
	}
	return items, nil
}

func (r *SupportCoverageRepository) FindNearestOpenGapsByEmbedding(ctx context.Context, workspaceID, embedding, provider, modelName, version string, dimensions int, limit int) ([]model.SupportCoverageGapListItem, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if embedding == "" || r.db.Dialector.Name() != "postgres" {
		return []model.SupportCoverageGapListItem{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return r.findNearestGapsByEmbedding(ctx, workspaceID, embedding, provider, modelName, version, dimensions, model.SupportCoverageGapStatusOpen, nil, limit)
}

func (r *SupportCoverageRepository) FindNearestRecentClosedGapsByEmbedding(ctx context.Context, workspaceID, embedding, provider, modelName, version string, dimensions int, since time.Time, limit int) ([]model.SupportCoverageGapListItem, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if embedding == "" || r.db.Dialector.Name() != "postgres" {
		return []model.SupportCoverageGapListItem{}, nil
	}
	if since.IsZero() {
		return []model.SupportCoverageGapListItem{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return r.findNearestGapsByEmbedding(ctx, workspaceID, embedding, provider, modelName, version, dimensions, model.SupportCoverageGapStatusDone, &since, limit)
}

func (r *SupportCoverageRepository) ListRecentDoneGapsForRecurrence(ctx context.Context, workspaceID string, since time.Time, limit int) ([]model.SupportCoverageGapListItem, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	var items []model.SupportCoverageGapListItem
	err := r.db.WithContext(ctx).
		Table("support_coverage_gaps g").
		Select(`g.*,
			COALESCE(t.title, '') AS topic_title,
			COALESCE(t.canonical_title, t.title, '') AS canonical_title`).
		Joins("LEFT JOIN support_coverage_topics t ON t.id = g.topic_id").
		Where("g.workspace_id = ? AND g.status = ? AND g.closed_at >= ?", workspaceID, model.SupportCoverageGapStatusDone, since).
		Order("g.closed_at DESC, g.last_seen_at DESC").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list recent done gaps for recurrence: %w", err)
	}
	return items, nil
}

func (r *SupportCoverageRepository) findNearestGapsByEmbedding(ctx context.Context, workspaceID, embedding, provider, modelName, version string, dimensions int, status string, closedSince *time.Time, limit int) ([]model.SupportCoverageGapListItem, error) {
	evidenceCutoff := time.Now().AddDate(0, 0, -30)
	customerNeedExpr := "COALESCE(latest_evidence.metadata ->> 'customer_need', '')"
	baseSQL := `
		SELECT g.*,
		       COALESCE(t.title, '') AS topic_title,
		       COALESCE(t.canonical_title, t.title, '') AS canonical_title,
		       COALESCE(latest_evidence.excerpt, '') AS evidence_text,
		       ` + customerNeedExpr + ` AS customer_need_text,
		       (SELECT COUNT(*) FROM support_gap_suggestions s WHERE s.gap_id = g.id) AS suggestion_count,
		       (SELECT ga.document_id FROM support_coverage_gap_articles ga WHERE ga.gap_id = g.id LIMIT 1) AS related_article_id,
		       (SELECT COUNT(DISTINCT COALESCE(e.conversation_id, e.id)) FROM support_gap_evidence e WHERE e.gap_id = g.id AND e.created_at > ?) AS evidence_30d
		FROM support_coverage_gaps g
		LEFT JOIN support_coverage_topics t ON t.id = g.topic_id
		LEFT JOIN support_gap_evidence latest_evidence ON latest_evidence.id = (
			SELECT e2.id FROM support_gap_evidence e2 WHERE e2.gap_id = g.id ORDER BY e2.created_at DESC LIMIT 1
		)
		WHERE g.workspace_id = ?
		  AND g.status = ?
		  AND g.embedding IS NOT NULL
		  AND g.embedding_provider = ?
		  AND g.embedding_model = ?
		  AND g.embedding_version = ?
		  AND g.embedding_dimensions = ?
	`
	args := []interface{}{evidenceCutoff, workspaceID, status, provider, modelName, version, dimensions}
	if closedSince != nil {
		baseSQL += " AND g.closed_at >= ?"
		args = append(args, *closedSince)
	}
	baseSQL += `
		ORDER BY g.embedding <=> CAST(? AS vector) ASC, g.last_seen_at DESC
		LIMIT ?
	`
	args = append(args, embedding, limit)
	var items []model.SupportCoverageGapListItem
	if err := r.db.WithContext(ctx).Raw(baseSQL, args...).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("find nearest coverage gaps by embedding: %w", err)
	}
	return items, nil
}

// UpsertTopicByIssueKey returns an existing topic or creates one.
func (r *SupportCoverageRepository) UpsertTopicByIssueKey(ctx context.Context, workspaceID, issueKey, title string) (*model.SupportCoverageTopic, error) {
	if workspaceID == "" || issueKey == "" {
		return nil, fmt.Errorf("workspace_id and issue_key are required")
	}

	var topic model.SupportCoverageTopic
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND issue_key = ?", workspaceID, issueKey).
		First(&topic).Error

	if err == nil {
		return &topic, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("lookup topic: %w", err)
	}

	topic = model.SupportCoverageTopic{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		IssueKey:    issueKey,
		Title:       title,
	}
	if err := r.db.WithContext(ctx).Create(&topic).Error; err != nil {
		// Race: another writer may have created it.
		var existing model.SupportCoverageTopic
		if findErr := r.db.WithContext(ctx).
			Where("workspace_id = ? AND issue_key = ?", workspaceID, issueKey).
			First(&existing).Error; findErr == nil {
			return &existing, nil
		}
		return nil, fmt.Errorf("create topic: %w", err)
	}
	return &topic, nil
}

// UpsertTopicByClusterKey returns an existing cluster topic or creates one.
func (r *SupportCoverageRepository) UpsertTopicByClusterKey(ctx context.Context, workspaceID, clusterKey, title string) (*model.SupportCoverageTopic, error) {
	if workspaceID == "" || clusterKey == "" {
		return nil, fmt.Errorf("workspace_id and cluster_key are required")
	}

	var topic model.SupportCoverageTopic
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND cluster_key = ?", workspaceID, clusterKey).
		First(&topic).Error
	if err == nil {
		return &topic, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("lookup cluster topic: %w", err)
	}

	topic = model.SupportCoverageTopic{
		ID:             uuid.New().String(),
		WorkspaceID:    workspaceID,
		IssueKey:       clusterKey,
		Title:          title,
		ClusterKey:     &clusterKey,
		CanonicalTitle: &title,
	}
	if err := r.db.WithContext(ctx).Create(&topic).Error; err != nil {
		var existing model.SupportCoverageTopic
		if findErr := r.db.WithContext(ctx).
			Where("workspace_id = ? AND cluster_key = ?", workspaceID, clusterKey).
			First(&existing).Error; findErr == nil {
			return &existing, nil
		}
		return nil, fmt.Errorf("create cluster topic: %w", err)
	}
	return &topic, nil
}

func (r *SupportCoverageRepository) GetTopic(ctx context.Context, topicID string) (*model.SupportCoverageTopic, error) {
	var topic model.SupportCoverageTopic
	err := r.db.WithContext(ctx).Where("id = ?", topicID).First(&topic).Error
	if err == nil {
		return &topic, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, fmt.Errorf("get topic: %w", err)
}

// UpsertGapByDedupeKey creates a new gap or increments evidence on
// an existing one. Returns the gap and whether it was newly created.
func (r *SupportCoverageRepository) UpsertGapByDedupeKey(ctx context.Context, gap *model.SupportCoverageGap) (*model.SupportCoverageGap, bool, error) {
	if gap.WorkspaceID == "" || gap.DedupeKey == "" {
		return nil, false, fmt.Errorf("workspace_id and dedupe_key are required")
	}

	var existing model.SupportCoverageGap
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dedupe_key = ? AND status != ?", gap.WorkspaceID, gap.DedupeKey, model.SupportCoverageGapStatusMerged).
		First(&existing).Error

	if err == nil {
		// Update existing gap: bump evidence count and last seen.
		updates := map[string]interface{}{
			"evidence_count": gorm.Expr("evidence_count + 1"),
			"last_seen_at":   gap.LastSeenAt,
			"updated_at":     time.Now(),
		}
		if err := r.db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
			return nil, false, fmt.Errorf("update gap: %w", err)
		}
		existing.EvidenceCount++
		existing.LastSeenAt = gap.LastSeenAt
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, fmt.Errorf("lookup gap: %w", err)
	}

	// Create new gap.
	if gap.ID == "" {
		gap.ID = uuid.New().String()
	}
	gap.EvidenceCount = 1
	if gap.Metadata == nil {
		gap.Metadata = []byte("{}")
	}
	if err := r.createGap(ctx, gap); err != nil {
		// Race: check if concurrent writer created it.
		var raceExisting model.SupportCoverageGap
		if findErr := r.db.WithContext(ctx).
			Where("workspace_id = ? AND dedupe_key = ? AND status != ?", gap.WorkspaceID, gap.DedupeKey, model.SupportCoverageGapStatusMerged).
			First(&raceExisting).Error; findErr == nil {
			return &raceExisting, false, nil
		}
		return nil, false, fmt.Errorf("create gap: %w", err)
	}
	return gap, true, nil
}

// UpsertOpenGapByTopic creates a new open gap for a topic or increments the
// existing open gap. Closed gaps are intentionally ignored so recurrence after
// Done/Rejected can create a fresh open lifecycle.
func (r *SupportCoverageRepository) UpsertOpenGapByTopic(ctx context.Context, gap *model.SupportCoverageGap) (*model.SupportCoverageGap, bool, error) {
	if gap.WorkspaceID == "" || gap.TopicID == nil || *gap.TopicID == "" {
		return nil, false, fmt.Errorf("workspace_id and topic_id are required")
	}

	var existing model.SupportCoverageGap
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND topic_id = ? AND status = ?", gap.WorkspaceID, *gap.TopicID, model.SupportCoverageGapStatusOpen).
		First(&existing).Error
	if err == nil {
		updates := map[string]interface{}{
			"evidence_count": gorm.Expr("evidence_count + 1"),
			"last_seen_at":   gap.LastSeenAt,
			"updated_at":     time.Now(),
		}
		if err := r.db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
			return nil, false, fmt.Errorf("update open topic gap: %w", err)
		}
		existing.EvidenceCount++
		existing.LastSeenAt = gap.LastSeenAt
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, fmt.Errorf("lookup open topic gap: %w", err)
	}

	if gap.ID == "" {
		gap.ID = uuid.New().String()
	}
	if gap.Status == "" {
		gap.Status = model.SupportCoverageGapStatusOpen
	}
	if gap.GapKind == "" {
		gap.GapKind = "content"
	}
	gap.EvidenceCount = 1
	if gap.Metadata == nil {
		gap.Metadata = []byte("{}")
	}
	if err := r.createGap(ctx, gap); err != nil {
		var raceExisting model.SupportCoverageGap
		if findErr := r.db.WithContext(ctx).
			Where("workspace_id = ? AND topic_id = ? AND status = ?", gap.WorkspaceID, *gap.TopicID, model.SupportCoverageGapStatusOpen).
			First(&raceExisting).Error; findErr == nil {
			return &raceExisting, false, nil
		}
		return nil, false, fmt.Errorf("create open topic gap: %w", err)
	}
	return gap, true, nil
}

// createGap inserts a gap row, omitting the pgvector embedding column when it
// is unset — Go's zero-value "" is not a valid vector literal, and most
// event-derived gaps are created before any embedding is computed (the
// enrichment pipeline backfills it later).
func (r *SupportCoverageRepository) createGap(ctx context.Context, gap *model.SupportCoverageGap) error {
	tx := r.db.WithContext(ctx)
	if strings.TrimSpace(gap.Embedding) == "" {
		tx = tx.Omit("Embedding")
	}
	return tx.Create(gap).Error
}

func (r *SupportCoverageRepository) UpsertOpenGapByDedupeKeyNoBump(ctx context.Context, gap *model.SupportCoverageGap) (*model.SupportCoverageGap, bool, error) {
	if gap.WorkspaceID == "" || gap.DedupeKey == "" {
		return nil, false, fmt.Errorf("workspace_id and dedupe_key are required")
	}

	var existing model.SupportCoverageGap
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dedupe_key = ? AND status = ?", gap.WorkspaceID, gap.DedupeKey, model.SupportCoverageGapStatusOpen).
		First(&existing).Error
	if err == nil {
		updates := map[string]interface{}{
			"last_seen_at": gap.LastSeenAt,
			"updated_at":   time.Now(),
		}
		if gap.LastSeenAt.IsZero() {
			delete(updates, "last_seen_at")
		}
		if len(updates) > 1 {
			if err := r.db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
				return nil, false, fmt.Errorf("touch open dedupe gap: %w", err)
			}
			if !gap.LastSeenAt.IsZero() {
				existing.LastSeenAt = gap.LastSeenAt
			}
		}
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, fmt.Errorf("lookup open dedupe gap: %w", err)
	}

	if gap.ID == "" {
		gap.ID = uuid.New().String()
	}
	if gap.Status == "" {
		gap.Status = model.SupportCoverageGapStatusOpen
	}
	if gap.GapKind == "" {
		gap.GapKind = "content"
	}
	gap.EvidenceCount = 0
	if gap.Metadata == nil {
		gap.Metadata = []byte("{}")
	}
	if err := r.createGap(ctx, gap); err != nil {
		var raceExisting model.SupportCoverageGap
		if findErr := r.db.WithContext(ctx).
			Where("workspace_id = ? AND dedupe_key = ? AND status = ?", gap.WorkspaceID, gap.DedupeKey, model.SupportCoverageGapStatusOpen).
			First(&raceExisting).Error; findErr == nil {
			return &raceExisting, false, nil
		}
		return nil, false, fmt.Errorf("create open dedupe gap: %w", err)
	}
	return gap, true, nil
}

func (r *SupportCoverageRepository) IncrementOpenGapEvidence(ctx context.Context, workspaceID, gapID string, lastSeenAt time.Time) (*model.SupportCoverageGap, error) {
	if workspaceID == "" || gapID == "" {
		return nil, fmt.Errorf("workspace_id and gap_id are required")
	}
	var gap model.SupportCoverageGap
	if err := r.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND status = ?", gapID, workspaceID, model.SupportCoverageGapStatusOpen).
		First(&gap).Error; err != nil {
		return nil, fmt.Errorf("lookup open gap: %w", err)
	}
	if err := r.db.WithContext(ctx).Model(&gap).Updates(map[string]interface{}{
		"evidence_count": gorm.Expr("evidence_count + 1"),
		"last_seen_at":   lastSeenAt,
		"updated_at":     time.Now(),
	}).Error; err != nil {
		return nil, fmt.Errorf("increment open gap evidence: %w", err)
	}
	gap.EvidenceCount++
	gap.LastSeenAt = lastSeenAt
	return &gap, nil
}

// CreateEvidence links a gap to evidence (conversation, search, feedback).
func (r *SupportCoverageRepository) CreateEvidence(ctx context.Context, evidence *model.SupportGapEvidence) error {
	if evidence.GapID == "" || evidence.WorkspaceID == "" {
		return fmt.Errorf("gap_id and workspace_id are required")
	}
	if evidence.ID == "" {
		evidence.ID = uuid.New().String()
	}
	if evidence.Metadata == nil {
		evidence.Metadata = []byte("{}")
	}
	if err := r.db.WithContext(ctx).Create(evidence).Error; err != nil {
		return fmt.Errorf("create gap evidence: %w", err)
	}
	return nil
}

func (r *SupportCoverageRepository) CreateEvidenceIfAbsent(ctx context.Context, evidence *model.SupportGapEvidence) (bool, error) {
	if evidence.GapID == "" || evidence.WorkspaceID == "" {
		return false, fmt.Errorf("gap_id and workspace_id are required")
	}
	if evidence.ID == "" {
		evidence.ID = uuid.New().String()
	}
	if evidence.Metadata == nil {
		evidence.Metadata = []byte("{}")
	}
	if evidence.SourceKey != "" && r.db.Dialector.Name() == "sqlite" {
		var count int64
		if err := r.db.WithContext(ctx).
			Model(&model.SupportGapEvidence{}).
			Where("workspace_id = ? AND source_key = ?", evidence.WorkspaceID, evidence.SourceKey).
			Count(&count).Error; err != nil {
			return false, fmt.Errorf("lookup gap evidence source key: %w", err)
		}
		if count > 0 {
			return false, nil
		}
	}
	tx := r.db.WithContext(ctx)
	if evidence.SourceKey != "" {
		tx = tx.Clauses(clause.OnConflict{DoNothing: true})
	}
	result := tx.Create(evidence)
	if result.Error != nil {
		return false, fmt.Errorf("create gap evidence if absent: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (r *SupportCoverageRepository) FindGapByEvidenceSourceKey(ctx context.Context, workspaceID, sourceKey string) (*model.SupportCoverageGap, error) {
	if workspaceID == "" || sourceKey == "" {
		return nil, nil
	}
	var gap model.SupportCoverageGap
	err := r.db.WithContext(ctx).
		Table("support_coverage_gaps AS g").
		Select("g.*").
		Joins("JOIN support_gap_evidence e ON e.gap_id = g.id AND e.workspace_id = g.workspace_id").
		Where("g.workspace_id = ? AND e.source_key = ?", workspaceID, sourceKey).
		Order("e.created_at DESC").
		Limit(1).
		Scan(&gap).Error
	if err != nil {
		return nil, fmt.Errorf("find gap by evidence source key: %w", err)
	}
	if gap.ID == "" {
		return nil, nil
	}
	return &gap, nil
}

func (r *SupportCoverageRepository) IncrementGapEvidenceAfterInsert(ctx context.Context, workspaceID, gapID string, lastSeenAt time.Time) error {
	if workspaceID == "" || gapID == "" {
		return fmt.Errorf("workspace_id and gap_id are required")
	}
	result := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ? AND workspace_id = ?", gapID, workspaceID).
		Updates(map[string]interface{}{
			"evidence_count": gorm.Expr("evidence_count + 1"),
			"last_seen_at":   lastSeenAt,
			"updated_at":     time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("increment gap evidence after insert: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("gap not found")
	}
	return nil
}

func (r *SupportCoverageRepository) UpdateGapEmbedding(ctx context.Context, gapID string, embedding string, provider string, modelName string, version string, dimensions int, textHash string, updatedAt time.Time) error {
	if gapID == "" {
		return fmt.Errorf("gap_id is required")
	}
	updates := map[string]interface{}{
		"embedding":            embedding,
		"embedding_provider":   provider,
		"embedding_model":      modelName,
		"embedding_version":    version,
		"embedding_dimensions": dimensions,
		"embedding_text_hash":  textHash,
		"embedding_updated_at": updatedAt,
		"updated_at":           time.Now(),
	}
	result := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ?", gapID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update gap embedding: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("gap not found")
	}
	return nil
}

func (r *SupportCoverageRepository) UpdateGapKnowledgeMatch(ctx context.Context, workspaceID, gapID string, failureMode string, score float64, documentID *string, title string, checkedAt time.Time) error {
	if workspaceID == "" || gapID == "" {
		return fmt.Errorf("workspace_id and gap_id are required")
	}
	updates := map[string]interface{}{
		"nearest_content_score":       score,
		"nearest_content_document_id": documentID,
		"nearest_content_title":       title,
		"nearest_content_checked_at":  checkedAt,
		"updated_at":                  time.Now(),
	}
	if failureMode != "" {
		updates["failure_mode"] = failureMode
	}
	result := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ? AND workspace_id = ?", gapID, workspaceID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update gap knowledge match: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("gap not found")
	}
	return nil
}

func (r *SupportCoverageRepository) MarkGapRecurrenceWatch(ctx context.Context, workspaceID, gapID string, occurredAt time.Time) error {
	if workspaceID == "" || gapID == "" {
		return fmt.Errorf("workspace_id and gap_id are required")
	}
	var gap model.SupportCoverageGap
	if err := r.db.WithContext(ctx).Where("id = ? AND workspace_id = ?", gapID, workspaceID).First(&gap).Error; err != nil {
		return fmt.Errorf("load gap recurrence metadata: %w", err)
	}
	metadata := map[string]any{}
	if len(gap.Metadata) > 0 {
		_ = json.Unmarshal(gap.Metadata, &metadata)
	}
	count := 0
	switch value := metadata["post_close_evidence_count"].(type) {
	case float64:
		count = int(value)
	case int:
		count = value
	}
	count++
	metadata["recurrence_watch"] = true
	metadata["post_close_evidence_count"] = count
	metadata["last_post_close_evidence_at"] = occurredAt.UTC().Format(time.RFC3339)
	updates := map[string]interface{}{
		"metadata":     mustMarshalRawMessage(metadata),
		"last_seen_at": occurredAt,
		"updated_at":   time.Now(),
	}
	if count >= 3 && gap.Status == model.SupportCoverageGapStatusDone {
		updates["status"] = model.SupportCoverageGapStatusOpen
		updates["closed_at"] = nil
		updates["closed_evidence_count"] = nil
		metadata["recurrence_reopened"] = true
		metadata["post_close_evidence_count"] = 0
		updates["metadata"] = mustMarshalRawMessage(metadata)
	}
	if err := r.db.WithContext(ctx).Model(&model.SupportCoverageGap{}).Where("id = ? AND workspace_id = ?", gapID, workspaceID).Updates(updates).Error; err != nil {
		return fmt.Errorf("mark gap recurrence watch: %w", err)
	}
	return nil
}

func (r *SupportCoverageRepository) MarkGapSplitReviewNeeded(ctx context.Context, workspaceID, gapID, reason string) error {
	if workspaceID == "" || gapID == "" {
		return fmt.Errorf("workspace_id and gap_id are required")
	}
	var gap model.SupportCoverageGap
	if err := r.db.WithContext(ctx).Where("id = ? AND workspace_id = ?", gapID, workspaceID).First(&gap).Error; err != nil {
		return fmt.Errorf("load gap split metadata: %w", err)
	}
	metadata := map[string]any{}
	if len(gap.Metadata) > 0 {
		_ = json.Unmarshal(gap.Metadata, &metadata)
	}
	metadata["split_review_needed"] = true
	metadata["split_review_reason"] = reason
	if err := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ? AND workspace_id = ?", gapID, workspaceID).
		Updates(map[string]interface{}{
			"metadata":   mustMarshalRawMessage(metadata),
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("mark gap split review needed: %w", err)
	}
	return nil
}

func (r *SupportCoverageRepository) CountEvidenceSince(ctx context.Context, gapID string, since time.Time) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.SupportGapEvidence{}).
		Where("gap_id = ? AND created_at > ?", gapID, since).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count recent gap evidence: %w", err)
	}
	return count, nil
}

func (r *SupportCoverageRepository) CountEvidence30d(ctx context.Context, gapID string) (int, error) {
	count, err := r.CountEvidenceSince(ctx, gapID, time.Now().AddDate(0, 0, -30))
	return int(count), err
}

// ListGaps returns gaps for a workspace. Reads from gap table only,
// never scans support_events.
func (r *SupportCoverageRepository) ListGaps(ctx context.Context, workspaceID string, filter model.SupportCoverageGapFilter) ([]model.SupportCoverageGapListItem, int64, error) {
	evidenceCutoff := time.Now().AddDate(0, 0, -30)
	metrics := supportCoverageMetricExpressions(r.db.Dialector.Name())
	evidence30dExpr := metrics.evidence30d
	evidenceAllExpr := metrics.evidenceAll
	distinctCustomers30dExpr := metrics.customers30d
	distinctCustomersAllExpr := metrics.customersAll
	actionableBonusExpr := supportCoverageActionableBonusExpr(r.db.Dialector.Name())
	kbBonusExpr := "CASE WHEN g.nearest_content_score <= 0 THEN 2 WHEN g.failure_mode IN ('no_retrieval', 'weak_retrieval') AND g.nearest_content_score > 0 THEN 2 ELSE 0 END"
	impactExpr := fmt.Sprintf("((%s) * 4 + (%s) * 12 + (%s) + COALESCE(g.confidence, 0) * 2 + %s + %s)", evidence30dExpr, distinctCustomers30dExpr, evidenceAllExpr, kbBonusExpr, actionableBonusExpr)
	q := r.db.WithContext(ctx).
		Table("support_coverage_gaps g").
		Select(fmt.Sprintf(`g.*,
			COALESCE(t.title, '') AS topic_title,
			COALESCE(t.canonical_title, t.title, '') AS canonical_title,
			(SELECT COUNT(*) FROM support_gap_suggestions s WHERE s.gap_id = g.id) AS suggestion_count,
			(SELECT ga.document_id FROM support_coverage_gap_articles ga WHERE ga.gap_id = g.id LIMIT 1) AS related_article_id,
			%s AS evidence_30d,
			%s AS distinct_customers_30d,
			%s AS distinct_customers_all,
			%s AS evidence_all,
			%s AS conversations_30d,
			%s AS conversations_all,
			%s AS evidence_records_30d,
			%s AS impact_score,
			%s AS computed_impact_score`, evidence30dExpr, distinctCustomers30dExpr, distinctCustomersAllExpr, evidenceAllExpr, metrics.conversations30d, metrics.conversationsAll, metrics.records30d, impactExpr, impactExpr), evidenceCutoff, evidenceCutoff, evidenceCutoff, evidenceCutoff, evidenceCutoff, evidenceCutoff, evidenceCutoff, evidenceCutoff).
		Joins("LEFT JOIN support_coverage_topics t ON t.id = g.topic_id").
		Where("g.workspace_id = ?", workspaceID).
		Where("g.status != ?", model.SupportCoverageGapStatusMerged)
	if filter.ReviewOnly {
		q = applyRawEventDetectionGaps(q, "g", false)
	} else if !filter.ShowRaw {
		q = applyHideRawEventDetectionGaps(q, "g")
	}
	if filter.ConversationID != "" {
		q = applyCoverageConversationFilter(q, "g", filter.ConversationID)
	}

	if filter.Status != "" {
		q = q.Where("g.status = ?", filter.Status)
	}
	if filter.GapKind != "" {
		if filter.GapKind == "action" {
			q = q.Where("g.gap_kind IN (?)", []string{"action", "policy"})
		} else {
			q = q.Where("g.gap_kind = ?", filter.GapKind)
		}
	}
	if filter.GapCategory != "" {
		q = q.Where("g.gap_category = ?", filter.GapCategory)
	}
	if filter.V1GapType != "" {
		q = q.Where("g.v1_gap_type = ?", filter.V1GapType)
	}
	if filter.IssueKey != "" {
		q = q.Where("g.issue_key = ?", filter.IssueKey)
	}
	if filter.Search != "" {
		q = q.Where("g.title LIKE ?", "%"+filter.Search+"%")
	}
	if filter.HasMergeSuggestions {
		q = q.Where(`EXISTS (
			SELECT 1
			FROM support_coverage_gap_merge_suggestions ms
			WHERE ms.workspace_id = g.workspace_id
			  AND ms.status = ?
			  AND (ms.source_gap_id = g.id OR ms.target_gap_id = g.id)
		)`, model.SupportCoverageMergeSuggestionStatusPending)
	}

	var total int64
	countQ := r.db.WithContext(ctx).
		Table("support_coverage_gaps").
		Where("workspace_id = ? AND status != ?", workspaceID, model.SupportCoverageGapStatusMerged)
	if filter.ReviewOnly {
		countQ = applyRawEventDetectionGaps(countQ, "support_coverage_gaps", false)
	} else if !filter.ShowRaw {
		countQ = applyHideRawEventDetectionGaps(countQ, "support_coverage_gaps")
	}
	if filter.ConversationID != "" {
		countQ = applyCoverageConversationFilter(countQ, "support_coverage_gaps", filter.ConversationID)
	}
	if filter.Status != "" {
		countQ = countQ.Where("status = ?", filter.Status)
	}
	if filter.GapKind != "" {
		if filter.GapKind == "action" {
			countQ = countQ.Where("gap_kind IN (?)", []string{"action", "policy"})
		} else {
			countQ = countQ.Where("gap_kind = ?", filter.GapKind)
		}
	}
	if filter.GapCategory != "" {
		countQ = countQ.Where("gap_category = ?", filter.GapCategory)
	}
	if filter.V1GapType != "" {
		countQ = countQ.Where("v1_gap_type = ?", filter.V1GapType)
	}
	if filter.IssueKey != "" {
		countQ = countQ.Where("issue_key = ?", filter.IssueKey)
	}
	if filter.Search != "" {
		countQ = countQ.Where("title LIKE ?", "%"+filter.Search+"%")
	}
	if filter.HasMergeSuggestions {
		countQ = countQ.Where(`EXISTS (
			SELECT 1
			FROM support_coverage_gap_merge_suggestions ms
			WHERE ms.workspace_id = support_coverage_gaps.workspace_id
			  AND ms.status = ?
			  AND (ms.source_gap_id = support_coverage_gaps.id OR ms.target_gap_id = support_coverage_gaps.id)
		)`, model.SupportCoverageMergeSuggestionStatusPending)
	}
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count gaps: %w", err)
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage <= 0 || perPage > 100 {
		perPage = 25
	}

	q = q.Order("computed_impact_score DESC").
		Order("evidence_30d DESC, g.last_seen_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage)

	var items []model.SupportCoverageGapListItem
	if err := q.Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list gaps: %w", err)
	}
	for idx := range items {
		items[idx].ImpactExplanation = supportCoverageImpactExplanation(items[idx])
		items[idx].SplitReviewNeeded, items[idx].RecurrenceReopened = supportCoverageGapReviewFlags(items[idx].Metadata)
	}
	return items, total, nil
}

func supportCoverageActionableBonusExpr(dialect string) string {
	activeSuggestionPredicate := "sg.is_active = 1"
	if dialect == "postgres" {
		activeSuggestionPredicate = "sg.is_active = TRUE"
	}
	return fmt.Sprintf("CASE WHEN EXISTS(SELECT 1 FROM support_coverage_recommendations r WHERE r.gap_id = g.id AND r.status IN ('open', 'draft')) OR EXISTS(SELECT 1 FROM support_gap_suggestions sg WHERE sg.gap_id = g.id AND %s) THEN 2 ELSE 0 END", activeSuggestionPredicate)
}

func supportCoverageImpactExplanation(item model.SupportCoverageGapListItem) string {
	signal := "no nearby content"
	if item.NearestContentScore > 0 {
		signal = "existing content nearby"
	}
	return supportCoverageMetricsExplanation(item.SupportCoverageImpactMetrics) + ", " + signal
}

func mustMarshalRawMessage(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func supportCoverageGapReviewFlags(raw json.RawMessage) (bool, bool) {
	if len(raw) == 0 {
		return false, false
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return false, false
	}
	return metadata["split_review_needed"] == true, metadata["recurrence_reopened"] == true
}

func applyHideRawEventDetectionGaps(q *gorm.DB, tableAlias string) *gorm.DB {
	return applyRawEventDetectionGaps(q, tableAlias, true)
}

func applyCoverageConversationFilter(q *gorm.DB, tableAlias, conversationID string) *gorm.DB {
	return q.Where(fmt.Sprintf(`EXISTS (SELECT 1 FROM support_gap_evidence ce
		WHERE ce.gap_id = %s.id AND ce.workspace_id = %s.workspace_id AND ce.conversation_id = ?)`, tableAlias, tableAlias), conversationID)
}

func applyRawEventDetectionGaps(q *gorm.DB, tableAlias string, hide bool) *gorm.DB {
	prefix := ""
	if hide {
		prefix = "NOT "
	}
	return q.Where(
		prefix+fmt.Sprintf(`(%s AND %s.v1_gap_type = ? AND %s.confidence < ?)`,
			metadataSourceEqualsCondition(q, tableAlias),
			tableAlias,
			tableAlias,
		),
		model.SupportCoverageGapSourceEventDetection,
		model.SupportCoverageV1GapNeedsReview,
		0.7,
	)
}

func metadataSourceEqualsCondition(q *gorm.DB, tableAlias string) string {
	switch q.Dialector.Name() {
	case "postgres":
		return fmt.Sprintf("COALESCE(%s.metadata ->> 'source' = ?, false)", tableAlias)
	case "sqlite":
		return fmt.Sprintf("COALESCE(json_extract(%s.metadata, '$.source') = ?, 0)", tableAlias)
	default:
		return fmt.Sprintf("COALESCE(CAST(%s.metadata AS TEXT) LIKE '%%\"source\":\"' || ? || '\"%%', false)", tableAlias)
	}
}

func (r *SupportCoverageRepository) ListWorkspacesWithOpenGaps(ctx context.Context) ([]string, error) {
	var workspaceIDs []string
	if err := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Distinct("workspace_id").
		Where("status = ? AND evidence_count > 0", model.SupportCoverageGapStatusOpen).
		Pluck("workspace_id", &workspaceIDs).Error; err != nil {
		return nil, fmt.Errorf("list workspaces with open gaps: %w", err)
	}
	return workspaceIDs, nil
}

func (r *SupportCoverageRepository) ListTopicsDueForEnrichment(ctx context.Context, workspaceID string, olderThan time.Duration, minEvidence int) ([]string, error) {
	cutoff := time.Now().Add(-olderThan)
	var topicIDs []string
	if err := r.db.WithContext(ctx).
		Table("support_coverage_gaps g").
		Joins("JOIN support_coverage_topics t ON t.id = g.topic_id").
		Where("g.workspace_id = ? AND g.status = ? AND g.topic_id IS NOT NULL", workspaceID, model.SupportCoverageGapStatusOpen).
		Where("g.evidence_count >= ?", minEvidence).
		Where("(t.last_enriched_at IS NULL OR t.last_enriched_at < ?)", cutoff).
		Where("(t.cooldown_until IS NULL OR t.cooldown_until < ?)", time.Now()).
		Distinct("g.topic_id").
		Pluck("g.topic_id", &topicIDs).Error; err != nil {
		return nil, fmt.Errorf("list topics due for enrichment: %w", err)
	}
	return topicIDs, nil
}

// GetGapDetail returns a gap with its evidence, suggestions, and related articles.
func (r *SupportCoverageRepository) GetGapDetail(ctx context.Context, workspaceID, gapID string) (*model.SupportCoverageGapDetail, error) {
	var gap model.SupportCoverageGap
	if err := r.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", gapID, workspaceID).
		First(&gap).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get gap: %w", err)
	}

	metrics, err := r.getGapImpactMetrics(ctx, workspaceID, gapID)
	if err != nil {
		return nil, err
	}

	var topicTitle string
	if gap.TopicID != nil {
		var topic model.SupportCoverageTopic
		if err := r.db.WithContext(ctx).Where("id = ?", *gap.TopicID).First(&topic).Error; err == nil {
			topicTitle = topic.Title
		}
	}

	var evidence []model.SupportGapEvidenceView
	if err := r.db.WithContext(ctx).
		Table("support_gap_evidence AS e").
		Select("e.*, COALESCE(sm.sender_type, '') AS sender_role").
		Joins("LEFT JOIN support_messages sm ON sm.id = e.message_id AND sm.workspace_id = e.workspace_id").
		Where("e.gap_id = ? AND e.workspace_id = ?", gapID, workspaceID).
		Order("e.created_at DESC").
		Limit(50).
		Find(&evidence).Error; err != nil {
		return nil, fmt.Errorf("list gap evidence: %w", err)
	}

	var analysisExplanation *model.SupportCoverageAnalysisExplanation
	for _, ev := range evidence {
		if ev.SourceSignal != "daily_conversation_analysis" && ev.EvidenceType != "daily_conversation_analysis" {
			continue
		}
		var metadata struct {
			CustomerNeed    string `json:"customer_need"`
			AIFailure       string `json:"ai_failure"`
			HumanResolution string `json:"human_resolution"`
			DecisionReason  string `json:"decision_reason"`
		}
		if err := json.Unmarshal(ev.Metadata, &metadata); err != nil {
			continue
		}
		if metadata.CustomerNeed == "" && metadata.AIFailure == "" && metadata.HumanResolution == "" && metadata.DecisionReason == "" {
			continue
		}
		analysisExplanation = &model.SupportCoverageAnalysisExplanation{
			CustomerNeed:    metadata.CustomerNeed,
			AIFailure:       metadata.AIFailure,
			HumanResolution: metadata.HumanResolution,
			DecisionReason:  metadata.DecisionReason,
		}
		break
	}

	var suggestions []model.SupportGapSuggestion
	if err := r.db.WithContext(ctx).
		Where("gap_id = ? AND workspace_id = ?", gapID, workspaceID).
		Order("created_at DESC").
		Find(&suggestions).Error; err != nil {
		return nil, fmt.Errorf("list gap suggestions: %w", err)
	}

	var recommendations []model.SupportCoverageRecommendation
	if err := r.db.WithContext(ctx).
		Where("gap_id = ? AND workspace_id = ?", gapID, workspaceID).
		Order("CASE WHEN priority = 'primary' THEN 0 ELSE 1 END, created_at DESC").
		Find(&recommendations).Error; err != nil {
		return nil, fmt.Errorf("list gap recommendations: %w", err)
	}

	var relatedArticles []model.SupportCoverageGapArticle
	if err := r.db.WithContext(ctx).
		Where("gap_id = ? AND workspace_id = ?", gapID, workspaceID).
		Find(&relatedArticles).Error; err != nil {
		return nil, fmt.Errorf("list gap articles: %w", err)
	}

	// Resolve article titles from documents table.
	for i := range relatedArticles {
		var doc struct{ Title string }
		if err := r.db.WithContext(ctx).Table("docs_documents").Select("title").Where("id = ?", relatedArticles[i].DocumentID).First(&doc).Error; err == nil {
			relatedArticles[i].ArticleTitle = doc.Title
		}
	}

	var statusChangedByName string
	if gap.StatusChangedBy != nil && *gap.StatusChangedBy != "" {
		var user struct{ Name string }
		if err := r.db.WithContext(ctx).Table("users").Select("name").Where("id = ?", *gap.StatusChangedBy).First(&user).Error; err == nil {
			statusChangedByName = user.Name
		}
	}

	metrics.ImpactExplanation = supportCoverageImpactExplanation(model.SupportCoverageGapListItem{SupportCoverageGap: gap, SupportCoverageImpactMetrics: metrics})
	splitReviewNeeded, recurrenceReopened := supportCoverageGapReviewFlags(gap.Metadata)
	return &model.SupportCoverageGapDetail{
		SupportCoverageImpactMetrics: metrics,
		SupportCoverageGap:           gap,
		TopicTitle:                   topicTitle,
		StatusChangedByName:          statusChangedByName,
		AnalysisExplanation:          analysisExplanation,
		Recommendations:              recommendations,
		Evidence:                     evidence,
		Suggestions:                  suggestions,
		RelatedArticles:              relatedArticles,
		SplitReviewNeeded:            splitReviewNeeded,
		RecurrenceReopened:           recurrenceReopened,
	}, nil
}

// CreateSuggestion creates a proposed fix for a gap.
func (r *SupportCoverageRepository) CreateSuggestion(ctx context.Context, suggestion *model.SupportGapSuggestion) (*model.SupportGapSuggestion, error) {
	if suggestion.GapID == "" || suggestion.WorkspaceID == "" {
		return nil, fmt.Errorf("gap_id and workspace_id are required")
	}
	if suggestion.ID == "" {
		suggestion.ID = uuid.New().String()
	}
	if suggestion.Metadata == nil {
		suggestion.Metadata = []byte("{}")
	}
	if err := r.db.WithContext(ctx).Create(suggestion).Error; err != nil {
		return nil, fmt.Errorf("create suggestion: %w", err)
	}
	return suggestion, nil
}

func (r *SupportCoverageRepository) SupersedeActiveSuggestions(ctx context.Context, gapID string, now time.Time) error {
	if gapID == "" {
		return fmt.Errorf("gap_id is required")
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportGapSuggestion{}).
		Where("gap_id = ? AND is_active", gapID).
		Updates(map[string]interface{}{
			"is_active":     false,
			"superseded_at": now,
			"updated_at":    now,
		}).Error; err != nil {
		return fmt.Errorf("supersede active suggestions: %w", err)
	}
	return nil
}

// GetSuggestionByID loads a suggestion by ID into the provided pointer.
func (r *SupportCoverageRepository) GetSuggestionByID(ctx context.Context, suggestionID string, out *model.SupportGapSuggestion) error {
	if err := r.db.WithContext(ctx).Where("id = ?", suggestionID).First(out).Error; err != nil {
		return fmt.Errorf("get suggestion: %w", err)
	}
	return nil
}

// UpdateSuggestionResult records the outcome of applying a suggestion.
func (r *SupportCoverageRepository) UpdateSuggestionResult(ctx context.Context, suggestionID string, documentID *string, articleID *string, status string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if documentID != nil {
		updates["result_document_id"] = *documentID
	}
	if articleID != nil {
		updates["result_article_id"] = *articleID
	}
	if status == model.SupportCoverageSuggestionStatusApplied {
		now := time.Now()
		updates["applied_at"] = &now
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportGapSuggestion{}).
		Where("id = ?", suggestionID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update suggestion result: %w", err)
	}
	return nil
}

// DiscardSuggestion sets a suggestion to rejected and reverts its gap to open.
func (r *SupportCoverageRepository) DiscardSuggestion(ctx context.Context, suggestionID, workspaceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var suggestion model.SupportGapSuggestion
		if err := tx.Where("id = ? AND workspace_id = ?", suggestionID, workspaceID).First(&suggestion).Error; err != nil {
			return fmt.Errorf("suggestion not found: %w", err)
		}
		if err := tx.Model(&suggestion).Updates(map[string]interface{}{
			"status":     model.SupportCoverageSuggestionStatusRejected,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("reject suggestion: %w", err)
		}
		if err := tx.Model(&model.SupportCoverageGap{}).
			Where("id = ? AND workspace_id = ?", suggestion.GapID, workspaceID).
			Updates(map[string]interface{}{
				"status":     model.SupportCoverageGapStatusOpen,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("revert gap status: %w", err)
		}
		return nil
	})
}

// GetConversationCoverageState checks if docs-issue feedback was
// already submitted for a conversation.
func (r *SupportCoverageRepository) GetConversationCoverageState(ctx context.Context, workspaceID, conversationID string) (*model.SupportConversationCoverageState, error) {
	state := &model.SupportConversationCoverageState{}

	var event model.SupportEvent
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ? AND event_type = ?",
			workspaceID, conversationID, model.SupportEventDocsIssueFeedback).
		Order("occurred_at DESC").
		First(&event).Error

	if err == nil {
		state.DocsIssueFeedbackSubmitted = true
		// Check metadata for docs_issue value.
		if event.SourceSignal == model.SupportCoverageSourceAgentFeedback {
			val := true
			state.DocsIssueValue = &val
		}
	}
	if r.db.Migrator().HasTable("coverage_findings") && r.db.Migrator().HasTable("coverage_topic_memberships") {
		var current struct {
			TopicID *string
		}
		if err := r.db.WithContext(ctx).Table("coverage_findings f").
			Select("m.topic_id").
			Joins("JOIN coverage_topic_memberships m ON m.workspace_id = f.workspace_id AND m.finding_id = f.id AND m.valid_to IS NULL").
			Where("f.workspace_id = ? AND f.conversation_id = ? AND f.is_current = ?", workspaceID, conversationID, true).
			Order("f.created_at DESC").Limit(1).Scan(&current).Error; err != nil {
			return nil, fmt.Errorf("get canonical conversation coverage topic: %w", err)
		}
		state.CanonicalTopicID = current.TopicID
	}

	return state, nil
}

// CreateDigestDelivery records that a digest was sent to prevent duplicates.
func (r *SupportCoverageRepository) CreateDigestDelivery(ctx context.Context, delivery *model.SupportCoverageDigestDelivery) error {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(delivery).Error; err != nil {
		return fmt.Errorf("create digest delivery: %w", err)
	}
	return nil
}

// GetDigestDelivery checks if a digest was already sent for a given week.
func (r *SupportCoverageRepository) GetDigestDelivery(ctx context.Context, workspaceID string, weekStart time.Time, recipientUserID string) (*model.SupportCoverageDigestDelivery, error) {
	var delivery model.SupportCoverageDigestDelivery
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND week_start = ? AND recipient_user_id = ?",
			workspaceID, weekStart, recipientUserID).
		First(&delivery).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get digest delivery: %w", err)
	}
	return &delivery, nil
}

// FindOpenGapByConversation returns the most recent open
// gap that has evidence linked to the given conversation. Used to
// attach human reply evidence to the original AI handoff gap.
func (r *SupportCoverageRepository) FindOpenGapByConversation(ctx context.Context, workspaceID, conversationID string) (*model.SupportCoverageGap, error) {
	var gap model.SupportCoverageGap
	err := r.db.WithContext(ctx).
		Table("support_coverage_gaps g").
		Joins("JOIN support_gap_evidence e ON e.gap_id = g.id").
		Where("g.workspace_id = ? AND e.conversation_id = ? AND g.status = ?",
			workspaceID, conversationID, model.SupportCoverageGapStatusOpen).
		Order("g.last_seen_at DESC").
		Limit(1).
		Select("g.*").
		First(&gap).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find open gap by conversation: %w", err)
	}
	return &gap, nil
}

// UpdateGapStatus sets the status of a gap.
func (r *SupportCoverageRepository) UpdateGapStatus(ctx context.Context, workspaceID, gapID, status, userID string, issueResolved *bool) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":            status,
		"status_changed_by": userID,
		"status_changed_at": now,
		"updated_at":        now,
	}
	if status == model.SupportCoverageGapStatusDone || status == model.SupportCoverageGapStatusRejected {
		updates["closed_at"] = now
		updates["closed_evidence_count"] = gorm.Expr("evidence_count")
	} else if status == model.SupportCoverageGapStatusOpen {
		updates["closed_at"] = nil
		updates["closed_evidence_count"] = nil
		updates["rejection_reason"] = nil
	}
	if issueResolved != nil {
		updates["issue_resolved"] = *issueResolved
	}
	result := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ? AND workspace_id = ?", gapID, workspaceID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update gap status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("gap not found")
	}
	return nil
}

func (r *SupportCoverageRepository) MarkGapDone(ctx context.Context, workspaceID, gapID, documentID string, evidence30d int) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ? AND workspace_id = ? AND status = ?", gapID, workspaceID, model.SupportCoverageGapStatusOpen).
		Updates(map[string]interface{}{
			"status":                model.SupportCoverageGapStatusDone,
			"closed_at":             now,
			"closed_evidence_count": evidence30d,
			"result_document_id":    documentID,
			"status_changed_at":     now,
			"updated_at":            now,
		})
	if result.Error != nil {
		return fmt.Errorf("mark gap done: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrGapAlreadyClosed
	}
	return nil
}

func (r *SupportCoverageRepository) MarkGapRejected(ctx context.Context, workspaceID, gapID, userID string, rejectionReason *string, evidence30d int) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":                model.SupportCoverageGapStatusRejected,
		"closed_at":             now,
		"closed_evidence_count": evidence30d,
		"status_changed_by":     userID,
		"status_changed_at":     now,
		"updated_at":            now,
	}
	if rejectionReason != nil {
		updates["rejection_reason"] = *rejectionReason
	}
	result := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ? AND workspace_id = ? AND status = ?", gapID, workspaceID, model.SupportCoverageGapStatusOpen).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("mark gap rejected: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrGapAlreadyClosed
	}
	return nil
}

// ReclassifyGap changes the v1 gap type.
func (r *SupportCoverageRepository) ReclassifyGap(ctx context.Context, workspaceID, gapID, v1GapType string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ? AND workspace_id = ?", gapID, workspaceID).
		Updates(map[string]interface{}{
			"v1_gap_type": v1GapType,
			"updated_at":  time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("reclassify gap: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("gap not found")
	}
	return nil
}

// MergeGaps marks the source gap as merged and moves its evidence,
// suggestions, and article links to the target gap.
func (r *SupportCoverageRepository) MergeGaps(ctx context.Context, workspaceID, sourceGapID, targetGapID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Verify both gaps exist in the workspace.
		var source, target model.SupportCoverageGap
		if err := tx.Where("id = ? AND workspace_id = ?", sourceGapID, workspaceID).First(&source).Error; err != nil {
			return fmt.Errorf("source gap not found: %w", err)
		}
		if err := tx.Where("id = ? AND workspace_id = ?", targetGapID, workspaceID).First(&target).Error; err != nil {
			return fmt.Errorf("target gap not found: %w", err)
		}

		// Move evidence.
		if err := tx.Model(&model.SupportGapEvidence{}).
			Where("gap_id = ?", sourceGapID).
			Update("gap_id", targetGapID).Error; err != nil {
			return fmt.Errorf("move evidence: %w", err)
		}

		// Move suggestions.
		if err := tx.Model(&model.SupportGapSuggestion{}).
			Where("gap_id = ?", sourceGapID).
			Update("gap_id", targetGapID).Error; err != nil {
			return fmt.Errorf("move suggestions: %w", err)
		}

		// Move article links (ignore conflicts on unique index).
		tx.Exec(
			"UPDATE support_coverage_gap_articles SET gap_id = ? WHERE gap_id = ? AND document_id NOT IN (SELECT document_id FROM support_coverage_gap_articles WHERE gap_id = ?)",
			targetGapID, sourceGapID, targetGapID,
		)
		tx.Where("gap_id = ?", sourceGapID).Delete(&model.SupportCoverageGapArticle{})

		// Mark source as merged.
		if err := tx.Model(&source).Updates(map[string]interface{}{
			"status":     model.SupportCoverageGapStatusMerged,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("mark source merged: %w", err)
		}

		// Update target evidence count.
		if err := tx.Model(&target).Updates(map[string]interface{}{
			"evidence_count": gorm.Expr("evidence_count + ?", source.EvidenceCount),
			"updated_at":     time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("update target evidence count: %w", err)
		}

		return nil
	})
}

// LinkGapArticle creates a gap-article relationship. Ignores if already linked.
func (r *SupportCoverageRepository) LinkGapArticle(ctx context.Context, gapID, documentID, workspaceID string) error {
	link := model.SupportCoverageGapArticle{
		GapID:       gapID,
		DocumentID:  documentID,
		WorkspaceID: workspaceID,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&link).Error; err != nil {
		return fmt.Errorf("link gap article: %w", err)
	}
	return nil
}

// GetSummary returns aggregate coverage metrics for a workspace.
func (r *SupportCoverageRepository) GetSummary(ctx context.Context, workspaceID string) (*model.SupportCoverageSummary, error) {
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	summary := &model.SupportCoverageSummary{}

	visible := func() *gorm.DB {
		return applyHideRawEventDetectionGaps(r.db.WithContext(ctx).
			Table("support_coverage_gaps g").
			Where("g.workspace_id = ? AND g.status != ?", workspaceID, model.SupportCoverageGapStatusMerged), "g")
	}
	var newGaps, openGaps, fixedGaps, reviewGaps, totalEvidence int64
	if err := visible().Where("g.created_at >= ?", weekAgo).Count(&newGaps).Error; err != nil {
		return nil, fmt.Errorf("count new coverage gaps: %w", err)
	}
	if err := visible().Where("g.status = ?", model.SupportCoverageGapStatusOpen).Count(&openGaps).Error; err != nil {
		return nil, fmt.Errorf("count open coverage gaps: %w", err)
	}
	if err := visible().Where("g.status = ? AND g.updated_at >= ?", model.SupportCoverageGapStatusDone, weekAgo).Count(&fixedGaps).Error; err != nil {
		return nil, fmt.Errorf("count fixed coverage gaps: %w", err)
	}
	if err := applyRawEventDetectionGaps(r.db.WithContext(ctx).Table("support_coverage_gaps g").
		Where("g.workspace_id = ? AND g.status = ?", workspaceID, model.SupportCoverageGapStatusOpen), "g", false).Count(&reviewGaps).Error; err != nil {
		return nil, fmt.Errorf("count unreviewed coverage detections: %w", err)
	}
	if err := visible().Joins("JOIN support_gap_evidence e ON e.gap_id = g.id AND e.workspace_id = g.workspace_id").
		Count(&totalEvidence).Error; err != nil {
		return nil, fmt.Errorf("count visible coverage evidence: %w", err)
	}
	summary.NewGapsThisWeek = int(newGaps)
	summary.TotalOpenGaps = int(openGaps)
	summary.GapsFixedThisWeek = int(fixedGaps)
	summary.UnreviewedDetectionCount = int(reviewGaps)
	summary.TotalEvidenceCount = int(totalEvidence)

	var lastRun model.SupportCoverageAnalysisRun
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND status = ?", workspaceID, model.SupportCoverageAnalysisRunStatusCompleted).
		Order("completed_at DESC").
		First(&lastRun).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find last coverage analysis: %w", err)
	}
	if err == nil {
		summary.LastAnalyzedAt = lastRun.CompletedAt
	}

	return summary, nil
}

// HasCompletedAnalysisRun returns true if the workspace has at least one
// completed daily coverage analysis run, meaning the LLM analyzer is active
// and v1 heuristic gap creation can be suppressed for human-resolution signals.
func (r *SupportCoverageRepository) HasCompletedAnalysisRun(ctx context.Context, workspaceID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("support_coverage_analysis_runs").
		Where("workspace_id = ? AND status = ?", workspaceID, model.SupportCoverageAnalysisRunStatusCompleted).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check completed analysis run: %w", err)
	}
	return count > 0, nil
}

func (r *SupportCoverageRepository) CreateClusterRebuildRun(ctx context.Context, run *model.SupportCoverageClusterRebuildRun) error {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	if run.Status == "" {
		run.Status = model.SupportCoverageClusterRebuildStatusRunning
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if len(run.Metadata) == 0 {
		run.Metadata = []byte("{}")
	}
	if err := r.db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("create cluster rebuild run: %w", err)
	}
	return nil
}

func (r *SupportCoverageRepository) CompleteClusterRebuildRun(ctx context.Context, runID string, gapsScanned, clustersFound, autoMerged, suggestionsCreated, skipped int, metadata json.RawMessage) error {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":              model.SupportCoverageClusterRebuildStatusCompleted,
		"gaps_scanned":        gapsScanned,
		"clusters_found":      clustersFound,
		"auto_merged":         autoMerged,
		"suggestions_created": suggestionsCreated,
		"skipped":             skipped,
		"completed_at":        &now,
	}
	if len(metadata) > 0 {
		updates["metadata"] = metadata
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportCoverageClusterRebuildRun{}).
		Where("id = ?", runID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("complete cluster rebuild run: %w", err)
	}
	return nil
}

func (r *SupportCoverageRepository) FailClusterRebuildRun(ctx context.Context, runID string, cause error) error {
	return r.FailClusterRebuildRunWithMetadata(ctx, runID, cause, nil)
}

func (r *SupportCoverageRepository) FailClusterRebuildRunWithMetadata(ctx context.Context, runID string, cause error, metadata json.RawMessage) error {
	now := time.Now().UTC()
	msg := ""
	if cause != nil {
		msg = cause.Error()
	}
	updates := map[string]any{
		"status":        model.SupportCoverageClusterRebuildStatusFailed,
		"error_message": &msg,
		"completed_at":  &now,
	}
	if len(metadata) > 0 {
		updates["metadata"] = metadata
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportCoverageClusterRebuildRun{}).
		Where("id = ?", runID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("fail cluster rebuild run: %w", err)
	}
	return nil
}

func (r *SupportCoverageRepository) LatestClusterRebuildRun(ctx context.Context, workspaceID string) (*model.SupportCoverageClusterRebuildRun, error) {
	var run model.SupportCoverageClusterRebuildRun
	err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("started_at DESC").
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest cluster rebuild run: %w", err)
	}
	return &run, nil
}

func (r *SupportCoverageRepository) UpsertMergeSuggestion(ctx context.Context, suggestion *model.SupportCoverageGapMergeSuggestion) (bool, error) {
	if suggestion.ID == "" {
		suggestion.ID = uuid.New().String()
	}
	suggestion.PairKey = CoverageGapPairKey(suggestion.SourceGapID, suggestion.TargetGapID)
	if suggestion.PairKey == "" {
		return false, fmt.Errorf("merge suggestion pair key is required")
	}
	if suggestion.Status == "" {
		suggestion.Status = model.SupportCoverageMergeSuggestionStatusPending
	}
	if len(suggestion.Metadata) == 0 {
		suggestion.Metadata = []byte("{}")
	}
	var existing model.SupportCoverageGapMergeSuggestion
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND status = ? AND pair_key = ?",
			suggestion.WorkspaceID,
			model.SupportCoverageMergeSuggestionStatusPending,
			suggestion.PairKey,
		).
		First(&existing).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("lookup merge suggestion: %w", err)
	}
	if err := r.db.WithContext(ctx).Create(suggestion).Error; err != nil {
		return false, fmt.Errorf("create merge suggestion: %w", err)
	}
	return true, nil
}

func (r *SupportCoverageRepository) ListMergeSuggestionsForGap(ctx context.Context, workspaceID, gapID string) ([]model.SupportCoverageGapMergeSuggestion, error) {
	var suggestions []model.SupportCoverageGapMergeSuggestion
	if err := r.db.WithContext(ctx).
		Preload("SourceGap").
		Preload("TargetGap").
		Where("workspace_id = ? AND status = ? AND (source_gap_id = ? OR target_gap_id = ?)",
			workspaceID,
			model.SupportCoverageMergeSuggestionStatusPending,
			gapID,
			gapID,
		).
		Order("similarity_score DESC, created_at DESC").
		Find(&suggestions).Error; err != nil {
		return nil, fmt.Errorf("list merge suggestions for gap: %w", err)
	}
	return suggestions, nil
}

func (r *SupportCoverageRepository) GetMergeSuggestion(ctx context.Context, workspaceID, suggestionID string) (*model.SupportCoverageGapMergeSuggestion, error) {
	var suggestion model.SupportCoverageGapMergeSuggestion
	if err := r.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", suggestionID, workspaceID).
		First(&suggestion).Error; err != nil {
		return nil, fmt.Errorf("get merge suggestion: %w", err)
	}
	return &suggestion, nil
}

func (r *SupportCoverageRepository) MarkMergeSuggestionReviewed(ctx context.Context, workspaceID, suggestionID, status, userID string) error {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":      status,
		"reviewed_at": &now,
	}
	if userID != "" {
		updates["reviewed_by"] = userID
	}
	result := r.db.WithContext(ctx).
		Model(&model.SupportCoverageGapMergeSuggestion{}).
		Where("id = ? AND workspace_id = ?", suggestionID, workspaceID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("mark merge suggestion reviewed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("merge suggestion not found")
	}
	return nil
}

func (r *SupportCoverageRepository) KeepSeparateMergeSuggestion(ctx context.Context, workspaceID, suggestionID, userID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := r.WithTx(tx)
		suggestion, err := txRepo.GetMergeSuggestion(ctx, workspaceID, suggestionID)
		if err != nil {
			return err
		}
		if suggestion.Status != model.SupportCoverageMergeSuggestionStatusPending {
			return fmt.Errorf("merge suggestion is no longer pending")
		}
		leftHash, rightHash, err := txRepo.GetGapPairHashes(ctx, workspaceID, suggestion.SourceGapID, suggestion.TargetGapID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		var decidedBy *string
		if strings.TrimSpace(userID) != "" {
			decidedBy = &userID
		}
		if err := txRepo.UpsertGapPairDecision(ctx, &model.SupportCoverageGapPairDecision{
			WorkspaceID:          workspaceID,
			GapAID:               suggestion.SourceGapID,
			GapBID:               suggestion.TargetGapID,
			Decision:             "keep_separate",
			DecidedBy:            decidedBy,
			DecidedAt:            now,
			SimilarityAtDecision: suggestion.SimilarityScore,
			GapATextHash:         leftHash,
			GapBTextHash:         rightHash,
		}); err != nil {
			return err
		}
		return txRepo.MarkMergeSuggestionReviewed(ctx, workspaceID, suggestionID, model.SupportCoverageMergeSuggestionStatusDismissed, userID)
	})
}

func (r *SupportCoverageRepository) UpsertGapPairDecision(ctx context.Context, decision *model.SupportCoverageGapPairDecision) error {
	if decision.ID == "" {
		decision.ID = uuid.New().String()
	}
	decision.PairKey = CoverageGapPairKey(decision.GapAID, decision.GapBID)
	if decision.PairKey == "" {
		return fmt.Errorf("gap pair decision pair key is required")
	}
	if decision.Decision == "" {
		decision.Decision = "keep_separate"
	}
	if decision.DecidedAt.IsZero() {
		decision.DecidedAt = time.Now().UTC()
	}
	ids := []string{decision.GapAID, decision.GapBID}
	sort.Strings(ids)
	decision.GapAID = ids[0]
	decision.GapBID = ids[1]
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "workspace_id"}, {Name: "pair_key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"gap_a_id":               decision.GapAID,
				"gap_b_id":               decision.GapBID,
				"decision":               decision.Decision,
				"decided_by":             decision.DecidedBy,
				"decided_at":             decision.DecidedAt,
				"similarity_at_decision": decision.SimilarityAtDecision,
				"gap_a_text_hash":        decision.GapATextHash,
				"gap_b_text_hash":        decision.GapBTextHash,
				"updated_at":             time.Now().UTC(),
			}),
		}).
		Create(decision).Error; err != nil {
		return fmt.Errorf("upsert gap pair decision: %w", err)
	}
	return nil
}

func (r *SupportCoverageRepository) ListKeepSeparatePairDecisions(ctx context.Context, workspaceID string) ([]model.SupportCoverageGapPairDecision, error) {
	var decisions []model.SupportCoverageGapPairDecision
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND decision = ?", workspaceID, "keep_separate").
		Find(&decisions).Error; err != nil {
		return nil, fmt.Errorf("list keep separate pair decisions: %w", err)
	}
	return decisions, nil
}

func (r *SupportCoverageRepository) GetGapPairHashes(ctx context.Context, workspaceID, leftGapID, rightGapID string) (string, string, error) {
	ids := []string{leftGapID, rightGapID}
	sort.Strings(ids)
	var gaps []model.SupportCoverageGap
	if err := r.db.WithContext(ctx).
		Select("id", "embedding_text_hash").
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&gaps).Error; err != nil {
		return "", "", fmt.Errorf("get gap pair hashes: %w", err)
	}
	hashes := map[string]string{}
	for _, gap := range gaps {
		hashes[gap.ID] = gap.EmbeddingTextHash
	}
	if _, ok := hashes[ids[0]]; !ok {
		return "", "", fmt.Errorf("gap %s not found", ids[0])
	}
	if _, ok := hashes[ids[1]]; !ok {
		return "", "", fmt.Errorf("gap %s not found", ids[1])
	}
	return hashes[ids[0]], hashes[ids[1]], nil
}

// CreateSnapshot stores a pre-computed coverage snapshot.
func (r *SupportCoverageRepository) CreateSnapshot(ctx context.Context, snapshot *model.SupportCoverageSnapshot) error {
	if snapshot.Metrics == nil {
		snapshot.Metrics = []byte("{}")
	}
	if err := r.db.WithContext(ctx).Create(snapshot).Error; err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	return nil
}

// CleanOldSnapshots removes snapshots older than the retention period.
func (r *SupportCoverageRepository) CleanOldSnapshots(ctx context.Context, workspaceID string, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND snapshot_at < ?", workspaceID, cutoff).
		Delete(&model.SupportCoverageSnapshot{}).Error; err != nil {
		return fmt.Errorf("clean old snapshots: %w", err)
	}
	return nil
}
