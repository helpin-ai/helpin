package service

import (
	"slices"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// supportSLAReply is one public reply on a conversation's timeline.
type supportSLAReply struct {
	At         time.Time
	SenderType string
}

// supportSLAEvaluation is the recomputed SLA state and the metrics whose
// targets were missed since the previous evaluation.
type supportSLAEvaluation struct {
	Record      model.SupportConversationSLARecord
	NewBreaches []string
}

// supportSLAPolicyMatches reports whether policy applies to conversation,
// whose company is company (nil without one).
func supportSLAPolicyMatches(policy model.SupportSLAPolicy, conversation model.SupportConversation, company *repository.SupportSLACompany) bool {
	if !policy.Enabled || conversation.CreatedAt.Before(policy.ActiveSince) {
		return false
	}
	if len(policy.Priorities) > 0 && !slices.Contains([]string(policy.Priorities), conversation.Priority) {
		return false
	}
	if len(policy.MailboxIDs) > 0 {
		mailbox := "shared"
		if conversation.MailboxID != nil && *conversation.MailboxID != "" {
			mailbox = *conversation.MailboxID
		}
		if !slices.Contains([]string(policy.MailboxIDs), mailbox) {
			return false
		}
	}
	if len(policy.Channels) > 0 && !slices.Contains([]string(policy.Channels), conversation.Channel) {
		return false
	}
	if len(policy.CompanyTierIDs) > 0 && !slices.Contains([]string(policy.CompanyTierIDs), supportSLACompanyTier(company)) {
		return false
	}
	if len(policy.CompanyIDs) > 0 && (company == nil || !slices.Contains([]string(policy.CompanyIDs), company.ID)) {
		return false
	}
	return true
}

// supportSLACompanyTier is the tier policies match for company:
// model.SupportSLANoTier without a company or tier.
func supportSLACompanyTier(company *repository.SupportSLACompany) string {
	if company == nil || company.TierID == nil || *company.TierID == "" {
		return model.SupportSLANoTier
	}
	return *company.TierID
}

// supportSLACompanyKey identifies the company and tier a conversation is
// matched with; a change of either counts new targets from the change.
func supportSLACompanyKey(company *repository.SupportSLACompany) string {
	if company == nil {
		return "company:none"
	}
	return "company:" + company.ID + "|tier:" + supportSLACompanyTier(company)
}

// supportSLAWaitingOnCustomer mirrors the inbox's Waiting rule: the status
// says so, or a teammate replied last and the customer has not answered.
func supportSLAWaitingOnCustomer(conversation model.SupportConversation) bool {
	if conversation.SnoozedUntil != nil {
		return true
	}
	if conversation.Status == model.SupportConversationStatusWaitingOnCustomer {
		return true
	}
	return conversation.Status == model.SupportConversationStatusOpen &&
		conversation.LastPublicSenderType != nil && *conversation.LastPublicSenderType == "user" &&
		!conversation.CustomerAwaitingResponse
}

func minutesDuration(minutes *int) time.Duration {
	return time.Duration(*minutes) * time.Minute
}

func earliestTime(values ...*time.Time) *time.Time {
	var earliest *time.Time
	for _, value := range values {
		if value != nil && (earliest == nil || value.Before(*earliest)) {
			v := *value
			earliest = &v
		}
	}
	return earliest
}

// supportSLATargets is the set of targets a conversation is measured
// against: a policy's settings at the moment it applied.
type supportSLATargets struct {
	PolicyID             string
	PolicyName           string
	FirstResponseMinutes *int
	NextResponseMinutes  *int
	ResolutionMinutes    *int
	BusinessHours        bool
	PauseWhileWaiting    bool
}

func supportSLATargetsFromPolicy(policy model.SupportSLAPolicy) supportSLATargets {
	return supportSLATargets{
		PolicyID: policy.ID, PolicyName: policy.Name,
		FirstResponseMinutes: policy.FirstResponseMinutes, NextResponseMinutes: policy.NextResponseMinutes,
		ResolutionMinutes: policy.ResolutionMinutes, BusinessHours: policy.BusinessHours,
		PauseWhileWaiting: policy.PauseWhileWaiting,
	}
}

// supportSLATargetsFromRecord returns the snapshot stored on a record, or
// false when the record has none.
func supportSLATargetsFromRecord(record model.SupportConversationSLARecord) (supportSLATargets, bool) {
	if record.PolicyID == nil || (record.TargetFirstResponseMinutes == nil && record.TargetNextResponseMinutes == nil && record.TargetResolutionMinutes == nil) {
		return supportSLATargets{}, false
	}
	return supportSLATargets{
		PolicyID: *record.PolicyID, PolicyName: record.PolicyName,
		FirstResponseMinutes: record.TargetFirstResponseMinutes, NextResponseMinutes: record.TargetNextResponseMinutes,
		ResolutionMinutes: record.TargetResolutionMinutes, BusinessHours: record.TargetBusinessHours,
		PauseWhileWaiting: record.TargetPauseWhileWaiting,
	}, true
}

// supportSLAInput is everything one evaluation reads.
type supportSLAInput struct {
	Targets      supportSLATargets
	Conversation model.SupportConversation
	Replies      []supportSLAReply
	Clock        supportSLAClock
	Now          time.Time
	// Previous is the stored record, nil for the first evaluation.
	Previous *model.SupportConversationSLARecord
	// Switched means Targets replace those of Previous: results Previous
	// already recorded are kept, and next-response cycles that ended
	// before AppliedAt stay judged by the earlier targets.
	Switched  bool
	AppliedAt *time.Time
	// AIInvolved marks a conversation the AI handled first; HumanSince is
	// its handoff to the team, nil while the AI still owns it or when the
	// AI resolved it alone. AIOwnsNow pauses response clocks while the AI
	// answers.
	AIInvolved bool
	HumanSince *time.Time
	AIOwnsNow  bool
	// Resumed means the team took back a conversation the AI answered
	// again after the handoff; cycles before now keep their results.
	Resumed bool
	// TargetsFrom is the earliest time any target counts from.
	TargetsFrom *time.Time
	// CompanyKey is the company and tier the targets were selected with.
	CompanyKey string
	// HistoryBefore marks replies before it as history: an imported
	// conversation a teammate already answered elsewhere has no
	// first-response target.
	HistoryBefore *time.Time
}

// evaluateSupportSLA recomputes a conversation's SLA state. Only teammate
// replies meet response targets. Achieved results and recorded breaches are
// final; a target that applies later never re-judges them.
func evaluateSupportSLA(in supportSLAInput) supportSLAEvaluation {
	previous := in.Previous
	if previous == nil {
		previous = &model.SupportConversationSLARecord{}
	}
	targets, conversation, clock, now := in.Targets, in.Conversation, in.Clock, in.Now
	record := model.SupportConversationSLARecord{
		ConversationID: conversation.ID, WorkspaceID: conversation.WorkspaceID,
		PolicyID: &targets.PolicyID, PolicyName: targets.PolicyName, PolicyAppliedAt: in.AppliedAt,
		TargetFirstResponseMinutes: targets.FirstResponseMinutes, TargetNextResponseMinutes: targets.NextResponseMinutes,
		TargetResolutionMinutes: targets.ResolutionMinutes, TargetBusinessHours: targets.BusinessHours,
		TargetPauseWhileWaiting: targets.PauseWhileWaiting,
		AIInvolved:              in.AIInvolved, HumanSince: in.HumanSince, TargetsFrom: in.TargetsFrom, CompanyKey: in.CompanyKey,
		FirstResponsePausedSeconds: previous.FirstResponsePausedSeconds,
	}
	if previous.DetachedAt != nil && previous.FirstResponseAchievedAt == nil && previous.FirstResponseBreachedAt == nil {
		// Reattached targets start from now; earlier holds are outside that clock.
		record.FirstResponsePausedSeconds = 0
	}
	sorted := slices.Clone(in.Replies)
	slices.SortStableFunc(sorted, func(a, b supportSLAReply) int { return a.At.Compare(b.At) })

	state := supportSLAEvalState{now: now, clock: clock}
	state.resolved = conversation.Status == model.SupportConversationStatusResolved
	state.closed = state.resolved || conversation.Status == model.SupportConversationStatusSpam
	state.closeTime = now
	if state.resolved && conversation.ResolvedAt != nil {
		state.closeTime = *conversation.ResolvedAt
	} else if !state.resolved && state.closed && conversation.ClosedAt != nil {
		state.closeTime = *conversation.ClosedAt
	}
	// The AI owns conversations it handled until the handoff; the team's
	// targets do not run before then.
	state.aiOnly = in.AIInvolved && in.HumanSince == nil
	state.aiHolding = in.AIOwnsNow && !state.closed
	state.humanSince = in.HumanSince
	state.targetsFrom = in.TargetsFrom
	state.historyBefore = in.HistoryBefore
	if state.aiHolding && in.HumanSince != nil {
		record.AIHoldingSince = previous.AIHoldingSince
		if record.AIHoldingSince == nil {
			record.AIHoldingSince = timePtr(now)
		}
	}

	if targets.FirstResponseMinutes != nil {
		state.firstResponse(&record, previous, sorted, minutesDuration(targets.FirstResponseMinutes))
	} else {
		keepSupportSLAFirstResponse(&record, previous)
	}
	if targets.NextResponseMinutes != nil {
		state.nextResponse(&record, previous, in, sorted, minutesDuration(targets.NextResponseMinutes))
	} else {
		keepSupportSLANextResponse(&record, previous)
	}
	if targets.ResolutionMinutes != nil {
		state.resolution(&record, previous, conversation, targets, minutesDuration(targets.ResolutionMinutes))
	} else {
		keepSupportSLAResolution(&record, previous)
	}
	record.NextDueAt = earliestTime(state.openDue...)
	record.WakeAt = earliestTime(state.wakeDue...)
	record.Breached = state.breachedNow
	return supportSLAEvaluation{Record: record, NewBreaches: state.newBreaches}
}

// Results recorded under earlier targets stay when the current targets no
// longer measure that metric; open ones end.
func keepSupportSLAFirstResponse(record, previous *model.SupportConversationSLARecord) {
	if previous.FirstResponseAchievedAt == nil && previous.FirstResponseBreachedAt == nil {
		return
	}
	record.FirstResponseState = previous.FirstResponseState
	if previous.FirstResponseAchievedAt == nil {
		record.FirstResponseState = model.SupportSLAStateBreached
	}
	record.FirstResponseDueAt = previous.FirstResponseDueAt
	record.FirstResponseAchievedAt = previous.FirstResponseAchievedAt
	record.FirstResponseBreachedAt = previous.FirstResponseBreachedAt
}

func keepSupportSLANextResponse(record, previous *model.SupportConversationSLARecord) {
	switch {
	case previous.NextResponseBreaches > 0:
		record.NextResponseState = model.SupportSLAStateBreached
	case previous.NextResponseState == model.SupportSLAStateAchieved:
		record.NextResponseState = model.SupportSLAStateAchieved
	default:
		return
	}
	record.NextResponseBreaches = previous.NextResponseBreaches
	record.NextResponseCarriedBreaches = previous.NextResponseBreaches
	record.NextResponseBreachedAt = previous.NextResponseBreachedAt
}

func keepSupportSLAResolution(record, previous *model.SupportConversationSLARecord) {
	if previous.ResolutionAchievedAt == nil && previous.ResolutionBreachedAt == nil {
		return
	}
	record.ResolutionState = previous.ResolutionState
	if previous.ResolutionAchievedAt == nil {
		record.ResolutionState = model.SupportSLAStateBreached
	}
	record.ResolutionDueAt = previous.ResolutionDueAt
	record.ResolutionAchievedAt = previous.ResolutionAchievedAt
	record.ResolutionBreachedAt = previous.ResolutionBreachedAt
	record.PausedSeconds = previous.PausedSeconds
}

type supportSLAEvalState struct {
	now, closeTime          time.Time
	clock                   supportSLAClock
	resolved, closed        bool
	aiOnly, aiHolding       bool
	humanSince, targetsFrom *time.Time
	historyBefore           *time.Time
	newBreaches             []string
	openDue, wakeDue        []*time.Time
	breachedNow             bool
}

// start moves a clock start to the handoff when the AI handled the
// conversation first.
func (st *supportSLAEvalState) start(at time.Time) time.Time {
	for _, floor := range []*time.Time{st.humanSince, st.targetsFrom} {
		if floor != nil && floor.After(at) {
			at = *floor
		}
	}
	return at
}

// open tracks a running target due at due.
func (st *supportSLAEvalState) open(due time.Time) {
	st.openDue = append(st.openDue, &due)
	if st.now.Before(due) {
		st.wakeDue = append(st.wakeDue, &due)
	} else {
		st.breachedNow = true
	}
}

// firstResponse: from the customer's first message (or the handoff) to the
// first teammate reply. Conversations a teammate started have no target.
func (st *supportSLAEvalState) firstResponse(record, previous *model.SupportConversationSLARecord, replies []supportSLAReply, target time.Duration) {
	if previous.FirstResponseAchievedAt != nil {
		record.FirstResponseState = previous.FirstResponseState
		record.FirstResponseDueAt = previous.FirstResponseDueAt
		record.FirstResponseAchievedAt = previous.FirstResponseAchievedAt
		record.FirstResponseBreachedAt = previous.FirstResponseBreachedAt
		return
	}
	var firstCustomer, firstTeammate *time.Time
	for _, reply := range replies {
		if reply.SenderType == "customer" && firstCustomer == nil {
			firstCustomer = timePtr(reply.At)
		}
		if reply.SenderType == "user" && firstTeammate == nil {
			firstTeammate = timePtr(reply.At)
		}
	}
	if firstCustomer == nil || (firstTeammate != nil && firstTeammate.Before(*firstCustomer)) {
		return
	}
	if st.historyBefore != nil && firstTeammate != nil && firstTeammate.Before(*st.historyBefore) {
		return
	}
	if st.aiOnly && firstTeammate == nil {
		record.FirstResponseState = model.SupportSLAStatePaused
		if st.closed {
			record.FirstResponseState = model.SupportSLAStateCancelled
		}
		return
	}
	if previous.FirstResponseBreachedAt == nil && previous.AIHoldingSince != nil && (!st.aiHolding || firstTeammate != nil) {
		// Finish the hold at the reply or closure, even if evaluation runs
		// later. The accumulated duration survives later evaluations and holds.
		end := st.now
		if firstTeammate != nil && firstTeammate.Before(end) {
			end = *firstTeammate
		}
		if st.closed && st.closeTime.Before(end) {
			end = st.closeTime
		}
		start := st.start(*previous.AIHoldingSince)
		if firstCustomer.After(start) {
			start = *firstCustomer
		}
		record.FirstResponsePausedSeconds += int64(st.clock.Between(start, end) / time.Second)
	}
	due := st.clock.Add(st.start(*firstCustomer), target+time.Duration(record.FirstResponsePausedSeconds)*time.Second)
	record.FirstResponseDueAt = &due
	missed := previous.FirstResponseBreachedAt != nil
	switch {
	case firstTeammate != nil:
		record.FirstResponseAchievedAt = firstTeammate
		record.FirstResponseState = model.SupportSLAStateAchieved
		missed = missed || firstTeammate.After(due)
	case st.closed:
		record.FirstResponseState = model.SupportSLAStateCancelled
		missed = missed || st.closeTime.After(due)
	case st.aiHolding && !missed:
		record.FirstResponseState = model.SupportSLAStatePaused
		record.FirstResponseDueAt = nil
	default:
		record.FirstResponseState = model.SupportSLAStatePending
		missed = missed || !st.now.Before(due)
		st.open(due)
	}
	if missed {
		record.FirstResponseState = model.SupportSLAStateBreached
		record.FirstResponseBreachedAt = earliestTime(previous.FirstResponseBreachedAt, &due)
		if previous.FirstResponseBreachedAt == nil {
			st.newBreaches = append(st.newBreaches, model.SupportSLAMetricFirstResponse)
		}
	}
}

// nextResponse: each customer message after a teammate replied starts a
// cycle that the next teammate reply ends. Cycles that started before the
// current targets applied keep their earlier result, counted in
// NextResponseCarriedBreaches; the open cycle at a switch is judged by the
// new targets unless it was already missed.
func (st *supportSLAEvalState) nextResponse(record, previous *model.SupportConversationSLARecord, in supportSLAInput, replies []supportSLAReply, target time.Duration) {
	type cycle struct {
		start time.Time
		end   *time.Time
	}
	var cycles []cycle
	var waitingSince *time.Time
	seenTeammate := false
	for _, reply := range replies {
		switch reply.SenderType {
		case "user":
			if waitingSince != nil {
				cycles = append(cycles, cycle{start: *waitingSince, end: timePtr(reply.At)})
				waitingSince = nil
			}
			seenTeammate = true
		case "customer":
			if seenTeammate && waitingSince == nil {
				waitingSince = timePtr(reply.At)
			}
		}
	}
	if waitingSince != nil {
		cycles = append(cycles, cycle{start: *waitingSince})
	}
	// The open cycle already carries a recorded breach when the last
	// recorded breach falls inside it.
	openMissed := waitingSince != nil && previous.NextResponseBreachedAt != nil && !previous.NextResponseBreachedAt.Before(*waitingSince)

	appliedAt, carried := in.AppliedAt, previous.NextResponseCarriedBreaches
	switch {
	case in.Switched:
		carried = previous.NextResponseBreaches
		appliedAt = timePtr(st.now)
		if waitingSince != nil && !openMissed && in.TargetsFrom == nil {
			appliedAt = waitingSince
		}
	case in.Resumed:
		// The open cycle restarts now; earlier cycles keep their results.
		carried = previous.NextResponseBreaches
		appliedAt = timePtr(st.now)
	}
	record.PolicyAppliedAt = appliedAt

	breaches := 0
	lastBreachDue := previous.NextResponseBreachedAt
	for _, c := range cycles {
		judged := appliedAt == nil || !c.start.Before(*appliedAt)
		if c.end != nil {
			if !judged {
				continue
			}
			if due := st.clock.Add(c.start, target); c.end.After(due) {
				breaches++
				lastBreachDue = timePtr(due)
			}
			continue
		}
		// The open cycle. One that started before the current targets
		// applied and was not missed yet runs from when they applied.
		start := c.start
		if !judged && !openMissed {
			start, judged = *appliedAt, true
		}
		if st.aiHolding && !openMissed {
			// The AI is answering; the team's clock waits for the hand-back.
			continue
		}
		due := st.clock.Add(start, target)
		missed := false
		if judged {
			end := st.now
			if st.closed {
				end = st.closeTime
			}
			missed = openMissed || !end.Before(due)
			if missed {
				breaches++
				if !openMissed {
					lastBreachDue = timePtr(due)
				}
			}
		} else {
			// Missed under earlier targets; it stays overdue.
			missed = true
			if previous.NextResponseBreachedAt != nil {
				due = *previous.NextResponseBreachedAt
			}
		}
		if st.closed {
			continue
		}
		record.NextResponseDueAt = timePtr(due)
		st.open(due)
		if missed {
			st.breachedNow = true
		}
	}
	record.NextResponseCarriedBreaches = carried
	record.NextResponseBreaches = carried + breaches
	// A breach later than any recorded one is new, whatever the count says.
	newerBreach := lastBreachDue != nil && (previous.NextResponseBreachedAt == nil || lastBreachDue.After(*previous.NextResponseBreachedAt))
	// A recorded breach stays recorded, even if a message is deleted later;
	// a new breach still adds to the count.
	if !in.Switched && !in.Resumed && previous.NextResponseBreaches > record.NextResponseBreaches {
		total := previous.NextResponseBreaches
		if newerBreach {
			total++
		}
		// Keep the difference as carried breaches so later evaluations
		// count from the recorded total.
		record.NextResponseCarriedBreaches += total - record.NextResponseBreaches
		record.NextResponseBreaches = total
	}
	record.NextResponseBreachedAt = lastBreachDue
	switch {
	case record.NextResponseDueAt != nil && !st.breachedNowFor(*record.NextResponseDueAt, openMissed):
		record.NextResponseState = model.SupportSLAStatePending
	case record.NextResponseBreaches > 0:
		record.NextResponseState = model.SupportSLAStateBreached
	case waitingSince != nil && st.aiHolding && !st.closed:
		record.NextResponseState = model.SupportSLAStatePaused
	case len(cycles) > 0:
		record.NextResponseState = model.SupportSLAStateAchieved
	}
	if record.NextResponseBreaches > previous.NextResponseBreaches && newerBreach {
		st.newBreaches = append(st.newBreaches, model.SupportSLAMetricNextResponse)
	}
}

// breachedNowFor reports whether an open target due at due is overdue.
func (st *supportSLAEvalState) breachedNowFor(due time.Time, alreadyMissed bool) bool {
	return alreadyMissed || !st.now.Before(due)
}

// resolution: from creation (or the handoff) to the first resolution,
// optionally excluding time spent waiting on the customer or snoozed. The
// first resolution is final; reopening keeps its result.
func (st *supportSLAEvalState) resolution(record, previous *model.SupportConversationSLARecord, conversation model.SupportConversation, targets supportSLATargets, target time.Duration) {
	if previous.ResolutionAchievedAt != nil {
		record.ResolutionState = previous.ResolutionState
		record.ResolutionDueAt = previous.ResolutionDueAt
		record.ResolutionAchievedAt = previous.ResolutionAchievedAt
		record.ResolutionBreachedAt = previous.ResolutionBreachedAt
		record.PausedSeconds = previous.PausedSeconds
		return
	}
	if st.aiOnly {
		record.ResolutionState = model.SupportSLAStatePaused
		if st.closed {
			record.ResolutionState = model.SupportSLAStateCancelled
		}
		return
	}
	pausedSeconds, pausedSince := previous.PausedSeconds, previous.PausedSince
	pausedNow := !st.closed && ((targets.PauseWhileWaiting && supportSLAWaitingOnCustomer(conversation)) || st.aiHolding)
	if pausedSince != nil && !pausedNow {
		end := st.now
		if st.closed && st.closeTime.Before(st.now) {
			end = st.closeTime
		}
		pausedSeconds += int64(st.clock.Between(*pausedSince, end) / time.Second)
		pausedSince = nil
	}
	if pausedNow && pausedSince == nil {
		pausedSince = timePtr(st.now)
	}
	record.PausedSeconds, record.PausedSince = pausedSeconds, pausedSince
	due := st.clock.Add(st.start(conversation.CreatedAt), target+time.Duration(pausedSeconds)*time.Second)
	missed := previous.ResolutionBreachedAt != nil
	record.ResolutionDueAt = &due
	switch {
	case st.resolved:
		record.ResolutionAchievedAt = timePtr(st.closeTime)
		record.ResolutionState = model.SupportSLAStateAchieved
		missed = missed || st.closeTime.After(due)
	case st.closed:
		record.ResolutionState = model.SupportSLAStateCancelled
	case missed:
		// A target missed before a pause stays missed and overdue.
		st.openDue = append(st.openDue, &due)
		st.breachedNow = true
	case pausedNow:
		record.ResolutionState = model.SupportSLAStatePaused
		record.ResolutionDueAt = nil
	default:
		record.ResolutionState = model.SupportSLAStatePending
		missed = !st.now.Before(due)
		st.open(due)
	}
	if missed {
		record.ResolutionState = model.SupportSLAStateBreached
		record.ResolutionBreachedAt = earliestTime(previous.ResolutionBreachedAt, &due)
		if previous.ResolutionBreachedAt == nil {
			st.newBreaches = append(st.newBreaches, model.SupportSLAMetricResolution)
		}
	}
}

// detachSupportSLARecord ends SLA tracking for a conversation no policy
// applies to any more: open targets are cancelled, recorded results kept.
func detachSupportSLARecord(previous model.SupportConversationSLARecord, now time.Time) model.SupportConversationSLARecord {
	record := previous
	cancel := func(state *string, due **time.Time) {
		if *state == model.SupportSLAStatePending || *state == model.SupportSLAStatePaused {
			*state = model.SupportSLAStateCancelled
			*due = nil
		}
	}
	cancel(&record.FirstResponseState, &record.FirstResponseDueAt)
	cancel(&record.NextResponseState, &record.NextResponseDueAt)
	cancel(&record.ResolutionState, &record.ResolutionDueAt)
	record.NextResponseDueAt = nil
	record.PausedSince = nil
	record.NextDueAt, record.WakeAt, record.Breached = nil, nil, false
	record.DetachedAt = timePtr(now)
	return record
}
