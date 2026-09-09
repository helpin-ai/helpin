import { ColumnsThreeCogIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import {
  useBoardDisplayStore,
  DISPLAY_PROPERTY_LABELS,
  LIST_PROPERTY_KEYS,
  type DisplayPropertyKey,
} from '@/stores/boardDisplayStore';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

interface ListDisplayMenuProps {
  /** Keys hidden at team-field-visibility level — these won't appear as toggleable. */
  disabledKeys?: Set<DisplayPropertyKey>;
}

export function ListDisplayMenu({ disabledKeys }: ListDisplayMenuProps) {
  const { properties, toggleProperty } = useBoardDisplayStore();

  const availableKeys = disabledKeys
    ? LIST_PROPERTY_KEYS.filter((k) => !disabledKeys.has(k))
    : LIST_PROPERTY_KEYS;

  return (
    <QuietDropdown
      label="Display columns"
      multiple
      selected={availableKeys.filter((key) => properties[key])}
      options={availableKeys.map((key) => ({
        value: key,
        label: DISPLAY_PROPERTY_LABELS[key],
      }))}
      onSelect={(key) => toggleProperty(key as DisplayPropertyKey)}
      contentProps={{ align: 'end' }}
      triggerWrapper={(trigger) => (
        <QuickTooltip label="Display columns">{trigger}</QuickTooltip>
      )}
      trigger={
        <Button variant="ghost" size="icon-sm" aria-label="Display columns">
          <ColumnsThreeCogIcon className="h-4 w-4" />
        </Button>
      }
    />
  );
}
