import { useState } from 'react';
import { Popover, PopoverTrigger } from '@/components/ui/popover';
import { PMDropdownContent } from './PMDropdownContent';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import type { Epic, Task } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { EpicBadge } from './EpicBadge';
import { groupEpicsByLifecycle } from './epicPickerGroups';

export function InlineEpicCell({
  task,
  epicMap,
  onUpdate,
  triggerClassName,
}: {
  task: Task;
  epicMap: ReadonlyMap<string, Epic>;
  triggerClassName?: string;
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
        className={cn('flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer', triggerClassName)}
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
          className={cn('flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer', triggerClassName)}
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
        <PMDropdownContent
          className="w-[220px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command defaultValue={task.epic_id || '__none__'}>
            <CommandInput placeholder="Search epics..." className="h-8 text-ui" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-ui text-muted-foreground">No epics found</CommandEmpty>
              <CommandGroup>
                <CommandItem
                  value="__none__"
                  keywords={['None']}
                  data-checked={!task.epic_id}
                  onSelect={() => {
                    onUpdate(task.id, { epic_id: '' });
                    setOpen(false);
                  }}
                  className="text-ui"
                >
                  None
                </CommandItem>
              </CommandGroup>
              {groupEpicsByLifecycle(epicMap.values()).filter((group) => group.epics.length > 0).map((group) => (
                <CommandGroup key={group.label} heading={group.label}>
                  {group.epics.map((epic) => (
                    <CommandItem
                      key={epic.id}
                      value={epic.id}
                      keywords={[epic.name]}
                      data-checked={task.epic_id === epic.id}
                      onSelect={() => {
                        const newEpicId = task.epic_id === epic.id ? '' : epic.id;
                        onUpdate(task.id, { epic_id: newEpicId });
                        setOpen(false);
                      }}
                      className="flex items-center gap-2 text-ui"
                    >
                      <EpicBadge name={epic.name} color={epic.color} className="text-[length:inherit]" />
                    </CommandItem>
                  ))}
                </CommandGroup>
              ))}
            </CommandList>
          </Command>
        </PMDropdownContent>
      )}
    </Popover>
  );
}
