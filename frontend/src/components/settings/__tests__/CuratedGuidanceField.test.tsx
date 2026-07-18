// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CuratedGuidanceField } from '../CuratedGuidanceField';

vi.mock('@/hooks/queries/useSupport', () => ({
  useCuratedGuidance: () => ({
    data: [{
      id: 'guidance-1',
      workspace_id: 'workspace-1',
      agent_id: 'agent-1',
      title: 'Current pricing',
      question_patterns: ['What is your pricing?', 'How much does it cost?'],
      answer: 'Growth starts at $49 per month. Enterprise pricing is custom.',
      intent: 'pricing_general',
      topics: [],
      language: 'en',
      status: 'active',
      created_at: '2026-07-17T00:00:00Z',
      updated_at: '2026-07-17T00:00:00Z',
    }],
    isLoading: false,
  }),
  useCreateCuratedGuidance: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useUpdateCuratedGuidance: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useDeleteCuratedGuidance: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('CuratedGuidanceField', () => {
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
  });

  it('shows canonical answers and their matching questions', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="agent-1" />);
    });

    expect(container.textContent).toContain('Current pricing');
    expect(container.textContent).toContain('Growth starts at $49 per month');
    expect(container.textContent).toContain('What is your pricing?');
    expect(container.textContent).toContain('never shown as citations');
  });
});
