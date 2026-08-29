package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	errCommercialStateStale    = errors.New("commercial state patch is stale")
	errCommercialStateFuture   = errors.New("commercial state timestamp is in the future")
	errCommercialStateConflict = errors.New("commercial state timestamp conflicts")
	errCommercialStateInvalid  = errors.New("commercial state patch is invalid")
)

const (
	commercialStateFutureSkew   = 5 * time.Minute
	usageAnomalyMinimumAccounts = 100
	usageAnomalyRatio           = 0.35
)

var allowedCommercialStateFields = map[string]struct{}{
	"subscription_status": {}, "plan_key": {}, "billing_interval": {},
	"currency": {}, "mrr_minor": {}, "trial_started_at": {}, "trial_ends_at": {},
	"current_period_started_at": {}, "current_period_ends_at": {}, "renewal_at": {},
	"cancel_scheduled_at": {}, "canceled_at": {}, "seats_purchased": {},
	"seats_used": {}, "onboarding_started_at": {}, "first_value_at": {},
	"customer_success_owner_member_id": {},
}

// MaterializeCompanyCommercialState applies a server-authenticated group state patch.
func (s *CRMSignalService) MaterializeCompanyCommercialState(
	ctx context.Context,
	workspaceID, companyID, sourceEventID, identityMethod string,
	patch model.JSONB,
	stateUpdatedAt, now time.Time,
) (bool, error) {
	if identityMethod != model.IdentityMethodServerEvent {
		return false, fmt.Errorf("commercial state requires server_event identity")
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(companyID) == "" || strings.TrimSpace(sourceEventID) == "" {
		return false, fmt.Errorf("workspace_id, company_id, and source_event_id are required")
	}
	current, err := s.signalRepo.GetCompanyCommercialState(ctx, workspaceID, companyID)
	if err != nil {
		return false, err
	}
	currentState, currentUpdatedAt := model.JSONB{}, time.Time{}
	if current != nil {
		currentState, currentUpdatedAt = current.State, current.StateUpdatedAt
	}
	nextState, changed, err := ApplyCommercialStatePatch(currentState, currentUpdatedAt, patch, stateUpdatedAt, now)
	if err != nil {
		if errors.Is(err, errCommercialStateStale) {
			return s.signalRepo.RetainStaleCompanyCommercialState(
				ctx, workspaceID, companyID, sourceEventID, patch, currentState, stateUpdatedAt, now,
			)
		}
		_ = s.signalRepo.RecordRejectedCompanyCommercialState(
			ctx, workspaceID, companyID, commercialStateRejectionCode(err), now,
		)
		return false, err
	}
	if !changed {
		return false, nil
	}
	inserted, err := s.signalRepo.AcceptCompanyCommercialState(
		ctx, workspaceID, companyID, sourceEventID, patch, nextState,
		currentUpdatedAt, stateUpdatedAt, now,
	)
	if err != nil {
		_ = s.signalRepo.RecordRejectedCompanyCommercialState(ctx, workspaceID, companyID, "storage_error", now)
		return false, err
	}
	if !inserted {
		return false, nil
	}
	if err := s.RefreshEntityMotionSignals(ctx, workspaceID, "company", companyID); err != nil {
		return false, err
	}
	return true, nil
}

// ApplyCommercialStatePatch validates and applies one ordered server-authenticated patch.
func ApplyCommercialStatePatch(
	current model.JSONB,
	currentUpdatedAt time.Time,
	patch model.JSONB,
	stateUpdatedAt time.Time,
	now time.Time,
) (model.JSONB, bool, error) {
	if stateUpdatedAt.IsZero() {
		return nil, false, fmt.Errorf("%w: state_updated_at is required", errCommercialStateInvalid)
	}
	if stateUpdatedAt.After(now.Add(commercialStateFutureSkew)) {
		return nil, false, fmt.Errorf("%w: exceeds the five minute allowance", errCommercialStateFuture)
	}
	result := cloneCommercialState(current)
	for key, value := range patch {
		key = strings.TrimSpace(key)
		if _, ok := allowedCommercialStateFields[key]; !ok {
			return nil, false, fmt.Errorf("%w: field %q is not supported", errCommercialStateInvalid, key)
		}
		if value == nil {
			delete(result, key)
			continue
		}
		result[key] = value
	}
	if stateUpdatedAt.Before(currentUpdatedAt) {
		return result, false, errCommercialStateStale
	}
	if stateUpdatedAt.Equal(currentUpdatedAt) {
		if reflect.DeepEqual(normalizeCommercialJSON(current), normalizeCommercialJSON(result)) {
			return result, false, nil
		}
		return nil, false, fmt.Errorf("%w with an accepted update", errCommercialStateConflict)
	}
	return result, true, nil
}

func commercialStateRejectionCode(err error) string {
	switch {
	case errors.Is(err, errCommercialStateFuture):
		return "future_timestamp"
	case errors.Is(err, errCommercialStateConflict):
		return "timestamp_conflict"
	case errors.Is(err, errCommercialStateInvalid):
		return "invalid_patch"
	default:
		return "validation_error"
	}
}

// ShouldSuppressUsageDeclineBatch applies the workspace-wide holiday/anomaly guard.
func ShouldSuppressUsageDeclineBatch(eligibleAccounts, trippedAccounts int) bool {
	if eligibleAccounts < usageAnomalyMinimumAccounts || trippedAccounts <= 0 {
		return false
	}
	return float64(trippedAccounts)/float64(eligibleAccounts) >= usageAnomalyRatio
}

// IsUsageDecline reports whether at least five of seven days are below 60% of weekday expectation.
func IsUsageDecline(actual, expected []float64) bool {
	if len(actual) != 7 || len(expected) != 7 {
		return false
	}
	below := 0
	for index := range actual {
		if expected[index] <= 0 {
			return false
		}
		if actual[index] < expected[index]*0.60 {
			below++
		}
	}
	return below >= 5
}

// CapacityConditionTransition returns whether a crossing emits and the next armed state.
func CapacityConditionTransition(armed bool, seatsUsed, seatsPurchased float64) (bool, bool) {
	if seatsPurchased <= 0 {
		return false, armed
	}
	ratio := seatsUsed / seatsPurchased
	if armed && ratio >= 0.85 {
		return true, false
	}
	if !armed && ratio <= 0.80 {
		return false, true
	}
	return false, armed
}

func cloneCommercialState(source model.JSONB) model.JSONB {
	result := model.JSONB{}
	for key, value := range source {
		result[key] = value
	}
	return result
}

func normalizeCommercialJSON(value model.JSONB) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}
