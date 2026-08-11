import { useEffect, useRef, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { AiMagicIcon, ArrowRight01Icon, Cancel01Icon, Loading01Icon } from '@/lib/icons';
import { ArrowReloadHorizontalIcon } from '@/lib/pmIcons';
import { Button } from '@/components/ui/button';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import {
  useDismissTaskStandingBriefSuggestion,
  useRefreshTaskStandingBrief,
  useTaskStandingBrief,
} from '@/hooks/queries';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import type { TaskStandingBriefSuggestion } from '@/lib/pmTypes';

interface TaskStandingBriefCardProps {
  workspaceId: string;
  taskId: string;
  canEdit: boolean;
  onSuggestion: (suggestion: TaskStandingBriefSuggestion) => void;
}

export function TaskStandingBriefCard({ workspaceId, taskId, canEdit, onSuggestion }: TaskStandingBriefCardProps) {
  const briefQuery = useTaskStandingBrief(workspaceId, taskId);
  const refreshBrief = useRefreshTaskStandingBrief(workspaceId, taskId);
  const dismissSuggestion = useDismissTaskStandingBriefSuggestion(workspaceId, taskId);
  const autoRefreshRequested = useRef(false);
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null);
  const brief = briefQuery.data;

  const refresh = async (manual = false) => {
    try {
      await refreshBrief.mutateAsync();
    } catch (error) {
      if (manual) {
        const reason = getUpgradeRequiredReason(error instanceof Error ? error.message : String(error));
        if (reason) setUpgradeReason(reason);
      }
    }
  };

  useEffect(() => {
    if (!canEdit || !brief?.is_stale || autoRefreshRequested.current || refreshBrief.isPending) return;
    autoRefreshRequested.current = true;
    void refresh();
  }, [brief?.is_stale, canEdit, refreshBrief.isPending]);

  if (briefQuery.isLoading || !brief || (!brief.narrative && brief.suggestions.length === 0)) return null;

  const freshness = brief.computed_at
    ? `Updated ${formatDistanceToNow(new Date(brief.computed_at), { addSuffix: true })}`
    : brief.status === 'pending_refresh' ? 'Preparing brief' : 'Needs refresh';

  return (
    <>
      <section className="rounded-xl border border-amber-200/60 bg-amber-50/40 px-4 py-4 dark:border-amber-900/40 dark:bg-amber-950/10" aria-label="AI-generated task standing brief">
        <div className="flex items-center gap-2">
          <AiMagicIcon className="h-3.5 w-3.5 text-amber-700 dark:text-amber-300" />
          <h2 className="text-[11px] font-semibold uppercase tracking-[0.12em] text-amber-800/80 dark:text-amber-200/80">
            Where this stands
          </h2>
          <span className="ml-auto text-[11px] text-muted-foreground">{freshness}</span>
          {canEdit && (
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label="Refresh task standing brief"
              disabled={refreshBrief.isPending}
              onClick={() => void refresh(true)}
            >
              {refreshBrief.isPending ? <Loading01Icon className="animate-spin" /> : <ArrowReloadHorizontalIcon />}
            </Button>
          )}
        </div>

        <p className="mt-3 max-w-[64ch] text-sm leading-6 text-foreground/80">{brief.narrative}</p>

        {brief.suggestions.length > 0 && (
          <div className="mt-4">
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Suggested next steps</p>
            <div className="divide-y divide-border/50">
              {brief.suggestions.map((suggestion) => (
                <div key={suggestion.key} className="group flex items-center gap-3 py-2">
                  <span className="h-4 w-4 shrink-0 rounded border border-dashed border-amber-400/60" aria-hidden />
                  <span className="min-w-0 flex-1 text-sm text-foreground/85">{suggestion.label}</span>
                  <Button type="button" variant="outline" size="xs" onClick={() => onSuggestion(suggestion)}>
                    {suggestion.action.type === 'reply_to_comment' ? 'Reply' : suggestion.action.type === 'add_checklist_item' ? 'Add' : suggestion.action.type === 'retry_run' ? 'Retry' : 'Open'}
                  </Button>
                  {canEdit && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      className="opacity-0 group-hover:opacity-100 group-focus-within:opacity-100"
                      aria-label={`Dismiss ${suggestion.label}`}
                      onClick={() => dismissSuggestion.mutate(suggestion.key)}
                    >
                      <Cancel01Icon />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {brief.evidence.length > 0 && (
          <div className="mt-3 border-t border-amber-200/50 pt-3 dark:border-amber-900/30">
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Evidence</p>
            {brief.evidence.slice(0, 3).map((item) => (
              <div key={`${item.type}-${item.id}`} className="flex items-center gap-2 py-1 text-xs text-muted-foreground">
                <span className="w-14 shrink-0 uppercase tracking-wide text-muted-foreground/70">{item.type}</span>
                <span className="min-w-0 flex-1 truncate">{item.label}</span>
                <ArrowRight01Icon className="h-3 w-3" />
              </div>
            ))}
          </div>
        )}
      </section>
      <UpgradeRequiredDialog open={upgradeReason !== null} onOpenChange={(open) => { if (!open) setUpgradeReason(null); }} reason={upgradeReason} />
    </>
  );
}
