package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type supportAvailabilitySnapshot struct {
	IsWithinOfficeHours bool
	WidgetAvailability  model.WidgetConfigAvailability
}

func resolveSupportAvailability(settings model.SupportInboxSettings, now time.Time) supportAvailabilitySnapshot {
	return resolveSupportAvailabilityForMailbox(settings, nil, now)
}

// resolveSupportAvailabilityForMailbox returns the availability snapshot
// for a given workspace + optional mailbox context. The mailbox, when
// present, can override the reply-time expectation; office-hours
// calculation itself is workspace-wide today.
func resolveSupportAvailabilityForMailbox(settings model.SupportInboxSettings, mailbox *model.SupportMailbox, now time.Time) supportAvailabilitySnapshot {
	expectation := ResolveReplyExpectation(settings, mailbox)
	offlineMessage := defaultOutsideHoursMessage(settings)
	specialNotice := normalizedSpecialNotice(settings.SpecialNoticeText)

	onlineAvailability := resolveSupportOnlineAvailability(settings, mailbox)

	if !settings.BusinessHoursEnabled {
		return supportAvailabilitySnapshot{
			IsWithinOfficeHours: true,
			WidgetAvailability:  onlineAvailability,
		}
	}

	loc, err := time.LoadLocation(settings.BusinessHoursTimezone)
	if err != nil {
		return supportAvailabilitySnapshot{
			IsWithinOfficeHours: true,
			WidgetAvailability:  onlineAvailability,
		}
	}

	localNow := now.In(loc)
	if isWithinBusinessHours(settings, localNow) {
		return supportAvailabilitySnapshot{
			IsWithinOfficeHours: true,
			WidgetAvailability:  onlineAvailability,
		}
	}

	availability := model.WidgetConfigAvailability{
		IsOnline:            false,
		StatusText:          "Offline now",
		ReplyTimeText:       offlineMessage,
		OutsideHoursMessage: &offlineMessage,
		ReplyTimePreset:     expectation.Preset,
		ReplyTimeMinutes:    optionalMinutes(expectation),
		SpecialNoticeText:   specialNotice,
		MailboxID:           expectation.FromMailboxID,
	}

	if nextOpen := nextBusinessHoursStart(settings, localNow); nextOpen != nil {
		statusText := fmt.Sprintf("Offline now - Back %s", nextOpen.Format("Mon 3:04 PM"))
		nextOnlineAt := nextOpen.Format(time.RFC3339)
		availability.StatusText = statusText
		availability.NextOnlineAt = &nextOnlineAt
	}

	return supportAvailabilitySnapshot{
		IsWithinOfficeHours: false,
		WidgetAvailability:  availability,
	}
}

// resolveSupportOnlineAvailability builds the canonical "online" widget
// availability snapshot: an explicit online status with reply-time expectation
// copy and no offline "back later" framing. It is the single source of truth
// for the online presentation, reused by both the business-hours online branch
// and the presence-driven widget path (where a teammate online trumps hours).
func resolveSupportOnlineAvailability(settings model.SupportInboxSettings, mailbox *model.SupportMailbox) model.WidgetConfigAvailability {
	expectation := ResolveReplyExpectation(settings, mailbox)
	return model.WidgetConfigAvailability{
		IsOnline:          true,
		StatusText:        "Online now",
		ReplyTimeText:     expectation.Text,
		ReplyTimePreset:   expectation.Preset,
		ReplyTimeMinutes:  optionalMinutes(expectation),
		SpecialNoticeText: normalizedSpecialNotice(settings.SpecialNoticeText),
		MailboxID:         expectation.FromMailboxID,
	}
}

// optionalMinutes returns a pointer only for the custom preset so the
// JSON payload stays minimal for preset-only cases.
func optionalMinutes(exp ReplyExpectation) *int {
	if exp.Preset != model.SupportReplyTimePresetCustom || exp.CustomMinutes <= 0 {
		return nil
	}
	n := exp.CustomMinutes
	return &n
}

// normalizedSpecialNotice collapses nil / whitespace / empty to nil so
// the widget treats "no banner" and "empty banner" identically.
func normalizedSpecialNotice(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func loadSupportAvailability(
	ctx context.Context,
	installationRepo *repository.SupportInboxInstallationRepository,
	workspaceID string,
	now time.Time,
) (model.SupportInboxSettings, supportAvailabilitySnapshot, error) {
	settings := model.DefaultSupportInboxSettings()
	if installationRepo == nil || workspaceID == "" {
		return settings, resolveSupportAvailability(settings, now), nil
	}

	inst, err := installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return settings, supportAvailabilitySnapshot{}, err
	}
	if inst != nil {
		settings = parseSettings(inst.Settings)
	}

	return settings, resolveSupportAvailability(settings, now), nil
}

func resolveSupportTeammatePresenceStatuses(
	ctx context.Context,
	workspaceRepo *repository.WorkspaceRepository,
	presence websocket.PresenceProvider,
	statusOverrideRepo *repository.SupportTeammateStatusOverrideRepository,
	workspaceID string,
	now time.Time,
) ([]model.SupportTeammatePresenceStatus, error) {
	if workspaceID == "" || workspaceRepo == nil {
		return []model.SupportTeammatePresenceStatus{}, nil
	}

	supportUserIDs, err := workspaceRepo.ListSupportAccessibleUserIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(supportUserIDs) == 0 {
		return []model.SupportTeammatePresenceStatus{}, nil
	}

	onlineSet := make(map[string]struct{})
	lastSeen := map[string]time.Time{}
	manualOverrides := map[string]string{}
	if presence != nil {
		onlineAgents, err := presence.GetOnlineAgents(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		for _, userID := range onlineAgents {
			onlineSet[userID] = struct{}{}
		}
		lastSeen, err = presence.GetAgentLastSeen(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
	}
	if statusOverrideRepo != nil {
		overrides, err := statusOverrideRepo.ListByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		for _, override := range overrides {
			manualOverrides[override.UserID] = override.ManualStatus
		}
	}

	now = now.UTC()
	statuses := make([]model.SupportTeammatePresenceStatus, 0, len(supportUserIDs))
	for _, rawUserID := range supportUserIDs {
		userID := strings.TrimSpace(rawUserID)
		if userID == "" {
			continue
		}
		entry := model.SupportTeammatePresenceStatus{
			UserID: userID,
			Status: model.SupportTeammateStatusOffline,
			Source: model.SupportTeammateStatusSourceAuto,
		}
		if ts, ok := lastSeen[userID]; ok && !ts.IsZero() {
			t := ts.UTC()
			entry.LastSeenAt = &t
		}
		if manualStatus, ok := manualOverrides[userID]; ok {
			entry.Status = manualStatus
			entry.Source = model.SupportTeammateStatusSourceManual
			entry.ManualStatus = &manualStatus
		} else {
			recentlyActive := entry.LastSeenAt != nil && now.Sub(*entry.LastSeenAt) <= supportTeammateAwayThreshold
			if _, ok := onlineSet[userID]; ok {
				if recentlyActive {
					entry.Status = model.SupportTeammateStatusOnline
				} else {
					entry.Status = model.SupportTeammateStatusAway
				}
			} else if recentlyActive {
				entry.Status = model.SupportTeammateStatusAway
			}
		}
		statuses = append(statuses, entry)
	}

	return statuses, nil
}
