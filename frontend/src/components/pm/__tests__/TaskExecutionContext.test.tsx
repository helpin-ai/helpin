// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it, vi } from 'vitest';
import { AgentRunExecutionContext } from '../AgentRunPanel';
import { TooltipProvider } from '@/components/ui/tooltip';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

it('keeps unconfigured execution context visible and opens its settings', async () => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const edit = vi.fn();
  try {
    await act(async () => root.render(<TooltipProvider><AgentRunExecutionContext delivery={{ target: null, loading: false, selectedRepository: null, resolvedBaseBranch: 'main', branchPreview: 'task-preview' }} onEdit={edit} /></TooltipProvider>));
    expect(container.textContent).toContain('Execution context');
    expect(container.textContent).toContain('Not configured');
    expect(container.textContent).not.toContain('main');
    await act(async () => container.querySelector('button')?.click());
    expect(edit).toHaveBeenCalledOnce();
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});

it('prevents editing when context is locked and preserves configured values', async () => {
  const container = document.createElement('div');
  const root = createRoot(container);
  const edit = vi.fn();
  try {
    await act(async () => root.render(<TooltipProvider><AgentRunExecutionContext delivery={{ target: null, loading: false, selectedRepository: { full_name: 'acme/app' } as never, resolvedBaseBranch: 'develop', branchPreview: 'task-42' }} onEdit={edit} editReason="Wait for the active run to finish." /></TooltipProvider>));
    expect(container.textContent).toContain('acme/app');
    expect(container.textContent).toContain('develop');
    expect(container.textContent).toContain('task-42');
    expect(container.querySelector('button')?.disabled).toBe(true);
    await act(async () => container.querySelector('button')?.click());
    expect(edit).not.toHaveBeenCalled();
  } finally {
    await act(async () => root.unmount());
  }
});
