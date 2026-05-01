import { describe, expect, it } from 'vitest';
import {
  DEFAULT_SHORTCUT_CATEGORY,
  normalizeShortcutCategory,
  shortcutCategoryOptions,
} from '../shortcutCategories';

describe('shortcutCategories', () => {
  it('defaults blank categories to General', () => {
    expect(DEFAULT_SHORTCUT_CATEGORY).toBe('General');
    expect(normalizeShortcutCategory('')).toBe('General');
    expect(normalizeShortcutCategory('   ')).toBe('General');
  });

  it('offers built-in and saved categories without duplicates', () => {
    expect(shortcutCategoryOptions([
      { tag: 'Billing' },
      { tag: '  Billing  ' },
      { tag: 'Refunds' },
      { tag: 'Others' },
      { tag: '' },
    ])).toEqual(['General', 'Support', 'Sales', 'Billing', 'Refunds']);
  });
});
