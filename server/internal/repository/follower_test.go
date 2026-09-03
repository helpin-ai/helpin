package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupFollowerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:follower_repo_%d?mode=memory&cache=shared", time.Now().UnixNano())
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

func TestFollowerRepository_FollowAndGetFollowers(t *testing.T) {
	db := setupFollowerTestDB(t)
	repo := NewFollowerRepository(db)
	ctx := context.Background()

	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-2", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "assigned",
	}); err != nil {
		t.Fatalf("Follow user-2: %v", err)
	}

	followers, err := repo.GetFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("GetFollowers: %v", err)
	}
	if len(followers) != 2 {
		t.Fatalf("follower count = %d, want 2", len(followers))
	}
}

func TestFollowerRepository_FollowUpsertNoDuplicate(t *testing.T) {
	db := setupFollowerTestDB(t)
	repo := NewFollowerRepository(db)
	ctx := context.Background()

	follower := &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "manual",
	}
	if err := repo.Follow(ctx, follower); err != nil {
		t.Fatalf("Follow first: %v", err)
	}
	// Second follow should not error (upsert)
	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "assigned",
	}); err != nil {
		t.Fatalf("Follow duplicate: %v", err)
	}

	followers, err := repo.GetFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("GetFollowers: %v", err)
	}
	if len(followers) != 1 {
		t.Fatalf("follower count = %d, want 1 (no duplicate)", len(followers))
	}
}

func TestFollowerRepository_Unfollow(t *testing.T) {
	db := setupFollowerTestDB(t)
	repo := NewFollowerRepository(db)
	ctx := context.Background()

	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow: %v", err)
	}

	if err := repo.Unfollow(ctx, "user-1", "task", "task-1"); err != nil {
		t.Fatalf("Unfollow: %v", err)
	}

	followers, err := repo.GetFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("GetFollowers: %v", err)
	}
	if len(followers) != 0 {
		t.Fatalf("follower count = %d, want 0 after unfollow", len(followers))
	}
}

func TestFollowerRepository_UnfollowNonexistent(t *testing.T) {
	db := setupFollowerTestDB(t)
	repo := NewFollowerRepository(db)
	ctx := context.Background()

	// Should not error even when nothing to delete
	if err := repo.Unfollow(ctx, "user-1", "task", "task-999"); err != nil {
		t.Fatalf("Unfollow nonexistent: %v", err)
	}
}

func TestFollowerRepository_IsFollowing(t *testing.T) {
	db := setupFollowerTestDB(t)
	repo := NewFollowerRepository(db)
	ctx := context.Background()

	following, err := repo.IsFollowing(ctx, "user-1", "task", "task-1")
	if err != nil {
		t.Fatalf("IsFollowing before follow: %v", err)
	}
	if following {
		t.Fatal("expected not following before follow")
	}

	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow: %v", err)
	}

	following, err = repo.IsFollowing(ctx, "user-1", "task", "task-1")
	if err != nil {
		t.Fatalf("IsFollowing after follow: %v", err)
	}
	if !following {
		t.Fatal("expected following after follow")
	}
}

func TestFollowerRepository_ListFollowers(t *testing.T) {
	db := setupFollowerTestDB(t)
	repo := NewFollowerRepository(db)
	ctx := context.Background()

	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow user-1: %v", err)
	}
	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-2", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "assigned",
	}); err != nil {
		t.Fatalf("Follow user-2: %v", err)
	}

	records, err := repo.ListFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("ListFollowers: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("follower records = %d, want 2", len(records))
	}
	for _, f := range records {
		if f.EntityType != "task" || f.EntityID != "task-1" {
			t.Fatalf("unexpected follower record: %+v", f)
		}
	}
}

func TestFollowerRepository_ListUserFollowing(t *testing.T) {
	db := setupFollowerTestDB(t)
	repo := NewFollowerRepository(db)
	ctx := context.Background()

	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow task-1: %v", err)
	}
	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "epic", EntityID: "epic-1",
		WorkspaceID: "ws-1", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow epic-1: %v", err)
	}
	// Different workspace — should not appear
	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-2",
		WorkspaceID: "ws-2", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow ws-2: %v", err)
	}

	following, err := repo.ListUserFollowing(ctx, "user-1", "ws-1")
	if err != nil {
		t.Fatalf("ListUserFollowing: %v", err)
	}
	if len(following) != 2 {
		t.Fatalf("following count = %d, want 2", len(following))
	}
}

func TestFollowerRepository_GetFollowers_DifferentEntities(t *testing.T) {
	db := setupFollowerTestDB(t)
	repo := NewFollowerRepository(db)
	ctx := context.Background()

	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-1",
		WorkspaceID: "ws-1", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow task-1: %v", err)
	}
	if err := repo.Follow(ctx, &model.EntityFollower{
		UserID: "user-1", EntityType: "task", EntityID: "task-2",
		WorkspaceID: "ws-1", Reason: "manual",
	}); err != nil {
		t.Fatalf("Follow task-2: %v", err)
	}

	followers, err := repo.GetFollowers(ctx, "task", "task-1")
	if err != nil {
		t.Fatalf("GetFollowers task-1: %v", err)
	}
	if len(followers) != 1 {
		t.Fatalf("task-1 follower count = %d, want 1", len(followers))
	}

	followers, err = repo.GetFollowers(ctx, "task", "task-2")
	if err != nil {
		t.Fatalf("GetFollowers task-2: %v", err)
	}
	if len(followers) != 1 {
		t.Fatalf("task-2 follower count = %d, want 1", len(followers))
	}

	followers, err = repo.GetFollowers(ctx, "task", "task-999")
	if err != nil {
		t.Fatalf("GetFollowers nonexistent: %v", err)
	}
	if len(followers) != 0 {
		t.Fatalf("nonexistent follower count = %d, want 0", len(followers))
	}
}
