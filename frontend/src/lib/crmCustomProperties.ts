const SYSTEM_CRM_CUSTOM_PROPERTY_KEYS = new Set([
  'agent_enrichment',
]);

/** Internal CRM metadata is retained for audit/provenance, but is not a customer attribute. */
export function isSystemCRMCustomProperty(key: string): boolean {
  return SYSTEM_CRM_CUSTOM_PROPERTY_KEYS.has(key);
}

export interface CustomPropertyRow {
  key: string;
  label: string;
  value: string;
  href?: string;
}

/** Turns a snake_case property key into a sentence-case label. */
export function customPropertyLabel(key: string): string {
  const words = key.split('_').filter(Boolean);
  if (words.length === 0) return key;
  const label = words.join(' ');
  return label.charAt(0).toUpperCase() + label.slice(1);
}

/** Formats a custom property value for display, or null when it has none. */
export function formatCustomPropertyValue(value: unknown): string | null {
  if (value === null || value === undefined) return null;
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'number') return Number.isFinite(value) ? value.toLocaleString() : null;
  if (typeof value === 'string') return value.trim() || null;
  if (Array.isArray(value)) {
    const items = value.filter((item): item is string | number => typeof item === 'string' || typeof item === 'number');
    return items.length > 0 ? items.join(', ') : null;
  }
  return null;
}

function safeHref(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined;
  try {
    const url = new URL(value.trim());
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.toString() : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Returns displayable custom properties sorted by key. System keys, empty
 * values, and nested objects are left out.
 */
export function customPropertyRows(properties: Record<string, unknown> | null | undefined): CustomPropertyRow[] {
  if (!properties) return [];
  return Object.keys(properties)
    .filter((key) => !isSystemCRMCustomProperty(key))
    .sort()
    .flatMap((key) => {
      const raw = properties[key];
      const value = formatCustomPropertyValue(raw);
      if (value === null) return [];
      return [{ key, label: customPropertyLabel(key), value, href: safeHref(raw) }];
    });
}
