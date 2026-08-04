export type AgentRunUpdateKind = 'progress' | 'lifecycle';

export type AgentRunEventDetail = {
  action?: string;
  entity?: string;
  entity_id?: string;
  parent_type?: string;
  parent_id?: string;
  agent_id?: string;
  status?: string;
  pause_reason?: string;
  approval_state?: string;
  sent_at?: string;
  update_kind?: AgentRunUpdateKind;
  data?: Record<string, unknown>;
};

export function isAgentRunEventDetail(value: unknown): value is AgentRunEventDetail {
  return typeof value === 'object' && value !== null;
}

export function classifyAgentRunUpdate(event: Pick<AgentRunEventDetail, 'action' | 'data'>): AgentRunUpdateKind {
  const status = typeof event.data?.status === 'string' ? event.data.status : '';
  const pauseReason = typeof event.data?.pause_reason === 'string' ? event.data.pause_reason : '';
  return event.action === 'updated' && status === 'running' && (!pauseReason || pauseReason === 'none')
    ? 'progress'
    : 'lifecycle';
}

export function isAgentRunLifecycleEvent(event: Event): boolean {
  const detail = (event as CustomEvent<unknown>).detail;
  return !isAgentRunEventDetail(detail) || detail.update_kind !== 'progress';
}
