export const DEFAULT_EPIC_COLOR = '#788596';

/** Resolve legacy hex values without passing arbitrary CSS through to epic visuals. */
export function resolveEpicColor(value?: string | null): string {
  const hex = value?.trim().replace(/^#/, '') ?? '';
  if (/^[\da-f]{6}$/i.test(hex)) return `#${hex.toLowerCase()}`;
  if (/^[\da-f]{3}$/i.test(hex)) return `#${[...hex].map((digit) => digit.repeat(2)).join('').toLowerCase()}`;
  return DEFAULT_EPIC_COLOR;
}

/** Choose the higher-contrast text color against an opaque epic background. */
export function getEpicBadgeTextColor(value?: string | null): '#000000' | '#ffffff' {
  const hex = resolveEpicColor(value);
  const [red, green, blue] = [1, 3, 5].map((offset) => {
    const channel = parseInt(hex.slice(offset, offset + 2), 16) / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  const luminance = 0.2126 * red + 0.7152 * green + 0.0722 * blue;
  const darkContrast = (luminance + 0.05) / 0.05;
  const lightContrast = 1.05 / (luminance + 0.05);
  return lightContrast > darkContrast ? '#ffffff' : '#000000';
}
