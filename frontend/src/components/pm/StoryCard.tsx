import { useMemo } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import {
  AlertTriangle,
  CalendarDays,
  Circle,
} from 'lucide-react';
import { format, isBefore, parseISO } from 'date-fns';
import { Badge } from '@/components/ui/badge';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { cn } from '@/lib/utils';
import { PriorityIcon, StoryTypeIcon } from '@/lib/pmConstants';
import type { Story } from '@/lib/pmTypes';

interface StoryCardProps {
  story: Story;
  onOpen: (story: Story) => void;
  isOverlay?: boolean;
}

const toInitials = (ownerId?: string) => {
  if (!ownerId) return '??';
  return ownerId.slice(0, 2).toUpperCase();
};

export function StoryCard({ story, onOpen, isOverlay = false }: StoryCardProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: story.id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  const due = useMemo(() => {
    if (!story.deadline) return null;
    const date = parseISO(story.deadline);
    const overdue = isBefore(date, new Date()) && !story.completed;
    return {
      label: format(date, 'MMM d'),
      overdue,
    };
  }, [story.deadline, story.completed]);

  return (
    <article
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      role="button"
      tabIndex={0}
      onClick={() => onOpen(story)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen(story);
        }
      }}
      className={cn(
        'group rounded-md border border-border/60 bg-background px-3 py-2.5 transition-colors',
        'hover:bg-accent/40',
        isDragging && 'opacity-60',
        isOverlay && 'ring-1 ring-primary/30'
      )}
    >
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <StoryTypeIcon storyType={story.story_type} className="h-3.5 w-3.5" />
        <span className="font-medium text-foreground">TP-{story.display_id}</span>
        <span className="ml-auto" title={`Priority: ${story.priority}`}>
          <PriorityIcon priority={story.priority} className="h-3.5 w-3.5" />
        </span>
      </div>

      <h4 className="mt-1.5 line-clamp-2 text-sm font-semibold leading-snug">
        {story.name}
      </h4>

      <div className="mt-2 flex items-center gap-2 text-xs">
        {story.estimate !== undefined && story.estimate !== null ? (
          <Badge variant="secondary" className="h-5 rounded-full px-2 text-[10px]">
            {story.estimate} pts
          </Badge>
        ) : (
          <Badge variant="outline" className="h-5 rounded-full px-2 text-[10px] text-muted-foreground">
            No estimate
          </Badge>
        )}

        {story.blocked ? (
          <Badge variant="destructive" className="h-5 rounded-full px-2 text-[10px]">
            <AlertTriangle className="h-3 w-3" />
            Blocked
          </Badge>
        ) : (
          <Badge variant="outline" className="h-5 rounded-full px-2 text-[10px] text-muted-foreground">
            <Circle className="h-2.5 w-2.5" />
            Clear
          </Badge>
        )}

        <Avatar className="ml-auto h-5 w-5 border border-border/80">
          <AvatarFallback className="text-[9px] font-semibold bg-muted/60">
            {toInitials(story.owner_id)}
          </AvatarFallback>
        </Avatar>
      </div>

      <div className="mt-2 flex items-center justify-between text-[11px] text-muted-foreground">
        {story.epic_id ? (
          <span className="truncate max-w-[70%]">Epic {story.epic_id.slice(0, 8)}</span>
        ) : (
          <span>No epic</span>
        )}
        {due ? (
          <span className={cn('inline-flex items-center gap-1', due.overdue && 'text-red-600 font-medium')}>
            <CalendarDays className="h-3 w-3" />
            {due.label}
          </span>
        ) : (
          <span className="text-muted-foreground/70">No due date</span>
        )}
      </div>
    </article>
  );
}
