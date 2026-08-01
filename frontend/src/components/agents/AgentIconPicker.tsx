import { useState } from 'react';

import { AgentAvatar, AGENT_ICON_PRESETS } from '@/components/agents/AgentAvatar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import type { AgentIconKey } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

interface AgentIconPickerProps {
  value: AgentIconKey;
  onValueChange: (value: AgentIconKey) => void;
  disabled?: boolean;
}

export function AgentIconPicker({ value, onValueChange, disabled = false }: AgentIconPickerProps) {
  const [open, setOpen] = useState(false);
  const selected = AGENT_ICON_PRESETS.find((preset) => preset.key === value) ?? AGENT_ICON_PRESETS[0];

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          className="group flex items-center gap-3 rounded-md border border-input bg-background px-3 py-2 text-left hover:bg-muted/40 disabled:cursor-not-allowed disabled:opacity-50"
          aria-label={`Change agent icon. Current icon: ${selected.label}`}
        >
          <AgentAvatar iconKey={selected.key} className="h-10 w-10 rounded-none border-0 bg-transparent shadow-none" genericBare />
          <span>
            <span className="block text-sm font-medium">{selected.label}</span>
            <span className="block text-xs text-muted-foreground group-hover:text-foreground">Choose icon</span>
          </span>
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 p-3">
        <div className="mb-3">
          <p className="text-sm font-medium">Agent icon library</p>
          <p className="mt-0.5 text-xs text-muted-foreground">Pick a visual identity for this custom agent.</p>
        </div>
        <div className="grid grid-cols-4 gap-1.5">
          {AGENT_ICON_PRESETS.map((preset) => {
            const active = preset.key === value;
            return (
              <button
                key={preset.key}
                type="button"
                aria-pressed={active}
                className={cn(
                  'flex flex-col items-center gap-1.5 rounded-md border px-1.5 py-2 text-xs transition-colors',
                  active
                    ? 'border-primary bg-primary/10 text-foreground'
                    : 'border-transparent text-muted-foreground hover:border-border hover:bg-muted/40 hover:text-foreground',
                )}
                onClick={() => {
                  onValueChange(preset.key);
                  setOpen(false);
                }}
              >
                <AgentAvatar iconKey={preset.key} className="h-9 w-9 rounded-none border-0 bg-transparent shadow-none" genericBare />
                <span>{preset.label}</span>
              </button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
