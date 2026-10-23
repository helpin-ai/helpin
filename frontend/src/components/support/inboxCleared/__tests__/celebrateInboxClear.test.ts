// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/lib/services/supportService', () => ({
  supportService: { recordInboxClear: vi.fn() },
}));

import { supportService } from '@/lib/services/supportService';
import { useSupportInboxCelebrationStore } from '@/stores/supportInboxCelebrationStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { celebrateInboxClear } from '../useInboxClearedDetection';

const scope = { scope: 'shared' as const, mailboxId: null, scopeKey: 'shared' };
const input = {
  workspaceId: 'ws-1',
  userId: 'user-1',
  scope,
  names: { inbox: 'Shared Inbox', inboxInSentence: 'Shared Inbox' },
  conversationId: 'conv-9',
  fallbackCount: 3,
  now: new Date(2026, 9, 21, 15, 0, 0), // a Wednesday afternoon
};

describe('celebrateInboxClear', () => {
  beforeEach(() => {
    localStorage.clear();
    useSupportInboxStore.setState({ navFilter: 'inbox', selectedMailboxId: 'all', selectedConversationId: null });
    useSupportInboxCelebrationStore.setState({ celebration: null, ownRemovals: {} });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('records the clear and shows a celebration sized by the server count', async () => {
    vi.mocked(supportService.recordInboxClear).mockResolvedValue({
      data: { streak_days: 8, clears_today: 1, resolved_today: 14, cleared_count: 6 },
      error: null,
    } as Awaited<ReturnType<typeof supportService.recordInboxClear>>);

    await celebrateInboxClear(input);

    expect(supportService.recordInboxClear).toHaveBeenCalledWith('ws-1', {
      conversation_id: 'conv-9',
      scope: 'shared',
      mailbox_id: null,
      local_date: '2026-10-21',
      day_start: new Date(2026, 9, 21).toISOString(),
    });
    const celebration = useSupportInboxCelebrationStore.getState().celebration;
    expect(celebration).toMatchObject({ scopeKey: 'shared', clearedCount: 6, confetti: true });
    expect(celebration?.message.achievementId).toBe('hot-streak');
  });

  it('still celebrates when the server cannot be reached', async () => {
    vi.mocked(supportService.recordInboxClear).mockRejectedValue(new Error('offline'));

    await celebrateInboxClear(input);

    const celebration = useSupportInboxCelebrationStore.getState().celebration;
    // An everyday clear without the server's numbers stays quiet.
    expect(celebration).toMatchObject({ clearedCount: 3, confetti: false });
    expect(celebration?.message.rarity).toBe('common');
  });

  it('skips the celebration when the viewer already opened a conversation', async () => {
    vi.mocked(supportService.recordInboxClear).mockResolvedValue({ data: null, error: 'nope' } as Awaited<ReturnType<typeof supportService.recordInboxClear>>);
    useSupportInboxStore.setState({ selectedConversationId: 'conv-new' });

    await celebrateInboxClear(input);

    expect(useSupportInboxCelebrationStore.getState().celebration).toBeNull();
  });
});
