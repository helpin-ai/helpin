import { useState } from 'react';
import {
  useSaveTranslationSettings,
  useSupportTranslationOptions,
} from '@/hooks/queries/useSupportTranslation';
import { TranslationLanguagePicker } from './SupportTranslationControls';

export function useOutgoingSupportTranslation(workspaceId: string, conversationId: string) {
  const options = useSupportTranslationOptions(workspaceId, conversationId);
  const save = useSaveTranslationSettings(workspaceId, conversationId);
  const [selected, setSelected] = useState({ conversationId, language: '' });
  const selectedLanguage = selected.conversationId === conversationId ? selected.language : '';
  const setLanguage = (language: string) => setSelected({ conversationId, language });
  const language =
    options.data?.conversation.customer_language ||
    selectedLanguage ||
    options.data?.detected_customer_language ||
    '';
  const enabled = Boolean(options.data?.available && options.data.preference.auto_translate_outgoing);
  return { options, save, enabled, language, setLanguage };
}

export function AutoTranslateReplyControls({
  translation,
  disabled,
}: {
  translation: ReturnType<typeof useOutgoingSupportTranslation>;
  disabled?: boolean;
}) {
  const { options, save, enabled, language, setLanguage } = translation;
  if (options.isError)
    return (
      <div role="alert" className="px-3 py-2 text-xs text-destructive">
        Translation settings are unavailable.{' '}
        <button type="button" onClick={() => void options.refetch()}>
          Retry
        </button>
      </div>
    );
  if (!options.data?.available) return null;
  return (
    <div className="space-y-2 border-t border-quiet-divider px-3 py-2 text-xs">
      <div className="flex flex-wrap items-center gap-3">
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={enabled}
            disabled={disabled || save.isPending}
            onChange={(event) =>
              save.mutate({
                preference: { ...options.data!.preference, auto_translate_outgoing: event.target.checked },
              })
            }
          />
          Auto-translate replies
        </label>
        {enabled && (
          <TranslationLanguagePicker
            label="Send in"
            value={language}
            languages={options.data.languages}
            allowAuto
            disabled={disabled || !!options.data.conversation.customer_language}
            onChange={setLanguage}
          />
        )}
      </div>
      {enabled && !language && (
        <p className="text-quiet-text-secondary">
          The customer’s language will be detected from their latest message on Send.
        </p>
      )}
      {save.isError && (
        <p role="alert" className="text-destructive">
          Could not save your translation preference. Try again.
        </p>
      )}
    </div>
  );
}
