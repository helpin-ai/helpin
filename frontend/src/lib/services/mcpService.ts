import { api } from '@/lib/api';
import type {
  CreateMCPServicePrincipalRequest,
  MCPAuthorizeDecision,
  MCPAuthorizeResult,
  MCPAuthorizationQuery,
  MCPAuthorizationRequest,
  MCPAuditEvent,
  MCPDashboard,
  MCPServicePrincipal,
  MCPServiceToken,
  MCPServiceTokenSecret,
  MCPWorkspacePolicy,
  UpdateMCPPolicyRequest,
} from '@/lib/mcpTypes';

const workspaceQuery = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

const authorizationQuery = (query: MCPAuthorizationQuery) => {
  const values = new URLSearchParams(query);
  return `?${values.toString()}`;
};

export const mcpService = {
  getDashboard: (workspaceId: string) =>
    api.get<MCPDashboard>(`/mcp/${workspaceQuery(workspaceId)}`),

  updatePolicy: (workspaceId: string, request: UpdateMCPPolicyRequest) =>
    api.put<MCPWorkspacePolicy>(`/mcp/policy${workspaceQuery(workspaceId)}`, request),

  revokeConnection: (workspaceId: string, connectionId: string) =>
    api.del<void>(`/mcp/connections/${connectionId}${workspaceQuery(workspaceId)}`),

  revokeWorkspaceAccess: (workspaceId: string) =>
    api.del<{ revoked: number }>(`/mcp/connections${workspaceQuery(workspaceId)}`),

  listActivity: (workspaceId: string, limit = 100) =>
    api.get<MCPAuditEvent[]>(`/mcp/activity${workspaceQuery(workspaceId)}&limit=${limit}`),

  createServicePrincipal: (workspaceId: string, request: CreateMCPServicePrincipalRequest) =>
    api.post<{ principal: MCPServicePrincipal; secret: MCPServiceTokenSecret }>(
      `/mcp/service-principals${workspaceQuery(workspaceId)}`,
      request,
    ),

  revokeServicePrincipal: (workspaceId: string, principalId: string) =>
    api.del<void>(`/mcp/service-principals/${principalId}${workspaceQuery(workspaceId)}`),

  listServiceTokens: (workspaceId: string, principalId: string) =>
    api.get<MCPServiceToken[]>(`/mcp/service-principals/${principalId}/tokens${workspaceQuery(workspaceId)}`),

  rotateServiceToken: (workspaceId: string, principalId: string) =>
    api.post<MCPServiceTokenSecret>(`/mcp/service-principals/${principalId}/tokens${workspaceQuery(workspaceId)}`, {}),

  revokeServiceToken: (workspaceId: string, principalId: string, tokenId: string) =>
    api.del<void>(`/mcp/service-principals/${principalId}/tokens/${tokenId}${workspaceQuery(workspaceId)}`),

  getAuthorizationRequest: (query: MCPAuthorizationQuery) =>
    api.get<MCPAuthorizationRequest>(`/mcp/oauth/request${authorizationQuery(query)}`),

  authorize: (decision: MCPAuthorizeDecision) =>
    api.post<MCPAuthorizeResult>('/mcp/oauth/authorize', decision),
};
