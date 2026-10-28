// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { beforeEach, afterEach, expect, it, vi } from 'vitest';
import type { ComponentProps } from 'react';
import type { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { SupportCreateTaskModal } from '../SupportCreateTaskModal';

const mocks = vi.hoisted(() => ({
  analyze: vi.fn(), draft: vi.fn(), create: vi.fn(), preferences: vi.fn(), review: vi.fn(),
  linkedTeam: 'marketing', inbox: 'inbox-1', defaultTeam: 'engineering',
  modal: null as ComponentProps<typeof CreateTaskModal> | null,
}));
vi.mock('@/components/pm/CreateTaskModal', () => ({ CreateTaskModal: (props: ComponentProps<typeof CreateTaskModal>) => {
  mocks.modal = props;
  return props.renderPreparation ? <div>{props.renderPreparation({ error: '', retry: vi.fn(), cancel: () => props.onOpenChange(false) })}</div> : <div data-task-form>Create task{props.preparationContent}</div>;
} }));
vi.mock('@edition', () => ({ UpgradeRequiredDialog: () => null }));
vi.mock('@/components/ui/dialog', () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  DialogDescription: ({ children }: { children: React.ReactNode }) => <p>{children}</p>,
}));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
vi.mock('@tanstack/react-query', () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));
vi.mock('@/hooks/queries/useSupport', () => ({
  useConversation: () => ({ data: { mailbox_id: mocks.inbox } }),
  useInboxScopes: () => ({ data: { mailboxes: [{ id: mocks.inbox, linked_team_id: mocks.linkedTeam }] } }),
  useCreateTaskFromConversation: () => ({ mutateAsync: mocks.create }),
}));
vi.mock('@/hooks/queries/useSession', () => ({
  useWorkspaceAccess: () => ({ data: { membership: { support_default_team_id: mocks.defaultTeam, support_task_team_by_inbox: { 'inbox-1': 'marketing', 'inbox-2': 'engineering' } } } }),
  useUpdateSupportTaskPreferences: () => ({ mutate: mocks.preferences }),
}));
vi.mock('@/hooks/useAccessibleTeams', () => ({ useAccessibleTeams: () => ({ teams: [{ id: 'engineering' }, { id: 'marketing' }], loading: false }) }));
vi.mock('@/lib/services/pmTriageService', () => ({ pmTriageService: { analyze: mocks.analyze, draft: mocks.draft, review: mocks.review } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let container: HTMLDivElement;
const draft = { name: 'Campaign brief', description: '<p>Publish a guide</p>', task_type: 'chore' as const, priority: 'low' as const, source_hash: 'source-1' };
beforeEach(() => {
  vi.clearAllMocks(); mocks.modal = null; mocks.linkedTeam = 'marketing'; mocks.inbox = 'inbox-1'; mocks.defaultTeam = 'engineering';
  mocks.analyze.mockResolvedValue({ data: { status: 'ready', source_kind: 'support_conversation', assessment: { matches: [] }, candidates: [] } });
  mocks.draft.mockResolvedValue({ data: draft });
  mocks.create.mockResolvedValue({ task_id: 'task-1', task_name: draft.name });
  container = document.createElement('div'); document.body.append(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render(onOpenChange = vi.fn()) { await act(async () => { root.render(<SupportCreateTaskModal workspaceId="ws" workspaceSlug="acme" conversationId="conversation" onOpenChange={onOpenChange} />); }); }
it('opens before matching finishes and then prepares a draft automatically with the linked team', async () => {
  let resolve!: (value: unknown) => void;
  mocks.analyze.mockReturnValue(new Promise(r => { resolve = r; }));
  await render();
  expect(container.textContent).toContain('Checking for existing tasks');
  expect(container.querySelector('[data-task-form]')).toBeNull();
  expect(container.textContent).not.toContain('Conversation and team loaded');
  expect(mocks.modal?.initialTeamId).toBe('marketing');
  expect(mocks.modal?.prepareDraft).toBeUndefined();
  await act(async () => resolve({ data: { status: 'ready', source_kind: 'support_conversation', assessment: { matches: [] }, candidates: [] } }));
  expect(mocks.modal?.prepareDraft).toBeTypeOf('function');
  await mocks.modal?.prepareDraft?.('marketing');
  expect(mocks.draft).toHaveBeenCalledWith('ws', 'conversation', 'marketing');
  expect(container.querySelector('[data-task-form]')).toBeNull();
  await act(async () => { mocks.modal?.onDraftPrepared?.(draft, 'marketing'); });
  expect(container.querySelector('[data-task-form]')).not.toBeNull();
});
it('shows matches without drafting until the user chooses to create new', async () => {
  mocks.analyze.mockResolvedValue({ data: { id: 'assessment', status: 'ready', source_kind: 'support_conversation', assessment: { matches: [{ task_id: 'existing', relationship: 'duplicates' }] }, candidates: [{ id: 'existing', name: 'Existing campaign' }] } });
  await render();
  expect(container.textContent).toContain('Existing campaign');
  expect(container.querySelector('[data-task-form]')).toBeNull();
  expect(mocks.modal?.prepareDraft).toBeUndefined();
  await act(async () => { Array.from(container.querySelectorAll('button')).find(b => b.textContent === 'Create new task')?.click(); });
  expect(mocks.modal?.prepareDraft).toBeTypeOf('function');
});
it('reports failed matching instead of treating it as no matches', async () => {
  mocks.analyze.mockRejectedValue(new Error('offline'));
  await render();
  expect(container.textContent).toContain('Couldn’t check existing tasks');
  expect(mocks.modal?.creationBlocked).toBe(true);
  expect(mocks.modal?.prepareDraft).toBeUndefined();
});
it('uses a remembered team per inbox when there is no linked team', async () => {
  mocks.linkedTeam = ''; mocks.inbox = 'inbox-2'; await render();
  expect(mocks.modal?.initialTeamId).toBe('engineering');
});
it('falls back to the current person’s saved team when the inbox has no preference', async () => {
  mocks.linkedTeam = ''; mocks.inbox = 'new-inbox'; mocks.defaultTeam = 'marketing'; await render();
  expect(mocks.modal?.initialTeamId).toBe('marketing');
});
it('links an existing task from preparation without creating a new task', async () => {
  mocks.analyze.mockResolvedValue({ data: { id: 'assessment', status: 'ready', source_kind: 'support_conversation', assessment: { matches: [{ task_id: 'existing', relationship: 'duplicates' }] }, candidates: [{ id: 'existing', display_id: 42, name: 'Existing campaign' }] } });
  mocks.review.mockResolvedValue({ data: { key: 'match:existing', status: 'accepted' } });
  const close = vi.fn();
  await render(close);
  await act(async () => { Array.from(container.querySelectorAll('button')).find(b => b.textContent === 'Link conversation')!.click(); });
  expect(mocks.review).toHaveBeenCalledWith('ws', 'support_conversation', 'conversation', { assessment_id: 'assessment', action: 'match', value: 'existing', dismiss: false });
  expect(close).toHaveBeenCalledWith(false);
  expect(mocks.create).not.toHaveBeenCalled();
  expect(mocks.draft).not.toHaveBeenCalled();
});
it('continues preparation after dismissing the last existing-task match', async () => {
  mocks.analyze.mockResolvedValue({ data: { id: 'assessment', status: 'ready', source_kind: 'support_conversation', assessment: { matches: [{ task_id: 'existing', relationship: 'duplicates' }] }, candidates: [{ id: 'existing', display_id: 42, name: 'Existing campaign' }] } });
  mocks.review.mockResolvedValue({ data: { key: 'match:existing', status: 'dismissed' } });
  await render();
  await act(async () => { Array.from(container.querySelectorAll('button')).find(b => b.textContent === 'Dismiss')!.click(); });
  expect(mocks.modal?.prepareDraft).toBeTypeOf('function');
  expect(container.textContent).toContain('Preparing task details');
  expect(container.querySelector('[data-task-form]')).toBeNull();
});
it('saves reviewed fields, multiple assignees and agent options and remembers only after success', async () => {
  await render();
  await act(async () => { mocks.modal?.onDraftPrepared?.(draft, 'marketing'); });
  const payload = { name: 'Edited title', description: '<p>Edited brief</p>', team_id: 'marketing', owner_member_ids: ['a', 'b'], assigned_agent_id: 'agent', run_on_create: true };
  mocks.create.mockRejectedValueOnce(new Error('failed'));
  await expect(mocks.modal?.onCreate(payload as never)).rejects.toThrow('failed');
  expect(mocks.preferences).not.toHaveBeenCalled();
  await expect(mocks.modal?.onCreate({ ...payload, team_id: 'engineering' } as never)).rejects.toThrow('Wait for the draft');
  await act(async () => { await mocks.modal?.onCreate(payload as never); });
  expect(mocks.create).toHaveBeenCalledWith({ conversationId: 'conversation', teamId: 'marketing', draft: { ...payload, reviewed_draft: true, source_hash: 'source-1' } });
  expect(mocks.preferences).toHaveBeenCalledWith({ support_default_team_id: 'marketing', support_task_inbox_id: 'inbox-1' }, expect.anything());
  expect(mocks.modal?.creationBlocked).toBe(true);
  await expect(mocks.modal?.onCreate(payload as never)).rejects.toThrow('already been created');
});
