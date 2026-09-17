package service

import (
	"context"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"testing"
	"time"
)

func TestPMTriageSupportLinkReview(t *testing.T) {
	for _, scenario := range []string{"accepted", "changed message", "changed target", "archived target", "association failure"} {
		t.Run(scenario, func(t *testing.T) {
			env := newTaskTestEnv(t)
			ctx := context.Background()
			conversations := repository.NewSupportConversationRepository(env.db)
			messages := repository.NewSupportMessageRepository(env.db)
			support := NewSupportInboxService(conversations, nil, messages, nil, repository.NewCRMAssociationRepository(env.db), nil, nil, nil, nil, nil, nil, repository.NewUserRepository(env.db), nil, nil, nil)
			support.SetTaskService(env.svc)
			conversation := &model.SupportConversation{WorkspaceID: env.wsID, Subject: "Invoice export fails", Status: "open", Priority: "medium"}
			if err := conversations.Create(ctx, conversation); err != nil {
				t.Fatal(err)
			}
			task, err := env.svc.Create(ctx, model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "Fix export", TeamID: &env.teamID, TaskType: "bug"}, env.userID)
			if err != nil {
				t.Fatal(err)
			}
			conversation, err = conversations.GetByID(ctx, env.wsID, conversation.ID, "", model.RoleAdmin)
			if err != nil {
				t.Fatal(err)
			}
			_, hash := supportPMTriageEvidence(conversation, nil)
			switch scenario {
			case "changed message":
				if err := messages.Create(ctx, &model.SupportMessage{WorkspaceID: env.wsID, ConversationID: conversation.ID, SenderType: "customer", MessageType: "reply", Content: "Different issue"}); err != nil {
					t.Fatal(err)
				}
			case "changed target":
				if err := env.db.Model(&model.PMTask{}).Where("id = ?", task.Task.ID).Updates(map[string]any{"name": "Changed task", "updated_at": time.Now().Add(time.Second)}).Error; err != nil {
					t.Fatal(err)
				}
			case "archived target":
				if err := env.db.Model(&model.PMTask{}).Where("id = ?", task.Task.ID).Update("archived", true).Error; err != nil {
					t.Fatal(err)
				}
			case "association failure":
				if err := env.db.Exec(`CREATE TRIGGER reject_review_association BEFORE INSERT ON crm_associations BEGIN SELECT RAISE(ABORT, 'test association failure'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			err = support.linkReviewedConversationTask(ctx, env.wsID, conversation.ID, task.Task.ID, env.userID, hash, task.Task.UpdatedAt)
			if scenario == "accepted" && err != nil {
				t.Fatal(err)
			}
			if scenario != "accepted" && err == nil {
				t.Fatal("expected review refusal")
			}
			if scenario != "accepted" && scenario != "association failure" && !errors.Is(err, ErrPMTriageStale) {
				t.Fatalf("expected stale error, got %v", err)
			}
			linked, err := conversations.GetByID(ctx, env.wsID, conversation.ID, "", model.RoleAdmin)
			if err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := env.db.Model(&model.CRMAssociation{}).Where("from_object_id = ?", conversation.ID).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if scenario == "accepted" {
				if linked.LinkedTaskID == nil || *linked.LinkedTaskID != task.Task.ID || count != 1 {
					t.Fatal("link and association did not commit together")
				}
			} else if linked.LinkedTaskID != nil || count != 0 {
				t.Fatal("rejected review left partial link state")
			}
		})
	}
}

func TestPMTriageReviewedCreationTransaction(t *testing.T) {
	for _, scenario := range []string{"stale at transaction", "association failure"} {
		t.Run(scenario, func(t *testing.T) {
			env := newTaskTestEnv(t)
			ctx := context.Background()
			conversations := repository.NewSupportConversationRepository(env.db)
			conversation := &model.SupportConversation{WorkspaceID: env.wsID, Subject: "Export fails", Status: "open", Priority: "medium"}
			if err := conversations.Create(ctx, conversation); err != nil {
				t.Fatal(err)
			}
			conversation, err := conversations.GetByID(ctx, env.wsID, conversation.ID, "", model.RoleAdmin)
			if err != nil {
				t.Fatal(err)
			}
			_, hash := supportPMTriageEvidence(conversation, nil)
			if scenario == "stale at transaction" {
				if err := conversations.UpdateSubject(ctx, conversation.ID, "Updated requirements"); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := env.db.Exec(`CREATE TRIGGER reject_review_association BEFORE INSERT ON crm_associations BEGIN SELECT RAISE(ABORT, 'test association failure'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			_, err = env.svc.create(ctx, model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "Reviewed export fix", TeamID: &env.teamID, TaskType: "bug"}, env.userID, &supportTaskCreateReview{conversationID: conversation.ID, sourceHash: hash})
			if err == nil {
				t.Fatal("expected reviewed creation to fail")
			}
			if scenario == "stale at transaction" && !errors.Is(err, ErrPMTriageStale) {
				t.Fatalf("expected stale evidence rejection: %v", err)
			}
			var count int64
			if err := env.db.Model(&model.PMTask{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("failed reviewed creation left an orphan task")
			}
			conversation, err = conversations.GetByID(ctx, env.wsID, conversation.ID, "", model.RoleAdmin)
			if err != nil {
				t.Fatal(err)
			}
			if conversation.LinkedTaskID != nil {
				t.Fatal("failed reviewed creation left a link")
			}
		})
	}
}
