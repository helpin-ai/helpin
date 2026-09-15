import { expect, it } from 'vitest';
import { buildCRMDealQueryFields } from '../crmDealQueryBuilder';
import {
  getOperatorsForField,
  serializeQueryFilterGroup,
  parseQueryFilterGroup,
} from '../queryBuilder';
it('offers numeric amount comparisons and stage choices from the selected pipeline', () => {
  const fields = buildCRMDealQueryFields([], [{ id: 'lead', name: 'Lead' }]);
  expect(fields.find((f) => f.field === 'stage_id')?.options).toEqual([
    { value: 'lead', label: 'Lead' },
  ]);
  expect(
    getOperatorsForField(fields.find((f) => f.field === 'amount')!),
  ).toContain('gte');
  expect(
    getOperatorsForField(fields.find((f) => f.field === 'amount')!),
  ).not.toContain('contains');
});
it('preserves zero and numeric ranges through URL serialization', () => {
  const group = {
    logic: 'and' as const,
    rules: [
      { field: 'amount', operator: 'between' as const, values: ['0', '1000'] },
    ],
  };
  expect(parseQueryFilterGroup(serializeQueryFilterGroup(group))).toEqual(
    group,
  );
});
