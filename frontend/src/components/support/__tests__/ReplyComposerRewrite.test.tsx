// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ReplyComposer } from '../ReplyComposer';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { Editor } from '@tiptap/core';
import { clearReplyDeliveryDraft, loadReplyDelivery, saveReplyDelivery, saveReplySubject } from '../replyDelivery';

const mocks = vi.hoisted(() => ({
  translation: false,
  rewrite: vi.fn(), send: vi.fn(), empty: [], mutation: { isPending: false, mutateAsync: vi.fn() },
  conversation: null as Record<string, unknown> | null,
}));
vi.mock('@/hooks/queries/useSupport', () => ({
  useRewriteSupportDraft: () => ({ isPending: false, mutateAsync: mocks.rewrite }),
  useSendMessage: () => ({ isPending: false, mutateAsync: mocks.send }),
  useConversation: () => ({ data: mocks.conversation }),
  useCannedResponses: () => ({ data: mocks.empty }),
  useCreateCannedResponse: () => mocks.mutation,
  useUpdateCannedResponse: () => mocks.mutation,
  useDeleteCannedResponse: () => mocks.mutation,
  useUpdateConversationEmailRecipients: () => mocks.mutation,
  useUploadSupportAttachment: () => mocks.mutation,
}));
vi.mock('@/hooks/queries/useSupportTranslation', () => ({
 useSupportTranslationOptions: () => ({ data: { available: mocks.translation, languages: { en: 'English', de: 'German' }, conversation: { customer_language: 'de' }, preference: { reading_language: 'en', auto_translate_incoming: true, auto_translate_outgoing: true } } }),
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
  function setup(emailConversation = false, widgetConversation = false) {
    mocks.send.mockReset();
    mocks.conversation = emailConversation ? { source: 'email', customer_email: 'customer@example.com', subject: 'Invoice question' }
      : widgetConversation ? { source: 'widget', anonymous_id: 'visitor-1', customer_email: 'customer@example.com' } : null;
    useSupportInboxStore.setState({ replyMode: 'reply', drafts: { 'conv-1': '**Original** draft' } });
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => { root.render(<TooltipProvider><ReplyComposer workspaceId="ws-1" conversationId="conv-1" emailDeliveryEnabled /></TooltipProvider>); });
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
    mocks.translation = false;
    saveReplySubject('ws-1', 'conv-1');
    clearReplyDeliveryDraft('ws-1', 'conv-1');
  });

  it('translates on normal Send without a preview step', async () => {
    mocks.translation = true;
    setup(false, true);
    expect(container.textContent).not.toContain('Preview translation');
    await act(async () => { button('Send').click(); });
    expect(mocks.send).toHaveBeenCalledOnce();
    expect(mocks.send.mock.calls[0][0]).toMatchObject({ auto_translate: true, translation_target_language: 'de', content: '**Original** draft' });
    expect(mocks.send.mock.calls[0][0].client_message_id).toBeTruthy();
  });
  it('keeps the draft and same send identity after translation failure', async () => {
    mocks.translation = true;
    setup(false, true);
    mocks.send.mockRejectedValueOnce(new Error('Translation failed'));
    await act(async () => { button('Send').click(); });
    expect(container.querySelector('[contenteditable="true"]')?.textContent).toContain('Original');
    const firstID = mocks.send.mock.calls[0][0].client_message_id;
    await act(async () => { button('Send').click(); });
    expect(mocks.send.mock.calls[1][0].client_message_id).toBe(firstID);
  });
  it('never translates internal notes', async () => {
    mocks.translation = true;
    setup(false, true);
    act(() => useSupportInboxStore.setState({ replyMode: 'note' }));
    await act(async () => { button('Add Note').click(); });
    expect(mocks.send.mock.calls[0][0].auto_translate).toBeUndefined();
  });

  it('sends email without a subject editor or a stale draft subject override', async () => {
    saveReplySubject('ws-1', 'conv-1', 'Old edited subject');
    setup(true);
    expect([...container.querySelectorAll('label')].some((label) => label.textContent?.includes('Subject'))).toBe(false);
    await act(async () => { button('Send').click(); });
    expect(mocks.send).toHaveBeenCalledOnce();
    expect(mocks.send.mock.calls[0][0]).toMatchObject({ delivery_mode: 'email_only', channels: ['email'] });
    expect(mocks.send.mock.calls[0][0]).not.toHaveProperty('email_subject');
  });

  it('uses the chosen email destination for this chat reply and resets after sending', async () => {
    saveReplyDelivery('ws-1', 'conv-1', 'email_only');
    setup(false, true);
    await act(async () => { button('Send').click(); });
    expect(mocks.send.mock.calls[0][0]).toMatchObject({ delivery_mode: 'email_only', channels: ['email'] });
    expect(loadReplyDelivery('ws-1', 'conv-1')).toBeUndefined();
    expect(container.querySelector('[aria-label="Sending options: Chat only"]')).not.toBeNull();
  });

  it('blocks button and keyboard sending for the full upload batch and failed attachments', async () => {
    let completeFirst!: (value: { id: string }) => void;
    let failSecond!: (error: Error) => void;
    mocks.mutation.mutateAsync.mockReset();
    mocks.mutation.mutateAsync
      .mockImplementationOnce(() => new Promise((resolve) => { completeFirst = resolve; }))
      .mockImplementationOnce(() => new Promise((_, reject) => { failSecond = reject; }));
    setup();
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    Object.defineProperty(input, 'files', { value: [new File(['video'], 'first.mp4', { type: 'video/mp4' }), new File(['video'], 'second.mp4', { type: 'video/mp4' })] });
    await act(async () => { input.dispatchEvent(new Event('change', { bubbles: true })); });
    expect(container.querySelector('[aria-label="Remove first.mp4"]')).not.toBeNull();
    expect(container.querySelector('[aria-label="Remove second.mp4"]')).not.toBeNull();
    expect(mocks.mutation.mutateAsync).toHaveBeenCalledTimes(1);
    expect(button('Send').disabled).toBe(true);
    act(() => { container.querySelector('.tiptap')?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true })); });
    expect(mocks.send).not.toHaveBeenCalled();
    await act(async () => { completeFirst({ id: 'uploaded-first' }); });
    expect(mocks.mutation.mutateAsync).toHaveBeenCalledTimes(2);
    expect(button('Send').disabled).toBe(true);
    await act(async () => { failSecond(new Error('upload failed')); });
    expect(button('Send').disabled).toBe(true);
    act(() => { container.querySelector('.tiptap')?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true })); });
    expect(mocks.send).not.toHaveBeenCalled();
    expect(container.querySelector('.tiptap')?.textContent).toContain('Original');
    act(() => { (container.querySelector('[aria-label="Remove second.mp4"]') as HTMLButtonElement).click(); });
    expect(button('Send').disabled).toBe(false);
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
