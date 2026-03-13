import { useMemo } from 'react';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useAIAutomations } from '@/hooks/queries/useSettings';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { ChevronRight, Gauge, ShieldCheck, Bot } from 'lucide-react';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import type { AutomationHealthStatus, AutomationInventoryItem } from '@/lib/types';

// ── Helpers ──

const HEALTH_DOT: Record<AutomationHealthStatus, string> = {
  healthy: 'bg-emerald-500',
  warning: 'bg-amber-500',
  error: 'bg-rose-500',
  inactive: 'bg-slate-300',
  unknown: 'bg-slate-300',
};

function relativeTime(isoString?: string): string {
  if (!isoString) return '--';
  const diff = Date.now() - new Date(isoString).getTime();
  if (Number.isNaN(diff) || diff < 0) return '--';
  const mins = Math.floor(diff / 60_000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function displayPath(path: string | undefined, slug: string | undefined) {
  if (!path) return undefined;
  if (!slug) return path.replace('/w/$slug', '/w/<workspace>');
  return path.replace('$slug', slug);
}

/** Determine which context info to show on the right side of a row */
function getContextInfo(item: AutomationInventoryItem): string | null {
  // CRM built-ins: show freshness + last success time
  if (item.module === 'crm' && item.kind === 'built_in_automation') {
    const fresh = item.health.freshness !== 'unknown' ? item.health.freshness : null;
    const lastSuccess = relativeTime(item.health.last_success_at);
    if (fresh && lastSuccess !== '--') return `${fresh} · ${lastSuccess}`;
    if (fresh) return fresh;
    if (lastSuccess !== '--') return lastSuccess;
    return null;
  }
  // Agents: show last run status + time
  if (item.kind === 'contextual_agent') {
    const status = item.health.status !== 'unknown' ? item.health.status : null;
    const lastSeen = relativeTime(item.health.last_seen_at);
    if (status && lastSeen !== '--') return `${status} · ${lastSeen}`;
    if (status) return status;
    if (lastSeen !== '--') return lastSeen;
    return null;
  }
  // PM rules: no extra context
  return null;
}

// ── Subgroup config ──

interface SubgroupDef {
  label: string;
  filter: (item: AutomationInventoryItem) => boolean;
}

const SUBGROUPS: SubgroupDef[] = [
  { label: 'CRM system intelligence', filter: (i) => i.module === 'crm' && i.kind === 'built_in_automation' },
  { label: 'PM built-in rules', filter: (i) => i.module === 'pm' && i.kind === 'built_in_automation' },
  { label: 'PM & Support agents', filter: (i) => i.kind === 'contextual_agent' },
];

// ── Row component ──

function AutomationRow({ item, slug }: { item: AutomationInventoryItem; slug?: string }) {
  const managePath = displayPath(item.current_write_path, slug);
  const contextInfo = getContextInfo(item);

  return (
    <div className={item.enabled ? '' : 'opacity-60'}>
      <div className="flex items-center gap-3 px-4 py-2.5">
        {/* Left: status dot + title + scope */}
        <span className={`h-2 w-2 shrink-0 rounded-full ${HEALTH_DOT[item.health.status]}`} />
        <span className="min-w-0 truncate text-sm font-medium">{item.title}</span>
        <Badge variant="outline" className="shrink-0 text-[10px]">
          {item.scope_label}
        </Badge>

        {/* Spacer */}
        <span className="flex-1" />

        {/* Right: context info */}
        {contextInfo && (
          <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">
            {contextInfo}
          </span>
        )}

        {/* Enabled badge */}
        <Badge
          variant="outline"
          className={`shrink-0 text-[10px] ${
            item.enabled
              ? 'border-emerald-500/30 bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400'
              : ''
          }`}
        >
          {item.enabled ? 'On' : 'Off'}
        </Badge>

        {/* Manage link */}
        {managePath ? (
          <a href={managePath} className="shrink-0 text-muted-foreground hover:text-foreground">
            <ChevronRight className="h-4 w-4" />
          </a>
        ) : (
          <span className="w-4 shrink-0" />
        )}
      </div>

      {/* Error banner */}
      {item.health.last_error_message && (
        <div className="mx-4 mb-2 rounded-sm border border-rose-200 bg-rose-50 px-3 py-1.5 text-xs text-rose-700 dark:border-rose-800 dark:bg-rose-950/30 dark:text-rose-400">
          {item.health.last_error_message}
        </div>
      )}
    </div>
  );
}

// ── Status bar ──

function StatusBar({ items }: { items: AutomationInventoryItem[] }) {
  const errorCount = items.filter((i) => i.health.status === 'error').length;
  const warningCount = items.filter((i) => i.health.status === 'warning').length;

  if (errorCount === 0 && warningCount === 0) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span className="h-2 w-2 rounded-full bg-emerald-500" />
        All {items.length} automations operational
      </div>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-3 text-sm">
      {errorCount > 0 && (
        <span className="flex items-center gap-1.5 text-rose-700 dark:text-rose-400">
          <span className="h-2 w-2 rounded-full bg-rose-500" />
          {errorCount} error{errorCount !== 1 ? 's' : ''}
        </span>
      )}
      {warningCount > 0 && (
        <span className="flex items-center gap-1.5 text-amber-700 dark:text-amber-400">
          <span className="h-2 w-2 rounded-full bg-amber-500" />
          {warningCount} warning{warningCount !== 1 ? 's' : ''}
        </span>
      )}
      <span className="text-muted-foreground">
        · {items.length - errorCount - warningCount} healthy
      </span>
    </div>
  );
}

// ── Main tab ──

export function AIAutomationsTab({ workspaceId }: { workspaceId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const { data, isLoading, isError, error } = useAIAutomations(workspaceId);

  const slug = currentWorkspace?.slug;
  const items = data?.items ?? [];

  const subgroups = useMemo(() => {
    return SUBGROUPS.map((sg) => ({
      ...sg,
      items: items.filter(sg.filter),
    })).filter((sg) => sg.items.length > 0);
  }, [items]);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-full rounded-none" />
        <Skeleton className="h-48 w-full rounded-none" />
      </div>
    );
  }

  if (isError) {
    return (
      <Card className={LINEAR_CARD_CLASS}>
        <CardContent className="flex flex-col gap-2 py-8">
          <div className="flex items-center gap-2 text-sm font-medium text-destructive">
            <Gauge className="h-4 w-4" />
            Could not load AI & Automations inventory
          </div>
          <p className="text-sm text-muted-foreground">{error instanceof Error ? error.message : 'Unknown error'}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {/* Status bar */}
      {items.length > 0 && (
        <div className="px-1">
          <StatusBar items={items} />
        </div>
      )}

      {/* Main inventory card */}
      {subgroups.length > 0 && (
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-2">
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Built-in Automations</CardTitle>
              <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {items.filter((i) => i.kind === 'built_in_automation').length}
              </span>
            </div>
          </CardHeader>
          <CardContent className="space-y-4 px-0 pb-2">
            {subgroups
              .filter((sg) => sg.items.some((i) => i.kind === 'built_in_automation'))
              .map((sg) => (
                <div key={sg.label}>
                  <div className="px-6 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                    {sg.label}
                  </div>
                  <div className="divide-y divide-border/50">
                    {sg.items.map((item) => (
                      <AutomationRow key={item.inventory_id} item={item} slug={slug} />
                    ))}
                  </div>
                </div>
              ))}
          </CardContent>
        </Card>
      )}

      {/* Agents card */}
      {subgroups.some((sg) => sg.items.some((i) => i.kind === 'contextual_agent')) && (
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-2">
            <div className="flex items-center gap-2">
              <Bot className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Contextual Agents</CardTitle>
              <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {items.filter((i) => i.kind === 'contextual_agent').length}
              </span>
            </div>
          </CardHeader>
          <CardContent className="px-0 pb-2">
            {subgroups
              .filter((sg) => sg.items.some((i) => i.kind === 'contextual_agent'))
              .map((sg) => (
                <div key={sg.label}>
                  <div className="px-6 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                    {sg.label}
                  </div>
                  <div className="divide-y divide-border/50">
                    {sg.items
                      .filter((i) => i.kind === 'contextual_agent')
                      .map((item) => (
                        <AutomationRow key={item.inventory_id} item={item} slug={slug} />
                      ))}
                  </div>
                </div>
              ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
