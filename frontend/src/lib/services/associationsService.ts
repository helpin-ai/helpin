import { api } from '../api';
import { docsService } from './docsService';
import type { CreateCRMAssociationRequest, CRMAssociation } from '../crmTypes';
import type { CreateDocsLinkRequest } from '../docsTypes';
import type {
  CreateTaskRelationshipRequest,
  GroupedAssociations,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const associationsService = {
  listByTask: (workspaceId: string, taskId: string) =>
    api.get<GroupedAssociations>(`/pm/tasks/${taskId}/associations${qs(workspaceId)}`),
  listByEpic: (workspaceId: string, epicId: string) =>
    api.get<GroupedAssociations>(`/pm/epics/${epicId}/associations${qs(workspaceId)}`),
  listByConversation: (workspaceId: string, conversationId: string) =>
    api.get<GroupedAssociations>(`/support/conversations/${conversationId}/associations${qs(workspaceId)}`),
  createTaskRelationship: (workspaceId: string, taskId: string, payload: CreateTaskRelationshipRequest) =>
    api.post(`/pm/tasks/${taskId}/relationships${qs(workspaceId)}`, payload),
  deleteTaskRelationship: (workspaceId: string, relationshipId: string) =>
    api.del(`/pm/task-relationships/${relationshipId}${qs(workspaceId)}`),
  createAssociation: (payload: CreateCRMAssociationRequest) =>
    api.post<CRMAssociation>(`/pm/associations${qs(payload.workspace_id)}`, payload),
  deleteAssociation: (workspaceId: string, associationId: string) =>
    api.del(`/pm/associations/${associationId}${qs(workspaceId)}`),
  createDocAssociation: (workspaceId: string, documentId: string, payload: CreateDocsLinkRequest) =>
    docsService.createLink(workspaceId, documentId, payload),
  deleteDocAssociation: (workspaceId: string, linkId: string) =>
    docsService.deleteLink(workspaceId, linkId),
};
