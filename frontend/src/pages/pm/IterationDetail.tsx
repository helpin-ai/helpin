import { useEffect, useMemo, useState } from 'react';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import {
  ArrowLeft,
  CalendarDays,
  ChevronRight,
  Loader2,
  RefreshCw,
  Users,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { pmIterationService } from '@/lib/services/pmIterationService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { IterationWithStats, IterationStatus, Story, UpdateIterationRequest } from '@/lib/pmTypes';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/iterations/$iterationId');

const statusOptions: IterationStatus[] = ['unstarted', 'started', 'done'];
const statusConfig: Record<IterationStatus, { label: string; color: string }> = {
  unstarted: { label: 'Unstarted', color: 'text-muted-foreground' },
  started: { label: 'Started', color: 'text-blue-600' },
  done: { label: 'Done', color: 'text-green-600' },
};

// ── Sidebar Popover Select ─────────────────────────────────────────

function SidebarPopoverSelect<T extends string>({
  value,
  options,
  onChange,
  renderTrigger,
}: {
  value: T;
  options: { value: T; label: string; className?: string }[];
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        >
          {renderTrigger()}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0.5" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs transition-colors cursor-pointer
                ${value === option.value ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}
              `}
              onClick={() => { onChange(option.value); setOpen(false); }}
            >
              <span className={`truncate ${option.className ?? ''}`}>{option.label}</span>
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ── Metadata Row ───────────────────────────────────────────────────

function MetadataRow({
  icon: Icon,
  label,
  children,
}: {
  icon: React.ElementType;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
      <span className="text-xs text-muted-foreground self-center">{label}</span>
      <div className="min-w-0 self-center">{children}</div>
    </>
  );
}

// ── Main Page ──────────────────────────────────────────────────────

interface IterationFormState {
  name: string;
  description: string;
  team_id: string;
  start_date: string;
  end_date: string;
}

const buildForm = (iter: IterationWithStats): IterationFormState => ({
  name: iter.iteration.name,
  description: iter.iteration.description ?? '',
  team_id: iter.iteration.team_id ?? '',
  start_date: iter.iteration.start_date ? iter.iteration.start_date.slice(0, 10) : '',
  end_date: iter.iteration.end_date ? iter.iteration.end_date.slice(0, 10) : '',
});

export function IterationDetailPage() {
  const { iterationId, slug } = routeApi.useParams();
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id;

  const [iteration, setIteration] = useState<IterationWithStats | null>(null);
  const [stories, setStories] = useState<Story[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState<IterationFormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateIterationRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const { teams, people, findTeamName, getTeamMembers } = useWorkspaceTeams(workspaceId);

  // Load iteration data
  useEffect(() => {
    if (!workspaceId) return;
    (async () => {
      setLoading(true);
      setError(null);
      const [iterRes, storiesRes] = await Promise.all([
        pmIterationService.get(workspaceId, iterationId),
        pmIterationService.listStories(workspaceId, iterationId),
      ]);
      if (iterRes.error || !iterRes.data) {
        setError(iterRes.error ?? 'Iteration not found');
        setLoading(false);
        return;
      }
      setIteration(iterRes.data);
      setForm(buildForm(iterRes.data));
      setStories(storiesRes.data ?? []);
      setLoading(false);
    })();
  }, [workspaceId, iterationId]);

  // Auto-save debounce
  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0 || !workspaceId || !iteration) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      const { data, error: err } = await pmIterationService.update(workspaceId, iteration.iteration.id, patch);
      if (err || !data) {
        setSaveError(err ?? 'Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        setIteration(data);
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [workspaceId, iteration, pendingPatch, saving]);

  const queuePatch = (patch: UpdateIterationRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof IterationFormState>(key: K, value: IterationFormState[K], patch: UpdateIterationRequest) => {
    setForm((current) => current ? { ...current, [key]: value } : current);
    queuePatch(patch);
  };

  // Derived data
  const progress = useMemo(() => {
    if (!iteration || iteration.stats.story_count === 0) return 0;
    return Math.round((iteration.stats.done_story_count / iteration.stats.story_count) * 100);
  }, [iteration]);

  const currentTeamName = useMemo(
    () => (form?.team_id ? findTeamName(form.team_id) ?? 'No team' : 'No team'),
    [form?.team_id, findTeamName],
  );

  // Resources: unique people from story owners + iteration team members
  const resources = useMemo(() => {
    const personMap = new Map<string, { id: string; name: string; email: string }>();

    for (const story of stories) {
      if (story.owner_id) {
        const person = people.find((p) => p.id === story.owner_id);
        if (person) personMap.set(person.id, { id: person.id, name: person.name, email: person.email });
      }
    }

    if (form?.team_id) {
      for (const member of getTeamMembers(form.team_id)) {
        if (!personMap.has(member.id)) {
          personMap.set(member.id, { id: member.id, name: member.name, email: member.email });
        }
      }
    }

    return Array.from(personMap.values());
  }, [stories, people, form?.team_id, getTeamMembers]);

  const goBack = () => navigate({ to: '/w/$slug/pm/iterations', params: { slug } });

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !iteration || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">{error ?? 'Iteration not found'}</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          Back to Iterations
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* ── Header bar ──────────────────────────────────────────── */}
      <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={goBack}>
          <ArrowLeft className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          <RefreshCw className="h-3.5 w-3.5 shrink-0 text-blue-500" />
          <button type="button" className="shrink-0 hover:text-foreground transition-colors cursor-pointer" onClick={goBack}>
            Iterations
          </button>
          <ChevronRight className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1 text-xs text-muted-foreground">
          {saving ? (
            <span className="inline-flex items-center gap-1">
              <Loader2 className="h-3 w-3 animate-spin" />
              Saving...
            </span>
          ) : (
            <span>All changes saved</span>
          )}
          {saveError && <span className="ml-2 text-destructive">{saveError}</span>}
        </div>
      </div>

      {/* ── Two-column layout ───────────────────────────────────── */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]">
        {/* ── Left column ────────────────────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 py-6">
          {/* Title */}
          <input
            type="text"
            aria-label="Iteration title"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
            placeholder="Untitled"
          />

          {/* Description */}
          <div className="mt-4">
            <TiptapEditor
              content={form.description}
              onChange={(html) => updateField('description', html, { description: html })}
              placeholder="Add a description..."
              className="border-transparent shadow-none"
            />
          </div>

          <Separator className="my-6" />

          {/* Progress */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Progress</h3>
              <span className="text-xs text-muted-foreground">{progress}%</span>
            </div>
            <Progress value={progress} />
            <p className="text-xs text-muted-foreground">
              {iteration.stats.done_story_count}/{iteration.stats.story_count} stories done · {iteration.stats.done_points}/{iteration.stats.total_points} points
            </p>
          </div>

          <Separator className="my-6" />

          {/* Resources */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Resources</h3>
            {resources.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">No people assigned yet.</p>
            ) : (
              <div className="mt-3 flex flex-wrap gap-2">
                {resources.map((person) => (
                  <div key={person.id} className="flex items-center gap-2 rounded-md border border-border/60 px-3 py-1.5">
                    <Avatar className="h-6 w-6 border border-border/60">
                      <AvatarFallback className="text-[9px] font-semibold bg-muted/60">
                        {person.name
                          ? person.name.split(' ').map((w) => w[0]).join('').toUpperCase().slice(0, 2)
                          : person.email.slice(0, 2).toUpperCase()}
                      </AvatarFallback>
                    </Avatar>
                    <span className="text-xs font-medium">{person.name || person.email}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          <Separator className="my-6" />

          {/* Stories */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Stories</h3>
            {stories.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">No stories linked yet.</p>
            ) : (
              <div className="mt-3 space-y-1">
                {stories.map((story) => (
                  <article key={story.id} className="rounded-md border border-border/70 px-3 py-2 text-sm">
                    <p className="font-medium">TP-{story.display_id} · {story.name}</p>
                    <p className="text-xs text-muted-foreground">{story.priority} priority · {story.story_type}</p>
                  </article>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* ── Right column — metadata sidebar ────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>

          <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
            {/* Status */}
            <MetadataRow icon={RefreshCw} label="Status">
              <SidebarPopoverSelect
                value={iteration.iteration.status}
                options={statusOptions.map((s) => ({ value: s, label: statusConfig[s].label, className: statusConfig[s].color }))}
                onChange={() => {/* status is computed server-side */}}
                renderTrigger={() => (
                  <span className={statusConfig[iteration.iteration.status]?.color}>
                    {statusConfig[iteration.iteration.status]?.label}
                  </span>
                )}
              />
            </MetadataRow>

            {/* Team */}
            <MetadataRow icon={Users} label="Team">
              <SidebarPopoverSelect
                value={form.team_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No team' },
                  ...teams.map((t) => ({ value: t.id, label: t.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('team_id', val, { team_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentTeamName}</span>}
              />
            </MetadataRow>

            {/* Start Date */}
            <MetadataRow icon={CalendarDays} label="Start date">
              <DatePicker
                value={form.start_date}
                onChange={(v) => updateField('start_date', v, { start_date: v || undefined })}
                placeholder="None"
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>

            {/* End Date */}
            <MetadataRow icon={CalendarDays} label="End date">
              <DatePicker
                value={form.end_date}
                onChange={(v) => updateField('end_date', v, { end_date: v || undefined })}
                placeholder="None"
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>
          </div>
        </aside>
      </div>
    </div>
  );
}
