package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func weekdayHours(start, end string) map[string]model.BusinessHoursDay {
	schedule := map[string]model.BusinessHoursDay{}
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri"} {
		schedule[day] = model.BusinessHoursDay{Start: start, End: end, Enabled: true}
	}
	return schedule
}

func businessClock(t *testing.T, tz string) supportSLAClock {
	t.Helper()
	clock := newSupportSLAClock(model.SupportInboxSettings{
		BusinessHoursEnabled: true, BusinessHoursTimezone: tz, BusinessHoursSchedule: weekdayHours("09:00", "17:00"),
	}, true)
	if clock.calendar() {
		t.Fatal("expected a business-hours clock")
	}
	return clock
}

func TestSupportSLAClockCalendar(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{}, true)
	start := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC)
	if got := clock.Add(start, 90*time.Minute); !got.Equal(start.Add(90 * time.Minute)) {
		t.Fatalf("calendar add = %v", got)
	}
	if got := clock.Between(start, start.Add(time.Hour)); got != time.Hour {
		t.Fatalf("calendar between = %v", got)
	}
	// Business hours requested but none open: calendar time, never stuck.
	closed := newSupportSLAClock(model.SupportInboxSettings{BusinessHoursEnabled: true, BusinessHoursTimezone: "UTC"}, true)
	if !closed.calendar() {
		t.Fatal("a schedule with no open day must fall back to calendar time")
	}
}

func TestSupportSLAClockBusinessHours(t *testing.T) {
	clock := businessClock(t, "UTC")
	friday4pm := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC) // Friday
	// One hour left on Friday, the rest Monday morning.
	if got, want := clock.Add(friday4pm, 2*time.Hour), time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("add over weekend = %v, want %v", got, want)
	}
	// Starting before opening counts from 09:00.
	early := time.Date(2026, 10, 26, 6, 0, 0, 0, time.UTC)
	if got, want := clock.Add(early, 30*time.Minute), time.Date(2026, 10, 26, 9, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("add before opening = %v, want %v", got, want)
	}
	// Starting on Saturday counts from Monday.
	saturday := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	if got, want := clock.Add(saturday, time.Hour), time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("add from weekend = %v, want %v", got, want)
	}
	// Exactly one business day.
	if got, want := clock.Add(time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC), 8*time.Hour), time.Date(2026, 10, 26, 17, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("add full day = %v, want %v", got, want)
	}
	if got := clock.Between(friday4pm, time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)); got != 2*time.Hour {
		t.Fatalf("between over weekend = %v", got)
	}
	if got := clock.Between(saturday, saturday.Add(5*time.Hour)); got != 0 {
		t.Fatalf("weekend counts %v", got)
	}
}

func TestSupportSLAClockDaylightSaving(t *testing.T) {
	clock := businessClock(t, "Europe/Berlin")
	// Clocks go back on Sunday 2026-10-25; Monday still opens at 09:00 local.
	friday := time.Date(2026, 10, 23, 16, 0, 0, 0, clock.location)
	got := clock.Add(friday, 2*time.Hour).In(clock.location)
	if got.Weekday() != time.Monday || got.Hour() != 10 || got.Minute() != 0 {
		t.Fatalf("across DST = %v", got)
	}
}

func slaPolicy(first, next, resolution *int) model.SupportSLAPolicy {
	return model.SupportSLAPolicy{
		ID: "policy", Name: "Standard", Enabled: true, PauseWhileWaiting: true,
		FirstResponseMinutes: first, NextResponseMinutes: next, ResolutionMinutes: resolution,
		ActiveSince: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

var slaStart = time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)

func slaConversation(status string) model.SupportConversation {
	return model.SupportConversation{ID: "c", WorkspaceID: "w", Status: status, Priority: "high", Channel: "widget", CreatedAt: slaStart}
}

func reply(minutes int, sender string) supportSLAReply {
	return supportSLAReply{At: slaStart.Add(time.Duration(minutes) * time.Minute), SenderType: sender}
}

func TestEvaluateSupportSLAFirstResponse(t *testing.T) {
	policy := slaPolicy(intPtr(30), nil, nil)
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)

	pending := evalSLAPolicy(policy, open, []supportSLAReply{reply(0, "customer")}, clock, slaStart.Add(10*time.Minute), nil)
	r := pending.Record
	if r.FirstResponseState != model.SupportSLAStatePending || !r.FirstResponseDueAt.Equal(slaStart.Add(30*time.Minute)) ||
		!r.WakeAt.Equal(*r.FirstResponseDueAt) || r.Breached || len(pending.NewBreaches) != 0 {
		t.Fatalf("pending: %+v %v", r, pending.NewBreaches)
	}

	// AI replies do not meet the target.
	overdue := evalSLAPolicy(policy, open, []supportSLAReply{reply(0, "customer"), reply(1, "ai")}, clock, slaStart.Add(31*time.Minute), &r)
	if overdue.Record.FirstResponseState != model.SupportSLAStateBreached || !overdue.Record.Breached || overdue.Record.WakeAt != nil ||
		!overdue.Record.NextDueAt.Equal(slaStart.Add(30*time.Minute)) || len(overdue.NewBreaches) != 1 {
		t.Fatalf("overdue: %+v %v", overdue.Record, overdue.NewBreaches)
	}
	// Already recorded: no second breach event; a late reply keeps the breach.
	late := evalSLAPolicy(policy, open, []supportSLAReply{reply(0, "customer"), reply(45, "user")}, clock, slaStart.Add(46*time.Minute), &overdue.Record)
	if late.Record.FirstResponseState != model.SupportSLAStateBreached || late.Record.Breached || len(late.NewBreaches) != 0 ||
		!late.Record.FirstResponseAchievedAt.Equal(slaStart.Add(45*time.Minute)) {
		t.Fatalf("late reply: %+v %v", late.Record, late.NewBreaches)
	}

	met := evalSLAPolicy(policy, open, []supportSLAReply{reply(0, "customer"), reply(20, "user")}, clock, slaStart.Add(2*time.Hour), nil)
	if met.Record.FirstResponseState != model.SupportSLAStateAchieved || met.Record.NextDueAt != nil {
		t.Fatalf("met: %+v", met.Record)
	}

	outbound := evalSLAPolicy(policy, open, []supportSLAReply{reply(0, "user"), reply(5, "customer")}, clock, slaStart.Add(2*time.Hour), nil)
	if outbound.Record.FirstResponseState != "" {
		t.Fatalf("teammate-started conversation got a first response target: %+v", outbound.Record)
	}

	resolved := slaConversation(model.SupportConversationStatusResolved)
	resolved.ResolvedAt = timePtr(slaStart.Add(10 * time.Minute))
	cancelled := evalSLAPolicy(policy, resolved, []supportSLAReply{reply(0, "customer"), reply(1, "ai")}, clock, slaStart.Add(2*time.Hour), nil)
	if cancelled.Record.FirstResponseState != model.SupportSLAStateCancelled || len(cancelled.NewBreaches) != 0 {
		t.Fatalf("resolved by AI before the target: %+v", cancelled.Record)
	}
}

func TestEvaluateSupportSLANextResponseCycles(t *testing.T) {
	policy := slaPolicy(nil, intPtr(60), nil)
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)
	replies := []supportSLAReply{
		reply(0, "customer"), reply(5, "user"),
		reply(10, "customer"), reply(90, "user"), // 80m: missed
		reply(100, "customer"), reply(120, "user"), // 20m: met
		reply(130, "customer"), // open cycle due at 190m
	}
	pending := evalSLAPolicy(policy, open, replies, clock, slaStart.Add(150*time.Minute), nil)
	r := pending.Record
	if r.NextResponseState != model.SupportSLAStatePending || r.NextResponseBreaches != 1 ||
		!r.NextResponseDueAt.Equal(slaStart.Add(190*time.Minute)) || !r.NextResponseBreachedAt.Equal(slaStart.Add(70*time.Minute)) ||
		len(pending.NewBreaches) != 1 {
		t.Fatalf("pending cycle: %+v %v", r, pending.NewBreaches)
	}
	overdue := evalSLAPolicy(policy, open, replies, clock, slaStart.Add(200*time.Minute), &r)
	if overdue.Record.NextResponseState != model.SupportSLAStateBreached || overdue.Record.NextResponseBreaches != 2 ||
		!overdue.Record.Breached || len(overdue.NewBreaches) != 1 {
		t.Fatalf("overdue cycle: %+v %v", overdue.Record, overdue.NewBreaches)
	}
	again := evalSLAPolicy(policy, open, replies, clock, slaStart.Add(210*time.Minute), &overdue.Record)
	if len(again.NewBreaches) != 0 {
		t.Fatalf("re-evaluation repeated the breach: %v", again.NewBreaches)
	}
	noTeammate := evalSLAPolicy(policy, open, []supportSLAReply{reply(0, "customer"), reply(5, "customer")}, clock, slaStart.Add(5*time.Hour), nil)
	if noTeammate.Record.NextResponseState != "" {
		t.Fatalf("next response started before any teammate reply: %+v", noTeammate.Record)
	}
}

func TestEvaluateSupportSLAResolutionPauses(t *testing.T) {
	policy := slaPolicy(nil, nil, intPtr(120))
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	open := slaConversation(model.SupportConversationStatusOpen)

	first := evalSLAPolicy(policy, open, nil, clock, slaStart.Add(30*time.Minute), nil)
	if first.Record.ResolutionState != model.SupportSLAStatePending || !first.Record.ResolutionDueAt.Equal(slaStart.Add(2*time.Hour)) {
		t.Fatalf("pending: %+v", first.Record)
	}
	waiting := slaConversation(model.SupportConversationStatusWaitingOnCustomer)
	paused := evalSLAPolicy(policy, waiting, nil, clock, slaStart.Add(time.Hour), &first.Record)
	if paused.Record.ResolutionState != model.SupportSLAStatePaused || paused.Record.PausedSince == nil || paused.Record.ResolutionDueAt != nil || paused.Record.WakeAt != nil {
		t.Fatalf("paused: %+v", paused.Record)
	}
	// Three hours waiting on the customer push the due time back three hours.
	resumed := evalSLAPolicy(policy, open, nil, clock, slaStart.Add(4*time.Hour), &paused.Record)
	if resumed.Record.ResolutionState != model.SupportSLAStatePending || resumed.Record.PausedSeconds != 3*3600 ||
		!resumed.Record.ResolutionDueAt.Equal(slaStart.Add(5*time.Hour)) {
		t.Fatalf("resumed: %+v", resumed.Record)
	}
	resolved := slaConversation(model.SupportConversationStatusResolved)
	resolved.ResolvedAt = timePtr(slaStart.Add(4*time.Hour + 30*time.Minute))
	done := evalSLAPolicy(policy, resolved, nil, clock, slaStart.Add(6*time.Hour), &resumed.Record)
	if done.Record.ResolutionState != model.SupportSLAStateAchieved || done.Record.NextDueAt != nil {
		t.Fatalf("resolved in time: %+v", done.Record)
	}

	// Without pausing, the same history misses the target.
	strict := policy
	strict.PauseWhileWaiting = false
	missed := evalSLAPolicy(strict, resolved, nil, clock, slaStart.Add(6*time.Hour), nil)
	if missed.Record.ResolutionState != model.SupportSLAStateBreached || len(missed.NewBreaches) != 1 {
		t.Fatalf("strict: %+v", missed.Record)
	}

	snoozed := slaConversation(model.SupportConversationStatusOpen)
	snoozed.SnoozedUntil = timePtr(slaStart.Add(24 * time.Hour))
	if got := evalSLAPolicy(policy, snoozed, nil, clock, slaStart.Add(time.Hour), nil); got.Record.ResolutionState != model.SupportSLAStatePaused {
		t.Fatalf("snoozed conversation should pause resolution: %+v", got.Record)
	}
}

func TestEvaluateSupportSLABusinessHours(t *testing.T) {
	policy := slaPolicy(intPtr(60), nil, nil)
	policy.BusinessHours = true
	clock := businessClock(t, "UTC")
	conversation := slaConversation(model.SupportConversationStatusOpen)
	friday := time.Date(2026, 10, 23, 16, 30, 0, 0, time.UTC)
	conversation.CreatedAt = friday
	got := evalSLAPolicy(policy, conversation, []supportSLAReply{{At: friday, SenderType: "customer"}}, clock, friday.Add(time.Hour), nil)
	if want := time.Date(2026, 10, 26, 9, 30, 0, 0, time.UTC); !got.Record.FirstResponseDueAt.Equal(want) || got.Record.FirstResponseState != model.SupportSLAStatePending {
		t.Fatalf("business-hours due = %v (%s)", got.Record.FirstResponseDueAt, got.Record.FirstResponseState)
	}
}

func TestSupportSLAPolicyMatches(t *testing.T) {
	policy := slaPolicy(intPtr(5), nil, nil)
	conversation := slaConversation(model.SupportConversationStatusOpen)
	if !supportSLAPolicyMatches(policy, conversation, nil) {
		t.Fatal("empty conditions should match")
	}
	policy.Priorities = model.DocsStringArray{"urgent"}
	if supportSLAPolicyMatches(policy, conversation, nil) {
		t.Fatal("priority condition ignored")
	}
	policy.Priorities = model.DocsStringArray{"high"}
	policy.MailboxIDs = model.DocsStringArray{"shared"}
	if !supportSLAPolicyMatches(policy, conversation, nil) {
		t.Fatal("shared inbox should match conversations without an inbox")
	}
	mailbox := "billing"
	conversation.MailboxID = &mailbox
	if supportSLAPolicyMatches(policy, conversation, nil) {
		t.Fatal("inbox condition ignored")
	}
	policy.MailboxIDs = nil
	policy.Channels = model.DocsStringArray{"email"}
	if supportSLAPolicyMatches(policy, conversation, nil) {
		t.Fatal("channel condition ignored")
	}
	policy.Channels = nil
	policy.ActiveSince = slaStart.Add(time.Minute)
	if supportSLAPolicyMatches(policy, conversation, nil) {
		t.Fatal("policies must not apply to conversations created before they were enabled")
	}
	policy.ActiveSince = slaStart
	policy.Enabled = false
	if supportSLAPolicyMatches(policy, conversation, nil) {
		t.Fatal("disabled policy matched")
	}
}

// evalSLAPolicy evaluates a conversation that has no AI involvement against
// a policy, continuing previous when given.
func evalSLAPolicy(policy model.SupportSLAPolicy, conversation model.SupportConversation, replies []supportSLAReply, clock supportSLAClock, now time.Time, previous *model.SupportConversationSLARecord) supportSLAEvaluation {
	in := supportSLAInput{Targets: supportSLATargetsFromPolicy(policy), Conversation: conversation, Replies: replies, Clock: clock, Now: now, Previous: previous}
	if previous != nil {
		in.AppliedAt = previous.PolicyAppliedAt
	}
	return evaluateSupportSLA(in)
}

func TestSupportSLAClockLongTargets(t *testing.T) {
	// One open hour a week: a 90-day target needs 2,160 open hours.
	clock := newSupportSLAClock(model.SupportInboxSettings{
		BusinessHoursEnabled: true, BusinessHoursTimezone: "Europe/Berlin",
		BusinessHoursSchedule: map[string]model.BusinessHoursDay{"mon": {Start: "09:00", End: "10:00", Enabled: true}},
	}, true)
	start := time.Date(2026, 10, 26, 9, 0, 0, 0, clock.location)
	due := clock.Add(start, 2160*time.Hour)
	if got := clock.Between(start, due); got != 2160*time.Hour {
		t.Fatalf("counted %v between start and due %v", got, due)
	}
	local := due.In(clock.location)
	if local.Weekday() != time.Monday || local.Hour() != 10 {
		t.Fatalf("due %v should end a Monday window", local)
	}
	// Weekly skipping agrees with walking day by day across DST.
	business := businessClock(t, "Europe/Berlin")
	from := time.Date(2026, 3, 20, 15, 0, 0, 0, business.location)
	for _, hours := range []int{1, 8, 41, 200, 1000} {
		due := business.Add(from, time.Duration(hours)*time.Hour)
		if got := business.Between(from, due); got != time.Duration(hours)*time.Hour {
			t.Fatalf("%dh: between = %v (due %v)", hours, got, due)
		}
	}
}
