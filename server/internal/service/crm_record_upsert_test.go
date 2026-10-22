package service

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const _upsertTestWorkspace = "workspace-upsert"

func setupCRMRecordUpsertTest(t *testing.T) (*gorm.DB, *CRMRecordUpsertService) {
	t.Helper()
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE crm_activities (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		activity_type TEXT NOT NULL DEFAULT 'note',
		contact_id TEXT,
		company_id TEXT,
		deal_id TEXT,
		owner_member_id TEXT,
		subject TEXT,
		body TEXT,
		occurred_at DATETIME NOT NULL,
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create crm_activities: %v", err)
	}
	companyRepo := repository.NewCRMCompanyRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	upserter := NewCRMRecordUpsertService(
		companyRepo, contactRepo,
		NewCRMCompanyService(companyRepo), NewCRMContactService(contactRepo),
		NewCRMActivityService(repository.NewCRMActivityRepository(db)),
	)
	return db, upserter
}

func upsertCompanies(t *testing.T, upserter *CRMRecordUpsertService, req model.CRMRecordUpsertRequest) *model.CRMRecordUpsertResponse {
	t.Helper()
	req.WorkspaceID = _upsertTestWorkspace
	if req.ObjectType == "" {
		req.ObjectType = model.CRMUpsertObjectCompany
	}
	response, err := upserter.Upsert(context.Background(), req)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	return response
}

func acmeRecord() model.CRMRecordUpsertInput {
	return model.CRMRecordUpsertInput{
		Ref:              "lead-1",
		Fields:           map[string]any{"name": "Acme", "domain": "https://www.Acme.com/pricing", "headquarters": "Germany"},
		CustomProperties: map[string]any{"fit": "yes", "support_tools": []any{"zendesk", "intercom"}},
		Note:             &model.CRMRecordUpsertNote{Subject: "Research", Body: "Help center at help.acme.com"},
	}
}

func TestCRMRecordUpsertCreatesThenRerunIsUnchanged(t *testing.T) {
	db, upserter := setupCRMRecordUpsertTest(t)
	request := model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{acmeRecord()}}

	first := upsertCompanies(t, upserter, request)
	created := first.Results[0]
	if created.Action != model.CRMUpsertActionCreated || created.ID == "" || !created.NoteAdded || created.Ref != "lead-1" {
		t.Fatalf("first result = %+v", created)
	}
	var company model.CRMCompany
	if err := db.First(&company, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("load company: %v", err)
	}
	if company.Domain == nil || *company.Domain != "acme.com" || company.CustomProperties["fit"] != "yes" {
		t.Fatalf("stored company = %+v", company)
	}

	second := upsertCompanies(t, upserter, request)
	if got := second.Results[0]; got.Action != model.CRMUpsertActionUnchanged || got.ID != created.ID || got.NoteAdded {
		t.Fatalf("rerun result = %+v", got)
	}
	var companies, notes int64
	db.Model(&model.CRMCompany{}).Count(&companies)
	db.Model(&model.CRMActivity{}).Count(&notes)
	if companies != 1 || notes != 1 {
		t.Fatalf("companies = %d, notes = %d; want 1 and 1", companies, notes)
	}
}

func TestCRMRecordUpsertMatchesStoredDomainSpellings(t *testing.T) {
	db, upserter := setupCRMRecordUpsertTest(t)
	legacy := model.CRMCompany{WorkspaceID: _upsertTestWorkspace, DisplayID: "COM-1", Name: "Acme", Domain: stringPtr("https://www.acme.com/")}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("seed company: %v", err)
	}
	response := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{{
		Fields: map[string]any{"domain": "acme.com"}, CustomProperties: map[string]any{"fit": "no"},
	}}})
	got := response.Results[0]
	if got.ID != legacy.ID || got.Action != model.CRMUpsertActionUpdated {
		t.Fatalf("result = %+v, want update of %s", got, legacy.ID)
	}
	var companies int64
	db.Model(&model.CRMCompany{}).Count(&companies)
	if companies != 1 {
		t.Fatalf("companies = %d, want 1", companies)
	}
}

func TestCRMRecordUpsertFillOnlyKeepsExistingValues(t *testing.T) {
	db, upserter := setupCRMRecordUpsertTest(t)
	upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{acmeRecord()}})

	response := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{FillOnly: true, Records: []model.CRMRecordUpsertInput{{
		Fields:           map[string]any{"domain": "acme.com", "headquarters": "France", "industry": "Software"},
		CustomProperties: map[string]any{"fit": "no", "batch": "b2"},
	}}})
	got := response.Results[0]
	if got.Action != model.CRMUpsertActionUpdated {
		t.Fatalf("result = %+v", got)
	}
	wantChanged := []string{"industry", "custom_properties.batch"}
	wantKept := []string{"headquarters", "custom_properties.fit"}
	if !equalStrings(got.Changed, wantChanged) || !equalStrings(got.Kept, wantKept) {
		t.Fatalf("changed = %v kept = %v; want %v and %v", got.Changed, got.Kept, wantChanged, wantKept)
	}
	var company model.CRMCompany
	db.First(&company, "id = ?", got.ID)
	if *company.Headquarters != "Germany" || *company.Industry != "Software" ||
		company.CustomProperties["fit"] != "yes" || company.CustomProperties["batch"] != "b2" {
		t.Fatalf("stored company = %+v", company)
	}
}

func TestCRMRecordUpsertOverwritesAndRemovesCustomProperties(t *testing.T) {
	db, upserter := setupCRMRecordUpsertTest(t)
	upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{acmeRecord()}})

	response := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{{
		Fields: map[string]any{"domain": "acme.com"}, CustomProperties: map[string]any{"fit": "no", "support_tools": nil},
	}}})
	var company model.CRMCompany
	db.First(&company, "id = ?", response.Results[0].ID)
	if _, present := company.CustomProperties["support_tools"]; present || company.CustomProperties["fit"] != "no" {
		t.Fatalf("custom properties = %v", company.CustomProperties)
	}
}

func TestCRMRecordUpsertModes(t *testing.T) {
	_, upserter := setupCRMRecordUpsertTest(t)
	upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{acmeRecord()}})

	tests := []struct {
		name       string
		mode       string
		domain     string
		wantAction string
		wantReason string
	}{
		{name: "update only skips unknown domains", mode: model.CRMUpsertModeUpdateOnly, domain: "unknown.io", wantAction: model.CRMUpsertActionSkipped, wantReason: "not_found"},
		{name: "create only skips existing domains", mode: model.CRMUpsertModeCreateOnly, domain: "acme.com", wantAction: model.CRMUpsertActionSkipped, wantReason: "exists"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Mode: tt.mode, Records: []model.CRMRecordUpsertInput{{
				Fields: map[string]any{"name": "Any", "domain": tt.domain},
			}}})
			if got := response.Results[0]; got.Action != tt.wantAction || got.Reason != tt.wantReason {
				t.Fatalf("result = %+v", got)
			}
		})
	}
}

func TestCRMRecordUpsertReportsRecordErrorsWithoutStoppingBatch(t *testing.T) {
	db, upserter := setupCRMRecordUpsertTest(t)
	response := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{Records: []model.CRMRecordUpsertInput{
		{Ref: "missing-name", Fields: map[string]any{"domain": "nameless.io"}},
		{Ref: "bad-key", Fields: map[string]any{"name": "Bad", "domain": "bad.io"}, CustomProperties: map[string]any{"Fit": "yes"}},
		{Ref: "reserved", Fields: map[string]any{"name": "Res", "domain": "res.io"}, CustomProperties: map[string]any{"agent_enrichment": "x"}},
		{Ref: "unknown-field", Fields: map[string]any{"name": "Unk", "domain": "unk.io", "email": "a@b.io"}},
		{Ref: "good", Fields: map[string]any{"name": "Good", "domain": "good.io"}},
		{Ref: "dupe", Fields: map[string]any{"name": "Good again", "domain": "www.good.io"}},
	}})
	actions := map[string]string{}
	for _, result := range response.Results {
		actions[result.Ref] = result.Action
		if result.Action == model.CRMUpsertActionError && result.Error == "" {
			t.Errorf("%s has no error message", result.Ref)
		}
	}
	want := map[string]string{
		"missing-name": model.CRMUpsertActionError, "bad-key": model.CRMUpsertActionError,
		"reserved": model.CRMUpsertActionError, "unknown-field": model.CRMUpsertActionError,
		"good": model.CRMUpsertActionCreated, "dupe": model.CRMUpsertActionError,
	}
	for ref, action := range want {
		if actions[ref] != action {
			t.Errorf("%s action = %q, want %q", ref, actions[ref], action)
		}
	}
	if response.Counts[model.CRMUpsertActionError] != 5 || response.Counts[model.CRMUpsertActionCreated] != 1 {
		t.Fatalf("counts = %v", response.Counts)
	}
	var companies int64
	db.Model(&model.CRMCompany{}).Count(&companies)
	if companies != 1 {
		t.Fatalf("companies = %d, want 1", companies)
	}
}

func TestCRMRecordUpsertRejectsInvalidRequests(t *testing.T) {
	_, upserter := setupCRMRecordUpsertTest(t)
	record := []model.CRMRecordUpsertInput{{Fields: map[string]any{"name": "X", "domain": "x.io"}}}
	tests := []struct {
		name string
		req  model.CRMRecordUpsertRequest
	}{
		{name: "unknown object type", req: model.CRMRecordUpsertRequest{ObjectType: "deal", Records: record}},
		{name: "email match for companies", req: model.CRMRecordUpsertRequest{ObjectType: model.CRMUpsertObjectCompany, MatchOn: model.CRMUpsertMatchEmail, Records: record}},
		{name: "domain match for contacts", req: model.CRMRecordUpsertRequest{ObjectType: model.CRMUpsertObjectContact, MatchOn: model.CRMUpsertMatchDomain, Records: record}},
		{name: "create by id", req: model.CRMRecordUpsertRequest{ObjectType: model.CRMUpsertObjectCompany, MatchOn: model.CRMUpsertMatchID, Mode: model.CRMUpsertModeCreateOnly, Records: record}},
		{name: "empty batch", req: model.CRMRecordUpsertRequest{ObjectType: model.CRMUpsertObjectCompany}},
		{name: "unknown mode", req: model.CRMRecordUpsertRequest{ObjectType: model.CRMUpsertObjectCompany, Mode: "merge", Records: record}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.WorkspaceID = _upsertTestWorkspace
			_, err := upserter.Upsert(context.Background(), tt.req)
			if _, ok := CRMUpsertInputMessage(err); !ok {
				t.Fatalf("Upsert() error = %v, want input error", err)
			}
		})
	}
}

func TestCRMRecordUpsertMatchByIDStaysInWorkspace(t *testing.T) {
	db, upserter := setupCRMRecordUpsertTest(t)
	other := model.CRMCompany{WorkspaceID: "other-workspace", DisplayID: "COM-1", Name: "Other"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("seed company: %v", err)
	}
	response := upsertCompanies(t, upserter, model.CRMRecordUpsertRequest{MatchOn: model.CRMUpsertMatchID, Records: []model.CRMRecordUpsertInput{{
		ID: other.ID, Fields: map[string]any{"industry": "Leaked"},
	}}})
	if got := response.Results[0]; got.Action != model.CRMUpsertActionError {
		t.Fatalf("result = %+v, want not-found error", got)
	}
	var stored model.CRMCompany
	db.First(&stored, "id = ?", other.ID)
	if stored.Industry != nil {
		t.Fatalf("cross-workspace company was modified: %+v", stored)
	}
}

func TestCRMRecordUpsertExternalIDMatch(t *testing.T) {
	_, upserter := setupCRMRecordUpsertTest(t)
	request := model.CRMRecordUpsertRequest{MatchOn: model.CRMUpsertMatchExternalID, Records: []model.CRMRecordUpsertInput{{
		Fields: map[string]any{"external_id": "leads:42", "name": "Acme"},
	}}}
	first := upsertCompanies(t, upserter, request)
	request.Records[0].Fields["employee_count"] = float64(40)
	second := upsertCompanies(t, upserter, request)
	if first.Results[0].ID != second.Results[0].ID || second.Results[0].Action != model.CRMUpsertActionUpdated {
		t.Fatalf("first = %+v second = %+v", first.Results[0], second.Results[0])
	}
}

func TestCRMRecordUpsertContactsByEmail(t *testing.T) {
	db, upserter := setupCRMRecordUpsertTest(t)
	request := model.CRMRecordUpsertRequest{ObjectType: model.CRMUpsertObjectContact, Records: []model.CRMRecordUpsertInput{{
		Fields:           map[string]any{"first_name": "Ada", "email": "Ada@Acme.com", "lifecycle_stage": model.CRMLifecycleLead},
		CustomProperties: map[string]any{"first_line": "Loved your docs"},
	}}}
	first := upsertCompanies(t, upserter, request)
	request.Records[0].Fields["email"] = "ada@acme.com"
	request.Records[0].Fields["job_title"] = "Head of Support"
	second := upsertCompanies(t, upserter, request)
	if first.Results[0].Action != model.CRMUpsertActionCreated || second.Results[0].Action != model.CRMUpsertActionUpdated ||
		first.Results[0].ID != second.Results[0].ID {
		t.Fatalf("first = %+v second = %+v", first.Results[0], second.Results[0])
	}
	var contact model.CRMContact
	db.First(&contact, "id = ?", first.Results[0].ID)
	if contact.JobTitle == nil || *contact.JobTitle != "Head of Support" || contact.CustomProperties["first_line"] != "Loved your docs" {
		t.Fatalf("stored contact = %+v", contact)
	}
}

func TestNormalizeCRMUpsertFields(t *testing.T) {
	tests := []struct {
		name       string
		objectType string
		fields     map[string]any
		want       map[string]any
		wantErr    bool
	}{
		{name: "domain is normalized", objectType: model.CRMUpsertObjectCompany, fields: map[string]any{"domain": " HTTPS://www.Acme.com/x "}, want: map[string]any{"domain": "acme.com"}},
		{name: "blank strings are dropped", objectType: model.CRMUpsertObjectCompany, fields: map[string]any{"industry": "  "}, want: map[string]any{}},
		{name: "email is lowercased", objectType: model.CRMUpsertObjectContact, fields: map[string]any{"email": "A@B.IO"}, want: map[string]any{"email": "a@b.io"}},
		{name: "fractional employee count", objectType: model.CRMUpsertObjectCompany, fields: map[string]any{"employee_count": 1.5}, wantErr: true},
		{name: "negative revenue", objectType: model.CRMUpsertObjectCompany, fields: map[string]any{"annual_revenue": float64(-1)}, wantErr: true},
		{name: "invalid domain", objectType: model.CRMUpsertObjectCompany, fields: map[string]any{"domain": "not a domain"}, wantErr: true},
		{name: "non http url", objectType: model.CRMUpsertObjectCompany, fields: map[string]any{"linkedin_url": "ftp://x.io"}, wantErr: true},
		{name: "unknown lifecycle stage", objectType: model.CRMUpsertObjectContact, fields: map[string]any{"lifecycle_stage": "vip"}, wantErr: true},
		{name: "null value", objectType: model.CRMUpsertObjectCompany, fields: map[string]any{"industry": nil}, wantErr: true},
		{name: "too long", objectType: model.CRMUpsertObjectContact, fields: map[string]any{"country_code": "DEU"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeCRMUpsertFields(tt.objectType, tt.fields)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeCRMUpsertFields() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for key, value := range tt.want {
				if got[key] != value {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
