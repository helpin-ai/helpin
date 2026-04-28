package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type coverageRebuildGap struct {
	ID            string
	WorkspaceID   string
	IssueKey      string
	Title         string
	EvidenceCount int
	FirstSeenAt   time.Time
	CreatedAt     time.Time
}

type coverageRebuildEvidence struct {
	EvidenceType string
	DocumentID   *string
	Excerpt      string
}

type coverageRebuildPlan struct {
	ClusterKey string
	TopicID    string
	Gaps       []coverageRebuildGap
}

func runClusterRebuild(ctx context.Context, sqlDB *sql.DB) error {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open gorm over migrate db: %w", err)
	}
	return runClusterRebuildGORM(ctx, db, time.Now())
}

func runClusterRebuildGORM(ctx context.Context, db *gorm.DB, now time.Time) error {
	var workspaces []string
	if err := db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Distinct("workspace_id").
		Where("status = ?", model.SupportCoverageGapStatusOpen).
		Pluck("workspace_id", &workspaces).Error; err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	repo := repository.NewSupportCoverageRepository(db)
	for _, workspaceID := range workspaces {
		plans, err := buildCoverageRebuildPlans(ctx, db, repo, workspaceID)
		if err != nil {
			return err
		}
		var topicsCreated, gapsMerged int
		for _, plan := range plans {
			if len(plan.Gaps) == 0 {
				continue
			}
			if len(plan.Gaps) == 1 {
				if err := assignCoverageGapTopic(ctx, db, plan.Gaps[0].ID, plan.TopicID, plan.ClusterKey); err != nil {
					return err
				}
				continue
			}
			topicsCreated++
			merged, err := mergeCoverageRebuildGroup(ctx, db, now, plan)
			if err != nil {
				return err
			}
			gapsMerged += merged
		}
		slog.InfoContext(ctx, "coverage cluster rebuild workspace complete",
			"workspace_id", workspaceID,
			"topics_created", topicsCreated,
			"gaps_merged", gapsMerged)
	}
	return nil
}

func buildCoverageRebuildPlans(ctx context.Context, db *gorm.DB, repo *repository.SupportCoverageRepository, workspaceID string) ([]coverageRebuildPlan, error) {
	var gaps []coverageRebuildGap
	if err := db.WithContext(ctx).
		Table("support_coverage_gaps").
		Select("id, workspace_id, issue_key, title, evidence_count, first_seen_at, created_at").
		Where("workspace_id = ? AND status = ?", workspaceID, model.SupportCoverageGapStatusOpen).
		Order("created_at ASC").
		Find(&gaps).Error; err != nil {
		return nil, fmt.Errorf("list open gaps for workspace %s: %w", workspaceID, err)
	}

	byCluster := map[string][]coverageRebuildGap{}
	for _, gap := range gaps {
		evidence, err := firstCoverageRebuildEvidence(ctx, db, gap.ID)
		if err != nil {
			return nil, err
		}
		summary := evidence.Excerpt
		if summary == "" {
			summary = gap.Title
		}
		documentID := ""
		if evidence.DocumentID != nil {
			documentID = *evidence.DocumentID
		}
		clusterKey := service.ComputeSupportCoverageClusterKey(gap.WorkspaceID, evidence.EvidenceType, documentID, gap.IssueKey, summary)
		byCluster[clusterKey] = append(byCluster[clusterKey], gap)
	}

	plans := make([]coverageRebuildPlan, 0, len(byCluster))
	for clusterKey, clusterGaps := range byCluster {
		title := clusterGaps[0].Title
		if title == "" {
			title = clusterKey
		}
		topic, err := repo.UpsertTopicByClusterKey(ctx, workspaceID, clusterKey, title)
		if err != nil {
			return nil, fmt.Errorf("upsert rebuild topic: %w", err)
		}
		plans = append(plans, coverageRebuildPlan{
			ClusterKey: clusterKey,
			TopicID:    topic.ID,
			Gaps:       clusterGaps,
		})
	}
	return plans, nil
}

func firstCoverageRebuildEvidence(ctx context.Context, db *gorm.DB, gapID string) (coverageRebuildEvidence, error) {
	var evidence coverageRebuildEvidence
	err := db.WithContext(ctx).
		Table("support_gap_evidence").
		Select("evidence_type, document_id, excerpt").
		Where("gap_id = ?", gapID).
		Order("created_at ASC").
		First(&evidence).Error
	if err == nil {
		return evidence, nil
	}
	if err == gorm.ErrRecordNotFound {
		return coverageRebuildEvidence{EvidenceType: "unknown"}, nil
	}
	return evidence, fmt.Errorf("load first evidence for gap %s: %w", gapID, err)
}

func assignCoverageGapTopic(ctx context.Context, db *gorm.DB, gapID, topicID, clusterKey string) error {
	if err := db.WithContext(ctx).
		Model(&model.SupportCoverageGap{}).
		Where("id = ?", gapID).
		Updates(map[string]interface{}{
			"topic_id":   topicID,
			"dedupe_key": clusterKey,
			"gap_kind":   "content",
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("assign gap topic: %w", err)
	}
	return nil
}

func mergeCoverageRebuildGroup(ctx context.Context, db *gorm.DB, now time.Time, plan coverageRebuildPlan) (int, error) {
	primary := chooseCoverageRebuildPrimary(plan.Gaps)
	if err := assignCoverageGapTopic(ctx, db, primary.ID, plan.TopicID, plan.ClusterKey); err != nil {
		return 0, err
	}

	merged := 0
	for _, gap := range plan.Gaps {
		if gap.ID == primary.ID {
			continue
		}
		if err := mergeCoverageRebuildDuplicate(ctx, db, now, primary.ID, gap, plan); err != nil {
			return merged, err
		}
		merged++
	}
	return merged, nil
}

func chooseCoverageRebuildPrimary(gaps []coverageRebuildGap) coverageRebuildGap {
	primary := gaps[0]
	for _, gap := range gaps[1:] {
		if gap.EvidenceCount > primary.EvidenceCount {
			primary = gap
			continue
		}
		if gap.EvidenceCount == primary.EvidenceCount && gap.CreatedAt.Before(primary.CreatedAt) {
			primary = gap
		}
	}
	return primary
}

func mergeCoverageRebuildDuplicate(ctx context.Context, db *gorm.DB, now time.Time, primaryID string, source coverageRebuildGap, plan coverageRebuildPlan) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SupportGapEvidence{}).Where("gap_id = ?", source.ID).Update("gap_id", primaryID).Error; err != nil {
			return fmt.Errorf("move evidence: %w", err)
		}
		if err := tx.Model(&model.SupportGapSuggestion{}).Where("gap_id = ?", source.ID).Update("gap_id", primaryID).Error; err != nil {
			return fmt.Errorf("move suggestions: %w", err)
		}
		if err := tx.Exec(
			"UPDATE support_coverage_gap_articles SET gap_id = ? WHERE gap_id = ? AND document_id NOT IN (SELECT document_id FROM support_coverage_gap_articles WHERE gap_id = ?)",
			primaryID, source.ID, primaryID,
		).Error; err != nil {
			return fmt.Errorf("move article links: %w", err)
		}
		if err := tx.Where("gap_id = ?", source.ID).Delete(&model.SupportCoverageGapArticle{}).Error; err != nil {
			return fmt.Errorf("delete duplicate article links: %w", err)
		}
		if err := tx.Model(&model.SupportCoverageGap{}).
			Where("id = ?", primaryID).
			Updates(map[string]interface{}{
				"evidence_count": gorm.Expr("evidence_count + ?", source.EvidenceCount),
				"updated_at":     now,
			}).Error; err != nil {
			return fmt.Errorf("update primary evidence count: %w", err)
		}
		reason := fmt.Sprintf("merged into %s during cluster rebuild 2026-04-27", primaryID)
		if err := tx.Model(&model.SupportCoverageGap{}).
			Where("id = ?", source.ID).
			Updates(map[string]interface{}{
				"status":                model.SupportCoverageGapStatusRejected,
				"topic_id":              plan.TopicID,
				"dedupe_key":            plan.ClusterKey,
				"gap_kind":              "content",
				"rejection_reason":      reason,
				"closed_at":             now,
				"closed_evidence_count": source.EvidenceCount,
				"status_changed_at":     now,
				"updated_at":            now,
			}).Error; err != nil {
			return fmt.Errorf("mark duplicate rejected: %w", err)
		}
		return nil
	})
}
