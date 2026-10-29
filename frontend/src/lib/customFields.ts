import { formatDateValue } from '@/lib/datePicker';
import { customPropertyLabel, isNumericCustomProperty } from '@/lib/crmCustomProperties';
import type { CRMCustomPropertySummary } from '@/lib/crmTypes';
import type { QueryBuilderFieldDefinition } from '@/lib/queryBuilder';

/** Where fields are defined: support conversations or CRM records. */
export type CustomFieldObjectType = 'conversation' | 'crm_contact' | 'crm_company';

export type CustomFieldType = 'text' | 'long_text' | 'number' | 'date' | 'boolean' | 'select' | 'multi_select';

export interface CustomFieldOption {
  value: string;
  label: string;
}

/** A workspace field definition. The key is its identity and never changes. */
export interface CustomFieldDefinition {
  id: string;
  object_type: CustomFieldObjectType;
  key: string;
  label: string;
  display_label: string;
  description: string;
  field_type: CustomFieldType;
  options: CustomFieldOption[];
  position: number;
  archived_at?: string | null;
  usage_count?: number;
  mismatch_count?: number;
  usage_sampled?: boolean;
}

export interface CustomFieldDefinitionInput {
  key: string;
  label: string;
  description: string;
  field_type: CustomFieldType;
  options: CustomFieldOption[];
}

export const CUSTOM_FIELD_TYPES: Array<{ value: CustomFieldType; label: string }> = [
  { value: 'text', label: 'Text' },
  { value: 'long_text', label: 'Long text' },
  { value: 'number', label: 'Number' },
  { value: 'date', label: 'Date' },
  { value: 'boolean', label: 'Yes / no' },
  { value: 'select', label: 'Single choice' },
  { value: 'multi_select', label: 'Multiple choice' },
];

/** Filter menu headings: defined fields, then keys in use that nobody defined. */
export const DEFINED_FIELDS_GROUP = 'Fields';
export const UNDEFINED_FIELDS_GROUP = 'Not defined';

export const CUSTOM_FIELD_KEY_PATTERN = /^[a-z][a-z0-9_]{0,63}$/;

export function customFieldTypeLabel(type: CustomFieldType): string {
  return CUSTOM_FIELD_TYPES.find((entry) => entry.value === type)?.label ?? type;
}

export function isChoiceField(type: CustomFieldType): boolean {
  return type === 'select' || type === 'multi_select';
}

/** Suggests a key from a label: "Billing plan" becomes billing_plan. */
export function customFieldKeyFromLabel(label: string): string {
  const key = label
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^[^a-z]+/, '')
    .replace(/_+$/, '');
  return key.slice(0, 64);
}

function optionLabel(definition: CustomFieldDefinition, value: string): string {
  return definition.options.find((option) => option.value === value)?.label ?? value;
}

const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/;
// Decimal numbers only, as the server reads numeric text.
const NUMBER_TEXT_PATTERN = /^-?(\d+\.?\d*|\.\d+)$/;

/** Whether text is an exact, real YYYY-MM-DD date. */
function isCalendarDate(text: string): boolean {
  if (!DATE_PATTERN.test(text)) return false;
  const date = new Date(`${text}T00:00:00Z`);
  return !Number.isNaN(date.getTime()) && date.toISOString().slice(0, 10) === text;
}

/**
 * Whether a stored value fits the definition. Values that do not fit stay
 * stored but read as empty, as the server counts them.
 */
export function customFieldValueFits(definition: CustomFieldDefinition, value: unknown): boolean {
  switch (definition.field_type) {
    case 'text':
    case 'long_text':
      return typeof value === 'string';
    case 'number':
      return typeof value === 'number' ? Number.isFinite(value) : typeof value === 'string' && NUMBER_TEXT_PATTERN.test(value.trim());
    case 'date':
      return typeof value === 'string' && isCalendarDate(value);
    case 'boolean':
      return typeof value === 'boolean';
    case 'select':
      return typeof value === 'string' && definition.options.some((option) => option.value === value);
    case 'multi_select':
      return Array.isArray(value) && value.every((item) => typeof item === 'string' && definition.options.some((option) => option.value === item));
  }
}

/** Formats a stored value with the definition, or null when it is empty or does not fit. */
export function formatCustomFieldValue(definition: CustomFieldDefinition, value: unknown): string | null {
  if (value === null || value === undefined || value === '' || !customFieldValueFits(definition, value)) return null;
  switch (definition.field_type) {
    case 'number':
      return Number(value).toLocaleString();
    case 'date':
      return formatDateValue(value as string, value as string);
    case 'boolean':
      return value ? 'Yes' : 'No';
    case 'select':
      return optionLabel(definition, value as string);
    case 'multi_select': {
      const labels = (value as string[]).map((item) => optionLabel(definition, item));
      return labels.length > 0 ? labels.join(', ') : null;
    }
    default:
      return String(value).trim() || null;
  }
}

/** Raw text of a stored value that does not fit its definition, for a hint. */
export function rawCustomFieldValue(value: unknown): string {
  if (Array.isArray(value)) return value.map(String).join(', ');
  if (value && typeof value === 'object') return JSON.stringify(value);
  return String(value ?? '');
}

/**
 * Query-builder field for a defined field. Options come from the definition;
 * values in the data that are not options still match through the API.
 */
export function customFieldQueryField(definition: CustomFieldDefinition, group?: string): QueryBuilderFieldDefinition {
  const field = `custom_properties.${definition.key}`;
  const label = definition.display_label || customPropertyLabel(definition.key);
  const choice = ['is_any_of', 'is_none_of', 'is_empty', 'is_not_empty'] as const;
  switch (definition.field_type) {
    case 'number':
      return { field, label, group, type: 'number', placeholder: 'Number' };
    case 'date':
      return { field, label, group, type: 'date' };
    case 'boolean':
      return { field, label, group, type: 'enum', operators: [...choice], options: [{ value: 'true', label: 'Yes' }, { value: 'false', label: 'No' }] };
    case 'select':
    case 'multi_select':
      return { field, label, group, type: 'enum', placeholder: 'Select value', operators: [...choice], options: definition.options };
    default:
      return { field, label, group, type: 'text', placeholder: label };
  }
}

// Text keys with at most this many distinct values are offered as a choice.
const CHOICE_SUGGESTION_LIMIT = 20

/** Suggests a definition for a key found in the data. Defining never changes values. */
export function suggestDefinition(summary: CRMCustomPropertySummary): Partial<CustomFieldDefinitionInput> {
  const base = { key: summary.key, label: '', description: '' }
  if (isNumericCustomProperty(summary)) return { ...base, field_type: 'number', options: [] }
  if (summary.value_type === 'boolean') return { ...base, field_type: 'boolean', options: [] }
  const options = summary.values.map((entry) => ({ value: entry.value, label: '' }))
  if (summary.value_type === 'list') return { ...base, field_type: 'multi_select', options }
  if (!summary.has_more_values && summary.values.length > 0 && summary.values.length <= CHOICE_SUGGESTION_LIMIT && summary.values.length < summary.count) {
    return { ...base, field_type: 'select', options }
  }
  return { ...base, field_type: 'text', options: [] }
}
