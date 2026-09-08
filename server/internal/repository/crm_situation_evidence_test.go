package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSituationEvidenceIsScopedAndReadOnly(t *testing.T) {
	db := crmsituationtest.Open(t)
	repo := NewCRMSituationRepository(db)
	crmsituationtest.Exec(t, db, `UPDATE crm_signals SET summary = ?, evidence_excerpt = ?, source_type = ?,
		source_thread_id = ?, company_id = ?, evidence_identity_trust = ?, detected_at = CURRENT_TIMESTAMP,
		metadata = ? WHERE id = ?`, "Customer requested pricing", "Please quote 40 seats", "email",
		crmsituationtest.Signal, crmsituationtest.Company, "verified", `{"internal":"not for this view"}`, crmsituationtest.Signal)
	crmsituationtest.Exec(t, db, "UPDATE crm_signals SET summary = ? WHERE id = ?", "Foreign secret", crmsituationtest.ForeignSignal)
	// Legacy action evidence may include malformed foreign IDs. They must not disclose evidence.
	crmsituationtest.Exec(t, db, "UPDATE crm_suggestions SET signal_ids = ? WHERE id = ?",
		model.StringArray{crmsituationtest.Signal, crmsituationtest.ForeignSignal}, crmsituationtest.Suggestion)
	input := crmsituationtest.Situation("conversion")
	stored, _, err := repo.Create(context.Background(), input, []model.CRMSituationReference{
		{Kind: "signal", SourceID: crmsituationtest.Signal},
		{Kind: "suggestion", SourceID: crmsituationtest.Suggestion},
	})
	if err != nil {
		t.Fatal(err)
	}
	item, err := repo.GetByID(context.Background(), crmsituationtest.Workspace, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Evidence) != 1 || item.Evidence[0].ID != crmsituationtest.Signal || item.Evidence[0].Summary != "Customer requested pricing" {
		t.Fatalf("scoped, deduplicated evidence = %#v", item.Evidence)
	}
	if item.Evidence[0].SourceThreadID == nil || *item.Evidence[0].SourceThreadID != crmsituationtest.Signal {
		t.Fatalf("missing exact source provenance: %#v", item.Evidence[0])
	}
	encoded, err := json.Marshal(item.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Foreign secret") || strings.Contains(string(encoded), "internal") {
		t.Fatalf("disclosed non-projection fields: %s", encoded)
	}
	var changed int64
	if err := db.Table("crm_signals").Where("reviewed_at IS NOT NULL OR acted_at IS NOT NULL").Count(&changed).Error; err != nil {
		t.Fatal(err)
	}
	if changed != 0 || item.Actions[0].Status != "pending" {
		t.Fatal("reading evidence changed an action or reviewed its source")
	}
}
