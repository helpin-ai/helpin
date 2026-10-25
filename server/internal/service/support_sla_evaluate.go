package service

import (
	"slices"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
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

// supportSLAPolicyMatches reports whether policy applies to conversation.
func supportSLAPolicyMatches(policy model.SupportSLAPolicy, conversation model.SupportConversation) bool {
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
	return true
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


// evaluateSupportSLA recomputes a conversation's SLA state from its policy,
// public replies and status. Only teammate replies meet response targets.
// Breach times already recorded are kept.
func evaluateSupportSLA(
	policy model.SupportSLAPolicy,
	conversation model.SupportConversation,
	replies []supportSLAReply,
	clock supportSLAClock,
	now time.Time,
	previous *model.SupportConversationSLARecord,
) supportSLAEvaluation {
	if previous == nil {
		previous = &model.SupportConversationSLARecord{}
	}
	record := model.SupportConversationSLARecord{
		ConversationID: conversation.ID, WorkspaceID: conversation.WorkspaceID,
		PolicyID: &policy.ID, PolicyName: policy.Name,
	}
	sorted := slices.Clone(replies)
	slices.SortStableFunc(sorted, func(a, b supportSLAReply) int { return a.At.Compare(b.At) })

	resolved := conversation.Status == model.SupportConversationStatusResolved
	closed := resolved || conversation.Status == model.SupportConversationStatusSpam
	closeTime := now
	if resolved && conversation.ResolvedAt != nil {
		closeTime = *conversation.ResolvedAt
	} else if !resolved && closed && conversation.ClosedAt != nil {
		closeTime = *conversation.ClosedAt
	}
	var newBreaches []string
	var openDue, wakeDue []*time.Time
	breachedNow := false

	// First response: from the customer's first message to the first
	// teammate reply. Conversations a teammate started have no target.
	if policy.FirstResponseMinutes != nil {
		var firstCustomer, firstTeammate *time.Time
		for _, reply := range sorted {
			if reply.SenderType == "customer" && firstCustomer == nil {
				firstCustomer = timePtr(reply.At)
			}
			if reply.SenderType == "user" && firstTeammate == nil {
				firstTeammate = timePtr(reply.At)
			}
		}
		if firstCustomer != nil && (firstTeammate == nil || !firstTeammate.Before(*firstCustomer)) {
			due := clock.Add(*firstCustomer, minutesDuration(policy.FirstResponseMinutes))
			record.FirstResponseDueAt = &due
			switch {
			case firstTeammate != nil:
				record.FirstResponseAchievedAt = firstTeammate
				record.FirstResponseState = model.SupportSLAStateAchieved
				if firstTeammate.After(due) {
					record.FirstResponseState = model.SupportSLAStateBreached
				}
			case closed:
				record.FirstResponseState = model.SupportSLAStateCancelled
				if closeTime.After(due) {
					record.FirstResponseState = model.SupportSLAStateBreached
				}
			default:
				record.FirstResponseState = model.SupportSLAStatePending
				if !now.Before(due) {
					record.FirstResponseState = model.SupportSLAStateBreached
					breachedNow = true
					openDue = append(openDue, &due)
				} else {
					openDue = append(openDue, &due)
					wakeDue = append(wakeDue, &due)
				}
			}
			if record.FirstResponseState == model.SupportSLAStateBreached {
				record.FirstResponseBreachedAt = earliestTime(previous.FirstResponseBreachedAt, &due)
				if previous.FirstResponseBreachedAt == nil {
					newBreaches = append(newBreaches, model.SupportSLAMetricFirstResponse)
				}
			}
		}
	}

	// Next response: each customer message after a teammate replied starts
	// a cycle that the next teammate reply ends.
	if policy.NextResponseMinutes != nil {
		target := minutesDuration(policy.NextResponseMinutes)
		var waitingSince, lastBreachDue *time.Time
		seenTeammate, cycles, breaches := false, 0, 0
		for _, reply := range sorted {
			switch reply.SenderType {
			case "user":
				if waitingSince != nil {
					cycles++
					if due := clock.Add(*waitingSince, target); reply.At.After(due) {
						breaches++
						lastBreachDue = timePtr(due)
					}
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
			due := clock.Add(*waitingSince, target)
			end := now
			if closed {
				end = closeTime
			}
			if end.After(due) || end.Equal(due) {
				breaches++
				lastBreachDue = timePtr(due)
			}
			if !closed {
				cycles++
				record.NextResponseDueAt = &due
				openDue = append(openDue, &due)
				if now.Before(due) {
					wakeDue = append(wakeDue, &due)
				} else {
					breachedNow = true
				}
			}
		}
		record.NextResponseBreaches = breaches
		record.NextResponseBreachedAt = lastBreachDue
		switch {
		case record.NextResponseDueAt != nil && now.Before(*record.NextResponseDueAt):
			record.NextResponseState = model.SupportSLAStatePending
		case breaches > 0:
			record.NextResponseState = model.SupportSLAStateBreached
		case cycles > 0:
			record.NextResponseState = model.SupportSLAStateAchieved
		}
		if lastBreachDue != nil && (previous.NextResponseBreachedAt == nil || lastBreachDue.After(*previous.NextResponseBreachedAt)) &&
			breaches > previous.NextResponseBreaches {
			newBreaches = append(newBreaches, model.SupportSLAMetricNextResponse)
		}
	}

	// Resolution: from creation to resolution, optionally excluding time
	// spent waiting on the customer or snoozed.
	if policy.ResolutionMinutes != nil {
		pausedSeconds, pausedSince := previous.PausedSeconds, previous.PausedSince
		pausedNow := policy.PauseWhileWaiting && !closed && supportSLAWaitingOnCustomer(conversation)
		if pausedSince != nil && !pausedNow {
			end := now
			if closed && closeTime.Before(now) {
				end = closeTime
			}
			pausedSeconds += int64(clock.Between(*pausedSince, end) / time.Second)
			pausedSince = nil
		}
		if pausedNow && pausedSince == nil {
			pausedSince = timePtr(now)
		}
		record.PausedSeconds, record.PausedSince = pausedSeconds, pausedSince
		due := clock.Add(conversation.CreatedAt, minutesDuration(policy.ResolutionMinutes)+time.Duration(pausedSeconds)*time.Second)
		switch {
		case resolved:
			record.ResolutionDueAt = &due
			record.ResolutionAchievedAt = timePtr(closeTime)
			record.ResolutionState = model.SupportSLAStateAchieved
			if closeTime.After(due) || previous.ResolutionBreachedAt != nil {
				record.ResolutionState = model.SupportSLAStateBreached
			}
		case closed:
			record.ResolutionDueAt = &due
			record.ResolutionState = model.SupportSLAStateCancelled
			if previous.ResolutionBreachedAt != nil {
				record.ResolutionState = model.SupportSLAStateBreached
			}
		case previous.ResolutionBreachedAt != nil:
			// A target missed before a pause stays missed.
			record.ResolutionDueAt = &due
			record.ResolutionState = model.SupportSLAStateBreached
			breachedNow = true
			openDue = append(openDue, &due)
		case pausedNow:
			record.ResolutionState = model.SupportSLAStatePaused
		default:
			record.ResolutionDueAt = &due
			record.ResolutionState = model.SupportSLAStatePending
			openDue = append(openDue, &due)
			if !now.Before(due) {
				record.ResolutionState = model.SupportSLAStateBreached
				breachedNow = true
			} else {
				wakeDue = append(wakeDue, &due)
			}
		}
		if record.ResolutionState == model.SupportSLAStateBreached {
			record.ResolutionBreachedAt = earliestTime(previous.ResolutionBreachedAt, &due)
			if previous.ResolutionBreachedAt == nil {
				newBreaches = append(newBreaches, model.SupportSLAMetricResolution)
			}
		}
	}

	record.NextDueAt = earliestTime(openDue...)
	record.WakeAt = earliestTime(wakeDue...)
	record.Breached = breachedNow
	return supportSLAEvaluation{Record: record, NewBreaches: newBreaches}
}
