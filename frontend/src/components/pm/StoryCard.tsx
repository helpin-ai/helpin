import { useMemo } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import {
  AlertTriangle,
  CalendarDays,
} from 'lucide-react';
import { differenceInDays, format, isBefore, parseISO, startOfDay } from 'date-fns';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { PriorityIcon, SEVERITY_CONFIG, SeverityIcon, StoryTypeIcon } from '@/lib/pmConstants';
import { UserAvatar } from './UserAvatar';
import type { Story } from '@/lib/pmTypes';

interface StoryCardProps {
  story: Story;
  onOpen: (story: Story) => void;
  isOverlay?: boolean;
  teamName?: string;
}

export function StoryCard({ story, onOpen, isOverlay = false, teamName }: StoryCardProps) {
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
    const today = startOfDay(new Date());
    const overdue = isBefore(date, today) && !story.completed;
    const daysAway = differenceInDays(date, today);
    const approaching = !story.completed && !overdue && daysAway <= 3;
    return {
      label: format(date, 'MMM d'),
      overdue,
      approaching,
    };
  }, [story.deadline, story.completed]);

  const severityCfg = story.severity !== 'none' && story.severity in SEVERITY_CONFIG
    ? SEVERITY_CONFIG[story.severity]
    : null;

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
        'group cursor-pointer rounded-md border border-border/60 bg-background px-3 py-2.5 transition-colors',
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
        {story.estimate != null && (
          <Badge variant="secondary" className="h-5 rounded-full px-2 text-[10px]">
            {story.estimate} pts
          </Badge>
        )}

        {severityCfg && (
          <Badge variant="outline" className={cn('h-5 rounded-full px-2 text-[10px] font-medium', severityCfg.color)}>
            <SeverityIcon severity={story.severity} className="h-3 w-3" />
            {severityCfg.label}
          </Badge>
        )}

        {story.blocked && (
          <Badge variant="destructive" className="h-5 rounded-full px-2 text-[10px]">
            <AlertTriangle className="h-3 w-3" />
            Blocked
          </Badge>
        )}

        <UserAvatar name={story.owner_name} className="ml-auto h-5 w-5" />
      </div>

      <div className="mt-2 flex items-center justify-between text-[11px] text-muted-foreground">
        <div className="flex items-center gap-1.5 min-w-0">
          {teamName && (
            <Badge variant="outline" className="h-4 shrink-0 rounded px-1.5 text-[9px] font-medium">
              {teamName}
            </Badge>
          )}
          {story.epic_name && (
            <span className="truncate max-w-[70%]">{story.epic_name}</span>
          )}
        </div>
        {due && (
          <span className={cn(
            'inline-flex items-center gap-1',
            (due.overdue || due.approaching) && 'text-red-600 font-medium'
          )}>
            <CalendarDays className="h-3 w-3" />
            {due.label}
          </span>
        )}
      </div>
    </article>
  );
}
