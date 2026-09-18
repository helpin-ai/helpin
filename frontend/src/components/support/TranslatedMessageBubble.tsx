import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { QuietTextAction } from '@/components/design-system/quiet';
import {
  requestSupportTranslation,
  useSupportTranslationOptions,
} from '@/hooks/queries/useSupportTranslation';
import { MessageBubble, type MessageBubbleProps } from './MessageBubble';

export function TranslatedMessageBubble({
  workspaceId,
  ...props
}: MessageBubbleProps & { workspaceId: string }) {
  const { message } = props;
  const options = useSupportTranslationOptions(workspaceId, message.conversation_id).data;
  const client = useQueryClient();
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  const [original, setOriginal] = useState(false);
  useEffect(() => {
    if (!ref.current) return;
    if (typeof IntersectionObserver === 'undefined') {
      setVisible(true);
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => setVisible(entries.some((e) => e.isIntersecting)),
      { rootMargin: '100px' },
    );
    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);
  const enabled =
    !!options?.available &&
    options.preference.auto_translate_incoming &&
    message.sender_type === 'customer' &&
    !message.is_internal &&
    message.message_type === 'reply' &&
    !!message.content.trim();
  const language = options?.preference.reading_language || 'en';
  const query = useQuery({
    queryKey: ['support', workspaceId, 'message-translation', message.id, message.content, language],
    enabled: enabled && visible,
    queryFn: () =>
      requestSupportTranslation(workspaceId, message.conversation_id, {
        message_id: message.id,
        target_language: language,
      }),
    staleTime: Infinity,
    retry: false,
    refetchInterval: (q) =>
      q.state.data?.status === 'pending' && Date.now() - Date.parse(q.state.data.created_at) < 65_000
        ? 2000
        : false,
  });
  const result = query.data;
  useEffect(() => {
    if (result?.status === 'ready')
      void client.invalidateQueries({
        queryKey: ['support', workspaceId, 'translation', message.conversation_id],
      });
  }, [client, result?.id, result?.status, workspaceId, message.conversation_id]);
  const translated =
    enabled &&
    result?.status === 'ready' &&
    result.source_text === message.content &&
    result.translated_text !== message.content;
  return (
    <div ref={ref}>
      <MessageBubble
        {...props}
        translatedContent={translated && !original ? result.translated_text : undefined}
      />
      {enabled && (
        <div
          className="mb-2 ml-9 flex flex-wrap items-center gap-2 text-xs text-quiet-text-secondary"
          aria-live="polite"
        >
          {translated ? (
            <>
              <span>Translated from {options?.languages[result.source_language] || 'detected language'}</span>
              <QuietTextAction type="button" onClick={() => setOriginal(!original)}>
                {original ? 'Show translation' : 'Show original'}
              </QuietTextAction>
            </>
          ) : visible && (query.isFetching || result?.status === 'pending') ? (
            <span>Translating…</span>
          ) : query.isError || result?.status === 'failed' || result?.status === 'expired' ? (
            <>
              <span>Translation unavailable. Original shown.</span>
              <QuietTextAction type="button" disabled={query.isFetching} onClick={() => void query.refetch()}>
                Retry
              </QuietTextAction>
            </>
          ) : null}
        </div>
      )}
    </div>
  );
}
