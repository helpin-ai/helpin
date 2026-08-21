import type { AgentRun } from '@/lib/pmTypes';

/** Opens the dock target returned by an agent-run launch. */
export function openAgentRunInDock(run: Pick<AgentRun, 'id' | 'dock_chat_id'>) {
  window.dispatchEvent(new CustomEvent('helpin:ask-agents', {
    detail: run.dock_chat_id
      ? { chatId: run.dock_chat_id }
      : { runId: run.id, mode: 'runs' },
  }));
}
