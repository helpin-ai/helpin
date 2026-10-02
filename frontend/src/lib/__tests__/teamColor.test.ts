import { describe, expect, it } from 'vitest';
import { normalizeTeamColor, teamColorStyle, TEAM_PRESET_COLORS } from '../teamColor';

describe('team colors', () => {
  it('normalizes saved hex values and rejects arbitrary CSS', () => {
    expect(normalizeTeamColor(' #AbC ')).toBe('#aabbcc');
    expect(normalizeTeamColor('#ABCDEF')).toBe('#abcdef');
    for (const value of [undefined, null, '', 'red', 'var(--primary)', '#11223344', '#zzzzzz']) {
      expect(normalizeTeamColor(value)).toBeNull();
    }
  });

  it('keeps initials readable on both light and dark team colors', () => {
    const luminance = (hex: string) => {
      const channels = [1, 3, 5].map(offset => {
        const value = parseInt(hex.slice(offset, offset + 2), 16) / 255;
        return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      });
      return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
    };
    for (const color of ['#ffffff', '#000000', ...TEAM_PRESET_COLORS]) {
      const style = teamColorStyle(color)!;
      expect(style['--team-color-bg']).toBe(color);
      expect(style['--team-color-bg-dark']).toBe(color);
      for (const suffix of ['', '-dark']) {
        const bg = luminance(style[`--team-color-bg${suffix}`]);
        const fg = luminance(style[`--team-color-fg${suffix}`]);
        expect((Math.max(bg, fg) + 0.05) / (Math.min(bg, fg) + 0.05)).toBeGreaterThanOrEqual(4.5);
      }
    }
    expect(teamColorStyle('var(--primary)')).toBeUndefined();
  });
});
