import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { formatDistanceToNow } from 'date-fns';
import { ArrowDown01Icon, BotIcon, Cancel01Icon, Loading01Icon, ArrowReloadHorizontalIcon, SentIcon, Tick01Icon, ViewIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { commandBarService } from '@/lib/services/commandBarService';
import { cn } from '@/lib/utils';
import { SettingsPageFrame } from './SettingsPageFrame';
import type {
  CommandBarPageContext,
  CommandBarUnmetIntent,
  CommandBarUnmetIntentStatus,
} from '@/lib/pmTypes';

const STATUS_TABS: Array<{ id: CommandBarUnmetIntentStatus | 'all'; label: string }> = [
  { id: 'open', label: 'Open' },
  { id: 'accepted', label: 'Accepted' },
  { id: 'deferred', label: 'Deferred' },
  { id: 'rejected', label: 'Rejected' },
  { id: 'all', label: 'All' },
];

const STATUS_BADGE: Record<CommandBarUnmetIntentStatus, { label: string; className: string }> = {
  open: { label: 'Open', className: 'border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-300' },
  accepted: { label: 'Accepted', className: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300' },
  rejected: { label: 'Rejected', className: 'border-zinc-500/30 bg-zinc-500/10 text-zinc-600 dark:text-zinc-300' },
  deferred: { label: 'Deferred', className: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300' },
};

function pageContextLabel(ctx: CommandBarPageContext | undefined): string {
  const type = (ctx?.entity_type ?? '').toString().trim();
  const title = (ctx?.display_title ?? ctx?.entity_id ?? '').toString().trim();
  if (!type) return title || 'workspace';
  if (!title || title === type) return type;
  return `${type} · ${title}`;
}

function relativeTime(iso: string): string {
  try {
    return formatDistanceToNow(new Date(iso), { addSuffix: true });
  } catch {
    return iso;
  }
}

interface IntentRowProps {
  intent: CommandBarUnmetIntent;
  workspaceId: string;
  showFullPrompts: boolean;
  onUpdated: (next: CommandBarUnmetIntent) => void;
}

function IntentRow({ intent, workspaceId, showFullPrompts, onUpdated }: IntentRowProps) {
  const [expanded, setExpanded] = useState(false);
  const [notes, setNotes] = useState(intent.review_notes ?? '');
  const [busy, setBusy] = useState<CommandBarUnmetIntentStatus | null>(null);

  const fullPrompt = intent.prompt && intent.prompt.length > 0 ? intent.prompt : null;
  const previewText = intent.prompt_preview || fullPrompt || '';
  const promptRedacted = intent.prompt_redacted && !fullPrompt;

  useEffect(() => {
    setNotes(intent.review_notes ?? '');
  }, [intent.review_notes]);

  const submit = async (status: CommandBarUnmetIntentStatus) => {
    setBusy(status);
    try {
      const res = await commandBarService.reviewUnmetIntent(workspaceId, intent.id, {
        status,
        notes: notes.trim() || undefined,
      });
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to update intent');
        return;
      }
      toast.success(`Marked as ${STATUS_BADGE[status].label.toLowerCase()}`);
      onUpdated(res.data);
      setExpanded(false);
    } finally {
      setBusy(null);
    }
  };

  const badge = STATUS_BADGE[intent.status] ?? STATUS_BADGE.open;
  const candidateNames = intent.candidate_agents?.map((c) => c.name).filter(Boolean) ?? [];

  return (
    <div className="rounded-md border border-border/70 bg-background transition hover:border-border">
      <button
        type="button"
        className="flex w-full items-start gap-3 px-4 py-3 text-left"
        onClick={() => setExpanded((prev) => !prev)}
      >
        <BotIcon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
        <div className="min-w-0 flex-1 space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" className={cn('text-[10px]', badge.className)}>
              {badge.label}
            </Badge>
            <Badge variant="outline" className="text-[10px] text-muted-foreground">
              {pageContextLabel(intent.page_context)}
            </Badge>
            <span className="text-[11px] text-muted-foreground">{relativeTime(intent.created_at)}</span>
            {intent.reviewed_at ? (
              <span className="text-[11px] text-muted-foreground">· reviewed {relativeTime(intent.reviewed_at)}</span>
            ) : null}
          </div>
          <p className="line-clamp-2 text-sm text-foreground">
            "{previewText}"
            {promptRedacted ? (
              <span className="ml-1.5 text-[11px] text-muted-foreground">· redacted</span>
            ) : null}
          </p>
          <p className="line-clamp-1 text-xs text-muted-foreground">{intent.reason}</p>
        </div>
        <ArrowDown01Icon className={cn('mt-0.5 h-4 w-4 shrink-0 text-muted-foreground transition-transform', expanded && 'rotate-180')} />
      </button>

      {expanded ? (
        <div className="space-y-4 border-t border-border/60 px-4 py-3">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <section className="space-y-1">
              <div className="flex items-center justify-between">
                <h4 className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  {fullPrompt ? 'Full prompt' : 'Preview'}
                </h4>
                {promptRedacted && !showFullPrompts ? (
                  <span className="text-[11px] text-muted-foreground">Toggle "Show full prompts" above to reveal.</span>
                ) : null}
              </div>
              <p className="rounded bg-muted/40 px-3 py-2 text-sm">"{fullPrompt ?? previewText}"</p>
            </section>
            <section className="space-y-1">
              <h4 className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Reason</h4>
              <p className="rounded bg-muted/40 px-3 py-2 text-sm">{intent.reason}</p>
            </section>
            <section className="space-y-1 md:col-span-2">
              <h4 className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Page context</h4>
              <pre className="overflow-x-auto rounded bg-muted/40 px-3 py-2 text-[11px] font-mono">
                {JSON.stringify(intent.page_context ?? {}, null, 2)}
              </pre>
            </section>
            {candidateNames.length > 0 ? (
              <section className="space-y-1 md:col-span-2">
                <h4 className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Candidates considered</h4>
                <div className="flex flex-wrap gap-1.5">
                  {candidateNames.map((name) => (
                    <Badge key={name} variant="secondary" className="text-[10px]">{name}</Badge>
                  ))}
                </div>
              </section>
            ) : null}
          </div>

          <div className="space-y-2">
            <h4 className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Review notes</h4>
            <Textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="Why this should be added, deferred, or rejected. Visible to admins reviewing later."
              rows={3}
            />
            <div className="flex flex-wrap items-center justify-end gap-2">
              <Button variant="outline" size="sm" disabled={busy !== null} onClick={() => void submit('deferred')}>
                {busy === 'deferred' ? <Loading01Icon className="mr-1.5 h-3 w-3 animate-spin" /> : null}
                Defer
              </Button>
              <Button variant="outline" size="sm" disabled={busy !== null} onClick={() => void submit('rejected')}>
                {busy === 'rejected' ? <Loading01Icon className="mr-1.5 h-3 w-3 animate-spin" /> : <Cancel01Icon className="mr-1.5 h-3 w-3" />}
                Reject
              </Button>
              <Button size="sm" disabled={busy !== null} onClick={() => void submit('accepted')}>
                {busy === 'accepted' ? <Loading01Icon className="mr-1.5 h-3 w-3 animate-spin" /> : <Tick01Icon className="mr-1.5 h-3 w-3" />}
                Accept
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function CommandIntentsView({ workspaceId }: { workspaceId: string }) {
  const [statusFilter, setStatusFilter] = useState<CommandBarUnmetIntentStatus | 'all'>('open');
  const [intents, setIntents] = useState<CommandBarUnmetIntent[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [showFullPrompts, setShowFullPrompts] = useState(false);

  const load = useCallback(async () => {
    setRefreshing(true);
    try {
      const res = await commandBarService.listUnmetIntents(workspaceId, {
        status: statusFilter,
        limit: 50,
        includeSensitive: showFullPrompts,
      });
      if (res.error) {
        toast.error(res.error);
        return;
      }
      setIntents(res.data?.intents ?? []);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [showFullPrompts, statusFilter, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const grouped = useMemo(() => intents, [intents]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap gap-1">
          {STATUS_TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setStatusFilter(tab.id)}
              className={cn(
                'rounded-full border px-3 py-1 text-xs transition',
                statusFilter === tab.id
                  ? 'border-primary bg-primary text-primary-foreground'
                  : 'border-border/70 text-muted-foreground hover:border-border hover:text-foreground',
              )}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant={showFullPrompts ? 'default' : 'ghost'}
            size="sm"
            onClick={() => setShowFullPrompts((prev) => !prev)}
            title="Reveal redacted prompts. Logged for audit."
          >
            <ViewIcon className="mr-1.5 h-3 w-3" />
            {showFullPrompts ? 'Showing full prompts' : 'Show full prompts'}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => void load()} disabled={refreshing}>
            {refreshing ? <Loading01Icon className="mr-1.5 h-3 w-3 animate-spin" /> : <ArrowReloadHorizontalIcon className="mr-1.5 h-3 w-3" />}
            Refresh
          </Button>
        </div>
      </div>

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-20" />
          ))}
        </div>
      ) : grouped.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-md border border-dashed border-border/70 px-6 py-12 text-center">
          <SentIcon className="h-6 w-6 text-muted-foreground" />
          <p className="text-sm font-medium">No {statusFilter === 'all' ? '' : statusFilter} intents</p>
          <p className="max-w-md text-xs text-muted-foreground">
            When the command bar can't match a prompt to an existing agent, the request lands here so you can decide whether to extend coverage.
          </p>
        </div>
      ) : (
        <div className="space-y-2">
          {grouped.map((intent) => (
            <IntentRow
              key={intent.id}
              intent={intent}
              workspaceId={workspaceId}
              showFullPrompts={showFullPrompts}
              onUpdated={(next) =>
                setIntents((prev) => {
                  if (statusFilter !== 'all' && next.status !== statusFilter) {
                    return prev.filter((item) => item.id !== next.id);
                  }
                  return prev.map((item) => (item.id === next.id ? next : item));
                })
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function CommandIntentsSettingsPage() {
  return (
    <SettingsPageFrame section="command-intents">
      {({ workspaceId }) => <CommandIntentsView workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
