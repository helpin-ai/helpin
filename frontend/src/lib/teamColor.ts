import type { CSSProperties } from 'react';

export const TEAM_PRESET_COLORS = ['#4e8fea', '#2da88e', '#45a557', '#c7a53d', '#e58c3a', '#e2564a', '#e54e78', '#8b5cf6'] as const;

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

/** Soft team backgrounds with readable foregrounds in either theme. */
export function teamColorStyle(value?: string | null): (CSSProperties & Record<string, string>) | undefined {
  const hex = normalizeTeamColor(value);
  if (!hex) return undefined;
  const mix = (target: number, amount: number) => `#${[1, 3, 5].map(offset => {
    const channel = parseInt(hex.slice(offset, offset + 2), 16);
    return Math.round(channel * (1 - amount) + target * amount).toString(16).padStart(2, '0');
  }).join('')}`;
  return {
    '--team-color-bg': mix(255, 0.88),
    '--team-color-fg': mix(0, 0.65),
    '--team-color-bg-dark': mix(0, 0.74),
    '--team-color-fg-dark': mix(255, 0.66),
  };
}
