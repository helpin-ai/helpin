import { useState } from 'react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import type { EpicWithStats, Task } from '@/lib/pmTypes';
import { EpicBadge } from './EpicBadge';

export function InlineEpicCell({
  task,
  epics,
  epicMap,
  onUpdate,
}: {
  task: Task;
  epics: EpicWithStats[];
  epicMap: Map<string, EpicWithStats['epic']>;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const linkedEpic = task.epic_id ? epicMap.get(task.epic_id) : undefined;
  const epicName = task.epic_id ? linkedEpic?.name ?? task.epic_name ?? 'Unknown' : null;
  const epicColor = linkedEpic?.color;

  if (!open) {
    return (
      <button
        type="button"
        className="flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer"
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
      >
        {epicName ? (
          <EpicBadge name={epicName} color={epicColor} />
        ) : (
          <span className="text-muted-foreground">No Epic</span>
        )}
      </button>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {epicName ? (
            <EpicBadge name={epicName} color={epicColor} />
          ) : (
            <span className="text-muted-foreground">No Epic</span>
          )}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-[220px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search epics..." className="h-8 text-ui" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-ui text-muted-foreground">No epics found</CommandEmpty>
              <CommandGroup>
                {epics.map((e) => (
                  <CommandItem
                    key={e.epic.id}
                    value={e.epic.id}
                    keywords={[e.epic.name]}
                    data-checked={task.epic_id === e.epic.id}
                    onSelect={() => {
                      const newEpicId = task.epic_id === e.epic.id ? undefined : e.epic.id;
                      onUpdate(task.id, { epic_id: newEpicId });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-ui"
                  >
                    <EpicBadge name={e.epic.name} color={e.epic.color} />
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      )}
    </Popover>
  );
}
