import { describe, expect, it } from 'vitest';
import {
  parseQueryFilterGroup,
  serializeQueryFilterGroup,
  type QueryFilterGroup,
} from '@/lib/queryBuilder';

describe('queryBuilder helpers', () => {
  it('serializes and parses a complete filter group', () => {
    const group: QueryFilterGroup = {
      logic: 'and',
      rules: [
        { field: 'job_title', operator: 'contains', value: 'Founder' },
        { field: 'created_at', operator: 'between', values: ['2026-04-10', '2026-04-13'] },
      ],
    };

    const serialized = serializeQueryFilterGroup(group);
    expect(serialized).toBeTruthy();
    expect(parseQueryFilterGroup(serialized)).toEqual(group);
  });

  it('drops incomplete rules during serialization', () => {
    const serialized = serializeQueryFilterGroup({
      logic: 'and',
      rules: [
        { field: 'job_title', operator: 'contains', value: 'Founder' },
        { field: 'owner_member_id', operator: 'is' },
      ],
    });

    expect(parseQueryFilterGroup(serialized)).toEqual({
      logic: 'and',
      rules: [
        { field: 'job_title', operator: 'contains', value: 'Founder' },
      ],
    });
  });
});
