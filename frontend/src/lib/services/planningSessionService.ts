import { api } from '../api';
import type {
  PlanningSession,
  PlanningSessionMessage,
  StartPlanningSessionRequest,
} from '../pmTypes';

const qs = (wsId: string) => `?workspace_id=${encodeURIComponent(wsId)}`;

export const planningSessionService = {
  start: (wsId: string, epicId: string, body: StartPlanningSessionRequest) =>
    api.post<PlanningSession>(`/pm/epics/${epicId}/planning-session${qs(wsId)}`, body),

  get: (wsId: string, sessionId: string) =>
    api.get<PlanningSession>(`/pm/planning-sessions/${sessionId}${qs(wsId)}`),

  getMessages: (wsId: string, sessionId: string) =>
    api.get<PlanningSessionMessage[]>(`/pm/planning-sessions/${sessionId}/messages${qs(wsId)}`),

  sendMessage: (wsId: string, sessionId: string, content: string, messageType?: string) =>
    api.post<PlanningSessionMessage>(`/pm/planning-sessions/${sessionId}/messages${qs(wsId)}`, {
      content,
      message_type: messageType,
    }),

  finalize: (wsId: string, sessionId: string) =>
    api.post<PlanningSession>(`/pm/planning-sessions/${sessionId}/finalize${qs(wsId)}`, {}),

  abandon: (wsId: string, sessionId: string) =>
    api.post<void>(`/pm/planning-sessions/${sessionId}/abandon${qs(wsId)}`, {}),
};
