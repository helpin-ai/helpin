package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIMessageProcessingFailedAttemptRetryOwnership(t *testing.T) {
	ctx := context.Background()
	db := setupProcessingTestDB(t)
	repo := NewAIMessageProcessingRepository(db)
	first, ok, err := repo.BeginAttempt(ctx, "ws", "source", "conv")
	if err != nil || !ok {
		t.Fatalf("claim: %v", err)
	}
	if err := repo.FailAttempt(ctx, first); err != nil {
		t.Fatal(err)
	}
	second, ok, err := repo.BeginAttempt(ctx, "ws", "source", "conv")
	if err != nil || !ok || second.Attempts != 2 {
		t.Fatalf("retry: %+v %v %v", second, ok, err)
	}
	// A late failure from the original worker must not release the new worker.
	if err := repo.FailAttempt(ctx, first); err != nil {
		t.Fatal(err)
	}
	var saved model.AIMessageProcessing
	if err := db.First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != "processing" || saved.Attempts != 2 {
		t.Fatalf("new owner overwritten: %+v", saved)
	}
	if err := repo.MarkCompleted(ctx, second.ID, nil, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.FailAttempt(ctx, second); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := repo.BeginAttempt(ctx, "ws", "source", "conv"); err != nil || claimed {
		t.Fatalf("completed turn retried: %v %v", claimed, err)
	}
}

func TestAIMessageProcessingClaimReportsDatabaseError(t *testing.T) {
	db := setupProcessingTestDB(t)
	repo := NewAIMessageProcessingRepository(db)
	if err := db.Exec(`DROP TABLE ai_message_processing`).Error; err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.BeginAttempt(context.Background(), "ws", "source", "conv"); err == nil || ok {
		t.Fatalf("database failure was treated as handled: %v %v", ok, err)
	}
}

func TestAIMessageProcessingStaleClaimReleasesOnFailure(t *testing.T) {
	ctx := context.Background()
	db := setupProcessingTestDB(t)
	repo := NewAIMessageProcessingRepository(db)
	first, _, err := repo.BeginAttempt(ctx, "ws", "source", "conv")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AIMessageProcessing{}).Where("id = ?", first.ID).Update("updated_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	reclaimed, ok, err := repo.BeginAttempt(ctx, "ws", "source", "conv")
	if err != nil || !ok {
		t.Fatalf("reclaim %v %v", ok, err)
	}
	if err := repo.FailAttempt(ctx, reclaimed); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.BeginAttempt(ctx, "ws", "source", "conv"); err != nil || !ok {
		t.Fatalf("retry reclaim %v %v", ok, err)
	}
}
