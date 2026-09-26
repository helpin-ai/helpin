//go:build integration

package repository

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"os"
	"sync"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestJevProductPostgresMigrationAndConcurrentAdmission(t *testing.T) {
	env := setupPMTriagePostgres(t)
	for _, name := range []string{"202609170003_jev_product_decisions.sql", "202609170004_flow_condition_activity.sql"} {
		if name == "202609170004_flow_condition_activity.sql" {
			if err := env.db.AutoMigrate(&model.AgentTriggerExecution{}); err != nil {
				t.Fatal(err)
			}
		}
		data, err := os.ReadFile("../dbmigrate/sql/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := env.db.Exec(string(data)).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	repo := NewJevDecisionRepository(env.db)
	for _, scenario := range []string{"same source", "independent sources"} {
		t.Run(scenario, func(t *testing.T) {
			feature := "meeting_routing"
			if scenario == "independent sources" {
				feature = "answer_evidence"
			}
			var wg sync.WaitGroup
			var mu sync.Mutex
			calls, limited := 0, 0
			for i := range 12 {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					source := "same"
					if scenario == "independent sources" {
						source = fmt.Sprint(i)
					}
					admission, err := repo.Reserve(context.Background(), model.JevDecisionAttempt{WorkspaceID: env.workspace, Feature: feature, SourceID: source, InputHash: "hash", Mode: "primary"}, 3)
					if err != nil {
						t.Error(err)
						return
					}
					mu.Lock()
					defer mu.Unlock()
					if admission.CallProvider {
						calls++
					}
					if admission.Limited {
						limited++
					}
				}(i)
			}
			wg.Wait()
			if scenario == "same source" && calls != 1 {
				t.Fatalf("dedup calls=%d", calls)
			}
			if scenario == "independent sources" && (calls != 3 || limited != 9) {
				t.Fatalf("cap calls=%d limited=%d", calls, limited)
			}
		})
	}
	outcome := "unavailable"
	execution := &model.AgentTriggerExecution{WorkspaceID: env.workspace, BindingID: "semantic_condition", BindingKind: "automation_rule", ReferenceID: &env.task, Status: "skipped", ConditionOutcome: &outcome, FiredAt: env.revision}
	if err := NewAgentTriggerExecutionRepository(env.db).CreateCondition(context.Background(), execution); err != nil {
		t.Fatal(err)
	}
	var saved model.AgentTriggerExecution
	if err := env.db.First(&saved, "id = ?", execution.ID).Error; err != nil {
		t.Fatal(err)
	}
	counts, err := NewAgentTriggerExecutionRepository(env.db).CountAutomationRuleExecutions(context.Background(), env.workspace, []string{env.task})
	if err != nil {
		t.Fatal(err)
	}
	if counts[env.task].Total != 0 {
		t.Fatal("condition counted as an executed action")
	}
	if saved.AgentID != "" || saved.ConditionOutcome == nil || *saved.ConditionOutcome != outcome {
		t.Fatalf("activity=%+v", saved)
	}
}

// Vector retrieval is outside this test; the scalar stand-in keeps the fixture
// focused on PostgreSQL transaction locks without requiring a vector extension.
type jevTestCoverageFinding struct {
	model.CoverageFinding
	Embedding string `gorm:"type:text"`
}

func (jevTestCoverageFinding) TableName() string { return "coverage_findings" }

func TestJevProductPostgresManualReviewWinsConcurrentAssessment(t *testing.T) {
	env := setupPMTriagePostgres(t)
	if err := env.db.AutoMigrate(&jevTestCoverageFinding{}, &model.CoverageTopicV2{}, &model.CoverageTopicMembership{}, &model.CoverageAssignmentAttempt{}, &model.CoverageUnreviewedSignal{}); err != nil {
		t.Fatal(err)
	}
	if err := env.db.Exec(`CREATE UNIQUE INDEX topics_key ON coverage_topics(workspace_id,canonical_key)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewCoverageV2Repository(env.db)
	ctx := context.Background()
	finding := coverageFindingFixture(uuid.NewString(), uuid.NewString())
	finding.WorkspaceID = env.workspace
	if err := repo.ReplaceCurrentFinding(ctx, finding); err != nil {
		t.Fatal(err)
	}
	if err := env.db.First(finding, "id = ?", finding.ID).Error; err != nil {
		t.Fatal(err)
	}
	topic, err := repo.CreateTopic(ctx, &model.CoverageTopicV2{WorkspaceID: env.workspace, CanonicalKey: "reset", Title: "Reset password", CustomerNeed: finding.CustomerNeed, AssignmentPolicy: "jev-topic-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.db.First(topic, "id = ?", topic.ID).Error; err != nil {
		t.Fatal(err)
	}
	manualTopic, err := repo.CreateTopic(ctx, &model.CoverageTopicV2{WorkspaceID: env.workspace, CanonicalKey: "manual", Title: "Manual topic", CustomerNeed: "Manual reviewed need", AssignmentPolicy: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	signalID := uuid.NewString()
	if err := env.db.Create(&model.CoverageUnreviewedSignal{ID: signalID, WorkspaceID: env.workspace, SourceKind: "conversation", SourceID: finding.SourceID, NormalizedQuery: finding.CustomerNeed, SignalKey: "review", Status: model.CoverageSignalUnreviewed, FindingID: &finding.ID, ObservedAt: env.revision}).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	start := make(chan struct{})
	go func() {
		defer wg.Done()
		<-start
		err := repo.ApplySemanticTopicAssignment(ctx, *finding, topic, &model.CoverageAssignmentAttempt{WorkspaceID: env.workspace, FindingID: finding.ID, Outcome: model.CoverageAssignmentAttach, PolicyVersion: "jev-topic-v1", Similarity: .99})
		if err != nil {
			t.Error(err)
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		err := repo.ReviewSignal(ctx, env.workspace, signalID, manualTopic.ID, env.actor)
		if err != nil {
			t.Error(err)
		}
	}()
	close(start)
	wg.Wait()
	var memberships []model.CoverageTopicMembership
	if err := env.db.Where("finding_id = ? AND valid_to IS NULL", finding.ID).Find(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 1 || memberships[0].TopicID != manualTopic.ID || memberships[0].DecisionSource != model.CoverageMembershipManual {
		t.Fatalf("manual choice lost: %+v", memberships)
	}
}
