import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { CheckSquare, GripVertical, Plus, Trash2 } from 'lucide-react';
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
import type { ChecklistItem } from '@/lib/pmTypes';

interface ChecklistItemsProps {
  workspaceId: string;
  storyId: string;
}

function SortableItem({
  item,
  onToggle,
  onDelete,
}: {
  item: ChecklistItem;
  onToggle: (item: ChecklistItem) => void;
  onDelete: (id: string) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: item.id,
  });

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
        {item.text}
      </span>
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

export function ChecklistItems({ workspaceId, storyId }: ChecklistItemsProps) {
  const [items, setItems] = useState<ChecklistItem[]>([]);
  const [newText, setNewText] = useState('');
  const [adding, setAdding] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const textRef = useRef(newText);
  textRef.current = newText;

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

  const handleAdd = async () => {
    const text = textRef.current.trim();
    if (!text || adding) return;
    setAdding(true);
    const { data } = await pmChecklistService.create(workspaceId, storyId, { text });
    setAdding(false);
    if (!data) return;
    setItems((prev) => [...prev, data]);
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
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>

      {/* Add input */}
      <div className="flex items-center gap-2">
        <Plus className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <input
          ref={inputRef}
          type="text"
          value={newText}
          placeholder="Add an item..."
          className="flex-1 bg-transparent text-sm placeholder:text-muted-foreground/50 focus:outline-none"
          disabled={adding}
          onChange={(e) => setNewText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              handleAdd();
            }
          }}
        />
      </div>
    </div>
  );
}
