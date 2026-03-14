import { useEffect, useMemo, useState } from 'react';
import { differenceInDays, parseISO, format } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';
import {
  AlertCircle,
  BarChart3,
  CalendarDays,
  CircleDot,
  ClipboardCheck,
  Clock,
  Layers,
  PenLine,
  Plus,
  SquareKanban,
  Target,
  Timer,
  Users,
} from 'lucide-react';
import { useTitle } from '@/hooks/useTitle';
import { Button } from '@/components/ui/button';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { useStoryPanelStore } from '@/stores/storyPanelStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { PRIORITY_BORDER_COLOR, PRIORITY_CONFIG, StateTypeIcon, StoryTypeIcon, PriorityIcon } from '@/lib/pmConstants';
import type { Priority, Story, StateType } from '@/lib/pmTypes';

type Mode = 'assigned' | 'requested';
type DeadlineStatus = 'overdue' | 'approaching' | 'normal';

const DEADLINE_PILL_STYLE: Record<DeadlineStatus, string> = {
  overdue: 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400',
  approaching: 'border-amber-300 bg-amber-50 text-amber-600 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-400',
  normal: 'border-border bg-muted/50 text-muted-foreground',
};

const DEADLINE_TOOLTIP: Record<DeadlineStatus, string> = {
  overdue: 'Overdue',
  approaching: 'Due soon',
  normal: 'Due date',
};



export function MyWorkPage() {
  useTitle('My Work');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const openStoryPanel = useStoryPanelStore((s) => s.openStory);

  const { data: access } = useWorkspaceAccess(workspaceId);
  const memberId = access?.membership?.id;

  const { teams, findTeamName } = useWorkspaceTeams(workspaceId);
  const showTeam = teams.length > 1;

  const [mode, setMode] = useState<Mode>('assigned');
  const [stories, setStories] = useState<Story[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    if (!workspaceId || !memberId) return;
    setLoading(true);
    const filters =
      mode === 'assigned'
        ? { owner_member_id: memberId, archived: false as const }
        : { requester_member_id: memberId, archived: false as const };

    pmStoryService
      .list(workspaceId, { ...filters, per_page: 200 })
      .then((res) => {
        if (res.data) {
          setStories(res.data.data);
        }
      })
      .finally(() => setLoading(false));
  }, [workspaceId, memberId, mode, refreshKey]);

  // Refresh list when a story is updated or archived via the global panel
  useEffect(() => {
    const refresh = () => setRefreshKey((k) => k + 1);
    window.addEventListener('story-panel-updated', refresh);
    window.addEventListener('story-panel-archived', refresh);
    return () => {
      window.removeEventListener('story-panel-updated', refresh);
      window.removeEventListener('story-panel-archived', refresh);
    };
  }, []);

  // ── Derived data ──────────────────────────────────────────────────

  const counts = useMemo(() => {
    const now = new Date();
    let inProgress = 0;
    let dueSoon = 0;
    let overdue = 0;
    let blocked = 0;

    for (const s of stories) {
      if (s.completed) continue;
      if (s.state_type === 'started') inProgress++;
      if (s.blocked) blocked++;
      if (s.deadline) {
        const days = differenceInDays(parseISO(s.deadline), now);
        if (days < 0) overdue++;
        else if (days <= 3) dueSoon++;
      }
    }
    return { inProgress, dueSoon, overdue, blocked };
  }, [stories]);

  const { focus, blockedStories, rest } = useMemo(() => {
    const now = new Date();
    const active = stories.filter((s) => !s.completed);
    const done = stories.filter((s) => s.completed);

    const scoreFn = (s: Story): number => {
      let score = 0;
      if (s.deadline) {
        const days = differenceInDays(parseISO(s.deadline), now);
        if (days < 0) score += 1000 + Math.abs(days);
        else if (days <= 3) score += 500;
      }
      if (s.blocked) score += 400;
      if (s.priority === 'urgent') score += 300;
      else if (s.priority === 'high') score += 200;
      if (s.state_type === 'started') score += 100;
      const updatedDaysAgo = differenceInDays(now, parseISO(s.updated_at));
      if (updatedDaysAgo <= 1) score += 50;
      return score;
    };

    const sorted = [...active].sort((a, b) => scoreFn(b) - scoreFn(a));

    const blocked: Story[] = [];
    const focusItems: Story[] = [];
    const restItems: Story[] = [];

    for (const s of sorted) {
      if (s.blocked) {
        blocked.push(s);
      } else if (scoreFn(s) >= 100) {
        focusItems.push(s);
      } else {
        restItems.push(s);
      }
    }

    const recentDone = done
      .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
      .slice(0, 10);

    return { focus: focusItems, blockedStories: blocked, rest: [...restItems, ...recentDone] };
  }, [stories]);

  // ── Render ────────────────────────────────────────────────────────

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const openStory = (s: Story) => openStoryPanel(s.id);

  return (
    <div className="max-w-4xl mx-auto">
      <header className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-semibold">My Work</h2>
          <p className="text-[13px] text-muted-foreground">
            {mode === 'assigned'
              ? 'Stories assigned to you across all teams.'
              : 'Stories you requested across all teams.'}
          </p>
        </div>

        {/* Mode toggle */}
        <div className="flex gap-0.5 rounded-lg bg-muted/60 p-0.5">
          {(['assigned', 'requested'] as const).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                mode === m
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              {m === 'assigned' ? 'Assigned to me' : 'Requested by me'}
            </button>
          ))}
        </div>
      </header>

      {/* Summary cards */}
      {stories.length > 0 && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-8">
          <SummaryCard icon={CircleDot} iconColor="text-amber-500" label="In progress" value={counts.inProgress} />
          <SummaryCard icon={Clock} iconColor="text-blue-500" label="Due soon" value={counts.dueSoon} />
          <SummaryCard icon={Timer} iconColor="text-red-500" label="Overdue" value={counts.overdue} />
          <SummaryCard icon={AlertCircle} iconColor="text-orange-500" label="Blocked" value={counts.blocked} />
        </div>
      )}

      {/* Content */}
      {loading ? null : stories.length === 0 ? (
        <MyWorkEmptyState mode={mode} workspaceSlug={workspace.slug} />
      ) : (
        <div className="space-y-10">
          {focus.length > 0 && (
            <StorySection title="Focus now" count={focus.length} stories={focus} onClickStory={openStory} findTeamName={findTeamName} showTeam={showTeam} />
          )}
          {blockedStories.length > 0 && (
            <StorySection title="Blocked" count={blockedStories.length} stories={blockedStories} onClickStory={openStory} findTeamName={findTeamName} showTeam={showTeam} />
          )}
          {rest.length > 0 && (
            <StorySection title="Everything else" count={rest.length} stories={rest} onClickStory={openStory} findTeamName={findTeamName} showTeam={showTeam} />
          )}
        </div>
      )}
    </div>
  );
}

// ── Empty state ───────────────────────────────────────────────────

const WORKFLOW_STEPS = [
  { icon: PenLine, title: 'Create stories', description: 'Describe work to be done — bugs, features, or tasks' },
  { icon: Users, title: 'Assign to team', description: 'Set an owner, priority, and deadline for each story' },
  { icon: BarChart3, title: 'Track progress', description: 'Stories move through workflow states as work gets done' },
];

function MyWorkEmptyState({ mode, workspaceSlug }: { mode: Mode; workspaceSlug: string }) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data: access } = useWorkspaceAccess(workspace?.id ?? '');
  const { canEdit } = usePermissions(access);
  const openCreate = useGlobalCreateStore((s) => s.openCreate);

  const exploreLinks = [
    { icon: SquareKanban, label: 'Stories', path: `/w/${workspaceSlug}/pm/stories` },
    { icon: Timer, label: 'Sprints', path: `/w/${workspaceSlug}/pm/sprints` },
    { icon: Layers, label: 'Epics', path: `/w/${workspaceSlug}/pm/epics` },
    { icon: Target, label: 'Objectives', path: `/w/${workspaceSlug}/pm/objectives` },
  ];

  return (
    <div className="flex flex-col items-center py-16 px-4">
      {/* Hero */}
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-blue-500/10 mb-5">
        <ClipboardCheck className="h-7 w-7 text-blue-500" />
      </div>
      <h3 className="text-base font-medium mb-1">
        {mode === 'assigned' ? 'No stories assigned to you yet' : 'No stories requested by you yet'}
      </h3>
      <p className="text-sm text-muted-foreground text-center max-w-md">
        {mode === 'assigned'
          ? 'When teammates assign stories to you, they appear here — prioritized so you always know what to focus on first.'
          : 'Stories you create or request will appear here so you can track their progress.'}
      </p>

      {/* Quick actions */}
      <div className="flex items-center gap-3 mt-6">
        {canEdit && (
          <Button size="sm" onClick={() => openCreate('story')}>
            <Plus className="h-4 w-4 mr-1.5" />
            Create a Story
          </Button>
        )}
        <Button
          variant="outline"
          size="sm"
          onClick={() => navigate({ to: '/w/$slug/pm/stories', params: { slug: workspaceSlug } })}
        >
          <SquareKanban className="h-4 w-4 mr-1.5" />
          View Stories
        </Button>
      </div>
      {!canEdit && (
        <p className="text-xs text-muted-foreground mt-2">
          Ask a teammate to assign stories to you to get started.
        </p>
      )}

      <div className="w-full max-w-4xl mt-10">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          {WORKFLOW_STEPS.map(({ icon: Icon, title, description }) => (
            <div key={title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
              <Icon className="h-5 w-5 text-muted-foreground mb-3" />
              <p className="text-sm font-medium mb-1">{title}</p>
              <p className="text-[13px] text-muted-foreground leading-relaxed">{description}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Explore */}
      <div className="w-full max-w-lg mt-8">
        <h4 className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-3 text-center">
          Explore
        </h4>
        <div className="flex items-center justify-center gap-2">
          {exploreLinks.map(({ icon: Icon, label, path }) => (
            <button
              key={label}
              type="button"
              onClick={() => navigate({ to: path })}
              className="flex items-center gap-1.5 rounded-md border border-border/50 bg-muted/30 px-3 py-2 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
            >
              <Icon className="h-3.5 w-3.5" />
              {label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

// ── Summary card ──────────────────────────────────────────────────

function SummaryCard({ icon: Icon, iconColor, label, value }: {
  icon: React.ElementType;
  iconColor: string;
  label: string;
  value: number;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg border border-border/30 bg-muted/20 px-4 py-3">
      <Icon className={`h-4 w-4 shrink-0 ${iconColor}`} />
      <div className="min-w-0">
        <p className="text-lg font-semibold leading-none tabular-nums">{value}</p>
        <p className="text-[11px] text-muted-foreground/70 mt-0.5">{label}</p>
      </div>
    </div>
  );
}

// ── Story section ─────────────────────────────────────────────────

const COLLAPSE_THRESHOLD = 5;

function StorySection({ title, count, stories, onClickStory, findTeamName, showTeam }: {
  title: string;
  count: number;
  stories: Story[];
  onClickStory: (s: Story) => void;
  findTeamName: (id?: string) => string | undefined;
  showTeam: boolean;
}) {
  const collapsible = stories.length > COLLAPSE_THRESHOLD;
  const [expanded, setExpanded] = useState(!collapsible);
  const visible = expanded ? stories : stories.slice(0, COLLAPSE_THRESHOLD);
  const hiddenCount = stories.length - COLLAPSE_THRESHOLD;

  return (
    <div>
      <div className="flex items-center gap-2 mb-1 px-1">
        <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {title}
        </h3>
        <span className="text-xs text-muted-foreground/50 tabular-nums">{count}</span>
      </div>
      <div className="divide-y divide-border/40">
        {visible.map((story) => (
          <StoryRow
            key={story.id}
            story={story}
            onClick={() => onClickStory(story)}
            teamName={showTeam ? findTeamName(story.team_id) : undefined}
          />
        ))}
      </div>
      {collapsible && (
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="mt-1 px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {expanded ? 'Show less' : `Show ${hiddenCount} more`}
        </button>
      )}
    </div>
  );
}

// ── Story row ─────────────────────────────────────────────────────

function StoryRow({ story, onClick, teamName }: {
  story: Story;
  onClick: () => void;
  teamName?: string;
}) {
  const deadlineInfo = useMemo(() => {
    if (!story.deadline) return null;
    const d = parseISO(story.deadline);
    const days = differenceInDays(d, new Date());
    const status: 'overdue' | 'approaching' | 'normal' =
      days < 0 ? 'overdue' : days <= 3 ? 'approaching' : 'normal';
    return { label: format(d, 'MMM d'), status };
  }, [story.deadline]);

  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-2.5 px-2 py-2.5 w-full text-left rounded-md hover:bg-muted/40 transition-colors group"
    >
      <span className="text-xs text-muted-foreground/50 font-mono shrink-0 w-8 text-right tabular-nums">
        {story.display_id}
      </span>

      <span className={`text-[13px] truncate flex-1 min-w-0 ${story.completed ? 'line-through text-muted-foreground/60' : 'text-foreground'}`}>
        {story.name}
      </span>

      {/* Metadata pills — matches StoryCard style */}
      <div className="flex items-center gap-1.5 shrink-0">
        {story.priority !== 'none' && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={`flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1 ${PRIORITY_BORDER_COLOR[story.priority]}`}>
                <PriorityIcon priority={story.priority} className="h-3.5 w-3.5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Priority: {PRIORITY_CONFIG[story.priority].label}</TooltipContent>
          </Tooltip>
        )}

        {story.state_name && story.state_type && (
          <span className="flex h-5 items-center gap-1 rounded-sm border-[0.5px] border-border bg-muted/50 px-2 text-[11px] font-medium text-muted-foreground shrink-0 hidden md:flex">
            <StateTypeIcon stateType={story.state_type as StateType} className="h-3 w-3" />
            {story.state_name}
          </span>
        )}

        {story.blocked && (
          <span className="flex h-5 items-center gap-1 rounded-sm border-[0.5px] border-red-300 bg-red-50 px-2 text-[11px] font-medium text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400 shrink-0">
            Blocked
          </span>
        )}

        {deadlineInfo && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={`flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium shrink-0 ${DEADLINE_PILL_STYLE[deadlineInfo.status]}`}>
                <CalendarDays className="h-3 w-3" />
                <span className="hidden sm:inline">{deadlineInfo.label}</span>
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">
              {DEADLINE_TOOLTIP[deadlineInfo.status]}: {deadlineInfo.label}
            </TooltipContent>
          </Tooltip>
        )}

        {teamName && (
          <span className="flex h-5 items-center rounded-sm border-[0.5px] border-border bg-muted/50 px-2 text-[11px] font-medium text-muted-foreground shrink-0 hidden lg:flex truncate max-w-[100px]">
            {teamName}
          </span>
        )}
      </div>
    </button>
  );
}
