import { useMemo, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { QuietTextAction } from '@/components/design-system/quiet';
import { requestSupportTranslation, useSupportTranslationOptions } from '@/hooks/queries/useSupportTranslation';
import { isMessageInOtherLanguage, isMessageInReadingLanguage, normalizeReadingLanguage } from './messageReadingLanguage';
import type { SupportTranslation } from '@/lib/pmTypes';
import { MessageBubble, type MessageBubbleProps } from './MessageBubble';

export function TranslatedMessageBubble({ workspaceId, cachedTranslation, ...props }: MessageBubbleProps & {
  workspaceId: string;
  cachedTranslation?: SupportTranslation;
}) {
  const { message } = props;
  const options = useSupportTranslationOptions(workspaceId, message.conversation_id).data;
  const language = options?.preference.reading_language || 'en';
  const [original, setOriginal] = useState(false);
  const client = useQueryClient();
  const request = useMutation({
    mutationFn: () => requestSupportTranslation(workspaceId, message.conversation_id, { message_id: message.id, target_language: language }),
    onSuccess: () => client.invalidateQueries({ queryKey: ['support', workspaceId, 'cached-translations', message.conversation_id] }),
  });
  const candidate = request.data?.target_language === language &&
    (!cachedTranslation || Date.parse(request.data.updated_at) >= Date.parse(cachedTranslation.updated_at))
    ? request.data : cachedTranslation;
  const outgoing = cachedTranslation?.purpose === 'outgoing_reply' && cachedTranslation.sent_message_id === message.id &&
    cachedTranslation.status === 'ready' && cachedTranslation.translated_text === message.content ? cachedTranslation : undefined;
  const result = outgoing ?? (candidate?.source_text === message.content && candidate.target_language === language ? candidate : undefined);
  const eligible = (!!options?.available || result?.status === 'ready') && !message.pending_send &&
    (message.sender_type === 'customer' || message.sender_type === 'ai' || !!outgoing) && !message.is_internal && message.message_type === 'reply' && !!message.content.trim();
  // Only completed language evidence overrides the local hint. Failed/pending
  // artifacts must not reintroduce Translate for text already in the reading language.
  const sourceLanguage = result?.status === 'ready' ? normalizeReadingLanguage(result.source_language || '') : '';
  const hasSourceLanguage = !!sourceLanguage && sourceLanguage !== 'und';
  // Once the conversation language is known to match, short or unclear text
  // stays untranslated; only a message that clearly differs offers Translate.
  const conversationLanguage = options?.conversation?.customer_language || options?.detected_customer_language || '';
  const conversationInReadingLanguage = !!conversationLanguage &&
    normalizeReadingLanguage(conversationLanguage) === normalizeReadingLanguage(language);
  const locallySame = useMemo(
    () => eligible && !outgoing && !hasSourceLanguage && (conversationInReadingLanguage
      ? !isMessageInOtherLanguage(message.content, language)
      : isMessageInReadingLanguage(message.content, language)),
    [eligible, outgoing, hasSourceLanguage, conversationInReadingLanguage, message.content, language],
  );
  const same = (result?.status === 'ready' && result.translated_text === result.source_text) ||
    (!outgoing && (locallySame || (hasSourceLanguage && sourceLanguage === normalizeReadingLanguage(language))));
  const translated = !same && result?.status === 'ready' && result.translated_text !== result.source_text;
  const pending = request.isPending || result?.status === 'pending';
  const failed = request.isError || result?.status === 'failed' || result?.status === 'expired';
  const sourceName = result && options?.languages[result.source_language];
  const footer = eligible && !same ? <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground" aria-live="polite">
    {translated ? <>
      <span>{original ? 'Original' : sourceName ? `Translated from ${sourceName}` : 'Translated'}</span>
      <span aria-hidden="true">·</span>
      <QuietTextAction type="button" onClick={() => setOriginal(!original)}>{original ? 'Show translation' : 'Show original'}</QuietTextAction>
    </> : pending ? <span>{result?.source_language && result.source_language !== language ? 'Translating…' : 'Preparing…'}</span> : <>
      {failed && <span>Translation unavailable</span>}
      <QuietTextAction
        type="button"
        title={`Translate to ${options?.languages[language] || language}`}
        onClick={() => { setOriginal(false); request.mutate(); }}
      >{failed ? 'Retry' : 'Translate'}</QuietTextAction>
    </>}
  </div> : undefined;
  return <MessageBubble {...props} translatedContent={translated ? (original ? result.source_text : result.translated_text) : undefined} translationFooter={pending || failed ? undefined : footer} translationStatus={pending || failed ? footer : undefined} />;
}
