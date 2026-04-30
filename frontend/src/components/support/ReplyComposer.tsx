import { useRef, useEffect, useState, useCallback, useMemo, type ReactNode } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import { Markdown } from 'tiptap-markdown';
import {
  SentIcon, AttachmentIcon, Cancel01Icon, Loading01Icon,
  Mail01Icon, SparklesIcon, ArrowUp01Icon, ArrowUpDownIcon, ArrowReloadHorizontalIcon,
  TickDouble01Icon, SmileIcon, Briefcase01Icon, Copy01Icon, PlusSignIcon,
  TextBoldIcon, TextItalicIcon, TextUnderlineIcon, TextStrikethroughIcon,
  CodeIcon, QuoteDownIcon, LeftToRightListBulletIcon, LeftToRightListNumberIcon, Link01Icon,
} from '@/lib/icons';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Checkbox } from '@/components/ui/checkbox';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Label } from '@/components/ui/label';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { MentionHighlight } from '@/components/pm/mention-highlight';
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList';
import { getMentionSuggestions, type MentionSuggestionItem } from '@/components/pm/mentionSuggestions';
import { useCannedResponses, useRewriteSupportDraft, useSendMessage, useUploadSupportAttachment } from '@/hooks/queries/useSupport';
import { queryKeys } from '@/lib/queryKeys';
import { workspacesService } from '@/lib/services/workspacesService';
import { unwrap } from '@/lib/queryUtils';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useAuthStore } from '@/stores/authStore';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import type { AssignableMember } from '@/lib/types';
import type { SupportAIRewriteOperation, SupportCannedResponse } from '@/lib/pmTypes';
import { EmojiPicker } from './EmojiPicker';
import { LinkInsertModal } from './LinkInsertModal';
import { AddShortcutDialog } from './AddShortcutDialog';

const OFFLINE_EMAIL_CONFIRM_STORAGE_PREFIX = 'support_offline_email_confirm';
const RESTORE_SUPPORT_DRAFT_EVENT = 'support:restore-draft';

interface ReplyComposerProps {
  workspaceId: string;
  conversationId: string;
  emailFallbackHint?: {
    email: string;
  } | null;
}

function loadSkipOfflineEmailConfirm(storageKey: string): boolean {
  try {
    return localStorage.getItem(storageKey) === '1';
  } catch {
    return false;
  }
}

function saveSkipOfflineEmailConfirm(storageKey: string, skip: boolean) {
  try {
    if (skip) {
      localStorage.setItem(storageKey, '1');
    } else {
      localStorage.removeItem(storageKey);
    }
  } catch {}
}

function getEditorMarkdown(editorInstance: ReturnType<typeof useEditor> | null | undefined): string {
  if (!editorInstance) return '';
  const storage = (editorInstance.storage as { markdown?: { getMarkdown(): string } }).markdown;
  if (storage?.getMarkdown) return storage.getMarkdown();
  return editorInstance.getText();
}

const URL_TOKEN_REGEX =
  /^((?:https?|ftp):\/\/\S+|www\.\S+\.[a-z]{2,}\S*|[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+\.[a-z]{2,}(?:[/?#]\S*)?)$/i;
const TRAILING_PUNCT_REGEX = /[.,;:!?)\]}>'"]+$/;

function normalizeUrl(raw: string): string {
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(raw)) return raw;
  return `https://${raw}`;
}

function findBareUrlBeforeCaret(
  editorInstance: ReturnType<typeof useEditor> | null | undefined,
): { from: number; to: number; href: string } | null {
  if (!editorInstance) return null;
  const { state } = editorInstance;
  const { selection } = state;
  if (!selection.empty) return null;

  const $from = selection.$from;
  const paragraphStart = $from.start();
  const caret = $from.pos;
  if (caret <= paragraphStart) return null;

  const textBefore = state.doc.textBetween(paragraphStart, caret, '\n', '\ufffc');
  const match = textBefore.match(/(\S+)$/);
  if (!match) return null;

  const rawToken = match[1];
  const trail = rawToken.match(TRAILING_PUNCT_REGEX);
  const trimmed = trail ? rawToken.slice(0, -trail[0].length) : rawToken;
  if (!trimmed || !URL_TOKEN_REGEX.test(trimmed)) return null;

  const tokenStart = caret - rawToken.length;
  const tokenEnd = tokenStart + trimmed.length;

  const linkMark = state.schema.marks.link;
  if (linkMark && state.doc.rangeHasMark(tokenStart, tokenEnd, linkMark)) return null;

  return { from: tokenStart, to: tokenEnd, href: normalizeUrl(trimmed) };
}

function FormatButton({
  active = false,
  title,
  onClick,
  children,
}: {
  active?: boolean;
  title: string;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={title}
          aria-pressed={active}
          onClick={(e) => { e.preventDefault(); onClick(); }}
          onMouseDown={(e) => e.preventDefault()}
          className={cn(
            'inline-flex h-6 w-6 items-center justify-center rounded-md transition-colors',
            active
              ? 'bg-accent text-foreground'
              : 'text-muted-foreground hover:bg-muted hover:text-foreground',
          )}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="text-xs">{title}</TooltipContent>
    </Tooltip>
  );
}

function detectMentions(
  editorInstance: ReturnType<typeof useEditor>,
  members: AssignableMember[],
): { from: number; to: number; items: MentionSuggestionItem[]; selectedIndex: number } | null {
  if (!editorInstance) return null;
  if (members.length === 0) return null;

  const { selection } = editorInstance.state;
  if (!selection.empty) return null;

  const textBefore = selection.$from.parent.textBetween(
    0,
    selection.$from.parentOffset,
    undefined,
    '\ufffc',
  );
  const match = textBefore.match(/(?:^|\s)@([a-z0-9._-]*)$/i);
  if (!match) return null;

  const query = match[1].toLowerCase();
  const items = getMentionSuggestions(query, members, [], 8);
  if (items.length === 0) return null;

  return {
    from: selection.from - (query.length + 1),
    to: selection.from,
    items: items.slice(0, 8),
    selectedIndex: 0,
  };
}

function stripShortcutContent(value: string) {
  if (!value) return '';
  const doc = new DOMParser().parseFromString(value, 'text/html');
  return doc.body.textContent?.replace(/\s+/g, ' ').trim() || value.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
}

function filterShortcuts(shortcuts: SupportCannedResponse[], query: string) {
  const normalized = query.trim().toLowerCase().replace(/^!/, '');
  return shortcuts
    .filter((item) => {
      if (!normalized) return true;
      return item.short_code.toLowerCase().replace(/^!/, '').includes(normalized) ||
        item.title.toLowerCase().includes(normalized) ||
        stripShortcutContent(item.content).toLowerCase().includes(normalized) ||
        (item.tag || 'Others').toLowerCase().includes(normalized);
    })
    .sort((a, b) => a.short_code.localeCompare(b.short_code))
    .slice(0, 8);
}

function groupShortcuts(shortcuts: SupportCannedResponse[]) {
  const groups = new Map<string, SupportCannedResponse[]>();
  for (const shortcut of shortcuts) {
    const tag = shortcut.tag || 'Others';
    groups.set(tag, [...(groups.get(tag) ?? []), shortcut]);
  }
  return Array.from(groups.entries()).map(([tag, items]) => ({ tag, items }));
}

function detectShortcuts(
  editorInstance: ReturnType<typeof useEditor>,
  shortcuts: SupportCannedResponse[],
): { from: number; to: number; query: string; items: SupportCannedResponse[]; selectedIndex: number } | null {
  if (!editorInstance) return null;
  const { selection } = editorInstance.state;
  if (!selection.empty) return null;

  const textBefore = selection.$from.parent.textBetween(
    0,
    selection.$from.parentOffset,
    undefined,
    '\ufffc',
  );
  const match = textBefore.match(/!([^\s!]*)$/);
  if (!match) return null;

  const query = match[1].toLowerCase();
  return {
    from: selection.from - (query.length + 1),
    to: selection.from,
    query,
    items: filterShortcuts(shortcuts, query),
    selectedIndex: 0,
  };
}

function highlightMatch(text: string, query: string): ReactNode {
  if (!text) return text;
  const needle = query.replace(/^!/, '').trim();
  if (!needle) return text;
  const lower = text.toLowerCase();
  const idx = lower.indexOf(needle.toLowerCase());
  if (idx === -1) return text;
  const end = idx + needle.length;
  return (
    <>
      {text.slice(0, idx)}
      <mark className="bg-transparent font-semibold text-foreground">{text.slice(idx, end)}</mark>
      {text.slice(end)}
    </>
  );
}

function ShortcutsList({
  shortcuts,
  selectedIndex,
  onSelect,
  compact = false,
  query = '',
}: {
  shortcuts: SupportCannedResponse[];
  selectedIndex?: number;
  onSelect: (shortcut: SupportCannedResponse) => void;
  compact?: boolean;
  query?: string;
}) {
  const selectedRef = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    selectedRef.current?.scrollIntoView({ block: 'nearest' });
  }, [selectedIndex]);

  let flatIndex = 0;
  return (
    <div role="listbox" aria-label="Message shortcuts" className="space-y-2">
      {groupShortcuts(shortcuts).map((group) => (
        <div key={group.tag}>
          <div className="flex items-center justify-between px-2 pb-1">
            <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/60">
              {group.tag === 'Others' ? 'Others' : group.tag}
            </span>
            <span className="text-[10px] tabular-nums text-muted-foreground/50">{group.items.length}</span>
          </div>
          <div className="space-y-0.5">
            {group.items.map((shortcut) => {
              const currentIndex = flatIndex++;
              const active = selectedIndex === currentIndex;
              return (
                <button
                  key={shortcut.id}
                  ref={active ? selectedRef : undefined}
                  type="button"
                  role="option"
                  aria-selected={active}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => onSelect(shortcut)}
                  className={cn(
                    'flex w-full items-start gap-3 rounded-md border-l-2 border-transparent px-2 py-2 text-left transition-colors',
                    active ? 'border-primary bg-accent text-accent-foreground' : 'hover:bg-muted',
                  )}
                >
                  <span className="mt-0.5 shrink-0 font-mono text-xs font-semibold text-primary">
                    {highlightMatch(shortcut.short_code, query)}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">
                      {highlightMatch(shortcut.title || shortcut.short_code, query)}
                    </span>
                    <span className={cn('block truncate text-xs text-muted-foreground', compact && 'max-w-[520px]')}>
                      {stripShortcutContent(shortcut.content)}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

export function ReplyComposer({ workspaceId, conversationId, emailFallbackHint }: ReplyComposerProps) {
  const { replyMode, setReplyMode, setDraft, clearDraft } = useSupportInboxStore();
  const sendMutation = useSendMessage(workspaceId, conversationId);
  const rewriteMutation = useRewriteSupportDraft(workspaceId, conversationId);
  const userId = useAuthStore((s) => s.user?.id ?? null);

  const { data: members = [] } = useQuery({
    queryKey: [...queryKeys.workspaces.members(workspaceId), 'assignable'],
    queryFn: async () => unwrap(await workspacesService.listAssignableMembers(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
  const { data: shortcuts = [] } = useCannedResponses(workspaceId);

  const draftTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isTypingRef = useRef(false);
  const isNote = replyMode === 'note';
  const isNoteRef = useRef(isNote);
  isNoteRef.current = isNote;

  const lastTypingSentRef = useRef(0);
  const wsSend = useSupportPresenceStore((s) => s.wsSend);
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected);
  const offlineEmailConfirmStorageKey = useMemo(
    () => `${OFFLINE_EMAIL_CONFIRM_STORAGE_PREFIX}:${workspaceId}:${userId ?? 'anonymous'}`,
    [workspaceId, userId],
  );
  const [skipOfflineEmailConfirm, setSkipOfflineEmailConfirm] = useState(() =>
    loadSkipOfflineEmailConfirm(offlineEmailConfirmStorageKey),
  );
  const [offlineEmailConfirmOpen, setOfflineEmailConfirmOpen] = useState(false);
  const [doNotAskAgain, setDoNotAskAgain] = useState(false);

  // Link insertion modal
  const [linkModalOpen, setLinkModalOpen] = useState(false);
  const [linkInitial, setLinkInitial] = useState<{ label: string; url: string }>({ label: '', url: '' });

  // Toolbar visibility — show when the editor is focused, or while interacting
  // with the toolbar itself, or when the link modal is open.
  const [, setEditorFocused] = useState(false);
  const toolbarHasPointerRef = useRef(false);
  // toolbarHasPointerRef still used by the merged bottom bar to keep editor focus state

  useEffect(() => {
    setSkipOfflineEmailConfirm(loadSkipOfflineEmailConfirm(offlineEmailConfirmStorageKey));
  }, [offlineEmailConfirmStorageKey]);

  // File attachments
  const fileInputRef = useRef<HTMLInputElement>(null);
  const uploadMutation = useUploadSupportAttachment(workspaceId, conversationId);
  const [pendingAttachments, setPendingAttachments] = useState<
    { localId: string; fileName: string; fileType: string; status: 'uploading' | 'done' | 'error'; attachmentId?: string; previewUrl?: string }[]
  >([]);

  const handleFileSelect = useCallback(async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    for (const file of Array.from(files)) {
      if (file.size > 10 * 1024 * 1024) {
        toast.error(`${file.name} exceeds 10 MB limit`);
        continue;
      }
      const localId = `att-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
      const previewUrl = file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined;
      setPendingAttachments((prev) => [...prev, { localId, fileName: file.name, fileType: file.type, status: 'uploading', previewUrl }]);
      try {
        const result = await uploadMutation.mutateAsync({ file });
        setPendingAttachments((prev) => prev.map((a) => a.localId === localId ? { ...a, status: 'done', attachmentId: result.id } : a));
      } catch {
        setPendingAttachments((prev) => prev.map((a) => a.localId === localId ? { ...a, status: 'error' } : a));
      }
    }
    if (fileInputRef.current) fileInputRef.current.value = '';
  }, [uploadMutation]);

  const removeAttachment = useCallback((localId: string) => {
    setPendingAttachments((prev) => {
      const att = prev.find((a) => a.localId === localId);
      if (att?.previewUrl) URL.revokeObjectURL(att.previewUrl);
      return prev.filter((a) => a.localId !== localId);
    });
  }, []);

  // Mention state
  const [mentionState, setMentionState] = useState<{
    from: number;
    to: number;
    items: MentionSuggestionItem[];
    selectedIndex: number;
  } | null>(null);
  const mentionStateRef = useRef(mentionState);
  mentionStateRef.current = mentionState;
  const membersRef = useRef(members);
  membersRef.current = members;
  const shortcutsPanelOpenRef = useRef(false);
  const panelIndexRef = useRef(0);
  const [addShortcutOpen, setAddShortcutOpen] = useState(false);
  const [addShortcutSeed, setAddShortcutSeed] = useState('');
  const [shortcutsPanelOpen, setShortcutsPanelOpen] = useState(false);
  shortcutsPanelOpenRef.current = shortcutsPanelOpen;
  const [shortcutState, setShortcutState] = useState<{
    from: number;
    to: number;
    query: string;
    items: SupportCannedResponse[];
    selectedIndex: number;
  } | null>(null);
  const shortcutStateRef = useRef(shortcutState);
  shortcutStateRef.current = shortcutState;
  const shortcutsRef = useRef(shortcuts);
  shortcutsRef.current = shortcuts;
  const [panelIndex, setPanelIndex] = useState(0);
  panelIndexRef.current = panelIndex;

  // Typing indicator
  const sendTyping = useCallback((typing: boolean, typingContent?: string) => {
    if (isNoteRef.current || !wsSend || !wsConnected) return;
    if (!typing && typingTimerRef.current) {
      clearTimeout(typingTimerRef.current);
      typingTimerRef.current = null;
    }
    if (!typing) {
      isTypingRef.current = false;
      wsSend('support:typing:stop', { conversation_id: conversationId });
      return;
    }
    const now = Date.now();
    if (isTypingRef.current && now - lastTypingSentRef.current < 300) return;
    const eventType = isTypingRef.current ? 'support:typing:update' : 'support:typing:start';
    isTypingRef.current = true;
    lastTypingSentRef.current = now;
    wsSend(eventType, {
      conversation_id: conversationId,
      content: typingContent ?? '',
    });
  }, [conversationId, wsSend, wsConnected]);

  const handleTyping = useCallback((typingContent: string) => {
    sendTyping(true, typingContent);
    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    typingTimerRef.current = setTimeout(() => sendTyping(false), 5000);
  }, [sendTyping]);

  // Clean up typing indicator on unmount or conversation change
  useEffect(() => {
    return () => {
      if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
      if (isTypingRef.current && wsSend) {
        isTypingRef.current = false;
        wsSend('support:typing:stop', { conversation_id: conversationId });
      }
    };
  }, [conversationId, wsSend]);

  const handleSendRef = useRef<() => void>(() => {});

  const extensions = useMemo(() => [
    StarterKit.configure({
      heading: false,
      codeBlock: false,
      horizontalRule: false,
      link: {
        openOnClick: false,
        autolink: true,
        linkOnPaste: true,
        HTMLAttributes: {
          target: '_blank',
          rel: 'noopener noreferrer nofollow',
        },
      },
    }),
    Markdown.configure({
      html: false,
      linkify: true,
      breaks: true,
      transformPastedText: true,
      transformCopiedText: true,
    }),
    Placeholder.configure({
      placeholder: () => isNoteRef.current ? 'Add an internal note... (@ to mention)' : 'Write a reply... (@ to mention)',
    }),
    MentionHighlight,
  ], []);

  const editor = useEditor({
    extensions,
    editorProps: {
      attributes: {
        class: 'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[40px] max-h-[160px] overflow-y-auto text-sm leading-relaxed',
      },
      handleKeyDown: (_view, event) => {
        const currentShortcut = shortcutStateRef.current;
        if (currentShortcut) {
          if (event.key === 'ArrowDown' && currentShortcut.items.length > 0) {
            event.preventDefault();
            setShortcutState({
              ...currentShortcut,
              selectedIndex: (currentShortcut.selectedIndex + 1) % currentShortcut.items.length,
            });
            return true;
          }
          if (event.key === 'ArrowUp' && currentShortcut.items.length > 0) {
            event.preventDefault();
            setShortcutState({
              ...currentShortcut,
              selectedIndex:
                (currentShortcut.selectedIndex - 1 + currentShortcut.items.length) %
                currentShortcut.items.length,
            });
            return true;
          }
          if ((event.key === 'Enter' || event.key === 'Tab') && currentShortcut.items.length > 0) {
            const selected = currentShortcut.items[currentShortcut.selectedIndex];
            if (!selected || !editorRef.current) return false;
            event.preventDefault();
            editorRef.current
              .chain()
              .focus()
              .insertContentAt(
                { from: currentShortcut.from, to: currentShortcut.to },
                selected.content,
              )
              .run();
            setShortcutState(null);
            setShortcutsPanelOpen(false);
            setReplyMode('reply');
            return true;
          }
          if (event.key === 'Escape') {
            event.preventDefault();
            setShortcutState(null);
            setShortcutsPanelOpen(false);
            return true;
          }
        }

        // Browse-mode keyboard nav (panel opened via toolbar, no `!` query)
        if (!currentShortcut && shortcutsPanelOpenRef.current && shortcutsRef.current.length > 0) {
          const items = shortcutsRef.current;
          if (event.key === 'ArrowDown') {
            event.preventDefault();
            setPanelIndex((idx) => (idx + 1) % items.length);
            return true;
          }
          if (event.key === 'ArrowUp') {
            event.preventDefault();
            setPanelIndex((idx) => (idx - 1 + items.length) % items.length);
            return true;
          }
          if (event.key === 'Enter') {
            const selected = items[panelIndexRef.current];
            if (!selected || !editorRef.current) return false;
            event.preventDefault();
            editorRef.current.chain().focus().insertContent(selected.content).run();
            setShortcutsPanelOpen(false);
            return true;
          }
          if (event.key === 'Escape') {
            event.preventDefault();
            setShortcutsPanelOpen(false);
            return true;
          }
        }

        const currentMention = mentionStateRef.current;
        if (currentMention && currentMention.items.length > 0) {
          if (event.key === 'ArrowDown') {
            event.preventDefault();
            setMentionState({
              ...currentMention,
              selectedIndex: (currentMention.selectedIndex + 1) % currentMention.items.length,
            });
            return true;
          }
          if (event.key === 'ArrowUp') {
            event.preventDefault();
            setMentionState({
              ...currentMention,
              selectedIndex:
                (currentMention.selectedIndex - 1 + currentMention.items.length) %
                currentMention.items.length,
            });
            return true;
          }
          if (event.key === 'Enter' || event.key === 'Tab') {
            const selected = currentMention.items[currentMention.selectedIndex];
            if (!selected || !editorRef.current) return false;
            event.preventDefault();
            editorRef.current
              .chain()
              .focus()
              .insertContentAt(
                { from: currentMention.from, to: currentMention.to },
                `@${selected.handle} `,
              )
              .run();
            setMentionState(null);
            return true;
          }
          if (event.key === 'Escape') {
            event.preventDefault();
            setMentionState(null);
            return true;
          }
        }

        // Ctrl/Cmd+Enter to submit
        if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
          event.preventDefault();
          handleSendRef.current();
          return true;
        }

        // Shift+Enter — auto-link the trailing URL token, then insert a hard break
        if (event.key === 'Enter' && event.shiftKey && !event.ctrlKey && !event.metaKey) {
          const ed = editorRef.current;
          const found = findBareUrlBeforeCaret(ed);
          if (ed && found) {
            event.preventDefault();
            ed.chain()
              .focus()
              .setTextSelection({ from: found.from, to: found.to })
              .setLink({ href: found.href })
              .setTextSelection(found.to)
              .unsetMark('link')
              .setHardBreak()
              .run();
            return true;
          }
        }

        return false;
      },
    },
    onUpdate: ({ editor: ed }) => {
      const text = ed.getText();
      const markdown = getEditorMarkdown(ed);

      // Debounce draft save (markdown so formatting persists across reloads)
      if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
      draftTimerRef.current = setTimeout(() => setDraft(conversationId, markdown), 500);

      // Typing indicator (plain text preview is enough)
      text.trim() ? handleTyping(text) : sendTyping(false);

      // Mention detection
      const mention = detectMentions(ed, membersRef.current);
      setMentionState(mention);
      const shortcut = detectShortcuts(ed, shortcutsRef.current);
      setShortcutState(shortcut);
      setShortcutsPanelOpen(!!shortcut);

      // Auto-switch to note mode when mention detected in reply mode
      if (mention && useSupportInboxStore.getState().replyMode === 'reply') {
        setReplyMode('note');
      }
    },
    onBlur: () => {
      setMentionState(null);
      setShortcutState(null);
      setShortcutsPanelOpen(false);
    },
  });

  const editorRef = useRef(editor);
  editorRef.current = editor;

  const insertShortcut = useCallback((shortcut: SupportCannedResponse, range?: { from: number; to: number }) => {
    const ed = editorRef.current;
    if (!ed) return;
    const chain = ed.chain().focus();
    if (range) {
      chain.insertContentAt(range, shortcut.content).run();
    } else {
      chain.insertContent(shortcut.content).run();
    }
    setShortcutState(null);
    setShortcutsPanelOpen(false);
    setReplyMode('reply');
  }, [setReplyMode]);

  // Backup event handlers for TipTap v3 compatibility
  useEffect(() => {
    if (!editor) return;

    const handleUpdate = () => {
      const text = editor.getText();
      const markdown = getEditorMarkdown(editor);
      if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
      draftTimerRef.current = setTimeout(() => setDraft(conversationId, markdown), 500);
      text.trim() ? handleTyping(text) : sendTyping(false);
      const mention = detectMentions(editor, membersRef.current);
      setMentionState(mention);
      const shortcut = detectShortcuts(editor, shortcutsRef.current);
      setShortcutState(shortcut);
      setShortcutsPanelOpen(!!shortcut);
      if (mention && useSupportInboxStore.getState().replyMode === 'reply') {
        setReplyMode('note');
      }
    };
    const handleBlur = () => {
      setMentionState(null);
      setShortcutState(null);
      setShortcutsPanelOpen(false);
      // Defer so that clicking a toolbar button (which steals focus briefly)
      // doesn't immediately collapse the toolbar.
      setTimeout(() => {
        if (!toolbarHasPointerRef.current) {
          setEditorFocused(false);
        }
      }, 0);
    };
    const handleFocus = () => setEditorFocused(true);

    editor.on('update', handleUpdate);
    editor.on('blur', handleBlur);
    editor.on('focus', handleFocus);

    return () => {
      editor.off('update', handleUpdate);
      editor.off('blur', handleBlur);
      editor.off('focus', handleFocus);
    };
  }, [editor, conversationId, handleTyping, sendTyping, setDraft, setReplyMode]);

  // Load draft when switching conversations
  useEffect(() => {
    if (!editor) return;
    const saved = useSupportInboxStore.getState().drafts[conversationId] ?? '';
    if (saved) {
      // Markdown extension parses markdown when content is a string
      editor.commands.setContent(saved);
    } else {
      editor.commands.clearContent();
    }
    return () => {
      if (draftTimerRef.current) {
        clearTimeout(draftTimerRef.current);
        draftTimerRef.current = null;
      }
    };
  }, [conversationId, editor]);

  useEffect(() => {
    if (!editor) return;
    const handleRestoreDraft = (event: Event) => {
      const detail = (event as CustomEvent<{ conversationId?: string; markdown?: string }>).detail;
      if (detail?.conversationId !== conversationId) return;
      const markdown = detail.markdown ?? '';
      setReplyMode('reply');
      setDraft(conversationId, markdown);
      editor.commands.setContent(markdown);
      editor.commands.focus('end');
    };
    window.addEventListener(RESTORE_SUPPORT_DRAFT_EVENT, handleRestoreDraft);
    return () => window.removeEventListener(RESTORE_SUPPORT_DRAFT_EVENT, handleRestoreDraft);
  }, [conversationId, editor, setDraft, setReplyMode]);

  // Force placeholder redecoration when mode changes
  useEffect(() => {
    if (editor && editor.isEmpty) {
      editor.view.dispatch(editor.state.tr);
    }
  }, [editor, isNote]);

  const sendReply = useCallback(async () => {
    if (!editor) return;
    const markdown = getEditorMarkdown(editor).trim();
    const doneAttachments = pendingAttachments.filter((a) => a.status === 'done' && a.attachmentId);
    if ((!markdown && doneAttachments.length === 0) || sendMutation.isPending) return;

    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
    sendTyping(false);

    const attachmentIds = doneAttachments.map((a) => a.attachmentId!);

    await sendMutation.mutateAsync({
      content: markdown || ' ',
      is_internal: useSupportInboxStore.getState().replyMode === 'note',
      ...(attachmentIds.length > 0 ? { attachment_ids: attachmentIds } : {}),
    });

    // Clean up preview URLs
    pendingAttachments.forEach((a) => { if (a.previewUrl) URL.revokeObjectURL(a.previewUrl); });
    setPendingAttachments([]);
    editor.commands.clearContent();
    clearDraft(conversationId);
    editor.commands.focus();
  }, [editor, sendMutation, sendTyping, clearDraft, conversationId, pendingAttachments]);

  const handleSend = useCallback(async () => {
    if (!editor) return;
    const markdown = getEditorMarkdown(editor).trim();
    const hasUploadedAttachments = pendingAttachments.some((a) => a.status === 'done' && a.attachmentId);
    if ((!markdown && !hasUploadedAttachments) || sendMutation.isPending) return;

    if (!isNote && emailFallbackHint && !skipOfflineEmailConfirm) {
      setDoNotAskAgain(false);
      setOfflineEmailConfirmOpen(true);
      return;
    }

    await sendReply();
  }, [editor, emailFallbackHint, isNote, pendingAttachments, sendMutation.isPending, sendReply, skipOfflineEmailConfirm]);

  const handleRewrite = useCallback(async (operation: SupportAIRewriteOperation) => {
    if (!editor) return;
    const text = editor.getText().trim();
    if (!text || rewriteMutation.isPending) return;

    const rewritten = await rewriteMutation.mutateAsync({
      content: text,
      operation,
    });

    editor.commands.setContent(rewritten.content);
    editor.commands.focus('end');
  }, [editor, rewriteMutation]);

  const handleConfirmOfflineEmailSend = useCallback(async () => {
    if (doNotAskAgain) {
      saveSkipOfflineEmailConfirm(offlineEmailConfirmStorageKey, true);
      setSkipOfflineEmailConfirm(true);
    }
    setOfflineEmailConfirmOpen(false);
    await sendReply();
  }, [doNotAskAgain, offlineEmailConfirmStorageKey, sendReply]);

  handleSendRef.current = handleSend;

  const openLinkModal = useCallback(() => {
    if (!editor) return;
    const attrs = editor.getAttributes('link') as { href?: string };
    const { from, to, empty } = editor.state.selection;
    let initialLabel = '';
    if (editor.isActive('link')) {
      // Expand selection to entire link mark range
      editor.chain().focus().extendMarkRange('link').run();
      const expanded = editor.state.selection;
      initialLabel = editor.state.doc.textBetween(expanded.from, expanded.to, ' ');
    } else if (!empty) {
      initialLabel = editor.state.doc.textBetween(from, to, ' ');
    }
    setLinkInitial({ label: initialLabel, url: attrs.href ?? '' });
    setLinkModalOpen(true);
  }, [editor]);

  const handleLinkInsert = useCallback((label: string, url: string) => {
    if (!editor) return;
    const chain = editor.chain().focus();
    if (editor.isActive('link')) {
      chain.extendMarkRange('link').unsetLink().run();
    }
    const { from, to, empty } = editor.state.selection;
    if (empty) {
      editor.chain().focus()
        .insertContent({ type: 'text', text: label, marks: [{ type: 'link', attrs: { href: url } }] })
        .run();
    } else {
      editor.chain().focus()
        .insertContentAt({ from, to }, { type: 'text', text: label, marks: [{ type: 'link', attrs: { href: url } }] })
        .run();
    }
  }, [editor]);

  const handleLinkRemove = useCallback(() => {
    if (!editor) return;
    editor.chain().focus().extendMarkRange('link').unsetLink().run();
  }, [editor]);

  if (!editor) return null;

  const content = editor.getText();
  const hasContent = content.trim().length > 0;
  const canUseAITools = hasContent && !rewriteMutation.isPending;
  const aiTools: Array<{ operation: SupportAIRewriteOperation; label: string; icon: typeof ArrowUpDownIcon }> = [
    { operation: 'expand', label: 'Expand', icon: ArrowUpDownIcon },
    { operation: 'rephrase', label: 'Rephrase', icon: ArrowReloadHorizontalIcon },
    { operation: 'fix_grammar', label: 'Fix grammar', icon: TickDouble01Icon },
    { operation: 'more_friendly', label: 'More friendly', icon: SmileIcon },
    { operation: 'more_formal', label: 'More formal', icon: Briefcase01Icon },
  ];

  return (
    <div
      className={cn(
        'relative mx-3 mb-4 rounded-xl border border-border/40 bg-card shadow-lg transition-all',
        isNote && 'bg-amber-50/50 dark:bg-amber-950/10'
      )}
    >
      {emailFallbackHint && !isNote && editor && !editor.isEmpty && (
        <div className="flex items-start gap-2 border-b border-border/20 bg-muted/20 px-4 py-2.5 text-xs text-muted-foreground rounded-t-xl">
          <Mail01Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-blue-500" />
          <p>
            User is offline. Replies sent here will also be queued as an email to{' '}
            <span className="font-medium text-foreground">{emailFallbackHint.email}</span>.
          </p>
        </div>
      )}

      {/* Mention suggestions popover — floats above the composer */}
      {mentionState && mentionState.items.length > 0 && (
        <div className="absolute bottom-full left-0 right-0 z-50 mb-2 px-1">
          <div className="max-h-[260px] overflow-y-auto rounded-xl border border-border/60 bg-popover p-1.5 shadow-lg">
            <p className="px-2 pb-1 pt-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/50">
              Suggestions
            </p>
            <MentionSuggestionsList
              items={mentionState.items}
              selectedIndex={mentionState.selectedIndex}
              compact
              onSelect={(item) => {
                if (!editor) return;
                editor
                  .chain()
                  .focus()
                  .insertContentAt(
                    { from: mentionState.from, to: mentionState.to },
                    `@${item.handle} `,
                  )
                  .run();
                setMentionState(null);
              }}
            />
          </div>
        </div>
      )}

      {(shortcutState || shortcutsPanelOpen) && (() => {
        const items = shortcutState ? shortcutState.items : shortcuts;
        const isFiltering = !!shortcutState;
        const queryToken = shortcutState ? `!${shortcutState.query}` : '';
        return (
          <div className="absolute bottom-full left-0 right-0 z-50 mb-2 px-1">
            <div className="flex max-h-[340px] flex-col overflow-hidden rounded-xl border border-border/60 bg-popover shadow-lg">
              <div className="flex items-center justify-between gap-2 border-b border-border/40 px-3 py-2">
                <div className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                  <StickyNote01Icon className="h-3.5 w-3.5" />
                  <span>Shortcuts</span>
                  {isFiltering && shortcutState ? (
                    <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px] text-foreground">{queryToken}</span>
                  ) : (
                    <span className="text-muted-foreground/60">· browse all</span>
                  )}
                </div>
                <span className="text-[11px] tabular-nums text-muted-foreground/70">
                  {items.length} {items.length === 1 ? 'match' : 'matches'}
                </span>
              </div>

              <div className="flex-1 overflow-y-auto p-1.5">
                {items.length > 0 ? (
                  <ShortcutsList
                    shortcuts={items}
                    selectedIndex={shortcutState?.selectedIndex ?? (isFiltering ? undefined : panelIndex)}
                    query={shortcutState?.query ?? ''}
                    compact
                    onSelect={(shortcut) => insertShortcut(shortcut, shortcutState ? { from: shortcutState.from, to: shortcutState.to } : undefined)}
                  />
                ) : (
                  <div className="flex flex-col items-center gap-2 px-3 py-6 text-center">
                    <div className="rounded-full bg-muted/60 p-2">
                      <StickyNote01Icon className="h-4 w-4 text-muted-foreground" />
                    </div>
                    <p className="text-sm font-medium">
                      {isFiltering ? <>No shortcut matches <span className="font-mono">{queryToken}</span></> : 'No shortcuts yet'}
                    </p>
                    <p className="max-w-xs text-xs text-muted-foreground">
                      Save replies you send often, then trigger them with a bang prefix.
                    </p>
                    <button
                      type="button"
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => {
                        setAddShortcutSeed(queryToken);
                        setShortcutState(null);
                        setShortcutsPanelOpen(false);
                        setAddShortcutOpen(true);
                      }}
                      className="mt-1 inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90"
                    >
                      <PlusSignIcon className="h-3.5 w-3.5" />
                      {isFiltering && shortcutState?.query ? <>Add <span className="font-mono">{queryToken}</span></> : 'Add shortcut'}
                    </button>
                  </div>
                )}
              </div>

              <div className="flex items-center gap-3 border-t border-border/40 bg-muted/30 px-3 py-1.5 text-[10px] text-muted-foreground/80">
                <span className="flex items-center gap-1"><kbd className="rounded border border-border/60 bg-background px-1 font-mono text-[10px]">↑</kbd><kbd className="rounded border border-border/60 bg-background px-1 font-mono text-[10px]">↓</kbd> navigate</span>
                <span className="flex items-center gap-1"><kbd className="rounded border border-border/60 bg-background px-1 font-mono text-[10px]">↵</kbd> insert</span>
                <span className="flex items-center gap-1"><kbd className="rounded border border-border/60 bg-background px-1 font-mono text-[10px]">esc</kbd> close</span>
              </div>
            </div>
          </div>
        );
      })()}

      {/* Mode toggle */}
      <div className="flex items-center gap-1 px-4 pt-3">
        <button
          type="button"
          onClick={() => {
            setReplyMode('reply');
            setShortcutsPanelOpen(false);
          }}
          className={cn(
            'rounded-full px-3 py-1 text-xs font-medium transition-colors',
            !isNote
              ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/25 dark:text-blue-400'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted'
          )}
        >
          Reply
        </button>
        <button
          type="button"
          onClick={() => {
            setReplyMode('note');
            setShortcutsPanelOpen(false);
          }}
          className={cn(
            'rounded-full px-3 py-1 text-xs font-medium transition-colors',
            isNote
              ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-400'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted'
          )}
        >
          Note
        </button>
        <button
          type="button"
          onClick={() => {
            setShortcutsPanelOpen((open) => !open);
            setShortcutState(null);
            setPanelIndex(0);
            setReplyMode('reply');
          }}
          className={cn(
            'flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors',
            shortcutsPanelOpen
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:bg-muted hover:text-foreground',
          )}
        >
          <Copy01Icon className="h-3 w-3" />
          Shortcuts
        </button>
        <DropdownMenu>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex">
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={!canUseAITools}
                    className={cn(
                      'h-auto rounded-full px-3 py-1 text-xs font-medium transition-colors',
                      canUseAITools
                        ? 'text-muted-foreground hover:bg-muted hover:text-foreground'
                        : 'text-muted-foreground/50'
                    )}
                  >
                    {rewriteMutation.isPending ? (
                      <Loading01Icon className="h-3 w-3 animate-spin" />
                    ) : (
                      <SparklesIcon className="h-3 w-3" />
                    )}
                    AI Tools
                    <ArrowUp01Icon className="h-3 w-3 rotate-180" />
                  </Button>
                </DropdownMenuTrigger>
              </span>
            </TooltipTrigger>
            {!canUseAITools && (
              <TooltipContent side="top" className="text-xs">Write something first to use AI tools</TooltipContent>
            )}
          </Tooltip>
          <DropdownMenuContent align="start" className="w-48">
            {aiTools.slice(0, 3).map((tool) => {
              const Icon = tool.icon;
              return (
                <DropdownMenuItem key={tool.operation} onSelect={() => { void handleRewrite(tool.operation); }}>
                  <Icon className="h-4 w-4" />
                  <span>{tool.label}</span>
                </DropdownMenuItem>
              );
            })}
            <DropdownMenuSeparator />
            {aiTools.slice(3).map((tool) => {
              const Icon = tool.icon;
              return (
                <DropdownMenuItem key={tool.operation} onSelect={() => { void handleRewrite(tool.operation); }}>
                  <Icon className="h-4 w-4" />
                  <span>{tool.label}</span>
                </DropdownMenuItem>
              );
            })}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* TipTap Editor */}
      <div
        className="px-4 py-3"
        onClickCapture={(e) => {
          const target = e.target as HTMLElement | null;
          const anchor = target?.closest('a');
          if (anchor) {
            e.preventDefault();
            e.stopPropagation();
            openLinkModal();
          }
        }}
      >
        <EditorContent editor={editor} />
      </div>

      <LinkInsertModal
        open={linkModalOpen}
        onOpenChange={setLinkModalOpen}
        workspaceId={workspaceId}
        initialLabel={linkInitial.label}
        initialUrl={linkInitial.url}
        onInsert={handleLinkInsert}
        onRemove={editor.isActive('link') ? handleLinkRemove : undefined}
      />

      <AddShortcutDialog
        open={addShortcutOpen}
        workspaceId={workspaceId}
        seedShortCode={addShortcutSeed}
        onOpenChange={setAddShortcutOpen}
      />

      {/* Attachment preview strip */}
      {pendingAttachments.length > 0 && (
        <div className="flex gap-2 overflow-x-auto px-4 pb-2">
          {pendingAttachments.map((att) => (
            <div key={att.localId} className="relative flex-shrink-0">
              {att.previewUrl ? (
                <img src={att.previewUrl} alt={att.fileName} className="h-14 w-14 rounded-lg object-cover border border-border" />
              ) : (
                <div className="flex h-14 w-14 flex-col items-center justify-center rounded-lg border border-border bg-muted px-1">
                  <AttachmentIcon className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="mt-0.5 max-w-[48px] truncate text-[8px] text-muted-foreground">{att.fileName}</span>
                </div>
              )}
              {att.status === 'uploading' && (
                <div className="absolute inset-0 flex items-center justify-center rounded-lg bg-black/40">
                  <Loading01Icon className="h-4 w-4 animate-spin text-white" />
                </div>
              )}
              {att.status === 'error' && (
                <div className="absolute inset-0 flex items-center justify-center rounded-lg bg-red-500/20 border border-red-400">
                  <span className="text-[9px] font-medium text-red-600">Failed</span>
                </div>
              )}
              <button
                type="button"
                onClick={() => removeAttachment(att.localId)}
                className="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-foreground/80 text-background hover:bg-foreground"
              >
                <Cancel01Icon className="h-2.5 w-2.5" />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Bottom toolbar — formatting + actions in one row */}
      <div
        className="flex items-center justify-between px-4 pb-3"
        onMouseEnter={() => { toolbarHasPointerRef.current = true; }}
        onMouseLeave={() => {
          toolbarHasPointerRef.current = false;
          if (!editor.isFocused) setEditorFocused(false);
        }}
      >
        <div className="flex items-center gap-0.5">
          <EmojiPicker
            onEmojiSelect={(emoji) => {
              if (editorRef.current) {
                editorRef.current.chain().focus().insertContent(emoji).run();
              }
            }}
          />
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground/60 hover:text-foreground transition-colors" onClick={() => fileInputRef.current?.click()}>
                <AttachmentIcon className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Attach file</TooltipContent>
          </Tooltip>
          <input
            ref={fileInputRef}
            type="file"
            className="hidden"
            multiple
            accept="image/*,.pdf,.doc,.docx,.txt,.csv,.xls,.xlsx,.zip,.gz,.tar,.md"
            onChange={(e) => handleFileSelect(e.target.files)}
          />
          <div className="mx-0.5 h-4 w-px bg-border/40" />
          <FormatButton
            title="Bold (Ctrl+B)"
            active={editor.isActive('bold')}
            onClick={() => editor.chain().focus().toggleBold().run()}
          >
            <TextBoldIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Italic (Ctrl+I)"
            active={editor.isActive('italic')}
            onClick={() => editor.chain().focus().toggleItalic().run()}
          >
            <TextItalicIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Underline (Ctrl+U)"
            active={editor.isActive('underline')}
            onClick={() => editor.chain().focus().toggleUnderline().run()}
          >
            <TextUnderlineIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Insert link"
            active={editor.isActive('link')}
            onClick={openLinkModal}
          >
            <Link01Icon className="h-3.5 w-3.5" />
          </FormatButton>
          <div className="mx-0.5 h-4 w-px bg-border/40" />
          <FormatButton
            title="Bullet list"
            active={editor.isActive('bulletList')}
            onClick={() => editor.chain().focus().toggleBulletList().run()}
          >
            <LeftToRightListBulletIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Numbered list"
            active={editor.isActive('orderedList')}
            onClick={() => editor.chain().focus().toggleOrderedList().run()}
          >
            <LeftToRightListNumberIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Quote"
            active={editor.isActive('blockquote')}
            onClick={() => editor.chain().focus().toggleBlockquote().run()}
          >
            <QuoteDownIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <div className="mx-0.5 h-4 w-px bg-border/40" />
          <FormatButton
            title="Inline code"
            active={editor.isActive('code')}
            onClick={() => editor.chain().focus().toggleCode().run()}
          >
            <CodeIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Strikethrough"
            active={editor.isActive('strike')}
            onClick={() => editor.chain().focus().toggleStrike().run()}
          >
            <TextStrikethroughIcon className="h-3.5 w-3.5" />
          </FormatButton>
        </div>

        <div className="flex items-center gap-2">
          <kbd className="hidden text-xs leading-none text-muted-foreground sm:inline">
            {navigator.platform?.includes('Mac') ? '\u2318' : 'Ctrl'}{'\u21B5'}
          </kbd>
          <Button
            size="sm"
            disabled={sendMutation.isPending || (!content.trim() && !pendingAttachments.some((a) => a.status === 'done'))}
            onClick={handleSend}
            className={cn(
              'h-7 gap-1.5 rounded-full px-3 text-xs',
              isNote && 'bg-amber-500 hover:bg-amber-600 text-white'
            )}
          >
            <SentIcon className="h-3 w-3" />
            {isNote ? 'Add Note' : 'Send'}
          </Button>
        </div>
      </div>

      <AlertDialog open={offlineEmailConfirmOpen} onOpenChange={setOfflineEmailConfirmOpen}>
        <AlertDialogContent
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            window.requestAnimationFrame(() => {
              document.getElementById('offline-email-confirm-send')?.focus();
            });
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>Send this reply by email too?</AlertDialogTitle>
            <AlertDialogDescription>
              This visitor is currently offline. If you send this reply, Helpin will queue an email to{' '}
              <span className="font-medium text-foreground">{emailFallbackHint?.email}</span> and skip the email if the visitor comes back online before it sends.
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className="flex items-center gap-2">
            <Checkbox
              id="offline-email-dont-ask"
              checked={doNotAskAgain}
              onCheckedChange={(checked) => setDoNotAskAgain(!!checked)}
            />
            <Label htmlFor="offline-email-dont-ask" className="text-sm font-normal">
              Do not ask me again
            </Label>
          </div>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={sendMutation.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              id="offline-email-confirm-send"
              variant="default"
              disabled={sendMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                void handleConfirmOfflineEmailSend();
              }}
            >
              {sendMutation.isPending ? 'Sending...' : 'Send Reply'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
