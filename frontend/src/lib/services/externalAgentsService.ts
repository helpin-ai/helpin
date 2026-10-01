import { api } from '@/lib/api';
import type {
  AgentCardSummary,
  CreateExternalAgentRequest,
  ExternalAgent,
  PreviewExternalAgentRequest,
  UpdateExternalAgentRequest,
} from '@/lib/externalAgentTypes';

const basePath = (workspaceId: string) => `/workspaces/${encodeURIComponent(workspaceId)}/external-agents`;
const agentPath = (workspaceId: string, externalAgentId: string) =>
  `${basePath(workspaceId)}/${encodeURIComponent(externalAgentId)}`;

export const externalAgentsService = {
  list: (workspaceId: string) =>
    api.get<{ items: ExternalAgent[] | null }>(basePath(workspaceId)),

  preview: (workspaceId: string, request: PreviewExternalAgentRequest) =>
    api.post<{ card: AgentCardSummary }>(`${basePath(workspaceId)}/preview`, request),

  create: (workspaceId: string, request: CreateExternalAgentRequest) =>
    api.post<ExternalAgent>(basePath(workspaceId), request),

  update: (workspaceId: string, externalAgentId: string, request: UpdateExternalAgentRequest) =>
    api.patch<ExternalAgent>(agentPath(workspaceId, externalAgentId), request),

  refreshCard: (workspaceId: string, externalAgentId: string) =>
    api.post<ExternalAgent>(`${agentPath(workspaceId, externalAgentId)}/refresh-card`, {}),

  remove: (workspaceId: string, externalAgentId: string) =>
    api.del<void>(agentPath(workspaceId, externalAgentId)),
};
