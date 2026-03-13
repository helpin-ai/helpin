import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { differenceInDays, parseISO, format } from 'date-fns';
import {
  AlertCircle,
  CalendarDays,
  CircleDot,
  Clock,
  ClipboardCheck,
  Timer,
} from 'lucide-react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { StateTypeIcon, StoryTypeIcon, PriorityIcon } from '@/lib/pmConstants';
import type { Story, StateType } from '@/lib/pmTypes';

type Mode = 'assigned' | 'requested';

export function MyWorkPage() {
  useTitle('My Work');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: access } = useWorkspaceAccess(workspaceId);
  const memberId = access?.membership?.id;

  const { findTeamName } = useWorkspaceTeams(workspaceId);

  const [mode, setMode] = useState<Mode>('assigned');
  const [stories, setStories] = useState<Story[]>([]);
  const [loading, setLoading] = useState(false);

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
  }, [workspaceId, memberId, mode]);

  // ── Derived data ──────────────────────────────────────────────────

  const now = new Date();

  const counts = useMemo(() => {
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
  }, [stories, now]);

  const { focus, blockedStories, rest } = useMemo(() => {
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
  }, [stories, now]);

  // ── Render ────────────────────────────────────────────────────────

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const openStory = (s: Story) => {
    navigate({ to: '/w/$slug/pm/stories/$storyId', params: { slug, storyId: s.id } });
  };

  return (
    <div className="space-y-4 max-w-4xl mx-auto">
      <header className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">My Work</h2>
          <p className="text-sm text-muted-foreground">
            {mode === 'assigned'
              ? 'Stories assigned to you across all teams.'
              : 'Stories you requested across all teams.'}
          </p>
        </div>

        {/* Mode toggle */}
        <div className="flex gap-1 rounded-lg bg-muted p-1">
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
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <SummaryCard icon={CircleDot} iconColor="text-amber-500" label="In progress" value={counts.inProgress} />
          <SummaryCard icon={Clock} iconColor="text-blue-500" label="Due soon" value={counts.dueSoon} />
          <SummaryCard icon={Timer} iconColor="text-red-500" label="Overdue" value={counts.overdue} />
          <SummaryCard icon={AlertCircle} iconColor="text-orange-500" label="Blocked" value={counts.blocked} />
        </div>
      )}

      {/* Content */}
      {loading ? null : stories.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 px-4">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-blue-500/10 mb-5">
            <ClipboardCheck className="h-7 w-7 text-blue-500" />
          </div>
          <h3 className="text-lg font-semibold mb-1.5">
            {mode === 'assigned' ? 'No stories assigned to you' : 'No stories requested by you'}
          </h3>
          <p className="text-sm text-muted-foreground text-center max-w-md">
            {mode === 'assigned'
              ? 'Stories assigned to you will appear here so you can track your work across all teams.'
              : "Stories you've created or requested will appear here."}
          </p>
        </div>
      ) : (
        <div className="space-y-8">
          {focus.length > 0 && (
            <StorySection title="Focus now" stories={focus} onClickStory={openStory} findTeamName={findTeamName} />
          )}
          {blockedStories.length > 0 && (
            <StorySection title="Blocked" stories={blockedStories} onClickStory={openStory} findTeamName={findTeamName} />
          )}
          {rest.length > 0 && (
            <StorySection title="Everything else" stories={rest} onClickStory={openStory} findTeamName={findTeamName} />
          )}
        </div>
      )}
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
    <div className="flex items-center gap-3 rounded-lg border border-border/50 bg-muted/30 px-4 py-3">
      <Icon className={`h-4 w-4 shrink-0 ${iconColor}`} />
      <div className="min-w-0">
        <p className="text-lg font-semibold leading-none">{value}</p>
        <p className="text-[11px] text-muted-foreground mt-0.5">{label}</p>
      </div>
    </div>
  );
}

// ── Story section ─────────────────────────────────────────────────

function StorySection({ title, stories, onClickStory, findTeamName }: {
  title: string;
  stories: Story[];
  onClickStory: (s: Story) => void;
  findTeamName: (id?: string) => string | undefined;
}) {
  return (
    <div>
      <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">
        {title}
        <span className="ml-1.5 text-muted-foreground/60">{stories.length}</span>
      </h3>
      <div className="rounded-lg border border-border overflow-hidden divide-y divide-border">
        {stories.map((story) => (
          <StoryRow
            key={story.id}
            story={story}
            onClick={() => onClickStory(story)}
            teamName={findTeamName(story.team_id)}
          />
        ))}
      </div>
    </div>
  );
}

// ── Compact story row ─────────────────────────────────────────────

function StoryRow({ story, onClick, teamName }: {
  story: Story;
  onClick: () => void;
  teamName?: string;
}) {
  const deadlineInfo = useMemo(() => {
    if (!story.deadline) return null;
    const d = parseISO(story.deadline);
    const days = differenceInDays(d, new Date());
    let color = 'text-muted-foreground';
    if (days < 0) color = 'text-red-500';
    else if (days <= 3) color = 'text-amber-500';
    return { label: format(d, 'MMM d'), color };
  }, [story.deadline]);

  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-2.5 px-3 py-2 w-full text-left hover:bg-muted/50 transition-colors"
    >
      <StoryTypeIcon storyType={story.story_type} className="h-3.5 w-3.5 shrink-0" />

      <span className="text-xs text-muted-foreground font-mono shrink-0 w-8 text-right">
        {story.display_id}
      </span>

      <span className={`text-sm truncate flex-1 min-w-0 ${story.completed ? 'line-through text-muted-foreground' : ''}`}>
        {story.name}
      </span>

      {story.state_name && story.state_type && (
        <span className="flex items-center gap-1 shrink-0">
          <StateTypeIcon stateType={story.state_type as StateType} className="h-3 w-3" />
          <span className="text-[11px] text-muted-foreground hidden md:inline">{story.state_name}</span>
        </span>
      )}

      {story.priority !== 'none' && (
        <PriorityIcon priority={story.priority} className="h-3.5 w-3.5 shrink-0" />
      )}

      {story.blocked && (
        <span className="text-[10px] font-medium text-orange-500 bg-orange-500/10 rounded px-1.5 py-0.5 shrink-0">
          Blocked
        </span>
      )}

      {deadlineInfo && (
        <span className={`flex items-center gap-1 text-[11px] shrink-0 ${deadlineInfo.color}`}>
          <CalendarDays className="h-3 w-3" />
          <span className="hidden sm:inline">{deadlineInfo.label}</span>
        </span>
      )}

      {teamName && (
        <span className="text-[11px] text-muted-foreground shrink-0 hidden lg:inline truncate max-w-[100px]">
          {teamName}
        </span>
      )}
    </button>
  );
}
