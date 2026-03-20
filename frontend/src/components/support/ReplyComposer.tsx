import { useRef, useEffect, useState, useCallback, useMemo } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import { Send, Smile, Paperclip, StickyNote, MessageCircle } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { MentionHighlight } from '@/components/pm/mention-highlight';
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList';
import { getMentionSuggestions, type MentionSuggestionItem } from '@/components/pm/mentionSuggestions';
import { useSendMessage } from '@/hooks/queries/useSupport';
import { queryKeys } from '@/lib/queryKeys';
import { workspacesService } from '@/lib/services/workspacesService';
import { unwrap } from '@/lib/queryUtils';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { cn } from '@/lib/utils';
import type { AssignableMember } from '@/lib/types';

interface ReplyComposerProps {
  workspaceId: string;
  conversationId: string;
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

export function ReplyComposer({ workspaceId, conversationId }: ReplyComposerProps) {
  const { replyMode, setReplyMode, setDraft, clearDraft } = useSupportInboxStore();
  const sendMutation = useSendMessage(workspaceId, conversationId);

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

  const handleSend = useCallback(async () => {
    if (!editor) return;
    const text = editor.getText().trim();
    if (!text || sendMutation.isPending) return;

    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
    sendTyping(false);

    await sendMutation.mutateAsync({
      content: text,
      is_internal: useSupportInboxStore.getState().replyMode === 'note',
    });

    editor.commands.clearContent();
    clearDraft(conversationId);
    editor.commands.focus();
  }, [editor, sendMutation, sendTyping, clearDraft, conversationId]);

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

      {/* Bottom toolbar */}
      <div className="flex items-center justify-between px-3 pb-2.5">
        <div className="flex items-center gap-0.5">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground">
                <Smile className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Emoji</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground">
                <Paperclip className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Attach file</TooltipContent>
          </Tooltip>
          {members.length > 0 && (
            <span className="ml-1 text-[10px] text-muted-foreground">
              Type @ to mention
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <kbd className="hidden text-[10px] text-muted-foreground/50 sm:inline">
            {navigator.platform?.includes('Mac') ? '\u2318' : 'Ctrl'}+\u21B5
          </kbd>
          <Button
            size="sm"
            disabled={sendMutation.isPending || !content.trim()}
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
    </div>
  );
}
