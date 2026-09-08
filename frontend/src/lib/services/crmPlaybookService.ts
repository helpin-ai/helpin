import { api } from '@/lib/api';
import type { CRMPlaybookAgentUsage, CRMPlaybookAutomationActivityPage } from '@/lib/crmPlaybookTypes';
import type {
  ApplyCRMPlaybookRequest, CreateCRMPlaybookRequest, CRMPlaybookCommandRequest, CRMPlaybookAutomationPreview,
  CRMPlaybookCommandResult, CRMPlaybookDefinition, CRMPlaybookHistory, CRMPlaybookItem,
  CRMPlaybookList, CRMPlaybookMilestoneRequest, CRMPlaybookPreview, CRMPlaybookVersions, CRMPlaybook,
  CRMPlaybookConnectionSelection, CRMPlaybookConnectionReview, PublishCRMPlaybookConnectionRequest,
  CRMPlaybookConnectionResult, CRMPlaybookConnection, CRMPlaybookConnections,
  CRMPlaybookAutomationOverview, CRMPlaybookAutomationBinding, CRMPlaybookAutomationCommand,
  CRMPlaybookAutomationAdoption, CRMPlaybookAutomationReceipt, CRMPlaybookActionIntent, CRMPlaybookActionInspection,
} from '@/lib/crmPlaybookTypes';
import type { CRMSuggestion } from '@/lib/crmTypes';
import type { CRMSituationCommandRequest, CRMSituationCommandResult, CRMSituationItem, CRMSituationList } from '@/lib/crmSituationTypes';

export interface PlaybookListFilters { q?: string; state?: 'all' | 'draft' | 'accepting' | 'stopped'; page?: number }
export interface PlaybookParticipantFilters { q?: string; state?: string; scope?: string; category?: string; page?: number }

function query(workspaceId: string, params: Record<string, string | number | undefined> = {}) {
  const search = new URLSearchParams({ workspace_id: workspaceId });
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value));
  }
  return `?${search}`;
}

// Keep HTTP status available for conflict recovery without changing the shared API client.
export class PlaybookRequestError extends Error {
  status?: number;
  constructor(message: string, status?: number) { super(message); this.status = status; }
}
export function readPlaybookResponse<T>(response: { data: T | null; error: string | null; status?: number }): T {
  if (response.error || response.data == null) throw new PlaybookRequestError(response.error || 'No response received. Try again.', response.status);
  return response.data;
}

export const crmPlaybookService = {
  templates: (ws: string) => api.get<CRMPlaybookDefinition[]>(`/crm/playbooks/templates${query(ws)}`),
  list: (ws: string, filters: PlaybookListFilters) => api.get<CRMPlaybookList>(`/crm/playbooks${query(ws, { ...filters, page_size: 25 })}`),
  get: (ws: string, id: string) => api.get<CRMPlaybookItem>(`/crm/playbooks/${id}${query(ws)}`),
  create: (ws: string, body: CreateCRMPlaybookRequest) => api.post<CRMPlaybook>(`/crm/playbooks${query(ws)}`, body),
  command: (ws: string, id: string, body: CRMPlaybookCommandRequest) => api.post<CRMPlaybookCommandResult>(`/crm/playbooks/${id}/commands${query(ws)}`, body),
  history: (ws: string, id: string, before?: number) => api.get<CRMPlaybookHistory>(`/crm/playbooks/${id}/history${query(ws, { before })}`),
  versions: (ws: string, id: string, before?: number) => api.get<CRMPlaybookVersions>(`/crm/playbooks/${id}/versions${query(ws, { before, page_size: 100 })}`),
  preview: (ws: string, id: string, revision: number, versionId?: string, page = 1) => api.get<CRMPlaybookPreview>(`/crm/playbooks/${id}/preview${query(ws, { revision, version_id: versionId, page, page_size: 25 })}`),
  automationPreview: (ws: string, id: string, revision: number, versionId?: string) => api.get<CRMPlaybookAutomationPreview>(`/crm/playbooks/${id}/automation/preview${query(ws, { revision, version_id: versionId })}`),
  automation: (ws: string, id: string) => api.get<CRMPlaybookAutomationOverview>(`/crm/playbooks/${id}/automation${query(ws)}`),
  agentUsage: (ws: string, id: string) => api.get<CRMPlaybookAgentUsage[]>(`/crm/playbooks/agent-usage/${id}${query(ws)}`),
  automationActivity: (ws: string, id: string, page: number) => api.get<CRMPlaybookAutomationActivityPage>(`/crm/playbooks/${id}/automation/activity${query(ws, { page })}`),
  prepareSetup: (ws: string, id: string, body: { expected_revision: number; playbook_version_id: string }) => api.post<CRMPlaybookConnectionReview>(`/crm/playbooks/${id}/automation/setup${query(ws)}`, body),
  configureAutomation: (ws: string, id: string, body: CRMPlaybookAutomationCommand) => api.post<CRMPlaybookAutomationReceipt>(`/crm/playbooks/${id}/automation/settings${query(ws)}`, body),
  signalAutomation: (ws: string, id: string) => api.get<CRMPlaybookAutomationBinding | null>(`/crm/situations/${id}/automation${query(ws)}`),
  adoptAutomation: (ws: string, id: string, body: CRMPlaybookAutomationAdoption) => api.post<CRMPlaybookAutomationReceipt>(`/crm/situations/${id}/automation${query(ws)}`, body),
  actionIntent: (ws: string, id: string) => api.get<CRMPlaybookActionIntent>(`/crm/playbook-actions/${id}${query(ws)}`),
  reconcileAction: (ws: string, id: string) => api.post<CRMSuggestion>(`/crm/playbook-actions/${id}/reconcile${query(ws)}`, {}),
  inspectAction: (ws: string, id: string, body: CRMPlaybookActionInspection) => api.post<CRMSuggestion>(`/crm/playbook-actions/${id}/inspect${query(ws)}`, body),
  reviewConnection: (ws: string, id: string, body: CRMPlaybookConnectionSelection) => api.post<CRMPlaybookConnectionReview>(`/crm/playbooks/${id}/automation/connection/preview${query(ws)}`, body),
  publishConnection: (ws: string, id: string, body: PublishCRMPlaybookConnectionRequest) => api.post<CRMPlaybookConnectionResult>(`/crm/playbooks/${id}/automation/connections${query(ws)}`, body),
  connection: (ws: string, id: string, connectionId: string) => api.get<CRMPlaybookConnection>(`/crm/playbooks/${id}/automation/connections/${connectionId}${query(ws)}`),
  connections: (ws: string, id: string, before?: number) => api.get<CRMPlaybookConnections>(`/crm/playbooks/${id}/automation/connections${query(ws, { before })}`),
  participants: (ws: string, id: string, filters: PlaybookParticipantFilters) => api.get<CRMSituationList>(`/crm/playbooks/${id}/participants${query(ws, { scope: 'all', state: 'all', ...filters, page_size: 25 })}`),
  apply: (ws: string, id: string, body: ApplyCRMPlaybookRequest) => api.post<CRMSituationCommandResult>(`/crm/playbooks/${id}/apply${query(ws)}`, body),
  milestone: (ws: string, id: string, signalId: string, body: CRMPlaybookMilestoneRequest) => api.post<CRMSituationCommandResult>(`/crm/playbooks/${id}/participants/${signalId}/milestones${query(ws)}`, body),
  signal: (ws: string, id: string) => api.get<CRMSituationItem>(`/crm/situations/${id}${query(ws)}`),
  signalCommand: (ws: string, id: string, body: CRMSituationCommandRequest) => api.post<CRMSituationCommandResult>(`/crm/situations/${id}/commands${query(ws)}`, body),
};
