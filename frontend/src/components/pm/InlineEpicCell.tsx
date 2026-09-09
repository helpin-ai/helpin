import { useQuietDropdownFocusReturn } from '@/components/design-system/use-quiet-dropdown-focus-return';
import { useState } from 'react';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
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
  const triggerRef = useQuietDropdownFocusReturn(open);
  const linkedEpic = task.epic_id ? epicMap.get(task.epic_id) : undefined;
  const epicName = task.epic_id
    ? (linkedEpic?.name ?? task.epic_name ?? 'Unknown')
    : null;
  const epicColor = linkedEpic?.color;

  const trigger = (
    <button
      ref={triggerRef}
      type="button"
      className={cn(
        'flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer',
        triggerClassName,
      )}
      onClick={(event) => {
        event.stopPropagation();
        setOpen(true);
      }}
    >
      {epicName ? (
        <EpicBadge name={epicName} color={epicColor} />
      ) : (
        <span className="text-muted-foreground">No Epic</span>
      )}
    </button>
  );
  if (!open) return trigger;
  return (
    <QuietDropdown
      label="Epic"
      trigger={trigger}
      open={open}
      onOpenChange={setOpen}
      selected={[task.epic_id || '__none__']}
      searchPlaceholder="Search epics..."
      contentClassName="w-[220px]"
      empty="No epics found"
      onSelect={(value) => {
        void onUpdate(task.id, {
          epic_id: value === '__none__' || value === task.epic_id ? '' : value,
        });
      }}
      groups={[
        { id: 'none', options: [{ value: '__none__', label: 'None' }] },
        ...groupEpicsByLifecycle(epicMap.values()).map((group) => ({
          id: group.label,
          label: group.label,
          options: group.epics.map((epic) => ({
            value: epic.id,
            label: epic.name,
            content: (
              <EpicBadge
                name={epic.name}
                color={epic.color}
                className="text-[length:inherit]"
              />
            ),
          })),
        })),
      ]}
    />
  );
}
