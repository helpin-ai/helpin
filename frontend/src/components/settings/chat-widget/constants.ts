import { HelpCircleIcon, LifebuoyIcon, Message01Icon, ComputerIcon, Moon02Icon, Sun01Icon } from '@/lib/icons';
import type { BusinessHoursDay } from '@/lib/pmTypes';

export const ICON_OPTIONS = [
  { value: 'chat_bubble', label: 'Chat Bubble', icon: Message01Icon },
  { value: 'question_mark', label: 'Question Mark', icon: HelpCircleIcon },
  { value: 'help', label: 'Help', icon: LifebuoyIcon },
];

export const COLOR_SCHEME_OPTIONS = [
  { value: 'system', label: 'System', icon: ComputerIcon },
  { value: 'light', label: 'Light', icon: Sun01Icon },
  { value: 'dark', label: 'Dark', icon: Moon02Icon },
];

export const DAYS = [
  { key: 'mon', label: 'Monday' },
  { key: 'tue', label: 'Tuesday' },
  { key: 'wed', label: 'Wednesday' },
  { key: 'thu', label: 'Thursday' },
  { key: 'fri', label: 'Friday' },
  { key: 'sat', label: 'Saturday' },
  { key: 'sun', label: 'Sunday' },
];

export const COMMON_TIMEZONES = [
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Anchorage',
  'Pacific/Honolulu',
  'Europe/London',
  'Europe/Berlin',
  'Europe/Paris',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Asia/Kolkata',
  'Australia/Sydney',
  'Pacific/Auckland',
  'UTC',
];

export const NO_AGENT_VALUE = '__none__';
export const DEFAULT_ONLINE_REPLY_TEXT = 'We typically reply in a few minutes';
export const DEFAULT_EMAIL_FALLBACK_DELAY_SECS = 10;
export const DEFAULT_BUSINESS_HOURS_DAY: BusinessHoursDay = { start: '09:00', end: '17:00', enabled: false };

export { canRemoveHelpinBranding } from '@edition';
