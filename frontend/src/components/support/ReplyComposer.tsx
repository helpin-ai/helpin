import { useRef, useEffect, useState, useCallback } from 'react';
import { Send, Smile, Paperclip, StickyNote, MessageCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useSendMessage } from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { supportService } from '@/lib/services/supportService';
import { cn } from '@/lib/utils';

interface ReplyComposerProps {
  workspaceId: string;
  conversationId: string;
}

export function ReplyComposer({ workspaceId, conversationId }: ReplyComposerProps) {
  const [content, setContent] = useState('');
  const { replyMode, setReplyMode } = useSupportInboxStore();
  const sendMutation = useSendMessage(workspaceId, conversationId);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isTypingRef = useRef(false);
  const isNote = replyMode === 'note';

  // Send typing indicator with debounce (fire once per 3s, auto-stop after 5s of inactivity)
  const sendTyping = useCallback((typing: boolean) => {
    if (isNote) return; // don't send typing for internal notes
    if (typing === isTypingRef.current) return;
    isTypingRef.current = typing;
    supportService.sendTypingIndicator(workspaceId, conversationId, typing).catch(() => {});
  }, [workspaceId, conversationId, isNote]);

  const handleTyping = useCallback(() => {
    sendTyping(true);
    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    typingTimerRef.current = setTimeout(() => sendTyping(false), 5000);
  }, [sendTyping]);

  // Clean up typing indicator on unmount or conversation change
  useEffect(() => {
    return () => {
      if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
      if (isTypingRef.current) {
        isTypingRef.current = false;
        supportService.sendTypingIndicator(workspaceId, conversationId, false).catch(() => {});
      }
    };
  }, [workspaceId, conversationId]);

  // Auto-resize textarea
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = '0';
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
  }, [content]);

  const handleSend = async () => {
    if (!content.trim() || sendMutation.isPending) return;
    // Stop typing indicator before sending
    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    sendTyping(false);
    await sendMutation.mutateAsync({
      content: content.trim(),
      is_internal: isNote,
    });
    setContent('');
    textareaRef.current?.focus();
  };

  return (
    <div
      className={cn(
        'border-t transition-colors',
        isNote && 'border-l-2 border-l-amber-400 bg-amber-50/50 dark:bg-amber-950/10'
      )}
    >
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

      {/* Textarea */}
      <div className="px-3 py-1.5">
        <textarea
          ref={textareaRef}
          className={cn(
            'w-full resize-none bg-transparent text-sm leading-relaxed placeholder:text-muted-foreground/50 focus:outline-none',
            'min-h-[40px] max-h-[160px]'
          )}
          placeholder={isNote ? 'Add an internal note...' : 'Write a reply...'}
          value={content}
          onChange={(e) => { setContent(e.target.value); handleTyping(); }}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              handleSend();
            }
          }}
          rows={1}
        />
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
        </div>

        <div className="flex items-center gap-2">
          <kbd className="hidden text-[10px] text-muted-foreground/50 sm:inline">
            {navigator.platform?.includes('Mac') ? '⌘' : 'Ctrl'}+↵
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
