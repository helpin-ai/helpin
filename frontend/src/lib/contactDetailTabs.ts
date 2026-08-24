export const contactDetailTabs = [
  'overview',
  'tasks',
  'emails',
  'meetings',
  'calls',
  'deals',
  'support',
  'notes',
] as const;

export type ContactDetailTab = (typeof contactDetailTabs)[number];

export function normalizeContactDetailTab(value: unknown): ContactDetailTab {
  return typeof value === 'string' && contactDetailTabs.includes(value as ContactDetailTab)
    ? (value as ContactDetailTab)
    : 'overview';
}
