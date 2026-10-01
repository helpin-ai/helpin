import type { CSSProperties } from 'react';
import { EPIC_PRESET_COLORS } from './colorPresets';
import { getEpicBadgeTextColor } from '@/components/pm/epicColor';

export const TEAM_PRESET_COLORS = EPIC_PRESET_COLORS;

export function randomTeamColor(): string {
  return TEAM_PRESET_COLORS[Math.floor(Math.random() * TEAM_PRESET_COLORS.length)];
}

/** Accept saved hex values without passing arbitrary CSS into team identity. */
export function normalizeTeamColor(value?: string | null): string | null {
  const hex = value?.trim().toLowerCase() ?? '';
  if (/^#[\da-f]{6}$/.test(hex)) return hex;
  if (/^#[\da-f]{3}$/.test(hex)) return `#${[...hex.slice(1)].map(digit => digit.repeat(2)).join('')}`;
  return null;
}

/** Keep the selected shade exact, with readable initials in either theme. */
export function teamColorStyle(value?: string | null): (CSSProperties & Record<string, string>) | undefined {
  const hex = normalizeTeamColor(value);
  if (!hex) return undefined;
  const foreground = getEpicBadgeTextColor(hex);
  return {
    '--team-color-bg': hex,
    '--team-color-fg': foreground,
    '--team-color-bg-dark': hex,
    '--team-color-fg-dark': foreground,
  };
}
