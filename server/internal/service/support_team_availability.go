package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportTeamAvailabilityForRun tells a support run whether the team is open,
// when it is back, and how soon it usually replies, so the AI can answer
// "when can I talk to someone?" and never promises a reply sooner than that.
// With business hours off it says nothing: whatever else is set may be stale.
func (s *SupportAIService) SupportTeamAvailabilityForRun(ctx context.Context, conv *model.SupportConversation) string {
	if s == nil || conv == nil {
		return ""
	}
	now := time.Now()
	settings, availability, err := loadSupportAvailability(ctx, s.installationRepo, conv.WorkspaceID, now)
	if err != nil {
		slog.WarnContext(ctx, "load team availability for support run", "error", err, "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID)
		return ""
	}
	if !settings.BusinessHoursEnabled {
		return ""
	}
	replyTime := shortReplyTimePhrase(availability.WidgetAvailability.ReplyTimePreset, availability.WidgetAvailability.ReplyTimeMinutes)
	line := supportTeamHoursLine(availability.IsWithinOfficeHours, s.nextOpenForCustomer(ctx, conv, settings, now), s.customerLocation(ctx, conv) != nil, replyTime)
	return line + " Mention it only when the customer asks when the team is around or when a person will reply, and never promise a sooner reply."
}

// supportTeamHoursLine describes business hours for a support run.
func supportTeamHoursLine(open bool, nextOpen string, customerClock bool, replyTime string) string {
	line := "Team availability: open now, inside business hours."
	if !open {
		line = "Team availability: closed now, outside business hours."
		if nextOpen != "" {
			line = "Team availability: closed now; back " + nextOpen
			if customerClock {
				line += " on the customer's clock"
			}
			line += "."
		}
	}
	if replyTime != "" {
		line += " During business hours the team usually replies within " + replyTime + "."
	}
	return line
}
