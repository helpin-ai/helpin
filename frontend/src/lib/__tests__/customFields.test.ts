import { describe, expect, it } from 'vitest';

import {
  customFieldKeyFromLabel,
  customFieldQueryField,
  customFieldValueFits,
  formatCustomFieldValue,
  suggestDefinition,
  type CustomFieldDefinition,
} from '../customFields';

function field(partial: Partial<CustomFieldDefinition>): CustomFieldDefinition {
  return { id: 'f', object_type: 'crm_company', key: 'plan', label: '', display_label: 'Plan', description: '', field_type: 'text', options: [], position: 0, ...partial };
}

const plan = field({ field_type: 'select', options: [{ value: 'pro', label: 'Pro' }, { value: 'free', label: 'Free' }] });

describe('customFieldValueFits', () => {
  it.each([
    [field({ field_type: 'number' }), 12, true],
    [field({ field_type: 'number' }), '12.5', true],
    [field({ field_type: 'number' }), 'many', false],
    [field({ field_type: 'date' }), '2026-10-05', true],
    [field({ field_type: 'date' }), 'soon', false],
    [field({ field_type: 'boolean' }), false, true],
    [field({ field_type: 'boolean' }), 'yes', false],
    [plan, 'pro', true],
    [plan, 'enterprise', false],
    [field({ field_type: 'multi_select', options: plan.options }), ['pro', 'free'], true],
    [field({ field_type: 'multi_select', options: plan.options }), ['pro', 'gold'], false],
    [field({ field_type: 'text' }), { nested: true }, false],
  ])('%#', (definition, value, fits) => {
    expect(customFieldValueFits(definition, value)).toBe(fits);
  });
});

describe('formatCustomFieldValue', () => {
  it('uses option labels, yes/no and dates, and reads values that do not fit as empty', () => {
    expect(formatCustomFieldValue(plan, 'pro')).toBe('Pro');
    expect(formatCustomFieldValue(field({ field_type: 'multi_select', options: plan.options }), ['free', 'pro'])).toBe('Free, Pro');
    expect(formatCustomFieldValue(field({ field_type: 'boolean' }), true)).toBe('Yes');
    expect(formatCustomFieldValue(field({ field_type: 'date' }), '2026-10-05')).toBe('Oct 5, 2026');
    expect(formatCustomFieldValue(plan, 'enterprise')).toBeNull();
    expect(formatCustomFieldValue(plan, null)).toBeNull();
  });
});

describe('customFieldKeyFromLabel', () => {
  it('suggests a valid key', () => {
    expect(customFieldKeyFromLabel('Billing plan')).toBe('billing_plan');
    expect(customFieldKeyFromLabel('  2nd Région! ')).toBe('nd_region');
    expect(customFieldKeyFromLabel('')).toBe('');
  });
});

describe('suggestDefinition', () => {
  const summary = { key: 'tier', count: 10, value_type: 'text' as const, values: [{ value: 'gold', count: 6 }, { value: 'silver', count: 4 }], has_more_values: false, numeric_count: 0 };
  it('suggests choices for few repeated values, numbers for numeric keys and lists for arrays', () => {
    expect(suggestDefinition(summary)).toMatchObject({ key: 'tier', field_type: 'select', options: [{ value: 'gold' }, { value: 'silver' }] });
    expect(suggestDefinition({ ...summary, numeric_count: 10 })).toMatchObject({ field_type: 'number', options: [] });
    expect(suggestDefinition({ ...summary, value_type: 'list' })).toMatchObject({ field_type: 'multi_select' });
    expect(suggestDefinition({ ...summary, has_more_values: true })).toMatchObject({ field_type: 'text' });
  });
});

describe('customFieldQueryField', () => {
  it('types filters by the definition', () => {
    expect(customFieldQueryField(field({ field_type: 'date' }), 'Fields')).toMatchObject({ field: 'custom_properties.plan', type: 'date', group: 'Fields' });
    expect(customFieldQueryField(plan)).toMatchObject({ type: 'enum', options: plan.options });
    expect(customFieldQueryField(field({ field_type: 'boolean' })).options).toEqual([{ value: 'true', label: 'Yes' }, { value: 'false', label: 'No' }]);
  });
});
