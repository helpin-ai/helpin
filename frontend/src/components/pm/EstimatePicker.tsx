import { useQuietDropdownFocusReturn } from '@/components/design-system/use-quiet-dropdown-focus-return';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { useState } from 'react';
import { Popover, PopoverTrigger } from '@/components/ui/popover';
import { PMDropdownContent } from './PMDropdownContent';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useTeamEstimateSettingsForTeam } from '@/hooks/queries';
import { getEstimateOptions, formatEstimateValue } from '@/lib/estimateScales';
import type { TeamEstimateSettings } from '@/lib/types';

interface EstimatePickerProps {
  value: string;
  teamId?: string | null;
  onChange: (displayValue: string, apiValue: number | undefined) => void;
  className?: string;
  estimateSettings?: TeamEstimateSettings | null;
  lazyMount?: boolean;
}

interface EstimatePickerBaseProps extends EstimatePickerProps {
  config: TeamEstimateSettings | null;
}

function EstimatePickerBase({
  value,
  onChange,
  className,
  config,
  lazyMount = false,
}: EstimatePickerBaseProps) {
  const [open, setOpen] = useState(false);
  const triggerRef = useQuietDropdownFocusReturn(open, lazyMount);

  // If team has estimate settings enabled, use scale-aware picker
  if (config?.enabled) {
    const options = getEstimateOptions(config.scale, config.extended, config.allow_zero);
    const numValue = value === '' ? undefined : Number(value);
    const displayLabel = formatEstimateValue(numValue, config.scale);
    const trigger = (
      <button
      ref={triggerRef}
        type="button"
        className={cn(
          'inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer',
          className,
        )}
        onClick={(event) => {
          event.stopPropagation();
          if (lazyMount && !open) {
            setOpen(true);
          }
        }}
      >
        {displayLabel}
      </button>
    );

    if (lazyMount && !open) {
      return trigger;
    }

    return <QuietDropdown label="Estimate" trigger={trigger} open={open} onOpenChange={setOpen}
      selected={[value || '__none__']} contentClassName="w-40"
      options={[{ value: '__none__', label: 'None' }, ...options.map(option => ({ value: String(option.value), label: option.label }))]}
      onSelect={next => onChange(next === '__none__' ? '' : next, next === '__none__' ? undefined : Number(next))} />;
  }

  // Fallback: free-form number input (no team config or disabled)
  const trigger = (
    <button
      ref={triggerRef}
      type="button"
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer',
        className,
      )}
      onClick={(event) => {
        event.stopPropagation();
        if (lazyMount && !open) {
          setOpen(true);
        }
      }}
    >
      {value ? `${value} pts` : 'None'}
    </button>
  );

  if (lazyMount && !open) {
    return trigger;
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PMDropdownContent className="w-36 p-3" align="start">
        <Input
          type="number"
          min={0}
          placeholder="Points"
          className="h-8 text-sm"
          value={value}
          onChange={(e) => {
            const next = e.target.value;
            onChange(next, next === '' ? undefined : Number(next));
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              setOpen(false);
            }
          }}
        />
      </PMDropdownContent>
    </Popover>
  );
}

function EstimatePickerWithQuery(props: EstimatePickerProps) {
  const wsId = useWorkspaceStore((s) => s.currentWorkspace?.id) ?? '';
  const config = useTeamEstimateSettingsForTeam(wsId, props.teamId);

  return <EstimatePickerBase {...props} config={config} />;
}

export function EstimatePicker(props: EstimatePickerProps) {
  if (props.estimateSettings !== undefined) {
    return <EstimatePickerBase {...props} config={props.estimateSettings} />;
  }

  return <EstimatePickerWithQuery {...props} />;
}

// eslint-disable-next-line react-refresh/only-export-components
export function formatEstimateDisplay(value: number | undefined | null, _teamId: string | undefined | null, config?: TeamEstimateSettings | null): string {
  if (config?.enabled) {
    return formatEstimateValue(value, config.scale);
  }
  if (value == null) return '';
  return `${value} pts`;
}
