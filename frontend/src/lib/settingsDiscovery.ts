import { SETTINGS_ROUTE_SECTIONS, type SettingsRouteSection, type SettingsSidebarGroup } from './settingsSections';

export const buildSettingsHomePath = (slug: string) => `/w/${slug}/settings`;

export type SettingsResult = { sectionId: SettingsRouteSection; label: string; pageLabel: string; group: string; description: string; optionId?: string; scope?: 'organization' };
const normalize = (value: string) => value.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, ' ').trim();
export function searchSettings(groups: SettingsSidebarGroup[], query: string): SettingsResult[] {
  const words = normalize(query).split(/\s+/).filter(Boolean);
  if (!words.length) return [];
  return groups.flatMap(group => group.sections.flatMap(section => {
    const base = { sectionId: section.id, pageLabel: section.label, group: group.label, scope: section.scope };
    const candidates = [
      { ...base, label: section.label, description: section.description, keywords: section.keywords ?? [], optionId: undefined as string | undefined },
      ...(section.options ?? []).map(option => ({ ...base, label: option.label, description: `Open ${option.label.toLowerCase()}`, keywords: [...(section.keywords ?? []), ...(option.keywords ?? [])], optionId: option.id })),
    ];
    return candidates.filter(item => words.every(word => normalize([item.label, section.label, group.label, item.description, ...item.keywords].join(' ')).includes(word)))
      .map(item => ({ ...item, score: words.reduce((score, word) => score + (normalize(item.label).includes(word) ? 10 : 1), item.optionId ? 1 : 0) }));
  })).sort((a, b) => b.score - a.score || a.label.localeCompare(b.label));
}
const recentKey = (scope: string) => `settings_recent:${scope}`;
export function readRecentSettings(scope: string): SettingsRouteSection[] {
  try {
    const stored: unknown = JSON.parse(localStorage.getItem(recentKey(scope)) ?? '[]');
    return Array.isArray(stored) ? stored.filter((id): id is SettingsRouteSection => SETTINGS_ROUTE_SECTIONS.some(section => section.id === id)).slice(0, 4) : [];
  } catch { return []; }
}
export function rememberSetting(scope: string, section: SettingsRouteSection) {
  try { localStorage.setItem(recentKey(scope), JSON.stringify([section, ...readRecentSettings(scope).filter(id => id !== section)].slice(0, 4))); } catch { /* Optional local history. */ }
}
export function revealSettingOption(optionId: string): boolean {
  if (!/^[a-z0-9-]+$/.test(optionId)) return false;
  const targets = [...document.querySelectorAll<HTMLElement>(`[data-settings-option="${optionId}"]`)];
  if (!targets.length) return false;
  for (const target of targets) {
    for (let element: HTMLElement | null = target; element; element = element.parentElement) {
      if (element instanceof HTMLDetailsElement) element.open = true;
    }
    if (target.getAttribute('aria-expanded') === 'false') target.click();
  }
  const target = targets[0];
  const focusTarget = target.matches('details') ? target.querySelector<HTMLElement>('summary') : target;
  focusTarget?.focus({ preventScroll: true });
  target.scrollIntoView?.({ block: 'center', behavior: 'smooth' });
  return true;
}
