import type { CSSProperties } from 'react';

export function normalizeHexColor(color: string | null | undefined): string | null {
  const value = color?.trim().replace(/^#/, '') ?? '';
  if (/^[0-9a-f]{3}$/i.test(value)) {
    return `#${value.split('').map((digit) => `${digit}${digit}`).join('').toLowerCase()}`;
  }
  return /^[0-9a-f]{6}$/i.test(value) ? `#${value.toLowerCase()}` : null;
}

export function hexToRgb(color: string) {
  const normalized = normalizeHexColor(color);
  if (!normalized) return null;
  return {
    r: Number.parseInt(normalized.slice(1, 3), 16),
    g: Number.parseInt(normalized.slice(3, 5), 16),
    b: Number.parseInt(normalized.slice(5, 7), 16),
  };
}

export function tintBg(color: string | null | undefined) {
  const normalized = normalizeHexColor(color);
  return normalized ? `${normalized}1f` : undefined;
}

export function tintBorder(color: string | null | undefined) {
  const normalized = normalizeHexColor(color);
  return normalized ? `${normalized}40` : undefined;
}

function rgbToHex({ r, g, b }: { r: number; g: number; b: number }) {
  const clamp = (value: number) => Math.max(0, Math.min(255, Math.round(value)));
  return `#${clamp(r).toString(16).padStart(2, '0')}${clamp(g).toString(16).padStart(2, '0')}${clamp(b).toString(16).padStart(2, '0')}`;
}

function tagTextColor(color: string | null | undefined) {
  const rgb = color ? hexToRgb(color) : null;
  if (!rgb) return undefined;
  const luminance = ((0.2126 * rgb.r) + (0.7152 * rgb.g) + (0.0722 * rgb.b)) / 255;
  const factor = luminance > 0.58 ? 0.55 : 0.82;
  return rgbToHex({ r: rgb.r * factor, g: rgb.g * factor, b: rgb.b * factor });
}

/** Shared color treatment for support tags in the sidebar and conversation list. */
export function getSupportTagStyle(color: string | null | undefined): CSSProperties {
  return {
    backgroundColor: tintBg(color) ?? 'var(--muted)',
    borderColor: tintBorder(color) ?? 'var(--border)',
    color: tagTextColor(color) ?? 'var(--foreground)',
  };
}
