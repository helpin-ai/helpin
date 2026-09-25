// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TaskDraftSuggestions } from './TaskDraftSuggestions';
import { pmTriageService, type TriageView } from '@/lib/services/pmTriageService';

vi.mock('@/lib/services/pmTriageService', () => ({ pmTriageService: { analyzeDraft: vi.fn() } }));
let container: HTMLDivElement;
let root: Root;
const pending = vi.fn();
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
  await act(async () => root.render(<TaskDraftSuggestions workspaceId="workspace" workspaceSlug="workspace" name={name} description="Export fails" teamId={teamId} taskType="bug" disabled={false} onPending={pending} onApply={apply} onOpenTask={open} />));
}
async function debounce() { await act(async () => { await vi.advanceTimersByTimeAsync(700); }); }
beforeEach(() => {
  vi.useFakeTimers(); vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true); vi.clearAllMocks();
  container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); vi.useRealTimers(); vi.unstubAllGlobals(); });
describe('Task draft matching', () => {
  it('debounces edits and holds creation until the current request finishes', async () => {
    const result = deferred(); vi.mocked(pmTriageService.analyzeDraft).mockReturnValue(result.promise);
    await render('Invoice'); await render('Invoice export failure');
    expect(pending).toHaveBeenLastCalledWith(true);
    expect(pmTriageService.analyzeDraft).not.toHaveBeenCalled();
    await debounce();
    expect(pmTriageService.analyzeDraft).toHaveBeenCalledTimes(1);
    expect(pmTriageService.analyzeDraft).toHaveBeenCalledWith('workspace', expect.objectContaining({ name: 'Invoice export failure', team_id: 'payments' }));
    await act(async () => result.resolve({ error: null, data: ready('Existing export bug') }));
    expect(pending).toHaveBeenLastCalledWith(false);
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
    expect(pending).toHaveBeenLastCalledWith(true);
    await act(async () => currentResult.resolve({ error: null, data: ready('Current team task') }));
    expect(container.textContent).toContain('Current team task');
    expect(pending).toHaveBeenLastCalledWith(false);
  });
  it('releases creation and preserves manual saving when matching fails', async () => {
    vi.mocked(pmTriageService.analyzeDraft).mockRejectedValue(new Error('Matching unavailable'));
    await render(); await debounce();
    expect(pending).toHaveBeenLastCalledWith(false);
    expect(container.textContent).toContain('You can still save this task.');
  });
  it('cancels pending matching when the title is cleared', async () => {
    const result = deferred(); vi.mocked(pmTriageService.analyzeDraft).mockReturnValue(result.promise);
    await render(); await debounce(); await render('');
    expect(pending).toHaveBeenLastCalledWith(false);
    await act(async () => result.resolve({ error: null, data: ready('Obsolete suggestion') }));
    expect(container.textContent).toBe('');
    expect(apply).not.toHaveBeenCalled();
  });
});
