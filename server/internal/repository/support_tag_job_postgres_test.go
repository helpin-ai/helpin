//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupSupportTagJobsPostgres(t *testing.T) (*gorm.DB, model.SupportConversation) {
	t.Helper()
	env := setupPMTriagePostgres(t)
	if err := env.db.AutoMigrate(&model.SupportConversation{}, &model.SupportMessage{}); err != nil {
		t.Fatal(err)
	}
	// This composite identity already exists in the production message schema.
	if err := env.db.Exec("CREATE UNIQUE INDEX support_messages_scope ON support_messages(workspace_id, conversation_id, id)").Error; err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../dbmigrate/sql/202610030001_support_tag_jobs.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := env.db.Exec(string(migration)).Error; err != nil {
			t.Fatal(err)
		}
	}
	conv := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: env.workspace, Subject: "Tagging", Status: "open", Source: "widget"}
	if err := env.db.Create(&conv).Error; err != nil {
		t.Fatal(err)
	}
	return env.db, conv
}

func createSupportTagJobMessage(t *testing.T, db *gorm.DB, conv model.SupportConversation) model.SupportMessage {
	t.Helper()
	msg := model.SupportMessage{ID: uuid.NewString(), WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "customer", MessageType: "reply", Content: "What does this cost?"}
	if err := NewSupportMessageRepository(db).Create(context.Background(), &msg); err != nil {
		t.Fatal(err)
	}
	return msg
}

func TestSupportTagJobPostgresTransactionalEnqueue(t *testing.T) {
	db, conv := setupSupportTagJobsPostgres(t)
	public := createSupportTagJobMessage(t, db, conv)
	for _, change := range []func(*model.SupportMessage){
		func(m *model.SupportMessage) { m.IsInternal = true },
		func(m *model.SupportMessage) { m.SenderType = "user" },
		func(m *model.SupportMessage) { m.MessageType = "system" },
		func(m *model.SupportMessage) { m.Content = "  " },
		func(m *model.SupportMessage) { m.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true} },
	} {
		msg := public
		msg.ID = uuid.NewString()
		change(&msg)
		if err := db.Create(&msg).Error; err != nil {
			t.Fatal(err)
		}
	}
	var jobs int64
	if err := db.Model(&model.SupportTagJob{}).Count(&jobs).Error; err != nil || jobs != 1 {
		t.Fatalf("eligible jobs=%d %v", jobs, err)
	}
	rollback := errors.New("rollback message")
	var rolledBack model.SupportMessage
	err := db.Transaction(func(tx *gorm.DB) error {
		rolledBack = createSupportTagJobMessage(t, tx, conv)
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if err := db.Model(&model.SupportTagJob{}).Where("message_id = ?", rolledBack.ID).Count(&jobs).Error; err != nil || jobs != 0 {
		t.Fatalf("orphan job=%d %v", jobs, err)
	}
	// A failed enqueue must fail the message insertion too.
	if err := db.Exec("ALTER TABLE support_tag_jobs ADD CONSTRAINT reject_new_test_job CHECK (false) NOT VALID").Error; err != nil {
		t.Fatal(err)
	}
	rejected := public
	rejected.ID = uuid.NewString()
	if err := NewSupportMessageRepository(db).Create(context.Background(), &rejected); err == nil {
		t.Fatal("message succeeded without durable job")
	}
	if err := db.Model(&model.SupportMessage{}).Where("id = ?", rejected.ID).Count(&jobs).Error; err != nil || jobs != 0 {
		t.Fatalf("message escaped failed enqueue: %d %v", jobs, err)
	}
}

func TestSupportTagJobPostgresConcurrentClaimAndFencing(t *testing.T) {
	db, conv := setupSupportTagJobsPostgres(t)
	createSupportTagJobMessage(t, db, conv)
	repo := NewSupportTagJobRepository(db)
	ctx := context.Background()
	var wg sync.WaitGroup
	claims := make([]*model.SupportTagJob, 8)
	for i := range claims {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			claims[i], err = repo.ClaimNext(ctx, time.Now().UTC())
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	var first *model.SupportTagJob
	for _, claim := range claims {
		if claim != nil {
			if first != nil {
				t.Fatal("duplicate claim")
			}
			first = claim
		}
	}
	if first == nil {
		t.Fatal("no claim")
	}
	if err := db.Model(first).Update("lease_until", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	second, err := repo.ClaimNext(ctx, time.Now().UTC())
	if err != nil || second == nil || second.Attempts != 2 || second.LeaseToken == first.LeaseToken {
		t.Fatalf("recovery=%+v %v", second, err)
	}
	applied := false
	if err := repo.Finalize(ctx, *first, func(*gorm.DB) (string, error) { applied = true; return "tagged", nil }); !errors.Is(err, ErrSupportTagLeaseLost) || applied {
		t.Fatalf("stale claimant applied=%v err=%v", applied, err)
	}
	if err := repo.Retry(ctx, *first, time.Now().UTC()); !errors.Is(err, ErrSupportTagLeaseLost) {
		t.Fatalf("stale retry=%v", err)
	}
	rollback := errors.New("audit unavailable")
	err = repo.Finalize(ctx, *second, func(tx *gorm.DB) (string, error) {
		if err := tx.Model(&model.SupportConversation{}).Where("id = ?", conv.ID).Update("subject", "must roll back").Error; err != nil {
			return "", err
		}
		return "", rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	var current model.SupportConversation
	if err := db.First(&current, "id = ?", conv.ID).Error; err != nil || current.Subject != conv.Subject {
		t.Fatalf("effect escaped rollback: %q %v", current.Subject, err)
	}
	if err := repo.Finalize(ctx, *second, func(*gorm.DB) (string, error) { return "tagged", nil }); err != nil {
		t.Fatal(err)
	}
	if claim, err := repo.ClaimNext(ctx, time.Now().UTC()); err != nil || claim != nil {
		t.Fatalf("completed work repeated=%+v %v", claim, err)
	}
}

func TestSupportTagJobPostgresBoundedRetriesAndPrivacy(t *testing.T) {
	db, conv := setupSupportTagJobsPostgres(t)
	msg := createSupportTagJobMessage(t, db, conv)
	repo := NewSupportTagJobRepository(db)
	ctx := context.Background()
	for attempt := 1; attempt <= 5; attempt++ {
		claim, err := repo.ClaimNext(ctx, time.Now().UTC())
		if err != nil || claim == nil || claim.Attempts != attempt {
			t.Fatalf("attempt=%d claim=%+v err=%v", attempt, claim, err)
		}
		if err := repo.Retry(ctx, *claim, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if next, err := repo.ClaimNext(ctx, time.Now().UTC()); err != nil || next != nil {
			t.Fatalf("backoff ignored: %+v %v", next, err)
		}
		if err := db.Model(&model.SupportTagJob{}).Where("message_id = ?", msg.ID).Update("available_at", time.Now().Add(-time.Second)).Error; err != nil {
			t.Fatal(err)
		}
	}
	var final model.SupportTagJob
	if err := db.First(&final).Error; err != nil || final.Status != "failed" || final.Attempts != 5 {
		t.Fatalf("unbounded retries: %+v %v", final, err)
	}
	if err := db.Model(&model.SupportConversation{}).Where("id = ?", conv.ID).Update("anonymized_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.SupportTagJob{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("privacy left jobs=%d %v", count, err)
	}
}

func TestSupportTagJobPostgresFullLedger(t *testing.T) {
	db := contactPrivacyDB(t)
	workspace := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, workspace)
	conv := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspace, Subject: "Tagging", Status: "open", Source: "widget"}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatal(err)
	}
	message := createSupportTagJobMessage(t, db, conv)
	var job model.SupportTagJob
	if err := db.First(&job, "message_id = ?", message.ID).Error; err != nil || job.WorkspaceID != workspace || job.Status != "pending" {
		t.Fatalf("full ledger enqueue=%+v %v", job, err)
	}
	// Live translation and tagging each receive work from the same message transaction.
	var translationJobs int64
	if err := db.Model(&model.SupportLiveMessage{}).Where("message_id = ?", message.ID).Count(&translationJobs).Error; err != nil || translationJobs != 1 {
		t.Fatalf("translation enqueue changed: %d %v", translationJobs, err)
	}
	if err := db.Model(&conv).Update("anonymized_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	var jobs int64
	if err := db.Model(&model.SupportTagJob{}).Where("message_id = ?", message.ID).Count(&jobs).Error; err != nil || jobs != 0 {
		t.Fatalf("privacy retained queue row: %d %v", jobs, err)
	}
}
