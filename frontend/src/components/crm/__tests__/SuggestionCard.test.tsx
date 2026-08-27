// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { SuggestionCard } from '../SuggestionCard';
import type { CRMSuggestion } from '@/lib/crmTypes';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({ navigate: vi.fn(), confirm: vi.fn(async () => true) }));

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => mocks.navigate }));
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => mocks.confirm }));
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { slug: string } }) => unknown) => selector({ currentWorkspace: { slug: 'acme' } }),
}));

const suggestion: CRMSuggestion = {
  id: 'suggestion-1', workspace_id: 'ws-1', suggestion_type: 'deal_create', title: 'Create Acme expansion',
  context: { deal_name: 'Acme expansion', contact_id: 'contact-1', pipeline_id: 'pipeline-1', stage_id: 'stage-1' },
  status: 'pending', confidence: 0.92, created_at: '2026-08-27T08:00:00Z', updated_at: '2026-08-27T08:00:00Z',
  signal_ids: ['signal-1'],
  signals: [{
    id: 'signal-1', workspace_id: 'ws-1', contact_id: 'contact-1', signal_type: 'buying_intent', source_type: 'email',
    source_thread_id: 'thread-1', summary: 'Buyer asked for procurement terms', evidence_excerpt: 'Please send the security package.',
    confidence: 0.9, evidence_identity_trust: 'verified', detected_at: '2026-08-27T07:00:00Z', created_at: '2026-08-27T07:00:00Z',
  }],
};

describe('SuggestionCard evidence and approval', () => {
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

  it('shows supporting evidence and confirms an immediate mutation', async () => {
    const accept = vi.fn();
    act(() => root.render(<SuggestionCard suggestion={suggestion} onAccept={accept} onDismiss={vi.fn()} />));

    expect(container.textContent).toContain('Why Helpin recommends this');
    expect(container.textContent).toContain('Please send the security package.');
    const createButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Create deal'));
    await act(async () => createButton?.click());

    expect(mocks.confirm).toHaveBeenCalledWith(expect.objectContaining({ title: 'Create Acme expansion?', confirmText: 'Create deal' }));
    expect(accept).toHaveBeenCalledWith('suggestion-1');
  });

  it('opens the exact linked email source', () => {
    act(() => root.render(<SuggestionCard suggestion={suggestion} onAccept={vi.fn()} onDismiss={vi.fn()} />));
    const sourceButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Open exact source');
    act(() => sourceButton?.click());

    expect(mocks.navigate).toHaveBeenCalledWith(expect.objectContaining({
      to: '/w/$slug/crm/contacts/$contactId',
      search: { tab: 'emails', thread: 'thread-1' },
    }));
  });
});
