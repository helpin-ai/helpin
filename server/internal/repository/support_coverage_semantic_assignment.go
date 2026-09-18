package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrCoverageTopicChanged requests a fresh candidate snapshot before assignment.
var ErrCoverageTopicChanged = errors.New("coverage topic changed during assessment")

// ApplySemanticTopicAssignment rechecks source/target revisions and preserves
// manual membership decisions while atomically recording the semantic outcome.
func (r *CoverageV2Repository) ApplySemanticTopicAssignment(ctx context.Context, expected model.CoverageFinding, topic *model.CoverageTopicV2, attempt *model.CoverageAssignmentAttempt) error {
	if attempt == nil || attempt.WorkspaceID != expected.WorkspaceID || attempt.FindingID != expected.ID {
		return errors.New("invalid semantic assignment identity")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var finding model.CoverageFinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", expected.WorkspaceID, expected.ID).First(&finding).Error; err != nil {
			return err
		}
		if !finding.IsCurrent || finding.CustomerNeed != expected.CustomerNeed || finding.AIFailure != expected.AIFailure {
			return nil
		}
		var previous model.CoverageTopicMembership
		err := tx.Where("workspace_id = ? AND finding_id = ? AND valid_to IS NULL", finding.WorkspaceID, finding.ID).First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && previous.DecisionSource != model.CoverageMembershipAutomatic {
			return nil
		}
		var reviewed int64
		if err := tx.Model(&model.CoverageUnreviewedSignal{}).Where("workspace_id = ? AND finding_id = ? AND status <> ?", finding.WorkspaceID, finding.ID, model.CoverageSignalUnreviewed).Count(&reviewed).Error; err != nil {
			return err
		}
		if reviewed > 0 {
			return nil
		}
		scoped := NewCoverageV2Repository(tx)
		switch attempt.Outcome {
		case model.CoverageAssignmentAttach:
			if topic == nil || topic.WorkspaceID != finding.WorkspaceID {
				return errors.New("invalid semantic topic")
			}
			if err := lockCoverageTopics(tx, finding.WorkspaceID, previous.TopicID, topic.ID); err != nil {
				return err
			}
			var current model.CoverageTopicV2
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", finding.WorkspaceID, topic.ID).First(&current).Error; err != nil {
				return err
			}
			if current.Status != model.CoverageTopicOpen || current.CustomerNeed != topic.CustomerNeed || !current.UpdatedAt.Equal(topic.UpdatedAt) {
				return ErrCoverageTopicChanged
			}
		case model.CoverageAssignmentCreate:
			if topic == nil || topic.WorkspaceID != finding.WorkspaceID {
				return errors.New("invalid semantic topic")
			}
			expectedNeed := topic.CustomerNeed
			created, err := scoped.CreateTopic(ctx, topic)
			if err != nil {
				return err
			}
			topic = created
			if err := lockCoverageTopics(tx, finding.WorkspaceID, previous.TopicID, topic.ID); err != nil {
				return err
			}
			if err := tx.Where("workspace_id = ? AND id = ?", finding.WorkspaceID, topic.ID).First(topic).Error; err != nil {
				return err
			}
			if topic.Status != model.CoverageTopicOpen || topic.CustomerNeed != expectedNeed {
				return ErrCoverageTopicChanged
			}
		case model.CoverageAssignmentReview:
		default:
			return fmt.Errorf("invalid semantic assignment outcome")
		}
		if err := scoped.AppendAssignmentAttempt(ctx, attempt); err != nil {
			return err
		}
		if attempt.Outcome == model.CoverageAssignmentReview {
			if err := scoped.UpdateFindingAssignmentStatus(ctx, finding.WorkspaceID, finding.ID, "review"); err != nil {
				return err
			}
			return scoped.UpsertUnreviewedSignal(ctx, &model.CoverageUnreviewedSignal{WorkspaceID: finding.WorkspaceID, SourceKind: finding.SourceKind, SourceID: finding.SourceID, NormalizedQuery: finding.CustomerNeed, SignalKey: "jev-finding:" + finding.ID, Status: model.CoverageSignalUnreviewed, Confidence: finding.Confidence, FindingID: &finding.ID, ObservedAt: finding.CreatedAt})
		}
		if err := scoped.SetCurrentMembership(ctx, &model.CoverageTopicMembership{WorkspaceID: finding.WorkspaceID, FindingID: finding.ID, TopicID: topic.ID, DecisionSource: model.CoverageMembershipAutomatic, Confidence: math.Max(finding.Confidence, attempt.Similarity), PolicyVersion: attempt.PolicyVersion, AssignmentAttemptID: &attempt.ID, ValidFrom: time.Now().UTC(), Metadata: attempt.Metadata}); err != nil {
			return err
		}
		if err := scoped.UpdateFindingAssignmentStatus(ctx, finding.WorkspaceID, finding.ID, "assigned"); err != nil {
			return err
		}
		if previous.TopicID != "" && previous.TopicID != topic.ID {
			if err := scoped.RefreshTopicCounts(ctx, finding.WorkspaceID, previous.TopicID); err != nil {
				return err
			}
		}
		return scoped.RefreshTopicCounts(ctx, finding.WorkspaceID, topic.ID)
	})
}

func lockCoverageTopics(tx *gorm.DB, workspaceID, previousID, targetID string) error {
	ids := []string{targetID}
	if previousID != "" && previousID != targetID {
		ids = append(ids, previousID)
	}
	var topics []model.CoverageTopicV2
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id IN ?", workspaceID, ids).Order("id").Find(&topics).Error
}
