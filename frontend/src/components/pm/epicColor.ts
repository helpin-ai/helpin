export const DEFAULT_EPIC_COLOR = '#788596';

/** Resolve legacy hex values without passing arbitrary CSS through to epic visuals. */
export function resolveEpicColor(value?: string | null): string {
  const hex = value?.trim().replace(/^#/, '') ?? '';
  if (/^[\da-f]{6}$/i.test(hex)) return `#${hex.toLowerCase()}`;
  if (/^[\da-f]{3}$/i.test(hex)) return `#${[...hex].map((digit) => digit.repeat(2)).join('').toLowerCase()}`;
  return DEFAULT_EPIC_COLOR;
}
