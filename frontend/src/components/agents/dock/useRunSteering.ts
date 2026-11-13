import { useCallback, useEffect, useRef, useState } from 'react';
import { dockChatService, type RunSteeringCommand } from '@/lib/services/dockChatService';
import { codingSessionService } from '@/lib/services/codingSessionService';

type Run = { id: string; status: string; runtime_kind: string; execution_location?: string };

export function useRunSteering(workspaceId: string, run: Run | null, active: boolean, scope: 'dock' | 'pm' = 'dock', chatId?: string) {
  const runId = run?.id ?? '';
  const key = JSON.stringify([workspaceId, runId, scope, chatId]);
  const live = run?.status === 'running' || run?.status === 'queued';
  const supported = run?.runtime_kind === 'native_sdk' && run.execution_location !== 'local';
  const [state, setState] = useState<{ key: string; available: boolean; receipt: RunSteeringCommand | null }>({ key: '', available: false, receipt: null });
  const pending = useRef<{ key: string; command_id: string; expected_revision: number; content: string } | null>(null);
  const sending = useRef(false);
  const currentKey = useRef(key);
  currentKey.current = key;
  const read = useCallback(() => chatId
    ? dockChatService.chatRunSteering(workspaceId, chatId, runId)
    : scope === 'pm' ? codingSessionService.runSteering(workspaceId, runId)
      : dockChatService.runSteering(workspaceId, runId), [workspaceId, runId, scope, chatId]);
  const receipt = state.key === key ? state.receipt : null;

  useEffect(() => {
    if (!active || !supported || !runId || !workspaceId) return;
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    const refresh = async () => {
      try {
        const result = await read();
        if (!disposed && !result.error && Array.isArray(result.data)) {
          setState(previous => {
            const latest = result.data!.at(-1) ?? null;
            const retained = previous.key === key ? previous.receipt : null;
            return { key, available: true, receipt: retained && (!latest || (retained.revision > latest.revision || (retained.revision === latest.revision && retained.state !== 'accepted'))) ? retained : latest };
          });
        }
      } catch { /* Keep the last receipt during a connection interruption. */ }
      if (!disposed && (live || receipt?.state === 'accepted')) timer = setTimeout(() => void refresh(), 5000);
    };
    void refresh();
    return () => { disposed = true; clearTimeout(timer); };
  }, [active, supported, runId, workspaceId, key, read, live, receipt?.state]);

  const send = useCallback(async (content: string) => {
    if (!active || !supported || !live) throw new Error('This run is no longer accepting steering.');
    if (sending.current) throw new Error('An instruction is already being sent.');
    sending.current = true;
    try {
      if (!pending.current || pending.current.key !== key || pending.current.content !== content) {
        const history = await read();
        if (history.error || !history.data) throw new Error(history.error ?? 'Unable to load steering');
        if (currentKey.current !== key) throw new Error('The selected run changed.');
        pending.current = { key, command_id: crypto.randomUUID(), expected_revision: history.data.at(-1)?.revision ?? 0, content };
      }
      const { key: _, ...command } = pending.current;
      const result = await (chatId ? dockChatService.steerChatRun(workspaceId, chatId, runId, command)
        : scope === 'pm' ? codingSessionService.steerRun(workspaceId, runId, command)
          : dockChatService.steerRun(workspaceId, runId, command));
      if (result.error || !result.data) {
        if (result.status === 409 && pending.current?.command_id === command.command_id) pending.current = null;
        throw new Error(result.error ?? 'Instruction was not accepted');
      }
      if (currentKey.current === key) {
        setState({ key, available: true, receipt: result.data });
        pending.current = null;
      }
    } finally { sending.current = false; }
  }, [active, supported, live, key, read, chatId, workspaceId, runId, scope]);

  return { canSteer: active && supported && live && state.key === key && state.available, receipt, send };
}
