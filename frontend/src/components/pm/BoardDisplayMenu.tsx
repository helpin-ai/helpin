import { ColumnsThreeCogIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { Switch } from '@/components/ui/switch';
import {
  useBoardDisplayStore,
  DISPLAY_PROPERTY_LABELS,
  BOARD_PROPERTY_KEYS,
} from '@/stores/boardDisplayStore';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

export function BoardDisplayMenu() {
  const {
    properties,
    showEmptyColumns,
    toggleProperty,
    toggleShowEmptyColumns,
  } = useBoardDisplayStore();

  return (
    <QuietDropdown
      label="Display properties"
      multiple
      selected={BOARD_PROPERTY_KEYS.filter((key) => properties[key])}
      options={BOARD_PROPERTY_KEYS.map((key) => ({
        value: key,
        label: DISPLAY_PROPERTY_LABELS[key],
      }))}
      onSelect={(key) =>
        toggleProperty(key as (typeof BOARD_PROPERTY_KEYS)[number])
      }
      contentProps={{ align: 'end' }}
      triggerWrapper={(trigger) => (
        <QuickTooltip label="Display settings">{trigger}</QuickTooltip>
      )}
      trigger={
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          aria-label="Display settings"
        >
          <ColumnsThreeCogIcon className="h-4 w-4" />
        </Button>
      }
      footer={
        <label className="flex items-center justify-between gap-2 px-2 py-1.5">
          <span>Show empty columns</span>
          <Switch
            checked={showEmptyColumns}
            onCheckedChange={toggleShowEmptyColumns}
            className="scale-75"
          />
        </label>
      }
    />
  );
}
