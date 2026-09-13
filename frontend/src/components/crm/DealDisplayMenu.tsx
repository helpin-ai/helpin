import { DisplaySettingsMenu } from '@/components/design-system/display-settings-menu';
import { Switch } from '@/components/ui/switch';
import {
  useDealDisplayStore,
  DEAL_DISPLAY_PROPERTY_LABELS,
  DEAL_BOARD_PROPERTY_KEYS,
  DEAL_LIST_PROPERTY_KEYS,
} from '@/stores/dealDisplayStore';
export function DealDisplayMenu({ mode }: { mode: 'board' | 'list' }) {
  const { properties, showEmptyStages, toggleProperty, toggleShowEmptyStages } =
    useDealDisplayStore();
  const keys =
    mode === 'board' ? DEAL_BOARD_PROPERTY_KEYS : DEAL_LIST_PROPERTY_KEYS;
  return (
    <DisplaySettingsMenu
      options={keys.map((key) => ({
        value: key,
        label: DEAL_DISPLAY_PROPERTY_LABELS[key],
      }))}
      selected={keys.filter((key) => properties[key])}
      onToggle={(key) => toggleProperty(key as (typeof keys)[number])}
      footer={
        mode === 'board' ? (
          <label className="flex items-center justify-between gap-2 px-2 py-1.5">
            <span>Show empty stages</span>
            <Switch
              checked={showEmptyStages}
              onCheckedChange={toggleShowEmptyStages}
              className="scale-75"
            />
          </label>
        ) : undefined
      }
    />
  );
}
