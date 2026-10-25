import type { CRMCustomPropertySummary } from '@/lib/crmTypes';

const SYSTEM_CRM_CUSTOM_PROPERTY_KEYS = new Set([
  'agent_enrichment',
]);

/** Internal CRM metadata is retained for audit/provenance, but is not a customer attribute. */
export function isSystemCRMCustomProperty(key: string): boolean {
  return SYSTEM_CRM_CUSTOM_PROPERTY_KEYS.has(key);
}

/**
 * A key filters as a number when at least half its values are numeric.
 * Records whose value is not numeric never match number comparisons.
 */
export function isNumericCustomProperty(summary: CRMCustomPropertySummary): boolean {
  return summary.numeric_count > 0 && summary.numeric_count * 2 >= summary.count;
}

/** Labels boolean option values Yes and No; other values show as stored. */
export function customPropertyOptionLabel(summary: CRMCustomPropertySummary, value: string): string {
  if (summary.value_type === 'boolean' || summary.value_type === 'mixed') {
    if (value === 'true') return 'Yes';
    if (value === 'false') return 'No';
  }
  return value;
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
