// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { SupportConversation } from '@/lib/pmTypes';
import { queryKeys } from '@/lib/queryKeys';
import { SupportFollowUpStatus } from './SupportFollowUpStatus';
const mocks = vi.hoisted(() => ({ cancel: vi.fn(), error: vi.fn(), success: vi.fn() }));
vi.mock('@/lib/services/supportService', () => ({
  supportService: { cancelConversationFollowUp: mocks.cancel },
}));
vi.mock('sonner', () => ({ toast: { error: mocks.error, success: mocks.success } }));
let container: HTMLDivElement, root: Root, client: QueryClient;
const base = {
  id: 'c',
  workspace_id: 'ws',
  status: 'open',
  flow_state: 'ai_handling',
  ai_state: 'pending',
  last_public_message_id: 'source',
  last_public_sender_type: 'ai',
  last_public_message_at: '2026-10-10T10:00:00Z',
  ai_follow_up: {
    id: 'episode',
    source_message_id: 'source',
    status: 'scheduled',
    sequence_version: 2,
    due_at: '2026-10-11T10:00:00Z',
    created_at: '2026-10-10T10:01:00Z',
    updated_at: '2026-10-10T10:01:00Z',
  },
} as SupportConversation;
beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  vi.clearAllMocks();
});
afterEach(() => {
  act(() => root.unmount());
  client.clear();
  container.remove();
  vi.unstubAllGlobals();
});
async function render(conversation = base, canEdit = true) {
  await act(async () =>
    root.render(
      <QueryClientProvider client={client}>
        <TooltipProvider>
          <SupportFollowUpStatus conversation={conversation} canEdit={canEdit} />
        </TooltipProvider>
      </QueryClientProvider>,
    ),
  );
}
function stop() {
  return [...container.querySelectorAll('button')].find((button) =>
    button.textContent?.startsWith('Stop'),
  );
}
describe('current follow-up row', () => {
  it('shows one schedule with a scoped stop action', async () => {
    await render();
    expect(container.querySelectorAll('[data-support-follow-up]')).toHaveLength(1);
    expect(container.textContent).toContain('AI follow-up scheduled');
    expect(container.querySelector('time')?.dateTime).toBe(base.ai_follow_up!.due_at);
    expect(stop()?.textContent).toBe('Stop follow-ups');
  });
  it('viewers can see the schedule without a stop action', async () => {
    await render(base, false);
    expect(container.textContent).toContain('AI follow-up scheduled');
    expect(stop()).toBeUndefined();
  });
  it('replaces the first schedule with automatic closure', async () => {
    await render();
    await render({
      ...base,
      ai_follow_up: {
        ...base.ai_follow_up!,
        status: 'waiting',
        sequence_version: 1,
        close_at: '2026-10-12T10:00:00Z',
      },
    });
    expect(container.querySelectorAll('[data-support-follow-up]')).toHaveLength(1);
    expect(container.textContent).toContain('Conversation will close if there’s no reply');
    expect(container.textContent).not.toContain('AI follow-up scheduled');
    expect(stop()?.textContent).toBe('Stop auto-close');
  });
  it('stops the displayed episode and updates the correct conversation cache', async () => {
    const stopped = {
      ...base,
      ai_follow_up: {
        ...base.ai_follow_up!,
        status: 'cancelled' as const,
        reason: 'cancelled_by_teammate',
      },
    };
    mocks.cancel.mockResolvedValue({ data: stopped, error: null });
    await render();
    await act(async () => stop()!.click());
    expect(mocks.cancel).toHaveBeenCalledWith('ws', 'c', 'episode');
    expect(client.getQueryData(queryKeys.support.conversation('ws', 'c'))).toEqual(stopped);
    await render(stopped);
    expect(container.textContent).toContain('AI follow-ups stopped');
    expect(stop()).toBeUndefined();
  });
  it('keeps the schedule on a failed stop', async () => {
    mocks.cancel.mockResolvedValue({ data: null, error: 'network failure' });
    await render();
    await act(async () => stop()!.click());
    expect(mocks.error).toHaveBeenCalled();
    expect(container.textContent).toContain('AI follow-up scheduled');
  });
  it('removes all notices when the customer replies or the conversation resolves', async () => {
    await render();
    await render({
      ...base,
      last_public_message_id: 'customer-reply',
      last_public_sender_type: 'customer',
    });
    expect(container.textContent).toBe('');
    await render({ ...base, status: 'resolved' });
    expect(container.textContent).toBe('');
  });
  it('shows only a concise failure, with technical detail kept out of the row', async () => {
    await render({
      ...base,
      ai_follow_up: { ...base.ai_follow_up!, status: 'failed', reason: 'assessment_timeout' },
    });
    expect(container.textContent).toContain('AI follow-up couldn’t be completed');
    expect(container.textContent).not.toContain('assessment_timeout');
    expect(stop()).toBeUndefined();
  });
});
