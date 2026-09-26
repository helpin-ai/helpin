package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DispatchDue runs on the existing Temporal scheduler; durable claims and delivery
// records are the authority, so a retry never blindly repeats an external send.
func (s *CRMOutreachService) DispatchDue(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	entryCtx, entryCancel := context.WithTimeout(ctx, 3*time.Second)
	entryErr := s.processEntries(entryCtx)
	entryCancel()
	if errors.Is(entryErr, context.DeadlineExceeded) {
		entryErr = nil
	}
	visited := []string{}
	for range 100 {
		row, err := s.repo.Claim(ctx, s.now(), visited...)
		if err != nil {
			return err
		}
		if row == nil {
			return entryErr
		}
		visited = append(visited, row.AccountID)
		if err = s.processRecipient(ctx, row); err != nil {
			// Delivery records survive this failure. The next claim reconciles instead of resending.
			_ = s.repo.Finish(ctx, row, "failed", err.Error(), s.now().Add(time.Minute), row.StepIndex)
		}
	}
	return entryErr
}
func (s *CRMOutreachService) next(row *model.CRMSequenceEnrollment) (string, time.Time, int) {
	index := row.StepIndex + 1
	if index >= len(row.Steps) {
		return "completed", s.now(), index
	}
	return "active", NextCRMSequenceWindow(s.now().AddDate(0, 0, row.Steps[index].DelayDays), *row), index
}
func (s *CRMOutreachService) advance(ctx context.Context, row *model.CRMSequenceEnrollment) error {
	status, next, index := s.next(row)
	return s.repo.Finish(ctx, row, status, "", next, index)
}
func (s *CRMOutreachService) action(row *model.CRMSequenceEnrollment, d *model.CRMSequenceDelivery) model.CRMPlaybookEmailAction {
	return model.CRMPlaybookEmailAction{AccountID: row.AccountID, To: row.Email, Subject: d.Subject, BodyHTML: d.BodyHTML}
}
func (s *CRMOutreachService) processRecipient(ctx context.Context, row *model.CRMSequenceEnrollment) error {
	var err error
	ctx, err = s.access(ctx, row.WorkspaceID, row.OwnerID)
	if err != nil {
		return err
	}
	reason, err := s.repo.StopReason(ctx, row)
	if err != nil {
		return err
	}
	if reason != "" {
		return s.repo.Finish(ctx, row, reason, "", s.now(), row.StepIndex)
	}
	seq, err := s.repo.Sequence(ctx, row.WorkspaceID, row.SequenceID)
	if err != nil {
		return err
	}
	if seq.Status != "active" {
		return s.repo.Finish(ctx, row, row.Status, "Sequence is paused", s.now().Add(5*time.Minute), row.StepIndex)
	}
	if row.StepIndex >= len(row.Steps) {
		return s.repo.Finish(ctx, row, "completed", "", s.now(), row.StepIndex)
	}
	step := row.Steps[row.StepIndex]
	now := s.now()
	window := NextCRMSequenceWindow(now, *row)
	if window.After(now) {
		return s.repo.Finish(ctx, row, row.Status, "", window, row.StepIndex)
	}
	if step.Kind == "email" {
		if err := s.mail.ValidateActionSender(ctx, row.WorkspaceID, row.AccountID, row.OwnerID); err != nil {
			return err
		}
		account, err := s.accounts.GetAccountByIDForWorkspace(ctx, row.WorkspaceID, row.AccountID)
		if err != nil {
			return err
		}
		if account == nil || account.LastSyncedAt == nil || now.Sub(*account.LastSyncedAt) > 15*time.Minute {
			return s.repo.Finish(ctx, row, row.Status, "Waiting for mailbox sync before checking replies", now.Add(5*time.Minute), row.StepIndex)
		}
		if step.Mode == "review" {
			return s.repo.Finish(ctx, row, "needs_review", "", now, row.StepIndex)
		}
	}
	d := &model.CRMSequenceDelivery{ID: uuid.NewString(), WorkspaceID: row.WorkspaceID, EnrollmentID: row.ID, StepIndex: row.StepIndex, AccountID: row.AccountID, Kind: step.Kind, Status: "sending", Subject: step.Subject, BodyHTML: step.BodyHTML, CreatedAt: now, UpdatedAt: now}
	fresh, err := s.repo.PrepareDelivery(ctx, row, d, now)
	if errors.Is(err, repository.ErrOutreachRateLimit) {
		return s.repo.Finish(ctx, row, "active", "Waiting for mailbox sending capacity", now.Add(5*time.Minute), row.StepIndex)
	}
	if err != nil {
		return err
	}
	if step.Kind == "task" {
		return s.processTask(ctx, row, d, step, fresh)
	}
	if !fresh {
		if d.Status == "sent" {
			return s.advance(ctx, row)
		}
		found, id, err := s.mail.ReconcileActionEmail(ctx, row.WorkspaceID, row.OwnerID, d.ID, s.action(row, d))
		if err != nil {
			return s.repo.Finish(ctx, row, "uncertain", "Could not confirm delivery. Check delivery before continuing.", now, row.StepIndex)
		}
		if !found {
			return s.repo.Finish(ctx, row, "uncertain", "Delivery is unconfirmed. Check the sender's Sent folder; no automatic resend will occur.", now, row.StepIndex)
		}
		d.Status = "sent"
		d.ResultID = stringValue(id)
		if err := s.repo.SaveDelivery(ctx, d); err != nil {
			return err
		}
		return s.advance(ctx, row)
	}
	// Recheck stop criteria after preparing the intent, immediately before the provider call.
	if reason, err := s.repo.StopReason(ctx, row); err != nil {
		return err
	} else if reason != "" {
		return s.repo.Finish(ctx, row, reason, "", now, row.StepIndex)
	}
	firstEmail := true
	for _, prior := range row.Steps[:row.StepIndex] {
		if prior.Kind == "email" {
			firstEmail = false
		}
	}
	sendCtx := context.WithValue(ctx, sequenceBudgetKey{}, sequenceBudget{SequenceID: row.SequenceID, FirstEmail: firstEmail, DailyNew: seq.DailyNewRecipients})
	message, err := s.mail.SendActionEmail(sendCtx, row.WorkspaceID, row.OwnerID, d.ID, s.action(row, d))
	if err != nil {
		var capacity *repository.SendCapacityError
		if errors.As(err, &capacity) {
			d.Status = "deferred"
			d.Error = capacity.Error()
			if saveErr := s.repo.SaveDelivery(ctx, d); saveErr != nil {
				return saveErr
			}
			return s.repo.Finish(ctx, row, "active", capacity.Error(), NextCRMSequenceWindow(capacity.RetryAt, *row), row.StepIndex)
		}
		if definitiveRejection(err) {
			d.Status = "rejected"
			d.Error = err.Error()
			if saveErr := s.repo.SaveDelivery(ctx, d); saveErr != nil {
				return saveErr
			}
			return s.repo.Finish(ctx, row, "failed", "The email provider rejected this email. Check the mailbox and recipient before continuing.", now, row.StepIndex)
		}
		d.Error = err.Error()
		_ = s.repo.SaveDelivery(ctx, d)
		return s.repo.Finish(ctx, row, "uncertain", "Delivery could not be confirmed. Check delivery before continuing.", now, row.StepIndex)
	}
	if message == nil {
		return s.repo.Finish(ctx, row, "uncertain", "Delivery could not be confirmed", now, row.StepIndex)
	}
	d.Status = "sent"
	d.ResultID = message.ID
	d.UpdatedAt = s.now()
	if err := s.repo.SaveDelivery(ctx, d); err != nil {
		return err
	}
	if row.DealID != "" && message.ThreadID != nil {
		if err := s.mail.LinkThreadDeal(ctx, row.WorkspaceID, *message.ThreadID, &row.DealID); err != nil {
			d.Error = "Sent; deal link could not be saved"
			_ = s.repo.SaveDelivery(ctx, d)
		}
	}
	return s.advance(ctx, row)
}
func (s *CRMOutreachService) processTask(ctx context.Context, row *model.CRMSequenceEnrollment, d *model.CRMSequenceDelivery, step model.CRMSequenceStep, fresh bool) error {
	actor := authorization.GetActor(ctx)
	allowed, err := s.auth.CanAccessModule(ctx, actor, model.ModulePM)
	if err != nil {
		return err
	}
	if !allowed || !s.auth.Can(actor, authorization.PermPMEdit) {
		return fmt.Errorf("Tasks access is required for this step")
	}
	externalID := "crm-sequence-" + d.ID
	taskID, state, err := s.repo.NativeTask(ctx, row.WorkspaceID, externalID)
	if err != nil {
		return err
	}
	if taskID == "" {
		if !fresh {
			return s.repo.Finish(ctx, row, "failed", "Task creation was interrupted; check Tasks before continuing", s.now(), row.StepIndex)
		}
		if actor.WorkspaceMemberID == "" {
			return fmt.Errorf("active workspace membership is required")
		}
		if s.tasks == nil {
			return fmt.Errorf("task creation is unavailable")
		}
		description := "Follow up with " + row.ContactName + " (" + row.Email + ") as part of " + row.SequenceName
		task, err := s.tasks.Create(ctx, model.CreateTaskRequest{WorkspaceID: row.WorkspaceID, Name: step.TaskName, Description: &description, TeamID: &step.TeamID, OwnerMemberIDs: []string{actor.WorkspaceMemberID}, ExternalID: &externalID}, row.OwnerID)
		if err != nil {
			return err
		}
		taskID = task.Task.ID
	}
	if err := s.repo.LinkTask(ctx, row.WorkspaceID, taskID, row.ContactID, row.DealID); err != nil {
		return err
	}
	d.Status = "task_created"
	d.ResultID = taskID
	if err := s.repo.SaveDelivery(ctx, d); err != nil {
		return err
	}
	if state == "done" || state == "completed" {
		d.Status = "completed"
		if err := s.repo.SaveDelivery(ctx, d); err != nil {
			return err
		}
		return s.advance(ctx, row)
	}
	if state == "canceled" || state == "cancelled" {
		return s.repo.Finish(ctx, row, "stopped", "Follow-up task was canceled", s.now(), row.StepIndex)
	}
	return s.repo.Finish(ctx, row, "waiting_task", "", s.now().Add(5*time.Minute), row.StepIndex)
}
func (s *CRMOutreachService) checkDelivery(ctx context.Context, row *model.CRMSequenceEnrollment) error {
	if row.Status != "uncertain" {
		return fmt.Errorf("this delivery does not need reconciliation")
	}
	deliveries, err := s.repo.Deliveries(ctx, row.WorkspaceID, row.ID)
	if err != nil {
		return err
	}
	for _, d := range deliveries {
		if d.StepIndex == row.StepIndex && d.Kind == "email" {
			found, id, err := s.mail.ReconcileActionEmail(ctx, row.WorkspaceID, row.OwnerID, d.ID, s.action(row, &d))
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("delivery is still unconfirmed; no email has been resent")
			}
			d.Status = "sent"
			d.ResultID = stringValue(id)
			d.Error = ""
			status, next, index := s.next(row)
			return s.repo.ResolveDelivery(ctx, row, &d, next, index, status)
		}
	}
	return fmt.Errorf("no delivery record found")
}
func (s *CRMOutreachService) processEntries(ctx context.Context) error {
	sequences, err := s.repo.EntrySequences(ctx)
	if err != nil {
		return err
	}
	for _, seq := range sequences {
		events, err := s.repo.EntryEvents(ctx, seq)
		if err != nil {
			return err
		}
		for _, event := range events {
			contact, err := s.repo.PrimaryDealContact(ctx, seq.WorkspaceID, event.EntityID)
			issue := ""
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					if err := s.repo.EntryIssue(ctx, seq, "Enrollment temporarily unavailable; retrying automatically"); err != nil {
						return err
					}
					break
				}
				issue = "Deal has no primary contact"
			} else {
				_, err = s.Enroll(ctx, seq.WorkspaceID, seq.OwnerID, seq.ID, model.CRMSequenceEnrollRequest{AccountID: seq.EntryAccountID, ContactIDs: []string{contact}, DealID: event.EntityID, Version: seq.Version})
				if err != nil {
					var ineligible *outreachIneligibleError
					if errors.As(err, &ineligible) {
						issue = ineligible.Error()
					} else {
						if saveErr := s.repo.EntryIssue(ctx, seq, "Enrollment temporarily unavailable; retrying automatically"); saveErr != nil {
							return saveErr
						}
						break
					}
				}
			}
			if err := s.repo.AdvanceEntry(ctx, seq, event, issue); err != nil {
				return err
			}
		}
	}
	return nil
}
