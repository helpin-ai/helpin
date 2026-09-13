package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupCRMPipelineRepositoryTest(t *testing.T) (*CRMDealRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:pipeline_repo_%d?mode=memory&cache=shared&_foreign_keys=on", time.Now().UnixNano())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, sql := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY)`,
		`INSERT INTO workspaces VALUES ('ws')`,
		`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),workspace_id TEXT,name TEXT,is_default BOOLEAN,default_commercial_motion TEXT,position INTEGER,created_at DATETIME,updated_at DATETIME)`,
		`CREATE TABLE crm_pipeline_stages (color TEXT NOT NULL DEFAULT '#788596', id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),pipeline_id TEXT REFERENCES crm_pipelines(id),name TEXT,stage_type TEXT,position INTEGER,probability INTEGER,created_at DATETIME,updated_at DATETIME)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY,pipeline_id TEXT REFERENCES crm_pipelines(id),stage_id TEXT REFERENCES crm_pipeline_stages(id),updated_at DATETIME)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return NewCRMDealRepository(db), db
}

func TestReplaceStagesPreservesForeignKeys(t *testing.T) {
	repo, db := setupCRMPipelineRepositoryTest(t)
	pipeline := &model.CRMPipeline{ID: "p", WorkspaceID: "ws", Name: "Sales", Stages: []model.CRMPipelineStage{{ID: "a", Name: "Lead", StageType: "open"}, {ID: "b", Name: "Qualified", StageType: "open", Position: 1}}}
	if err := repo.CreatePipeline(context.Background(), pipeline); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO crm_deals (id,pipeline_id,stage_id) VALUES ('deal','p','a')").Error; err != nil {
		t.Fatal(err)
	}
	created := pipeline.Stages[0].CreatedAt
	pipeline.Stages[0].Position = 1
	pipeline.Stages[1].Position = 0
	if err := repo.ReplaceStages(context.Background(), pipeline.ID, pipeline.Stages); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetPipeline(context.Background(), pipeline.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stages[1].ID != "a" || !got.Stages[1].CreatedAt.Equal(created) || got.Stages[1].DealCount != 1 {
		t.Errorf("stage reference lost: %+v", got.Stages)
	}
}

func TestSeedDefaultPipelineIsIdempotent(t *testing.T) {
	repo, _ := setupCRMPipelineRepositoryTest(t)
	for i := 0; i < 2; i++ {
		if err := repo.SeedDefaultPipeline(context.Background(), "ws"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.ListPipelines(context.Background(), "ws")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].IsDefault || len(got[0].Stages) != 5 {
		t.Fatalf("seeded pipelines = %+v", got)
	}
	wantNames := []string{"Lead", "In Discussion", "Proposal Sent", "Won", "Lost"}
	wantTypes := []string{"open", "open", "open", "won", "lost"}
	wantProbabilities := []int{20, 50, 80, 100, 0}
	for i, stage := range got[0].Stages {
		if stage.Name != wantNames[i] || stage.StageType != wantTypes[i] || stage.Position != i || stage.Probability != wantProbabilities[i] {
			t.Errorf("stage %d = %+v", i, stage)
		}
	}
}

func TestSeedDefaultPipelinePreservesCustomPipeline(t *testing.T) {
	repo, _ := setupCRMPipelineRepositoryTest(t)
	pipeline := &model.CRMPipeline{ID: "custom", WorkspaceID: "ws", Name: "Custom", IsDefault: true, Stages: []model.CRMPipelineStage{{ID: "custom-stage", Name: "Consultation", StageType: "open", Probability: 35}}}
	if err := repo.CreatePipeline(context.Background(), pipeline); err != nil {
		t.Fatal(err)
	}
	if err := repo.SeedDefaultPipeline(context.Background(), "ws"); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListPipelines(context.Background(), "ws")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "custom" || len(got[0].Stages) != 1 || got[0].Stages[0].Name != "Consultation" {
		t.Fatalf("custom pipeline changed: %+v", got)
	}
}
