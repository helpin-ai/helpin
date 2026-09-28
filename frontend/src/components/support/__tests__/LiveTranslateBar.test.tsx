// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { LiveTranslateBar } from '../LiveTranslateBar';

const mocks = vi.hoisted(() => ({ save: vi.fn(), mode: 'on' }));
vi.mock('@/hooks/queries/useSupportTranslation', () => ({
  setLiveTranslate: mocks.save,
  translationOptionsKey: () => ['support', 'ws', 'translation', 'conv'],
  useSupportTranslationOptions: () => ({ data: {
    available: true, conversation: { customer_language: '', translation_mode: mocks.mode },
    detected_customer_language: 'zh-CN', preference: { reading_language: 'en' },
    languages: { 'zh-CN': 'Chinese (Simplified)', en: 'English' },
  } }),
}));
(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const cleanups: (() => void)[] = [];
afterEach(() => { cleanups.splice(0).forEach(cleanup => cleanup()); mocks.save.mockReset(); mocks.mode = 'on'; });
function render(editable = true) {
  const container = document.createElement('div'); document.body.appendChild(container);
  const root = createRoot(container); const client = new QueryClient();
  act(() => root.render(<QueryClientProvider client={client}><TooltipProvider>
    <LiveTranslateBar workspaceId="ws" conversationId="conv" editable={editable} />
  </TooltipProvider></QueryClientProvider>));
  cleanups.push(() => { act(() => root.unmount()); client.clear(); container.remove(); });
  return container;
}
it.each(['on', 'off'])('uses an accessible %s action that preserves automatic language detection', async mode => {
  mocks.mode = mode;
  mocks.save.mockResolvedValue({});
  const container = render();
  const label = mode === 'on' ? 'Pause live translation' : 'Resume live translation';
  const action = container.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
  expect(action).not.toBeNull();
  expect(container.querySelector('[role="switch"]')).toBeNull();
  expect(container.textContent).toContain('Live translate from');
  expect(container.textContent).not.toContain('↔');
  expect(container.querySelector('select')?.selectedOptions[0].textContent).toContain('Chinese (Simplified) (detected)');
  await act(async () => { action!.click(); });
  expect(mocks.save).toHaveBeenCalledWith('ws', 'conv', mode !== 'on', '');
});
it('keeps read-only controls disabled', () => {
  const container = render(false);
  expect(container.querySelector<HTMLSelectElement>('select')?.disabled).toBe(true);
  expect(container.querySelector<HTMLButtonElement>('button[aria-label="Pause live translation"]')?.disabled).toBe(true);
});
it('preserves the translation state when changing the customer language', async () => {
  mocks.mode = 'off'; mocks.save.mockResolvedValue({});
  const container = render();
  await act(async () => {
    const select = container.querySelector('select')!;
    select.value = 'zh-CN'; select.dispatchEvent(new Event('change', { bubbles: true }));
  });
  expect(mocks.save).toHaveBeenCalledWith('ws', 'conv', false, 'zh-CN');
});
