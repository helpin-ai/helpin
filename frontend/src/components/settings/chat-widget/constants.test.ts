import { describe, expect, it } from 'vitest';
import { ICON_OPTIONS } from './constants';

describe('chat widget settings constants', () => {
  it('uses distinct icons for launcher icon choices', () => {
    const iconComponents = ICON_OPTIONS.map((option) => option.icon);

    expect(new Set(iconComponents).size).toBe(iconComponents.length);
  });

});
