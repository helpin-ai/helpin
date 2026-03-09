import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { CheckSquare, GripVertical, Plus, Trash2, X } from 'lucide-react';
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
import { UserAvatar } from '@/components/pm/UserAvatar';
import { MentionText } from '@/components/pm/MentionText';
import type { ChecklistItem } from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';

interface ChecklistItemsProps {
  workspaceId: string;
  storyId: string;
  members?: AssignableMember[];
}

function buildMemberHandle(member: AssignableMember): string {
  return member.display_name.toLowerCase().replace(/\s+/g, '.');
}

function SortableItem({
  item,
  onToggle,
  onDelete,
  onAssigneeChange,
  members = [],
}: {
  item: ChecklistItem;
  onToggle: (item: ChecklistItem) => void;
  onDelete: (id: string) => void;
  onAssigneeChange: (id: string, assigneeId: string | null) => void;
  members?: AssignableMember[];
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: item.id,
  });
  const [showPicker, setShowPicker] = useState(false);
  const pickerRef = useRef<HTMLDivElement>(null);

  const assignee = useMemo(
    () => members.find((m) => (m.user_id || m.id) === item.assignee_id),
    [members, item.assignee_id],
  );

  // Close picker on outside click.
  useEffect(() => {
    if (!showPicker) return;
    const handler = (e: MouseEvent) => {
      if (pickerRef.current && !pickerRef.current.contains(e.target as Node)) setShowPicker(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showPicker]);

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`group flex items-center gap-1.5 rounded-md px-1 py-1 hover:bg-accent/50 transition-colors ${isDragging ? 'opacity-50 bg-accent/50' : ''}`}
    >
      <button
        type="button"
        className="h-5 w-5 shrink-0 flex items-center justify-center cursor-grab text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing"
        {...attributes}
        {...listeners}
      >
        <GripVertical className="h-3 w-3" />
      </button>
      <input
        type="checkbox"
        checked={item.completed}
        onChange={() => onToggle(item)}
        className="h-3.5 w-3.5 shrink-0 rounded border-border cursor-pointer accent-primary"
      />
      <span
        className={`flex-1 text-sm ${
          item.completed ? 'line-through text-muted-foreground' : 'text-foreground'
        }`}
      >
        <MentionText text={item.text} members={members} />
      </span>

      {/* Assignee avatar / picker */}
      <div className="relative" ref={pickerRef}>
        {assignee ? (
          <button
            type="button"
            className="flex items-center gap-1 cursor-pointer group/assignee"
            onClick={() => setShowPicker(!showPicker)}
            title={assignee.display_name}
          >
            <UserAvatar
              name={assignee.display_name}
              avatarUrl={assignee.avatar_url}
              className="h-5 w-5 text-[8px]"
            />
            <button
              type="button"
              className="h-3.5 w-3.5 flex items-center justify-center rounded-full opacity-0 group-hover/assignee:opacity-100 text-muted-foreground hover:text-foreground transition-opacity cursor-pointer"
              onClick={(e) => {
                e.stopPropagation();
                onAssigneeChange(item.id, null);
              }}
            >
              <X className="h-2.5 w-2.5" />
            </button>
          </button>
        ) : (
          <button
            type="button"
            className="h-5 w-5 shrink-0 flex items-center justify-center rounded-full border border-dashed border-border/60 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-foreground hover:border-foreground/40 cursor-pointer"
            onClick={() => setShowPicker(!showPicker)}
            title="Assign member"
          >
            <Plus className="h-2.5 w-2.5" />
          </button>
        )}
        {showPicker && (
          <div className="absolute right-0 top-full z-50 mt-1 w-52 rounded-lg border border-border/60 bg-popover shadow-lg overflow-hidden">
            <div className="max-h-48 overflow-y-auto py-1">
              {members
                .filter((m) => m.status === 'active')
                .map((m) => {
                  const uid = m.user_id || m.id;
                  return (
                    <button
                      key={uid}
                      type="button"
                      className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-accent transition-colors cursor-pointer"
                      onClick={() => {
                        onAssigneeChange(item.id, uid);
                        setShowPicker(false);
                      }}
                    >
                      <UserAvatar
                        name={m.display_name}
                        avatarUrl={m.avatar_url}
                        className="h-5 w-5 text-[8px]"
                      />
                      <span className="truncate">{m.display_name}</span>
                    </button>
                  );
                })}
            </div>
          </div>
        )}
      </div>

      <button
        type="button"
        className="h-5 w-5 shrink-0 flex items-center justify-center rounded opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-destructive cursor-pointer"
        onClick={() => onDelete(item.id)}
      >
        <Trash2 className="h-3 w-3" />
      </button>
    </div>
  );
}

export function ChecklistItems({ workspaceId, storyId, members = [] }: ChecklistItemsProps) {
  const [items, setItems] = useState<ChecklistItem[]>([]);
  const [newText, setNewText] = useState('');
  const [adding, setAdding] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const textRef = useRef(newText);
  textRef.current = newText;

  // Mention autocomplete state.
  const [mentionQuery, setMentionQuery] = useState<string | null>(null);
  const [mentionIndex, setMentionIndex] = useState(0);

  const mentionResults = useMemo(() => {
    if (mentionQuery === null) return [];
    const q = mentionQuery.toLowerCase();
    return members
      .filter(
        (m) =>
          m.status === 'active' &&
          (!q ||
            m.display_name.toLowerCase().includes(q) ||
            buildMemberHandle(m).includes(q) ||
            m.email.toLowerCase().includes(q)),
      )
      .slice(0, 6);
  }, [mentionQuery, members]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const reload = useCallback(async () => {
    const { data } = await pmChecklistService.list(workspaceId, storyId);
    if (data) setItems(data.sort((a, b) => a.position - b.position));
  }, [workspaceId, storyId]);

  useEffect(() => { reload(); }, [reload]);

  // Re-fetch when another client changes checklist items
  useEffect(() => {
    const handler = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.parent_id === storyId && d?.entity === 'checklist_item') reload();
    };
    window.addEventListener('story-child-updated', handler);
    return () => window.removeEventListener('story-child-updated', handler);
  }, [storyId, reload]);

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

  const insertMention = (member: AssignableMember) => {
    const handle = buildMemberHandle(member);
    const text = newText.replace(/(?:^|\s)@[a-z0-9._-]*$/i, (match) => {
      const prefix = match.startsWith(' ') ? ' ' : '';
      return `${prefix}@${handle} `;
    });
    setNewText(text);
    setMentionQuery(null);
    inputRef.current?.focus();
  };

  const handleAdd = async () => {
    const text = textRef.current.trim();
    if (!text || adding) return;
    setAdding(true);
    const { data } = await pmChecklistService.create(workspaceId, storyId, { text });
    setAdding(false);
    if (!data) return;
    // Don't optimistically append — the WebSocket event triggers reload()
    // which fetches the full list, avoiding duplicates.
    await reload();
    setNewText('');
    inputRef.current?.focus();
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
    await pmChecklistService.update(workspaceId, id, { assignee_id: assigneeId ?? undefined });
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
    <div className="space-y-2">
      <div className="flex items-center gap-1.5">
        <CheckSquare className="h-3.5 w-3.5 text-muted-foreground" />
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
          Checklist
          {items.length > 0 && (
            <span className="ml-1 font-normal">
              {completedCount}/{items.length}
            </span>
          )}
        </h3>
      </div>

      {/* Progress bar */}
      {items.length > 0 && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground tabular-nums w-8 text-right">
            {Math.round((completedCount / items.length) * 100)}%
          </span>
          <div className="flex-1 h-1.5 rounded-full bg-muted overflow-hidden">
            <div
              className={`h-full rounded-full transition-all duration-300 ${
                completedCount === items.length ? 'bg-green-500' : 'bg-primary'
              }`}
              style={{ width: `${(completedCount / items.length) * 100}%` }}
            />
          </div>
        </div>
      )}

      {/* Items */}
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
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>

      {/* Add input */}
      <div className="relative">
        <div className="flex items-center gap-2">
          <Plus className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <input
            ref={inputRef}
            type="text"
            value={newText}
            placeholder="Add an item... (type @ to mention)"
            className="flex-1 bg-transparent text-sm placeholder:text-muted-foreground/50 focus:outline-none"
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
                  setMentionQuery(null);
                  return;
                }
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
        </div>
        {/* Mention autocomplete dropdown */}
        {mentionQuery !== null && mentionResults.length > 0 && (
          <div className="absolute left-6 bottom-full z-50 mb-1 w-52 rounded-lg border border-border/60 bg-popover shadow-lg overflow-hidden">
            <div className="py-1">
              <div className="mb-1 px-3 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                Mention
              </div>
              {mentionResults.map((m, idx) => (
                <button
                  key={m.user_id || m.id}
                  type="button"
                  className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm transition-colors cursor-pointer ${
                    idx === mentionIndex ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                  }`}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    insertMention(m);
                  }}
                >
                  <UserAvatar
                    name={m.display_name}
                    avatarUrl={m.avatar_url}
                    className="h-5 w-5 text-[8px]"
                  />
                  <span className="flex-1 truncate">{m.display_name}</span>
                  <span className="font-mono text-xs text-muted-foreground">@{buildMemberHandle(m)}</span>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
