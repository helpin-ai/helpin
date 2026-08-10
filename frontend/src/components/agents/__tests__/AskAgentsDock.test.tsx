// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AskAgentsDock } from '../AskAgentsDock';
import { PageContextProvider } from '@/components/command-bar/pageContext';
import { TooltipProvider } from '@/components/ui/tooltip';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDockStore } from '@/stores/dockStore';
import type { DockChat, DockChatDetail, DockRunSummary } from '@/lib/dockTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  listChats: vi.fn(),
  createChat: vi.fn(),
  getChat: vi.fn(),
  updateChat: vi.fn(),
  sendMessage: vi.fn(),
  getChatRun: vi.fn(),
  listChatRunEvents: vi.fn(),
  listChatRunInteractions: vi.fn(),
  resolveInteraction: vi.fn(),
  cancelChatRun: vi.fn(),
  listRuns: vi.fn(),
  getRunSnapshot: vi.fn(),
  listRunEvents: vi.fn(),
  listRunInteractions: vi.fn(),
  resolveRunInteraction: vi.fn(),
  sendRunMessage: vi.fn(),
  continueRun: vi.fn(),
  cancelRun: vi.fn(),
  startRunAuth: vi.fn(),
  cancelRunAuth: vi.fn(),
  getPlan: vi.fn(),
}));

vi.mock('@/lib/services/dockChatService', () => ({
  dockChatService: {
    listChats: mocks.listChats,
    createChat: mocks.createChat,
    getChat: mocks.getChat,
    updateChat: mocks.updateChat,
    sendMessage: mocks.sendMessage,
    getChatRun: mocks.getChatRun,
    listChatRunEvents: mocks.listChatRunEvents,
    listChatRunInteractions: mocks.listChatRunInteractions,
    resolveInteraction: mocks.resolveInteraction,
    cancelChatRun: mocks.cancelChatRun,
    listRuns: mocks.listRuns,
    getRunSnapshot: mocks.getRunSnapshot,
    listRunEvents: mocks.listRunEvents,
    listRunInteractions: mocks.listRunInteractions,
    resolveRunInteraction: mocks.resolveRunInteraction,
    sendRunMessage: mocks.sendRunMessage,
    continueRun: mocks.continueRun,
    cancelRun: mocks.cancelRun,
    startRunAuth: mocks.startRunAuth,
    cancelRunAuth: mocks.cancelRunAuth,
  },
}));

vi.mock('@/lib/services/commandBarService', () => ({
  commandBarService: {
    getPlan: mocks.getPlan,
  },
}));

const CHAT: DockChat = {
  id: 'chat-1',
  workspace_id: 'ws-1',
  user_id: 'user-1',
  title: 'Sprint questions',
  active_run_id: null,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
};

const DOCK_RUN: DockRunSummary = {
  run: {
    id: 'agent-run-1',
    workspace_id: 'ws-1',
    agent_id: 'agent-review',
    target_type: 'task',
    target_id: 'task-42',
    target_info: { task_key: 'HLP-42', title: 'Polish the agent dock' },
    status: 'paused',
    pause_reason: 'human_approval',
    updated_at: '2026-08-09T12:00:00Z',
  } as never,
  agent: { id: 'agent-review', name: 'Review Agent', preset_key: 'task_reviewer' },
  attention_kind: 'approval',
  last_activity_at: '2026-08-09T12:00:00Z',
};

function chatDetail(overrides: Partial<DockChatDetail> = {}): DockChatDetail {
  return { chat: CHAT, run: null, plan_ids: [], ...overrides };
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  localStorage.clear();
  localStorage.setItem('helpin:agent-dock-selection:ws-1', JSON.stringify({ tab: 'chats', chatId: 'chat-1' }));
  useDockStore.setState({ collapsed: false, view: 'chat', tab: 'chats', workspaceId: null, activeChatId: null, activeRunId: null, chats: [], drafts: {}, lastAttentionIds: [] });
  useWorkspaceStore.setState({
    currentWorkspace: { id: 'ws-1', name: 'Acme' } as never,
  });
  mocks.listChats.mockResolvedValue({ data: { chats: [CHAT] }, error: null });
  mocks.listRuns.mockResolvedValue({ data: { runs: [], attention_count: 0 }, error: null });
  mocks.getChat.mockResolvedValue({ data: chatDetail(), error: null });
  mocks.getChatRun.mockResolvedValue({ data: null, error: null });
  mocks.getRunSnapshot.mockResolvedValue({
    data: { id: 'agent-run-1', status: 'paused', pause_reason: 'human_approval', stream_state_snapshot: null },
    error: null,
  });
  mocks.listRunEvents.mockResolvedValue({ data: { events: [], next_sequence_no: 0 }, error: null });
  mocks.listRunInteractions.mockResolvedValue({ data: { interactions: [] }, error: null });
  mocks.listChatRunEvents.mockResolvedValue({ data: { events: [], next_sequence_no: 0 }, error: null });
  mocks.listChatRunInteractions.mockResolvedValue({ data: { interactions: [] }, error: null });
  mocks.getPlan.mockResolvedValue({ data: null, error: null });
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
  document.body.innerHTML = '';
  useWorkspaceStore.setState({ currentWorkspace: null });
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

function dockTextarea(): HTMLTextAreaElement {
  const textarea = document.body.querySelector('[data-helpin-dock] textarea');
  if (!textarea) throw new Error('dock textarea not found');
  return textarea as HTMLTextAreaElement;
}

function setTextareaValue(textarea: HTMLTextAreaElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')?.set;
  setter?.call(textarea, value);
  textarea.dispatchEvent(new Event('input', { bubbles: true }));
}

describe('AskAgentsDock', () => {
  it('renders the collapsed pill and expands via the / key', async () => {
    useDockStore.setState({ collapsed: true });
    await renderDock();
    await waitForText('Ask agents');

    await act(async () => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }));
    });
    await waitForText('Sprint questions');
    expect(useDockStore.getState().collapsed).toBe(false);
  });

  it('loads chats and shows the active chat view with an enabled composer', async () => {
    await renderDock();
    await waitForText('Sprint questions');
    expect(mocks.listChats).toHaveBeenCalled();
    expect(mocks.getChat).toHaveBeenCalledWith('ws-1', 'chat-1');
    const textarea = dockTextarea();
    expect(textarea.disabled).toBe(false);
  });

  it('sends a message through the dock chat service', async () => {
    mocks.sendMessage.mockResolvedValue({
      data: chatDetail({
        chat: { ...CHAT, active_run_id: 'run-1' },
        run: { id: 'run-1', status: 'running', pause_reason: 'none' } as never,
      }),
      error: null,
    });
    await renderDock();
    await waitForText('Sprint questions');

    const textarea = dockTextarea();
    await act(async () => {
      setTextareaValue(textarea, 'list open tasks');
    });
    await act(async () => {
      textarea.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }),
      );
    });
    await flush();

    expect(mocks.sendMessage).toHaveBeenCalledWith(
      'ws-1',
      'chat-1',
      expect.objectContaining({ content: 'list open tasks' }),
    );
  });

  it('shows a stop button for an active run and cancels through the chat service', async () => {
    mocks.getChat.mockResolvedValue({
      data: chatDetail({
        chat: { ...CHAT, active_run_id: 'run-1' },
        run: { id: 'run-1', status: 'running', pause_reason: 'none' } as never,
      }),
      error: null,
    });
    mocks.getChatRun.mockResolvedValue({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: null },
      error: null,
    });
    mocks.cancelChatRun.mockResolvedValue({ data: null, error: null });

    await renderDock();
    await waitForText('Sprint questions');

    const stopButton = document.body.querySelector('[data-helpin-dock] [aria-label="Stop agent"]');
    expect(stopButton).not.toBeNull();
    await act(async () => {
      (stopButton as HTMLButtonElement).click();
    });
    await flush();

    expect(mocks.cancelChatRun).toHaveBeenCalledWith('ws-1', 'chat-1');
  });

  it('renders an inline error with retry when sending fails', async () => {
    mocks.sendMessage.mockResolvedValue({ data: null, error: 'network unreachable' });
    await renderDock();
    await waitForText('Sprint questions');

    const textarea = dockTextarea();
    await act(async () => {
      setTextareaValue(textarea, 'list open tasks');
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    await flush();

    await waitForText('network unreachable');
    const retry = Array.from(document.body.querySelectorAll('[data-helpin-dock] button'))
      .find((button) => button.textContent === 'Retry');
    expect(retry).not.toBeUndefined();

    mocks.sendMessage.mockResolvedValue({
      data: chatDetail({
        chat: { ...CHAT, active_run_id: 'run-1' },
        run: { id: 'run-1', status: 'running', pause_reason: 'none' } as never,
      }),
      error: null,
    });
    await act(async () => {
      (retry as HTMLButtonElement).click();
    });
    await flush();

    expect(mocks.sendMessage).toHaveBeenCalledTimes(2);
    expect(mocks.sendMessage).toHaveBeenLastCalledWith(
      'ws-1',
      'chat-1',
      expect.objectContaining({ content: 'list open tasks' }),
    );
  });

  it('reconciles immediately when a message continues on the same run', async () => {
    const run = { id: 'run-1', status: 'running', pause_reason: 'none' } as never;
    const detail = chatDetail({
      chat: { ...CHAT, active_run_id: 'run-1' },
      run,
    });
    mocks.getChat.mockResolvedValue({ data: detail, error: null });
    mocks.getChatRun.mockResolvedValue({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: null },
      error: null,
    });
    mocks.sendMessage.mockResolvedValue({ data: detail, error: null });

    await renderDock();
    await waitForText('Sprint questions');
    const snapshotCallsBeforeSend = mocks.getChatRun.mock.calls.length;
    const eventCallsBeforeSend = mocks.listChatRunEvents.mock.calls.length;

    const textarea = dockTextarea();
    await act(async () => {
      setTextareaValue(textarea, 'continue this run');
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    await flush();

    expect(mocks.getChatRun.mock.calls.length).toBeGreaterThan(snapshotCallsBeforeSend);
    expect(mocks.listChatRunEvents.mock.calls.length).toBeGreaterThan(eventCallsBeforeSend);
  });

  it('opens with a prefilled draft from the helpin:ask-agents event', async () => {
    useDockStore.setState({ collapsed: true });
    await renderDock();
    await act(async () => {
      window.dispatchEvent(
        new CustomEvent('helpin:ask-agents', { detail: { query: 'enrich this contact' } }),
      );
    });
    await waitForText('Sprint questions');
    await act(async () => {
      await new Promise((resolve) => window.setTimeout(resolve, 0));
    });
    expect(dockTextarea().value).toBe('enrich this contact');
  });

  it('switches to the chat list and back', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    const agentsButton = Array.from(document.body.querySelectorAll('[data-helpin-dock] button')).find(
      (b) => b.textContent === 'Agents',
    );
    expect(agentsButton).toBeTruthy();
    await act(async () => {
      (agentsButton as HTMLButtonElement).click();
    });
    expect(useDockStore.getState().tab).toBe('agents');

    const chatsButton = Array.from(document.body.querySelectorAll('[data-helpin-dock] button')).find(
      (b) => b.textContent === 'Chats',
    );
    await act(async () => {
      (chatsButton as HTMLButtonElement).click();
    });
    await waitForText('Sprint questions');
    expect(useDockStore.getState().tab).toBe('chats');
  });

  it('creates a new chat', async () => {
    const newChat: DockChat = { ...CHAT, id: 'chat-2', title: '' };
    mocks.createChat.mockResolvedValue({ data: newChat, error: null });
    mocks.getChat.mockImplementation((_: string, chatId: string) =>
      Promise.resolve({
        data: chatDetail({ chat: chatId === 'chat-2' ? newChat : CHAT }),
        error: null,
      }),
    );
    await renderDock();
    await waitForText('Sprint questions');

    const newButton = Array.from(document.body.querySelectorAll('[data-helpin-dock] button')).find(
      (b) => b.textContent?.includes('New chat or task'),
    );
    await act(async () => {
      (newButton as HTMLButtonElement).click();
    });
    await flush();
    expect(mocks.createChat).toHaveBeenCalledWith('ws-1');
    expect(useDockStore.getState().activeChatId).toBe('chat-2');
  });

  it('shows attention in the collapsed dock trigger', async () => {
    useDockStore.setState({ collapsed: true });
    mocks.listRuns.mockResolvedValue({ data: { runs: [DOCK_RUN], attention_count: 1 }, error: null });

    await renderDock();
    await waitForText('1 need you');

    expect(document.body.textContent).toContain('Ask agents');
    expect(document.body.textContent).toContain('1 agent need your attention');
  });

  it('renders and resolves an approval for a personal agent run', async () => {
    localStorage.setItem('helpin:agent-dock-selection:ws-1', JSON.stringify({ tab: 'agents', runId: 'agent-run-1' }));
    mocks.listRuns.mockResolvedValue({ data: { runs: [DOCK_RUN], attention_count: 1 }, error: null });
    mocks.listRunInteractions.mockResolvedValue({
      data: {
        interactions: [{
          id: 'run-int-1',
          interaction_kind: 'approval_request',
          status: 'pending',
          request_schema_version: '1',
          request_payload: { title: 'Approve agent action', raw_input: { action: { steps: [{ instructions: 'Review the dock' }] } } },
        }],
      },
      error: null,
    });
    mocks.resolveRunInteraction.mockResolvedValue({ data: null, error: null });

    await renderDock();
    await waitForText('HLP-42 · Polish the agent dock');
    await waitForText('Approve agent action');

    const approve = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Approve');
    await act(async () => {
      (approve as HTMLButtonElement).click();
    });
    await flush();

    expect(mocks.resolveRunInteraction).toHaveBeenCalledWith(
      'ws-1',
      'agent-run-1',
      'run-int-1',
      expect.objectContaining({ response_payload: { decision: 'approve' } }),
    );
  });

  it('renders the approval card from the interactions fallback when events are empty', async () => {
    const run = { id: 'run-1', status: 'paused', pause_reason: 'human_approval' } as never;
    mocks.getChat.mockResolvedValue({
      data: chatDetail({ chat: { ...CHAT, active_run_id: 'run-1' }, run }),
      error: null,
    });
    mocks.listChatRunInteractions.mockResolvedValue({
      data: {
        interactions: [
          {
            id: 'int-9',
            interaction_kind: 'approval_request',
            status: 'pending',
            request_schema_version: '1',
            request_payload: {
              phase: 'dock_plan_confirm',
              title: 'Confirm fallback launch',
              raw_input: { action: { steps: [{ instructions: 'Do the thing' }] } },
            },
          },
        ],
      },
      error: null,
    });
    mocks.resolveInteraction.mockResolvedValue({ data: null, error: null });

    await renderDock();
    await waitForText('Confirm fallback launch');

    const approve = Array.from(document.body.querySelectorAll('button')).find(
      (b) => b.textContent === 'Approve',
    );
    await act(async () => {
      (approve as HTMLButtonElement).click();
    });
    await flush();

    expect(mocks.resolveInteraction).toHaveBeenCalledWith(
      'ws-1',
      'chat-1',
      'int-9',
      expect.objectContaining({ response_payload: { decision: 'approve' } }),
    );
  });

  it('renders the dock_plan_confirm card and approves through the chat resolver', async () => {
    const run = { id: 'run-1', status: 'paused', pause_reason: 'human_approval' } as never;
    mocks.getChat.mockResolvedValue({
      data: chatDetail({ chat: { ...CHAT, active_run_id: 'run-1' }, run }),
      error: null,
    });
    mocks.listChatRunEvents.mockResolvedValue({
      data: {
        events: [
          {
            id: 'ev-1',
            session_id: 'run-1',
            run_id: 'run-1',
            sequence_no: 1,
            timestamp: '2026-08-01T00:00:01Z',
            type: 'interaction.requested',
            runtime_kind: 'native_sdk',
            payload: {
              interaction_id: 'int-1',
              interaction_kind: 'approval_request',
              status: 'pending',
              request_schema_version: '1',
              request_payload: {
                kind: 'dock_plan_confirm',
                summary: 'Run Review Agent on HLP-12',
                action: {
                  steps: [
                    {
                      agent_id: 'agent-lens',
                      target: { type: 'task', id: 'task-12' },
                      instructions: 'Review the task',
                    },
                  ],
                },
              },
            },
          },
        ],
        next_sequence_no: 1,
      },
      error: null,
    });
    mocks.resolveInteraction.mockResolvedValue({ data: null, error: null });

    await renderDock();
    await waitForText('Run Review Agent on HLP-12');

    const approve = Array.from(document.body.querySelectorAll('button')).find(
      (b) => b.textContent === 'Approve',
    );
    expect(approve).toBeTruthy();
    await act(async () => {
      (approve as HTMLButtonElement).click();
    });
    await flush();

    expect(mocks.resolveInteraction).toHaveBeenCalledWith(
      'ws-1',
      'chat-1',
      'int-1',
      expect.objectContaining({ response_payload: { decision: 'approve' } }),
    );
  });
});
