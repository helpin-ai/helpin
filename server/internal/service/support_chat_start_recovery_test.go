package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestSupportLaunchErrorReleasesAttemptForImmediateRetry(t *testing.T) {
	chat, _, db, conv, _, _ := setupSupportProgressTest(t)
	mustExec(t, db, `DELETE FROM ai_message_processing`)
	mustExec(t, db, `UPDATE support_conversations SET ai_active_run_id=NULL WHERE id='conv'`)
	if err := db.Callback().Query().Before("gorm:query").Register("fail_history", func(tx *gorm.DB) {
		if tx.Statement.Table == "support_messages" {
			tx.AddError(errors.New("transcript temporarily unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Query().Remove("fail_history")
	msg := &model.SupportMessage{ID: "source", WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: "My publishing failed"}
	if err := chat.HandleVisitorMessage(context.Background(), conv.WorkspaceID, conv.ID, msg); err == nil {
		t.Fatal("expected transcript load failure")
	}
	var row model.AIMessageProcessing
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "failed" {
		t.Fatalf("failed launch left status %q; immediate retry would be acknowledged without processing", row.Status)
	}
	retried, ok, err := chat.processingRepo.BeginAttempt(context.Background(), "ws", "source", "conv")
	if err != nil || !ok || retried.Attempts != 2 {
		t.Fatalf("immediate retry: row=%+v claimed=%v err=%v", retried, ok, err)
	}
}

func TestSupportSweepRecoversUnstartedTurn(t *testing.T) {
	for _, tc := range []struct {
		name, status                 string
		recent, active, human, newer bool
		change                       string
		want                         bool
	}{
		{name: "dispatch never claimed", status: "missing", want: true},
		{name: "stale processing", status: "processing", want: true},
		{name: "failed retry", status: "failed", want: true},
		{name: "parked without run", status: "deferred", want: true},
		{name: "fresh launch", status: "processing", recent: true},
		{name: "existing run", status: "processing", active: true},
		{name: "human takeover", status: "processing", human: true},
		{name: "newer message", status: "processing", newer: true},
		{name: "completed", status: "completed"},
		{name: "paused", status: "processing", change: "ai_paused_at=CURRENT_TIMESTAMP"},
		{name: "merged", status: "processing", change: "merged_into_id='destination'"},
		{name: "anonymized", status: "processing", change: "anonymized_at=CURRENT_TIMESTAMP"},
		{name: "resolved", status: "processing", change: "status='resolved'"},
		{name: "assigned teammate", status: "processing", change: "assigned_user_id='human'"},
		{name: "progress in flight", status: "waiting_for_result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			chat, commands, db, _, _, _ := setupSupportProgressTest(t)
			chat.runRepo = commands.agentRunRepo
			chat.supportAIService.handoffRepo = repository.NewAgentHandoffRepository(db)
			mustExec(t, db, `ALTER TABLE agent_runs ADD COLUMN updated_at DATETIME`)
			mustExec(t, db, `ALTER TABLE agent_runs ADD COLUMN created_at DATETIME`)
			mustExec(t, db, `ALTER TABLE agent_runs ADD COLUMN pause_reason TEXT`)
			mustExec(t, db, `DELETE FROM command_bar_plans`)
			if !tc.active {
				mustExec(t, db, `UPDATE support_conversations SET ai_active_run_id=NULL, ai_state='pending' WHERE id='conv'`)
			}
			at := time.Now().Add(-3 * time.Minute)
			if tc.recent {
				at = time.Now()
			}
			mustExec(t, db, `UPDATE ai_message_processing SET status=?,updated_at=?`, tc.status, at)
			mustExec(t, db, `UPDATE support_messages SET created_at=? WHERE id='source'`, at)
			if tc.status == "missing" {
				mustExec(t, db, `DELETE FROM ai_message_processing`)
			}
			if tc.change != "" {
				mustExec(t, db, "UPDATE support_conversations SET "+tc.change+" WHERE id='conv'")
			}
			if tc.human {
				mustExec(t, db, `UPDATE support_conversations SET human_takeover=true WHERE id='conv'`)
			}
			if tc.newer {
				mustExec(t, db, `UPDATE support_conversations SET last_public_message_id='newer' WHERE id='conv'`)
			}
			if err := chat.SweepSupportChat(ctx, time.Second, 10); err != nil {
				t.Fatal(err)
			}
			conv, err := chat.conversationRepo.GetByID(ctx, "ws", "conv", "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if got := derefString(conv.AIState) == "escalated"; got != tc.want {
				t.Fatalf("handoff=%v want %v", got, tc.want)
			}
			if tc.want {
				if err := chat.SweepSupportChat(ctx, time.Second, 10); err != nil {
					t.Fatal(err)
				}
				var count int64
				db.Model(&model.SupportMessage{}).Where("is_internal = ? AND message_type = ?", true, "note").Count(&count)
				if count != 1 {
					t.Fatalf("handover notes=%d want 1", count)
				}
			}
		})
	}
}

func TestSupportUnstartedHandoffFencePreservesRun(t *testing.T) {
	chat, _, db, conv, _, _ := setupSupportProgressTest(t)
	chat.supportAIService.handoffRepo = repository.NewAgentHandoffRepository(db)
	err := chat.supportAIService.EscalateToHumanForMessageWithIssue(context.Background(), "ws", "conv", "source", "ai_pipeline_error", "", "", SupportHandoffBrief{ExpectedNoRun: true, ExpectedControlVersion: &conv.AIControlVersion, ExpectedMessageID: "source"})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := chat.conversationRepo.GetByID(context.Background(), "ws", "conv", "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if derefString(saved.AIState) == "escalated" || derefString(saved.AIActiveRunID) != "run" {
		t.Fatalf("recovery took ownership from new run: %+v", saved)
	}
}
