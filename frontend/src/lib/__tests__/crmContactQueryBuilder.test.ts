import { describe, expect, it } from 'vitest';
import { buildCRMContactQueryFields } from '../crmContactQueryBuilder';

describe('buildCRMContactQueryFields', () => {
  it('offers portal access as an enum filter', () => {
    const field = buildCRMContactQueryFields([]).find((item) => item.field === 'portal_access');
    expect(field?.type).toBe('enum');
    expect(field?.options?.map((option) => option.value)).toEqual(['allowed', 'blocked']);
  });
});
