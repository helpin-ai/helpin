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
import { PRIORITY_CONFIG, PriorityIcon, SEVERITY_CONFIG, SeverityIcon, STORY_TYPE_CONFIG, StoryTypeIcon } from '@/lib/pmConstants';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { UserAvatar } from './UserAvatar';
import type { Priority, Severity, Story } from '@/lib/pmTypes';
import type { MemberWithUser } from '@/lib/types';
import { formatEstimateDisplay } from '@/components/pm/EstimatePicker';
import { LabelBadge } from '@/components/pm/LabelPicker';
import { useTeamFieldVisibilityStore } from '@/stores/teamFieldVisibilityStore';
import { useBoardDisplayStore } from '@/stores/boardDisplayStore';

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
const ALL_SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];

// ── Component ───────────────────────────────────────────────────────

interface StoryCardProps {
  story: Story;
  onOpen: (story: Story) => void;
  isOverlay?: boolean;
  teamName?: string;
  workspaceId?: string;
  members?: MemberWithUser[];
  onOwnerChanged?: (story: Story) => void;
  onPriorityChanged?: (story: Story) => void;
  onSeverityChanged?: (story: Story) => void;
}

export function StoryCard({
  story,
  onOpen,
  isOverlay = false,
  teamName,
  workspaceId,
  members,
  onOwnerChanged,
  onPriorityChanged,
  onSeverityChanged,
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
  const [severityOpen, setSeverityOpen] = useState(false);
  const fieldVis = useTeamFieldVisibilityStore((s) => s.getForTeam(story.team_id));
  const displayProps = useBoardDisplayStore((s) => s.properties);
  const vis = useMemo(() => ({
    story_type: fieldVis.story_type && displayProps.story_type,
    priority: fieldVis.priority && displayProps.priority,
    severity: fieldVis.severity && displayProps.severity,
    epic: fieldVis.epic && displayProps.epic,
    labels: (fieldVis.labels ?? true) && displayProps.labels,
    estimate: fieldVis.estimate && displayProps.estimate,
    due_date: fieldVis.due_date && displayProps.due_date,
    blocked: fieldVis.blocked && displayProps.blocked,
    assignee: displayProps.assignee,
  }), [fieldVis, displayProps]);

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
  const handleAssignOwner = useCallback(
    async (member: MemberWithUser) => {
      if (!workspaceId) return;
      const newOwnerId = story.owner_id === member.user_id ? undefined : member.user_id;
      try {
        const result = await pmStoryService.update(workspaceId, story.id, { owner_id: newOwnerId });
        if (result.data?.story) {
          onOwnerChanged?.(result.data.story);
        }
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
        const result = await pmStoryService.update(workspaceId, story.id, { priority });
        if (result.data?.story) {
          onPriorityChanged?.(result.data.story);
        }
      } catch {
        // Board will show stale data until next refresh
      }
      setPriorityOpen(false);
    },
    [workspaceId, story.id, story.priority, onPriorityChanged],
  );

  const handleChangeSeverity = useCallback(
    async (severity: Severity) => {
      if (!workspaceId || severity === story.severity) {
        setSeverityOpen(false);
        return;
      }
      try {
        const result = await pmStoryService.update(workspaceId, story.id, { severity });
        if (result.data?.story) {
          onSeverityChanged?.(result.data.story);
        }
      } catch {
        // Board will show stale data until next refresh
      }
      setSeverityOpen(false);
    },
    [workspaceId, story.id, story.severity, onSeverityChanged],
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
      {/* Row 1: Story type + Epic + Team + Priority */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {vis.story_type && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="shrink-0">
              <StoryTypeIcon storyType={story.story_type} className="h-3.5 w-3.5" />
            </span>
          </TooltipTrigger>
          <TooltipContent side="top">{storyTypeCfg.label}</TooltipContent>
        </Tooltip>
        )}
        {vis.epic && story.epic_name && (
          <span className="truncate text-[11px] text-muted-foreground max-w-[120px]">{story.epic_name}</span>
        )}

        <span className="flex-1" />

        {teamName && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(pillBase, 'shrink-0 border-border bg-muted/50 text-muted-foreground')}>
                {teamName}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Team: {teamName}</TooltipContent>
          </Tooltip>
        )}

        {/* Priority pill — clickable dropdown */}
        {vis.priority && (workspaceId ? (
          <Popover open={priorityOpen} onOpenChange={setPriorityOpen}>
            <Tooltip open={priorityOpen ? false : undefined}>
              <TooltipTrigger asChild>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className={cn(
                      'flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1 transition-colors hover:bg-muted',
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
                'flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1',
                PRIORITY_BORDER_COLOR[story.priority],
              )}>
                <PriorityIcon priority={story.priority} className="h-3.5 w-3.5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Priority: {priorityCfg.label}</TooltipContent>
          </Tooltip>
        ))}
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
        {/* Severity pill — clickable dropdown */}
        {vis.severity && (severityCfg && workspaceId ? (
          <Popover open={severityOpen} onOpenChange={setSeverityOpen}>
            <Tooltip open={severityOpen ? false : undefined}>
              <TooltipTrigger asChild>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className={cn(pillBase, 'border-border bg-muted/50 transition-colors hover:bg-muted', severityCfg.color)}
                    onClick={(e) => { e.stopPropagation(); setSeverityOpen(true); }}
                  >
                    <SeverityIcon severity={story.severity} className="h-3 w-3" />
                    {severityCfg.label}
                  </button>
                </PopoverTrigger>
              </TooltipTrigger>
              <TooltipContent side="top">Severity: {severityCfg.label}</TooltipContent>
            </Tooltip>
            {severityOpen && (
              <PopoverContent
                className="w-[180px] p-0"
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
                      {ALL_SEVERITIES.map((sev) => {
                        const cfg = SEVERITY_CONFIG[sev];
                        return (
                          <CommandItem
                            key={sev}
                            value={cfg.label}
                            onSelect={() => handleChangeSeverity(sev)}
                            className="flex items-center gap-2 text-xs"
                          >
                            <SeverityIcon severity={sev} className="h-3.5 w-3.5" />
                            <span>{cfg.label}</span>
                            {story.severity === sev && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
                          </CommandItem>
                        );
                      })}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            )}
          </Popover>
        ) : severityCfg ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(pillBase, 'border-border bg-muted/50', severityCfg.color)}>
                <SeverityIcon severity={story.severity} className="h-3 w-3" />
                {severityCfg.label}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Severity: {severityCfg.label}</TooltipContent>
          </Tooltip>
        ) : null)}

        {vis.blocked && story.blocked && (
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

        {/* Labels */}
        {vis.labels && story.labels && story.labels.length > 0 && story.labels.map((label) => (
          <LabelBadge key={label.id} label={label} />
        ))}
      </div>

      {/* Row 4: Footer - due date, estimate + assignee */}
      <div className="mt-2 flex items-center gap-1.5">
        {vis.due_date && due && (
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
        {vis.estimate && story.estimate != null && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(pillBase, 'border-border bg-muted/50 text-muted-foreground')}>
                {formatEstimateDisplay(story.estimate, story.team_id)}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Estimate: {formatEstimateDisplay(story.estimate, story.team_id)}</TooltipContent>
          </Tooltip>
        )}
        <span className="flex-1" />
        {/* Assignee avatar / assign button */}
        {vis.assignee && (members && workspaceId ? (
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
        ))}
      </div>
    </article>
  );
}
