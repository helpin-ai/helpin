import type { ReactNode } from 'react';
import { ColumnsThreeCogIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { QuietDropdown } from './quiet-dropdown';
export function DisplaySettingsMenu({
  options,
  selected,
  onToggle,
  footer,
}: {
  options: { value: string; label: string }[];
  selected: string[];
  onToggle: (value: string) => void;
  footer?: ReactNode;
}) {
  return (
    <QuietDropdown
      label="Display settings"
      multiple
      options={options}
      selected={selected}
      onSelect={onToggle}
      contentProps={{ align: 'end' }}
      footer={footer}
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
    />
  );
}
