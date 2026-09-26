import type { QueryBuilderFieldDefinition } from './queryBuilder';
import type { AssignableMember } from './types';
export function buildCRMDealQueryFields(
  members: AssignableMember[],
  stages: { id: string; name: string }[],
): QueryBuilderFieldDefinition[] {
  return [
    {
      field: 'name',
      label: 'Deal name',
      type: 'text',
      placeholder: 'Deal name',
    },
    {
      field: 'stage_id',
      label: 'Stage',
      type: 'enum',
      options: stages.map((s) => ({ value: s.id, label: s.name })),
    },
    {
      field: 'stage_type',
      label: 'Outcome',
      type: 'enum',
      options: [
        { value: 'open', label: 'Open' },
        { value: 'won', label: 'Won' },
        { value: 'lost', label: 'Lost' },
      ],
    },
    {
      field: 'owner_member_id',
      label: 'Owner',
      type: 'member',
      options: members.map((m) => ({
        value: m.id,
        label: m.display_name || m.email,
      })),
    },
    { field: 'amount', label: 'Amount', type: 'number' },
    {
      field: 'currency',
      label: 'Currency',
      type: 'enum',
      options: ['USD', 'EUR', 'GBP', 'CAD', 'AUD'].map((value) => ({
        value,
        label: value,
      })),
    },
    {
      field: 'revenue_type',
      label: 'Revenue type',
      type: 'enum',
      options: [
        { value: 'one_time', label: 'One-time' },
        { value: 'monthly', label: 'Monthly' },
        { value: 'annual', label: 'Annual' },
      ],
    },
    { field: 'probability', label: 'Probability', type: 'number' },
    { field: 'close_date', label: 'Expected close date', type: 'date' },
    { field: 'created_at', label: 'Created at', type: 'date' },
    { field: 'updated_at', label: 'Updated at', type: 'date' },
  ];
}
