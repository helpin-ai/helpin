import { useState } from 'react';
import { Tick01Icon, PlusSignIcon, Target01Icon, Cancel01Icon } from '@/lib/icons';

import { Popover, PopoverTrigger } from '@/components/ui/popover';
import { PMDropdownContent } from './PMDropdownContent';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { cn } from '@/lib/utils';
import type { Objective } from '@/lib/pmTypes';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';

export interface ObjectivePickerSelection {
  id: string;
  name: string;
  archived?: boolean;
}

interface ObjectivePickerProps {
  objectives: Objective[];
  selectedObjectiveIds: string[];
  selectedObjectives?: ObjectivePickerSelection[];
  onChange: (objectiveIds: string[]) => void | Promise<void>;
  className?: string;
  addLabel?: string;
  /** Show only the trigger button, no badges — used for compact inline table cells */
  triggerOnly?: boolean;
}

function ObjectiveBadge({
  objective,
  onRemove,
}: {
  objective: ObjectivePickerSelection;
  onRemove?: () => void;
}) {
  return (
    <span className="inline-flex h-5 max-w-full min-w-0 items-center gap-1 rounded-sm border-[0.5px] border-border px-2 text-[11px] font-medium text-foreground/80">
      <Target01Icon className="h-3 w-3 shrink-0 text-muted-foreground" />
      <span className="min-w-0 truncate">{objective.name}</span>
      {onRemove ? (
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
          className="ml-0.5 rounded-sm opacity-60 transition-opacity hover:opacity-100"
        >
          <Cancel01Icon className="h-3 w-3" />
        </button>
      ) : null}
    </span>
  );
}

export function ObjectivePicker({
  objectives,
  selectedObjectiveIds,
  selectedObjectives: selectedObjectivesProp,
  onChange,
  className,
  addLabel = 'Add objective',
  triggerOnly = false,
}: ObjectivePickerProps) {
  const [open, setOpen] = useState(false);
  const availableObjectives = objectives.filter((objective) => !objective.archived);
  const selectedObjectives = selectedObjectivesProp ?? objectives
    .filter((objective) => selectedObjectiveIds.includes(objective.id))
    .map((objective) => ({
      id: objective.id,
      name: objective.name,
      archived: objective.archived,
    }));

  const toggleObjective = (objectiveId: string) => {
    if (selectedObjectiveIds.includes(objectiveId)) {
      void onChange(selectedObjectiveIds.filter((id) => id !== objectiveId));
      return;
    }
    void onChange([...selectedObjectiveIds, objectiveId]);
  };

  const removeObjective = (objectiveId: string) => {
    void onChange(selectedObjectiveIds.filter((id) => id !== objectiveId));
  };

  return (
    <div className={cn('flex min-w-0 flex-wrap items-center gap-1', className)}>
      {!triggerOnly && selectedObjectives.map((objective) => (
        <ObjectiveBadge
          key={objective.id}
          objective={{
            id: objective.id,
            name: objective.archived ? `${objective.name} (Archived)` : objective.name,
            archived: !!objective.archived,
          }}
          onRemove={() => removeObjective(objective.id)}
        />
      ))}

      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-ui text-muted-foreground transition-colors hover:bg-accent cursor-pointer"
            onClick={(event) => {
              event.stopPropagation();
              setOpen(true);
            }}
          >
            <PlusSignIcon className="h-3 w-3" />
            {selectedObjectives.length === 0 ? addLabel : 'Add'}
          </button>
        </PopoverTrigger>
        {open ? (
          <PMDropdownContent
            className="w-[260px] p-0"
            align="start"
            side="bottom"
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
          >
            {availableObjectives.length === 0 ? (
              <div className="flex flex-col items-center gap-1.5 px-3 py-4 text-center">
                <Target01Icon className="h-4 w-4 text-muted-foreground" />
                <p className="text-ui text-muted-foreground">No objectives yet</p>
                <button
                  type="button"
                  className="inline-flex items-center gap-1 rounded-md bg-primary px-2.5 py-1 text-ui font-medium text-primary-foreground transition-colors hover:bg-primary/90"
                  onClick={() => {
                    setOpen(false);
                    useGlobalCreateStore.getState().openCreate('objective');
                  }}
                >
                  <PlusSignIcon className="h-3 w-3" />
                  Create objective
                </button>
              </div>
            ) : (
              <Command>
                <CommandInput placeholder="Search objectives..." className="h-8 text-ui" />
                <CommandList>
                  <CommandEmpty className="py-3 text-center text-ui text-muted-foreground">
                    No objectives found
                  </CommandEmpty>
                  <CommandGroup>
                    {availableObjectives.map((objective) => {
                      const isSelected = selectedObjectiveIds.includes(objective.id);
                      return (
                        <CommandItem
                          key={objective.id}
                          value={objective.name}
                          className="flex items-center gap-2 text-ui"
                          onSelect={() => toggleObjective(objective.id)}
                        >
                          <Target01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                          <span className="min-w-0 flex-1 truncate">{objective.name}</span>
                          {isSelected ? <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" /> : null}
                        </CommandItem>
                      );
                    })}
                  </CommandGroup>
                </CommandList>
              </Command>
            )}
          </PMDropdownContent>
        ) : null}
      </Popover>
    </div>
  );
}
