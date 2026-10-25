package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMRecordUpsertReportsNewCustomPropertyKeys(t *testing.T) {
	db, upserter := setupCRMRecordUpsertTest(t)
	first := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{acmeRecord()}})
	if !equalStrings(first.NewCustomPropertyKeys, []string{"fit", "support_tools"}) {
		t.Fatalf("first batch new keys = %v, want fit and support_tools", first.NewCustomPropertyKeys)
	}

	// Another workspace's keys do not count as existing here.
	other := model.CRMCompany{WorkspaceID: "workspace-other", DisplayID: "COM-9", Name: "Other", CustomProperties: model.JSONB{"tier": "gold"}}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	second := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{{
		Fields:           map[string]any{"domain": "acme.com"},
		CustomProperties: map[string]any{"fit": "no", "tier": "gold", "dropped": nil},
	}}})
	if !equalStrings(second.NewCustomPropertyKeys, []string{"tier"}) {
		t.Fatalf("second batch new keys = %v, want only tier", second.NewCustomPropertyKeys)
	}
}

func TestCRMRecordUpsertDoesNotReportKeysOfFailedRecords(t *testing.T) {
	_, upserter := setupCRMRecordUpsertTest(t)
	response := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{
		Mode: model.CRMUpsertModeUpdateOnly,
		Records: []model.CRMRecordUpsertInput{{
			Fields: map[string]any{"domain": "missing.example"}, CustomProperties: map[string]any{"ghost": "yes"},
		}},
	})
	if response.Results[0].Action == model.CRMUpsertActionCreated || len(response.NewCustomPropertyKeys) != 0 {
		t.Fatalf("response = %+v, want no new keys for a record that was not written", response)
	}
}

func TestCRMUpsertIntroducedKeysUsesWrittenChanges(t *testing.T) {
	results := []model.CRMRecordUpsertResult{
		{Action: model.CRMUpsertActionCreated, Changed: []string{"name", "custom_properties.tier"}},
		{Action: model.CRMUpsertActionError, Changed: []string{"custom_properties.ghost"}},
		{Action: model.CRMUpsertActionUpdated, Changed: []string{"custom_properties.fit"}},
	}
	got := crmUpsertIntroducedKeys([]string{"fit", "ghost", "seats", "tier"}, map[string]bool{"fit": true}, results)
	if !equalStrings(got, []string{"tier"}) {
		t.Fatalf("introduced = %v, want only tier: fit existed, ghost failed, seats was never written", got)
	}
}
