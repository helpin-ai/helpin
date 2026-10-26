package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportSLAFirstResponsePreservesTimeAcrossAIHolds(t *testing.T) {
	targets := supportSLATargetsFromPolicy(slaPolicy(intPtr(30), nil, nil))
	conversation := slaConversation(model.SupportConversationStatusOpen)
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	handoff := timePtr(slaStart.Add(10 * time.Minute))
	replies := []supportSLAReply{reply(0, "customer"), reply(5, "ai")}
	var previous *model.SupportConversationSLARecord
	evaluate := func(minutes int, aiOwns bool) supportSLAEvaluation {
		result := evaluateSupportSLA(supportSLAInput{
			Targets: targets, Conversation: conversation, Replies: replies, Clock: clock,
			Now: slaStart.Add(time.Duration(minutes) * time.Minute), Previous: previous,
			AIInvolved: true, HumanSince: handoff, AIOwnsNow: aiOwns,
		})
		previous = &result.Record
		return result
	}
	evaluate(10, false)
	evaluate(20, true) // Ten of the thirty minutes have elapsed.
	holding := evaluate(100, true)
	if holding.Record.FirstResponseState != model.SupportSLAStatePaused || holding.Record.NextDueAt != nil {
		t.Fatalf("first response did not pause: %+v", holding)
	}
	resumed := evaluate(120, false)
	if resumed.Record.FirstResponseState != model.SupportSLAStatePending || len(resumed.NewBreaches) != 0 ||
		!resumed.Record.FirstResponseDueAt.Equal(slaStart.Add(140*time.Minute)) {
		t.Fatalf("resume should leave twenty minutes: %+v", resumed)
	}
	// Save and reload the record as a new worker would; the hold must not be
	// forgotten or counted a second time on the next evaluation.
	repo := repository.NewSupportSLARepository(newTestDB(t))
	if err := repo.SaveRecord(context.Background(), &resumed.Record); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.RecordsByConversation(context.Background(), conversation.WorkspaceID, []string{conversation.ID})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, ok := stored[conversation.ID]
	if !ok {
		t.Fatal("saved SLA record not found")
	}
	previous = &reloaded
	evaluate(125, false)
	evaluate(130, true) // Ten minutes remain for the second hand-back.
	resumed = evaluate(160, false)
	if resumed.Record.FirstResponseState != model.SupportSLAStatePending ||
		!resumed.Record.FirstResponseDueAt.Equal(slaStart.Add(170*time.Minute)) ||
		resumed.Record.FirstResponsePausedSeconds != 130*60 {
		t.Fatalf("second hold lost the remaining time: %+v", resumed)
	}
	breached := evaluate(170, false)
	if breached.Record.FirstResponseState != model.SupportSLAStateBreached || len(breached.NewBreaches) != 1 {
		t.Fatalf("target should breach at its adjusted deadline: %+v", breached)
	}
	evaluate(180, true)
	stillBreached := evaluate(200, false)
	if stillBreached.Record.FirstResponseState != model.SupportSLAStateBreached || len(stillBreached.NewBreaches) != 0 ||
		!stillBreached.Record.FirstResponseDueAt.Equal(*breached.Record.FirstResponseDueAt) {
		t.Fatalf("a later hold changed an existing breach: %+v", stillBreached)
	}
}

func TestSupportSLAFirstResponseEndsAIHoldAtReplyOrResolution(t *testing.T) {
	for _, resolve := range []bool{false, true} {
		name := "reply"
		if resolve {
			name = "resolution"
		}
		t.Run(name, func(t *testing.T) {
			in := supportSLAInput{
				Targets:      supportSLATargetsFromPolicy(slaPolicy(intPtr(30), nil, nil)),
				Conversation: slaConversation(model.SupportConversationStatusOpen),
				Replies:      []supportSLAReply{reply(0, "customer"), reply(5, "ai")},
				Clock:        newSupportSLAClock(model.SupportInboxSettings{}, false),
				Now:          slaStart.Add(20 * time.Minute), AIInvolved: true,
				HumanSince: timePtr(slaStart.Add(10 * time.Minute)), AIOwnsNow: true,
			}
			hold := evaluateSupportSLA(in)
			in.Previous, in.AIOwnsNow, in.Now = &hold.Record, false, slaStart.Add(150*time.Minute)
			wantState := model.SupportSLAStateAchieved
			if resolve {
				in.Conversation.Status = model.SupportConversationStatusResolved
				in.Conversation.ResolvedAt = timePtr(slaStart.Add(120 * time.Minute))
				wantState = model.SupportSLAStateCancelled
			} else {
				in.Replies = append(in.Replies, reply(120, "user"))
			}
			result := evaluateSupportSLA(in)
			if result.Record.FirstResponseState != wantState || len(result.NewBreaches) != 0 || result.Record.FirstResponsePausedSeconds != 100*60 {
				t.Fatalf("hold should end at the event, not the later evaluation: %+v", result)
			}
		})
	}
}

func TestSupportSLAFirstResponseAIHoldUsesBusinessHours(t *testing.T) {
	friday := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC)
	conversation := slaConversation(model.SupportConversationStatusOpen)
	conversation.CreatedAt = friday
	in := supportSLAInput{
		Targets:      supportSLATargetsFromPolicy(slaPolicy(intPtr(120), nil, nil)),
		Conversation: conversation, Replies: []supportSLAReply{{At: friday, SenderType: "customer"}},
		Clock: businessClock(t, "UTC"), Now: friday.Add(30 * time.Minute),
		AIInvolved: true, HumanSince: &friday, AIOwnsNow: true,
	}
	hold := evaluateSupportSLA(in)
	monday := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	in.Now, in.Previous, in.AIOwnsNow = monday, &hold.Record, false
	resumed := evaluateSupportSLA(in)
	if resumed.Record.FirstResponseState != model.SupportSLAStatePending || len(resumed.NewBreaches) != 0 ||
		resumed.Record.FirstResponsePausedSeconds != 90*60 || !resumed.Record.FirstResponseDueAt.Equal(monday.Add(90*time.Minute)) {
		t.Fatalf("business-hours resume should leave ninety minutes: %+v", resumed)
	}
}
