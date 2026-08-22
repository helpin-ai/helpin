export const companyDetailTabs = [
  'overview',
  'tasks',
  'emails',
  'meetings',
  'calls',
  'deals',
  'support',
  'notes',
] as const;

export type CompanyDetailTab = (typeof companyDetailTabs)[number];

export function normalizeCompanyDetailTab(value: unknown): CompanyDetailTab {
  return typeof value === 'string' && companyDetailTabs.includes(value as CompanyDetailTab)
    ? (value as CompanyDetailTab)
    : 'overview';
}
