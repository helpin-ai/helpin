import { api } from '../api';
import type { CapabilitiesResponse, TestEmailResult } from '../capabilityTypes';

export const capabilityService = {
  workspace: (workspaceId: string) =>
    api.get<CapabilitiesResponse>(`/workspaces/${workspaceId}/capabilities`),
  sendTestEmail: (workspaceId: string) =>
    api.post<TestEmailResult>(`/workspaces/${workspaceId}/email/test`),
};
