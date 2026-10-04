import { api } from '@/lib/api';
import type {
  CreateExternalMCPServerRequest,
  ExternalMCPProvider,
  ExternalMCPServer,
  UpdateExternalMCPToolsRequest,
} from '@/lib/externalMCPTypes';

const workspaceQuery = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const externalMCPService = {
  listProviders: (workspaceId: string) =>
    api.get<{ providers: ExternalMCPProvider[]; enabled: boolean }>(`/external-mcp/providers${workspaceQuery(workspaceId)}`),

  listServers: (workspaceId: string) =>
    api.get<{ servers: ExternalMCPServer[] }>(`/external-mcp/servers${workspaceQuery(workspaceId)}`),

  createServer: (workspaceId: string, request: CreateExternalMCPServerRequest) =>
    api.post<ExternalMCPServer>(`/external-mcp/servers${workspaceQuery(workspaceId)}`, request),

  updateServer: (workspaceId: string, serverId: string, request: { name?: string; enabled?: boolean }) =>
    api.put<ExternalMCPServer>(`/external-mcp/servers/${serverId}${workspaceQuery(workspaceId)}`, request),

  deleteServer: (workspaceId: string, serverId: string) =>
    api.del<void>(`/external-mcp/servers/${serverId}${workspaceQuery(workspaceId)}`),

  startOAuth: (workspaceId: string, serverId: string, returnPath: string) =>
    api.post<{ authorization_url: string }>(`/external-mcp/servers/${serverId}/oauth/start${workspaceQuery(workspaceId)}`, {
      return_path: returnPath,
    }),

  completeOAuth: (response: { state: string; code: string; error: string }) =>
    api.post<{ redirect_url: string }>('/external-mcp/oauth/callback', response),

  refreshTools: (workspaceId: string, serverId: string) =>
    api.post<ExternalMCPServer>(`/external-mcp/servers/${serverId}/tools/refresh${workspaceQuery(workspaceId)}`, {}),

  updateTools: (workspaceId: string, serverId: string, request: UpdateExternalMCPToolsRequest) =>
    api.put<ExternalMCPServer>(`/external-mcp/servers/${serverId}/tools${workspaceQuery(workspaceId)}`, request),
};
