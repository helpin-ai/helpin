package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupCRMPipelineTest(t *testing.T) (*CRMDealService, *gorm.DB, *model.CRMPipeline) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:pipeline_%d?mode=memory&cache=shared&_foreign_keys=on", time.Now().UnixNano())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
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
		`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT NOT NULL, name TEXT NOT NULL, is_default BOOLEAN NOT NULL DEFAULT false, default_commercial_motion TEXT DEFAULT 'new_business', position INTEGER DEFAULT 0, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_pipeline_stages (color TEXT NOT NULL DEFAULT '#788596', id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), pipeline_id TEXT NOT NULL REFERENCES crm_pipelines(id), name TEXT NOT NULL, stage_type TEXT NOT NULL, position INTEGER, probability INTEGER, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, pipeline_id TEXT NOT NULL REFERENCES crm_pipelines(id), stage_id TEXT NOT NULL REFERENCES crm_pipeline_stages(id), probability INTEGER, updated_at DATETIME)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.NewCRMDealRepository(db)
	svc := NewCRMDealService(repo, nil)
	p := &model.CRMPipeline{ID: "p", WorkspaceID: "ws", Name: "Sales", IsDefault: true, Stages: []model.CRMPipelineStage{
		{ID: "a", Name: "Lead", StageType: "open", Position: 0, Probability: 10},
		{ID: "b", Name: "Qualified", StageType: "open", Position: 1, Probability: 60},
		{ID: "w", Name: "Won", StageType: "won", Position: 2, Probability: 100},
		{ID: "l", Name: "Lost", StageType: "lost", Position: 3, Probability: 0},
	}}
	if err := repo.CreatePipeline(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	p, err = repo.GetPipeline(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	return svc, db, p
}
func pipelineRequest(t *testing.T, p *model.CRMPipeline, extra map[string]interface{}) model.UpdateCRMPipelineRequest {
	t.Helper()
	payload := map[string]interface{}{"stages": p.Stages}
	for k, v := range extra {
		payload[k] = v
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var req model.UpdateCRMPipelineRequest
	if err := json.Unmarshal(b, &req); err != nil {
		t.Fatal(err)
	}
	return req
}
func addPipelineDeal(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("INSERT INTO crm_deals (id,pipeline_id,stage_id,probability,updated_at) VALUES ('deal','p','a',42,?)", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
}
func TestUpdatePipelineReorderPreservesDeals(t *testing.T) {
	svc, db, p := setupCRMPipelineTest(t)
	addPipelineDeal(t, db)
	created := p.Stages[0].CreatedAt
	p.Stages[0].Position = 1
	p.Stages[1].Position = 0
	got, err := svc.UpdatePipeline(context.Background(), p.ID, pipelineRequest(t, p, nil))
	if err != nil {
		t.Fatalf("reorder occupied stages: %v", err)
	}
	if got.Stages[0].ID != "b" || got.Stages[1].ID != "a" {
		t.Errorf("order = %+v", got.Stages)
	}
	if !got.Stages[1].CreatedAt.Equal(created) {
		t.Error("stage creation time changed")
	}
	var stage string
	if err := db.Raw("SELECT stage_id FROM crm_deals WHERE id = 'deal'").Scan(&stage).Error; err != nil {
		t.Fatal(err)
	}
	if stage != "a" {
		t.Errorf("deal stage = %q", stage)
	}
}
func TestUpdatePipelineMigratesDeals(t *testing.T) {
	svc, db, p := setupCRMPipelineTest(t)
	addPipelineDeal(t, db)
	p.Stages = p.Stages[1:]
	got, err := svc.UpdatePipeline(context.Background(), p.ID, pipelineRequest(t, p, map[string]interface{}{"stage_migrations": map[string]string{"a": "b"}}))
	if err != nil {
		t.Fatal(err)
	}
	var deal model.CRMDeal
	if err := db.First(&deal, "id = ?", "deal").Error; err != nil {
		t.Fatal(err)
	}
	if deal.StageID != "b" || deal.Probability == nil || *deal.Probability != 42 || time.Since(deal.UpdatedAt) > time.Minute {
		t.Errorf("migration = %+v", deal)
	}
	if len(got.Stages) != 3 {
		t.Errorf("stages = %d", len(got.Stages))
	}
}
func TestUpdatePipelineRejectsInvalidChangesAtomically(t *testing.T) {
	tests := []struct {
		name   string
		change func(*model.CRMPipeline, map[string]interface{})
		want   string
	}{
		{"missing migration", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages = p.Stages[1:] }, "destination"},
		{"foreign migration", func(p *model.CRMPipeline, e map[string]interface{}) {
			p.Stages = p.Stages[1:]
			e["stage_migrations"] = map[string]string{"a": "foreign"}
		}, "destination"},
		{"different type migration", func(p *model.CRMPipeline, e map[string]interface{}) {
			p.Stages = p.Stages[1:]
			e["stage_migrations"] = map[string]string{"a": "w"}
		}, "same type"},
		{"occupied type change", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages[0].StageType = "won" }, "move its deals"},
		{"foreign stage", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages[1].ID = "foreign" }, "belong"},
		{"duplicate stage", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages[1].ID = "a" }, "duplicate"},
		{"empty stage name", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages[1].Name = " " }, "name"},
		{"invalid type", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages[1].StageType = "oops" }, "type"},
		{"negative probability", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages[1].Probability = -1 }, "probability"},
		{"large probability", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages[1].Probability = 101 }, "probability"},
		{"no open stage", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages = p.Stages[2:] }, "open"},
		{"last won removed", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages = append(p.Stages[:2], p.Stages[3]) }, "won"},
		{"last lost removed", func(p *model.CRMPipeline, e map[string]interface{}) { p.Stages = p.Stages[:3] }, "lost"},
		{"stale version", func(p *model.CRMPipeline, e map[string]interface{}) {
			e["expected_updated_at"] = p.UpdatedAt.Add(-time.Minute)
		}, "changed"},
		{"unset default", func(p *model.CRMPipeline, e map[string]interface{}) { e["is_default"] = false }, "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db, p := setupCRMPipelineTest(t)
			addPipelineDeal(t, db)
			extra := map[string]interface{}{"name": "Changed"}
			tt.change(p, extra)
			_, err := svc.UpdatePipeline(context.Background(), p.ID, pipelineRequest(t, p, extra))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			got, err := svc.GetPipeline(context.Background(), p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != "Sales" || len(got.Stages) != 4 {
				t.Errorf("partial write: %+v", got)
			}
		})
	}
}
func TestPipelineStageDealCounts(t *testing.T) {
	svc, db, p := setupCRMPipelineTest(t)
	addPipelineDeal(t, db)
	got, err := svc.GetPipeline(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ListPipelines(context.Background(), "ws")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []*model.CRMPipeline{got, &rows[0]} {
		b, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			DealCount int64 `json:"deal_count"`
			Stages    []struct {
				ID        string `json:"id"`
				DealCount int64  `json:"deal_count"`
			} `json:"stages"`
		}
		if err := json.Unmarshal(b, &response); err != nil {
			t.Fatal(err)
		}
		if response.DealCount != 1 || response.Stages[0].DealCount != 1 || response.Stages[1].DealCount != 0 {
			t.Errorf("counts = %s", b)
		}
	}
}
func TestUpdatePipelineRenameLeavesStagesUntouched(t *testing.T) {
	svc, db, p := setupCRMPipelineTest(t)
	addPipelineDeal(t, db)
	name := "Renamed"
	got, err := svc.UpdatePipeline(context.Background(), p.ID, model.UpdateCRMPipelineRequest{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != name || !got.Stages[0].UpdatedAt.Equal(p.Stages[0].UpdatedAt) {
		t.Error("rename unexpectedly changed stages")
	}
}
func TestDeleteDefaultPipelineRequiresReplacement(t *testing.T) {
	svc, _, p := setupCRMPipelineTest(t)
	if err := svc.DeletePipeline(context.Background(), p.ID); err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("error = %v", err)
	}
}
func TestCreateDefaultPipelineReplacesPreviousDefault(t *testing.T) {
	svc, _, p := setupCRMPipelineTest(t)
	yes := true
	_, err := svc.CreatePipeline(context.Background(), model.CreateCRMPipelineRequest{WorkspaceID: "ws", Name: "Second", IsDefault: &yes, Stages: []model.CreateCRMPipelineStageItem{{Name: "Lead", StageType: "open"}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetPipeline(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsDefault {
		t.Error("previous pipeline still default")
	}
}

func TestUpdatePipelineRollsBackDatabaseFailure(t *testing.T) {
	svc, db, p := setupCRMPipelineTest(t)
	addPipelineDeal(t, db)
	if err := db.Exec(`CREATE TRIGGER reject_pipeline_update BEFORE UPDATE ON crm_pipelines BEGIN SELECT RAISE(ABORT, 'test metadata failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	p.Stages = p.Stages[1:]
	_, err := svc.UpdatePipeline(context.Background(), p.ID, pipelineRequest(t, p, map[string]interface{}{"stage_migrations": map[string]string{"a": "b"}}))
	if err == nil {
		t.Fatal("expected injected database failure")
	}
	got, err := svc.GetPipeline(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Stages) != 4 || got.Stages[0].DealCount != 1 || got.Stages[1].DealCount != 0 {
		t.Errorf("migration was not rolled back: %+v", got.Stages)
	}
}

func TestUpdatePipelineDefaultReplacesPreviousDefault(t *testing.T) {
	svc, db, p := setupCRMPipelineTest(t)
	second := &model.CRMPipeline{ID: "second", WorkspaceID: "ws", Name: "Second", Stages: []model.CRMPipelineStage{{ID: "second-open", Name: "Lead", StageType: "open"}}}
	if err := repository.NewCRMDealRepository(db).CreatePipeline(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	yes := true
	got, err := svc.UpdatePipeline(context.Background(), second.ID, model.UpdateCRMPipelineRequest{IsDefault: &yes})
	if err != nil {
		t.Fatal(err)
	}
	original, err := svc.GetPipeline(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if original.IsDefault || !got.IsDefault {
		t.Error("default selection did not move")
	}
	if err := svc.DeletePipeline(context.Background(), p.ID); err != nil {
		t.Fatalf("delete former default: %v", err)
	}
}

func TestUpdatePipelineNormalizesStageGroups(t *testing.T) {
	svc, _, p := setupCRMPipelineTest(t)
	p.Stages[0].Position = 50
	p.Stages[1].Position = 10
	p.Stages[2].Position = 0
	p.Stages[3].Position = 1
	p.Stages[2].Probability = 50
	p.Stages[3].Probability = 50
	got, err := svc.UpdatePipeline(context.Background(), p.ID, pipelineRequest(t, p, nil))
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"b", "a", "w", "l"} {
		if got.Stages[i].ID != id || got.Stages[i].Position != i {
			t.Errorf("stage %d = %+v", i, got.Stages[i])
		}
	}
	if got.Stages[2].Probability != 100 || got.Stages[3].Probability != 0 {
		t.Error("closed stage probabilities not normalized")
	}
}

func TestUpdatePipelineSameVersionCanOnlyBeSavedOnce(t *testing.T) {
	svc, _, p := setupCRMPipelineTest(t)
	req := pipelineRequest(t, p, map[string]interface{}{"expected_updated_at": p.UpdatedAt, "name": "First edit"})
	if _, err := svc.UpdatePipeline(context.Background(), p.ID, req); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdatePipeline(context.Background(), p.ID, req); err == nil {
		t.Fatal("stale second save succeeded")
	}
}

func TestExistingBusinessPipelineValueRoundTrips(t *testing.T) {
	svc, _, p := setupCRMPipelineTest(t)
	motion := "existing_business"
	got, err := svc.UpdatePipeline(context.Background(), p.ID, model.UpdateCRMPipelineRequest{DefaultCommercialMotion: &motion})
	if err != nil || got.DefaultCommercialMotion != motion {
		t.Fatalf("pipeline=%+v err=%v", got, err)
	}
	for _, value := range []string{"new_business", "existing_business", "expansion", "renewal"} {
		if !validCRMDealCommercialMotion(value) {
			t.Errorf("rejected supported deal type %s", value)
		}
	}
}

func TestPipelineStageColorsPersistAndSurviveLegacyEdits(t *testing.T) {
	svc, db, pipeline := setupCRMPipelineTest(t)
	payload := pipelineRequest(t, pipeline, nil)
	encoded, _ := json.Marshal(payload)
	var raw map[string]interface{}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	raw["stages"].([]interface{})[0].(map[string]interface{})["color"] = "#b44ec9"
	encoded, _ = json.Marshal(raw)
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdatePipeline(context.Background(), pipeline.ID, payload)
	if err != nil {
		t.Fatal(err)
	}
	var color string
	db.Table("crm_pipeline_stages").Select("color").Where("id = ?", "a").Scan(&color)
	if color != "#b44ec9" {
		t.Fatalf("color not saved: %q", color)
	}
	// A client that sends stage edits without color must preserve stored customization.
	raw["stages"].([]interface{})[0].(map[string]interface{})["name"] = "Discovery"
	for _, stage := range raw["stages"].([]interface{}) {
		delete(stage.(map[string]interface{}), "color")
	}
	encoded, _ = json.Marshal(raw)
	payload = model.UpdateCRMPipelineRequest{}
	json.Unmarshal(encoded, &payload)
	if _, err = svc.UpdatePipeline(context.Background(), updated.ID, payload); err != nil {
		t.Fatal(err)
	}
	db.Table("crm_pipeline_stages").Select("color").Where("id = ?", "a").Scan(&color)
	if color != "#b44ec9" {
		t.Fatal("legacy edit erased color")
	}
	raw["stages"].([]interface{})[0].(map[string]interface{})["color"] = "invalid"
	encoded, _ = json.Marshal(raw)
	json.Unmarshal(encoded, &payload)
	if _, err = svc.UpdatePipeline(context.Background(), updated.ID, payload); err == nil {
		t.Fatal("accepted invalid color")
	}
}
