import { DisplaySettingsMenu } from '@/components/design-system/display-settings-menu';
import { Switch } from '@/components/ui/switch';
import {
  useBoardDisplayStore,
  DISPLAY_PROPERTY_LABELS,
  BOARD_PROPERTY_KEYS,
} from '@/stores/boardDisplayStore';
export function BoardDisplayMenu() {
  const {
    properties,
    showEmptyColumns,
    toggleProperty,
    toggleShowEmptyColumns,
  } = useBoardDisplayStore();
  return (
    <DisplaySettingsMenu
      options={BOARD_PROPERTY_KEYS.map((key) => ({
        value: key,
        label: DISPLAY_PROPERTY_LABELS[key],
      }))}
      selected={BOARD_PROPERTY_KEYS.filter((key) => properties[key])}
      onToggle={(key) =>
        toggleProperty(key as (typeof BOARD_PROPERTY_KEYS)[number])
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
