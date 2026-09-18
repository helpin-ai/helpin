import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { QuietTextAction } from '@/components/design-system/quiet';
import {
  useSaveTranslationSettings,
  useSupportTranslationOptions,
} from '@/hooks/queries/useSupportTranslation';

export function TranslationLanguagePicker({
  label,
  value,
  languages,
  disabled,
  onChange,
  allowAuto,
}: {
  label: string;
  value: string;
  languages: Record<string, string>;
  disabled?: boolean;
  allowAuto?: boolean;
  onChange: (value: string) => void;
}) {
  const options = Object.entries(languages)
    .map(([value, label]) => ({ value, label }))
    .sort((a, b) => a.label.localeCompare(b.label));
  if (allowAuto) options.unshift({ value: '', label: 'Detect automatically' });
  return (
    <QuietDropdown
      label={label}
      trigger={
        <QuietTextAction
          type="button"
          aria-label={`${label}: ${languages[value] || 'Detect automatically'}`}
          disabled={disabled}
        >
          {label}: {languages[value] || 'Detect automatically'}
        </QuietTextAction>
      }
      selected={[value]}
      options={options}
      onSelect={onChange}
      disabled={disabled}
      searchMode="always"
    />
  );
}
export function SupportTranslationControls({
  workspaceId,
  conversationId,
  canEdit,
}: {
  workspaceId: string;
  conversationId: string;
  canEdit: boolean;
}) {
  const query = useSupportTranslationOptions(workspaceId, conversationId);
  const save = useSaveTranslationSettings(workspaceId, conversationId);
  const options = query.data;
  if (!options)
    return (
      <div className="border-b px-4 py-2 text-xs text-quiet-text-secondary">
        {query.isPending ? 'Loading translation settings…' : 'Translation settings unavailable.'}
      </div>
    );
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b px-4 py-2 text-xs">
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={options.preference.auto_translate_incoming}
          disabled={save.isPending || !options.available}
          onChange={(e) =>
            save.mutate({ preference: { ...options.preference, auto_translate_incoming: e.target.checked } })
          }
        />
        Auto-translate incoming
      </label>
      <TranslationLanguagePicker
        label="Read in"
        value={options.preference.reading_language}
        languages={options.languages}
        disabled={save.isPending}
        onChange={(reading_language) =>
          save.mutate({ preference: { ...options.preference, reading_language } })
        }
      />
      {canEdit && (
        <TranslationLanguagePicker
          label="Customer language"
          value={options.conversation.customer_language}
          languages={options.languages}
          allowAuto
          disabled={save.isPending}
          onChange={(customer_language) =>
            save.mutate({ conversation: { ...options.conversation, customer_language } })
          }
        />
      )}
      {canEdit && (
        <QuietTextAction
          type="button"
          disabled={save.isPending}
          onClick={() =>
            save.mutate({
              conversation: {
                ...options.conversation,
                translation_mode: options.conversation.translation_mode === 'off' ? 'on' : 'off',
              },
            })
          }
        >
          {options.conversation.translation_mode === 'off'
            ? 'Enable translation'
            : 'Turn off for this conversation'}
        </QuietTextAction>
      )}
      {!options.available && (
        <span className="basis-full text-quiet-text-secondary">{options.unavailable_reason}</span>
      )}
      {save.isError && (
        <span role="alert" className="basis-full text-destructive">
          Could not save translation settings. Try again.
        </span>
      )}
    </div>
  );
}
