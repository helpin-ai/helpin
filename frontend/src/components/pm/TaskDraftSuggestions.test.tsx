// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TaskDraftSuggestions } from './TaskDraftSuggestions';
import { pmTriageService, type TriageView } from '@/lib/services/pmTriageService';

vi.mock('@/lib/services/pmTriageService', () => ({ pmTriageService: { analyzeDraft: vi.fn() } }));
let container: HTMLDivElement;
let root: Root;
const apply = vi.fn();
const open = vi.fn();
const ready = (name: string): TriageView => ({
  id: 'assessment', status: 'ready', source_kind: 'task_draft', source_id: 'draft', teams: [], labels: [],
  candidates: [{ id: name, name, display_id: 1 }],
  assessment: { actionable: true, labels: [], matches: [{ task_id: name, relationship: 'duplicates', probability: .99 }], candidates_checked: 1 },
});
function deferred() {
  let resolve!: (value: { data: TriageView; error: null }) => void;
  const promise = new Promise<{ data: TriageView; error: null }>((done) => { resolve = done; });
  return { promise, resolve };
}
async function render(name = 'Invoice failure', teamId = 'payments') {
  await act(async () => root.render(<TaskDraftSuggestions workspaceId="workspace" workspaceSlug="workspace" name={name} description="Export fails" teamId={teamId} taskType="bug" disabled={false} onApply={apply} onOpenTask={open} />));
}
async function debounce() { await act(async () => { await vi.advanceTimersByTimeAsync(700); }); }
beforeEach(() => {
  vi.useFakeTimers(); vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true); vi.clearAllMocks();
  container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); vi.useRealTimers(); vi.unstubAllGlobals(); });
describe('Task draft matching', () => {
  it('debounces edits and waits for the current request before showing suggestions', async () => {
    const result = deferred(); vi.mocked(pmTriageService.analyzeDraft).mockReturnValue(result.promise);
    await render('Invoice'); await render('Invoice export failure');
    expect(pmTriageService.analyzeDraft).not.toHaveBeenCalled();
    expect(container.textContent).toBe('');
    await debounce();
    expect(pmTriageService.analyzeDraft).toHaveBeenCalledTimes(1);
    expect(pmTriageService.analyzeDraft).toHaveBeenCalledWith('workspace', expect.objectContaining({ name: 'Invoice export failure', team_id: 'payments' }));
    await act(async () => result.resolve({ error: null, data: ready('Existing export bug') }));
    expect(container.textContent).toContain('Existing export bug');
    expect(open).not.toHaveBeenCalled();
    await act(async () => [...container.querySelectorAll('button')].find((button) => button.textContent === 'Use existing task')!.click());
    expect(open).toHaveBeenCalledWith('Existing export bug');
  });
  it('ignores an old response after the draft team changes', async () => {
    const oldResult = deferred(); const currentResult = deferred();
    vi.mocked(pmTriageService.analyzeDraft).mockReturnValueOnce(oldResult.promise).mockReturnValueOnce(currentResult.promise);
    await render(); await debounce(); await render('Invoice failure', 'new-team'); await debounce();
    await act(async () => oldResult.resolve({ error: null, data: ready('Old team task') }));
    expect(container.textContent).not.toContain('Old team task');
    await act(async () => currentResult.resolve({ error: null, data: ready('Current team task') }));
    expect(container.textContent).toContain('Current team task');
  });
  it('releases creation and preserves manual saving when matching fails', async () => {
    vi.mocked(pmTriageService.analyzeDraft).mockRejectedValue(new Error('Matching unavailable'));
    await render(); await debounce();
    expect(container.textContent).toBe('');
  });
  it('hides the entire section when there are no reviewable suggestions', async () => {
    const result = ready('Unavailable task');
    result.candidates = [];
    result.assessment!.task_type = { id: 'bug', probability: .99 };
    result.assessment!.team = { id: 'payments', probability: .99 };
    vi.mocked(pmTriageService.analyzeDraft).mockResolvedValue({ data: result, error: null });
    await render(); await debounce();
    expect(container.textContent).toBe('');
  });
  it('hides the section after the last suggestion is dismissed', async () => {
    vi.mocked(pmTriageService.analyzeDraft).mockResolvedValue({ data: ready('Existing task'), error: null });
    await render(); await debounce();
    expect(container.textContent).toContain('Existing work and suggestions');
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label^="Dismiss"]')!.click());
    expect(container.textContent).toBe('');
  });
  it('cancels pending matching when the title is cleared', async () => {
    const result = deferred(); vi.mocked(pmTriageService.analyzeDraft).mockReturnValue(result.promise);
    await render(); await debounce(); await render('');
    await act(async () => result.resolve({ error: null, data: ready('Obsolete suggestion') }));
    expect(container.textContent).toBe('');
    expect(apply).not.toHaveBeenCalled();
  });
});
