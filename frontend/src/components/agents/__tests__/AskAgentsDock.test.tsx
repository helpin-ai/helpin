// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AskAgentsDock } from '../AskAgentsDock';
import { PageContextProvider } from '@/components/command-bar/pageContext';
import { TooltipProvider } from '@/components/ui/tooltip';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useDockStore } from '@/stores/dockStore';
import type { DockChat, DockChatDetail, DockRunSummary } from '@/lib/dockTypes';
import type { CommandBarPageContext, CommandBarPlanSummary } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
(globalThis as typeof globalThis & { ResizeObserver: typeof ResizeObserver }).ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};
Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(() => 'blob:clipboard-preview') });
Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() });

const mocks = vi.hoisted(() => ({
  listChats: vi.fn(),
  createChat: vi.fn(),
  getChat: vi.fn(),
  updateChat: vi.fn(),
  sendMessage: vi.fn(),
  listMessages: vi.fn(),
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
	getPublicShare: vi.fn(),
	createPublicShare: vi.fn(),
	revokePublicShare: vi.fn(),
  getPlan: vi.fn(),
  searchEntities: vi.fn(),
  uploadEditorFile: vi.fn(),
}));

vi.mock('@/lib/services/dockChatService', () => ({
  dockChatService: {
    listChats: mocks.listChats,
    createChat: mocks.createChat,
    getChat: mocks.getChat,
    updateChat: mocks.updateChat,
    sendMessage: mocks.sendMessage,
    listMessages: mocks.listMessages,
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
	getPublicShare: mocks.getPublicShare,
	createPublicShare: mocks.createPublicShare,
	revokePublicShare: mocks.revokePublicShare,
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

vi.mock('@/hooks/useEditorImageUpload', () => ({
  uploadEditorFile: mocks.uploadEditorFile,
}));

const CHAT: DockChat = {
  id: 'chat-1',
  workspace_id: 'ws-1',
  user_id: 'user-1',
  title: 'Sprint questions',
  visibility: 'private',
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
  useAuthStore.setState({ user: { id: 'user-1', email: 'owner@example.com' } as never });
  mocks.listChats.mockResolvedValue({ data: { chats: [CHAT] }, error: null });
  mocks.createChat.mockResolvedValue({ data: null, error: 'not configured' });
  mocks.listRuns.mockResolvedValue({ data: { runs: [], attention_count: 0 }, error: null });
  mocks.getChat.mockResolvedValue({ data: chatDetail(), error: null });
  mocks.listMessages.mockResolvedValue({ data: { messages: [], next_before: null }, error: null });
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
  mocks.uploadEditorFile.mockResolvedValue({ attachmentId: 'attachment-clipboard-1', publicUrl: '/clipboard.png' });
	mocks.getPublicShare.mockResolvedValue({ data: null, error: null });
	mocks.createPublicShare.mockResolvedValue({ data: { token: 'share-token', url: 'https://helpin.ai/shared/share-token' }, error: null });
	mocks.revokePublicShare.mockResolvedValue({ data: null, error: null });
	Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
  document.body.innerHTML = '';
  useWorkspaceStore.setState({ currentWorkspace: null });
  useAuthStore.setState({ user: null });
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

async function renderDockWithHiddenTrigger() {
  await act(async () => {
    root.render(
      <TooltipProvider>
        <PageContextProvider>
          <AskAgentsDock hideCollapsedTrigger />
        </PageContextProvider>
      </TooltipProvider>,
    );
  });
  await flush();
}

async function renderEmbeddedDock(
  requiredPageContext: CommandBarPageContext,
  onClose = vi.fn(),
  active = true,
) {
  await act(async () => {
    root.render(
      <TooltipProvider>
        <PageContextProvider>
          <AskAgentsDock
            presentation="embedded"
            requiredPageContext={requiredPageContext}
            associatedSupportConversationId={requiredPageContext.entity_id}
            active={active}
            onClose={onClose}
          />
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
  it('shows attachment types before opening a file picker', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    await act(async () => {
      document.body.querySelector<HTMLButtonElement>('[aria-label="Attach files"]')?.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0 }));
    });

    await waitForText('Images & videos');
    await waitForText('Documents');
  });

  it('uploads images pasted into the Ask composer', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    const firstImage = new File(['clipboard image'], 'screenshot.png', { type: 'image/png' });
    const secondImage = new File(['clipboard image'], 'diagram.webp', { type: 'image/webp' });
    const textFile = new File(['ignore me'], 'notes.txt', { type: 'text/plain' });
    mocks.uploadEditorFile
      .mockResolvedValueOnce({ attachmentId: 'attachment-clipboard-1', publicUrl: '/clipboard-1.png' })
      .mockResolvedValueOnce({ attachmentId: 'attachment-clipboard-2', publicUrl: '/clipboard-2.webp' });
    const paste = new Event('paste', { bubbles: true, cancelable: true });
    Object.defineProperty(paste, 'clipboardData', { value: { files: [firstImage, textFile, secondImage] } });

    await act(async () => {
      dockTextarea().dispatchEvent(paste);
    });
    await waitForCondition(() => mocks.uploadEditorFile.mock.calls.length === 2, 'Pasted images were not uploaded');

    expect(paste.defaultPrevented).toBe(true);
    expect(mocks.uploadEditorFile).toHaveBeenNthCalledWith(1, firstImage, expect.objectContaining({
      workspaceId: 'ws-1',
      entityType: 'editor_upload',
      private: true,
    }));
    expect(mocks.uploadEditorFile).toHaveBeenNthCalledWith(2, secondImage, expect.any(Object));
    await waitForText('screenshot.png');
    await waitForText('diagram.webp');
  });

  it('leaves ordinary text paste to the Ask composer', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    const paste = new Event('paste', { bubbles: true, cancelable: true });
    Object.defineProperty(paste, 'clipboardData', { value: { files: [], getData: () => 'Pasted text' } });
    await act(async () => {
      dockTextarea().dispatchEvent(paste);
    });

    expect(paste.defaultPrevented).toBe(false);
    expect(mocks.uploadEditorFile).not.toHaveBeenCalled();
  });

  it('turns a long text paste into a private text attachment', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    const longText = 'A'.repeat(10_001);
    const paste = new Event('paste', { bubbles: true, cancelable: true });
    Object.defineProperty(paste, 'clipboardData', { value: { files: [], getData: (type: string) => type === 'text/plain' ? longText : '' } });
    await act(async () => {
      dockTextarea().dispatchEvent(paste);
    });
    await waitForCondition(() => mocks.uploadEditorFile.mock.calls.length === 1, 'Long pasted text was not attached');

    const pastedFile = mocks.uploadEditorFile.mock.calls[0]?.[0] as File;
    expect(paste.defaultPrevented).toBe(true);
    expect(pastedFile.name).toBe('Pasted text.txt');
    expect(pastedFile.type).toBe('text/plain');
    expect(await pastedFile.text()).toBe(longText);
    await waitForCondition(
      () => document.body.querySelector<HTMLButtonElement>('button[title="Send"]')?.disabled === false,
      'Attachment-only message did not enable Send',
    );
  });


  it('stays hidden when collapsed in support but opens from the global sidebar event', async () => {
    useDockStore.setState({ collapsed: true });
    await renderDockWithHiddenTrigger();
    expect(document.body.querySelector('[aria-label="Agent dock"]')).toBeNull();

    await act(async () => {
      window.dispatchEvent(new CustomEvent('helpin:ask-agents', { detail: { mode: 'runs' } }));
    });
    await waitForCondition(
      () => document.body.querySelector('[aria-label="Agent runs and chats"]') !== null,
      'global Ask Agents panel did not open',
    );
    expect(useDockStore.getState().collapsed).toBe(false);
  });

  it('reopens the chat associated with the active support conversation', async () => {
    const supportContext: CommandBarPageContext = {
      entity_type: 'support_conversation',
      entity_id: 'conv-42',
      display_title: 'Refund request',
    };
    const associatedChat = { ...CHAT, id: 'chat-support', support_conversation_id: 'conv-42' };
    mocks.listChats.mockResolvedValue({ data: { chats: [CHAT, associatedChat] }, error: null });

    await renderEmbeddedDock(supportContext);
    await waitForCondition(
      () => useDockStore.getState().activeChatId === 'chat-support',
      'associated support chat was not selected',
    );

    expect(mocks.createChat).not.toHaveBeenCalled();
  });

  it('describes support-specific agent capabilities in an empty conversation chat', async () => {
    const supportContext: CommandBarPageContext = {
      entity_type: 'support_conversation',
      entity_id: 'conv-42',
      display_title: 'Refund request',
    };
    const associatedChat = { ...CHAT, support_conversation_id: 'conv-42' };
    mocks.listChats.mockResolvedValue({ data: { chats: [associatedChat] }, error: null });

    await renderEmbeddedDock(supportContext);
    await waitForText('Ask about this conversation, draft a reply, investigate the issue, or have an agent take the next step.');
    await waitForText('Draft a reply');
    await waitForText('Investigate the issue');
    await waitForText('Add context');
    expect(document.body.textContent).not.toContain('Press / to open');
  });

  it('creates an associated chat when the support conversation has no history', async () => {
    const supportContext: CommandBarPageContext = {
      entity_type: 'support_conversation',
      entity_id: 'conv-new',
      display_title: 'New request',
    };
    const newChat = { ...CHAT, id: 'chat-new', support_conversation_id: 'conv-new' };
    mocks.createChat.mockResolvedValue({ data: newChat, error: null });

    await renderEmbeddedDock(supportContext);
    await waitForCondition(() => mocks.createChat.mock.calls.length === 1, 'support chat was not created');

    expect(mocks.createChat).toHaveBeenCalledWith('ws-1', '', 'conv-new', 'support');
    expect(useDockStore.getState().activeChatId).toBe('chat-new');
  });

  it('embeds the full chat without a floating trigger and sends mandatory support context', async () => {
    const supportContext: CommandBarPageContext = {
      entity_type: 'support_conversation',
      entity_id: 'conv-42',
      display_title: 'Refund request',
    };
    mocks.listChats.mockResolvedValue({
      data: { chats: [{ ...CHAT, support_conversation_id: 'conv-42' }] },
      error: null,
    });
    mocks.sendMessage.mockResolvedValue({ data: chatDetail(), error: null });
    await renderEmbeddedDock(supportContext);
    await waitForText('Sprint questions');

    expect(document.body.querySelector('[data-helpin-dock-presentation="embedded"]')).not.toBeNull();
    expect(document.body.querySelector('[aria-label="Agent dock"]')).toBeNull();
    expect(document.body.textContent).toContain('Refund request');

    const textarea = dockTextarea();
    await act(async () => {
      setTextareaValue(textarea, 'Draft a helpful response');
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    await flush();

    expect(mocks.sendMessage).toHaveBeenCalledWith('ws-1', 'chat-1', expect.objectContaining({
      content: 'Draft a helpful response',
      page_context: supportContext,
    }));
  });

  it('switches the embedded composer to the next support conversation chat', async () => {
    const firstContext: CommandBarPageContext = {
      entity_type: 'support_conversation',
      entity_id: 'conv-1',
      display_title: 'First conversation',
    };
    const nextContext: CommandBarPageContext = {
      entity_type: 'support_conversation',
      entity_id: 'conv-2',
      display_title: 'Next conversation',
    };
    const firstChat = { ...CHAT, support_conversation_id: 'conv-1' };
    const nextChat = { ...CHAT, id: 'chat-2', title: 'Next questions', support_conversation_id: 'conv-2' };
    mocks.listChats.mockResolvedValue({ data: { chats: [firstChat, nextChat] }, error: null });
    mocks.sendMessage.mockResolvedValue({ data: chatDetail(), error: null });
    await renderEmbeddedDock(firstContext);
    await waitForText('First conversation');

    await renderEmbeddedDock(nextContext);
    await waitForText('Next conversation');
    await waitForCondition(
      () => useDockStore.getState().activeChatId === 'chat-2',
      'next support chat was not selected',
    );
    const nextTextarea = dockTextarea();

    await act(async () => {
      setTextareaValue(nextTextarea, 'Use the new conversation');
      nextTextarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    await flush();

    expect(mocks.sendMessage).toHaveBeenCalledWith('ws-1', 'chat-2', expect.objectContaining({
      page_context: nextContext,
    }));
  });
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
    const scrollContainer = document.body.querySelector<HTMLElement>('[data-agent-dock-chat-scroll]');
    expect(scrollContainer?.className).toContain('min-h-0');
    expect(scrollContainer?.className).not.toContain('max-h-[60vh]');
    const textarea = dockTextarea();
    expect(textarea.disabled).toBe(false);
  });

  it('scrolls an active chat to its latest message when selected again', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    const scrollContainer = document.body.querySelector<HTMLElement>('[data-agent-dock-chat-scroll]');
    expect(scrollContainer).not.toBeNull();
    Object.defineProperties(scrollContainer!, {
      scrollHeight: { configurable: true, value: 1_000 },
      clientHeight: { configurable: true, value: 300 },
      scrollTop: { configurable: true, writable: true, value: 200 },
    });

    await act(async () => {
      scrollContainer?.dispatchEvent(new Event('scroll', { bubbles: true }));
      await Promise.resolve();
    });
    expect(scrollContainer?.scrollTop).toBe(200);

    const activeChatButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Sprint questions, Stopped"]');
    expect(activeChatButton).not.toBeNull();
    await act(async () => {
      activeChatButton?.click();
      await Promise.resolve();
    });

    expect(scrollContainer?.scrollTop).toBe(1_000);
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
      expect.objectContaining({
        content: 'list open tasks',
        client_message_id: expect.stringMatching(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i),
      }),
    );
    expect(mocks.generateTitle).not.toHaveBeenCalled();
  });

  it('keeps an identical optimistic message visible until its own id is accepted', async () => {
    mocks.listMessages.mockResolvedValue({
      data: {
        messages: [{
          id: 'message-old', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
          dock_chat_sequence: 1, client_message_id: '11111111-1111-4111-8111-111111111111',
          role: 'user', content: 'yes', message_type: 'prompt', sequence_no: 1,
          created_at: '2026-08-01T00:00:01Z', delivery_status: 'sent',
        }],
        next_before: null,
      },
      error: null,
    });
    mocks.sendMessage.mockResolvedValue({ data: chatDetail(), error: null });

    await renderDock();
    await waitForText('yes');
    const textarea = dockTextarea();
    await waitForCondition(() => !textarea.disabled, 'Dock composer did not become ready');
    await act(async () => {
      setTextareaValue(textarea, 'yes');
    });
    await act(async () => {
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
	await flush();
	expect(mocks.sendMessage).toHaveBeenCalledTimes(1);
	expect(mocks.sendMessage.mock.calls[0]?.[2]?.client_message_id).not.toBe('11111111-1111-4111-8111-111111111111');
    await waitForCondition(
      () => document.body.textContent?.includes('Sending…') === true,
      'The repeated optimistic message was removed by the older identical message',
    );
  });

  it.each([
    { label: 'the first message', existingMessages: [] },
    {
      label: 'an ongoing conversation',
      existingMessages: [{
        id: 'message-existing', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
        dock_chat_sequence: 1, client_message_id: 'existing-client-id',
        role: 'user', content: 'Earlier question', message_type: 'prompt', sequence_no: 1,
        created_at: '2026-08-01T00:00:01Z', delivery_status: 'sent',
      }],
    },
  ])('never renders the accepted server row beside the optimistic echo for $label', async ({ existingMessages }) => {
    mocks.listMessages.mockResolvedValue({
      data: { messages: existingMessages, next_before: null },
      error: null,
    });
    let acceptMessage: (() => void) | undefined;
    mocks.sendMessage.mockImplementation((_workspaceId, _chatId, payload) => new Promise((resolve) => {
      acceptMessage = () => resolve({
        data: chatDetail({
          accepted_message: {
            id: 'message-accepted', workspace_id: 'ws-1', run_id: 'run-1', dock_chat_id: 'chat-1',
            dock_chat_sequence: existingMessages.length + 1, client_message_id: payload.client_message_id,
            role: 'user', content: 'Unique optimistic handoff message', message_type: 'prompt',
            sequence_no: existingMessages.length + 1, created_at: '2026-08-15T10:00:00Z', delivery_status: 'sent',
          },
        }),
        error: null,
      });
    }));

    await renderDock();
    await waitForText('Sprint questions');
    const textarea = dockTextarea();
    await act(async () => {
      setTextareaValue(textarea, 'Unique optimistic handoff message');
    });
    await act(async () => {
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    });
    await waitForCondition(() => mocks.sendMessage.mock.calls.length === 1, 'Message was not submitted');
    await waitForText('Unique optimistic handoff message');
    expect(document.body.textContent).toContain('Sending…');

    const observedCounts = [1];
    const observer = new MutationObserver(() => {
      observedCounts.push((document.body.textContent?.match(/Unique optimistic handoff message/g) ?? []).length);
    });
    observer.observe(document.body, { childList: true, subtree: true, characterData: true });

    await act(async () => acceptMessage?.());
    await waitForCondition(
      () => !document.body.textContent?.includes('Sending…'),
      'Accepted message remained optimistic',
    );
    observer.disconnect();

    expect(Math.max(...observedCounts)).toBe(1);
    expect((document.body.textContent?.match(/Unique optimistic handoff message/g) ?? [])).toHaveLength(1);
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
    await waitForText('Add context');

    const addReference = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.textContent?.includes('Add context'));
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
    mocks.cancelChatRun.mockResolvedValue({
      data: { id: 'run-1', status: 'cancelled', pause_reason: 'none' },
      error: null,
    });

    await renderDock();
    await waitForText('Sprint questions');

    const liveStatus = document.body.querySelector('[data-helpin-dock] [data-agent-live-status]');
    const scrollContainer = document.body.querySelector('[data-helpin-dock] [data-agent-dock-chat-scroll]');
    expect(liveStatus?.textContent).toContain('Working…');
    expect(scrollContainer?.contains(liveStatus)).toBe(true);
    expect(document.body.querySelector('[data-agent-live-status-region]')?.contains(liveStatus)).toBe(true);

    const stopButtons = document.body.querySelectorAll('[data-helpin-dock] [aria-label="Stop agent"]');
    expect(stopButtons).toHaveLength(1);
    const stopButton = stopButtons[0];
    expect(stopButton).not.toBeNull();
    await act(async () => {
      (stopButton as HTMLButtonElement).click();
    });
    await flush();

    expect(mocks.cancelChatRun).toHaveBeenCalledWith('ws-1', 'chat-1');
  });

  it('keeps a persisted cancellation visibly pending and prevents repeat stop requests', async () => {
    mocks.getChat.mockResolvedValue({
      data: chatDetail({
        chat: { ...CHAT, active_run_id: 'run-1' },
        run: {
          id: 'run-1',
          status: 'running',
          pause_reason: 'none',
          execution_stage: 'cancelling',
        } as never,
      }),
      error: null,
    });
    mocks.getChatRun.mockResolvedValue({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: null },
      error: null,
    });

    await renderDock();
    await waitForText('Sprint questions');

    const stoppingButton = document.body.querySelector<HTMLButtonElement>(
      '[data-helpin-dock] [aria-label="Stopping agent"]',
    );
    expect(stoppingButton).not.toBeNull();
    expect(stoppingButton?.disabled).toBe(true);
    expect(mocks.cancelChatRun).not.toHaveBeenCalled();
  });

  it('lets a teammate stop an active shared chat', async () => {
    const sharedChat: DockChat = {
      ...CHAT,
      visibility: 'module',
      module_id: 'support',
      active_run_id: 'run-1',
    };
    useAuthStore.setState({ user: { id: 'user-2', email: 'teammate@example.com' } as never });
    mocks.listChats.mockResolvedValue({ data: { chats: [sharedChat] }, error: null });
    mocks.getChat.mockResolvedValue({
      data: chatDetail({
        chat: sharedChat,
        run: { id: 'run-1', status: 'running', pause_reason: 'none' } as never,
      }),
      error: null,
    });
    mocks.getChatRun.mockResolvedValue({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: null },
      error: null,
    });
    mocks.cancelChatRun.mockResolvedValue({
      data: { id: 'run-1', status: 'cancelled', pause_reason: 'none' },
      error: null,
    });

    await renderDock();
    await waitForText('Visible to teammates in Support');

    const stopButton = document.body.querySelector<HTMLButtonElement>(
      '[data-helpin-dock] [aria-label="Stop agent"]',
    );
    expect(stopButton).not.toBeNull();
    await act(async () => stopButton?.click());
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
	const firstClientMessageID = mocks.sendMessage.mock.calls[0]?.[2]?.client_message_id;

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
	expect(mocks.sendMessage.mock.calls[1]?.[2]?.client_message_id).toBe(firstClientMessageID);
  });

  it('loads persisted chat history independently of the active backing run', async () => {
    mocks.listMessages.mockResolvedValue({
      data: {
        messages: [
          {
            id: 'message-old-user', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
            dock_chat_sequence: 1, role: 'user', content: 'What changed?', message_type: 'prompt',
            sequence_no: 1, created_at: '2026-08-01T00:00:01Z', delivery_status: 'sent',
          },
          {
            id: 'message-old-assistant', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
            dock_chat_sequence: 2, runtime_message_id: 'runtime-old-answer', role: 'assistant',
            content: 'The earlier run completed successfully.', message_type: 'assistant_turn',
            sequence_no: 2, created_at: '2026-08-01T00:00:02Z', delivery_status: 'sent',
          },
        ],
        next_before: null,
      },
      error: null,
    });

    await renderDock();
    await waitForText('The earlier run completed successfully.');

    expect(mocks.listMessages).toHaveBeenCalledWith('ws-1', 'chat-1', undefined, 50);
  });

  it('keeps earlier assistant progress and the final reply outside working groups', async () => {
    mocks.listMessages.mockResolvedValue({
      data: {
        messages: [
          {
            id: 'message-user', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
            dock_chat_sequence: 1, role: 'user', content: 'Investigate the issue.', message_type: 'prompt',
            sequence_no: 1, created_at: '2026-08-01T00:00:01Z', delivery_status: 'sent',
          },
          {
            id: 'message-progress', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
            dock_chat_sequence: 2, role: 'assistant', content: 'I will inspect another file.', message_type: 'assistant_turn',
            sequence_no: 2, created_at: '2026-08-01T00:00:02Z', delivery_status: 'sent',
          },
          {
            id: 'message-final', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
            dock_chat_sequence: 3, role: 'assistant', content: 'The issue is caused by stale pagination state.', message_type: 'assistant_turn',
            sequence_no: 3, created_at: '2026-08-01T00:00:03Z', delivery_status: 'sent',
          },
        ],
        next_before: null,
      },
      error: null,
    });

    await renderDock();
    await waitForText('I will inspect another file.');
    await waitForText('The issue is caused by stale pagination state.');

    const workingGroup = document.querySelector('[data-agent-working-group]');
    expect(workingGroup).toBeNull();
    expect(document.body.textContent).toContain('I will inspect another file.');
    expect(document.body.textContent).toContain('The issue is caused by stale pagination state.');
  });

  it('renders earlier history as a compact outlined button with an icon', async () => {
    mocks.listMessages.mockResolvedValue({
      data: { messages: [], next_before: 51 },
      error: null,
    });

    await renderDock();
    await waitForText('Load earlier messages');

    const loadEarlier = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.textContent?.trim() === 'Load earlier messages');
    expect(loadEarlier?.dataset.variant).toBe('outline');
    expect(loadEarlier?.dataset.size).toBe('xs');
    expect(loadEarlier?.querySelector('svg')).not.toBeNull();
  });

  it('shows a spinner while earlier history is loading', async () => {
    mocks.listMessages
      .mockResolvedValueOnce({ data: { messages: [], next_before: 51 }, error: null })
      .mockImplementationOnce(() => new Promise(() => {}));

    await renderDock();
    await waitForText('Load earlier messages');
    const loadEarlier = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.textContent?.trim() === 'Load earlier messages');

    await act(async () => {
      loadEarlier?.click();
      await Promise.resolve();
    });

    expect(loadEarlier?.disabled).toBe(true);
    expect(loadEarlier?.textContent).toContain('Loading…');
    expect(loadEarlier?.querySelector('svg')?.classList.contains('animate-spin')).toBe(true);
  });

  it('loads an older failed sub-agent attempt from its visible result marker', async () => {
    mocks.listMessages.mockResolvedValue({
      data: {
        messages: [
          {
            id: 'message-launch', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
            dock_chat_sequence: 63, role: 'assistant', content: 'I am launching Beacon.',
            message_type: 'assistant_turn', sequence_no: 63,
            created_at: '2026-08-14T08:23:20Z', delivery_status: 'sent',
          },
          {
            id: 'message-result', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
            dock_chat_sequence: 64, role: 'user',
            content: '<child_run_result>{"plan_id":"plan-old","status":"failed","error":"Model unavailable under current pricing","runs":[]}</child_run_result>',
            message_type: 'message', sequence_no: 64,
            created_at: '2026-08-14T08:23:31Z', delivery_status: 'sent',
          },
          {
            id: 'message-after', workspace_id: 'ws-1', run_id: 'run-old', dock_chat_id: 'chat-1',
            dock_chat_sequence: 65, role: 'assistant', content: 'Beacon could not start.',
            message_type: 'assistant_turn', sequence_no: 65,
            created_at: '2026-08-14T08:23:52Z', delivery_status: 'sent',
          },
        ],
        next_before: 63,
      },
      error: null,
    });
    const failedPlan: CommandBarPlanSummary = {
      id: 'plan-old',
      status: 'failed',
      plan_kind: 'one_shot_command',
      prompt: 'Create a CRM deal',
      page_context: { entity_type: 'crm_contact', entity_id: 'contact-1' },
      steps: [{
        agent_id: 'agent-beacon',
        agent_name: 'Beacon',
        target: { entity_type: 'crm_contact', entity_id: 'contact-1' },
        instructions: 'Create the deal',
      }],
      run_ids_by_step: {},
      current_step_index: 0,
      run_count: 0,
      created_at: '2026-08-14T08:23:21Z',
      updated_at: '2026-08-14T08:23:31Z',
      runs: [],
    };
    mocks.getPlan.mockResolvedValue({ data: { plan: failedPlan }, error: null });

    await renderDock();
    await waitForText('Failed to start');

    expect(mocks.getPlan).toHaveBeenCalledWith('ws-1', 'plan-old');
    expect(document.body.textContent).toContain('Beacon');
    expect(document.body.textContent).not.toContain('<child_run_result>');
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

  it('opens a local new-chat composer without creating an abandoned chat', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    const newButton = Array.from(document.body.querySelectorAll('[data-helpin-dock] button')).find(
      (b) => b.textContent?.includes('New chat or task'),
    );
    const footer = document.body.querySelector('.agent-dock-new-chat-footer');
    expect(footer).not.toBeNull();
    expect(footer?.className).toContain('border-t');
    expect(footer?.contains(newButton ?? null)).toBe(true);
    expect(newButton?.className).toContain('bg-[#1c1b19]');
    expect(document.body.querySelector('.agent-dock-roster-controls')?.contains(newButton ?? null)).toBe(false);
    await act(async () => {
      (newButton as HTMLButtonElement).click();
    });
    await waitForText('New chat');
    expect(mocks.createChat).not.toHaveBeenCalled();
    expect(useDockStore.getState().activeChatId).toBeNull();

    const createdChat: DockChat = { ...CHAT, id: 'chat-2', title: '' };
    mocks.createChat.mockResolvedValue({ data: createdChat, error: null });
    mocks.sendMessage.mockResolvedValue({ data: chatDetail({ chat: createdChat }), error: null });
    const textarea = dockTextarea();
    await act(async () => {
      setTextareaValue(textarea, 'Investigate the signup issue');
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    await waitForCondition(() => mocks.sendMessage.mock.calls.length === 1, 'draft chat message was not sent');
    expect(mocks.createChat).toHaveBeenCalledWith('ws-1', '', undefined, null);
    expect(mocks.sendMessage).toHaveBeenCalledWith('ws-1', 'chat-2', expect.objectContaining({ content: 'Investigate the signup issue' }));
  });

  it('shows plain-language visibility and lets the owner share with the workspace', async () => {
    const sharedChat: DockChat = { ...CHAT, visibility: 'workspace' };
    mocks.updateChat.mockResolvedValue({ data: sharedChat, error: null });
    await renderDock();
    await waitForText('Sprint questions');

    const visibilityButton = document.body.querySelector<HTMLButtonElement>(
      '[aria-label="Only you can see this. Change who can see this chat"]',
    );
    expect(visibilityButton).not.toBeNull();
    await act(async () => {
      visibilityButton?.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0 }));
    });
    await waitForText('Who can see this chat?');
    expect(document.body.textContent).not.toContain('Customers and external users can never see Ask Agent chats.');

    const workspaceItem = Array.from(document.body.querySelectorAll<HTMLElement>('[role="menuitem"]'))
      .find((item) => item.textContent?.includes('Everyone at Acme'));
    expect(workspaceItem).toBeTruthy();
    await act(async () => workspaceItem?.click());
    await flush();

    expect(mocks.updateChat).toHaveBeenCalledWith('ws-1', 'chat-1', { visibility: 'workspace' });
    expect(document.body.textContent).toContain('Visible to everyone at Acme');
  });

  it('lets another teammate continue a shared module chat', async () => {
    const sharedChat: DockChat = { ...CHAT, visibility: 'module', module_id: 'support' };
    useAuthStore.setState({ user: { id: 'user-2', email: 'teammate@example.com' } as never });
    mocks.listChats.mockResolvedValue({ data: { chats: [sharedChat] }, error: null });
    mocks.getChat.mockResolvedValue({ data: chatDetail({ chat: sharedChat }), error: null });
    mocks.sendMessage.mockResolvedValue({ data: chatDetail({ chat: sharedChat }), error: null });

    await renderDock();
    await waitForText('Visible to teammates in Support');

    expect(document.body.querySelector('textarea')).not.toBeNull();
    expect(document.body.textContent).not.toContain('Only its creator can continue it.');
    expect(document.body.querySelector('[aria-label*="Change who can see this chat"]')).toBeNull();
    expect(document.body.querySelector('[aria-label="Actions for Sprint questions"]')).toBeNull();

    const textarea = dockTextarea();
    await act(async () => {
      setTextareaValue(textarea, 'I will continue this conversation.');
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    await flush();

    expect(mocks.sendMessage).toHaveBeenCalledWith(
      'ws-1',
      'chat-1',
      expect.objectContaining({ content: 'I will continue this conversation.' }),
    );
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

    const closeButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Minimize"]');
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

	it('creates and copies a public Ask chat link from the header menu', async () => {
		await renderDock();
		await waitForText('Sprint questions');
		const trigger = document.body.querySelector<HTMLButtonElement>('[aria-label="Conversation actions"]');
		await act(async () => trigger?.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0 })));
		await waitForText('Share publicly');
		const share = Array.from(document.body.querySelectorAll<HTMLElement>('[role="menuitem"]'))
			.find((item) => item.textContent === 'Share publicly');
		await act(async () => share?.click());
		await flush();
		expect(mocks.createPublicShare).toHaveBeenCalledWith('ws-1', 'dock_chat', 'chat-1');
		expect(navigator.clipboard.writeText).toHaveBeenCalledWith('https://helpin.ai/shared/share-token');
	});

	it('shows public sharing in the agent run header menu', async () => {
		localStorage.setItem('helpin:agent-dock-selection:ws-1', JSON.stringify({ tab: 'agents', runId: 'agent-run-1' }));
		useDockStore.setState({ tab: 'agents', activeRunId: 'agent-run-1' });
		mocks.listRuns.mockResolvedValue({ data: { runs: [DOCK_RUN], attention_count: 1 }, error: null });
		await renderDock();
		await waitForText('HLP-42');
		const trigger = document.body.querySelector<HTMLButtonElement>('[aria-label="Agent run actions"]');
		expect(trigger).not.toBeNull();
		await act(async () => trigger?.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0 })));
		await waitForText('Share publicly');
	});

  it('presents the dock close control as a minimized action with a tooltip', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    const minimizeButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Minimize"]');
    expect(minimizeButton).not.toBeNull();
    expect(minimizeButton?.getAttribute('data-slot')).toBe('tooltip-trigger');
    expect(document.body.querySelector('[data-dock-header] [aria-label="Close agent dock"]')).toBeNull();

    await act(async () => {
      minimizeButton?.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerType: 'mouse' }));
      await new Promise((resolve) => window.setTimeout(resolve, 350));
    });
    const tooltip = Array.from(document.body.querySelectorAll<HTMLElement>('[data-slot="tooltip-content"]'))
      .find((element) => element.textContent?.includes('Minimize'));
    expect(tooltip?.className).toContain('z-[70]');
  });

  it('maximizes and restores the dock without changing its active conversation', async () => {
    await renderDock();
    await waitForText('Sprint questions');

    const panel = document.body.querySelector<HTMLElement>('#agent-dock-panel');
    const maximizeButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Maximize agent dock"]');
    expect(panel?.getAttribute('data-maximized')).toBeNull();
    expect(maximizeButton).not.toBeNull();

    await act(async () => {
      maximizeButton?.click();
      await Promise.resolve();
    });

    expect(panel?.getAttribute('data-maximized')).toBe('true');
    expect(panel?.className).toContain('h-full');
    expect(document.body.querySelector('[aria-label="Restore agent dock"]')).not.toBeNull();
    expect(document.body.textContent).toContain('Sprint questions');

    const restoreButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Restore agent dock"]');
    await act(async () => {
      restoreButton?.click();
      await Promise.resolve();
    });

    expect(panel?.getAttribute('data-maximized')).toBeNull();
    expect(panel?.className).toContain('w-[min(900px,92vw)]');
    expect(document.body.querySelector('[aria-label="Maximize agent dock"]')).not.toBeNull();
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

    const closeButton = document.body.querySelector<HTMLButtonElement>('[aria-label="Minimize"]');
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

    const scrollContainer = document.body.querySelector<HTMLElement>('[data-agent-dock-chat-scroll]');
    expect(scrollContainer).not.toBeNull();
    Object.defineProperties(scrollContainer!, {
      scrollHeight: { configurable: true, value: 1_000 },
      clientHeight: { configurable: true, value: 300 },
      scrollTop: { configurable: true, writable: true, value: 200 },
    });
    await act(async () => {
      scrollContainer?.dispatchEvent(new Event('scroll', { bubbles: true }));
      await Promise.resolve();
    });

    const approvalNotice = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.includes('Agent needs your approval'),
    );
    expect(approvalNotice).not.toBeUndefined();
    await act(async () => {
      approvalNotice?.click();
      await Promise.resolve();
    });

    expect(scrollContainer?.scrollTop).toBe(1_000);
    expect(document.body.textContent).not.toContain('Agent needs your approval');

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
