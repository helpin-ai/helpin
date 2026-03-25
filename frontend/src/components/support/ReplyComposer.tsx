import { useRef, useEffect, useState, useCallback, useMemo } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import { Send, Paperclip, StickyNote, MessageCircle, X as XIcon, Loader2, Mail } from 'lucide-react';
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
import { Label } from '@/components/ui/label';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { MentionHighlight } from '@/components/pm/mention-highlight';
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList';
import { getMentionSuggestions, type MentionSuggestionItem } from '@/components/pm/mentionSuggestions';
import { useSendMessage, useUploadSupportAttachment } from '@/hooks/queries/useSupport';
import { queryKeys } from '@/lib/queryKeys';
import { workspacesService } from '@/lib/services/workspacesService';
import { unwrap } from '@/lib/queryUtils';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useAuthStore } from '@/stores/authStore';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import type { AssignableMember } from '@/lib/types';
import { EmojiPicker } from './EmojiPicker';

const OFFLINE_EMAIL_CONFIRM_STORAGE_PREFIX = 'support_offline_email_confirm';

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

export function ReplyComposer({ workspaceId, conversationId, emailFallbackHint }: ReplyComposerProps) {
  const { replyMode, setReplyMode, setDraft, clearDraft } = useSupportInboxStore();
  const sendMutation = useSendMessage(workspaceId, conversationId);
  const userId = useAuthStore((s) => s.user?.id ?? null);

  const { data: members = [] } = useQuery({
    queryKey: [...queryKeys.workspaces.members(workspaceId), 'assignable'],
    queryFn: async () => unwrap(await workspacesService.listAssignableMembers(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });

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
    isTypingRef.current = true;
    lastTypingSentRef.current = now;
    wsSend(isTypingRef.current ? 'support:typing:update' : 'support:typing:start', {
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
      blockquote: false,
      codeBlock: false,
      horizontalRule: false,
      bulletList: false,
      orderedList: false,
      listItem: false,
    }),
    Placeholder.configure({
      placeholder: () => isNoteRef.current ? 'Add an internal note...' : 'Write a reply...',
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

        return false;
      },
    },
    onUpdate: ({ editor: ed }) => {
      const text = ed.getText();

      // Debounce draft save
      if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
      draftTimerRef.current = setTimeout(() => setDraft(conversationId, text), 500);

      // Typing indicator
      text.trim() ? handleTyping(text) : sendTyping(false);

      // Mention detection
      const mention = detectMentions(ed, membersRef.current);
      setMentionState(mention);

      // Auto-switch to note mode when mention detected in reply mode
      if (mention && useSupportInboxStore.getState().replyMode === 'reply') {
        setReplyMode('note');
      }
    },
    onBlur: () => setMentionState(null),
  });

  const editorRef = useRef(editor);
  editorRef.current = editor;

  // Backup event handlers for TipTap v3 compatibility
  useEffect(() => {
    if (!editor) return;

    const handleUpdate = () => {
      const text = editor.getText();
      if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
      draftTimerRef.current = setTimeout(() => setDraft(conversationId, text), 500);
      text.trim() ? handleTyping(text) : sendTyping(false);
      const mention = detectMentions(editor, membersRef.current);
      setMentionState(mention);
      if (mention && useSupportInboxStore.getState().replyMode === 'reply') {
        setReplyMode('note');
      }
    };
    const handleBlur = () => setMentionState(null);

    editor.on('update', handleUpdate);
    editor.on('blur', handleBlur);

    return () => {
      editor.off('update', handleUpdate);
      editor.off('blur', handleBlur);
    };
  }, [editor, conversationId, handleTyping, sendTyping, setDraft, setReplyMode]);

  // Load draft when switching conversations
  useEffect(() => {
    if (!editor) return;
    const saved = useSupportInboxStore.getState().drafts[conversationId] ?? '';
    if (saved) {
      const html = saved.split('\n').map(line => `<p>${line || '<br>'}</p>`).join('');
      editor.commands.setContent(html);
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

  // Force placeholder redecoration when mode changes
  useEffect(() => {
    if (editor && editor.isEmpty) {
      editor.view.dispatch(editor.state.tr);
    }
  }, [editor, isNote]);

  const sendReply = useCallback(async () => {
    if (!editor) return;
    const text = editor.getText().trim();
    const doneAttachments = pendingAttachments.filter((a) => a.status === 'done' && a.attachmentId);
    if ((!text && doneAttachments.length === 0) || sendMutation.isPending) return;

    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
    sendTyping(false);

    const attachmentIds = doneAttachments.map((a) => a.attachmentId!);

    await sendMutation.mutateAsync({
      content: text || ' ',
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
    const text = editor.getText().trim();
    const hasUploadedAttachments = pendingAttachments.some((a) => a.status === 'done' && a.attachmentId);
    if ((!text && !hasUploadedAttachments) || sendMutation.isPending) return;

    if (!isNote && emailFallbackHint && !skipOfflineEmailConfirm) {
      setDoNotAskAgain(false);
      setOfflineEmailConfirmOpen(true);
      return;
    }

    await sendReply();
  }, [editor, emailFallbackHint, isNote, pendingAttachments, sendMutation.isPending, sendReply, skipOfflineEmailConfirm]);

  const handleConfirmOfflineEmailSend = useCallback(async () => {
    if (doNotAskAgain) {
      saveSkipOfflineEmailConfirm(offlineEmailConfirmStorageKey, true);
      setSkipOfflineEmailConfirm(true);
    }
    setOfflineEmailConfirmOpen(false);
    await sendReply();
  }, [doNotAskAgain, offlineEmailConfirmStorageKey, sendReply]);

  handleSendRef.current = handleSend;

  if (!editor) return null;

  const content = editor.getText();

  return (
    <div
      className={cn(
        'relative border-t transition-colors',
        isNote && 'border-l-2 border-l-amber-400 bg-amber-50/50 dark:bg-amber-950/10'
      )}
    >
      {emailFallbackHint && !isNote && (
        <div className="flex items-start gap-2 border-b border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          <Mail className="mt-0.5 h-3.5 w-3.5 shrink-0 text-blue-500" />
          <p>
            User is offline. Replies sent here will also be queued as an email to{' '}
            <span className="font-medium text-foreground">{emailFallbackHint.email}</span>.
          </p>
        </div>
      )}

      {/* Mention suggestions popover — floats above the composer */}
      {mentionState && mentionState.items.length > 0 && (
        <div className="absolute bottom-full left-0 right-0 z-50 mb-1 px-3">
          <div className="max-h-[240px] overflow-y-auto rounded-lg border border-border/60 bg-background px-2 py-2 shadow-md">
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

      {/* Mode toggle */}
      <div className="flex items-center gap-0.5 px-3 pt-2.5">
        <button
          type="button"
          onClick={() => setReplyMode('reply')}
          className={cn(
            'flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors',
            !isNote
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted'
          )}
        >
          <MessageCircle className="h-3 w-3" />
          Reply
        </button>
        <button
          type="button"
          onClick={() => setReplyMode('note')}
          className={cn(
            'flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors',
            isNote
              ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-400'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted'
          )}
        >
          <StickyNote className="h-3 w-3" />
          Note
        </button>
      </div>

      {/* TipTap Editor */}
      <div className="px-3 py-1.5">
        <EditorContent editor={editor} />
      </div>

      {/* Attachment preview strip */}
      {pendingAttachments.length > 0 && (
        <div className="flex gap-2 overflow-x-auto px-3 pb-1.5">
          {pendingAttachments.map((att) => (
            <div key={att.localId} className="relative flex-shrink-0">
              {att.previewUrl ? (
                <img src={att.previewUrl} alt={att.fileName} className="h-14 w-14 rounded-lg object-cover border border-border" />
              ) : (
                <div className="flex h-14 w-14 flex-col items-center justify-center rounded-lg border border-border bg-muted px-1">
                  <Paperclip className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="mt-0.5 max-w-[48px] truncate text-[8px] text-muted-foreground">{att.fileName}</span>
                </div>
              )}
              {att.status === 'uploading' && (
                <div className="absolute inset-0 flex items-center justify-center rounded-lg bg-black/40">
                  <Loader2 className="h-4 w-4 animate-spin text-white" />
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
                <XIcon className="h-2.5 w-2.5" />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Bottom toolbar */}
      <div className="flex items-center justify-between px-3 pb-2.5">
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
              <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground" onClick={() => fileInputRef.current?.click()}>
                <Paperclip className="h-4 w-4" />
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
          {members.length > 0 && (
            <span className="ml-1 text-[10px] text-muted-foreground">
              Type @ to mention
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <kbd className="hidden text-[10px] text-muted-foreground/50 sm:inline">
            {navigator.platform?.includes('Mac') ? '\u2318' : 'Ctrl'}{'+\u21B5'}
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
            <Send className="h-3 w-3" />
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
