// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AskAgentsDock } from '../AskAgentsDock';
import { PageContextProvider } from '@/components/command-bar/pageContext';
import { TooltipProvider } from '@/components/ui/tooltip';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCommandBarRunStore } from '@/stores/commandBarStore';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  listPlans: vi.fn(),
  getPlan: vi.fn(),
  chatTurn: vi.fn(),
  listChatThreads: vi.fn(),
  dispatchPlan: vi.fn(),
  cancelPlan: vi.fn(),
  resumePlan: vi.fn(),
  retryPlan: vi.fn(),
  confirmChatCreateAgent: vi.fn(),
  listRecentRuns: vi.fn(),
  getRun: vi.fn(),
  getRunSnapshot: vi.fn(),
  listRunEvents: vi.fn(),
  approveRun: vi.fn(),
  cancelRun: vi.fn(),
}));

vi.mock('@/lib/services/commandBarService', () => ({
  commandBarService: {
    listPlans: mocks.listPlans,
    getPlan: mocks.getPlan,
    chatTurn: mocks.chatTurn,
    listChatThreads: mocks.listChatThreads,
    dispatchPlan: mocks.dispatchPlan,
    cancelPlan: mocks.cancelPlan,
    resumePlan: mocks.resumePlan,
    retryPlan: mocks.retryPlan,
    confirmChatCreateAgent: mocks.confirmChatCreateAgent,
  },
}));

vi.mock('@/lib/services/agentService', () => ({
  agentService: {
    listRecentRuns: mocks.listRecentRuns,
    getRun: mocks.getRun,
    getRunSnapshot: mocks.getRunSnapshot,
    listRunEvents: mocks.listRunEvents,
    approveRun: mocks.approveRun,
    cancelRun: mocks.cancelRun,
  },
}));

vi.mock('@/components/command-bar/PromotionDialog', () => ({
  PromotionDialog: () => null,
}));

vi.mock('@/components/pm/CodingSession/CodingSessionDrawer', () => ({
  CodingSessionDrawer: () => null,
}));

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  localStorage.clear();
  useCommandBarRunStore.getState().clear();
  useWorkspaceStore.setState({
    currentWorkspace: { id: 'ws-1', name: 'Acme' } as never,
  });
  mocks.listPlans.mockResolvedValue({ data: { plans: [] }, error: null });
  mocks.listRecentRuns.mockResolvedValue({ data: { runs: [] }, error: null });
  mocks.listChatThreads.mockResolvedValue({ data: { threads: [] }, error: null });
  mocks.getRunSnapshot.mockResolvedValue({ data: { stream_state_snapshot: null }, error: null });
  mocks.listRunEvents.mockResolvedValue({ data: { events: [], next_sequence_no: 0 }, error: null });
  mocks.chatTurn.mockResolvedValue({ data: null, error: null });
  mocks.dispatchPlan.mockResolvedValue({ data: null, error: null });
  mocks.confirmChatCreateAgent.mockResolvedValue({ data: { agent: { id: 'agent-1' } }, error: null });
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
  document.body.innerHTML = '';
  useWorkspaceStore.setState({ currentWorkspace: null });
  useCommandBarRunStore.getState().clear();
  vi.clearAllMocks();
});

async function renderDock() {
  await act(async () => {
    root.render(
      <TooltipProvider>
        <PageContextProvider>
          <AskAgentsDock />
        </PageContextProvider>
      </TooltipProvider>,
    );
  });
  await flush();
}

async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
}

async function waitForText(text: string) {
  for (let i = 0; i < 10; i += 1) {
    if (document.body.textContent?.includes(text)) return;
    await flush();
  }
  throw new Error(`Missing text: ${text}`);
}

async function waitForDockClass(className: string) {
  for (let i = 0; i < 10; i += 1) {
    const dock = document.body.querySelector('[data-helpin-dock="true"]');
    if (dock?.classList.contains(className)) return dock;
    await flush();
  }
  throw new Error(`Missing dock class: ${className}`);
}

function setTextareaValue(value: string) {
  const textarea = document.body.querySelector('textarea');
  if (!textarea) throw new Error('textarea not found');
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')?.set;
  setter?.call(textarea, value);
  textarea.dispatchEvent(new Event('input', { bubbles: true }));
}

async function clickSend() {
  const button = document.body.querySelector<HTMLButtonElement>('button[title="Send"]');
  if (!button) throw new Error('send button not found');
  await act(async () => {
    button.click();
  });
}

describe('AskAgentsDock chat', () => {
  it('does not show active runs from the previous workspace after switching workspaces', async () => {
    mocks.listRecentRuns
      .mockResolvedValueOnce({
        data: {
          runs: [{
            id: 'run-ws-1',
            workspace_id: 'ws-1',
            agent_id: 'agent-command',
            target_type: 'workspace',
            target_id: 'ws-1',
            runtime_kind: 'native_sdk',
            invocation_mode: 'autonomous',
            approval_state: 'approved',
            pause_reason: 'none',
            status: 'running',
            input: { text: 'summarize workspace one' },
            output_summary: {},
            cached_input_tokens: 0,
            input_tokens: 0,
            output_tokens: 0,
            tokens_used: 0,
            created_at: '2026-05-15T00:00:05Z',
            updated_at: '2026-05-15T00:01:00Z',
            target_info: { target_type: 'workspace', target_id: 'ws-1', title: 'Acme' },
          }],
        },
        error: null,
      })
      .mockResolvedValueOnce({ data: { runs: [] }, error: null });

    await renderDock();
    await waitForText('1 running');

    await act(async () => {
      useWorkspaceStore.setState({
        currentWorkspace: { id: 'ws-2', name: 'Beta' } as never,
      });
    });
    await flush();

    expect(document.body.textContent).not.toContain('1 running');
    expect(document.body.textContent).not.toContain('summarize workspace one');
  });

  it('smoothly hides while a dialog is open', async () => {
    await renderDock();
    expect(document.body.querySelector('[data-helpin-dock="true"]')).toBeTruthy();

    const dialog = document.createElement('div');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('data-state', 'open');
    dialog.setAttribute('data-slot', 'dialog-content');
    document.body.appendChild(dialog);

    const dock = await waitForDockClass('opacity-0');
    expect(dock.classList.contains('pointer-events-none')).toBe(true);
    expect(dock.classList.contains('translate-y-4')).toBe(true);
    expect(document.body.querySelector('textarea')).toBeTruthy();
  });

  it('smoothly hides while a sheet drawer is open', async () => {
    await renderDock();

    const sheet = document.createElement('div');
    sheet.setAttribute('role', 'dialog');
    sheet.setAttribute('data-state', 'open');
    sheet.setAttribute('data-slot', 'sheet-content');
    sheet.setAttribute('data-side', 'right');
    document.body.appendChild(sheet);

    const dock = await waitForDockClass('opacity-0');
    expect(dock.classList.contains('pointer-events-none')).toBe(true);
    expect(dock.classList.contains('translate-y-4')).toBe(true);
    expect(document.body.querySelector('textarea')).toBeTruthy();
  });

  it('hydrates the latest chat thread', async () => {
    mocks.listChatThreads.mockResolvedValue({
      data: {
        threads: [{
          thread: {
            id: 'thread-1',
            workspace_id: 'ws-1',
            title: 'Last chat',
            status: 'open',
            created_at: '2026-05-15T00:00:00Z',
            updated_at: '2026-05-15T00:00:01Z',
          },
          messages: [{
            id: 'msg-1',
            thread_id: 'thread-1',
            role: 'assistant',
            content: 'Hydrated answer',
            created_at: '2026-05-15T00:00:01Z',
          }],
        }],
      },
      error: null,
    });

    await renderDock();

    await waitForText('Hydrated answer');
  });

  it('hydrates completed runs that belong to the latest chat thread', async () => {
    mocks.listChatThreads.mockResolvedValue({
      data: {
        threads: [{
          thread: {
            id: 'thread-1',
            workspace_id: 'ws-1',
            title: 'Run chat',
            status: 'open',
            created_at: '2026-05-15T00:00:00Z',
            updated_at: '2026-05-15T00:02:00Z',
          },
          messages: [
            {
              id: 'user-msg',
              thread_id: 'thread-1',
              role: 'user',
              content: 'update USE-239',
              created_at: '2026-05-15T00:00:00Z',
            },
            {
              id: 'assistant-msg',
              thread_id: 'thread-1',
              role: 'assistant',
              content: 'Approve the one-shot run.',
              created_at: '2026-05-15T00:00:01Z',
            },
          ],
        }],
      },
      error: null,
    });
    mocks.listPlans.mockResolvedValue({
      data: {
        plans: [{
          id: 'plan-1',
          status: 'completed',
          plan_kind: 'one_shot_command',
          prompt: 'update USE-239',
          page_context: { entity_type: 'workspace', entity_id: 'ws-1', display_title: 'Acme' },
          steps: [{
            agent_id: 'agent-command',
            agent_name: 'Command Agent',
            plan_kind: 'one_shot_command',
            target: { entity_type: 'task', entity_id: 'task-1', display_title: 'USE-239' },
            instructions: 'update USE-239',
          }],
          run_ids_by_step: { 0: 'run-1' },
          current_step_index: 0,
          run_count: 1,
          created_at: '2026-05-15T00:00:05Z',
          updated_at: '2026-05-15T00:01:00Z',
          runs: [{
            id: 'run-1',
            workspace_id: 'ws-1',
            agent_id: 'agent-command',
            target_type: 'task',
            target_id: 'task-1',
            runtime_kind: 'native_sdk',
            invocation_mode: 'autonomous',
            approval_state: 'approved',
            pause_reason: 'none',
            status: 'completed',
            input: { text: 'update USE-239' },
            output_summary: { summary: 'Updated USE-239 with the latest findings.' },
            cached_input_tokens: 0,
            input_tokens: 0,
            output_tokens: 0,
            tokens_used: 0,
            created_at: '2026-05-15T00:00:05Z',
            updated_at: '2026-05-15T00:01:00Z',
            completed_at: '2026-05-15T00:01:00Z',
            target_info: { target_type: 'task', target_id: 'task-1', title: 'USE-239' },
          }],
        }],
      },
      error: null,
    });

    await renderDock();

    await waitForText('Updated USE-239 with the latest findings.');
  });

  it('renders inline answers without dispatching a plan', async () => {
    mocks.chatTurn.mockResolvedValue({
      data: {
        thread: {
          id: 'thread-1',
          workspace_id: 'ws-1',
          title: 'Docs',
          status: 'open',
          created_at: '2026-05-15T00:00:00Z',
          updated_at: '2026-05-15T00:00:01Z',
        },
        user_message: {
          id: 'user-msg',
          thread_id: 'thread-1',
          role: 'user',
          content: 'list docs',
          created_at: '2026-05-15T00:00:00Z',
        },
        assistant_message: {
          id: 'assistant-msg',
          thread_id: 'thread-1',
          role: 'assistant',
          content: 'I found 2 visible documents.',
          proposal: { type: 'inline_answer', answer: 'I found 2 visible documents.' },
          created_at: '2026-05-15T00:00:01Z',
        },
        proposal: { type: 'inline_answer', answer: 'I found 2 visible documents.' },
      },
      error: null,
    });
    await renderDock();

    await act(async () => {
      setTextareaValue('list docs');
    });
    await clickSend();
    await waitForText('I found 2 visible documents.');

    expect(mocks.dispatchPlan).not.toHaveBeenCalled();
  });

  it('renders and confirms a custom agent proposal', async () => {
    mocks.listChatThreads.mockResolvedValue({
      data: {
        threads: [{
          thread: {
            id: 'thread-1',
            workspace_id: 'ws-1',
            title: 'Agent',
            status: 'open',
            created_at: '2026-05-15T00:00:00Z',
            updated_at: '2026-05-15T00:00:01Z',
          },
          messages: [{
            id: 'proposal-msg',
            thread_id: 'thread-1',
            role: 'assistant',
            content: 'Review the draft before approving.',
            proposal: {
              type: 'create_agent',
              draft: {
                name: 'Doc Reviewer',
                role: 'Review documentation for gaps.',
                runtime_kind: 'native_sdk',
                allowed_tools: ['read_document'],
                allowed_targets: ['document'],
                approval_mode: 'always',
                default_invocation_mode: 'interactive',
                max_concurrent_runs: 1,
                system_prompt: 'Review docs.',
                skills: [],
              },
            },
            created_at: '2026-05-15T00:00:01Z',
          }],
        }],
      },
      error: null,
    });
    await renderDock();
    await waitForText('Doc Reviewer');

    const button = [...document.body.querySelectorAll('button')]
      .find((candidate) => candidate.textContent?.includes('Create agent'));
    expect(button).toBeTruthy();
    await act(async () => {
      button?.click();
    });

    expect(mocks.confirmChatCreateAgent).toHaveBeenCalledWith('ws-1', 'proposal-msg');
  });
});
