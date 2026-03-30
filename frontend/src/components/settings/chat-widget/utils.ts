import type { BusinessHoursDay, SupportInboxSettings } from '@/lib/pmTypes';
import type { WidgetConfig } from '@helpin/widget-core';
import { DAYS, DEFAULT_BUSINESS_HOURS_DAY, DEFAULT_ONLINE_REPLY_TEXT } from './constants';

export type ChatSettingsDraft = Omit<SupportInboxSettings, 'ai_agent_id'> & {
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

export function buildPreviewAvailability(
  businessHoursEnabled: boolean,
  timezone: string,
  schedule: Record<string, BusinessHoursDay>,
  outsideMessage: string,
): WidgetConfig['availability'] {
  const fallbackMessage = outsideMessage || "We're currently offline. Leave a message and we'll get back to you!";
  if (!businessHoursEnabled) {
    return {
      isOnline: true,
      statusText: 'Online now',
      replyTimeText: DEFAULT_ONLINE_REPLY_TEXT,
    };
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

    if (withinHours) {
      return {
        isOnline: true,
        statusText: 'Online now',
        replyTimeText: DEFAULT_ONLINE_REPLY_TEXT,
      };
    }

    return {
      isOnline: false,
      statusText: 'Offline now',
      replyTimeText: fallbackMessage,
      outsideHoursMessage: fallbackMessage,
    };
  } catch {
    return {
      isOnline: true,
      statusText: 'Online now',
      replyTimeText: DEFAULT_ONLINE_REPLY_TEXT,
    };
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

export function buildSettingsDraftFromServer(settings: SupportInboxSettings): ChatSettingsDraft {
  return {
    ...settings,
    ai_agent_id: settings.ai_agent_id ?? '',
    business_hours_schedule: normalizeBusinessHoursSchedule(settings.business_hours_schedule),
    widget_help_space_ids: sortHelpSpaceIds(settings.widget_help_space_ids),
  };
}

export function serializeSettingsDraft(draft: ChatSettingsDraft): string {
  return JSON.stringify(draft);
}
