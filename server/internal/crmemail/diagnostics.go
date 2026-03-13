package crmemail

import (
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	SyncPhaseBackfill     = "backfill"
	SyncPhaseIncremental  = "incremental"
	SyncPhaseRecovery     = "recovery"
	SyncPhaseDisconnected = "disconnected"
	SyncPhaseError        = "error"
	SyncPhaseIdle         = "idle"
)

// BeginSyncCycle marks the start of a sync cycle.
func BeginSyncCycle(syncState model.JSONB, phase, historyID string, startedAt time.Time) model.JSONB {
	state := cloneSyncState(syncState)
	state["status"] = model.CRMEmailAccountStatusConnected
	state["phase"] = strings.TrimSpace(phase)
	state["last_attempt_at"] = formatSyncTime(startedAt)
	if historyID = strings.TrimSpace(historyID); historyID != "" {
		state["last_history_id"] = historyID
	}
	return state
}

// CompleteSyncCycle records a successful sync cycle and resets error state.
func CompleteSyncCycle(syncState model.JSONB, historyID string, completedAt time.Time, cycle model.CRMEmailSyncCycleStats) model.JSONB {
	state := cloneSyncState(syncState)
	state["status"] = model.CRMEmailAccountStatusConnected
	state["phase"] = SyncPhaseIdle
	state["last_success_at"] = formatSyncTime(completedAt)
	state["consecutive_failures"] = 0
	delete(state, "last_error")
	if historyID = strings.TrimSpace(historyID); historyID != "" {
		state["last_history_id"] = historyID
	}
	cycle.CompletedAt = timePtr(completedAt)
	state["last_cycle"] = mapSyncCycle(cycle)
	return state
}

// FailSyncCycle records a failed sync cycle and preserves the last successful checkpoint.
func FailSyncCycle(syncState model.JSONB, historyID string, failedAt time.Time, operation, code, message string, cycle *model.CRMEmailSyncCycleStats) model.JSONB {
	state := cloneSyncState(syncState)
	state["status"] = model.CRMEmailAccountStatusError
	state["phase"] = SyncPhaseError
	state["last_failure_at"] = formatSyncTime(failedAt)
	state["consecutive_failures"] = DecodeSyncDiagnostics(state, nil).ConsecutiveFailures + 1
	state["last_error"] = map[string]interface{}{
		"operation": strings.TrimSpace(operation),
		"code":      strings.TrimSpace(code),
		"message":   strings.TrimSpace(message),
	}
	if historyID = strings.TrimSpace(historyID); historyID != "" {
		state["last_history_id"] = historyID
	}
	if cycle != nil {
		state["last_cycle"] = mapSyncCycle(*cycle)
	}
	return state
}

// MarkDisconnected marks the mailbox as disconnected while preserving its checkpoint.
func MarkDisconnected(syncState model.JSONB, historyID string, at time.Time) model.JSONB {
	state := cloneSyncState(syncState)
	state["status"] = model.CRMEmailAccountStatusDisconnected
	state["phase"] = SyncPhaseDisconnected
	state["last_attempt_at"] = formatSyncTime(at)
	if historyID = strings.TrimSpace(historyID); historyID != "" {
		state["last_history_id"] = historyID
	}
	return state
}

// MarkConnectedIdle clears stale error state after a reconnect or successful lifecycle change.
func MarkConnectedIdle(syncState model.JSONB, historyID string) model.JSONB {
	state := cloneSyncState(syncState)
	state["status"] = model.CRMEmailAccountStatusConnected
	state["phase"] = SyncPhaseIdle
	state["consecutive_failures"] = 0
	delete(state, "last_error")
	if historyID = strings.TrimSpace(historyID); historyID != "" {
		state["last_history_id"] = historyID
	}
	return state
}

// DecodeSyncDiagnostics normalizes sync_state into a typed diagnostics model.
func DecodeSyncDiagnostics(syncState model.JSONB, fallbackHistoryID *string) model.CRMEmailSyncDiagnostics {
	diag := model.CRMEmailSyncDiagnostics{
		Status:              syncStateStringValue(syncState, "status"),
		Phase:               syncStateStringValue(syncState, "phase"),
		LastAttemptAt:       syncStateTimeValue(syncState, "last_attempt_at"),
		LastSuccessAt:       syncStateTimeValue(syncState, "last_success_at"),
		LastFailureAt:       syncStateTimeValue(syncState, "last_failure_at"),
		ConsecutiveFailures: syncStateIntValue(syncState, "consecutive_failures"),
	}

	if rawHistory := strings.TrimSpace(syncStateStringValue(syncState, "last_history_id")); rawHistory != "" {
		diag.LastHistoryID = &rawHistory
	} else if fallbackHistoryID != nil && strings.TrimSpace(*fallbackHistoryID) != "" {
		value := strings.TrimSpace(*fallbackHistoryID)
		diag.LastHistoryID = &value
	}

	if rawError, ok := syncState["last_error"].(map[string]interface{}); ok {
		errInfo := &model.CRMEmailSyncError{
			Operation: stringMapValue(rawError, "operation"),
			Code:      stringMapValue(rawError, "code"),
			Message:   stringMapValue(rawError, "message"),
		}
		if errInfo.Operation != "" || errInfo.Code != "" || errInfo.Message != "" {
			diag.LastError = errInfo
		}
	}

	if rawCycle, ok := syncState["last_cycle"].(map[string]interface{}); ok {
		cycle := &model.CRMEmailSyncCycleStats{
			Mode:                stringMapValue(rawCycle, "mode"),
			StartedAt:           timeMapValue(rawCycle, "started_at"),
			CompletedAt:         timeMapValue(rawCycle, "completed_at"),
			MessagesSeen:        intMapValue(rawCycle, "messages_seen"),
			MessagesStored:      intMapValue(rawCycle, "messages_stored"),
			DuplicatesSkipped:   intMapValue(rawCycle, "duplicates_skipped"),
			FilteredSkipped:     intMapValue(rawCycle, "filtered_skipped"),
			InternalSkipped:     intMapValue(rawCycle, "internal_skipped"),
			ContactsCreated:     intMapValue(rawCycle, "contacts_created"),
			AssociationsWritten: intMapValue(rawCycle, "associations_written"),
			ThreadsTouched:      intMapValue(rawCycle, "threads_touched"),
			RecoveryTriggered:   boolMapValue(rawCycle, "recovery_triggered"),
		}
		diag.LastCycle = cycle
	}

	return diag
}

func mapSyncCycle(cycle model.CRMEmailSyncCycleStats) map[string]interface{} {
	result := map[string]interface{}{
		"mode":                 strings.TrimSpace(cycle.Mode),
		"messages_seen":        cycle.MessagesSeen,
		"messages_stored":      cycle.MessagesStored,
		"duplicates_skipped":   cycle.DuplicatesSkipped,
		"filtered_skipped":     cycle.FilteredSkipped,
		"internal_skipped":     cycle.InternalSkipped,
		"contacts_created":     cycle.ContactsCreated,
		"associations_written": cycle.AssociationsWritten,
		"threads_touched":      cycle.ThreadsTouched,
		"recovery_triggered":   cycle.RecoveryTriggered,
	}
	if cycle.StartedAt != nil {
		result["started_at"] = formatSyncTime(*cycle.StartedAt)
	}
	if cycle.CompletedAt != nil {
		result["completed_at"] = formatSyncTime(*cycle.CompletedAt)
	}
	return result
}

func cloneSyncState(syncState model.JSONB) model.JSONB {
	if syncState == nil {
		return model.JSONB{}
	}
	clone := make(model.JSONB, len(syncState))
	for key, value := range syncState {
		clone[key] = value
	}
	return clone
}

func formatSyncTime(ts time.Time) string {
	return ts.UTC().Format(time.RFC3339Nano)
}

func timePtr(ts time.Time) *time.Time {
	ts = ts.UTC()
	return &ts
}

func syncStateStringValue(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	switch value := values[key].(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func stringMapValue(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	switch value := values[key].(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func syncStateIntValue(values map[string]interface{}, key string) int {
	if values == nil {
		return 0
	}
	return numericToInt(values[key])
}

func intMapValue(values map[string]interface{}, key string) int {
	if values == nil {
		return 0
	}
	return numericToInt(values[key])
}

func boolMapValue(values map[string]interface{}, key string) bool {
	if values == nil {
		return false
	}
	value, _ := values[key].(bool)
	return value
}

func syncStateTimeValue(values map[string]interface{}, key string) *time.Time {
	if values == nil {
		return nil
	}
	return parseSyncTime(values[key])
}

func timeMapValue(values map[string]interface{}, key string) *time.Time {
	if values == nil {
		return nil
	}
	return parseSyncTime(values[key])
}

func parseSyncTime(value interface{}) *time.Time {
	raw, ok := value.(string)
	if !ok {
		return nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func numericToInt(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}
