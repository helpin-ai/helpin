const COLLAPSIBLE_SETTINGS_GROUP_LABELS = ['Project Settings', 'Support & Docs', 'CRM Settings', 'AI & Automations', 'Data'];

export const COLLAPSIBLE_SETTINGS_GROUPS = new Set(COLLAPSIBLE_SETTINGS_GROUP_LABELS);

export function getExpandedTeams(workspaceId: string): Set<string> {
  try {
    const raw = localStorage.getItem(`pm_sidebar_expanded_teams_${workspaceId}`);
    if (raw) {
      return new Set(JSON.parse(raw));
    }
  } catch {}

  return new Set();
}

export function saveExpandedTeams(workspaceId: string, teams: Set<string>) {
  try {
    localStorage.setItem(`pm_sidebar_expanded_teams_${workspaceId}`, JSON.stringify([...teams]));
  } catch {}
}

export function getCollapsedSettingsGroups(): Set<string> {
  try {
    const raw = localStorage.getItem('settings_sidebar_collapsed');
    if (raw) {
      return new Set(JSON.parse(raw));
    }
  } catch {}

  return new Set(COLLAPSIBLE_SETTINGS_GROUPS);
}

export function saveCollapsedSettingsGroups(groups: Set<string>) {
  try {
    localStorage.setItem('settings_sidebar_collapsed', JSON.stringify([...groups]));
  } catch {}
}

export function getDocsOutlineOpen(workspaceId: string): boolean | null {
  try {
    const raw = localStorage.getItem(`docs_outline_open_${workspaceId}`);
    if (raw === 'true') return true;
    if (raw === 'false') return false;
  } catch {}
  return null;
}

export function setDocsOutlineOpen(workspaceId: string, open: boolean) {
  try {
    localStorage.setItem(`docs_outline_open_${workspaceId}`, open ? 'true' : 'false');
  } catch {}
}
