// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../inboxClearedConfetti', () => ({ launchInboxClearedConfetti: vi.fn(() => vi.fn()) }));

import { launchInboxClearedConfetti } from '../inboxClearedConfetti';
import { InboxClearedCelebration } from '../InboxClearedCelebration';
import type { InboxCelebration } from '@/stores/supportInboxCelebrationStore';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

const celebration: InboxCelebration = {
  id: 'shared:1',
  scopeKey: 'shared',
  clearedCount: 7,
  confetti: true,
  message: { achievementId: 'early-bird', rarity: 'rare', name: 'Early bird', title: 'Clear before 9.', body: 'The rest of the morning is yours.' },
};

function mockReducedMotion(reduce: boolean) {
  window.matchMedia = vi.fn().mockReturnValue({ matches: reduce }) as unknown as typeof window.matchMedia;
}

describe('InboxClearedCelebration', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.clearAllMocks();
  });

  it('shows the achievement and launches confetti sized by the clear', () => {
    mockReducedMotion(false);
    act(() => root.render(<InboxClearedCelebration celebration={celebration} />));

    const status = container.querySelector('[role="status"]');
    expect(status?.getAttribute('data-rarity')).toBe('rare');
    expect(status?.textContent).toContain('Rare');
    expect(status?.textContent).toContain('Early bird');
    expect(container.querySelector('h2')?.textContent).toBe('Clear before 9.');
    expect(status?.textContent).toContain('The rest of the morning is yours.');
    expect(launchInboxClearedConfetti).toHaveBeenCalledTimes(1);
    expect(vi.mocked(launchInboxClearedConfetti).mock.calls[0][0]).toMatchObject({ size: 7 });
  });

  it('shows an everyday clear without confetti', () => {
    mockReducedMotion(false);
    act(() => root.render(<InboxClearedCelebration celebration={{ ...celebration, confetti: false }} />));

    expect(container.querySelector('h2')?.textContent).toBe('Clear before 9.');
    expect(launchInboxClearedConfetti).not.toHaveBeenCalled();
  });

  it('skips confetti for reduced motion', () => {
    mockReducedMotion(true);
    act(() => root.render(<InboxClearedCelebration celebration={celebration} />));

    expect(container.querySelector('h2')?.textContent).toBe('Clear before 9.');
    expect(launchInboxClearedConfetti).not.toHaveBeenCalled();
  });
});
