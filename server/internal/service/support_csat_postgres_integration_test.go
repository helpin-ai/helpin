//go:build integration

package service

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Concurrent first ratings (a double click, or widget and portal at once)
// must neither fail on the unique index nor record the rating twice.
func TestSubmitConversationCSATConcurrentPostgres(t *testing.T) {
	dsn := os.Getenv("SUPPORT_OPS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SUPPORT_OPS_TEST_DATABASE_URL to a migrated disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ownerID, workspaceID := uuid.NewString(), uuid.NewString()
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name) VALUES (?, ?, 'x', 'Owner')`, ownerID, ownerID+"@example.com")
	mustExec(t, db, `INSERT INTO workspaces (id, name, slug, workspace_key, owner_id) VALUES (?, 'CSAT race', ?, ?, ?)`,
		workspaceID, "csat-"+workspaceID[:8], strings.ToUpper(workspaceID[:5]), ownerID)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM support_messages WHERE workspace_id = ?`, workspaceID)
		db.Exec(`DELETE FROM support_conversations WHERE workspace_id = ?`, workspaceID)
		db.Exec(`DELETE FROM workspaces WHERE id = ?`, workspaceID)
		db.Exec(`DELETE FROM users WHERE id = ?`, ownerID)
	})
	flow := model.SupportConversationFlowStateResolvedByHuman
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 1, Subject: "Race", Status: model.SupportConversationStatusResolved, FlowState: &flow}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewSupportInboxService(repository.NewSupportConversationRepository(db), nil, repository.NewSupportMessageRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	for round := 0; round < 5; round++ {
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(rating int) {
				defer wg.Done()
				_, err := svc.SubmitConversationCSAT(ctx, model.SupportCSATSubmission{
					WorkspaceID: workspaceID, ConversationID: conversation.ID, Channel: model.SupportCSATChannelWidget, Rating: rating,
				}, model.SupportInboxSettings{CSATEnabled: true})
				errs <- err
			}(1 + (round+i)%5)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
	}
	var rows int64
	if err := db.Model(&model.SupportCSATResponse{}).Where("conversation_id = ?", conversation.ID).Count(&rows).Error; err != nil || rows != 1 {
		t.Fatalf("rating rows = %d (%v)", rows, err)
	}
}
