package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSampleDataPMUsesExistingTeamWorkflow(t *testing.T) {
	env := newTaskTestEnv(t)
	workflowID := seedTaskTeamWorkflow(t, env, env.teamID)
	sample := &SampleDataEnv{Tx: env.db, WorkspaceID: env.wsID, ActorID: env.userID, ActorMemberID: "member-story-001", Now: time.Now(), Repo: repository.NewSampleDataRepository(env.db), TaskIDs: map[string]string{}}
	if err := (pmSampleSeeder{}).Seed(context.Background(), sample); err != nil {
		t.Fatalf("sample data cannot load: %v", err)
	}
	var tasks []model.PMTask
	if err := env.db.Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != len(sampleTasks) {
		t.Fatalf("created %d tasks, want %d", len(tasks), len(sampleTasks))
	}
	for _, task := range tasks {
		if task.WorkflowID != workflowID {
			t.Fatalf("sample task used workflow %s", task.WorkflowID)
		}
	}
}
