export const DEFAULT_SHORTCUT_CATEGORY = 'General';
export const BUILT_IN_SHORTCUT_CATEGORIES = [DEFAULT_SHORTCUT_CATEGORY, 'Support', 'Sales', 'Billing'];

export function normalizeShortcutCategory(value?: string | null) {
  const category = value?.trim() ?? '';
  if (!category || category === 'Others') return DEFAULT_SHORTCUT_CATEGORY;
  return category;
}

export function shortcutCategoryOptions(items: Array<{ tag?: string | null }>) {
  return Array.from(new Set([
    ...BUILT_IN_SHORTCUT_CATEGORIES,
    ...items.map((item) => normalizeShortcutCategory(item.tag)),
  ]));
}
