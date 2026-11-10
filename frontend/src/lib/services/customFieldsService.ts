import { api } from '../api';
import type { CustomFieldDefinition, CustomFieldDefinitionInput, CustomFieldObjectType, SupportRequestType, SupportRequestTypeInput } from '../customFields';
import type { SupportConversation } from '../pmTypes';

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
  listRequestTypes: (workspaceId: string) =>
    api.get<SupportRequestType[]>(`/support/request-types?workspace_id=${encodeURIComponent(workspaceId)}`),
  createRequestType: (workspaceId: string, input: SupportRequestTypeInput) =>
    api.post<SupportRequestType>(`/support/request-types?workspace_id=${encodeURIComponent(workspaceId)}`, input),
  updateRequestType: (workspaceId: string, id: string, input: SupportRequestTypeInput) =>
    api.put<SupportRequestType>(`/support/request-types/${id}?workspace_id=${encodeURIComponent(workspaceId)}`, input),
  deleteRequestType: (workspaceId: string, id: string) =>
    api.del<void>(`/support/request-types/${id}?workspace_id=${encodeURIComponent(workspaceId)}`),
  reorderRequestTypes: (workspaceId: string, ids: string[]) =>
    api.post<SupportRequestType[]>(`/support/request-types/reorder?workspace_id=${encodeURIComponent(workspaceId)}`, { ids }),
  setConversationRequestType: (workspaceId: string, conversationId: string, requestTypeId: string | null) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/request-type?workspace_id=${encodeURIComponent(workspaceId)}`, { request_type_id: requestTypeId }),
  setConversationFields: (workspaceId: string, conversationId: string, values: Record<string, unknown>) =>
    api.patch<SupportConversation>(`/support/inbox/conversations/${conversationId}/fields?workspace_id=${encodeURIComponent(workspaceId)}`, { values }),
};
