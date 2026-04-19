import type { ElementType } from 'react';
import {
  AiMagicIcon,
  Alert01Icon,
  ArrowRight02Icon,
  ChartIncreaseIcon,
  Loading01Icon,
  SparklesIcon,
  UserGroupIcon,
  ZapIcon,
} from '@/lib/icons';
import { formatDistanceToNow } from 'date-fns';

import { Button } from '@/components/ui/button';
import { useContactSummary, useDealSummary } from '@/hooks/queries';
import type { CRMEntitySummary, SummaryHighlight } from '@/lib/crmTypes';
import { cn } from '@/lib/utils';

interface EntitySummaryCardProps {
  workspaceId: string;
  contactId?: string;
  dealId?: string;
}

const highlightIcons: Record<SummaryHighlight['kind'], ElementType> = {
  momentum: ChartIncreaseIcon,
  risk: Alert01Icon,
  next_step: ArrowRight02Icon,
  stakeholder: UserGroupIcon,
  signal: ZapIcon,
};

function SummaryHeading() {
  return (
    <div className="mb-3 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
      <AiMagicIcon className="h-3.5 w-3.5 text-foreground/70" />
      Summary
    </div>
  );
}

function DashedShell({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'rounded-xl border border-dashed border-border/70 bg-muted/30 px-5 py-4 sm:px-6 sm:py-5 dark:bg-muted/20',
        className,
      )}
    >
      {children}
    </div>
  );
}

function SummaryBody({ summary }: { summary: CRMEntitySummary }) {
  const hasSummary = summary.summary_markdown.trim().length > 0;

  return (
    <>
      {hasSummary ? (
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">
          {summary.summary_markdown}
        </p>
      ) : (
        <p className="text-sm text-muted-foreground">
          Summary generation has been queued. This record will populate after the background automation finishes.
        </p>
      )}

      {summary.highlights.length > 0 && (
        <div className="mt-3 grid gap-1.5">
          {summary.highlights.map((highlight, index) => {
            const Icon = highlightIcons[highlight.kind] ?? SparklesIcon;
            return (
              <div
                key={`${highlight.kind}-${index}`}
                className="flex items-start gap-2 text-xs leading-relaxed text-foreground/85"
              >
                <Icon className="mt-0.5 h-3 w-3 shrink-0 text-muted-foreground" />
                <span>{highlight.text}</span>
              </div>
            );
          })}
        </div>
      )}

      {summary.last_error && (
        <p className="mt-3 text-[11px] text-amber-700 dark:text-amber-400">
          {summary.status === 'error' ? 'Latest refresh failed.' : 'Latest refresh is delayed.'} {summary.last_error}
        </p>
      )}
    </>
  );
}

export function EntitySummaryCard({ workspaceId, contactId, dealId }: EntitySummaryCardProps) {
  const contactQuery = useContactSummary(workspaceId, contactId ?? '');
  const dealQuery = useDealSummary(workspaceId, dealId ?? '');
  const query = contactId ? contactQuery : dealQuery;
  const summary = query.data;

  if (query.isLoading) {
    return (
      <div>
        <SummaryHeading />
        <DashedShell>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
            Loading latest summary…
          </div>
        </DashedShell>
      </div>
    );
  }

  const hasBody = !!summary && summary.summary_markdown.trim().length > 0;

  if (!hasBody) {
    return (
      <div>
        <SummaryHeading />
        <DashedShell>
          <div className="flex flex-col items-start gap-4 sm:flex-row sm:items-center sm:justify-between sm:gap-6">
            <div className="min-w-0">
              <h3 className="text-[14px] font-semibold leading-snug text-foreground">
                Not enough signal yet
              </h3>
              <p className="mt-1 text-[12.5px] leading-relaxed text-muted-foreground">
                Once there are 2–3 activities — an email, a meeting, a note — the AI summary will kick in with context and suggested next steps.
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="shrink-0 gap-1.5"
              onClick={() => query.refetch()}
              disabled={query.isFetching}
            >
              <SparklesIcon className="h-3.5 w-3.5" />
              {query.isFetching ? 'Checking…' : 'Generate anyway'}
            </Button>
          </div>
        </DashedShell>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
          <AiMagicIcon className="h-3.5 w-3.5 text-foreground/70" />
          Summary
          {summary!.computed_at && (
            <span className="ml-1 font-normal normal-case tracking-normal text-muted-foreground/70">
              · {formatDistanceToNow(new Date(summary!.computed_at), { addSuffix: true })}
            </span>
          )}
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="h-6 gap-1 px-2 text-[11px] text-muted-foreground hover:text-foreground"
          onClick={() => query.refetch()}
          disabled={query.isFetching}
        >
          <SparklesIcon className="h-3 w-3" />
          Regenerate
        </Button>
      </div>
      <DashedShell className="border-solid bg-muted/20">
        <SummaryBody summary={summary!} />
      </DashedShell>
    </div>
  );
}
