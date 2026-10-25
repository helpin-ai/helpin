package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func slaSwitch(policy model.SupportSLAPolicy, conversation model.SupportConversation, replies []supportSLAReply, now time.Time, previous model.SupportConversationSLARecord) supportSLAEvaluation {
	return evaluateSupportSLA(supportSLAInput{
		Targets: supportSLATargetsFromPolicy(policy), Conversation: conversation, Replies: replies,
		Clock: newSupportSLAClock(model.SupportInboxSettings{}, false), Now: now,
		Previous: &previous, Switched: true, AppliedAt: timePtr(now),
	})
}

func TestEvaluateSupportSLASwitchKeepsAchievedFirstResponse(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)
	replies := []supportSLAReply{reply(0, "customer"), reply(120, "user")}
	medium := evalSLAPolicy(slaPolicy(intPtr(240), nil, nil), open, replies, clock, slaStart.Add(3*time.Hour), nil)
	if medium.Record.FirstResponseState != model.SupportSLAStateAchieved {
		t.Fatalf("met under 4h: %+v", medium.Record)
	}
	urgent := slaPolicy(intPtr(60), nil, nil)
	urgent.ID = "urgent"
	switched := slaSwitch(urgent, open, replies, slaStart.Add(4*time.Hour), medium.Record)
	if switched.Record.FirstResponseState != model.SupportSLAStateAchieved || len(switched.NewBreaches) != 0 || *switched.Record.PolicyID != "urgent" {
		t.Fatalf("raising priority re-judged an achieved first response: %+v %v", switched.Record, switched.NewBreaches)
	}
}

func TestEvaluateSupportSLASwitchRejudgesPendingTargets(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)
	replies := []supportSLAReply{reply(0, "customer")}
	medium := evalSLAPolicy(slaPolicy(intPtr(240), nil, nil), open, replies, clock, slaStart.Add(90*time.Minute), nil)
	urgent := slaPolicy(intPtr(60), nil, nil)
	urgent.ID = "urgent"
	switched := slaSwitch(urgent, open, replies, slaStart.Add(90*time.Minute), medium.Record)
	if switched.Record.FirstResponseState != model.SupportSLAStateBreached || len(switched.NewBreaches) != 1 ||
		!switched.Record.FirstResponseDueAt.Equal(slaStart.Add(time.Hour)) {
		t.Fatalf("an unanswered message must meet the new, tighter target: %+v %v", switched.Record, switched.NewBreaches)
	}
}

func TestEvaluateSupportSLASwitchKeepsNextResponseHistory(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)
	replies := []supportSLAReply{
		reply(0, "customer"), reply(5, "user"),
		reply(10, "customer"), reply(50, "user"), // 40m: missed under 30m, met under 60m
		reply(60, "customer"), // open, due at 90m under 30m
	}
	strict := slaPolicy(nil, intPtr(30), nil)
	before := evalSLAPolicy(strict, open, replies, clock, slaStart.Add(100*time.Minute), nil)
	if before.Record.NextResponseBreaches != 2 || !before.Record.Breached {
		t.Fatalf("two missed cycles under 30m: %+v", before.Record)
	}
	loose := slaPolicy(nil, intPtr(240), nil)
	loose.ID = "loose"
	switched := slaSwitch(loose, open, replies, slaStart.Add(110*time.Minute), before.Record)
	r := switched.Record
	if r.NextResponseBreaches != 2 || len(switched.NewBreaches) != 0 || r.NextResponseState != model.SupportSLAStateBreached || !r.Breached {
		t.Fatalf("a looser policy erased recorded breaches: %+v %v", r, switched.NewBreaches)
	}
	// The open cycle ends after the switch, within the loose target: still
	// one missed cycle from before plus the one already recorded.
	answered := append(append([]supportSLAReply{}, replies...), reply(120, "user"))
	later := evalSLAPolicy(loose, open, answered, clock, slaStart.Add(130*time.Minute), &r)
	if later.Record.NextResponseBreaches != 2 || len(later.NewBreaches) != 0 || later.Record.Breached {
		t.Fatalf("answering after the switch changed recorded breaches: %+v %v", later.Record, later.NewBreaches)
	}
	// A new cycle after the switch is judged by the loose target.
	next := append(answered, reply(140, "customer"), reply(200, "user"))
	met := evalSLAPolicy(loose, open, next, clock, slaStart.Add(210*time.Minute), &later.Record)
	if met.Record.NextResponseBreaches != 2 || len(met.NewBreaches) != 0 {
		t.Fatalf("cycle after the switch: %+v %v", met.Record, met.NewBreaches)
	}
	missed := append(answered, reply(140, "customer"))
	overdue := evalSLAPolicy(loose, open, missed, clock, slaStart.Add(400*time.Minute), &later.Record)
	if overdue.Record.NextResponseBreaches != 3 || len(overdue.NewBreaches) != 1 {
		t.Fatalf("new breach after the switch: %+v %v", overdue.Record, overdue.NewBreaches)
	}
}

func TestEvaluateSupportSLASwitchJudgesOpenCycleByNewTarget(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)
	replies := []supportSLAReply{reply(0, "customer"), reply(5, "user"), reply(10, "customer")}
	loose := slaPolicy(nil, intPtr(240), nil)
	before := evalSLAPolicy(loose, open, replies, clock, slaStart.Add(20*time.Minute), nil)
	urgent := slaPolicy(nil, intPtr(15), nil)
	urgent.ID = "urgent"
	switched := slaSwitch(urgent, open, replies, slaStart.Add(30*time.Minute), before.Record)
	if switched.Record.NextResponseBreaches != 1 || len(switched.NewBreaches) != 1 || !switched.Record.NextResponseDueAt.Equal(slaStart.Add(25*time.Minute)) {
		t.Fatalf("open cycle under the tighter target: %+v %v", switched.Record, switched.NewBreaches)
	}
}

func TestEvaluateSupportSLADroppedMetricKeepsResult(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)
	replies := []supportSLAReply{reply(0, "customer"), reply(90, "user")}
	before := evalSLAPolicy(slaPolicy(intPtr(60), nil, nil), open, replies, clock, slaStart.Add(2*time.Hour), nil)
	resolutionOnly := slaPolicy(nil, nil, intPtr(600))
	resolutionOnly.ID = "resolution"
	switched := slaSwitch(resolutionOnly, open, replies, slaStart.Add(2*time.Hour), before.Record)
	if switched.Record.FirstResponseState != model.SupportSLAStateBreached || switched.Record.FirstResponseAchievedAt == nil || len(switched.NewBreaches) != 0 {
		t.Fatalf("a policy without a first response target dropped its result: %+v", switched.Record)
	}
}

func TestEvaluateSupportSLAReopenKeepsResolution(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	policy := slaPolicy(nil, nil, intPtr(24*60))
	resolved := slaConversation(model.SupportConversationStatusResolved)
	resolved.ResolvedAt = timePtr(slaStart.Add(2 * time.Hour))
	done := evalSLAPolicy(policy, resolved, nil, clock, slaStart.Add(2*time.Hour), nil)
	if done.Record.ResolutionState != model.SupportSLAStateAchieved {
		t.Fatalf("resolved in time: %+v", done.Record)
	}
	reopened := slaConversation(model.SupportConversationStatusOpen)
	again := evalSLAPolicy(policy, reopened, []supportSLAReply{reply(3*24*60, "customer")}, clock, slaStart.Add(72*time.Hour), &done.Record)
	if again.Record.ResolutionState != model.SupportSLAStateAchieved || len(again.NewBreaches) != 0 || again.Record.NextDueAt != nil {
		t.Fatalf("reopening re-judged the first resolution: %+v %v", again.Record, again.NewBreaches)
	}
}

func aiConversation() model.SupportConversation {
	conversation := slaConversation(model.SupportConversationStatusOpen)
	flow := model.SupportConversationFlowStateAIHandling
	state := "pending"
	conversation.FlowState, conversation.AIState = &flow, &state
	return conversation
}

func TestEvaluateSupportSLAWaitsForAIHandoff(t *testing.T) {
	policy := slaPolicy(intPtr(5), nil, intPtr(60))
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	conversation := aiConversation()
	replies := []supportSLAReply{reply(0, "customer"), reply(1, "ai")}
	involved, humanSince, aiOwns := supportSLAHandoff(conversation, nil, nil, slaStart)
	if !involved || humanSince != nil || !aiOwns {
		t.Fatalf("AI-handled conversation: %v %v %v", involved, humanSince, aiOwns)
	}
	holding := evaluateSupportSLA(supportSLAInput{Targets: supportSLATargetsFromPolicy(policy), Conversation: conversation, Replies: replies, Clock: clock,
		Now: slaStart.Add(3 * time.Hour), AIInvolved: true, AIOwnsNow: true})
	r := holding.Record
	if r.FirstResponseState != model.SupportSLAStatePaused || r.ResolutionState != model.SupportSLAStatePaused || r.NextDueAt != nil || len(holding.NewBreaches) != 0 {
		t.Fatalf("targets ran while the AI handled the conversation: %+v %v", r, holding.NewBreaches)
	}

	// The AI resolves it alone: nothing to measure.
	resolved := conversation
	resolved.Status, resolved.ResolvedAt = model.SupportConversationStatusResolved, timePtr(slaStart.Add(4*time.Hour))
	flow := model.SupportConversationFlowStateResolvedByAI
	resolved.FlowState = &flow
	alone := evaluateSupportSLA(supportSLAInput{Targets: supportSLATargetsFromPolicy(policy), Conversation: resolved, Replies: replies, Clock: clock,
		Now: slaStart.Add(4 * time.Hour), Previous: &r, AIInvolved: true})
	if alone.Record.FirstResponseState != model.SupportSLAStateCancelled || alone.Record.ResolutionState != model.SupportSLAStateCancelled || len(alone.NewBreaches) != 0 {
		t.Fatalf("AI resolution counted against the team: %+v", alone.Record)
	}

	// Escalated after 30 minutes: clocks start at the handoff.
	escalated := conversation
	escalatedState, waiting := "escalated", model.SupportConversationFlowStateWaitingForHuman
	escalated.AIState, escalated.FlowState, escalated.AIEscalatedAt = &escalatedState, &waiting, timePtr(slaStart.Add(30*time.Minute))
	involved, humanSince, aiOwns = supportSLAHandoff(escalated, nil, &r, slaStart.Add(31*time.Minute))
	if !involved || humanSince == nil || !humanSince.Equal(slaStart.Add(30*time.Minute)) || aiOwns {
		t.Fatalf("handoff: %v %v %v", involved, humanSince, aiOwns)
	}
	handedOff := evaluateSupportSLA(supportSLAInput{Targets: supportSLATargetsFromPolicy(policy), Conversation: escalated, Replies: replies, Clock: clock,
		Now: slaStart.Add(31 * time.Minute), Previous: &r, AIInvolved: true, HumanSince: humanSince})
	if !handedOff.Record.FirstResponseDueAt.Equal(slaStart.Add(35*time.Minute)) || !handedOff.Record.ResolutionDueAt.Equal(slaStart.Add(90*time.Minute)) ||
		handedOff.Record.FirstResponseState != model.SupportSLAStatePending || len(handedOff.NewBreaches) != 0 {
		t.Fatalf("clocks after the handoff: %+v", handedOff.Record)
	}
}

func TestSupportSLAHandoffWithoutAI(t *testing.T) {
	conversation := slaConversation(model.SupportConversationStatusOpen)
	if involved, humanSince, aiOwns := supportSLAHandoff(conversation, nil, nil, slaStart); involved || humanSince != nil || aiOwns {
		t.Fatalf("team-only conversation: %v %v %v", involved, humanSince, aiOwns)
	}
}

func TestDetachSupportSLARecord(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)
	replies := []supportSLAReply{reply(0, "customer"), reply(90, "user"), reply(100, "customer")}
	before := evalSLAPolicy(slaPolicy(intPtr(60), intPtr(600), intPtr(6000)), open, replies, clock, slaStart.Add(2*time.Hour), nil)
	detached := detachSupportSLARecord(before.Record, slaStart.Add(2*time.Hour))
	if detached.FirstResponseState != model.SupportSLAStateBreached || detached.NextResponseState != model.SupportSLAStateCancelled ||
		detached.ResolutionState != model.SupportSLAStateCancelled || detached.NextDueAt != nil || detached.WakeAt != nil || detached.Breached || detached.DetachedAt == nil {
		t.Fatalf("detached: %+v", detached)
	}
}
