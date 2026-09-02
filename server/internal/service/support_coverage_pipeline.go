package service

import (
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	CoverageEventClaim     = "claim"
	CoverageEventSuccess   = "success"
	CoverageEventFailure   = "failure"
	CoverageEventLeaseLost = "lease_lost"
	CoverageEventReplay    = "replay"
)

type CoverageAttemptState struct {
	Status          string
	RetryBudgetUsed int
	FailureClass    string
}

type CoverageAttemptEvent struct {
	Kind         string
	FailureClass string
	MaxRetries   int
}

// TransitionCoverageAttempt is the pure state machine shared by workers and replay APIs.
func TransitionCoverageAttempt(state CoverageAttemptState, event CoverageAttemptEvent) (CoverageAttemptState, error) {
	next := state
	switch event.Kind {
	case CoverageEventClaim:
		if state.Status != model.CoverageAttemptQueued && state.Status != model.CoverageAttemptRetryable && state.Status != model.CoverageAttemptPausedConfiguration {
			return state, fmt.Errorf("cannot claim coverage attempt in %s", state.Status)
		}
		next.Status = model.CoverageAttemptLeased
		next.FailureClass = ""
	case CoverageEventSuccess:
		if state.Status != model.CoverageAttemptLeased {
			return state, fmt.Errorf("cannot complete coverage attempt in %s", state.Status)
		}
		next.Status = model.CoverageAttemptSucceeded
		next.FailureClass = ""
	case CoverageEventFailure, CoverageEventLeaseLost:
		if state.Status != model.CoverageAttemptLeased {
			return state, fmt.Errorf("cannot fail coverage attempt in %s", state.Status)
		}
		failureClass := event.FailureClass
		if event.Kind == CoverageEventLeaseLost {
			failureClass = model.CoverageFailureLeaseLost
		}
		if !model.IsCoverageFailureClass(failureClass) {
			return state, fmt.Errorf("unsupported coverage failure class %q", failureClass)
		}
		next.FailureClass = failureClass
		if failureClass == model.CoverageFailureConfiguration {
			next.Status = model.CoverageAttemptPausedConfiguration
			return next, nil
		}
		next.RetryBudgetUsed++
		if event.MaxRetries > 0 && next.RetryBudgetUsed >= event.MaxRetries {
			next.Status = model.CoverageAttemptDeadLetter
		} else {
			next.Status = model.CoverageAttemptRetryable
		}
	case CoverageEventReplay:
		if state.Status != model.CoverageAttemptDeadLetter && state.Status != model.CoverageAttemptPausedConfiguration {
			return state, fmt.Errorf("cannot replay coverage attempt in %s", state.Status)
		}
		next.Status = model.CoverageAttemptQueued
		next.RetryBudgetUsed = 0
		next.FailureClass = ""
	default:
		return state, fmt.Errorf("unsupported coverage event %q", event.Kind)
	}
	return next, nil
}
