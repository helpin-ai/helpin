import { describe, expect, it } from 'vitest';
import { normalizeQueryFilterGroup, operatorUsesValueList } from '../queryBuilder';

describe('value-list operators', () => {
  it('keeps is_any_of values and drops rules without any', () => {
    expect(operatorUsesValueList('is_any_of')).toBe(true);
    expect(operatorUsesValueList('is')).toBe(false);
    expect(normalizeQueryFilterGroup({
      logic: 'and',
      rules: [
        { field: 'custom_properties.plan_tier', operator: 'is_any_of', values: ['pro'] },
        { field: 'custom_properties.beta', operator: 'is_none_of', values: [] },
      ],
    })).toEqual({ logic: 'and', rules: [{ field: 'custom_properties.plan_tier', operator: 'is_any_of', values: ['pro'] }] });
  });
});
