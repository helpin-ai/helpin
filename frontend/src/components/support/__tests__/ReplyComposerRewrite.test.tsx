// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ReplyComposer } from '../ReplyComposer';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { TooltipProvider } from '@/components/ui/tooltip';

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
