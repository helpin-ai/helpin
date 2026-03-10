import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { associationsService } from '@/lib/services/associationsService';
import type { CreateCRMAssociationRequest } from '@/lib/crmTypes';
import type { CreateDocsLinkRequest } from '@/lib/docsTypes';
import type { CreateStoryRelationshipRequest } from '@/lib/pmTypes';

export function useStoryAssociations(wsId: string, storyId: string) {
  return useQuery({
    queryKey: queryKeys.pm.storyAssociations(wsId, storyId),
    queryFn: async () => unwrap(await associationsService.listByStory(wsId, storyId)),
    enabled: !!wsId && !!storyId,
  });
}

export function useEpicAssociations(wsId: string, epicId: string) {
  return useQuery({
    queryKey: queryKeys.pm.epicAssociations(wsId, epicId),
    queryFn: async () => unwrap(await associationsService.listByEpic(wsId, epicId)),
    enabled: !!wsId && !!epicId,
  });
}

export function useTicketAssociations(wsId: string, ticketId: string) {
  return useQuery({
    queryKey: queryKeys.support.ticketAssociations(wsId, ticketId),
    queryFn: async () => unwrap(await associationsService.listByTicket(wsId, ticketId)),
    enabled: !!wsId && !!ticketId,
  });
}

export function useCreateStoryRelationship(wsId: string, storyId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: CreateStoryRelationshipRequest) =>
      unwrap(await associationsService.createStoryRelationship(wsId, storyId, payload)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.storyAssociations(wsId, storyId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.stories(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.board(wsId) });
    },
  });
}

export function useDeleteStoryRelationship(wsId: string, storyId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (relationshipId: string) =>
      unwrap(await associationsService.deleteStoryRelationship(wsId, relationshipId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.storyAssociations(wsId, storyId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.stories(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.board(wsId) });
    },
  });
}

export function useCreatePMAssociation(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: CreateCRMAssociationRequest) =>
      unwrap(await associationsService.createAssociation(payload)),
    onSuccess: (_, payload) => {
      qc.invalidateQueries({ queryKey: ['pm', wsId] });
      qc.invalidateQueries({ queryKey: ['support', wsId] });
      if (payload.from_object_type === 'support_ticket' || payload.to_object_type === 'support_ticket') {
        const ticketId = payload.from_object_type === 'support_ticket' ? payload.from_object_id : payload.to_object_id;
        qc.invalidateQueries({ queryKey: queryKeys.support.ticketAssociations(wsId, ticketId) });
        qc.invalidateQueries({ queryKey: queryKeys.support.ticket(wsId, ticketId) });
      }
      if (payload.from_object_type === 'story' || payload.to_object_type === 'story') {
        const storyId = payload.from_object_type === 'story' ? payload.from_object_id : payload.to_object_id;
        qc.invalidateQueries({ queryKey: queryKeys.pm.storyAssociations(wsId, storyId) });
        qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) });
      }
      if (payload.from_object_type === 'epic' || payload.to_object_type === 'epic') {
        const epicId = payload.from_object_type === 'epic' ? payload.from_object_id : payload.to_object_id;
        qc.invalidateQueries({ queryKey: queryKeys.pm.epicAssociations(wsId, epicId) });
        qc.invalidateQueries({ queryKey: queryKeys.pm.epic(wsId, epicId) });
      }
    },
  });
}

export function useDeletePMAssociation(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (associationId: string) =>
      unwrap(await associationsService.deleteAssociation(wsId, associationId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId] });
      qc.invalidateQueries({ queryKey: ['support', wsId] });
    },
  });
}

export function useCreateDocAssociation(wsId: string, objectType: 'epic' | 'story' | 'support_ticket', objectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ documentId, payload }: { documentId: string; payload: CreateDocsLinkRequest }) =>
      unwrap(await associationsService.createDocAssociation(wsId, documentId, payload)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.linkedDocs(wsId, objectType, objectId) });
      if (objectType === 'story') {
        qc.invalidateQueries({ queryKey: queryKeys.pm.storyAssociations(wsId, objectId) });
      } else if (objectType === 'epic') {
        qc.invalidateQueries({ queryKey: queryKeys.pm.epicAssociations(wsId, objectId) });
      } else {
        qc.invalidateQueries({ queryKey: queryKeys.support.ticketAssociations(wsId, objectId) });
      }
    },
  });
}

export function useDeleteDocAssociation(wsId: string, objectType: 'epic' | 'story' | 'support_ticket', objectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (linkId: string) =>
      unwrap(await associationsService.deleteDocAssociation(wsId, linkId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.linkedDocs(wsId, objectType, objectId) });
      if (objectType === 'story') {
        qc.invalidateQueries({ queryKey: queryKeys.pm.storyAssociations(wsId, objectId) });
      } else if (objectType === 'epic') {
        qc.invalidateQueries({ queryKey: queryKeys.pm.epicAssociations(wsId, objectId) });
      } else {
        qc.invalidateQueries({ queryKey: queryKeys.support.ticketAssociations(wsId, objectId) });
      }
    },
  });
}
