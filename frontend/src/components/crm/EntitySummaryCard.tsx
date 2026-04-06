import type { ElementType } from 'react';
import { Alert01Icon, ArrowRight02Icon, Loading01Icon, SparklesIcon, ChartIncreaseIcon, UserGroupIcon, ZapIcon } from '@/lib/icons';
import { formatDistanceToNow } from 'date-fns';

import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useContactSummary, useDealSummary } from '@/hooks/queries';
import type { CRMEntitySummary, SummaryHighlight } from '@/lib/crmTypes';

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

function statusLabel(summary: CRMEntitySummary): string {
  switch (summary.status) {
    case 'pending_refresh':
      return 'Refreshing';
    case 'stale':
      return 'Stale';
    case 'error':
      return 'Error';
    default:
      return 'Ready';
  }
}

function statusVariant(summary: CRMEntitySummary): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (summary.status) {
    case 'error':
      return 'destructive';
    case 'stale':
      return 'outline';
    case 'pending_refresh':
      return 'secondary';
    default:
      return 'default';
  }
}

function SummaryBody({ summary }: { summary: CRMEntitySummary }) {
  const hasSummary = summary.summary_markdown.trim().length > 0;

  return (
    <>
      {hasSummary ? (
        <div className="whitespace-pre-wrap text-sm leading-6 text-foreground/90">
          {summary.summary_markdown}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">
          Summary generation has been queued. This record will populate after the background automation finishes.
        </p>
      )}

      {summary.highlights.length > 0 ? (
        <div className="mt-4 grid gap-2">
          {summary.highlights.map((highlight, index) => {
            const Icon = highlightIcons[highlight.kind] ?? SparklesIcon;
            return (
              <div key={`${highlight.kind}-${index}`} className="flex items-start gap-2 rounded-sm border border-border/60 bg-muted/30 px-3 py-2">
                <Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <p className="text-xs leading-5 text-foreground/85">{highlight.text}</p>
              </div>
            );
          })}
        </div>
      ) : null}

      {summary.last_error ? (
        <div className="mt-4 rounded-sm border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
          {summary.status === 'error' ? 'Latest refresh failed.' : 'Latest refresh is delayed.'} {summary.last_error}
        </div>
      ) : null}
    </>
  );
}

export function EntitySummaryCard({ workspaceId, contactId, dealId }: EntitySummaryCardProps) {
  const contactQuery = useContactSummary(workspaceId, contactId ?? '');
  const dealQuery = useDealSummary(workspaceId, dealId ?? '');
  const query = contactId ? contactQuery : dealQuery;
  const summary = query.data;
  const label = contactId ? 'Contact Summary' : 'Deal Summary';

  if (query.isLoading) {
    return (
      <Card className="gap-0 rounded-md border-border/70 py-0">
        <CardHeader className="px-4 py-3">
          <CardTitle className="flex items-center gap-2 text-sm font-semibold">
            <SparklesIcon className="h-4 w-4 text-sky-500" />
            {label}
          </CardTitle>
          <CardDescription className="flex items-center gap-2">
            <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
            Loading latest summary
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  if (!summary) {
    return (
      <Card className="gap-0 rounded-md border-border/70 py-0">
        <CardHeader className="px-4 py-3">
          <CardTitle className="flex items-center gap-2 text-sm font-semibold">
            <SparklesIcon className="h-4 w-4 text-sky-500" />
            {label}
          </CardTitle>
          <CardDescription>
            This summary will appear after CRM email activity or buyer signals have been processed.
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card className="gap-0 rounded-md border-border/70 py-0">
      <CardHeader className="px-4 py-3">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle className="flex items-center gap-2 text-sm font-semibold">
              <SparklesIcon className="h-4 w-4 text-sky-500" />
              {label}
            </CardTitle>
            <CardDescription className="mt-1">
              {summary.computed_at
                ? `Updated ${formatDistanceToNow(new Date(summary.computed_at), { addSuffix: true })}`
                : 'Not generated yet'}
            </CardDescription>
          </div>
          <Badge variant={statusVariant(summary)} className="shrink-0">
            {statusLabel(summary)}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="border-t border-border/60 px-4 py-4">
        <SummaryBody summary={summary} />
      </CardContent>
    </Card>
  );
}
