import { useCallback, useMemo, useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import {
  AlertTriangle,
  CalendarDays,
  Check,
  UserPlus,
} from 'lucide-react';
import { differenceInDays, format, isBefore, parseISO, startOfDay } from 'date-fns';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { cn } from '@/lib/utils';
import { PRIORITY_CONFIG, PriorityIcon, SEVERITY_CONFIG, SeverityIcon, StateTypeIcon, STORY_TYPE_CONFIG, StoryTypeIcon } from '@/lib/pmConstants';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { UserAvatar } from './UserAvatar';
import type { Priority, Story, WorkflowState } from '@/lib/pmTypes';
import type { MemberWithUser } from '@/lib/types';

// ── Shared constants ────────────────────────────────────────────────

const pillBase = 'flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium';

const PRIORITY_BORDER_COLOR: Record<Priority, string> = {
  urgent: 'border-red-400 dark:border-red-600',
  high: 'border-orange-400 dark:border-orange-600',
  medium: 'border-amber-400 dark:border-amber-600',
  low: 'border-sky-400 dark:border-sky-600',
  none: 'border-border',
};

const ALL_PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];

// ── Component ───────────────────────────────────────────────────────

interface StoryCardProps {
  story: Story;
  onOpen: (story: Story) => void;
  isOverlay?: boolean;
  teamName?: string;
  workspaceId?: string;
  members?: MemberWithUser[];
  states?: WorkflowState[];
  onOwnerChanged?: () => void;
  onStoryMoved?: (storyId: string, fromStateId: string, toStateId: string) => void;
  onPriorityChanged?: () => void;
}

export function StoryCard({
  story,
  onOpen,
  isOverlay = false,
  teamName,
  workspaceId,
  members,
  states,
  onOwnerChanged,
  onStoryMoved,
  onPriorityChanged,
}: StoryCardProps) {
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

  const [memberOpen, setMemberOpen] = useState(false);
  const [priorityOpen, setPriorityOpen] = useState(false);
  const [stateOpen, setStateOpen] = useState(false);

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

  const priorityCfg = PRIORITY_CONFIG[story.priority];
  const storyTypeCfg = STORY_TYPE_CONFIG[story.story_type];
  const currentState = states?.find((s) => s.id === story.workflow_state_id);

  const handleAssignOwner = useCallback(
    async (member: MemberWithUser) => {
      if (!workspaceId) return;
      const newOwnerId = story.owner_id === member.user_id ? undefined : member.user_id;
      try {
        await pmStoryService.update(workspaceId, story.id, { owner_id: newOwnerId });
        onOwnerChanged?.();
      } catch {
        // Board will show stale data until next refresh
      }
      setMemberOpen(false);
    },
    [workspaceId, story.id, story.owner_id, onOwnerChanged],
  );

  const handleChangePriority = useCallback(
    async (priority: Priority) => {
      if (!workspaceId || priority === story.priority) {
        setPriorityOpen(false);
        return;
      }
      try {
        await pmStoryService.update(workspaceId, story.id, { priority });
        onPriorityChanged?.();
      } catch {
        // Board will show stale data until next refresh
      }
      setPriorityOpen(false);
    },
    [workspaceId, story.id, story.priority, onPriorityChanged],
  );

  const handleChangeState = useCallback(
    (stateId: string) => {
      if (stateId === story.workflow_state_id) {
        setStateOpen(false);
        return;
      }
      onStoryMoved?.(story.id, story.workflow_state_id, stateId);
      setStateOpen(false);
    },
    [story.id, story.workflow_state_id, onStoryMoved],
  );

  const titleIsLong = story.name.length > 60;

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
        'group cursor-pointer rounded-lg border border-border/60 bg-background p-3 shadow-sm transition-all',
        'hover:border-border hover:shadow-md',
        isDragging && 'opacity-50',
        isOverlay && 'ring-1 ring-primary/30 shadow-lg',
      )}
    >
      {/* Row 1: ID + Story type + Priority */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="shrink-0">
              <StoryTypeIcon storyType={story.story_type} className="h-3.5 w-3.5" />
            </span>
          </TooltipTrigger>
          <TooltipContent side="top">{storyTypeCfg.label}</TooltipContent>
        </Tooltip>
        <span className="font-medium text-foreground/80">TP-{story.display_id}</span>

        {/* Priority pill — clickable dropdown */}
        {workspaceId ? (
          <Popover open={priorityOpen} onOpenChange={setPriorityOpen}>
            <Tooltip open={priorityOpen ? false : undefined}>
              <TooltipTrigger asChild>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className={cn(
                      'ml-auto flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1 transition-colors hover:bg-muted',
                      PRIORITY_BORDER_COLOR[story.priority],
                    )}
                    onClick={(e) => { e.stopPropagation(); setPriorityOpen(true); }}
                  >
                    <PriorityIcon priority={story.priority} className="h-3.5 w-3.5" />
                  </button>
                </PopoverTrigger>
              </TooltipTrigger>
              <TooltipContent side="top">Priority: {priorityCfg.label}</TooltipContent>
            </Tooltip>
            {priorityOpen && (
              <PopoverContent
                className="w-[180px] p-0"
                align="end"
                side="bottom"
                onClick={(e) => e.stopPropagation()}
                onKeyDown={(e) => e.stopPropagation()}
              >
                <Command>
                  <CommandInput placeholder="Search..." className="h-8 text-xs" />
                  <CommandList>
                    <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
                    <CommandGroup>
                      {ALL_PRIORITIES.map((p) => {
                        const cfg = PRIORITY_CONFIG[p];
                        return (
                          <CommandItem
                            key={p}
                            value={cfg.label}
                            onSelect={() => handleChangePriority(p)}
                            className="flex items-center gap-2 text-xs"
                          >
                            <PriorityIcon priority={p} className="h-3.5 w-3.5" />
                            <span>{cfg.label}</span>
                            {story.priority === p && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
                          </CommandItem>
                        );
                      })}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            )}
          </Popover>
        ) : (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(
                'ml-auto flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1',
                PRIORITY_BORDER_COLOR[story.priority],
              )}>
                <PriorityIcon priority={story.priority} className="h-3.5 w-3.5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Priority: {priorityCfg.label}</TooltipContent>
          </Tooltip>
        )}
      </div>

      {/* Row 2: Title */}
      {titleIsLong ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <h4 className="mt-1.5 line-clamp-2 text-[13px] font-medium leading-snug text-foreground">
              {story.name}
            </h4>
          </TooltipTrigger>
          <TooltipContent side="bottom" className="max-w-[300px]">{story.name}</TooltipContent>
        </Tooltip>
      ) : (
        <h4 className="mt-1.5 line-clamp-2 text-[13px] font-medium leading-snug text-foreground">
          {story.name}
        </h4>
      )}

      {/* Row 3: Property pills */}
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        {/* State pill — clickable dropdown */}
        {currentState && states && states.length > 0 && onStoryMoved ? (
          <Popover open={stateOpen} onOpenChange={setStateOpen}>
            <Tooltip open={stateOpen ? false : undefined}>
              <TooltipTrigger asChild>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className={cn(pillBase, 'border-border bg-muted/50 text-muted-foreground transition-colors hover:bg-muted')}
                    onClick={(e) => { e.stopPropagation(); setStateOpen(true); }}
                  >
                    <StateTypeIcon stateType={currentState.state_type} className="h-3 w-3" />
                    {currentState.name}
                  </button>
                </PopoverTrigger>
              </TooltipTrigger>
              <TooltipContent side="top">State: {currentState.name}</TooltipContent>
            </Tooltip>
            {stateOpen && (
              <PopoverContent
                className="w-[200px] p-0"
                align="start"
                side="bottom"
                onClick={(e) => e.stopPropagation()}
                onKeyDown={(e) => e.stopPropagation()}
              >
                <Command>
                  <CommandInput placeholder="Search..." className="h-8 text-xs" />
                  <CommandList>
                    <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
                    <CommandGroup>
                      {states.map((s) => (
                        <CommandItem
                          key={s.id}
                          value={s.name}
                          onSelect={() => handleChangeState(s.id)}
                          className="flex items-center gap-2 text-xs"
                        >
                          <StateTypeIcon stateType={s.state_type} className="h-3.5 w-3.5" />
                          <span>{s.name}</span>
                          {story.workflow_state_id === s.id && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            )}
          </Popover>
        ) : currentState ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(pillBase, 'border-border bg-muted/50 text-muted-foreground')}>
                <StateTypeIcon stateType={currentState.state_type} className="h-3 w-3" />
                {currentState.name}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">State: {currentState.name}</TooltipContent>
          </Tooltip>
        ) : null}

        {story.estimate != null && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(pillBase, 'border-border bg-muted/50 text-muted-foreground')}>
                {story.estimate} pts
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Estimate: {story.estimate} points</TooltipContent>
          </Tooltip>
        )}

        {severityCfg && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(pillBase, 'border-border bg-muted/50', severityCfg.color)}>
                <SeverityIcon severity={story.severity} className="h-3 w-3" />
                {severityCfg.label}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Severity: {severityCfg.label}</TooltipContent>
          </Tooltip>
        )}

        {story.blocked && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(pillBase, 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400')}>
                <AlertTriangle className="h-3 w-3" />
                Blocked
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">This story is blocked</TooltipContent>
          </Tooltip>
        )}

        {due && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(
                pillBase,
                due.overdue
                  ? 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400'
                  : due.approaching
                    ? 'border-amber-300 bg-amber-50 text-amber-600 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-400'
                    : 'border-border bg-muted/50 text-muted-foreground',
              )}>
                <CalendarDays className="h-3 w-3 shrink-0" />
                {due.label}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">
              {due.overdue ? 'Overdue' : due.approaching ? 'Due soon' : 'Due date'}: {due.label}
            </TooltipContent>
          </Tooltip>
        )}
      </div>

      {/* Row 4: Footer - team, epic, assignee */}
      <div className="mt-2 flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5 min-w-0">
          {teamName && (
            <span className={cn(pillBase, 'shrink-0 border-border bg-muted/50 text-muted-foreground')}>
              {teamName}
            </span>
          )}
          {story.epic_name && (
            <span className="truncate text-[11px] text-muted-foreground max-w-[120px]">{story.epic_name}</span>
          )}
        </div>

        {/* Assignee avatar / assign button */}
        {members && workspaceId ? (
          <Popover open={memberOpen} onOpenChange={setMemberOpen}>
            <Tooltip open={memberOpen ? false : undefined}>
              <TooltipTrigger asChild>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className="shrink-0 rounded-full transition-opacity hover:opacity-80"
                    onClick={(e) => {
                      e.stopPropagation();
                      setMemberOpen(true);
                    }}
                  >
                    {story.owner_name ? (
                      <UserAvatar name={story.owner_name} className="h-5 w-5" />
                    ) : (
                      <span className="flex h-5 w-5 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground transition-colors hover:border-primary/40 hover:text-primary">
                        <UserPlus className="h-2.5 w-2.5" />
                      </span>
                    )}
                  </button>
                </PopoverTrigger>
              </TooltipTrigger>
              <TooltipContent side="top">{story.owner_name || 'Assign member'}</TooltipContent>
            </Tooltip>
            {memberOpen && (
              <PopoverContent
                className="w-[220px] p-0"
                align="end"
                side="bottom"
                onClick={(e) => e.stopPropagation()}
                onKeyDown={(e) => e.stopPropagation()}
              >
                <Command>
                  <CommandInput placeholder="Search members..." className="h-8 text-xs" />
                  <CommandList>
                    <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">
                      No members found
                    </CommandEmpty>
                    <CommandGroup>
                      {members.map((m) => (
                        <CommandItem
                          key={m.user_id}
                          value={m.full_name || m.email}
                          onSelect={() => handleAssignOwner(m)}
                          className="flex items-center gap-2 text-xs"
                        >
                          <UserAvatar name={m.full_name || m.email} className="h-5 w-5" />
                          <span className="truncate">{m.full_name || m.email}</span>
                          {story.owner_id === m.user_id && (
                            <Check className="ml-auto h-3.5 w-3.5 text-primary" />
                          )}
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            )}
          </Popover>
        ) : (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="shrink-0">
                <UserAvatar name={story.owner_name} className="h-5 w-5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">{story.owner_name || 'Unassigned'}</TooltipContent>
          </Tooltip>
        )}
      </div>
    </article>
  );
}
