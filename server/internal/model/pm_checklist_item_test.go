package model

import "testing"

func TestChecklistItem_AssigneeID_Nil(t *testing.T) {
	item := PMChecklistItem{
		ID:      "item-1",
		TaskID: "story-1",
		Text:    "Fix the bug",
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
