import { useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import type { Message } from '@helpin-ai/widget-core';
import { unwrap } from '@/lib/queryUtils';
import { supportQuickstartService, type SupportPreviewAnswer } from '@/lib/services/supportQuickstartService';
import { prefersReducedMotion } from './useSupportPreview';

export type PreviewConversation = SupportPreviewAnswer & { from: 'agent' | 'gap' | 'visitor' };

const HANDOFF_REPLY = 'I don’t see that in your help center yet, so I’d pass this to your team with the conversation so far.';
/** How often the next word of an answer appears. */
const WORD_MS = 40;
const NONE: never[] = [];

/**
 * What the preview widget is showing: one of the questions the agent answered
 * from the company's articles, one it found no article for, or one the visitor
 * typed, with the reply arriving a few words at a time.
 */
export function useAgentPreview({ websiteUrl, helpUrl, answers = NONE, gaps = NONE }: {
  websiteUrl: string;
  helpUrl: string;
  answers?: SupportPreviewAnswer[];
  gaps?: string[];
}) {
  const [asked, setAsked] = useState<PreviewConversation[]>([]);
  /** The chosen question; the first one until the visitor picks or asks another. */
  const [selected, setSelected] = useState<string>();
  const [openedAt] = useState(() => Date.now());
  const ask = useMutation({
    mutationFn: async (text: string) => unwrap(await supportQuickstartService.previewAnswer({
      website_url: websiteUrl, help_center_url: helpUrl || undefined, question: text,
    })),
    onSuccess: (result, text) => {
      setAsked((previous) => [...previous, { ...result, question: text, from: 'visitor' }]);
      setSelected(text);
    },
  });

  const conversations: PreviewConversation[] = [
    ...answers.map((answer) => ({ ...answer, from: 'agent' as const })),
    ...gaps.map((gap) => ({ available: true, question: gap, can_answer: false, sources: [], from: 'gap' as const })),
    ...asked,
  ];
  // While a typed question is being answered, the widget shows it waiting.
  const pending = ask.isPending ? ask.variables : undefined;
  const current = pending ? undefined : conversations.find((item) => item.question === selected) ?? conversations[0];
  const answered = Boolean(current?.can_answer && current.answer);
  const reply = current ? (answered ? current.answer! : HANDOFF_REPLY) : '';
  const shown = useStreamed(reply);

  const messages = useMemo<Message[]>(() => {
    // The widget orders messages by time: its own greeting, then the question, then the reply.
    const message = (id: string, role: Message['role'], content: string): Message => ({
      id, conversationId: 'preview', role, content, isInternal: false,
      createdAt: new Date(openedAt + (role === 'customer' ? 5000 : 6000)).toISOString(),
    });
    if (pending) return [message('pending', 'customer', pending)];
    if (!current) return [];
    const done = shown.length >= reply.length;
    return [
      message(`question-${current.question}`, 'customer', current.question),
      {
        ...message(`answer-${current.question}`, 'ai', shown),
        isStreaming: !done,
        sources: done && answered
          ? current.sources.map((source) => ({ url: source.url, docId: source.url, title: source.title, snippet: '', confidence: 1, language: 'en' }))
          : undefined,
      },
    ];
    // `current` is a fresh object each render; its question and reply identify it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pending, current?.question, shown, reply, answered, openedAt]);

  const { isPending, mutate } = ask;
  const questions = conversations.map((item) => item.question).join('\n');
  const send = useCallback((content: string) => {
    // The preview endpoint takes questions up to 300 characters.
    const question = content.trim().slice(0, 300);
    if (!question || isPending) return;
    // A question already on the list is shown again, not asked again.
    if (questions.split('\n').includes(question)) setSelected(question);
    else mutate(question);
  }, [isPending, mutate, questions]);

  return { conversations, current, pending, select: setSelected, messages, send, thinking: ask.isPending, error: ask.error?.message };
}

export type AgentPreview = ReturnType<typeof useAgentPreview>;

/**
 * An answer a few words at a time, the way the widget shows a reply as it
 * arrives. An answer already shown once comes back whole.
 */
function useStreamed(text: string) {
  const [seen, setSeen] = useState<string[]>([]);
  const [reveal, setReveal] = useState({ text: '', length: 0 });
  const whole = !text || seen.includes(text) || prefersReducedMotion();

  useEffect(() => {
    if (whole) return;
    let length = 0;
    const timer = setInterval(() => {
      const space = text.indexOf(' ', length + 1);
      length = space === -1 ? text.length : space;
      setReveal({ text, length });
      if (length >= text.length) {
        clearInterval(timer);
        setSeen((previous) => [...previous, text]);
      }
    }, WORD_MS);
    return () => clearInterval(timer);
  }, [text, whole]);

  if (whole) return text;
  return reveal.text === text ? text.slice(0, reveal.length) : '';
}
