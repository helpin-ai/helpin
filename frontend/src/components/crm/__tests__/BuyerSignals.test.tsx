// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { BuyerSignals } from '../BuyerSignals';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const state = vi.hoisted(() => ({
  dismiss: vi.fn(),
  feedback: vi.fn(),
  companyData: {
    data: [{
      id: 'signal-1', workspace_id: 'ws-1', signal_type: 'buying_intent', source_type: 'email',
      source_thread_id: 'thread-1', summary: 'The buyer requested pricing.',
      evidence_excerpt: 'Please send enterprise pricing.', confidence: 0.91,
      detected_at: '2026-08-23T12:00:00Z', created_at: '2026-08-23T12:00:00Z',
      contact_name: 'Ava Buyer', deal_name: 'Expansion', deal_display_id: 'DEAL-7',
    }],
  },
}));

vi.mock('@/hooks/queries', () => ({
  useCompanySignals: () => ({ data: state.companyData, isLoading: false }),
  useContactSignals: () => ({ data: { data: [] }, isLoading: false }),
  useDealSignals: () => ({ data: { data: [] }, isLoading: false }),
  useDismissBuyerSignal: () => ({ mutate: state.dismiss, isPending: false }),
  useBuyerSignalFeedback: () => ({ mutate: state.feedback, isPending: false }),
  useEmailAccounts: () => ({ data: [] }),
  useWorkspaceMembers: () => ({ data: [] }),
}));

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { slug: string } }) => unknown) => selector({ currentWorkspace: { slug: 'acme' } }),
}));

describe('BuyerSignals company roll-up', () => {
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

  it('shows canonical evidence with contact and deal context', () => {
    act(() => root.render(<BuyerSignals workspaceId="ws-1" companyId="company-1" presentation="overview" />));

    expect(container.textContent).toContain('Please send enterprise pricing.');
    expect(container.textContent).toContain('Ava Buyer');
    expect(container.textContent).toContain('DEAL-7 · Expansion');
    expect(container.textContent).toContain('Open thread');
  });
});
