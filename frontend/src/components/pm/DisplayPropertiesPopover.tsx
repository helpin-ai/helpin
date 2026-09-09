import { ColumnsThreeCogIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

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
  iconOnly = false,
}: DisplayPropertiesPopoverProps) {
  const toggle = (key: string) => {
    onChange(
      visible.includes(key)
        ? visible.filter((k) => k !== key)
        : [...visible, key],
    );
  };

  return (
    <QuietDropdown
      label="Display properties"
      multiple
      selected={visible}
      onSelect={toggle}
      options={allProperties.map((property) => ({
        value: property.key,
        label: property.label,
      }))}
      contentProps={{ align: 'end' }}
      contentClassName="w-72"
      triggerWrapper={(trigger) => (
        <QuickTooltip label="Display properties">{trigger}</QuickTooltip>
      )}
      trigger={
        <Button
          variant={iconOnly ? 'ghost' : 'outline'}
          size={iconOnly ? 'icon' : 'sm'}
          className={iconOnly ? 'h-7 w-7' : 'h-8 gap-1.5 text-ui'}
          aria-label="Display properties"
        >
          <ColumnsThreeCogIcon className="h-3.5 w-3.5" />
          {iconOnly ? null : 'Display'}
        </Button>
      }
    />
  );
}
