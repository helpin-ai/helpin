import { useState } from 'react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
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

  // If team has estimate settings enabled, use scale-aware picker
  if (config?.enabled) {
    const options = getEstimateOptions(config.scale, config.extended, config.allow_zero);
    const numValue = value === '' ? undefined : Number(value);
    const displayLabel = formatEstimateValue(numValue, config.scale);
    const trigger = (
      <button
        type="button"
        className={cn(
          'inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer',
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

    return (
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>{trigger}</PopoverTrigger>
        <PopoverContent className="w-auto p-2" align="start">
          <div className="flex flex-col gap-0.5">
            <button
              type="button"
              className={cn(
                'rounded-md px-3 py-1.5 text-left text-xs transition-colors hover:bg-accent',
                value === '' && 'bg-accent font-medium',
              )}
              onClick={() => {
                onChange('', undefined);
                setOpen(false);
              }}
            >
              None
            </button>
            {options.map((opt) => (
              <button
                key={opt.value}
                type="button"
                className={cn(
                  'rounded-md px-3 py-1.5 text-left text-xs transition-colors hover:bg-accent',
                  numValue === opt.value && 'bg-accent font-medium',
                )}
                onClick={() => {
                  onChange(String(opt.value), opt.value);
                  setOpen(false);
                }}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </PopoverContent>
      </Popover>
    );
  }

  // Fallback: free-form number input (no team config or disabled)
  const trigger = (
    <button
      type="button"
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer',
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
      <PopoverContent className="w-36 p-3" align="start">
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
      </PopoverContent>
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
