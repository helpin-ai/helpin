package service

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type crmCheckpointStore interface {
	DeliverCheckpoint(context.Context, model.AutomationScheduledEvent, time.Time, func(model.CRMSituationItem) string) error
}

// CRMSituationCheckpointService reevaluates a commitment using current CRM facts.
// The registered shared Automation consumer never sends messages or creates PM tasks.
type CRMSituationCheckpointService struct{ store crmCheckpointStore }

// NewCRMSituationCheckpointService binds the transactional CRM checkpoint adapter.
func NewCRMSituationCheckpointService(store crmCheckpointStore) *CRMSituationCheckpointService {
	return &CRMSituationCheckpointService{store: store}
}

// HandleScheduledEvent records what currently needs attention, not a customer outcome.
func (s *CRMSituationCheckpointService) HandleScheduledEvent(ctx context.Context, event model.AutomationScheduledEvent, now time.Time) error {
	return s.store.DeliverCheckpoint(ctx, event, now, evaluateCRMCheckpoint)
}

func evaluateCRMCheckpoint(item model.CRMSituationItem) string {
	if (item.Situation.NextActionOwnerMemberID != nil && !item.NextActionOwnerAvailable) ||
		(item.Situation.NextActionOwnerMemberID == nil && !item.OwnerAvailable) {
		return "needs_owner"
	}
	if item.FailedActionCount+item.UncertainActionCount > 0 {
		return model.CRMSituationAutomationFailed
	}
	if item.PendingActionCount > 0 {
		return model.CRMSituationNeedsApproval
	}
	if item.ManualActionCount > 0 || item.Situation.NextStep == "" {
		return model.CRMSituationNeedsContext
	}
	if item.ExecutingActionCount > 0 {
		return model.CRMSituationWaitingWork
	}
	return model.CRMSituationFollowUpDue
}
