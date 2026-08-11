import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { CheckmarkSquare02Icon, DragDropVerticalIcon, PlusSignIcon, Delete01Icon, Cancel01Icon } from '@/lib/icons';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Button } from '@/components/ui/button';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { pmChecklistService } from '@/lib/services/pmChecklistService';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList';
import { MentionText } from '@/components/pm/MentionText';
import {
  getMentionSuggestions,
  type MentionSuggestionItem,
} from '@/components/pm/mentionSuggestions';
import type { ChecklistItem } from '@/lib/pmTypes';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';

interface ChecklistItemsProps {
  workspaceId: string;
  taskId: string;
  members?: AssignableMember[];
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[];
  onContentChange?: (hasContent: boolean) => void;
  onStatsChange?: (stats: { completed: number; total: number }) => void;
}

export type ChecklistMentionOption = MentionSuggestionItem;

const CHECKLIST_TABLE_GRID_CLASS = 'grid-cols-[1.25rem_minmax(0,1fr)_5rem_4rem]';

export function buildChecklistMentionOptions(
  mentionQuery: string | null,
  members: AssignableMember[],
  teams: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[] = [],
): ChecklistMentionOption[] {
  return getMentionSuggestions(mentionQuery, members, teams, 6);
}

function SortableItem({
  item,
  onToggle,
  onDelete,
  onAssigneeChange,
  members = [],
  teams = [],
}: {
  item: ChecklistItem;
  onToggle: (item: ChecklistItem) => void;
  onDelete: (id: string) => void;
  onAssigneeChange: (id: string, assigneeId: string | null) => void;
  members?: AssignableMember[];
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[];
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: item.id,
  });

  const assignee = useMemo(
    () => members.find((m) => (m.user_id || m.id) === item.assignee_id),
    [members, item.assignee_id],
  );
  const assigneeLabel = assignee ? `Assigned to ${assignee.display_name}` : 'Assign';

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      data-testid="checklist-table-row"
      className={`group grid ${CHECKLIST_TABLE_GRID_CLASS} items-center gap-x-2 border-b border-border/50 px-1 py-1.5 transition-colors hover:bg-muted/40 ${isDragging ? 'opacity-50' : ''}`}
    >
      <input
        type="checkbox"
        checked={item.completed}
        onChange={() => onToggle(item)}
        className="h-3.5 w-3.5 shrink-0 rounded border-border cursor-pointer accent-primary"
      />
      <span
        className={`min-w-0 truncate text-sm ${
          item.completed ? 'line-through text-muted-foreground' : 'text-foreground'
        }`}
      >
        <MentionText text={item.text} members={members} teams={teams} />
      </span>

      <div className="flex min-w-0 justify-end">
        <MemberPickerPopover
          value={item.assignee_id || '__none__'}
          members={members}
          noneLabel="Unassigned"
          triggerLabel={assigneeLabel}
          getMemberValue={(member) => member.user_id || member.id}
          onChange={(value) => {
            onAssigneeChange(item.id, value === '__none__' ? null : value);
          }}
          align="end"
          triggerClassName={assignee
            ? 'ml-auto h-7 w-7 shrink-0 justify-center overflow-visible rounded-full px-0 py-0 hover:bg-accent'
            : 'ml-auto h-7 w-7 shrink-0 justify-center rounded-full border border-dashed border-border/80 bg-muted/40 px-0 py-0 text-muted-foreground opacity-0 transition-opacity hover:border-foreground/40 hover:bg-accent hover:text-foreground group-hover:opacity-100'
          }
          contentClassName="w-[220px]"
          renderTrigger={() => assignee ? (
            <>
              <span className="flex h-6 w-6 shrink-0 items-center justify-center overflow-visible rounded-full">
                <UserAvatar
                  name={assignee.display_name}
                  avatarUrl={assignee.avatar_url}
                  avatarStyle={assignee.avatar_style}
                  avatarSeed={assignee.avatar_seed}
                  avatarBackgroundMode={assignee.avatar_background_mode}
                  avatarBackgroundColor={assignee.avatar_background_color}
                  className="h-5 w-5 text-[8px]"
                  fallbackClassName="text-[8px]"
                />
              </span>
              <span className="sr-only">{assigneeLabel}</span>
            </>
          ) : (
            <PlusSignIcon className="h-3.5 w-3.5" />
          )}
        />
      </div>

      <div className="flex items-center justify-end gap-1">
        <QuickTooltip label="Delete">
          <button
            type="button"
            className="h-7 w-7 shrink-0 flex items-center justify-center rounded-md opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:bg-accent hover:text-destructive cursor-pointer"
            onClick={() => onDelete(item.id)}
          >
            <Delete01Icon className="h-3.5 w-3.5" />
          </button>
        </QuickTooltip>

        <QuickTooltip label="Move item">
          <button
            type="button"
            className="h-7 w-7 shrink-0 flex items-center justify-center rounded-md cursor-grab text-muted-foreground/60 opacity-0 transition-opacity hover:bg-accent hover:text-muted-foreground group-hover:opacity-100 active:cursor-grabbing"
            {...attributes}
            {...listeners}
          >
            <DragDropVerticalIcon className="h-3.5 w-3.5" />
          </button>
        </QuickTooltip>
      </div>
    </div>
  );
}

export function ChecklistItems({
  workspaceId,
  taskId,
  members = [],
  teams = [],
  onContentChange,
  onStatsChange,
}: ChecklistItemsProps) {
  const [items, setItems] = useState<ChecklistItem[]>([]);
  const [newText, setNewText] = useState('');
  const [adding, setAdding] = useState(false);
  const [addingItem, setAddingItem] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const textRef = useRef(newText);
  textRef.current = newText;

  // Mention autocomplete state.
  const [mentionQuery, setMentionQuery] = useState<string | null>(null);
  const [mentionIndex, setMentionIndex] = useState(0);

  const mentionResults = useMemo(
    () => buildChecklistMentionOptions(mentionQuery, members, teams),
    [mentionQuery, members, teams],
  );

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const reload = useCallback(async () => {
    const { data } = await pmChecklistService.list(workspaceId, taskId);
    if (data) setItems(data.sort((a, b) => a.position - b.position));
  }, [workspaceId, taskId]);

  useEffect(() => { reload(); }, [reload]);

  useEffect(() => {
    onContentChange?.(items.length > 0);
    onStatsChange?.({
      completed: items.filter((item) => item.completed).length,
      total: items.length,
    });
  }, [items, onContentChange, onStatsChange]);

  useEffect(() => {
    if (addingItem) inputRef.current?.focus();
  }, [addingItem]);

  // Re-fetch when another client changes checklist items
  useEffect(() => {
    const handler = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.parent_id === taskId && d?.entity === 'checklist_item') reload();
    };
    window.addEventListener('task-child-updated', handler);
    return () => window.removeEventListener('task-child-updated', handler);
  }, [taskId, reload]);

  const completedCount = useMemo(() => items.filter((i) => i.completed).length, [items]);

  const detectMention = (value: string) => {
    const match = value.match(/(?:^|\s)@([a-z0-9._-]*)$/i);
    if (match) {
      setMentionQuery(match[1]);
      setMentionIndex(0);
    } else {
      setMentionQuery(null);
    }
  };

  const insertMention = (item: ChecklistMentionOption) => {
    const text = newText.replace(/(?:^|\s)@[a-z0-9._-]*$/i, (match) => {
      const prefix = match.startsWith(' ') ? ' ' : '';
      return `${prefix}@${item.handle} `;
    });
    setNewText(text);
    setMentionQuery(null);
    inputRef.current?.focus();
  };

  const handleAdd = async () => {
    const text = textRef.current.trim();
    if (!text || adding) return;
    setAdding(true);
    const { data } = await pmChecklistService.create(workspaceId, taskId, { text });
    setAdding(false);
    if (!data) return;
    // Don't optimistically append — the WebSocket event triggers reload()
    // which fetches the full list, avoiding duplicates.
    await reload();
    setNewText('');
    inputRef.current?.focus();
  };

  const closeAddItemInput = () => {
    setAddingItem(false);
    setNewText('');
    setMentionQuery(null);
  };

  const handleToggle = async (item: ChecklistItem) => {
    const newCompleted = !item.completed;
    setItems((prev) => prev.map((i) => (i.id === item.id ? { ...i, completed: newCompleted } : i)));
    const { error } = await pmChecklistService.update(workspaceId, item.id, { completed: newCompleted });
    if (error) {
      setItems((prev) => prev.map((i) => (i.id === item.id ? { ...i, completed: item.completed } : i)));
    }
  };

  const handleDelete = async (id: string) => {
    setItems((prev) => prev.filter((i) => i.id !== id));
    await pmChecklistService.remove(workspaceId, id);
  };

  const handleAssigneeChange = async (id: string, assigneeId: string | null) => {
    setItems((prev) =>
      prev.map((i) => (i.id === id ? { ...i, assignee_id: assigneeId ?? undefined } : i)),
    );
    await pmChecklistService.update(workspaceId, id, { assignee_id: assigneeId ?? '' });
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = items.findIndex((i) => i.id === active.id);
    const newIndex = items.findIndex((i) => i.id === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    const reordered = [...items];
    const [moved] = reordered.splice(oldIndex, 1);
    reordered.splice(newIndex, 0, moved);

    // Update positions
    const updated = reordered.map((item, idx) => ({ ...item, position: idx }));
    setItems(updated);

    // Persist the moved item's new position
    await pmChecklistService.update(workspaceId, moved.id, { position: newIndex });
  };

  return (
    <div className="space-y-2" data-testid="checklist-table">
      <div className="flex items-center justify-between">
        <div className="flex min-w-0 flex-1 items-center gap-1.5" data-testid="checklist-heading">
          <CheckmarkSquare02Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <span className="text-xs font-semibold text-foreground/70 uppercase tracking-wide">
            Checklist
          </span>
          {items.length > 0 ? (
            <>
              <span className="shrink-0 text-xs font-normal text-muted-foreground" data-testid="checklist-count">
                ({completedCount}/{items.length})
              </span>
              <div
                className="mx-1 h-1.5 min-w-12 flex-1 overflow-hidden rounded-full bg-muted"
                data-testid="checklist-progress"
              >
                <div
                  className="h-full rounded-full bg-green-500 transition-all duration-300"
                  style={{ width: `${(completedCount / items.length) * 100}%` }}
                />
              </div>
              <span className="shrink-0 text-xs tabular-nums text-muted-foreground" data-testid="checklist-percentage">
                {Math.round((completedCount / items.length) * 100)}%
              </span>
            </>
          ) : null}
        </div>
        <QuickTooltip label="Add item">
          <button
            type="button"
            aria-label="Add checklist item"
            className="ml-4 shrink-0 rounded-md p-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            onClick={() => setAddingItem(true)}
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
          </button>
        </QuickTooltip>
      </div>

      {items.length > 0 ? (
        <div>
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
            <SortableContext items={items.map((i) => i.id)} strategy={verticalListSortingStrategy}>
              <div className="space-y-0.5">
                {items.map((item) => (
                  <SortableItem
                    key={item.id}
                    item={item}
                    onToggle={handleToggle}
                    onDelete={handleDelete}
                    onAssigneeChange={handleAssigneeChange}
                    members={members}
                    teams={teams}
                  />
                ))}
              </div>
            </SortableContext>
          </DndContext>
        </div>
      ) : (
        <p className="py-1 text-sm text-muted-foreground">No checklist items</p>
      )}

      {addingItem ? (
        <div className="relative py-1">
          <div className="flex items-center gap-2">
            <PlusSignIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground/50" />
            <input
              ref={inputRef}
              type="text"
              value={newText}
              placeholder="Add an item... (type @ to mention)"
              className="min-w-0 flex-1 bg-transparent py-1 text-sm placeholder:text-muted-foreground/50 focus:outline-none"
              disabled={adding}
              onChange={(e) => {
                setNewText(e.target.value);
                detectMention(e.target.value);
              }}
              onKeyDown={(e) => {
                if (mentionQuery !== null && mentionResults.length > 0) {
                  if (e.key === 'ArrowDown') {
                    e.preventDefault();
                    setMentionIndex((prev) => (prev + 1) % mentionResults.length);
                    return;
                  }
                  if (e.key === 'ArrowUp') {
                    e.preventDefault();
                    setMentionIndex((prev) => (prev - 1 + mentionResults.length) % mentionResults.length);
                    return;
                  }
                  if (e.key === 'Enter' || e.key === 'Tab') {
                    e.preventDefault();
                    insertMention(mentionResults[mentionIndex]);
                    return;
                  }
                  if (e.key === 'Escape') {
                    e.preventDefault();
                    e.stopPropagation();
                    setMentionQuery(null);
                    return;
                  }
                }
                if (e.key === 'Escape') {
                  e.preventDefault();
                  e.stopPropagation();
                  closeAddItemInput();
                }
                if (e.key === 'Enter') {
                  e.preventDefault();
                  handleAdd();
                }
              }}
              onBlur={() => {
                // Small delay to allow click on mention item.
                setTimeout(() => setMentionQuery(null), 150);
              }}
            />
            <QuickTooltip label="Close">
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                aria-label="Close checklist item input"
                onClick={closeAddItemInput}
              >
                <Cancel01Icon />
              </Button>
            </QuickTooltip>
            {newText.trim() && (
              <Button variant="outline" size="xs" disabled={adding} onClick={handleAdd}>
                <PlusSignIcon />
                Add item
              </Button>
            )}
          </div>
          {mentionQuery !== null && mentionResults.length > 0 && (
            <div className="absolute bottom-full left-6 z-50 mb-1.5 max-h-[260px] w-56 overflow-y-auto rounded-xl border border-border/60 bg-popover p-1.5 shadow-lg">
              <p className="px-2 pb-1 pt-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/50">
                Suggestions
              </p>
              <MentionSuggestionsList
                items={mentionResults}
                selectedIndex={mentionIndex}
                onSelect={insertMention}
              />
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}
