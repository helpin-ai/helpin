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
  generateTitle: vi.fn(),
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
  searchEntities: vi.fn(),
}));

vi.mock('@/lib/services/dockChatService', () => ({
  dockChatService: {
    listChats: mocks.listChats,
    createChat: mocks.createChat,
    getChat: mocks.getChat,
    updateChat: mocks.updateChat,
    sendMessage: mocks.sendMessage,
    generateTitle: mocks.generateTitle,
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

vi.mock('@/components/docs/entitySearch', () => ({
  entityTypeLabel: (type: string) => ({
    task: 'Task', document: 'Doc', epic: 'Epic', contact: 'Contact', deal: 'Deal',
  })[type] ?? type,
  searchDocsEntityItems: mocks.searchEntities,
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

function dockRunWithStatus(
  id: string,
  title: string,
  status: 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled',
  attentionKind?: DockRunSummary['attention_kind'],
): DockRunSummary {
  return {
    ...DOCK_RUN,
    run: {
      ...DOCK_RUN.run,
      id,
      status,
      pause_reason: attentionKind ? 'human_approval' : 'none',
      target_info: { task_key: id.toUpperCase(), title },
    } as never,
    agent: { ...DOCK_RUN.agent, id: `agent-${id}`, name: title },
    attention_kind: attentionKind,
    last_activity_at: `2026-08-09T12:00:0${id.length}Z`,
  };
}

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
  mocks.generateTitle.mockResolvedValue({ data: null, error: null });
  mocks.getRunSnapshot.mockResolvedValue({
    data: { id: 'agent-run-1', status: 'paused', pause_reason: 'human_approval', stream_state_snapshot: null },
    error: null,
  });
  mocks.listRunEvents.mockResolvedValue({ data: { events: [], next_sequence_no: 0 }, error: null });
  mocks.listRunInteractions.mockResolvedValue({ data: { interactions: [] }, error: null });
  mocks.listChatRunEvents.mockResolvedValue({ data: { events: [], next_sequence_no: 0 }, error: null });
  mocks.listChatRunInteractions.mockResolvedValue({ data: { interactions: [] }, error: null });
  mocks.getPlan.mockResolvedValue({ data: null, error: null });
  mocks.searchEntities.mockResolvedValue({ items: [], error: null });
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

async function waitForCondition(condition: () => boolean, message: string) {
  for (let i = 0; i < 20; i += 1) {
    if (condition()) return;
    await act(async () => {
      await new Promise((resolve) => window.setTimeout(resolve, 0));
    });
  }
  throw new Error(message);
}

function dockTextarea(): HTMLTextAreaElement {
  const textarea = document.body.querySelector('[data-helpin-dock] textarea');
  if (!textarea) throw new Error('dock textarea not found');
  return textarea as HTMLTextAreaElement;
}

async function waitForComposerFocus() {
  for (let i = 0; i < 10; i += 1) {
    const textarea = dockTextarea();
    if (document.activeElement === textarea) return;
    await act(async () => {
      await new Promise((resolve) => window.setTimeout(resolve, 0));
    });
  }
  expect(document.activeElement).toBe(dockTextarea());
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
    await waitForText('Ask Agent');

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
    expect(document.querySelector('.agent-dock-chat-row-dot')).not.toBeNull();
    expect(document.querySelector('.agent-dock-chat-marker')).not.toBeNull();
    expect(document.body.textContent).not.toContain('Ask Agent · Conversation');
    const textarea = dockTextarea();
    expect(textarea.disabled).toBe(false);
  });

  it('shows running, paused, and stopped chat lifecycle indicators', async () => {
    const recent = new Date().toISOString();
    const chats: DockChat[] = [
      { ...CHAT, id: 'chat-running', title: 'Running chat', active_run_id: 'run-running', active_run_status: 'running' },
      { ...CHAT, id: 'chat-paused', title: 'Paused chat', active_run_id: 'run-paused', active_run_status: 'paused' },
      { ...CHAT, id: 'chat-fallback-paused', title: 'Fallback paused chat', active_run_id: 'run-fallback-paused', updated_at: recent },
      { ...CHAT, id: 'chat-stopped', title: 'Stopped chat', active_run_id: 'run-stopped', active_run_status: 'completed' },
      { ...CHAT, id: 'chat-timeout', title: 'Timed out chat', active_run_id: 'run-timeout' },
    ];
    mocks.listChats.mockResolvedValue({ data: { chats }, error: null });

    await renderDock();
    await waitForText('Stopped chat');

    const runningRow = document.body.querySelector('[aria-label="Running chat, Running"]')?.closest('.agent-dock-roster-row');
    const pausedRow = document.body.querySelector('[aria-label="Paused chat, Paused"]')?.closest('.agent-dock-roster-row');
    const fallbackPausedRow = document.body.querySelector('[aria-label="Fallback paused chat, Paused"]')?.closest('.agent-dock-roster-row');
    const stoppedRow = document.body.querySelector('[aria-label="Stopped chat, Stopped"]')?.closest('.agent-dock-roster-row');
    const timedOutRow = document.body.querySelector('[aria-label="Timed out chat, Stopped"]')?.closest('.agent-dock-roster-row');
    expect(runningRow?.querySelector('[data-agent-dock-chat-status="running"]')).not.toBeNull();
    expect(runningRow?.querySelector('.agent-dock-chat-running-pulse')).not.toBeNull();
    expect(pausedRow?.querySelector('[data-agent-dock-chat-status="paused"]')?.children).toHaveLength(2);
    expect(fallbackPausedRow?.querySelector('[data-agent-dock-chat-status="paused"]')).not.toBeNull();
    expect(stoppedRow?.querySelector('[data-agent-dock-chat-status="stopped"]')).not.toBeNull();
    expect(timedOutRow?.querySelector('[data-agent-dock-chat-status="stopped"]')).not.toBeNull();
  });

  it('applies the authoritative paused status from chat detail when the list omits it', async () => {
    const listedChat = { ...CHAT, active_run_id: 'run-paused' };
    const pausedRun = { id: 'run-paused', status: 'paused', pause_reason: 'awaiting_user_message' } as never;
    mocks.listChats.mockResolvedValue({ data: { chats: [listedChat] }, error: null });
    mocks.getChat.mockResolvedValue({ data: chatDetail({ chat: listedChat, run: pausedRun }), error: null });

    await renderDock();
    await waitForCondition(
      () => useDockStore.getState().chats[0]?.active_run_status === 'paused',
      'Paused chat detail was not projected into the roster',
    );

    expect(document.body.querySelector('[aria-label="Sprint questions, Paused"]')).not.toBeNull();
    expect(useDockStore.getState().chats[0]?.active_run_status).toBe('paused');
  });

  it('refreshes chat lifecycle state when a run update arrives', async () => {
    await renderDock();
    await waitForText('Sprint questions');
    expect(mocks.listChats).toHaveBeenCalledTimes(1);

    await act(async () => {
      window.dispatchEvent(new CustomEvent('agent_run-updated', { detail: { entity_id: 'run-1' } }));
      await new Promise((resolve) => window.setTimeout(resolve, 220));
    });

    expect(mocks.listChats).toHaveBeenCalledTimes(2);
  });

  it('opens chat actions above the dock and archives a conversation', async () => {
    mocks.updateChat.mockResolvedValue({
      data: { ...CHAT, archived_at: '2026-08-10T00:00:00Z' },
      error: null,
    });
    await renderDock();
    await waitForText('Sprint questions');

    const trigger = document.body.querySelector<HTMLButtonElement>('[aria-label="Actions for Sprint questions"]');
    expect(trigger).not.toBeNull();
    expect(trigger?.className).toContain('absolute');
    expect(trigger?.className).toContain('group-hover:opacity-100');
    const chatRow = trigger?.closest('.agent-dock-roster-row');
    const timestamp = chatRow?.querySelector('[data-agent-dock-chat-time]');
    expect(timestamp?.className).toContain('ms-auto');
    await act(async () => {
      trigger?.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0 }));
    });
    await waitForText('Rename');

    const menu = document.body.querySelector<HTMLElement>('[data-slot="dropdown-menu-content"]');
    expect(menu?.className).toContain('z-[70]');
    expect(menu?.textContent).toContain('Rename');
    expect(menu?.textContent).toContain('Archive');

    const archive = Array.from(menu?.querySelectorAll<HTMLElement>('[data-slot="dropdown-menu-item"]') ?? [])
      .find((item) => item.textContent === 'Archive');
    await act(async () => {
      archive?.click();
    });
    await flush();

    expect(mocks.updateChat).toHaveBeenCalledWith('ws-1', 'chat-1', { archived: true });
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
    expect(mocks.generateTitle).not.toHaveBeenCalled();
  });

  it('generates a semantic title after the first message without blocking send', async () => {
    const untitled = { ...CHAT, title: '' };
    const titled = { ...CHAT, title: 'Prioritize open tasks', active_run_id: 'run-1' };
    mocks.listChats.mockResolvedValue({ data: { chats: [untitled] }, error: null });
    mocks.getChat.mockResolvedValue({ data: chatDetail({ chat: untitled }), error: null });
    mocks.sendMessage.mockResolvedValue({
      data: chatDetail({
        chat: { ...untitled, active_run_id: 'run-1' },
        run: { id: 'run-1', status: 'running', pause_reason: 'none' } as never,
      }),
      error: null,
    });
    mocks.generateTitle.mockImplementation(async () => {
      mocks.listChats.mockResolvedValue({ data: { chats: [titled] }, error: null });
      return { data: titled, error: null };
    });

    await renderDock();
    await waitForText('Untitled chat');
    const textarea = dockTextarea();
    await waitForCondition(() => !textarea.disabled, 'Dock composer did not become ready');
    await act(async () => {
      setTextareaValue(textarea, 'Which open tasks should we prioritize this week?');
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    await waitForCondition(() => mocks.generateTitle.mock.calls.length === 1, 'Semantic title request was not sent');

    expect(mocks.generateTitle).toHaveBeenCalledWith('ws-1', 'chat-1', expect.objectContaining({
      content: 'Which open tasks should we prioritize this week?',
    }));
    await waitForText('Prioritize open tasks');
    const titleWords = Array.from(document.body.querySelectorAll<HTMLElement>('.agent-dock-title-word'));
    expect(titleWords.length).toBeGreaterThan(0);
    expect(titleWords.some((word) => word.style.animationDelay === '110ms')).toBe(true);
  });

  it('attaches a searched task reference to the message payload', async () => {
    mocks.searchEntities.mockResolvedValue({
      items: [{
        entityType: 'task',
        entityId: 'task-42',
        title: 'Polish the agent dock',
        displayId: 'HLP-42',
        meta: 'HLP-42 · Platform',
      }],
      error: null,
    });
    mocks.sendMessage.mockResolvedValue({ data: chatDetail(), error: null });
    await renderDock();
    await waitForText('Add reference');

    const addReference = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.textContent?.includes('Add reference'));
    await act(async () => {
      addReference?.click();
      await new Promise((resolve) => window.setTimeout(resolve, 220));
    });
    await waitForText('Polish the agent dock');
    await flush();
    expect(document.querySelector('[data-helpin-dock]')?.getAttribute('aria-hidden')).toBeNull();

    const taskResult = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.textContent?.includes('Polish the agent dock'));
    await act(async () => {
      taskResult?.click();
    });
    await waitForText('HLP-42 · Polish the agent dock');

    const textarea = dockTextarea();
    await act(async () => {
      setTextareaValue(textarea, 'Use this task as context');
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    await flush();

    expect(mocks.sendMessage).toHaveBeenCalledWith('ws-1', 'chat-1', expect.objectContaining({
      content: 'Use this task as context',
      references: [{
        entity_type: 'task',
        entity_id: 'task-42',
        display_title: 'HLP-42 · Polish the agent dock',
      }],
    }));
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
      (b) => b.textContent === 'Agent runs',
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
    const footer = document.body.querySelector('.agent-dock-new-chat-footer');
    expect(footer).not.toBeNull();
    expect(footer?.className).toContain('border-t');
    expect(footer?.contains(newButton ?? null)).toBe(true);
    expect(newButton?.className).toContain('border-[#e6e3dd]');
    expect(document.body.querySelector('.agent-dock-roster-controls')?.contains(newButton ?? null)).toBe(false);
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

    expect(document.body.textContent).toContain('Ask Agent');
    expect(document.body.textContent).toContain('1 agent need your attention');
  });

  it('keeps completed and other inactive agents out of the minimized dock', async () => {
    useDockStore.setState({ collapsed: true });
    mocks.listRuns.mockResolvedValue({
      data: {
        runs: [
          dockRunWithStatus('completed', 'Completed review', 'completed'),
          dockRunWithStatus('failed', 'Failed review', 'failed'),
          dockRunWithStatus('cancelled', 'Cancelled review', 'cancelled'),
          dockRunWithStatus('paused', 'Passively paused review', 'paused'),
          dockRunWithStatus('queued', 'Queued review', 'queued'),
          dockRunWithStatus('running', 'Running review', 'running'),
          DOCK_RUN,
        ],
        attention_count: 1,
      },
      error: null,
    });

    await renderDock();
    await waitForText('1 need you');

    expect(document.body.querySelector('[aria-label^="Open QUEUED"]')).toBeTruthy();
    expect(document.body.querySelector('[aria-label^="Open RUNNING"]')).toBeTruthy();
    expect(document.body.querySelector('[aria-label^="Open HLP-42"]')).toBeTruthy();
    const statusDots = Array.from(document.body.querySelectorAll<HTMLElement>('[data-agent-dock-trigger-status-dot]'));
    expect(statusDots).toHaveLength(3);
    expect(statusDots.every((dot) => dot.className.includes('h-2.5') && dot.className.includes('w-2.5'))).toBe(true);
    expect(document.body.querySelector('[aria-label^="Open COMPLETED"]')).toBeNull();
    expect(document.body.querySelector('[aria-label^="Open FAILED"]')).toBeNull();
    expect(document.body.querySelector('[aria-label^="Open CANCELLED"]')).toBeNull();
    expect(document.body.querySelector('[aria-label^="Open PAUSED"]')).toBeNull();
  });

  it('routes each collapsed dock segment to its own destination', async () => {
    useDockStore.setState({ collapsed: true, tab: 'chats' });
    mocks.listRuns.mockResolvedValue({ data: { runs: [DOCK_RUN], attention_count: 1 }, error: null });
    await renderDock();
    await waitForText('Ask Agent');

    const runButton = document.body.querySelector<HTMLButtonElement>('[aria-label^="Open HLP-42"]');
    expect(runButton).toBeTruthy();
    await act(async () => { runButton?.click(); });
    expect(useDockStore.getState()).toMatchObject({ collapsed: false, tab: 'agents', activeRunId: 'agent-run-1' });

    const closeButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Close agent dock"]');
    await act(async () => {
      closeButton?.click();
      await new Promise((resolve) => window.setTimeout(resolve, 0));
    });

    const askButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Ask Agent"]');
    await act(async () => {
      askButton?.click();
      await new Promise((resolve) => window.setTimeout(resolve, 0));
    });
    expect(useDockStore.getState()).toMatchObject({ collapsed: false, tab: 'chats' });
    await waitForComposerFocus();
  });

  it('keeps agent status and full-session actions out of chat headers', async () => {
    useDockStore.setState({ tab: 'chats', activeRunId: 'agent-run-1' });
    mocks.listRuns.mockResolvedValue({ data: { runs: [DOCK_RUN], attention_count: 1 }, error: null });
    await renderDock();
    await waitForText('Sprint questions');

    const header = document.body.querySelector('[data-dock-header]');
    expect(header?.textContent).not.toContain('Approve');
    expect(header?.querySelector('[aria-label="Open full agent session"]')).toBeNull();
    expect(header?.querySelector('[aria-label="Conversation actions"]')).not.toBeNull();
  });

  it('restores focus to the segment that opened the dock', async () => {
    useDockStore.setState({ collapsed: true });
    await renderDock();
    const askButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Ask Agent"]');
    await act(async () => {
      askButton?.click();
      await new Promise((resolve) => window.setTimeout(resolve, 0));
    });
    await waitForComposerFocus();
    expect(document.body.style.overflow).toBe('hidden');

    const closeButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Close agent dock"]');
    await act(async () => {
      closeButton?.click();
      await new Promise((resolve) => window.setTimeout(resolve, 0));
    });
    expect(document.activeElement).toBe(askButton);
    expect(document.body.style.overflow).toBe('');
  });

  it('loads the next cursor page when the chat roster nears its end', async () => {
    const olderChat: DockChat = { ...CHAT, id: 'chat-older', title: 'Older conversation' };
    mocks.listChats.mockReset();
    mocks.listChats
      .mockResolvedValueOnce({ data: { chats: [CHAT], next_cursor: 'cursor-1' }, error: null })
      .mockResolvedValueOnce({ data: { chats: [olderChat], next_cursor: null }, error: null });
    await renderDock();
    await waitForText('Sprint questions');

    const roster = document.body.querySelector<HTMLElement>('[role="tabpanel"]');
    expect(roster).toBeTruthy();
    Object.defineProperties(roster!, {
      scrollHeight: { configurable: true, value: 1_000 },
      clientHeight: { configurable: true, value: 500 },
      scrollTop: { configurable: true, value: 450 },
    });
    await act(async () => {
      roster?.dispatchEvent(new Event('scroll', { bubbles: true }));
      await Promise.resolve();
    });

    expect(mocks.listChats).toHaveBeenLastCalledWith('ws-1', 'cursor-1');
    await waitForText('Older conversation');
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
