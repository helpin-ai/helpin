package repository

import (
	"context"
	"encoding/json"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDealQueryFilters(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	for _, sql := range []string{
		`CREATE TABLE crm_pipelines(id text,workspace_id text)`,
		`CREATE TABLE crm_pipeline_stages(id text,pipeline_id text,stage_type text,probability integer)`,
		`INSERT INTO crm_pipeline_stages VALUES('lead','sales','open',20)`,
		`CREATE TABLE crm_deals(id text,workspace_id text,name text,pipeline_id text,stage_id text,amount real,revenue_type text,owner_member_id text,probability integer,created_at datetime)`,
		`INSERT INTO crm_deals VALUES('a','ws','Large','sales','lead',1200,'annual','owner',NULL,CURRENT_TIMESTAMP),('b','ws','Small','sales','lead',100,'monthly',NULL,45,CURRENT_TIMESTAMP),('c','ws','Unset','sales','lead',NULL,'one_time',NULL,NULL,CURRENT_TIMESTAMP),('foreign','other','Small','sales','lead',9999,'annual',NULL,NULL,CURRENT_TIMESTAMP)`,
	} {
		if err = db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, filter string
		want         int
		invalid      bool
	}{
		{"amount and revenue", `{"logic":"and","rules":[{"field":"amount","operator":"gte","value":"1000"},{"field":"revenue_type","operator":"is","value":"annual"}]}`, 1, false},
		{"or stays scoped", `{"logic":"or","rules":[{"field":"amount","operator":"gte","value":"1000"},{"field":"name","operator":"is","value":"Small"}]}`, 2, false},
		{"empty amount", `{"logic":"and","rules":[{"field":"amount","operator":"is_empty"}]}`, 1, false},
		{"stage probability fallback", `{"logic":"and","rules":[{"field":"probability","operator":"is","value":"20"}]}`, 2, false},
		{"range", `{"logic":"and","rules":[{"field":"amount","operator":"between","values":["100","1200"]}]}`, 2, false},
		{"invalid field", `{"rules":[{"field":"private_data","operator":"is","value":"x"}]}`, 0, true},
		{"invalid number", `{"rules":[{"field":"amount","operator":"gte","value":"NaN"}]}`, 0, true},
		{"invalid range", `{"rules":[{"field":"amount","operator":"between","values":["1200","100"]}]}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var filters model.CRMDealListFilters
			if err := json.Unmarshal([]byte(`{"Query":`+tc.filter+`}`), &filters); err != nil {
				t.Fatal(err)
			}
			rows, total, err := NewCRMDealRepository(db).List(context.Background(), "ws", filters, model.PMPagination{Page: 1, PerPage: 20})
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid filter accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != tc.want || int(total) != tc.want {
				t.Fatalf("got %d rows/%d total, want %d", len(rows), total, tc.want)
			}
		})
	}
}
