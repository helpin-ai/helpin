import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { associationsService } from '@/lib/services/associationsService';
import type { CreateCRMAssociationRequest } from '@/lib/crmTypes';
import type { CreateDocsLinkRequest } from '@/lib/docsTypes';
import type { CreateTaskRelationshipRequest } from '@/lib/pmTypes';

export function useTaskAssociations(wsId: string, taskId: string) {
  return useQuery({
    queryKey: queryKeys.pm.taskAssociations(wsId, taskId),
    queryFn: async () => unwrap(await associationsService.listByTask(wsId, taskId)),
    enabled: !!wsId && !!taskId,
  });
}

export function useEpicAssociations(wsId: string, epicId: string) {
  return useQuery({
    queryKey: queryKeys.pm.epicAssociations(wsId, epicId),
    queryFn: async () => unwrap(await associationsService.listByEpic(wsId, epicId)),
    enabled: !!wsId && !!epicId,
  });
}

export function useConversationAssociations(wsId: string, conversationId: string) {
  return useQuery({
    queryKey: queryKeys.support.conversationAssociations(wsId, conversationId),
    queryFn: async () => unwrap(await associationsService.listByConversation(wsId, conversationId)),
    enabled: !!wsId && !!conversationId,
  });
}

export function useCreateTaskRelationship(wsId: string, taskId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: CreateTaskRelationshipRequest) =>
      unwrap(await associationsService.createTaskRelationship(wsId, taskId, payload)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.taskAssociations(wsId, taskId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.tasks(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.board(wsId) });
    },
  });
}

export function useDeleteTaskRelationship(wsId: string, taskId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (relationshipId: string) =>
      unwrap(await associationsService.deleteTaskRelationship(wsId, relationshipId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.taskAssociations(wsId, taskId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.tasks(wsId) });
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
      if (payload.from_object_type === 'support_conversation' || payload.to_object_type === 'support_conversation') {
        const conversationId = payload.from_object_type === 'support_conversation' ? payload.from_object_id : payload.to_object_id;
        qc.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(wsId, conversationId) });
        qc.invalidateQueries({ queryKey: queryKeys.support.conversation(wsId, conversationId) });
      }
      if (payload.from_object_type === 'task' || payload.to_object_type === 'task') {
        const taskId = payload.from_object_type === 'task' ? payload.from_object_id : payload.to_object_id;
        qc.invalidateQueries({ queryKey: queryKeys.pm.taskAssociations(wsId, taskId) });
        qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) });
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

export function useCreateDocAssociation(wsId: string, objectType: 'epic' | 'task' | 'support_conversation', objectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ documentId, payload }: { documentId: string; payload: CreateDocsLinkRequest }) =>
      unwrap(await associationsService.createDocAssociation(wsId, documentId, payload)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.linkedDocs(wsId, objectType, objectId) });
      if (objectType === 'task') {
        qc.invalidateQueries({ queryKey: queryKeys.pm.taskAssociations(wsId, objectId) });
      } else if (objectType === 'epic') {
        qc.invalidateQueries({ queryKey: queryKeys.pm.epicAssociations(wsId, objectId) });
      } else {
        qc.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(wsId, objectId) });
      }
    },
  });
}

export function useDeleteDocAssociation(wsId: string, objectType: 'epic' | 'task' | 'support_conversation', objectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (linkId: string) =>
      unwrap(await associationsService.deleteDocAssociation(wsId, linkId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.linkedDocs(wsId, objectType, objectId) });
      if (objectType === 'task') {
        qc.invalidateQueries({ queryKey: queryKeys.pm.taskAssociations(wsId, objectId) });
      } else if (objectType === 'epic') {
        qc.invalidateQueries({ queryKey: queryKeys.pm.epicAssociations(wsId, objectId) });
      } else {
        qc.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(wsId, objectId) });
      }
    },
  });
}
