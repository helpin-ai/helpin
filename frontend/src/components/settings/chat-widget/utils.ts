import type { BusinessHoursDay, SupportInboxSettings } from '@/lib/pmTypes';
import type { WidgetConfig } from '@helpin-ai/widget-core';
import { formatReplyTimeCopy } from '@helpin-ai/shared';
import { DAYS, DEFAULT_BUSINESS_HOURS_DAY } from './constants';
import { getChatWidgetAIResponseModeForUI, isChatWidgetAIResponseModeActive } from './responseModes';

export type ChatSettingsDraft = Omit<
  SupportInboxSettings,
  | 'ai_agent_id'
  | 'triage_enabled'
  | 'triage_auto_move_enabled'
  | 'triage_confidence_threshold'
  | 'triage_widget_enabled'
  | 'triage_email_enabled'
  | 'triage_internal_enabled'
  | 'triage_fallback_behavior'
  | 'triage_rerun_on_meaning_change'
  | 'triage_daily_budget'
  | 'triage_skip_spam_conversations'
  | 'triage_deduplicate_first_message'
> & {
  ai_agent_id: string;
};

export function parseTimeToMinutes(value: string): number {
  const [hours = '0', minutes = '0'] = value.split(':');
  return Number(hours) * 60 + Number(minutes);
}

function weekdayKey(date: Date, timezone: string): string {
  const day = new Intl.DateTimeFormat('en-US', { timeZone: timezone, weekday: 'short' }).format(date).toLowerCase();
  return day.slice(0, 3);
}

function zonedMinutes(date: Date, timezone: string): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: timezone,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(date);
  const hour = Number(parts.find((part) => part.type === 'hour')?.value ?? '0');
  const minute = Number(parts.find((part) => part.type === 'minute')?.value ?? '0');
  return (hour * 60) + minute;
}

export interface PreviewAvailabilityInput {
  businessHoursEnabled: boolean;
  timezone: string;
  schedule: Record<string, BusinessHoursDay>;
  outsideMessage: string;
  replyTimePreset: string;
  replyTimeCustomMinutes: number | null;
  specialNoticeText: string | null;
}

type PreviewAvailability = NonNullable<WidgetConfig['availability']>;

export function buildPreviewAvailability(input: PreviewAvailabilityInput): PreviewAvailability {
  const { businessHoursEnabled, timezone, schedule, outsideMessage, replyTimePreset, replyTimeCustomMinutes, specialNoticeText } = input;
  const fallbackMessage = outsideMessage || "We're currently offline. Leave a message and we'll get back to you!";
  const replyTimeText = formatReplyTimeCopy(replyTimePreset || 'few_minutes', replyTimeCustomMinutes ?? 0);
  const notice = specialNoticeText && specialNoticeText.trim() ? specialNoticeText : undefined;

  const online: PreviewAvailability = {
    isOnline: true,
    statusText: 'Online now',
    replyTimeText,
    replyTimePreset: (replyTimePreset || 'few_minutes') as PreviewAvailability['replyTimePreset'],
    replyTimeMinutes: replyTimePreset === 'custom' && replyTimeCustomMinutes ? replyTimeCustomMinutes : undefined,
    specialNoticeText: notice,
  };

  if (!businessHoursEnabled) {
    return online;
  }

  try {
    const now = new Date();
    const day = schedule[weekdayKey(now, timezone)];
    const nowMinutes = zonedMinutes(now, timezone);
    const withinHours = Boolean(
      day?.enabled
      && nowMinutes >= parseTimeToMinutes(day.start)
      && nowMinutes < parseTimeToMinutes(day.end),
    );

    if (withinHours) return online;

    return {
      isOnline: false,
      statusText: 'Offline now',
      replyTimeText: fallbackMessage,
      outsideHoursMessage: fallbackMessage,
      specialNoticeText: notice,
    };
  } catch {
    return online;
  }
}

export function normalizeBusinessHoursDay(day?: Partial<BusinessHoursDay> | null): BusinessHoursDay {
  return {
    start: day?.start ?? DEFAULT_BUSINESS_HOURS_DAY.start,
    end: day?.end ?? DEFAULT_BUSINESS_HOURS_DAY.end,
    enabled: day?.enabled ?? DEFAULT_BUSINESS_HOURS_DAY.enabled,
  };
}

export function normalizeBusinessHoursSchedule(
  schedule?: Record<string, BusinessHoursDay> | null,
): Record<string, BusinessHoursDay> {
  return DAYS.reduce<Record<string, BusinessHoursDay>>((acc, day) => {
    acc[day.key] = normalizeBusinessHoursDay(schedule?.[day.key]);
    return acc;
  }, {});
}

export function sortHelpSpaceIds(ids?: string[] | null): string[] {
  return [...(ids ?? [])].sort();
}

const ESCALATION_FALLBACK_TIME = 'as soon as possible';
// Static, illustrative stand-in for {next_open} — the real value depends on
// the workspace's business-hours schedule/timezone and is only known server-side.
const ESCALATION_NEXT_OPEN_EXAMPLE = 'on Monday at 9:00 AM';

// Mirrors server/internal/service/support_handoff_state.go#shortReplyTimePhrase
// so the settings preview matches what customers actually see in {reply_time}.
export function shortReplyTimePhrase(preset: string, customMinutes: number | null): string {
  switch (preset) {
    case 'few_minutes':
      return 'a few minutes';
    case 'few_hours':
      return 'a few hours';
    case 'same_day':
      return 'a day';
    case 'custom':
      return customMinutes && customMinutes > 0 ? `about ${customMinutes} minutes` : '';
    default:
      return '';
  }
}

/**
 * Renders an escalation message template with {reply_time}/{next_open} tokens
 * substituted for a read-only settings preview. Mirrors the token substitution
 * in server/internal/service/support_handoff_state.go#renderEscalationMessage;
 * {next_open} uses a static illustrative example since the real value is
 * timezone/schedule-dependent and only resolved server-side.
 */
export function previewEscalationMessage(
  template: string,
  replyTimePreset: string,
  replyTimeCustomMinutes: number | null,
): string {
  const replyTime = shortReplyTimePhrase(replyTimePreset, replyTimeCustomMinutes) || ESCALATION_FALLBACK_TIME;
  return template
    .split('{reply_time}').join(replyTime)
    .split('{next_open}').join(ESCALATION_NEXT_OPEN_EXAMPLE);
}

export function buildSettingsDraftFromServer(settings: SupportInboxSettings): ChatSettingsDraft {
  const {
    ai_agent_id,
    triage_enabled,
    triage_auto_move_enabled,
    triage_confidence_threshold,
    triage_widget_enabled,
    triage_email_enabled,
    triage_internal_enabled,
    triage_fallback_behavior,
    triage_rerun_on_meaning_change,
    triage_daily_budget,
    triage_skip_spam_conversations,
    triage_deduplicate_first_message,
    ...rest
  } = settings;
  void triage_enabled;
  void triage_auto_move_enabled;
  void triage_confidence_threshold;
  void triage_widget_enabled;
  void triage_email_enabled;
  void triage_internal_enabled;
  void triage_fallback_behavior;
  void triage_rerun_on_meaning_change;
  void triage_daily_budget;
  void triage_skip_spam_conversations;
  void triage_deduplicate_first_message;

  return {
    ...rest,
    ai_enabled: rest.ai_enabled && isChatWidgetAIResponseModeActive(rest.ai_response_mode),
    ai_response_mode: getChatWidgetAIResponseModeForUI(rest.ai_response_mode),
    ai_agent_id: ai_agent_id ?? '',
    business_hours_schedule: normalizeBusinessHoursSchedule(rest.business_hours_schedule),
    widget_help_space_ids: sortHelpSpaceIds(rest.widget_help_space_ids),
  };
}

export function serializeSettingsDraft(draft: ChatSettingsDraft): string {
  return JSON.stringify(draft);
}
