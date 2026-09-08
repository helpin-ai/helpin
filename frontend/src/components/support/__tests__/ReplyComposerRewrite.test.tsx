// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ReplyComposer } from '../ReplyComposer';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { Editor } from '@tiptap/core';

const mocks = vi.hoisted(() => ({
  rewrite: vi.fn(), send: vi.fn(), empty: [], mutation: { isPending: false, mutateAsync: vi.fn() },
}));
vi.mock('@/hooks/queries/useSupport', () => ({
  useRewriteSupportDraft: () => ({ isPending: false, mutateAsync: mocks.rewrite }),
  useSendMessage: () => ({ isPending: false, mutateAsync: mocks.send }),
  useConversation: () => ({ data: null }),
  useCannedResponses: () => ({ data: mocks.empty }),
  useCreateCannedResponse: () => mocks.mutation,
  useUpdateCannedResponse: () => mocks.mutation,
  useDeleteCannedResponse: () => mocks.mutation,
  useUpdateConversationEmailRecipients: () => mocks.mutation,
  useUploadSupportAttachment: () => mocks.mutation,
}));
vi.mock('@tanstack/react-query', () => ({ useQuery: () => ({ data: mocks.empty }) }));
vi.mock('@/stores/dockStore', () => ({ useDockStore: () => null }));
vi.mock('@/stores/authStore', () => ({ useAuthStore: () => null }));
vi.mock('@/stores/workspaceStore', () => ({ useWorkspaceStore: () => null }));
vi.mock('@/stores/supportPresenceStore', () => ({ useSupportPresenceStore: () => null }));
vi.mock('../EmojiPicker', () => ({ EmojiPicker: () => <button>Emoji</button> }));
vi.mock('../LinkInsertModal', () => ({ LinkInsertModal: () => null }));
vi.mock('../SupportAskAgentsButton', () => ({ SupportAskAgentsButton: () => null }));
vi.mock('@/components/ui/dropdown-menu', () => {
  const Wrapper = ({ children }: { children: ReactNode }) => <>{children}</>;
  return {
    DropdownMenu: Wrapper, DropdownMenuContent: Wrapper, DropdownMenuTrigger: Wrapper,
    DropdownMenuSeparator: () => null,
    DropdownMenuItem: ({ children, onSelect }: { children: ReactNode; onSelect: () => void }) => <button onClick={onSelect}>{children}</button>,
  };
});

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('ReplyComposer AI loading state', () => {
  let container: HTMLDivElement;
  let root: Root;
  function setup() {
    mocks.send.mockReset();
    useSupportInboxStore.setState({ replyMode: 'reply', drafts: { 'conv-1': '**Original** draft' } });
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => { root.render(<TooltipProvider><ReplyComposer workspaceId="ws-1" conversationId="conv-1" /></TooltipProvider>); });
  }
  function button(label: string) {
    const found = [...container.querySelectorAll('button')].find((element) => element.textContent === label);
    if (!found) throw new Error(`Missing ${label} button`);
    return found;
  }
  afterEach(() => {
    act(() => { root?.unmount(); });
    container?.remove();
    vi.useRealTimers();
  });

  it('enables draft actions immediately for restored text without focusing the editor', () => {
    setup();
    expect(button('Send').disabled).toBe(false);
    expect(button('AI Tools').disabled).toBe(false);
  });

  it('keeps the latest draft when changing conversations before the save debounce', () => {
    vi.useFakeTimers();
    setup();
    const editor = (container.querySelector('.tiptap') as HTMLElement & { editor: Editor }).editor;
    act(() => { editor.commands.setContent('Just typed'); });
    act(() => { root.render(<TooltipProvider><ReplyComposer key="conv-2" workspaceId="ws-1" conversationId="conv-2" /></TooltipProvider>); });
    expect(container.querySelector('.tiptap')?.textContent).not.toContain('Just typed');
    expect(useSupportInboxStore.getState().drafts['conv-1']).toBe('Just typed');
    expect(mocks.send).not.toHaveBeenCalled();
  });

  it.each([0, 501])('does not overwrite a newer draft from another composer on unmount after %i ms', (elapsed) => {
    vi.useFakeTimers();
    setup();
    if (elapsed) {
      const editor = (container.querySelector('.tiptap') as HTMLElement & { editor: Editor }).editor;
      act(() => { editor.commands.setContent('Previously saved typing'); vi.advanceTimersByTime(elapsed); });
    }
    act(() => { useSupportInboxStore.getState().setDraft('conv-1', 'Typing in the visible mobile composer'); });
    act(() => { root.render(<TooltipProvider><ReplyComposer key="conv-2" workspaceId="ws-1" conversationId="conv-2" /></TooltipProvider>); });
    expect(useSupportInboxStore.getState().drafts['conv-1']).toBe('Typing in the visible mobile composer');
  });

  it('preserves mobile typing when the hidden desktop composer unmounts last', () => {
    vi.useFakeTimers();
    setup();
    const desktop = <ReplyComposer key="desktop" workspaceId="ws-1" conversationId="conv-1" />;
    act(() => root.render(<TooltipProvider>{desktop}<ReplyComposer key="mobile" workspaceId="ws-1" conversationId="conv-1" /></TooltipProvider>));
    const mobile = (container.querySelectorAll('.tiptap')[1] as HTMLElement & { editor: Editor }).editor;
    act(() => { mobile.commands.setContent('Latest mobile reply'); });
    act(() => root.render(<TooltipProvider>{desktop}</TooltipProvider>));
    expect(useSupportInboxStore.getState().drafts['conv-1']).toBe('Latest mobile reply');
    act(() => root.render(null));
    expect(useSupportInboxStore.getState().drafts['conv-1']).toBe('Latest mobile reply');
  });

  it('shows a loader and blocks editing and sending until the rewritten draft is ready', async () => {
    let resolve!: (value: { content: string }) => void;
    mocks.rewrite.mockImplementation(() => new Promise((res) => { resolve = res; }));
    setup();
    act(() => { button('Fix grammar').click(); });
    expect(container.querySelector('[role="status"]')?.textContent).toContain('Rewriting draft…');
    expect(container.querySelector('.tiptap')?.getAttribute('contenteditable')).toBe('false');
    expect(container.querySelector('[inert]')).not.toBeNull();
    expect(button('Send').disabled).toBe(true);
    expect(mocks.rewrite).toHaveBeenCalledWith({ content: '**Original** draft', operation: 'fix_grammar' });
    act(() => {
      container.querySelector('.tiptap')?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true }));
      button('Send').click();
    });
    expect(mocks.send).not.toHaveBeenCalled();
    await act(async () => { resolve({ content: '**Rewritten** draft' }); });
    expect(container.querySelector('[role="status"]')).toBeNull();
    expect(container.querySelector('.tiptap')?.getAttribute('contenteditable')).toBe('true');
    expect(container.querySelector('.tiptap strong')?.textContent).toBe('Rewritten');
    expect(button('Send').disabled).toBe(false);
  });

  it('restores the composer and original formatted draft after a failed rewrite', async () => {
    let reject!: (reason: Error) => void;
    mocks.rewrite.mockImplementation(() => new Promise((_, rej) => { reject = rej; }));
    setup();
    act(() => { button('Rephrase').click(); });
    await act(async () => { reject(new Error('Provider failed')); });
    expect(container.querySelector('[role="status"]')).toBeNull();
    expect(container.querySelector('.tiptap')?.getAttribute('contenteditable')).toBe('true');
    expect(container.querySelector('.tiptap strong')?.textContent).toBe('Original');
    expect(button('Send').disabled).toBe(false);
  });
});
