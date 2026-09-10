package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/model"
	"strings"
	"testing"
	"time"
)

func TestSupportFollowUpTwoRemindersThenAssumedClosure(t *testing.T) {
	svc, db, run, e, d := setupFollowUpTest(t)
	mustExec(t, db, `UPDATE support_ai_follow_ups SET sequence_version=2,second_delay_hours=24,close_hours=1`)
	d.ClosureNotice = "We haven't heard back, so we'll close this conversation shortly. Reply anytime to reopen it."
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	first, _ := svc.repo.Latest(context.Background(), "ws", "conv")
	if first.CloseAt != nil || first.DueAt.Sub(*first.SentAt) != 24*time.Hour {
		t.Fatalf("wrong first stage: %+v", first)
	}
	var message model.SupportMessage
	db.First(&message, "id = ?", *first.SentMessageID)
	if strings.Contains(message.Content, "close") {
		t.Fatal("first question contained closing notice")
	}
	mustExec(t, db, `UPDATE support_conversations SET last_public_message_id=?`, *first.SentMessageID)
	if err := svc.process(context.Background(), *first, first.DueAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	before, _ := svc.repo.Latest(context.Background(), "ws", "conv")
	if before.SecondSentAt != nil {
		t.Fatal("sent second reminder early")
	}
	if err := svc.process(context.Background(), *first, first.DueAt); err != nil {
		t.Fatal(err)
	}
	second, _ := svc.repo.Latest(context.Background(), "ws", "conv")
	if second.SecondSentAt == nil || second.CloseAt == nil || second.CloseAt.Sub(*second.SecondSentAt) != time.Hour {
		t.Fatalf("wrong second stage: %+v", second)
	}
	mustExec(t, db, `UPDATE support_conversations SET last_public_message_id=?`, *second.SecondMessageID)
	if err := svc.process(context.Background(), *second, second.CloseAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	var conv model.SupportConversation
	db.First(&conv, "id = 'conv'")
	if conv.Status != "open" {
		t.Fatal("closed before internal wait elapsed")
	}
	if err := svc.process(context.Background(), *second, *second.CloseAt); err != nil {
		t.Fatal(err)
	}
	if err := svc.process(context.Background(), *second, *second.CloseAt); err != nil {
		t.Fatal(err)
	}
	db.First(&conv, "id = 'conv'")
	if conv.Status != "resolved" || derefString(conv.AIResolutionType) != "assumed" {
		t.Fatal("did not resolve as assumed")
	}
	var count int64
	db.Model(&model.SupportMessage{}).Where("id <> 'source'").Count(&count)
	if count != 2 {
		t.Fatalf("sent %d reminders", count)
	}
}

func TestSupportFollowUpHandoffMakesWaitingConversationActionable(t *testing.T) {
	svc, db, run, e, d := setupFollowUpTest(t)
	mustExec(t, db, `UPDATE support_conversations SET status='waiting_on_customer'`)
	d.Action = "handoff"
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	var conv model.SupportConversation
	db.First(&conv, "id = 'conv'")
	if conv.Status != "open" || !conv.CustomerAwaitingResponse {
		t.Fatal("handoff did not make conversation actionable")
	}
}

func TestSupportFollowUpCustomerReplyCancelsEitherStage(t *testing.T) {
	for _, second := range []bool{false, true} {
		t.Run(map[bool]string{false: "first", true: "second"}[second], func(t *testing.T) {
			svc, db, run, e, d := setupFollowUpTest(t)
			mustExec(t, db, `UPDATE support_ai_follow_ups SET sequence_version=2,second_delay_hours=24,close_hours=1`)
			d.ClosureNotice = "We'll close this conversation shortly. Reply anytime to reopen it."
			if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
				t.Fatal(err)
			}
			row, _ := svc.repo.Latest(context.Background(), "ws", "conv")
			mustExec(t, db, `UPDATE support_conversations SET last_public_message_id=?`, *row.SentMessageID)
			if second {
				if err := svc.process(context.Background(), *row, row.DueAt); err != nil {
					t.Fatal(err)
				}
				row, _ = svc.repo.Latest(context.Background(), "ws", "conv")
			}
			mustExec(t, db, `UPDATE support_conversations SET last_public_message_id='customer-reply',last_public_sender_type='customer'`)
			if err := svc.process(context.Background(), *row, row.DueAt); err != nil {
				t.Fatal(err)
			}
			row, _ = svc.repo.Latest(context.Background(), "ws", "conv")
			if row.Status != "cancelled" {
				t.Fatalf("status %s", row.Status)
			}
		})
	}
}

func TestSupportFollowUpNewSequenceRejectsCustomerDeadline(t *testing.T) {
	svc, db, run, e, d := setupFollowUpTest(t)
	mustExec(t, db, `UPDATE support_ai_follow_ups SET sequence_version=2,second_delay_hours=24,close_hours=1`)
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err == nil {
		t.Fatal("accepted a customer-visible deadline")
	}
}

func TestSupportFollowUpRejectsDeadlinesInEitherReminder(t *testing.T) {
	for _, tc := range []struct{ question, notice string }{
		{"Did it work? We will close in 1 hour.", "We'll close shortly. Reply anytime."},
		{"Did it work?", "We'll close by Friday. Reply anytime."},
		{"Did it work?", "We'll close at noon. Reply anytime."},
	} {
		if err := validateDeadlineFreeFollowUp(supportFollowUpDecision{Question: tc.question, ClosureNotice: tc.notice}); err == nil {
			t.Fatalf("accepted deadline: %+v", tc)
		}
	}
}

func TestSupportFollowUpUnconfirmedEmailDoesNotAdvance(t *testing.T) {
	svc, db, run, e, d := setupFollowUpTest(t)
	mustExec(t, db, `UPDATE support_ai_follow_ups SET sequence_version=2,second_delay_hours=24,close_hours=1`)
	d.ClosureNotice = "We'll close this conversation shortly. Reply anytime to reopen it."
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	row, _ := svc.repo.Latest(context.Background(), "ws", "conv")
	mustExec(t, db, `UPDATE support_conversations SET channel='email',last_public_message_id=?`, *row.SentMessageID)
	if err := svc.process(context.Background(), *row, row.DueAt); err != nil {
		t.Fatal(err)
	}
	row, _ = svc.repo.Latest(context.Background(), "ws", "conv")
	var conv model.SupportConversation
	db.First(&conv, "id = 'conv'")
	if row.SecondSentAt != nil || row.Status != "failed" || conv.Status != "open" || !conv.CustomerAwaitingResponse {
		t.Fatal("unconfirmed email did not stop sequence and hand off")
	}
}
