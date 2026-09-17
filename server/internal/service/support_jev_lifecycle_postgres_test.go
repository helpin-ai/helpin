//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestJevFollowUpPostgresFences(t *testing.T) {
	for _, scenario := range []string{"handoff", "confirmed", "waiting", "edit during provider", "reply during provider", "takeover during provider"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, _, row, _ := setupFollowUpPostgres(t)
			if err := db.AutoMigrate(&model.SupportConversationTriageEvent{}, &model.AIExecutionUsage{}); err != nil {
				t.Fatal(err)
			}
			p := &lifecycleJev{choice: "team_owes_work", probability: .99}
			want := "handoff"
			switch scenario {
			case "confirmed":
				p.choice = "confirmed_resolved"
				want = "skipped"
			case "waiting":
				p.choice = "waiting_customer"
				want = "assessing"
			case "edit during provider":
				want = "cancelled"
				p.during = func() {
					mustExec(t, db, `UPDATE support_messages SET content='Actually the issue is fixed' WHERE id=?`, row.SourceMessageID)
				}
			case "reply during provider":
				want = "cancelled"
				p.during = func() {
					if err := db.Create(&model.SupportMessage{WorkspaceID: row.WorkspaceID, ConversationID: row.ConversationID, SenderType: "customer", MessageType: "reply", Content: "New details"}).Error; err != nil {
						t.Fatal(err)
					}
				}
			case "takeover during provider":
				want = "cancelled"
				p.during = func() {
					mustExec(t, db, `UPDATE support_conversations SET human_takeover=true,ai_control_version=ai_control_version+1 WHERE id=?`, row.ConversationID)
				}
			}
			tags := NewSupportTagService(repository.NewSupportTagRepository(db), repository.NewSupportConversationRepository(db), nil)
			jev, err := NewSupportJevService(SupportJevConfig{RoutingMode: "off", TagsMode: "off", RoutingThreshold: .9, TagThreshold: .95, HandoffThreshold: .95, FollowUpThreshold: .95, DailyLimit: 100}, p, repository.NewSupportJevRepository(db), repository.NewAIExecutionUsageRepository(db), tags)
			if err != nil {
				t.Fatal(err)
			}
			svc.chat.SetJevService(jev)
			var conv model.SupportConversation
			if err := db.First(&conv, "id = ?", row.ConversationID).Error; err != nil {
				t.Fatal(err)
			}
			_, _, err = svc.assessJevFollowUp(context.Background(), row, &conv, svc.now())
			if err != nil {
				t.Fatal(err)
			}
			latest, err := svc.repo.Latest(context.Background(), row.WorkspaceID, row.ConversationID)
			if err != nil || latest.Status != want || p.calls != 1 {
				t.Fatalf("episode=%+v/%v calls=%d", latest, err, p.calls)
			}
			if err := db.First(&conv, "id = ?", row.ConversationID).Error; err != nil {
				t.Fatal(err)
			}
			if scenario == "handoff" && (derefString(conv.AIState) != "escalated" || !conv.CustomerAwaitingResponse) {
				t.Fatalf("handoff not actionable: %+v", conv)
			}
			var usage int64
			if err := db.Model(&model.AIExecutionUsage{}).Count(&usage).Error; err != nil || usage != 1 {
				t.Fatalf("usage=%d/%v", usage, err)
			}
		})
	}
}
