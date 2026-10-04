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

    expect(document.body.textContent).toContain('Current pricing');
    expect(document.body.textContent).toContain('Growth starts at $49 per month');
    expect(document.body.textContent).toContain('What is your pricing?');
    expect(document.body.textContent).toContain('Preferred answers');
    expect(document.body.textContent).toContain('Example questions');
  });

  it('explains when answer guidance is useful', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="empty-agent" />);
    });

    expect(document.body.textContent).toContain('No preferred answers yet');
    expect(document.body.textContent).toContain('pricing, refunds, plan limits, security, and company policies');
  });

  it('explains that guidance belongs to the selected support agent', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" />);
    });

    expect(document.body.textContent).toContain('Select a support agent first');
    expect(document.body.textContent).toContain('Choose an agent in the Setup tab');
  });

  it('opens the add form in a dialog', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="empty-agent" />);
    });

    const addButton = Array.from(document.querySelectorAll('button')).find((button) => button.textContent?.includes('Add preferred answer'));
    await act(async () => {
      addButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    const answer = document.querySelector('#new-guidance-answer');
    expect(document.body.textContent).toContain('New preferred answer');
    expect(document.body.textContent).not.toContain('Changes take effect after you save.');
    expect(document.querySelector('[role="dialog"]')).not.toBeNull();
    expect(answer).toBeInstanceOf(HTMLTextAreaElement);
    expect(answer?.className).toContain('field-sizing-content');
    expect(document.querySelector('#new-guidance-language')).toBeNull();
    const options = document.querySelector('details');
    expect(options?.querySelector('summary')?.textContent).toContain('Additional options');
    expect(options?.open).toBe(false);
  });

  it('edits existing guidance in a dialog', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="agent-1" />);
    });

    const editButton = document.querySelector('button[aria-label="Edit Current pricing"]');
    await act(async () => {
      editButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(document.body.textContent).toContain('Edit preferred answer');
    expect(document.querySelector('#edit-guidance-title')).toHaveProperty('value', 'Current pricing');
    expect(document.querySelector('#edit-guidance-answer')).toHaveProperty(
      'value',
      'Growth starts at $49 per month. Enterprise pricing is custom.',
    );
    expect(document.querySelector('[role="dialog"]')).not.toBeNull();
    expect(document.querySelector('#edit-guidance-language')).toBeNull();
    expect(document.querySelector('#edit-guidance-patterns')).toHaveProperty('value', 'What is your pricing?\nHow much does it cost?');
    const saveButton = document.querySelector<HTMLButtonElement>('button[type="submit"]')!;
    await act(async () => saveButton.click());
    expect(mutations.update).toHaveBeenCalledWith({
      agentId: 'agent-1', guidanceId: 'guidance-1',
      payload: {
        title: 'Current pricing', question_patterns: ['What is your pricing?', 'How much does it cost?'],
        answer: 'Growth starts at $49 per month. Enterprise pricing is custom.', status: 'active',
      },
    });
  });

  it('saves new guidance only after the explicit save action', async () => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="empty-agent" />);
    });

    const addButton = Array.from(document.querySelectorAll('button')).find((button) => button.textContent?.includes('Add preferred answer'));
    await act(async () => {
      addButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    const setValue = async (selector: string, value: string) => {
      const field = document.querySelector(selector);
      expect(field).toBeInstanceOf(HTMLElement);
      await act(async () => {
        const prototype = field instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
        Object.getOwnPropertyDescriptor(prototype, 'value')?.set?.call(field, value);
        field?.dispatchEvent(new Event('input', { bubbles: true }));
        field?.dispatchEvent(new Event('change', { bubbles: true }));
      });
    };

    await setValue('#new-guidance-title', 'Refund policy');
    await setValue('#new-guidance-answer', 'Refunds are available within 14 days.');

    expect(mutations.create).not.toHaveBeenCalled();

    const saveButton = Array.from(document.querySelectorAll('button')).find((button) => button.textContent?.includes('Save answer'));
    expect(saveButton?.disabled).toBe(false);
    await act(async () => {
      saveButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(document.querySelector('[role="dialog"]')).toBeNull();
    expect(mutations.create).toHaveBeenCalledWith({
      agentId: 'empty-agent',
      payload: {
        title: 'Refund policy',
        question_patterns: [],
        answer: 'Refunds are available within 14 days.',
        language: '',
        intent: 'unknown',
        topics: [],
      },
    });
  });

  it.each(['Cancel', 'Escape'])('protects a changed dialog when closing with %s', async (method) => {
    await act(async () => {
      root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="empty-agent" />);
    });

    const addButton = Array.from(document.querySelectorAll('button')).find((button) => button.textContent?.includes('Add preferred answer'));
    await act(async () => {
      addButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    const title = document.querySelector('#new-guidance-title');
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(title, 'Refund policy');
      title?.dispatchEvent(new Event('input', { bubbles: true }));
    });

    const cancelButton = Array.from(document.querySelectorAll('button')).find((button) => button.textContent === 'Cancel');
    await act(async () => {
      if (method === 'Escape') {
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      } else {
        cancelButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      }
    });

    expect(document.body.textContent).toContain('Discard unsaved changes?');
    expect(document.body.textContent).toContain('New preferred answer');
    const keepEditing = Array.from(document.querySelectorAll('button')).find((button) => button.textContent === 'Keep editing');
    await act(async () => keepEditing?.click());
    expect(document.querySelector('#new-guidance-title')).toHaveProperty('value', 'Refund policy');
    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
  });

  it('keeps the dialog and draft available when saving fails', async () => {
    mutations.update.mockRejectedValueOnce(new Error('Save failed'));
    await act(async () => root.render(<CuratedGuidanceField workspaceId="workspace-1" agentId="agent-1" />));
    await act(async () => document.querySelector<HTMLButtonElement>('button[aria-label="Edit Current pricing"]')?.click());
    await act(async () => document.querySelector<HTMLButtonElement>('button[type="submit"]')?.click());
    expect(document.querySelector('[role="dialog"]')).not.toBeNull();
    expect(document.querySelector('#edit-guidance-title')).toHaveProperty('value', 'Current pricing');
  });
});
