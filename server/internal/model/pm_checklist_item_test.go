package model

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestChecklistItem_AssigneeID_Nil(t *testing.T) {
	item := PMChecklistItem{
		ID:     "item-1",
		TaskID: "story-1",
		Text:   "Fix the bug",
	}
	if item.AssigneeID != nil {
		t.Error("expected nil assignee_id for unassigned item")
	}
}

func TestChecklistItem_AssigneeID_Set(t *testing.T) {
	uid := "user-abc"
	item := PMChecklistItem{
		ID:         "item-2",
		TaskID:     "story-1",
		Text:       "Review PR",
		AssigneeID: &uid,
	}
	if item.AssigneeID == nil || *item.AssigneeID != "user-abc" {
		t.Errorf("expected assignee_id='user-abc', got %v", item.AssigneeID)
	}
}

func TestCreateChecklistItemRequest_WithAssignee(t *testing.T) {
	uid := "user-xyz"
	req := CreateChecklistItemRequest{
		Text:       "@john.doe check this",
		AssigneeID: &uid,
	}
	if req.AssigneeID == nil || *req.AssigneeID != "user-xyz" {
		t.Errorf("expected assignee_id='user-xyz'")
	}
}

func TestCreateChecklistItemRequest_WithoutAssignee(t *testing.T) {
	req := CreateChecklistItemRequest{
		Text: "Simple task",
	}
	if req.AssigneeID != nil {
		t.Errorf("expected nil assignee_id")
	}
}

func TestUpdateChecklistItemRequest_AssigneeChange(t *testing.T) {
	uid := "user-new"
	req := UpdateChecklistItemRequest{
		AssigneeID: &uid,
	}
	if req.AssigneeID == nil || *req.AssigneeID != "user-new" {
		t.Errorf("expected assignee_id='user-new'")
	}
}

func TestChecklistItem_WithMentionText(t *testing.T) {
	item := PMChecklistItem{
		Text: "@azhar.usermaven Review the deployment",
	}
	if item.Text == "" {
		t.Error("expected non-empty text")
	}
	// The text stores raw @mention — extraction happens in the service layer.
	if item.Text != "@azhar.usermaven Review the deployment" {
		t.Errorf("unexpected text: %q", item.Text)
	}
}

func TestChecklistItemDueDateDTOs(t *testing.T) {
	field, ok := reflect.TypeOf(PMChecklistItem{}).FieldByName("DueDate")
	if !ok {
		t.Fatal("PMChecklistItem.DueDate field is missing")
	}
	if got := field.Tag.Get("gorm"); got != "type:date" {
		t.Fatalf("DueDate GORM tag = %q, want type:date", got)
	}

	dueDate := time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC)
	item := PMChecklistItem{DueDate: &dueDate}
	if item.DueDate == nil || !item.DueDate.Equal(dueDate) {
		t.Fatalf("item due date = %v, want %v", item.DueDate, dueDate)
	}

	createReq := CreateChecklistItemRequest{DueDate: &dueDate}
	if createReq.DueDate == nil || !createReq.DueDate.Equal(dueDate) {
		t.Fatalf("create due date = %v, want %v", createReq.DueDate, dueDate)
	}

	updateReq := UpdateChecklistItemRequest{DueDate: &dueDate, DueDateSet: true}
	payload, err := json.Marshal(updateReq)
	if err != nil {
		t.Fatalf("marshal update request: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode update request JSON: %v", err)
	}
	if _, exists := decoded["due_date"]; !exists {
		t.Fatalf("due date missing from JSON: %s", payload)
	}
	if _, exists := decoded["due_date_set"]; exists {
		t.Fatalf("internal presence flag leaked into JSON: %s", payload)
	}
}

func TestUpdateChecklistItemRequestDueDateJSONPresence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		payload  string
		wantSet  bool
		wantNil  bool
		wantDate string
	}{
		{name: "omitted", payload: `{"text":"Keep date"}`, wantSet: false, wantNil: true},
		{name: "null clears", payload: `{"due_date":null}`, wantSet: true, wantNil: true},
		{name: "empty clears", payload: `{"due_date":""}`, wantSet: true, wantNil: true},
		{name: "date only", payload: `{"due_date":"2026-09-15"}`, wantSet: true, wantDate: "2026-09-15"},
		{name: "rfc3339", payload: `{"due_date":"2026-09-16T12:30:00Z"}`, wantSet: true, wantDate: "2026-09-16"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req UpdateChecklistItemRequest
			if err := json.Unmarshal([]byte(tc.payload), &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if req.DueDateSet != tc.wantSet {
				t.Fatalf("DueDateSet = %v, want %v", req.DueDateSet, tc.wantSet)
			}
			if tc.wantNil {
				if req.DueDate != nil {
					t.Fatalf("DueDate = %v, want nil", req.DueDate)
				}
				return
			}
			if req.DueDate == nil || req.DueDate.Format("2006-01-02") != tc.wantDate {
				t.Fatalf("DueDate = %v, want %s", req.DueDate, tc.wantDate)
			}
		})
	}
}

func TestCreateChecklistItemRequestAcceptsDateOnlyAndRFC3339(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{`{"text":"Date only","due_date":"2026-09-15"}`, `{"text":"Timestamp","due_date":"2026-09-16T12:30:00Z"}`} {
		var req CreateChecklistItemRequest
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			t.Fatalf("unmarshal %s: %v", payload, err)
		}
		if req.DueDate == nil {
			t.Fatalf("due date missing for %s", payload)
		}
	}
}
