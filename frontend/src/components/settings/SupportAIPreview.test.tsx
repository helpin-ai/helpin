// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, it, vi } from 'vitest';
import { SupportAIPreview } from './SupportAIPreview';
import { agentService } from '@/lib/services/agentService';

vi.mock('@/lib/services/agentService', () => ({ agentService: {
  previewSupportReply: vi.fn(), getSupportPreview: vi.fn(), cancelSupportPreview: vi.fn(),
} }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
afterEach(() => { vi.clearAllMocks(); vi.useRealTimers(); });

it('waits for the captured runtime outcome and identifies a rejected answer', async () => {
  vi.useFakeTimers();
  const pending = { run_id: 'run', final_decision: 'pending', retrieval: { results: [] } };
  vi.mocked(agentService.previewSupportReply).mockResolvedValue({ data: pending, error: null } as never);
  vi.mocked(agentService.getSupportPreview).mockResolvedValue({ data: { ...pending,
    provider: 'openai_compatible', model: 'local-model', final_decision: 'handoff', final_reason: 'low_confidence',
    answer: { content: 'Unverified claim', can_answer: false },
  }, error: null } as never);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => root.render(<QueryClientProvider client={client}><SupportAIPreview workspaceId="workspace" agentId="agent" /></QueryClientProvider>));
    expect(container.querySelector('textarea')).toBeNull();
    const trigger = container.querySelector<HTMLButtonElement>('button')!;
    expect(trigger.textContent).toBe('Test AI response');
    await act(async () => trigger.click());
    const dialog = document.querySelector<HTMLElement>('[role="dialog"]')!;
    expect(dialog).not.toBeNull();
    const textarea = dialog.querySelector('textarea')!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(textarea, 'Customer question');
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await act(async () => dialog.querySelector('button')!.click());
    for (let i = 0; i < 4; i++) await act(async () => { await vi.advanceTimersByTimeAsync(100); });
    expect(agentService.previewSupportReply).toHaveBeenCalledWith('workspace', 'agent', { message: 'Customer question' });
    expect(agentService.getSupportPreview).toHaveBeenCalledWith('workspace', 'agent', 'run');
    expect(dialog.textContent).toContain('would not be sent');
    expect(dialog.textContent).toContain('Unverified claim');
    expect(dialog.textContent).toContain('openai_compatible / local-model');
    expect(dialog.querySelector('button')!.disabled).toBe(false);
    await act(async () => dialog.querySelector<HTMLButtonElement>('[data-slot="dialog-close"]')!.click());
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    await act(async () => trigger.click());
    expect(document.querySelector('textarea')?.value).toBe('Customer question');
    expect(document.querySelector('[role="dialog"]')?.textContent).toContain('Unverified claim');
    expect(agentService.previewSupportReply).toHaveBeenCalledTimes(1);
  } finally {
    await act(async () => root.unmount());
    client.clear(); container.remove();
  }
});
