package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const cannedResponseShortCodeIndex = "idx_support_canned_responses_ws_short_code"

var (
	ErrCannedResponseDuplicate = errors.New("shortcut already exists")
	ErrCannedResponseInvalid   = errors.New("invalid shortcut")
)

type defaultCannedResponse struct {
	ShortCode string
	Content   string
	Tag       string
}

var defaultSupportCannedResponses = []defaultCannedResponse{
	{
		ShortCode: "!hello",
		Content: `Hi {{customer.first_name | fallback: "there"}},

Thanks for reaching out. I'm checking this now and will get back to you shortly.`,
		Tag: "General",
	},
	{
		ShortCode: "!thanks",
		Content:   "Thanks for sending this over - that helps 👍",
		Tag:       "General",
	},
	{
		ShortCode: "!closing",
		Content:   "I'm going to close this for now, but reply here anytime if you need more help.",
		Tag:       "General",
	},
	{
		ShortCode: "!needinfo",
		Content:   "Could you send a little more detail about what you're seeing?\n\nA screenshot, error message, or the steps you took would help us investigate.",
		Tag:       "Support",
	},
	{
		ShortCode: "!steps",
		Content:   "Could you try these steps and let me know what happens?\n\n1. Refresh the page\n2. Sign out and back in\n3. Try again",
		Tag:       "Support",
	},
	{
		ShortCode: "!bug",
		Content:   "Thanks for reporting this.\n\nI can reproduce the issue from your description and I'm sharing it with our team to investigate.",
		Tag:       "Support",
	},
	{
		ShortCode: "!escalate",
		Content:   "I'm going to escalate this to the right teammate so we can take a closer look.\n\nWe'll keep you updated here.",
		Tag:       "Support",
	},
	{
		ShortCode: "!invoice",
		Content:   "I can help with that.\n\nCould you confirm the billing email or invoice number so I can look it up?",
		Tag:       "Billing",
	},
	{
		ShortCode: "!refund",
		Content:   "I can check the refund status for you.\n\nPlease send the order ID or billing email, and I'll take a look.",
		Tag:       "Billing",
	},
	{
		ShortCode: "!pricing",
		Content:   "Happy to help with pricing.\n\nCould you share your team size and what you're looking to use the product for?",
		Tag:       "Sales",
	},
	{
		ShortCode: "!demo",
		Content:   "We'd be happy to walk you through it. What day and time works best for a quick demo?",
		Tag:       "Sales",
	},
	{
		ShortCode: "!followup",
		Content: `Hi {{customer.first_name | fallback: "there"}},

Just checking in to see if you had a chance to review my last message.`,
		Tag: "Follow-up",
	},
}

// parseSettings unmarshals the JSONB settings string, applying defaults for missing fields.
func parseSettings(raw string) model.SupportInboxSettings {
	defaults := model.DefaultSupportInboxSettings()
	if raw == "" || raw == "{}" {
		return defaults
	}
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		return model.DefaultSupportInboxSettings()
	}
	defaults.EmailFallbackEnabled = true
	if !defaults.ForwardedEmailDetectionEnabled && defaults.ForwardedEmailDetectionMode == "" && defaults.ForwardedEmailMinConfidence == 0 {
		defaults.ForwardedEmailDetectionEnabled = true
		defaults.ForwardedEmailDetectionMode = "high_confidence_any_sender"
		defaults.ForwardedEmailMinConfidence = forwardedEmailDefaultMinConfidence
	}
	if defaults.ForwardedEmailDetectionMode == "" {
		defaults.ForwardedEmailDetectionMode = "high_confidence_any_sender"
	}
	if defaults.ForwardedEmailMinConfidence <= 0 {
		defaults.ForwardedEmailMinConfidence = forwardedEmailDefaultMinConfidence
	}
	return defaults
}

// mergeSettingsUpdate applies non-nil patch fields onto current settings.
func mergeSettingsUpdate(current model.SupportInboxSettings, patch model.UpdateInstallationSettingsRequest) model.SupportInboxSettings {
	if patch.RequireEmailBeforeChat != nil {
		current.RequireEmailBeforeChat = *patch.RequireEmailBeforeChat
	}
	if patch.RequirePhoneAfterEmail != nil {
		current.RequirePhoneAfterEmail = *patch.RequirePhoneAfterEmail
	}
	if patch.WelcomeMessage != nil {
		current.WelcomeMessage = *patch.WelcomeMessage
	}
	if patch.AIEnabled != nil {
		current.AIEnabled = *patch.AIEnabled
	}
	if patch.AIAgentID != nil {
		trimmed := strings.TrimSpace(*patch.AIAgentID)
		if trimmed == "" {
			current.AIAgentID = nil
		} else {
			current.AIAgentID = &trimmed
		}
	}
	if patch.AIConfidenceThreshold != nil {
		current.AIConfidenceThreshold = *patch.AIConfidenceThreshold
	}
	if patch.AIResponseMode != nil {
		current.AIResponseMode = *patch.AIResponseMode
	}
	if patch.AIPreRouterMode != nil {
		current.AIPreRouterMode = *patch.AIPreRouterMode
	}
	if patch.AIMaxFollowups != nil {
		current.AIMaxFollowups = *patch.AIMaxFollowups
	}
	if patch.AIFollowUpEnabled != nil {
		current.AIFollowUpEnabled = *patch.AIFollowUpEnabled
	}
	if patch.AIFollowUpDelayHours != nil {
		current.AIFollowUpDelayHours = *patch.AIFollowUpDelayHours
	}
	if patch.AIFollowUpCloseHours != nil {
		current.AIFollowUpCloseHours = *patch.AIFollowUpCloseHours
	}
	if patch.AIFollowUpMaxPerConversation != nil {
		current.AIFollowUpMaxPerConversation = *patch.AIFollowUpMaxPerConversation
	}
	if patch.AIAutoResolveTimeout != nil {
		current.AIAutoResolveTimeout = *patch.AIAutoResolveTimeout
	}
	if patch.ShowTalkToHuman != nil {
		current.ShowTalkToHuman = *patch.ShowTalkToHuman
	}
	if patch.DelayedTeamReplyMinutes != nil {
		current.DelayedTeamReplyMinutes = *patch.DelayedTeamReplyMinutes
	}
	if patch.DelayedTeamReplyMessage != nil {
		current.DelayedTeamReplyMessage = strings.TrimSpace(*patch.DelayedTeamReplyMessage)
	}
	if patch.DelayedTeamReplyMessageNoEmail != nil {
		current.DelayedTeamReplyMessageNoEmail = strings.TrimSpace(*patch.DelayedTeamReplyMessageNoEmail)
	}
	if patch.EscalationMessage != nil {
		current.EscalationMessage = *patch.EscalationMessage
	}
	if patch.EscalationMessageBusy != nil {
		current.EscalationMessageBusy = *patch.EscalationMessageBusy
	}
	if patch.EscalationMessageAfterHours != nil {
		current.EscalationMessageAfterHours = *patch.EscalationMessageAfterHours
	}
	if patch.HandoffBehavior != nil {
		current.HandoffBehavior = *patch.HandoffBehavior
	}
	if patch.HandoffTeamID != nil {
		current.HandoffTeamID = patch.HandoffTeamID
	}
	if patch.DefaultMailboxID != nil {
		current.DefaultMailboxID = patch.DefaultMailboxID
	}
	if patch.AIHandoffMailboxID != nil {
		current.AIHandoffMailboxID = patch.AIHandoffMailboxID
	}
	if patch.TriageEnabled != nil {
		current.TriageEnabled = *patch.TriageEnabled
	}
	if patch.TriageAutoMoveEnabled != nil {
		current.TriageAutoMoveEnabled = *patch.TriageAutoMoveEnabled
	}
	if patch.TriageConfidenceThreshold != nil {
		current.TriageConfidenceThreshold = *patch.TriageConfidenceThreshold
	}
	if patch.TriageWidgetEnabled != nil {
		current.TriageWidgetEnabled = *patch.TriageWidgetEnabled
	}
	if patch.TriageEmailEnabled != nil {
		current.TriageEmailEnabled = *patch.TriageEmailEnabled
	}
	if patch.TriageInternalEnabled != nil {
		current.TriageInternalEnabled = *patch.TriageInternalEnabled
	}
	if patch.TriageFallbackBehavior != nil {
		current.TriageFallbackBehavior = *patch.TriageFallbackBehavior
	}
	if patch.TriageRerunOnMeaningChange != nil {
		current.TriageRerunOnMeaningChange = *patch.TriageRerunOnMeaningChange
	}
	if patch.TriageDailyBudget != nil {
		current.TriageDailyBudget = *patch.TriageDailyBudget
	}
	if patch.TriageSkipSpamConversations != nil {
		current.TriageSkipSpamConversations = *patch.TriageSkipSpamConversations
	}
	if patch.TriageDeduplicateFirstMessage != nil {
		current.TriageDeduplicateFirstMessage = *patch.TriageDeduplicateFirstMessage
	}
	if patch.BusinessHoursEnabled != nil {
		current.BusinessHoursEnabled = *patch.BusinessHoursEnabled
	}
	if patch.BusinessHoursTimezone != nil {
		current.BusinessHoursTimezone = *patch.BusinessHoursTimezone
	}
	if patch.BusinessHoursSchedule != nil {
		current.BusinessHoursSchedule = patch.BusinessHoursSchedule
	}
	if patch.OutsideHoursMessage != nil {
		current.OutsideHoursMessage = *patch.OutsideHoursMessage
	}
	if patch.ReplyTimePreset != nil {
		current.ReplyTimePreset = strings.TrimSpace(*patch.ReplyTimePreset)
	}
	if patch.ClearReplyTimeCustomMinutes != nil && *patch.ClearReplyTimeCustomMinutes {
		current.ReplyTimeCustomMinutes = nil
	} else if patch.ReplyTimeCustomMinutes != nil {
		minutes := *patch.ReplyTimeCustomMinutes
		current.ReplyTimeCustomMinutes = &minutes
	}
	if patch.ClearSpecialNotice != nil && *patch.ClearSpecialNotice {
		current.SpecialNoticeText = nil
	} else if patch.SpecialNoticeText != nil {
		trimmed := strings.TrimSpace(*patch.SpecialNoticeText)
		if trimmed == "" {
			current.SpecialNoticeText = nil
		} else {
			current.SpecialNoticeText = &trimmed
		}
	}
	current.EmailFallbackEnabled = true
	if patch.EmailFallbackDelaySecs != nil {
		current.EmailFallbackDelaySecs = *patch.EmailFallbackDelaySecs
	}
	if patch.EmailFallbackFromName != nil {
		current.EmailFallbackFromName = *patch.EmailFallbackFromName
	}
	if patch.EmailFallbackMaxDeliveryAgeSecs != nil {
		current.EmailFallbackMaxDeliveryAgeSecs = *patch.EmailFallbackMaxDeliveryAgeSecs
	}
	if patch.ForwardedEmailDetectionEnabled != nil {
		current.ForwardedEmailDetectionEnabled = *patch.ForwardedEmailDetectionEnabled
	}
	if patch.ForwardedEmailDetectionMode != nil {
		current.ForwardedEmailDetectionMode = strings.TrimSpace(*patch.ForwardedEmailDetectionMode)
	}
	if patch.ForwardedEmailMinConfidence != nil {
		current.ForwardedEmailMinConfidence = *patch.ForwardedEmailMinConfidence
	}
	if patch.WidgetName != nil {
		current.WidgetName = *patch.WidgetName
	}
	if patch.WidgetAvatarURL != nil {
		current.WidgetAvatarURL = *patch.WidgetAvatarURL
	}
	if patch.WidgetHelpSpaceIDs != nil {
		current.WidgetHelpSpaceIDs = append([]string(nil), patch.WidgetHelpSpaceIDs...)
	}
	if patch.BrandColor != nil {
		current.BrandColor = *patch.BrandColor
	}
	if patch.ShowBranding != nil {
		current.ShowBranding = *patch.ShowBranding
	}
	if patch.ColorScheme != nil {
		current.ColorScheme = *patch.ColorScheme
	}
	if patch.ButtonColor != nil {
		current.ButtonColor = *patch.ButtonColor
	}
	if patch.ButtonIconColor != nil {
		current.ButtonIconColor = *patch.ButtonIconColor
	}
	if patch.LogoURL != nil {
		current.LogoURL = *patch.LogoURL
	}
	if patch.LauncherPosition != nil {
		current.LauncherPosition = *patch.LauncherPosition
	}
	if patch.LauncherIcon != nil {
		current.LauncherIcon = *patch.LauncherIcon
	}
	if patch.CSATEnabled != nil {
		current.CSATEnabled = *patch.CSATEnabled
	}
	if patch.FileUploadsEnabled != nil {
		current.FileUploadsEnabled = *patch.FileUploadsEnabled
	}
	if patch.ForceVisitorIdentity != nil {
		current.ForceVisitorIdentity = *patch.ForceVisitorIdentity
	}
	return current
}

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validateSettings checks settings field constraints.
func (s *SupportInboxService) validateSettings(ctx context.Context, workspaceID string, settings model.SupportInboxSettings) error {
	if settings.AIConfidenceThreshold < 0 || settings.AIConfidenceThreshold > 1 {
		return fmt.Errorf("ai_confidence_threshold must be between 0.0 and 1.0")
	}
	if settings.TriageConfidenceThreshold < 0 || settings.TriageConfidenceThreshold > 1 {
		return fmt.Errorf("triage_confidence_threshold must be between 0.0 and 1.0")
	}
	if settings.BrandColor != "" && !hexColorRegex.MatchString(settings.BrandColor) {
		return fmt.Errorf("brand_color must be a valid hex color (e.g. #6366F1)")
	}
	if settings.ButtonColor != "" && !hexColorRegex.MatchString(settings.ButtonColor) {
		return fmt.Errorf("button_color must be a valid hex color (e.g. #000000)")
	}
	if settings.ButtonIconColor != "" && !hexColorRegex.MatchString(settings.ButtonIconColor) {
		return fmt.Errorf("button_icon_color must be a valid hex color (e.g. #FFFFFF)")
	}
	validColorScheme := map[string]bool{"system": true, "light": true, "dark": true}
	if settings.ColorScheme != "" && !validColorScheme[settings.ColorScheme] {
		return fmt.Errorf("color_scheme must be system, light, or dark")
	}
	validHandoff := map[string]bool{"unassigned": true, "assign_to_team": true, "round_robin": true}
	if !validHandoff[settings.HandoffBehavior] {
		return fmt.Errorf("handoff_behavior must be unassigned, assign_to_team, or round_robin")
	}
	validTriageFallback := map[string]bool{"shared": true, "default": true}
	if !validTriageFallback[settings.TriageFallbackBehavior] {
		return fmt.Errorf("triage_fallback_behavior must be shared or default")
	}
	if settings.HandoffBehavior == "assign_to_team" && (settings.HandoffTeamID == nil || *settings.HandoffTeamID == "") {
		return fmt.Errorf("handoff_team_id is required when handoff_behavior is assign_to_team")
	}
	if s.mailboxRepo != nil {
		for fieldName, mailboxID := range map[string]*string{
			"default_mailbox_id":    settings.DefaultMailboxID,
			"ai_handoff_mailbox_id": settings.AIHandoffMailboxID,
		} {
			if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
				continue
			}
			mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, strings.TrimSpace(*mailboxID))
			if err != nil {
				return fmt.Errorf("validate %s: %w", fieldName, err)
			}
			if mailbox == nil || !mailbox.Active {
				return fmt.Errorf("%s must reference an active mailbox", fieldName)
			}
		}
	}
	validPosition := map[string]bool{"bottom_right": true, "bottom_left": true}
	if !validPosition[settings.LauncherPosition] {
		return fmt.Errorf("launcher_position must be bottom_right or bottom_left")
	}
	validIcon := map[string]bool{"chat_bubble": true, "question_mark": true, "help": true}
	if !validIcon[settings.LauncherIcon] {
		return fmt.Errorf("launcher_icon must be chat_bubble, question_mark, or help")
	}
	validResponseMode := map[string]bool{"ai_first": true, "internal_note": true, "off": true}
	if settings.AIResponseMode != "" && !validResponseMode[settings.AIResponseMode] {
		return fmt.Errorf("ai_response_mode must be ai_first, internal_note, or off")
	}
	validPreRouterMode := map[string]bool{
		model.SupportAIPreRouterModeOff:     true,
		model.SupportAIPreRouterModeShadow:  true,
		model.SupportAIPreRouterModeEnabled: true,
	}
	if !validPreRouterMode[settings.AIPreRouterMode] {
		return fmt.Errorf("ai_pre_router_mode must be off, shadow, or enabled")
	}
	if settings.ReplyTimePreset != "" && !model.IsValidSupportReplyTimePreset(settings.ReplyTimePreset) {
		return fmt.Errorf("reply_time_preset must be few_minutes, few_hours, same_day, or custom")
	}
	if settings.ReplyTimePreset == model.SupportReplyTimePresetCustom {
		if settings.ReplyTimeCustomMinutes == nil {
			return fmt.Errorf("reply_time_custom_minutes is required when reply_time_preset is custom")
		}
		if !model.IsValidSupportReplyTimeCustomMinutes(*settings.ReplyTimeCustomMinutes) {
			return fmt.Errorf("reply_time_custom_minutes must be between %d and %d", model.SupportReplyTimeCustomMinutesMin, model.SupportReplyTimeCustomMinutesMax)
		}
	}
	if settings.SpecialNoticeText != nil && len(*settings.SpecialNoticeText) > model.SupportSpecialNoticeMaxLength {
		return fmt.Errorf("special_notice_text must be %d characters or fewer", model.SupportSpecialNoticeMaxLength)
	}
	if settings.AIMaxFollowups < 0 || settings.AIMaxFollowups > 50 {
		return fmt.Errorf("ai_max_followups must be between 0 and 50")
	}
	if settings.AIFollowUpDelayHours < 1 || settings.AIFollowUpDelayHours > 720 || settings.AIFollowUpCloseHours < 1 || settings.AIFollowUpCloseHours > 720 || settings.AIFollowUpMaxPerConversation < 1 || settings.AIFollowUpMaxPerConversation > 5 {
		return fmt.Errorf("AI follow-up delays must be 1–720 hours and conversation limit 1–5")
	}
	if settings.AIAutoResolveTimeout < 0 {
		return fmt.Errorf("ai_auto_resolve_timeout must be >= 0")
	}
	if settings.EmailFallbackDelaySecs < 10 || settings.EmailFallbackDelaySecs > 600 {
		return fmt.Errorf("email_fallback_delay_secs must be between 10 and 600")
	}
	if settings.EmailFallbackMaxDeliveryAgeSecs < 120 || settings.EmailFallbackMaxDeliveryAgeSecs > 1800 {
		return fmt.Errorf("email_fallback_max_delivery_age_secs must be between 120 and 1800")
	}
	if settings.EmailFallbackMaxDeliveryAgeSecs < settings.EmailFallbackDelaySecs {
		return fmt.Errorf("email_fallback_max_delivery_age_secs must be greater than or equal to email_fallback_delay_secs")
	}
	if settings.TriageDailyBudget < 0 {
		return fmt.Errorf("triage_daily_budget must be >= 0")
	}
	if settings.AIEnabled {
		if settings.AIAgentID == nil || strings.TrimSpace(*settings.AIAgentID) == "" {
			return fmt.Errorf("ai_agent_id is required when ai_enabled is true")
		}
		if s == nil || s.agentRepo == nil {
			return fmt.Errorf("support agent validation is unavailable")
		}
		agent, err := s.agentRepo.GetByID(ctx, workspaceID, strings.TrimSpace(*settings.AIAgentID))
		if err != nil {
			return fmt.Errorf("get support ai agent: %w", err)
		}
		if agent == nil {
			return fmt.Errorf("selected support ai agent was not found")
		}
		if err := validateAgentTarget(agent, "support_conversation"); err != nil {
			return fmt.Errorf("selected ai agent must support support conversations: %w", err)
		}
	}
	return nil
}

// isOnline computes whether the widget is currently within business hours.
func isOnline(s model.SupportInboxSettings) bool {
	return resolveSupportAvailability(s, time.Now()).IsWithinOfficeHours
}

func parseTimeToMinutes(t string) int {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h*60 + m
}

func weekdayKey(day time.Weekday) string {
	switch day {
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	case time.Saturday:
		return "sat"
	default:
		return "sun"
	}
}

func defaultOutsideHoursMessage(settings model.SupportInboxSettings) string {
	if strings.TrimSpace(settings.OutsideHoursMessage) != "" {
		return settings.OutsideHoursMessage
	}
	return model.DefaultSupportInboxSettings().OutsideHoursMessage
}

func isWithinBusinessHours(settings model.SupportInboxSettings, localNow time.Time) bool {
	day, ok := settings.BusinessHoursSchedule[weekdayKey(localNow.Weekday())]
	if !ok || !day.Enabled {
		return false
	}

	currentMinutes := localNow.Hour()*60 + localNow.Minute()
	startMinutes := parseTimeToMinutes(day.Start)
	endMinutes := parseTimeToMinutes(day.End)
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

func nextBusinessHoursStart(settings model.SupportInboxSettings, localNow time.Time) *time.Time {
	loc := localNow.Location()
	for dayOffset := 0; dayOffset < 8; dayOffset += 1 {
		candidateDay := localNow.AddDate(0, 0, dayOffset)
		day, ok := settings.BusinessHoursSchedule[weekdayKey(candidateDay.Weekday())]
		if !ok || !day.Enabled {
			continue
		}

		startMinutes := parseTimeToMinutes(day.Start)
		candidateStart := time.Date(
			candidateDay.Year(),
			candidateDay.Month(),
			candidateDay.Day(),
			startMinutes/60,
			startMinutes%60,
			0,
			0,
			loc,
		)

		if candidateStart.After(localNow) {
			return &candidateStart
		}
	}

	return nil
}

// buildWidgetAvailability returns the widget-facing availability snapshot.
//
// IsOnline reflects ACTUAL teammate presence (hasOnlineAgent) rather than
// business hours alone. Presence trumps hours (matching the escalation path):
//
//   - A teammate online => a fully consistent ONLINE snapshot regardless of
//     hours: online StatusText and no offline "back later" framing
//     (NextOnlineAt / OutsideHoursMessage cleared). This avoids the
//     contradictory "IsOnline=true but StatusText='Offline now'" state that the
//     widget would otherwise render.
//   - Nobody online => the business-hours snapshot with IsOnline forced false.
//     Within hours we surface the reply-time expectation instead of claiming
//     "Online now"; outside hours keeps the offline copy, NextOnlineAt, and
//     OutsideHoursMessage unchanged.
//
// Invariant: IsOnline == true iff StatusText is an online message and
// NextOnlineAt is empty.
func buildWidgetAvailability(settings model.SupportInboxSettings, now time.Time, hasOnlineAgent bool) model.WidgetConfigAvailability {
	if hasOnlineAgent {
		return resolveSupportOnlineAvailability(settings, nil)
	}

	snapshot := resolveSupportAvailability(settings, now)
	availability := snapshot.WidgetAvailability
	availability.IsOnline = false
	if snapshot.IsWithinOfficeHours {
		// Within hours but no teammate online: don't claim "Online now" —
		// surface the reply-time expectation instead.
		availability.StatusText = availability.ReplyTimeText
	}
	return availability
}

// GetInstallation returns the installation and its parsed settings for a workspace.
// If no installation exists yet, one is auto-created with default settings and a new widget key.
func (s *SupportInboxService) GetInstallation(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if inst == nil {
		widgetKey, err := generateSecureToken(16)
		if err != nil {
			return nil, nil, fmt.Errorf("generate widget key: %w", err)
		}
		secretKey, err := generateSecureToken(32)
		if err != nil {
			return nil, nil, fmt.Errorf("generate secret key: %w", err)
		}

		defaults := model.DefaultSupportInboxSettings()
		raw, err := json.Marshal(defaults)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal default settings: %w", err)
		}

		inst = &model.SupportWidgetInstallation{
			WorkspaceID:              workspaceID,
			WidgetKey:                widgetKey,
			SecretKey:                secretKey,
			IdentityVerificationMode: model.IdentityVerificationModeEnforced,
			Settings:                 string(raw),
			Active:                   true,
		}
		if err := s.installationRepo.Create(ctx, inst); err != nil {
			return nil, nil, fmt.Errorf("create widget installation: %w", err)
		}

		slog.InfoContext(ctx, "auto-created support widget installation", "workspace_id", workspaceID, "installation_id", inst.ID)
		return inst, &defaults, nil
	}
	settings := parseSettings(inst.Settings)
	return inst, &settings, nil
}

func (s *SupportInboxService) GetRoutingUsageStatus(ctx context.Context, workspaceID string) (*model.SupportRoutingUsageStatus, error) {
	_, settings, err := s.GetInstallation(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		defaults := model.DefaultSupportInboxSettings()
		settings = &defaults
	}

	now := time.Now().UTC()
	startOfDay := now.Truncate(24 * time.Hour)
	resetAt := startOfDay.Add(24 * time.Hour)
	used := int64(0)
	if s != nil && s.triageEventRepo != nil {
		count, countErr := s.triageEventRepo.CountAIEvaluationsSince(ctx, workspaceID, startOfDay)
		if countErr != nil {
			return nil, countErr
		}
		used = count
	}

	var remaining *int
	exhausted := false
	if settings.TriageDailyBudget > 0 {
		value := settings.TriageDailyBudget - int(used)
		if value < 0 {
			value = 0
		}
		remaining = &value
		exhausted = int(used) >= settings.TriageDailyBudget
	}

	return &model.SupportRoutingUsageStatus{
		TriageEnabled:  settings.TriageEnabled,
		DailyBudget:    settings.TriageDailyBudget,
		UsedToday:      int(used),
		RemainingToday: remaining,
		ResetAt:        resetAt,
		Exhausted:      exhausted,
	}, nil
}

// UpdateInstallationSettings merges, validates, and saves settings.
func (s *SupportInboxService) UpdateInstallationSettings(ctx context.Context, workspaceID string, req model.UpdateInstallationSettingsRequest) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if inst == nil {
		return nil, nil, fmt.Errorf("no widget installation found for this workspace")
	}

	current := parseSettings(inst.Settings)
	merged := mergeSettingsUpdate(current, req)
	if merged.DelayedTeamReplyMinutes < 1 || merged.DelayedTeamReplyMinutes > 1440 {
		return nil, nil, fmt.Errorf("delayed team reply wait must be between 1 and 1440 minutes")
	}
	defaults := model.DefaultSupportInboxSettings()
	if strings.TrimSpace(merged.DelayedTeamReplyMessage) == "" {
		merged.DelayedTeamReplyMessage = defaults.DelayedTeamReplyMessage
	}
	if strings.TrimSpace(merged.DelayedTeamReplyMessageNoEmail) == "" {
		merged.DelayedTeamReplyMessageNoEmail = defaults.DelayedTeamReplyMessageNoEmail
	}
	if len([]rune(merged.DelayedTeamReplyMessage)) > 2000 || len([]rune(merged.DelayedTeamReplyMessageNoEmail)) > 2000 {
		return nil, nil, fmt.Errorf("delayed team reply messages must be at most 2000 characters")
	}

	if req.AllowedOrigins != nil {
		origins, err := normalizeAllowedOrigins(*req.AllowedOrigins)
		if err != nil {
			return nil, nil, err
		}
		inst.AllowedOrigins = model.DocsStringArray(origins)
	}
	if req.IdentityVerificationMode != nil {
		mode := strings.TrimSpace(*req.IdentityVerificationMode)
		if mode != model.IdentityVerificationModeReportOnly && mode != model.IdentityVerificationModeEnforced {
			return nil, nil, fmt.Errorf("identity_verification_mode must be report_only or enforced")
		}
		if mode == model.IdentityVerificationModeEnforced && len(inst.AllowedOrigins) == 0 {
			return nil, nil, fmt.Errorf("allowed_origins is required before identity enforcement")
		}
		inst.IdentityVerificationMode = mode
	}
	if s.entitlementSvc != nil {
		if !merged.ShowBranding {
			if err := s.entitlementSvc.RequireFeature(ctx, workspaceID, EntitlementFeatureRemoveBranding); err != nil {
				return nil, nil, err
			}
		}
		if merged.TriageEnabled || merged.TriageAutoMoveEnabled {
			if err := s.entitlementSvc.RequireFeature(ctx, workspaceID, EntitlementFeatureAIConversationRouting); err != nil {
				return nil, nil, err
			}
		}
		if strings.TrimSpace(merged.HandoffBehavior) == "round_robin" {
			if err := s.entitlementSvc.RequireFeature(ctx, workspaceID, EntitlementFeatureRoundRobinAssignment); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := s.validateSettings(ctx, workspaceID, merged); err != nil {
		return nil, nil, err
	}

	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal settings: %w", err)
	}
	inst.Settings = string(raw)

	if err := s.installationRepo.Update(ctx, inst); err != nil {
		return nil, nil, err
	}

	// Broadcast config update to all connected widget clients in this workspace
	if configResp, err := s.buildWidgetConfigResponse(ctx, inst); err == nil {
		configData, _ := json.Marshal(configResp)
		s.wsPublisher.Publish(websocket.Event{
			Action:      "config_updated",
			Entity:      "support_widget",
			EntityID:    inst.ID,
			WorkspaceID: workspaceID,
			Data:        configData,
		})
	} else {
		slog.ErrorContext(ctx, "failed to build updated support widget config", "error", err, "workspace_id", workspaceID)
	}

	slog.InfoContext(ctx, "updated support installation settings", "workspace_id", workspaceID)
	return inst, &merged, nil
}

// RegenerateWidgetKey generates a new widget key + secret key.
// If no installation exists yet, one is created with default settings.
func (s *SupportInboxService) RegenerateWidgetKey(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	// No installation yet — create one (handles edge cases where seeding was missed).
	if inst == nil {
		inst, settings, err := s.GetInstallation(ctx, workspaceID)
		if err != nil {
			return nil, nil, fmt.Errorf("create widget installation: %w", err)
		}
		return inst, settings, nil
	}

	newWidgetKey, err := generateSecureToken(16)
	if err != nil {
		return nil, nil, fmt.Errorf("generate widget key: %w", err)
	}
	newSecretKey, err := generateSecureToken(32)
	if err != nil {
		return nil, nil, fmt.Errorf("generate secret key: %w", err)
	}

	if err := s.installationRepo.RegenerateKeys(ctx, inst.ID, newWidgetKey, newSecretKey); err != nil {
		return nil, nil, err
	}

	inst.WidgetKey = newWidgetKey
	inst.SecretKey = newSecretKey

	settings := parseSettings(inst.Settings)
	slog.InfoContext(ctx, "regenerated support widget keys", "workspace_id", workspaceID, "installation_id", inst.ID)
	return inst, &settings, nil
}

// RotateWidgetSecret rotates the S2S/signing secret without changing the public widget key.
func (s *SupportInboxService) RotateWidgetSecret(ctx context.Context, workspaceID, actorUserID string) (string, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if inst == nil {
		return "", fmt.Errorf("no widget installation found for this workspace")
	}
	secretKey, err := generateSecureToken(32)
	if err != nil {
		return "", fmt.Errorf("generate secret key: %w", err)
	}
	if err := s.installationRepo.RotateSecret(ctx, inst.ID, secretKey, actorUserID); err != nil {
		return "", err
	}
	slog.InfoContext(ctx, "rotated support widget server/signing secret",
		"workspace_id", workspaceID,
		"installation_id", inst.ID,
		"actor_user_id", actorUserID,
	)
	return secretKey, nil
}

// SeedWorkspaceDefaults creates default support inbox data for a new workspace.
func (s *SupportInboxService) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	if s.installationRepo != nil {
		existing, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("check existing installation: %w", err)
		}
		if existing == nil {
			widgetKey, err := generateSecureToken(16)
			if err != nil {
				return fmt.Errorf("generate widget key: %w", err)
			}
			secretKey, err := generateSecureToken(32)
			if err != nil {
				return fmt.Errorf("generate secret key: %w", err)
			}

			defaults := model.DefaultSupportInboxSettings()
			raw, err := json.Marshal(defaults)
			if err != nil {
				return fmt.Errorf("marshal default settings: %w", err)
			}

			inst := &model.SupportWidgetInstallation{
				WorkspaceID:              workspaceID,
				WidgetKey:                widgetKey,
				SecretKey:                secretKey,
				IdentityVerificationMode: model.IdentityVerificationModeEnforced,
				Settings:                 string(raw),
				Active:                   true,
			}
			if err := s.installationRepo.Create(ctx, inst); err != nil {
				return fmt.Errorf("create widget installation: %w", err)
			}

			slog.InfoContext(ctx, "seeded support widget installation", "workspace_id", workspaceID, "installation_id", inst.ID)
		}
	}
	if err := s.seedDefaultCannedResponses(ctx, workspaceID, actorID); err != nil {
		return fmt.Errorf("seed default shortcuts: %w", err)
	}
	return nil
}

func (s *SupportInboxService) seedDefaultCannedResponses(ctx context.Context, workspaceID, actorID string) error {
	if s.cannedResponseRepo == nil {
		return nil
	}
	existing, err := s.cannedResponseRepo.List(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	for _, def := range defaultSupportCannedResponses {
		response := &model.SupportCannedResponse{
			WorkspaceID: workspaceID,
			ShortCode:   def.ShortCode,
			Content:     def.Content,
			Tag:         def.Tag,
			CreatedByID: actorID,
		}
		if err := s.cannedResponseRepo.Create(ctx, response); err != nil {
			if isDuplicateCannedResponseError(err) {
				continue
			}
			return err
		}
	}
	slog.InfoContext(ctx, "seeded support shortcut defaults", "workspace_id", workspaceID, "count", len(defaultSupportCannedResponses))
	return nil
}

// ListCannedResponses returns all canned responses for a workspace.
func (s *SupportInboxService) ListCannedResponses(ctx context.Context, workspaceID string) ([]model.SupportCannedResponse, error) {
	return s.cannedResponseRepo.List(ctx, workspaceID)
}

// SearchCannedResponses returns canned responses matching a query.
func (s *SupportInboxService) SearchCannedResponses(ctx context.Context, workspaceID, query string) ([]model.SupportCannedResponse, error) {
	return s.cannedResponseRepo.Search(ctx, workspaceID, query)
}

func normalizeCannedResponseRequest(req model.CannedResponseRequest) (model.CannedResponseRequest, error) {
	req.ShortCode = strings.TrimSpace(req.ShortCode)
	req.Content = strings.TrimSpace(req.Content)
	if req.Tag != nil {
		tag := strings.TrimSpace(*req.Tag)
		req.Tag = &tag
	}
	if req.ShortCode == "" || req.Content == "" {
		return req, fmt.Errorf("%w: shortcut and content are required", ErrCannedResponseInvalid)
	}
	if !strings.HasPrefix(req.ShortCode, "!") || strings.ContainsAny(req.ShortCode, " \t\r\n") {
		return req, fmt.Errorf("%w: shortcut must start with ! and contain no spaces", ErrCannedResponseInvalid)
	}
	if len(req.ShortCode) < 2 {
		return req, fmt.Errorf("%w: shortcut must have at least one character after !", ErrCannedResponseInvalid)
	}
	if req.Tag == nil || *req.Tag == "" {
		tag := "General"
		req.Tag = &tag
	}
	return req, nil
}

// isDuplicateCannedResponseError detects a race-condition violation of the
// (workspace_id, short_code) unique index. The pre-flight GetByShortCode check
// covers the common case; this fallback only fires when two creates land
// between that lookup and the INSERT.
func isDuplicateCannedResponseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), cannedResponseShortCodeIndex)
}

// CreateCannedResponse creates a new canned response.
func (s *SupportInboxService) CreateCannedResponse(ctx context.Context, workspaceID string, req model.CannedResponseRequest, createdByID string) (*model.SupportCannedResponse, error) {
	req, err := normalizeCannedResponseRequest(req)
	if err != nil {
		return nil, err
	}
	existing, err := s.cannedResponseRepo.GetByShortCode(ctx, workspaceID, req.ShortCode)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrCannedResponseDuplicate
	}

	response := &model.SupportCannedResponse{
		WorkspaceID: workspaceID,
		ShortCode:   req.ShortCode,
		Content:     req.Content,
		Tag:         *req.Tag,
		CreatedByID: createdByID,
	}
	if err := s.cannedResponseRepo.Create(ctx, response); err != nil {
		if isDuplicateCannedResponseError(err) {
			return nil, ErrCannedResponseDuplicate
		}
		return nil, fmt.Errorf("create canned response: %w", err)
	}
	return response, nil
}

// UpdateCannedResponse updates an existing canned response.
func (s *SupportInboxService) UpdateCannedResponse(ctx context.Context, workspaceID, id string, req model.CannedResponseRequest) (*model.SupportCannedResponse, error) {
	req, err := normalizeCannedResponseRequest(req)
	if err != nil {
		return nil, err
	}
	response, err := s.cannedResponseRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("canned response not found")
	}
	existing, err := s.cannedResponseRepo.GetByShortCode(ctx, workspaceID, req.ShortCode)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != id {
		return nil, ErrCannedResponseDuplicate
	}

	response.ShortCode = req.ShortCode
	response.Content = req.Content
	response.Tag = *req.Tag

	if err := s.cannedResponseRepo.Update(ctx, response); err != nil {
		if isDuplicateCannedResponseError(err) {
			return nil, ErrCannedResponseDuplicate
		}
		return nil, fmt.Errorf("update canned response: %w", err)
	}
	return response, nil
}

// DeleteCannedResponse deletes a canned response.
func (s *SupportInboxService) DeleteCannedResponse(ctx context.Context, workspaceID, id string) error {
	return s.cannedResponseRepo.Delete(ctx, workspaceID, id)
}
