package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCoverageSemanticAssignmentPreservesDecisions(t *testing.T) {
	for _, scenario := range []string{"attach", "manual", "dismissed", "superseded", "edited topic", "closed topic", "wrong workspace", "review"} {
		t.Run(scenario, func(t *testing.T) {
			db := setupCoverageV2TestDB(t, "semantic_"+scenario)
			for _, statement := range []string{
				"CREATE UNIQUE INDEX coverage_topics_key ON coverage_topics(workspace_id, canonical_key)",
				"CREATE TABLE coverage_assignment_attempts(id TEXT PRIMARY KEY,workspace_id TEXT,finding_id TEXT,attempt INTEGER,policy_version TEXT,candidate_topic_id TEXT,similarity REAL,compatibility REAL,outcome TEXT,failure_class TEXT,failure_message TEXT,ai_execution_id TEXT,metadata TEXT,created_at DATETIME)",
			} {
				if err := db.Exec(statement).Error; err != nil {
					t.Fatal(err)
				}
			}
			repo := NewCoverageV2Repository(db)
			ctx := context.Background()
			finding := coverageFindingFixture("finding", "attempt")
			if err := repo.ReplaceCurrentFinding(ctx, finding); err != nil {
				t.Fatal(err)
			}
			if err := db.First(&finding, "id = ?", finding.ID).Error; err != nil {
				t.Fatal(err)
			}
			topic, err := repo.CreateTopic(ctx, &model.CoverageTopicV2{ID: "topic", WorkspaceID: "ws-1", CanonicalKey: "export", Title: "Export", CustomerNeed: finding.CustomerNeed, Status: model.CoverageTopicOpen, AssignmentPolicy: "jev-topic-v1"})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.First(topic, "id = ?", topic.ID).Error; err != nil {
				t.Fatal(err)
			}
			outcome := model.CoverageAssignmentAttach
			switch scenario {
			case "manual":
				if err := repo.SetCurrentMembership(ctx, &model.CoverageTopicMembership{WorkspaceID: "ws-1", FindingID: finding.ID, TopicID: "manual-topic", DecisionSource: model.CoverageMembershipManual, PolicyVersion: "manual", ValidFrom: time.Now().UTC()}); err != nil {
					t.Fatal(err)
				}
			case "dismissed":
				if err := repo.UpsertUnreviewedSignal(ctx, &model.CoverageUnreviewedSignal{ID: "signal", WorkspaceID: "ws-1", SourceKind: "conversation", SourceID: "conversation", NormalizedQuery: "export", SignalKey: "key", FindingID: &finding.ID, Status: model.CoverageSignalUnreviewed, ObservedAt: time.Now().UTC()}); err != nil {
					t.Fatal(err)
				}
				if err := repo.DismissSignal(ctx, "ws-1", "signal", "actor"); err != nil {
					t.Fatal(err)
				}
			case "superseded":
				if err := db.Model(&model.CoverageFinding{}).Where("id = ?", finding.ID).UpdateColumn("is_current", false).Error; err != nil {
					t.Fatal(err)
				}
			case "edited topic":
				if err := db.Model(&model.CoverageTopicV2{}).Where("id = ?", topic.ID).Update("customer_need", "different need").Error; err != nil {
					t.Fatal(err)
				}
			case "closed topic":
				if err := db.Model(&model.CoverageTopicV2{}).Where("id = ?", topic.ID).Update("status", "closed").Error; err != nil {
					t.Fatal(err)
				}
			case "wrong workspace":
				topic.WorkspaceID = "ws-2"
			case "review":
				outcome = model.CoverageAssignmentReview
			}
			attempt := &model.CoverageAssignmentAttempt{WorkspaceID: "ws-1", FindingID: finding.ID, Outcome: outcome, PolicyVersion: "jev-topic-v1", Similarity: .99, Metadata: []byte(`{}`)}
			err = repo.ApplySemanticTopicAssignment(ctx, *finding, topic, attempt)
			if scenario == "edited topic" || scenario == "closed topic" {
				if !errors.Is(err, ErrCoverageTopicChanged) {
					t.Fatalf("err=%v", err)
				}
			} else if scenario == "wrong workspace" {
				if err == nil {
					t.Fatal("cross-workspace assignment")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if scenario == "manual" || scenario == "dismissed" || scenario == "superseded" {
				if err := repo.SetCurrentMembership(ctx, &model.CoverageTopicMembership{WorkspaceID: "ws-1", FindingID: finding.ID, TopicID: topic.ID, DecisionSource: model.CoverageMembershipAutomatic, PolicyVersion: "v1"}); err != nil {
					t.Fatal(err)
				}
			}
			var memberships []model.CoverageTopicMembership
			if err := db.Where("valid_to IS NULL").Find(&memberships).Error; err != nil {
				t.Fatal(err)
			}
			want := 0
			if scenario == "attach" || scenario == "manual" {
				want = 1
			}
			if len(memberships) != want {
				t.Fatalf("memberships=%+v", memberships)
			}
			if scenario == "manual" && memberships[0].TopicID != "manual-topic" {
				t.Fatal("manual assignment overwritten")
			}
			var count int64
			if err := db.Model(&model.CoverageAssignmentAttempt{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			wantCount := int64(0)
			if scenario == "attach" || scenario == "review" {
				wantCount = 1
			}
			if count != wantCount {
				t.Fatalf("attempt count=%d", count)
			}
		})
	}
}
