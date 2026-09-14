package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrSendAlreadyReserved prevents replay after uncertain external delivery.
var ErrSendAlreadyReserved = errors.New("email delivery is already reserved; confirm delivery before retrying")

// SendCapacityError describes a safe, unsent deferral.
type SendCapacityError struct {
	Reason  string
	RetryAt time.Time
}

func (e *SendCapacityError) Error() string {
	switch e.Reason {
	case "daily_starts":
		return "Waiting for the daily new-recipient limit"
	case "provider_cooldown":
		return "Waiting for the email provider to resume sending"
	default:
		return "Waiting for mailbox sending capacity"
	}
}
func mailboxKey(a *model.CRMEmailAccount) string {
	return a.Provider + ":" + strings.ToLower(strings.TrimSpace(a.EmailAddress))
}
func sendingPolicy(tx *gorm.DB, key string, lock bool) (*model.CRMMailboxSendingPolicy, error) {
	p := model.CRMMailboxSendingPolicy{MailboxKey: key, DailyLimit: 100, ManualReserve: 10, MinIntervalSeconds: 60}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p).Error; err != nil {
		return nil, err
	}
	q := tx.Where("mailbox_key = ?", key)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return &p, q.First(&p).Error
}
func sendingUsage(tx *gorm.DB, key string, now time.Time) ([]model.CRMEmailSendReservation, error) {
	rows := []model.CRMEmailSendReservation{}
	err := tx.Where("mailbox_key = ? AND status <> 'rejected' AND created_at > ?", key, now.Add(-24*time.Hour)).Order("created_at").Find(&rows).Error
	return rows, err
}
func sendingWait(p *model.CRMMailboxSendingPolicy, rows []model.CRMEmailSendReservation, automated bool, now time.Time) *SendCapacityError {
	var next time.Time
	reason := "mailbox_capacity"
	if p.CooldownUntil != nil && p.CooldownUntil.After(now) {
		next = *p.CooldownUntil
		reason = "provider_cooldown"
	}
	extend := func(t time.Time) {
		if t.After(next) {
			next = t
		}
	}
	if len(rows) >= p.DailyLimit && len(rows) > 0 {
		extend(rows[len(rows)-p.DailyLimit].CreatedAt.Add(24 * time.Hour))
	}
	if automated {
		auto := []model.CRMEmailSendReservation{}
		for _, row := range rows {
			if row.Automated {
				auto = append(auto, row)
			}
		}
		cap := p.DailyLimit - p.ManualReserve
		if len(auto) >= cap && len(auto) > 0 {
			extend(auto[len(auto)-cap].CreatedAt.Add(24 * time.Hour))
		}
		if len(rows) > 0 {
			extend(rows[len(rows)-1].CreatedAt.Add(time.Duration(p.MinIntervalSeconds) * time.Second))
		}
	}
	if next.After(now) {
		return &SendCapacityError{Reason: reason, RetryAt: next}
	}
	return nil
}

// ReserveSend atomically enforces daily, spacing, cooldown and first-email limits.
func (r *CRMEmailRepository) ReserveSend(ctx context.Context, a *model.CRMEmailAccount, request model.CRMEmailSendReservation, newLimit int, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if request.SequenceID != "" {
			var seq model.CRMEmailSequence
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", request.SequenceID).First(&seq).Error; err != nil {
				return err
			}
		}
		p, err := sendingPolicy(tx, mailboxKey(a), true)
		if err != nil {
			return err
		}
		var old model.CRMEmailSendReservation
		err = tx.Where("id = ?", request.ID).First(&old).Error
		if err == nil && old.Status != "rejected" {
			return ErrSendAlreadyReserved
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		rows, err := sendingUsage(tx, p.MailboxKey, now)
		if err != nil {
			return err
		}
		if wait := sendingWait(p, rows, request.Automated, now); wait != nil {
			return wait
		}
		if request.FirstEmail && newLimit > 0 {
			starts := []model.CRMEmailSendReservation{}
			if err := tx.Where("sequence_id = ? AND first_email = ? AND status <> 'rejected' AND created_at > ?", request.SequenceID, true, now.Add(-24*time.Hour)).Order("created_at").Find(&starts).Error; err != nil {
				return err
			}
			if len(starts) >= newLimit {
				return &SendCapacityError{Reason: "daily_starts", RetryAt: starts[len(starts)-newLimit].CreatedAt.Add(24 * time.Hour)}
			}
		}
		request.MailboxKey = p.MailboxKey
		request.AccountID = a.ID
		request.WorkspaceID = a.WorkspaceID
		request.Status = "pending"
		request.CreatedAt = now
		// A confirmed-rejected intent can be retried later. Its quota timestamp must
		// move with the new attempt; GORM's UpdateAll preserves auto-create times.
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"mailbox_key", "workspace_id", "account_id", "sequence_id", "automated", "first_email", "status", "created_at"}),
		}).Create(&request).Error
	})
}

// FinishSend records confirmed acceptance or rejection; uncertain sends remain reserved.
func (r *CRMEmailRepository) FinishSend(ctx context.Context, id, status string) error {
	return r.db.WithContext(ctx).Model(&model.CRMEmailSendReservation{}).Where("id = ?", id).Update("status", status).Error
}

// CooldownMailbox postpones all sending after a definitive provider throttle.
func (r *CRMEmailRepository) CooldownMailbox(ctx context.Context, a *model.CRMEmailAccount, until time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, err := sendingPolicy(tx, mailboxKey(a), true)
		if err != nil {
			return err
		}
		if p.CooldownUntil != nil && p.CooldownUntil.After(until) {
			return nil
		}
		return tx.Model(p).Update("cooldown_until", until).Error
	})
}

// UpdateSendingPolicy updates only editable limits, preserving provider cooldowns.
func (r *CRMEmailRepository) UpdateSendingPolicy(ctx context.Context, a *model.CRMEmailAccount, p model.CRMMailboxSendingPolicy) error {
	if p.DailyLimit < 1 || p.DailyLimit > 1000 || p.ManualReserve < 0 || p.ManualReserve >= p.DailyLimit || p.MinIntervalSeconds < 60 || p.MinIntervalSeconds > 3600 {
		return fmt.Errorf("use a daily limit of 1–1000, a smaller personal reserve, and spacing of 60–3600 seconds")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := sendingPolicy(tx, mailboxKey(a), true)
		if err != nil {
			return err
		}
		return tx.Model(current).Updates(map[string]any{"daily_limit": p.DailyLimit, "manual_reserve": p.ManualReserve, "min_interval_seconds": p.MinIntervalSeconds}).Error
	})
}

// SendingCapacity reports Helpin usage, excluding external mail sent directly in Gmail.
func (r *CRMEmailRepository) SendingCapacity(ctx context.Context, a *model.CRMEmailAccount, now time.Time) (*model.CRMMailboxCapacity, error) {
	tx := r.db.WithContext(ctx)
	p, err := sendingPolicy(tx, mailboxKey(a), false)
	if err != nil {
		return nil, err
	}
	rows, err := sendingUsage(tx, p.MailboxKey, now)
	if err != nil {
		return nil, err
	}
	out := &model.CRMMailboxCapacity{CRMMailboxSendingPolicy: *p, AccountID: a.ID, Email: a.EmailAddress, Used: len(rows), Remaining: max(0, p.DailyLimit-len(rows))}
	auto := 0
	for _, row := range rows {
		if row.Automated {
			auto++
		}
		if row.Status == "sent" {
			out.Sent++
		}
	}
	out.SequenceRemaining = max(0, min(out.Remaining, p.DailyLimit-p.ManualReserve-auto))
	if wait := sendingWait(p, rows, true, now); wait != nil {
		out.NextAvailableAt = &wait.RetryAt
	}
	return out, nil
}
