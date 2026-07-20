// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CuratedGuidanceField } from '../CuratedGuidanceField';

const mutations = vi.hoisted(() => ({
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}));

vi.mock('@/hooks/queries/useSupport', () => ({
  useCuratedGuidance: (_workspaceId: string, agentId?: string) => ({
    data: agentId === 'empty-agent' ? [] : [{
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
  useCreateCuratedGuidance: () => ({ isPending: false, mutateAsync: mutations.create }),
  useUpdateCuratedGuidance: () => ({ isPending: false, mutateAsync: mutations.update }),
  useDeleteCuratedGuidance: () => ({ isPending: false, mutateAsync: mutations.remove }),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('CuratedGuidanceField', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    mutations.create.mockReset().mockResolvedValue(undefined);
    mutations.update.mockReset().mockResolvedValue(undefined);
    mutations.remove.mockReset().mockResolvedValue(undefined);
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it('shows answer guidance and its matching questions', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="agent-1" />);
    });

    expect(container.textContent).toContain('Current pricing');
    expect(container.textContent).toContain('Growth starts at $49 per month');
    expect(container.textContent).toContain('What is your pricing?');
    expect(container.textContent).toContain('Answer Guidance');
    expect(container.textContent).toContain('Applies to:');
  });

  it('explains when answer guidance is useful', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="empty-agent" />);
    });

    expect(container.textContent).toContain('Guide answers where consistency matters');
    expect(container.textContent).toContain('pricing, refunds, plan limits, security, and company policies');
  });

  it('explains that guidance belongs to the selected support agent', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" />);
    });

    expect(container.textContent).toContain('Select a support agent first');
    expect(container.textContent).toContain('Choose an agent in Configuration above');
  });

  it('opens a growing inline editor instead of a modal', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="empty-agent" />);
    });

    const addButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Add guidance'));
    await act(async () => {
      addButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    const answer = container.querySelector('#new-guidance-answer');
    expect(container.textContent).toContain('New answer guidance');
    expect(container.textContent).toContain('Changes take effect after you save.');
    expect(container.querySelector('[role="dialog"]')).toBeNull();
    expect(answer).toBeInstanceOf(HTMLTextAreaElement);
    expect(answer?.className).toContain('field-sizing-content');
  });

  it('edits existing guidance in place', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="agent-1" />);
    });

    const editButton = container.querySelector('button[aria-label="Edit Current pricing"]');
    await act(async () => {
      editButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(container.textContent).toContain('Edit answer guidance');
    expect(container.querySelector('#edit-guidance-title')).toHaveProperty('value', 'Current pricing');
    expect(container.querySelector('#edit-guidance-answer')).toHaveProperty(
      'value',
      'Growth starts at $49 per month. Enterprise pricing is custom.',
    );
    expect(container.querySelector('[role="dialog"]')).toBeNull();
  });

  it('saves new guidance only after the explicit save action', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="empty-agent" />);
    });

    const addButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Add guidance'));
    await act(async () => {
      addButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    const setValue = async (selector: string, value: string) => {
      const field = container.querySelector(selector);
      expect(field).toBeInstanceOf(HTMLElement);
      await act(async () => {
        const prototype = field instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
        Object.getOwnPropertyDescriptor(prototype, 'value')?.set?.call(field, value);
        field?.dispatchEvent(new Event('input', { bubbles: true }));
        field?.dispatchEvent(new Event('change', { bubbles: true }));
      });
    };

    await setValue('#new-guidance-title', 'Refund policy');
    await setValue('#new-guidance-patterns', 'Can I get a refund?\nDo you offer refunds?');
    await setValue('#new-guidance-answer', 'Refunds are available within 14 days.');

    expect(mutations.create).not.toHaveBeenCalled();

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Save guidance'));
    expect(saveButton?.disabled).toBe(false);
    await act(async () => {
      saveButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(mutations.create).toHaveBeenCalledWith({
      agentId: 'empty-agent',
      payload: {
        title: 'Refund policy',
        question_patterns: ['Can I get a refund?', 'Do you offer refunds?'],
        answer: 'Refunds are available within 14 days.',
        language: '',
        intent: 'unknown',
        topics: [],
      },
    });
  });

  it('asks before discarding a changed inline editor', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="empty-agent" />);
    });

    const addButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Add guidance'));
    await act(async () => {
      addButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    const title = container.querySelector('#new-guidance-title');
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(title, 'Refund policy');
      title?.dispatchEvent(new Event('input', { bubbles: true }));
    });

    const cancelButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Cancel');
    await act(async () => {
      cancelButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(document.body.textContent).toContain('Discard unsaved changes?');
    expect(container.textContent).toContain('New answer guidance');
  });
});
