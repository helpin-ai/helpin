import { type QueryBuilderFieldDefinition } from '@/lib/queryBuilder';
import type { LifecycleStage, LeadStatus } from '@/lib/crmTypes';
import type { AssignableMember } from '@/lib/types';

const LIFECYCLE_STAGES: Array<{ value: LifecycleStage; label: string }> = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'Marketing Qualified' },
  { value: 'sales_qualified', label: 'Sales Qualified' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
];

const LEAD_STATUSES: Array<{ value: LeadStatus; label: string }> = [
  { value: 'new', label: 'New' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'unqualified', label: 'Unqualified' },
];

// "Is empty" matches contacts without a portal decision.
const PORTAL_ACCESS_OPTIONS: Array<{ value: 'allowed' | 'blocked'; label: string }> = [
  { value: 'allowed', label: 'Allowed' },
  { value: 'blocked', label: 'Blocked' },
];

export function buildCRMContactQueryFields(assignableMembers: AssignableMember[]): QueryBuilderFieldDefinition[] {
  return [
    {
      field: 'name',
      label: 'Name',
      type: 'text',
      placeholder: 'Contact name',
    },
    {
      field: 'email',
      label: 'Email',
      type: 'text',
      placeholder: 'Email address',
    },
    {
      field: 'phone',
      label: 'Phone',
      type: 'text',
      placeholder: 'Phone number',
    },
    {
      field: 'job_title',
      label: 'Job title',
      type: 'text',
      placeholder: 'Job title',
    },
    {
      field: 'lifecycle_stage',
      label: 'Lifecycle stage',
      type: 'enum',
      placeholder: 'Select lifecycle stage',
      options: LIFECYCLE_STAGES,
    },
    {
      field: 'lead_status',
      label: 'Lead status',
      type: 'enum',
      placeholder: 'Select lead status',
      options: LEAD_STATUSES,
    },
    {
      field: 'owner_member_id',
      label: 'Owner',
      type: 'member',
      placeholder: 'Select owner',
      options: assignableMembers.map((member) => ({
        value: member.id,
        label: member.display_name || member.email,
      })),
    },
    {
      field: 'portal_access',
      label: 'Portal access',
      type: 'enum',
      placeholder: 'Select portal access',
      options: PORTAL_ACCESS_OPTIONS,
    },
    {
      field: 'source',
      label: 'Source',
      type: 'text',
      placeholder: 'Source',
    },
    {
      field: 'created_at',
      label: 'Created at',
      type: 'date',
    },
    {
      field: 'updated_at',
      label: 'Updated at',
      type: 'date',
    },
  ];
}
