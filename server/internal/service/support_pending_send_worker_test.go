package service

import (
	"context"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestSupportSendClaimSerializesConversationsAndRecoversStaleWork(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&model.SupportPendingSend{}); err != nil {
		t.Fatal(err)
	}
	svc := &SupportInboxService{messageRepo: repository.NewSupportMessageRepository(db)}
	now := time.Now().UTC()
	for _, job := range []model.SupportPendingSend{
		{ID: "a", WorkspaceID: "ws", ConversationID: "one", Status: "queued", CreatedAt: now, UpdatedAt: now},
		{ID: "b", WorkspaceID: "ws", ConversationID: "one", Status: "queued", CreatedAt: now, UpdatedAt: now},
		{ID: "c", WorkspaceID: "ws", ConversationID: "two", Status: "queued", CreatedAt: now.Add(time.Second), UpdatedAt: now},
	} {
		if err := db.Create(&job).Error; err != nil {
			t.Fatal(err)
		}
	}
	claim := func(want string) {
		t.Helper()
		job, err := svc.claimPendingSupportSend(context.Background())
		if err != nil || job.ID != want {
			t.Fatalf("claim=%+v err=%v want=%s", job, err, want)
		}
	}
	claim("a")
	claim("c")
	if _, err := svc.claimPendingSupportSend(context.Background()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("overlapping conversation claim: %v", err)
	}
	db.Model(&model.SupportPendingSend{}).Where("id = ?", "a").Update("updated_at", now.Add(-3*time.Minute))
	claim("a")
	db.Model(&model.SupportPendingSend{}).Where("id = ?", "a").Update("status", "sent")
	claim("b")
	db.Model(&model.SupportPendingSend{}).Where("id = ?", "a").Update("status", "queued")
	if _, err := svc.claimPendingSupportSend(context.Background()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("retry overlapped a later active reply: %v", err)
	}
}

func TestSupportSendWakeIsBufferedAndCoalesced(t *testing.T) {
	svc := &SupportInboxService{}
	for i := 0; i < 100; i++ {
		svc.wakePendingSupportSends()
	}
	wake := svc.pendingSupportSendWake()
	select {
	case <-wake:
	default:
		t.Fatal("enqueue wake was lost before worker started")
	}
	if len(wake) > supportSendWorkers {
		t.Fatal("wake queue is unbounded")
	}
}
