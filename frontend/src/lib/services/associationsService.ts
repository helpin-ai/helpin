import { api } from '../api';
import { docsService } from './docsService';
import type { CreateCRMAssociationRequest, CRMAssociation } from '../crmTypes';
import type { CreateDocsLinkRequest } from '../docsTypes';
import type { CreateStoryRelationshipRequest, GroupedAssociations } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const associationsService = {
  listByStory: (workspaceId: string, storyId: string) =>
    api.get<GroupedAssociations>(`/pm/stories/${storyId}/associations${qs(workspaceId)}`),
  listByEpic: (workspaceId: string, epicId: string) =>
    api.get<GroupedAssociations>(`/pm/epics/${epicId}/associations${qs(workspaceId)}`),
  listByTicket: (workspaceId: string, ticketId: string) =>
    api.get<GroupedAssociations>(`/support/tickets/${ticketId}/associations${qs(workspaceId)}`),
  createStoryRelationship: (workspaceId: string, storyId: string, payload: CreateStoryRelationshipRequest) =>
    api.post(`/pm/stories/${storyId}/relationships${qs(workspaceId)}`, payload),
  deleteStoryRelationship: (workspaceId: string, relationshipId: string) =>
    api.del(`/pm/story-relationships/${relationshipId}${qs(workspaceId)}`),
  createAssociation: (payload: CreateCRMAssociationRequest) =>
    api.post<CRMAssociation>(`/pm/associations${qs(payload.workspace_id)}`, payload),
  deleteAssociation: (workspaceId: string, associationId: string) =>
    api.del(`/pm/associations/${associationId}${qs(workspaceId)}`),
  createDocAssociation: (workspaceId: string, documentId: string, payload: CreateDocsLinkRequest) =>
    docsService.createLink(workspaceId, documentId, payload),
  deleteDocAssociation: (workspaceId: string, linkId: string) =>
    docsService.deleteLink(workspaceId, linkId),
};
