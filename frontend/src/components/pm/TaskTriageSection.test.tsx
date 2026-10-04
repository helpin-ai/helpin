// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { TaskTriageSection } from './TaskTriageSection';
import { pmTriageService, type TriageView } from '@/lib/services/pmTriageService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import type { TaskDetail } from '@/lib/pmTypes';

vi.mock('@/lib/services/pmTriageService', () => ({ pmTriageService: { analyze: vi.fn(), review: vi.fn() } }));
vi.mock('@/lib/services/pmTaskService', () => ({ pmTaskService: { get: vi.fn() } }));
vi.mock('@/stores/authStore', () => ({ useAuthStore: (select: (state: { user: { id: string } }) => unknown) => select({ user: { id: 'actor' } }) }));
let container: HTMLDivElement;
let root: Root;
let client: QueryClient;
const updated = vi.fn();
const detail = { task: { id: 'task', updated_at: 'revision', task_type: 'feature', team_id: 'team' } } as TaskDetail;
const ready = (): TriageView => ({ id: 'assessment', status: 'ready', source_kind: 'task', source_id: 'task', teams: [], labels: [], candidates: [], assessment: { actionable: true, task_type: { id: 'bug', probability: .99 }, labels: [], matches: [], candidates_checked: 0 } });
async function render(disabled = false) {
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider><TaskTriageSection workspaceId="workspace" workspaceSlug="workspace" detail={detail} disabled={disabled} onTaskUpdated={updated} /></TooltipProvider></QueryClientProvider>));
  await settle();
}
async function settle() { await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); }); }
function button(text: string) { return [...container.querySelectorAll('button')].find((item) => item.textContent === text)!; }
beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true); vi.resetAllMocks();
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
  vi.mocked(pmTriageService.analyze).mockResolvedValue({ error: null, data: ready() });
});
afterEach(() => { act(() => root.unmount()); client.clear(); container.remove(); vi.unstubAllGlobals(); });
describe('Task detail suggestion review', () => {
  it('keeps assessment details in a tooltip without linking to the current task', async () => {
    const view = ready();
    view.assessment!.actionable = false;
    view.assessment!.task_type = undefined;
    view.assessment!.candidates_checked = 8;
    vi.mocked(pmTriageService.analyze).mockResolvedValue({ error: null, data: view });
    await render();
    expect(container.textContent).toContain('No suggestions to review.');
    expect(container.textContent).not.toContain('Compared with');
    expect(container.textContent).not.toContain('concrete product work');
    expect(container.querySelector('a[href="/w/workspace/pm/tasks/task"]')).toBeNull();
    const info = container.querySelector<HTMLButtonElement>('button[aria-label="About task suggestions"]')!;
    await act(async () => info.focus());
    expect(document.querySelector('[role="tooltip"]')?.textContent).toContain('Based on this task’s saved title and description.');
    expect(document.querySelector('[role="tooltip"]')?.textContent).toContain('Compared with 8 accessible tasks. Other matches may exist.');
  });
  it('prevents assessment and review while unsaved edits exist', async () => {
    await render(true);
    expect(pmTriageService.analyze).not.toHaveBeenCalled();
    expect(container.textContent).toContain('Save your changes');
    expect(button('Refresh').disabled).toBe(true);
    await render();
    expect(container.textContent).toContain('Type: bug');
    await render(true);
    expect(button('Apply type').disabled).toBe(true);
    expect(pmTriageService.review).not.toHaveBeenCalled();
  });
  it('shows loading until the assessment arrives', async () => {
    let resolve!: (value: { data: TriageView; error: null }) => void;
    vi.mocked(pmTriageService.analyze).mockReturnValue(new Promise((done) => { resolve = done; }));
    await render();
    expect(container.textContent).toContain('Checking task suggestions');
    expect(button('Refresh').disabled).toBe(true);
    await act(async () => resolve({ error: null, data: ready() })); await settle();
    expect(container.textContent).toContain('Type: bug');
  });
  it.each(['disabled', 'shadow'] as const)('hides %s assessments', async (status) => {
    vi.mocked(pmTriageService.analyze).mockResolvedValue({ error: null, data: { ...ready(), status } });
    await render(); expect(container.textContent).toBe('');
  });
  it('retains the suggestion when a stale review is rejected and allows refresh', async () => {
    vi.mocked(pmTriageService.review).mockRejectedValue(new Error('Task changed. Refresh suggestions.'));
    await render(); await act(async () => button('Apply type').click()); await settle();
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Task changed');
    expect(container.textContent).toContain('Type: bug');
    expect(updated).not.toHaveBeenCalled();
    await act(async () => button('Refresh').click()); await settle();
    expect(container.querySelector('[role="alert"]')).toBeNull();
    expect(pmTriageService.analyze).toHaveBeenCalledTimes(2);
  });
  it('refreshes task data after explicit acceptance', async () => {
    vi.mocked(pmTriageService.review).mockResolvedValue({ error: null, data: { key: 'task_type:bug', status: 'accepted' } });
    vi.mocked(pmTaskService.get).mockResolvedValue({ error: null, data: detail });
    await render(); await act(async () => button('Apply type').click()); await settle();
    expect(pmTriageService.review).toHaveBeenCalledWith('workspace', 'task', 'task', { assessment_id: 'assessment', action: 'task_type', value: 'bug', dismiss: false });
    expect(updated).toHaveBeenCalledWith(detail);
    expect(container.textContent).not.toContain('Type: bug');
  });
});
