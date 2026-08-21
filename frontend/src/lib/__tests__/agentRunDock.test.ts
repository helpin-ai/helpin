// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';

import { openAgentRunInDock } from '../agentRunDock';

describe('openAgentRunInDock', () => {
  it('opens the returned dock chat when one is available', () => {
    const listener = vi.fn();
    window.addEventListener('helpin:ask-agents', listener);

    openAgentRunInDock({ id: 'run-1', dock_chat_id: 'chat-1' });

    expect(listener).toHaveBeenCalledTimes(1);
    expect((listener.mock.calls[0][0] as CustomEvent).detail).toEqual({ chatId: 'chat-1' });
    window.removeEventListener('helpin:ask-agents', listener);
  });

  it('opens the returned run in the runs view without a dock chat', () => {
    const listener = vi.fn();
    window.addEventListener('helpin:ask-agents', listener);

    openAgentRunInDock({ id: 'run-1' });

    expect((listener.mock.calls[0][0] as CustomEvent).detail).toEqual({ runId: 'run-1', mode: 'runs' });
    window.removeEventListener('helpin:ask-agents', listener);
  });
});
