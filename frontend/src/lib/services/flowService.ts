import { api } from '../api';
import type {
  FlowNodeRun,
  FlowRunView,
  FlowSpec,
  FlowTemplate,
  FlowTemplateNode,
  FlowTemplateView,
  CreateFlowTemplateRequest,
  UpdateFlowTemplateRequest,
  UpdateFlowTemplateNodeRequest,
  CreateFlowTemplateNodeRequest,
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

  cancelActiveByTarget: (wsId: string, targetType: string, targetId: string) =>
    api.post<FlowRunView | { status: string }>(`/pm/flow-runs/cancel-by-target${qs(wsId)}`, {
      target_type: targetType,
      target_id: targetId,
    }),

  retryNode: (wsId: string, flowRunId: string, nodeRunId: string) =>
    api.post<FlowRunView>(`/pm/flow-runs/${flowRunId}/nodes/${nodeRunId}/retry${qs(wsId)}`, {}),

  listRuns: (wsId: string, limit = 20, offset = 0) =>
    api.get<{ data: FlowRunView[]; total: number }>(`/pm/flow-runs${qs(wsId)}&limit=${limit}&offset=${offset}`),

  listTemplates: (wsId: string) =>
    api.get<FlowSpec[]>(`/pm/flow-templates${qs(wsId)}`),

  // --- Flow Template CRUD ---

  createTemplate: (wsId: string, body: CreateFlowTemplateRequest) =>
    api.post<FlowTemplateView>(`/pm/flow-templates${qs(wsId)}`, body),

  getTemplate: (wsId: string, templateId: string) =>
    api.get<FlowTemplateView>(`/pm/flow-templates/${templateId}${qs(wsId)}`),

  updateTemplate: (wsId: string, templateId: string, body: UpdateFlowTemplateRequest) =>
    api.put<FlowTemplateView>(`/pm/flow-templates/${templateId}${qs(wsId)}`, body),

  deleteTemplate: (wsId: string, templateId: string) =>
    api.del<{ status: string }>(`/pm/flow-templates/${templateId}${qs(wsId)}`),

  listDBTemplates: (wsId: string) =>
    api.get<FlowTemplate[]>(`/pm/flow-db-templates${qs(wsId)}`),

  duplicateTemplate: (wsId: string, templateId: string) =>
    api.post<FlowTemplateView>(`/pm/flow-templates/${templateId}/duplicate${qs(wsId)}`, {}),

  duplicateFromSlug: (wsId: string, templateSlug: string) =>
    api.post<FlowTemplateView>(`/pm/flow-templates/duplicate-from-slug${qs(wsId)}`, { template_slug: templateSlug }),

  createTemplateNode: (wsId: string, templateId: string, body: CreateFlowTemplateNodeRequest) =>
    api.post<FlowTemplateNode>(`/pm/flow-templates/${templateId}/nodes${qs(wsId)}`, body),

  updateTemplateNode: (wsId: string, templateId: string, nodeId: string, body: UpdateFlowTemplateNodeRequest) =>
    api.put<FlowTemplateNode>(`/pm/flow-templates/${templateId}/nodes/${nodeId}${qs(wsId)}`, body),

  deleteTemplateNode: (wsId: string, templateId: string, nodeId: string) =>
    api.del<{ status: string }>(`/pm/flow-templates/${templateId}/nodes/${nodeId}${qs(wsId)}`),
};
