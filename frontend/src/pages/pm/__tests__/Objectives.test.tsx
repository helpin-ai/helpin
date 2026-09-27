// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { ObjectivesPage } from '../Objectives';
import { ObjectiveDetailPage } from '../ObjectiveDetail';
import type { ObjectiveWithDetails } from '@/lib/pmTypes';

const context = vi.hoisted(() => ({
  workspace: { id: 'ws', slug: 'acme' },
  permissions: { canEdit: true, isAdmin: true, isTeamManager: () => true, canManageTeam: () => true, teamMemberships: [{ team_id: 'team', role: 'owner' }] },
  members: [{ id: 'alice', user_id: 'alice-user', display_name: 'Alice', email: 'alice@example.com', status: 'active' }, { id: 'bob', user_id: 'bob-user', display_name: 'Bob', email: 'bob@example.com', status: 'active' }],
}));
vi.mock('@/stores/workspaceStore', () => ({ useWorkspaceStore: (selector: (state: unknown) => unknown) => selector({ currentWorkspace: context.workspace }) }));
vi.mock('@/hooks/queries/useSession', () => ({ useWorkspaceAccess: () => ({ data: {} }), usePermissions: () => context.permissions }));
vi.mock('@/hooks/queries', () => ({ useWorkspaceAccess: () => ({ data: {} }), usePermissions: () => context.permissions, useWorkspaceMemberPresenceMap: () => ({ data: new Map() }) }));
vi.mock('@/hooks/useAssignableWorkspaceMembers', () => ({ useAssignableWorkspaceMembers: () => ({ members: context.members }) }));
vi.mock('@/hooks/useAccessibleTeams', () => ({ useAccessibleTeams: () => ({ teams: [{ id: 'team', name: 'Product' }] }) }));
vi.mock('@/components/notifications/FollowButton', () => ({ FollowButton: () => <button>Follow</button> }));
vi.mock('@/components/pm/Attachments', () => ({ Attachments: () => <div>Attachments</div> }));
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => async () => true }));
vi.mock('@/components/ui/tiptap-editor', () => ({ TiptapEditor: ({ content, onChange }: { content: string; onChange: (html: string) => void }) => <textarea aria-label="Description editor" value={content} onChange={event => onChange(event.target.value)} /> }));
vi.mock('@/components/pm/RichTextMentionContent', () => ({ RichTextMentionContent: ({ html }: { html: string }) => <div>{html}</div> }));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let records: ObjectiveWithDetails[];
function objective(id: string, name: string, owner: string): ObjectiveWithDetails {
  return {
    objective: { id, workspace_id: 'ws', name, objective_type: 'strategic', state: 'active', health: 'on_track', position: 0, archived: false, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' },
    teams: ['team'], owners: [owner], owner_member_ids: [owner], labels: [], epics: [], key_results: [], suggested_health: 'on_track',
    stats: { key_result_count: 0, key_result_avg_pct: 0, epic_count: 0, epic_done_count: 0, epic_task_count: 0, epic_done_tasks: 0, epic_progress_pct: 0 },
  };
}
const response = <T,>(data: T) => ({ data, error: null, status: 200 });
const settle = () => new Promise(resolve => setTimeout(resolve, 25));
beforeEach(() => {
  vi.spyOn(pmObjectiveService, 'keyResultActivity').mockResolvedValue(response({ data: [], total: 0, page: 1, per_page: 20, total_pages: 0 }));
  records = [objective('one', 'Launch platform', 'alice'), objective('two', 'Improve retention', 'bob')];
  context.permissions.canEdit = true; context.permissions.isAdmin = true;
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
  vi.spyOn(pmObjectiveService, 'list').mockImplementation(async (_ws, filters) => response(records.filter(record => !filters?.state || record.objective.state === filters.state)));
  vi.spyOn(pmObjectiveService, 'get').mockImplementation(async (_ws, id) => response(records.find(record => record.objective.id === id)!));
  vi.spyOn(pmObjectiveService, 'update').mockImplementation(async (_ws, id, patch) => {
    records = records.map(record => record.objective.id === id ? { ...record, objective: { ...record.objective, ...patch } } : record);
    return response(records.find(record => record.objective.id === id)!);
  });
});
afterEach(() => { act(() => root?.unmount()); document.body.innerHTML = ''; vi.restoreAllMocks(); vi.unstubAllGlobals(); });

async function render(path = '/w/acme/pm/objectives') {
  const rootRoute = createRootRoute({ component: Outlet });
  const auth = createRoute({ getParentRoute: () => rootRoute, id: '_authenticated', component: Outlet });
  const workspace = createRoute({ getParentRoute: () => auth, path: '/w/$slug', component: Outlet });
  const index = createRoute({ getParentRoute: () => workspace, path: '/pm/objectives', component: ObjectivesPage });
  const detail = createRoute({ getParentRoute: () => workspace, path: '/pm/objectives/$objectiveId', component: () => {
    const { objectiveId } = detail.useParams();
    return <ObjectiveDetailPage key={objectiveId} />;
  } });
  const router = createRouter({ routeTree: rootRoute.addChildren([auth.addChildren([workspace.addChildren([index, detail])])]), history: createMemoryHistory({ initialEntries: [path] }) });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } } });
  const container = document.createElement('div'); document.body.append(container); root = createRoot(container);
  await act(async () => { await router.load(); root.render(<QueryClientProvider client={client}><TooltipProvider><RouterProvider router={router} /></TooltipProvider></QueryClientProvider>); });
  await act(settle);
  return { container, router, client };
}
async function change(input: HTMLInputElement | HTMLTextAreaElement, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(input instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype, 'value')!.set!.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

function button(container: HTMLElement, text: string) { return [...container.querySelectorAll('button')].find(element => element.textContent === text)!; }

describe('Objectives redesign', () => {
  it('keeps cards and synchronizes owner avatars, search, and editable filter pills', async () => {
    const { container } = await render();
    expect(container.querySelectorAll('article')).toHaveLength(2);
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Filter by owner Alice"]')!.click());
    expect(container.querySelectorAll('article')).toHaveLength(1);
    expect(container.querySelector('[aria-label="Remove Owner filter"]')).not.toBeNull();
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Filter by owner Bob"]')!.click());
    expect(container.querySelectorAll('article')).toHaveLength(2);
    await change(container.querySelector<HTMLInputElement>('[aria-label="Search objectives"]')!, 'LAUNCH');
    expect(container.querySelectorAll('article')).toHaveLength(1);
    await act(async () => button(container, 'Clear all').click());
    expect(container.querySelectorAll('article')).toHaveLength(2);
    expect(container.querySelector('[aria-label="Remove Owner filter"]')).toBeNull();
    expect(container.querySelector<HTMLInputElement>('[aria-label="Search objectives"]')!.value).toBe('');
  });

  it('navigates to a full detail page and flushes a title edit before returning to updated cards', async () => {
    const { container, router } = await render();
    await act(async () => container.querySelector<HTMLAnchorElement>('a[href="/w/acme/pm/objectives/one"]')!.click());
    await act(settle);
    expect(router.state.location.pathname).toBe('/w/acme/pm/objectives/one');
    expect(container.querySelectorAll('article')).toHaveLength(0);
    expect(document.querySelector('[data-slot="sheet-content"]')).toBeNull();
    expect(container.textContent).toContain('Key Results');
    await change(container.querySelector<HTMLInputElement>('[aria-label="Objective title"]')!, 'Launch revised');
    await act(async () => button(container, 'Objectives').click());
    await act(settle);
    expect(pmObjectiveService.update).toHaveBeenCalledWith('ws', 'one', { name: 'Launch revised' });
    expect(router.state.location.pathname).toBe('/w/acme/pm/objectives');
    expect(container.textContent).toContain('Launch revised');
  });

  it('retains the page and draft when saving before navigation fails, then retries', async () => {
    const { container, router } = await render('/w/acme/pm/objectives/one');
    vi.mocked(pmObjectiveService.update).mockResolvedValueOnce({ data: null, error: 'Offline', status: 500 });
    await change(container.querySelector<HTMLInputElement>('[aria-label="Objective title"]')!, 'Unsaved title');
    await act(async () => button(container, 'Objectives').click());
    await act(settle);
    expect(router.state.location.pathname).toBe('/w/acme/pm/objectives/one');
    expect(container.querySelector<HTMLInputElement>('[aria-label="Objective title"]')!.value).toBe('Unsaved title');
    expect(container.textContent).toContain('Offline');
    await act(async () => button(container, 'Retry').click());
    await act(settle);
    expect(container.textContent).not.toContain('Offline');
    expect(pmObjectiveService.update).toHaveBeenCalledTimes(2);
  });

  it('keeps the header and filters visible when loading fails', async () => {
    vi.mocked(pmObjectiveService.list).mockResolvedValue({ data: null, error: 'Unavailable', status: 500 });
    const { container } = await render();
    expect(container.querySelector('h1')?.textContent).toBe('Objectives');
    expect(container.querySelector('[aria-label="Search objectives"]')).not.toBeNull();
    expect(container.textContent).toContain('Couldn’t load objectives');
  });

  it('makes key-result values and completion understandable without technical type labels', async () => {
    records[0].key_results = [
      { id: 'activation', objective_id: 'one', name: 'Increase activation', result_type: 'percent', initial_value: 20, current_value: 35, target_value: 50, progress: 50, position: 0, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' },
      { id: 'launch', objective_id: 'one', name: 'Launch onboarding', result_type: 'boolean', initial_value: 0, current_value: 0, target_value: 1, progress: 0, position: 1, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' },
    ];
    const update = vi.spyOn(pmObjectiveService, 'updateKeyResult').mockImplementation(async (_ws, id, patch) => {
      const updated = { ...records[0].key_results.find(kr => kr.id === id)!, ...patch, progress: 100 };
      records[0] = { ...records[0], key_results: records[0].key_results.map(kr => kr.id === id ? updated : kr) };
      return response(updated);
    });
    const { container } = await render('/w/acme/pm/objectives/one');
    const section = container.querySelector('[aria-label="Key results"]')!;
    expect(section).not.toBeNull();
    expect(section.textContent).toContain('Current');
    expect(section.textContent).toContain('Target');
    expect(section.textContent).toContain('50%');
    expect(section.textContent).toContain('At start');
    expect(section.textContent).toContain('20%');
    expect(section.querySelector('input')).toBeNull();
    expect(section.textContent).not.toContain('boolean');
    expect(section.textContent).not.toContain('outcome progress');
    expect(section.textContent).toContain('50% achieved');
    expect(section.querySelector('[role="progressbar"]')).toBeNull();
    await act(async () => section.querySelector<HTMLButtonElement>('[aria-label="Log result for Launch onboarding"]')!.click());
    const dialog = document.querySelector<HTMLElement>('[role="dialog"]')!;
    await act(async () => dialog.querySelector<HTMLButtonElement>('[role="checkbox"]')!.click());
    await act(async () => button(dialog, 'Log result').click());
    expect(update).toHaveBeenCalledWith('ws', 'launch', { current_value: 1 });
    expect(section.textContent).toContain('Complete');
  });

  it('keeps key-result values readable and removes editing controls for viewers', async () => {
    records[0].key_results = [{ id: 'kr', objective_id: 'one', name: 'Reduce support wait time', result_type: 'numeric', initial_value: 60, current_value: 30, target_value: 10, progress: 60, position: 0, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' }];
    context.permissions.canEdit = false; context.permissions.isAdmin = false;
    const { container } = await render('/w/acme/pm/objectives/one');
    const section = container.querySelector('[aria-label="Key results"]')!;
    expect(section).not.toBeNull();
    expect(section.textContent).toContain('Current');
    expect(section.textContent).toContain('30');
    expect(section.textContent).toContain('Target');
    expect(section.textContent).toContain('10');
    expect(section.querySelector('input')).toBeNull();
    expect(section.querySelector('[aria-label^="Delete key result"]')).toBeNull();
    expect(section.textContent).toContain('60% achieved');
    expect(section.querySelector('[aria-label^="Log result"]')).toBeNull();
  });

  it('opens a visible editor for the full key result and retains changes after a failed save', async () => {
    const kr = { id: 'kr', objective_id: 'one', name: 'Activation', result_type: 'percent' as const, initial_value: 20, current_value: 35, target_value: 50, progress: 50, position: 0, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' };
    records[0].key_results = [kr];
    const update = vi.spyOn(pmObjectiveService, 'updateKeyResult').mockResolvedValueOnce({ data: null, error: 'Could not save changes', status: 500 }).mockImplementation(async (_ws, _id, patch) => {
      const updated = { ...kr, ...patch };
      records[0] = { ...records[0], key_results: [updated] };
      return response(updated);
    });
    const { container } = await render('/w/acme/pm/objectives/one');
    const edit = container.querySelector<HTMLButtonElement>('[aria-label="Edit key result Activation"]');
    expect(edit).not.toBeNull();
    await act(async () => edit!.click());
    const dialog = () => document.querySelector<HTMLElement>('[role="dialog"]')!;
    expect(dialog().textContent).toContain('Edit key result');
    expect(dialog().querySelector('#key-result-current')).toBeNull();
    expect(dialog().querySelector<HTMLInputElement>('#key-result-target')!.value).toBe('50');
    await change(dialog().querySelector<HTMLInputElement>('#key-result-name')!, 'Increase activation');
    await change(dialog().querySelector<HTMLInputElement>('#key-result-target')!, '60');
    await act(async () => button(dialog(), 'Save changes').click());
    expect(dialog().textContent).toContain('Could not save changes');
    expect(dialog().querySelector<HTMLInputElement>('#key-result-target')!.value).toBe('60');
    await act(async () => button(dialog(), 'Save changes').click());
    expect(update).toHaveBeenLastCalledWith('ws', 'kr', { name: 'Increase activation', result_type: 'percent', initial_value: 20, target_value: 60 });
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    expect(container.textContent).toContain('Increase activation');
  });

  it('logs results only through a dialog and preserves the entered value after failure', async () => {
    const kr = { id: 'kr', objective_id: 'one', name: 'Activation', result_type: 'numeric' as const, initial_value: 20, current_value: 35, target_value: 50, progress: 50, position: 0, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' };
    records[0].key_results = [kr];
    const update = vi.spyOn(pmObjectiveService, 'updateKeyResult').mockResolvedValueOnce({ data: null, error: 'Cannot save result', status: 500 }).mockImplementation(async (_ws, _id, patch) => {
      const updated = { ...kr, ...patch };
      records[0] = { ...records[0], key_results: [updated] };
      return response(updated);
    });
    const { container } = await render('/w/acme/pm/objectives/one');
    expect(container.querySelector('[aria-label="Current value for Activation"]')).toBeNull();
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Log result for Activation"]')!.click());
    const dialog = () => document.querySelector<HTMLElement>('[role="dialog"]')!;
    expect(dialog().textContent).not.toContain('increment');
    await change(dialog().querySelector<HTMLInputElement>('#key-result-value')!, '45');
    expect(update).not.toHaveBeenCalled();
    await act(async () => button(dialog(), 'Log result').click());
    expect(dialog().textContent).toContain('Cannot save result');
    expect(dialog().querySelector<HTMLInputElement>('#key-result-value')!.value).toBe('45');
    await act(async () => button(dialog(), 'Log result').click());
    expect(update).toHaveBeenLastCalledWith('ws', 'kr', { current_value: 45 });
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    expect(container.textContent).toContain('45');
  });

  it('shows actual value history with earlier entries collapsed and paginated', async () => {
    records[0].key_results = [{ id: 'kr', objective_id: 'one', name: 'Signups', result_type: 'numeric', initial_value: 13, current_value: 18, target_value: 100, progress: 6, position: 0, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-27T00:00:00Z' }];
    const entry = (id: string, value: string) => ({ activity: { id, workspace_id: 'ws', entity_type: 'key_result', entity_id: 'kr', actor_id: 'alice-user', action: 'updated', field_name: 'current_value', new_value: value, created_at: '2026-09-26T12:00:00Z' } });
    vi.mocked(pmObjectiveService.keyResultActivity).mockImplementation(async (_ws, _id, page) => response({ data: page === 1 ? [entry('a', '18'), entry('b', '16')] : [entry('c', '14')], total: 3, page: page ?? 1, per_page: 2, total_pages: 2 }));
    const { container } = await render('/w/acme/pm/objectives/one');
    expect(container.textContent).toContain('Alice changed the current value to 18');
    expect(container.textContent).not.toContain('changed the current value to 16');
    await act(async () => button(container, 'Show 2 earlier updates').click());
    expect(container.textContent).toContain('changed the current value to 16');
    await act(async () => button(container, 'Show more updates').click());
    await act(settle);
    expect(container.textContent).toContain('changed the current value to 14');
    await act(async () => button(container, 'Hide earlier updates').click());
    expect(container.textContent).not.toContain('changed the current value to 14');
  });

  it('keeps existing status filtering single-select when edited from a pill', async () => {
    const { container } = await render();
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Status: Status"]')!.click());
    expect(document.querySelector('[cmdk-item][data-value="active"] svg')?.getAttribute('class')).toContain('text-amber-500');
    await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="active"]')!.click());
    await act(settle);
    const pill = () => container.querySelector('[aria-label="Remove Status filter"]')!.parentElement!;
    expect(pill().textContent).toContain('In Progress');
    await act(async () => pill().querySelector('button')!.click());
    expect(document.querySelector('[cmdk-item][data-value="closed"] svg')?.getAttribute('class')).toContain('text-green-500');
    await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="closed"]')!.click());
    await act(settle);
    expect(pill().textContent).toContain('Done');
    expect(pill().textContent).not.toContain('2 selected');
    expect(container.querySelectorAll('article')).toHaveLength(0);
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Remove Status filter"]')!.click());
    await act(settle);
    expect(container.querySelectorAll('article')).toHaveLength(2);
  });

  it('preserves health colors in toolbar and applied-filter menus', async () => {
    const { container } = await render();
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Health: Health"]')!.click());
    expect(document.querySelector('[cmdk-item][data-value="at_risk"]')?.className).toContain('text-yellow-600');
    await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="at_risk"]')!.click());
    await act(settle);
    const pill = container.querySelector('[aria-label="Remove Health filter"]')!.parentElement!;
    await act(async () => pill.querySelector('button')!.click());
    expect(document.querySelector('[cmdk-item][data-value="off_track"] .text-red-600')?.textContent).toBe('Off Track');
  });

  it('restores progress bars and uses the shared description cancel/done flow', async () => {
    records[0].objective.description = '<p>Original description</p>';
    records[0].objective.planned_start_date = '2026-01-01';
    records[0].objective.deadline = '2026-12-31';
    records[0].stats.epic_task_count = 10;
    records[0].stats.epic_done_tasks = 6;
    const { container } = await render('/w/acme/pm/objectives/one');
    expect(container.querySelector('[aria-label="Back to objectives"]')).not.toBeNull();
    expect(container.querySelector('[aria-label="Epic progress"]')?.getAttribute('aria-valuenow')).toBe('60');
    expect(container.querySelector('[aria-label="Time elapsed toward target date"]')).not.toBeNull();
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Edit description"]')!.click());
    await change(container.querySelector<HTMLTextAreaElement>('[aria-label="Description editor"]')!, '<p>Changed description</p>');
    await act(async () => button(container, 'Cancel').click());
    expect(container.querySelector('[aria-label="Description editor"]')).toBeNull();
    expect(container.textContent).toContain('Original description');
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Edit description"]')!.click());
    await change(container.querySelector<HTMLTextAreaElement>('[aria-label="Description editor"]')!, '<p>Final description</p>');
    await act(async () => button(container, 'Done').click());
    await act(async () => button(container, 'Objectives').click());
    await act(settle);
    expect(pmObjectiveService.update).toHaveBeenLastCalledWith('ws', 'one', { description: '<p>Final description</p>' });
  });

  it('updates health through the colored property dropdown', async () => {
    const { container } = await render('/w/acme/pm/objectives/one');
    await act(async () => button(container, 'On track').click());
    const option = document.querySelector<HTMLElement>('[cmdk-item][data-value="at_risk"]')!;
    expect(option.textContent).toContain('At risk');
    await act(async () => option.click());
    await act(async () => button(container, 'Objectives').click());
    await act(settle);
    expect(pmObjectiveService.update).toHaveBeenCalledWith('ws', 'one', { health: 'at_risk' });
  });

  it('keeps original description images available for Cancel after autosave', async () => {
    const original = '<p>Original</p><img data-attachment-id="image-1" src="/image.png">';
    records[0].objective.description = original;
    const remove = vi.spyOn(pmAttachmentService, 'remove').mockResolvedValue(response(null));
    const { container } = await render('/w/acme/pm/objectives/one');
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Edit description"]')!.click());
    await change(container.querySelector<HTMLTextAreaElement>('[aria-label="Description editor"]')!, '<p>Without image</p>');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 750)); });
    await act(settle);
    expect(pmObjectiveService.update).toHaveBeenCalledWith('ws', 'one', { description: '<p>Without image</p>' });
    expect(remove).not.toHaveBeenCalled();
    await act(async () => button(container, 'Cancel').click());
    await act(async () => button(container, 'Objectives').click());
    await act(settle);
    expect(records[0].objective.description).toBe(original);
    expect(remove).not.toHaveBeenCalled();
  });

  it('preserves read-only details and starts fresh when navigating between objectives', async () => {
    context.permissions.canEdit = false; context.permissions.isAdmin = false;
    const { container, router } = await render('/w/acme/pm/objectives/one');
    expect(container.querySelector('[aria-label="Objective title"]')).toBeNull();
    expect(container.textContent).toContain('Launch platform');
    expect(container.textContent).not.toContain('Delete objective');
    await act(async () => router.navigate({ to: '/w/$slug/pm/objectives/$objectiveId', params: { slug: 'acme', objectiveId: 'two' } }));
    await act(settle);
    expect(container.textContent).toContain('Improve retention');
    expect(container.textContent).not.toContain('Launch platform');
  });
});
