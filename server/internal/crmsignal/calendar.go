package crmsignal

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CalendarPayload builds a targetable source payload for calendar signal
// detection. Events without a linked contact or deal are intentionally skipped.
func CalendarPayload(event *model.CRMCalendarEvent) (*model.SignalSourcePayload, bool) {
	if event == nil {
		return nil, false
	}
	payload := model.PayloadFromCalendarEvent(event)
	if payload.SourceID == "" || (payload.ContactID == nil && payload.DealID == nil) {
		return nil, false
	}
	return &payload, true
}

// CalendarWorkflowKey versions the workflow by canonical payload content so a
// provider update (accepted → declined, rescheduled, cancelled) is reanalyzed.
func CalendarWorkflowKey(payload model.SignalSourcePayload) string {
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("calendar-%s-%x", payload.SourceID, sum[:8])
}
