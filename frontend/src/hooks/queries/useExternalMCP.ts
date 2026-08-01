import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { externalMCPService } from '@/lib/services/externalMCPService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { CreateExternalMCPServerRequest, UpdateExternalMCPToolsRequest } from '@/lib/externalMCPTypes';

function invalidateExternalMCP(queryClient: ReturnType<typeof useQueryClient>, workspaceId: string) {
  queryClient.invalidateQueries({ queryKey: queryKeys.mcp.externalRoot(workspaceId) });
  queryClient.invalidateQueries({ queryKey: queryKeys.automation.toolCatalog(workspaceId) });
}

export function useExternalMCPProviders(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.mcp.externalProviders(workspaceId),
    queryFn: async () => {
      const result = unwrap(await externalMCPService.listProviders(workspaceId));
      return {
        ...result,
        providers: Array.isArray(result.providers) ? result.providers.map((provider) => ({
          ...provider,
          default_scopes: Array.isArray(provider.default_scopes) ? provider.default_scopes : [],
          optional_scopes: Array.isArray(provider.optional_scopes) ? provider.optional_scopes : [],
        })) : [],
      };
    },
    enabled: Boolean(workspaceId),
    staleTime: 60_000,
    retry: false,
  });
}

export function useExternalMCPServers(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.mcp.externalServers(workspaceId),
    queryFn: async () => {
      const result = unwrap(await externalMCPService.listServers(workspaceId));
      return Array.isArray(result.servers) ? result.servers : [];
    },
    enabled: Boolean(workspaceId) && enabled,
    staleTime: 15_000,
    retry: false,
  });
}

export function useCreateExternalMCPServer(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CreateExternalMCPServerRequest) => unwrap(await externalMCPService.createServer(workspaceId, request)),
    onSuccess: () => invalidateExternalMCP(queryClient, workspaceId),
  });
}

export function useUpdateExternalMCPServer(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ serverId, request }: { serverId: string; request: { name?: string; enabled?: boolean } }) =>
      unwrap(await externalMCPService.updateServer(workspaceId, serverId, request)),
    onSuccess: () => invalidateExternalMCP(queryClient, workspaceId),
  });
}

export function useDeleteExternalMCPServer(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (serverId: string) => unwrap(await externalMCPService.deleteServer(workspaceId, serverId)),
    onSuccess: () => invalidateExternalMCP(queryClient, workspaceId),
  });
}

export function useStartExternalMCPOAuth(workspaceId: string) {
  return useMutation({
    mutationFn: async (serverId: string) => unwrap(await externalMCPService.startOAuth(
      workspaceId,
      serverId,
      window.location.pathname,
    )),
  });
}

export function useRefreshExternalMCPTools(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (serverId: string) => unwrap(await externalMCPService.refreshTools(workspaceId, serverId)),
    onSuccess: () => invalidateExternalMCP(queryClient, workspaceId),
  });
}

export function useUpdateExternalMCPTools(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ serverId, request }: { serverId: string; request: UpdateExternalMCPToolsRequest }) =>
      unwrap(await externalMCPService.updateTools(workspaceId, serverId, request)),
    onSuccess: () => invalidateExternalMCP(queryClient, workspaceId),
  });
}
