import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { QuietTextAction } from '@/components/design-system/quiet';

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
