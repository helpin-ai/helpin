package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	gmailsync "github.com/helpin-ai/helpin/server/internal/sync"
)

type sequenceBudgetKey struct{}
type sequenceBudget struct {
	SequenceID string
	FirstEmail bool
	DailyNew   int
}

func (s *CRMEmailService) reserveEmail(ctx context.Context, a *model.CRMEmailAccount, intentIDs []string) (string, error) {
	id := uuid.NewString()
	if len(intentIDs) > 0 {
		id = intentIDs[0]
	}
	request := model.CRMEmailSendReservation{ID: id, Automated: len(intentIDs) > 0}
	daily := 0
	if budget, ok := ctx.Value(sequenceBudgetKey{}).(sequenceBudget); ok {
		request.SequenceID = budget.SequenceID
		request.FirstEmail = budget.FirstEmail
		daily = budget.DailyNew
	}
	return id, s.emailRepo.ReserveSend(ctx, a, request, daily, time.Now().UTC())
}
func providerCooldown(err error, now time.Time) (time.Time, bool) {
	var apiErr *gmailsync.GmailAPIError
	if !errors.As(err, &apiErr) {
		return time.Time{}, false
	}
	body := strings.ToLower(apiErr.Body)
	limited := apiErr.StatusCode == 429 || apiErr.StatusCode == 403 && (strings.Contains(body, "ratelimit") || strings.Contains(body, "quota") || strings.Contains(body, "dailylimit") || strings.Contains(body, "daily limit"))
	if !limited {
		return time.Time{}, false
	}
	until := now.Add(15 * time.Minute)
	if strings.Contains(body, "dailylimit") || strings.Contains(body, "daily limit") {
		until = now.Add(24 * time.Hour)
	}
	if seconds, e := strconv.Atoi(apiErr.RetryAfter); e == nil && seconds > 0 {
		until = now.Add(time.Duration(min(seconds, 86400)) * time.Second)
	} else if date, e := http.ParseTime(apiErr.RetryAfter); e == nil && date.After(now) {
		until = date
	}
	return until, true
}
func definitiveRejection(err error) bool {
	var apiErr *gmailsync.GmailAPIError
	return errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != http.StatusRequestTimeout
}
func (s *CRMEmailService) finishEmailAttempt(ctx context.Context, a *model.CRMEmailAccount, id string, sendErr error, accepted bool) error {
	// The provider call can exhaust its deadline. Persist the outcome with a short independent cleanup context.
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if until, limited := providerCooldown(sendErr, time.Now().UTC()); limited {
		// Store the cooldown first. If persistence fails, retain the reservation as uncertain.
		if err := s.emailRepo.CooldownMailbox(cleanup, a, until); err != nil {
			return err
		}
		if err := s.emailRepo.FinishSend(cleanup, id, "rejected"); err != nil {
			return err
		}
		return &repository.SendCapacityError{Reason: "provider_cooldown", RetryAt: until}
	}
	if definitiveRejection(sendErr) {
		if err := s.emailRepo.FinishSend(cleanup, id, "rejected"); err != nil {
			return err
		}
	}
	if accepted {
		if err := s.emailRepo.FinishSend(cleanup, id, "sent"); err != nil {
			slog.ErrorContext(cleanup, "record accepted CRM email reservation", "workspace_id", a.WorkspaceID, "reservation_id", id, "error", err)
		}
	}
	return sendErr
}
