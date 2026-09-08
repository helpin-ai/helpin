import type { AgentTargetType } from './pmTypes';

export type CRMRecordTargetType = Extract<AgentTargetType, 'crm_deal' | 'crm_contact' | 'crm_company'>;

// Display names only. These options never grant an agent access to a record or tool.
export const CRM_RECORD_TARGETS = {
  crm_deal: { singular: 'Deal', plural: 'Deals', collection: 'deals' },
  crm_contact: { singular: 'Contact', plural: 'Contacts', collection: 'contacts' },
  crm_company: { singular: 'Company', plural: 'Companies', collection: 'companies' },
} as const satisfies Record<CRMRecordTargetType, { singular: string; plural: string; collection: string }>;

export const CRM_AGENT_TARGET_OPTIONS = (Object.keys(CRM_RECORD_TARGETS) as CRMRecordTargetType[]).map((value) => ({
  value,
  label: `CRM ${CRM_RECORD_TARGETS[value].plural.toLowerCase()}`,
  description: `Run on CRM ${CRM_RECORD_TARGETS[value].singular.toLowerCase()} records.`,
}));

export function isCRMRecordTarget(value: string): value is CRMRecordTargetType {
  return Object.hasOwn(CRM_RECORD_TARGETS, value);
}
