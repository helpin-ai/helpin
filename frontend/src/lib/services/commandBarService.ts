import { api } from '../api';
import type {
  CommandBarDispatchRequest,
  CommandBarDispatchResponse,
  CommandBarParseRequest,
  CommandBarParseResponse,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const commandBarService = {
  parseIntent: (workspaceId: string, payload: CommandBarParseRequest) =>
    api.post<CommandBarParseResponse>(`/command-bar/intents/parse${qs(workspaceId)}`, payload),
  dispatchPlan: (workspaceId: string, payload: CommandBarDispatchRequest) =>
    api.post<CommandBarDispatchResponse>(`/command-bar/plans/dispatch${qs(workspaceId)}`, payload),
};
