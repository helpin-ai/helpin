package model

import (
	"testing"
	"time"
)

func TestCommentWithAuthor_ReplyFields(t *testing.T) {
	now := time.Now()
	parentID := "parent-123"

	parent := CommentWithAuthor{
		Comment: PMComment{
			ID:         "parent-123",
			EntityType: "task",
			EntityID:   "task-1",
			AuthorID:   "user-1",
			Body:       "Top-level comment",
			ParentID:   nil,
			CreatedAt:  now,
		},
		Author:     User{ID: "user-1", Email: "alice@example.com", FullName: "Alice"},
		ReplyCount: 2,
		Replies: []CommentWithAuthor{
			{
				Comment: PMComment{
					ID:         "reply-1",
					EntityType: "task",
					EntityID:   "task-1",
					AuthorID:   "user-2",
					Body:       "First reply",
					ParentID:   &parentID,
					CreatedAt:  now,
				},
				Author: User{ID: "user-2", Email: "bob@example.com", FullName: "Bob"},
			},
			{
				Comment: PMComment{
					ID:         "reply-2",
					EntityType: "task",
					EntityID:   "task-1",
					AuthorID:   "user-1",
					Body:       "Second reply",
					ParentID:   &parentID,
					CreatedAt:  now,
				},
				Author: User{ID: "user-1", Email: "alice@example.com", FullName: "Alice"},
			},
		},
	}

	if parent.ReplyCount != 2 {
		t.Errorf("expected reply_count=2, got %d", parent.ReplyCount)
	}
	if len(parent.Replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(parent.Replies))
	}
	if parent.Replies[0].Comment.ID != "reply-1" {
		t.Errorf("expected first reply ID 'reply-1', got %q", parent.Replies[0].Comment.ID)
	}
	if parent.Replies[1].Comment.ParentID == nil || *parent.Replies[1].Comment.ParentID != parentID {
		t.Errorf("expected reply parent_id=%q", parentID)
	}
}

func TestPMComment_ParentID_Nil(t *testing.T) {
	c := PMComment{
		ID:         "c1",
		EntityType: "task",
		EntityID:   "s1",
		AuthorID:   "u1",
		Body:       "Top-level",
		ParentID:   nil,
	}
	if c.ParentID != nil {
		t.Error("expected nil parent_id for top-level comment")
	}
}

func TestPMComment_ParentID_Set(t *testing.T) {
	parentID := "parent-abc"
	c := PMComment{
		ID:         "c2",
		EntityType: "task",
		EntityID:   "s1",
		AuthorID:   "u2",
		Body:       "Reply",
		ParentID:   &parentID,
	}
	if c.ParentID == nil {
		t.Fatal("expected non-nil parent_id for reply")
	}
	if *c.ParentID != "parent-abc" {
		t.Errorf("expected parent_id='parent-abc', got %q", *c.ParentID)
	}
}

func TestCommentWithAuthor_EmptyReplies(t *testing.T) {
	cwa := CommentWithAuthor{
		Comment: PMComment{
			ID:       "c1",
			Body:     "No replies",
			ParentID: nil,
		},
		ReplyCount: 0,
		Replies:    nil,
	}
	if cwa.ReplyCount != 0 {
		t.Errorf("expected reply_count=0, got %d", cwa.ReplyCount)
	}
	if cwa.Replies != nil {
		t.Errorf("expected nil replies, got %v", cwa.Replies)
	}
}

func TestCreateCommentRequest_WithParentID(t *testing.T) {
	parentID := "parent-xyz"
	req := CreateCommentRequest{
		EntityType: "task",
		EntityID:   "task-1",
		Body:       "This is a reply",
		ParentID:   &parentID,
	}
	if req.ParentID == nil || *req.ParentID != "parent-xyz" {
		t.Errorf("expected parent_id='parent-xyz'")
	}
}

func TestCreateCommentRequest_WithoutParentID(t *testing.T) {
	req := CreateCommentRequest{
		EntityType: "task",
		EntityID:   "task-1",
		Body:       "Top-level comment",
	}
	if req.ParentID != nil {
		t.Errorf("expected nil parent_id for top-level comment")
	}
}
