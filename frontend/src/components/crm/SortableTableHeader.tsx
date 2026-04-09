import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { DragDropVerticalIcon } from '@/lib/icons';

const NON_DRAGGABLE = new Set(['select', 'actions']);

interface SortableTableHeaderProps {
  headerId: string;
  columnId: string;
  disabled?: boolean;
  children: React.ReactNode;
}

export function SortableTableHeader({
  headerId,
  columnId,
  children,
}: SortableTableHeaderProps) {
  const disabled = NON_DRAGGABLE.has(columnId);

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: headerId,
    disabled,
  });

  const style: React.CSSProperties = {
    transform: CSS.Translate.toString(transform),
    transition,
    position: 'relative',
    zIndex: isDragging ? 50 : undefined,
    opacity: isDragging ? 0.8 : 1,
  };

  if (disabled) {
    return <>{children}</>;
  }

  return (
    <div ref={setNodeRef} style={style} className="flex items-center">
      <div className="flex-1">{children}</div>
      <button
        className="ml-1 flex h-full shrink-0 cursor-grab items-center opacity-0 group-hover/header:opacity-100 active:cursor-grabbing"
        {...attributes}
        {...listeners}
        onClick={(e) => e.stopPropagation()}
      >
        <DragDropVerticalIcon className="h-3 w-3 text-muted-foreground" />
      </button>
    </div>
  );
}
