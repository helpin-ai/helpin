import { SETTINGS_ROUTE_SECTIONS } from '@/lib/settingsSections';
const CRM_SECTIONS = new Set(['overview', 'contacts', 'companies', 'deals', 'meetings', 'emails', 'review', 'insights']);

const crmLastPathKey = (workspaceId: string) => `crm_sidebar_last_path_${workspaceId}`;

export const COLLAPSIBLE_SETTINGS_GROUPS = new Set(SETTINGS_ROUTE_SECTIONS.map(section => section.group));

export function getExpandedTeams(workspaceId: string): Set<string> {
  try {
    const raw = localStorage.getItem(`pm_sidebar_expanded_teams_${workspaceId}`);
    if (raw) {
      return new Set(JSON.parse(raw));
    }
  } catch { /* local preference is best effort */ }

  return new Set();
}

export function saveExpandedTeams(workspaceId: string, teams: Set<string>) {
  try {
    localStorage.setItem(`pm_sidebar_expanded_teams_${workspaceId}`, JSON.stringify([...teams]));
  } catch { /* local preference is best effort */ }
}

export function getCollapsedSettingsGroups(): Set<string> {
  try {
    const raw = localStorage.getItem('settings_sidebar_collapsed_v2');
    if (raw) {
      return new Set(JSON.parse(raw));
    }
  } catch { /* local preference is best effort */ }

  return new Set(COLLAPSIBLE_SETTINGS_GROUPS);
}

export function saveCollapsedSettingsGroups(groups: Set<string>) {
  try {
    localStorage.setItem('settings_sidebar_collapsed_v2', JSON.stringify([...groups]));
  } catch { /* local preference is best effort */ }
}

export function normalizeCRMSectionPath(workspaceSlug: string, pathname?: string): string {
  const basePath = `/w/${workspaceSlug}/crm`;
  if (!pathname?.startsWith(`${basePath}/`)) {
    return `${basePath}/overview`;
  }

  const section = pathname.slice(basePath.length + 1).split('/')[0];
  return CRM_SECTIONS.has(section) ? `${basePath}/${section}` : `${basePath}/overview`;
}

export function getLastCRMPath(workspaceId: string, workspaceSlug: string): string {
  if (!workspaceId) {
    return normalizeCRMSectionPath(workspaceSlug);
  }

  try {
    return normalizeCRMSectionPath(workspaceSlug, localStorage.getItem(crmLastPathKey(workspaceId)) ?? undefined);
  } catch {
    return normalizeCRMSectionPath(workspaceSlug);
  }
}

export function saveLastCRMPath(workspaceId: string, workspaceSlug: string, pathname: string): string {
  const normalizedPath = normalizeCRMSectionPath(workspaceSlug, pathname);
  if (!workspaceId) {
    return normalizedPath;
  }

  try {
    localStorage.setItem(crmLastPathKey(workspaceId), normalizedPath);
  } catch { /* local preference is best effort */ }
  return normalizedPath;
}
