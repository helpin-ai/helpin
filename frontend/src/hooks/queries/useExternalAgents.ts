import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { externalAgentsService } from '@/lib/services/externalAgentsService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type {
  AgentCardSummary,
  CreateExternalAgentRequest,
  ExternalAgent,
  ExternalAgentList,
  PreviewExternalAgentRequest,
  UpdateExternalAgentRequest,
} from '@/lib/externalAgentTypes';

function normalizeCard<T extends AgentCardSummary>(card: T): T {
  return {
    ...card,
    skills: Array.isArray(card.skills) ? card.skills : [],
    capabilities: card.capabilities ?? {},
  };
}

function normalizeExternalAgent(agent: ExternalAgent): ExternalAgent {
  return {
    ...normalizeCard(agent),
    allowed_team_ids: Array.isArray(agent.allowed_team_ids) ? agent.allowed_team_ids : [],
    last_error: agent.last_error ?? '',
    token_hint: agent.token_hint ?? '',
  };
}

// Creating, updating, and deleting an external agent also changes its linked
// Helpin agent, so agent lists and pickers must refresh with it.
function invalidateExternalAgents(queryClient: ReturnType<typeof useQueryClient>, workspaceId: string) {
  queryClient.invalidateQueries({ queryKey: queryKeys.externalAgents.root(workspaceId) });
  queryClient.invalidateQueries({ queryKey: queryKeys.automation.agentsRoot(workspaceId) });
}

export function useExternalAgents(workspaceId: string) {
  return useQuery<ExternalAgentList>({
    queryKey: queryKeys.externalAgents.list(workspaceId),
    queryFn: async () => {
      const response = await externalAgentsService.list(workspaceId);
      if (response.status === 503) return { configured: false, items: [] };
      const result = unwrap(response);
      return {
        configured: true,
        items: Array.isArray(result?.items) ? result.items.map(normalizeExternalAgent) : [],
      };
    },
    enabled: Boolean(workspaceId),
    staleTime: 15_000,
    retry: false,
  });
}

export function usePreviewExternalAgent(workspaceId: string) {
  return useMutation({
    mutationFn: async (request: PreviewExternalAgentRequest) =>
      normalizeCard(unwrap(await externalAgentsService.preview(workspaceId, request)).card),
  });
}

export function useCreateExternalAgent(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CreateExternalAgentRequest) =>
      normalizeExternalAgent(unwrap(await externalAgentsService.create(workspaceId, request))),
    onSuccess: () => invalidateExternalAgents(queryClient, workspaceId),
  });
}

export function useUpdateExternalAgent(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ externalAgentId, request }: { externalAgentId: string; request: UpdateExternalAgentRequest }) =>
      normalizeExternalAgent(unwrap(await externalAgentsService.update(workspaceId, externalAgentId, request))),
    onSuccess: () => invalidateExternalAgents(queryClient, workspaceId),
  });
}

export function useRefreshExternalAgentCard(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (externalAgentId: string) =>
      normalizeExternalAgent(unwrap(await externalAgentsService.refreshCard(workspaceId, externalAgentId))),
    onSuccess: () => invalidateExternalAgents(queryClient, workspaceId),
  });
}

export function useDeleteExternalAgent(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (externalAgentId: string) => {
      unwrap(await externalAgentsService.remove(workspaceId, externalAgentId));
    },
    onSuccess: () => invalidateExternalAgents(queryClient, workspaceId),
  });
}
