package service

import "github.com/helpin-ai/helpin/server/internal/model"

// resolveHandoffState maps availability at handoff time to a customer-facing state.
// Presence trumps business hours: an available recipient always yields "live".
func resolveHandoffState(hasAvailableRecipient bool, withinOfficeHours bool) string {
	if hasAvailableRecipient {
		return model.HandoffStateLive
	}
	if withinOfficeHours {
		return model.HandoffStateBusy
	}
	return model.HandoffStateAfterHours
}
