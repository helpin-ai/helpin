import { DisplaySettingsMenu } from '@/components/design-system/display-settings-menu';
interface DisplayPropertiesPopoverProps {
  allProperties: { key: string; label: string }[];
  visible: string[];
  onChange: (next: string[]) => void;
  iconOnly?: boolean;
}
export function DisplayPropertiesPopover({
  allProperties,
  visible,
  onChange,
}: DisplayPropertiesPopoverProps) {
  return (
    <DisplaySettingsMenu
      options={allProperties.map((p) => ({ value: p.key, label: p.label }))}
      selected={visible}
      onToggle={(key) =>
        onChange(
          visible.includes(key)
            ? visible.filter((value) => value !== key)
            : [...visible, key],
        )
      }
    />
  );
}
