import { api } from '../api';
import type { CustomFieldDefinition, CustomFieldDefinitionInput, CustomFieldObjectType } from '../customFields';

const base = (workspaceId: string, objectType: CustomFieldObjectType, path = '', params = '') => {
  const workspace = `workspace_id=${encodeURIComponent(workspaceId)}`;
  if (objectType === 'conversation') return `/support/conversation-fields${path}?${workspace}${params}`;
  return `/crm/field-definitions${path}?${workspace}&object_type=${objectType}${params}`;
};

/** Field definitions for conversations and CRM records, and CRM record values. */
export const customFieldsService = {
  list: (workspaceId: string, objectType: CustomFieldObjectType, options: { archived?: boolean; usage?: boolean } = {}) =>
    api.get<CustomFieldDefinition[]>(base(workspaceId, objectType, '', `${options.archived ? '&archived=true' : ''}${options.usage ? '&usage=true' : ''}`)),
  create: (workspaceId: string, objectType: CustomFieldObjectType, input: CustomFieldDefinitionInput) =>
    api.post<CustomFieldDefinition>(base(workspaceId, objectType), input),
  update: (workspaceId: string, objectType: CustomFieldObjectType, id: string, input: CustomFieldDefinitionInput) =>
    api.put<CustomFieldDefinition>(base(workspaceId, objectType, `/${id}`), input),
  setArchived: (workspaceId: string, objectType: CustomFieldObjectType, id: string, archived: boolean) =>
    api.post<void>(base(workspaceId, objectType, `/${id}/${archived ? 'archive' : 'restore'}`), {}),
  reorder: (workspaceId: string, objectType: CustomFieldObjectType, ids: string[]) =>
    api.post<CustomFieldDefinition[]>(base(workspaceId, objectType, '/reorder'), { ids }),
  setCRMValues: (workspaceId: string, objectType: 'crm_contact' | 'crm_company', recordId: string, values: Record<string, unknown>) =>
    api.patch<void>(`/crm/${objectType === 'crm_contact' ? 'contacts' : 'companies'}/${recordId}/custom-properties?workspace_id=${encodeURIComponent(workspaceId)}`, { values }),
};
