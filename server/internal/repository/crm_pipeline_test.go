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
		`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),pipeline_id TEXT REFERENCES crm_pipelines(id),name TEXT,stage_type TEXT,position INTEGER,probability INTEGER,created_at DATETIME,updated_at DATETIME)`,
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
	if len(got) != 1 || !got[0].IsDefault || len(got[0].Stages) != 7 {
		t.Errorf("seeded pipelines = %+v", got)
	}
}
