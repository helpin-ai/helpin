package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

func newFollowerServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:follower-svc-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE entity_followers (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			reason TEXT,
			created_at DATETIME,
			UNIQUE(user_id, entity_type, entity_id)
		)
	`).Error; err != nil {
		t.Fatalf("create entity_followers table: %v", err)
	}
	return db
}

func TestFollowerService_FollowAndGetFollowers(t *testing.T) {
	db := newFollowerServiceTestDB(t)
	svc := NewFollowerService(repository.NewFollowerRepository(db))
	ctx := context.Background()

	if err := svc.Follow(ctx, "user-1", "task", "task-1", "ws-1", "manual"); err != nil {
		t.Fatalf("Follow: %v", err)
	}

	followers, err := svc.GetFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("GetFollowers: %v", err)
	}
	if len(followers) != 1 || followers[0] != "user-1" {
		t.Fatalf("followers = %v, want [user-1]", followers)
	}
}

func TestFollowerService_Follow_DefaultReasonIsManual(t *testing.T) {
	db := newFollowerServiceTestDB(t)
	svc := NewFollowerService(repository.NewFollowerRepository(db))
	ctx := context.Background()

	if err := svc.Follow(ctx, "user-1", "task", "task-1", "ws-1", ""); err != nil {
		t.Fatalf("Follow: %v", err)
	}

	followers, err := svc.ListFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("ListFollowers: %v", err)
	}
	if len(followers) != 1 {
		t.Fatalf("follower count = %d, want 1", len(followers))
	}
	if followers[0].Reason != "manual" {
		t.Fatalf("reason = %q, want manual", followers[0].Reason)
	}
}

func TestFollowerService_Unfollow(t *testing.T) {
	db := newFollowerServiceTestDB(t)
	svc := NewFollowerService(repository.NewFollowerRepository(db))
	ctx := context.Background()

	if err := svc.Follow(ctx, "user-1", "task", "task-1", "ws-1", "manual"); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	if err := svc.Unfollow(ctx, "user-1", "task", "task-1"); err != nil {
		t.Fatalf("Unfollow: %v", err)
	}

	followers, err := svc.GetFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("GetFollowers: %v", err)
	}
	if len(followers) != 0 {
		t.Fatalf("followers after unfollow = %v, want empty", followers)
	}
}

func TestFollowerService_IsFollowing(t *testing.T) {
	db := newFollowerServiceTestDB(t)
	svc := NewFollowerService(repository.NewFollowerRepository(db))
	ctx := context.Background()

	following, err := svc.IsFollowing(ctx, "user-1", "task", "task-1")
	if err != nil {
		t.Fatalf("IsFollowing: %v", err)
	}
	if following {
		t.Fatal("expected not following before follow")
	}

	if err := svc.Follow(ctx, "user-1", "task", "task-1", "ws-1", "manual"); err != nil {
		t.Fatalf("Follow: %v", err)
	}

	following, err = svc.IsFollowing(ctx, "user-1", "task", "task-1")
	if err != nil {
		t.Fatalf("IsFollowing after: %v", err)
	}
	if !following {
		t.Fatal("expected following after follow")
	}
}

func TestFollowerService_ListFollowers(t *testing.T) {
	db := newFollowerServiceTestDB(t)
	svc := NewFollowerService(repository.NewFollowerRepository(db))
	ctx := context.Background()

	if err := svc.Follow(ctx, "user-1", "task", "task-1", "ws-1", "manual"); err != nil {
		t.Fatalf("Follow user-1: %v", err)
	}
	if err := svc.Follow(ctx, "user-2", "task", "task-1", "ws-1", "assigned"); err != nil {
		t.Fatalf("Follow user-2: %v", err)
	}

	records, err := svc.ListFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("ListFollowers: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("follower count = %d, want 2", len(records))
	}
}

func TestFollowerService_ListUserFollowing(t *testing.T) {
	db := newFollowerServiceTestDB(t)
	svc := NewFollowerService(repository.NewFollowerRepository(db))
	ctx := context.Background()

	if err := svc.Follow(ctx, "user-1", "task", "task-1", "ws-1", "manual"); err != nil {
		t.Fatalf("Follow task-1: %v", err)
	}
	if err := svc.Follow(ctx, "user-1", "epic", "epic-1", "ws-1", "manual"); err != nil {
		t.Fatalf("Follow epic-1: %v", err)
	}
	if err := svc.Follow(ctx, "user-1", "task", "task-2", "ws-2", "manual"); err != nil {
		t.Fatalf("Follow ws-2: %v", err)
	}

	following, err := svc.ListUserFollowing(ctx, "user-1", "ws-1")
	if err != nil {
		t.Fatalf("ListUserFollowing: %v", err)
	}
	if len(following) != 2 {
		t.Fatalf("following count = %d, want 2 (ws-1 only)", len(following))
	}
}

func TestFollowerService_FollowIdempotent(t *testing.T) {
	db := newFollowerServiceTestDB(t)
	svc := NewFollowerService(repository.NewFollowerRepository(db))
	ctx := context.Background()

	if err := svc.Follow(ctx, "user-1", "task", "task-1", "ws-1", "manual"); err != nil {
		t.Fatalf("Follow first: %v", err)
	}
	if err := svc.Follow(ctx, "user-1", "task", "task-1", "ws-1", "assigned"); err != nil {
		t.Fatalf("Follow second: %v", err)
	}

	followers, err := svc.GetFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("GetFollowers: %v", err)
	}
	if len(followers) != 1 {
		t.Fatalf("followers = %d, want 1 (no duplicates)", len(followers))
	}
}

func TestFollowerService_UnfollowNonexistent(t *testing.T) {
	db := newFollowerServiceTestDB(t)
	svc := NewFollowerService(repository.NewFollowerRepository(db))
	ctx := context.Background()

	if err := svc.Unfollow(ctx, "user-1", "task", "task-999"); err != nil {
		t.Fatalf("Unfollow nonexistent should not error: %v", err)
	}
}
