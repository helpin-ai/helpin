import { useEffect, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { Bot, HelpCircle, Loader2, Send, Wrench, ChevronDown, ChevronRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import type { PlanningSessionMessage, ToolInvocation } from '@/lib/pmTypes';
import { parseStructuredQuestions } from './parseStructuredQuestions';
import { StructuredQuestionCard } from './StructuredQuestionCard';

interface Props {
  messages: PlanningSessionMessage[];
  isStreaming: boolean;
  agentBusy: boolean;
  activeToolCall: { tool_name: string } | null;
  toolResults: ToolInvocation[];
  onSend: (content: string) => void;
  sending: boolean;
}

function ToolCallBadge({ invocation }: { invocation: ToolInvocation }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="mt-1">
      <button
        type="button"
        className="inline-flex items-center gap-1 rounded-md border border-border/60 bg-muted/40 px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-muted/60"
        onClick={() => setExpanded(!expanded)}
      >
        <Wrench className="h-2.5 w-2.5" />
        {invocation.tool_name}
        {invocation.duration_ms > 0 && <span className="opacity-60">{invocation.duration_ms}ms</span>}
        {expanded ? <ChevronDown className="h-2.5 w-2.5" /> : <ChevronRight className="h-2.5 w-2.5" />}
      </button>
      {expanded && invocation.output_summary && (
        <pre className="mt-1 max-h-32 overflow-auto whitespace-pre-wrap rounded border border-border/40 bg-muted/20 p-1.5 text-[10px] text-muted-foreground">
          {invocation.output_summary}
        </pre>
      )}
    </div>
  );
}

function stripXmlTags(text: string): string {
  return text
    .replace(/<spec_draft>[\s\S]*?<\/spec_draft>/g, '')
    .replace(/<questions>[\s\S]*?<\/questions>/g, '')
    .trim();
}

interface MessageBubbleProps {
  message: PlanningSessionMessage;
  isAnswered: boolean;
  onSend: (content: string) => void;
  disabled: boolean;
}

function MessageBubble({ message, isAnswered, onSend, disabled }: MessageBubbleProps) {
  const isUser = message.role === 'user';
  const isProposal = message.message_type === 'proposal';
  const isQuestion = message.message_type === 'question';

  // Parse structured questions from assistant messages.
  const parsed = !isUser ? parseStructuredQuestions(message.content) : null;
  const displayContent = isUser ? message.content : stripXmlTags(message.content);

  return (
    <div className={cn('flex gap-2', isUser ? 'justify-end' : 'justify-start')}>
      {!isUser && (
        <div className="mt-1 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted">
          {isQuestion ? (
            <HelpCircle className="h-3.5 w-3.5 text-amber-500" />
          ) : (
            <Bot className="h-3.5 w-3.5 text-muted-foreground" />
          )}
        </div>
      )}
      <div
        className={cn(
          'max-w-[85%] rounded-lg px-3 py-2 text-xs',
          isUser
            ? 'bg-primary text-primary-foreground'
            : isProposal
              ? 'border-2 border-amber-300 bg-amber-50/50 dark:border-amber-700 dark:bg-amber-950/20'
              : 'bg-muted/60',
        )}
      >
        {isUser ? (
          <div className="whitespace-pre-wrap break-words">{displayContent}</div>
        ) : (
          <>
            {displayContent && (
              <div className="prose prose-xs dark:prose-invert max-w-none">
                <ReactMarkdown>{displayContent}</ReactMarkdown>
              </div>
            )}
            {parsed && (
              <StructuredQuestionCard
                questions={parsed.questions}
                onSubmit={onSend}
                disabled={disabled}
                readOnly={isAnswered}
              />
            )}
          </>
        )}
        {message.tool_invocations && message.tool_invocations.length > 0 && (
          <div className="mt-2 space-y-0.5">
            {message.tool_invocations.map((inv, i) => (
              <ToolCallBadge key={`${inv.tool_name}-${i}`} invocation={inv} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export function PlanningChat({
  messages,
  isStreaming,
  agentBusy,
  activeToolCall,
  toolResults,
  onSend,
  sending,
}: Props) {
  const [input, setInput] = useState('');
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Auto-scroll only when the user is already near the bottom.
  useEffect(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 150;
    if (nearBottom) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [messages, activeToolCall, agentBusy]);

  const handleSubmit = () => {
    const trimmed = input.trim();
    if (!trimmed || agentBusy || sending) return;
    onSend(trimmed);
    setInput('');
    textareaRef.current?.focus();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className="flex h-full flex-col">
      {/* Message list */}
      <div ref={scrollContainerRef} className="min-h-0 flex-1 overflow-y-auto px-3 py-3">
        <div className="space-y-3">
          {messages.map((msg, idx) => {
            // A question is "answered" if the next message is from the user.
            const nextMsg = messages[idx + 1];
            const isAnswered = msg.role === 'assistant' && nextMsg?.role === 'user';
            return (
              <MessageBubble
                key={msg.id}
                message={msg}
                isAnswered={isAnswered}
                onSend={onSend}
                disabled={agentBusy || sending}
              />
            );
          })}

          {/* Agent busy / streaming bubble */}
          {(agentBusy || isStreaming || toolResults.length > 0 || activeToolCall) && (
            <div className="flex gap-2">
              <div className="mt-1 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted">
                <Bot className="h-3.5 w-3.5 text-muted-foreground" />
              </div>
              <div className="max-w-[85%] rounded-lg bg-muted/60 px-3 py-2 text-xs">
                {/* Completed tool results */}
                {toolResults.map((tr, i) => (
                  <ToolCallBadge key={`${tr.tool_name}-${i}`} invocation={tr} />
                ))}

                {/* Active tool call */}
                {activeToolCall && (
                  <div className="mt-1 inline-flex items-center gap-1.5 text-[11px] text-muted-foreground">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Running {activeToolCall.tool_name}...
                  </div>
                )}

                {/* Waiting for agent (no tokens yet) */}
                {!activeToolCall && toolResults.length === 0 && (
                  <div className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Thinking...
                  </div>
                )}
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>
      </div>

      {/* Input */}
      <div className="border-t p-3">
        <div className="flex gap-2">
          <Textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask a question or provide guidance... (Ctrl+Enter to send)"
            rows={2}
            className="min-h-[52px] resize-none text-sm"
            disabled={agentBusy || sending}
          />
          <Button
            size="icon"
            onClick={handleSubmit}
            disabled={!input.trim() || agentBusy || sending}
            className="h-[52px] w-10 shrink-0"
          >
            {sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
          </Button>
        </div>
      </div>
    </div>
  );
}
