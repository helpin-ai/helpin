// @vitest-environment jsdom
import { act, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { queryKeys } from '@/lib/queryKeys';
import { startVoiceRecording } from '@/lib/voiceRecording';
import { transcribeVoice } from '@/lib/services/voiceInputService';
import { DockInput } from '../DockInput';

vi.mock('@/lib/voiceRecording', async importOriginal => ({
  ...await importOriginal<typeof import('@/lib/voiceRecording')>(), startVoiceRecording: vi.fn(),
}));
vi.mock('@/lib/services/voiceInputService', () => ({ voiceInputAvailable: vi.fn().mockResolvedValue(true), transcribeVoice: vi.fn() }));

it('keeps edits made during transcription and requires an explicit send afterwards', async () => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(queryKeys.voiceInput('ws'), true);
  const container = document.createElement('div'); document.body.append(container);
  const root = createRoot(container);
  const onSubmit = vi.fn();
  let setDraft!: (value: string) => void;
  let resolve!: (text: string) => void;
  vi.mocked(startVoiceRecording).mockResolvedValue({ cancel: vi.fn(), stop: vi.fn().mockResolvedValue(new Blob(['audio'])) });
  vi.mocked(transcribeVoice).mockImplementation(() => new Promise(r => { resolve = r; }));
  function Composer() {
    const [value, setValue] = useState('Original draft.'); setDraft = setValue;
    return <DockInput mode="conversation" pageContext={null} workspaceId="ws" value={value} onChange={setValue} onSubmit={onSubmit} />;
  }
  try {
    await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider><Composer /></TooltipProvider></QueryClientProvider>));
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Dictate message"]')!.click());
    const textarea = container.querySelector('textarea')!;
    act(() => textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    expect(onSubmit).not.toHaveBeenCalled();
    await act(async () => Array.from(container.querySelectorAll('button')).find(b => b.textContent === 'Stop')!.click());
    expect(container.textContent).toContain('Transcribing…');
    act(() => { setDraft('Edited while waiting.'); textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); });
    expect(onSubmit).not.toHaveBeenCalled();
    await act(async () => resolve('New speech.'));
    expect(textarea.value).toBe('Edited while waiting. New speech.');
    expect(onSubmit).not.toHaveBeenCalled();
    act(() => textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  } finally { act(() => root.unmount()); client.clear(); container.remove(); }
});
