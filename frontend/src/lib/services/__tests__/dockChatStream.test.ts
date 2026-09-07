import { beforeEach, describe, expect, it, vi } from 'vitest';
import { dockChatService } from '../dockChatService';

const { get } = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock('@/lib/api', () => ({ api: { get } }));

beforeEach(() => { get.mockReset(); get.mockResolvedValue({ data: null, error: null }); });

describe('dock stream API contracts', () => {
  it('opts both dock event routes out of the duplicate snapshot and forwards cancellation', async () => {
    const signal = new AbortController().signal;
    await dockChatService.listChatRunEvents('ws-1', 'chat-1', 37, signal);
    await dockChatService.listRunEvents('ws-1', 'run-1', 8, signal);
    expect(get.mock.calls).toEqual([
      ['/dock/chats/chat-1/run/events?workspace_id=ws-1&after=37&include_snapshot=false', { signal }],
      ['/dock/runs/run-1/events?workspace_id=ws-1&after=8&include_snapshot=false', { signal }],
    ]);
  });

  it('keeps full run snapshots and forwards cancellation on both dock routes', async () => {
    const signal = new AbortController().signal;
    await dockChatService.getChatRun('ws-1', 'chat-1', signal);
    await dockChatService.getRunSnapshot('ws-1', 'run-1', signal);
    expect(get.mock.calls).toEqual([
      ['/dock/chats/chat-1/run?workspace_id=ws-1', { signal }],
      ['/dock/runs/run-1/snapshot?workspace_id=ws-1', { signal }],
    ]);
  });
});
