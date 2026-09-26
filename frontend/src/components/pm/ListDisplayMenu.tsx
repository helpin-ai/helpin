import { DisplaySettingsMenu } from '@/components/design-system/display-settings-menu';
import {
  useBoardDisplayStore,
  DISPLAY_PROPERTY_LABELS,
  LIST_PROPERTY_KEYS,
  type DisplayPropertyKey,
} from '@/stores/boardDisplayStore';
export function ListDisplayMenu({
  disabledKeys,
}: {
  disabledKeys?: Set<DisplayPropertyKey>;
}) {
  const { properties, toggleProperty } = useBoardDisplayStore();
  const keys = LIST_PROPERTY_KEYS.filter((key) => !disabledKeys?.has(key));
  return (
    <DisplaySettingsMenu
      options={keys.map((key) => ({
        value: key,
        label: DISPLAY_PROPERTY_LABELS[key],
      }))}
      selected={keys.filter((key) => properties[key])}
      onToggle={(key) => toggleProperty(key as DisplayPropertyKey)}
    />
  );
}
