// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { TriageSuggestions } from './TriageSuggestions';
import type { TriageView } from '@/lib/services/pmTriageService';

let container: HTMLDivElement;
let root: Root;
const onReview = vi.fn();
const view = (): TriageView => ({
  id: 'assessment', status: 'ready', source_kind: 'task', source_id: 'source',
  teams: [{ id: 'payments', name: 'Payments' }], labels: [],
  candidates: [{ id: 'match', name: 'Invoice export fails', display_id: 42 }],
  assessment: { actionable: true, task_type: { id: 'bug', probability: .99 }, team: { id: 'payments', probability: .98 }, labels: [], matches: [{ task_id: 'match', relationship: 'duplicates', probability: .99 }], candidates_checked: 4 },
});
beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  onReview.mockReset();
  container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); vi.unstubAllGlobals(); });
async function render(value = view(), disabled = false) {
  await act(async () => root.render(<TriageSuggestions view={value} disabled={disabled} currentType="feature" currentTeam="other" taskHref={(id) => `/tasks/${id}`} onReview={onReview} />));
}
describe('Triage suggestion review', () => {
  it('requires an explicit click and sends the exact saved suggestion', async () => {
    await render();
    expect(onReview).not.toHaveBeenCalled();
    const button = [...container.querySelectorAll('button')].find((item) => item.textContent === 'Add relationship')!;
    await act(async () => button.click());
    expect(onReview).toHaveBeenCalledWith({ assessment_id: 'assessment', action: 'match', value: 'match', dismiss: false });
    expect(container.querySelector('a')?.getAttribute('href')).toBe('/tasks/match');
    expect(container.textContent).toContain('Compared with 4 accessible tasks. Other matches may exist.');
  });
  it('persists the distinction between dismissal and acceptance', async () => {
    await render();
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="Dismiss Type: bug"]')!.click());
    expect(onReview).toHaveBeenCalledWith({ assessment_id: 'assessment', action: 'task_type', value: 'bug', dismiss: true });
    const dismissed = view(); dismissed.reviewed = { 'task_type:bug': 'dismissed', 'match:match': 'accepted' };
    await render(dismissed);
    expect(container.textContent).not.toContain('Type: bug');
    expect(container.textContent).not.toContain('Invoice export fails');
    expect(container.textContent).toContain('Team: Payments');
  });
  it('disables mutations while source edits or another review are pending', async () => {
    await render(view(), true);
    for (const button of container.querySelectorAll('button')) { expect(button.disabled).toBe(true); button.click(); }
    expect(onReview).not.toHaveBeenCalled();
  });
  it('offers support linking without changing a task type or team', async () => {
    const support = view(); support.source_kind = 'support_conversation';
    await render(support);
    expect(container.textContent).toContain('Link conversation');
    expect(container.textContent).not.toContain('Apply type');
    expect(container.textContent).not.toContain('Change team');
  });
  it('never renders a match without authorized candidate metadata', async () => {
    const inaccessible = view(); inaccessible.candidates = [];
    await render(inaccessible);
    expect(container.textContent).not.toContain('Possible duplicate');
  });
});
