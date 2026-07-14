import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { mcpService } from '@/lib/services/mcpService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type {
  CreateMCPServicePrincipalRequest,
  MCPAuthorizationQuery,
  MCPDashboard,
  UpdateMCPPolicyRequest,
} from '@/lib/mcpTypes';

function sanitizeDashboard(value: MCPDashboard): MCPDashboard {
  return {
    ...value,
    connections: Array.isArray(value.connections) ? value.connections : [],
    service_principals: Array.isArray(value.service_principals) ? value.service_principals : [],
    available_toolsets: Array.isArray(value.available_toolsets) ? value.available_toolsets : [],
    available_scopes: Array.isArray(value.available_scopes) ? value.available_scopes : [],
    policy: {
      ...value.policy,
      allowed_toolsets: Array.isArray(value.policy.allowed_toolsets) ? value.policy.allowed_toolsets : [],
      allowed_scopes: Array.isArray(value.policy.allowed_scopes) ? value.policy.allowed_scopes : [],
    },
  };
}

export function useMCPDashboard(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.mcp.dashboard(workspaceId),
    queryFn: async () => sanitizeDashboard(unwrap(await mcpService.getDashboard(workspaceId))),
    enabled: Boolean(workspaceId),
    staleTime: 30_000,
  });
}

export function useUpdateMCPPolicy(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: UpdateMCPPolicyRequest) => unwrap(await mcpService.updatePolicy(workspaceId, request)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.mcp.root(workspaceId) }),
  });
}

export function useRevokeMCPConnection(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (connectionId: string) => unwrap(await mcpService.revokeConnection(workspaceId, connectionId)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.mcp.root(workspaceId) }),
  });
}

export function useRevokeMCPWorkspaceAccess(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => unwrap(await mcpService.revokeWorkspaceAccess(workspaceId)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.mcp.root(workspaceId) }),
  });
}

export function useMCPActivity(workspaceId: string, enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.mcp.activity(workspaceId),
    queryFn: async () => unwrap(await mcpService.listActivity(workspaceId)),
    enabled: Boolean(workspaceId) && enabled,
    staleTime: 15_000,
  });
}

export function useCreateMCPServicePrincipal(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: CreateMCPServicePrincipalRequest) => unwrap(await mcpService.createServicePrincipal(workspaceId, request)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.mcp.root(workspaceId) }),
  });
}

export function useRevokeMCPServicePrincipal(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (principalId: string) => unwrap(await mcpService.revokeServicePrincipal(workspaceId, principalId)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.mcp.root(workspaceId) }),
  });
}

export function useMCPServiceTokens(workspaceId: string, principalId: string, enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.mcp.serviceTokens(workspaceId, principalId),
    queryFn: async () => unwrap(await mcpService.listServiceTokens(workspaceId, principalId)),
    enabled: Boolean(workspaceId && principalId) && enabled,
  });
}

export function useRotateMCPServiceToken(workspaceId: string, principalId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => unwrap(await mcpService.rotateServiceToken(workspaceId, principalId)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.mcp.serviceTokens(workspaceId, principalId) }),
  });
}

export function useRevokeMCPServiceToken(workspaceId: string, principalId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (tokenId: string) => unwrap(await mcpService.revokeServiceToken(workspaceId, principalId, tokenId)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.mcp.serviceTokens(workspaceId, principalId) }),
  });
}

export function useMCPAuthorizationRequest(query: MCPAuthorizationQuery, enabled = true) {
  return useQuery({
    queryKey: queryKeys.mcp.authorization(query),
    queryFn: async () => unwrap(await mcpService.getAuthorizationRequest(query)),
    enabled: enabled && Boolean(query.client_id && query.redirect_uri && query.state && query.code_challenge),
    retry: false,
  });
}

export function useAuthorizeMCP() {
  return useMutation({
    mutationFn: async (decision: Parameters<typeof mcpService.authorize>[0]) => unwrap(await mcpService.authorize(decision)),
  });
}
