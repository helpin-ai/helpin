import type { QueryBuilderFieldDefinition } from './queryBuilder';
import type { AssignableMember } from './types';
import type { CRMPipeline } from './crmTypes';

export function buildCRMPlaybookQueryFields(members: AssignableMember[], pipelines: CRMPipeline[]): QueryBuilderFieldDefinition[] {
  return [
    { field: 'company_domain', label: 'Company domain', type: 'text', placeholder: 'example.com' },
    { field: 'company_id', label: 'Company', type: 'enum' },
    { field: 'contact_id', label: 'Contact', type: 'enum' },
    { field: 'deal_id', label: 'Deal', type: 'enum' },
    { field: 'owner_member_id', label: 'Signal owner', type: 'member', options: members.filter((member) => member.status === 'active').map((member) => ({ value: member.id, label: member.display_name || member.email })) },
    { field: 'pipeline_id', label: 'Pipeline', type: 'enum', options: pipelines.map((pipeline) => ({ value: pipeline.id, label: pipeline.name })) },
    { field: 'stage_id', label: 'Deal stage', type: 'enum', options: pipelines.flatMap((pipeline) => (pipeline.stages ?? []).map((stage) => ({ value: stage.id, label: `${pipeline.name} · ${stage.name}` }))) },
    { field: 'created_at', label: 'Signal created', type: 'date' },
  ];
}
