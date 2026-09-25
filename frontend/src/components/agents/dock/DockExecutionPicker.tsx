import { ArrowDown01Icon, TerminalIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

export function DockExecutionPicker({
  enabled,
  disabled = false,
  onChange,
}: {
  enabled: boolean;
  disabled?: boolean;
  onChange: (enabled: boolean) => void | Promise<void>;
}) {
  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex">
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                disabled={disabled}
                aria-label={`Code and Python tools ${enabled ? 'enabled' : 'disabled'}`}
                className={cn(
                  'inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs transition disabled:cursor-not-allowed disabled:opacity-50',
                  enabled
                    ? 'bg-muted text-foreground hover:bg-muted/80'
                    : 'text-quiet-text-secondary hover:bg-muted hover:text-quiet-text-primary',
                )}
              >
                <TerminalIcon className="h-3.5 w-3.5" aria-hidden="true" />
                <span>Code</span>
                <ArrowDown01Icon className="h-3 w-3 opacity-60" aria-hidden="true" />
              </button>
            </DropdownMenuTrigger>
          </span>
        </TooltipTrigger>
        <TooltipContent side="top" className="z-[70]">
          Code and Python tools are {enabled ? 'on' : 'off'}
        </TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="end" side="top" className="z-[70] w-72">
        <DropdownMenuCheckboxItem
          checked={enabled}
          disabled={disabled}
          onCheckedChange={(checked) => void onChange(checked === true)}
          className="items-start rounded-lg"
        >
          <TerminalIcon className="mt-0.5 h-4 w-4" aria-hidden="true" />
          <span className="flex min-w-0 flex-col gap-0.5">
            <span>Code &amp; Python</span>
            <span className="text-[11px] font-normal leading-4 text-muted-foreground">
              Edit files and run commands. Worker files are not isolated between runs.
            </span>
          </span>
        </DropdownMenuCheckboxItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
