import { type QueryBuilderFieldDefinition } from '@/lib/queryBuilder';
import {
  customPropertyLabel,
  customPropertyOptionLabel,
  isNumericCustomProperty,
  isSystemCRMCustomProperty,
} from '@/lib/crmCustomProperties';
import type { CRMContactFilterOptions, LeadStatus } from '@/lib/crmTypes';
import { DEFINED_FIELDS_GROUP, UNDEFINED_FIELDS_GROUP, customFieldQueryField, type CustomFieldDefinition } from '@/lib/customFields';
import type { AssignableMember } from '@/lib/types';
import { LIFECYCLE_STAGE_OPTIONS } from '@/lib/crmLifecycle';

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

/**
 * Query fields for custom properties: defined fields first, typed by their
 * definition, then keys in use that nobody defined. Option-style keys offer
 * the values found in the data; is_any_of also matches list elements.
 */
function buildContactCustomPropertyFields(
  options: CRMContactFilterOptions | undefined,
  definitions: CustomFieldDefinition[],
): QueryBuilderFieldDefinition[] {
  const defined = new Set(definitions.map((definition) => definition.key));
  const undefinedFields = (options?.custom_properties ?? [])
    .filter((summary) => !isSystemCRMCustomProperty(summary.key) && !defined.has(summary.key))
    .map((summary): QueryBuilderFieldDefinition => {
      const field = `custom_properties.${summary.key}`;
      const label = customPropertyLabel(summary.key);
      const group = definitions.length > 0 ? UNDEFINED_FIELDS_GROUP : undefined;
      if (isNumericCustomProperty(summary)) {
        return { field, label, group, type: 'number', placeholder: 'Number' };
      }
      return {
        field,
        label,
        group,
        type: 'enum',
        placeholder: 'Select value',
        operators: ['is_any_of', 'is_none_of', 'is_empty', 'is_not_empty'],
        options: summary.values.map((entry) => ({ value: entry.value, label: customPropertyOptionLabel(summary, entry.value) })),
      };
    });
  return [...definitions.map((definition) => customFieldQueryField(definition, DEFINED_FIELDS_GROUP)), ...undefinedFields];
}

export function buildCRMContactQueryFields(
  assignableMembers: AssignableMember[],
  filterOptions?: CRMContactFilterOptions,
  definitions: CustomFieldDefinition[] = [],
): QueryBuilderFieldDefinition[] {
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
      options: LIFECYCLE_STAGE_OPTIONS,
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
    // Synced from product events; contacts without events are empty.
    {
      field: 'last_seen_at',
      label: 'Last seen',
      type: 'date',
    },
    {
      field: 'first_seen_at',
      label: 'First seen',
      type: 'date',
    },
    {
      field: 'sessions_30d',
      label: 'Sessions (30 days)',
      type: 'number',
      placeholder: 'Sessions',
    },
    ...buildContactCustomPropertyFields(filterOptions, definitions),
  ];
}
