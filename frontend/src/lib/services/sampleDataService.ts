import { api } from '../api';
import type { SampleDataStatus } from '../sampleDataTypes';

export const sampleDataService = {
  status: (workspaceId: string) => api.get<SampleDataStatus>(`/workspaces/${workspaceId}/sample-data`),
  load: (workspaceId: string) => api.post<SampleDataStatus>(`/workspaces/${workspaceId}/sample-data`),
  remove: (workspaceId: string) => api.del<SampleDataStatus>(`/workspaces/${workspaceId}/sample-data`),
};
