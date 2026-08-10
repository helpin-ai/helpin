package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	customerIOOutboxLease       = 5 * time.Minute
	customerIOOutboxMaxAttempts = 10
	customerIOOutboxBatchSize   = 25
)

// CustomerIOLifecycleOutboxWorker delivers durable lifecycle events.
type CustomerIOLifecycleOutboxWorker struct {
	repo          *repository.CustomerIOLifecycleOutboxRepository
	workspaceRepo *repository.WorkspaceRepository
	identity      *CustomerIOIdentityService
	now           func() time.Time
}

func NewCustomerIOLifecycleOutboxWorker(repo *repository.CustomerIOLifecycleOutboxRepository, workspaceRepo *repository.WorkspaceRepository, identity *CustomerIOIdentityService) *CustomerIOLifecycleOutboxWorker {
	return &CustomerIOLifecycleOutboxWorker{repo: repo, workspaceRepo: workspaceRepo, identity: identity, now: time.Now}
}

// ProcessDue claims and delivers one bounded batch. Disabled credentials leave
// rows untouched, including their attempt counters.
func (w *CustomerIOLifecycleOutboxWorker) ProcessDue(ctx context.Context) error {
	if w == nil || w.repo == nil || w.identity == nil || !w.identity.Enabled() {
		return nil
	}
	now := w.now().UTC()
	rows, err := w.repo.ClaimDue(ctx, now, customerIOOutboxLease, customerIOOutboxBatchSize)
	if err != nil {
		return err
	}
	for i := range rows {
		if err := w.deliver(ctx, &rows[i]); err != nil {
			slog.ErrorContext(ctx, "customer.io outbox delivery state update failed", "error", err, "outbox_id", rows[i].ID)
		}
	}
	return nil
}

func (w *CustomerIOLifecycleOutboxWorker) deliver(ctx context.Context, row *model.CustomerIOOutbox) error {
	token := ""
	if row.ClaimToken != nil {
		token = *row.ClaimToken
	}
	var snapshot []model.CustomerIOOutboxRecipient
	if err := json.Unmarshal(row.RecipientSnapshot, &snapshot); err != nil {
		_, markErr := w.repo.MarkFailed(ctx, row.ID, token, "decode recipient snapshot: "+err.Error())
		return markErr
	}
	var attrs map[string]any
	if err := json.Unmarshal(row.Attributes, &attrs); err != nil {
		_, markErr := w.repo.MarkFailed(ctx, row.ID, token, "decode attributes: "+err.Error())
		return markErr
	}
	if attrs == nil {
		attrs = make(map[string]any)
	}
	active := map[string]bool{}
	workspaceID := ""
	if row.WorkspaceID != nil {
		workspaceID = *row.WorkspaceID
	}
	members, err := w.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return w.handleDeliveryError(ctx, row, token, err)
	}
	for _, member := range members {
		active[member.UserID] = true
	}
	if workspace, err := w.workspaceRepo.GetByID(ctx, workspaceID); err != nil {
		return w.handleDeliveryError(ctx, row, token, err)
	} else if workspace != nil {
		attrs["workspace_id"], attrs["workspace_name"], attrs["workspace_slug"] = workspace.ID, workspace.Name, workspace.Slug
		if workspace.OrganizationID != nil {
			attrs["organization_id"] = *workspace.OrganizationID
		}
	}
	for _, recipient := range snapshot {
		if !active[recipient.UserID] {
			continue
		}
		properties := cloneAnalyticsAttributes(attrs)
		properties["workspace_role"] = recipient.WorkspaceRole
		properties["membership_status"] = model.WorkspaceMemberStatusActive
		if err := w.identity.TrackOutboxEvent(ctx, row.ID, CustomerIOEvent{UserID: recipient.UserID, Name: row.EventName, OccurredAt: row.OccurredAt, Attributes: properties}); err != nil {
			return w.handleDeliveryError(ctx, row, token, err)
		}
	}
	_, err = w.repo.MarkDelivered(ctx, row.ID, token)
	return err
}

func (w *CustomerIOLifecycleOutboxWorker) handleDeliveryError(ctx context.Context, row *model.CustomerIOOutbox, token string, deliveryErr error) error {
	retry, retryAfter := customerIOOutboxRetry(deliveryErr)
	if row.Attempts >= customerIOOutboxMaxAttempts {
		retry = false
	}
	if !retry {
		_, err := w.repo.MarkFailed(ctx, row.ID, token, deliveryErr.Error())
		return err
	}
	if retryAfter <= 0 {
		shift := row.Attempts - 1
		if shift < 0 {
			shift = 0
		}
		if shift > 8 {
			shift = 8
		}
		retryAfter = time.Second * time.Duration(1<<shift)
		retryAfter += time.Duration(rand.Int63n(int64(retryAfter/4 + 1)))
	}
	if retryAfter > time.Hour {
		retryAfter = time.Hour
	}
	_, err := w.repo.ScheduleRetry(ctx, row.ID, token, w.now().UTC().Add(retryAfter), deliveryErr.Error())
	return err
}

func customerIOOutboxRetry(err error) (bool, time.Duration) {
	var deliveryErr *CustomerIODeliveryError
	if !errors.As(err, &deliveryErr) {
		return true, 0
	}
	status := deliveryErr.StatusCode
	return status == 0 || status == 408 || status == 429 || status >= 500, deliveryErr.RetryAfter
}

// Run polls until cancellation.
func (w *CustomerIOLifecycleOutboxWorker) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if err := w.ProcessDue(ctx); err != nil {
		slog.ErrorContext(ctx, "customer.io outbox poll failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.ProcessDue(ctx); err != nil {
				slog.ErrorContext(ctx, "customer.io outbox poll failed", "error", err)
			}
		}
	}
}
