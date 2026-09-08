export type AgentRunUpdateKind = 'progress' | 'lifecycle' | 'content' | 'duplicate';

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
  change_kind?: string;
  dock_chat_id?: string;
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

// Only explicitly typed snapshots can be deduplicated. Older producers also
// use run updates to announce persisted messages without changing run state.
export function createAgentRunUpdateTracker() {
  type State = { signature: string; attention: boolean; values: string[] };
  const states = new Map<string, State>();
  return (event: AgentRunEventDetail): { kind: AgentRunUpdateKind; attentionChanged: boolean } => {
    const data = event.data ?? {};
    // Content rows are committed before the run row during reconciliation.
    // They must not consume the later, authoritative state notification.
    if (['message', 'interaction', 'artifact'].includes(String(data.change_kind))) {
      return { kind: 'content', attentionChanged: false };
    }
    if (!event.entity_id || event.action !== 'updated' || typeof data.status !== 'string') {
      const kind = classifyAgentRunUpdate(event);
      return { kind, attentionChanged: kind === 'lifecycle' };
    }
    // Some consumers listen to one alias only. Each alias must deliver its own
    // transition, even when the other arrives first; readers coalesce the pair.
    const key = JSON.stringify([event.entity, event.entity_id]);
    const previous = states.get(key);
    const typed = data.change_kind === 'state';
    const values = ['status', 'pause_reason', 'approval_state', 'execution_stage', 'state_revision'].map((field, index) =>
      typeof data[field] === 'string' ? data[field] as string : (!typed && previous ? previous.values[index] : ''));
    if (values[1] === 'none') values[1] = '';
    const attention = values[0] === 'paused' && values[1] !== 'awaiting_user_message';
    const signature = JSON.stringify(values);
    // Bound memory for a long-lived workspace tab. An evicted state is handled
    // conservatively on its next event, so no update can be lost.
    if (states.size >= 4000 && !previous) states.delete(states.keys().next().value!);
    states.set(key, { signature, attention, values });
    const changed = !!previous && previous.signature !== signature;
    const kind = changed ? 'lifecycle' : !typed ? classifyAgentRunUpdate(event)
      : previous ? 'duplicate' : 'lifecycle';
    return { kind, attentionChanged: !previous || previous.attention !== attention };
  };
}

export function isAgentRunLifecycleEvent(event: Event): boolean {
  const detail = (event as CustomEvent<unknown>).detail;
  return !isAgentRunEventDetail(detail) || !detail.update_kind || detail.update_kind === 'lifecycle';
}
