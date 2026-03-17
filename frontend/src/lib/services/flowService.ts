import { api } from '../api';
import type {
  FlowNodeRun,
  FlowRunView,
  PlanningSessionMessage,
  StartFlowRunRequest,
} from '../pmTypes';

const qs = (wsId: string) => `?workspace_id=${encodeURIComponent(wsId)}`;

export const flowService = {
  startRun: (wsId: string, body: StartFlowRunRequest) =>
    api.post<FlowRunView>(`/pm/flow-runs${qs(wsId)}`, body),

  getRun: (wsId: string, flowRunId: string) =>
    api.get<FlowRunView>(`/pm/flow-runs/${flowRunId}${qs(wsId)}`),

  listNodeRuns: (wsId: string, flowRunId: string) =>
    api.get<FlowNodeRun[]>(`/pm/flow-runs/${flowRunId}/nodes${qs(wsId)}`),

  sendNodeAction: (wsId: string, flowRunId: string, nodeRunId: string, actionType: string, payload?: unknown) =>
    api.post<FlowRunView>(`/pm/flow-runs/${flowRunId}/nodes/${nodeRunId}/actions${qs(wsId)}`, {
      action_type: actionType,
      payload: payload ?? {},
    }),

  getNodeMessages: (wsId: string, flowRunId: string, nodeRunId: string) =>
    api.get<PlanningSessionMessage[]>(`/pm/flow-runs/${flowRunId}/nodes/${nodeRunId}/messages${qs(wsId)}`),

  sendNodeMessage: (wsId: string, flowRunId: string, nodeRunId: string, content: string) =>
    api.post<PlanningSessionMessage>(`/pm/flow-runs/${flowRunId}/nodes/${nodeRunId}/messages${qs(wsId)}`, {
      content,
    }),

  cancelRun: (wsId: string, flowRunId: string) =>
    api.post<FlowRunView>(`/pm/flow-runs/${flowRunId}/cancel${qs(wsId)}`, {}),

  retryNode: (wsId: string, flowRunId: string, nodeRunId: string) =>
    api.post<FlowRunView>(`/pm/flow-runs/${flowRunId}/nodes/${nodeRunId}/retry${qs(wsId)}`, {}),
};
