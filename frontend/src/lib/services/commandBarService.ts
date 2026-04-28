import { api } from '../api';
import type {
  CommandBarDispatchRequest,
  CommandBarDispatchResponse,
  CommandBarCancelPlanResponse,
  CommandBarPlanDetailResponse,
  CommandBarPlanListResponse,
  CommandBarParseRequest,
  CommandBarParseResponse,
  CommandBarRetryPlanResponse,
  CommandBarToolCatalogResponse,
  CommandBarUnmetIntent,
  CommandBarUnmetIntentListResponse,
  CommandBarUnmetIntentStatus,
  PromoteCommandBarRunRequest,
  PromoteCommandBarRunResponse,
  ReviewCommandBarUnmetIntentRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const commandBarService = {
  parseIntent: (workspaceId: string, payload: CommandBarParseRequest) =>
    api.post<CommandBarParseResponse>(`/command-bar/intents/parse${qs(workspaceId)}`, payload),
  listPlans: (workspaceId: string, limit = 20) =>
    api.get<CommandBarPlanListResponse>(`/command-bar/plans${qs(workspaceId)}&limit=${encodeURIComponent(limit)}`),
  getPlan: (workspaceId: string, planId: string) =>
    api.get<CommandBarPlanDetailResponse>(`/command-bar/plans/${encodeURIComponent(planId)}${qs(workspaceId)}`),
  getAgentToolCatalog: (workspaceId: string, agentId: string, selectedTools?: string[]) => {
    const params = (selectedTools ?? []).map((tool) => `&selected=${encodeURIComponent(tool)}`).join('');
    return api.get<CommandBarToolCatalogResponse>(`/command-bar/agents/${encodeURIComponent(agentId)}/tools${qs(workspaceId)}${params}`);
  },
  dispatchPlan: (workspaceId: string, payload: CommandBarDispatchRequest) =>
    api.post<CommandBarDispatchResponse>(`/command-bar/plans/dispatch${qs(workspaceId)}`, payload),
  cancelPlan: (workspaceId: string, planId: string) =>
    api.post<CommandBarCancelPlanResponse>(`/command-bar/plans/${encodeURIComponent(planId)}/cancel${qs(workspaceId)}`),
  retryPlan: (workspaceId: string, planId: string, stepIndex: number) =>
    api.post<CommandBarRetryPlanResponse>(`/command-bar/plans/${encodeURIComponent(planId)}/retry${qs(workspaceId)}`, { step_index: stepIndex }),
  promoteRunToAgent: (workspaceId: string, runId: string, payload: PromoteCommandBarRunRequest) =>
    api.post<PromoteCommandBarRunResponse>(`/command-bar/runs/${encodeURIComponent(runId)}/promote-agent${qs(workspaceId)}`, payload),
  listUnmetIntents: (workspaceId: string, opts?: { status?: CommandBarUnmetIntentStatus | 'all'; limit?: number; includeSensitive?: boolean }) => {
    const status = opts?.status && opts.status !== 'all' ? `&status=${encodeURIComponent(opts.status)}` : '';
    const limit = opts?.limit ? `&limit=${encodeURIComponent(opts.limit)}` : '';
    const includeSensitive = opts?.includeSensitive ? `&include_sensitive=true` : '';
    return api.get<CommandBarUnmetIntentListResponse>(`/command-bar/unmet-intents${qs(workspaceId)}${status}${limit}${includeSensitive}`);
  },
  reviewUnmetIntent: (workspaceId: string, intentId: string, payload: ReviewCommandBarUnmetIntentRequest) =>
    api.post<CommandBarUnmetIntent>(`/command-bar/unmet-intents/${encodeURIComponent(intentId)}/review${qs(workspaceId)}`, payload),
};
