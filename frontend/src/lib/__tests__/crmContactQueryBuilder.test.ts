import { describe, expect, it } from 'vitest';
import { buildCRMContactQueryFields } from '../crmContactQueryBuilder';
import type { CRMContactFilterOptions } from '../crmTypes';

const OPTIONS: CRMContactFilterOptions = {
  scanned_contacts: 3,
  truncated: false,
  custom_properties: [
    { key: 'plan_tier', count: 3, value_type: 'text', values: [{ value: 'pro', count: 2 }, { value: 'scale', count: 1 }], has_more_values: false, numeric_count: 0 },
    { key: 'seats', count: 3, value_type: 'number', values: [{ value: '12', count: 1 }], has_more_values: false, numeric_count: 3, numeric_min: 1, numeric_max: 40 },
    { key: 'beta', count: 2, value_type: 'boolean', values: [{ value: 'true', count: 2 }], has_more_values: false, numeric_count: 0 },
    { key: 'agent_enrichment', count: 1, value_type: 'text', values: [], has_more_values: false, numeric_count: 0 },
  ],
};

describe('buildCRMContactQueryFields', () => {
  it('offers portal access as an enum filter', () => {
    const field = buildCRMContactQueryFields([]).find((item) => item.field === 'portal_access');
    expect(field?.type).toBe('enum');
    expect(field?.options?.map((option) => option.value)).toEqual(['allowed', 'blocked']);
  });

  it('offers synced behavior fields', () => {
    const fields = buildCRMContactQueryFields([]);
    expect(fields.find((item) => item.field === 'last_seen_at')?.type).toBe('date');
    expect(fields.find((item) => item.field === 'sessions_30d')?.type).toBe('number');
  });

  it('adds custom properties in use, as option or number filters', () => {
    const fields = buildCRMContactQueryFields([], OPTIONS);
    const plan = fields.find((item) => item.field === 'custom_properties.plan_tier');
    expect(plan).toMatchObject({ label: 'Plan tier', type: 'enum', operators: ['is_any_of', 'is_none_of', 'is_empty', 'is_not_empty'] });
    expect(plan?.options?.map((option) => option.value)).toEqual(['pro', 'scale']);
    expect(fields.find((item) => item.field === 'custom_properties.seats')?.type).toBe('number');
    expect(fields.find((item) => item.field === 'custom_properties.beta')?.options?.[0].label).toBe('Yes');
    expect(fields.some((item) => item.field === 'custom_properties.agent_enrichment')).toBe(false);
  });
});
