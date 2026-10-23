// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/lib/services/supportService', () => ({
  supportService: { recordInboxClear: vi.fn() },
}));

import { supportService } from '@/lib/services/supportService';
import { useSupportInboxCelebrationStore } from '@/stores/supportInboxCelebrationStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useInboxClearedDetection } from '../useInboxClearedDetection';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

const scope = { scope: 'all' as const, mailboxId: null, scopeKey: 'all' };
const names = { inbox: 'Your inbox', inboxInSentence: 'your inbox' };

function Harness({ ids }: { ids: string[] }) {
  useInboxClearedDetection({ workspaceId: 'ws-1', userId: 'user-1', scope, names, conversationIds: ids, settled: true });
  return null;
}

describe('useInboxClearedDetection', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    localStorage.clear();
    useSupportInboxStore.setState({ navFilter: 'inbox', selectedMailboxId: 'all', selectedConversationId: null });
    useSupportInboxCelebrationStore.setState({ celebration: null, ownRemovals: {} });
    vi.mocked(supportService.recordInboxClear).mockResolvedValue({
      data: { streak_days: 1, clears_today: 1, resolved_today: 2, cleared_count: 2 },
      error: null,
    } as Awaited<ReturnType<typeof supportService.recordInboxClear>>);
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.clearAllMocks();
  });

  async function renderIds(ids: string[]) {
    await act(async () => {
      root.render(<Harness ids={ids} />);
    });
    await act(async () => {});
  }

  it('celebrates when the viewer resolves the last conversations', async () => {
    await renderIds(['a', 'b']);
    useSupportInboxCelebrationStore.getState().noteOwnRemovals(['a', 'b']);
    await renderIds([]);

    expect(supportService.recordInboxClear).toHaveBeenCalledTimes(1);
    expect(useSupportInboxCelebrationStore.getState().celebration?.scopeKey).toBe('all');
  });

  it('stays quiet when someone else empties the list', async () => {
    await renderIds(['a', 'b']);
    useSupportInboxCelebrationStore.getState().noteOwnRemovals(['a']);
    await renderIds([]);

    expect(supportService.recordInboxClear).not.toHaveBeenCalled();
    expect(useSupportInboxCelebrationStore.getState().celebration).toBeNull();
  });

  it('puts the celebration away when a new conversation arrives', async () => {
    await renderIds(['a']);
    useSupportInboxCelebrationStore.getState().noteOwnRemovals(['a']);
    await renderIds([]);
    expect(useSupportInboxCelebrationStore.getState().celebration).not.toBeNull();

    await renderIds(['new']);
    expect(useSupportInboxCelebrationStore.getState().celebration).toBeNull();
  });
});
