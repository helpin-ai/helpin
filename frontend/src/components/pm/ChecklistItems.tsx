import { useEffect, useRef, useState } from 'react';
import { CheckSquare, Plus, Trash2 } from 'lucide-react';
import { pmChecklistService } from '@/lib/services/pmChecklistService';
import type { ChecklistItem } from '@/lib/pmTypes';

interface ChecklistItemsProps {
  workspaceId: string;
  storyId: string;
}

export function ChecklistItems({ workspaceId, storyId }: ChecklistItemsProps) {
  const [items, setItems] = useState<ChecklistItem[]>([]);
  const [newText, setNewText] = useState('');
  const [adding, setAdding] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const textRef = useRef(newText);
  textRef.current = newText;

  useEffect(() => {
    (async () => {
      const { data } = await pmChecklistService.list(workspaceId, storyId);
      setItems(data ?? []);
    })();
  }, [workspaceId, storyId]);

  const completedCount = items.filter((i) => i.completed).length;

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

  return (
    <div className="max-w-sm space-y-2">
      <div className="flex items-center gap-1.5">
        <CheckSquare className="h-3.5 w-3.5 text-muted-foreground" />
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Checklist</h3>
        {items.length > 0 && (
          <span className="text-[11px] text-muted-foreground">
            {completedCount} of {items.length}
          </span>
        )}
      </div>

      {/* Progress bar */}
      {items.length > 0 && (
        <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-primary transition-all duration-300"
            style={{ width: `${(completedCount / items.length) * 100}%` }}
          />
        </div>
      )}

      {/* Items */}
      <div className="space-y-0.5">
        {items.map((item) => (
          <div
            key={item.id}
            className="group flex items-center gap-2 rounded-md px-1 py-1 hover:bg-accent/50 transition-colors"
          >
            <input
              type="checkbox"
              checked={item.completed}
              onChange={() => handleToggle(item)}
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
              onClick={() => handleDelete(item.id)}
            >
              <Trash2 className="h-3 w-3" />
            </button>
          </div>
        ))}
      </div>

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
