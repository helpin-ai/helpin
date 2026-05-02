import type { SupportCannedResponse } from '@/lib/pmTypes';

export function stripShortcutContent(value: string) {
  if (!value) return '';
  if (typeof DOMParser === 'undefined') {
    return value.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  }
  const doc = new DOMParser().parseFromString(value, 'text/html');
  return doc.body.textContent?.replace(/\s+/g, ' ').trim() || value.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
}

export function filterShortcuts(shortcuts: SupportCannedResponse[], query: string, limit = 8) {
  const normalized = query.trim().toLowerCase().replace(/^!/, '');
  // Shortcut filtering matches short_code first (prefix preferred), then falls
  // back to the saved body. Category matching is intentionally excluded so a
  // typed token like "h" does not match almost every grouped reply.
  return shortcuts
    .filter((item) => {
      if (!normalized) return true;
      const code = item.short_code.toLowerCase().replace(/^!/, '');
      const content = stripShortcutContent(item.content).toLowerCase();
      return code.includes(normalized) || content.includes(normalized);
    })
    .sort((a, b) => {
      if (!normalized) return a.short_code.localeCompare(b.short_code);
      const aCode = a.short_code.toLowerCase().replace(/^!/, '');
      const bCode = b.short_code.toLowerCase().replace(/^!/, '');
      const aPrefix = aCode.startsWith(normalized) ? 0 : 1;
      const bPrefix = bCode.startsWith(normalized) ? 0 : 1;
      if (aPrefix !== bPrefix) return aPrefix - bPrefix;
      return a.short_code.localeCompare(b.short_code);
    })
    .slice(0, limit);
}
