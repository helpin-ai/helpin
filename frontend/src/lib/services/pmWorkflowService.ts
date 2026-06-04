import { api } from '../api';
import type {
  CreateWorkflowRequest,
  CreateWorkflowStateRequest,
  EpicWorkflowState,
  UpdateWorkflowRequest,
  UpdateWorkflowStateRequest,
  WorkflowState,
  WorkflowWithStates,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmWorkflowService = {
  list: (workspaceId: string) => api.get<WorkflowWithStates[]>(`/pm/workflows${qs(workspaceId)}`),
  listEpicStates: (workspaceId: string) => api.get<EpicWorkflowState[]>(`/pm/workflows/epic-states${qs(workspaceId)}`),
  create: (payload: CreateWorkflowRequest) => api.post<WorkflowWithStates>(`/pm/workflows${qs(payload.workspace_id)}`, payload),
  get: (workspaceId: string, id: string) => api.get<WorkflowWithStates>(`/pm/workflows/${id}${qs(workspaceId)}`),
  update: (workspaceId: string, id: string, payload: UpdateWorkflowRequest) => api.put<WorkflowWithStates>(`/pm/workflows/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/pm/workflows/${id}${qs(workspaceId)}`),
  createState: (workspaceId: string, workflowId: string, payload: CreateWorkflowStateRequest) =>
    api.post(`/pm/workflows/${workflowId}/states${qs(workspaceId)}`, payload),
  updateState: (workspaceId: string, workflowId: string, stateId: string, payload: UpdateWorkflowStateRequest) =>
    api.put<WorkflowState>(`/pm/workflows/${workflowId}/states/${stateId}${qs(workspaceId)}`, payload),
  removeState: (workspaceId: string, workflowId: string, stateId: string) =>
    api.del(`/pm/workflows/${workflowId}/states/${stateId}${qs(workspaceId)}`),
  reorderStates: (workspaceId: string, workflowId: string, stateIds: string[]) =>
    api.put(`/pm/workflows/${workflowId}/states/reorder${qs(workspaceId)}`, { state_ids: stateIds }),
  resolveTeamWorkflow: (workspaceId: string, teamId: string) =>
    api.get<WorkflowWithStates>(`/pm/workflows/resolve${qs(workspaceId)}&team_id=${encodeURIComponent(teamId)}`),
  copyToTeam: (workspaceId: string, workflowId: string, teamId: string) =>
    api.post<WorkflowWithStates>(`/pm/workflows/${workflowId}/copy-to-team${qs(workspaceId)}`, { team_id: teamId }),
};
