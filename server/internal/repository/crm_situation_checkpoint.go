package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func situationCheckpointKey(id string, revision int64) string {
	return fmt.Sprintf("crm.checkpoint:%s:%d", id, revision)
}

func syncSituationCheckpoint(tx *gorm.DB, ws, id string, revision int64, state model.CRMSituationWorkState, now time.Time) error {
	events := NewAutomationScheduledEventRepository(tx)
	if err := events.CancelTarget(tx.Statement.Context, ws, model.CRMCheckpointEvent, "crm_situation", id, now); err != nil {
		return fmt.Errorf("revoke old CRM checkpoint: %w", err)
	}
	if state.Lifecycle != model.CRMSituationOpen || state.NextCheckpointAt == nil {
		return nil
	}
	return events.Enqueue(tx.Statement.Context, model.AutomationScheduledEvent{
		WorkspaceID: ws, EventKey: situationCheckpointKey(id, revision), Kind: model.CRMCheckpointEvent,
		TargetType: "crm_situation", TargetID: id, ExpectedRevision: revision, DueAt: *state.NextCheckpointAt,
	})
}

// DeliverCheckpoint reads current CRM facts and atomically records reevaluation
// under the same workspace lock used by pause/close and approval admission.
// It never starts an Agent, approves an action, or writes a customer outcome.
func (r *CRMSituationRepository) DeliverCheckpoint(
	ctx context.Context, claim model.AutomationScheduledEvent, now time.Time,
	evaluate func(model.CRMSituationItem) string,
) error {
	started := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, claim.WorkspaceID); err != nil {
			return err
		}
		events := NewAutomationScheduledEventRepository(tx)
		// Include time spent waiting for the domain lock in the lease check.
		checkTime := func() time.Time { return now.Add(time.Since(started)).UTC().Truncate(time.Microsecond) }
		event, err := events.LockClaim(ctx, claim, checkTime())
		if err != nil {
			return err
		}
		if event.Kind != model.CRMCheckpointEvent || event.TargetType != "crm_situation" {
			return fmt.Errorf("checkpoint handler received another event type")
		}
		var item model.CRMSituationItem
		err = situationReadQuery(tx, event.WorkspaceID).Where("s.id = ?", event.TargetID).Take(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return events.Complete(ctx, *event, "stale", checkTime())
		}
		if err != nil {
			return err
		}
		state := item.Situation
		if state.Lifecycle != model.CRMSituationOpen || state.Revision != event.ExpectedRevision ||
			state.NextCheckpointAt == nil || !state.NextCheckpointAt.Equal(event.DueAt) {
			return events.Complete(ctx, *event, "stale", checkTime())
		}
		if event.DueAt.After(now) {
			return fmt.Errorf("checkpoint has not reached its due time")
		}
		return events.Complete(ctx, *event, evaluate(item), checkTime())
	})
}
