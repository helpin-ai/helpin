export const companyDetailTabs = [
  'overview',
  'tasks',
  'emails',
  'meetings',
  'calls',
  'deals',
  'support',
  'portal',
  'notes',
] as const;

export type CompanyDetailTab = (typeof companyDetailTabs)[number];

export function normalizeCompanyDetailTab(value: unknown): CompanyDetailTab {
  return typeof value === 'string' && companyDetailTabs.includes(value as CompanyDetailTab)
    ? (value as CompanyDetailTab)
    : 'overview';
}

/**
 * visibleCompanyDetailTab is the tab to show. The Portal tab is for support
 * admins; anyone else asking for it sees the overview once access is known.
 */
export function visibleCompanyDetailTab(tab: CompanyDetailTab, canManagePortal: boolean, accessKnown: boolean): CompanyDetailTab {
  return tab === 'portal' && accessKnown && !canManagePortal ? 'overview' : tab;
}
